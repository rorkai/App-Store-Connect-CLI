package asc

import (
	"fmt"
	"os"
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

func printAppStoreReviewDetailTable(resp *AppStoreReviewDetailResponse) error {
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
	RenderTable(headers, rows)
	return nil
}

func printAppStoreReviewDetailMarkdown(resp *AppStoreReviewDetailResponse) error {
	attr := resp.Data.Attributes
	fmt.Fprintln(os.Stdout, "| ID | Contact | Email | Phone | Demo Required | Demo Account | Notes |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- | --- | --- | --- | --- |")
	fmt.Fprintf(os.Stdout, "| %s | %s | %s | %s | %t | %s | %s |\n",
		escapeMarkdown(resp.Data.ID),
		escapeMarkdown(formatReviewDetailContactName(attr)),
		escapeMarkdown(attr.ContactEmail),
		escapeMarkdown(attr.ContactPhone),
		attr.DemoAccountRequired,
		escapeMarkdown(attr.DemoAccountName),
		escapeMarkdown(attr.Notes),
	)
	return nil
}
