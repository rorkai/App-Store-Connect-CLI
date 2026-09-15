package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/readonly"
)

const (
	developerPortalAgreementHistoryPath = "/services-account/QH65B2/account/getAgreementHistory"
	developerPortalAcceptAgreementsPath = "/services-account/QH65B2/account/acceptAgreements"
	olympusContractMessagesPath         = "/contractMessages"
)

// AgreementsAcceptRequest accepts one or more Developer Portal agreements.
type AgreementsAcceptRequest struct {
	AgreementIDs []string
}

// developerAgreementsEnvelope is the QH65B2 account services response shape.
// These endpoints answer HTTP 200 even for failures; resultCode carries the
// outcome (0 success; for example 3050 missing team, 3100 unknown team).
type developerAgreementsEnvelope struct {
	ResultCode   *int                       `json:"resultCode"`
	ResultString string                     `json:"resultString"`
	UserString   string                     `json:"userString"`
	Agreements   []developerAgreementRecord `json:"agreements"`
}

type developerAgreementRecord struct {
	AgreementID          string `json:"agreementId"`
	Title                string `json:"title"`
	Status               string `json:"status"`
	Version              string `json:"version"`
	DateEffective        int64  `json:"dateEffective"`
	DateAccepted         int64  `json:"dateAccepted"`
	DateAgreeBy          int64  `json:"dateAgreeBy"`
	IsAgreementPLA       bool   `json:"isAgreementPLA"`
	AgreementDownloadURL string `json:"agreementDownloadUrl"`
}

// DeveloperPortalAgreementsResultError reports an agreement-services failure
// returned inside an otherwise successful HTTP response.
type DeveloperPortalAgreementsResultError struct {
	ResultCode int
	Message    string
}

func (e *DeveloperPortalAgreementsResultError) Error() string {
	return fmt.Sprintf("developer portal agreements request failed (resultCode %d): %s", e.ResultCode, e.Message)
}

// AgreementDownload is the fetched content of one Developer Portal agreement.
// It never carries the download URL so callers cannot print it by accident.
type AgreementDownload struct {
	AgreementID string
	TeamID      string
	Title       string
	Version     string
	ContentType string
	Body        []byte
}

// maxAgreementDownloadBytes bounds an agreement download so a misbehaving
// endpoint cannot exhaust memory; program agreements are a few megabytes.
const maxAgreementDownloadBytes int64 = 64 << 20

// GetAgreementsStatus reports the App Store Connect agreement banner and the
// team's Developer Portal agreement history in one pending-aware summary.
func (c *Client) GetAgreementsStatus(ctx context.Context) (*asc.WebAgreementsStatusResult, error) {
	messages, err := c.getContractMessages(ctx)
	if err != nil {
		return nil, err
	}
	result := &asc.WebAgreementsStatusResult{
		Pending:          len(messages) > 0,
		ContractMessages: messages,
	}
	teamID, envelope, err := c.fetchAgreementHistory(ctx)
	result.TeamID = teamID
	if err != nil {
		return result, err
	}
	c.applyAgreementHistory(result, envelope)
	return result, nil
}

// GetAgreementHistory reads only the team's Developer Portal agreement history.
// It skips the App Store Connect contract-message banner so post-mutation
// verification does not fail on that unrelated read; ContractMessages is
// always empty in the result.
func (c *Client) GetAgreementHistory(ctx context.Context) (*asc.WebAgreementsStatusResult, error) {
	teamID, envelope, err := c.fetchAgreementHistory(ctx)
	if err != nil {
		return nil, err
	}
	result := &asc.WebAgreementsStatusResult{
		TeamID:           teamID,
		ContractMessages: []asc.WebAgreementContractMessage{},
	}
	c.applyAgreementHistory(result, envelope)
	return result, nil
}

func (c *Client) applyAgreementHistory(result *asc.WebAgreementsStatusResult, envelope *developerAgreementsEnvelope) {
	agreements := make([]asc.WebAgreement, 0, len(envelope.Agreements))
	for _, record := range envelope.Agreements {
		agreement := c.newAgreement(record)
		if agreement.Pending {
			result.Pending = true
		}
		agreements = append(agreements, agreement)
	}
	result.Agreements = agreements
}

// fetchAgreementHistory bootstraps the Developer Portal session and returns the
// selected team together with its agreement history envelope. The team ID is
// returned even when the history request fails so callers can report it.
func (c *Client) fetchAgreementHistory(ctx context.Context) (string, *developerAgreementsEnvelope, error) {
	if err := c.ensureDeveloperPortalSession(ctx); err != nil {
		return "", nil, err
	}
	teamID := c.developerPortalTeamID()
	if teamID == "" {
		return "", nil, fmt.Errorf("developer portal team is not selected; %s", developerPortalAuthHint)
	}
	// Agreement history is a POST-shaped read on the Developer Portal.
	envelope, err := c.doDeveloperPortalAgreementsRequest(readonly.WithReadIntent(ctx), developerPortalAgreementHistoryPath, map[string]string{"teamId": teamID})
	if err != nil {
		return teamID, nil, err
	}
	return teamID, envelope, nil
}

// DownloadAgreement fetches the content of one agreement from the team's
// Developer Portal agreement history. The reported download URL must be an
// HTTPS URL on the Developer Portal origin, and redirects to any other origin
// or scheme are rejected. Error messages never include the URL because it may
// be signed.
func (c *Client) DownloadAgreement(ctx context.Context, agreementID string) (*AgreementDownload, error) {
	agreementID = strings.TrimSpace(agreementID)
	if agreementID == "" {
		return nil, fmt.Errorf("agreement id is required")
	}
	if c == nil || c.httpClient == nil {
		return nil, fmt.Errorf("web client is not configured for Developer Portal")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	teamID, envelope, err := c.fetchAgreementHistory(ctx)
	if err != nil {
		return nil, err
	}
	var agreement *asc.WebAgreement
	for _, record := range envelope.Agreements {
		candidate := c.newAgreement(record)
		if candidate.AgreementID == agreementID {
			agreement = &candidate
			break
		}
	}
	if agreement == nil {
		return nil, fmt.Errorf("agreement %q was not found in the team's agreement history; run 'asc web agreements status' to list agreement IDs", agreementID)
	}
	if strings.TrimSpace(agreement.DownloadURL) == "" {
		return nil, fmt.Errorf("agreement %q does not report a download URL", agreementID)
	}

	portalOrigin, err := url.Parse(c.developerPortalOrigin())
	if err != nil {
		return nil, fmt.Errorf("invalid Developer Portal base URL: %w", err)
	}
	target, err := url.Parse(agreement.DownloadURL)
	if err != nil {
		return nil, fmt.Errorf("agreement %q reports an invalid download URL", agreementID)
	}
	if err := validateAgreementDownloadTarget(portalOrigin, target, "download URL"); err != nil {
		return nil, err
	}

	if err := c.waitForRateLimit(ctx); err != nil {
		return nil, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, target.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create agreement download request")
	}
	request.Header.Set("Accept", "application/pdf, application/octet-stream, */*;q=0.1")
	request.Header.Set("Referer", developerPortalBaseURL+"/account")
	request.Header.Set("User-Agent", "App-Store-Connect-CLI")
	setModifiedCookieHeader(c.httpClient, request)

	httpClient := *c.httpClient
	previousCheckRedirect := httpClient.CheckRedirect
	httpClient.CheckRedirect = func(redirect *http.Request, via []*http.Request) error {
		if err := validateAgreementDownloadTarget(portalOrigin, redirect.URL, "redirect"); err != nil {
			return err
		}
		if len(via) >= 10 {
			return fmt.Errorf("agreement download stopped after 10 redirects")
		}
		if previousCheckRedirect != nil {
			if err := previousCheckRedirect(redirect, via); err != nil {
				return err
			}
			// The wrapped policy receives the mutable upcoming request and may
			// have rewritten its URL; never send the session to an unchecked target.
			return validateAgreementDownloadTarget(portalOrigin, redirect.URL, "redirect")
		}
		return nil
	}
	response, err := httpClient.Do(request)
	if err != nil {
		return nil, redactedAgreementDownloadError(err)
	}
	defer func() { _ = response.Body.Close() }()

	if response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden {
		return nil, developerPortalSessionError(response.StatusCode)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, &APIError{
			Status:         response.StatusCode,
			AppleRequestID: extractAppleRequestID(response.Header),
			CorrelationKey: strings.TrimSpace(response.Header.Get("X-Apple-Jingle-Correlation-Key")),
		}
	}
	contentType := strings.TrimSpace(response.Header.Get("Content-Type"))
	if mediaType, _, _ := mime.ParseMediaType(contentType); strings.EqualFold(mediaType, "text/html") {
		return nil, fmt.Errorf("developer portal returned an HTML page instead of agreement %q content; %s", agreementID, developerPortalAuthHint)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, maxAgreementDownloadBytes+1))
	if err != nil {
		return nil, fmt.Errorf("failed to read agreement download response: %w", err)
	}
	if int64(len(body)) > maxAgreementDownloadBytes {
		return nil, fmt.Errorf("agreement %q content exceeds the %d byte download limit", agreementID, maxAgreementDownloadBytes)
	}
	if len(body) == 0 {
		return nil, fmt.Errorf("developer portal returned empty content for agreement %q (HTTP %d); nothing was saved", agreementID, response.StatusCode)
	}
	return &AgreementDownload{
		AgreementID: agreement.AgreementID,
		TeamID:      teamID,
		Title:       agreement.Title,
		Version:     agreement.Version,
		ContentType: contentType,
		Body:        body,
	}, nil
}

// validateAgreementDownloadTarget requires an HTTPS URL on the Developer Portal
// origin. Messages name only the host so a signed URL is never echoed.
func validateAgreementDownloadTarget(portalOrigin, target *url.URL, kind string) error {
	if target == nil || target.Hostname() == "" {
		return fmt.Errorf("agreement %s is missing a host", kind)
	}
	if !strings.EqualFold(target.Scheme, "https") {
		return fmt.Errorf("agreement %s to %s must use https", kind, target.Hostname())
	}
	if !sameURLOrigin(portalOrigin, target) {
		return fmt.Errorf("agreement %s targets %s instead of the Developer Portal origin %s", kind, target.Host, portalOrigin.Host)
	}
	return nil
}

// redactedAgreementDownloadError strips the request URL that net/http embeds
// in *url.Error so a signed download URL never reaches logs or stderr.
func redactedAgreementDownloadError(err error) error {
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		if errors.Is(urlErr.Err, context.Canceled) || errors.Is(urlErr.Err, context.DeadlineExceeded) {
			return urlErr.Err
		}
		// net/http rejects an unparsable Location before CheckRedirect runs and
		// quotes the raw header, which may carry signed query data.
		if strings.Contains(urlErr.Err.Error(), "Location header") {
			return fmt.Errorf("agreement download request failed: redirect carried a malformed Location header")
		}
		return fmt.Errorf("agreement download request failed: %w", urlErr.Err)
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	return fmt.Errorf("agreement download request failed: %w", err)
}

// AcceptAgreements accepts the given Developer Portal agreements for the web
// session's team. Apple only allows the Account Holder to accept agreements.
func (c *Client) AcceptAgreements(ctx context.Context, req AgreementsAcceptRequest) (*asc.WebAgreementsAcceptResult, error) {
	agreementIDs := make([]string, 0, len(req.AgreementIDs))
	for _, id := range req.AgreementIDs {
		id = strings.TrimSpace(id)
		if id != "" {
			agreementIDs = append(agreementIDs, id)
		}
	}
	if len(agreementIDs) == 0 {
		return nil, fmt.Errorf("at least one agreement id is required")
	}

	if err := c.ensureDeveloperPortalSession(ctx); err != nil {
		return nil, err
	}
	teamID := c.developerPortalTeamID()
	if teamID == "" {
		return nil, fmt.Errorf("developer portal team is not selected; %s", developerPortalAuthHint)
	}
	payload := struct {
		TeamID       string   `json:"teamId"`
		AgreementIDs []string `json:"agreementIds"`
	}{TeamID: teamID, AgreementIDs: agreementIDs}
	envelope, err := c.doDeveloperPortalAgreementsRequest(ctx, developerPortalAcceptAgreementsPath, payload)
	if err != nil {
		return nil, err
	}

	agreements := make([]asc.WebAgreement, 0, len(envelope.Agreements))
	for _, record := range envelope.Agreements {
		agreements = append(agreements, c.newAgreement(record))
	}
	return &asc.WebAgreementsAcceptResult{
		TeamID:       teamID,
		AgreementIDs: agreementIDs,
		Status:       "accepted",
		Agreements:   agreements,
	}, nil
}

func (c *Client) getContractMessages(ctx context.Context) ([]asc.WebAgreementContractMessage, error) {
	body, err := c.doOlympusGet(ctx, olympusContractMessagesPath)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch App Store Connect contract messages: %w", err)
	}
	var messages []asc.WebAgreementContractMessage
	if err := json.Unmarshal(body, &messages); err != nil {
		return nil, fmt.Errorf("failed to parse App Store Connect contract messages: %w", err)
	}
	return messages, nil
}

func (c *Client) doDeveloperPortalAgreementsRequest(ctx context.Context, path string, payload any) (*developerAgreementsEnvelope, error) {
	headers := developerPortalAgreementsHeaders()
	if err := c.applyDeveloperPortalCSRF(headers, false); err != nil {
		return nil, err
	}
	body, err := c.doDeveloperPortalHTTPAndCapture(ctx, http.MethodPost, c.developerPortalOrigin()+path, payload, headers)
	if err != nil {
		return nil, err
	}
	var envelope developerAgreementsEnvelope
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, fmt.Errorf("failed to parse Developer Portal agreements response: %w", err)
	}
	if envelope.ResultCode == nil {
		return nil, fmt.Errorf("developer portal agreements response is missing resultCode")
	}
	if *envelope.ResultCode != 0 {
		message := strings.TrimSpace(envelope.UserString)
		if message == "" {
			message = strings.TrimSpace(envelope.ResultString)
		}
		if message == "" {
			message = "unknown Developer Portal error"
		}
		return nil, &DeveloperPortalAgreementsResultError{ResultCode: *envelope.ResultCode, Message: message}
	}
	return &envelope, nil
}

func (c *Client) newAgreement(record developerAgreementRecord) asc.WebAgreement {
	downloadURL := strings.TrimSpace(record.AgreementDownloadURL)
	if downloadURL != "" && strings.HasPrefix(downloadURL, "/") {
		downloadURL = c.developerPortalOrigin() + downloadURL
	}
	return asc.WebAgreement{
		AgreementID:               strings.TrimSpace(record.AgreementID),
		Title:                     strings.TrimSpace(record.Title),
		Status:                    strings.TrimSpace(record.Status),
		Version:                   strings.TrimSpace(record.Version),
		IsProgramLicenseAgreement: record.IsAgreementPLA,
		Pending:                   record.DateAccepted < record.DateEffective,
		DateEffective:             formatAgreementDate(record.DateEffective),
		DateAccepted:              formatAgreementDate(record.DateAccepted),
		DateAgreeBy:               formatAgreementDate(record.DateAgreeBy),
		DownloadURL:               downloadURL,
	}
}

func formatAgreementDate(milliseconds int64) string {
	if milliseconds <= 0 {
		return ""
	}
	return time.UnixMilli(milliseconds).UTC().Format(time.RFC3339)
}

func developerPortalAgreementsHeaders() http.Header {
	headers := make(http.Header)
	headers.Set("Accept", "application/json, text/javascript, */*; q=0.01")
	headers.Set("Content-Type", "application/json")
	headers.Set("Referer", developerPortalBaseURL+"/account")
	headers.Set("User-Agent", "App-Store-Connect-CLI")
	headers.Set("X-Requested-With", "XMLHttpRequest")
	return headers
}
