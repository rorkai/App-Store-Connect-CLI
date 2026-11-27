#!/usr/bin/env python3
"""Create and verify provenance for qualified ASC release assets."""

import argparse
import json
from pathlib import Path
import re
import subprocess
import sys

from release_rehearsal import expected_artifact_names

PREDICATE_TYPE = 'https://slsa.dev/provenance/v1'
WORKFLOW_PATH = '.github/workflows/release.yml'


def predicate(repository, version, source_sha, workflow_sha):
    origin = f'https://github.com/{repository}'
    return {
        'buildDefinition': {
            'buildType': f'{origin}/blob/main/docs/RELEASE_PROVENANCE.md#release-candidate-v1',
            'externalParameters': {'version': version},
            'resolvedDependencies': [{
                'uri': f'git+{origin}@refs/tags/{version}',
                'digest': {'gitCommit': source_sha},
            }],
        },
        'runDetails': {'builder': {'id': f'{origin}/{WORKFLOW_PATH}@{workflow_sha}'}},
    }


def run(args):
    if not re.fullmatch(r'[0-9]+\.[0-9]+\.[0-9]+', args.version):
        raise ValueError('version must match x.y.z')
    if not all(re.fullmatch(r'[0-9a-f]{40}', sha) for sha in (args.source_sha, args.workflow_sha)):
        raise ValueError('source and workflow identities must be full commit SHAs')
    if not re.fullmatch(r'[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+', args.repository):
        raise ValueError('repository must be owner/name')
    validation = subprocess.run([
        sys.executable, str(Path(__file__).with_name('verify_release_assets.py')),
        '--release-dir', str(args.release_dir), '--version', args.version,
    ], text=True, capture_output=True)
    if validation.returncode:
        raise ValueError(validation.stderr.strip())
    expected = predicate(args.repository, args.version, args.source_sha, args.workflow_sha)
    if args.mode == 'predicate':
        print(json.dumps(expected, indent=2))
        return

    names = sorted([*expected_artifact_names(args.version), f'asc_{args.version}_checksums.txt'])
    for name in names:
        command = [
            'gh', 'attestation', 'verify', str(args.release_dir / name),
            '--repo', args.repository,
            '--signer-workflow', f'{args.repository}/{WORKFLOW_PATH}',
            '--signer-digest', args.workflow_sha,
            '--predicate-type', PREDICATE_TYPE,
            '--deny-self-hosted-runners', '--format', 'json',
        ]
        if args.bundle:
            command.extend(['--bundle', str(args.bundle)])
        verified = subprocess.run(command, text=True, capture_output=True)
        if verified.returncode:
            raise ValueError(f'{name}: attestation verification failed: {verified.stderr.strip()}')
        results = json.loads(verified.stdout)
        matched = False
        if isinstance(results, list):
            for result in results:
                statement = result.get('verificationResult', {}).get('statement', {})
                actual = statement.get('predicate', {})
                if (statement.get('predicateType') == PREDICATE_TYPE
                        and actual.get('buildDefinition') == expected['buildDefinition']
                        and actual.get('runDetails', {}).get('builder') == expected['runDetails']['builder']):
                    matched = True
                    break
        if not matched:
            raise ValueError(f'{name}: verified provenance does not match the expected source commit, version, or release workflow')
    print(f'Verified release provenance for {len(names)} downloaded assets.')


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('mode', choices=('predicate', 'verify'))
    parser.add_argument('--release-dir', required=True, type=Path)
    parser.add_argument('--version', required=True)
    parser.add_argument('--source-sha', required=True)
    parser.add_argument('--workflow-sha', required=True)
    parser.add_argument('--repository', default='rorkai/App-Store-Connect-CLI')
    parser.add_argument('--bundle', type=Path)
    args = parser.parse_args()
    try:
        run(args)
    except (ValueError, OSError) as error:
        parser.exit(1, f'release provenance verification failed: {error}\n')


if __name__ == '__main__':
    main()
