package cmdtest

import "testing"

type usageExitCase struct {
	name    string
	args    []string
	wantErr string
	deps    bool // the command is built on shared.Deps: run it on test deps
}

func runUsageExitCases(t *testing.T, cases []usageExitCase) {
	t.Helper()

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if test.deps {
				assertUsageExitWithDeps(t, nil, test.args, test.wantErr)
				return
			}
			assertUsageExit(t, test.args, test.wantErr)
		})
	}
}

func runExactUsageExitCases(t *testing.T, cases []usageExitCase) {
	t.Helper()

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if test.deps {
				stdout, stderr, runErr := runRootWithDeps(t, nil, test.args...)
				assertUsageExitCodeResult(t, stdout, stderr, runErr, test.wantErr)
				return
			}
			assertUsageExitCode(t, test.args, test.wantErr)
		})
	}
}
