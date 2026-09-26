package devices

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"

	"go.mozilla.org/pkcs7"
	"howett.net/plist"

	"github.com/mdp/qrterminal/v3"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/rootfs"
)

type deviceURLServer struct {
	token      string
	name       string
	stopped    bool
	failures   map[string]asc.DeviceURLRegistration
	platform   string
	confirm    bool
	client     *asc.Client
	publicURL  string
	seen       map[string]struct{}
	devices    []asc.DeviceURLRegistration
	mu         sync.Mutex
	outputRows []deviceBatchRecord
}

func serveDeviceRegistration(ctx context.Context, options deviceURLServeOptions) (*asc.DeviceURLRegistrationResult, error) {
	if err := validateDeviceURLServeOptions(options); err != nil {
		return nil, err
	}
	token, err := newDeviceRegistrationToken()
	if err != nil {
		return nil, err
	}
	listener, err := net.Listen("tcp", options.Listen)
	if err != nil {
		return nil, fmt.Errorf("listen on %s: %w", options.Listen, err)
	}
	defer listener.Close()
	bound := listener.Addr().String()
	if err := validateDeviceRegisterListen(bound, options.ListenExplicit); err != nil {
		return nil, err
	}
	publicURL := strings.TrimRight(strings.TrimSpace(options.PublicURL), "/")
	if publicURL == "" {
		publicURL = "http://" + bound
	}
	serverState := &deviceURLServer{
		token:     token,
		name:      options.Name,
		platform:  options.Platform,
		confirm:   options.Confirm,
		client:    options.Client,
		publicURL: publicURL,
		seen:      map[string]struct{}{},
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/enroll", serverState.enroll)
	mux.HandleFunc("/profile", serverState.profile)
	mux.HandleFunc("/callback", serverState.callback)
	waitCtx, cancel := context.WithTimeout(ctx, options.TTL)
	defer cancel()
	httpServer := &http.Server{
		Handler:           mux,
		BaseContext:       func(net.Listener) context.Context { return waitCtx },
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
	}
	errCh := make(chan error, 1)
	go func() {
		errCh <- httpServer.Serve(listener)
	}()
	pageURL := publicURL + "/enroll?token=" + url.QueryEscape(token)
	fmt.Fprintf(os.Stderr, "Open this URL on the device, or scan the QR code. Unsigned profiles show as Not Verified on iOS.\n%s\n", pageURL)
	printDeviceRegistrationQR(os.Stderr, pageURL)
	var serveErr error
	select {
	case serveErr = <-errCh:
	case <-waitCtx.Done():
	}
	cancel()
	if errors.Is(serveErr, http.ErrServerClosed) {
		serveErr = nil
	}
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer shutdownCancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		_ = httpServer.Close()
		serveErr = errors.Join(serveErr, fmt.Errorf("stop registration server: %w", err))
	}
	serverState.mu.Lock()
	defer serverState.mu.Unlock()
	serverState.stopped = true
	if !options.Confirm {
		if err := writeDeviceRegistrationBatch(options.OutputFile, serverState.outputRows); err != nil {
			return nil, err
		}
	}
	result := &asc.DeviceURLRegistrationResult{
		URL:         pageURL,
		Token:       token,
		CollectOnly: !options.Confirm,
		OutputFile:  options.OutputFile,
		Devices:     append([]asc.DeviceURLRegistration{}, serverState.devices...),
	}
	for _, failure := range serverState.failures {
		result.Failures = append(result.Failures, failure)
	}
	sort.Slice(result.Failures, func(i, j int) bool { return result.Failures[i].UDID < result.Failures[j].UDID })
	if len(result.Failures) > 0 {
		serveErr = errors.Join(serveErr, fmt.Errorf("%d device registration(s) failed", len(result.Failures)))
	}
	if serveErr != nil {
		return result, serveErr
	}
	if options.Confirm && len(result.Devices) == 0 {
		return result, fmt.Errorf("no devices registered before timeout")
	}
	return result, nil
}

type deviceURLServeOptions struct {
	Name           string
	Listen         string
	ListenExplicit bool
	PublicURL      string
	TTL            time.Duration
	Platform       string
	Confirm        bool
	OutputFile     string
	Client         *asc.Client
}

func (server *deviceURLServer) enroll(w http.ResponseWriter, r *http.Request) {
	if !server.validToken(r) {
		http.Error(w, "unknown registration token", http.StatusForbidden)
		return
	}
	profileURL := server.publicURL + "/profile?token=" + url.QueryEscape(server.token)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, "<!doctype html><title>Register device</title><p>Install the profile. iOS will say the profile is Not Verified because it is unsigned.</p><p><a href=%q>Install profile</a></p>", profileURL)
}

func (server *deviceURLServer) profile(w http.ResponseWriter, r *http.Request) {
	if !server.validToken(r) {
		http.Error(w, "unknown registration token", http.StatusForbidden)
		return
	}
	callback := server.publicURL + "/callback?token=" + url.QueryEscape(server.token)
	payload, err := deviceRegistrationProfile(callback)
	if err != nil {
		http.Error(w, "profile unavailable", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/x-apple-aspen-config")
	_, _ = w.Write(payload)
}

func (server *deviceURLServer) callback(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !server.validToken(r) {
		http.Error(w, "unknown registration token", http.StatusForbidden)
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			http.Error(w, "device callback too large", http.StatusRequestEntityTooLarge)
			return
		}
		http.Error(w, "read body", http.StatusBadRequest)
		return
	}
	udid, product, err := parseDeviceRegistrationCallback(body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	record, err := server.acceptDevice(r.Context(), udid, product)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(record)
}

func (server *deviceURLServer) validToken(r *http.Request) bool {
	return subtleTokenEqual(r.URL.Query().Get("token"), server.token)
}

func (server *deviceURLServer) acceptDevice(ctx context.Context, udid, product string) (record asc.DeviceURLRegistration, err error) {
	normalized := normalizeDeviceUDIDForComparison(udid)
	name := strings.TrimSpace(server.name)
	if name == "" {
		name = strings.TrimSpace(product)
		if name == "" {
			name = "device"
		}
		if len(normalized) >= 8 {
			name += " " + normalized[:8]
		}
	}
	record = asc.DeviceURLRegistration{Name: name, UDID: udid, Platform: server.platform, Status: "collected"}
	server.mu.Lock()
	defer server.mu.Unlock()
	if server.stopped {
		return record, fmt.Errorf("registration session ended")
	}
	defer func() {
		if err != nil {
			if server.failures == nil {
				server.failures = map[string]asc.DeviceURLRegistration{}
			}
			record.Status = "failed"
			record.Error = err.Error()
			server.failures[normalized] = record
		} else {
			delete(server.failures, normalized)
		}
	}()
	if err := ctx.Err(); err != nil {
		return record, err
	}
	if _, ok := server.seen[normalized]; ok {
		record.Status = "skipped"
		return record, nil
	}
	if server.confirm {
		if server.client == nil {
			return record, fmt.Errorf("registration client unavailable")
		}
		requestCtx, cancel := shared.ContextWithTimeout(ctx)
		defer cancel()
		existing, err := findExistingDeviceByNormalizedUDID(requestCtx, server.client, udid, server.platform)
		if err != nil {
			return record, err
		}
		if existing != nil {
			record.Status = "skipped"
			record.DeviceID = existing.Data.ID
			server.seen[normalized] = struct{}{}
			server.devices = append(server.devices, record)
			return record, nil
		}
		created, err := server.client.CreateDevice(requestCtx, asc.DeviceCreateAttributes{
			Name:     name,
			UDID:     udid,
			Platform: asc.DevicePlatform(server.platform),
		})
		if err != nil {
			return record, err
		}
		record.Status = "registered"
		record.DeviceID = created.Data.ID
	}
	server.seen[normalized] = struct{}{}
	server.devices = append(server.devices, record)
	server.outputRows = append(server.outputRows, deviceBatchRecord{UDID: udid, Name: name, Platform: server.platform})
	return record, nil
}

func parseDeviceRegistrationCallback(body []byte) (string, string, error) {
	trimmed := strings.TrimSpace(string(body))
	if trimmed == "" {
		return "", "", fmt.Errorf("empty device callback")
	}
	if strings.HasPrefix(trimmed, "{") {
		var payload struct {
			UDID    string `json:"UDID"`
			PRODUCT string `json:"PRODUCT"`
		}
		if err := json.Unmarshal(body, &payload); err != nil {
			return "", "", fmt.Errorf("decode device callback")
		}
		return requireCallbackUDID(payload.UDID, payload.PRODUCT)
	}
	// iOS profile-service callbacks encapsulate the plist in CMS SignedData.
	// Verify content integrity only; the registration URL token authorizes this
	// request. This does not authenticate an Apple-issued device identity.
	if signed, err := pkcs7.Parse(body); err == nil {
		if err := signed.Verify(); err != nil {
			return "", "", fmt.Errorf("invalid device callback signature")
		}
		body = signed.Content
	}
	var payload map[string]any
	if _, err := plist.Unmarshal(body, &payload); err == nil {
		udid, _ := payload["UDID"].(string)
		product, _ := payload["PRODUCT"].(string)
		return requireCallbackUDID(udid, product)
	}
	return "", "", fmt.Errorf("unrecognized device callback")
}

func requireCallbackUDID(udid, product string) (string, string, error) {
	if strings.ContainsFunc(udid, unicode.IsControl) || strings.ContainsFunc(product, unicode.IsControl) {
		return "", "", fmt.Errorf("device callback contains control characters")
	}
	udid = strings.TrimSpace(udid)
	if normalizeDeviceUDIDForComparison(udid) == "" || strings.HasPrefix(udid, "#") {
		return "", "", fmt.Errorf("device callback missing UDID")
	}
	return udid, strings.TrimSpace(product), nil
}

func deviceRegistrationProfile(callback string) ([]byte, error) {
	tokenID, err := newDeviceRegistrationToken()
	if err != nil {
		return nil, err
	}
	payload := map[string]any{
		"PayloadContent": map[string]any{
			"URL":              callback,
			"DeviceAttributes": []string{"UDID", "PRODUCT", "VERSION"},
		},
		"PayloadOrganization": "asc",
		"PayloadDisplayName":  "Register device",
		"PayloadIdentifier":   "com.asc.device-registration",
		"PayloadUUID":         tokenID,
		"PayloadType":         "Profile Service",
		"PayloadVersion":      1,
		"PayloadDescription":  "Collect this device UDID. This unsigned profile is Not Verified.",
	}
	return plist.Marshal(payload, plist.XMLFormat)
}

func writeDeviceRegistrationBatch(path string, records []deviceBatchRecord) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("--output-file is required without --confirm")
	}
	root, err := rootfs.New(filepath.Dir(path))
	if err != nil {
		return err
	}
	defer root.Close()
	var data bytes.Buffer
	writer := csv.NewWriter(&data)
	writer.Comma = '\t'
	if err := writer.Write([]string{"Device ID", "Device Name", "Device Platform"}); err != nil {
		return err
	}
	for _, record := range records {
		if err := writer.Write([]string{record.UDID, record.Name, record.Platform}); err != nil {
			return err
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return err
	}
	return root.CreateNewFile(filepath.Base(path), data.Bytes(), 0o600)
}

func newDeviceRegistrationToken() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func validateDeviceRegisterListen(address string, allowPublic bool) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return shared.UsageErrorf("--listen must be host:port: %v", err)
	}
	if allowPublic {
		return nil
	}
	if host == "localhost" {
		return nil
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return shared.UsageError("--listen must be a loopback address unless you explicitly bind a public address")
	}
	return nil
}

func listenExplicit(fs *flag.FlagSet) bool {
	explicit := false
	fs.Visit(func(item *flag.Flag) {
		if item.Name == "listen" {
			explicit = true
		}
	})
	return explicit
}

func printDeviceRegistrationQR(w io.Writer, pageURL string) {
	config := qrterminal.Config{
		Level:     qrterminal.L,
		Writer:    w,
		BlackChar: qrterminal.BLACK,
		WhiteChar: qrterminal.WHITE,
		QuietZone: 1,
	}
	qrterminal.GenerateWithConfig(pageURL, config)
}

func subtleTokenEqual(got, want string) bool {
	if len(got) != len(want) || want == "" {
		return false
	}
	var diff byte
	for i := range got {
		diff |= got[i] ^ want[i]
	}
	return diff == 0
}

func validateDeviceURLServeOptions(options deviceURLServeOptions) error {
	if options.TTL <= 0 {
		return shared.UsageError("--ttl must be greater than zero")
	}
	if err := validateDeviceRegisterListen(options.Listen, options.ListenExplicit); err != nil {
		return err
	}
	if strings.ContainsFunc(options.Name, unicode.IsControl) {
		return shared.UsageError("--name cannot contain control characters")
	}
	if options.Confirm && options.OutputFile != "" {
		return shared.UsageError("--output-file cannot be combined with --confirm")
	}
	if raw := strings.TrimSpace(options.PublicURL); raw != "" {
		parsed, err := url.Parse(raw)
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Hostname() == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.ForceQuery || parsed.Fragment != "" || parsed.Opaque != "" {
			return shared.UsageError("--public-url must be an absolute HTTP(S) base URL without credentials, query, or fragment")
		}
	}
	return nil
}
