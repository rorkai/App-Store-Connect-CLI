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
			userBoolCell(item.Attributes.AllAppsVisible, item.Attributes.hasField("allAppsVisible")),
			userBoolCell(item.Attributes.ProvisioningAllowed, item.Attributes.hasField("provisioningAllowed")),
		})
	}
	return headers, rows
}

// userBoolCell leaves an attribute Apple did not return blank instead of false.
func userBoolCell(value, present bool) string {
	if !present {
		return ""
	}
	return fmt.Sprintf("%t", value)
}

func userInvitationBoolCell(value *bool) string {
	if value == nil {
		return ""
	}
	return userBoolCell(*value, true)
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
			userInvitationBoolCell(item.Attributes.AllAppsVisible),
			userInvitationBoolCell(item.Attributes.ProvisioningAllowed),
			compactWhitespace(item.Attributes.ExpirationDate),
		})
	}
	return headers, rows
}

func userInvitationRevokeResultRows(result *UserInvitationRevokeResult) ([]string, [][]string) {
	headers := []string{"ID", "Revoked"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Revoked)}}
	return headers, rows
}
