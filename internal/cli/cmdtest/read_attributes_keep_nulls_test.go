package cmdtest

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

func TestReadCommandsPrintAppleNullAndEmptyAttributes(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		responses map[string]string
		attrs     map[string]string
		deps      bool // the command is built on shared.Deps: run it on test deps
	}{
		{
			name: "review details-get",
			args: []string{"review", "details-get", "--id", "detail-1"},
			responses: map[string]string{
				"/v1/appStoreReviewDetails/detail-1": `{"data":{"type":"appStoreReviewDetails","id":"detail-1","attributes":{"contactFirstName":"Dev","demoAccountRequired":false,"demoAccountName":null,"demoAccountPassword":null,"notes":""}}}`,
			},
			attrs: map[string]string{
				"demoAccountRequired": "false",
				"demoAccountName":     "null",
				"demoAccountPassword": "null",
				"notes":               `""`,
			},
		},
		{
			name: "review details-for-version with include-sensitive",
			args: []string{"review", "details-for-version", "--version-id", "version-1", "--include-sensitive"},
			responses: map[string]string{
				"/v1/appStoreVersions/version-1/appStoreReviewDetail": `{"data":{"type":"appStoreReviewDetails","id":"detail-1","attributes":{"demoAccountRequired":false,"demoAccountName":null,"demoAccountPassword":null}}}`,
			},
			attrs: map[string]string{
				"demoAccountRequired": "false",
				"demoAccountName":     "null",
				"demoAccountPassword": "null",
			},
		},
		{
			name: "localizations list version",
			args: []string{"localizations", "list", "--version", "version-1"},
			responses: map[string]string{
				"/v1/appStoreVersions/version-1/appStoreVersionLocalizations": `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-1","attributes":{"locale":"en-US","marketingUrl":null,"whatsNew":null}}],"links":{}}`,
			},
			attrs: map[string]string{
				"locale":       `"en-US"`,
				"marketingUrl": "null",
				"whatsNew":     "null",
			},
		},
		{
			name: "localizations list app-info",
			args: []string{"localizations", "list", "--app", "app-1", "--type", "app-info", "--app-info", "info-1"},
			responses: map[string]string{
				"/v1/appInfos/info-1/appInfoLocalizations": `{"data":[{"type":"appInfoLocalizations","id":"loc-1","attributes":{"locale":"en-US","name":"App","privacyChoicesUrl":null,"privacyPolicyText":null}}],"links":{}}`,
			},
			attrs: map[string]string{
				"privacyChoicesUrl": "null",
				"privacyPolicyText": "null",
			},
		},
		{
			name: "age-rating view",
			args: []string{"age-rating", "view", "--app-info-id", "info-1"},
			responses: map[string]string{
				"/v1/appInfos/info-1/ageRatingDeclaration": `{"data":{"type":"ageRatingDeclarations","id":"age-1","attributes":{"gambling":false,"developerAgeRatingInfoUrl":null,"gracRatingClassificationNumber":null,"kidsAgeBand":null}}}`,
			},
			attrs: map[string]string{
				"gambling":                       "false",
				"developerAgeRatingInfoUrl":      "null",
				"gracRatingClassificationNumber": "null",
				"kidsAgeBand":                    "null",
			},
			deps: true,
		},
		{
			name: "builds test-notes list",
			args: []string{"builds", "test-notes", "list", "--build-id", "build-1"},
			responses: map[string]string{
				"/v1/builds/build-1": `{"data":{"type":"builds","id":"build-1","attributes":{}}}`,
				"/v1/betaBuildLocalizations?filter[build]=build-1": `{"data":[{"type":"betaBuildLocalizations","id":"note-1","attributes":{"locale":"en-US","whatsNew":null}}],"links":{}}`,
			},
			attrs: map[string]string{
				"whatsNew": "null",
			},
		},
		{
			name: "builds test-notes view",
			args: []string{"builds", "test-notes", "view", "--localization-id", "note-1"},
			responses: map[string]string{
				"/v1/betaBuildLocalizations/note-1": `{"data":{"type":"betaBuildLocalizations","id":"note-1","attributes":{"locale":"en-US","whatsNew":null}}}`,
			},
			attrs: map[string]string{
				"whatsNew": "null",
			},
		},
		{
			name: "iap review-screenshots view",
			args: []string{"iap", "review-screenshots", "view", "--screenshot-id", "shot-1"},
			responses: map[string]string{
				"/v1/inAppPurchaseAppStoreReviewScreenshots/shot-1": `{"data":{"type":"inAppPurchaseAppStoreReviewScreenshots","id":"shot-1","attributes":{"fileName":"shot.png","uploadOperations":[],"sourceFileChecksum":null}}}`,
			},
			attrs: map[string]string{
				"uploadOperations":   "[]",
				"sourceFileChecksum": "null",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupAuth(t)
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
			reg := asctest.NewRegistry(t)
			for request, body := range test.responses {
				path, query, _ := strings.Cut(request, "?")
				reg.Register(asctest.GET(path, query), asctest.JSON(http.StatusOK, body))
			}

			var stdout string
			var err error
			if test.deps {
				stdout, _, err = runRootWithDeps(t, stubClient(t, reg), append(test.args, "--output", "json")...)
			} else {
				reg.Install()
				stdout, _, err = runCommand(t, append(test.args, "--output", "json"))
			}
			if err != nil {
				t.Fatalf("run error: %v", err)
			}

			var envelope struct {
				Data json.RawMessage `json:"data"`
			}
			if err := json.Unmarshal([]byte(stdout), &envelope); err != nil {
				t.Fatalf("unmarshal output: %v\nstdout=%s", err, stdout)
			}
			var resource struct {
				Attributes map[string]json.RawMessage `json:"attributes"`
			}
			data := envelope.Data
			var list []json.RawMessage
			if json.Unmarshal(data, &list) == nil {
				if len(list) != 1 {
					t.Fatalf("expected one resource, got %s", data)
				}
				data = list[0]
			}
			if err := json.Unmarshal(data, &resource); err != nil {
				t.Fatalf("unmarshal resource: %v\nstdout=%s", err, stdout)
			}
			for name, want := range test.attrs {
				got, present := resource.Attributes[name]
				if !present || string(got) != want {
					t.Errorf("attributes.%s = %s (present=%t), want %s\nstdout=%s", name, got, present, want, stdout)
				}
			}
		})
	}
}
