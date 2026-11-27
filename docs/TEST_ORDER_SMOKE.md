# Test order smoke

`Test order smoke` runs weekly and by manual dispatch. It adds no job to PR or
push workflows. Scheduled runs use the run ID as their seed; manual runs accept
a nonnegative integer seed. The seed, source commit, Go version, test counts,
isolated sample and execution logs are retained for seven days.

Replay from the same source, platform and Go version, using a new output directory:

```bash
ASC_BYPASS_KEYCHAIN=1 DO_NOT_TRACK=1 python3 scripts/go_test_order.py \
  --seed 780 --output-dir /tmp/asc-test-order-780
```

The config and shared CLI packages exercise configuration, environment and
shared command state. Each package is compiled once, then its tests run with
`-test.shuffle=SEED`, `-test.short`, `-test.count=1` and one worker. A stable hash
of seed, package and test name selects three top-level tests per package. Each
selected test runs in a fresh process; normal subtests and TestMain still run.
Every invocation gets a private HOME/config/temp directory and keychain bypass.
Ambient `ASC_*` settings are removed before the isolated settings are applied.

Failures preserve the process exit code and identify the seed and log path.
This is a bounded sample, not exhaustive permutation testing. It can expose
order or initialization dependencies; its introduction does not establish that
the existing suite contains one. Regular PR tests and their sharding remain
unchanged.
