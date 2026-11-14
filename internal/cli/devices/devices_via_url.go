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

// maxPendingDeviceCallbackTokens bounds unused per-profile callback tokens so
// repeated profile downloads cannot grow session memory without limit.
const maxPendingDeviceCallbackTokens = 1024

var errDeviceCallbackToken = errors.New("unknown registration token")

// errDeviceRegistrationSessionEnded refuses requests after the session reached
// --max-devices or began shutting down.
var errDeviceRegistrationSessionEnded = errors.New("registration session ended")

type deviceURLServer struct {
	token      string
	name       string
	stopped    bool
	failures   map[string]asc.DeviceURLRegistration
	platform   string
	confirm    bool
	client     *asc.Client
	timeout    func(context.Context) (context.Context, context.CancelFunc)
	publicURL  string
	seen       map[string]struct{}
	devices    []asc.DeviceURLRegistration
	mu         sync.Mutex
	outputRows []deviceBatchRecord
	// callbackTokens holds single-use callback tokens minted per served
	// profile. A value is the normalized UDID the token is bound to after its
	// first valid callback, or empty while unbound.
	callbackTokens map[string]string
	// stream, when set, receives one JSON receipt line per device arrival.
	stream    io.Writer
	streamErr error
	// maxDevices, when positive, ends the session once that many new devices
	// are registered (or collected without --confirm). Duplicates and devices
	// already in App Store Connect do not count.
	maxDevices int
	accepted   int
	// limitReached is set, and limitDone closed, when maxDevices is reached.
	limitReached bool
	limitDone    chan struct{}
}

func serveDeviceRegistration(ctx context.Context, d *shared.Deps, options deviceURLServeOptions) (*asc.DeviceURLRegistrationResult, error) {
	if err := validateDeviceURLServeOptions(d, options); err != nil {
		return nil, err
	}
	if !options.Confirm {
		root, err := rootfs.New(filepath.Dir(options.OutputFile))
		if err != nil {
			return nil, err
		}
		defer root.Close()
		if err := root.CheckCreateNewFile(filepath.Base(options.OutputFile)); err != nil {
			return nil, err
		}
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
	if err := validateDeviceRegisterListen(d, bound, options.ListenExplicit); err != nil {
		return nil, err
	}
	publicURL := strings.TrimRight(strings.TrimSpace(options.PublicURL), "/")
	if publicURL == "" {
		publicURL = "http://" + bound
	}
	serverState := &deviceURLServer{
		token:      token,
		name:       options.Name,
		platform:   options.Platform,
		confirm:    options.Confirm,
		client:     options.Client,
		timeout:    d.Timeout,
		publicURL:  publicURL,
		seen:       map[string]struct{}{},
		stream:     options.Stream,
		maxDevices: options.MaxDevices,
		limitDone:  make(chan struct{}),
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
	fmt.Fprintf(d.IO.ErrOut, "Open this URL on the device, or scan the QR code. Unsigned profiles show as Not Verified on iOS.\n%s\n", pageURL)
	printDeviceRegistrationQR(d.IO.ErrOut, pageURL)
	var serveErr error
	select {
	case serveErr = <-errCh:
	case <-waitCtx.Done():
	case <-serverState.limitDone:
		fmt.Fprintf(d.IO.ErrOut, "Reached --max-devices %d; ending the registration session.\n", options.MaxDevices)
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
	if serverState.streamErr != nil {
		serveErr = errors.Join(serveErr, fmt.Errorf("write device receipt: %w", serverState.streamErr))
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
	// MaxDevices, when positive, ends the session after that many new devices.
	MaxDevices int
	Client     *asc.Client
	// Stream, when non-nil, receives one JSON receipt line per device arrival.
	Stream io.Writer
}

func (server *deviceURLServer) enroll(w http.ResponseWriter, r *http.Request) {
	if !server.validToken(r) {
		http.Error(w, "unknown registration token", http.StatusForbidden)
		return
	}
	if server.sessionEnded() {
		http.Error(w, errDeviceRegistrationSessionEnded.Error(), http.StatusGone)
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
	callbackToken, err := server.issueCallbackToken()
	if errors.Is(err, errDeviceRegistrationSessionEnded) {
		http.Error(w, err.Error(), http.StatusGone)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	callback := server.publicURL + "/callback?token=" + url.QueryEscape(callbackToken)
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
	token := r.URL.Query().Get("token")
	if !server.pendingCallbackToken(token) {
		http.Error(w, errDeviceCallbackToken.Error(), http.StatusForbidden)
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
	record, err := server.acceptDevice(r.Context(), token, udid, product)
	if errors.Is(err, errDeviceCallbackToken) {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	if errors.Is(err, errDeviceRegistrationSessionEnded) {
		http.Error(w, err.Error(), http.StatusGone)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(record)
}

// sessionEnded reports whether the session stopped or reached --max-devices.
func (server *deviceURLServer) sessionEnded() bool {
	server.mu.Lock()
	defer server.mu.Unlock()
	return server.stopped || server.limitReached
}

func (server *deviceURLServer) validToken(r *http.Request) bool {
	return subtleTokenEqual(r.URL.Query().Get("token"), server.token)
}

// issueCallbackToken mints the single-use token embedded in one served profile.
func (server *deviceURLServer) issueCallbackToken() (string, error) {
	server.mu.Lock()
	defer server.mu.Unlock()
	if server.stopped || server.limitReached {
		return "", errDeviceRegistrationSessionEnded
	}
	if len(server.callbackTokens) >= maxPendingDeviceCallbackTokens {
		return "", fmt.Errorf("too many pending device registrations")
	}
	token, err := newDeviceRegistrationToken()
	if err != nil {
		return "", err
	}
	if server.callbackTokens == nil {
		server.callbackTokens = map[string]string{}
	}
	server.callbackTokens[token] = ""
	return token, nil
}

func (server *deviceURLServer) pendingCallbackToken(token string) bool {
	if token == "" {
		return false
	}
	server.mu.Lock()
	defer server.mu.Unlock()
	_, ok := server.callbackTokens[token]
	return ok
}

// claimCallbackToken binds a pending token to the first UDID that uses it, so a
// failed registration can be retried for that device but not reused for another.
// The caller must hold server.mu.
func (server *deviceURLServer) claimCallbackToken(token, normalizedUDID string) error {
	bound, ok := server.callbackTokens[token]
	if token == "" || !ok {
		return errDeviceCallbackToken
	}
	if bound != "" && bound != normalizedUDID {
		return fmt.Errorf("%w: token is bound to another device", errDeviceCallbackToken)
	}
	server.callbackTokens[token] = normalizedUDID
	return nil
}

func (server *deviceURLServer) acceptDevice(ctx context.Context, token, udid, product string) (record asc.DeviceURLRegistration, err error) {
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
	if server.stopped || server.limitReached {
		return record, errDeviceRegistrationSessionEnded
	}
	if err := server.claimCallbackToken(token, normalized); err != nil {
		return record, err
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
			// The token is spent once its device is recorded; replays are refused.
			delete(server.callbackTokens, token)
		}
		server.streamArrival(record)
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
		requestCtx, cancel := server.timeout(ctx)
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
	server.countAcceptedDevice()
	return record, nil
}

// countAcceptedDevice records a newly registered or collected device and ends
// the session once --max-devices is reached. The caller must hold server.mu.
func (server *deviceURLServer) countAcceptedDevice() {
	server.accepted++
	if server.maxDevices <= 0 || server.limitReached || server.accepted < server.maxDevices {
		return
	}
	server.limitReached = true
	if server.limitDone != nil {
		close(server.limitDone)
	}
}

// streamArrival writes one JSON receipt line for a device arrival. The caller
// must hold server.mu so receipts are written whole and in arrival order.
func (server *deviceURLServer) streamArrival(record asc.DeviceURLRegistration) {
	if server.stream == nil || server.streamErr != nil {
		return
	}
	server.streamErr = json.NewEncoder(server.stream).Encode(record)
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
	// Verify content integrity only; the single-use callback token authorizes this
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

func validateDeviceRegisterListen(d *shared.Deps, address string, allowPublic bool) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return d.UsageErrorf("--listen must be host:port: %v", err)
	}
	if allowPublic {
		return nil
	}
	if host == "localhost" {
		return nil
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return d.UsageError("--listen must be a loopback address unless you explicitly bind a public address")
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

func validateDeviceURLServeOptions(d *shared.Deps, options deviceURLServeOptions) error {
	if options.TTL <= 0 {
		return d.UsageError("--ttl must be greater than zero")
	}
	if options.MaxDevices < 0 {
		return d.UsageError("--max-devices must be at least 1")
	}
	if err := validateDeviceRegisterListen(d, options.Listen, options.ListenExplicit); err != nil {
		return err
	}
	if strings.ContainsFunc(options.Name, unicode.IsControl) {
		return d.UsageError("--name cannot contain control characters")
	}
	if options.Confirm && options.OutputFile != "" {
		return d.UsageError("--output-file cannot be combined with --confirm")
	}
	if raw := strings.TrimSpace(options.PublicURL); raw != "" {
		parsed, err := url.Parse(raw)
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Hostname() == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.ForceQuery || parsed.Fragment != "" || parsed.Opaque != "" {
			return d.UsageError("--public-url must be an absolute HTTP(S) base URL without credentials, query, or fragment")
		}
		if parsed.Path != "" && parsed.Path != "/" {
			return d.UsageError("--public-url must not contain a path prefix")
		}
	}
	return nil
}
