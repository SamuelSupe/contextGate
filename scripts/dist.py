#!/usr/bin/env python3
"""Build versioned Linux archives from the final Docker image binaries."""
import argparse
import hashlib
import json
import pathlib
import re
import shutil
import subprocess
import tarfile
import tempfile
import uuid

ROOT = pathlib.Path(__file__).resolve().parents[1]


def run(*args):
    return subprocess.check_output(args, cwd=ROOT, text=True).strip()


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--version', default='0.1.0')
    parser.add_argument('--arch', action='append', choices=['arm64', 'amd64'])
    parser.add_argument('--build-ca', help='Optional trusted dependency-download CA')
    args = parser.parse_args()
    if not re.fullmatch(r'\d+\.\d+\.\d+', args.version):
        parser.error('version must be major.minor.patch')
    commit = run('git', 'rev-parse', 'HEAD')
    if run('git', 'status', '--porcelain'):
        parser.error('commit the release source before packaging')
    if f'const Version = "{args.version}"' not in (ROOT/'internal/version/version.go').read_text():
        parser.error('version does not match internal/version/version.go')
    licenses = ROOT/'third_party/licenses'
    if not licenses.is_dir():
        parser.error('collect third-party license notices before packaging')
    output = ROOT/'dist'
    output.mkdir(exist_ok=True)
    for arch in args.arch or ['arm64', 'amd64']:
        image = f'mcpdbhub:{args.version}-{arch}'
        build = ['docker', 'build', '--platform', f'linux/{arch}', '--build-arg', f'VCS_REF={commit}', '-t', image]
        if args.build_ca:
            build += ['--secret', 'id=build_ca,src='+str(pathlib.Path(args.build_ca).resolve())]
        subprocess.run([*build, '.'], cwd=ROOT, check=True)
        version = run('docker', 'run', '--rm', '--platform', f'linux/{arch}', image, 'version')
        if version != f'mcpdbhub {args.version} (commit {commit})':
            raise RuntimeError('image version does not match source commit')
        name = f'mcpdbhub-{args.version}-linux-{arch}'
        with tempfile.TemporaryDirectory(prefix='mcpdbhub-dist-') as temporary:
            package = pathlib.Path(temporary)/name
            package.mkdir()
            for directory in ('lib', 'libexec', 'licenses'):
                (package/directory).mkdir()
            container = 'mcpdbhub-extract-'+uuid.uuid4().hex[:10]
            run('docker', 'create', '--name', container, '--platform', f'linux/{arch}', image)
            try:
                for source, target in [('/usr/local/bin/mcpdbhub', 'libexec/mcpdbhub'),
                                       ('/usr/local/lib/libstdc++.so.6', 'lib/libstdc++.so.6'),
                                       ('/usr/local/lib/libgcc_s.so.1', 'lib/libgcc_s.so.1'),
                                       ('/usr/share/doc/mcpdbhub-runtime', 'licenses/gcc-runtime')]:
                    run('docker', 'cp', container+':'+source, str(package/target))
            finally:
                run('docker', 'rm', container)
            launcher = package/'mcpdbhub'
            launcher.write_text('#!/bin/sh\nset -eu\nhub_root=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)\nexport LD_LIBRARY_PATH="$hub_root/lib${LD_LIBRARY_PATH:+:$LD_LIBRARY_PATH}"\nexec "$hub_root/libexec/mcpdbhub" "$@"\n')
            launcher.chmod(0o755)
            (package/'libexec/mcpdbhub').chmod(0o755)
            for filename in ('README.md', 'README.en.md', 'README.zh-CN.md',
                             'CONTRIBUTING.md', 'CHANGELOG.md', 'SECURITY.md',
                             'THIRD_PARTY_NOTICES.md', 'LICENSE', 'go.mod'):
                if (ROOT/filename).is_file():
                    shutil.copy2(ROOT/filename, package/filename)
            for directory in ('docs', 'examples', 'third_party'):
                shutil.copytree(ROOT/directory, package/directory)
            binary_sha = hashlib.sha256((package/'libexec/mcpdbhub').read_bytes()).hexdigest()
            manifest = {'version': args.version, 'commit': commit, 'os': 'linux', 'arch': arch,
                        'minimum_glibc': '2.36', 'binary_sha256': binary_sha,
                        'image_id': run('docker', 'image', 'inspect', '--format', '{{.Id}}', image)}
            (package/'BUILD.json').write_text(json.dumps(manifest, indent=2)+'\n')
            (package/'VERSION').write_text(args.version+'\n')
            archive = output/(name+'.tar.gz')
            with tarfile.open(archive, 'w:gz') as tar:
                tar.add(package, arcname=name, filter=portable_metadata)
            print(archive, flush=True)
    archives = sorted(output.glob(f'mcpdbhub-{args.version}-linux-*.tar.gz'))
    (output/'SHA256SUMS').write_text(''.join(hashlib.sha256(p.read_bytes()).hexdigest()+'  '+p.name+'\n' for p in archives))


def portable_metadata(info):
    info.uid = info.gid = 0
    info.uname = info.gname = 'root'
    info.pax_headers = {}
    return info


if __name__ == '__main__':
    main()
