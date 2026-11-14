package cmdtest

import (
	"net/http"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

func TestPricingUsageErrors(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")

	runUsageExitCases(t, []usageExitCase{
		{
			name:    "PricingTiersCommand_MissingApp",
			args:    []string{"pricing", "tiers"},
			wantErr: "--app is required",
		},
		{
			name:    "PricingScheduleCreate_TierAndPricePointMutualExclusion",
			args:    []string{"pricing", "schedule", "create", "--app", "APP", "--price-point", "PP", "--tier", "5", "--base-territory", "USA", "--start-date", "2026-03-01"},
			wantErr: "mutually exclusive",
		},
		{
			name:    "PricingScheduleCreate_AllThreeMutualExclusion",
			args:    []string{"pricing", "schedule", "create", "--app", "APP", "--price-point", "PP", "--tier", "5", "--price", "4.99", "--base-territory", "USA", "--start-date", "2026-03-01"},
			wantErr: "mutually exclusive",
		},
		{
			name:    "PricingScheduleCreate_TierRequiresTerritory",
			args:    []string{"pricing", "schedule", "create", "--app", "APP", "--tier", "5", "--start-date", "2026-03-01"},
			wantErr: "--base-territory is required",
		},
		{
			name:    "PricingScheduleCreatePriceValidationError",
			args:    []string{"pricing", "schedule", "create", "--app", "app-1", "--price", "abc", "--base-territory", "USA", "--start-date", "2026-03-01"},
			wantErr: "Error: --price must be a number",
		},
		{
			name:    "PricingScheduleCreatePriceValidationFiniteError",
			args:    []string{"pricing", "schedule", "create", "--app", "app-1", "--price", "NaN", "--base-territory", "USA", "--start-date", "2026-03-01"},
			wantErr: "Error: --price must be a finite number",
		},
	})
}

func TestPricingTiersCommand_Success(t *testing.T) {
	setupAuth(t)

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/apps/app-1/appPricePoints", "filter[territory]=USA", "limit=200"),
		asctest.JSON(http.StatusOK, `{
			"data":[
				{"type":"appPricePoints","id":"pp-099","attributes":{"customerPrice":"0.99","proceeds":"0.70"}},
				{"type":"appPricePoints","id":"pp-299","attributes":{"customerPrice":"2.99","proceeds":"2.10"}},
				{"type":"appPricePoints","id":"pp-199","attributes":{"customerPrice":"1.99","proceeds":"1.40"}}
			],
			"links":{"next":""}
		}`))

	t.Setenv("HOME", t.TempDir())

	stdout, stderr, err := runCommand(t, []string{
		"pricing", "tiers",
		"--app", "app-1",
		"--territory", "USA",
		"--refresh",
	})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	if !strings.Contains(stdout, "pp-099") {
		t.Fatalf("expected pp-099 in output, got %q", stdout)
	}
	if !strings.Contains(stdout, "pp-199") {
		t.Fatalf("expected pp-199 in output, got %q", stdout)
	}
}

func TestPricingTiersCommand_NormalizesTerritory(t *testing.T) {
	setupAuth(t)

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/apps/app-1/appPricePoints", "filter[territory]=USA", "limit=200"),
		asctest.JSON(http.StatusOK, `{
			"data":[
				{"type":"appPricePoints","id":"pp-099","attributes":{"customerPrice":"0.99","proceeds":"0.70"}}
			],
			"links":{"next":""}
		}`))

	t.Setenv("HOME", t.TempDir())

	if _, _, err := runCommand(t, []string{
		"pricing", "tiers",
		"--app", "app-1",
		"--territory", "United States",
	}); err != nil {
		t.Fatalf("run error: %v", err)
	}
}

func TestPricingScheduleCreate_TierFlag(t *testing.T) {
	setupAuth(t)

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/apps/app-1/appPricePoints", "filter[territory]=USA", "limit=200"),
		asctest.JSON(http.StatusOK, `{
			"data":[
				{"type":"appPricePoints","id":"pp-099","attributes":{"customerPrice":"0.99","proceeds":"0.70"}},
				{"type":"appPricePoints","id":"pp-199","attributes":{"customerPrice":"1.99","proceeds":"1.40"}},
				{"type":"appPricePoints","id":"pp-299","attributes":{"customerPrice":"2.99","proceeds":"2.10"}}
			],
			"links":{"next":""}
		}`))
	reg.Register(asctest.POST("/v1/appPriceSchedules").JSONBody(`{"data":{"type":"appPriceSchedules","relationships":{"app":{"data":{"type":"apps","id":"app-1"}},"baseTerritory":{"data":{"type":"territories","id":"USA"}},"manualPrices":{"data":[{"type":"appPrices","id":"${local-manual-price-1}"}]}}},"included":[{"type":"appPrices","id":"${local-manual-price-1}","attributes":{"startDate":"2026-03-01"},"relationships":{"appPricePoint":{"data":{"type":"appPricePoints","id":"pp-199"}}}}]}`),
		asctest.JSON(http.StatusCreated, `{"data":{"type":"appPriceSchedules","id":"sched-1","attributes":{}}}`))

	t.Setenv("HOME", t.TempDir())

	stdout, stderr, err := runCommand(t, []string{
		"pricing", "schedule", "create",
		"--app", "app-1",
		"--tier", "2",
		"--base-territory", "USA",
		"--start-date", "2026-03-01",
		"--refresh",
	})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"id":"sched-1"`) {
		t.Fatalf("expected schedule output, got %q", stdout)
	}
}
