package cmdtest

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

// TestGameCenterImageUploadTargets locks which relationship reaches Apple and
// which ID the receipt reports. A stray "localization":null on a version
// upload, or a version key on a localization upload, fails the exact match.
func TestGameCenterImageUploadTargets(t *testing.T) {
	tests := []struct {
		name              string
		group             string
		resource          string
		flag              string
		wantRelationships string
		wantReceiptField  string
		omitReceiptField  string
	}{
		{
			name:              "activity localization",
			group:             "activities",
			resource:          "gameCenterActivityImages",
			flag:              "--localization-id",
			wantRelationships: `{"localization":{"data":{"type":"gameCenterActivityLocalizations","id":"TARGET_ID"}}}`,
			wantReceiptField:  "localizationId",
			omitReceiptField:  "versionId",
		},
		{
			name:              "activity version",
			group:             "activities",
			resource:          "gameCenterActivityImages",
			flag:              "--version-id",
			wantRelationships: `{"version":{"data":{"type":"gameCenterActivityVersions","id":"TARGET_ID"}}}`,
			wantReceiptField:  "versionId",
			omitReceiptField:  "localizationId",
		},
		{
			name:              "challenge localization",
			group:             "challenges",
			resource:          "gameCenterChallengeImages",
			flag:              "--localization-id",
			wantRelationships: `{"localization":{"data":{"type":"gameCenterChallengeLocalizations","id":"TARGET_ID"}}}`,
			wantReceiptField:  "localizationId",
			omitReceiptField:  "versionId",
		},
		{
			name:              "challenge version",
			group:             "challenges",
			resource:          "gameCenterChallengeImages",
			flag:              "--version-id",
			wantRelationships: `{"version":{"data":{"type":"gameCenterChallengeVersions","id":"TARGET_ID"}}}`,
			wantReceiptField:  "versionId",
			omitReceiptField:  "localizationId",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupAuth(t)
			path, _ := creativeUploadFile(t)

			var gotRelationships string
			collectionPath := "/v1/" + test.resource
			installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
				switch {
				case req.Method == http.MethodPost && req.URL.Path == collectionPath:
					var body struct {
						Data struct {
							Relationships json.RawMessage `json:"relationships"`
						} `json:"data"`
					}
					if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
						t.Fatalf("decode reservation: %v", err)
					}
					gotRelationships = string(body.Data.Relationships)
					return jsonResponse(http.StatusCreated, `{"data":{"type":"`+test.resource+`","id":"img-1","attributes":{"fileName":"creative.png","fileSize":1,"uploadOperations":[{"method":"PUT","url":"https://storage.example/part","offset":0,"length":1}]}}}`)
				case req.Method == http.MethodPut && req.URL.Host == "storage.example":
					_, _ = io.Copy(io.Discard, req.Body)
					return jsonResponse(http.StatusOK, "")
				case req.Method == http.MethodPatch && req.URL.Path == collectionPath+"/img-1":
					return jsonResponse(http.StatusOK, `{"data":{"type":"`+test.resource+`","id":"img-1","attributes":{"fileName":"creative.png","fileSize":1,"uploaded":true}}}`)
				default:
					t.Fatalf("unexpected request: %s %s", req.Method, req.URL)
					return nil, nil
				}
			}))

			args := []string{"game-center", test.group, "images", "upload", test.flag, "TARGET_ID", "--file", path}
			stdout, _, err := runCommand(t, append(args, "--output", "json"))
			if err != nil {
				t.Fatalf("upload: %v", err)
			}
			if gotRelationships != test.wantRelationships {
				t.Fatalf("relationships = %s, want %s", gotRelationships, test.wantRelationships)
			}

			var receipt map[string]any
			if err := json.Unmarshal([]byte(stdout), &receipt); err != nil {
				t.Fatalf("decode receipt %q: %v", stdout, err)
			}
			if receipt[test.wantReceiptField] != "TARGET_ID" {
				t.Fatalf("receipt %s = %v, want TARGET_ID", test.wantReceiptField, receipt[test.wantReceiptField])
			}
			if _, present := receipt[test.omitReceiptField]; present {
				t.Fatalf("receipt must omit %s: %s", test.omitReceiptField, stdout)
			}

			table, _, err := runCommand(t, append(args, "--output", "table"))
			if err != nil {
				t.Fatalf("upload with table output: %v", err)
			}
			if !strings.Contains(table, "TARGET_ID") {
				t.Fatalf("table output lost the target ID: %q", table)
			}
		})
	}
}
