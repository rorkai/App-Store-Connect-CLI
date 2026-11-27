package asc

import (
	"bytes"
	"io"
	"strings"
	"testing"
)

func TestCatalogHumanReceiptsKeepSavedPlanIdentityAndRecovery(t *testing.T) {
	for format, print := range map[string]func(io.Writer, any) error{"table": PrintTableTo, "markdown": PrintMarkdownTo} {
		for _, tc := range []struct {
			name    string
			result  CatalogResult
			want    []string
			missing []string
		}{
			{
				name:   "saved empty plan",
				result: CatalogResult{AppID: "123", Status: "planned", PlanSHA256: strings.Repeat("a", 64)},
				want:   []string{"Plan SHA256", strings.Repeat("a", 64)},
			},
			{
				name: "partial saved apply",
				result: CatalogResult{
					AppID: "123", Status: "partial", PlanSHA256: strings.Repeat("b", 64),
					Recovery: "Read the current catalog with a new plan before applying the remaining work.",
					Items:    []CatalogItem{{Kind: "subscription", Key: "monthly", Status: "unknown"}},
				},
				want: []string{"Plan SHA256", strings.Repeat("b", 64), "Recovery", "Read the current catalog with a new plan before applying the remaining work.", "monthly", "unknown"},
			},
			{
				name:    "legacy receipt",
				result:  CatalogResult{AppID: "123", Status: "planned", Items: []CatalogItem{{Kind: "subscription", Key: "monthly", Status: "planned"}}},
				want:    []string{"monthly", "planned"},
				missing: []string{"Plan SHA256", "Recovery"},
			},
		} {
			t.Run(format+"/"+tc.name, func(t *testing.T) {
				var output bytes.Buffer
				if err := print(&output, &tc.result); err != nil {
					t.Fatal(err)
				}
				for _, want := range tc.want {
					if !strings.Contains(output.String(), want) {
						t.Fatalf("receipt lost %q: %s", want, &output)
					}
				}
				for _, absent := range tc.missing {
					if strings.Contains(output.String(), absent) {
						t.Fatalf("legacy receipt gained %q: %s", absent, &output)
					}
				}
			})
		}
	}
}
