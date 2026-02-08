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

func printAppStoreReviewDetailTable(resp *AppStoreReviewDetailResponse) error {
	h, r := appStoreReviewDetailRows(resp)
	RenderTable(h, r)
	return nil
}

func printAppStoreReviewDetailMarkdown(resp *AppStoreReviewDetailResponse) error {
	h, r := appStoreReviewDetailRows(resp)
	RenderMarkdown(h, r)
	return nil
}
