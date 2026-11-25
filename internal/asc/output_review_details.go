package asc

import (
	"fmt"
	"strings"
)

func formatReviewDetailContactName(attr AppStoreReviewDetailAttributes) string {
	first := strings.TrimSpace(attr.ContactFirstName)
	last := strings.TrimSpace(attr.ContactLastName)
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

func appStoreReviewDetailRows(resp *AppStoreReviewDetailResponse) ([]string, [][]string) {
	headers := []string{"ID", "Contact", "Email", "Phone", "Demo Required", "Demo Account", "Notes"}
	attr := resp.Data.Attributes
	rows := [][]string{{
		resp.Data.ID,
		compactWhitespace(formatReviewDetailContactName(attr)),
		compactWhitespace(attr.ContactEmail),
		compactWhitespace(attr.ContactPhone),
		fmt.Sprintf("%t", attr.DemoAccountRequired),
		compactWhitespace(attr.DemoAccountName),
		compactWhitespace(attr.Notes),
	}}
	return headers, rows
}

// ReviewDetailsResult reports a file-driven review detail plan or set.
type ReviewDetailsResult struct {
	VersionID string                `json:"versionId"`
	Action    string                `json:"action"`
	Status    string                `json:"status"`
	Changes   []ReviewDetailsChange `json:"changes"`
	Error     string                `json:"error,omitempty"`
}

// ReviewDetailsChange describes a managed field with secrets already redacted.
type ReviewDetailsChange struct {
	Field string `json:"field"`
	From  string `json:"from,omitempty"`
	To    string `json:"to"`
}

func reviewDetailsResultRows(result *ReviewDetailsResult) ([]string, [][]string) {
	fields := make([]string, 0, len(result.Changes))
	for _, change := range result.Changes {
		fields = append(fields, change.Field)
	}
	return []string{"Version ID", "Action", "Status", "Changed fields", "Error"}, [][]string{{result.VersionID, result.Action, result.Status, strings.Join(fields, ", "), result.Error}}
}
