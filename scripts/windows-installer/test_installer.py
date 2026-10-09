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
import time
import unittest
import uuid
import xml.etree.ElementTree as ET

ROOT = Path(__file__).resolve().parents[2]
SOURCE = Path(__file__).resolve().parent
COMPILER = shutil.which("makensis")
LANGUAGES = {"English", "SimpChinese", "TradChinese", "Japanese", "Korean",
             "German", "French", "Spanish", "PortugueseBR"}


class SourceContract(unittest.TestCase):
    def test_notification_uninstall_identity_matches_adapter(self):
        source = (SOURCE / "project.nsi").read_text(encoding="utf-8")
        adapter = (ROOT / "internal/platform/windows/notifications_windows.go").read_text()
        for define, constant in (("DAYGO_NOTIFICATION_APP_ID", "notificationAppID"),
                                 ("DAYGO_NOTIFICATION_CLSID", "notificationCLSID")):
            identity = re.search(r'const ' + constant + r' = "([^"]+)"', adapter).group(1)
            self.assertIn('!define ' + define + ' "' + identity + '"', source)
            self.assertIn('${' + define + '}', source.split('Section "uninstall"')[1])

    def test_windows_feed_marks_installer_as_update(self):
        with tempfile.TemporaryDirectory(prefix="daygo-appcast-fixture-") as directory:
            root = Path(directory)
            mac, windows, output = root / "fixture.dmg", root / "fixture.exe", root / "appcast.xml"
            mac.write_bytes(b"anonymous mac fixture")
            windows.write_bytes(b"anonymous windows fixture")
            subprocess.run([os.sys.executable, str(ROOT / "scripts/generate-appcast.py"),
                "--version", "0.0.2", "--mac", str(mac), "--mac-signature", "fixture-mac-signature",
                "--windows", str(windows), "--windows-signature", "fixture-windows-signature",
                "--output", str(output)], check=True)
            sparkle = "{http://www.andymatuschak.org/xml-namespaces/sparkle}"
            enclosures = ET.parse(output).findall("./channel/item/enclosure")
            self.assertEqual(len(enclosures), 2)
            for enclosure in enclosures:
                if enclosure.get(sparkle + "os") == "windows":
                    self.assertEqual(enclosure.get(sparkle + "installerArguments"), "/DAYGO_UPDATE")
                else:
                    self.assertIsNone(enclosure.get(sparkle + "installerArguments"))

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
        self.notification_id = self.product + ".Notifications"
        self.notification_clsid = "{" + str(uuid.uuid4()).upper() + "}"
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
            self.addCleanup(lambda: shutil.rmtree(Path(os.environ["LOCALAPPDATA"]) /
                "Programs" / self.product, ignore_errors=True))
            self.addCleanup(lambda: (Path(os.environ["APPDATA"]) /
                "Microsoft/Windows/Start Menu/Programs" / (self.product + ".lnk")).unlink(missing_ok=True))

    def compile(self, scope="user", expect_success=True, wait_ticks=None):
        prefix = "/" if os.name == "nt" else "-"
        args = [COMPILER, prefix + "V2", prefix + "WX",
                prefix + "DARG_WAILS_AMD64_BINARY=" + str(self.bin / (self.product + ".exe")),
                prefix + "DDAYGO_WEBVIEW_MACHINE_KEY=" + self.key,
                prefix + "DDAYGO_WEBVIEW_USER_KEY=" + self.key,
                prefix + "DDAYGO_NOTIFICATION_APP_ID=" + self.notification_id,
                prefix + "DDAYGO_NOTIFICATION_CLSID=" + self.notification_clsid]
        if scope == "user":
            args += [prefix + "DWAILS_INSTALL_SCOPE=user", prefix + "DREQUEST_EXECUTION_LEVEL=user"]
        if wait_ticks is not None:
            args += [prefix + "DDAYGO_UPDATE_WAIT_TICKS=" + str(wait_ticks)]
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
        clsid_key = "Software\\Classes\\CLSID\\" + self.notification_clsid
        for key in (clsid_key + "\\LocalServer32", clsid_key):
            try:
                winreg.DeleteKey(winreg.HKEY_CURRENT_USER, key)
            except FileNotFoundError:
                pass
        for key in (self.key, "Software\\Microsoft\\Windows\\CurrentVersion\\Uninstall\\" +
                    self.product * 2, "Software\\Classes\\AppUserModelId\\" + self.notification_id):
            try:
                winreg.DeleteKeyEx(winreg.HKEY_CURRENT_USER, key, winreg.KEY_WOW64_64KEY)
            except FileNotFoundError:
                pass

    def installer_command(self, executable, update=False, explicit_directory=True):
        # NSIS /D= and _?= must be last and UNQUOTED, even for paths with spaces.
        # A subprocess argument list would quote the entire special parameter.
        command = '"' + str(executable) + '" /S'
        if update:
            command += " /DAYGO_UPDATE"
        if explicit_directory:
            command += " /D=" + str(self.target)
        return command

    def run_installer(self, executable, update=False, explicit_directory=True):
        return subprocess.run(self.installer_command(executable, update, explicit_directory),
                              timeout=40).returncode

    def lock_file(self, path):
        kernel = ctypes.WinDLL("kernel32", use_last_error=True)
        kernel.CreateFileW.argtypes = [ctypes.c_wchar_p, ctypes.c_uint32, ctypes.c_uint32,
            ctypes.c_void_p, ctypes.c_uint32, ctypes.c_uint32, ctypes.c_void_p]
        kernel.CreateFileW.restype = ctypes.c_void_p
        kernel.CloseHandle.argtypes = [ctypes.c_void_p]
        handle = kernel.CreateFileW(str(path), 0x80000000, 0, None, 3, 0, None)
        self.assertNotEqual(handle, ctypes.c_void_p(-1).value)
        return lambda: kernel.CloseHandle(handle)

    @unittest.skipUnless(os.name == "nt", "Windows execution required")
    def test_update_waits_for_old_process_files(self):
        installer = self.compile()
        self.target.mkdir()
        dll = self.target / "daygo_windows_native.dll"
        dll.write_bytes(b"old fixture")
        unlock = self.lock_file(dll)
        process = subprocess.Popen(self.installer_command(installer, update=True))
        try:
            time.sleep(1)
            self.assertIsNone(process.poll(), "update failed before old files were released")
            self.assertFalse((self.target / (self.product + ".exe")).exists())
        finally:
            unlock()
            result = process.wait(timeout=40)
        self.assertEqual(result, 0)
        self.assertEqual(dll.read_bytes(), self.payload)

    @unittest.skipUnless(os.name == "nt", "Windows execution required")
    def test_update_timeout_preserves_all_payload(self):
        installer = self.compile(wait_ticks=3)
        self.target.mkdir()
        paths = [self.target / name for name in
                 (self.product + ".exe", "daygo_windows_native.dll", "WinSparkle.dll")]
        for path in paths:
            path.write_bytes(b"old fixture")
        unlock = self.lock_file(paths[-1])
        try:
            self.assertEqual(self.run_installer(installer, update=True), 10)
        finally:
            unlock()
        for path in paths:
            self.assertEqual(path.read_bytes(), b"old fixture")

    @unittest.skipUnless(os.name == "nt", "Windows execution required")
    def test_update_uses_registered_custom_directory(self):
        import winreg
        installer = self.compile()
        self.assertEqual(self.run_installer(installer), 0)
        key = "Software\\Microsoft\\Windows\\CurrentVersion\\Uninstall\\" + self.product * 2
        # Cover both new InstallLocation and older Wails DisplayIcon records.
        for legacy in (False, True):
            with self.subTest(legacy=legacy):
                if legacy:
                    with winreg.OpenKey(winreg.HKEY_CURRENT_USER, key, 0,
                            winreg.KEY_WRITE | winreg.KEY_WOW64_64KEY) as handle:
                        winreg.DeleteValue(handle, "InstallLocation")
                target = self.target / (self.product + ".exe")
                target.write_bytes(b"old fixture")
                self.assertEqual(self.run_installer(installer, update=True, explicit_directory=False), 0)
                self.assertEqual(target.read_bytes(), self.payload)

    @unittest.skipUnless(os.name == "nt", "Windows execution required")
    def test_update_explicit_directory_overrides_registration(self):
        installer = self.compile()
        self.assertEqual(self.run_installer(installer), 0)
        original = self.target
        self.target = self.root / "another directory with spaces"
        self.assertEqual(self.run_installer(installer, update=True), 0)
        self.assertTrue((self.target / (self.product + ".exe")).exists())
        self.assertTrue((original / (self.product + ".exe")).exists())

    @unittest.skipUnless(os.name == "nt", "Windows execution required")
    def test_update_without_target_fails_before_install(self):
        self.assertEqual(self.run_installer(self.compile(), update=True, explicit_directory=False), 30)
        default = Path(os.environ["LOCALAPPDATA"]) / "Programs" / self.product
        self.assertFalse(default.exists())

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
        import winreg
        self.assertEqual(self.run_installer(self.compile()), 0)
        self.assertEqual((self.target / (self.product + ".exe")).read_bytes(), self.payload)
        sentinel = self.target / "must-not-delete.txt"
        sentinel.write_text("unowned data", encoding="utf-8")
        notification_key = "Software\\Classes\\AppUserModelId\\" + self.notification_id
        activator_key = "Software\\Classes\\CLSID\\" + self.notification_clsid
        for path in (notification_key, activator_key + "\\LocalServer32"):
            with winreg.CreateKey(winreg.HKEY_CURRENT_USER, path) as key:
                winreg.SetValueEx(key, "", 0, winreg.REG_SZ, "anonymous fixture")
        unrelated_key = notification_key + ".Other"
        with winreg.CreateKey(winreg.HKEY_CURRENT_USER, unrelated_key) as key:
            winreg.SetValueEx(key, "", 0, winreg.REG_SZ, "unowned fixture")
        self.addCleanup(lambda: winreg.DeleteKey(winreg.HKEY_CURRENT_USER, unrelated_key))
        self.assertEqual(subprocess.run('"' + str(self.target / "uninstall.exe") +
            '" /S _?=' + str(self.target), timeout=30).returncode, 0)
        self.assertEqual(sentinel.read_text(), "unowned data")
        self.assertFalse((self.target / (self.product + ".exe")).exists())
        self.assertFalse((self.target / "daygo_windows_native.dll").exists())
        for path in (notification_key, activator_key):
            with self.assertRaises(FileNotFoundError):
                winreg.OpenKey(winreg.HKEY_CURRENT_USER, path)
        with winreg.OpenKey(winreg.HKEY_CURRENT_USER, unrelated_key) as key:
            self.assertEqual(winreg.QueryValueEx(key, "")[0], "unowned fixture")

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
        unlock = self.lock_file(dll)
        try:
            self.assertEqual(self.run_installer(installer), 10)
        finally:
            unlock()
        self.assertEqual(dll.read_bytes(), b"old fixture")


if __name__ == "__main__":
    unittest.main(verbosity=2)
