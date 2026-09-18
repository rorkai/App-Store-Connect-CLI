package devices

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"howett.net/plist"

	"github.com/mdp/qrterminal/v3"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

type deviceURLRegistration struct {
	Name     string `json:"name"`
	UDID     string `json:"udid"`
	Platform string `json:"platform"`
	Status   string `json:"status"`
	DeviceID string `json:"deviceId,omitempty"`
}

type deviceURLRegistrationResult struct {
	URL         string                  `json:"url"`
	Token       string                  `json:"token"`
	CollectOnly bool                    `json:"collectOnly"`
	OutputFile  string                  `json:"outputFile,omitempty"`
	Devices     []deviceURLRegistration `json:"devices"`
}

type deviceURLServer struct {
	token      string
	platform   string
	confirm    bool
	client     *asc.Client
	publicURL  string
	seen       map[string]struct{}
	devices    []deviceURLRegistration
	mu         sync.Mutex
	outputFile string
	outputRows []deviceBatchRecord
}

func serveDeviceRegistration(ctx context.Context, options deviceURLServeOptions) (*deviceURLRegistrationResult, error) {
	if err := validateDeviceRegisterListen(options.Listen, options.ListenExplicit); err != nil {
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
		token:      token,
		platform:   options.Platform,
		confirm:    options.Confirm,
		client:     options.Client,
		publicURL:  publicURL,
		seen:       map[string]struct{}{},
		outputFile: options.OutputFile,
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/enroll", serverState.enroll)
	mux.HandleFunc("/profile", serverState.profile)
	mux.HandleFunc("/callback", serverState.callback)
	httpServer := &http.Server{Handler: mux}
	errCh := make(chan error, 1)
	go func() {
		errCh <- httpServer.Serve(listener)
	}()
	pageURL := publicURL + "/enroll?token=" + url.QueryEscape(token)
	fmt.Fprintf(os.Stderr, "Open this URL on the device, or scan the QR code. Unsigned profiles show as Not Verified on iOS.\n%s\n", pageURL)
	printDeviceRegistrationQR(os.Stderr, pageURL)
	waitCtx, cancel := context.WithTimeout(ctx, options.TTL)
	defer cancel()
	<-waitCtx.Done()
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer shutdownCancel()
	_ = httpServer.Shutdown(shutdownCtx)
	<-errCh
	if !options.Confirm {
		if err := writeDeviceRegistrationBatch(options.OutputFile, serverState.outputRows); err != nil {
			return nil, err
		}
	}
	result := &deviceURLRegistrationResult{
		URL:         pageURL,
		Token:       token,
		CollectOnly: !options.Confirm,
		OutputFile:  options.OutputFile,
		Devices:     append([]deviceURLRegistration(nil), serverState.devices...),
	}
	if options.Confirm && len(result.Devices) == 0 {
		return result, fmt.Errorf("no devices registered before timeout")
	}
	return result, nil
}

type deviceURLServeOptions struct {
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
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
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

func (server *deviceURLServer) acceptDevice(ctx context.Context, udid, product string) (deviceURLRegistration, error) {
	normalized := normalizeDeviceUDIDForComparison(udid)
	name := strings.TrimSpace(product)
	if name == "" {
		name = "device"
	}
	if len(normalized) >= 8 {
		name += " " + normalized[:8]
	}
	record := deviceURLRegistration{Name: name, UDID: udid, Platform: server.platform, Status: "collected"}
	server.mu.Lock()
	defer server.mu.Unlock()
	if _, ok := server.seen[normalized]; ok {
		record.Status = "skipped"
		return record, nil
	}
	server.seen[normalized] = struct{}{}
	if server.confirm && server.client != nil {
		existing, err := findExistingDeviceByNormalizedUDID(ctx, server.client, udid, server.platform)
		if err != nil {
			return record, err
		}
		if existing != nil {
			record.Status = "skipped"
			record.DeviceID = existing.Data.ID
			server.devices = append(server.devices, record)
			return record, nil
		}
		created, err := server.client.CreateDevice(ctx, asc.DeviceCreateAttributes{
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
	var payload map[string]any
	if _, err := plist.Unmarshal(body, &payload); err == nil {
		udid, _ := payload["UDID"].(string)
		product, _ := payload["PRODUCT"].(string)
		return requireCallbackUDID(udid, product)
	}
	return "", "", fmt.Errorf("unrecognized device callback")
}

func requireCallbackUDID(udid, product string) (string, string, error) {
	udid = strings.TrimSpace(udid)
	if udid == "" {
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
	file, err := shared.OpenNewFileNoFollow(path, 0o600)
	if err != nil {
		return fmt.Errorf("write registration file: %w", err)
	}
	defer file.Close()
	if _, err := fmt.Fprintln(file, "Device ID\tDevice Name\tDevice Platform"); err != nil {
		return err
	}
	for _, record := range records {
		if _, err := fmt.Fprintf(file, "%s\t%s\t%s\n", record.UDID, record.Name, record.Platform); err != nil {
			return err
		}
	}
	return file.Sync()
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
