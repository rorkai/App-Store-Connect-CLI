package cmdtest

import (
	"encoding/json"
	"errors"
	"flag"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type metadataReviewDetailServer struct {
	detail       map[string]any
	storeOnWrite bool
	writes       []string
}

// metadataReviewInformationHTTP serves version resolution plus the review
// detail in server.detail (nil means none exists), and records each write as
// "METHOD attributes-json". When storeOnWrite is true, writes update detail.
func metadataReviewInformationHTTP(t *testing.T, server *metadataReviewDetailServer) {
	t.Helper()
	withMetadataReviewHTTP(t, func(req *http.Request) (*http.Response, error) {
		switch {
		case req.URL.Path == "/v1/apps/app-1/appStoreVersions":
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.2.3","platform":"IOS"}}],"links":{"next":""}}`), nil
		case req.URL.Path == "/v1/appStoreVersions/version-1/appStoreReviewDetail" && req.Method == http.MethodGet:
			if server.detail == nil {
				return jsonHTTPResponse(http.StatusNotFound, `{"errors":[{"status":"404","code":"NOT_FOUND","title":"Not found"}]}`), nil
			}
			body, err := json.Marshal(map[string]any{"data": map[string]any{"type": "appStoreReviewDetails", "id": "rd-1", "attributes": server.detail}})
			if err != nil {
				t.Fatalf("encode review detail: %v", err)
			}
			return jsonHTTPResponse(http.StatusOK, string(body)), nil
		case (req.URL.Path == "/v1/appStoreReviewDetails/rd-1" && req.Method == http.MethodPatch) ||
			(req.URL.Path == "/v1/appStoreReviewDetails" && req.Method == http.MethodPost):
			var payload struct {
				Data struct {
					Attributes map[string]any `json:"attributes"`
				} `json:"data"`
			}
			if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
				t.Fatalf("decode write payload: %v", err)
			}
			encoded, err := json.Marshal(payload.Data.Attributes)
			if err != nil {
				t.Fatalf("encode attributes: %v", err)
			}
			server.writes = append(server.writes, req.Method+" "+string(encoded))
			if server.storeOnWrite {
				if server.detail == nil {
					server.detail = map[string]any{}
				}
				for name, value := range payload.Data.Attributes {
					server.detail[name] = value
				}
			}
			return jsonHTTPResponse(http.StatusOK, `{"data":{"type":"appStoreReviewDetails","id":"rd-1"}}`), nil
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})
}

func TestMetadataReviewInformationPullPlanApplyKeepsPasswordOutOfFilesAndOutput(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	const oldPassword = "old-demo-secret"
	const newPassword = "new-demo-secret"
	t.Setenv("TEST_DEMO_PASSWORD", newPassword)

	server := &metadataReviewDetailServer{storeOnWrite: true, detail: map[string]any{
		"contactEmail": "review@example.com", "demoAccountRequired": true, "demoAccountName": "demo", "demoAccountPassword": oldPassword, "notes": "Use the demo account.",
	}}
	metadataReviewInformationHTTP(t, server)

	dir := t.TempDir()
	reviewDir := filepath.Join(t.TempDir(), "review")
	common := []string{"--app", "app-1", "--version", "1.2.3", "--dir", dir, "--include", "review-information", "--output", "json"}
	filePath := filepath.Join(dir, "review-information.json")

	runMetadataReviewCommand(t, append([]string{"metadata", "pull"}, common...))
	pulled, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("read pulled review information: %v", err)
	}
	var pulledValues map[string]any
	if err := json.Unmarshal(pulled, &pulledValues); err != nil {
		t.Fatalf("parse pulled review information: %v\n%s", err, pulled)
	}
	if strings.Contains(string(pulled), oldPassword) || pulledValues["demoAccountPassword"] != "@env:ASC_DEMO_ACCOUNT_PASSWORD" ||
		pulledValues["demoAccountRequired"] != true || pulledValues["notes"] != "Use the demo account." {
		t.Fatalf("pulled review information = %s", pulled)
	}

	pulledValues["notes"] = "Sign in with the demo account."
	pulledValues["demoAccountPassword"] = "@env:TEST_DEMO_PASSWORD"
	edited, err := json.Marshal(pulledValues)
	if err != nil {
		t.Fatalf("encode edited file: %v", err)
	}
	if err := os.WriteFile(filePath, edited, 0o644); err != nil {
		t.Fatalf("write review information: %v", err)
	}

	planStdout := runMetadataReviewCommand(t, append([]string{"metadata", "plan", "--review-dir", reviewDir}, common...))
	planArtifact, err := os.ReadFile(filepath.Join(reviewDir, "plan.json"))
	if err != nil {
		t.Fatalf("read plan artifact: %v", err)
	}
	for _, text := range []string{planStdout, string(planArtifact)} {
		if strings.Contains(text, oldPassword) || strings.Contains(text, newPassword) {
			t.Fatalf("plan leaked a demo password: %s", text)
		}
	}
	var planOut struct {
		Plan struct {
			Updates []map[string]string `json:"updates"`
		} `json:"plan"`
	}
	if err := json.Unmarshal([]byte(planStdout), &planOut); err != nil {
		t.Fatalf("parse plan: %v\n%s", err, planStdout)
	}
	if len(planOut.Plan.Updates) != 2 ||
		planOut.Plan.Updates[0]["key"] != "review-information:demoAccountPassword" || planOut.Plan.Updates[0]["from"] != "<redacted>" || planOut.Plan.Updates[0]["to"] != "<redacted> @env:TEST_DEMO_PASSWORD" ||
		planOut.Plan.Updates[1]["key"] != "review-information:notes" || planOut.Plan.Updates[1]["to"] != "Sign in with the demo account." {
		t.Fatalf("plan updates = %v", planOut.Plan.Updates)
	}

	runMetadataReviewCommand(t, []string{"metadata", "approve", "--review-dir", reviewDir, "--scope", "review-information", "--output", "json"})
	applyStdout := runMetadataReviewCommand(t, append([]string{"metadata", "apply", "--review-dir", reviewDir, "--confirm"}, common...))
	if !strings.Contains(applyStdout, `"applied":true`) || strings.Contains(applyStdout, newPassword) {
		t.Fatalf("apply output = %s", applyStdout)
	}
	want := `PATCH {"demoAccountPassword":"` + newPassword + `","notes":"Sign in with the demo account."}`
	if len(server.writes) != 1 || server.writes[0] != want {
		t.Fatalf("writes = %v, want [%s]", server.writes, want)
	}
}

func TestMetadataReviewInformationApplyCreatesMissingDetail(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	server := &metadataReviewDetailServer{storeOnWrite: true}
	metadataReviewInformationHTTP(t, server)

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "review-information.json"), []byte(`{"contactFirstName":"Ada","contactEmail":"ada@example.com","demoAccountRequired":false}`), 0o644); err != nil {
		t.Fatalf("write review information: %v", err)
	}
	stdout := runMetadataReviewCommand(t, []string{
		"metadata", "apply", "--app", "app-1", "--version", "1.2.3", "--dir", dir, "--include", "review-information", "--output", "json",
	})
	want := `POST {"contactEmail":"ada@example.com","contactFirstName":"Ada","demoAccountRequired":false}`
	if !strings.Contains(stdout, `"applied":true`) || len(server.writes) != 1 || server.writes[0] != want {
		t.Fatalf("stdout=%s writes=%v, want [%s]", stdout, server.writes, want)
	}
}

func TestMetadataReviewInformationReadBackMismatchRedactsPassword(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Chdir(t.TempDir())
	const newPassword = "rejected-demo-secret"
	t.Setenv("TEST_DEMO_PASSWORD", newPassword)

	server := &metadataReviewDetailServer{detail: map[string]any{"demoAccountPassword": "kept-secret"}}
	metadataReviewInformationHTTP(t, server)

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "review-information.json"), []byte(`{"demoAccountPassword":"@env:TEST_DEMO_PASSWORD"}`), 0o644); err != nil {
		t.Fatalf("write review information: %v", err)
	}
	stdout, stderr, runErr := runMetadataReviewCommandRaw(t, []string{
		"metadata", "apply", "--app", "app-1", "--version", "1.2.3", "--dir", dir, "--include", "review-information", "--output", "json",
	})
	if runErr == nil || len(server.writes) == 0 {
		t.Fatalf("expected a write and a failed apply, err=%v writes=%v", runErr, server.writes)
	}
	if !strings.Contains(stdout, "demoAccountPassword differs") {
		t.Fatalf("apply output = %s", stdout)
	}
	reports, _ := filepath.Glob(filepath.Join(".asc", "reports", "metadata-apply", "*.json"))
	texts := []string{stdout, stderr}
	for _, report := range reports {
		data, err := os.ReadFile(report)
		if err != nil {
			t.Fatalf("read failure artifact: %v", err)
		}
		texts = append(texts, string(data))
	}
	for _, text := range texts {
		if strings.Contains(text, newPassword) || strings.Contains(text, "kept-secret") {
			t.Fatalf("failed apply leaked a demo password: %s", text)
		}
	}
}

func TestMetadataReviewInformationRejectsUnsafePasswordSources(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	withMetadataReviewHTTP(t, func(req *http.Request) (*http.Response, error) {
		t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
		return nil, nil
	})

	parent := t.TempDir()
	if err := os.WriteFile(filepath.Join(parent, "outside.txt"), []byte("outside-secret"), 0o600); err != nil {
		t.Fatalf("write outside file: %v", err)
	}
	tests := []struct {
		name    string
		value   string
		wantErr string
	}{
		{name: "clear text", value: "plain-secret", wantErr: "demoAccountPassword must reference the secret with @env:NAME or @file:PATH"},
		{name: "file outside dir", value: "@file:../outside.txt", wantErr: "demoAccountPassword: cannot read the referenced file below --dir"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dir := filepath.Join(parent, "metadata-"+strings.ReplaceAll(test.name, " ", "-"))
			if err := os.MkdirAll(dir, 0o755); err != nil {
				t.Fatalf("mkdir: %v", err)
			}
			if err := os.WriteFile(filepath.Join(dir, "review-information.json"), []byte(`{"demoAccountPassword":"`+test.value+`"}`), 0o644); err != nil {
				t.Fatalf("write review information: %v", err)
			}
			stdout, stderr, runErr := runMetadataReviewCommandRaw(t, []string{
				"metadata", "plan", "--app", "app-1", "--version", "1.2.3", "--dir", dir, "--include", "review-information",
				"--review-dir", filepath.Join(t.TempDir(), "review"),
			})
			if !errors.Is(runErr, flag.ErrHelp) || stdout != "" || !strings.Contains(stderr, test.wantErr) ||
				strings.Contains(stderr, "plain-secret") || strings.Contains(stderr, "outside-secret") {
				t.Fatalf("err=%v stdout=%q stderr=%q", runErr, stdout, stderr)
			}
		})
	}
}
