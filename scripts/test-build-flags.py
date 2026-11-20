#!/usr/bin/env python3
"""Check production/dev linker arguments without compiling or packaging an app."""
import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[1]


class BuildFlags(unittest.TestCase):
    def test_shell_linker_flags(self):
        for development in (False, True):
            with self.subTest(development=development), tempfile.TemporaryDirectory() as directory:
                root = Path(directory)
                (root / "scripts").mkdir()
                for name in ("build-desktop.sh", "bootstrap-frontend.sh"):
                    shutil.copyfile(ROOT / "scripts" / name, root / "scripts" / name)
                tools = root / "tools"
                tools.mkdir()
                log = root / "args.json"
                go = tools / "go"
                go.write_text("#!/usr/bin/env python3\nimport json,os,sys\n"
                              "if sys.argv[1:] == ['env','GOOS']: print('linux')\n"
                              "elif sys.argv[1:] == ['env','GOARCH']: print('amd64')\n"
                              "else: open(os.environ['DAYGO_FLAGS_LOG'],'w').write(json.dumps(sys.argv[1:]))\n")
                go.chmod(0o755)
                pkg = tools / "pkg-config"
                pkg.write_text("#!/bin/sh\nexit 0\n")
                pkg.chmod(0o755)
                env = dict(os.environ, PATH=str(tools) + os.pathsep + os.environ["PATH"],
                           DAYGO_SKIP_FRONTEND="1", DAYGO_DEV="1" if development else "0",
                           DAYGO_FLAGS_LOG=str(log))
                env.pop("DAYGO_LINUX_WEBKIT_TAG", None)
                result = subprocess.run(["bash", str(root / "scripts/build-desktop.sh")],
                                        env=env, capture_output=True, text=True, timeout=15)
                self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
                args = json.loads(log.read_text())
                tags = args[args.index("-tags") + 1].split(",")
                self.assertEqual("production" in tags, not development)
                if development:
                    self.assertNotIn("-ldflags", args)
                else:
                    flags = args[args.index("-ldflags") + 1].split()
                    self.assertIn("-s", flags)
                    self.assertIn("-w", flags)


if __name__ == "__main__":
    unittest.main()
