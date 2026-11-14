package web

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestTransactionTaxDownloadBuildsExactGenerationQuery(t *testing.T) {
	var generationCalls, statusCalls, downloadCalls int
	var server *httptest.Server
	server = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/WebObjects/iTunesConnect.woa/ra/paymentConsolidation/localization/keyValueMapping":
			_, _ = io.WriteString(w, `{"data":{"region.a":"Alpha","region.z":"Zed","region.d":"Deferred"}}`)
		case r.URL.Path == "/WebObjects/iTunesConnect.woa/ra/paymentConsolidation/regionCountriesMapping":
			_, _ = io.WriteString(w, `{"data":{}}`)
		case r.URL.Path == "/WebObjects/iTunesConnect.woa/ra/paymentConsolidation/providers/1234/sapVendorNumbers":
			_, _ = io.WriteString(w, `{"data":[{"sapVendorNumber":42,"lastSoldUnit":100}]}`)
		case r.URL.Path == "/WebObjects/iTunesConnect.woa/ra/paymentConsolidation/providers/1234/sapVendorNumbers/42":
			_, _ = io.WriteString(w, `{"hasVendorTaxReport":true,"isArcadeVendor":false,"reportSummaries":[{"proceedsByRegion":[{"regionCurrency":{"regionCurrencyId":12,"regionCode":"US","regionName":"Zed","regionNameKey":"region.z"},"financialReportType":"R","earned":"1"},{"regionCurrency":{"regionCurrencyId":13,"regionCode":"CA","regionName":"Alpha","regionNameKey":"region.a"},"financialReportType":"R","earned":"1"},{"regionCurrency":{"regionCurrencyId":99,"regionCode":"Z2","regionName":"Ignored","regionNameKey":"region.ignored"},"financialReportType":"R","earned":"1"},{"regionCurrency":{"regionCurrencyId":88,"regionCode":"DE","regionName":"Deferred","regionNameKey":"region.d"},"financialReportType":"D","earned":"0"}]}]}`)
		case r.URL.Path == "/WebObjects/iTunesConnect.woa/ra/paymentConsolidation/providers/1234/sapVendorNumbers/42/reports":
			generationCalls++
			if r.Method != http.MethodGet {
				t.Errorf("generation method = %s, want GET", r.Method)
			}
			body, readErr := io.ReadAll(r.Body)
			if readErr != nil {
				t.Errorf("read generation body: %v", readErr)
			} else if len(body) != 0 {
				t.Errorf("generation body = %q, want empty", body)
			}
			if got := r.URL.Query().Get("year"); got != "2026" {
				t.Errorf("generation year = %q, want 2026", got)
			}
			if got := r.URL.Query().Get("month"); got != "7" {
				t.Errorf("generation month = %q, want 7", got)
			}
			if got := r.URL.Query().Get("regionCurrencyIds"); got != "13,88,12" {
				t.Errorf("generation regionCurrencyIds = %q, want 13,88,12", got)
			}
			if raw, ok := r.URL.Query()["reportTypes"]; !ok || len(raw) != 1 || raw[0] != "" {
				t.Errorf("generation reportTypes = %#v, want one empty value", raw)
			}
			if got := r.URL.Query().Get("isVendorTaxReportReq"); got != "true" {
				t.Errorf("generation isVendorTaxReportReq = %q, want true", got)
			}
			_, _ = io.WriteString(w, `{"uuid":"job-123","estimatedWaitingTime":1}`)
		case strings.HasSuffix(r.URL.Path, "/reports/job-123/status"):
			statusCalls++
			_, _ = io.WriteString(w, `{"status":"readyForDownload","downloadUrl":"`+server.URL+`/download?signature=SECRET"}`)
		case r.URL.Path == "/download":
			downloadCalls++
			w.Header().Set("Content-Type", "application/octet-stream")
			w.Header().Set("Content-Disposition", `attachment; filename="report.zip"`)
			_, _ = w.Write([]byte("PK\x03\x04transaction-tax"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := &Client{
		httpClient:         server.Client(),
		baseURL:            server.URL,
		minRequestInterval: 0,
	}

	result, err := client.DownloadTransactionTaxReport(context.Background(), TransactionTaxReportRequest{
		ProviderID: 1234,
		Date:       "2026-07",
	})
	if err != nil {
		t.Fatalf("DownloadTransactionTaxReport() error: %v", err)
	}
	defer result.Body.Close()
	if generationCalls != 1 || statusCalls != 1 || downloadCalls != 1 {
		t.Fatalf("calls = generation %d, status %d, download %d; want 1 each", generationCalls, statusCalls, downloadCalls)
	}
	if result.PollStatus != "readyForDownload" {
		t.Fatalf("PollStatus = %q, want readyForDownload", result.PollStatus)
	}
	if result.ContentType != "application/octet-stream" || !result.ContentDispositionPresent {
		t.Fatalf("download metadata = %#v, want octet-stream and disposition", result)
	}
	if _, err := io.ReadAll(result.Body); err != nil {
		t.Fatalf("read download body: %v", err)
	}
}

func TestTransactionTaxDownloadPollsUntilReadyWithoutRegenerating(t *testing.T) {
	previousWait := transactionTaxPollWaitFn
	transactionTaxPollWaitFn = func(context.Context) error { return nil }
	t.Cleanup(func() { transactionTaxPollWaitFn = previousWait })

	generationCalls, statusCalls := 0, 0
	var server *httptest.Server
	server = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/WebObjects/iTunesConnect.woa/ra/paymentConsolidation/localization/keyValueMapping":
			_, _ = io.WriteString(w, `{"data":{}}`)
		case "/WebObjects/iTunesConnect.woa/ra/paymentConsolidation/regionCountriesMapping":
			_, _ = io.WriteString(w, `{"data":{}}`)
		case "/WebObjects/iTunesConnect.woa/ra/paymentConsolidation/providers/1234/sapVendorNumbers":
			_, _ = io.WriteString(w, `{"data":[{"sapVendorNumber":42,"lastSoldUnit":100}]}`)
		case "/WebObjects/iTunesConnect.woa/ra/paymentConsolidation/providers/1234/sapVendorNumbers/42":
			_, _ = io.WriteString(w, `{"hasVendorTaxReport":true,"isArcadeVendor":false,"reportSummaries":[{"proceedsByRegion":[{"regionCurrency":{"regionCurrencyId":13,"regionCode":"CA","regionName":"Alpha","regionNameKey":"region.a"},"financialReportType":"R","earned":"1"}]}]}`)
		case "/WebObjects/iTunesConnect.woa/ra/paymentConsolidation/providers/1234/sapVendorNumbers/42/reports":
			generationCalls++
			_, _ = io.WriteString(w, `{"uuid":"job-123","estimatedWaitingTime":1}`)
		case "/WebObjects/iTunesConnect.woa/ra/paymentConsolidation/providers/1234/sapVendorNumbers/42/reports/job-123/status":
			statusCalls++
			if statusCalls == 1 {
				_, _ = io.WriteString(w, `{"status":"generating"}`)
				return
			}
			_, _ = io.WriteString(w, `{"status":"readyForDownload","downloadUrl":"`+server.URL+`/download"}`)
		case "/download":
			w.Header().Set("Content-Type", "application/octet-stream")
			_, _ = w.Write([]byte("PK\x03\x04archive"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := &Client{httpClient: server.Client(), baseURL: server.URL, minRequestInterval: 0}
	result, err := client.DownloadTransactionTaxReport(context.Background(), TransactionTaxReportRequest{ProviderID: 1234, Date: "2026-07"})
	if err != nil {
		t.Fatalf("DownloadTransactionTaxReport() error: %v", err)
	}
	defer result.Body.Close()
	if generationCalls != 1 || statusCalls != 2 {
		t.Fatalf("calls = generation %d, status %d, want generation 1/status 2", generationCalls, statusCalls)
	}
}

func TestTransactionTaxGenerationDoesNotRetryReusedConnection(t *testing.T) {
	var primeCalls, generationCalls int
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/prime":
			primeCalls++
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{}`)
		case "/WebObjects/iTunesConnect.woa/ra/paymentConsolidation/providers/1234/sapVendorNumbers/42/reports":
			generationCalls++
			connection, _, err := http.NewResponseController(w).Hijack()
			if err != nil {
				t.Fatalf("hijack generation connection: %v", err)
			}
			_ = connection.Close()
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := &Client{httpClient: server.Client(), baseURL: server.URL, minRequestInterval: 0}
	if _, err := client.transactionTaxJSON(context.Background(), "/prime"); err != nil {
		t.Fatalf("prime transactionTaxJSON() error: %v", err)
	}
	if primeCalls != 1 {
		t.Fatalf("prime calls = %d, want 1", primeCalls)
	}

	_, err := client.generateTransactionTaxReport(context.Background(), 1234, 42, "2026", "7", []int64{13})
	if err == nil {
		t.Fatal("generateTransactionTaxReport() error = nil, want connection failure")
	}
	if generationCalls != 1 {
		t.Fatalf("generation calls = %d, want one attempt", generationCalls)
	}
	if err.Error() != "transaction tax report generation failed" {
		t.Fatalf("error = %q, want redacted generation failure", err)
	}
}

func TestTransactionTaxSingleAttemptHTTPClientClonesConfiguredTransport(t *testing.T) {
	originalTransport := &http.Transport{
		MaxIdleConns:        10,
		MaxIdleConnsPerHost: 4,
		TLSClientConfig:     &tls.Config{NextProtos: []string{"h2", "http/1.1"}},
	}
	original := &http.Client{Transport: originalTransport, Timeout: 17 * time.Second}
	clone := transactionTaxSingleAttemptHTTPClient(original)
	if clone == original {
		t.Fatal("single-attempt client aliases the configured client")
	}
	transport, ok := clone.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("single-attempt transport type = %T, want *http.Transport", clone.Transport)
	}
	if transport == originalTransport {
		t.Fatal("single-attempt transport aliases the configured transport")
	}
	if !transport.DisableKeepAlives {
		t.Fatal("single-attempt transport leaves keep-alives enabled")
	}
	if transport.Protocols == nil || !transport.Protocols.HTTP1() || transport.Protocols.HTTP2() || transport.Protocols.UnencryptedHTTP2() {
		t.Fatalf("single-attempt protocols = %v, want HTTP/1 only", transport.Protocols)
	}
	if len(transport.TLSNextProto) != 0 {
		t.Fatalf("single-attempt TLSNextProto = %#v, want empty", transport.TLSNextProto)
	}
	if transport.TLSClientConfig == originalTransport.TLSClientConfig {
		t.Fatal("single-attempt TLS config aliases the configured TLS config")
	}
	if got := strings.Join(transport.TLSClientConfig.NextProtos, ","); got != "http/1.1" {
		t.Fatalf("single-attempt TLS NextProtos = %q, want HTTP/1.1 only", got)
	}
	if got := strings.Join(originalTransport.TLSClientConfig.NextProtos, ","); got != "h2,http/1.1" {
		t.Fatalf("configured TLS NextProtos = %q, want unchanged", got)
	}
	if clone.Timeout != original.Timeout {
		t.Fatalf("single-attempt timeout = %s, want %s", clone.Timeout, original.Timeout)
	}
	if originalTransport.DisableKeepAlives {
		t.Fatal("single-attempt setup mutated the configured transport")
	}
}

func TestTransactionTaxSingleAttemptHTTPClientNegotiatesHTTP1AfterHTTP2Base(t *testing.T) {
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{}`)
	}))
	server.EnableHTTP2 = true
	server.StartTLS()
	defer server.Close()

	base := server.Client()
	response, err := base.Get(server.URL + "/prime")
	if err != nil {
		t.Fatalf("base GET() error: %v", err)
	}
	_ = response.Body.Close()
	if response.ProtoMajor != 2 {
		t.Fatalf("base response protocol = %s, want HTTP/2.0", response.Proto)
	}

	singleAttempt := transactionTaxSingleAttemptHTTPClient(base)
	response, err = singleAttempt.Get(server.URL + "/single")
	if err != nil {
		t.Fatalf("single-attempt GET() error: %v", err)
	}
	defer response.Body.Close()
	if response.ProtoMajor != 1 || response.ProtoMinor != 1 {
		t.Fatalf("single-attempt response protocol = %s, want HTTP/1.1", response.Proto)
	}
}

type transactionTaxTestRoundTripper struct{}

func (*transactionTaxTestRoundTripper) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, errors.New("not implemented")
}

func TestTransactionTaxSingleAttemptHTTPClientPreservesCustomTransport(t *testing.T) {
	originalTransport := &transactionTaxTestRoundTripper{}
	original := &http.Client{Transport: originalTransport}
	clone := transactionTaxSingleAttemptHTTPClient(original)
	if got, ok := clone.Transport.(*transactionTaxTestRoundTripper); !ok || got != originalTransport {
		t.Fatalf("single-attempt custom transport = %T %p, want %p", clone.Transport, clone.Transport, originalTransport)
	}
}

func TestTransactionTaxGenerationRejectsRedirectWithoutReplaying(t *testing.T) {
	var generationCalls, targetCalls int
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/WebObjects/iTunesConnect.woa/ra/paymentConsolidation/providers/1234/sapVendorNumbers/42/reports":
			generationCalls++
			http.Redirect(w, r, "/generation-target", http.StatusFound)
		case "/generation-target":
			targetCalls++
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"uuid":"job-123","estimatedWaitingTime":1}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := &Client{httpClient: server.Client(), baseURL: server.URL, minRequestInterval: 0}
	_, err := client.generateTransactionTaxReport(context.Background(), 1234, 42, "2026", "7", []int64{13})
	if err == nil {
		t.Fatal("generateTransactionTaxReport() error = nil, want redirect rejection")
	}
	if generationCalls != 1 || targetCalls != 0 {
		t.Fatalf("calls = generation %d, redirect target %d; want one generation and no target", generationCalls, targetCalls)
	}
}

func TestTransactionTaxDownloadUsesWorkflowDeadline(t *testing.T) {
	const clientTimeout = time.Minute
	var requestDeadline time.Time
	var requestBounded bool
	origin := &url.URL{Scheme: "https", Host: "appstoreconnect.example.test"}
	client := &Client{
		httpClient: &http.Client{
			Timeout: clientTimeout,
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				requestDeadline, requestBounded = req.Context().Deadline()
				if err := req.Context().Err(); err != nil {
					return nil, err
				}
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     http.Header{"Content-Type": []string{"application/zip"}},
					Body:       io.NopCloser(strings.NewReader("PK\x03\x04archive")),
					Request:    req,
				}, nil
			}),
		},
		baseURL: origin.String(),
	}
	archiveURL := origin.String() + "/archive"

	workflowDeadline := time.Now().Add(time.Hour)
	ctx, cancel := context.WithDeadline(context.Background(), workflowDeadline)
	defer cancel()
	result, err := client.openTransactionTaxDownload(ctx, origin, archiveURL)
	if err != nil {
		t.Fatalf("download within workflow deadline: %v", err)
	}
	result.Body.Close()
	if !requestBounded || !requestDeadline.Equal(workflowDeadline) {
		t.Fatalf("download deadline = %v (bounded %v), want workflow deadline %v", requestDeadline, requestBounded, workflowDeadline)
	}
	if client.httpClient.Timeout != clientTimeout {
		t.Fatal("changed shared client timeout")
	}

	for _, unbounded := range []context.Context{context.Background(), nil} {
		before := time.Now()
		result, err := client.openTransactionTaxDownload(unbounded, origin, archiveURL)
		after := time.Now()
		if err != nil {
			t.Fatalf("download without workflow deadline: %v", err)
		}
		result.Body.Close()
		if !requestBounded || requestDeadline.Before(before.Add(clientTimeout)) || requestDeadline.After(after.Add(clientTimeout)) {
			t.Fatalf("unbounded caller lost the configured client timeout: deadline %v (bounded %v)", requestDeadline, requestBounded)
		}
	}

	expiredCtx, expiredCancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer expiredCancel()
	if result, err := client.openTransactionTaxDownload(expiredCtx, origin, archiveURL); err == nil {
		result.Body.Close()
		t.Fatal("download ignored workflow cancellation")
	}
}

func TestTransactionTaxDownloadDoesNotRetryReusedConnection(t *testing.T) {
	var primeCalls, downloadCalls int
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/prime":
			primeCalls++
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{}`)
		case "/download":
			downloadCalls++
			connection, _, err := http.NewResponseController(w).Hijack()
			if err != nil {
				t.Fatalf("hijack download connection: %v", err)
			}
			_ = connection.Close()
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := &Client{httpClient: server.Client(), baseURL: server.URL, minRequestInterval: 0}
	if _, err := client.transactionTaxJSON(context.Background(), "/prime"); err != nil {
		t.Fatalf("prime transactionTaxJSON() error: %v", err)
	}
	if primeCalls != 1 {
		t.Fatalf("prime calls = %d, want 1", primeCalls)
	}
	origin, err := client.transactionTaxOrigin()
	if err != nil {
		t.Fatalf("transactionTaxOrigin() error: %v", err)
	}
	_, err = client.openTransactionTaxDownload(context.Background(), origin, server.URL+"/download")
	if err == nil {
		t.Fatal("openTransactionTaxDownload() error = nil, want connection failure")
	}
	if downloadCalls != 1 {
		t.Fatalf("download calls = %d, want one attempt", downloadCalls)
	}
}

func TestTransactionTaxPreflightPreservesRedactedAuthStatus(t *testing.T) {
	for _, status := range []int{http.StatusUnauthorized, http.StatusForbidden} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			var calls int
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if !strings.HasSuffix(r.URL.Path, "/localization/keyValueMapping") {
					t.Errorf("request continued beyond authentication failure: %s", r.URL.Path)
				}
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("X-Apple-Request-UUID", "SECRET_REQUEST")
				w.Header().Set("X-Apple-Jingle-Correlation-Key", "SECRET_CORRELATION")
				w.WriteHeader(status)
				_, _ = io.WriteString(w, `{"errors":[{"code":"SECRET_CODE","detail":"SECRET_FINANCE"}]}`)
			}))
			defer server.Close()
			client := &Client{httpClient: server.Client(), baseURL: server.URL, minRequestInterval: 0}
			_, err := client.DownloadTransactionTaxReport(context.Background(), TransactionTaxReportRequest{ProviderID: 1234, Date: "2026-07"})
			var apiErr *APIError
			if !errors.As(err, &apiErr) || apiErr.Status != status {
				t.Fatalf("error = %v, want detectable auth status %d", err, status)
			}
			if calls != 1 || strings.Contains(err.Error(), "SECRET") || apiErr.AppleRequestID != "" || apiErr.CorrelationKey != "" || len(apiErr.rawBody) != 0 {
				t.Fatalf("auth failure must be redacted and stop before generation: calls=%d error=%v", calls, err)
			}
		})
	}
}

func TestTransactionTaxDownloadRejectsIneligibleMonthBeforeGeneration(t *testing.T) {
	generationCalls := 0
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/WebObjects/iTunesConnect.woa/ra/paymentConsolidation/localization/keyValueMapping":
			_, _ = io.WriteString(w, `{"data":{}}`)
		case "/WebObjects/iTunesConnect.woa/ra/paymentConsolidation/regionCountriesMapping":
			_, _ = io.WriteString(w, `{"data":{}}`)
		case "/WebObjects/iTunesConnect.woa/ra/paymentConsolidation/providers/1234/sapVendorNumbers":
			_, _ = io.WriteString(w, `{"data":[{"sapVendorNumber":42,"lastSoldUnit":100}]}`)
		case "/WebObjects/iTunesConnect.woa/ra/paymentConsolidation/providers/1234/sapVendorNumbers/42":
			_, _ = io.WriteString(w, `{"data":{"hasVendorTaxReport":false,"isArcadeVendor":false,"reportSummaries":[]}}`)
		case "/WebObjects/iTunesConnect.woa/ra/paymentConsolidation/providers/1234/sapVendorNumbers/42/reports":
			generationCalls++
			http.Error(w, "generation must not run", http.StatusInternalServerError)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := &Client{httpClient: server.Client(), baseURL: server.URL, minRequestInterval: 0}
	_, err := client.DownloadTransactionTaxReport(context.Background(), TransactionTaxReportRequest{ProviderID: 1234, Date: "2026-07"})
	if err == nil || !strings.Contains(err.Error(), "not available") {
		t.Fatalf("error = %v, want ineligible-period error", err)
	}
	if generationCalls != 0 {
		t.Fatalf("generation calls = %d, want 0", generationCalls)
	}
}

func TestTransactionTaxDownloadRejectsForeignDownloadURLWithoutLeakingQuery(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/WebObjects/iTunesConnect.woa/ra/paymentConsolidation/localization/keyValueMapping":
			_, _ = io.WriteString(w, `{"data":{}}`)
		case r.URL.Path == "/WebObjects/iTunesConnect.woa/ra/paymentConsolidation/regionCountriesMapping":
			_, _ = io.WriteString(w, `{"data":{}}`)
		case r.URL.Path == "/WebObjects/iTunesConnect.woa/ra/paymentConsolidation/providers/1234/sapVendorNumbers":
			_, _ = io.WriteString(w, `{"data":[{"sapVendorNumber":42,"lastSoldUnit":100}]}`)
		case r.URL.Path == "/WebObjects/iTunesConnect.woa/ra/paymentConsolidation/providers/1234/sapVendorNumbers/42":
			_, _ = io.WriteString(w, `{"data":{"hasVendorTaxReport":true,"isArcadeVendor":false,"reportSummaries":[{"proceedsByRegion":[{"regionCurrency":{"regionCurrencyId":13,"regionCode":"CA","regionName":"Alpha","regionNameKey":"region.a"},"financialReportType":"R","earned":"1"}]}]}}`)
		case r.URL.Path == "/WebObjects/iTunesConnect.woa/ra/paymentConsolidation/providers/1234/sapVendorNumbers/42/reports":
			_, _ = io.WriteString(w, `{"data":{"uuid":"job-123","estimatedWaitingTime":1}}`)
		case strings.HasSuffix(r.URL.Path, "/reports/job-123/status"):
			_, _ = io.WriteString(w, `{"data":{"status":"readyForDownload","downloadUrl":"https://evil.example/download?signature=SECRET"}}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := &Client{httpClient: server.Client(), baseURL: server.URL, minRequestInterval: 0}
	_, err := client.DownloadTransactionTaxReport(context.Background(), TransactionTaxReportRequest{ProviderID: 1234, Date: "2026-07"})
	if err == nil || strings.Contains(err.Error(), "SECRET") || strings.Contains(err.Error(), "evil.example") {
		t.Fatalf("error = %v, want redacted foreign-origin error", err)
	}
	if parsed, parseErr := url.Parse(server.URL); parseErr != nil || parsed.Scheme != "https" {
		t.Fatalf("test server URL = %q, want TLS origin", server.URL)
	}
}

func TestTransactionTaxGenerationRequiresUUIDAndEstimate(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{name: "missing uuid", body: `{"estimatedWaitingTime":1}`},
		{name: "missing estimate", body: `{"uuid":"job-123"}`},
		{name: "non numeric estimate", body: `{"uuid":"job-123","estimatedWaitingTime":"1"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			generationCalls := 0
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				generationCalls++
				if r.Method != http.MethodGet {
					t.Errorf("generation method = %s, want GET", r.Method)
				}
				_, _ = io.WriteString(w, tc.body)
			}))
			defer server.Close()
			client := &Client{httpClient: server.Client(), baseURL: server.URL, minRequestInterval: 0}
			_, err := client.generateTransactionTaxReport(context.Background(), 1234, 42, "2026", "7", []int64{13})
			if err == nil || !strings.Contains(err.Error(), "malformed") {
				t.Fatalf("error = %v, want malformed response error", err)
			}
			if generationCalls != 1 {
				t.Fatalf("generation calls = %d, want 1", generationCalls)
			}
		})
	}
}

func TestTransactionTaxStatusHTTPErrorIsUnknownWithoutRegeneration(t *testing.T) {
	statusCalls := 0
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		statusCalls++
		http.Error(w, "sensitive status body", http.StatusBadGateway)
	}))
	defer server.Close()
	client := &Client{httpClient: server.Client(), baseURL: server.URL, minRequestInterval: 0}
	_, _, err := client.pollTransactionTaxReport(context.Background(), 1234, 42, "job-123")
	if err == nil || !strings.Contains(err.Error(), "unknown") || !strings.Contains(err.Error(), "502") {
		t.Fatalf("error = %v, want redacted unknown HTTP 502 outcome", err)
	}
	if strings.Contains(err.Error(), "sensitive status body") || strings.Contains(err.Error(), "job-123") {
		t.Fatalf("error = %v, leaked status body or job id", err)
	}
	if statusCalls != 1 {
		t.Fatalf("status calls = %d, want 1", statusCalls)
	}
}

func TestTransactionTaxRegionDerivationRejectsMalformedEarnedValue(t *testing.T) {
	_, err := deriveTransactionTaxRegionIDs(transactionTaxMonth{
		Summaries: []transactionTaxReportSummary{{Proceeds: []transactionTaxProceed{{
			RegionCurrency: &transactionTaxRegionCurrency{
				ID:            13,
				RegionCode:    "CA",
				RegionNameKey: "region.a",
			},
			FinancialReportType: "R",
			Earned:              []byte(`"not-a-number"`),
		}}}},
	}, map[string]string{"region.a": "Alpha"})
	if err == nil || !strings.Contains(err.Error(), "malformed proceeds") {
		t.Fatalf("error = %v, want malformed proceeds error", err)
	}
}
