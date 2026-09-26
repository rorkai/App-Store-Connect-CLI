package signing

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRemoveWrittenSigningFilesRemovesOnlyUnchangedFiles(t *testing.T) {
	dir := t.TempDir()
	profilePath := filepath.Join(dir, "App.mobileprovision")
	certPath := filepath.Join(dir, "cert.cer")
	if err := os.WriteFile(profilePath, []byte("profile"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(certPath, []byte("replaced by someone else"), 0o600); err != nil {
		t.Fatal(err)
	}

	err := removeWrittenSigningFiles(dir, []writtenSigningFile{
		{path: profilePath, data: []byte("profile")},
		{path: certPath, data: []byte("cert")},
	})
	if err == nil || !strings.Contains(err.Error(), "changed after it was written") {
		t.Fatalf("err = %v, want a changed-file error", err)
	}
	if _, err := os.Stat(profilePath); !os.IsNotExist(err) {
		t.Fatalf("unchanged partial file still present: %v", err)
	}
	if data, err := os.ReadFile(certPath); err != nil || string(data) != "replaced by someone else" {
		t.Fatalf("changed file was touched: %q, %v", data, err)
	}
}

func TestBundleIdentifierMatches(t *testing.T) {
	tests := []struct {
		parent    string
		candidate string
		expand    bool
		want      bool
	}{
		{parent: "com.app", candidate: "com.app", expand: false, want: true},
		{parent: "com.app", candidate: "com.app.widget", expand: false, want: false},
		{parent: "com.app", candidate: "com.app.widget", expand: true, want: true},
		{parent: "com.app", candidate: "com.app.clip", expand: true, want: true},
		{parent: "com.app", candidate: "com.apple.other", expand: true, want: false},
		{parent: "com.app", candidate: "com.app.*", expand: true, want: false},
		{parent: "com.example.*", candidate: "com.example.app", expand: true, want: false},
	}
	for _, test := range tests {
		if got := bundleIdentifierMatches(test.parent, test.candidate, test.expand); got != test.want {
			t.Fatalf("parent %q candidate %q expand %v = %v, want %v", test.parent, test.candidate, test.expand, got, test.want)
		}
	}
}

func TestListSigningBundleIDsReadsEveryFilteredPage(t *testing.T) {
	var requests []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		requests = append(requests, req.URL.RawQuery)
		if req.URL.Query().Get("cursor") == "2" {
			_, _ = io.WriteString(w, `{"data":[
				{"type":"bundleIds","id":"id-app","attributes":{"identifier":"com.app","platform":"IOS"}},
				{"type":"bundleIds","id":"id-clip","attributes":{"identifier":"com.app.clip","platform":"IOS"}}
			],"links":{}}`)
			return
		}
		if got := req.URL.Query().Get("filter[identifier]"); got != "com.app" {
			t.Errorf("filter[identifier] = %q, want com.app", got)
		}
		if got := req.URL.Query().Get("limit"); got != "200" {
			t.Errorf("limit = %q, want 200", got)
		}
		_, _ = io.WriteString(w, `{"data":[
			{"type":"bundleIds","id":"id-widget","attributes":{"identifier":"com.app.widget","platform":"IOS"}},
			{"type":"bundleIds","id":"id-wild","attributes":{"identifier":"com.app.*","platform":"IOS"}},
			{"type":"bundleIds","id":"id-other","attributes":{"identifier":"com.apple.other","platform":"IOS"}}
		],"links":{"next":"https://api.appstoreconnect.apple.com/v1/bundleIds?cursor=2"}}`)
	}))
	t.Cleanup(server.Close)
	client := newSigningFetchServerTestClient(t, server)

	matched, err := listSigningBundleIDs(context.Background(), client, "com.app", true)
	if err != nil {
		t.Fatalf("listSigningBundleIDs: %v", err)
	}
	got := make([]string, 0, len(matched))
	for _, item := range matched {
		got = append(got, item.Attributes.Identifier)
	}
	if strings.Join(got, ",") != "com.app,com.app.widget,com.app.clip" {
		t.Fatalf("matched = %v, want the exact ID first, then extensions from every page", got)
	}
	if len(requests) != 2 {
		t.Fatalf("requests = %v, want 2 pages", requests)
	}
}

func TestListSigningBundleIDsRejectsRepeatedNextURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"data":[{"type":"bundleIds","id":"id-app","attributes":{"identifier":"com.app"}}],"links":{"next":"https://api.appstoreconnect.apple.com/v1/bundleIds?cursor=2"}}`)
	}))
	t.Cleanup(server.Close)
	client := newSigningFetchServerTestClient(t, server)

	if _, err := listSigningBundleIDs(context.Background(), client, "com.app", true); err == nil || !strings.Contains(err.Error(), "repeated") {
		t.Fatalf("err = %v, want a repeated pagination error", err)
	}
}

func TestMatchedSyncTargetBundlesRejectsMoreThanBatchLimit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		items := []string{`{"type":"bundleIds","id":"id-app","attributes":{"identifier":"com.app"}}`}
		for i := 0; i < maxSigningSyncTargets; i++ {
			items = append(items, fmt.Sprintf(`{"type":"bundleIds","id":"id-%d","attributes":{"identifier":"com.app.ext%d"}}`, i, i))
		}
		_, _ = io.WriteString(w, `{"data":[`+strings.Join(items, ",")+`],"links":{}}`)
	}))
	t.Cleanup(server.Close)
	client := newSigningFetchServerTestClient(t, server)

	_, err := matchedSyncTargetBundles(context.Background(), client, "com.app")
	if err == nil || !strings.Contains(err.Error(), "matched 33 bundle IDs") {
		t.Fatalf("err = %v, want a target limit error", err)
	}
}

func TestFindBundleIDSelectsExactIdentifierFromSubstringMatches(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if req.URL.Query().Get("cursor") == "2" {
			_, _ = io.WriteString(w, `{"data":[{"type":"bundleIds","id":"id-app","attributes":{"identifier":"com.app"}}],"links":{}}`)
			return
		}
		_, _ = io.WriteString(w, `{"data":[
			{"type":"bundleIds","id":"id-widget","attributes":{"identifier":"com.app.widget"}},
			{"type":"bundleIds","id":"id-apple","attributes":{"identifier":"com.apple"}}
		],"links":{"next":"https://api.appstoreconnect.apple.com/v1/bundleIds?cursor=2"}}`)
	}))
	t.Cleanup(server.Close)
	client := newSigningFetchServerTestClient(t, server)

	got, err := findBundleID(context.Background(), client, "com.app")
	if err != nil {
		t.Fatalf("findBundleID: %v", err)
	}
	if got.Data.ID != "id-app" {
		t.Fatalf("bundle = %s (%s), want the exact com.app match", got.Data.ID, got.Data.Attributes.Identifier)
	}
}

func TestFindBundleIDRejectsOnlySubstringMatches(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"data":[{"type":"bundleIds","id":"id-widget","attributes":{"identifier":"com.app.widget"}}],"links":{}}`)
	}))
	t.Cleanup(server.Close)
	client := newSigningFetchServerTestClient(t, server)

	if _, err := findBundleID(context.Background(), client, "com.app"); err == nil || !strings.Contains(err.Error(), "bundle ID not found: com.app") {
		t.Fatalf("err = %v, want not found", err)
	}
}
