package telemetry

import (
	"context"
	"errors"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"testing"
	"testing/synctest"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

func TestDefaultEndpointUsesExistingRorkAPIRoute(t *testing.T) {
	const want = "https://rork.com/cf-api/asc/v1/events"
	if DefaultEndpoint != want {
		t.Fatalf("DefaultEndpoint = %q, want %q", DefaultEndpoint, want)
	}
}

func TestDebugEnabledTreatsFalsyValuesAsDisabled(t *testing.T) {
	for _, value := range []string{"", "0", "f", "false", "no", "n", "off", "OFF"} {
		t.Setenv("ASC_DEBUG", value)
		if debugEnabled() {
			t.Fatalf("debugEnabled() = true for ASC_DEBUG=%q", value)
		}
	}
	t.Setenv("ASC_DEBUG", "1")
	if !debugEnabled() {
		t.Fatal("debugEnabled() = false for ASC_DEBUG=1")
	}
}

func TestEmitQueuesEventAndSwallowsWorkerStartErrors(t *testing.T) {
	clearContextEnv(t)
	setTelemetryTestHome(t)
	t.Setenv("ASC_TELEMETRY_DISABLED", "")
	t.Setenv("DO_NOT_TRACK", "")

	workerStarted := false
	stubMaintenanceWorkerStart(t, func() error {
		workerStarted = true
		return errors.New("process unavailable")
	})

	EmitWithContext("asc builds list", "1.2.3", time.Millisecond, 0, EventContext{InvocationShape: InvocationShapeLeaf})
	if !workerStarted {
		t.Fatal("expected maintenance worker start")
	}

	records := readDefaultSpool(t)
	if len(records) != 1 {
		t.Fatalf("spool records = %d, want 1", len(records))
	}
	if records[0].Event.CommandPath != "asc builds list" {
		t.Fatalf("CommandPath = %q, want %q", records[0].Event.CommandPath, "asc builds list")
	}
}

func TestEmitQueuesEventWithoutForegroundHTTPDelivery(t *testing.T) {
	clearContextEnv(t)
	setTelemetryTestHome(t)
	t.Setenv("ASC_TELEMETRY_DISABLED", "")
	t.Setenv("DO_NOT_TRACK", "")
	t.Setenv(endpointEnvVar, "https://telemetry.example.test/events")

	transportCalls := 0
	originalClient := http.DefaultClient
	http.DefaultClient = &http.Client{
		Transport: roundTripFunc(func(_ *http.Request) (*http.Response, error) {
			transportCalls++
			return nil, errors.New("unexpected foreground telemetry request")
		}),
	}
	t.Cleanup(func() { http.DefaultClient = originalClient })
	workerStarted := false
	stubMaintenanceWorkerStart(t, func() error {
		workerStarted = true
		return nil
	})

	EmitWithContext("asc builds list", "1.2.3", time.Millisecond, 0, EventContext{InvocationShape: InvocationShapeLeaf})

	if transportCalls != 0 {
		t.Fatalf("foreground HTTP transport calls = %d, want 0", transportCalls)
	}
	if !workerStarted {
		t.Fatal("expected maintenance worker start")
	}
	records := readDefaultSpool(t)
	if len(records) != 1 {
		t.Fatalf("spool records = %d, want 1", len(records))
	}
	if got := records[0].Endpoint; got != "https://telemetry.example.test/events" {
		t.Fatalf("spooled endpoint = %q, want endpoint override", got)
	}
}

func TestEmitHonorsDisabledEnv(t *testing.T) {
	clearContextEnv(t)
	setTelemetryTestHome(t)
	t.Setenv("ASC_TELEMETRY_DISABLED", "1")

	stubMaintenanceWorkerStart(t, func() error {
		t.Fatal("worker should not start when telemetry is disabled")
		return nil
	})

	EmitWithContext("asc builds list", "1.2.3", time.Millisecond, 0, EventContext{InvocationShape: InvocationShapeLeaf})
	if records := readDefaultSpool(t); len(records) != 0 {
		t.Fatalf("spool records = %d, want 0 when disabled", len(records))
	}
}

func TestEmitMarksEphemeralRuntimeWithoutDisabling(t *testing.T) {
	clearContextEnv(t)
	setTelemetryTestHome(t)
	t.Setenv("ASC_TELEMETRY_DISABLED", "")
	t.Setenv("DO_NOT_TRACK", "")
	t.Setenv(telemetryEphemeralEnvVar, "1")
	t.Setenv("PI_CODING_AGENT", "true")

	stubMaintenanceWorkerStart(t, func() error {
		return nil
	})

	EmitWithContext("asc builds list", "1.2.3", time.Millisecond, 0, EventContext{InvocationShape: InvocationShapeLeaf})
	records := readDefaultSpool(t)
	if len(records) != 1 {
		t.Fatalf("spool records = %d, want 1", len(records))
	}
	ev := records[0].Event
	if ev.RuntimeContext != RuntimeEphemeral {
		t.Fatalf("RuntimeContext = %q, want %q", ev.RuntimeContext, RuntimeEphemeral)
	}
	if ev.InvocationSource != SourcePi {
		t.Fatalf("InvocationSource = %q, want %q", ev.InvocationSource, SourcePi)
	}
	if ev.InstallID != nil {
		t.Fatalf("expected nil install ID, got %q", *ev.InstallID)
	}
}

func TestEmitCapturesEndpointOverrideInSpool(t *testing.T) {
	clearContextEnv(t)
	setTelemetryTestHome(t)
	t.Setenv("ASC_TELEMETRY_DISABLED", "")
	t.Setenv("DO_NOT_TRACK", "")
	t.Setenv(endpointEnvVar, "https://override.example.test/events")
	stubMaintenanceWorkerStart(t, func() error { return nil })

	EmitWithContext("asc builds list", "1.2.3", time.Millisecond, 0, EventContext{InvocationShape: InvocationShapeLeaf})
	records := readDefaultSpool(t)
	if len(records) != 1 {
		t.Fatalf("spool records = %d, want 1", len(records))
	}
	if got := records[0].Endpoint; got != "https://override.example.test/events" {
		t.Fatalf("spooled endpoint = %q, want override", got)
	}
}

func TestEmitSkipsDefaultEndpointForUnstampedBuild(t *testing.T) {
	const unstampedVersion = "dev (commit: unknown, date: unknown)"
	tests := []struct {
		name        string
		endpoint    string
		wantRecords int
	}{
		{name: "default endpoint", endpoint: "", wantRecords: 0},
		{name: "endpoint override", endpoint: "https://override.example.test/events", wantRecords: 1},
		{name: "explicit default endpoint", endpoint: DefaultEndpoint, wantRecords: 1},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			clearContextEnv(t)
			setTelemetryTestHome(t)
			t.Setenv("ASC_TELEMETRY_DISABLED", "")
			t.Setenv("DO_NOT_TRACK", "")
			t.Setenv(endpointEnvVar, test.endpoint)
			stubMaintenanceWorkerStart(t, func() error { return nil })

			EmitWithContext("asc workflow run", unstampedVersion, time.Millisecond, 0, EventContext{InvocationShape: InvocationShapeLeaf})
			if records := readDefaultSpool(t); len(records) != test.wantRecords {
				t.Fatalf("spool records = %d, want %d", len(records), test.wantRecords)
			}
		})
	}
}

func TestEmitDoesNotSpoolCredentialBearingEndpoint(t *testing.T) {
	clearContextEnv(t)
	setTelemetryTestHome(t)
	t.Setenv("ASC_TELEMETRY_DISABLED", "")
	t.Setenv("DO_NOT_TRACK", "")
	t.Setenv(endpointEnvVar, "https://user:secret@telemetry.example.test/events")
	stubMaintenanceWorkerStart(t, func() error {
		t.Fatal("worker should not start for invalid endpoint")
		return nil
	})

	EmitWithContext("asc builds list", "1.2.3", time.Millisecond, 0, EventContext{InvocationShape: InvocationShapeLeaf})
	if records := readDefaultSpool(t); len(records) != 0 {
		t.Fatalf("spool records = %d, want 0 for invalid endpoint", len(records))
	}
}

func stubMaintenanceWorkerStart(t *testing.T, start func() error) {
	t.Helper()
	original := startMaintenanceWorker
	startMaintenanceWorker = start
	t.Cleanup(func() { startMaintenanceWorker = original })
}

func readDefaultSpool(t *testing.T) []spoolRecord {
	t.Helper()
	store, err := defaultSpoolStore()
	if err != nil {
		t.Fatalf("defaultSpoolStore() error = %v", err)
	}
	records, err := store.snapshot(0)
	if err != nil {
		t.Fatalf("snapshot() error = %v", err)
	}
	return records
}

func TestSendHTTPEventHonorsASCTimeout(t *testing.T) {
	synctest.Test(t, testSendHTTPEventHonorsASCTimeout)
}

func testSendHTTPEventHonorsASCTimeout(t *testing.T) {
	originalClient := http.DefaultClient
	http.DefaultClient = &http.Client{
		Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			<-request.Context().Done()
			return nil, request.Context().Err()
		}),
	}
	t.Cleanup(func() { http.DefaultClient = originalClient })

	t.Setenv(endpointEnvVar, "https://telemetry.example.test/events")
	t.Setenv("ASC_TIMEOUT", "20ms")
	t.Setenv("ASC_TIMEOUT_SECONDS", "")
	setTelemetryTestHome(t)

	start := time.Now()
	err := sendHTTPEventToEndpoint(Event{}, endpoint())
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected request timeout")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("sendHTTPEvent() error = %v, want context deadline exceeded", err)
	}
	if elapsed >= 200*time.Millisecond {
		t.Fatalf("sendHTTPEvent() elapsed = %s, want ASC_TIMEOUT to stop it before 200ms", elapsed)
	}
}

func TestSendHTTPEventHonorsConfiguredTimeoutBelowCap(t *testing.T) {
	synctest.Test(t, testSendHTTPEventHonorsConfiguredTimeoutBelowCap)
}

func testSendHTTPEventHonorsConfiguredTimeoutBelowCap(t *testing.T) {
	originalClient := http.DefaultClient
	http.DefaultClient = &http.Client{
		Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			<-request.Context().Done()
			return nil, request.Context().Err()
		}),
	}
	t.Cleanup(func() { http.DefaultClient = originalClient })

	t.Setenv(endpointEnvVar, "https://telemetry.example.test/events")
	t.Setenv("ASC_TIMEOUT", "1s")
	t.Setenv("ASC_TIMEOUT_SECONDS", "")
	setTelemetryTestHome(t)

	start := time.Now()
	err := sendHTTPEventToEndpoint(Event{}, endpoint())
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected request timeout")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("sendHTTPEvent() error = %v, want context deadline exceeded", err)
	}
	if elapsed < 750*time.Millisecond || elapsed >= 1500*time.Millisecond {
		t.Fatalf("sendHTTPEvent() elapsed = %s, want configured timeout near 1s", elapsed)
	}
}

func TestSendHTTPEventAllowsSlowCollectorResponse(t *testing.T) {
	originalClient := http.DefaultClient
	http.DefaultClient = &http.Client{
		Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			select {
			case <-request.Context().Done():
				return nil, request.Context().Err()
			case <-time.After(350 * time.Millisecond):
				return &http.Response{StatusCode: http.StatusAccepted, Body: http.NoBody, Request: request}, nil
			}
		}),
	}
	t.Cleanup(func() { http.DefaultClient = originalClient })

	t.Setenv("ASC_TIMEOUT", "")
	t.Setenv("ASC_TIMEOUT_SECONDS", "")
	setTelemetryTestHome(t)

	var err error
	synctest.Test(t, func(*testing.T) {
		err = sendHTTPEventToEndpoint(Event{}, "https://telemetry.example.test/events")
	})
	if err != nil {
		t.Fatalf("sendHTTPEvent() error = %v, want slow collector response accepted", err)
	}
}

func TestSendHTTPEventClassifiesPermanentCollectorRejections(t *testing.T) {
	tests := []struct {
		name          string
		statusCode    int
		wantPermanent bool
	}{
		{name: "bad request", statusCode: http.StatusBadRequest, wantPermanent: true},
		{name: "unprocessable event", statusCode: http.StatusUnprocessableEntity, wantPermanent: true},
		{name: "request timeout", statusCode: http.StatusRequestTimeout, wantPermanent: false},
		{name: "rate limited", statusCode: http.StatusTooManyRequests, wantPermanent: false},
		{name: "server error", statusCode: http.StatusInternalServerError, wantPermanent: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(test.statusCode)
			}))
			t.Cleanup(server.Close)

			originalClient := http.DefaultClient
			http.DefaultClient = server.Client()
			t.Cleanup(func() { http.DefaultClient = originalClient })

			err := sendHTTPEventToEndpoint(Event{}, server.URL)
			if err == nil {
				t.Fatalf("sendHTTPEvent() error = nil for status %d", test.statusCode)
			}
			if got := isPermanentDeliveryError(err); got != test.wantPermanent {
				t.Fatalf("isPermanentDeliveryError(%v) = %t, want %t", err, got, test.wantPermanent)
			}
		})
	}
}

func TestSendHTTPEventRejectsPlaintextRedirect(t *testing.T) {
	plaintextHit := false
	plaintextServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		plaintextHit = true
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(plaintextServer.Close)

	secureServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		http.Redirect(w, request, plaintextServer.URL, http.StatusTemporaryRedirect)
	}))
	t.Cleanup(secureServer.Close)

	originalClient := http.DefaultClient
	http.DefaultClient = secureServer.Client()
	t.Cleanup(func() { http.DefaultClient = originalClient })

	t.Setenv(endpointEnvVar, secureServer.URL)
	setTelemetryTestHome(t)

	if err := sendHTTPEventToEndpoint(Event{}, endpoint()); err == nil {
		t.Fatal("expected plaintext redirect to be rejected")
	}
	if plaintextHit {
		t.Fatal("telemetry request followed a redirect to plaintext HTTP")
	}
}

func TestSendHTTPEventRejectsHTTPSRedirect(t *testing.T) {
	redirectedPathHit := false
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/redirected" {
			redirectedPathHit = true
			w.WriteHeader(http.StatusNoContent)
			return
		}
		http.Redirect(w, request, "/redirected", http.StatusTemporaryRedirect)
	}))
	t.Cleanup(server.Close)

	originalClient := http.DefaultClient
	http.DefaultClient = server.Client()
	t.Cleanup(func() { http.DefaultClient = originalClient })

	t.Setenv(endpointEnvVar, server.URL+"/events")
	setTelemetryTestHome(t)

	if err := sendHTTPEventToEndpoint(Event{}, endpoint()); err == nil {
		t.Fatal("expected HTTPS redirect to be rejected")
	}
	if redirectedPathHit {
		t.Fatal("telemetry request followed an HTTPS redirect")
	}
}

func TestSendHTTPEventDoesNotSendAmbientCookies(t *testing.T) {
	var receivedCookie string
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		receivedCookie = request.Header.Get("Cookie")
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(server.Close)

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookiejar.New() error = %v", err)
	}
	serverURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("url.Parse() error = %v", err)
	}
	jar.SetCookies(serverURL, []*http.Cookie{{Name: "session", Value: "ambient-secret"}})

	originalClient := http.DefaultClient
	client := server.Client()
	client.Jar = jar
	http.DefaultClient = client
	t.Cleanup(func() { http.DefaultClient = originalClient })

	t.Setenv(endpointEnvVar, server.URL+"/events")
	setTelemetryTestHome(t)

	if err := sendHTTPEventToEndpoint(Event{}, endpoint()); err != nil {
		t.Fatalf("sendHTTPEvent() error = %v", err)
	}
	if receivedCookie != "" {
		t.Fatalf("telemetry request sent ambient cookie %q", receivedCookie)
	}
}

func TestSendHTTPEventRejectsCredentialBearingEndpoint(t *testing.T) {
	called := false
	originalClient := http.DefaultClient
	http.DefaultClient = &http.Client{
		Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			called = true
			return &http.Response{StatusCode: http.StatusNoContent, Body: http.NoBody}, nil
		}),
	}
	t.Cleanup(func() { http.DefaultClient = originalClient })

	t.Setenv(endpointEnvVar, "https://user:secret@telemetry.example.test/events")
	setTelemetryTestHome(t)

	if err := sendHTTPEventToEndpoint(Event{}, endpoint()); err == nil {
		t.Fatal("expected credential-bearing telemetry endpoint to be rejected")
	}
	if called {
		t.Fatal("telemetry sender contacted a credential-bearing endpoint")
	}
}
