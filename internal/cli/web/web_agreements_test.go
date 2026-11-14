package web

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

func stubWebAgreementsSession(t *testing.T) {
	t.Helper()
	origResolveSession := resolveSessionFn
	origNewWebClient := newWebClientFn
	origPersist := persistWebSessionFn
	t.Cleanup(func() {
		resolveSessionFn = origResolveSession
		newWebClientFn = origNewWebClient
		persistWebSessionFn = origPersist
	})
	resolveSessionFn = func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{}, "cache", nil
	}
	newWebClientFn = func(session *webcore.AuthSession) *webcore.Client {
		return &webcore.Client{}
	}
	persistWebSessionFn = func(session *webcore.AuthSession) error {
		return nil
	}
}

// stubWebAgreementsAccept replaces the accept and history hooks. The accept hook
// records every request; the history hook returns reread for the verification
// re-read after the write. The combined status hook (which also reads App Store
// Connect contract messages) fails the test if the accept flow touches it.
func stubWebAgreementsAccept(t *testing.T, reread *asc.WebAgreementsStatusResult, rereadErr error) (*[]webcore.AgreementsAcceptRequest, *int) {
	t.Helper()
	origAccept := acceptAgreementsFn
	origHistory := getAgreementHistoryFn
	origStatus := getAgreementsStatusFn
	t.Cleanup(func() {
		acceptAgreementsFn = origAccept
		getAgreementHistoryFn = origHistory
		getAgreementsStatusFn = origStatus
	})
	requests := &[]webcore.AgreementsAcceptRequest{}
	historyCalls := new(int)
	acceptAgreementsFn = func(ctx context.Context, client *webcore.Client, req webcore.AgreementsAcceptRequest) (*asc.WebAgreementsAcceptResult, error) {
		*requests = append(*requests, req)
		return &asc.WebAgreementsAcceptResult{
			TeamID:       "TEAM123456",
			AgreementIDs: req.AgreementIDs,
			Status:       "accepted",
		}, nil
	}
	getAgreementHistoryFn = func(ctx context.Context, client *webcore.Client) (*asc.WebAgreementsStatusResult, error) {
		*historyCalls++
		if rereadErr != nil {
			return nil, rereadErr
		}
		return reread, nil
	}
	getAgreementsStatusFn = func(ctx context.Context, client *webcore.Client) (*asc.WebAgreementsStatusResult, error) {
		t.Errorf("accept verification must use the history-only read, not the combined status read")
		return nil, errors.New("unexpected status read")
	}
	return requests, historyCalls
}

func acceptedAgreement(id, version string) asc.WebAgreement {
	return asc.WebAgreement{
		AgreementID:   id,
		Title:         "Agreement " + id,
		Status:        "active",
		Version:       version,
		Pending:       false,
		DateEffective: "2026-08-14T09:38:53Z",
		DateAccepted:  "2026-08-19T16:56:47Z",
	}
}

func TestWebAgreementsAcceptValidationErrors(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{name: "missing agreement id", args: []string{"--confirm"}, wantErr: "--agreement-id is required"},
		{name: "missing confirm", args: []string{"--agreement-id", "XG8DNV4HYY"}, wantErr: "--confirm is required"},
		{name: "missing confirm with repeated ids", args: []string{"--agreement-id", "XG8DNV4HYY", "--agreement-id", "AB12CD34EF"}, wantErr: "--confirm is required"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			stubWebAgreementsSession(t)
			requests, statusCalls := stubWebAgreementsAccept(t, nil, nil)

			cmd := WebAgreementsAcceptCommand()
			if err := cmd.FlagSet.Parse(tc.args); err != nil {
				t.Fatalf("parse error: %v", err)
			}
			stdout, stderr := captureWebCommandOutput(t, func() {
				err := cmd.Exec(context.Background(), nil)
				if !errors.Is(err, flag.ErrHelp) {
					t.Fatalf("expected flag.ErrHelp, got %v", err)
				}
			})
			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, tc.wantErr) {
				t.Fatalf("expected stderr to contain %q, got %q", tc.wantErr, stderr)
			}
			if len(*requests) != 0 || *statusCalls != 0 {
				t.Fatalf("expected no HTTP before validation passes, got accept=%d status=%d", len(*requests), *statusCalls)
			}
		})
	}
}

func TestWebAgreementsAcceptRejectsEmptyAgreementID(t *testing.T) {
	cmd := WebAgreementsAcceptCommand()
	cmd.FlagSet.Init(cmd.FlagSet.Name(), flag.ContinueOnError)
	cmd.FlagSet.SetOutput(io.Discard)
	err := cmd.FlagSet.Parse([]string{"--agreement-id", "  ", "--confirm"})
	if err == nil || !strings.Contains(err.Error(), "cannot be empty") {
		t.Fatalf("expected empty --agreement-id parse error, got %v", err)
	}
}

func TestWebAgreementsAcceptRepeatedAgreementIDsSendSingleRequest(t *testing.T) {
	stubWebAgreementsSession(t)
	requests, statusCalls := stubWebAgreementsAccept(t, &asc.WebAgreementsStatusResult{
		TeamID:  "TEAM123456",
		Pending: false,
		Agreements: []asc.WebAgreement{
			acceptedAgreement("XG8DNV4HYY", "5031"),
			acceptedAgreement("AB12CD34EF", "12"),
			acceptedAgreement("UNRELATED01", "3"),
		},
	}, nil)

	cmd := WebAgreementsAcceptCommand()
	args := []string{"--agreement-id", "XG8DNV4HYY", "--agreement-id", "AB12CD34EF", "--agreement-id", "XG8DNV4HYY", "--confirm", "--output", "json"}
	if err := cmd.FlagSet.Parse(args); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	stdout, stderr := captureWebCommandOutput(t, func() {
		if err := cmd.Exec(context.Background(), nil); err != nil {
			t.Fatalf("Exec() error: %v", err)
		}
	})
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	if len(*requests) != 1 {
		t.Fatalf("accept requests = %d, want exactly one request carrying every ID", len(*requests))
	}
	if got := (*requests)[0].AgreementIDs; len(got) != 2 || got[0] != "XG8DNV4HYY" || got[1] != "AB12CD34EF" {
		t.Fatalf("accept request agreement IDs = %v, want deduplicated [XG8DNV4HYY AB12CD34EF]", got)
	}
	if *statusCalls != 1 {
		t.Fatalf("status re-read calls = %d, want 1", *statusCalls)
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
		t.Fatalf("unmarshal stdout %q: %v", stdout, err)
	}
	if payload.Status != "accepted" || payload.TeamID != "TEAM123456" || !payload.Verified {
		t.Fatalf("payload = %+v, want verified accepted receipt", payload)
	}
	if len(payload.Agreements) != 2 || payload.Agreements[0].AgreementID != "XG8DNV4HYY" || payload.Agreements[1].AgreementID != "AB12CD34EF" {
		t.Fatalf("receipt agreements = %+v, want only the two requested agreements from the re-read", payload.Agreements)
	}
	for _, agreement := range payload.Agreements {
		if agreement.Pending || agreement.DateAccepted == "" {
			t.Fatalf("receipt agreement %+v should reflect the accepted re-read state", agreement)
		}
	}
}

func TestWebAgreementsAcceptFailsWhenAgreementStillPendingAfterWrite(t *testing.T) {
	stubWebAgreementsSession(t)
	stillPending := acceptedAgreement("AB12CD34EF", "12")
	stillPending.Pending = true
	stillPending.DateAccepted = ""
	requests, _ := stubWebAgreementsAccept(t, &asc.WebAgreementsStatusResult{
		TeamID:     "TEAM123456",
		Pending:    true,
		Agreements: []asc.WebAgreement{acceptedAgreement("XG8DNV4HYY", "5031"), stillPending},
	}, nil)

	cmd := WebAgreementsAcceptCommand()
	if err := cmd.FlagSet.Parse([]string{"--agreement-id", "XG8DNV4HYY", "--agreement-id", "AB12CD34EF", "--confirm", "--output", "json"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	var execErr error
	stdout, _ := captureWebCommandOutput(t, func() {
		execErr = cmd.Exec(context.Background(), nil)
	})
	if execErr == nil || errors.Is(execErr, flag.ErrHelp) {
		t.Fatalf("Exec() error = %v, want a non-usage failure", execErr)
	}
	if !strings.Contains(execErr.Error(), "AB12CD34EF") || !strings.Contains(execErr.Error(), "still pending") {
		t.Fatalf("Exec() error = %q, want the still-pending agreement ID", execErr)
	}
	if strings.Contains(execErr.Error(), "XG8DNV4HYY") {
		t.Fatalf("Exec() error = %q, should not list the verified agreement as pending", execErr)
	}
	if stdout != "" {
		t.Fatalf("expected no receipt on stdout, got %q", stdout)
	}
	if len(*requests) != 1 {
		t.Fatalf("accept requests = %d, want 1", len(*requests))
	}
}

func TestWebAgreementsAcceptFailsWhenReReadOmitsAgreement(t *testing.T) {
	stubWebAgreementsSession(t)
	stubWebAgreementsAccept(t, &asc.WebAgreementsStatusResult{
		TeamID:     "TEAM123456",
		Agreements: []asc.WebAgreement{acceptedAgreement("XG8DNV4HYY", "5031")},
	}, nil)

	cmd := WebAgreementsAcceptCommand()
	if err := cmd.FlagSet.Parse([]string{"--agreement-id", "MISSING0001", "--confirm"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	var execErr error
	stdout, _ := captureWebCommandOutput(t, func() {
		execErr = cmd.Exec(context.Background(), nil)
	})
	if execErr == nil || !strings.Contains(execErr.Error(), "MISSING0001") || !strings.Contains(execErr.Error(), "not present") {
		t.Fatalf("Exec() error = %v, want missing agreement failure", execErr)
	}
	if stdout != "" {
		t.Fatalf("expected no receipt on stdout, got %q", stdout)
	}
}

func TestWebAgreementsAcceptPersistsTeamWhenAcceptResponseIsAmbiguous(t *testing.T) {
	stubWebAgreementsSession(t)
	origAccept := acceptAgreementsFn
	origHistory := getAgreementHistoryFn
	t.Cleanup(func() {
		acceptAgreementsFn = origAccept
		getAgreementHistoryFn = origHistory
	})
	acceptAgreementsFn = func(context.Context, *webcore.Client, webcore.AgreementsAcceptRequest) (*asc.WebAgreementsAcceptResult, error) {
		return nil, errors.New("failed to parse Developer Portal agreement accept response")
	}
	getAgreementHistoryFn = func(context.Context, *webcore.Client) (*asc.WebAgreementsStatusResult, error) {
		t.Fatal("verification must not run when accept itself failed")
		return nil, errors.New("unexpected history read")
	}
	persistCalls := 0
	persistWebSessionFn = func(*webcore.AuthSession) error {
		persistCalls++
		return nil
	}

	cmd := WebAgreementsAcceptCommand()
	if err := cmd.FlagSet.Parse([]string{"--agreement-id", "XG8DNV4HYY", "--confirm"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	var execErr error
	stdout, _ := captureWebCommandOutput(t, func() {
		execErr = cmd.Exec(context.Background(), nil)
	})
	if execErr == nil || !strings.Contains(execErr.Error(), "parse Developer Portal agreement accept response") {
		t.Fatalf("Exec() error = %v, want ambiguous accept failure", execErr)
	}
	if stdout != "" {
		t.Fatalf("expected no receipt on stdout, got %q", stdout)
	}
	if persistCalls != 1 {
		t.Fatalf("persist calls = %d, want 1 so a later status without --developer-team still inspects the team that may have accepted", persistCalls)
	}
}

func TestWebAgreementsAcceptFailsWhenVerificationReadFails(t *testing.T) {
	stubWebAgreementsSession(t)
	stubWebAgreementsAccept(t, nil, errors.New("portal unavailable"))
	var persistCalls int
	persistWebSessionFn = func(session *webcore.AuthSession) error {
		persistCalls++
		return nil
	}

	cmd := WebAgreementsAcceptCommand()
	if err := cmd.FlagSet.Parse([]string{"--agreement-id", "XG8DNV4HYY", "--confirm"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	var execErr error
	stdout, _ := captureWebCommandOutput(t, func() {
		execErr = cmd.Exec(context.Background(), nil)
	})
	if execErr == nil || !strings.Contains(execErr.Error(), "portal unavailable") || !strings.Contains(execErr.Error(), "accept request succeeded") {
		t.Fatalf("Exec() error = %v, want verification failure that reports the write already happened", execErr)
	}
	if stdout != "" {
		t.Fatalf("expected no receipt on stdout, got %q", stdout)
	}
	if persistCalls != 1 {
		t.Fatalf("persist calls = %d, want 1 so the selected team is retained after a verification failure", persistCalls)
	}
}

func TestWebAgreementsAcceptVerificationGetsFreshTimeout(t *testing.T) {
	stubWebAgreementsSession(t)
	stubWebAgreementsAccept(t, &asc.WebAgreementsStatusResult{
		TeamID:     "TEAM123456",
		Agreements: []asc.WebAgreement{acceptedAgreement("XG8DNV4HYY", "5031")},
	}, nil)

	var acceptDeadline, verifyDeadline time.Time
	origAccept := acceptAgreementsFn
	origHistory := getAgreementHistoryFn
	t.Cleanup(func() {
		acceptAgreementsFn = origAccept
		getAgreementHistoryFn = origHistory
	})
	acceptAgreementsFn = func(ctx context.Context, client *webcore.Client, req webcore.AgreementsAcceptRequest) (*asc.WebAgreementsAcceptResult, error) {
		acceptDeadline, _ = ctx.Deadline()
		time.Sleep(30 * time.Millisecond)
		return origAccept(ctx, client, req)
	}
	getAgreementHistoryFn = func(ctx context.Context, client *webcore.Client) (*asc.WebAgreementsStatusResult, error) {
		verifyDeadline, _ = ctx.Deadline()
		return origHistory(ctx, client)
	}

	cmd := WebAgreementsAcceptCommand()
	if err := cmd.FlagSet.Parse([]string{"--agreement-id", "XG8DNV4HYY", "--confirm", "--output", "json"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	captureWebCommandOutput(t, func() {
		var execErr error
		synctest.Test(t, func(*testing.T) { execErr = cmd.Exec(context.Background(), nil) })
		if execErr != nil {
			t.Fatalf("Exec() error: %v", execErr)
		}
	})
	if acceptDeadline.IsZero() || verifyDeadline.IsZero() {
		t.Fatalf("deadlines accept=%v verify=%v, want both requests bounded by a timeout", acceptDeadline, verifyDeadline)
	}
	if verifyDeadline.Sub(acceptDeadline) != 30*time.Millisecond {
		t.Fatalf("verification deadline %v is not fresh relative to accept deadline %v; the post-mutation read must get its own timeout", verifyDeadline, acceptDeadline)
	}
}

// stubWebAgreementsDownload replaces the download hook and records the
// requested agreement IDs.
func stubWebAgreementsDownload(t *testing.T, download *webcore.AgreementDownload, downloadErr error) *[]string {
	t.Helper()
	orig := downloadAgreementFn
	t.Cleanup(func() { downloadAgreementFn = orig })
	requested := &[]string{}
	downloadAgreementFn = func(ctx context.Context, client *webcore.Client, agreementID string) (*webcore.AgreementDownload, error) {
		*requested = append(*requested, agreementID)
		if downloadErr != nil {
			return nil, downloadErr
		}
		return download, nil
	}
	return requested
}

func sampleAgreementDownload() *webcore.AgreementDownload {
	return &webcore.AgreementDownload{
		AgreementID: "XG8DNV4HYY",
		TeamID:      "TEAM123456",
		Title:       "Apple Developer Program License Agreement",
		Version:     "5031",
		ContentType: "application/pdf",
		Body:        []byte("%PDF-1.7 agreement body"),
	}
}

func TestWebAgreementsDownloadValidationErrors(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{name: "missing agreement id", args: []string{"--out", "agreement.pdf"}, wantErr: "--agreement-id is required"},
		{name: "missing out", args: []string{"--agreement-id", "XG8DNV4HYY"}, wantErr: "--out is required"},
		{name: "blank out", args: []string{"--agreement-id", "XG8DNV4HYY", "--out", "   "}, wantErr: "--out is required"},
		{name: "out is a directory path", args: []string{"--agreement-id", "XG8DNV4HYY", "--out", "downloads/"}, wantErr: "--out must be a file path"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			stubWebAgreementsSession(t)
			requested := stubWebAgreementsDownload(t, sampleAgreementDownload(), nil)

			cmd := WebAgreementsDownloadCommand()
			if err := cmd.FlagSet.Parse(tc.args); err != nil {
				t.Fatalf("parse error: %v", err)
			}
			stdout, stderr := captureWebCommandOutput(t, func() {
				err := cmd.Exec(context.Background(), nil)
				if !errors.Is(err, flag.ErrHelp) {
					t.Fatalf("expected flag.ErrHelp, got %v", err)
				}
			})
			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, tc.wantErr) {
				t.Fatalf("expected stderr to contain %q, got %q", tc.wantErr, stderr)
			}
			if len(*requested) != 0 {
				t.Fatalf("expected no download before validation passes, got %v", *requested)
			}
		})
	}
}

func TestWebAgreementsDownloadWritesFileAndPrintsReceipt(t *testing.T) {
	stubWebAgreementsSession(t)
	requested := stubWebAgreementsDownload(t, sampleAgreementDownload(), nil)

	dir := t.TempDir()
	outPath := filepath.Join(dir, "nested", "agreement.pdf")
	cmd := WebAgreementsDownloadCommand()
	if err := cmd.FlagSet.Parse([]string{"--agreement-id", " XG8DNV4HYY ", "--out", outPath, "--output", "json"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	stdout, stderr := captureWebCommandOutput(t, func() {
		if err := cmd.Exec(context.Background(), nil); err != nil {
			t.Fatalf("Exec() error: %v", err)
		}
	})
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if len(*requested) != 1 || (*requested)[0] != "XG8DNV4HYY" {
		t.Fatalf("download requests = %v, want [XG8DNV4HYY]", *requested)
	}

	body, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read downloaded agreement: %v", err)
	}
	if string(body) != "%PDF-1.7 agreement body" {
		t.Fatalf("downloaded body = %q", body)
	}
	info, err := os.Stat(outPath)
	if err != nil {
		t.Fatalf("stat downloaded agreement: %v", err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
		t.Fatalf("downloaded file mode = %o, want 0600", info.Mode().Perm())
	}

	if strings.Contains(stdout, "%PDF") || strings.Contains(strings.ToLower(stdout), "http") {
		t.Fatalf("stdout must not include file content or URLs: %q", stdout)
	}
	var payload struct {
		AgreementID  string `json:"agreementId"`
		TeamID       string `json:"teamId"`
		Title        string `json:"title"`
		Version      string `json:"version"`
		Path         string `json:"path"`
		BytesWritten int64  `json:"bytesWritten"`
		ContentType  string `json:"contentType"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal stdout %q: %v", stdout, err)
	}
	if payload.AgreementID != "XG8DNV4HYY" || payload.TeamID != "TEAM123456" || payload.Version != "5031" {
		t.Fatalf("receipt identity = %+v", payload)
	}
	if payload.Path != outPath || payload.BytesWritten != int64(len(body)) || payload.ContentType != "application/pdf" {
		t.Fatalf("receipt = %+v, want path %q, %d bytes, application/pdf", payload, outPath, len(body))
	}
}

func TestWebAgreementsDownloadPreservesWhitespaceInOutPath(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("trailing whitespace in file names is not portable on Windows")
	}
	stubWebAgreementsSession(t)
	stubWebAgreementsDownload(t, sampleAgreementDownload(), nil)

	dir := t.TempDir()
	trimmedPath := filepath.Join(dir, "agreement.pdf")
	if err := os.WriteFile(trimmedPath, []byte("keep me"), 0o600); err != nil {
		t.Fatalf("seed existing file: %v", err)
	}
	outPath := trimmedPath + " "
	cmd := WebAgreementsDownloadCommand()
	if err := cmd.FlagSet.Parse([]string{"--agreement-id", "XG8DNV4HYY", "--out", outPath, "--overwrite", "--output", "json"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	stdout, _ := captureWebCommandOutput(t, func() {
		if err := cmd.Exec(context.Background(), nil); err != nil {
			t.Fatalf("Exec() error: %v", err)
		}
	})

	if body, err := os.ReadFile(outPath); err != nil || string(body) != "%PDF-1.7 agreement body" {
		t.Fatalf("selected path %q content = %q, err = %v; want the downloaded agreement", outPath, body, err)
	}
	if body, err := os.ReadFile(trimmedPath); err != nil || string(body) != "keep me" {
		t.Fatalf("trimmed path %q content = %q, err = %v; must not be replaced", trimmedPath, body, err)
	}
	var payload struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal stdout %q: %v", stdout, err)
	}
	if payload.Path != outPath {
		t.Fatalf("receipt path = %q, want the selected path %q unchanged", payload.Path, outPath)
	}
}

func TestWebAgreementsDownloadRefusesOverwriteWithoutFlag(t *testing.T) {
	stubWebAgreementsSession(t)
	requested := stubWebAgreementsDownload(t, sampleAgreementDownload(), nil)

	outPath := filepath.Join(t.TempDir(), "agreement.pdf")
	if err := os.WriteFile(outPath, []byte("keep me"), 0o600); err != nil {
		t.Fatalf("seed existing file: %v", err)
	}

	cmd := WebAgreementsDownloadCommand()
	if err := cmd.FlagSet.Parse([]string{"--agreement-id", "XG8DNV4HYY", "--out", outPath}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	var execErr error
	stdout, _ := captureWebCommandOutput(t, func() {
		execErr = cmd.Exec(context.Background(), nil)
	})
	if execErr == nil || errors.Is(execErr, flag.ErrHelp) {
		t.Fatalf("Exec() error = %v, want overwrite refusal", execErr)
	}
	if !strings.Contains(execErr.Error(), "already exists") || !strings.Contains(execErr.Error(), "--overwrite") {
		t.Fatalf("Exec() error = %q, want existing-file refusal mentioning --overwrite", execErr)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if len(*requested) != 0 {
		t.Fatalf("expected no download when the destination exists, got %v", *requested)
	}
	body, err := os.ReadFile(outPath)
	if err != nil || string(body) != "keep me" {
		t.Fatalf("existing file was modified: %q, %v", body, err)
	}
}

func TestWebAgreementsDownloadOverwritesWithFlag(t *testing.T) {
	stubWebAgreementsSession(t)
	stubWebAgreementsDownload(t, sampleAgreementDownload(), nil)

	outPath := filepath.Join(t.TempDir(), "agreement.pdf")
	if err := os.WriteFile(outPath, []byte("stale"), 0o600); err != nil {
		t.Fatalf("seed existing file: %v", err)
	}

	cmd := WebAgreementsDownloadCommand()
	if err := cmd.FlagSet.Parse([]string{"--agreement-id", "XG8DNV4HYY", "--out", outPath, "--overwrite", "--output", "table"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	stdout, _ := captureWebCommandOutput(t, func() {
		if err := cmd.Exec(context.Background(), nil); err != nil {
			t.Fatalf("Exec() error: %v", err)
		}
	})
	body, err := os.ReadFile(outPath)
	if err != nil || string(body) != "%PDF-1.7 agreement body" {
		t.Fatalf("overwritten file = %q, %v", body, err)
	}
	for _, want := range []string{"Agreement ID", "Path", "Bytes", "Content Type", "XG8DNV4HYY", "application/pdf"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("table output missing %q: %q", want, stdout)
		}
	}
}

func TestWebAgreementsDownloadOverwritesReadOnlyFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("read-only attribute semantics differ on Windows")
	}
	stubWebAgreementsSession(t)
	stubWebAgreementsDownload(t, sampleAgreementDownload(), nil)

	outPath := filepath.Join(t.TempDir(), "agreement.pdf")
	if err := os.WriteFile(outPath, []byte("stale"), 0o400); err != nil {
		t.Fatalf("seed read-only file: %v", err)
	}

	cmd := WebAgreementsDownloadCommand()
	if err := cmd.FlagSet.Parse([]string{"--agreement-id", "XG8DNV4HYY", "--out", outPath, "--overwrite", "--output", "json"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	captureWebCommandOutput(t, func() {
		if err := cmd.Exec(context.Background(), nil); err != nil {
			t.Fatalf("Exec() error: %v; --overwrite must replace a read-only regular file by rename", err)
		}
	})
	body, err := os.ReadFile(outPath)
	if err != nil || string(body) != "%PDF-1.7 agreement body" {
		t.Fatalf("overwritten file = %q, %v", body, err)
	}
}

func TestWebAgreementsDownloadRejectsExistingDirectory(t *testing.T) {
	stubWebAgreementsSession(t)
	requested := stubWebAgreementsDownload(t, sampleAgreementDownload(), nil)

	dir := t.TempDir()
	cmd := WebAgreementsDownloadCommand()
	if err := cmd.FlagSet.Parse([]string{"--agreement-id", "XG8DNV4HYY", "--out", dir, "--overwrite"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	var execErr error
	captureWebCommandOutput(t, func() {
		execErr = cmd.Exec(context.Background(), nil)
	})
	if execErr == nil {
		t.Fatal("Exec() error = nil, want directory destination rejection")
	}
	if len(*requested) != 0 {
		t.Fatalf("expected no download for a directory destination, got %v", *requested)
	}
}

func TestWebAgreementsDownloadPersistsTeamBeforeSaveFailure(t *testing.T) {
	stubWebAgreementsSession(t)
	outPath := filepath.Join(t.TempDir(), "agreement.pdf")
	origDownload := downloadAgreementFn
	t.Cleanup(func() { downloadAgreementFn = origDownload })
	downloadAgreementFn = func(ctx context.Context, client *webcore.Client, agreementID string) (*webcore.AgreementDownload, error) {
		if err := os.WriteFile(outPath, []byte("stolen destination"), 0o600); err != nil {
			t.Fatalf("seed destination: %v", err)
		}
		return sampleAgreementDownload(), nil
	}
	persistCalls := 0
	persistWebSessionFn = func(*webcore.AuthSession) error {
		persistCalls++
		return nil
	}

	cmd := WebAgreementsDownloadCommand()
	if err := cmd.FlagSet.Parse([]string{"--agreement-id", "XG8DNV4HYY", "--out", outPath}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	var execErr error
	stdout, _ := captureWebCommandOutput(t, func() {
		execErr = cmd.Exec(context.Background(), nil)
	})
	if execErr == nil || !strings.Contains(execErr.Error(), "downloaded but saving") {
		t.Fatalf("Exec() error = %v, want local save failure after download", execErr)
	}
	if stdout != "" {
		t.Fatalf("expected no receipt on stdout, got %q", stdout)
	}
	if persistCalls != 1 {
		t.Fatalf("persist calls = %d, want 1 so a later retry without --developer-team still targets the team that produced the download", persistCalls)
	}
}

func TestWebAgreementsDownloadSurfacesClientErrorWithoutFile(t *testing.T) {
	stubWebAgreementsSession(t)
	stubWebAgreementsDownload(t, nil, errors.New("agreement download was redirected to cdn.example.test instead of the Developer Portal"))

	outPath := filepath.Join(t.TempDir(), "agreement.pdf")
	cmd := WebAgreementsDownloadCommand()
	if err := cmd.FlagSet.Parse([]string{"--agreement-id", "XG8DNV4HYY", "--out", outPath}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	var execErr error
	stdout, _ := captureWebCommandOutput(t, func() {
		execErr = cmd.Exec(context.Background(), nil)
	})
	if execErr == nil || !strings.Contains(execErr.Error(), "web agreements download failed") || !strings.Contains(execErr.Error(), "redirected") {
		t.Fatalf("Exec() error = %v, want wrapped download failure", execErr)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if _, err := os.Lstat(outPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected no file after a failed download, got %v", err)
	}
}

func TestWebAgreementsStatusNonJSONOutputIncludesBannerOnlyPendingState(t *testing.T) {
	stubWebAgreementsSession(t)

	origStatus := getAgreementsStatusFn
	t.Cleanup(func() { getAgreementsStatusFn = origStatus })
	getAgreementsStatusFn = func(ctx context.Context, client *webcore.Client) (*asc.WebAgreementsStatusResult, error) {
		return &asc.WebAgreementsStatusResult{
			TeamID:   "TEAM123456",
			Pending:  true,
			Warnings: []string{"Review the unresolved agreement alert."},
			ContractMessages: []asc.WebAgreementContractMessage{{
				ID:      "contract_message",
				Group:   "Alert",
				Subject: "Apple Developer Program License Agreement Updated",
			}},
		}, nil
	}

	for _, format := range []string{"table", "markdown"} {
		t.Run(format, func(t *testing.T) {
			cmd := WebAgreementsStatusCommand()
			if err := cmd.FlagSet.Parse([]string{"--output", format}); err != nil {
				t.Fatalf("parse error: %v", err)
			}
			stdout, _ := captureWebCommandOutput(t, func() {
				if err := cmd.Exec(context.Background(), nil); err != nil {
					t.Fatalf("Exec() error: %v", err)
				}
			})
			for _, want := range []string{"TEAM123456", "true", "Apple Developer Program License Agreement Updated", "Pending or alert", "Pending Agreement IDs", "none", "Review the unresolved agreement alert."} {
				if !strings.Contains(stdout, want) {
					t.Fatalf("%s output missing %q: %q", format, want, stdout)
				}
			}
		})
	}
}

func TestWebAgreementsStatusPrintsJSON(t *testing.T) {
	stubWebAgreementsSession(t)

	origStatus := getAgreementsStatusFn
	t.Cleanup(func() { getAgreementsStatusFn = origStatus })
	getAgreementsStatusFn = func(ctx context.Context, client *webcore.Client) (*asc.WebAgreementsStatusResult, error) {
		return &asc.WebAgreementsStatusResult{
			TeamID:  "TEAM123456",
			Pending: true,
			ContractMessages: []asc.WebAgreementContractMessage{{
				ID:      "contract_message",
				Group:   "Alert",
				Subject: "Apple Developer Program License Agreement Updated",
			}},
			Agreements: []asc.WebAgreement{{
				AgreementID:               "XG8DNV4HYY",
				Title:                     "Apple Developer Program License Agreement",
				Status:                    "active",
				Version:                   "5031",
				IsProgramLicenseAgreement: true,
				Pending:                   true,
				DateEffective:             "2026-08-14T09:38:53Z",
				DateAgreeBy:               "2026-10-01T23:59:59Z",
			}},
		}, nil
	}

	cmd := WebAgreementsStatusCommand()
	if err := cmd.FlagSet.Parse([]string{"--output", "json"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	stdout, _ := captureWebCommandOutput(t, func() {
		if err := cmd.Exec(context.Background(), nil); err != nil {
			t.Fatalf("Exec() error: %v", err)
		}
	})

	var payload struct {
		TeamID           string `json:"teamId"`
		Pending          bool   `json:"pending"`
		ContractMessages []struct {
			Subject string `json:"subject"`
		} `json:"contractMessages"`
		Agreements []struct {
			AgreementID string `json:"agreementId"`
			Pending     bool   `json:"pending"`
		} `json:"agreements"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal stdout %q: %v", stdout, err)
	}
	if payload.TeamID != "TEAM123456" || !payload.Pending {
		t.Fatalf("payload = %+v, want pending TEAM123456", payload)
	}
	if len(payload.ContractMessages) != 1 || !strings.Contains(payload.ContractMessages[0].Subject, "License Agreement Updated") {
		t.Fatalf("contractMessages = %+v, want banner entry", payload.ContractMessages)
	}
	if len(payload.Agreements) != 1 || payload.Agreements[0].AgreementID != "XG8DNV4HYY" || !payload.Agreements[0].Pending {
		t.Fatalf("agreements = %+v, want pending XG8DNV4HYY", payload.Agreements)
	}
}

func TestWebAgreementsAcceptCallsClient(t *testing.T) {
	stubWebAgreementsSession(t)
	requests, statusCalls := stubWebAgreementsAccept(t, &asc.WebAgreementsStatusResult{
		TeamID:     "TEAM123456",
		Agreements: []asc.WebAgreement{acceptedAgreement("XG8DNV4HYY", "5031")},
	}, nil)

	cmd := WebAgreementsAcceptCommand()
	if err := cmd.FlagSet.Parse([]string{"--agreement-id", "XG8DNV4HYY", "--confirm", "--output", "json"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	stdout, _ := captureWebCommandOutput(t, func() {
		if err := cmd.Exec(context.Background(), nil); err != nil {
			t.Fatalf("Exec() error: %v", err)
		}
	})

	if len(*requests) != 1 || len((*requests)[0].AgreementIDs) != 1 || (*requests)[0].AgreementIDs[0] != "XG8DNV4HYY" {
		t.Fatalf("accept requests = %+v, want one request for agreement XG8DNV4HYY", *requests)
	}
	if *statusCalls != 1 {
		t.Fatalf("status re-read calls = %d, want 1", *statusCalls)
	}

	var payload struct {
		TeamID       string   `json:"teamId"`
		AgreementIDs []string `json:"agreementIds"`
		Status       string   `json:"status"`
		Verified     bool     `json:"verified"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal stdout %q: %v", stdout, err)
	}
	if payload.Status != "accepted" || payload.TeamID != "TEAM123456" || len(payload.AgreementIDs) != 1 || !payload.Verified {
		t.Fatalf("payload = %+v, want verified accepted receipt", payload)
	}
}

func TestWebAgreementsAcceptTableOutputShowsVerification(t *testing.T) {
	stubWebAgreementsSession(t)
	stubWebAgreementsAccept(t, &asc.WebAgreementsStatusResult{
		TeamID:     "TEAM123456",
		Agreements: []asc.WebAgreement{acceptedAgreement("XG8DNV4HYY", "5031"), acceptedAgreement("AB12CD34EF", "12")},
	}, nil)

	cmd := WebAgreementsAcceptCommand()
	if err := cmd.FlagSet.Parse([]string{"--agreement-id", "XG8DNV4HYY", "--agreement-id", "AB12CD34EF", "--confirm", "--output", "table"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	stdout, _ := captureWebCommandOutput(t, func() {
		if err := cmd.Exec(context.Background(), nil); err != nil {
			t.Fatalf("Exec() error: %v", err)
		}
	})
	for _, want := range []string{"Verified", "XG8DNV4HYY", "AB12CD34EF", "accepted", "true", "2026-08-19T16:56:47Z"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("table output missing %q: %q", want, stdout)
		}
	}
	if strings.Contains(stdout, "XG8DNV4HYY, AB12CD34EF") {
		t.Fatalf("table output joins agreements into one row; want one row per agreement: %q", stdout)
	}
}
