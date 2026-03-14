package web

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"reflect"
	"strconv"
	"strings"
)

// NewCIClient creates a CI API client reusing an authenticated web session.
// The CI API lives at /ci/api and uses the same session cookies as IRIS.
func NewCIClient(session *AuthSession) *Client {
	return &Client{
		httpClient:         session.Client,
		baseURL:            appStoreBaseURL + "/ci/api",
		minRequestInterval: resolveWebMinRequestInterval(),
	}
}

// NOTE: The CI API (/ci/api) uses snake_case JSON keys and query parameters,
// unlike the IRIS API (/iris/v1) which uses camelCase. Confirmed via browser
// network inspection of the ASC web UI.

// CIUsageSummary is the response from the usage summary endpoint.
type CIUsageSummary struct {
	Plan  CIUsagePlan       `json:"plan"`
	Links map[string]string `json:"links,omitempty"`
}

// CIUsagePlan describes the Xcode Cloud plan quota.
type CIUsagePlan struct {
	Name          string `json:"name"`
	ResetDate     string `json:"reset_date"`
	ResetDateTime string `json:"reset_date_time"`
	Available     int    `json:"available"`
	Used          int    `json:"used"`
	Total         int    `json:"total"`
}

// CIUsageMonths is the response from the monthly usage endpoint.
type CIUsageMonths struct {
	Usage        []CIMonthUsage   `json:"usage"`
	ProductUsage []CIProductUsage `json:"product_usage"`
	Info         CIUsageInfo      `json:"info"`
}

// CIMonthUsage describes usage for a single month.
type CIMonthUsage struct {
	Month          int `json:"month"`
	Year           int `json:"year"`
	Duration       int `json:"duration"`
	NumberOfBuilds int `json:"number_of_builds,omitempty"`
}

// CIProductUsage describes per-product monthly usage.
type CIProductUsage struct {
	ProductID              string         `json:"product_id"`
	ProductName            string         `json:"product_name,omitempty"`
	BundleID               string         `json:"bundle_id,omitempty"`
	Usage                  []CIMonthUsage `json:"usage,omitempty"`
	UsageInMinutes         int            `json:"usage_in_minutes,omitempty"`
	UsageInSeconds         int            `json:"usage_in_seconds,omitempty"`
	NumberOfBuilds         int            `json:"number_of_builds,omitempty"`
	PreviousUsageInMinutes int            `json:"previous_usage_in_minutes,omitempty"`
	PreviousNumberOfBuilds int            `json:"previous_number_of_builds,omitempty"`
}

// CIUsageInfo holds metadata about the usage response.
type CIUsageInfo struct {
	StartMonth         int                `json:"start_month,omitempty"`
	StartYear          int                `json:"start_year,omitempty"`
	EndMonth           int                `json:"end_month,omitempty"`
	EndYear            int                `json:"end_year,omitempty"`
	CanViewAllProducts bool               `json:"can_view_all_products,omitempty"`
	Current            CIUsageInfoCurrent `json:"current,omitempty"`
	Previous           CIUsageInfoCurrent `json:"previous,omitempty"`
	Links              map[string]string  `json:"links,omitempty"`
}

// CIUsageInfoCurrent summarizes usage in the current/previous period.
type CIUsageInfoCurrent struct {
	Builds        int `json:"builds"`
	Used          int `json:"used"`
	Average30Days int `json:"average_30_days"`
}

// CIUsageDays is the response from the daily usage endpoint.
type CIUsageDays struct {
	Usage         []CIDayUsage      `json:"usage"`
	ProductUsage  []CIProductUsage  `json:"product_usage,omitempty"`
	WorkflowUsage []CIWorkflowUsage `json:"workflow_usage"`
	Info          CIUsageInfo       `json:"info"`
}

// CIDayUsage describes usage for a single day.
type CIDayUsage struct {
	Date           string `json:"date"`
	Duration       int    `json:"duration"`
	NumberOfBuilds int    `json:"number_of_builds,omitempty"`
}

// CIWorkflowUsage describes per-workflow daily usage.
type CIWorkflowUsage struct {
	WorkflowID             string       `json:"workflow_id"`
	WorkflowName           string       `json:"workflow_name,omitempty"`
	Usage                  []CIDayUsage `json:"usage,omitempty"`
	UsageInMinutes         int          `json:"usage_in_minutes,omitempty"`
	NumberOfBuilds         int          `json:"number_of_builds,omitempty"`
	PreviousUsageInMinutes int          `json:"previous_usage_in_minutes,omitempty"`
	PreviousNumberOfBuilds int          `json:"previous_number_of_builds,omitempty"`
}

// CIProduct describes a Xcode Cloud product.
type CIProduct struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	BundleID string `json:"bundle_id"`
	Type     string `json:"type"`
	IconURL  string `json:"icon_url,omitempty"`
}

// CIProductListResponse is the response from the products endpoint.
type CIProductListResponse struct {
	Items []CIProduct `json:"items"`
}

// CIWorkflow describes a Xcode Cloud workflow.
type CIWorkflow struct {
	ID      string            `json:"id"`
	Content CIWorkflowContent `json:"content"`
}

// CIWorkflowContent holds the workflow's configuration including its name.
type CIWorkflowContent struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// CIWorkflowListResponse is the response from the workflows endpoint.
type CIWorkflowListResponse struct {
	Items []CIWorkflow `json:"items"`
}

func (m *CIMonthUsage) UnmarshalJSON(data []byte) error {
	type alias struct {
		Month          int  `json:"month"`
		Year           int  `json:"year"`
		Duration       *int `json:"duration"`
		Minutes        *int `json:"minutes"`
		NumberOfBuilds int  `json:"number_of_builds"`
	}
	var value alias
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	m.Month = value.Month
	m.Year = value.Year
	m.NumberOfBuilds = value.NumberOfBuilds
	switch {
	case value.Duration != nil:
		m.Duration = *value.Duration
	case value.Minutes != nil:
		m.Duration = *value.Minutes
	default:
		m.Duration = 0
	}
	return nil
}

func (d *CIDayUsage) UnmarshalJSON(data []byte) error {
	type alias struct {
		Date           string `json:"date"`
		Duration       *int   `json:"duration"`
		Minutes        *int   `json:"minutes"`
		NumberOfBuilds int    `json:"number_of_builds"`
	}
	var value alias
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	d.Date = value.Date
	d.NumberOfBuilds = value.NumberOfBuilds
	switch {
	case value.Duration != nil:
		d.Duration = *value.Duration
	case value.Minutes != nil:
		d.Duration = *value.Minutes
	default:
		d.Duration = 0
	}
	return nil
}

// GetCIUsageSummary retrieves the Xcode Cloud plan usage summary.
func (c *Client) GetCIUsageSummary(ctx context.Context, teamID string) (*CIUsageSummary, error) {
	teamID = strings.TrimSpace(teamID)
	if teamID == "" {
		return nil, fmt.Errorf("team id is required")
	}
	path := "/teams/" + url.PathEscape(teamID) + "/usage/summary"
	body, err := c.doRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}
	var result CIUsageSummary
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to decode ci usage summary: %w", err)
	}
	return &result, nil
}

// GetCIUsageMonths retrieves monthly Xcode Cloud usage for a date range.
func (c *Client) GetCIUsageMonths(ctx context.Context, teamID string, startMonth, startYear, endMonth, endYear int) (*CIUsageMonths, error) {
	teamID = strings.TrimSpace(teamID)
	if teamID == "" {
		return nil, fmt.Errorf("team id is required")
	}
	query := url.Values{}
	query.Set("start_month", strconv.Itoa(startMonth))
	query.Set("start_year", strconv.Itoa(startYear))
	query.Set("end_month", strconv.Itoa(endMonth))
	query.Set("end_year", strconv.Itoa(endYear))
	path := queryPath("/teams/"+url.PathEscape(teamID)+"/usage/months", query)
	body, err := c.doRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}
	var result CIUsageMonths
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to decode ci usage months: %w", err)
	}
	return &result, nil
}

// GetCIUsageDays retrieves daily Xcode Cloud usage for a product in a date range.
func (c *Client) GetCIUsageDays(ctx context.Context, teamID, productID, start, end string) (*CIUsageDays, error) {
	teamID = strings.TrimSpace(teamID)
	if teamID == "" {
		return nil, fmt.Errorf("team id is required")
	}
	productID = strings.TrimSpace(productID)
	if productID == "" {
		return nil, fmt.Errorf("product id is required")
	}
	start = strings.TrimSpace(start)
	if start == "" {
		return nil, fmt.Errorf("start date is required")
	}
	end = strings.TrimSpace(end)
	if end == "" {
		return nil, fmt.Errorf("end date is required")
	}
	query := url.Values{}
	query.Set("start", start)
	query.Set("end", end)
	path := queryPath("/teams/"+url.PathEscape(teamID)+"/products/"+url.PathEscape(productID)+"/usage/days", query)
	body, err := c.doRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}
	var result CIUsageDays
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to decode ci usage days: %w", err)
	}
	return &result, nil
}

// GetCIUsageDaysOverall retrieves daily Xcode Cloud usage overview for a team.
func (c *Client) GetCIUsageDaysOverall(ctx context.Context, teamID, start, end string) (*CIUsageDays, error) {
	teamID = strings.TrimSpace(teamID)
	if teamID == "" {
		return nil, fmt.Errorf("team id is required")
	}
	start = strings.TrimSpace(start)
	if start == "" {
		return nil, fmt.Errorf("start date is required")
	}
	end = strings.TrimSpace(end)
	if end == "" {
		return nil, fmt.Errorf("end date is required")
	}
	query := url.Values{}
	query.Set("start", start)
	query.Set("end", end)
	path := queryPath("/teams/"+url.PathEscape(teamID)+"/usage/days", query)
	body, err := c.doRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}
	var result CIUsageDays
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to decode ci usage days overview: %w", err)
	}
	return &result, nil
}

// ListCIProducts lists Xcode Cloud products for a team.
// The CI API does not expose pagination for this endpoint; limit=100 covers
// the vast majority of teams.
func (c *Client) ListCIProducts(ctx context.Context, teamID string) (*CIProductListResponse, error) {
	teamID = strings.TrimSpace(teamID)
	if teamID == "" {
		return nil, fmt.Errorf("team id is required")
	}
	query := url.Values{}
	query.Set("limit", "100")
	path := queryPath("/teams/"+url.PathEscape(teamID)+"/products-v4", query)
	body, err := c.doRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}
	var result CIProductListResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to decode ci products: %w", err)
	}
	return &result, nil
}

// CIEnvironmentVariable represents a workflow environment variable.
type CIEnvironmentVariable struct {
	ID    string                     `json:"id"`
	Name  string                     `json:"name"`
	Value CIEnvironmentVariableValue `json:"value"`
}

// CIEnvironmentVariableValue holds exactly one of plaintext, ciphertext, or redacted.
type CIEnvironmentVariableValue struct {
	Plaintext     *string `json:"plaintext,omitempty"`
	Ciphertext    *string `json:"ciphertext,omitempty"`
	RedactedValue *string `json:"redacted_value,omitempty"`
}

// CIWorkflowFull is the full workflow body for GET/PUT round-trips.
// Uses json.RawMessage for Content to preserve unknown fields.
type CIWorkflowFull struct {
	ID      string          `json:"id"`
	Content json.RawMessage `json:"content"`
}

// CIWorkflowConfig captures workflow fields surfaced by the web UI.
// Nested and evolving structures are kept as raw JSON for forward compatibility.
type CIWorkflowConfig struct {
	Name                        string          `json:"name"`
	Description                 string          `json:"description,omitempty"`
	Disabled                    bool            `json:"disabled"`
	Locked                      bool            `json:"locked"`
	XcodeVersion                json.RawMessage `json:"xcode_version,omitempty"`
	MacOSVersion                json.RawMessage `json:"macos_version,omitempty"`
	StartConditions             json.RawMessage `json:"start_conditions,omitempty"`
	Actions                     json.RawMessage `json:"actions,omitempty"`
	PostActions                 json.RawMessage `json:"post_actions,omitempty"`
	Clean                       json.RawMessage `json:"clean,omitempty"`
	ContainerFilePath           string          `json:"container_file_path,omitempty"`
	Repo                        json.RawMessage `json:"repo,omitempty"`
	ProductEnvironmentVariables []string        `json:"product_environment_variables,omitempty"`
}

// CIEncryptionKeyResponse is the response from /auth/keys/client-encryption.
type CIEncryptionKeyResponse struct {
	Key string `json:"key"`
}

// ListCIWorkflows lists Xcode Cloud workflows for a product.
func (c *Client) ListCIWorkflows(ctx context.Context, teamID, productID string) (*CIWorkflowListResponse, error) {
	teamID = strings.TrimSpace(teamID)
	if teamID == "" {
		return nil, fmt.Errorf("team id is required")
	}
	productID = strings.TrimSpace(productID)
	if productID == "" {
		return nil, fmt.Errorf("product id is required")
	}
	query := url.Values{}
	query.Set("limit", "100")
	query.Set("include_deleted", "false")
	path := queryPath("/teams/"+url.PathEscape(teamID)+"/products/"+url.PathEscape(productID)+"/workflows-v15", query)
	body, err := c.doRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}
	var result CIWorkflowListResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to decode ci workflows: %w", err)
	}
	return &result, nil
}

// GetCIWorkflow gets a single workflow (full body including env vars).
// GET /teams/{teamID}/products/{productID}/workflows-v15/{workflowID}
func (c *Client) GetCIWorkflow(ctx context.Context, teamID, productID, workflowID string) (*CIWorkflowFull, error) {
	teamID = strings.TrimSpace(teamID)
	if teamID == "" {
		return nil, fmt.Errorf("team id is required")
	}
	productID = strings.TrimSpace(productID)
	if productID == "" {
		return nil, fmt.Errorf("product id is required")
	}
	workflowID = strings.TrimSpace(workflowID)
	if workflowID == "" {
		return nil, fmt.Errorf("workflow id is required")
	}
	path := "/teams/" + url.PathEscape(teamID) + "/products/" + url.PathEscape(productID) + "/workflows-v15/" + url.PathEscape(workflowID)
	body, err := c.doRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}
	var result CIWorkflowFull
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to decode ci workflow: %w", err)
	}
	return &result, nil
}

// UpdateCIWorkflow updates a workflow (PUT full body).
// PUT /teams/{teamID}/products/{productID}/workflows-v15/{workflowID}
func (c *Client) UpdateCIWorkflow(ctx context.Context, teamID, productID, workflowID string, content json.RawMessage) error {
	teamID = strings.TrimSpace(teamID)
	if teamID == "" {
		return fmt.Errorf("team id is required")
	}
	productID = strings.TrimSpace(productID)
	if productID == "" {
		return fmt.Errorf("product id is required")
	}
	workflowID = strings.TrimSpace(workflowID)
	if workflowID == "" {
		return fmt.Errorf("workflow id is required")
	}
	path := "/teams/" + url.PathEscape(teamID) + "/products/" + url.PathEscape(productID) + "/workflows-v15/" + url.PathEscape(workflowID)
	_, err := c.doRequest(ctx, "PUT", path, content)
	return err
}

// GetCIConfigurationOptions retrieves team-wide workflow editor options.
// GET /teams/{teamID}/configuration-options-v10
func (c *Client) GetCIConfigurationOptions(ctx context.Context, teamID string) (json.RawMessage, error) {
	teamID = strings.TrimSpace(teamID)
	if teamID == "" {
		return nil, fmt.Errorf("team id is required")
	}
	path := "/teams/" + url.PathEscape(teamID) + "/configuration-options-v10"
	body, err := c.doRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}
	return compactRawJSON(body, "ci configuration options")
}

// GetCIBuildVersions retrieves build-version configuration options.
// GET /teams/{teamID}/configuration-options/build-versions
func (c *Client) GetCIBuildVersions(ctx context.Context, teamID string) (json.RawMessage, error) {
	teamID = strings.TrimSpace(teamID)
	if teamID == "" {
		return nil, fmt.Errorf("team id is required")
	}
	path := "/teams/" + url.PathEscape(teamID) + "/configuration-options/build-versions"
	body, err := c.doRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}
	return compactRawJSON(body, "ci build versions")
}

// GetCIProductConfigurationOptions retrieves product-scoped workflow editor options.
// GET /teams/{teamID}/products/{productID}/product-configuration-options-v4
func (c *Client) GetCIProductConfigurationOptions(ctx context.Context, teamID, productID string) (json.RawMessage, error) {
	teamID = strings.TrimSpace(teamID)
	if teamID == "" {
		return nil, fmt.Errorf("team id is required")
	}
	productID = strings.TrimSpace(productID)
	if productID == "" {
		return nil, fmt.Errorf("product id is required")
	}
	path := "/teams/" + url.PathEscape(teamID) + "/products/" + url.PathEscape(productID) + "/product-configuration-options-v4"
	body, err := c.doRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}
	return compactRawJSON(body, "ci product configuration options")
}

// GetCISchemes retrieves available schemes for a product.
// GET /teams/{teamID}/products/{productID}/schemes
func (c *Client) GetCISchemes(
	ctx context.Context,
	teamID, productID, containerFilePath string,
	limit int,
	continuationOffset string,
) (json.RawMessage, error) {
	teamID = strings.TrimSpace(teamID)
	if teamID == "" {
		return nil, fmt.Errorf("team id is required")
	}
	productID = strings.TrimSpace(productID)
	if productID == "" {
		return nil, fmt.Errorf("product id is required")
	}
	if limit < 0 {
		return nil, fmt.Errorf("limit must be zero or greater")
	}
	query := url.Values{}
	if trimmed := strings.TrimSpace(containerFilePath); trimmed != "" {
		query.Set("container_file_path", trimmed)
	}
	if limit > 0 {
		query.Set("limit", strconv.Itoa(limit))
	}
	if trimmed := strings.TrimSpace(continuationOffset); trimmed != "" {
		query.Set("continuation_offset", trimmed)
	}
	path := queryPath("/teams/"+url.PathEscape(teamID)+"/products/"+url.PathEscape(productID)+"/schemes", query)
	body, err := c.doRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}
	return compactRawJSON(body, "ci schemes")
}

// GetCITestDestinations retrieves workflow test destination options for an Xcode version.
// GET /teams/{teamID}/test-destinations-v3
func (c *Client) GetCITestDestinations(ctx context.Context, teamID, xcodeVersion string) (json.RawMessage, error) {
	teamID = strings.TrimSpace(teamID)
	if teamID == "" {
		return nil, fmt.Errorf("team id is required")
	}
	xcodeVersion = strings.TrimSpace(xcodeVersion)
	if xcodeVersion == "" {
		return nil, fmt.Errorf("xcode version is required")
	}
	query := url.Values{}
	query.Set("xcode_version", xcodeVersion)
	path := queryPath("/teams/"+url.PathEscape(teamID)+"/test-destinations-v3", query)
	body, err := c.doRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}
	return compactRawJSON(body, "ci test destinations")
}

// GetCISlackProvider retrieves the team's Slack integration state for workflow notifications.
// GET /teams/{teamID}/integrations/slack
func (c *Client) GetCISlackProvider(ctx context.Context, teamID string) (json.RawMessage, error) {
	teamID = strings.TrimSpace(teamID)
	if teamID == "" {
		return nil, fmt.Errorf("team id is required")
	}
	path := "/teams/" + url.PathEscape(teamID) + "/integrations/slack"
	body, err := c.doRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}
	return compactRawJSON(body, "ci slack provider")
}

// GetCISlackChannels retrieves the team's Slack channel options for workflow notifications.
// GET /teams/{teamID}/integrations/slack/channels
func (c *Client) GetCISlackChannels(ctx context.Context, teamID string) (json.RawMessage, error) {
	teamID = strings.TrimSpace(teamID)
	if teamID == "" {
		return nil, fmt.Errorf("team id is required")
	}
	path := "/teams/" + url.PathEscape(teamID) + "/integrations/slack/channels"
	body, err := c.doRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}
	return compactRawJSON(body, "ci slack channels")
}

// GetCIEncryptionKey fetches the P-256 public key for secret encryption.
// GET /auth/keys/client-encryption (relative to /ci/api base URL)
func (c *Client) GetCIEncryptionKey(ctx context.Context) (*CIEncryptionKeyResponse, error) {
	body, err := c.doRequest(ctx, "GET", "/auth/keys/client-encryption", nil)
	if err != nil {
		return nil, err
	}
	var result CIEncryptionKeyResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to decode ci encryption key: %w", err)
	}
	return &result, nil
}

// CIProductEnvironmentVariable represents a shared (product-level) environment variable.
type CIProductEnvironmentVariable struct {
	ID                       string                     `json:"id"`
	Name                     string                     `json:"name"`
	Value                    CIEnvironmentVariableValue `json:"value"`
	IsLocked                 bool                       `json:"is_locked"`
	RelatedWorkflowSummaries []CIRelatedWorkflowSummary `json:"related_workflow_summaries,omitempty"`
}

// CIRelatedWorkflowSummary describes a workflow linked to a shared env var.
type CIRelatedWorkflowSummary struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Disabled       bool   `json:"disabled"`
	Locked         bool   `json:"locked"`
	LastModifiedBy string `json:"last_modified_by,omitempty"`
	LastModifiedAt string `json:"last_modified_at,omitempty"`
}

// CIProductEnvVarRequest is the PUT body for creating/updating a shared env var.
type CIProductEnvVarRequest struct {
	Name        string                     `json:"name"`
	Value       CIEnvironmentVariableValue `json:"value"`
	IsLocked    bool                       `json:"is_locked"`
	WorkflowIDs []string                   `json:"workflow_ids"`
}

// ListCIProductEnvVars lists shared (product-level) environment variables.
// GET /teams/{teamID}/products/{productID}/product-environment-variables
func (c *Client) ListCIProductEnvVars(ctx context.Context, teamID, productID string) ([]CIProductEnvironmentVariable, error) {
	teamID = strings.TrimSpace(teamID)
	if teamID == "" {
		return nil, fmt.Errorf("team id is required")
	}
	productID = strings.TrimSpace(productID)
	if productID == "" {
		return nil, fmt.Errorf("product id is required")
	}
	path := "/teams/" + url.PathEscape(teamID) + "/products/" + url.PathEscape(productID) + "/product-environment-variables"
	body, err := c.doRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}
	var result []CIProductEnvironmentVariable
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to decode product environment variables: %w", err)
	}
	return result, nil
}

// SetCIProductEnvVar creates or updates a shared (product-level) environment variable.
// PUT /teams/{teamID}/products/{productID}/product-environment-variables/{varID}
func (c *Client) SetCIProductEnvVar(ctx context.Context, teamID, productID, varID string, req CIProductEnvVarRequest) (*CIProductEnvironmentVariable, error) {
	teamID = strings.TrimSpace(teamID)
	if teamID == "" {
		return nil, fmt.Errorf("team id is required")
	}
	productID = strings.TrimSpace(productID)
	if productID == "" {
		return nil, fmt.Errorf("product id is required")
	}
	varID = strings.TrimSpace(varID)
	if varID == "" {
		return nil, fmt.Errorf("variable id is required")
	}
	path := "/teams/" + url.PathEscape(teamID) + "/products/" + url.PathEscape(productID) + "/product-environment-variables/" + url.PathEscape(varID)
	body, err := c.doRequest(ctx, "PUT", path, req)
	if err != nil {
		return nil, err
	}
	var result CIProductEnvironmentVariable
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to decode product environment variable response: %w", err)
	}
	return &result, nil
}

// DeleteCIProductEnvVar deletes a shared (product-level) environment variable.
// DELETE /teams/{teamID}/products/{productID}/product-environment-variables/{varID}
func (c *Client) DeleteCIProductEnvVar(ctx context.Context, teamID, productID, varID string) error {
	teamID = strings.TrimSpace(teamID)
	if teamID == "" {
		return fmt.Errorf("team id is required")
	}
	productID = strings.TrimSpace(productID)
	if productID == "" {
		return fmt.Errorf("product id is required")
	}
	varID = strings.TrimSpace(varID)
	if varID == "" {
		return fmt.Errorf("variable id is required")
	}
	path := "/teams/" + url.PathEscape(teamID) + "/products/" + url.PathEscape(productID) + "/product-environment-variables/" + url.PathEscape(varID)
	_, err := c.doRequest(ctx, "DELETE", path, nil)
	return err
}

// ExtractEnvVars extracts environment_variables from raw workflow content.
func ExtractEnvVars(content json.RawMessage) ([]CIEnvironmentVariable, error) {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(content, &m); err != nil {
		return nil, fmt.Errorf("failed to decode workflow content: %w", err)
	}
	raw, ok := m["environment_variables"]
	if !ok {
		return nil, nil
	}
	var vars []CIEnvironmentVariable
	if err := json.Unmarshal(raw, &vars); err != nil {
		return nil, fmt.Errorf("failed to decode environment_variables: %w", err)
	}
	return vars, nil
}

// SetEnvVars sets environment_variables in raw workflow content, preserving other fields.
func SetEnvVars(content json.RawMessage, vars []CIEnvironmentVariable) (json.RawMessage, error) {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(content, &m); err != nil {
		return nil, fmt.Errorf("failed to decode workflow content: %w", err)
	}
	if m == nil {
		return nil, fmt.Errorf("failed to decode workflow content: expected JSON object")
	}
	varsJSON, err := json.Marshal(vars)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal environment_variables: %w", err)
	}
	m["environment_variables"] = varsJSON
	// Use compact JSON to match API expectations.
	result, err := json.Marshal(m)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal workflow content: %w", err)
	}
	// Compact to remove any extra whitespace.
	var buf bytes.Buffer
	if err := json.Compact(&buf, result); err != nil {
		return nil, fmt.Errorf("failed to compact workflow content: %w", err)
	}
	return buf.Bytes(), nil
}

// ExtractWorkflowConfig extracts known workflow configuration fields from raw workflow content.
func ExtractWorkflowConfig(content json.RawMessage) (*CIWorkflowConfig, error) {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(content, &m); err != nil {
		return nil, fmt.Errorf("failed to decode workflow content: %w", err)
	}
	cfg := &CIWorkflowConfig{
		Name:              decodeJSONString(m["name"]),
		Description:       decodeJSONString(m["description"]),
		Disabled:          decodeJSONBool(m["disabled"]),
		Locked:            decodeJSONBool(m["locked"]),
		XcodeVersion:      m["xcode_version"],
		MacOSVersion:      m["macos_version"],
		StartConditions:   m["start_conditions"],
		Actions:           m["actions"],
		PostActions:       m["post_actions"],
		Clean:             m["clean"],
		ContainerFilePath: decodeJSONString(m["container_file_path"]),
		Repo:              m["repo"],
	}

	if len(m["product_environment_variables"]) > 0 {
		var refs []string
		if err := json.Unmarshal(m["product_environment_variables"], &refs); err == nil {
			cfg.ProductEnvironmentVariables = refs
		}
	}

	return cfg, nil
}

// SetWorkflowDisabled sets the disabled field on raw workflow content while preserving all other fields.
func SetWorkflowDisabled(content json.RawMessage, disabled bool) (json.RawMessage, error) {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(content, &m); err != nil {
		return nil, fmt.Errorf("failed to decode workflow content: %w", err)
	}
	if m == nil {
		return nil, fmt.Errorf("failed to decode workflow content: expected JSON object")
	}

	disabledJSON, err := json.Marshal(disabled)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal disabled: %w", err)
	}
	m["disabled"] = disabledJSON

	result, err := json.Marshal(m)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal workflow content: %w", err)
	}

	var buf bytes.Buffer
	if err := json.Compact(&buf, result); err != nil {
		return nil, fmt.Errorf("failed to compact workflow content: %w", err)
	}

	return buf.Bytes(), nil
}

// ApplyJSONMergePatch applies an RFC 7396-style merge patch to workflow content.
// Both the existing content and the patch must be JSON objects.
func ApplyJSONMergePatch(content json.RawMessage, patch json.RawMessage) (json.RawMessage, bool, error) {
	var contentValue any
	if err := json.Unmarshal(content, &contentValue); err != nil {
		return nil, false, fmt.Errorf("failed to decode workflow content: %w", err)
	}
	if _, ok := contentValue.(map[string]any); !ok {
		return nil, false, fmt.Errorf("failed to decode workflow content: expected JSON object")
	}

	var patchValue any
	if err := json.Unmarshal(patch, &patchValue); err != nil {
		return nil, false, fmt.Errorf("failed to decode workflow patch: %w", err)
	}
	if _, ok := patchValue.(map[string]any); !ok {
		return nil, false, fmt.Errorf("failed to decode workflow patch: expected JSON object")
	}

	mergedValue := applyJSONMergePatchValue(contentValue, patchValue)
	result, err := json.Marshal(mergedValue)
	if err != nil {
		return nil, false, fmt.Errorf("failed to marshal merged workflow content: %w", err)
	}

	var buf bytes.Buffer
	if err := json.Compact(&buf, result); err != nil {
		return nil, false, fmt.Errorf("failed to compact merged workflow content: %w", err)
	}

	changed := !reflect.DeepEqual(contentValue, mergedValue)
	return buf.Bytes(), changed, nil
}

func applyJSONMergePatchValue(target, patch any) any {
	patchMap, ok := patch.(map[string]any)
	if !ok {
		return patch
	}

	targetMap, ok := target.(map[string]any)
	if !ok {
		targetMap = map[string]any{}
	}

	merged := make(map[string]any, len(targetMap))
	for key, value := range targetMap {
		merged[key] = value
	}

	for key, value := range patchMap {
		if value == nil {
			delete(merged, key)
			continue
		}
		merged[key] = applyJSONMergePatchValue(merged[key], value)
	}

	return merged
}

func compactRawJSON(body []byte, label string) (json.RawMessage, error) {
	var buf bytes.Buffer
	if err := json.Compact(&buf, body); err != nil {
		return nil, fmt.Errorf("failed to decode %s: %w", label, err)
	}
	return buf.Bytes(), nil
}

func decodeJSONString(raw json.RawMessage) string {
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return ""
	}
	return strings.TrimSpace(value)
}

func decodeJSONBool(raw json.RawMessage) bool {
	var value bool
	if err := json.Unmarshal(raw, &value); err != nil {
		return false
	}
	return value
}
