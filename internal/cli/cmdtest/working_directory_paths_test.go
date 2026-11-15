package cmdtest

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
)

func TestCommandsResolveRelativePathsAgainstDepsWorkingDirectory(t *testing.T) {
	const report = "date\tunits\n2026-10-01\t3\n"
	var compressed bytes.Buffer
	writer := gzip.NewWriter(&compressed)
	if _, err := writer.Write([]byte(report)); err != nil {
		t.Fatalf("gzip: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("gzip: %v", err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		analyticsPages := map[string]string{
			"/reports":   `{"data":[{"type":"analyticsReports","id":"report-1"}],"links":{}}`,
			"/instances": `{"data":[{"type":"analyticsReportInstances","id":"INSTANCE_ID"}],"links":{}}`,
			"/segments":  `{"data":[{"type":"analyticsReportSegments","id":"segment-1","attributes":{"url":"https://mzstatic.com/report.csv.gz"}}],"links":{}}`,
			"/builds/DSYM_BUILD_ID": `{"data":{"type":"builds","id":"DSYM_BUILD_ID","attributes":{"version":"42","processingState":"VALID"}},` +
				`"included":[{"type":"buildBundles","id":"bundle-1","attributes":{"bundleId":"com.example.app","dSYMUrl":"https://downloads.example.com/app.dSYM.zip"}}]}`,
			"/betaTesters": `{"data":[],"links":{}}`,
		}
		for suffix, page := range analyticsPages {
			if strings.HasSuffix(req.URL.Path, suffix) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(page))
				return
			}
		}
		w.Header().Set("Content-Type", "application/a-gzip")
		_, _ = w.Write(compressed.Bytes())
	}))
	t.Cleanup(server.Close)
	serverURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("parse server URL: %v", err)
	}
	serverTransport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		cloned := req.Clone(req.Context())
		cloned.URL.Scheme = serverURL.Scheme
		cloned.URL.Host = serverURL.Host
		return server.Client().Transport.RoundTrip(cloned)
	})

	tests := []struct {
		name       string
		args       []string
		files      map[string]string
		want       map[string]string
		exist      []string
		wantStdout string
		wantErr    func(dir string) string
	}{
		{
			name: "finance reports",
			args: []string{"finance", "reports", "--vendor", "12345678", "--report-type", "FINANCIAL", "--region", "US", "--date", "2026-10", "--output", "report.tsv.gz", "--decompress"},
			want: map[string]string{"report.tsv.gz": compressed.String(), "report.tsv": report},
		},
		{
			name: "analytics sales",
			args: []string{"analytics", "sales", "--vendor", "12345678", "--type", "SALES", "--subtype", "SUMMARY", "--frequency", "DAILY", "--date", "2026-10-01", "--output", "report.tsv.gz", "--decompress"},
			want: map[string]string{"report.tsv.gz": compressed.String(), "report.tsv": report},
		},
		{
			name: "analytics download",
			args: []string{"analytics", "download", "--request-id", "11111111-2222-4333-8444-555555555555", "--instance-id", "INSTANCE_ID", "--output", "report.csv.gz", "--decompress"},
			want: map[string]string{"report.csv.gz": compressed.String(), "report.csv": report},
		},
		{
			name: "performance download for an app",
			args: []string{"performance", "download", "--app", "123456789", "--output", "metrics.json.gz", "--decompress"},
			want: map[string]string{"metrics.json.gz": compressed.String(), "metrics.json": report},
		},
		{
			name: "performance download for a build",
			args: []string{"performance", "download", "--build-id", "BUILD_ID", "--output", "metrics.json.gz", "--decompress"},
			want: map[string]string{"metrics.json.gz": compressed.String(), "metrics.json": report},
		},
		{
			name: "performance download for a diagnostic",
			args: []string{"performance", "download", "--diagnostic-id", "SIGNATURE_ID", "--output", "logs.json.gz", "--decompress"},
			want: map[string]string{"logs.json.gz": compressed.String(), "logs.json": report},
		},
		{
			name:  "routing-coverage create",
			args:  []string{"routing-coverage", "create", "--version-id", "VERSION_ID", "--file", "coverage.json"},
			files: map[string]string{"coverage.json": "{}"},
			wantErr: func(dir string) string {
				return fmt.Sprintf("file must use the .geojson extension: %q", filepath.Join(dir, "coverage.json"))
			},
		},
		{
			name:    "iap import",
			args:    []string{"iap", "import", "--app", "123456789", "--file", "iap.json", "--dry-run"},
			files:   map[string]string{"iap.json": `{"products":[]}`},
			wantErr: func(string) string { return "the import file must contain at least one product" },
		},
		{
			name: "builds dsyms",
			args: []string{"builds", "dsyms", "--build-id", "DSYM_BUILD_ID", "--output-dir", "dsyms", "--output", "json"},
			want: map[string]string{filepath.Join("dsyms", "com.example.app-42.dSYM.zip"): compressed.String()},
		},
		{
			name:  "testflight testers export",
			args:  []string{"testflight", "testers", "export", "--app", "123456789", "--output", "testers.csv"},
			exist: []string{"testers.csv"},
		},
		{
			name:    "reviews digest",
			args:    []string{"reviews", "digest", "--app", "app-1", "--state-file", "state.json"},
			files:   map[string]string{"state.json": "{}"},
			wantErr: func(string) string { return "has no valid cursor" },
		},
		{
			name:  "docs init",
			args:  []string{"docs", "init", "--path", "ASC.md", "--link=false"},
			exist: []string{"ASC.md"},
		},
		{
			name:  "certificates csr generate",
			args:  []string{"certificates", "csr", "generate", "--key-out", "cert.key", "--csr-out", "cert.csr", "--output", "json"},
			exist: []string{"cert.key", "cert.csr"},
		},
		{
			name:    "encryption declarations exempt-declare",
			args:    []string{"encryption", "declarations", "exempt-declare", "--plist", "Info.plist"},
			files:   map[string]string{"Info.plist": "not a plist"},
			wantErr: func(string) string { return "failed to decode plist" },
		},
		{
			name:  "migrate validate",
			args:  []string{"migrate", "validate", "--fastlane-dir", "fastlane"},
			files: map[string]string{filepath.Join("fastlane", "metadata", "en-US", "name.txt"): "App"},
		},
		{
			name: "migrate import discovers the fastlane directory",
			args: []string{"migrate", "import", "--app", "123456789", "--version-id", "VERSION_ID", "--dry-run", "--skip-screenshots"},
			files: map[string]string{
				filepath.Join("fastlane", "Deliverfile"):                   "",
				filepath.Join("fastlane", "metadata", "en-US", "name.txt"): "App",
			},
			wantStdout: `"appInfoLocalizations":[{"locale":"en-US","name":"App"}]`,
		},
		{
			name:    "testflight testers import",
			args:    []string{"testflight", "testers", "import", "--app", "123456789", "--input", "testers.csv", "--dry-run"},
			files:   map[string]string{"testers.csv": ""},
			wantErr: func(string) string { return "CSV" },
		},
		{
			name:    "subscriptions pricing prices import",
			args:    []string{"subscriptions", "pricing", "prices", "import", "--subscription-id", "SUB_ID", "--input", "prices.csv", "--dry-run"},
			files:   map[string]string{"prices.csv": "x"},
			wantErr: func(string) string { return `CSV header must include required column "territory"` },
		},
		{
			name:    "subscriptions offers introductory import",
			args:    []string{"subscriptions", "offers", "introductory", "import", "--subscription-id", "SUB_ID", "--input", "offers.csv", "--dry-run"},
			files:   map[string]string{"offers.csv": "x"},
			wantErr: func(string) string { return `unknown CSV column "x"` },
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			processDir, depsDir := t.TempDir(), t.TempDir()
			t.Chdir(processDir)
			for name, content := range test.files {
				if err := os.MkdirAll(filepath.Dir(filepath.Join(depsDir, name)), 0o755); err != nil {
					t.Fatalf("create the directory of %s: %v", name, err)
				}
				if err := os.WriteFile(filepath.Join(depsDir, name), []byte(content), 0o600); err != nil {
					t.Fatalf("write %s: %v", name, err)
				}
			}

			d, stdout, stderr := newTestDeps(t, stubServerClient(t, server))
			d.Getwd = func() (string, error) { return depsDir, nil }
			d.HTTPTransport = func() http.RoundTripper { return serverTransport }
			root := rootcmd.RootCommandWithDeps("1.2.3", d)
			if err := root.Parse(test.args); err != nil {
				t.Fatalf("parse error: %v", err)
			}
			err := root.Run(context.Background())
			if test.wantErr != nil {
				wantErr := test.wantErr(depsDir)
				if err == nil || !strings.Contains(err.Error(), wantErr) {
					t.Fatalf("run error = %v, want %q", err, wantErr)
				}
			} else if err != nil {
				t.Fatalf("run error = %v; stderr = %q", err, stderr.String())
			}
			for name, content := range test.want {
				data, err := os.ReadFile(filepath.Join(depsDir, name))
				if err != nil || string(data) != content {
					t.Fatalf("%s in the deps working directory = %q, %v; want the report", name, data, err)
				}
			}
			if !strings.Contains(stdout.String(), test.wantStdout) {
				t.Fatalf("stdout = %q, want it to contain %q", stdout.String(), test.wantStdout)
			}
			for _, name := range test.exist {
				if _, err := os.Stat(filepath.Join(depsDir, name)); err != nil {
					t.Fatalf("%s in the deps working directory: %v", name, err)
				}
			}
			if entries, err := os.ReadDir(processDir); err != nil || len(entries) != 0 {
				t.Fatalf("process directory entries = %v, %v; want none", entries, err)
			}
		})
	}
}
