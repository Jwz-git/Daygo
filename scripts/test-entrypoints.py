#!/usr/bin/env python3
"""Run shell entrypoints in an anonymous checkout with tool doubles.

Never launch a GUI, stop a real process, install dependencies, or use user data.
Unlike bash -n, this exercises sourced helper names and argument forwarding.
"""
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[1]


class ShellEntrypoints(unittest.TestCase):
    def check_entrypoint(self, name):
        with tempfile.TemporaryDirectory(prefix="daygo-entrypoint-") as directory:
            root = Path(directory) / "anonymous checkout"
            scripts = root / "scripts"
            scripts.mkdir(parents=True)
            for source in (name, "bootstrap-frontend.sh"):
                shutil.copyfile(ROOT / "scripts" / source, scripts / source)
            frontend = root / "frontend" / "dist"
            frontend.mkdir(parents=True)
            (frontend / "index.html").write_text('<script src="assets/fixture.js"></script>')
            tools = root / "tools"
            tools.mkdir()
            log = root / "calls.log"
            env_log = root / "go-env.log"
            for tool in ("go", "npm", "pkill", "sleep", "clang", "pkg-config"):
                stub = tools / tool
                stub.write_text('#!/bin/sh\nprintf "%s\\n" "' + tool + ' $*" >> "$DAYGO_ENTRY_LOG"\n')
                if tool == "go":
                    with stub.open("a") as stream:
                        stream.write('printf "%s\\n" "CGO=${CGO_ENABLED:-unset} $*" >> "$DAYGO_ENV_LOG"\n')
                stub.chmod(0o755)
            uname = tools / "uname"
            uname.write_text('#!/bin/sh\nprintf "Darwin\\n"\n')
            uname.chmod(0o755)
            env = dict(os.environ, PATH=str(tools) + os.pathsep + os.environ["PATH"],
                       DAYGO_ENTRY_LOG=str(log), DAYGO_ENV_LOG=str(env_log), CGO_ENABLED="1")
            env.pop("DAYGO_LINUX_WEBKIT_TAG", None)
            result = subprocess.run(["bash", str(scripts / name), "-help"], cwd=root,
                                    env=env, text=True, capture_output=True, timeout=15)
            self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
            calls = log.read_text().splitlines()
            cli = "go run github.com/wailsapp/wails/v3/cmd/wails3@v3.1.0"
            self.assertTrue(any(line.startswith(cli + " generate bindings ") for line in calls), calls)
            command = "build" if name == "build-linux.sh" else "dev"
            self.assertEqual(calls[-1], cli + " " + command + " -help")
            environments = env_log.read_text().splitlines()
            self.assertTrue(any(line.startswith("CGO=0 run ") and " generate bindings " in line
                                for line in environments), environments)
            self.assertTrue(environments[-1].startswith("CGO=1 run "), environments)

    def test_macos_dev(self):
        self.check_entrypoint("dev.sh")

    def test_linux_dev(self):
        self.check_entrypoint("dev-linux.sh")

    def test_linux_build(self):
        self.check_entrypoint("build-linux.sh")


if __name__ == "__main__":
    unittest.main()
