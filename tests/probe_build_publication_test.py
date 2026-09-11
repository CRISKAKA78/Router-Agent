"""Test the real Bash orchestration with explicit compiler fixtures, not hardware builds.
Run on a Linux host with Bash, Python 3 and flock.
"""
import os
import pathlib
import subprocess
import sys
import tempfile
import unittest

@unittest.skipUnless(os.name == 'posix', 'Linux orchestration fixtures')
class PublicationTests(unittest.TestCase):
    def run_case(self, failure):
        with tempfile.TemporaryDirectory(prefix='rmp-dual-test-', dir='/root') as directory:
            root = pathlib.Path(directory)
            run = root / 'runs' / 'test-run'
            scripts = run / 'input' / 'scripts'
            scripts.mkdir(parents=True)
            driver = scripts / 'build-probe-all.sh'
            driver.write_text(DRIVER)
            for arch in ('armv7', 'mipsel'):
                (root / ('router-agent-' + arch)).write_bytes(b'previous-' + arch.encode())
            (root / 'latest-build.txt').write_text('previous-run\n')
            # Explicit staged compiler fixtures create minimal ELF headers only.
            for arch, version, machine in [('armv7', '52', 40), ('mipsel', '54', 8)]:
                code = '#!/bin/bash\nset -eu\n'
                argument = '4' if arch == 'armv7' else '3'
                code += 'printf %s "${' + argument + ':-missing}" > "$1/received-interfaces"\n'
                if failure == arch:
                    code += 'echo injected-compiler-failure >&2; exit 17\n'
                else:
                    header = bytearray(52)
                    header[:6] = b'\x7fELF\x01\x01'
                    header[18] = 0 if failure == 'wrong-elf' and arch == 'mipsel' else machine
                    name = 'router-agent-' + arch
                    code += 'mkdir -p "$1/output"\n'
                    code += "python3 - \"$1/output/" + name + "\" <<'PY'\n"
                    code += 'import pathlib,sys\npathlib.Path(sys.argv[1]).write_bytes(' + repr(bytes(header)) + ')\nPY\n'
                    code += 'printf license > "$1/output/MBEDTLS-LICENSE.txt"\n'
                    code += 'printf third-party > "$1/output/THIRD-PARTY.md"\n'
                (scripts / ('build-probe-gcc' + version + '.sh')).write_text(code)
            # The ELF parser above runs unchanged; stub only the ARM attribute reporter.
            tools = root / 'fixture-bin'
            tools.mkdir()
            readelf = tools / 'readelf'
            readelf.write_text('#!/bin/sh\nprintf "  Tag_CPU_arch: v7\\n"\n')
            readelf.chmod(0o700)
            env = os.environ.copy()
            env['PATH'] = str(tools) + ':' + env['PATH']
            result = subprocess.run(['bash', str(driver), str(run), '/root/gcc-5.2', str(root), 'br0,eth0'],
                                    capture_output=True, text=True, env=env, timeout=15)
            self.assertEqual((run / 'armv7/received-interfaces').read_text(), 'br0,eth0')
            if failure == 'armv7':
                self.assertFalse((run / 'mipsel/received-interfaces').exists())
            else:
                self.assertEqual((run / 'mipsel/received-interfaces').read_text(), 'br0,eth0')
            if failure:
                self.assertNotEqual(result.returncode, 0, result.stdout + result.stderr)
                for arch in ('armv7', 'mipsel'):
                    self.assertEqual((root / ('router-agent-' + arch)).read_bytes(), b'previous-' + arch.encode())
                self.assertEqual((root / 'latest-build.txt').read_text(), 'previous-run\n')
            else:
                self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
                for arch in ('armv7', 'mipsel'):
                    self.assertEqual((root / ('router-agent-' + arch)).read_bytes(),
                                     (run / arch / 'output' / ('router-agent-' + arch)).read_bytes())
                self.assertEqual((root / 'latest-build.txt').read_text(), str(run) + '\n')

    def test_arm_failure_preserves_both_outputs(self):
        self.run_case('armv7')

    def test_mips_failure_preserves_both_outputs(self):
        self.run_case('mipsel')

    def test_wrong_elf_preserves_both_outputs(self):
        self.run_case('wrong-elf')

    def test_success_publishes_both_outputs_and_pointer(self):
        self.run_case(None)

if __name__ == '__main__':
    if 'DRIVER' not in globals():
        DRIVER = (pathlib.Path(__file__).resolve().parents[1] / 'scripts/build-probe-all.sh').read_text()
    unittest.main()
