package cmdtest

import (
	"encoding/json"
	"net/http"
	"reflect"
	"strings"
	"testing"
)

func TestScreenshotsListBySetIDPassesEnvelopeThrough(t *testing.T) {
	const body = `{"data":[{"type":"appScreenshots","id":"shot-1","attributes":{"fileName":"home.png","fileSize":42,"assetDeliveryState":{"state":"COMPLETE","warnings":[{"code":"IMAGE_TOO_DARK","description":"retained"}]},"uploadOperations":[{"method":"PUT","url":"https://upload.example/1","length":42,"offset":0}]},"relationships":{"appScreenshotSet":{"data":{"type":"appScreenshotSets","id":"set-1"}}}}],"links":{"self":"https://api.appstoreconnect.apple.com/v1/appScreenshotSets/set-1/appScreenshots"},"meta":{"paging":{"total":1,"limit":5}}}`
	client := screenshotsListTestClient(t, func(w http.ResponseWriter, req *http.Request) {
		q := req.URL.Query()
		if req.Method != http.MethodGet || req.URL.Path != "/v1/appScreenshotSets/set-1/appScreenshots" || q.Get("fields[appScreenshots]") != "fileName,assetDeliveryState,uploadOperations" || q.Get("limit") != "5" || len(q) != 2 {
			t.Errorf("unexpected request: %s %s", req.Method, req.URL)
		}
		writeScreenshotsListJSON(t, w, body)
	})

	stdout, stderr, err := runRootWithDeps(t, client, []string{"screenshots", "list", "--set-id", "set-1", "--fields", "fileName,assetDeliveryState,uploadOperations", "--limit", "5", "--output", "json"}...)
	if err != nil {
		t.Fatalf("screenshots list error: %v (stderr %q)", err, stderr)
	}
	var got, want any
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("decode stdout %q: %v", stdout, err)
	}
	if err := json.Unmarshal([]byte(body), &want); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("stdout = %s, want Apple envelope %s", stdout, body)
	}
}

func TestScreenshotsListBySetIDUsageErrors(t *testing.T) {
	tests := []struct {
		args    []string
		wantErr string
	}{
		{[]string{"--set-id", "set-1", "--version-localization", "loc-1"}, "Error: screenshots list: --set-id cannot be combined with --version-localization"},
		{[]string{"--set-id", "set-1", "--app", "123", "--version", "1.0"}, "Error: screenshots list: --set-id cannot be combined with --version-localization"},
		{[]string{"--set-id", "set-1", "--fields", "fileName,bogus"}, "Error: screenshots list: --fields must be one of: fileSize, fileName"},
		{[]string{"--set-id", "set-1", "--limit", "201"}, "Error: screenshots list: --limit must be between 1 and 200"},
		{[]string{"--version-localization", "loc-1", "--fields", "fileName"}, "Error: screenshots list: --fields, --limit, and --paginate require --set-id"},
		{[]string{"--version-localization", "loc-1", "--limit", "5"}, "Error: screenshots list: --fields, --limit, and --paginate require --set-id"},
		{[]string{"--version-localization", "loc-1", "--paginate"}, "Error: screenshots list: --fields, --limit, and --paginate require --set-id"},
		{[]string{"--version", "version-1", "--platform", "IOS"}, "Error: --platform with --version requires --app or ASC_APP_ID so ownership and platform can be verified"},
		{[]string{"--version-id", "version-1", "--platform", "IOS"}, "Error: --platform with --version-id requires --app or ASC_APP_ID so ownership and platform can be verified"},
	}
	for _, tt := range tests {
		args := tt.args

		t.Run(strings.Join(args, " "), func(t *testing.T) {
			client := screenshotsListTestClient(t, func(w http.ResponseWriter, req *http.Request) {
				t.Errorf("unexpected request: %s %s", req.Method, req.URL)
			})
			t.Setenv("ASC_APP_ID", "")
			stdout, stderr, err := runRootWithDeps(t, client, append([]string{"screenshots", "list"}, args...)...)
			if !isUsageClassError(err) || stdout != "" || !strings.HasPrefix(stderr, tt.wantErr) {
				t.Fatalf("expected usage error, got err=%v stdout=%q stderr=%q", err, stdout, stderr)
			}
		})
	}
}
