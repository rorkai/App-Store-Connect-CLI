package optimize

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

type searchMetadataRoundTripFunc func(*http.Request) (*http.Response, error)

func (fn searchMetadataRoundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

func TestResolveSearchMetadataJoinsVersionAndAppInfoLocalizations(t *testing.T) {
	client := newSearchMetadataTestClient(t, func(request *http.Request) (*http.Response, error) {
		if request.Method != http.MethodGet {
			t.Fatalf("method = %s", request.Method)
		}
		switch request.URL.Path {
		case "/v1/apps/123456789/appStoreVersions":
			if request.URL.Query().Get("filter[versionString]") != "4.4.4" || request.URL.Query().Get("filter[platform]") != "IOS" {
				t.Fatalf("version query = %s", request.URL.RawQuery)
			}
			return searchMetadataResponse(`{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"platform":"IOS","versionString":"4.4.4","appStoreState":"READY_FOR_SALE"}}]}`), nil
		case "/v1/appStoreVersions/version-1/appStoreVersionLocalizations":
			if request.URL.Query().Get("filter[locale]") != "en-US" {
				t.Fatalf("version localization query = %s", request.URL.RawQuery)
			}
			return searchMetadataResponse(`{"data":[{"type":"appStoreVersionLocalizations","id":"version-loc-1","attributes":{"locale":"en-US","keywords":"focus,timer"}}]}`), nil
		case "/v1/apps/123456789/appInfos":
			return searchMetadataResponse(`{"data":[{"type":"appInfos","id":"info-rejected","attributes":{"appStoreState":"DEVELOPER_REJECTED"}},{"type":"appInfos","id":"info-live","attributes":{"appStoreState":"READY_FOR_DISTRIBUTION"}}]}`), nil
		case "/v1/appInfos/info-live/appInfoLocalizations":
			if request.URL.Query().Get("filter[locale]") != "en-US" {
				t.Fatalf("app info localization query = %s", request.URL.RawQuery)
			}
			return searchMetadataResponse(`{"data":[{"type":"appInfoLocalizations","id":"info-loc-1","attributes":{"locale":"en-US","name":"Focus Keeper","subtitle":"Habit tracker"}}]}`), nil
		default:
			t.Fatalf("unexpected request path %s", request.URL.Path)
			return nil, nil
		}
	})
	d := newOptimizeTestDeps(nil)
	d.Client = func() (*asc.Client, error) { return client, nil }

	result, err := resolveSearchMetadata(context.Background(), d, "123456789", "4.4.4", "IOS", "", "en-US")
	if err != nil {
		t.Fatal(err)
	}
	if result.AppID != "123456789" || result.VersionID != "version-1" || result.AppInfoID != "info-live" || result.Platform != "IOS" {
		t.Fatalf("identity = %+v", result)
	}
	if result.Metadata.Name != "Focus Keeper" || result.Metadata.Subtitle != "Habit tracker" || result.Metadata.Keywords != "focus,timer" {
		t.Fatalf("metadata = %+v", result.Metadata)
	}
}

func newSearchMetadataTestClient(t *testing.T, roundTrip searchMetadataRoundTripFunc) *asc.Client {
	t.Helper()
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		t.Fatal(err)
	}
	keyPath := filepath.Join(t.TempDir(), "AuthKey_TEST.p8")
	if err := os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}), 0o600); err != nil {
		t.Fatal(err)
	}
	client, err := asc.NewClientWithHTTPClient("TESTKEY", "TESTISSUER", keyPath, &http.Client{Transport: roundTrip})
	if err != nil {
		t.Fatal(err)
	}
	return client
}

func searchMetadataResponse(body string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Status:     "200 OK",
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}
