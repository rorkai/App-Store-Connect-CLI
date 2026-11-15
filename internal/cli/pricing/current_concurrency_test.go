package pricing

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

var currentPriceDeps = &shared.Deps{Timeout: shared.ContextWithTimeout}

type currentPriceTransport func(*http.Request) (*http.Response, error)

func (fn currentPriceTransport) RoundTrip(req *http.Request) (*http.Response, error) { return fn(req) }

func currentPriceClient(t testing.TB, transport currentPriceTransport) *asc.Client {
	t.Helper()
	t.Setenv("ASC_MAX_RETRIES", "0")
	original := http.DefaultTransport
	http.DefaultTransport = transport
	t.Cleanup(func() { http.DefaultTransport = original })
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	client, err := asc.NewClientFromPEM("KEY123", "issuer", string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})))
	if err != nil {
		t.Fatal(err)
	}
	return client
}

func currentPriceResponse(next string) *http.Response {
	return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(fmt.Sprintf(`{"data":[],"links":{"next":%q}}`, next)))}
}

func TestFetchAppCurrentSchedulePricesOverlapsSiblings(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	var started, active, peak atomic.Int32
	release := make(chan struct{})
	client := currentPriceClient(t, func(req *http.Request) (*http.Response, error) {
		n := active.Add(1)
		defer active.Add(-1)
		for old := peak.Load(); n > old; old = peak.Load() {
			if peak.CompareAndSwap(old, n) {
				break
			}
		}
		if started.Add(1) == 2 {
			close(release)
		}
		select {
		case <-release:
		case <-req.Context().Done():
			return nil, req.Context().Err()
		}
		return currentPriceResponse(""), nil
	})
	if _, _, _, err := fetchAppCurrentSchedulePrices(ctx, currentPriceDeps, client, "schedule", true); err != nil {
		t.Fatalf("siblings did not overlap: %v; requests=%d", err, started.Load())
	}
	if started.Load() != 2 || peak.Load() != 2 {
		t.Fatalf("requests=%d peak=%d", started.Load(), peak.Load())
	}
}

func TestFetchAppCurrentSchedulePricesManualErrorCancelsAndJoinsAutomatic(t *testing.T) {
	autoStarted := make(chan struct{})
	autoCancelled := make(chan struct{})
	manualErr := errors.New("manual failure")
	client := currentPriceClient(t, func(req *http.Request) (*http.Response, error) {
		if strings.HasSuffix(req.URL.Path, "/automaticPrices") {
			close(autoStarted)
			<-req.Context().Done()
			close(autoCancelled)
			return nil, req.Context().Err()
		}
		select {
		case <-autoStarted:
		case <-req.Context().Done():
			return nil, req.Context().Err()
		}
		return nil, manualErr
	})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, _, _, err := fetchAppCurrentSchedulePrices(ctx, currentPriceDeps, client, "schedule", true)
		done <- err
	}()
	select {
	case err := <-done:
		if !errors.Is(err, manualErr) || !strings.Contains(err.Error(), "fetch manual prices:") {
			t.Fatalf("error=%v", err)
		}
	case <-time.After(time.Second):
		cancel()
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatal("cancelled helper did not finish")
		}
		t.Fatal("manual error waited for blocked automatic request")
	}
	select {
	case <-autoCancelled:
	default:
		t.Fatal("automatic request was not cancelled and joined")
	}
}

func TestFetchAppCurrentSchedulePricesManualErrorPrecedesAutomaticError(t *testing.T) {
	autoFailed := make(chan struct{})
	manualErr := errors.New("manual failure")
	automaticErr := errors.New("automatic failure")
	client := currentPriceClient(t, func(req *http.Request) (*http.Response, error) {
		if strings.HasSuffix(req.URL.Path, "/automaticPrices") {
			close(autoFailed)
			return nil, automaticErr
		}
		select {
		case <-autoFailed:
		case <-req.Context().Done():
			return nil, req.Context().Err()
		}
		return nil, manualErr
	})
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_, _, _, err := fetchAppCurrentSchedulePrices(ctx, currentPriceDeps, client, "schedule", true)
	if !errors.Is(err, manualErr) || errors.Is(err, automaticErr) {
		t.Fatalf("error=%v", err)
	}
}

func TestFetchAppCurrentSchedulePricesBaseOnlyAndCancellation(t *testing.T) {
	t.Run("base only", func(t *testing.T) {
		var calls atomic.Int32
		client := currentPriceClient(t, func(req *http.Request) (*http.Response, error) {
			calls.Add(1)
			if !strings.HasSuffix(req.URL.Path, "/manualPrices") {
				return nil, errors.New("unexpected automatic request")
			}
			return currentPriceResponse(""), nil
		})
		if _, _, _, err := fetchAppCurrentSchedulePrices(context.Background(), currentPriceDeps, client, "schedule", false); err != nil {
			t.Fatal(err)
		}
		if calls.Load() != 1 {
			t.Fatalf("calls=%d", calls.Load())
		}
	})
	t.Run("caller cancellation", func(t *testing.T) {
		started := make(chan struct{}, 2)
		var cancelled atomic.Int32
		client := currentPriceClient(t, func(req *http.Request) (*http.Response, error) {
			started <- struct{}{}
			<-req.Context().Done()
			cancelled.Add(1)
			return nil, req.Context().Err()
		})
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		done := make(chan error, 1)
		go func() {
			_, _, _, err := fetchAppCurrentSchedulePrices(ctx, currentPriceDeps, client, "schedule", true)
			done <- err
		}()
		select {
		case <-started:
		case <-time.After(time.Second):
			cancel()
			t.Fatal("request did not start")
		}
		cancel()
		select {
		case err := <-done:
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("error=%v", err)
			}
		case <-time.After(time.Second):
			t.Fatal("caller cancellation stalled")
		}
		if cancelled.Load() == 0 {
			t.Fatal("no request cancelled")
		}
	})
}

func TestFetchAppCurrentSchedulePricesMergesInOriginalOrder(t *testing.T) {
	automaticReturned := make(chan struct{})
	client := currentPriceClient(t, func(req *http.Request) (*http.Response, error) {
		territory, price, currency := "USA", "1.99", "USD"
		manual := strings.HasSuffix(req.URL.Path, "/manualPrices")
		if manual {
			select {
			case <-automaticReturned:
			case <-req.Context().Done():
				return nil, req.Context().Err()
			}
		} else {
			territory, price, currency = "GBR", "2.99", "GBP"
			defer close(automaticReturned)
		}
		body := fmt.Sprintf(`{"data":[{"type":"appPrices","id":%q,"attributes":{"manual":%t},"relationships":{"territory":{"data":{"type":"territories","id":%q}},"appPricePoint":{"data":{"type":"appPricePoints","id":"pp"}}}}],"included":[{"type":"appPricePoints","id":"pp","attributes":{"customerPrice":%q}},{"type":"territories","id":"USA","attributes":{"currency":%q}}],"links":{}}`, territory, manual, territory, price, currency)
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}, nil
	})
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	entries, values, currencies, err := fetchAppCurrentSchedulePrices(ctx, currentPriceDeps, client, "schedule", true)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 || entries[0].TerritoryID != "USA" || entries[1].TerritoryID != "GBR" {
		t.Fatalf("entry order: %+v", entries)
	}
	if values["pp"].CustomerPrice != "2.99" || currencies["USA"] != "GBP" {
		t.Fatalf("automatic map overrides lost: values=%+v currencies=%+v", values, currencies)
	}
}

func BenchmarkFetchAppCurrentSchedulePricesLatency(b *testing.B) {
	var calls atomic.Int32
	client := currentPriceClient(b, func(req *http.Request) (*http.Response, error) {
		calls.Add(1)
		timer := time.NewTimer(10 * time.Millisecond)
		defer timer.Stop()
		select {
		case <-timer.C:
		case <-req.Context().Done():
			return nil, req.Context().Err()
		}
		next := ""
		switch req.URL.Query().Get("cursor") {
		case "":
			next = "https://api.appstoreconnect.apple.com" + req.URL.Path + "?cursor=second"
		case "second":
			next = "https://api.appstoreconnect.apple.com" + req.URL.Path + "?cursor=third"
		}
		return currentPriceResponse(next), nil
	})
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, _, _, err := fetchAppCurrentSchedulePrices(context.Background(), currentPriceDeps, client, "schedule", true); err != nil {
			b.Fatal(err)
		}
	}
	b.StopTimer()
	if calls.Load() != int32(b.N*6) {
		b.Fatalf("calls=%d want=%d", calls.Load(), b.N*6)
	}
	b.ReportMetric(6, "requests/op")
}
