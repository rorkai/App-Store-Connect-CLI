package subscriptions

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func pricingGroupsClient(t testing.TB, transport resolvedPricesRoundTripFunc) *asc.Client {
	t.Helper()
	t.Setenv("ASC_MAX_RETRIES", "0")
	original := http.DefaultTransport
	http.DefaultTransport = transport
	t.Cleanup(func() { http.DefaultTransport = original })
	client, err := asc.NewClientFromPEM("KEY123", "issuer", introImportTestPrivateKeyPEM(t))
	if err != nil {
		t.Fatal(err)
	}
	return client
}

func pricingGroupsFixture(n int) []asc.Resource[asc.SubscriptionGroupAttributes] {
	groups := make([]asc.Resource[asc.SubscriptionGroupAttributes], n)
	for i := range groups {
		groups[i].ID = fmt.Sprintf("g%d", i)
		groups[i].Attributes.ReferenceName = fmt.Sprintf("Group %d", i)
	}
	return groups
}

func TestFetchSubscriptionPricingGroupsOverlapsBoundedWave(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	var active, peak, started atomic.Int32
	release := make(chan struct{})
	client := pricingGroupsClient(t, func(req *http.Request) (*http.Response, error) {
		count := active.Add(1)
		defer active.Add(-1)
		for previous := peak.Load(); count > previous; previous = peak.Load() {
			if peak.CompareAndSwap(previous, count) {
				break
			}
		}
		if started.Add(1) == 4 {
			close(release)
		}
		select {
		case <-release:
		case <-req.Context().Done():
			return nil, req.Context().Err()
		}
		return resolvedPricesJSONResponse(`{"data":[],"links":{}}`), nil
	})
	_, err := fetchSubscriptionPricingGroups(ctx, shared.DefaultDeps(), client, pricingGroupsFixture(9))
	if err != nil {
		t.Fatalf("four requests did not overlap: %v (started %d)", err, started.Load())
	}
	if peak.Load() != 4 || started.Load() != 9 {
		t.Fatalf("peak=%d started=%d", peak.Load(), started.Load())
	}
}

func TestFetchSubscriptionPricingGroupsPreservesGroupAndPageOrder(t *testing.T) {
	firstFinished := make(chan struct{})
	var pages atomic.Int32
	client := pricingGroupsClient(t, func(req *http.Request) (*http.Response, error) {
		parts := strings.Split(req.URL.Path, "/")
		group := parts[len(parts)-2]
		page := req.URL.Query().Get("cursor")
		if group == "g0" && page == "" {
			select {
			case <-firstFinished:
			case <-req.Context().Done():
				return nil, req.Context().Err()
			}
		}
		if group == "g1" && page == "" {
			close(firstFinished)
		}
		if page != "" {
			pages.Add(1)
		}
		next := ""
		suffix := "a"
		if page == "" {
			next = "https://api.appstoreconnect.apple.com" + req.URL.Path + "?cursor=next"
		} else {
			suffix = "b"
		}
		return resolvedPricesJSONResponse(fmt.Sprintf(`{"data":[{"type":"subscriptions","id":%q}],"links":{"next":%q}}`, group+suffix, next)), nil
	})
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	items, err := fetchSubscriptionPricingGroups(ctx, shared.DefaultDeps(), client, pricingGroupsFixture(3))
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 6 || pages.Load() != 3 {
		t.Fatalf("items=%d pages=%d", len(items), pages.Load())
	}
	for i, item := range items {
		suffix := "a"
		if i%2 != 0 {
			suffix = "b"
		}
		if item.Sub.ID != fmt.Sprintf("g%d%s", i/2, suffix) || item.GroupName != fmt.Sprintf("Group %d", i/2) {
			t.Fatalf("item %d: %+v", i, item)
		}
	}
}

func TestFetchSubscriptionPricingGroupsOrderedErrorCancelsPeers(t *testing.T) {
	peerStarted := make(chan struct{})
	peerCancelled := make(chan struct{})
	laterFailed := make(chan struct{})
	var laterWave atomic.Bool
	client := pricingGroupsClient(t, func(req *http.Request) (*http.Response, error) {
		switch {
		case strings.Contains(req.URL.Path, "/g0/"):
			select {
			case <-peerStarted:
			case <-req.Context().Done():
				return nil, req.Context().Err()
			}
			select {
			case <-laterFailed:
			case <-req.Context().Done():
				return nil, req.Context().Err()
			}
			return nil, errors.New("first group failure")
		case strings.Contains(req.URL.Path, "/g1/"):
			close(peerStarted)
			<-req.Context().Done()
			close(peerCancelled)
			return nil, req.Context().Err()
		case strings.Contains(req.URL.Path, "/g2/"):
			close(laterFailed)
			return nil, errors.New("later group failure")
		case strings.Contains(req.URL.Path, "/g4/"):
			laterWave.Store(true)
		}
		return resolvedPricesJSONResponse(`{"data":[],"links":{}}`), nil
	})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, err := fetchSubscriptionPricingGroups(ctx, shared.DefaultDeps(), client, pricingGroupsFixture(5))
		done <- err
	}()
	select {
	case err := <-done:
		if err == nil || (!strings.Contains(err.Error(), "group g0:") || !strings.Contains(err.Error(), "first group failure")) {
			t.Fatalf("error=%v", err)
		}
	case <-time.After(time.Second):
		cancel()
		<-done
		t.Fatal("ordered error waited for blocked peer instead of cancelling it")
	}
	select {
	case <-peerCancelled:
	default:
		t.Fatal("peer not joined after cancellation")
	}
	if laterWave.Load() {
		t.Fatal("started subsequent wave after error")
	}
}

func TestFetchSubscriptionPricingGroupsCallerCancellation(t *testing.T) {
	started := make(chan struct{}, 4)
	client := pricingGroupsClient(t, func(req *http.Request) (*http.Response, error) {
		started <- struct{}{}
		<-req.Context().Done()
		return nil, req.Context().Err()
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, err := fetchSubscriptionPricingGroups(ctx, shared.DefaultDeps(), client, pricingGroupsFixture(4))
		done <- err
	}()
	<-started
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("error=%v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("caller cancellation did not finish")
	}
}

func BenchmarkFetchSubscriptionPricingGroupsLatency(b *testing.B) {
	client := pricingGroupsClient(b, func(req *http.Request) (*http.Response, error) {
		timer := time.NewTimer(10 * time.Millisecond)
		defer timer.Stop()
		select {
		case <-timer.C:
		case <-req.Context().Done():
			return nil, req.Context().Err()
		}
		next := ""
		if req.URL.Query().Get("cursor") == "" {
			next = "https://api.appstoreconnect.apple.com" + req.URL.Path + "?cursor=next"
		}
		return resolvedPricesJSONResponse(fmt.Sprintf(`{"data":[],"links":{"next":%q}}`, next)), nil
	})
	groups := pricingGroupsFixture(12)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := fetchSubscriptionPricingGroups(context.Background(), shared.DefaultDeps(), client, groups); err != nil {
			b.Fatal(err)
		}
	}
}
