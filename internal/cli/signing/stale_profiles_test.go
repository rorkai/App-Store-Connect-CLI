package signing

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

func TestProfileExpirationPassed(t *testing.T) {
	now := time.Date(2026, 9, 17, 0, 0, 0, 0, time.UTC)
	if !profileExpirationPassed("2000-01-01T00:00:00Z", now) {
		t.Fatal("expected a past RFC3339 date to be stale")
	}
	if profileExpirationPassed("2100-01-01T00:00:00Z", now) {
		t.Fatal("expected a future date to stay current")
	}
}

func TestProfileIsStaleSkipsUnrelatedType(t *testing.T) {
	profile := asc.Resource[asc.ProfileAttributes]{
		Attributes: asc.ProfileAttributes{
			ProfileType:    "IOS_APP_DEVELOPMENT",
			ProfileState:   asc.ProfileStateInvalid,
			ExpirationDate: "2000-01-01T00:00:00Z",
		},
	}
	if profileIsStale(profile, "IOS_APP_STORE", time.Now()) {
		t.Fatal("stale check must not select a different profile type")
	}
}

func TestDeleteStaleSigningProfilesDeletesOnlyExpiredActiveProfiles(t *testing.T) {
	var deleted []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/bundleIds/bundle-1/profiles":
			_, _ = io.WriteString(w, `{"data":[
				{"type":"profiles","id":"stale-1","attributes":{"name":"Old","profileType":"IOS_APP_STORE","profileState":"ACTIVE","expirationDate":"2000-01-01T00:00:00Z"}},
				{"type":"profiles","id":"fresh-1","attributes":{"name":"New","profileType":"IOS_APP_STORE","profileState":"ACTIVE","expirationDate":"2100-01-01T00:00:00Z"}},
				{"type":"profiles","id":"other-1","attributes":{"name":"Dev","profileType":"IOS_APP_DEVELOPMENT","profileState":"ACTIVE","expirationDate":"2000-01-01T00:00:00Z"}}
			],"links":{}}`)
		case req.Method == http.MethodDelete && strings.HasPrefix(req.URL.Path, "/v1/profiles/"):
			deleted = append(deleted, strings.TrimPrefix(req.URL.Path, "/v1/profiles/"))
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Errorf("unexpected request %s %s", req.Method, req.URL.Path)
			http.Error(w, "unexpected", http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)
	client := newSigningFetchServerTestClient(t, server)

	got, err := deleteStaleSigningProfiles(context.Background(), client, "bundle-1", "IOS_APP_STORE", false)
	if err != nil {
		t.Fatalf("deleteStaleSigningProfiles: %v", err)
	}
	if len(got) != 1 || got[0].ID != "stale-1" {
		t.Fatalf("receipt = %+v, want stale-1", got)
	}
	if strings.Join(deleted, ",") != "stale-1" {
		t.Fatalf("deleted = %v, want only stale-1", deleted)
	}
}

func TestSigningFetchDeleteStaleWithoutConfirmDoesNotContactApple(t *testing.T) {
	cmd := SigningFetchCommand()
	if err := cmd.FlagSet.Parse([]string{
		"--bundle-id", "com.example.app",
		"--profile-type", "IOS_APP_STORE",
		"--delete-stale-profiles",
	}); err != nil {
		t.Fatalf("parse: %v", err)
	}
	err := cmd.Exec(context.Background(), nil)
	if err == nil || !strings.Contains(err.Error(), "--confirm") {
		t.Fatalf("error = %v, want a --confirm usage error", err)
	}
}
