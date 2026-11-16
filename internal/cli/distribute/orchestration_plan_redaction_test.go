package distribute

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/signing"
)

func TestExecuteDistributionPlanCollapsesOperationalDiagnostics(t *testing.T) {
	const (
		providerCanary = "PROVIDER-BODY-CANARY"
		deviceCanary   = "00008110-PRIVATE-DEVICE-UDID"
		resourceCanary = "RESOURCE-ID-CANARY"
		credential     = "ASC_PRIVATE_KEY=CREDENTIAL-CANARY"
		sensitivePath  = "/Users/private/secrets/distribution.p12"
	)
	raw := errors.New("request failed at https://downloads.example.test/install?X-Amz-Signature=" + providerCanary +
		" device=" + deviceCanary + " resource=" + resourceCanary + " " + credential + " path=" + sensitivePath)

	tests := []struct {
		name string
		code string
		fail func(*distributionOrchestrationDependencies)
	}{
		{
			name: "configuration",
			code: "config_read_failed",
			fail: func(deps *distributionOrchestrationDependencies) {
				deps.readConfig = func(string) (distributionConfig, string, error) { return distributionConfig{}, "", raw }
			},
		},
		{
			name: "device input",
			code: "devices_read_failed",
			fail: func(deps *distributionOrchestrationDependencies) {
				deps.hashProtectedFile = func(string) (string, error) { return "", raw }
			},
		},
		{
			name: "identity inspection",
			code: "identity_inspection_failed",
			fail: func(deps *distributionOrchestrationDependencies) {
				deps.inspectIdentity = func(context.Context, signing.PKCS12IdentityOptions) (signing.PKCS12IdentityInfo, error) {
					return signing.PKCS12IdentityInfo{}, raw
				}
			},
		},
		{
			name: "archive inspection",
			code: "archive_inspection_failed",
			fail: func(deps *distributionOrchestrationDependencies) {
				deps.digestArchive = func(context.Context, string) (archiveTreeSnapshot, error) { return archiveTreeSnapshot{}, raw }
			},
		},
		{
			name: "account planning",
			code: "account_reconcile_failed",
			fail: func(deps *distributionOrchestrationDependencies) {
				deps.reconcilePlan = func(context.Context, signing.ReconcilePlanOptions) (signing.ReconcilePlanView, error) {
					return signing.ReconcilePlanView{}, raw
				}
			},
		},
		{
			name: "plan persistence",
			code: "plan_write_failed",
			fail: func(deps *distributionOrchestrationDependencies) {
				deps.writePlan = func(string, persistedDistributionPlan) error { return raw }
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			deps := validDistributionOrchestrationDependencies(t)
			test.fail(&deps)
			installDistributionOrchestrationDependencies(t, deps)

			d := distributionTestDepsAt(distributionTestNow)
			stdout, stderr, err := captureDistributionCommandOutput(t, d, func() error {
				_, planErr := executeDistributionPlan(context.Background(), d, distributionPlanRequest{
					ArchivePath: "/archives/Demo.xcarchive",
					ConfigPath:  "/configuration/distribution.json",
					PlanPath:    "/plans/plan.json",
					StateDir:    "/state/runs",
				})
				return planErr
			})
			if err == nil {
				t.Fatal("planning unexpectedly succeeded")
			}
			want := "distribution planning failed (" + test.code + "); inspect protected inputs and rerun"
			if err.Error() != want {
				t.Fatalf("error = %q, want %q", err, want)
			}
			if stdout != "" || stderr != "" {
				t.Fatalf("planning wrote output: stdout=%q stderr=%q", stdout, stderr)
			}
			for _, secret := range []string{providerCanary, deviceCanary, resourceCanary, credential, sensitivePath, "X-Amz-Signature"} {
				if strings.Contains(err.Error(), secret) || strings.Contains(stdout, secret) || strings.Contains(stderr, secret) {
					t.Fatalf("planning leaked %q: error=%q stdout=%q stderr=%q", secret, err, stdout, stderr)
				}
			}
		})
	}
}
