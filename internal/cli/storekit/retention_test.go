package storekit

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	storekitapi "github.com/rudrankriyam/App-Store-Connect-CLI/internal/storekit"
)

func TestReadPNGValidatesDocumentedImageRequirements(t *testing.T) {
	valid := writeTestPNG(t, image.Rect(0, 0, 3840, 160), color.NRGBA{R: 20, G: 30, B: 40, A: 255})
	if _, err := readPNG(valid, storekitapi.ImageSizeFull); err != nil {
		t.Fatalf("readPNG(valid) error = %v", err)
	}

	wrongSize := writeTestPNG(t, image.Rect(0, 0, 100, 100), color.NRGBA{A: 255})
	if _, err := readPNG(wrongSize, storekitapi.ImageSizeFull); err == nil || !strings.Contains(err.Error(), "3840 pixels wide") {
		t.Fatalf("readPNG(wrong size) error = %v", err)
	}

	transparent := writeTestPNG(t, image.Rect(0, 0, 1024, 1024), color.NRGBA{R: 20, A: 100})
	if _, err := readPNG(transparent, storekitapi.ImageSizeBulletPoint); err == nil || !strings.Contains(err.Error(), "must not contain transparency") {
		t.Fatalf("readPNG(transparent) error = %v", err)
	}
}

func TestPerformanceWaitFailsWhenAppleReportsFail(t *testing.T) {
	failed := &storekitapi.PerformanceTestResult{RequestID: "request-1", Result: "FAIL"}
	if err := performanceResultError(failed); err == nil || !strings.Contains(err.Error(), "request-1") {
		t.Fatalf("performanceResultError(FAIL) = %v", err)
	}
	passed := &storekitapi.PerformanceTestResult{RequestID: "request-2", Result: "PASS"}
	if err := performanceResultError(passed); err != nil {
		t.Fatalf("performanceResultError(PASS) = %v", err)
	}
}

func TestWaitForPerformancePollsOnDepsTimer(t *testing.T) {
	results := []string{"PENDING", "PENDING", "PASS"}
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/inApps/v1/messaging/performanceTest/result/request-1" {
			t.Errorf("path = %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"result":%q}`, results[min(int(requests.Add(1)), len(results))-1])
	}))
	defer server.Close()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	client, err := storekitapi.NewClient(storekitapi.Credentials{
		KeyID: "STOREKIT_KEY", IssuerID: "STOREKIT_ISSUER", BundleID: "com.example.app",
		PrivateKeyPEM: string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})),
	}, storekitapi.Sandbox, storekitapi.WithBaseURL(server.URL), storekitapi.WithHTTPClient(server.Client()))
	if err != nil {
		t.Fatal(err)
	}

	var waits []time.Duration
	d := &shared.Deps{
		Timeout: shared.ContextWithTimeout,
		After: func(delay time.Duration) <-chan time.Time {
			waits = append(waits, delay)
			fired := make(chan time.Time, 1)
			fired <- time.Now()
			return fired
		},
	}
	result, err := waitForPerformance(context.Background(), d, client, "request-1", 30*time.Second, time.Hour)
	if err != nil {
		t.Fatalf("waitForPerformance() error: %v", err)
	}
	if result.Result != "PASS" || result.RequestID != "request-1" {
		t.Fatalf("result = %+v, want PASS for request-1", result)
	}
	if want := []time.Duration{30 * time.Second, 30 * time.Second}; !reflect.DeepEqual(waits, want) {
		t.Fatalf("waits = %v, want %v", waits, want)
	}
	if got := requests.Load(); got != 3 {
		t.Fatalf("requests = %d, want 3", got)
	}
}

func writeTestPNG(t *testing.T, bounds image.Rectangle, fill color.NRGBA) string {
	t.Helper()
	img := image.NewNRGBA(bounds)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			img.SetNRGBA(x, y, fill)
		}
	}
	path := filepath.Join(t.TempDir(), "image.png")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(file, img); err != nil {
		_ = file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}
