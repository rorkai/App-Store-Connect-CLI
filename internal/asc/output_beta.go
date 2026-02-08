package asc

import (
	"fmt"
	"strings"
)

// BetaTesterInvitationResult represents CLI output for invitations.
type BetaTesterInvitationResult struct {
	InvitationID string `json:"invitationId"`
	TesterID     string `json:"testerId,omitempty"`
	AppID        string `json:"appId,omitempty"`
	Email        string `json:"email,omitempty"`
}

// BetaTesterDeleteResult represents CLI output for deletions.
type BetaTesterDeleteResult struct {
	ID      string `json:"id"`
	Email   string `json:"email,omitempty"`
	Deleted bool   `json:"deleted"`
}

// BetaTesterGroupsUpdateResult represents CLI output for beta tester group updates.
type BetaTesterGroupsUpdateResult struct {
	TesterID string   `json:"testerId"`
	GroupIDs []string `json:"groupIds"`
	Action   string   `json:"action"`
}

// BetaTesterAppsUpdateResult represents CLI output for beta tester app updates.
type BetaTesterAppsUpdateResult struct {
	TesterID string   `json:"testerId"`
	AppIDs   []string `json:"appIds"`
	Action   string   `json:"action"`
}

// BetaTesterBuildsUpdateResult represents CLI output for beta tester build updates.
type BetaTesterBuildsUpdateResult struct {
	TesterID string   `json:"testerId"`
	BuildIDs []string `json:"buildIds"`
	Action   string   `json:"action"`
}

// AppBetaTestersUpdateResult represents CLI output for app beta tester updates.
type AppBetaTestersUpdateResult struct {
	AppID     string   `json:"appId"`
	TesterIDs []string `json:"testerIds"`
	Action    string   `json:"action"`
}

// BetaFeedbackSubmissionDeleteResult represents CLI output for beta feedback deletions.
type BetaFeedbackSubmissionDeleteResult struct {
	ID      string `json:"id"`
	Deleted bool   `json:"deleted"`
}

func formatBetaTesterName(attr BetaTesterAttributes) string {
	first := strings.TrimSpace(attr.FirstName)
	last := strings.TrimSpace(attr.LastName)
	switch {
	case first == "" && last == "":
		return ""
	case first == "":
		return last
	case last == "":
		return first
	default:
		return first + " " + last
	}
}

func betaGroupsRows(resp *BetaGroupsResponse) ([]string, [][]string) {
	headers := []string{"ID", "Name", "Internal", "Public Link Enabled", "Public Link"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{
			item.ID,
			compactWhitespace(item.Attributes.Name),
			fmt.Sprintf("%t", item.Attributes.IsInternalGroup),
			fmt.Sprintf("%t", item.Attributes.PublicLinkEnabled),
			item.Attributes.PublicLink,
		})
	}
	return headers, rows
}

func printBetaGroupsTable(resp *BetaGroupsResponse) error {
	h, r := betaGroupsRows(resp)
	RenderTable(h, r)
	return nil
}

func printBetaGroupsMarkdown(resp *BetaGroupsResponse) error {
	h, r := betaGroupsRows(resp)
	RenderMarkdown(h, r)
	return nil
}

func betaTestersRows(resp *BetaTestersResponse) ([]string, [][]string) {
	headers := []string{"ID", "Email", "Name", "State", "Invite"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{
			item.ID,
			item.Attributes.Email,
			compactWhitespace(formatBetaTesterName(item.Attributes)),
			string(item.Attributes.State),
			string(item.Attributes.InviteType),
		})
	}
	return headers, rows
}

func printBetaTestersTable(resp *BetaTestersResponse) error {
	h, r := betaTestersRows(resp)
	RenderTable(h, r)
	return nil
}

func printBetaTesterTable(resp *BetaTesterResponse) error {
	return printBetaTestersTable(&BetaTestersResponse{
		Data: []Resource[BetaTesterAttributes]{resp.Data},
	})
}

func printBetaTestersMarkdown(resp *BetaTestersResponse) error {
	h, r := betaTestersRows(resp)
	RenderMarkdown(h, r)
	return nil
}

func printBetaTesterMarkdown(resp *BetaTesterResponse) error {
	return printBetaTestersMarkdown(&BetaTestersResponse{
		Data: []Resource[BetaTesterAttributes]{resp.Data},
	})
}

func betaTesterDeleteResultRows(result *BetaTesterDeleteResult) ([]string, [][]string) {
	headers := []string{"ID", "Email", "Deleted"}
	rows := [][]string{{result.ID, result.Email, fmt.Sprintf("%t", result.Deleted)}}
	return headers, rows
}

func printBetaTesterDeleteResultTable(result *BetaTesterDeleteResult) error {
	h, r := betaTesterDeleteResultRows(result)
	RenderTable(h, r)
	return nil
}

func printBetaTesterDeleteResultMarkdown(result *BetaTesterDeleteResult) error {
	h, r := betaTesterDeleteResultRows(result)
	RenderMarkdown(h, r)
	return nil
}

func betaTesterGroupsUpdateResultRows(result *BetaTesterGroupsUpdateResult) ([]string, [][]string) {
	headers := []string{"Tester ID", "Group IDs", "Action"}
	rows := [][]string{{result.TesterID, strings.Join(result.GroupIDs, ","), result.Action}}
	return headers, rows
}

func printBetaTesterGroupsUpdateResultTable(result *BetaTesterGroupsUpdateResult) error {
	h, r := betaTesterGroupsUpdateResultRows(result)
	RenderTable(h, r)
	return nil
}

func printBetaTesterGroupsUpdateResultMarkdown(result *BetaTesterGroupsUpdateResult) error {
	h, r := betaTesterGroupsUpdateResultRows(result)
	RenderMarkdown(h, r)
	return nil
}

func betaTesterAppsUpdateResultRows(result *BetaTesterAppsUpdateResult) ([]string, [][]string) {
	headers := []string{"Tester ID", "App IDs", "Action"}
	rows := [][]string{{result.TesterID, strings.Join(result.AppIDs, ","), result.Action}}
	return headers, rows
}

func printBetaTesterAppsUpdateResultTable(result *BetaTesterAppsUpdateResult) error {
	h, r := betaTesterAppsUpdateResultRows(result)
	RenderTable(h, r)
	return nil
}

func printBetaTesterAppsUpdateResultMarkdown(result *BetaTesterAppsUpdateResult) error {
	h, r := betaTesterAppsUpdateResultRows(result)
	RenderMarkdown(h, r)
	return nil
}

func betaTesterBuildsUpdateResultRows(result *BetaTesterBuildsUpdateResult) ([]string, [][]string) {
	headers := []string{"Tester ID", "Build IDs", "Action"}
	rows := [][]string{{result.TesterID, strings.Join(result.BuildIDs, ","), result.Action}}
	return headers, rows
}

func printBetaTesterBuildsUpdateResultTable(result *BetaTesterBuildsUpdateResult) error {
	h, r := betaTesterBuildsUpdateResultRows(result)
	RenderTable(h, r)
	return nil
}

func printBetaTesterBuildsUpdateResultMarkdown(result *BetaTesterBuildsUpdateResult) error {
	h, r := betaTesterBuildsUpdateResultRows(result)
	RenderMarkdown(h, r)
	return nil
}

func appBetaTestersUpdateResultRows(result *AppBetaTestersUpdateResult) ([]string, [][]string) {
	headers := []string{"App ID", "Tester IDs", "Action"}
	rows := [][]string{{result.AppID, strings.Join(result.TesterIDs, ","), result.Action}}
	return headers, rows
}

func printAppBetaTestersUpdateResultTable(result *AppBetaTestersUpdateResult) error {
	h, r := appBetaTestersUpdateResultRows(result)
	RenderTable(h, r)
	return nil
}

func printAppBetaTestersUpdateResultMarkdown(result *AppBetaTestersUpdateResult) error {
	h, r := appBetaTestersUpdateResultRows(result)
	RenderMarkdown(h, r)
	return nil
}

func betaFeedbackSubmissionDeleteResultRows(result *BetaFeedbackSubmissionDeleteResult) ([]string, [][]string) {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	return headers, rows
}

func printBetaFeedbackSubmissionDeleteResultTable(result *BetaFeedbackSubmissionDeleteResult) error {
	h, r := betaFeedbackSubmissionDeleteResultRows(result)
	RenderTable(h, r)
	return nil
}

func printBetaFeedbackSubmissionDeleteResultMarkdown(result *BetaFeedbackSubmissionDeleteResult) error {
	h, r := betaFeedbackSubmissionDeleteResultRows(result)
	RenderMarkdown(h, r)
	return nil
}

func betaTesterInvitationResultRows(result *BetaTesterInvitationResult) ([]string, [][]string) {
	headers := []string{"Invitation ID", "Tester ID", "App ID", "Email"}
	rows := [][]string{{result.InvitationID, result.TesterID, result.AppID, result.Email}}
	return headers, rows
}

func printBetaTesterInvitationResultTable(result *BetaTesterInvitationResult) error {
	h, r := betaTesterInvitationResultRows(result)
	RenderTable(h, r)
	return nil
}

func printBetaTesterInvitationResultMarkdown(result *BetaTesterInvitationResult) error {
	h, r := betaTesterInvitationResultRows(result)
	RenderMarkdown(h, r)
	return nil
}
