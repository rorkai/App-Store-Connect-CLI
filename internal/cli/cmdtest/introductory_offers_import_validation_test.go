package cmdtest

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

func TestSubscriptionsIntroductoryOffersImport_InvalidInputReturnsUsage(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))
	asctest.NewRegistry(t).Install()

	importArgs := func(body string, extra ...string) []string {
		args := []string{
			"subscriptions", "offers", "introductory", "import",
			"--subscription-id", "SUB_ID",
			"--input", writeTempIntroOffersCSV(t, body),
		}
		if len(extra) == 0 {
			extra = []string{"--confirm"}
		}
		return append(args, extra...)
	}

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "missing subscription id",
			args:    []string{"subscriptions", "offers", "introductory", "import", "--input", "offers.csv"},
			wantErr: "Error: --subscription-id is required",
		},
		{
			name:    "missing input",
			args:    []string{"subscriptions", "offers", "introductory", "import", "--subscription-id", "SUB_ID"},
			wantErr: "Error: --input is required",
		},
		{
			name: "invalid start date",
			args: []string{
				"subscriptions", "offers", "introductory", "import",
				"--subscription-id", "SUB_ID",
				"--input", "offers.csv",
				"--start-date", "2026-99-99",
			},
			wantErr: "--start-date must be in YYYY-MM-DD format",
		},
		{
			name: "invalid end date",
			args: []string{
				"subscriptions", "offers", "introductory", "import",
				"--subscription-id", "SUB_ID",
				"--input", "offers.csv",
				"--end-date", "2026-13-01",
			},
			wantErr: "--end-date must be in YYYY-MM-DD format",
		},
		{
			name: "invalid offer duration",
			args: []string{
				"subscriptions", "offers", "introductory", "import",
				"--subscription-id", "SUB_ID",
				"--input", "offers.csv",
				"--offer-duration", "bad",
			},
			wantErr: "--offer-duration must be one of:",
		},
		{
			name: "invalid offer mode",
			args: []string{
				"subscriptions", "offers", "introductory", "import",
				"--subscription-id", "SUB_ID",
				"--input", "offers.csv",
				"--offer-mode", "bad",
			},
			wantErr: "--offer-mode must be one of:",
		},
		{
			name: "negative number of periods",
			args: []string{
				"subscriptions", "offers", "introductory", "import",
				"--subscription-id", "SUB_ID",
				"--input", "offers.csv",
				"--number-of-periods", "-1",
			},
			wantErr: "--number-of-periods must be greater than or equal to 0",
		},
		{
			name:    "unknown CSV column",
			args:    importArgs("territory,unknown\nUSA,value\n"),
			wantErr: "unknown CSV column",
		},
		{
			name:    "unknown three-letter territory",
			args:    importArgs("territory\nZZZ\n", "--offer-duration", "ONE_WEEK", "--offer-mode", "FREE_TRIAL", "--number-of-periods", "1", "--dry-run"),
			wantErr: `row 1: territory "ZZZ" could not be mapped`,
		},
		{
			name:    "empty file",
			args:    importArgs(""),
			wantErr: "CSV file is empty",
		},
		{
			name:    "missing territory header",
			args:    importArgs("offer_mode\nFREE_TRIAL\n"),
			wantErr: `CSV header must include required column "territory"`,
		},
		{
			name:    "invalid row start date",
			args:    importArgs("territory,start_date\nUSA,2026-15-01\n"),
			wantErr: "row 1: --start-date must be in YYYY-MM-DD format",
		},
		{
			name:    "invalid row end date",
			args:    importArgs("territory,end_date\nUSA,2026-04-99\n"),
			wantErr: "row 1: --end-date must be in YYYY-MM-DD format",
		},
		{
			name:    "invalid row offer mode",
			args:    importArgs("territory,offer_mode\nUSA,bad\n"),
			wantErr: "row 1: --offer-mode must be one of:",
		},
		{
			name:    "invalid row offer duration",
			args:    importArgs("territory,offer_duration\nUSA,bad\n"),
			wantErr: "row 1: --offer-duration must be one of:",
		},
		{
			name:    "non integer periods",
			args:    importArgs("territory,number_of_periods\nUSA,abc\n"),
			wantErr: "row 1: number_of_periods must be a positive integer",
		},
		{
			name:    "zero periods",
			args:    importArgs("territory,number_of_periods\nUSA,0\n"),
			wantErr: "row 1: number_of_periods must be a positive integer",
		},
		{
			name:    "missing territory",
			args:    importArgs("territory,offer_mode\n,FREE_TRIAL\n"),
			wantErr: "row 1: territory is required",
		},
		{
			name:    "unmappable territory",
			args:    importArgs("territory\nAtlantis\n"),
			wantErr: `row 1: territory "Atlantis" could not be mapped to an App Store Connect territory ID`,
		},
		{
			name:    "missing offer mode after fallback",
			args:    importArgs("territory\nUSA\n"),
			wantErr: "row 1: offer_mode is required",
		},
		{
			name:    "missing offer duration after fallback",
			args:    importArgs("territory,offer_mode\nUSA,FREE_TRIAL\n"),
			wantErr: "row 1: offer_duration is required",
		},
		{
			name:    "missing periods after fallback",
			args:    importArgs("territory,offer_mode,offer_duration\nUSA,FREE_TRIAL,ONE_WEEK\n"),
			wantErr: "row 1: number_of_periods is required",
		},
	}

	runValidationTests(t, tests)
}

func writeTempIntroOffersCSV(t *testing.T, body string) string {
	t.Helper()

	csvPath := filepath.Join(t.TempDir(), "offers.csv")
	if err := os.WriteFile(csvPath, []byte(body), 0o600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}
	return csvPath
}

func TestSubscriptionsIntroductoryOffersImport_DryRunUsesDefaultsWithoutMutations(t *testing.T) {
	setupAuth(t)

	asctest.NewRegistry(t).Install()

	csvPath := writeTempIntroOffersCSV(t, "territory\nUSA\nAfghanistan\n")

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	type importSummary struct {
		DryRun  bool `json:"dryRun"`
		Total   int  `json:"total"`
		Created int  `json:"created"`
		Failed  int  `json:"failed"`
	}

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"subscriptions", "offers", "introductory", "import",
			"--subscription-id", "SUB_ID",
			"--input", csvPath,
			"--offer-duration", "one_week",
			"--offer-mode", "free_trial",
			"--number-of-periods", "1",
			"--dry-run",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var summary importSummary
	if err := json.Unmarshal([]byte(stdout), &summary); err != nil {
		t.Fatalf("parse JSON summary: %v", err)
	}
	if !summary.DryRun {
		t.Fatalf("expected dryRun=true")
	}
	if summary.Total != 2 || summary.Created != 2 || summary.Failed != 0 {
		t.Fatalf("unexpected summary: %+v", summary)
	}
}
