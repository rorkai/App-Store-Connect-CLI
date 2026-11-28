#!/usr/bin/env python3
"""Materialize only the expected regular release files in a fresh directory."""

import argparse
from contextlib import closing
from pathlib import Path
import re
import stat
import subprocess
import sys
import tarfile
import tempfile
import zipfile

from release_rehearsal import expected_artifact_names

MAX_ASSET_BYTES = 256 * 1024 * 1024
MAX_TOTAL_BYTES = 1024 * 1024 * 1024
MAX_ARCHIVE_BYTES = MAX_TOTAL_BYTES + 1024 * 1024
MAX_MANIFEST_BYTES = 16 * 1024


def bounded_file(path, limit):
    if path.is_symlink() or not path.is_file() or not 0 < path.stat().st_size <= limit:
        raise ValueError(f'expected a regular file of at most {limit} bytes: {path}')


def copy_member(source, target, size):
    with closing(source), target.open('xb') as output:
        remaining = size
        while remaining:
            data = source.read(min(remaining, 1024 * 1024))
            if not data:
                raise ValueError(f'truncated archive member: {target.name}')
            output.write(data)
            remaining -= len(data)
        if source.read(1):
            raise ValueError(f'archive member exceeds declared size: {target.name}')


def candidate_zip(path, stage, version):
    names = {f'candidate-release-{version}.tar': MAX_ARCHIVE_BYTES,
             f'candidate-release-{version}.commit': 41}
    with zipfile.ZipFile(path) as archive:
        members = archive.infolist()
        if len(members) != len(names) or {item.filename for item in members} != set(names):
            raise ValueError('unexpected archive member set in candidate ZIP')
        for item in members:
            mode = item.external_attr >> 16
            if (item.is_dir() or item.external_attr & 0x10 or stat.S_IFMT(mode) not in (0, stat.S_IFREG)
                    or item.flag_bits & 1 or item.compress_type not in (zipfile.ZIP_STORED, zipfile.ZIP_DEFLATED)
                    or not 0 < item.file_size <= names[item.filename]):
                raise ValueError(f'invalid candidate ZIP member: {item.filename}')
        for item in members:
            copy_member(archive.open(item), stage / item.filename, item.file_size)
    return stage / f'candidate-release-{version}.tar', stage / f'candidate-release-{version}.commit'


def release_tar(path, stage, version, require_executable):
    bounded_file(path, MAX_ARCHIVE_BYTES)
    checksum_name = f'asc_{version}_checksums.txt'
    names = {'release/' + name for name in [*expected_artifact_names(version), checksum_name]}
    with tarfile.open(path, 'r:') as archive:
        members = {}
        total = 0
        for item in archive:
            if item.name in members or item.name not in names | {'release'}:
                raise ValueError(f'unexpected archive member: {item.name}')
            if item.name == 'release':
                if not item.isdir() or item.size != 0:
                    raise ValueError('invalid release directory member')
            else:
                binary = item.name != 'release/' + checksum_name
                limit = MAX_ASSET_BYTES if binary else MAX_MANIFEST_BYTES
                if (item.type not in (tarfile.REGTYPE, tarfile.AREGTYPE) or item.issparse()
                        or not 0 < item.size <= limit or item.mode & ~0o777
                        or (require_executable and binary and not item.mode & 0o111)):
                    raise ValueError(f'invalid release archive member: {item.name}')
                total += item.size
                if total > MAX_TOTAL_BYTES:
                    raise ValueError('release archive exceeds total size limit')
            members[item.name] = item
        if set(members) != names | {'release'}:
            raise ValueError('incomplete release archive member set')
        release = stage / 'release'
        release.mkdir()
        for name in sorted(names):
            item = members[name]
            target = release / Path(name).name
            copy_member(archive.extractfile(item), target, item.size)
            target.chmod(item.mode)
    verification = subprocess.run([
        sys.executable, str(Path(__file__).with_name('verify_release_assets.py')),
        '--release-dir', str(release), '--version', version,
    ], text=True, capture_output=True)
    if verification.returncode:
        raise ValueError(verification.stderr.strip())


def unpack(path, output, version, mode, source_sha=None, commit_file=None):
    if not re.fullmatch(r'[0-9]+\.[0-9]+\.[0-9]+', version):
        raise ValueError('version must match x.y.z')
    if mode != 'published' and not re.fullmatch(r'[0-9a-f]{40}', source_sha or ''):
        raise ValueError('candidate source must be a full commit SHA')
    if output.exists() or output.is_symlink():
        raise ValueError('output must not already exist')
    bounded_file(path, MAX_ARCHIVE_BYTES)
    with tempfile.TemporaryDirectory(prefix='.release-archive-', dir=output.parent) as temporary:
        stage = Path(temporary) / 'verified'
        stage.mkdir()
        if mode == 'candidate-zip':
            path, commit_file = candidate_zip(path, stage, version)
        if mode != 'published':
            if commit_file is None:
                raise ValueError('candidate source marker is required')
            bounded_file(commit_file, 41)
            if commit_file.read_bytes() != (source_sha + '\n').encode():
                raise ValueError('candidate source marker does not match expected source')
        release_tar(path, stage, version, require_executable=mode != 'published')
        stage.rename(output)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--archive', required=True, type=Path)
    parser.add_argument('--output', required=True, type=Path)
    parser.add_argument('--version', required=True)
    parser.add_argument('--mode', required=True, choices=('candidate-zip', 'candidate', 'published'))
    parser.add_argument('--source-sha')
    parser.add_argument('--commit-file', type=Path)
    args = parser.parse_args()
    try:
        unpack(args.archive, args.output, args.version, args.mode, args.source_sha, args.commit_file)
    except (ValueError, OSError, tarfile.TarError, zipfile.BadZipFile) as error:
        parser.exit(1, f'release archive verification failed: {error}\n')


if __name__ == '__main__':
    main()
