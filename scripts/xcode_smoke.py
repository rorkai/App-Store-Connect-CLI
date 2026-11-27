#!/usr/bin/env python3
"""Exercise local project discovery, simulator enumeration, and xcresult parsing."""

import argparse
import hashlib
import json
import os
from pathlib import Path
import platform
import shutil
import subprocess
import xml.etree.ElementTree as ET


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--binary', required=True, type=Path)
    parser.add_argument('--developer-dir', required=True, type=Path)
    parser.add_argument('--xcode-version', required=True)
    parser.add_argument('--work-dir', required=True, type=Path)
    args = parser.parse_args()
    root = args.work_dir.resolve()
    root.mkdir(parents=True, exist_ok=False)
    home = root / 'home'
    home.mkdir()
    env = dict(os.environ, DEVELOPER_DIR=str(args.developer_dir.resolve()), HOME=str(home),
               ASC_CONFIG_PATH=str(home / 'config.json'), ASC_BYPASS_KEYCHAIN='1',
               DO_NOT_TRACK='1', ASC_READ_ONLY='1', GOMAXPROCS='1')
    binary = str(args.binary.resolve())
    with open(binary, 'rb') as executable:
        digest = hashlib.sha256()
        for chunk in iter(lambda: executable.read(1024 * 1024), b''):
            digest.update(chunk)
        environment = {'binarySha256': digest.hexdigest(),
                       'platform': platform.platform(), 'machine': platform.machine(),
                       'runnerImage': os.environ.get('ImageOS'), 'runnerImageVersion': os.environ.get('ImageVersion')}
    (root / 'environment.json').write_text(json.dumps(environment, indent=2) + '\n')

    def run(name, command, cwd=root):
        with (root / f'{name}.json').open('w') as out, (root / f'{name}.stderr').open('w') as err:
            subprocess.run(command, cwd=cwd, env=env, stdout=out, stderr=err, check=True)
        return json.loads((root / f'{name}.json').read_text())

    doctor = run('doctor', [binary, 'xcode', 'doctor', '--output', 'json'])
    if doctor.get('status') not in ('ok', 'warn') or doctor.get('xcodeVersion') != args.xcode_version:
        raise ValueError(f'expected Xcode {args.xcode_version}; selected toolchain: {doctor}')
    if Path(doctor['developerDir']).resolve() != args.developer_dir.resolve():
        raise ValueError('doctor did not use the requested developer directory')
    print(f"Xcode {doctor['xcodeVersion']} ({doctor['xcodeBuild']}): {doctor['developerDir']}", flush=True)
    destinations = run('destinations', [binary, 'xcode', 'test-destinations', '--platform', 'iOS', '--available-only', '--output', 'json'])
    if not destinations.get('destinations'):
        raise ValueError('the selected Xcode exposes no available iOS simulator destinations')

    project = root / 'fixture'
    shutil.copytree(Path(__file__).parent / 'fixtures/xcode-smoke', project)
    listing = run('discovery', ['xcodebuild', '-list', '-json', '-project', 'Smoke.xcodeproj'], cwd=project)
    if 'Smoke' not in listing['project']['schemes']:
        raise ValueError('Xcode did not discover the smoke scheme')
    result = run('tests', [binary, 'xcode', 'test', '--scheme', 'Smoke',
        '--destination', f'platform=macOS,arch={platform.machine()}', '--no-code-signing',
        '--parallel-testing=false', '--xcodebuild-flag=-jobs', '--xcodebuild-flag=1',
        '--derived-data-path', str(root / 'DerivedData'), '--result-bundle-path', str(root / 'Smoke.xcresult'),
        '--output', 'json'], cwd=project)
    expected = {'total': 2, 'passed': 1, 'failed': 0, 'skipped': 1}
    if not result.get('success') or result.get('project') != 'Smoke.xcodeproj' or any(result.get('tests', {}).get(key) != value for key, value in expected.items()):
        raise ValueError(f'project discovery or xcresult counts differ: {result}')
    run('junit', [binary, 'xcode', 'test', 'junit', '--xcresult', str(root / 'Smoke.xcresult'),
        '--report-file', str(root / 'junit.xml'), '--output', 'json'])
    report = ET.parse(root / 'junit.xml')
    cases = report.findall('.//testcase')
    if len(cases) != 2 or sum(case.find('skipped') is not None for case in cases) != 1 or report.findall('.//failure'):
        raise ValueError('JUnit does not preserve both test cases and the skipped result')
    print(f'Passed project discovery, {len(destinations["destinations"])} simulator destinations, xcresult and JUnit checks.')


if __name__ == '__main__':
    main()
