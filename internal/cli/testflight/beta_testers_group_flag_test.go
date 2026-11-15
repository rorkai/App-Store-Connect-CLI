package testflight

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"flag"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func TestBetaTestersAddCommand_CSVGroupsPassValidation(t *testing.T) {
	isolateTestFlightAuthEnvForAddTests(t)

	cmd := newBetaTestersAddCommand(passedValidationDeps())
	if err := cmd.FlagSet.Parse([]string{
		"--app", "123456789",
		"--email", "tester@example.com",
		"--group", "Beta,QA Team",
	}); err != nil {
		t.Fatalf("parse flags: %v", err)
	}

	err := cmd.Exec(context.Background(), []string{})
	if errors.Is(err, flag.ErrHelp) {
		t.Fatalf("comma-separated groups should pass validation, got %v", err)
	}
	if !errors.Is(err, errClientRequested) {
		t.Fatalf("comma-separated groups should request a client after validation, got %v", err)
	}
}

func TestResolveBetaGroupIDs_SingleFetchResolvesAllTokens(t *testing.T) {
	requests := 0
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		requests++
		if req.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", req.Method)
			http.Error(w, "unexpected method", http.StatusMethodNotAllowed)
			return
		}
		if req.URL.Path != "/v1/apps/123456789/betaGroups" {
			t.Errorf("unexpected request path %q", req.URL.Path)
			http.Error(w, "unexpected path", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{
				{"type": "betaGroups", "id": "group-beta", "attributes": map[string]any{"name": "Beta"}},
				{"type": "betaGroups", "id": "group-ios27", "attributes": map[string]any{"name": "iOS 27"}},
				{"type": "betaGroups", "id": "group-qa", "attributes": map[string]any{"name": "QA Team"}},
			},
			"links": map[string]string{},
		}); err != nil {
			t.Fatalf("marshal response: %v", err)
		}
	}))
	t.Cleanup(server.Close)
	client := newBetaGroupResolutionClient(t, server)

	got, err := resolveBetaGroupIDs(context.Background(), client, "123456789", "group-beta, ios 27,QA Team")
	if err != nil {
		t.Fatalf("resolveBetaGroupIDs() error: %v", err)
	}

	want := []string{"group-beta", "group-ios27", "group-qa"}
	if len(got) != len(want) {
		t.Fatalf("resolveBetaGroupIDs() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("resolveBetaGroupIDs()[%d] = %q, want %q (full: %v)", i, got[i], want[i], got)
		}
	}
	if requests != 1 {
		t.Fatalf("beta group fetches = %d, want 1", requests)
	}
}

func TestResolveBetaGroupIDs_UnknownTokenFails(t *testing.T) {
	client := newBetaGroupResolutionClient(t, newStaticBetaGroupsServer(t, map[string]string{
		"group-beta": "Beta",
	}))

	_, err := resolveBetaGroupIDs(context.Background(), client, "123456789", "Beta,Nope")
	if err == nil {
		t.Fatal("unknown group token should fail")
	}
	if !strings.Contains(err.Error(), "Nope") {
		t.Fatalf("error should name the unresolved group, got %q", err.Error())
	}
}

func TestResolveBetaGroupIDs_CommaNameResolvesAsSingleGroup(t *testing.T) {
	client := newBetaGroupResolutionClient(t, newStaticBetaGroupsServer(t, map[string]string{
		"group-qa-external": "QA, External",
		"group-qa":          "QA",
		"group-external":    "External",
	}))

	got, err := resolveBetaGroupIDs(context.Background(), client, "123456789", "QA, External")
	if err != nil {
		t.Fatalf("resolveBetaGroupIDs() error: %v", err)
	}
	if len(got) != 1 || got[0] != "group-qa-external" {
		t.Fatalf("comma-containing name should resolve as one group, got %v", got)
	}
}

func TestResolveBetaGroupID_RejectsMultiGroupValue(t *testing.T) {
	client := newBetaGroupResolutionClient(t, newStaticBetaGroupsServer(t, map[string]string{
		"group-beta": "Beta",
		"group-qa":   "QA",
	}))

	_, err := resolveBetaGroupID(context.Background(), client, "123456789", "Beta,QA")
	if err == nil {
		t.Fatal("singular resolver should reject a value resolving to multiple groups")
	}
	if !strings.Contains(err.Error(), "single beta group") {
		t.Fatalf("error should explain the single-group expectation, got %q", err.Error())
	}
}

func TestResolveBetaGroupIDs_EmptyTokenFails(t *testing.T) {
	client := newBetaGroupResolutionClient(t, newStaticBetaGroupsServer(t, map[string]string{
		"group-beta": "Beta",
	}))

	_, err := resolveBetaGroupIDs(context.Background(), client, "123456789", "Beta,,")
	if err == nil {
		t.Fatal("empty group token should fail")
	}
	if !strings.Contains(err.Error(), "empty group name") {
		t.Fatalf("error should call out the empty token, got %q", err.Error())
	}
}

func TestResolveBetaGroupIDs_FollowsPagination(t *testing.T) {
	requests := 0
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		requests++
		links := map[string]string{}
		var data []map[string]any
		if req.URL.Query().Get("cursor") == "" {
			data = []map[string]any{
				{"type": "betaGroups", "id": "group-beta", "attributes": map[string]any{"name": "Beta"}},
			}
			links["next"] = "https://api.appstoreconnect.apple.com/v1/apps/123456789/betaGroups?cursor=2"
		} else {
			data = []map[string]any{
				{"type": "betaGroups", "id": "group-second", "attributes": map[string]any{"name": "Second"}},
			}
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]any{"data": data, "links": links}); err != nil {
			t.Errorf("marshal response: %v", err)
		}
	}))
	t.Cleanup(server.Close)
	client := newBetaGroupResolutionClient(t, server)

	got, err := resolveBetaGroupIDs(context.Background(), client, "123456789", "Beta,Second")
	if err != nil {
		t.Fatalf("resolveBetaGroupIDs() error: %v", err)
	}
	if len(got) != 2 || got[0] != "group-beta" || got[1] != "group-second" {
		t.Fatalf("paginated resolution = %v, want [group-beta group-second]", got)
	}
	if requests != 2 {
		t.Fatalf("beta group fetches = %d, want 2", requests)
	}
}

func newStaticBetaGroupsServer(t *testing.T, groups map[string]string) *httptest.Server {
	t.Helper()

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		data := make([]map[string]any, 0, len(groups))
		for id, name := range groups {
			data = append(data, map[string]any{
				"type": "betaGroups", "id": id, "attributes": map[string]any{"name": name},
			})
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]any{
			"data":  data,
			"links": map[string]string{},
		}); err != nil {
			t.Errorf("marshal response: %v", err)
		}
	}))
	t.Cleanup(server.Close)
	return server
}

func newBetaGroupResolutionClient(t *testing.T, server *httptest.Server) *asc.Client {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey() error: %v", err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatalf("MarshalPKCS8PrivateKey() error: %v", err)
	}
	keyPath := filepath.Join(t.TempDir(), "AuthKey.p8")
	if err := os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}), 0o600); err != nil {
		t.Fatalf("write private key: %v", err)
	}

	transport, ok := server.Client().Transport.(*http.Transport)
	if !ok {
		t.Fatalf("test server transport type = %T, want *http.Transport", server.Client().Transport)
	}
	transport = transport.Clone()
	transport.TLSClientConfig = transport.TLSClientConfig.Clone()
	transport.TLSClientConfig.ServerName = "example.com"
	transport.DialContext = func(ctx context.Context, network, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, network, server.Listener.Addr().String())
	}

	client, err := asc.NewClientWithHTTPClient("KEY123", "ISS456", keyPath, &http.Client{Transport: transport})
	if err != nil {
		t.Fatalf("NewClientWithHTTPClient() error: %v", err)
	}
	return client
}

func TestBetaGroupNameResolvesAcrossUnicodeSpaces(t *testing.T) {
	nbspInternal := asc.Resource[asc.BetaGroupAttributes]{ID: "group-internal", Attributes: asc.BetaGroupAttributes{Name: "App Store\u00a0Connect Users", IsInternalGroup: true}}
	asciiExternal := asc.Resource[asc.BetaGroupAttributes]{ID: "group-external", Attributes: asc.BetaGroupAttributes{Name: "App Store Connect Users"}}

	cases := []struct {
		name   string
		groups []asc.Resource[asc.BetaGroupAttributes]
		input  string
		want   string
	}{
		{"ASCII input matches NBSP name", []asc.Resource[asc.BetaGroupAttributes]{nbspInternal}, "app store connect users", "group-internal"},
		{"exact name wins over NBSP name", []asc.Resource[asc.BetaGroupAttributes]{nbspInternal, asciiExternal}, "App Store Connect Users", "group-external"},
	}
	resolvers := []struct {
		name    string
		resolve func(*asc.BetaGroupsResponse, string) (string, error)
	}{
		{"testflight", matchBetaGroupID},
		{"config export", func(groups *asc.BetaGroupsResponse, input string) (string, error) {
			matched, err := filterBetaGroups(groups.Data, input)
			if err != nil {
				return "", err
			}
			if len(matched) != 1 {
				return "", fmt.Errorf("matched %d groups, want 1", len(matched))
			}
			return matched[0].ID, nil
		}},
		{"shared", func(groups *asc.BetaGroupsResponse, input string) (string, error) {
			resolved, err := shared.ResolveBetaGroupsFromList([]string{input}, groups, shared.ResolveBetaGroupsOptions{})
			if err != nil {
				return "", err
			}
			if len(resolved) != 1 {
				return "", fmt.Errorf("resolved %d groups, want 1", len(resolved))
			}
			return resolved[0].ID, nil
		}},
	}
	for _, resolver := range resolvers {
		for _, tc := range cases {
			t.Run(resolver.name+"/"+tc.name, func(t *testing.T) {
				got, err := resolver.resolve(&asc.BetaGroupsResponse{Data: tc.groups}, tc.input)
				if err != nil {
					t.Fatalf("resolve error: %v", err)
				}
				if got != tc.want {
					t.Fatalf("resolved %q, want %q", got, tc.want)
				}
			})
		}
	}
}
