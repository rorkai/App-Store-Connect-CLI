package web

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

// webAppCreateExistsCodes are the Apple error codes that make a failed
// POST /iris/v1/apps a candidate "already exists" conflict. Captured live on
// 2026-09-29 by re-creating disposable app 6759231657 with its own name,
// bundle ID and SKU: HTTP 409 with three errors[] entries, codes
// DUPLICATE, DUPLICATE and DUPLICATE.SAME_ACCOUNT. The detail and source of
// each entry were not captured, so which attribute each code refers to is
// unknown and nothing here keys on it; the public-API read-back is decisive.
// DUPLICATE.DIFFERENT_ACCOUNT (a name held by another team) is deliberately
// not listed: it cannot mean the caller's own app exists.
var webAppCreateExistsCodes = []string{
	"ENTITY_ERROR.ATTRIBUTE.INVALID.DUPLICATE",
	"ENTITY_ERROR.ATTRIBUTE.INVALID.DUPLICATE.SAME_ACCOUNT",
}

// webAppCreateIfExistsModes are the --if-exists modes web apps create accepts.
// update is not offered: the create inputs that could differ on an app whose
// bundle ID, name and SKU all match (initial platform and version string,
// company name) are create-only, and changing the primary locale of an
// existing app is not what a retried create means.
var webAppCreateIfExistsModes = []shared.IfExistsMode{shared.IfExistsSkip}

// isWebAppCreateExistsConflict reports whether err is a web API 409 any of
// whose errors[] codes is one of webAppCreateExistsCodes.
func isWebAppCreateExistsConflict(err error) bool {
	apiErr, ok := errors.AsType[*webcore.APIError](err)
	if !ok || apiErr == nil || apiErr.Status != http.StatusConflict {
		return false
	}
	for _, code := range apiErr.AllCodes() {
		for _, candidate := range webAppCreateExistsCodes {
			if strings.EqualFold(code, candidate) {
				return true
			}
		}
	}
	return false
}

// acceptedExistingAppNames lists the names an existing app may carry and still
// be the app this invocation asked for: the requested name and, when
// --auto-rename is on, every suffixed name the rename loop could have created
// on an earlier run.
func acceptedExistingAppNames(opts AppsCreateRunOptions) []string {
	names := []string{opts.Name}
	if !opts.AutoRename {
		return names
	}
	suffix := bundleIDNameSuffix(opts.BundleID)
	if suffix == "" {
		return names
	}
	for attempt := 0; attempt < appCreateAutoRenameAttempts; attempt++ {
		if candidate := autoRenameCandidate(opts.Name, suffix, attempt); candidate != "" {
			names = append(names, candidate)
		}
	}
	return names
}

// resolveWebAppCreateConflict applies --if-exists skip to a failed create.
//
// It returns handled=false when the failure is not a listed 409 or when
// neither the bundle ID nor the SKU is held by an app on this account; the
// caller then continues exactly as without --if-exists (including
// --auto-rename). It returns handled=true with a receipt when the app whose
// bundle ID matches also has the requested SKU and name, and handled=true with
// an error, which the caller must not auto-rename past, when the bundle ID or
// SKU belongs to a different app or the read-back fails.
func resolveWebAppCreateConflict(ctx context.Context, client *asc.Client, opts AppsCreateRunOptions, createErr error) (*asc.WebAppCreateIfExistsResult, bool, error) {
	if !isWebAppCreateExistsConflict(createErr) {
		return nil, false, nil
	}

	readCtx, cancel := shared.ContextWithTimeout(ctx)
	defer cancel()

	byBundleID, err := client.GetApps(readCtx, asc.WithAppsBundleIDs([]string{opts.BundleID}), asc.Limit[asc.AppsOption](200))
	if err != nil {
		return nil, true, fmt.Errorf("%w (read-back after conflict failed: %w)", createErr, err)
	}
	if existing := findAppByBundleID(byBundleID, opts.BundleID); existing != nil {
		attrs := existing.Attributes
		if attrs.SKU == opts.SKU && containsExactString(acceptedExistingAppNames(opts), attrs.Name) {
			return &asc.WebAppCreateIfExistsResult{
				ID:       strings.TrimSpace(existing.ID),
				Name:     attrs.Name,
				BundleID: attrs.BundleID,
				SKU:      attrs.SKU,
				IdempotentWriteReceipt: asc.IdempotentWriteReceipt{
					AlreadyExists: true,
					Action:        asc.IdempotentWriteActionSkipped,
				},
			}, true, nil
		}
		return nil, true, fmt.Errorf(
			"%w (--if-exists skip: bundle ID %q belongs to app %s with name %q and SKU %q, which does not match the requested name %q and SKU %q; not retrying with --auto-rename)",
			createErr, opts.BundleID, strings.TrimSpace(existing.ID), attrs.Name, attrs.SKU, opts.Name, opts.SKU,
		)
	}

	bySKU, err := client.GetApps(readCtx, asc.WithAppsSKUs([]string{opts.SKU}), asc.Limit[asc.AppsOption](200))
	if err != nil {
		return nil, true, fmt.Errorf("%w (read-back after conflict failed: %w)", createErr, err)
	}
	if bySKU != nil {
		for i := range bySKU.Data {
			app := &bySKU.Data[i]
			if app.Attributes.SKU == opts.SKU {
				return nil, true, fmt.Errorf(
					"%w (--if-exists skip: SKU %q is already used by app %s with bundle ID %q; not retrying with --auto-rename)",
					createErr, opts.SKU, strings.TrimSpace(app.ID), app.Attributes.BundleID,
				)
			}
		}
	}
	return nil, false, nil
}

func findAppByBundleID(resp *asc.AppsResponse, bundleID string) *asc.Resource[asc.AppAttributes] {
	if resp == nil {
		return nil
	}
	for i := range resp.Data {
		if strings.EqualFold(strings.TrimSpace(resp.Data[i].Attributes.BundleID), bundleID) {
			return &resp.Data[i]
		}
	}
	return nil
}

func containsExactString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
