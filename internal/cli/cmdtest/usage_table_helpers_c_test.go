package cmdtest

import "testing"

type usageExitCase struct {
	name    string
	args    []string
	wantErr string
}

func runUsageExitCases(t *testing.T, cases []usageExitCase) {
	t.Helper()

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			assertUsageExit(t, test.args, test.wantErr)
		})
	}
}

func runExactUsageExitCases(t *testing.T, cases []usageExitCase) {
	t.Helper()

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			assertUsageExitCode(t, test.args, test.wantErr)
		})
	}
}
