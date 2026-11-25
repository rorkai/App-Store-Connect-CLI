package asc

import (
	"fmt"
	"slices"
	"strings"
)

// AppRenameResult represents a localized app-name change.
type AppRenameResult struct {
	AppID          string `json:"appId"`
	AppInfoID      string `json:"appInfoId"`
	Locale         string `json:"locale"`
	Name           string `json:"name"`
	Action         string `json:"action"`
	LocalizationID string `json:"localizationId"`
}

// WebAppCreateResult is the mutation receipt for `asc web apps create` when
// --access is set. Access is observed from a post-create users re-read, not
// copied from the request flags.
type WebAppCreateResult struct {
	ID     string   `json:"id"`
	Access string   `json:"access"`
	Users  []string `json:"users"`
}

// WebAppCreateIfExistsResult is the receipt for `asc web apps create
// --if-exists skip` when the app already exists. Every field comes from the
// public-API read-back of the existing app, not from the request flags.
type WebAppCreateIfExistsResult struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	BundleID string `json:"bundleId"`
	SKU      string `json:"sku"`
	IdempotentWriteReceipt
}

func webAppCreateIfExistsResultRows(result *WebAppCreateIfExistsResult) ([]string, [][]string) {
	return []string{"ID", "Name", "Bundle ID", "SKU", "Already Exists", "Action"}, [][]string{{
		result.ID,
		compactWhitespace(result.Name),
		result.BundleID,
		compactWhitespace(result.SKU),
		fmt.Sprintf("%t", result.AlreadyExists),
		result.Action,
	}}
}

func appRenameResultRows(result *AppRenameResult) ([]string, [][]string) {
	return []string{"App ID", "App Info ID", "Locale", "Name", "Action", "Localization ID"}, [][]string{{
		result.AppID,
		result.AppInfoID,
		result.Locale,
		compactWhitespace(result.Name),
		result.Action,
		result.LocalizationID,
	}}
}

func webAppCreateResultRows(result *WebAppCreateResult) ([]string, [][]string) {
	users := result.Users
	if users == nil {
		users = []string{}
	}
	return []string{"ID", "Access", "Users"}, [][]string{{
		result.ID,
		compactWhitespace(result.Access),
		compactWhitespace(strings.Join(users, ",")),
	}}
}

func appsRows(resp *AppsResponse) ([]string, [][]string) {
	headers := []string{"ID", "Name", "Bundle ID", "SKU"}
	showInvitationMode := slices.ContainsFunc(resp.Data, func(app Resource[AppAttributes]) bool {
		return app.Attributes.GroupPurchaseInvitationMode != nil
	})
	if showInvitationMode {
		headers = append(headers, "Group Purchase Invitation Mode")
	}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		row := []string{
			item.ID,
			compactWhitespace(item.Attributes.Name),
			item.Attributes.BundleID,
			item.Attributes.SKU,
		}
		if showInvitationMode {
			row = append(row, formatOptionalString((*string)(item.Attributes.GroupPurchaseInvitationMode)))
		}
		rows = append(rows, row)
	}
	return headers, rows
}
