package telemetry

import (
	"context"
	"errors"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
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

func TestEmitIsEnabledByDefaultAndSwallowsSenderErrors(t *testing.T) {
	clearContextEnv(t)
	setTelemetryTestHome(t)
	t.Setenv("ASC_TELEMETRY_DISABLED", "")
	t.Setenv("DO_NOT_TRACK", "")

	called := false
	original := sendHTTP
	sendHTTP = func(ev Event) error {
		called = true
		if ev.CommandPath != "asc builds list" {
			t.Fatalf("CommandPath = %q", ev.CommandPath)
		}
		return errors.New("network down")
	}
	t.Cleanup(func() { sendHTTP = original })

	Emit("asc builds list", "1.2.3", time.Millisecond, 0)
	if !called {
		t.Fatal("expected sender to be called")
	}
}

func TestEmitHonorsDisabledEnv(t *testing.T) {
	clearContextEnv(t)
	setTelemetryTestHome(t)
	t.Setenv("ASC_TELEMETRY_DISABLED", "1")

	original := sendHTTP
	sendHTTP = func(ev Event) error {
		t.Fatal("sender should not be called when disabled")
		return nil
	}
	t.Cleanup(func() { sendHTTP = original })

	Emit("asc builds list", "1.2.3", time.Millisecond, 0)
}

func TestEmitMarksEphemeralRuntimeWithoutDisabling(t *testing.T) {
	clearContextEnv(t)
	setTelemetryTestHome(t)
	t.Setenv("ASC_TELEMETRY_DISABLED", "")
	t.Setenv("DO_NOT_TRACK", "")
	t.Setenv(telemetryEphemeralEnvVar, "1")
	t.Setenv("PI_CODING_AGENT", "true")

	called := false
	original := sendHTTP
	sendHTTP = func(ev Event) error {
		called = true
		if ev.RuntimeContext != RuntimeEphemeral {
			t.Fatalf("RuntimeContext = %q, want %q", ev.RuntimeContext, RuntimeEphemeral)
		}
		if ev.InvocationSource != SourcePi {
			t.Fatalf("InvocationSource = %q, want %q", ev.InvocationSource, SourcePi)
		}
		if ev.InstallID != nil {
			t.Fatalf("expected nil install ID, got %q", *ev.InstallID)
		}
		return nil
	}
	t.Cleanup(func() { sendHTTP = original })

	Emit("asc builds list", "1.2.3", time.Millisecond, 0)
	if !called {
		t.Fatal("expected sender to be called for ephemeral runtime")
	}
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
	err := sendHTTPEvent(Event{})
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

func TestSendHTTPEventCapsTelemetryTimeout(t *testing.T) {
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
	err := sendHTTPEvent(Event{})
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected request timeout")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("sendHTTPEvent() error = %v, want context deadline exceeded", err)
	}
	if elapsed >= 750*time.Millisecond {
		t.Fatalf("sendHTTPEvent() elapsed = %s, want telemetry timeout cap before 750ms", elapsed)
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

	if err := sendHTTPEvent(Event{}); err == nil {
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

	if err := sendHTTPEvent(Event{}); err == nil {
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

	if err := sendHTTPEvent(Event{}); err != nil {
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

	if err := sendHTTPEvent(Event{}); err == nil {
		t.Fatal("expected credential-bearing telemetry endpoint to be rejected")
	}
	if called {
		t.Fatal("telemetry sender contacted a credential-bearing endpoint")
	}
}
