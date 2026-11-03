package cmdtest

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

const (
	previewUploadCreateAppPreview = `POST /v1/appPreviews {"data":{"type":"appPreviews","attributes":{"fileSize":13,"fileName":"preview.mov","mimeType":"video/quicktime"},"relationships":{"appPreviewSet":{"data":{"type":"appPreviewSets","id":"set-1"}}}}}` + "\n"
	previewUploadPutBytes         = `PUT upload.example/preview-1 `
	previewUploadCommit           = `PATCH /v1/appPreviews/preview-1 {"data":{"type":"appPreviews","id":"preview-1","attributes":{"sourceFileChecksum":"eaf168573c1b42c40001436e7cd2f1d5","uploaded":true}}}` + "\n"
	previewUploadPoll             = `GET /v1/appPreviews/preview-1 `
)

// TestProductPagesPreviewSetsUploadRequests pins the exact request sequence and
// bodies of localization preview uploads. It catches a treatment preview set
// created under the wrong relationship, a duplicate set when one already
// exists, and any change to what custom product page uploads send.
func TestProductPagesPreviewSetsUploadRequests(t *testing.T) {
	const noSets = `{"data":[]}`
	const iphone65Set = `{"data":[{"type":"appPreviewSets","id":"set-1","attributes":{"previewType":"IPHONE_65"}}]}`

	tests := []struct {
		name         string
		command      []string
		extraArgs    []string
		setsPath     string
		existingSets string
		wantFirst    []string
		wantIDField  string
	}{
		{
			name:         "custom page upload creates the missing set",
			command:      []string{"product-pages", "custom-pages", "localizations", "preview-sets", "upload"},
			setsPath:     "/v1/appCustomProductPageLocalizations/loc-1/appPreviewSets",
			existingSets: noSets,
			wantFirst: []string{
				`POST /v1/appPreviewSets {"data":{"type":"appPreviewSets","attributes":{"previewType":"IPHONE_65"},"relationships":{"appCustomProductPageLocalization":{"data":{"type":"appCustomProductPageLocalizations","id":"loc-1"}}}}}` + "\n",
			},
			wantIDField: "customProductPageLocalizationId",
		},
		{
			name:         "custom page sync replaces the existing set contents",
			command:      []string{"product-pages", "custom-pages", "localizations", "preview-sets", "sync"},
			extraArgs:    []string{"--confirm"},
			setsPath:     "/v1/appCustomProductPageLocalizations/loc-1/appPreviewSets",
			existingSets: iphone65Set,
			wantFirst: []string{
				`GET /v1/appPreviewSets/set-1/appPreviews `,
				`DELETE /v1/appPreviews/old-1 `,
			},
			wantIDField: "customProductPageLocalizationId",
		},
		{
			name:         "treatment upload creates the missing set",
			command:      []string{"product-pages", "experiments", "treatments", "localizations", "preview-sets", "upload"},
			setsPath:     "/v1/appStoreVersionExperimentTreatmentLocalizations/loc-1/appPreviewSets",
			existingSets: noSets,
			wantFirst: []string{
				`POST /v1/appPreviewSets {"data":{"type":"appPreviewSets","attributes":{"previewType":"IPHONE_65"},"relationships":{"appStoreVersionExperimentTreatmentLocalization":{"data":{"type":"appStoreVersionExperimentTreatmentLocalizations","id":"loc-1"}}}}}` + "\n",
			},
			wantIDField: "experimentTreatmentLocalizationId",
		},
		{
			name:         "treatment upload reuses the existing set",
			command:      []string{"product-pages", "experiments", "treatments", "localizations", "preview-sets", "upload"},
			setsPath:     "/v1/appStoreVersionExperimentTreatmentLocalizations/loc-1/appPreviewSets",
			existingSets: iphone65Set,
			wantIDField:  "experimentTreatmentLocalizationId",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			setupAuth(t)
			filePath := writePreviewUploadFile(t)

			var got []string
			installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
				got = append(got, previewUploadRequestLine(t, req))
				switch {
				case req.Method == http.MethodGet && req.URL.Path == tc.setsPath:
					return jsonHTTPResponse(http.StatusOK, tc.existingSets), nil
				case req.Method == http.MethodPost && req.URL.Path == "/v1/appPreviewSets":
					return jsonHTTPResponse(http.StatusCreated, `{"data":{"type":"appPreviewSets","id":"set-1","attributes":{"previewType":"IPHONE_65"}}}`), nil
				case req.Method == http.MethodGet && req.URL.Path == "/v1/appPreviewSets/set-1/appPreviews":
					return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"appPreviews","id":"old-1"}]}`), nil
				case req.Method == http.MethodDelete && req.URL.Path == "/v1/appPreviews/old-1":
					return jsonHTTPResponse(http.StatusNoContent, ""), nil
				case req.Method == http.MethodPost && req.URL.Path == "/v1/appPreviews":
					return jsonHTTPResponse(http.StatusCreated, `{"data":{"type":"appPreviews","id":"preview-1","attributes":{"uploadOperations":[{"method":"PUT","url":"https://upload.example/preview-1","length":13,"offset":0}]}}}`), nil
				case req.Method == http.MethodPut && req.URL.Host == "upload.example":
					return jsonHTTPResponse(http.StatusOK, `{}`), nil
				case req.Method == http.MethodPatch && req.URL.Path == "/v1/appPreviews/preview-1":
					return jsonHTTPResponse(http.StatusOK, `{"data":{"type":"appPreviews","id":"preview-1","attributes":{"uploaded":true}}}`), nil
				case req.Method == http.MethodGet && req.URL.Path == "/v1/appPreviews/preview-1":
					return jsonHTTPResponse(http.StatusOK, `{"data":{"type":"appPreviews","id":"preview-1","attributes":{"assetDeliveryState":{"state":"COMPLETE"}}}}`), nil
				default:
					t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
					return nil, nil
				}
			}))

			args := append([]string{}, tc.command...)
			args = append(args, "--localization-id", "loc-1", "--path", filePath, "--device-type", "IPHONE_65", "--output", "json")
			args = append(args, tc.extraArgs...)

			root := RootCommand("1.2.3")
			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				if err := root.Run(context.Background()); err != nil {
					t.Fatalf("Run() error: %v", err)
				}
			})
			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}

			want := []string{"GET " + tc.setsPath + " "}
			want = append(want, tc.wantFirst...)
			want = append(want, previewUploadCreateAppPreview, previewUploadPutBytes, previewUploadCommit, previewUploadPoll)
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("requests:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
			}

			var parsed map[string]any
			if err := json.Unmarshal([]byte(stdout), &parsed); err != nil {
				t.Fatalf("failed to parse stdout JSON: %v (stdout=%q)", err, stdout)
			}
			if parsed[tc.wantIDField] != "loc-1" || parsed["setId"] != "set-1" || parsed["previewType"] != "IPHONE_65" {
				t.Fatalf("unexpected receipt: %s", stdout)
			}
			if results, ok := parsed["results"].([]any); !ok || len(results) != 1 {
				t.Fatalf("expected one upload result, got %#v", parsed["results"])
			}
		})
	}
}

// TestProductPagesExperimentTreatmentLocalizationPreviewSetsUploadFailureRollsBackSet
// catches an empty set left behind on the treatment after a failed upload, and
// a failed upload that exits without printing which file failed.
func TestProductPagesExperimentTreatmentLocalizationPreviewSetsUploadFailureRollsBackSet(t *testing.T) {
	setupAuth(t)
	filePath := writePreviewUploadFile(t)

	deletedSet := false
	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersionExperimentTreatmentLocalizations/tloc-1/appPreviewSets":
			return jsonHTTPResponse(http.StatusOK, `{"data":[]}`), nil
		case req.Method == http.MethodPost && req.URL.Path == "/v1/appPreviewSets":
			return jsonHTTPResponse(http.StatusCreated, `{"data":{"type":"appPreviewSets","id":"set-1","attributes":{"previewType":"IPHONE_65"}}}`), nil
		case req.Method == http.MethodPost && req.URL.Path == "/v1/appPreviews":
			return jsonHTTPResponse(http.StatusForbidden, `{"errors":[{"status":"403","code":"FORBIDDEN","title":"Forbidden","detail":"not allowed"}]}`), nil
		case req.Method == http.MethodDelete && req.URL.Path == "/v1/appPreviewSets/set-1":
			deletedSet = true
			return jsonHTTPResponse(http.StatusNoContent, ""), nil
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	}))

	root := RootCommand("1.2.3")
	var runErr error
	stdout, _ := captureOutput(t, func() {
		if err := root.Parse([]string{
			"product-pages", "experiments", "treatments", "localizations", "preview-sets", "upload",
			"--localization-id", "tloc-1", "--path", filePath, "--device-type", "IPHONE_65", "--output", "json",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if runErr == nil || !strings.HasPrefix(runErr.Error(), "experiments treatments localizations preview-sets upload: ") {
		t.Fatalf("Run() error = %v, want the command-prefixed upload failure", runErr)
	}
	if !deletedSet {
		t.Fatal("expected the set created by this run to be deleted")
	}
	var parsed struct {
		SetID    string `json:"setId"`
		Failures []struct {
			FileName string `json:"fileName"`
		} `json:"failures"`
	}
	if err := json.Unmarshal([]byte(stdout), &parsed); err != nil {
		t.Fatalf("failed to parse stdout JSON: %v (stdout=%q)", err, stdout)
	}
	if parsed.SetID != "set-1" || len(parsed.Failures) != 1 || parsed.Failures[0].FileName != "preview.mov" {
		t.Fatalf("unexpected failure receipt: %s", stdout)
	}
}

func writePreviewUploadFile(t *testing.T) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "preview.mov")
	if err := os.WriteFile(path, []byte("preview-bytes"), 0o600); err != nil {
		t.Fatalf("write preview: %v", err)
	}
	return path
}

func previewUploadRequestLine(t *testing.T, req *http.Request) string {
	t.Helper()

	if req.URL.Host == "upload.example" {
		return fmt.Sprintf("%s %s%s ", req.Method, req.URL.Host, req.URL.Path)
	}
	body := ""
	if req.Body != nil {
		data, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("read request body: %v", err)
		}
		body = string(data)
	}
	return fmt.Sprintf("%s %s %s", req.Method, req.URL.RequestURI(), body)
}
