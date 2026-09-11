"""Apply the bounded patch to an unpacked, writable go-gost/x v0.16.0 copy.
Never point this at the Go module cache. No downloads, dependency edits or builds.
"""
import argparse, hashlib, json, pathlib, subprocess

def main():
    ap=argparse.ArgumentParser();ap.add_argument('--source',required=True);a=ap.parse_args()
    root=pathlib.Path(a.source).resolve();here=pathlib.Path(__file__).resolve().parent/'patches'
    if not root.is_dir() or 'pkg/mod' in root.as_posix().lower():raise ValueError('Use an unpacked writable source copy, not the module cache')
    manifest=json.loads((here/'manifest.json').read_text(encoding='utf-8'))
    paths=[]
    for name,hashes in manifest['files'].items():
        p=(root/name).resolve()
        if not p.is_relative_to(root):raise ValueError('Patch path escapes source')
        digest=hashlib.sha256(p.read_bytes()).hexdigest() if p.exists() else None
        if digest!=hashes['before']:raise ValueError('Source mismatch (or already patched): '+name)
        paths.append((p,hashes['after']))
    patch=str(here/'x-v0.16.0-registration-udp.patch')
    subprocess.run(['git','-c','core.autocrlf=false','apply','--check',patch],cwd=root,check=True)
    subprocess.run(['git','-c','core.autocrlf=false','apply',patch],cwd=root,check=True)
    for p,want in paths:
        if hashlib.sha256(p.read_bytes()).hexdigest()!=want:raise RuntimeError('Patched content mismatch: '+str(p))
    print('Applied verified go-gost/x v0.16.0 PoC patch; build separately.')
if __name__=='__main__':main()
