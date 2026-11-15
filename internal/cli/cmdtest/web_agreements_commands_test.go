package cmdtest

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"

	cmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	webcmd "github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/web"
	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

const (
	webAgreementsPortalHost         = "developer.apple.com"
	webAgreementsTeamsPath          = "/services-account/QH65B2/account/listTeams.action"
	webAgreementsHistoryPath        = "/services-account/QH65B2/account/getAgreementHistory"
	webAgreementsAcceptPath         = "/services-account/QH65B2/account/acceptAgreements"
	webAgreementsContractMessageURL = "https://appstoreconnect.apple.com/olympus/v1/contractMessages"
)

// webAgreementsPortal is an in-memory Developer Portal agreements fixture.
// Accepting marks the requested agreements accepted unless stayPending is set,
// so the CLI's verification re-read observes server-side state.
type webAgreementsPortal struct {
	mu           sync.Mutex
	accepted     map[string]bool
	stayPending  map[string]bool
	acceptBody   []map[string]any
	teamsCalls   int
	acceptCalls  int
	readCalls    int
	contentCalls int
	messageCalls int
	// contractMessagesStatus overrides the App Store Connect banner response
	// status so tests can make that unrelated read fail.
	contractMessagesStatus int
	// contentResponse overrides the agreement content download response.
	contentResponse func(req *http.Request) *http.Response
}

func newWebAgreementsPortal(pendingIDs ...string) *webAgreementsPortal {
	portal := &webAgreementsPortal{accepted: map[string]bool{}, stayPending: map[string]bool{}}
	for _, id := range pendingIDs {
		portal.accepted[id] = false
	}
	return portal
}

func (p *webAgreementsPortal) historyJSON() string {
	ids := make([]string, 0, len(p.accepted))
	for id := range p.accepted {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	records := make([]string, 0, len(ids))
	for _, id := range ids {
		dateAccepted := "0"
		if p.accepted[id] {
			dateAccepted = "1787158607000"
		}
		records = append(records, `{"agreementDownloadUrl":"/services-account/agreement/`+id+`/content/pdf","dateEffective":1787060333000,"dateAccepted":`+dateAccepted+`,"dateAgreeBy":1790899199000,"status":"active","version":"5031","isAgreementPLA":true,"agreementId":"`+id+`","title":"Agreement `+id+`"}`)
	}
	return `{"resultCode":0,"agreements":[` + strings.Join(records, ",") + `]}`
}

func (p *webAgreementsPortal) roundTrip(t *testing.T, req *http.Request) (*http.Response, error) {
	t.Helper()
	p.mu.Lock()
	defer p.mu.Unlock()

	switch {
	case req.Method == http.MethodGet && req.URL.String() == webAgreementsContractMessageURL:
		p.messageCalls++
		if p.contractMessagesStatus != 0 {
			return webAgreementsJSONResponse(p.contractMessagesStatus, `{}`), nil
		}
		return webAgreementsJSONResponse(http.StatusOK, `[]`), nil
	case req.Method == http.MethodPost && req.URL.Host == webAgreementsPortalHost && req.URL.Path == webAgreementsTeamsPath:
		p.teamsCalls++
		return webAgreementsJSONResponse(http.StatusOK, `{"teams":[{"teamId":"TEAM123456","name":"Example Team","status":"active"}]}`), nil
	case req.Method == http.MethodPost && req.URL.Host == webAgreementsPortalHost && req.URL.Path == webAgreementsHistoryPath:
		p.readCalls++
		return webAgreementsJSONResponse(http.StatusOK, p.historyJSON()), nil
	case req.Method == http.MethodPost && req.URL.Host == webAgreementsPortalHost && req.URL.Path == webAgreementsAcceptPath:
		p.acceptCalls++
		var payload map[string]any
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Errorf("decode acceptAgreements payload: %v", err)
		}
		p.acceptBody = append(p.acceptBody, payload)
		if ids, ok := payload["agreementIds"].([]any); ok {
			for _, raw := range ids {
				id, _ := raw.(string)
				if _, known := p.accepted[id]; known && !p.stayPending[id] {
					p.accepted[id] = true
				}
			}
		}
		return webAgreementsJSONResponse(http.StatusOK, p.historyJSON()), nil
	case req.Method == http.MethodGet && req.URL.Host == webAgreementsPortalHost && strings.HasPrefix(req.URL.Path, "/services-account/agreement/") && strings.HasSuffix(req.URL.Path, "/content/pdf"):
		p.contentCalls++
		if p.contentResponse != nil {
			return p.contentResponse(req), nil
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/pdf"}},
			Body:       io.NopCloser(strings.NewReader("%PDF-1.7 agreement body")),
		}, nil
	default:
		t.Errorf("unexpected request %s %s", req.Method, req.URL.String())
		return webAgreementsJSONResponse(http.StatusInternalServerError, `{}`), nil
	}
}

func webAgreementsJSONResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func installWebAgreementsPortal(t *testing.T, portal *webAgreementsPortal) {
	t.Helper()
	setCmdtestHome(t)
	t.Setenv("ASC_WEB_MIN_REQUEST_INTERVAL", "0")
	restoreSession := webcmd.SetResolveWebSession(func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{
			Client: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				return portal.roundTrip(t, req)
			})},
		}, "cache", nil
	})
	t.Cleanup(restoreSession)
}

func TestWebAgreementsAcceptCommandRegistration(t *testing.T) {
	root := RootCommand("1.2.3")
	accept := findSubcommand(root, "web", "agreements", "accept")
	if accept == nil {
		t.Fatal("expected web agreements accept command")
	}
	for _, flagName := range []string{"agreement-id", "confirm", "apple-id", "output", "pretty"} {
		if accept.FlagSet.Lookup(flagName) == nil {
			t.Fatalf("expected --%s flag on accept", flagName)
		}
	}
	if usage := accept.FlagSet.Lookup("agreement-id").Usage; !strings.Contains(usage, "repeatable") {
		t.Fatalf("--agreement-id usage = %q, want repeatable hint", usage)
	}
}

func TestWebAgreementsDownloadCommandRegistration(t *testing.T) {
	root := RootCommand("1.2.3")
	download := findSubcommand(root, "web", "agreements", "download")
	if download == nil {
		t.Fatal("expected web agreements download command")
	}
	for _, flagName := range []string{"agreement-id", "out", "overwrite", "apple-id", "output", "pretty"} {
		if download.FlagSet.Lookup(flagName) == nil {
			t.Fatalf("expected --%s flag on download", flagName)
		}
	}
	if download.FlagSet.Lookup("confirm") != nil {
		t.Fatal("download is read-only and must not expose --confirm")
	}
}

func TestWebAgreementsDownloadRunWritesAgreementAndRefusesOverwrite(t *testing.T) {
	portal := newWebAgreementsPortal("XG8DNV4HYY")
	portal.accepted["XG8DNV4HYY"] = true
	installWebAgreementsPortal(t, portal)

	outPath := filepath.Join(t.TempDir(), "agreement.pdf")
	stdout, stderr, code := runWithDeps(t, nil,
		"--profile", "test-web",
		"web", "agreements", "download",
		"--agreement-id", "XG8DNV4HYY",
		"--out", outPath,
		"--output", "json",
	)
	if code != cmd.ExitSuccess {
		t.Fatalf("exit code = %d, want %d", code, cmd.ExitSuccess)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if portal.contentCalls != 1 {
		t.Fatalf("content downloads = %d, want 1", portal.contentCalls)
	}
	body, err := os.ReadFile(outPath)
	if err != nil || string(body) != "%PDF-1.7 agreement body" {
		t.Fatalf("downloaded body = %q, err = %v", body, err)
	}
	if strings.Contains(stdout, "%PDF") || strings.Contains(stdout, "content/pdf") {
		t.Fatalf("stdout must not include agreement content or the download URL: %q", stdout)
	}

	var payload struct {
		AgreementID  string `json:"agreementId"`
		TeamID       string `json:"teamId"`
		Path         string `json:"path"`
		BytesWritten int64  `json:"bytesWritten"`
		ContentType  string `json:"contentType"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("json.Unmarshal() error: %v; stdout=%q", err, stdout)
	}
	if payload.AgreementID != "XG8DNV4HYY" || payload.TeamID != "TEAM123456" || payload.Path != outPath {
		t.Fatalf("unexpected receipt: %+v", payload)
	}
	if payload.BytesWritten != int64(len(body)) || payload.ContentType != "application/pdf" {
		t.Fatalf("unexpected receipt size/type: %+v", payload)
	}

	stdout, stderr, code = runWithDeps(t, nil,
		"--profile", "test-web",
		"web", "agreements", "download",
		"--agreement-id", "XG8DNV4HYY",
		"--out", outPath,
	)
	if code != cmd.ExitError {
		t.Fatalf("exit code = %d, want %d", code, cmd.ExitError)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "already exists") || !strings.Contains(stderr, "--overwrite") {
		t.Fatalf("expected overwrite refusal, got %q", stderr)
	}
	if portal.contentCalls != 1 {
		t.Fatalf("content downloads = %d, want no second download without --overwrite", portal.contentCalls)
	}
}

func TestWebAgreementsDownloadRunRejectsCrossOriginRedirectWithoutLeakingURL(t *testing.T) {
	portal := newWebAgreementsPortal("XG8DNV4HYY")
	portal.accepted["XG8DNV4HYY"] = true
	portal.contentResponse = func(req *http.Request) *http.Response {
		return &http.Response{
			StatusCode: http.StatusFound,
			Header:     http.Header{"Location": []string{"https://cdn.example.test/agreement.pdf?token=very-secret&X-Amz-Signature=abc123"}},
			Body:       io.NopCloser(strings.NewReader("")),
		}
	}
	installWebAgreementsPortal(t, portal)

	outPath := filepath.Join(t.TempDir(), "agreement.pdf")
	stdout, stderr, code := runWithDeps(t, nil,
		"--profile", "test-web",
		"web", "agreements", "download",
		"--agreement-id", "XG8DNV4HYY",
		"--out", outPath,
	)
	if code != cmd.ExitError {
		t.Fatalf("exit code = %d, want %d", code, cmd.ExitError)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "redirect") {
		t.Fatalf("expected redirect rejection on stderr, got %q", stderr)
	}
	for _, leaked := range []string{"very-secret", "X-Amz-Signature", "?token="} {
		if strings.Contains(stderr, leaked) {
			t.Fatalf("stderr leaks %q: %q", leaked, stderr)
		}
	}
	if _, err := os.Lstat(outPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected no file after rejected download, got %v", err)
	}
}

func TestWebAgreementsAcceptRunVerifiesFromHistoryWhenContractMessagesFail(t *testing.T) {
	portal := newWebAgreementsPortal("XG8DNV4HYY")
	portal.contractMessagesStatus = http.StatusServiceUnavailable
	installWebAgreementsPortal(t, portal)

	stdout, stderr, code := runWithDeps(t, nil,
		"--profile", "test-web",
		"web", "agreements", "accept",
		"--agreement-id", "XG8DNV4HYY",
		"--confirm",
		"--output", "json",
	)
	if code != cmd.ExitSuccess {
		t.Fatalf("exit code = %d, want %d", code, cmd.ExitSuccess)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if portal.messageCalls != 0 {
		t.Fatalf("contractMessages calls = %d, want 0 during accept verification", portal.messageCalls)
	}
	if portal.acceptCalls != 1 || portal.readCalls != 1 {
		t.Fatalf("accept/history calls = %d/%d, want 1/1", portal.acceptCalls, portal.readCalls)
	}
	if !strings.Contains(stdout, `"verified":true`) {
		t.Fatalf("stdout = %q, want verified receipt", stdout)
	}
}

func TestWebAgreementsAcceptRunSendsOneRequestForRepeatedAgreementIDs(t *testing.T) {
	portal := newWebAgreementsPortal("XG8DNV4HYY", "AB12CD34EF")
	installWebAgreementsPortal(t, portal)

	stdout, stderr, code := runWithDeps(t, nil,
		"--profile", "test-web",
		"web", "agreements", "accept",
		"--agreement-id", "XG8DNV4HYY",
		"--agreement-id", "AB12CD34EF",
		"--confirm",
		"--output", "json",
	)
	if code != cmd.ExitSuccess {
		t.Fatalf("exit code = %d, want %d", code, cmd.ExitSuccess)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	if portal.acceptCalls != 1 {
		t.Fatalf("acceptAgreements calls = %d, want 1", portal.acceptCalls)
	}
	ids, _ := portal.acceptBody[0]["agreementIds"].([]any)
	if len(ids) != 2 || ids[0] != "XG8DNV4HYY" || ids[1] != "AB12CD34EF" {
		t.Fatalf("acceptAgreements agreementIds = %v, want both IDs in one request", ids)
	}
	if portal.acceptBody[0]["teamId"] != "TEAM123456" {
		t.Fatalf("acceptAgreements teamId = %v, want TEAM123456", portal.acceptBody[0]["teamId"])
	}
	if portal.readCalls != 1 {
		t.Fatalf("getAgreementHistory calls = %d, want one verification re-read after the write", portal.readCalls)
	}

	var payload struct {
		TeamID       string   `json:"teamId"`
		AgreementIDs []string `json:"agreementIds"`
		Status       string   `json:"status"`
		Verified     bool     `json:"verified"`
		Agreements   []struct {
			AgreementID  string `json:"agreementId"`
			Pending      bool   `json:"pending"`
			DateAccepted string `json:"dateAccepted"`
		} `json:"agreements"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("json.Unmarshal() error: %v; stdout=%q", err, stdout)
	}
	if payload.TeamID != "TEAM123456" || payload.Status != "accepted" || !payload.Verified {
		t.Fatalf("unexpected receipt: %+v", payload)
	}
	if len(payload.AgreementIDs) != 2 || len(payload.Agreements) != 2 {
		t.Fatalf("receipt should list both requested agreements, got %+v", payload)
	}
	for _, agreement := range payload.Agreements {
		if agreement.Pending || agreement.DateAccepted == "" {
			t.Fatalf("receipt agreement %+v should reflect re-read accepted state", agreement)
		}
	}
}

func TestWebAgreementsAcceptRunRequiresConfirmBeforeHTTP(t *testing.T) {
	portal := newWebAgreementsPortal("XG8DNV4HYY")
	installWebAgreementsPortal(t, portal)

	assertUsageExitWithDeps(t, nil, []string{
		"--profile", "test-web",
		"web", "agreements", "accept",
		"--agreement-id", "XG8DNV4HYY",
		"--agreement-id", "AB12CD34EF",
	}, "--confirm is required")

	if portal.teamsCalls != 0 || portal.acceptCalls != 0 || portal.readCalls != 0 {
		t.Fatalf("expected no HTTP without --confirm, got teams=%d accept=%d read=%d", portal.teamsCalls, portal.acceptCalls, portal.readCalls)
	}
}

func TestWebAgreementsAcceptRunFailsWhenAgreementStaysPending(t *testing.T) {
	portal := newWebAgreementsPortal("XG8DNV4HYY", "AB12CD34EF")
	portal.stayPending["AB12CD34EF"] = true
	installWebAgreementsPortal(t, portal)

	stdout, stderr, code := runWithDeps(t, nil,
		"--profile", "test-web",
		"web", "agreements", "accept",
		"--agreement-id", "XG8DNV4HYY",
		"--agreement-id", "AB12CD34EF",
		"--confirm",
		"--output", "json",
	)
	if code != cmd.ExitError {
		t.Fatalf("exit code = %d, want %d", code, cmd.ExitError)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "AB12CD34EF") || !strings.Contains(stderr, "still pending") {
		t.Fatalf("expected still-pending error naming AB12CD34EF, got %q", stderr)
	}
	if portal.acceptCalls != 1 || portal.readCalls != 1 {
		t.Fatalf("expected one write and one re-read, got accept=%d read=%d", portal.acceptCalls, portal.readCalls)
	}
}
