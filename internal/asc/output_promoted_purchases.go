package asc

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// PromotedPurchaseDeleteResult represents CLI output for promoted purchase deletions.
type PromotedPurchaseDeleteResult struct {
	ID      string `json:"id"`
	Deleted bool   `json:"deleted"`
}

// AppPromotedPurchasesLinkResult represents CLI output for linking promoted purchases.
type AppPromotedPurchasesLinkResult struct {
	AppID               string   `json:"appId"`
	PromotedPurchaseIDs []string `json:"promotedPurchaseIds"`
	Action              string   `json:"action"`
}

func promotedPurchaseBool(value *bool) string {
	if value == nil {
		return ""
	}
	return strconv.FormatBool(*value)
}

func printPromotedPurchasesTable(resp *PromotedPurchasesResponse) error {
	headers := []string{"ID", "Visible For All Users", "Enabled", "State"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{
			item.ID,
			promotedPurchaseBool(item.Attributes.VisibleForAllUsers),
			promotedPurchaseBool(item.Attributes.Enabled),
			item.Attributes.State,
		})
	}
	RenderTable(headers, rows)
	return nil
}

func printPromotedPurchasesMarkdown(resp *PromotedPurchasesResponse) error {
	fmt.Fprintln(os.Stdout, "| ID | Visible For All Users | Enabled | State |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- | --- |")
	for _, item := range resp.Data {
		fmt.Fprintf(os.Stdout, "| %s | %s | %s | %s |\n",
			escapeMarkdown(item.ID),
			escapeMarkdown(promotedPurchaseBool(item.Attributes.VisibleForAllUsers)),
			escapeMarkdown(promotedPurchaseBool(item.Attributes.Enabled)),
			escapeMarkdown(item.Attributes.State),
		)
	}
	return nil
}

func printPromotedPurchaseDeleteResultTable(result *PromotedPurchaseDeleteResult) error {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	RenderTable(headers, rows)
	return nil
}

func printPromotedPurchaseDeleteResultMarkdown(result *PromotedPurchaseDeleteResult) error {
	fmt.Fprintln(os.Stdout, "| ID | Deleted |")
	fmt.Fprintln(os.Stdout, "| --- | --- |")
	fmt.Fprintf(os.Stdout, "| %s | %t |\n", escapeMarkdown(result.ID), result.Deleted)
	return nil
}

func printAppPromotedPurchasesLinkResultTable(result *AppPromotedPurchasesLinkResult) error {
	headers := []string{"App ID", "Promoted Purchase IDs", "Action"}
	rows := [][]string{{
		result.AppID,
		strings.Join(result.PromotedPurchaseIDs, ", "),
		result.Action,
	}}
	RenderTable(headers, rows)
	return nil
}

func printAppPromotedPurchasesLinkResultMarkdown(result *AppPromotedPurchasesLinkResult) error {
	fmt.Fprintln(os.Stdout, "| App ID | Promoted Purchase IDs | Action |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- |")
	fmt.Fprintf(os.Stdout, "| %s | %s | %s |\n",
		escapeMarkdown(result.AppID),
		escapeMarkdown(strings.Join(result.PromotedPurchaseIDs, ", ")),
		escapeMarkdown(result.Action),
	)
	return nil
}
