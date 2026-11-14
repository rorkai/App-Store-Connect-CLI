package devices

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func deviceURLCallbackStatus(server *deviceURLServer, token, udid string) (int, string) {
	response := httptest.NewRecorder()
	server.callback(response, httptest.NewRequest(http.MethodPost, "/callback?token="+token, strings.NewReader(`{"UDID":"`+udid+`","PRODUCT":"iPhone"}`)))
	return response.Code, response.Body.String()
}

func decodeDeviceURLStream(t *testing.T, stream *bytes.Buffer) []asc.DeviceURLRegistration {
	t.Helper()
	var records []asc.DeviceURLRegistration
	for _, line := range strings.Split(strings.TrimSpace(stream.String()), "\n") {
		if line == "" {
			continue
		}
		var record asc.DeviceURLRegistration
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			t.Fatalf("stream line %q: %v", line, err)
		}
		records = append(records, record)
	}
	return records
}

func TestDeviceURLMaxDevicesEndsSessionAndRefusesLaterProfiles(t *testing.T) {
	var stream bytes.Buffer
	server := &deviceURLServer{
		token:      "token",
		platform:   "IOS",
		publicURL:  "http://127.0.0.1:1",
		seen:       map[string]struct{}{},
		stream:     &stream,
		maxDevices: 2,
		limitDone:  make(chan struct{}),
	}
	tokens := make([]string, 4)
	for i := range tokens {
		tokens[i] = mustDeviceCallbackToken(t, server)
	}
	if status, body := deviceURLCallbackStatus(server, tokens[0], "AAAA0001"); status != http.StatusOK || !strings.Contains(body, `"collected"`) {
		t.Fatalf("first callback status=%d body=%s", status, body)
	}
	// A duplicate UDID is skipped and does not count toward --max-devices.
	if status, body := deviceURLCallbackStatus(server, tokens[1], "aaaa-0001"); status != http.StatusOK || !strings.Contains(body, `"skipped"`) {
		t.Fatalf("duplicate callback status=%d body=%s", status, body)
	}
	select {
	case <-server.limitDone:
		t.Fatal("duplicate UDID counted toward --max-devices")
	default:
	}
	if status, body := deviceURLCallbackStatus(server, tokens[2], "BBBB0002"); status != http.StatusOK || !strings.Contains(body, `"collected"`) {
		t.Fatalf("second device callback status=%d body=%s", status, body)
	}
	select {
	case <-server.limitDone:
	default:
		t.Fatal("reaching --max-devices did not end the session")
	}
	// A profile downloaded before the cap can no longer add a device.
	if status, body := deviceURLCallbackStatus(server, tokens[3], "CCCC0003"); status != http.StatusGone {
		t.Fatalf("post-cap callback status=%d body=%s", status, body)
	}
	for _, path := range []string{"/profile?token=token", "/enroll?token=token"} {
		response := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, path, nil)
		if strings.HasPrefix(path, "/profile") {
			server.profile(response, request)
		} else {
			server.enroll(response, request)
		}
		if response.Code != http.StatusGone || response.Header().Get("Content-Type") == "application/x-apple-aspen-config" {
			t.Fatalf("%s after cap: status=%d body=%s", path, response.Code, response.Body.String())
		}
	}
	if len(server.devices) != 2 || len(server.outputRows) != 2 || len(server.failures) != 0 {
		t.Fatalf("devices=%#v rows=%#v failures=%#v", server.devices, server.outputRows, server.failures)
	}
	records := decodeDeviceURLStream(t, &stream)
	var statuses []string
	for _, record := range records {
		statuses = append(statuses, record.UDID+"="+record.Status)
	}
	if got, want := strings.Join(statuses, ","), "AAAA0001=collected,aaaa-0001=skipped,BBBB0002=collected"; got != want {
		t.Fatalf("stream = %s, want %s", got, want)
	}
}

func TestDeviceURLMaxDevicesIgnoresDevicesAlreadyInAppStoreConnect(t *testing.T) {
	posts := 0
	client := newDeviceURLTestClient(t, func(req *http.Request) *http.Response {
		if req.Method == http.MethodGet {
			return deviceURLJSON(http.StatusOK, `{"data":[{"type":"devices","id":"old","attributes":{"name":"Old","udid":"EXIST0001","platform":"IOS"}}]}`)
		}
		posts++
		return deviceURLJSON(http.StatusCreated, `{"data":{"type":"devices","id":"new"}}`)
	})
	server := &deviceURLServer{
		token:      "token",
		platform:   "IOS",
		confirm:    true,
		client:     client,
		timeout:    shared.ContextWithTimeout,
		publicURL:  "http://127.0.0.1:1",
		seen:       map[string]struct{}{},
		maxDevices: 1,
		limitDone:  make(chan struct{}),
	}
	if status, body := deviceURLCallbackStatus(server, mustDeviceCallbackToken(t, server), "EXIST0001"); status != http.StatusOK || !strings.Contains(body, `"skipped"`) {
		t.Fatalf("existing device status=%d body=%s", status, body)
	}
	select {
	case <-server.limitDone:
		t.Fatal("device already registered in App Store Connect counted toward --max-devices")
	default:
	}
	if status, body := deviceURLCallbackStatus(server, mustDeviceCallbackToken(t, server), "NEW00001"); status != http.StatusOK || !strings.Contains(body, `"registered"`) {
		t.Fatalf("new device status=%d body=%s", status, body)
	}
	select {
	case <-server.limitDone:
	default:
		t.Fatal("registered device did not reach --max-devices")
	}
	if posts != 1 {
		t.Fatalf("posts = %d, want 1", posts)
	}
}

func TestDeviceURLMaxDevicesSessionExitsWithoutWaitingForTTL(t *testing.T) {
	posts := 0
	client := newDeviceURLTestClient(t, func(req *http.Request) *http.Response {
		if req.Method == http.MethodGet {
			return deviceURLJSON(http.StatusOK, `{"data":[]}`)
		}
		posts++
		return deviceURLJSON(http.StatusCreated, `{"data":{"type":"devices","id":"dev-1"}}`)
	})
	results := make(chan *asc.DeviceURLRegistrationResult, 1)
	d, urls := newDeviceURLTestDeps(t, io.Discard)
	pageURL, _, done := startDeviceURLTestRunner(t, urls, func(ctx context.Context) error {
		result, err := serveDeviceRegistration(ctx, d, deviceURLServeOptions{Listen: "127.0.0.1:0", TTL: time.Hour, Platform: "IOS", Confirm: true, Client: client, MaxDevices: 1})
		results <- result
		return err
	})
	early := deviceURLProfileCallback(t, pageURL)
	if status, body := postDeviceURLCallback(t, deviceURLProfileCallback(t, pageURL), "ABC123"); status != http.StatusOK || !strings.Contains(body, `"registered"`) {
		t.Fatalf("callback status=%d body=%s", status, body)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("session kept running after --max-devices was reached")
	}
	result := <-results
	if len(result.Devices) != 1 || result.Devices[0].Status != "registered" || len(result.Failures) != 0 {
		t.Fatalf("summary = %#v", result)
	}
	// The listener is gone, so a profile downloaded before the cap cannot register.
	if resp, err := http.Post(early, "application/json", strings.NewReader(`{"UDID":"DEF456"}`)); err == nil {
		resp.Body.Close()
		t.Fatalf("post-session callback reached the server: %d", resp.StatusCode)
	}
	if posts != 1 {
		t.Fatalf("posts = %d, want 1", posts)
	}
}

func TestDeviceURLCommandMaxDevicesStreamsThenSummary(t *testing.T) {
	stdoutRead, stdoutWrite, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { stdoutWrite.Close(); stdoutRead.Close() })
	lines := scanDeviceURLLines(stdoutRead)

	d, urls := newDeviceURLTestDeps(t, stdoutWrite)
	cmd := newDevicesRegisterCommand(d)
	path := filepath.Join(t.TempDir(), "devices.tsv")
	if err := cmd.FlagSet.Parse([]string{"--via-url", "--max-devices", "2", "--stream", "--output", "json", "--output-file", path, "--ttl", "1h"}); err != nil {
		t.Fatal(err)
	}
	pageURL, _, done := startDeviceURLTestRunner(t, urls, func(ctx context.Context) error { return cmd.Exec(ctx, nil) })
	for _, udid := range []string{"ABC123", "ABC123", "DEF456"} {
		if status, body := postDeviceURLCallback(t, deviceURLProfileCallback(t, pageURL), udid); status != http.StatusOK {
			t.Fatalf("callback %s status=%d body=%s", udid, status, body)
		}
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("session kept running after --max-devices was reached")
	}
	var statuses []string
	for range 3 {
		var record asc.DeviceURLRegistration
		if err := json.Unmarshal([]byte(readDeviceURLStreamLine(t, lines)), &record); err != nil {
			t.Fatal(err)
		}
		statuses = append(statuses, record.UDID+"="+record.Status)
	}
	if got, want := strings.Join(statuses, ","), "ABC123=collected,ABC123=skipped,DEF456=collected"; got != want {
		t.Fatalf("stream = %s, want %s", got, want)
	}
	var summary asc.DeviceURLRegistrationResult
	if err := json.Unmarshal([]byte(readDeviceURLStreamLine(t, lines)), &summary); err != nil {
		t.Fatal(err)
	}
	if !summary.CollectOnly || len(summary.Devices) != 2 || summary.Devices[0].UDID != "ABC123" || summary.Devices[1].UDID != "DEF456" {
		t.Fatalf("final summary = %#v", summary)
	}
	rows, err := readDeviceBatchTSV(newTestDeps(), path, "IOS")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("collected rows = %#v", rows)
	}
}

func TestDeviceURLCommandMaxDevicesValidation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "devices.tsv")
	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"--via-url", "--output-file", path, "--max-devices", "0"}, "--max-devices must be at least 1"},
		{[]string{"--via-url", "--output-file", path, "--max-devices", "-3"}, "--max-devices must be at least 1"},
		{[]string{"--name", "x", "--udid", "ABC", "--platform", "IOS", "--max-devices", "2"}, "--max-devices requires --via-url"},
	} {
		t.Run(strings.Join(tc.args, " "), func(t *testing.T) {
			cmd := newDevicesRegisterCommand(newTestDeps())
			if err := cmd.FlagSet.Parse(tc.args); err != nil {
				t.Fatal(err)
			}
			err := cmd.Exec(context.Background(), nil)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want %q", err, tc.want)
			}
			if !errors.Is(err, flag.ErrHelp) {
				t.Fatalf("err %v is not a usage error (exit 2)", err)
			}
		})
	}
}
