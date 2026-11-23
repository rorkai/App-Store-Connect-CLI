package cmdtest

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
)

// betaAppReviewSubmissionsURLCeiling keeps --app requests below the size at
// which App Store Connect stops answering GET /v1/betaAppReviewSubmissions:
// live, it answered an 8,271-byte URL and returned HTTP 500 for 8,466 bytes
// (docs/API_NOTES.md, "TestFlight beta review submissions").
const betaAppReviewSubmissionsURLCeiling = 8192

// TestTestFlightReviewSubmissionsListAppFullPageOfBuildsFitsOneRequest covers
// --app on an app with a full page of builds: all of their IDs go in one
// filter[build] value, every request (including the next page) stays below the
// size Apple accepts, and --paginate merges the pages.
func TestTestFlightReviewSubmissionsListAppFullPageOfBuildsFitsOneRequest(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	buildIDs := make([]string, 200)
	for i := range buildIDs {
		buildIDs[i] = fmt.Sprintf("%08x-0000-4000-8000-%012x", i, i)
	}

	var filters []string
	rt := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if n := len(req.URL.String()); n > betaAppReviewSubmissionsURLCeiling {
			t.Errorf("%s %s URL is %d bytes, over %d", req.Method, req.URL.Path, n, betaAppReviewSubmissionsURLCeiling)
		}
		switch req.URL.Path {
		case "/v1/builds":
			data := make([]map[string]string, 0, len(buildIDs))
			for _, id := range buildIDs {
				data = append(data, map[string]string{"type": "builds", "id": id})
			}
			return jsonBodyResponse(t, map[string]any{"data": data})
		case "/v1/betaAppReviewSubmissions":
			query := req.URL.Query()
			if query.Get("cursor") != "" {
				return jsonBodyResponse(t, map[string]any{"data": []map[string]string{{"type": "betaAppReviewSubmissions", "id": "submission-2"}}})
			}
			filters = append(filters, query.Get("filter[build]"))
			// Apple repeats the whole filter in the next link.
			next := "https://api.appstoreconnect.apple.com" + req.URL.RequestURI() + "&cursor=Mg"
			return jsonBodyResponse(t, map[string]any{
				"data":  []map[string]string{{"type": "betaAppReviewSubmissions", "id": "submission-1"}},
				"links": map[string]string{"next": next},
			})
		}
		t.Errorf("unexpected request %s %s", req.Method, req.URL)
		return jsonHTTPResponse(http.StatusNotFound, `{"errors":[{"status":"404"}]}`), nil
	})

	stdout, _, err := runRootWithDeps(t, stubClient(t, rt), "testflight", "review", "submissions", "list", "--app", "100000001", "--paginate", "--output", "json")
	if err != nil {
		t.Fatalf("run error: %v", err)
	}
	if len(filters) != 1 || filters[0] != strings.Join(buildIDs, ",") {
		t.Fatalf("filter[build] requests = %d, want one naming all %d builds", len(filters), len(buildIDs))
	}
	var out struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &out); err != nil {
		t.Fatalf("decode stdout: %v\n%s", err, stdout)
	}
	if len(out.Data) != 2 || out.Data[0].ID != "submission-1" || out.Data[1].ID != "submission-2" {
		t.Fatalf("merged submissions = %+v, want submission-1 then submission-2", out.Data)
	}
}

func jsonBodyResponse(t *testing.T, body any) (*http.Response, error) {
	t.Helper()
	data, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal response: %v", err)
	}
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(string(data))),
	}, nil
}
