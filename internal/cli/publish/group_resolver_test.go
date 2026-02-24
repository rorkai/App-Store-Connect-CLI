package publish

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// --- helpers ---------------------------------------------------------------

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func setupTestAuth(t *testing.T) {
	t.Helper()
	tempDir := t.TempDir()
	keyPath := filepath.Join(tempDir, "AuthKey.p8")
	writeTestECDSAPEM(t, keyPath)
	// Force tests to use only the ephemeral env credentials set here.
	// This prevents host keychain/profile state from leaking into CI/local runs.
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_PROFILE", "")
	t.Setenv("ASC_KEY_ID", "TEST_KEY")
	t.Setenv("ASC_ISSUER_ID", "TEST_ISSUER")
	t.Setenv("ASC_PRIVATE_KEY_PATH", keyPath)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
}

func writeTestECDSAPEM(t *testing.T, path string) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate ECDSA key: %v", err)
	}
	der, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatalf("marshal ECDSA key: %v", err)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		t.Fatalf("create key file: %v", err)
	}
	defer func() { _ = f.Close() }()
	if err := pem.Encode(f, &pem.Block{Type: "EC PRIVATE KEY", Bytes: der}); err != nil {
		t.Fatalf("encode PEM: %v", err)
	}
}

func swapTransport(t *testing.T, rt http.RoundTripper) {
	t.Helper()
	orig := http.DefaultTransport
	http.DefaultTransport = rt
	t.Cleanup(func() { http.DefaultTransport = orig })
}

func jsonResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
	}
}

// --- resolvePublishBetaGroupIDsFromList (pure unit tests) ------------------

func TestResolvePublishBetaGroupIDsFromList_ResolvesByNameAndID(t *testing.T) {
	groups := &asc.BetaGroupsResponse{
		Data: []asc.Resource[asc.BetaGroupAttributes]{
			{ID: "GROUP_A", Attributes: asc.BetaGroupAttributes{Name: "External Testers"}},
			{ID: "GROUP_B", Attributes: asc.BetaGroupAttributes{Name: "Internal Team"}},
		},
	}

	got, err := resolvePublishBetaGroupIDsFromList(
		[]string{" external testers ", "GROUP_B", "GROUP_A", "EXTERNAL TESTERS"},
		groups,
	)
	if err != nil {
		t.Fatalf("resolvePublishBetaGroupIDsFromList() error = %v", err)
	}

	want := []string{"GROUP_A", "GROUP_B"}
	if len(got) != len(want) {
		t.Fatalf("expected %d groups, got %d (%v)", len(want), len(got), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("expected group %d to be %q, got %q", i, want[i], got[i])
		}
	}
}

func TestResolvePublishBetaGroupIDsFromList_MissingGroup(t *testing.T) {
	groups := &asc.BetaGroupsResponse{
		Data: []asc.Resource[asc.BetaGroupAttributes]{
			{ID: "GROUP_A", Attributes: asc.BetaGroupAttributes{Name: "External Testers"}},
		},
	}

	_, err := resolvePublishBetaGroupIDsFromList([]string{"does-not-exist"}, groups)
	if err == nil {
		t.Fatal("expected error for missing beta group")
	}
	if !strings.Contains(err.Error(), `beta group "does-not-exist" not found`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestResolvePublishBetaGroupIDsFromList_AmbiguousName(t *testing.T) {
	groups := &asc.BetaGroupsResponse{
		Data: []asc.Resource[asc.BetaGroupAttributes]{
			{ID: "GROUP_A", Attributes: asc.BetaGroupAttributes{Name: "QA"}},
			{ID: "GROUP_B", Attributes: asc.BetaGroupAttributes{Name: "QA"}},
		},
	}

	_, err := resolvePublishBetaGroupIDsFromList([]string{"qa"}, groups)
	if err == nil {
		t.Fatal("expected error for ambiguous beta group name")
	}
	if !strings.Contains(err.Error(), `multiple beta groups named "qa"; use group ID`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestResolvePublishBetaGroupIDsFromList_NoGroupsReturned(t *testing.T) {
	_, err := resolvePublishBetaGroupIDsFromList([]string{"group"}, nil)
	if err == nil {
		t.Fatal("expected error when no group list is available")
	}
	if !strings.Contains(err.Error(), "no beta groups returned for app") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestResolvePublishBetaGroupIDsFromList_SingleNameResolves(t *testing.T) {
	groups := &asc.BetaGroupsResponse{
		Data: []asc.Resource[asc.BetaGroupAttributes]{
			{ID: "G1", Attributes: asc.BetaGroupAttributes{Name: "Alpha"}},
			{ID: "G2", Attributes: asc.BetaGroupAttributes{Name: "Beta"}},
		},
	}

	got, err := resolvePublishBetaGroupIDsFromList([]string{"beta"}, groups)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0] != "G2" {
		t.Fatalf("expected [G2], got %v", got)
	}
}

func TestResolvePublishBetaGroupIDsFromList_EmptyDataList(t *testing.T) {
	groups := &asc.BetaGroupsResponse{
		Data: []asc.Resource[asc.BetaGroupAttributes]{},
	}

	_, err := resolvePublishBetaGroupIDsFromList([]string{"anything"}, groups)
	if err == nil {
		t.Fatal("expected error when data list is empty")
	}
	if !strings.Contains(err.Error(), `beta group "anything" not found`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestResolvePublishBetaGroupIDsFromList_AllWhitespaceInputs(t *testing.T) {
	groups := &asc.BetaGroupsResponse{
		Data: []asc.Resource[asc.BetaGroupAttributes]{
			{ID: "G1", Attributes: asc.BetaGroupAttributes{Name: "Alpha"}},
		},
	}

	_, err := resolvePublishBetaGroupIDsFromList([]string{"  ", "\t", ""}, groups)
	if err == nil {
		t.Fatal("expected error when all inputs are whitespace")
	}
	if !strings.Contains(err.Error(), "at least one beta group is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestResolvePublishBetaGroupIDsFromList_IDsTakePriorityOverNames(t *testing.T) {
	// A group whose name happens to be a valid ID of another group.
	groups := &asc.BetaGroupsResponse{
		Data: []asc.Resource[asc.BetaGroupAttributes]{
			{ID: "ABC123", Attributes: asc.BetaGroupAttributes{Name: "DEF456"}},
			{ID: "DEF456", Attributes: asc.BetaGroupAttributes{Name: "Other"}},
		},
	}

	// "DEF456" should resolve as an ID (exact match), not by name.
	got, err := resolvePublishBetaGroupIDsFromList([]string{"DEF456"}, groups)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0] != "DEF456" {
		t.Fatalf("expected [DEF456], got %v", got)
	}
}

func TestResolvePublishBetaGroupIDsFromList_SkipsGroupsWithEmptyIDOrName(t *testing.T) {
	groups := &asc.BetaGroupsResponse{
		Data: []asc.Resource[asc.BetaGroupAttributes]{
			{ID: "", Attributes: asc.BetaGroupAttributes{Name: "Ghost"}}, // empty ID, skipped
			{ID: "G1", Attributes: asc.BetaGroupAttributes{Name: ""}},    // empty name, but valid ID
			{ID: "G2", Attributes: asc.BetaGroupAttributes{Name: "Visible"}},
		},
	}

	// Resolve by ID: G1 should still be found even though its name is empty.
	got, err := resolvePublishBetaGroupIDsFromList([]string{"G1"}, groups)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0] != "G1" {
		t.Fatalf("expected [G1], got %v", got)
	}

	// Resolve by name "Ghost" should fail because the resource with that name
	// had an empty ID and was skipped.
	_, err = resolvePublishBetaGroupIDsFromList([]string{"Ghost"}, groups)
	if err == nil {
		t.Fatal("expected error for group with empty ID")
	}
}

func TestResolvePublishBetaGroupIDsFromList_DeduplicatesNameAndID(t *testing.T) {
	groups := &asc.BetaGroupsResponse{
		Data: []asc.Resource[asc.BetaGroupAttributes]{
			{ID: "G1", Attributes: asc.BetaGroupAttributes{Name: "Alpha"}},
		},
	}

	// Pass the same group by ID and by name: should resolve to just one entry.
	got, err := resolvePublishBetaGroupIDsFromList([]string{"G1", "alpha"}, groups)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0] != "G1" {
		t.Fatalf("expected [G1], got %v", got)
	}
}

func TestResolvePublishBetaGroupIDsFromList_DuplicateAPIEntries(t *testing.T) {
	// Simulate the same group appearing multiple times in the API response
	// (can happen with pagination when data changes between page fetches).
	groups := &asc.BetaGroupsResponse{
		Data: []asc.Resource[asc.BetaGroupAttributes]{
			{ID: "G1", Attributes: asc.BetaGroupAttributes{Name: "Alpha"}},
			{ID: "G1", Attributes: asc.BetaGroupAttributes{Name: "Alpha"}}, // duplicate
			{ID: "G2", Attributes: asc.BetaGroupAttributes{Name: "Beta"}},
		},
	}

	// Should resolve "alpha" to G1 without reporting a false ambiguous error.
	got, err := resolvePublishBetaGroupIDsFromList([]string{"alpha"}, groups)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0] != "G1" {
		t.Fatalf("expected [G1], got %v", got)
	}
}

// --- resolvePublishBetaGroupIDs (HTTP integration tests) -------------------

func TestResolvePublishBetaGroupIDs_SinglePage(t *testing.T) {
	setupTestAuth(t)

	swapTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Path != "/v1/apps/APP1/betaGroups" {
			t.Fatalf("unexpected path: %s", req.URL.Path)
		}
		if req.URL.Query().Get("limit") != "200" {
			t.Fatalf("expected limit=200, got %q", req.URL.Query().Get("limit"))
		}
		body := `{
			"data": [
				{"type":"betaGroups","id":"G1","attributes":{"name":"Alpha"}},
				{"type":"betaGroups","id":"G2","attributes":{"name":"Beta"}}
			]
		}`
		return jsonResponse(http.StatusOK, body), nil
	}))

	client, err := shared.GetASCClient()
	if err != nil {
		t.Fatalf("shared.GetASCClient: %v", err)
	}

	got, err := resolvePublishBetaGroupIDs(context.Background(), client, "APP1", []string{"Alpha", "G2"})
	if err != nil {
		t.Fatalf("resolvePublishBetaGroupIDs() error = %v", err)
	}

	want := []string{"G1", "G2"}
	if len(got) != len(want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("index %d: expected %q, got %q", i, want[i], got[i])
		}
	}
}

func TestResolvePublishBetaGroupIDs_APIError(t *testing.T) {
	setupTestAuth(t)

	swapTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		body := `{"errors":[{"status":"403","title":"Forbidden"}]}`
		return jsonResponse(http.StatusForbidden, body), nil
	}))

	client, err := shared.GetASCClient()
	if err != nil {
		t.Fatalf("shared.GetASCClient: %v", err)
	}

	_, err = resolvePublishBetaGroupIDs(context.Background(), client, "APP1", []string{"Alpha"})
	if err == nil {
		t.Fatal("expected error from API failure")
	}
	if !strings.Contains(err.Error(), "failed to list beta groups") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestResolvePublishBetaGroupIDs_NameNotFound(t *testing.T) {
	setupTestAuth(t)

	swapTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		body := `{
			"data": [
				{"type":"betaGroups","id":"G1","attributes":{"name":"Alpha"}}
			]
		}`
		return jsonResponse(http.StatusOK, body), nil
	}))

	client, err := shared.GetASCClient()
	if err != nil {
		t.Fatalf("shared.GetASCClient: %v", err)
	}

	_, err = resolvePublishBetaGroupIDs(context.Background(), client, "APP1", []string{"NonExistent"})
	if err == nil {
		t.Fatal("expected error for non-existent group name")
	}
	if !strings.Contains(err.Error(), `beta group "NonExistent" not found`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- listAllPublishBetaGroups (pagination tests) ---------------------------

func TestListAllPublishBetaGroups_SinglePage(t *testing.T) {
	setupTestAuth(t)

	swapTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		body := `{
			"data": [
				{"type":"betaGroups","id":"G1","attributes":{"name":"Alpha"}},
				{"type":"betaGroups","id":"G2","attributes":{"name":"Beta"}}
			]
		}`
		return jsonResponse(http.StatusOK, body), nil
	}))

	client, err := shared.GetASCClient()
	if err != nil {
		t.Fatalf("shared.GetASCClient: %v", err)
	}

	resp, err := listAllPublishBetaGroups(context.Background(), client, "APP1")
	if err != nil {
		t.Fatalf("listAllPublishBetaGroups() error = %v", err)
	}
	if len(resp.Data) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(resp.Data))
	}
}

func TestListAllPublishBetaGroups_Paginated(t *testing.T) {
	setupTestAuth(t)

	callCount := 0
	swapTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		callCount++
		switch callCount {
		case 1:
			// First page with a next link.
			if req.URL.Path != "/v1/apps/APP1/betaGroups" {
				t.Fatalf("page 1: unexpected path: %s", req.URL.Path)
			}
			body := fmt.Sprintf(`{
				"data": [
					{"type":"betaGroups","id":"G1","attributes":{"name":"Page1-A"}},
					{"type":"betaGroups","id":"G2","attributes":{"name":"Page1-B"}}
				],
				"links": {"next": "%s/v1/apps/APP1/betaGroups?cursor=page2"}
			}`, asc.BaseURL)
			return jsonResponse(http.StatusOK, body), nil
		case 2:
			// Second (final) page.
			if !strings.Contains(req.URL.String(), "cursor=page2") {
				t.Fatalf("page 2: expected cursor=page2 in URL, got %s", req.URL.String())
			}
			body := `{
				"data": [
					{"type":"betaGroups","id":"G3","attributes":{"name":"Page2-A"}}
				]
			}`
			return jsonResponse(http.StatusOK, body), nil
		default:
			t.Fatalf("unexpected request count %d", callCount)
			return nil, nil
		}
	}))

	client, err := shared.GetASCClient()
	if err != nil {
		t.Fatalf("shared.GetASCClient: %v", err)
	}

	resp, err := listAllPublishBetaGroups(context.Background(), client, "APP1")
	if err != nil {
		t.Fatalf("listAllPublishBetaGroups() error = %v", err)
	}
	if len(resp.Data) != 3 {
		t.Fatalf("expected 3 groups across 2 pages, got %d", len(resp.Data))
	}
	if callCount != 2 {
		t.Fatalf("expected 2 HTTP calls, got %d", callCount)
	}

	// Verify all groups are present in order.
	wantIDs := []string{"G1", "G2", "G3"}
	for i, want := range wantIDs {
		if resp.Data[i].ID != want {
			t.Fatalf("index %d: expected ID %q, got %q", i, want, resp.Data[i].ID)
		}
	}
}

func TestListAllPublishBetaGroups_PaginationAPIError(t *testing.T) {
	setupTestAuth(t)

	callCount := 0
	swapTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		callCount++
		switch callCount {
		case 1:
			body := fmt.Sprintf(`{
				"data": [{"type":"betaGroups","id":"G1","attributes":{"name":"A"}}],
				"links": {"next": "%s/v1/apps/APP1/betaGroups?cursor=page2"}
			}`, asc.BaseURL)
			return jsonResponse(http.StatusOK, body), nil
		case 2:
			body := `{"errors":[{"status":"500","title":"Internal Server Error"}]}`
			return jsonResponse(http.StatusInternalServerError, body), nil
		default:
			t.Fatalf("unexpected request count %d", callCount)
			return nil, nil
		}
	}))

	client, err := shared.GetASCClient()
	if err != nil {
		t.Fatalf("shared.GetASCClient: %v", err)
	}

	_, err = listAllPublishBetaGroups(context.Background(), client, "APP1")
	if err == nil {
		t.Fatal("expected error from second page failure")
	}
}

// --- end-to-end: resolve names through paginated API -----------------------

func TestResolvePublishBetaGroupIDs_PaginatedNameResolution(t *testing.T) {
	setupTestAuth(t)

	callCount := 0
	swapTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		callCount++
		switch callCount {
		case 1:
			body := fmt.Sprintf(`{
				"data": [
					{"type":"betaGroups","id":"G1","attributes":{"name":"Internal Team"}}
				],
				"links": {"next": "%s/v1/apps/APP1/betaGroups?cursor=page2"}
			}`, asc.BaseURL)
			return jsonResponse(http.StatusOK, body), nil
		case 2:
			body := `{
				"data": [
					{"type":"betaGroups","id":"G2","attributes":{"name":"External Testers"}}
				]
			}`
			return jsonResponse(http.StatusOK, body), nil
		default:
			t.Fatalf("unexpected request count %d", callCount)
			return nil, nil
		}
	}))

	client, err := shared.GetASCClient()
	if err != nil {
		t.Fatalf("shared.GetASCClient: %v", err)
	}

	// Resolve a name that only exists on page 2.
	got, err := resolvePublishBetaGroupIDs(context.Background(), client, "APP1", []string{"External Testers"})
	if err != nil {
		t.Fatalf("resolvePublishBetaGroupIDs() error = %v", err)
	}
	if len(got) != 1 || got[0] != "G2" {
		t.Fatalf("expected [G2], got %v", got)
	}
}
