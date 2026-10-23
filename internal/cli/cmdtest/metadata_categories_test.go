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

// metadataCategoriesHTTP serves version and app-info resolution plus category
// relationships from remote, and records each PATCH body. When storeOnPatch is
// true, a PATCH updates remote with the submitted relationships.
func metadataCategoriesHTTP(t *testing.T, remote map[string]string, storeOnPatch bool, patches *[]map[string]string) {
	t.Helper()
	withMetadataReviewHTTP(t, func(req *http.Request) (*http.Response, error) {
		switch {
		case req.URL.Path == "/v1/apps/app-1/appInfos":
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"appInfos","id":"appinfo-1","attributes":{"state":"PREPARE_FOR_SUBMISSION"}}]}`), nil
		case req.URL.Path == "/v1/apps/app-1/appStoreVersions":
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.2.3","platform":"IOS"}}],"links":{"next":""}}`), nil
		case req.URL.Path == "/v1/appInfos/appinfo-1" && req.Method == http.MethodPatch:
			var payload struct {
				Data struct {
					Relationships map[string]struct {
						Data struct {
							ID string `json:"id"`
						} `json:"data"`
					} `json:"relationships"`
				} `json:"data"`
			}
			if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
				t.Fatalf("decode PATCH payload: %v", err)
			}
			submitted := map[string]string{}
			for name, relationship := range payload.Data.Relationships {
				submitted[name] = relationship.Data.ID
				if storeOnPatch {
					remote[name] = relationship.Data.ID
				}
			}
			*patches = append(*patches, submitted)
			return jsonHTTPResponse(http.StatusOK, `{"data":{"type":"appInfos","id":"appinfo-1"}}`), nil
		case strings.HasPrefix(req.URL.Path, "/v1/appInfos/appinfo-1/relationships/") && req.Method == http.MethodGet:
			name := strings.TrimPrefix(req.URL.Path, "/v1/appInfos/appinfo-1/relationships/")
			if id := remote[name]; id != "" {
				return jsonHTTPResponse(http.StatusOK, `{"data":{"type":"appCategories","id":"`+id+`"}}`), nil
			}
			return jsonHTTPResponse(http.StatusOK, `{"data":null}`), nil
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})
}

func TestMetadataCategoriesPullPlanApproveApply(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	remote := map[string]string{"primaryCategory": "GAMES", "primarySubcategoryOne": "GAMES_ACTION"}
	var patches []map[string]string
	metadataCategoriesHTTP(t, remote, true, &patches)

	dir := t.TempDir()
	reviewDir := filepath.Join(t.TempDir(), "review")
	common := []string{"--app", "app-1", "--version", "1.2.3", "--dir", dir, "--include", "categories", "--output", "json"}

	runMetadataReviewCommand(t, append([]string{"metadata", "pull"}, common...))
	pulled, err := os.ReadFile(filepath.Join(dir, "categories.json"))
	if err != nil {
		t.Fatalf("read pulled categories: %v", err)
	}
	var pulledValues map[string]string
	if err := json.Unmarshal(pulled, &pulledValues); err != nil {
		t.Fatalf("parse pulled categories: %v\n%s", err, pulled)
	}
	if len(pulledValues) != 2 || pulledValues["primaryCategory"] != "GAMES" || pulledValues["primarySubcategoryOne"] != "GAMES_ACTION" {
		t.Fatalf("pulled categories = %v", pulledValues)
	}

	edited := `{"primaryCategory":"GAMES","primarySubcategoryOne":"GAMES_PUZZLE","secondaryCategory":"ENTERTAINMENT"}`
	if err := os.WriteFile(filepath.Join(dir, "categories.json"), []byte(edited), 0o644); err != nil {
		t.Fatalf("write categories: %v", err)
	}

	planStdout := runMetadataReviewCommand(t, append([]string{"metadata", "plan", "--review-dir", reviewDir}, common...))
	var planOut struct {
		Plan struct {
			Adds    []map[string]string `json:"adds"`
			Updates []map[string]string `json:"updates"`
		} `json:"plan"`
	}
	if err := json.Unmarshal([]byte(planStdout), &planOut); err != nil {
		t.Fatalf("parse plan: %v\n%s", err, planStdout)
	}
	if len(planOut.Plan.Adds) != 1 || planOut.Plan.Adds[0]["key"] != "categories:secondaryCategory" || planOut.Plan.Adds[0]["to"] != "ENTERTAINMENT" {
		t.Fatalf("plan adds = %v", planOut.Plan.Adds)
	}
	if len(planOut.Plan.Updates) != 1 || planOut.Plan.Updates[0]["key"] != "categories:primarySubcategoryOne" ||
		planOut.Plan.Updates[0]["from"] != "GAMES_ACTION" || planOut.Plan.Updates[0]["to"] != "GAMES_PUZZLE" {
		t.Fatalf("plan updates = %v", planOut.Plan.Updates)
	}
	if len(patches) != 0 {
		t.Fatalf("plan must not mutate, patches=%v", patches)
	}

	runMetadataReviewCommand(t, []string{"metadata", "approve", "--review-dir", reviewDir, "--scope", "categories", "--output", "json"})

	applyStdout := runMetadataReviewCommand(t, append([]string{"metadata", "apply", "--review-dir", reviewDir, "--confirm"}, common...))
	var applyOut struct {
		Applied bool `json:"applied"`
		Actions []struct {
			Scope  string `json:"scope"`
			Status string `json:"status"`
		} `json:"actions"`
	}
	if err := json.Unmarshal([]byte(applyStdout), &applyOut); err != nil {
		t.Fatalf("parse apply: %v\n%s", err, applyStdout)
	}
	if !applyOut.Applied || len(applyOut.Actions) != 1 || applyOut.Actions[0].Scope != "categories" || applyOut.Actions[0].Status != "succeeded" {
		t.Fatalf("apply output = %s", applyStdout)
	}
	if len(patches) != 1 || len(patches[0]) != 2 || patches[0]["primarySubcategoryOne"] != "GAMES_PUZZLE" || patches[0]["secondaryCategory"] != "ENTERTAINMENT" {
		t.Fatalf("PATCH relationships = %v, want only the changed categories", patches)
	}
}

func TestMetadataCategoriesApplyFailsWhenReadBackDiffers(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Chdir(t.TempDir())

	var patches []map[string]string
	metadataCategoriesHTTP(t, map[string]string{"primaryCategory": "GAMES"}, false, &patches)

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "categories.json"), []byte(`{"primaryCategory":"UTILITIES"}`), 0o644); err != nil {
		t.Fatalf("write categories: %v", err)
	}

	stdout, stderr, runErr := runMetadataReviewCommandRaw(t, []string{
		"metadata", "apply", "--app", "app-1", "--version", "1.2.3", "--dir", dir, "--include", "categories", "--output", "json",
	})
	if runErr == nil {
		t.Fatalf("expected apply to fail when App Store Connect keeps a different category\nstdout=%s", stdout)
	}
	if len(patches) == 0 {
		t.Fatal("expected a categories PATCH")
	}
	var out struct {
		Applied bool `json:"applied"`
		Failed  int  `json:"failed"`
		Actions []struct {
			Error string `json:"error"`
		} `json:"actions"`
	}
	if err := json.Unmarshal([]byte(stdout), &out); err != nil {
		t.Fatalf("parse apply: %v\n%s", err, stdout)
	}
	if out.Applied || out.Failed != 1 || len(out.Actions) != 1 || !strings.Contains(out.Actions[0].Error, `primaryCategory="GAMES" (want "UTILITIES")`) {
		t.Fatalf("apply output = %s\nstderr=%s", stdout, stderr)
	}
}

func TestMetadataCategoriesRejectsInvalidCanonicalFile(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	tests := []struct {
		name    string
		body    string
		wantErr string
	}{
		{name: "missing file", wantErr: "--include categories requires"},
		{name: "unknown field", body: `{"tertiaryCategory":"GAMES"}`, wantErr: `unknown field "tertiaryCategory"`},
		{name: "null clears are unsupported", body: `{"secondaryCategory":null}`, wantErr: "secondaryCategory cannot be null"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			if test.body != "" {
				if err := os.WriteFile(filepath.Join(dir, "categories.json"), []byte(test.body), 0o644); err != nil {
					t.Fatalf("write categories: %v", err)
				}
			}
			withMetadataReviewHTTP(t, func(req *http.Request) (*http.Response, error) {
				t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
				return nil, nil
			})
			stdout, stderr, runErr := runMetadataReviewCommandRaw(t, []string{
				"metadata", "plan", "--app", "app-1", "--version", "1.2.3", "--dir", dir, "--include", "categories",
				"--review-dir", filepath.Join(t.TempDir(), "review"),
			})
			if !errors.Is(runErr, flag.ErrHelp) {
				t.Fatalf("expected usage error, got %v", runErr)
			}
			if stdout != "" || !strings.Contains(stderr, test.wantErr) {
				t.Fatalf("stdout=%q stderr=%q, want %q", stdout, stderr, test.wantErr)
			}
		})
	}
}
