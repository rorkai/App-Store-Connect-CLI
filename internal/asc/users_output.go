package asc

import (
	"fmt"
	"os"
	"strings"
)

func formatPersonName(firstName, lastName string) string {
	first := strings.TrimSpace(firstName)
	last := strings.TrimSpace(lastName)
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

func formatUserUsername(attr UserAttributes) string {
	username := strings.TrimSpace(attr.Username)
	if username != "" {
		return username
	}
	return strings.TrimSpace(attr.Email)
}

func printUsersTable(resp *UsersResponse) error {
	headers := []string{"ID", "Username", "Name", "Roles", "All Apps", "Provisioning"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{
			item.ID,
			compactWhitespace(formatUserUsername(item.Attributes)),
			compactWhitespace(formatPersonName(item.Attributes.FirstName, item.Attributes.LastName)),
			compactWhitespace(strings.Join(item.Attributes.Roles, ",")),
			fmt.Sprintf("%t", item.Attributes.AllAppsVisible),
			fmt.Sprintf("%t", item.Attributes.ProvisioningAllowed),
		})
	}
	RenderTable(headers, rows)
	return nil
}

func printUsersMarkdown(resp *UsersResponse) error {
	fmt.Fprintln(os.Stdout, "| ID | Username | Name | Roles | All Apps | Provisioning |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- | --- | --- | --- |")
	for _, item := range resp.Data {
		fmt.Fprintf(os.Stdout, "| %s | %s | %s | %s | %t | %t |\n",
			escapeMarkdown(item.ID),
			escapeMarkdown(formatUserUsername(item.Attributes)),
			escapeMarkdown(formatPersonName(item.Attributes.FirstName, item.Attributes.LastName)),
			escapeMarkdown(strings.Join(item.Attributes.Roles, ",")),
			item.Attributes.AllAppsVisible,
			item.Attributes.ProvisioningAllowed,
		)
	}
	return nil
}

func printUserInvitationsTable(resp *UserInvitationsResponse) error {
	headers := []string{"ID", "Email", "Name", "Roles", "All Apps", "Provisioning", "Expires"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{
			item.ID,
			compactWhitespace(item.Attributes.Email),
			compactWhitespace(formatPersonName(item.Attributes.FirstName, item.Attributes.LastName)),
			compactWhitespace(strings.Join(item.Attributes.Roles, ",")),
			fmt.Sprintf("%t", item.Attributes.AllAppsVisible),
			fmt.Sprintf("%t", item.Attributes.ProvisioningAllowed),
			compactWhitespace(item.Attributes.ExpirationDate),
		})
	}
	RenderTable(headers, rows)
	return nil
}

func printUserInvitationsMarkdown(resp *UserInvitationsResponse) error {
	fmt.Fprintln(os.Stdout, "| ID | Email | Name | Roles | All Apps | Provisioning | Expires |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- | --- | --- | --- | --- |")
	for _, item := range resp.Data {
		fmt.Fprintf(os.Stdout, "| %s | %s | %s | %s | %t | %t | %s |\n",
			escapeMarkdown(item.ID),
			escapeMarkdown(item.Attributes.Email),
			escapeMarkdown(formatPersonName(item.Attributes.FirstName, item.Attributes.LastName)),
			escapeMarkdown(strings.Join(item.Attributes.Roles, ",")),
			item.Attributes.AllAppsVisible,
			item.Attributes.ProvisioningAllowed,
			escapeMarkdown(item.Attributes.ExpirationDate),
		)
	}
	return nil
}

func printUserDeleteResultTable(result *UserDeleteResult) error {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	RenderTable(headers, rows)
	return nil
}

func printUserDeleteResultMarkdown(result *UserDeleteResult) error {
	fmt.Fprintln(os.Stdout, "| ID | Deleted |")
	fmt.Fprintln(os.Stdout, "| --- | --- |")
	fmt.Fprintf(os.Stdout, "| %s | %t |\n", escapeMarkdown(result.ID), result.Deleted)
	return nil
}

func printUserInvitationRevokeResultTable(result *UserInvitationRevokeResult) error {
	headers := []string{"ID", "Revoked"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Revoked)}}
	RenderTable(headers, rows)
	return nil
}

func printUserInvitationRevokeResultMarkdown(result *UserInvitationRevokeResult) error {
	fmt.Fprintln(os.Stdout, "| ID | Revoked |")
	fmt.Fprintln(os.Stdout, "| --- | --- |")
	fmt.Fprintf(os.Stdout, "| %s | %t |\n", escapeMarkdown(result.ID), result.Revoked)
	return nil
}
