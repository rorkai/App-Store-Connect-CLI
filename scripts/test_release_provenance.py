#!/usr/bin/env python3
"""Exercise release provenance policy and the publication barrier without publishing."""

import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile
import unittest

ROOT = Path(__file__).resolve().parent.parent
SCRIPT = ROOT / 'scripts/release_provenance.py'
VERSION = '1.2.3'
SOURCE = 'a' * 40
WORKFLOW = 'b' * 40
REPO = 'rorkai/App-Store-Connect-CLI'


class ReleaseProvenanceTests(unittest.TestCase):
    def test_publication_requires_verification_of_downloaded_bytes(self):
        workflow = (ROOT / '.github/workflows/release.yml').read_text()
        publish = workflow.split('  publish:\n', 1)[1].split('\n  homebrew:', 1)[0]
        self.assertTrue('needs.attest.result == \'success\'' in publish, 'publication must require a successful attestation job')
        verification = publish.index('scripts/release_provenance.py verify')
        publication = publish.index('gh release edit "${VERSION}" --draft=false')
        self.assertLess(verification, publication)
        self.assertIn('--release-dir published-release', publish[verification:publication])
        self.assertIn('gh release download "${VERSION}" --dir published-release', publish[:verification])
        self.assertNotIn('id-token:', publish)
        rehearsal = (ROOT / '.github/workflows/release-rehearsal.yml').read_text()
        self.assertNotIn('id-token:', rehearsal)
        self.assertNotIn('attestations:', rehearsal)

    def test_publish_reuses_the_completed_attestation_artifact_on_rerun(self):
        workflow = (ROOT / '.github/workflows/release.yml').read_text()
        publish = workflow.split('  publish:\n', 1)[1].split('\n  homebrew:', 1)[0]
        self.assertTrue('artifact-ids: ${{ needs.attest.outputs.bundle-id }}' in publish,
                        'retrying only publication must retain the successful signing job artifact ID')

    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.release = self.root / 'release'
        self.release.mkdir()
        self.names = [f'asc_{VERSION}_{platform}' for platform in ('linux_amd64', 'linux_arm64', 'macOS_amd64', 'macOS_arm64', 'windows_amd64.exe')]
        for name in self.names:
            (self.release / name).write_bytes(('final bytes ' + name).encode())
        self.checksums()
        self.args = ['--release-dir', str(self.release), '--version', VERSION, '--source-sha', SOURCE, '--workflow-sha', WORKFLOW, '--repository', REPO]
        self.env = dict(os.environ, PATH=str(self.root) + os.pathsep + os.environ['PATH'], FIXTURE_ROOT=str(self.root), ASC_BYPASS_KEYCHAIN='1', DO_NOT_TRACK='1')

    def checksums(self):
        (self.release / f'asc_{VERSION}_checksums.txt').write_text(''.join(f'{hashlib.sha256((self.release / name).read_bytes()).hexdigest()}  {name}\n' for name in sorted(self.names)))

    def run_script(self, mode):
        return subprocess.run([sys.executable, str(SCRIPT), mode, *self.args], env=self.env, text=True, capture_output=True)

    def run_publication(self):
        workflow = (ROOT / '.github/workflows/release.yml').read_text()
        step = workflow.split('      - name: Create, resume, or verify GitHub Release\n', 1)[1].split('\n      - name:', 1)[0]
        script = '\n'.join(line.removeprefix('          ') for line in step.split('        run: |\n', 1)[1].splitlines())
        (self.root / 'workflow-source').symlink_to(ROOT, target_is_directory=True)
        shutil.copytree(self.release, self.root / 'remote')
        env = dict(self.env, VERSION=VERSION, RELEASE_SHA=SOURCE, WORKFLOW_SHA=WORKFLOW, GH_REPO=REPO, RUNNER_TEMP=str(self.root))
        return subprocess.run(['bash', '-e', '-o', 'pipefail', '-c', script], cwd=self.root, env=env, text=True, capture_output=True)

    def verifier_fixture(self, source=SOURCE):
        predicate = {
            'buildDefinition': {
                'buildType': 'https://actions.github.io/buildtypes/workflow/v1',
                'externalParameters': {'workflow': {'ref': 'refs/heads/main', 'repository': f'https://github.com/{REPO}', 'path': '.github/workflows/release.yml'}},
                'internalParameters': {'github': {'event_name': 'workflow_dispatch', 'repository_id': '1', 'repository_owner_id': '2', 'runner_environment': 'github-hosted'}},
                'resolvedDependencies': [{'uri': f'git+https://github.com/{REPO}@refs/heads/main', 'digest': {'gitCommit': WORKFLOW}}],
            },
            'runDetails': {'builder': {'id': f'https://github.com/{REPO}/.github/workflows/release.yml@refs/heads/main'}},
        }
        identity = {'repository': REPO, 'version': VERSION, 'sourceCommit': source, 'workflowCommit': WORKFLOW}
        identity_bytes = (json.dumps(identity, sort_keys=True, separators=(',', ':')) + '\n').encode()
        subjects = [{'name': p.name, 'digest': {'sha256': hashlib.sha256(p.read_bytes()).hexdigest()}} for p in self.release.iterdir()]
        subjects.append({'name': 'release-identity.json', 'digest': {'sha256': hashlib.sha256(identity_bytes).hexdigest()}})
        statement = {'predicateType': 'https://slsa.dev/provenance/v1', 'predicate': predicate, 'subject': subjects}
        (self.root / 'verification.json').write_text(json.dumps([{'verificationResult': {'statement': statement}}]))
        gh = self.root / 'gh'
        gh.write_text('''#!/usr/bin/env python3
import hashlib,json,os,pathlib,shutil,sys
root=pathlib.Path(os.environ['FIXTURE_ROOT']);args=sys.argv[1:]
if args[:2]==['release','view']:
 if '--json' in args:
  field=args[args.index('--json')+1]
  print('true' if field=='isDraft' else '\\n'.join(p.name for p in (root/'remote').iterdir()))
 sys.exit(0)
if args[:2]==['release','download']:
 target=pathlib.Path(args[args.index('--dir')+1])
 for p in (root/'remote').iterdir():shutil.copyfile(p,target/p.name)
 sys.exit(0)
if args[:2]==['release','edit']:
 assert '--draft=false' in args
 (root/'published').touch();sys.exit(0)
assert args[:2]==['attestation','verify'],args
assert args[args.index('--repo')+1]=='rorkai/App-Store-Connect-CLI'
assert args[args.index('--signer-workflow')+1]=='rorkai/App-Store-Connect-CLI/.github/workflows/release.yml'
assert args[args.index('--signer-digest')+1]=='b'*40
assert args[args.index('--format')+1]=='json'
assert '--deny-self-hosted-runners' in args
assert '--source-digest' not in args
artifact=pathlib.Path(args[2]);results=json.loads((root/'verification.json').read_text())
subjects=results[0]['verificationResult']['statement']['subject']
if not any(s['name']==artifact.name and s['digest']['sha256']==hashlib.sha256(artifact.read_bytes()).hexdigest() for s in subjects):
 print('artifact digest does not match attestation',file=sys.stderr);sys.exit(1)
with (root/'verified-names').open('a') as f:f.write(artifact.name+'\\n')
print(json.dumps(results))
''')
        gh.chmod(0o755)

    def test_tag_source_is_distinct_from_workflow_source(self):
        self.verifier_fixture()
        result = self.run_publication()
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(set((self.root / 'verified-names').read_text().splitlines()), set(self.names + [f'asc_{VERSION}_checksums.txt']))
        self.assertTrue((self.root / 'published').exists())
        result = self.run_script('identity')
        self.assertEqual(result.returncode, 0, result.stderr)
        identity = {'repository': REPO, 'version': VERSION, 'sourceCommit': SOURCE, 'workflowCommit': WORKFLOW}
        self.assertEqual(result.stdout, json.dumps(identity, sort_keys=True, separators=(',', ':')) + '\n')

    def test_workflow_attests_release_identity_with_standard_provenance(self):
        workflow = (ROOT / '.github/workflows/release.yml').read_text()
        attest = workflow.split('  attest:\n', 1)[1].split('\n  publish:', 1)[0]
        self.assertIn('release_provenance.py identity', attest)
        self.assertIn('> release-identity.json', attest)
        self.assertIn('subject-path: |\n            release/asc_*\n            release-identity.json', attest)
        self.assertNotIn('predicate-type:', attest)
        self.assertNotIn('predicate-path:', attest)

    def test_split_release_identity_and_binary_statements_are_rejected(self):
        self.verifier_fixture()
        path = self.root / 'verification.json'
        results = json.loads(path.read_text())
        statement = results[0]['verificationResult']['statement']
        identity_statement = dict(statement, subject=statement['subject'][-1:])
        statement['subject'] = statement['subject'][:-1]
        results.append({'verificationResult': {'statement': identity_statement}})
        path.write_text(json.dumps(results))
        result = self.run_publication()
        self.assertNotEqual(result.returncode, 0)
        self.assertIn('source commit, version, or release workflow', result.stderr)
        self.assertFalse((self.root / 'published').exists())

    def test_wrong_commit_is_rejected_after_signature_verification(self):
        self.verifier_fixture(source='c' * 40)
        result = self.run_publication()
        self.assertNotEqual(result.returncode, 0)
        self.assertIn('source commit', result.stderr)
        self.assertTrue((self.root / 'verified-names').exists())
        self.assertFalse((self.root / 'published').exists())

    def test_replaced_subject_bytes_fail_even_with_updated_checksums(self):
        self.verifier_fixture()
        (self.release / self.names[0]).write_bytes(b'replaced bytes')
        self.checksums()
        result = self.run_publication()
        self.assertNotEqual(result.returncode, 0)
        self.assertIn('artifact digest does not match attestation', result.stderr)
        self.assertFalse((self.root / 'published').exists())


if __name__ == '__main__':
    unittest.main()
