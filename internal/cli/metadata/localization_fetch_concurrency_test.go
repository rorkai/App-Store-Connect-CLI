package metadata

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func metadataConcurrencyTransport(t testing.TB, fn metadataFetchRoundTripFunc) {
	t.Helper()
	t.Setenv("ASC_MAX_RETRIES", "0")
	old := http.DefaultTransport
	http.DefaultTransport = fn
	t.Cleanup(func() { http.DefaultTransport = old })
}

func TestMetadataLocalizationFetchOverlapsCollections(t *testing.T) {
	started := make(chan string, 2)
	release := make(chan struct{})
	metadataConcurrencyTransport(t, func(req *http.Request) (*http.Response, error) {
		started <- req.URL.Path
		select {
		case <-release:
		case <-req.Context().Done():
			return nil, req.Context().Err()
		}
		return metadataFetchJSONResponse(`{"data":[],"links":{}}`), nil
	})
	client := newMetadataFetchClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, _, err := fetchMetadataLocalizations(ctx, metadataTestDeps(), client, "info-1", "version-1", true, true)
		done <- err
	}()
	for range 2 {
		select {
		case <-started:
		case <-time.After(time.Second):
			cancel()
			<-done
			t.Fatal("independent localization collections did not start together")
		}
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestMetadataLocalizationFetchScopeGating(t *testing.T) {
	for _, test := range []struct{ appInfo, version bool }{{false, false}, {true, false}, {false, true}, {true, true}} {
		t.Run(fmt.Sprintf("appInfo=%t/version=%t", test.appInfo, test.version), func(t *testing.T) {
			var appCalls, versionCalls atomic.Int32
			metadataConcurrencyTransport(t, func(req *http.Request) (*http.Response, error) {
				if strings.Contains(req.URL.Path, "appInfos") {
					appCalls.Add(1)
				} else {
					versionCalls.Add(1)
				}
				return metadataFetchJSONResponse(`{"data":[],"links":{}}`), nil
			})
			_, _, err := fetchMetadataLocalizations(context.Background(), metadataTestDeps(), newMetadataFetchClient(t), "info-1", "version-1", test.appInfo, test.version)
			if err != nil {
				t.Fatal(err)
			}
			wantApp, wantVersion := int32(0), int32(0)
			if test.appInfo {
				wantApp = 1
			}
			if test.version {
				wantVersion = 1
			}
			if appCalls.Load() != wantApp || versionCalls.Load() != wantVersion {
				t.Fatalf("calls = %d/%d, want %d/%d", appCalls.Load(), versionCalls.Load(), wantApp, wantVersion)
			}
		})
	}
}

func TestMetadataLocalizationFetchAppInfoFailureCancelsVersion(t *testing.T) {
	appInfoFailure := errors.New("app-info failure")
	versionStarted := make(chan struct{})
	versionStopped := make(chan struct{})
	metadataConcurrencyTransport(t, func(req *http.Request) (*http.Response, error) {
		if strings.Contains(req.URL.Path, "appInfos") {
			select {
			case <-versionStarted:
			case <-req.Context().Done():
				return nil, req.Context().Err()
			}
			return nil, appInfoFailure
		}
		close(versionStarted)
		<-req.Context().Done()
		close(versionStopped)
		return nil, req.Context().Err()
	})
	client := newMetadataFetchClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_, _, err := fetchMetadataLocalizations(ctx, metadataTestDeps(), client, "info-1", "version-1", true, true)
	if !errors.Is(err, appInfoFailure) {
		t.Fatalf("error = %v", err)
	}
	select {
	case <-versionStopped:
	default:
		t.Fatal("version worker did not finish before return")
	}
	if ctx.Err() != nil {
		t.Fatal("first-priority failure waited for parent timeout")
	}
}

func TestMetadataLocalizationFetchVersionFailurePreservesAppInfoPrecedence(t *testing.T) {
	appInfoFailure := errors.New("app-info failure")
	versionFailed := make(chan struct{})
	metadataConcurrencyTransport(t, func(req *http.Request) (*http.Response, error) {
		if strings.Contains(req.URL.Path, "appInfos") {
			select {
			case <-versionFailed:
			case <-req.Context().Done():
				return nil, req.Context().Err()
			}
			return nil, appInfoFailure
		}
		close(versionFailed)
		return nil, errors.New("version failure")
	})
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_, _, err := fetchMetadataLocalizations(ctx, metadataTestDeps(), newMetadataFetchClient(t), "info-1", "version-1", true, true)
	if !errors.Is(err, appInfoFailure) {
		t.Fatalf("error = %v", err)
	}
}

func TestMetadataLocalizationFetchReturnsVersionFailureAfterAppInfoSuccess(t *testing.T) {
	versionFailure := errors.New("version failure")
	versionFailed := make(chan struct{})
	metadataConcurrencyTransport(t, func(req *http.Request) (*http.Response, error) {
		if strings.Contains(req.URL.Path, "appInfos") {
			select {
			case <-versionFailed:
			case <-req.Context().Done():
				return nil, req.Context().Err()
			}
			return metadataFetchJSONResponse(`{"data":[],"links":{}}`), nil
		}
		close(versionFailed)
		return nil, versionFailure
	})
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_, _, err := fetchMetadataLocalizations(ctx, metadataTestDeps(), newMetadataFetchClient(t), "info-1", "version-1", true, true)
	if !errors.Is(err, versionFailure) {
		t.Fatalf("error = %v", err)
	}
}

func TestMetadataLocalizationFetchParentCancellation(t *testing.T) {
	started := make(chan struct{}, 2)
	var stopped atomic.Int32
	metadataConcurrencyTransport(t, func(req *http.Request) (*http.Response, error) {
		started <- struct{}{}
		<-req.Context().Done()
		stopped.Add(1)
		return nil, req.Context().Err()
	})
	client := newMetadataFetchClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, _, err := fetchMetadataLocalizations(ctx, metadataTestDeps(), client, "info-1", "version-1", true, true)
		done <- err
	}()
	for range 2 {
		select {
		case <-started:
		case <-time.After(time.Second):
			cancel()
			<-done
			t.Fatal("workers did not start")
		}
	}
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v", err)
	}
	if stopped.Load() != 2 {
		t.Fatalf("stopped workers = %d", stopped.Load())
	}
}

func TestMetadataLocalizationFetchPagination(t *testing.T) {
	var calls atomic.Int32
	metadataConcurrencyTransport(t, func(req *http.Request) (*http.Response, error) {
		calls.Add(1)
		kind := "appInfoLocalizations"
		if strings.Contains(req.URL.Path, "appStoreVersions") {
			kind = "appStoreVersionLocalizations"
		}
		if req.URL.Query().Get("cursor") == "next" {
			return metadataFetchJSONResponse(fmt.Sprintf(`{"data":[{"type":%q,"id":"loc-2","attributes":{"locale":"fr-FR"}}],"links":{}}`, kind)), nil
		}
		return metadataFetchJSONResponse(fmt.Sprintf(`{"data":[{"type":%q,"id":"loc-1","attributes":{"locale":"en-US"}}],"links":{"next":%q}}`, kind, req.URL.Path+"?cursor=next")), nil
	})
	appInfo, version, err := fetchMetadataLocalizations(context.Background(), metadataTestDeps(), newMetadataFetchClient(t), "info-1", "version-1", true, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(appInfo) != 2 || len(version) != 2 || calls.Load() != 4 {
		t.Fatalf("items=%d/%d calls=%d", len(appInfo), len(version), calls.Load())
	}
}

func BenchmarkMetadataLocalizationCollections(b *testing.B) {
	var body strings.Builder
	body.WriteString(`{"data":[`)
	for i := range 40 {
		if i > 0 {
			body.WriteByte(',')
		}
		fmt.Fprintf(&body, `{"type":"localizations","id":"loc-%d","attributes":{"locale":"locale-%d","description":"App description","keywords":"one,two,three","name":"App name"}}`, i, i)
	}
	body.WriteString(`],"links":{}}`)
	payload := body.String()
	metadataConcurrencyTransport(b, func(req *http.Request) (*http.Response, error) {
		select {
		case <-time.After(25 * time.Millisecond):
		case <-req.Context().Done():
			return nil, req.Context().Err()
		}
		return metadataFetchJSONResponse(payload), nil
	})
	client := newMetadataFetchClient(b)
	b.ResetTimer()
	for range b.N {
		appInfo, version, err := fetchMetadataLocalizations(context.Background(), metadataTestDeps(), client, "info-1", "version-1", true, true)
		if err != nil {
			b.Fatal(err)
		}
		if len(appInfo) != 40 || len(version) != 40 {
			b.Fatal("lost localizations")
		}
	}
}
