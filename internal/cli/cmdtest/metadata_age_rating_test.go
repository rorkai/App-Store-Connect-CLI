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

// metadataAgeRatingHTTP serves version and app-info resolution plus the age
// rating declaration in remote, and records each PATCH attributes object.
// When storeOnPatch is true, a PATCH updates remote.
func metadataAgeRatingHTTP(t *testing.T, remote map[string]any, storeOnPatch bool, patches *[]map[string]any) {
	t.Helper()
	withMetadataReviewHTTP(t, func(req *http.Request) (*http.Response, error) {
		switch {
		case req.URL.Path == "/v1/apps/app-1/appInfos":
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"appInfos","id":"appinfo-1","attributes":{"state":"PREPARE_FOR_SUBMISSION"}}]}`), nil
		case req.URL.Path == "/v1/apps/app-1/appStoreVersions":
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.2.3","platform":"IOS"}}],"links":{"next":""}}`), nil
		case req.URL.Path == "/v1/appInfos/appinfo-1/ageRatingDeclaration" && req.Method == http.MethodGet:
			body, err := json.Marshal(map[string]any{"data": map[string]any{"type": "ageRatingDeclarations", "id": "decl-1", "attributes": remote}})
			if err != nil {
				t.Fatalf("encode declaration: %v", err)
			}
			return jsonHTTPResponse(http.StatusOK, string(body)), nil
		case req.URL.Path == "/v1/ageRatingDeclarations/decl-1" && req.Method == http.MethodPatch:
			var payload struct {
				Data struct {
					Attributes map[string]any `json:"attributes"`
				} `json:"data"`
			}
			if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
				t.Fatalf("decode PATCH payload: %v", err)
			}
			*patches = append(*patches, payload.Data.Attributes)
			if storeOnPatch {
				for name, value := range payload.Data.Attributes {
					remote[name] = value
				}
			}
			return jsonHTTPResponse(http.StatusOK, `{"data":{"type":"ageRatingDeclarations","id":"decl-1"}}`), nil
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})
}

func TestMetadataAgeRatingPullPlanApproveApply(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	remote := map[string]any{"gambling": false, "violenceRealistic": "NONE", "kidsAgeBand": nil}
	var patches []map[string]any
	metadataAgeRatingHTTP(t, remote, true, &patches)

	dir := t.TempDir()
	reviewDir := filepath.Join(t.TempDir(), "review")
	common := []string{"--app", "app-1", "--version", "1.2.3", "--dir", dir, "--include", "age-rating", "--output", "json"}

	runMetadataReviewCommand(t, append([]string{"metadata", "pull"}, common...))
	pulled, err := os.ReadFile(filepath.Join(dir, "age-rating.json"))
	if err != nil {
		t.Fatalf("read pulled age rating: %v", err)
	}
	var pulledValues map[string]any
	if err := json.Unmarshal(pulled, &pulledValues); err != nil {
		t.Fatalf("parse pulled age rating: %v\n%s", err, pulled)
	}
	if len(pulledValues) != 2 || pulledValues["gambling"] != false || pulledValues["violenceRealistic"] != "NONE" {
		t.Fatalf("pulled age rating = %v, want typed values without null fields", pulledValues)
	}

	edited := `{"gambling":true,"violenceRealistic":"INFREQUENT_OR_MILD","kidsAgeBand":"NINE_TO_ELEVEN"}`
	if err := os.WriteFile(filepath.Join(dir, "age-rating.json"), []byte(edited), 0o644); err != nil {
		t.Fatalf("write age rating: %v", err)
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
	if len(planOut.Plan.Adds) != 1 || planOut.Plan.Adds[0]["key"] != "age-rating:kidsAgeBand" || planOut.Plan.Adds[0]["to"] != "NINE_TO_ELEVEN" {
		t.Fatalf("plan adds = %v", planOut.Plan.Adds)
	}
	if len(planOut.Plan.Updates) != 2 ||
		planOut.Plan.Updates[0]["key"] != "age-rating:gambling" || planOut.Plan.Updates[0]["from"] != "false" || planOut.Plan.Updates[0]["to"] != "true" ||
		planOut.Plan.Updates[1]["key"] != "age-rating:violenceRealistic" || planOut.Plan.Updates[1]["from"] != "NONE" || planOut.Plan.Updates[1]["to"] != "INFREQUENT_OR_MILD" {
		t.Fatalf("plan updates = %v", planOut.Plan.Updates)
	}

	runMetadataReviewCommand(t, []string{"metadata", "approve", "--review-dir", reviewDir, "--scope", "age-rating", "--output", "json"})
	applyStdout := runMetadataReviewCommand(t, append([]string{"metadata", "apply", "--review-dir", reviewDir, "--confirm"}, common...))
	if !strings.Contains(applyStdout, `"applied":true`) {
		t.Fatalf("apply output = %s", applyStdout)
	}
	if len(patches) != 1 || len(patches[0]) != 3 || patches[0]["gambling"] != true ||
		patches[0]["violenceRealistic"] != "INFREQUENT_OR_MILD" || patches[0]["kidsAgeBand"] != "NINE_TO_ELEVEN" {
		t.Fatalf("PATCH attributes = %v, want only the changed fields with their API types", patches)
	}
}

func TestMetadataAgeRatingApplyFailsWhenReadBackDiffers(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Chdir(t.TempDir())

	var patches []map[string]any
	metadataAgeRatingHTTP(t, map[string]any{"lootBox": false}, false, &patches)

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "age-rating.json"), []byte(`{"lootBox":true}`), 0o644); err != nil {
		t.Fatalf("write age rating: %v", err)
	}
	stdout, _, runErr := runRootCommand(t, []string{
		"metadata", "apply", "--app", "app-1", "--version", "1.2.3", "--dir", dir, "--include", "age-rating", "--output", "json",
	})
	if runErr == nil || len(patches) == 0 {
		t.Fatalf("expected a PATCH and a failed apply, err=%v patches=%v", runErr, patches)
	}
	if !strings.Contains(stdout, `"failed":1`) || !strings.Contains(stdout, `lootBox=\"false\" (want \"true\")`) {
		t.Fatalf("apply output = %s", stdout)
	}
}

func TestMetadataAgeRatingRejectsInvalidValuesBeforeRequests(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	tests := []struct {
		name    string
		body    string
		wantErr string
	}{
		{name: "boolean as string", body: `{"gambling":"true"}`, wantErr: "gambling must be a boolean"},
		{name: "unknown enum value", body: `{"violenceRealistic":"SOMETIMES"}`, wantErr: "violenceRealistic must be one of NONE, INFREQUENT_OR_MILD"},
		{name: "invalid URL", body: `{"developerAgeRatingInfoUrl":"not a URL"}`, wantErr: "developerAgeRatingInfoUrl must be a valid URL"},
		{name: "relative URL", body: `{"developerAgeRatingInfoUrl":"/age-rating"}`, wantErr: "developerAgeRatingInfoUrl must be a valid URL"},
		{name: "social media without user content", body: `{"socialMedia":true,"userGeneratedContent":false}`, wantErr: "socialMedia true cannot be combined with userGeneratedContent false"},
		{name: "restricted social media without age assurance", body: `{"socialMediaAgeRestricted":true,"ageAssurance":false}`, wantErr: "socialMediaAgeRestricted true cannot be combined with ageAssurance false"},
		{name: "restricted social media without social media", body: `{"socialMediaAgeRestricted":true,"socialMedia":false}`, wantErr: "socialMediaAgeRestricted true cannot be combined with socialMedia false"},
		{name: "restricted social media without user content", body: `{"socialMediaAgeRestricted":true,"userGeneratedContent":false}`, wantErr: "socialMediaAgeRestricted true cannot be combined with userGeneratedContent false"},
	}
	for _, test := range tests {
		for _, command := range []string{"plan", "apply"} {
			t.Run(test.name+"/"+command, func(t *testing.T) {
				withMetadataReviewHTTP(t, func(req *http.Request) (*http.Response, error) {
					t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
					return nil, nil
				})
				dir := t.TempDir()
				versionDir := filepath.Join(dir, "version", "1.2.3")
				if err := os.MkdirAll(versionDir, 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(versionDir, "en-US.json"), []byte(`{"description":"new description"}`), 0o644); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dir, "age-rating.json"), []byte(test.body), 0o644); err != nil {
					t.Fatalf("write age rating: %v", err)
				}
				args := []string{
					"metadata", command, "--app", "app-1", "--version", "1.2.3", "--dir", dir, "--include", "localizations,age-rating",
					"--review-dir", filepath.Join(t.TempDir(), "review"),
				}
				if command == "apply" {
					args = append(args, "--confirm")
				}
				stdout, stderr, runErr := runRootCommand(t, args)
				if !errors.Is(runErr, flag.ErrHelp) || stdout != "" || !strings.Contains(stderr, test.wantErr) {
					t.Fatalf("err=%v stdout=%q stderr=%q", runErr, stdout, stderr)
				}
			})
		}
	}
}

func TestMetadataAgeRatingValidatesResultingStateBeforeMutation(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	for _, test := range []struct {
		name    string
		local   string
		remote  map[string]any
		wantErr string
	}{
		{name: "remote user content false", local: `{"socialMedia":true}`, remote: map[string]any{"userGeneratedContent": false}, wantErr: "socialMedia true cannot be combined with userGeneratedContent false"},
		{name: "remote social media false", local: `{"socialMediaAgeRestricted":true}`, remote: map[string]any{"socialMedia": false}, wantErr: "socialMediaAgeRestricted true cannot be combined with socialMedia false"},
		{name: "local prerequisite disabled", local: `{"userGeneratedContent":false}`, remote: map[string]any{"socialMedia": true, "userGeneratedContent": true}, wantErr: "socialMedia true cannot be combined with userGeneratedContent false"},
		{name: "local prerequisite overrides remote", local: `{"socialMedia":true,"userGeneratedContent":true,"developerAgeRatingInfoUrl":"urn:example:age-rating"}`, remote: map[string]any{"userGeneratedContent": false}},
		{name: "remote prerequisite unknown", local: `{"socialMedia":true}`, remote: map[string]any{"userGeneratedContent": nil}, wantErr: "socialMedia true requires userGeneratedContent true"},
		{name: "remote prerequisite already true", local: `{"socialMedia":true}`, remote: map[string]any{"userGeneratedContent": true}},
	} {
		t.Run(test.name, func(t *testing.T) {
			var patches []map[string]any
			metadataAgeRatingHTTP(t, test.remote, true, &patches)
			transport := http.DefaultTransport
			mutations := 0
			withMetadataReviewHTTP(t, func(req *http.Request) (*http.Response, error) {
				if req.Method != http.MethodGet {
					mutations++
				}
				switch req.Method + " " + req.URL.Path {
				case "GET /v1/appStoreVersions/version-1/appStoreVersionLocalizations":
					return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-1","attributes":{"locale":"en-US","description":"old description"}}]}`), nil
				case "PATCH /v1/appStoreVersionLocalizations/loc-1":
					return jsonHTTPResponse(http.StatusOK, `{"data":{"type":"appStoreVersionLocalizations","id":"loc-1","attributes":{"locale":"en-US","description":"new description"}}}`), nil
				default:
					return transport.RoundTrip(req)
				}
			})
			dir := t.TempDir()
			versionDir := filepath.Join(dir, "version", "1.2.3")
			if err := os.MkdirAll(versionDir, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(versionDir, "en-US.json"), []byte(`{"description":"new description"}`), 0o644); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "age-rating.json"), []byte(test.local), 0o644); err != nil {
				t.Fatal(err)
			}
			stdout, stderr, runErr := runRootCommand(t, []string{
				"metadata", "apply", "--app", "app-1", "--version", "1.2.3", "--dir", dir, "--include", "localizations,age-rating", "--output", "json",
			})
			if test.wantErr != "" {
				if !errors.Is(runErr, flag.ErrHelp) || stdout != "" || !strings.Contains(stderr, test.wantErr) || mutations != 0 {
					t.Fatalf("err=%v stdout=%q stderr=%q mutations=%d", runErr, stdout, stderr, mutations)
				}
				return
			}
			if runErr != nil || stderr != "" || mutations != 2 || len(patches) != 1 {
				t.Fatalf("err=%v stdout=%q stderr=%q mutations=%d patches=%v", runErr, stdout, stderr, mutations, patches)
			}
			var wantPatch map[string]any
			if err := json.Unmarshal([]byte(test.local), &wantPatch); err != nil {
				t.Fatal(err)
			}
			if len(patches[0]) != len(wantPatch) {
				t.Fatalf("PATCH = %v, want %v", patches[0], wantPatch)
			}
			for field, value := range wantPatch {
				if patches[0][field] != value {
					t.Fatalf("PATCH %s = %v, want %v", field, patches[0][field], value)
				}
			}
		})
	}
}
