#!/usr/bin/env python3
"""Run a reproducible shuffled suite and a small sample in fresh Go processes."""

import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import shlex
import subprocess
import sys
import tempfile

ROOT = Path(__file__).resolve().parent.parent
PACKAGES = ('./internal/config', './internal/cli/shared')
SAMPLE_SIZE = 3


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--seed', required=True, type=int)
    parser.add_argument('--output-dir', required=True, type=Path)
    args = parser.parse_args()
    if not 0 <= args.seed < 2**63:
        parser.error('--seed must be a nonnegative signed 64-bit integer')
    output = args.output_dir.resolve()
    output.mkdir(parents=True, exist_ok=False)
    go = os.environ.get('GO') or 'go'
    caches = json.loads(subprocess.check_output([go, 'env', '-json', 'GOCACHE', 'GOMODCACHE'], text=True))
    base_env = {key: value for key, value in os.environ.items() if not key.startswith('ASC_')}
    base_env.update(caches, GOMAXPROCS='1', GOWORK='off', ASC_BYPASS_KEYCHAIN='1',
                    DO_NOT_TRACK='1', ASC_TELEMETRY_DISABLED='1')
    plan = {'seed': args.seed, 'goVersion': subprocess.check_output([go, 'version'], text=True).strip(),
            'sourceCommit': subprocess.check_output(['git', 'rev-parse', 'HEAD'], cwd=ROOT, text=True).strip(),
            'packages': {}}
    (output / 'plan.json').write_text(json.dumps(plan, indent=2) + '\n')
    print(json.dumps(plan), flush=True)

    def run(name, command, cwd):
        with tempfile.TemporaryDirectory(prefix='process-', dir=output) as home:
            temp = Path(home) / 'tmp'
            temp.mkdir()
            env = dict(base_env, HOME=home, ASC_CONFIG_PATH=str(Path(home) / 'config.json'), TMPDIR=str(temp))
            print(f'{name}: {shlex.join(map(str, command))}', flush=True)
            with (output / f'{name}.log').open('w') as log:
                result = subprocess.run(command, cwd=cwd, env=env, stdout=log, stderr=subprocess.STDOUT)
            if result.returncode:
                print(f'{name} failed with exit {result.returncode}; seed={args.seed}; see {output / (name + ".log")}', file=sys.stderr)
                raise SystemExit(result.returncode)
            return (output / f'{name}.log').read_text()

    for package in PACKAGES:
        label = package.removeprefix('./').replace('/', '-')
        binary = output / label
        run(f'{label}-compile', [go, 'test', '-p=1', '-mod=readonly', '-c', '-o', str(binary), package], ROOT)
        names = run(f'{label}-list', [str(binary), '-test.list=^Test'], ROOT / package)
        tests = [name for name in names.splitlines() if re.fullmatch(r'Test\w+', name)]
        if not tests:
            raise SystemExit(f'no top-level tests found in {package}')
        selected = sorted(tests, key=lambda name: hashlib.sha256(f'{args.seed}:{package}:{name}'.encode()).digest())[:SAMPLE_SIZE]
        plan['packages'][package] = {'testCount': len(tests), 'isolated': selected}
    (output / 'plan.json').write_text(json.dumps(plan, indent=2) + '\n')
    print(json.dumps(plan, indent=2), flush=True)

    for package, selection in plan['packages'].items():
        label = package.removeprefix('./').replace('/', '-')
        command = [str(output / label), '-test.short', '-test.count=1', '-test.parallel=1',
                   '-test.timeout=5m', f'-test.shuffle={args.seed}', '-test.v']
        run(f'{label}-shuffled', command, ROOT / package)
        for index, test in enumerate(selection['isolated']):
            run(f'{label}-isolated-{index}', [*command, f'-test.run=^{re.escape(test)}$'], ROOT / package)
    print(f'Passed shuffled tests and {len(PACKAGES) * SAMPLE_SIZE} fresh-process tests; seed={args.seed}.')


if __name__ == '__main__':
    main()
