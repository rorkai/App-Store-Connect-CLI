package metadata

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

type metadataFetchRoundTripFunc func(*http.Request) (*http.Response, error)

func (fn metadataFetchRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func TestLocalizationFetchesUseFreshDeadlinePerPage(t *testing.T) {
	tests := []struct {
		name      string
		firstBody string
		lastBody  string
		fetch     func(context.Context, *asc.Client) (int, error)
	}{
		{
			name:      "app info",
			firstBody: `{"data":[{"type":"appInfoLocalizations","id":"loc-1","attributes":{"locale":"en-US"}}],"links":{"next":"/v1/appInfos/info-1/appInfoLocalizations?cursor=next"}}`,
			lastBody:  `{"data":[{"type":"appInfoLocalizations","id":"loc-2","attributes":{"locale":"fr-FR"}}],"links":{"next":""}}`,
			fetch: func(ctx context.Context, client *asc.Client) (int, error) {
				items, err := fetchAppInfoLocalizations(ctx, metadataTestDeps(), client, "info-1")
				return len(items), err
			},
		},
		{
			name:      "version",
			firstBody: `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-1","attributes":{"locale":"en-US"}}],"links":{"next":"/v1/appStoreVersions/version-1/appStoreVersionLocalizations?cursor=next"}}`,
			lastBody:  `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-2","attributes":{"locale":"fr-FR"}}],"links":{"next":""}}`,
			fetch: func(ctx context.Context, client *asc.Client) (int, error) {
				items, err := fetchVersionLocalizations(ctx, metadataTestDeps(), client, "version-1")
				return len(items), err
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("ASC_TIMEOUT", "100ms")
			t.Setenv("ASC_MAX_RETRIES", "0")
			originalTransport := http.DefaultTransport
			t.Cleanup(func() { http.DefaultTransport = originalTransport })
			calls := 0
			var firstDeadline time.Time
			http.DefaultTransport = metadataFetchRoundTripFunc(func(req *http.Request) (*http.Response, error) {
				calls++
				if calls == 1 {
					var ok bool
					firstDeadline, ok = req.Context().Deadline()
					if !ok {
						t.Fatal("expected first-page request deadline")
					}
					return metadataFetchJSONResponse(test.firstBody), nil
				}
				deadline, ok := req.Context().Deadline()
				if !ok {
					t.Fatal("expected second-page request deadline")
				}
				if !deadline.After(firstDeadline) {
					t.Fatalf("expected second-page deadline after first-page deadline: first=%s second=%s", firstDeadline, deadline)
				}
				return metadataFetchJSONResponse(test.lastBody), nil
			})
			client := newMetadataFetchClient(t)

			count, err := test.fetch(context.Background(), client)
			if err != nil {
				t.Fatalf("fetch error: %v", err)
			}
			if calls != 2 || count != 2 {
				t.Fatalf("unexpected pagination result: calls=%d count=%d", calls, count)
			}
		})
	}
}

func newMetadataFetchClient(t testing.TB) *asc.Client {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatalf("marshal key: %v", err)
	}
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
	client, err := asc.NewClientFromPEM("KEY_ID", "ISSUER_ID", string(pemBytes))
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	return client
}

func metadataFetchJSONResponse(body string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
	}
}
