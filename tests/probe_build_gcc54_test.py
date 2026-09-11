"""Local regression for the non-interactive Windows GCC54 build entry; no SSH connections."""
import os
import pathlib
import subprocess
import tempfile
import unittest

ROOT = pathlib.Path(__file__).resolve().parents[1]

@unittest.skipUnless(os.name == 'nt', 'Windows OpenSSH askpass entry')
class PasswordFileBuildTests(unittest.TestCase):
    def run_helper(self, text):
        with tempfile.TemporaryDirectory(prefix='rmp at password ') as directory:
            path = pathlib.Path(directory) / 'password.txt'
            path.write_text(text, encoding='utf-8-sig')
            env = os.environ.copy()
            env['RMP_SSH_PASSWORD_FILE'] = str(path)
            return subprocess.run(['cmd.exe', '/d', '/c', str(ROOT / 'scripts/ssh-password-file.cmd')],
                                  capture_output=True, env=env, timeout=15,
                                  creationflags=subprocess.CREATE_NO_WINDOW)

    def test_special_characters_are_data(self):
        value = 'a b%&!^<>|$"z'
        result = self.run_helper(value + '\r\n')
        self.assertEqual(result.returncode, 0)
        self.assertEqual(result.stdout.decode(), value)

    def test_empty_and_multiline_are_rejected(self):
        for text in ['', '\r\n', 'first\nsecond']:
            with self.subTest(kind='empty' if not text.strip() else 'multiline'):
                result = self.run_helper(text)
                self.assertNotEqual(result.returncode, 0)
                self.assertEqual(result.stdout, b'')

    def test_missing_file_fails_before_upload(self):
        with tempfile.TemporaryDirectory(prefix='rmp-build-test-') as directory:
            result = subprocess.run(['powershell.exe', '-NoLogo', '-NoProfile', '-ExecutionPolicy', 'Bypass',
                                     '-File', str(ROOT / 'probe-build-gcc54.ps1'),
                                     '-PasswordFile', str(pathlib.Path(directory) / 'missing.txt')],
                                    capture_output=True, timeout=15, creationflags=subprocess.CREATE_NO_WINDOW)
        self.assertNotEqual(result.returncode, 0)
        self.assertIn(b'Build failed:', result.stdout)
        self.assertNotIn(b'Uploading', result.stdout)

    def test_powershell_syntax(self):
        path = str(ROOT / 'probe-build-gcc54.ps1').replace("'", "''")
        command = "$tokens=$null;$errors=$null;[void][Management.Automation.Language.Parser]::ParseFile('" + path + "',[ref]$tokens,[ref]$errors);if($errors.Count){$errors;exit 1}"
        result = subprocess.run(['powershell.exe', '-NoLogo', '-NoProfile', '-Command', command],
                                capture_output=True, timeout=15, creationflags=subprocess.CREATE_NO_WINDOW)
        self.assertEqual(result.returncode, 0, result.stderr.decode(errors='replace'))

if __name__ == '__main__':
    unittest.main()
