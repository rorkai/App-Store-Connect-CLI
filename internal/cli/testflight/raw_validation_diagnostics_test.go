package testflight

import (
	"context"
	"errors"
	"flag"
	"testing"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func TestTestFlightRawValidationDiagnosticsPreserveContracts(t *testing.T) {
	tests := []struct {
		name      string
		command   func() *ffcli.Command
		args      []string
		wantError string
		wantCode  shared.DiagnosticCode
		wantParam string
	}{
		{
			name:      "beta groups list limit",
			command:   BetaGroupsListCommand,
			args:      []string{"--limit", "201"},
			wantError: "beta-groups list: --limit must be between 1 and 200",
			wantCode:  shared.DiagnosticInvalidInput,
			wantParam: "--limit",
		},
		{
			name:      "beta groups relationships limit",
			command:   BetaGroupsRelationshipsGetCommand,
			args:      []string{"--group-id", "group-1", "--type", "betaTesters", "--limit", "201"},
			wantError: "testflight beta-groups relationships view: --limit must be between 1 and 200",
			wantCode:  shared.DiagnosticInvalidInput,
			wantParam: "--limit",
		},
		{
			name:      "beta license agreements list limit",
			command:   BetaLicenseAgreementsListCommand,
			args:      []string{"--limit", "201"},
			wantError: "beta-license-agreements list: --limit must be between 1 and 200",
			wantCode:  shared.DiagnosticInvalidInput,
			wantParam: "--limit",
		},
		{
			name:      "beta testers list limit",
			command:   BetaTestersListCommand,
			args:      []string{"--limit", "201"},
			wantError: "beta-testers list: --limit must be between 1 and 200",
			wantCode:  shared.DiagnosticInvalidInput,
			wantParam: "--limit",
		},
		{
			name:      "beta tester apps limit",
			command:   BetaTestersAppsListCommand,
			args:      []string{"--tester-id", "tester-1", "--limit", "201"},
			wantError: "testflight beta-testers apps list: --limit must be between 1 and 200",
			wantCode:  shared.DiagnosticInvalidInput,
			wantParam: "--limit",
		},
		{
			name:      "beta tester beta groups limit",
			command:   BetaTestersBetaGroupsListCommand,
			args:      []string{"--tester-id", "tester-1", "--limit", "201"},
			wantError: "testflight beta-testers beta-groups list: --limit must be between 1 and 200",
			wantCode:  shared.DiagnosticInvalidInput,
			wantParam: "--limit",
		},
		{
			name:      "beta tester builds limit",
			command:   BetaTestersBuildsListCommand,
			args:      []string{"--tester-id", "tester-1", "--limit", "201"},
			wantError: "testflight beta-testers builds list: --limit must be between 1 and 200",
			wantCode:  shared.DiagnosticInvalidInput,
			wantParam: "--limit",
		},
		{
			name:      "beta tester relationships limit",
			command:   BetaTestersRelationshipsGetCommand,
			args:      []string{"--tester-id", "tester-1", "--type", "apps", "--limit", "201"},
			wantError: "testflight beta-testers relationships view: --limit must be between 1 and 200",
			wantCode:  shared.DiagnosticInvalidInput,
			wantParam: "--limit",
		},
		{
			name:      "review view limit",
			command:   TestFlightReviewGetCommand,
			args:      []string{"--limit", "201"},
			wantError: "testflight review view: --limit must be between 1 and 200",
			wantCode:  shared.DiagnosticInvalidInput,
			wantParam: "--limit",
		},
		{
			name:      "review submissions limit",
			command:   TestFlightReviewSubmissionsListCommand,
			args:      []string{"--build-id", "build-1", "--limit", "201"},
			wantError: "testflight review submissions list: --limit must be between 1 and 200",
			wantCode:  shared.DiagnosticInvalidInput,
			wantParam: "--limit",
		},
		{
			name:      "beta details limit",
			command:   TestFlightBetaDetailsGetCommand,
			args:      []string{"--limit", "201"},
			wantError: "testflight beta-details view: --limit must be between 1 and 200",
			wantCode:  shared.DiagnosticInvalidInput,
			wantParam: "--limit",
		},
		{
			name:      "recruitment options limit",
			command:   TestFlightRecruitmentOptionsCommand,
			args:      []string{"--limit", "201"},
			wantError: "testflight recruitment options: --limit must be between 1 and 200",
			wantCode:  shared.DiagnosticInvalidInput,
			wantParam: "--limit",
		},
		{
			name:      "beta groups relationship aliases conflict",
			command:   BetaGroupsRelationshipsGetCommand,
			args:      []string{"--group-id", "group-1", "--id", "group-2", "--type", "betaTesters"},
			wantError: "testflight beta-groups relationships view: --group-id and --id must match",
			wantCode:  shared.DiagnosticConflictingInput,
		},
		{
			name:      "beta group app aliases conflict",
			command:   BetaGroupsAppGetCommand,
			args:      []string{"--group-id", "group-1", "--id", "group-2"},
			wantError: "testflight beta-groups app view: --group-id and --id must match",
			wantCode:  shared.DiagnosticConflictingInput,
		},
		{
			name:      "beta group recruitment criteria aliases conflict",
			command:   BetaGroupsRecruitmentCriteriaGetCommand,
			args:      []string{"--group-id", "group-1", "--id", "group-2"},
			wantError: "testflight beta-groups beta-recruitment-criteria view: --group-id and --id must match",
			wantCode:  shared.DiagnosticConflictingInput,
		},
		{
			name:      "beta group compatible build aliases conflict",
			command:   BetaGroupsRecruitmentCriterionCompatibleBuildCheckGetCommand,
			args:      []string{"--group-id", "group-1", "--id", "group-2"},
			wantError: "testflight beta-groups beta-recruitment-criterion-compatible-build-check view: --group-id and --id must match",
			wantCode:  shared.DiagnosticConflictingInput,
		},
		{
			name:      "beta tester apps aliases conflict",
			command:   BetaTestersAppsListCommand,
			args:      []string{"--tester-id", "tester-1", "--id", "tester-2"},
			wantError: "testflight beta-testers apps list: --tester-id and --id must match",
			wantCode:  shared.DiagnosticConflictingInput,
		},
		{
			name:      "beta tester beta groups aliases conflict",
			command:   BetaTestersBetaGroupsListCommand,
			args:      []string{"--tester-id", "tester-1", "--id", "tester-2"},
			wantError: "testflight beta-testers beta-groups list: --tester-id and --id must match",
			wantCode:  shared.DiagnosticConflictingInput,
		},
		{
			name:      "beta tester builds aliases conflict",
			command:   BetaTestersBuildsListCommand,
			args:      []string{"--tester-id", "tester-1", "--id", "tester-2"},
			wantError: "testflight beta-testers builds list: --tester-id and --id must match",
			wantCode:  shared.DiagnosticConflictingInput,
		},
		{
			name:      "beta tester relationships aliases conflict",
			command:   BetaTestersRelationshipsGetCommand,
			args:      []string{"--tester-id", "tester-1", "--id", "tester-2", "--type", "apps"},
			wantError: "testflight beta-testers relationships view: --tester-id and --id must match",
			wantCode:  shared.DiagnosticConflictingInput,
		},
		{
			name:      "beta tester metrics aliases conflict",
			command:   BetaTestersMetricsCommand,
			args:      []string{"--tester-id", "tester-1", "--id", "tester-2"},
			wantError: "testflight beta-testers metrics: --tester-id and --id must match",
			wantCode:  shared.DiagnosticConflictingInput,
		},
		{
			name:      "beta tester metrics period",
			command:   BetaTestersMetricsCommand,
			args:      []string{"--period", "P10D"},
			wantError: "--period must be one of: P7D, P30D, P90D, P365D",
			wantCode:  shared.DiagnosticInvalidInput,
			wantParam: "--period",
		},
		{
			name:      "recruitment options fields",
			command:   TestFlightRecruitmentOptionsCommand,
			args:      []string{"--fields", "invalid"},
			wantError: "testflight recruitment options: --fields must be one of: deviceFamilyOsVersions",
			wantCode:  shared.DiagnosticInvalidInput,
			wantParam: "--fields",
		},
		{
			name:      "recruitment set filter syntax",
			command:   TestFlightRecruitmentSetCommand,
			args:      []string{"--group", "group-1", "--os-version-filter", "IPHONE26"},
			wantError: "testflight recruitment set: --os-version-filter must use DEVICE_FAMILY=MIN_OS (e.g., IPHONE=26)",
			wantCode:  shared.DiagnosticInvalidInput,
			wantParam: "--os-version-filter",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cmd := test.command()
			if err := cmd.FlagSet.Parse(test.args); err != nil {
				t.Fatalf("parse flags: %v", err)
			}

			var runErr error
			stderr := captureTestFlightDiagnosticStderr(t, func() {
				runErr = cmd.Exec(context.Background(), nil)
			})
			if runErr == nil {
				t.Fatal("expected validation error")
			}
			if got := runErr.Error(); got != test.wantError {
				t.Fatalf("error = %q, want %q", got, test.wantError)
			}
			if stderr != "" {
				t.Fatalf("stderr = %q, want empty output for an unreported validation error", stderr)
			}
			if errors.Is(runErr, flag.ErrHelp) {
				t.Fatalf("errors.Is(flag.ErrHelp) = true, want generic exit semantics: %v", runErr)
			}
			if shared.ClassifyUsageError(runErr) != "" {
				t.Fatalf("usage classification = %q, want generic error classification", shared.ClassifyUsageError(runErr))
			}
			if !shared.IsValidationError(runErr) {
				t.Fatal("expected validation classification")
			}

			diagnostic, ok := shared.DiagnosticFromError(runErr)
			if !ok {
				t.Fatal("expected structured diagnostic")
			}
			if diagnostic.Code != test.wantCode || diagnostic.Parameter != test.wantParam {
				t.Fatalf("diagnostic = %+v, want code %q parameter %q", diagnostic, test.wantCode, test.wantParam)
			}
		})
	}
}

func TestTestFlightRawValidationHelpersPreserveContracts(t *testing.T) {
	tests := []struct {
		name      string
		run       func() error
		wantError string
		wantCode  shared.DiagnosticCode
		wantParam string
	}{
		{
			name: "unsupported beta group relationship",
			run: func() error {
				_, err := getBetaGroupRelationshipList(context.Background(), nil, "unsupported", "group-1")
				return err
			},
			wantError: "unsupported relationship type \"unsupported\"",
			wantCode:  shared.DiagnosticInvalidInput,
			wantParam: "--type",
		},
		{
			name: "unsupported beta tester relationship",
			run: func() error {
				_, err := getBetaTesterRelationshipList(context.Background(), nil, "unsupported", "tester-1")
				return err
			},
			wantError: "unsupported relationship type \"unsupported\"",
			wantCode:  shared.DiagnosticInvalidInput,
			wantParam: "--type",
		},
		{
			name: "invalid beta tester period",
			run: func() error {
				_, err := normalizeBetaTesterUsagePeriod("P10D")
				return err
			},
			wantError: "--period must be one of: P7D, P30D, P90D, P365D",
			wantCode:  shared.DiagnosticInvalidInput,
			wantParam: "--period",
		},
		{
			name: "invalid recruitment fields",
			run: func() error {
				_, err := normalizeBetaRecruitmentCriterionOptionsFields("invalid")
				return err
			},
			wantError: "--fields must be one of: deviceFamilyOsVersions",
			wantCode:  shared.DiagnosticInvalidInput,
			wantParam: "--fields",
		},
		{
			name: "missing recruitment filter separator",
			run: func() error {
				_, err := parseDeviceFamilyOsVersionFilters("IPHONE26")
				return err
			},
			wantError: "--os-version-filter must use DEVICE_FAMILY=MIN_OS (e.g., IPHONE=26)",
			wantCode:  shared.DiagnosticInvalidInput,
			wantParam: "--os-version-filter",
		},
		{
			name: "missing recruitment filter version",
			run: func() error {
				_, err := parseDeviceFamilyOsVersionFilters("IPHONE=")
				return err
			},
			wantError: "--os-version-filter must use DEVICE_FAMILY=MIN_OS (e.g., IPHONE=26)",
			wantCode:  shared.DiagnosticInvalidInput,
			wantParam: "--os-version-filter",
		},
		{
			name: "invalid recruitment filter range",
			run: func() error {
				_, err := parseDeviceFamilyOsVersionFilters("IPHONE=17..")
				return err
			},
			wantError: "--os-version-filter must use DEVICE_FAMILY=MIN_OS[..MAX_OS]",
			wantCode:  shared.DiagnosticInvalidInput,
			wantParam: "--os-version-filter",
		},
		{
			name: "invalid recruitment device family",
			run: func() error {
				_, err := normalizeBetaRecruitmentDeviceFamily("ANDROID")
				return err
			},
			wantError: "--os-version-filter device family must be one of: IPHONE, IPAD, MAC, VISION, APPLE_TV, APPLE_WATCH",
			wantCode:  shared.DiagnosticInvalidInput,
			wantParam: "--os-version-filter",
		},
		{
			name: "empty recruitment criteria state",
			run: func() error {
				return validateBetaRecruitmentCriteriaID("")
			},
			wantError: "testflight recruitment set: criteria id is empty",
			wantCode:  shared.DiagnosticStateNotReady,
			wantParam: "--group",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			runErr := test.run()
			if runErr == nil {
				t.Fatal("expected validation error")
			}
			if got := runErr.Error(); got != test.wantError {
				t.Fatalf("error = %q, want %q", got, test.wantError)
			}
			if errors.Is(runErr, flag.ErrHelp) {
				t.Fatalf("errors.Is(flag.ErrHelp) = true, want generic exit semantics: %v", runErr)
			}
			if shared.ClassifyUsageError(runErr) != "" {
				t.Fatalf("usage classification = %q, want generic error classification", shared.ClassifyUsageError(runErr))
			}
			if !shared.IsValidationError(runErr) {
				t.Fatal("expected validation classification")
			}

			diagnostic, ok := shared.DiagnosticFromError(runErr)
			if !ok {
				t.Fatal("expected structured diagnostic")
			}
			if diagnostic.Code != test.wantCode || diagnostic.Parameter != test.wantParam {
				t.Fatalf("diagnostic = %+v, want code %q parameter %q", diagnostic, test.wantCode, test.wantParam)
			}
		})
	}
}
