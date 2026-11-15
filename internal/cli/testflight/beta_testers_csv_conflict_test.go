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

func TestBetaTesterGroupConflictAlreadySatisfied(t *testing.T) {
	tests := []struct {
		name         string
		requestErr   error
		requested    []string
		memberships  [][]string
		want         bool
		wantRequests int
	}{
		{
			name:         "all requested relationships exist",
			requestErr:   &asc.APIError{Code: "ENTITY_ERROR.RELATIONSHIP.INVALID", StatusCode: http.StatusConflict},
			requested:    []string{"group-1", "group-2"},
			memberships:  [][]string{{"group-1"}, {"group-2"}},
			want:         true,
			wantRequests: 2,
		},
		{
			name:         "request entity conflict left a relationship missing",
			requestErr:   &asc.APIError{Code: "ENTITY_ERROR.ATTRIBUTE.INVALID", StatusCode: http.StatusConflict},
			requested:    []string{"group-1", "group-2"},
			memberships:  [][]string{{"group-1"}},
			want:         false,
			wantRequests: 1,
		},
		{
			name:         "non conflict does not query memberships",
			requestErr:   errors.New("network failure"),
			requested:    []string{"group-1"},
			memberships:  [][]string{{"group-1"}},
			want:         false,
			wantRequests: 0,
		},
		{
			name:         "conflict code without HTTP 409 does not query memberships",
			requestErr:   &asc.APIError{Code: "CONFLICT", StatusCode: http.StatusUnprocessableEntity},
			requested:    []string{"group-1"},
			memberships:  [][]string{{"group-1"}},
			want:         false,
			wantRequests: 0,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			requests := 0
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				pageIndex := requests
				requests++
				if req.Method != http.MethodGet {
					t.Fatalf("expected GET, got %s", req.Method)
				}
				if req.URL.Path != "/v1/betaTesters/tester-1/relationships/betaGroups" {
					t.Fatalf("unexpected request path %q", req.URL.Path)
				}
				if !strings.HasPrefix(req.Header.Get("Authorization"), "Bearer ") {
					t.Fatalf("expected bearer authorization, got %q", req.Header.Get("Authorization"))
				}
				if pageIndex == 0 && req.URL.Query().Get("limit") != "200" {
					t.Fatalf("first page limit = %q, want 200", req.URL.Query().Get("limit"))
				}
				if pageIndex > 0 && req.URL.Query().Get("cursor") != fmt.Sprint(pageIndex+1) {
					t.Fatalf("page %d cursor = %q, want %d", pageIndex+1, req.URL.Query().Get("cursor"), pageIndex+1)
				}

				if pageIndex >= len(test.memberships) {
					t.Fatalf("unexpected membership request %d", requests)
				}
				data := make([]map[string]string, 0, len(test.memberships[pageIndex]))
				for _, groupID := range test.memberships[pageIndex] {
					data = append(data, map[string]string{"type": "betaGroups", "id": groupID})
				}
				links := map[string]string{}
				if pageIndex+1 < len(test.memberships) {
					links["next"] = fmt.Sprintf(
						"https://api.appstoreconnect.apple.com/v1/betaTesters/tester-1/relationships/betaGroups?cursor=%d",
						pageIndex+2,
					)
				}
				w.Header().Set("Content-Type", "application/json")
				if err := json.NewEncoder(w).Encode(map[string]any{
					"data":  data,
					"links": links,
				}); err != nil {
					t.Fatalf("marshal response: %v", err)
				}
			}))
			t.Cleanup(server.Close)
			client := newBetaTesterCSVConflictClient(t, server)

			got, err := betaTesterGroupConflictAlreadySatisfied(
				context.Background(),
				&shared.Deps{Timeout: shared.ContextWithTimeout},
				client,
				"tester-1",
				test.requested,
				test.requestErr,
			)
			if err != nil {
				t.Fatalf("betaTesterGroupConflictAlreadySatisfied() error: %v", err)
			}
			if got != test.want {
				t.Fatalf("betaTesterGroupConflictAlreadySatisfied() = %t, want %t", got, test.want)
			}
			if requests != test.wantRequests {
				t.Fatalf("membership requests = %d, want %d", requests, test.wantRequests)
			}
		})
	}
}

func newBetaTesterCSVConflictClient(t *testing.T, server *httptest.Server) *asc.Client {
	t.Helper()
	t.Setenv("ASC_BASE_DELAY", "1ms")

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
