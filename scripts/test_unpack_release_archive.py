#!/usr/bin/env python3
"""Check release archive boundaries with ordinary packaging mistakes, offline."""

import hashlib
import io
import os
from pathlib import Path
import subprocess
import shutil
import stat
import tarfile
import tempfile
import unittest
from unittest import mock
import zipfile

import unpack_release_archive as unpacker

ROOT = Path(__file__).resolve().parent.parent
VERSION = '1.2.3'
SOURCE = 'a' * 40


class ReleaseArchiveTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.assets = {f'asc_{VERSION}_{platform}': platform.encode() for platform in
                       ('linux_amd64', 'linux_arm64', 'macOS_amd64', 'macOS_arm64', 'windows_amd64.exe')}
        self.assets[f'asc_{VERSION}_checksums.txt'] = ''.join(
            f'{hashlib.sha256(data).hexdigest()}  {name}\n' for name, data in sorted(self.assets.items())).encode()

    def archive(self, mistake=None, downloaded=False):
        path = self.root / 'candidate.tar'
        with tarfile.open(path, 'w') as archive:
            directory = tarfile.TarInfo('release')
            directory.type = tarfile.DIRTYPE
            directory.mode = 0o755
            archive.addfile(directory)
            entries = [('release/' + name, data) for name, data in self.assets.items()]
            if mistake == 'missing':
                entries.pop()
            if mistake == 'extra':
                entries.append(('release-notes.txt', b'ordinary packaging note'))
            if mistake == 'duplicate':
                entries.append(entries[0])
            for name, data in entries:
                info = tarfile.TarInfo(name)
                info.size = len(data)
                info.mode = 0o644 if downloaded or name.endswith('.txt') else 0o755
                if name == entries[0][0]:
                    if mistake == 'directory':
                        info.type = tarfile.DIRTYPE
                        info.size = 0
                        data = b''
                    if mistake == 'empty':
                        info.size = 0
                        data = b''
                    if mistake == 'nonexecutable':
                        info.mode = 0o644
                    if mistake == 'checksum':
                        data = b'x' * len(data)
                archive.addfile(info, io.BytesIO(data))
        return path

    def candidate_zip(self, mistake=None, source=SOURCE):
        path = self.root / 'candidate.zip'
        with zipfile.ZipFile(path, 'w', compression=zipfile.ZIP_DEFLATED) as archive:
            archive.write(self.archive('extra' if mistake == 'tar-extra' else None), f'candidate-release-{VERSION}.tar')
            marker = zipfile.ZipInfo(f'candidate-release-{VERSION}.commit')
            if mistake == 'member-type':
                marker.external_attr = (stat.S_IFDIR | 0o755) << 16 | 0x10
            archive.writestr(marker, source + '\n')
            if mistake == 'extra':
                archive.writestr('release-notes.txt', 'ordinary packaging note')
            if mistake == 'directory':
                archive.writestr('notes/', '')
        return path

    def test_clean_candidate_and_published_archives_preserve_final_bytes(self):
        marker = self.root / 'commit'
        marker.write_text(SOURCE + '\n')
        for mode in ('candidate-zip', 'candidate', 'published'):
            with self.subTest(mode=mode):
                path = self.candidate_zip() if mode == 'candidate-zip' else self.archive(downloaded=mode == 'published')
                output = self.root / mode
                unpacker.unpack(path, output, VERSION, mode, SOURCE, marker)
                self.assertEqual({entry.name: entry.read_bytes() for entry in (output / 'release').iterdir()}, self.assets)
                for entry in (output / 'release').iterdir():
                    self.assertEqual(entry.stat().st_mode & 0o777, 0o644 if mode == 'published' or entry.name.endswith('.txt') else 0o755)
                if mode == 'candidate-zip':
                    self.assertEqual((output / f'candidate-release-{VERSION}.commit').read_text(), SOURCE + '\n')

    def test_invalid_tar_is_rejected_without_materializing_output(self):
        marker = self.root / 'commit'
        marker.write_text(SOURCE + '\n')
        cases = {'extra': 'unexpected archive member', 'duplicate': 'unexpected archive member',
                 'missing': 'incomplete release archive', 'directory': 'invalid release archive member',
                 'empty': 'invalid release archive member', 'nonexecutable': 'invalid release archive member',
                 'checksum': 'checksum mismatch'}
        for mistake, error in cases.items():
            with self.subTest(mistake=mistake):
                output = self.root / 'output'
                with self.assertRaisesRegex(ValueError, error):
                    unpacker.unpack(self.archive(mistake), output, VERSION, 'candidate', SOURCE, marker)
                self.assertFalse(output.exists())
                self.assertFalse(list(self.root.glob('.release-archive-*')))

    def test_candidate_zip_rejects_extra_members_and_wrong_source(self):
        for mistake, source in [('extra', SOURCE), ('directory', SOURCE), ('member-type', SOURCE), ('tar-extra', SOURCE), (None, 'b' * 40)]:
            with self.subTest(mistake=mistake, source=source):
                output = self.root / 'output'
                with self.assertRaisesRegex(ValueError, 'unexpected archive member|invalid candidate ZIP member|source marker does not match'):
                    unpacker.unpack(self.candidate_zip(mistake, source), output, VERSION, 'candidate-zip', SOURCE)
                self.assertFalse(output.exists())

    def test_size_limits_and_existing_output_fail_closed(self):
        path = self.archive()
        for limit in ('MAX_ASSET_BYTES', 'MAX_TOTAL_BYTES', 'MAX_MANIFEST_BYTES', 'MAX_ARCHIVE_BYTES'):
            with self.subTest(limit=limit), mock.patch.object(unpacker, limit, 1):
                with self.assertRaises(ValueError):
                    unpacker.unpack(path, self.root / 'output', VERSION, 'published')
                self.assertFalse((self.root / 'output').exists())
        output = self.root / 'output'
        output.mkdir()
        (output / 'notes.txt').write_text('existing data')
        with self.assertRaisesRegex(ValueError, 'output must not already exist'):
            unpacker.unpack(path, output, VERSION, 'published')
        self.assertEqual((output / 'notes.txt').read_text(), 'existing data')

    def workflow_script(self, job, name):
        workflow = (ROOT / '.github/workflows/release.yml').read_text()
        job_text = workflow.split(f'\n  {job}:\n', 1)[1]
        step = job_text.split(f'      - name: {name}\n', 1)[1]
        script = step.split('        run: |\n', 1)[1].split('\n      - name:', 1)[0]
        return '\n'.join(line[10:] for line in script.splitlines())

    def workflow_fixture(self, name):
        work = self.root / name
        work.mkdir()
        (work / 'workflow-source').symlink_to(ROOT, target_is_directory=True)
        (work / 'workflow-artifact').mkdir()
        runner_temp = work / 'runner-temp'
        runner_temp.mkdir()
        env = dict(os.environ, VERSION=VERSION, RELEASE_SHA=SOURCE, WORKFLOW_SHA='b' * 40,
                   GH_REPO='rorkai/App-Store-Connect-CLI', RUNNER_TEMP=str(runner_temp), ASC_BYPASS_KEYCHAIN='1')
        return work, env

    def test_workflow_consumers_validate_before_materializing(self):
        for job in ('attest', 'publish', 'homebrew', 'winget'):
            name = 'Bind final bytes to verified release source' if job == 'attest' else 'Unpack and verify release artifact'
            script = self.workflow_script(job, name)
            for mistake in (None, 'extra'):
                with self.subTest(job=job, mistake=mistake):
                    work, env = self.workflow_fixture(f'{job}-{mistake}')
                    prefix = 'candidate' if job in ('attest', 'publish') else 'published'
                    artifacts = work / 'workflow-artifact'
                    shutil.copyfile(self.archive(mistake, downloaded=prefix == 'published'), artifacts / f'{prefix}-release-{VERSION}.tar')
                    (artifacts / f'candidate-release-{VERSION}.commit').write_text(SOURCE + '\n')
                    result = subprocess.run(['bash', '-e', '-c', script], cwd=work, env=env, text=True, capture_output=True)
                    if mistake:
                        self.assertNotEqual(result.returncode, 0, result.stdout + result.stderr)
                        self.assertIn('unexpected archive member', result.stderr)
                        self.assertFalse((work / 'release-notes.txt').exists())
                        self.assertFalse((work / 'release').exists())
                    else:
                        self.assertEqual(result.returncode, 0, result.stderr)
                        self.assertEqual({p.name: p.read_bytes() for p in (work / 'release').iterdir()}, self.assets)

    def test_resolver_reuses_only_valid_same_run_and_cross_run_packages(self):
        script = self.workflow_script('resolve', 'Check for existing immutable build artifact')
        for recovery in ('same', 'cross'):
            for mistake in (None, 'extra', 'tar-extra'):
                with self.subTest(recovery=recovery, mistake=mistake):
                    work, env = self.workflow_fixture(f'{recovery}-{mistake}')
                    fixture = self.candidate_zip(mistake)
                    commands = work / 'commands'
                    commands.mkdir()
                    gh = commands / 'gh'
                    gh.write_text('''#!/usr/bin/env python3
import json, os, pathlib, sys
args = sys.argv[1:]
if args[:2] == ['release', 'view']:
    sys.exit(1)
assert args[0] == 'api', args
path = next(arg for arg in args if arg.startswith('repos/'))
if path.endswith('/runs/123/artifacts?per_page=100'):
    entries = [] if os.environ['RECOVERY'] == 'cross' else [{'id': 111, 'name': 'candidate-release-1.2.3', 'expired': False}]
    print(json.dumps({'artifacts': entries}))
elif path.endswith('/actions/artifacts/111/zip'):
    sys.stdout.buffer.write(pathlib.Path(os.environ['FIXTURE_ZIP']).read_bytes())
elif path.endswith('/actions/artifacts'):
    print(json.dumps({'artifacts': [{'id': 111, 'workflow_run': {'id': 222}, 'created_at': '2026-01-01', 'expired': False}]}))
elif path.endswith('/actions/runs/222'):
    print('\\t'.join(['.github/workflows/release.yml', os.environ['RELEASE_SHA'], 'push', os.environ['GH_REPO'], '1.2.3']))
else:
    assert path == 'repos/' + os.environ['GH_REPO'], path
    print('main')
''')
                    gh.chmod(0o755)
                    git = commands / 'git'
                    git.write_text('#!/bin/sh\ntest "$1 $2" = "rev-parse HEAD" || exit 1\nprintf "%s\\n" "$RELEASE_SHA"\n')
                    git.chmod(0o755)
                    output = work / 'outputs'
                    env.update(PATH=str(commands) + os.pathsep + env['PATH'], RECOVERY=recovery,
                               FIXTURE_ZIP=str(fixture), GITHUB_RUN_ID='123', GITHUB_OUTPUT=str(output))
                    result = subprocess.run(['bash', '-e', '-c', script], cwd=work, env=env, text=True, capture_output=True)
                    if mistake and recovery == 'same':
                        self.assertNotEqual(result.returncode, 0, result.stdout + result.stderr)
                        self.assertIn('unexpected archive member', result.stderr)
                        self.assertFalse(output.exists())
                    else:
                        self.assertEqual(result.returncode, 0, result.stderr)
                        self.assertIn('reused=' + ('false' if mistake else 'true'), output.read_text())
                    retained = work / 'workflow-artifact' / f'candidate-release-{VERSION}.tar'
                    self.assertEqual(retained.exists(), recovery == 'cross' and mistake is None)
                    self.assertFalse((work / 'release-notes.txt').exists())


if __name__ == '__main__':
    unittest.main()
