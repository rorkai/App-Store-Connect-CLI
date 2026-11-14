package devices

import (
	"bufio"
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"howett.net/plist"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func TestDeviceURLCallbackRegistersAndSkipsDuplicates(t *testing.T) {
	var posts int
	client := newDeviceURLTestClient(t, func(req *http.Request) *http.Response {
		switch {
		case req.Method == http.MethodGet && strings.Contains(req.URL.Path, "/devices"):
			return deviceURLJSON(http.StatusOK, `{"data":[]}`)
		case req.Method == http.MethodPost && req.URL.Path == "/v1/devices":
			posts++
			return deviceURLJSON(http.StatusCreated, `{"data":{"type":"devices","id":"dev-1","attributes":{"name":"iPhone","udid":"ABCDEF0123456789","platform":"IOS"}}}`)
		default:
			t.Fatalf("unexpected %s %s", req.Method, req.URL.Path)
			return nil
		}
	})
	server := &deviceURLServer{
		token:    "token-1",
		platform: "IOS",
		confirm:  true,
		client:   client,
		timeout:  shared.ContextWithTimeout,
		seen:     map[string]struct{}{},
	}
	body := []byte(`{"UDID":"ABCDEF0123456789","PRODUCT":"iPhone"}`)
	firstToken := mustDeviceCallbackToken(t, server)
	first := httptest.NewRecorder()
	server.callback(first, httptest.NewRequest(http.MethodPost, "/callback?token="+firstToken, strings.NewReader(string(body))))
	if first.Code != http.StatusOK {
		t.Fatalf("first status %d body %s", first.Code, first.Body.String())
	}
	second := httptest.NewRecorder()
	server.callback(second, httptest.NewRequest(http.MethodPost, "/callback?token="+mustDeviceCallbackToken(t, server), strings.NewReader(string(body))))
	if second.Code != http.StatusOK || !strings.Contains(second.Body.String(), `"skipped"`) {
		t.Fatalf("second status %d body %s", second.Code, second.Body.String())
	}
	if posts != 1 || len(server.devices) != 1 || server.devices[0].Status != "registered" {
		t.Fatalf("devices = %#v", server.devices)
	}
	for _, token := range []string{firstToken, "token-1", "other", ""} {
		unknown := httptest.NewRecorder()
		server.callback(unknown, httptest.NewRequest(http.MethodPost, "/callback?token="+token, strings.NewReader(string(body))))
		if unknown.Code != http.StatusForbidden {
			t.Fatalf("token %q status = %d", token, unknown.Code)
		}
	}
}

func TestDeviceURLCollectOnlyRoundTripsRegisterBatch(t *testing.T) {
	path := filepath.Join(t.TempDir(), "devices.tsv")
	records := []deviceBatchRecord{{UDID: "ABCDEF0123456789", Name: "iPhone abcdef01", Platform: "IOS"}}
	if err := writeDeviceRegistrationBatch(path, records); err != nil {
		t.Fatal(err)
	}
	parsed, err := readDeviceBatchTSV(newTestDeps(), path, "IOS")
	if err != nil {
		t.Fatal(err)
	}
	if len(parsed) != 1 || parsed[0].UDID != records[0].UDID || parsed[0].Name != records[0].Name {
		t.Fatalf("parsed = %#v", parsed)
	}
}

func TestDeviceURLListenStaysOnLoopback(t *testing.T) {
	path := filepath.Join(t.TempDir(), "devices.tsv")
	result, err := serveDeviceRegistration(context.Background(), newTestDeps(), deviceURLServeOptions{
		Listen:     "127.0.0.1:0",
		TTL:        20 * time.Millisecond,
		Platform:   "IOS",
		OutputFile: path,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.URL, "127.0.0.1:") {
		t.Fatalf("url = %s", result.URL)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
	if err := validateDeviceRegisterListen(newTestDeps(), "0.0.0.0:8080", false); err == nil {
		t.Fatal("expected public bind without explicit listen to be rejected")
	}
	if err := validateDeviceRegisterListen(newTestDeps(), "0.0.0.0:8080", true); err != nil {
		t.Fatalf("explicit public bind rejected: %v", err)
	}
}

func newDeviceURLTestClient(t *testing.T, fn func(*http.Request) *http.Response) *asc.Client {
	t.Helper()
	transport := roundTripperFunc(fn)
	client, err := asc.NewClientWithHTTPClient("KEY", "ISSUER", writeDeviceURLKey(t), &http.Client{Transport: transport})
	if err != nil {
		t.Fatal(err)
	}
	return client
}

func mustDeviceCallbackToken(t *testing.T, server *deviceURLServer) string {
	t.Helper()
	token, err := server.issueCallbackToken()
	if err != nil {
		t.Fatal(err)
	}
	return token
}

type roundTripperFunc func(*http.Request) *http.Response

func (fn roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req), nil
}

func deviceURLJSON(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func writeDeviceURLKey(t *testing.T) string {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "key.p8")
	if err := os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestDeviceURLSignedCallback(t *testing.T) {
	body, err := os.ReadFile("testdata/device-callback.p7")
	if err != nil {
		t.Fatal(err)
	}
	var posts int
	client := newDeviceURLTestClient(t, func(req *http.Request) *http.Response {
		if req.Method == http.MethodGet {
			return deviceURLJSON(http.StatusOK, `{"data":[]}`)
		}
		posts++
		payload, _ := io.ReadAll(req.Body)
		if !bytes.Contains(payload, []byte("00008110-001234567890001E")) {
			t.Errorf("missing signed UDID: %s", payload)
		}
		return deviceURLJSON(http.StatusCreated, `{"data":{"type":"devices","id":"dev-signed"}}`)
	})
	server := &deviceURLServer{token: "token", platform: "IOS", confirm: true, client: client, timeout: shared.ContextWithTimeout, seen: map[string]struct{}{}}
	req := httptest.NewRequest(http.MethodPost, "/callback?token="+mustDeviceCallbackToken(t, server), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/pkcs7-signature")
	response := httptest.NewRecorder()
	server.callback(response, req)
	if response.Code != http.StatusOK || posts != 1 {
		t.Fatalf("signed callback status=%d posts=%d body=%s", response.Code, posts, response.Body.String())
	}
}

func TestDeviceURLCallbackRetriesAfterAPIFailure(t *testing.T) {
	for _, failureMethod := range []string{http.MethodGet, http.MethodPost} {
		t.Run(failureMethod, func(t *testing.T) {
			failed := false
			posts := 0
			client := newDeviceURLTestClient(t, func(req *http.Request) *http.Response {
				if req.Method == failureMethod && !failed {
					failed = true
					return deviceURLJSON(http.StatusForbidden, `{"errors":[{"status":"403","code":"FORBIDDEN","title":"temporary credential failure"}]}`)
				}
				if req.Method == http.MethodGet {
					return deviceURLJSON(http.StatusOK, `{"data":[]}`)
				}
				posts++
				return deviceURLJSON(http.StatusCreated, `{"data":{"type":"devices","id":"dev-retry"}}`)
			})
			server := &deviceURLServer{token: "token", platform: "IOS", confirm: true, client: client, timeout: shared.ContextWithTimeout, seen: map[string]struct{}{}}
			token := mustDeviceCallbackToken(t, server)
			for attempt, step := range []struct {
				token      string
				udid       string
				wantStatus int
			}{
				{token, "ABCDEF0123456789", http.StatusBadGateway},
				// A failed token stays bound to its device and cannot switch UDIDs.
				{token, "FEDCBA9876543210", http.StatusForbidden},
				{token, "ABCDEF0123456789", http.StatusOK},
				{mustDeviceCallbackToken(t, server), "ABCDEF0123456789", http.StatusOK},
			} {
				response := httptest.NewRecorder()
				server.callback(response, httptest.NewRequest(http.MethodPost, "/callback?token="+step.token, strings.NewReader(`{"UDID":"`+step.udid+`","PRODUCT":"iPhone"}`)))
				if response.Code != step.wantStatus {
					t.Fatalf("attempt %d status=%d body=%s", attempt, response.Code, response.Body.String())
				}
			}
			if posts != 1 || len(server.devices) != 1 || server.devices[0].Status != "registered" {
				t.Fatalf("retry did not register: posts=%d devices=%#v", posts, server.devices)
			}
		})
	}
}

func TestDeviceURLCallbackRequestTimeout(t *testing.T) {
	t.Setenv("ASC_TIMEOUT", "20ms")
	t.Setenv("ASC_TIMEOUT_SECONDS", "")
	for _, stalledMethod := range []string{http.MethodGet, http.MethodPost} {
		t.Run(stalledMethod, func(t *testing.T) {
			client := newDeviceURLTestClient(t, func(req *http.Request) *http.Response {
				if req.Method != stalledMethod {
					return deviceURLJSON(http.StatusOK, `{"data":[]}`)
				}
				deadline, ok := req.Context().Deadline()
				if !ok || time.Until(deadline) > time.Second {
					t.Error("callback API request has no shared bounded timeout")
					return deviceURLJSON(http.StatusForbidden, `{"errors":[]}`)
				}
				<-req.Context().Done()
				return deviceURLJSON(http.StatusForbidden, `{"errors":[]}`)
			})
			server := &deviceURLServer{token: "token", platform: "IOS", confirm: true, client: client, timeout: shared.ContextWithTimeout, seen: map[string]struct{}{}}
			response := httptest.NewRecorder()
			server.callback(response, httptest.NewRequest(http.MethodPost, "/callback?token="+mustDeviceCallbackToken(t, server), strings.NewReader(`{"UDID":"ABCDEF0123456789"}`)))
			if response.Code != http.StatusBadGateway {
				t.Fatalf("status=%d", response.Code)
			}
		})
	}
}

func TestDeviceURLShutdownCancelsInFlightCallback(t *testing.T) {
	t.Setenv("ASC_TIMEOUT", "10s")
	t.Setenv("ASC_TIMEOUT_SECONDS", "")
	entered := make(chan struct{})
	canceled := make(chan struct{})
	release := make(chan struct{})
	defer close(release)
	client := newDeviceURLTestClient(t, func(req *http.Request) *http.Response {
		close(entered)
		select {
		case <-req.Context().Done():
			close(canceled)
		case <-release:
		}
		return deviceURLJSON(http.StatusForbidden, `{"errors":[]}`)
	})
	pageURL, cancel, done := startDeviceURLTestSession(t, client)
	callbackURL := deviceURLProfileCallback(t, pageURL)
	requestDone := make(chan struct{})
	go func() {
		defer close(requestDone)
		resp, err := http.Post(callbackURL, "application/json", strings.NewReader(`{"UDID":"ABC123"}`))
		if err == nil {
			resp.Body.Close()
		}
	}()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("callback did not reach API")
	}
	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("server did not shut down")
	}
	select {
	case <-canceled:
	default:
		t.Error("shutdown left callback API request running")
	}
}

func TestDeviceURLCallbackRejectsTSVInjection(t *testing.T) {
	for _, body := range []string{`{"UDID":"ABC\nOTHER"}`, `{"UDID":"ABC","PRODUCT":"Phone\tOTHER"}`} {
		server := &deviceURLServer{token: "token", platform: "IOS", seen: map[string]struct{}{}}
		response := httptest.NewRecorder()
		server.callback(response, httptest.NewRequest(http.MethodPost, "/callback?token="+mustDeviceCallbackToken(t, server), strings.NewReader(body)))
		if response.Code != http.StatusBadRequest || len(server.devices) != 0 {
			t.Errorf("accepted TSV injection: status=%d devices=%v", response.Code, server.devices)
		}
	}
}

func TestDeviceURLSignedCallbackRejectsTampering(t *testing.T) {
	body, err := os.ReadFile("testdata/device-callback.p7")
	if err != nil {
		t.Fatal(err)
	}
	body = bytes.Replace(body, []byte("001234567890001E"), []byte("001234567890001F"), 1)
	if _, _, err := parseDeviceRegistrationCallback(body); err == nil {
		t.Fatal("accepted tampered signed callback")
	}
}

func TestDeviceURLCallbackRejectsOversizedBody(t *testing.T) {
	body := `{"UDID":"ABC123"}` + strings.Repeat(" ", 1<<20)
	server := &deviceURLServer{token: "token", platform: "IOS", seen: map[string]struct{}{}}
	response := httptest.NewRecorder()
	server.callback(response, httptest.NewRequest(http.MethodPost, "/callback?token="+mustDeviceCallbackToken(t, server), strings.NewReader(body)))
	if response.Code != http.StatusRequestEntityTooLarge || len(server.devices) != 0 {
		t.Fatalf("accepted oversized callback: status=%d", response.Code)
	}
}

// startDeviceURLTestSession exercises the real listener, callback, and shutdown path.
func startDeviceURLTestSession(t *testing.T, client *asc.Client) (string, context.CancelFunc, <-chan error) {
	t.Helper()
	d, urls := newDeviceURLTestDeps(t, io.Discard)
	return startDeviceURLTestRunner(t, urls, func(ctx context.Context) error {
		_, err := serveDeviceRegistration(ctx, d, deviceURLServeOptions{Listen: "127.0.0.1:0", TTL: time.Minute, Platform: "IOS", Confirm: true, Client: client})
		return err
	})
}

// newDeviceURLTestDeps returns deps that write stdout to out and stderr to a
// pipe, and the page URLs a session prints to that stderr.
func newDeviceURLTestDeps(t *testing.T, out io.Writer) (*shared.Deps, <-chan string) {
	t.Helper()
	read, write, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { write.Close(); read.Close() })
	urls := make(chan string, 1)
	go func() {
		scanner := bufio.NewScanner(read)
		for scanner.Scan() {
			if strings.HasPrefix(scanner.Text(), "http://") {
				urls <- scanner.Text()
			}
		}
	}()
	return &shared.Deps{IO: &shared.IO{Out: out, ErrOut: write}, Timeout: shared.ContextWithTimeout}, urls
}

func startDeviceURLTestRunner(t *testing.T, urls <-chan string, run func(context.Context) error) (string, context.CancelFunc, <-chan error) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	done := make(chan error, 1)
	go func() {
		done <- run(ctx)
	}()
	select {
	case pageURL := <-urls:
		return pageURL, cancel, done
	case <-time.After(5 * time.Second):
		t.Fatal("server did not start")
		return "", cancel, done
	}
}

func TestDeviceURLSessionReportsUnrecoveredAPIFailure(t *testing.T) {
	for _, retry := range []bool{false, true} {
		t.Run(fmt.Sprint(retry), func(t *testing.T) {
			posts := 0
			client := newDeviceURLTestClient(t, func(req *http.Request) *http.Response {
				if req.Method == http.MethodGet {
					return deviceURLJSON(http.StatusOK, `{"data":[]}`)
				}
				posts++
				if posts == 2 {
					return deviceURLJSON(http.StatusForbidden, `{"errors":[{"status":"403","code":"FORBIDDEN","title":"registration failed"}]}`)
				}
				return deviceURLJSON(http.StatusCreated, `{"data":{"type":"devices","id":"created"}}`)
			})
			pageURL, cancel, done := startDeviceURLTestSession(t, client)
			ids := []string{"ABC123", "DEF456"}
			if retry {
				ids = append(ids, "DEF456")
			}
			for _, id := range ids {
				resp, err := http.Post(deviceURLProfileCallback(t, pageURL), "application/json", strings.NewReader(`{"UDID":"`+id+`"}`))
				if err != nil {
					t.Fatal(err)
				}
				_, copyErr := io.Copy(io.Discard, resp.Body)
				resp.Body.Close()
				if copyErr != nil {
					t.Fatal(copyErr)
				}
			}
			cancel()
			select {
			case err := <-done:
				if !retry && err == nil {
					t.Fatal("session hid failed registration after earlier success")
				}
				if retry && err != nil {
					t.Fatalf("successful retry did not clear failure: %v", err)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("server did not stop")
			}
		})
	}
}

func TestDeviceURLCommandHonorsName(t *testing.T) {
	d, urls := newDeviceURLTestDeps(t, io.Discard)
	cmd := newDevicesRegisterCommand(d)
	path := filepath.Join(t.TempDir(), "devices.tsv")
	if err := cmd.FlagSet.Parse([]string{"--via-url", "--name", `QA "Phone"`, "--output-file", path}); err != nil {
		t.Fatal(err)
	}
	pageURL, cancel, done := startDeviceURLTestRunner(t, urls, func(ctx context.Context) error { return cmd.Exec(ctx, nil) })
	resp, err := http.Post(deviceURLProfileCallback(t, pageURL), "application/json", strings.NewReader(`{"UDID":"ABC123","PRODUCT":"iPhone"}`))
	if err != nil {
		t.Fatal(err)
	}
	var record struct {
		Name string `json:"name"`
	}
	err = json.NewDecoder(resp.Body).Decode(&record)
	resp.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if record.Name != `QA "Phone"` {
		t.Fatalf("--name ignored: %q", record.Name)
	}
	rows, err := readDeviceBatchTSV(newTestDeps(), path, "IOS")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Name != record.Name {
		t.Fatalf("collected name did not round trip: %#v", rows)
	}
}

// deviceURLProfileCallback downloads a profile from the running session and
// returns the callback URL embedded in it, as an iOS device would use it.
func deviceURLProfileCallback(t *testing.T, pageURL string) string {
	t.Helper()
	resp, err := http.Get(strings.Replace(pageURL, "/enroll?", "/profile?", 1))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("profile status=%d body=%s", resp.StatusCode, body)
	}
	var profile struct {
		PayloadContent struct {
			URL string `plist:"URL"`
		} `plist:"PayloadContent"`
	}
	if _, err := plist.Unmarshal(body, &profile); err != nil {
		t.Fatal(err)
	}
	if profile.PayloadContent.URL == "" {
		t.Fatalf("profile has no callback URL: %s", body)
	}
	return profile.PayloadContent.URL
}

func postDeviceURLCallback(t *testing.T, callbackURL, udid string) (int, string) {
	t.Helper()
	resp, err := http.Post(callbackURL, "application/json", strings.NewReader(`{"UDID":"`+udid+`","PRODUCT":"iPhone"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return resp.StatusCode, string(body)
}

func TestDeviceURLCallbackTokensAreSingleUsePerProfile(t *testing.T) {
	posts := 0
	client := newDeviceURLTestClient(t, func(req *http.Request) *http.Response {
		if req.Method == http.MethodGet {
			return deviceURLJSON(http.StatusOK, `{"data":[]}`)
		}
		posts++
		return deviceURLJSON(http.StatusCreated, fmt.Sprintf(`{"data":{"type":"devices","id":"dev-%d"}}`, posts))
	})
	pageURL, cancel, done := startDeviceURLTestSession(t, client)
	sessionToken, err := url.Parse(pageURL)
	if err != nil {
		t.Fatal(err)
	}

	// The shared enrollment token no longer authorizes callbacks directly.
	if status, body := postDeviceURLCallback(t, strings.Replace(pageURL, "/enroll?", "/callback?", 1), "AAAA0000"); status != http.StatusForbidden {
		t.Fatalf("session token callback status=%d body=%s", status, body)
	}

	first := deviceURLProfileCallback(t, pageURL)
	second := deviceURLProfileCallback(t, pageURL)
	firstURL, err := url.Parse(first)
	if err != nil {
		t.Fatal(err)
	}
	callbackToken := firstURL.Query().Get("token")
	if callbackToken == "" || callbackToken == sessionToken.Query().Get("token") || first == second {
		t.Fatalf("profiles must carry distinct per-profile callback tokens: session=%s first=%s second=%s", pageURL, first, second)
	}
	if len(callbackToken) < 32 {
		t.Fatalf("callback token is too short to be unguessable: %q", callbackToken)
	}

	if status, body := postDeviceURLCallback(t, first, "ABC123"); status != http.StatusOK || !strings.Contains(body, `"registered"`) {
		t.Fatalf("first callback status=%d body=%s", status, body)
	}
	// Replaying a used callback URL cannot register another device.
	if status, body := postDeviceURLCallback(t, first, "DEF456"); status != http.StatusForbidden {
		t.Fatalf("replayed callback status=%d body=%s", status, body)
	}
	if status, body := postDeviceURLCallback(t, first, "ABC123"); status != http.StatusForbidden {
		t.Fatalf("replayed same-device callback status=%d body=%s", status, body)
	}
	// A second profile download still registers a second device.
	if status, body := postDeviceURLCallback(t, second, "DEF456"); status != http.StatusOK || !strings.Contains(body, `"registered"`) {
		t.Fatalf("second profile callback status=%d body=%s", status, body)
	}
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("server did not stop")
	}
	if posts != 2 {
		t.Fatalf("posts = %d, want 2", posts)
	}
}

func readDeviceURLStreamLine(t *testing.T, lines <-chan string) string {
	t.Helper()
	select {
	case line, ok := <-lines:
		if !ok {
			t.Fatal("stream closed before the next receipt")
		}
		return line
	case <-time.After(5 * time.Second):
		t.Fatal("no receipt streamed for the arrival")
		return ""
	}
}

func scanDeviceURLLines(r io.Reader) <-chan string {
	lines := make(chan string, 16)
	go func() {
		defer close(lines)
		scanner := bufio.NewScanner(r)
		for scanner.Scan() {
			lines <- scanner.Text()
		}
	}()
	return lines
}

func TestDeviceURLStreamsReceiptPerArrival(t *testing.T) {
	posts := 0
	client := newDeviceURLTestClient(t, func(req *http.Request) *http.Response {
		if req.Method == http.MethodGet {
			return deviceURLJSON(http.StatusOK, `{"data":[]}`)
		}
		posts++
		if posts == 2 {
			return deviceURLJSON(http.StatusForbidden, `{"errors":[{"status":"403","code":"FORBIDDEN","title":"registration failed"}]}`)
		}
		return deviceURLJSON(http.StatusCreated, `{"data":{"type":"devices","id":"dev-1"}}`)
	})
	streamRead, streamWrite := io.Pipe()
	t.Cleanup(func() { streamRead.Close() })
	lines := scanDeviceURLLines(streamRead)
	d, urls := newDeviceURLTestDeps(t, io.Discard)
	pageURL, cancel, done := startDeviceURLTestRunner(t, urls, func(ctx context.Context) error {
		defer streamWrite.Close()
		_, err := serveDeviceRegistration(ctx, d, deviceURLServeOptions{Listen: "127.0.0.1:0", TTL: time.Minute, Platform: "IOS", Confirm: true, Client: client, Stream: streamWrite})
		return err
	})

	for _, want := range []struct {
		udid, status, deviceID string
	}{
		{"ABC123", "registered", "dev-1"},
		{"DEF456", "failed", ""},
	} {
		postDeviceURLCallback(t, deviceURLProfileCallback(t, pageURL), want.udid)
		var record asc.DeviceURLRegistration
		line := readDeviceURLStreamLine(t, lines)
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			t.Fatalf("streamed receipt is not one JSON object per line: %q: %v", line, err)
		}
		if record.UDID != want.udid || record.Status != want.status || record.DeviceID != want.deviceID || record.Platform != "IOS" {
			t.Fatalf("streamed receipt = %#v, want %+v", record, want)
		}
		if want.status == "failed" && record.Error == "" {
			t.Fatalf("failed receipt has no error: %#v", record)
		}
	}
	// Rejected callbacks are not device arrivals and produce no receipt.
	if status, _ := postDeviceURLCallback(t, strings.Replace(pageURL, "/enroll?", "/callback?", 1), "GHI789"); status != http.StatusForbidden {
		t.Fatalf("session token callback status = %d", status)
	}
	cancel()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("session hid failed registration")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("server did not stop")
	}
	if extra, ok := <-lines; ok {
		t.Fatalf("unexpected extra streamed receipt: %q", extra)
	}
}

func TestDeviceURLCommandStreamsJSONLinesThenSummary(t *testing.T) {
	stdoutRead, stdoutWrite, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { stdoutWrite.Close(); stdoutRead.Close() })
	lines := scanDeviceURLLines(stdoutRead)

	d, urls := newDeviceURLTestDeps(t, stdoutWrite)
	cmd := newDevicesRegisterCommand(d)
	path := filepath.Join(t.TempDir(), "devices.tsv")
	if err := cmd.FlagSet.Parse([]string{"--via-url", "--stream", "--output", "json", "--output-file", path}); err != nil {
		t.Fatal(err)
	}
	pageURL, cancel, done := startDeviceURLTestRunner(t, urls, func(ctx context.Context) error { return cmd.Exec(ctx, nil) })
	postDeviceURLCallback(t, deviceURLProfileCallback(t, pageURL), "ABC123")

	// The arrival receipt reaches stdout while the session is still running.
	var record asc.DeviceURLRegistration
	if err := json.Unmarshal([]byte(readDeviceURLStreamLine(t, lines)), &record); err != nil {
		t.Fatal(err)
	}
	if record.UDID != "ABC123" || record.Status != "collected" {
		t.Fatalf("streamed receipt = %#v", record)
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	var summary asc.DeviceURLRegistrationResult
	if err := json.Unmarshal([]byte(readDeviceURLStreamLine(t, lines)), &summary); err != nil {
		t.Fatal(err)
	}
	if !summary.CollectOnly || len(summary.Devices) != 1 || summary.Devices[0] != record {
		t.Fatalf("final summary = %#v", summary)
	}
}

func TestDeviceURLProfileBoundsPendingCallbackTokens(t *testing.T) {
	server := &deviceURLServer{token: "token", platform: "IOS", publicURL: "http://127.0.0.1:1", seen: map[string]struct{}{}, callbackTokens: map[string]string{}}
	for i := 0; i < maxPendingDeviceCallbackTokens; i++ {
		server.callbackTokens[fmt.Sprint(i)] = ""
	}
	response := httptest.NewRecorder()
	server.profile(response, httptest.NewRequest(http.MethodGet, "/profile?token=token", nil))
	if response.Code != http.StatusServiceUnavailable || len(server.callbackTokens) != maxPendingDeviceCallbackTokens {
		t.Fatalf("status=%d pending=%d", response.Code, len(server.callbackTokens))
	}
}
