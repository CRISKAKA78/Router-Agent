"""Fetch only the approved GOST v3.3.0 official artifacts for an isolated PoC."""
import argparse, concurrent.futures, hashlib, json, pathlib, tarfile, urllib.request, zipfile
TAG = 'v3.3.0'
NAMES = ['gost_3.3.0_linux_amd64.tar.gz', 'gost_3.3.0_linux_armv7.tar.gz',
         'gost_3.3.0_windows_amd64.zip', 'checksums.txt']

def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--output', required=True)
    args = parser.parse_args()
    root = pathlib.Path(args.output).resolve()
    root.mkdir(parents=True, exist_ok=True)
    with urllib.request.urlopen('https://api.github.com/repos/go-gost/gost/releases/tags/'+TAG, timeout=30) as r:
        release = json.load(r)
    if release['tag_name'] != TAG or release['draft'] or release['prerelease']:
        raise RuntimeError('Unexpected release metadata')
    (root/'release.json').write_text(json.dumps(release, indent=2), encoding='utf-8')
    assets = {a['name']: a for a in release['assets']}
    def fetch(name):
        path=root/name
        if not path.exists():
            with urllib.request.urlopen(assets[name]['browser_download_url'],timeout=90) as src, path.open('xb') as dst:
                while chunk:=src.read(1024*1024): dst.write(chunk)
        return path
    with concurrent.futures.ThreadPoolExecutor(max_workers=4) as pool: list(pool.map(fetch,NAMES))
    sums={l.split()[1].lstrip('*'): l.split()[0] for l in (root/'checksums.txt').read_text().splitlines()}
    for name in NAMES[:-1]:
        if hashlib.sha256((root/name).read_bytes()).hexdigest() != sums[name]:
            raise RuntimeError(f'Upstream checksum mismatch: {name}; remove only the failed artifact before retrying')
        print('verified', name)
    for arch in ['amd64','armv7']:
        directory=root/('linux-'+arch)
        if not directory.exists():
            with tarfile.open(root/f'gost_3.3.0_linux_{arch}.tar.gz') as t: t.extractall(directory,filter='data')
    directory=root/'windows-amd64'
    if not directory.exists():
        with zipfile.ZipFile(root/'gost_3.3.0_windows_amd64.zip') as z: z.extractall(directory)
    print('Fixed release downloaded; nothing installed or started.')

if __name__=='__main__': main()
