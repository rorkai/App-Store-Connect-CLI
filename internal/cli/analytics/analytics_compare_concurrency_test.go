package analytics

import (
	"bytes"
	"compress/gzip"
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
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/insights"
)

func newCompareHTTPClient(tb testing.TB, handler http.HandlerFunc) *asc.Client {
	tb.Helper()
	server := httptest.NewServer(handler)
	tb.Cleanup(server.Close)
	endpoint, err := url.Parse(server.URL)
	if err != nil {
		tb.Fatal(err)
	}
	transport := server.Client().Transport
	return newCompareTransportClient(tb, compareRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		local := req.Clone(req.Context())
		local.URL.Scheme, local.URL.Host = endpoint.Scheme, endpoint.Host
		return transport.RoundTrip(local)
	}))
}

func newCompareTransportClient(tb testing.TB, transport http.RoundTripper) *asc.Client {
	tb.Helper()
	original := http.DefaultTransport
	http.DefaultTransport = transport
	defer func() { http.DefaultTransport = original }()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		tb.Fatal(err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		tb.Fatal(err)
	}
	client, err := asc.NewClientFromPEM("KEY_ID", "ISSUER_ID", string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})))
	if err != nil {
		tb.Fatal(err)
	}
	return client
}

func compareGzipFixture(tb testing.TB, text string) []byte {
	tb.Helper()
	var buf bytes.Buffer
	writer := gzip.NewWriter(&buf)
	if _, err := writer.Write([]byte(text)); err != nil {
		tb.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		tb.Fatal(err)
	}
	return buf.Bytes()
}

func compareDates(count int) []string {
	dates := make([]string, count)
	for i := range dates {
		dates[i] = fmt.Sprintf("2026-01-%02d", i+1)
	}
	return dates
}

func fetchAndAggregateOne(ctx context.Context, client *asc.Client, dates []string, salesType asc.SalesReportType) (insights.SalesMetrics, int, error) {
	result := fetchAndAggregate(ctx, client, "V", insights.SalesScope{AppID: "123", AppSKU: "APP"}, [][]string{dates}, salesType, asc.SalesReportSubTypeSummary, asc.SalesReportFrequencyDaily, false)[0]
	return result.metrics, result.found, result.err
}

func TestFetchAndAggregateConcurrentBounded(t *testing.T) {
	fixture := compareGzipFixture(t, compareSalesReportTSV())
	started := make(chan struct{}, 12)
	release := make(chan struct{})
	var active, maximum atomic.Int32
	client := newCompareHTTPClient(t, func(w http.ResponseWriter, r *http.Request) {
		n := active.Add(1)
		defer active.Add(-1)
		for old := maximum.Load(); n > old; old = maximum.Load() {
			if maximum.CompareAndSwap(old, n) {
				break
			}
		}
		started <- struct{}{}
		select {
		case <-release:
		case <-r.Context().Done():
			return
		}
		_, _ = w.Write(fixture)
	})
	done := make(chan error, 1)
	go func() {
		metrics, found, err := fetchAndAggregateOne(context.Background(), client, compareDates(12), asc.SalesReportTypeSales)
		if err == nil && (found != 12 || metrics.UnitsTotal != 24) {
			err = fmt.Errorf("found=%d units=%v", found, metrics.UnitsTotal)
		}
		done <- err
	}()
	observed := 0
	timer := time.NewTimer(time.Second)
	defer timer.Stop()
	for observed < 4 {
		select {
		case <-started:
			observed++
		case <-timer.C:
			close(release)
			<-done
			t.Fatalf("only %d requests overlapped; want 4", observed)
		}
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if got := maximum.Load(); got != 4 {
		t.Fatalf("max in flight=%d, want 4", got)
	}
}

func TestFetchAndAggregateSharesPoolAcrossPeriods(t *testing.T) {
	fixture := compareGzipFixture(t, compareSalesReportTSV())
	var arrived atomic.Int32
	release := make(chan struct{})
	client := newCompareTestClient(t, func(req *http.Request) (*http.Response, error) {
		if arrived.Add(1) == 4 {
			close(release)
		}
		select {
		case <-release:
		case <-time.After(2 * time.Second):
			t.Errorf("%s did not overlap across periods", req.URL.Query().Get("filter[reportDate]"))
		}
		body := io.NopCloser(bytes.NewReader(fixture))
		if req.URL.Query().Get("filter[reportDate]") == "2026-01-01" {
			body = io.NopCloser(strings.NewReader("invalid gzip"))
		}
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Request: req, Body: body}, nil
	})
	dates := compareDates(4)
	periods := fetchAndAggregate(context.Background(), client, "V", insights.SalesScope{AppID: "123", AppSKU: "APP"}, [][]string{dates[:1], dates[1:]}, asc.SalesReportTypeSales, asc.SalesReportSubTypeSummary, asc.SalesReportFrequencyDaily, false)
	if periods[0].err == nil || !strings.Contains(periods[0].err.Error(), "parse report 2026-01-01:") {
		t.Fatalf("baseline err=%v", periods[0].err)
	}
	if periods[1].err != nil || periods[1].found != 3 {
		t.Fatalf("comparison found=%d err=%v", periods[1].found, periods[1].err)
	}
}

type compareClosingBody struct {
	io.ReadCloser
	onClose func()
}

func (body compareClosingBody) Close() error {
	defer body.onClose()
	return body.ReadCloser.Close()
}

func TestFetchAndAggregateReducesInDateOrder(t *testing.T) {
	secondClosed, thirdClosed := make(chan struct{}), make(chan struct{})
	var closed atomic.Int32
	fixtures := make(map[string][]byte)
	for date, units := range map[string]string{"2026-01-01": "10000000000000000", "2026-01-02": "-10000000000000000", "2026-01-03": "1"} {
		fixtures[date] = compareGzipFixture(t, strings.Replace(compareSalesReportTSV(), "\t2\t3.00", "\t"+units+"\t3.00", 1))
	}
	client := newCompareTestClient(t, func(req *http.Request) (*http.Response, error) {
		date := req.URL.Query().Get("filter[reportDate]")
		switch date {
		case "2026-01-01":
			select {
			case <-secondClosed:
			case <-req.Context().Done():
				return nil, req.Context().Err()
			}
		case "2026-01-02":
			select {
			case <-thirdClosed:
			case <-req.Context().Done():
				return nil, req.Context().Err()
			}
		}
		return &http.Response{
			StatusCode: http.StatusOK, Header: make(http.Header), Request: req,
			Body: compareClosingBody{ReadCloser: io.NopCloser(bytes.NewReader(fixtures[date])), onClose: func() {
				closed.Add(1)
				switch date {
				case "2026-01-02":
					close(secondClosed)
				case "2026-01-03":
					close(thirdClosed)
				}
			}},
		}, nil
	})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	metrics, found, err := fetchAndAggregateOne(ctx, client, compareDates(3), asc.SalesReportTypeSales)
	if err != nil || found != 3 || metrics.UnitsTotal != 1 {
		t.Fatalf("found=%d units=%v err=%v", found, metrics.UnitsTotal, err)
	}
	if closed.Load() != 3 {
		t.Fatalf("closed %d bodies, want 3", closed.Load())
	}
}

func TestFetchAndAggregateFirstDateErrorCancelsAndJoins(t *testing.T) {
	fixture := compareGzipFixture(t, compareSalesReportTSV())
	secondClosed, thirdClosed := make(chan struct{}), make(chan struct{})
	var active, closed atomic.Int32
	client := newCompareTestClient(t, func(req *http.Request) (*http.Response, error) {
		active.Add(1)
		defer active.Add(-1)
		date := req.URL.Query().Get("filter[reportDate]")
		var body io.ReadCloser
		switch date {
		case "2026-01-01":
			select {
			case <-secondClosed:
			case <-req.Context().Done():
				return nil, req.Context().Err()
			}
			body = io.NopCloser(bytes.NewReader(fixture))
		case "2026-01-02":
			select {
			case <-thirdClosed:
			case <-req.Context().Done():
				return nil, req.Context().Err()
			}
			body = io.NopCloser(strings.NewReader("invalid gzip"))
		case "2026-01-03":
			body = io.NopCloser(strings.NewReader("invalid gzip"))
		default:
			<-req.Context().Done()
			return nil, req.Context().Err()
		}
		return &http.Response{
			StatusCode: http.StatusOK, Header: make(http.Header), Request: req,
			Body: compareClosingBody{ReadCloser: body, onClose: func() {
				closed.Add(1)
				switch date {
				case "2026-01-02":
					close(secondClosed)
				case "2026-01-03":
					close(thirdClosed)
				}
			}},
		}, nil
	})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	metrics, found, err := fetchAndAggregateOne(ctx, client, compareDates(12), asc.SalesReportTypeSales)
	if err == nil || !strings.Contains(err.Error(), "parse report 2026-01-02:") {
		t.Fatalf("expected first date error, got %v", err)
	}
	if found != 1 || metrics.UnitsTotal != 2 {
		t.Fatalf("partial result: found=%d units=%v", found, metrics.UnitsTotal)
	}
	if active.Load() != 0 || closed.Load() != 3 {
		t.Fatalf("active=%d closed=%d", active.Load(), closed.Load())
	}
}

func TestFetchAndAggregateContextCancellationJoins(t *testing.T) {
	started := make(chan struct{}, 12)
	var active atomic.Int32
	client := newCompareTestClient(t, func(req *http.Request) (*http.Response, error) {
		active.Add(1)
		defer active.Add(-1)
		started <- struct{}{}
		<-req.Context().Done()
		return nil, req.Context().Err()
	})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, _, err := fetchAndAggregateOne(ctx, client, compareDates(12), asc.SalesReportTypeSales)
		done <- err
	}()
	for range 4 {
		select {
		case <-started:
		case <-ctx.Done():
			t.Fatal("requests did not overlap")
		}
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("got %v, want cancellation", err)
		}
	case <-time.After(time.Second):
		t.Fatal("canceled workers did not exit")
	}
	if active.Load() != 0 {
		t.Fatalf("%d requests still active", active.Load())
	}
}

type compareContextBody struct {
	ctx     context.Context
	started chan<- struct{}
	open    *atomic.Int32
}

func (body compareContextBody) Read([]byte) (int, error) {
	body.started <- struct{}{}
	<-body.ctx.Done()
	return 0, body.ctx.Err()
}

func (body compareContextBody) Close() error {
	body.open.Add(-1)
	return nil
}

func TestFetchAndAggregateErrorClosesBlockedBodies(t *testing.T) {
	reading := make(chan struct{}, 12)
	var open atomic.Int32
	client := newCompareTestClient(t, func(req *http.Request) (*http.Response, error) {
		var body io.ReadCloser
		if req.URL.Query().Get("filter[reportDate]") == "2026-01-01" {
			for range 3 {
				select {
				case <-reading:
				case <-req.Context().Done():
					return nil, req.Context().Err()
				}
			}
			body = io.NopCloser(strings.NewReader("invalid gzip"))
		} else {
			open.Add(1)
			body = compareContextBody{ctx: req.Context(), started: reading, open: &open}
		}
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Request: req, Body: body}, nil
	})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_, _, err := fetchAndAggregateOne(ctx, client, compareDates(12), asc.SalesReportTypeSales)
	if err == nil || !strings.Contains(err.Error(), "parse report 2026-01-01:") {
		t.Fatalf("expected earliest parse error, got %v", err)
	}
	if open.Load() != 0 {
		t.Fatalf("%d blocked bodies remain open", open.Load())
	}
}

// This measures local HTTP latency overlap, not App Store Connect response time.
func BenchmarkFetchAndAggregateHTTP(b *testing.B) {
	fixture := compareGzipFixture(b, compareSalesReportTSV())
	client := newCompareHTTPClient(b, func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-time.After(20 * time.Millisecond):
		case <-r.Context().Done():
			return
		}
		_, _ = w.Write(fixture)
	})
	dates := compareDates(16)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, found, err := fetchAndAggregateOne(context.Background(), client, dates, asc.SalesReportTypeSales)
		if err != nil || found != 16 {
			b.Fatalf("found=%d err=%v", found, err)
		}
	}
}
