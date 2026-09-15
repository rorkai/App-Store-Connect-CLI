package web

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/readonly"
)

const (
	developerPortalBaseURL    = "https://developer.apple.com"
	developerPortalLegacyPath = "/services-account/QH65B2"
	developerPortalTeamsPath  = developerPortalLegacyPath + "/account/listTeams.action"
	developerServicesPath     = "/services-account/v1"
	developerPortalAuthHint   = "run 'asc web auth logout --apple-id EMAIL', then 'asc web auth login --apple-id EMAIL', and try again"
)

type developerPortalTeam struct {
	TeamID string `json:"teamId"`
	Name   string `json:"name"`
	Status string `json:"status"`
}

type developerPortalTeamsResponse struct {
	Teams []developerPortalTeam `json:"teams"`
	Data  struct {
		Teams []developerPortalTeam `json:"teams"`
	} `json:"data"`
}

type developerPortalProxyReadRequest struct {
	URLEncodedQueryParams string `json:"urlEncodedQueryParams"`
	TeamID                string `json:"teamId"`
}

type developerPortalTeamSelection struct {
	Selector         string
	CachedTeamID     string
	PublicProviderID string
	ProviderName     string
}

// SetDeveloperTeamSelector sets the explicit Developer Portal team ID or exact
// team name for subsequent portal requests. An empty selector leaves matching
// to the cached team or the selected App Store Connect provider.
func (c *Client) SetDeveloperTeamSelector(selector string) {
	if c == nil {
		return
	}
	c.developerSessionMu.Lock()
	defer c.developerSessionMu.Unlock()
	c.developerTeamSelector = strings.TrimSpace(selector)
}

func (c *Client) currentDeveloperTeamSelector() string {
	if c == nil {
		return ""
	}
	c.developerSessionMu.Lock()
	defer c.developerSessionMu.Unlock()
	return c.developerTeamSelector
}

func (c *Client) rememberDeveloperPortalTeam(teamID string) {
	teamID = strings.TrimSpace(teamID)
	c.developerSessionMu.Lock()
	c.developerTeamID = teamID
	session := c.session
	c.developerSessionMu.Unlock()
	if session != nil {
		session.DeveloperTeamID = teamID
	}
}

func (c *Client) ensureDeveloperPortalSession(ctx context.Context) error {
	// The App Store Connect SRP session becomes usable by Developer Portal only
	// after its legacy team endpoint establishes Portal team and CSRF context.
	headers := developerPortalHeaders("")
	headers.Set("Accept", "application/json, text/javascript, */*; q=0.01")
	headers.Set("Content-Type", "application/x-www-form-urlencoded")
	// Listing teams only establishes Portal context; it changes nothing.
	body, response, err := c.doDeveloperPortalHTTP(readonly.WithReadIntent(ctx), http.MethodPost, c.developerPortalOrigin()+developerPortalTeamsPath, nil, headers)
	if err != nil {
		return err
	}
	c.captureDeveloperCSRFTokens(response.Header)
	if response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden {
		return developerPortalSessionError(response.StatusCode)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return &APIError{Status: response.StatusCode, AppleRequestID: extractAppleRequestID(response.Header), rawBody: body}
	}
	portalURL, parseErr := url.Parse(c.developerPortalOrigin())
	if parseErr != nil {
		return fmt.Errorf("invalid Developer Portal base URL: %w", parseErr)
	}
	if response.Request != nil && response.Request.URL != nil && !sameURLOrigin(portalURL, response.Request.URL) {
		return fmt.Errorf("authentication redirected to %s instead of Developer Portal; %s", response.Request.URL.Host, developerPortalAuthHint)
	}

	var payload developerPortalTeamsResponse
	if err := json.Unmarshal(body, &payload); err != nil {
		return fmt.Errorf("failed to parse Developer Portal teams response: %w", err)
	}
	teams := payload.Teams
	if len(teams) == 0 {
		teams = payload.Data.Teams
	}
	team, err := resolveDeveloperPortalTeam(teams, developerPortalTeamSelection{
		Selector:         c.currentDeveloperTeamSelector(),
		CachedTeamID:     c.developerPortalTeamID(),
		PublicProviderID: c.publicProviderID,
		ProviderName:     c.providerName,
	})
	if err != nil {
		return err
	}
	c.rememberDeveloperPortalTeam(team.TeamID)
	return nil
}

func selectDeveloperPortalTeam(teams []developerPortalTeam, publicProviderID, providerName string) (developerPortalTeam, error) {
	return resolveDeveloperPortalTeam(teams, developerPortalTeamSelection{
		PublicProviderID: publicProviderID,
		ProviderName:     providerName,
	})
}

func resolveDeveloperPortalTeam(teams []developerPortalTeam, selection developerPortalTeamSelection) (developerPortalTeam, error) {
	valid := make([]developerPortalTeam, 0, len(teams))
	for _, team := range teams {
		team.TeamID = strings.TrimSpace(team.TeamID)
		team.Name = strings.TrimSpace(team.Name)
		if team.TeamID != "" {
			valid = append(valid, team)
		}
	}
	if len(valid) == 0 {
		return developerPortalTeam{}, fmt.Errorf("apple account has no Developer Portal team; a paid Apple Developer Program membership may be required")
	}

	if selector := strings.TrimSpace(selection.Selector); selector != "" {
		return matchDeveloperPortalTeamSelector(valid, selector)
	}

	if cachedID := strings.TrimSpace(selection.CachedTeamID); cachedID != "" {
		for _, team := range valid {
			if strings.EqualFold(cachedID, team.TeamID) {
				return team, nil
			}
		}
	}

	publicProviderID := strings.TrimSpace(selection.PublicProviderID)
	if publicProviderID != "" {
		for _, team := range valid {
			if strings.EqualFold(publicProviderID, team.TeamID) {
				return team, nil
			}
		}
	}

	providerName := strings.TrimSpace(selection.ProviderName)
	if providerName != "" {
		var exactName []developerPortalTeam
		for _, team := range valid {
			if strings.EqualFold(providerName, team.Name) {
				exactName = append(exactName, team)
			}
		}
		if len(exactName) == 1 {
			return exactName[0], nil
		}
		if len(exactName) > 1 {
			return developerPortalTeam{}, ambiguousDeveloperPortalSelectorError(providerName, exactName)
		}
		var prefixMatches []developerPortalTeam
		for _, team := range valid {
			if team.Name != "" && strings.HasPrefix(strings.ToLower(providerName), strings.ToLower(team.Name)) {
				prefixMatches = append(prefixMatches, team)
			}
		}
		if len(prefixMatches) == 1 {
			return prefixMatches[0], nil
		}
	}

	if len(valid) == 1 {
		return valid[0], nil
	}
	return developerPortalTeam{}, ambiguousDeveloperPortalTeamError(providerName, valid)
}

func matchDeveloperPortalTeamSelector(teams []developerPortalTeam, selector string) (developerPortalTeam, error) {
	var idMatches []developerPortalTeam
	for _, team := range teams {
		if strings.EqualFold(selector, team.TeamID) {
			idMatches = append(idMatches, team)
		}
	}
	if len(idMatches) == 1 {
		return idMatches[0], nil
	}
	if len(idMatches) > 1 {
		return developerPortalTeam{}, ambiguousDeveloperPortalSelectorError(selector, idMatches)
	}

	var nameMatches []developerPortalTeam
	for _, team := range teams {
		if team.Name != "" && strings.EqualFold(selector, team.Name) {
			nameMatches = append(nameMatches, team)
		}
	}
	if len(nameMatches) == 1 {
		return nameMatches[0], nil
	}
	if len(nameMatches) > 1 {
		return developerPortalTeam{}, ambiguousDeveloperPortalSelectorError(selector, nameMatches)
	}
	return developerPortalTeam{}, unknownDeveloperPortalTeamError(selector, teams)
}

func unknownDeveloperPortalTeamError(selector string, teams []developerPortalTeam) error {
	return fmt.Errorf("unknown Developer Portal team %q; pass --developer-team with one of: %s", selector, formatDeveloperPortalTeamList(teams))
}

func ambiguousDeveloperPortalSelectorError(selector string, teams []developerPortalTeam) error {
	return fmt.Errorf("developer portal team %q matches more than one team; pass --developer-team with one of: %s", selector, formatDeveloperPortalTeamList(teams))
}

func ambiguousDeveloperPortalTeamError(providerName string, teams []developerPortalTeam) error {
	if strings.TrimSpace(providerName) != "" {
		return fmt.Errorf("could not match App Store Connect provider %q to a Developer Portal team; pass --developer-team with one of: %s", providerName, formatDeveloperPortalTeamList(teams))
	}
	return fmt.Errorf("apple account belongs to %d Developer Portal teams and none matches the selected App Store Connect provider; pass --developer-team with one of: %s", len(teams), formatDeveloperPortalTeamList(teams))
}

func formatDeveloperPortalTeamList(teams []developerPortalTeam) string {
	listed := append([]developerPortalTeam(nil), teams...)
	sort.Slice(listed, func(i, j int) bool {
		if listed[i].TeamID == listed[j].TeamID {
			return listed[i].Name < listed[j].Name
		}
		return listed[i].TeamID < listed[j].TeamID
	})
	parts := make([]string, 0, len(listed))
	for _, team := range listed {
		if team.Name == "" {
			parts = append(parts, team.TeamID)
			continue
		}
		parts = append(parts, fmt.Sprintf("%s (%s)", team.TeamID, team.Name))
	}
	return strings.Join(parts, ", ")
}

func developerPortalHeaders(bundleID string) http.Header {
	headers := make(http.Header)
	headers.Set("Accept", "application/vnd.api+json, application/json")
	headers.Set("Content-Type", "application/vnd.api+json")
	headers.Set("Referer", developerPortalBaseURL+"/account/resources/identifiers/list")
	if strings.TrimSpace(bundleID) != "" {
		headers.Set("Referer", developerPortalBaseURL+"/account/resources/identifiers/bundleId/edit/"+url.PathEscape(bundleID))
	}
	headers.Set("User-Agent", "App-Store-Connect-CLI")
	headers.Set("X-Requested-With", "XMLHttpRequest")
	return headers
}

func (c *Client) developerPortalOrigin() string {
	if c != nil && strings.TrimSpace(c.developerPortalURL) != "" {
		return strings.TrimRight(strings.TrimSpace(c.developerPortalURL), "/")
	}
	return developerPortalBaseURL
}

func (c *Client) doDeveloperPortalProxyRead(ctx context.Context, path string, query url.Values, headers http.Header) ([]byte, error) {
	// Developer Portal's cookie-authenticated v1 API proxies logical GETs as
	// POSTs carrying the team and encoded query in the request body.
	teamID := c.developerPortalTeamID()
	if teamID == "" {
		return nil, fmt.Errorf("developer portal team is not selected; %s", developerPortalAuthHint)
	}
	headers.Set("X-HTTP-Method-Override", http.MethodGet)
	return c.doDeveloperPortalRequest(ctx, http.MethodPost, path, developerPortalProxyReadRequest{
		URLEncodedQueryParams: query.Encode(),
		TeamID:                teamID,
	}, headers, false)
}

func (c *Client) doDeveloperPortalRequest(ctx context.Context, method, path string, body any, headers http.Header, requireCSRF bool) ([]byte, error) {
	if err := c.applyDeveloperPortalCSRF(headers, requireCSRF); err != nil {
		return nil, err
	}
	return c.doDeveloperPortalHTTPAndCapture(ctx, method, c.developerPortalOrigin()+developerServicesPath+path, body, headers)
}

func (c *Client) doDeveloperPortalLegacyFormRequest(ctx context.Context, path string, values url.Values, requireCSRF bool) ([]byte, error) {
	headers := developerPortalHeaders("")
	headers.Set("Accept", "application/json, text/javascript, */*; q=0.01")
	headers.Set("Content-Type", "application/x-www-form-urlencoded")
	if err := c.applyDeveloperPortalCSRF(headers, requireCSRF); err != nil {
		return nil, err
	}
	return c.doDeveloperPortalHTTPAndCapture(ctx, http.MethodPost, c.developerPortalOrigin()+developerPortalLegacyPath+path, values, headers)
}

func (c *Client) applyDeveloperPortalCSRF(headers http.Header, requireCSRF bool) error {
	csrf, csrfTS := c.developerCSRFTokens()
	if csrf != "" {
		headers.Set("csrf", csrf)
	}
	if csrfTS != "" {
		headers.Set("csrf_ts", csrfTS)
	}
	if requireCSRF && (csrf == "" || csrfTS == "") {
		return fmt.Errorf("missing Developer Portal CSRF headers; %s", developerPortalAuthHint)
	}
	return nil
}

func (c *Client) doDeveloperPortalHTTPAndCapture(ctx context.Context, method, requestURL string, body any, headers http.Header) ([]byte, error) {
	responseBody, response, err := c.doDeveloperPortalHTTP(ctx, method, requestURL, body, headers)
	if err != nil {
		return nil, err
	}
	c.captureDeveloperCSRFTokens(response.Header)
	if response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden {
		return nil, developerPortalSessionError(response.StatusCode)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, &APIError{
			Status:         response.StatusCode,
			AppleRequestID: extractAppleRequestID(response.Header),
			CorrelationKey: strings.TrimSpace(response.Header.Get("X-Apple-Jingle-Correlation-Key")),
			rawBody:        responseBody,
		}
	}
	return responseBody, nil
}

func (c *Client) doDeveloperPortalHTTP(ctx context.Context, method, requestURL string, body any, headers http.Header) ([]byte, *http.Response, error) {
	if c == nil || c.httpClient == nil {
		return nil, nil, fmt.Errorf("web client is not configured for Developer Portal")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	// Developer Portal proxies logical GETs as POSTs and says so with an
	// override header; honor that signal so read-only mode keeps reads working.
	if strings.EqualFold(headers.Get("X-HTTP-Method-Override"), http.MethodGet) {
		ctx = readonly.WithReadIntent(ctx)
	}
	if err := readonly.Check(ctx, method, readonly.Target(requestURL)); err != nil {
		return nil, nil, err
	}
	if err := c.waitForRateLimit(ctx); err != nil {
		return nil, nil, err
	}

	var requestBody io.Reader
	if body != nil {
		switch typed := body.(type) {
		case url.Values:
			requestBody = strings.NewReader(typed.Encode())
		default:
			encoded, err := json.Marshal(body)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to marshal Developer Portal request: %w", err)
			}
			requestBody = bytes.NewReader(encoded)
		}
	}
	request, err := http.NewRequestWithContext(ctx, method, requestURL, requestBody)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create Developer Portal request: %w", err)
	}
	request.Header = cloneHeaders(headers)
	setModifiedCookieHeader(c.httpClient, request)

	httpClient := *c.httpClient
	previousCheckRedirect := httpClient.CheckRedirect
	httpClient.CheckRedirect = func(redirect *http.Request, via []*http.Request) error {
		if !sameURLOrigin(request.URL, redirect.URL) {
			return fmt.Errorf("authentication redirected to %s instead of Developer Portal; %s", redirect.URL.Host, developerPortalAuthHint)
		}
		if previousCheckRedirect != nil {
			return previousCheckRedirect(redirect, via)
		}
		if len(via) >= 10 {
			return fmt.Errorf("stopped after 10 redirects")
		}
		return nil
	}
	response, err := httpClient.Do(request)
	if err != nil {
		logWebAuthHTTP("developer_portal_request", request, nil, nil, err)
		return nil, nil, fmt.Errorf("request to Developer Portal failed: %w", err)
	}
	defer func() { _ = response.Body.Close() }()
	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		logWebAuthHTTP("developer_portal_request", request, response, nil, err)
		return nil, response, fmt.Errorf("failed to read Developer Portal response: %w", err)
	}
	logWebAuthHTTP("developer_portal_request", request, response, responseBody, nil)
	return responseBody, response, nil
}

func sameURLOrigin(expected, actual *url.URL) bool {
	if expected == nil || actual == nil || expected.Scheme == "" || actual.Scheme == "" || expected.Hostname() == "" || actual.Hostname() == "" {
		return false
	}
	return strings.EqualFold(expected.Scheme, actual.Scheme) &&
		strings.EqualFold(expected.Hostname(), actual.Hostname()) &&
		effectiveURLPort(expected) == effectiveURLPort(actual)
}

func effectiveURLPort(value *url.URL) string {
	if port := value.Port(); port != "" {
		return port
	}
	switch strings.ToLower(value.Scheme) {
	case "http":
		return "80"
	case "https":
		return "443"
	default:
		return ""
	}
}

func (c *Client) captureDeveloperCSRFTokens(headers http.Header) {
	csrf := headerValueCaseInsensitive(headers, "csrf")
	csrfTS := headerValueCaseInsensitive(headers, "csrf_ts")
	if csrf == "" && csrfTS == "" {
		return
	}
	c.developerSessionMu.Lock()
	defer c.developerSessionMu.Unlock()
	if csrf != "" {
		c.developerCSRF = csrf
	}
	if csrfTS != "" {
		c.developerCSRFTS = csrfTS
	}
}

func (c *Client) clearDeveloperCSRFTokens() {
	c.developerSessionMu.Lock()
	defer c.developerSessionMu.Unlock()
	c.developerCSRF = ""
	c.developerCSRFTS = ""
}

func headerValueCaseInsensitive(headers http.Header, name string) string {
	for key, values := range headers {
		if !strings.EqualFold(key, name) || len(values) == 0 {
			continue
		}
		return strings.TrimSpace(values[0])
	}
	return ""
}

func (c *Client) developerCSRFTokens() (string, string) {
	c.developerSessionMu.Lock()
	defer c.developerSessionMu.Unlock()
	return c.developerCSRF, c.developerCSRFTS
}

func (c *Client) developerPortalTeamID() string {
	c.developerSessionMu.Lock()
	defer c.developerSessionMu.Unlock()
	return c.developerTeamID
}

func developerPortalSessionError(status int) error {
	return fmt.Errorf("web session is unauthorized or expired for Developer Portal (status %d); %s", status, developerPortalAuthHint)
}
