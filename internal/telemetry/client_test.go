package telemetry

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"
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

	Emit("asc builds list", "1.2.3", time.Millisecond, 0)
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

func TestEmitSpawnsWorkerAtMostOncePerCooldownWindow(t *testing.T) {
	clearContextEnv(t)
	setTelemetryTestHome(t)
	t.Setenv("ASC_TELEMETRY_DISABLED", "")
	t.Setenv("DO_NOT_TRACK", "")

	workerStarts := 0
	stubMaintenanceWorkerStart(t, func() error {
		workerStarts++
		return nil
	})

	Emit("asc builds list", "1.2.3", time.Millisecond, 0)
	if workerStarts != 1 {
		t.Fatalf("worker starts after first emit = %d, want 1", workerStarts)
	}
	Emit("asc builds list", "1.2.3", time.Millisecond, 0)
	if workerStarts != 1 {
		t.Fatalf("worker starts within cooldown = %d, want 1", workerStarts)
	}
	if records := readDefaultSpool(t); len(records) != 2 {
		t.Fatalf("spool records = %d, want 2 queued for the next eligible spawn", len(records))
	}

	store, err := defaultSpoolStore()
	if err != nil {
		t.Fatalf("defaultSpoolStore() error = %v", err)
	}
	expired := time.Now().Add(-workerSpawnCooldown - time.Minute)
	if err := os.Chtimes(workerSpawnMarkerPath(store), expired, expired); err != nil {
		t.Fatalf("age spawn marker: %v", err)
	}

	Emit("asc builds list", "1.2.3", time.Millisecond, 0)
	if workerStarts != 2 {
		t.Fatalf("worker starts after cooldown expired = %d, want 2", workerStarts)
	}
}

func TestEmitSpawnsWorkerDuringCooldownOnceBacklogReachesThreshold(t *testing.T) {
	clearContextEnv(t)
	setTelemetryTestHome(t)
	t.Setenv("ASC_TELEMETRY_DISABLED", "")
	t.Setenv("DO_NOT_TRACK", "")

	workerStarts := 0
	stubMaintenanceWorkerStart(t, func() error {
		workerStarts++
		return nil
	})

	store, err := defaultSpoolStore()
	if err != nil {
		t.Fatalf("defaultSpoolStore() error = %v", err)
	}
	for index := 0; index < workerSpawnSpoolThreshold-1; index++ {
		if err := store.append(testSpoolRecord(fmt.Sprintf("event-%02d", index))); err != nil {
			t.Fatalf("append backlog record %d: %v", index, err)
		}
	}
	markMaintenanceWorkerSpawn(workerSpawnMarkerPath(store), time.Now())

	Emit("asc builds list", "1.2.3", time.Millisecond, 0)
	if workerStarts != 1 {
		t.Fatalf("worker starts with backlog at threshold = %d, want 1 despite active cooldown", workerStarts)
	}
}

func TestEmitDoesNotWaitForBlockedSender(t *testing.T) {
	clearContextEnv(t)
	setTelemetryTestHome(t)
	t.Setenv("ASC_TELEMETRY_DISABLED", "")
	t.Setenv("DO_NOT_TRACK", "")
	t.Setenv(endpointEnvVar, "https://telemetry.example.test/events")

	originalClient := http.DefaultClient
	http.DefaultClient = &http.Client{
		Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			<-request.Context().Done()
			return nil, request.Context().Err()
		}),
	}
	t.Cleanup(func() { http.DefaultClient = originalClient })
	stubMaintenanceWorkerStart(t, func() error { return nil })

	start := time.Now()
	Emit("asc builds list", "1.2.3", time.Millisecond, 0)
	elapsed := time.Since(start)

	if elapsed >= 150*time.Millisecond {
		t.Fatalf("Emit() elapsed = %s, want foreground return before blocked network deadline", elapsed)
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

	Emit("asc builds list", "1.2.3", time.Millisecond, 0)
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

	Emit("asc builds list", "1.2.3", time.Millisecond, 0)
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

	Emit("asc builds list", "1.2.3", time.Millisecond, 0)
	records := readDefaultSpool(t)
	if len(records) != 1 {
		t.Fatalf("spool records = %d, want 1", len(records))
	}
	if got := records[0].Endpoint; got != "https://override.example.test/events" {
		t.Fatalf("spooled endpoint = %q, want override", got)
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

	Emit("asc builds list", "1.2.3", time.Millisecond, 0)
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
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(350 * time.Millisecond)
		w.WriteHeader(http.StatusAccepted)
	}))
	t.Cleanup(server.Close)

	originalClient := http.DefaultClient
	http.DefaultClient = server.Client()
	t.Cleanup(func() { http.DefaultClient = originalClient })

	t.Setenv("ASC_TIMEOUT", "")
	t.Setenv("ASC_TIMEOUT_SECONDS", "")
	setTelemetryTestHome(t)

	if err := sendHTTPEventToEndpoint(Event{}, server.URL); err != nil {
		t.Fatalf("sendHTTPEvent() error = %v, want slow collector response accepted", err)
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
