#!/usr/bin/env python3
"""Regressions for bundled macOS release artifacts and build failures."""

import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import textwrap
import unittest

ROOT = Path(__file__).resolve().parents[1]

FAKE_TOOL = r'''#!PYTHON
import json, os, pathlib, subprocess, sys
name = pathlib.Path(sys.argv[0]).name
args = sys.argv[1:]
keys = ['CGO_ENABLED','CGO_CFLAGS','CGO_LDFLAGS','PKG_CONFIG_LIBDIR','PKG_CONFIG_PATH','GOOS','GOARCH','MACOSX_DEPLOYMENT_TARGET']
with open(os.environ['BUILD_LOG'],'a') as f:
 f.write(json.dumps(dict(tool=name,args=args,env={k:os.environ.get(k,'') for k in keys}))+'\n')
if name == 'python3':
 if args and args[0].endswith('build_fido2_deps.py'):
  prefix=pathlib.Path(args[args.index('--prefix')+1]); (prefix/'lib/pkgconfig').mkdir(parents=True,exist_ok=True); (prefix/'include').mkdir(exist_ok=True)
 else:
  raise SystemExit(subprocess.run([sys.executable,*args]).returncode)
elif name == 'go':
 if args[0] == 'build':
  if os.environ.get('FAIL_BUILD'): raise SystemExit(23)
  p=pathlib.Path(args[args.index('-o')+1]); p.parent.mkdir(parents=True,exist_ok=True)
  p.write_text('#!/bin/sh\nexit 0\n'); p.chmod(0o755)
  p.with_suffix('.json').write_text(json.dumps(dict(args=args,env={k:os.environ.get(k,'') for k in keys})))
 elif args[:2] == ['version','-m']:
  meta=json.loads(pathlib.Path(args[2]).with_suffix('.json').read_text()); a=meta['args']; env=meta['env']
  print('binary: go1.27.3')
  if '-tags' in a and not os.environ.get('OMIT_BACKEND'): print('\tbuild\t-tags='+a[a.index('-tags')+1])
  for k in ['CGO_ENABLED','GOOS','GOARCH']: print('\tbuild\t'+k+'='+env[k])
elif name == 'otool':
 print(args[-1]+':\n\t'+('/opt/homebrew/lib/libfido2.dylib' if os.environ.get('DYNAMIC_LIBRARY') else '/usr/lib/libSystem.B.dylib')+' (compatibility version 1.0.0)')
elif name == 'vtool':
 print('  cmd LC_BUILD_VERSION\n platform MACOS\n minos '+('14.0' if os.environ.get('BAD_MINOS') else '13.0')+'\n sdk 27.2')
elif name == 'uname': print('Darwin' if args == ['-s'] else 'arm64')
elif name == 'git': print('abc1234' if args[:2] == ['rev-parse','--short'] else '2026-11-02T00:00:00Z')
'''


class MacOSReleaseTest(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.root = Path(self.tmp.name)
        self.tools = self.root / 'bin'
        self.tools.mkdir()
        for name in ('go', 'python3', 'otool', 'vtool', 'git', 'uname', 'brew'):
            p = self.tools / name
            p.write_text(FAKE_TOOL.replace('PYTHON', sys.executable, 1))
            p.chmod(0o755)
        self.log = self.root / 'calls.jsonl'
        self.env = dict(os.environ, PATH=f'{self.tools}:{os.environ["PATH"]}',
                        BUILD_LOG=str(self.log), VERSION='1.2.3',
                        RELEASE_DIR=str(self.root / 'release'),
                        FIDO2_DEPS_DIR=str(self.root / 'deps'),
                        GOMAXPROCS='2', ASC_BYPASS_KEYCHAIN='1')

    def run_build(self, **env):
        return subprocess.run(['sh', str(ROOT / 'scripts/build_release_macos.sh')],
                              cwd=ROOT, env=dict(self.env, **env), text=True,
                              stdout=subprocess.PIPE, stderr=subprocess.PIPE)

    def calls(self):
        return [json.loads(line) for line in self.log.read_text().splitlines()]

    # Regression: both architecture builds need the signer, their own dependency
    # prefix in the CGO cache key, and the supported macOS deployment target.
    def test_builds_both_architectures_with_isolated_native_dependencies(self):
        r = self.run_build()
        self.assertEqual(r.returncode, 0, r.stdout + r.stderr)
        builds = [c for c in self.calls() if c['tool'] == 'go' and c['args'][0] == 'build']
        self.assertEqual({c['env']['GOARCH'] for c in builds}, {'amd64', 'arm64'})
        for call in builds:
            args, env = call['args'], call['env']
            self.assertEqual(args[args.index('-tags') + 1], 'fido2')
            self.assertIn('-trimpath', args)
            self.assertIn('-s -w', args[args.index('-ldflags') + 1])
            self.assertEqual(env['CGO_ENABLED'], '1')
            self.assertEqual(env['GOOS'], 'darwin')
            self.assertEqual(env['MACOSX_DEPLOYMENT_TARGET'], '13.0')
            self.assertEqual(env['PKG_CONFIG_PATH'], '')
            prefix = str(self.root / 'deps' / env['GOARCH'])
            self.assertEqual(env['PKG_CONFIG_LIBDIR'], prefix + '/lib/pkgconfig')
            self.assertIn('-I' + prefix + '/include', env['CGO_CFLAGS'])
            self.assertIn('-L' + prefix + '/lib', env['CGO_LDFLAGS'])
            self.assertTrue((self.root / 'release' / f'asc_1.2.3_macOS_{env["GOARCH"]}').is_file())

    # Regression: apparent build success must not qualify a Homebrew-linked
    # binary, an omitted backend, or a binary that raises the minimum OS version.
    def test_rejects_unbundled_dependencies_missing_backend_and_wrong_minos(self):
        for bad, message in (('DYNAMIC_LIBRARY', 'native library'),
                             ('OMIT_BACKEND', 'FIDO2 backend'),
                             ('BAD_MINOS', 'deployment target')):
            with self.subTest(bad=bad):
                r = self.run_build(**{bad: '1'})
                self.assertNotEqual(r.returncode, 0, r.stdout + r.stderr)
                self.assertIn(message, r.stdout + r.stderr)

    def test_stops_on_build_failure(self):
        r = self.run_build(FAIL_BUILD='1')
        self.assertEqual(r.returncode, 23, r.stdout + r.stderr)

    # Regression: new source must use the native builder, while repairs of old
    # tags without that script must preserve their original build behavior.
    def test_workflow_dispatches_native_builder_and_preserves_old_refs(self):
        workflow = (ROOT / '.github/workflows/release.yml').read_text()
        block = workflow.split('      - name: Build macOS binaries\n', 1)[1].split('\n      - name:', 1)[0]
        command = textwrap.dedent(block.split('        run: |\n', 1)[1])
        for native in (False, True):
            with self.subTest(native=native):
                work = self.root / str(native)
                (work / 'scripts').mkdir(parents=True)
                marker = work / 'builder-ran'
                if native:
                    (work / 'scripts/build_release_macos.sh').write_text('printf native > "$BUILD_MARKER"\n')
                self.log.unlink(missing_ok=True)
                r = subprocess.run(['sh', '-c', command], cwd=work,
                                   env=dict(self.env, BUILD_MARKER=str(marker)),
                                   text=True, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
                self.assertEqual(r.returncode, 0, r.stdout + r.stderr)
                builds = [c for c in self.calls() if c['tool'] == 'go' and c['args'][0] == 'build']
                if native:
                    self.assertEqual(marker.read_text(), 'native')
                    self.assertEqual(builds, [])
                else:
                    self.assertEqual(len(builds), 2)
                    self.assertEqual({c['env']['GOARCH'] for c in builds}, {'amd64', 'arm64'})
                    for c in builds:
                        self.assertEqual(c['env']['GOOS'], 'darwin')
                        self.assertEqual(c['env']['CGO_ENABLED'], '1')
                        self.assertNotIn('-tags', c['args'])


if __name__ == '__main__':
    unittest.main()
