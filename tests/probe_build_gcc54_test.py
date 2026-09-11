"""Windows regressions for the unified Probe build entry; no build-host connection."""
import os
import pathlib
import socket
import subprocess
import tempfile
import unittest

ROOT = pathlib.Path(__file__).resolve().parents[1]
ENTRIES = ('probe-build.ps1', 'probe-build-gcc54.ps1')

@unittest.skipUnless(os.name == 'nt', 'Windows OpenSSH entry')
class PasswordFileBuildTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.helper_dir = tempfile.TemporaryDirectory(prefix='rmp-helper-')
        cls.helper = pathlib.Path(cls.helper_dir.name) / 'askpass.exe'
        compiler = pathlib.Path(os.environ['WINDIR']) / 'Microsoft.NET/Framework64/v4.0.30319/csc.exe'
        subprocess.run([str(compiler), '/nologo', '/target:exe', '/out:' + str(cls.helper),
                        str(ROOT / 'scripts/ssh-askpass.cs')], check=True, capture_output=True)

    @classmethod
    def tearDownClass(cls):
        cls.helper_dir.cleanup()

    def entry(self, entry, *args):
        return subprocess.run(['powershell.exe', '-NoLogo', '-NoProfile', '-ExecutionPolicy', 'Bypass',
                               '-File', str(ROOT / entry), *args], capture_output=True, timeout=25,
                              creationflags=subprocess.CREATE_NO_WINDOW)

    def test_native_helper_reads_special_characters_and_chinese_path(self):
        with tempfile.TemporaryDirectory(prefix='rmp password ') as directory:
            path = pathlib.Path(directory) / '中文 密码.txt'
            value = 'a b%&!^<>|$"z'
            path.write_text(value + '\r\n', encoding='utf-8-sig')
            env = os.environ.copy()
            env['RMP_KEY_PASSWORD_FILE'] = str(path)
            result = subprocess.run([str(self.helper)], capture_output=True, env=env, timeout=15,
                                    creationflags=subprocess.CREATE_NO_WINDOW)
            self.assertEqual(result.returncode, 0)
            self.assertEqual(result.stdout.decode(), value)

    def test_empty_and_multiline_are_rejected_before_ssh(self):
        for entry in ENTRIES:
            for text in ['', '\r\n', 'first\nsecond']:
                with self.subTest(entry=entry, text=text), tempfile.TemporaryDirectory() as directory:
                    path = pathlib.Path(directory) / 'password.txt'
                    path.write_text(text, encoding='utf-8-sig')
                    result = self.entry(entry, '-PasswordFile', str(path))
                    self.assertNotEqual(result.returncode, 0)
                    self.assertIn(b'one non-empty password line', result.stdout)
                    self.assertNotIn(b'Checking SSH', result.stdout)
                    self.assertNotIn(b'Uploading', result.stdout)

    def test_missing_file_fails_before_upload(self):
        for entry in ENTRIES:
            with self.subTest(entry=entry), tempfile.TemporaryDirectory() as directory:
                result = self.entry(entry, '-PasswordFile', str(pathlib.Path(directory) / 'missing.txt'))
                self.assertNotEqual(result.returncode, 0)
                self.assertIn(b'Build failed:', result.stdout)
                self.assertNotIn(b'Checking SSH', result.stdout)
                self.assertNotIn(b'Uploading', result.stdout)

    def test_invalid_interfaces_are_rejected(self):
        for value in ['eth0,eth0', '..', 'eth0;', 'a' * 16]:
            with self.subTest(value=value):
                result = self.entry('probe-build.ps1', '-NetworkInterfaces', value)
                self.assertNotEqual(result.returncode, 0)
                self.assertNotIn(b'Checking SSH', result.stdout)

    def test_ssh_failure_reports_stage_without_upload(self):
        # Reserve a local port without listening: real connection refusal, not a fake SSH success.
        with socket.socket() as sock, tempfile.TemporaryDirectory() as directory:
            sock.bind(('127.0.0.1', 0))
            path = pathlib.Path(directory) / 'password.txt'
            path.write_text('test-only-not-a-real-password', encoding='utf-8')
            result = self.entry('probe-build.ps1', '-SshTarget', 'root@127.0.0.1',
                                '-Port', str(sock.getsockname()[1]), '-PasswordFile', str(path))
            self.assertNotEqual(result.returncode, 0)
            self.assertIn(b'SSH login failed', result.stdout)
            self.assertNotIn(b'Uploading', result.stdout)
            self.assertNotIn(b'CopyTo', result.stdout)

    def test_powershell_syntax(self):
        for entry in ENTRIES:
            with self.subTest(entry=entry):
                path = str(ROOT / entry).replace("'", "''")
                command = "$tokens=$null;$errors=$null;[void][Management.Automation.Language.Parser]::ParseFile('" + path + "',[ref]$tokens,[ref]$errors);if($errors.Count){$errors;exit 1}"
                result = subprocess.run(['powershell.exe', '-NoLogo', '-NoProfile', '-Command', command],
                                        capture_output=True, timeout=15, creationflags=subprocess.CREATE_NO_WINDOW)
                self.assertEqual(result.returncode, 0, result.stderr.decode(errors='replace'))

if __name__ == '__main__':
    unittest.main()
