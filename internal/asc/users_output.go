package asc

import (
	"fmt"
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

func usersRows(resp *UsersResponse) ([]string, [][]string) {
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
	return headers, rows
}

func userInvitationsRows(resp *UserInvitationsResponse) ([]string, [][]string) {
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
	return headers, rows
}

func userDeleteResultRows(result *UserDeleteResult) ([]string, [][]string) {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	return headers, rows
}

func userInvitationRevokeResultRows(result *UserInvitationRevokeResult) ([]string, [][]string) {
	headers := []string{"ID", "Revoked"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Revoked)}}
	return headers, rows
}
