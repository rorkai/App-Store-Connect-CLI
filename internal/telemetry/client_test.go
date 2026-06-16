package telemetry

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

func TestEmitSwallowsSenderErrors(t *testing.T) {
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

	Emit([]string{"builds", "list"}, "asc builds list", "1.2.3", time.Millisecond, 0)
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

	Emit([]string{"builds", "list"}, "asc builds list", "1.2.3", time.Millisecond, 0)
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
