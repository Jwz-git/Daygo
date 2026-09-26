#!/usr/bin/env python3
"""Compile anonymous NSIS fixtures; execute them only on Windows, never Daygo.

Uses the pinned Wails helper, real project and real safeguards. Every runtime
fixture has a unique product/registry identity and a temporary install folder.
"""
import ctypes
import os
from pathlib import Path
import re
import shutil
import subprocess
import tempfile
import unittest
import uuid

ROOT = Path(__file__).resolve().parents[2]
SOURCE = Path(__file__).resolve().parent
COMPILER = shutil.which("makensis")
LANGUAGES = {"English", "SimpChinese", "TradChinese", "Japanese", "Korean",
             "German", "French", "Spanish", "PortugueseBR"}


class SourceContract(unittest.TestCase):
    def test_every_custom_string_is_translated(self):
        source = (SOURCE / "languages.nsh").read_text(encoding="utf-8-sig")
        loaded = set(re.findall(r'MUI_LANGUAGE "([^"]+)"', source))
        self.assertEqual(loaded, LANGUAGES)
        translations = {}
        for key, language, value in re.findall(
                r'LangString (\w+) \$\{LANG_(\w+)\} "(.*)"', source):
            strings = translations.setdefault(language, {})
            self.assertNotIn(key, strings, (language, key))
            strings[key] = value
        english = translations["ENGLISH"]
        self.assertGreaterEqual(len(english), 10)
        for language, strings in translations.items():
            self.assertEqual(set(strings), set(english), language)
            if language != "ENGLISH":
                for key in english:
                    self.assertNotEqual(strings[key], english[key], (language, key))
        self.assertEqual(len(translations), 9)

    def test_uninstall_preserves_unknown_files_and_user_data(self):
        source = (SOURCE / "project.nsi").read_text(encoding="utf-8")
        uninstall = source.split('Section "uninstall"')[1]
        self.assertNotIn("RMDir /r", uninstall)
        self.assertNotIn("$AppData", uninstall)
        self.assertNotIn("$LOCALAPPDATA", uninstall)
        self.assertNotIn("/REBOOTOK", uninstall)


@unittest.skipUnless(COMPILER, "makensis unavailable: install NSIS to compile fixtures")
class InstallerFixture(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.wails = Path(subprocess.check_output(
            ["go", "list", "-m", "-f", "{{.Dir}}", "github.com/wailsapp/wails/v2"],
            cwd=ROOT, text=True).strip())

    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(prefix="daygo-installer-fixture-")
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.product = "DaygoFixture" + uuid.uuid4().hex
        self.key = "Software\\DaygoInstallerFixtures\\" + self.product
        self.project = self.root / "build/windows/installer"
        self.project.mkdir(parents=True)
        self.bin = self.root / "build/bin"
        self.bin.mkdir()
        for name in ("project.nsi", "languages.nsh", "safeguards.nsh"):
            shutil.copyfile(SOURCE / name, self.project / name)
        shutil.copyfile(ROOT / "build/windows/icon.ico", self.project.parent / "icon.ico")
        helper = (self.wails / "pkg/buildassets/build/windows/installer/wails_tools.nsh").read_text()
        for key, value in {"Name": self.product, "Info.CompanyName": self.product,
                           "Info.ProductName": self.product, "Info.ProductVersion": "0.0.1",
                           "Info.Copyright": "Anonymous fixture"}.items():
            helper = helper.replace("{{." + key + "}}", value)
        # The real Wails config has no file associations or custom protocols.
        helper = re.sub(r"{{range [^}]+}}.*?{{end}}", "", helper, flags=re.DOTALL)
        self.assertNotIn("{{.", helper)
        (self.project / "wails_tools.nsh").write_text(helper, encoding="utf-8")
        self.payload = b"anonymous fixture, not a runnable application\n"
        for name in (self.product + ".exe", "daygo_windows_native.dll", "WinSparkle.dll"):
            (self.bin / name).write_bytes(self.payload)
        self.target = self.root / "installed with spaces"
        self.bootstrapper("success")
        if os.name == "nt":
            import winreg
            self.addCleanup(self.clean_registry)
            self.addCleanup(lambda: (Path(os.environ["APPDATA"]) /
                "Microsoft/Windows/Start Menu/Programs" / (self.product + ".lnk")).unlink(missing_ok=True))

    def compile(self, scope="user", expect_success=True):
        prefix = "/" if os.name == "nt" else "-"
        args = [COMPILER, prefix + "V2", prefix + "WX",
                prefix + "DARG_WAILS_AMD64_BINARY=" + str(self.bin / (self.product + ".exe")),
                prefix + "DDAYGO_WEBVIEW_MACHINE_KEY=" + self.key,
                prefix + "DDAYGO_WEBVIEW_USER_KEY=" + self.key]
        if scope == "user":
            args += [prefix + "DWAILS_INSTALL_SCOPE=user", prefix + "DREQUEST_EXECUTION_LEVEL=user"]
        args.append(str(self.project / "project.nsi"))
        result = subprocess.run(args, cwd=self.project, capture_output=True, text=True)
        if expect_success:
            self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        else:
            self.assertNotEqual(result.returncode, 0)
        return self.bin / (self.product + "-amd64-installer.exe")

    def bootstrapper(self, outcome):
        temp = self.project / "tmp"
        temp.mkdir(exist_ok=True)
        exe = temp / "MicrosoftEdgeWebview2Setup.exe"
        if os.name != "nt":
            exe.write_bytes(self.payload)
            return
        action = ('WriteRegStr HKCU "' + self.key + '" "pv" "1.2.3.4"' if outcome == "success"
                  else "SetErrorLevel " + ("42" if outcome == "failure" else "0"))
        script = temp / "bootstrapper.nsi"
        script.write_text('Unicode true\nRequestExecutionLevel user\nSilentInstall silent\n'
            'OutFile "' + str(exe) + '"\nSection\nSetRegView 64\n' + action +
            '\nSectionEnd\n', encoding="utf-8")
        result = subprocess.run([COMPILER, "/V2", "/WX", str(script)],
                                capture_output=True, text=True)
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)

    def clean_registry(self):
        import winreg
        for key in (self.key, "Software\\Microsoft\\Windows\\CurrentVersion\\Uninstall\\" +
                    self.product * 2):
            try:
                winreg.DeleteKeyEx(winreg.HKEY_CURRENT_USER, key, winreg.KEY_WOW64_64KEY)
            except FileNotFoundError:
                pass

    def run_installer(self, executable):
        # NSIS /D= and _?= must be last and UNQUOTED, even for paths with spaces.
        # A subprocess argument list would quote the entire special parameter.
        return subprocess.run('"' + str(executable) + '" /S /D=' + str(self.target),
                              timeout=30).returncode

    def test_compile_both_scopes(self):
        for scope in ("machine", "user"):
            with self.subTest(scope=scope):
                self.compile(scope)

    def test_missing_dll_fails_packaging(self):
        for name in ("daygo_windows_native.dll", "WinSparkle.dll"):
            with self.subTest(name=name):
                (self.bin / name).unlink()
                self.compile(expect_success=False)
                (self.bin / name).write_bytes(self.payload)

    def write_runtime_version(self, value):
        import winreg
        with winreg.CreateKeyEx(winreg.HKEY_CURRENT_USER, self.key, 0,
                winreg.KEY_WRITE | winreg.KEY_WOW64_64KEY) as key:
            winreg.SetValueEx(key, "pv", 0, winreg.REG_SZ, value)

    @unittest.skipUnless(os.name == "nt", "Windows execution required")
    def test_success_and_uninstall_preserve_unknown_file(self):
        self.assertEqual(self.run_installer(self.compile()), 0)
        self.assertEqual((self.target / (self.product + ".exe")).read_bytes(), self.payload)
        sentinel = self.target / "must-not-delete.txt"
        sentinel.write_text("unowned data", encoding="utf-8")
        self.assertEqual(subprocess.run('"' + str(self.target / "uninstall.exe") +
            '" /S _?=' + str(self.target), timeout=30).returncode, 0)
        self.assertEqual(sentinel.read_text(), "unowned data")
        self.assertFalse((self.target / (self.product + ".exe")).exists())
        self.assertFalse((self.target / "daygo_windows_native.dll").exists())

    @unittest.skipUnless(os.name == "nt", "Windows execution required")
    def test_bootstrapper_failure_does_not_install_payload(self):
        for outcome in ("failure", "false-success"):
            with self.subTest(outcome=outcome):
                self.bootstrapper(outcome)
                self.assertEqual(self.run_installer(self.compile()), 20)
                self.assertFalse((self.target / (self.product + ".exe")).exists())

    @unittest.skipUnless(os.name == "nt", "Windows execution required")
    def test_existing_runtime_skips_failing_bootstrapper_and_preserves_data(self):
        self.bootstrapper("failure")
        self.write_runtime_version("1.2.3.4")
        self.target.mkdir()
        sentinel = self.target / "must-keep.db"
        sentinel.write_bytes(b"anonymous fixture")
        self.assertEqual(self.run_installer(self.compile()), 0)
        self.assertEqual(sentinel.read_bytes(), b"anonymous fixture")

    @unittest.skipUnless(os.name == "nt", "Windows execution required")
    def test_zero_runtime_version_is_not_available(self):
        self.bootstrapper("failure")
        self.write_runtime_version("0.0.0.0")
        self.assertEqual(self.run_installer(self.compile()), 20)
        self.assertFalse((self.target / (self.product + ".exe")).exists())

    @unittest.skipUnless(os.name == "nt", "Windows execution required")
    def test_locked_payload_is_unchanged(self):
        installer = self.compile()
        self.target.mkdir()
        dll = self.target / "daygo_windows_native.dll"
        dll.write_bytes(b"old fixture")
        kernel = ctypes.WinDLL("kernel32", use_last_error=True)
        kernel.CreateFileW.argtypes = [ctypes.c_wchar_p, ctypes.c_uint32, ctypes.c_uint32,
            ctypes.c_void_p, ctypes.c_uint32, ctypes.c_uint32, ctypes.c_void_p]
        kernel.CreateFileW.restype = ctypes.c_void_p
        kernel.CloseHandle.argtypes = [ctypes.c_void_p]
        handle = kernel.CreateFileW(str(dll), 0x80000000, 0, None, 3, 0, None)
        self.assertNotEqual(handle, ctypes.c_void_p(-1).value)
        try:
            self.assertEqual(self.run_installer(installer), 10)
        finally:
            kernel.CloseHandle(handle)
        self.assertEqual(dll.read_bytes(), b"old fixture")


if __name__ == "__main__":
    unittest.main(verbosity=2)
