package asc

import (
	"fmt"
	"os"
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

func printBetaGroupsTable(resp *BetaGroupsResponse) error {
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
	RenderTable(headers, rows)
	return nil
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

func printBetaTestersTable(resp *BetaTestersResponse) error {
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
	RenderTable(headers, rows)
	return nil
}

func printBetaTesterTable(resp *BetaTesterResponse) error {
	return printBetaTestersTable(&BetaTestersResponse{
		Data: []Resource[BetaTesterAttributes]{resp.Data},
	})
}

func printBetaGroupsMarkdown(resp *BetaGroupsResponse) error {
	fmt.Fprintln(os.Stdout, "| ID | Name | Internal | Public Link Enabled | Public Link |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- | --- | --- |")
	for _, item := range resp.Data {
		fmt.Fprintf(os.Stdout, "| %s | %s | %t | %t | %s |\n",
			escapeMarkdown(item.ID),
			escapeMarkdown(item.Attributes.Name),
			item.Attributes.IsInternalGroup,
			item.Attributes.PublicLinkEnabled,
			escapeMarkdown(item.Attributes.PublicLink),
		)
	}
	return nil
}

func printBetaTestersMarkdown(resp *BetaTestersResponse) error {
	fmt.Fprintln(os.Stdout, "| ID | Email | Name | State | Invite |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- | --- | --- |")
	for _, item := range resp.Data {
		fmt.Fprintf(os.Stdout, "| %s | %s | %s | %s | %s |\n",
			escapeMarkdown(item.ID),
			escapeMarkdown(item.Attributes.Email),
			escapeMarkdown(formatBetaTesterName(item.Attributes)),
			escapeMarkdown(string(item.Attributes.State)),
			escapeMarkdown(string(item.Attributes.InviteType)),
		)
	}
	return nil
}

func printBetaTesterMarkdown(resp *BetaTesterResponse) error {
	return printBetaTestersMarkdown(&BetaTestersResponse{
		Data: []Resource[BetaTesterAttributes]{resp.Data},
	})
}

func printBetaTesterDeleteResultTable(result *BetaTesterDeleteResult) error {
	headers := []string{"ID", "Email", "Deleted"}
	rows := [][]string{{result.ID, result.Email, fmt.Sprintf("%t", result.Deleted)}}
	RenderTable(headers, rows)
	return nil
}

func printBetaTesterDeleteResultMarkdown(result *BetaTesterDeleteResult) error {
	fmt.Fprintln(os.Stdout, "| ID | Email | Deleted |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- |")
	fmt.Fprintf(os.Stdout, "| %s | %s | %t |\n",
		escapeMarkdown(result.ID),
		escapeMarkdown(result.Email),
		result.Deleted,
	)
	return nil
}

func printBetaTesterGroupsUpdateResultTable(result *BetaTesterGroupsUpdateResult) error {
	headers := []string{"Tester ID", "Group IDs", "Action"}
	rows := [][]string{{result.TesterID, strings.Join(result.GroupIDs, ","), result.Action}}
	RenderTable(headers, rows)
	return nil
}

func printBetaTesterGroupsUpdateResultMarkdown(result *BetaTesterGroupsUpdateResult) error {
	fmt.Fprintln(os.Stdout, "| Tester ID | Group IDs | Action |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- |")
	fmt.Fprintf(os.Stdout, "| %s | %s | %s |\n",
		escapeMarkdown(result.TesterID),
		escapeMarkdown(strings.Join(result.GroupIDs, ",")),
		escapeMarkdown(result.Action),
	)
	return nil
}

func printBetaTesterAppsUpdateResultTable(result *BetaTesterAppsUpdateResult) error {
	headers := []string{"Tester ID", "App IDs", "Action"}
	rows := [][]string{{result.TesterID, strings.Join(result.AppIDs, ","), result.Action}}
	RenderTable(headers, rows)
	return nil
}

func printBetaTesterAppsUpdateResultMarkdown(result *BetaTesterAppsUpdateResult) error {
	fmt.Fprintln(os.Stdout, "| Tester ID | App IDs | Action |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- |")
	fmt.Fprintf(os.Stdout, "| %s | %s | %s |\n",
		escapeMarkdown(result.TesterID),
		escapeMarkdown(strings.Join(result.AppIDs, ",")),
		escapeMarkdown(result.Action),
	)
	return nil
}

func printBetaTesterBuildsUpdateResultTable(result *BetaTesterBuildsUpdateResult) error {
	headers := []string{"Tester ID", "Build IDs", "Action"}
	rows := [][]string{{result.TesterID, strings.Join(result.BuildIDs, ","), result.Action}}
	RenderTable(headers, rows)
	return nil
}

func printBetaTesterBuildsUpdateResultMarkdown(result *BetaTesterBuildsUpdateResult) error {
	fmt.Fprintln(os.Stdout, "| Tester ID | Build IDs | Action |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- |")
	fmt.Fprintf(os.Stdout, "| %s | %s | %s |\n",
		escapeMarkdown(result.TesterID),
		escapeMarkdown(strings.Join(result.BuildIDs, ",")),
		escapeMarkdown(result.Action),
	)
	return nil
}

func printAppBetaTestersUpdateResultTable(result *AppBetaTestersUpdateResult) error {
	headers := []string{"App ID", "Tester IDs", "Action"}
	rows := [][]string{{result.AppID, strings.Join(result.TesterIDs, ","), result.Action}}
	RenderTable(headers, rows)
	return nil
}

func printAppBetaTestersUpdateResultMarkdown(result *AppBetaTestersUpdateResult) error {
	fmt.Fprintln(os.Stdout, "| App ID | Tester IDs | Action |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- |")
	fmt.Fprintf(os.Stdout, "| %s | %s | %s |\n",
		escapeMarkdown(result.AppID),
		escapeMarkdown(strings.Join(result.TesterIDs, ",")),
		escapeMarkdown(result.Action),
	)
	return nil
}

func printBetaFeedbackSubmissionDeleteResultTable(result *BetaFeedbackSubmissionDeleteResult) error {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	RenderTable(headers, rows)
	return nil
}

func printBetaFeedbackSubmissionDeleteResultMarkdown(result *BetaFeedbackSubmissionDeleteResult) error {
	fmt.Fprintln(os.Stdout, "| ID | Deleted |")
	fmt.Fprintln(os.Stdout, "| --- | --- |")
	fmt.Fprintf(os.Stdout, "| %s | %t |\n",
		escapeMarkdown(result.ID),
		result.Deleted,
	)
	return nil
}

func printBetaTesterInvitationResultTable(result *BetaTesterInvitationResult) error {
	headers := []string{"Invitation ID", "Tester ID", "App ID", "Email"}
	rows := [][]string{{result.InvitationID, result.TesterID, result.AppID, result.Email}}
	RenderTable(headers, rows)
	return nil
}

func printBetaTesterInvitationResultMarkdown(result *BetaTesterInvitationResult) error {
	fmt.Fprintln(os.Stdout, "| Invitation ID | Tester ID | App ID | Email |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- | --- |")
	fmt.Fprintf(os.Stdout, "| %s | %s | %s | %s |\n",
		escapeMarkdown(result.InvitationID),
		escapeMarkdown(result.TesterID),
		escapeMarkdown(result.AppID),
		escapeMarkdown(result.Email),
	)
	return nil
}
