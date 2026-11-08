#!/usr/bin/env python3
"""Bundle existing verified builds for the fixed RelayDesk preview release."""
import hashlib
import json
from pathlib import Path
import shutil
import subprocess
import zipfile

ROOT = Path(__file__).resolve().parents[1]
TAG = 'v0.1.0-preview.1'
OUT = ROOT / 'dist' / 'release'

def command(*args):
    return subprocess.check_output(args, cwd=ROOT, text=True).strip()

def main():
    version = command(str(ROOT / 'dist/relaydesk-linux-amd64'), 'version')
    if version != 'RelayDesk 0.1.0':
        raise SystemExit('Update release tag, asset names and download documentation for the new version.')
    commit = command('git', 'rev-parse', 'HEAD')
    OUT.mkdir(exist_ok=True)
    for name in ('relaydesk.exe', 'relaydesk-linux-amd64', 'relaydesk_0.1.0_amd64.deb',
                 'LICENSE', 'NOTICE', 'THIRD_PARTY_NOTICES.md'):
        shutil.copyfile(ROOT / 'dist' / name, OUT / name)
        (OUT / name).chmod(0o755 if name == 'relaydesk-linux-amd64' else 0o644)
    quick = (ROOT / 'docs/RELEASE_QUICKSTART.txt').read_text().replace('@COMMIT@', commit)
    (OUT / 'QUICKSTART.txt').write_text(quick)
    (OUT / 'BUILDINFO.txt').write_text('\n'.join([
        'Product: RelayDesk 0.1.0', 'Release tag: ' + TAG,
        'Source commit: ' + commit, 'Toolchain: ' + command('go', 'version'),
        'Targets: linux/amd64, windows/amd64; CGO_ENABLED=0',
        'Build: sh packaging/build.sh',
        'Windows artifact: unsigned portable EXE, not Setup.exe',
        'Checksums: SHA-256, unsigned',
        'Acceptance limits: docs/STATUS.md and docs/MANUAL_ACCEPTANCE.md', '']))
    archive_path = OUT / 'RelayDesk-0.1.0-windows-amd64.zip'
    with zipfile.ZipFile(archive_path, 'w', zipfile.ZIP_DEFLATED, compresslevel=9) as archive:
        for name in ('relaydesk.exe', 'LICENSE', 'NOTICE', 'THIRD_PARTY_NOTICES.md'):
            archive.write(OUT / name, 'dist/' + name)
        archive.write(ROOT / 'packaging/install-windows.ps1', 'packaging/install-windows.ps1')
        archive.write(ROOT / 'docs/MANUAL_ACCEPTANCE.md', 'docs/MANUAL_ACCEPTANCE.md')
        for name in ('QUICKSTART.txt', 'BUILDINFO.txt'):
            archive.write(OUT / name, name)
    with zipfile.ZipFile(archive_path) as archive:
        assert archive.testzip() is None
        assert archive.read('dist/relaydesk.exe') == (OUT / 'relaydesk.exe').read_bytes()
    files = sorted(path for path in OUT.iterdir() if path.is_file() and path.name != 'SHA256SUMS')
    (OUT / 'SHA256SUMS').write_text(''.join(
        hashlib.sha256(path.read_bytes()).hexdigest() + '  ' + path.name + '\n' for path in files))
    notes = (ROOT / 'docs/RELEASE_NOTES.md').read_text().replace('@COMMIT@', commit)
    (ROOT / 'dist/release-notes.md').write_text(notes)
    print(json.dumps({'tag': TAG, 'commit': commit, 'assets': [p.name for p in files] + ['SHA256SUMS']}))

if __name__ == '__main__':
    main()
