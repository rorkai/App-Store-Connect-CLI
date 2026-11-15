package metadata

import (
	"context"
	"fmt"
	"net/http"
	"path"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

func TestApplyMetadataPlanOverlapsLocaleWritesInOrder(t *testing.T) {
	locales := []string{"de-DE", "en-US", "es-ES", "fr-FR", "it", "ja"}
	var mu sync.Mutex
	inFlight, maxInFlight, arrivals := 0, 0, 0
	release := make(chan struct{})
	metadataConcurrencyTransport(t, func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodPatch {
			t.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
		}
		mu.Lock()
		inFlight++
		arrivals++
		maxInFlight = max(maxInFlight, inFlight)
		if arrivals == metadataWriteConcurrency {
			close(release)
		}
		mu.Unlock()
		select {
		case <-release:
		case <-time.After(30 * time.Second):
			t.Errorf("fewer than %d writes overlapped before timeout", metadataWriteConcurrency)
		}
		mu.Lock()
		inFlight--
		mu.Unlock()
		id := path.Base(req.URL.Path)
		return metadataFetchJSONResponse(fmt.Sprintf(`{"data":{"type":"appInfoLocalizations","id":%q,"attributes":{}}}`, id)), nil
	})

	local := map[string]appInfoLocalPatch{}
	remote := []asc.Resource[asc.AppInfoLocalizationAttributes]{}
	for _, locale := range locales {
		local[locale] = appInfoLocalPatch{
			localization: AppInfoLocalization{Name: "New"},
			setFields:    map[string]string{"name": "New"},
		}
		remote = append(remote, asc.Resource[asc.AppInfoLocalizationAttributes]{
			ID:         "loc-" + locale,
			Attributes: asc.AppInfoLocalizationAttributes{Locale: locale, Name: "Old"},
		})
	}

	actions, err := applyMetadataPlan(context.Background(), metadataTestDeps(), newMetadataFetchClient(t), "appinfo-1", "version-1", "1.0", local, nil, remote, nil, false, metadataIfExistsOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if maxInFlight != metadataWriteConcurrency {
		t.Fatalf("max in-flight writes = %d, want %d", maxInFlight, metadataWriteConcurrency)
	}
	got := make([]string, 0, len(actions))
	for _, action := range actions {
		got = append(got, action.Locale+"="+action.LocalizationID)
	}
	want := make([]string, 0, len(locales))
	for _, locale := range locales {
		want = append(want, locale+"=loc-"+locale)
	}
	if !slices.Equal(got, want) {
		t.Fatalf("actions = %v, want %v", got, want)
	}
}
