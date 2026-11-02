#!/usr/bin/env python3
"""Build checksum-pinned static FIDO2 dependencies for macOS."""

import argparse
import hashlib
import json
import os
from pathlib import Path
import platform
import subprocess
import sys
import tarfile
import tempfile
import urllib.request


def run(args, cwd, env):
    subprocess.run(args, cwd=cwd, env=env, check=True)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--arch', required=True, choices=('amd64', 'arm64'))
    parser.add_argument('--prefix', required=True, type=Path)
    parser.add_argument('--jobs', type=int, default=2)
    args = parser.parse_args()
    if sys.version_info < (3, 12):
        parser.error("Python 3.12 or newer is required for safe archive extraction")
    if args.jobs < 1:
        parser.error('--jobs must be positive')
    system = platform.system()
    if system != 'Darwin':
        parser.error('macOS dependency builds require a macOS host')
    prefix = args.prefix.resolve()
    prefix.mkdir(parents=True, exist_ok=True)
    manifest = json.loads(Path(__file__).with_name('fido2-deps.json').read_text())
    env = dict(os.environ)
    env['PKG_CONFIG_PATH'] = ''
    env['PKG_CONFIG_LIBDIR'] = str(prefix / 'lib/pkgconfig')
    env['CFLAGS'] = '-O2 -fPIC'
    env['LDFLAGS'] = ''
    cmake_flags = [f'-DCMAKE_INSTALL_PREFIX={prefix}', '-DCMAKE_INSTALL_LIBDIR=lib',
                   '-DCMAKE_BUILD_TYPE=Release', '-DBUILD_SHARED_LIBS=OFF',
                   f'-DCMAKE_PREFIX_PATH={prefix}']
    arch = 'x86_64' if args.arch == 'amd64' else 'arm64'
    env['MACOSX_DEPLOYMENT_TARGET'] = '13.0'
    env['CFLAGS'] += f' -arch {arch} -mmacosx-version-min=13.0'
    cmake_flags += [f'-DCMAKE_OSX_ARCHITECTURES={arch}', '-DCMAKE_OSX_DEPLOYMENT_TARGET=13.0']

    with tempfile.TemporaryDirectory(prefix='asc-fido2-') as temporary:
        workspace = Path(temporary)

        def source(name):
            entry = manifest['sources'][name]
            archive = workspace / (name + '.tar.gz')
            urllib.request.urlretrieve(entry['url'], archive)
            if hashlib.sha256(archive.read_bytes()).hexdigest() != entry['sha256']:
                raise ValueError(f'{name}: source checksum mismatch')
            directory = workspace / name
            directory.mkdir()
            with tarfile.open(archive) as tar:
                tar.extractall(directory, filter='data')
            return next(directory.iterdir())

        def cmake(directory, options):
            # libcbor ships a Bazel BUILD file, which collides with "build" on
            # the case-insensitive filesystems used by macOS runners.
            build = directory / '_asc_build'
            run(['cmake', '-S', str(directory), '-B', str(build), *cmake_flags, *options], directory, env)
            run(['cmake', '--build', str(build), '--parallel', str(args.jobs)], directory, env)
            run(['cmake', '--install', str(build)], directory, env)

        openssl = source('openssl')
        target = 'darwin64-x86_64-cc' if args.arch == 'amd64' else 'darwin64-arm64-cc'
        run(['perl', 'Configure', target, 'no-shared', 'no-module', 'no-tests', 'no-dso',
             f'--prefix={prefix}', '--libdir=lib'], openssl, env)
        run(['make', f'-j{args.jobs}', 'build_libs'], openssl, env)
        run(['make', 'install_dev'], openssl, env)
        cmake(source('libcbor'), ['-DWITH_EXAMPLES=OFF', '-DWITH_TESTS=OFF'])
        zlib = source('zlib')
        run(['./configure', '--static', f'--prefix={prefix}'], zlib, env)
        run(['make', f'-j{args.jobs}', 'install'], zlib, env)
        cmake(source('libfido2'), ['-DBUILD_STATIC_LIBS=ON', '-DBUILD_TESTS=OFF',
                                 '-DBUILD_TOOLS=OFF', '-DBUILD_EXAMPLES=OFF',
                                 '-DBUILD_MANPAGES=OFF'])

    # Upstream libfido2.pc omits static transitive dependencies. This prefix has
    # archives only, so neither pkg-config nor the linker can select host dylibs.
    extra = '-framework IOKit -framework CoreFoundation'
    (prefix / 'lib/pkgconfig/libfido2.pc').write_text(
        f'prefix={prefix}\nlibdir=${{prefix}}/lib\nincludedir=${{prefix}}/include\n'
        'Name: libfido2\nDescription: ASC static FIDO2 backend\n'
        f'Version: {manifest["sources"]["libfido2"]["version"]}\n'
        f'Libs: -L${{libdir}} -lfido2 -lcbor -lcrypto -lz {extra}\n'
        'Cflags: -I${includedir}\n')


if __name__ == '__main__':
    main()
