package cmdtest

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/migrate"
)

func TestMigrateExportReviewInformationRoundTrip(t *testing.T) {
	const clipWarning = "Warning: version has no App Clip default experience\n"
	tests := []struct {
		name       string
		status     int
		body       string
		existing   map[string]string
		wantFiles  map[string]string
		wantTotal  int
		wantStderr string
	}{
		{
			name:   "review detail",
			status: http.StatusOK,
			body: `{"data":{"type":"appStoreReviewDetails","id":"DETAIL_ID","attributes":{
				"contactFirstName":"Ada","contactLastName":"","contactPhone":"+1 555 0100","contactEmail":"ada@example.com",
				"demoAccountName":"reviewer","demoAccountPassword":"s3cret","demoAccountRequired":true,"notes":"Line one\nLine two"}}}`,
			wantFiles: map[string]string{
				"first_name.txt":    "Ada\n",
				"phone_number.txt":  "+1 555 0100\n",
				"email_address.txt": "ada@example.com\n",
				"demo_user.txt":     "reviewer\n",
				"demo_required.txt": "true\n",
				"notes.txt":         "Line one\nLine two\n",
			},
			wantTotal:  6,
			wantStderr: "Warning: demo account password not exported to metadata/review_information/demo_password.txt\n" + clipWarning,
		},
		{
			name:       "no review detail",
			status:     http.StatusNotFound,
			body:       `{"errors":[{"status":"404","code":"NOT_FOUND","title":"Not found"}]}`,
			wantStderr: clipWarning,
		},
		{
			name:   "no review detail over an earlier export",
			status: http.StatusNotFound,
			body:   `{"errors":[{"status":"404","code":"NOT_FOUND","title":"Not found"}]}`,
			existing: map[string]string{
				"first_name.txt": "Old\n", "last_name.txt": "Old\n", "phone_number.txt": "0\n", "email_address.txt": "old@example.com\n",
				"demo_user.txt": "old\n", "demo_required.txt": "true\n", "demo_account_required.txt": "true\n", "notes.txt": "old\n",
				"demo_password.txt": "kept\n", "keep.txt": "unrelated\n",
			},
			wantFiles:  map[string]string{"demo_password.txt": "kept\n", "keep.txt": "unrelated\n"},
			wantStderr: clipWarning,
		},
		{
			name:   "review detail over a conflicting alternate demo required file",
			status: http.StatusOK,
			body: `{"data":{"type":"appStoreReviewDetails","id":"DETAIL_ID","attributes":{
				"contactFirstName":"Ada","contactPhone":"+1 555 0100","contactEmail":"ada@example.com",
				"demoAccountName":"reviewer","demoAccountRequired":true,"notes":"Line one\nLine two"}}}`,
			existing: map[string]string{"demo_account_required.txt": "false\n"},
			wantFiles: map[string]string{
				"first_name.txt":    "Ada\n",
				"phone_number.txt":  "+1 555 0100\n",
				"email_address.txt": "ada@example.com\n",
				"demo_user.txt":     "reviewer\n",
				"demo_required.txt": "true\n",
				"notes.txt":         "Line one\nLine two\n",
			},
			wantTotal:  6,
			wantStderr: clipWarning,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupAuth(t)
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
			outputDir := filepath.Join(t.TempDir(), "exported")
			reviewDir := filepath.Join(outputDir, "metadata", "review_information")
			if test.existing != nil {
				if err := os.MkdirAll(reviewDir, 0o755); err != nil {
					t.Fatal(err)
				}
				for name, content := range test.existing {
					writeFile(t, filepath.Join(reviewDir, name), content)
				}
			}

			reg := asctest.NewRegistry(t)
			reg.Install()
			reg.Register(asctest.GET("/v1/appStoreVersions/VERSION_ID", "include=app"),
				asctest.JSON(http.StatusOK, `{"data":{"type":"appStoreVersions","id":"VERSION_ID","attributes":{"versionString":"1.0","platform":"IOS"},"relationships":{"app":{"data":{"type":"apps","id":"APP_ID"}}}}}`))
			reg.Register(asctest.GET("/v1/appStoreVersions/VERSION_ID/appStoreVersionLocalizations", "limit=200"),
				asctest.JSON(http.StatusOK, `{"data":[],"links":{"next":""}}`))
			reg.Register(asctest.GET("/v1/apps/APP_ID/appInfos"),
				asctest.JSON(http.StatusOK, `{"data":[],"links":{"next":""}}`))
			reg.Register(asctest.GET("/v1/appStoreVersions/VERSION_ID/appClipDefaultExperience"),
				asctest.JSON(http.StatusOK, `{"data":null}`))
			reg.Register(asctest.GET("/v1/appStoreVersions/VERSION_ID/appStoreReviewDetail"),
				asctest.JSON(test.status, test.body))

			stdout, stderr := captureOutput(t, func() {
				if err := RootCommand("1.2.3").ParseAndRun(context.Background(), []string{
					"migrate", "export", "--app", "APP_ID", "--version-id", "VERSION_ID", "--output-dir", outputDir, "--output", "json",
				}); err != nil {
					t.Errorf("migrate export error: %v", err)
				}
			})
			if stderr != test.wantStderr {
				t.Fatalf("stderr = %q, want %q", stderr, test.wantStderr)
			}
			var result struct {
				TotalFiles int `json:"totalFiles"`
			}
			if err := json.Unmarshal([]byte(stdout), &result); err != nil {
				t.Fatalf("decode export result: %v (%s)", err, stdout)
			}
			if result.TotalFiles != test.wantTotal {
				t.Fatalf("totalFiles = %d, want %d", result.TotalFiles, test.wantTotal)
			}

			if test.wantFiles == nil {
				if _, err := os.Stat(reviewDir); !os.IsNotExist(err) {
					t.Fatalf("review_information exists without a review detail, stat error = %v", err)
				}
				return
			}
			entries, err := os.ReadDir(reviewDir)
			if err != nil {
				t.Fatalf("read review_information: %v", err)
			}
			got := map[string]string{}
			for _, entry := range entries {
				data, err := os.ReadFile(filepath.Join(reviewDir, entry.Name()))
				if err != nil {
					t.Fatalf("read %s: %v", entry.Name(), err)
				}
				got[entry.Name()] = string(data)
			}
			if !reflect.DeepEqual(got, test.wantFiles) {
				t.Fatalf("review_information files = %#v, want %#v", got, test.wantFiles)
			}
			if test.status != http.StatusOK {
				return
			}

			importOut, _ := captureOutput(t, func() {
				rootCmd := RootCommand("1.2.3")
				rootCmd.FlagSet.SetOutput(io.Discard)
				if err := rootCmd.ParseAndRun(context.Background(), []string{
					"migrate", "import", "--app", "APP_ID", "--version-id", "VERSION_ID", "--fastlane-dir", outputDir, "--dry-run", "--skip-screenshots", "--output", "json",
				}); err != nil {
					t.Errorf("migrate import --dry-run error: %v", err)
				}
			})
			var plan migrate.MigrateImportResult
			if err := json.Unmarshal([]byte(importOut), &plan); err != nil {
				t.Fatalf("decode import plan: %v (%s)", err, importOut)
			}
			first, phone, email, user, notes, required := "Ada", "+1 555 0100", "ada@example.com", "reviewer", "Line one\nLine two", true
			want := &migrate.ReviewInformation{ContactFirstName: &first, ContactPhone: &phone, ContactEmail: &email, DemoAccountName: &user, DemoAccountRequired: &required, Notes: &notes}
			if !reflect.DeepEqual(plan.ReviewInformation, want) {
				gotJSON, _ := json.Marshal(plan.ReviewInformation)
				wantJSON, _ := json.Marshal(want)
				t.Fatalf("imported review information = %s, want %s", gotJSON, wantJSON)
			}
		})
	}
}
