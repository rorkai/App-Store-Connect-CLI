package cmdtest

import (
	"errors"
	"flag"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func TestTestFlightDistributionEditRejectsRemovedExternalTestingFlag(t *testing.T) {
	clientFactoryCalled := false
	t.Cleanup(shared.SetASCClientFactoryForTesting(func() (*asc.Client, error) {
		clientFactoryCalled = true
		return nil, errors.New("client factory must not be called")
	}))

	tests := []struct {
		name string
		args []string
	}{
		{
			name: "enable",
			args: []string{"testflight", "distribution", "edit", "--id", "detail-1", "--external-testing=true"},
		},
		{
			name: "disable",
			args: []string{"testflight", "distribution", "edit", "--id", "detail-1", "--external-testing=false"},
		},
		{
			name: "mixed with supported update",
			args: []string{"testflight", "distribution", "edit", "--id", "detail-1", "--auto-notify", "--external-testing", "true"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			clientFactoryCalled = false
			assertRemovedFlagGuidance(t, test.args, "--external-testing")
			if clientFactoryCalled {
				t.Fatal("expected removed flag to fail before client creation or HTTP")
			}
		})
	}
}

func TestTestFlightRecruitmentSetUpdatesExisting(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/betaGroups/group-1/betaRecruitmentCriteria"),
		asctest.JSON(http.StatusOK, `{"data":{"type":"betaRecruitmentCriteria","id":"criteria-1","attributes":{"deviceFamilyOsVersionFilters":[{"deviceFamily":"IPHONE","minimumOsInclusive":"26"}]}}}`))
	reg.Register(asctest.PATCH("/v1/betaRecruitmentCriteria/criteria-1").JSONBody(`{"data":{"type":"betaRecruitmentCriteria","id":"criteria-1","attributes":{"deviceFamilyOsVersionFilters":[{"deviceFamily":"IPHONE","minimumOsInclusive":"26"}]}}}`),
		asctest.JSON(http.StatusOK, `{"data":{"type":"betaRecruitmentCriteria","id":"criteria-1","attributes":{"deviceFamilyOsVersionFilters":[{"deviceFamily":"IPHONE","minimumOsInclusive":"26"}]}}}`))

	stdout, stderr, err := runCommand(t, []string{"testflight", "recruitment", "set", "--group", "group-1", "--os-version-filter", "IPHONE=26"})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"id":"criteria-1"`) {
		t.Fatalf("expected criteria id in output, got %q", stdout)
	}
}

func TestTestFlightRecruitmentSetCreatesWhenMissing(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/betaGroups/group-2/betaRecruitmentCriteria"),
		asctest.JSON(http.StatusNotFound, `{"errors":[{"code":"NOT_FOUND","title":"Not Found"}]}`))
	reg.Register(asctest.POST("/v1/betaRecruitmentCriteria").JSONBody(`{"data":{"type":"betaRecruitmentCriteria","attributes":{"deviceFamilyOsVersionFilters":[{"deviceFamily":"IPHONE","minimumOsInclusive":"26"}]},"relationships":{"betaGroup":{"data":{"type":"betaGroups","id":"group-2"}}}}}`),
		asctest.JSON(http.StatusCreated, `{"data":{"type":"betaRecruitmentCriteria","id":"criteria-2","attributes":{"deviceFamilyOsVersionFilters":[{"deviceFamily":"IPHONE","minimumOsInclusive":"26"}]}}}`))

	stdout, stderr, err := runCommand(t, []string{"testflight", "recruitment", "set", "--group", "group-2", "--os-version-filter", "IPHONE=26"})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"id":"criteria-2"`) {
		t.Fatalf("expected criteria id in output, got %q", stdout)
	}
}

func TestTestFlightRecruitmentSetReturnsErrorOnFetchFailure(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/betaGroups/group-3/betaRecruitmentCriteria"),
		asctest.JSON(http.StatusForbidden, `{"errors":[{"code":"FORBIDDEN","title":"Forbidden"}]}`))

	stdout, stderr, runErr := runCommand(t, []string{"testflight", "recruitment", "set", "--group", "group-3", "--os-version-filter", "IPHONE=26"})

	if runErr == nil {
		t.Fatal("expected error, got nil")
	}
	if errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("expected non-help error, got %v", runErr)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(runErr.Error(), "failed to fetch existing criteria") {
		t.Fatalf("expected fetch error, got %v (stderr: %q)", runErr, stderr)
	}
}

func TestTestFlightDistributionAndRecruitmentSendRequest(t *testing.T) {
	runTestFlightRequestCases(t, []testFlightRequestCase{
		{
			name:       "distribution view",
			args:       []string{"testflight", "distribution", "view", "--build-id", "build-1", "--limit", "1"},
			wantMethod: http.MethodGet,
			wantPath:   "/v1/buildBetaDetails",
			wantQuery:  url.Values{"filter[build]": {"build-1"}, "limit": {"1"}},
			respBody:   `{"data":[{"type":"buildBetaDetails","id":"detail-1","attributes":{"autoNotifyEnabled":true}}],"links":{"next":""}}`,
			wantStdout: []string{`"id":"detail-1"`},
		},
		{
			name:       "distribution build view",
			args:       []string{"testflight", "distribution", "build", "view", "--id", "detail-1"},
			wantMethod: http.MethodGet,
			wantPath:   "/v1/buildBetaDetails/detail-1/build",
			respBody:   `{"data":{"type":"builds","id":"build-1","attributes":{"version":"1.0"}}}`,
			wantStdout: []string{`"id":"build-1"`},
		},
		{
			name:       "distribution edit",
			args:       []string{"testflight", "distribution", "edit", "--id", "detail-1", "--auto-notify"},
			wantMethod: http.MethodPatch,
			wantPath:   "/v1/buildBetaDetails/detail-1",
			wantBody:   `{"data":{"type":"buildBetaDetails","id":"detail-1","attributes":{"autoNotifyEnabled":true}}}`,
			respBody:   `{"data":{"type":"buildBetaDetails","id":"detail-1","attributes":{"autoNotifyEnabled":true}}}`,
			wantStdout: []string{`"id":"detail-1"`},
		},
		{
			name:       "recruitment options",
			args:       []string{"testflight", "recruitment", "options", "--fields", "deviceFamilyOsVersions", "--limit", "1"},
			wantMethod: http.MethodGet,
			wantPath:   "/v1/betaRecruitmentCriterionOptions",
			wantQuery:  url.Values{"fields[betaRecruitmentCriterionOptions]": {"deviceFamilyOsVersions"}, "limit": {"1"}},
			respBody:   `{"data":[{"type":"betaRecruitmentCriterionOptions","id":"opt-1","attributes":{"deviceFamilyOsVersions":[{"deviceFamily":"IPHONE","osVersions":["26"]}]}}],"links":{"next":""}}`,
			wantStdout: []string{`"id":"opt-1"`},
		},
		{
			name:       "recruitment delete",
			args:       []string{"testflight", "recruitment", "delete", "--id", "criteria-1", "--confirm"},
			wantMethod: http.MethodDelete,
			wantPath:   "/v1/betaRecruitmentCriteria/criteria-1",
			respStatus: http.StatusNoContent,
			wantStdout: []string{`"id":"criteria-1"`, `"deleted":true`},
		},
	})
}
