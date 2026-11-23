package reviews

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/validation"
)

const (
	reviewDetailDemoAccountNameUsage     = "Demo account name when demo credentials are required"
	reviewDetailDemoAccountPasswordUsage = "Demo account password when demo credentials are required; 100 characters or fewer"
	reviewDetailDemoAccountRequiredUsage = "Set true only when App Review needs demo credentials; leave false when reviewer guidance in --notes is enough"
	reviewDetailNotesUsage               = "Review notes for reviewer instructions or context; supplemental when demo credentials are required; at most 4,000 characters"
	reviewDetailContactPhoneUsage        = "Contact phone with a plus sign and country code, for example +1 408 555 0100"
	reviewDetailDemoCredentialsError     = "Error: --demo-account-required=true requires both --demo-account-name and --demo-account-password"
	reviewDetailDemoPasswordLengthError  = "Error: --demo-account-password must be 100 characters or fewer"
	reviewDetailDemoPasswordMaxLength    = 100
)

func newReviewDetailsGetCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("details-get", flag.ExitOnError)

	detailID := shared.BindResourceIDFlag(fs, "id", "appStoreReviewDetails", "App Store review detail ID")
	versionID := shared.BindResourceIDFlag(fs, "version-id", "appStoreVersions", "App Store version ID: get that version's review detail")
	appID := fs.String("app", "", "App Store Connect app ID, bundle ID, or exact name: get the review detail of its editable (else live) version, or require --version-id to be this app's")
	platform := fs.String("platform", "", "Platform of the version --app picks when the app has several: IOS, MAC_OS, TV_OS, VISION_OS")
	includeSensitive := shared.BindIncludeSensitiveFlag(fs)
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "details-get",
		ShortUsage: "asc review details-get --id \"DETAIL_ID\" | --version-id \"VERSION_ID\" | --app \"APP_ID\"",
		ShortHelp:  "Get an App Store review detail by ID, version, or app.",
		LongHelp: `Get an App Store review detail by ID, by its App Store version, or for an
app's editable version (its live version when none is editable).

With --version-id, --app also checks that the version belongs to the app.

Examples:
  asc review details-get --id "DETAIL_ID"
  asc review details-get --version-id "VERSION_ID"
  asc review details-get --app "APP_ID"
  asc review details-get --app "APP_ID" --version-id "VERSION_ID"`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			detailValue := strings.TrimSpace(*detailID)
			versionValue := strings.TrimSpace(*versionID)
			appValue := strings.TrimSpace(*appID)
			platformValue := strings.TrimSpace(*platform)
			if detailValue == "" && versionValue == "" && appValue == "" {
				return d.RequiredFlag("--id")
			}
			if detailValue != "" && versionValue != "" {
				return shared.WithDiagnostic(d.UsageError("--id and --version-id are mutually exclusive"), shared.DiagnosticConflictingInput, "--version-id")
			}
			if detailValue != "" && appValue != "" {
				return shared.WithDiagnostic(d.UsageError("--app selects the review detail through the app's version; pass --app with or without --version-id, not with --id"), shared.DiagnosticConflictingInput, "--app")
			}
			if platformValue != "" {
				if appValue == "" || versionValue != "" {
					return shared.WithDiagnostic(d.UsageError("--platform picks the version for --app; pass it with --app and without --version-id"), shared.DiagnosticConflictingInput, "--platform")
				}
				normalized, err := shared.NormalizeAppStoreVersionPlatform(platformValue)
				if err != nil {
					return shared.WithDiagnostic(d.UsageError(err.Error()), shared.DiagnosticInvalidInput, "--platform")
				}
				platformValue = normalized
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("review details-get: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			if detailValue != "" {
				resp, err := client.GetAppStoreReviewDetail(requestCtx, detailValue)
				if err != nil {
					return fmt.Errorf("review details-get: failed to fetch: %w", err)
				}

				shared.WarnIncludeSensitive(d.IO.ErrOut, *includeSensitive)
				return d.PrintOutput(presentableReviewDetail(resp, *includeSensitive), *output.Output, *output.Pretty)
			}

			if appValue != "" {
				resolvedAppID, err := shared.ResolveAppIDWithLookup(requestCtx, client, appValue)
				if err != nil {
					return fmt.Errorf("review details-get: %w", err)
				}
				if versionValue != "" {
					if _, err := shared.ResolveOwnedAppStoreVersionByID(requestCtx, client, resolvedAppID, versionValue, ""); err != nil {
						return fmt.Errorf("review details-get: %w", err)
					}
				} else {
					selected, err := shared.ResolveDefaultAppStoreVersion(requestCtx, client, resolvedAppID, platformValue)
					if err != nil {
						return fmt.Errorf("review details-get: %w", err)
					}
					fmt.Fprintln(d.IO.ErrOut, selected.Note("--version-id"))
					versionValue = selected.ID
				}
			}

			return printReviewDetailForVersion(requestCtx, d, client, "review details-get", versionValue, *includeSensitive, output)
		},
	}
}

func newReviewDetailsForVersionCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("details-for-version", flag.ExitOnError)

	versionID := shared.BindResourceIDFlag(fs, "version-id", "appStoreVersions", "App Store version ID (required)")
	includeSensitive := shared.BindIncludeSensitiveFlag(fs)
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "details-for-version",
		ShortUsage: "asc review details-for-version --version-id \"VERSION_ID\"",
		ShortHelp:  "Get the review detail for a version.",
		LongHelp: `Get the review detail for a specific App Store version.

Examples:
  asc review details-for-version --version-id "VERSION_ID"`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			versionValue := strings.TrimSpace(*versionID)
			if versionValue == "" {
				return d.RequiredFlag("--version-id")
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("review details-for-version: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			return printReviewDetailForVersion(requestCtx, d, client, "review details-for-version", versionValue, *includeSensitive, output)
		},
	}
}

// printReviewDetailForVersion prints the review detail of versionID, or the
// not-configured state when the version exists without one.
func printReviewDetailForVersion(ctx context.Context, d *shared.Deps, client *asc.Client, command, versionValue string, includeSensitive bool, output shared.OutputFlags) error {
	resp, err := client.GetAppStoreReviewDetailForVersion(ctx, versionValue)
	if err != nil {
		if asc.IsNotFound(err) {
			if _, versionErr := client.GetAppStoreVersion(ctx, versionValue); versionErr != nil {
				return fmt.Errorf("%s: failed to fetch: %w", command, versionErr)
			}
			message := reviewDetailNotConfiguredMessage(versionValue)
			warnNotConfigured(d, message)
			result := reviewDetailNotConfiguredResult{
				VersionID:  versionValue,
				Configured: false,
				Message:    message,
			}
			return d.PrintOutputWithRenderers(
				result,
				*output.Output,
				*output.Pretty,
				func() error {
					renderNotConfiguredState(d, "Review Detail", "versionId", versionValue, message, false)
					return nil
				},
				func() error {
					renderNotConfiguredState(d, "Review Detail", "versionId", versionValue, message, true)
					return nil
				},
			)
		}
		return fmt.Errorf("%s: failed to fetch: %w", command, err)
	}

	shared.WarnIncludeSensitive(d.IO.ErrOut, includeSensitive)
	return d.PrintOutput(presentableReviewDetail(resp, includeSensitive), *output.Output, *output.Pretty)
}

func newReviewDetailsCreateCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("details-create", flag.ExitOnError)

	versionID := shared.BindResourceIDFlag(fs, "version-id", "appStoreVersions", "App Store version ID (required)")
	contactFirstName := fs.String("contact-first-name", "", "Contact first name (required for new review details)")
	contactLastName := fs.String("contact-last-name", "", "Contact last name")
	contactEmail := fs.String("contact-email", "", "Contact email")
	contactPhone := fs.String("contact-phone", "", reviewDetailContactPhoneUsage+" (required for new review details)")
	demoAccountName := fs.String("demo-account-name", "", reviewDetailDemoAccountNameUsage)
	demoAccountPassword := fs.String("demo-account-password", "", reviewDetailDemoAccountPasswordUsage)
	demoAccountRequired := fs.Bool("demo-account-required", false, reviewDetailDemoAccountRequiredUsage)
	notes := fs.String("notes", "", reviewDetailNotesUsage)
	ifExists := shared.BindIfExistsFlag(fs, shared.IfExistsSkip, shared.IfExistsUpdate)
	includeSensitive := shared.BindIncludeSensitiveFlag(fs)
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "details-create",
		ShortUsage: "asc review details-create --version-id \"VERSION_ID\" [flags]",
		ShortHelp:  "Create App Store review details for a version.",
		LongHelp: `Create App Store review details for a version.

New review details need ` + "`--contact-first-name`" + ` and ` + "`--contact-phone`" + `, which App
Store Connect rejects a new detail without; with ` + "`--if-exists skip`" + ` or ` + "`update`" + ` they
may come from the existing detail. ` + "`--notes`" + ` allow at most 4,000 characters, and
+1 phone numbers need 10 digits after the country code. These are checked
before any request.

Leave ` + "`--demo-account-required`" + ` false when ` + "`--notes`" + ` are enough for reviewer instructions.
Use ` + "`--demo-account-required=true`" + ` only when App Review needs demo credentials.
Do not use placeholder demo credentials just to satisfy the field shape.

Examples:
  asc review details-create --version-id "VERSION_ID" --contact-first-name "Dev" --contact-last-name "Support" --contact-email "dev@example.com" --contact-phone "+1 408 555 0100" --notes "Reviewer can use the guest flow from the welcome screen."
  asc review details-create --version-id "VERSION_ID" --contact-first-name "Dev" --contact-last-name "Support" --contact-email "dev@example.com" --contact-phone "+1 408 555 0100" --demo-account-required=true --demo-account-name "reviewer@example.com" --demo-account-password "app-specific-password" --notes "2FA is disabled for this review account."
  asc review details-create --version-id "VERSION_ID" --notes "Reviewer notes" --if-exists update

--if-exists controls what happens when App Store Connect answers 409 because
the version already has review details. fail (default) returns the error.
skip reads the existing detail back, prints it unchanged, and exits 0. update
applies the same flags to the existing detail with asc review details-update.
Any other 409 keeps failing.`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			versionValue := strings.TrimSpace(*versionID)
			if versionValue == "" {
				return d.RequiredFlag("--version-id")
			}
			ifExistsMode, err := d.ParseIfExistsMode(*ifExists, shared.IfExistsSkip, shared.IfExistsUpdate)
			if err != nil {
				return err
			}

			visited := map[string]bool{}
			fs.Visit(func(f *flag.Flag) {
				visited[f.Name] = true
			})

			needsExistingDemoCredentials := ifExistsMode == shared.IfExistsUpdate &&
				visited["demo-account-required"] && *demoAccountRequired &&
				(!visited["demo-account-name"] || !visited["demo-account-password"])
			if visited["demo-account-required"] && *demoAccountRequired && !needsExistingDemoCredentials {
				if err := validateReviewDetailDemoCredentialValues(d, strings.TrimSpace(*demoAccountName), strings.TrimSpace(*demoAccountPassword)); err != nil {
					return err
				}
			} else if visited["demo-account-password"] {
				if err := validateReviewDetailDemoPasswordLength(d, strings.TrimSpace(*demoAccountPassword)); err != nil {
					return err
				}
			}
			if err := validateReviewDetailInputValues(d, visited, strings.TrimSpace(*notes), strings.TrimSpace(*contactPhone)); err != nil {
				return err
			}
			// With --if-exists skip or update the version may already carry its
			// contact fields, so only a plain create must supply them.
			if ifExistsMode == shared.IfExistsFail {
				if err := validateReviewDetailCreateContacts(d, visited, map[string]string{
					"contact-first-name": *contactFirstName,
					"contact-phone":      *contactPhone,
				}); err != nil {
					return err
				}
			}

			var attrsPtr *asc.AppStoreReviewDetailCreateAttributes
			if hasReviewDetailUpdates(visited) {
				attrs := asc.AppStoreReviewDetailCreateAttributes{}
				if visited["contact-first-name"] {
					value := strings.TrimSpace(*contactFirstName)
					attrs.ContactFirstName = &value
				}
				if visited["contact-last-name"] {
					value := strings.TrimSpace(*contactLastName)
					attrs.ContactLastName = &value
				}
				if visited["contact-email"] {
					value := strings.TrimSpace(*contactEmail)
					attrs.ContactEmail = &value
				}
				if visited["contact-phone"] {
					value := strings.TrimSpace(*contactPhone)
					attrs.ContactPhone = &value
				}
				if visited["demo-account-name"] {
					value := strings.TrimSpace(*demoAccountName)
					attrs.DemoAccountName = &value
				}
				if visited["demo-account-password"] {
					value := strings.TrimSpace(*demoAccountPassword)
					attrs.DemoAccountPassword = &value
				}
				if visited["demo-account-required"] {
					value := *demoAccountRequired
					attrs.DemoAccountRequired = &value
				}
				if visited["notes"] {
					value := strings.TrimSpace(*notes)
					attrs.Notes = &value
				}
				attrsPtr = &attrs
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("review details-create: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			var resp *asc.AppStoreReviewDetailResponse
			var existing *asc.AppStoreReviewDetailResponse
			handled := false
			if needsExistingDemoCredentials {
				existing, err = client.GetAppStoreReviewDetailForVersion(requestCtx, versionValue)
				if err != nil {
					if asc.IsNotFound(err) {
						return validateReviewDetailDemoCredentialValues(d, strings.TrimSpace(*demoAccountName), strings.TrimSpace(*demoAccountPassword))
					}
					return fmt.Errorf("review details-create: failed to read existing review details for demo credential validation: %w", err)
				}
				if strings.TrimSpace(existing.Data.ID) == "" {
					return fmt.Errorf("review details-create: existing review detail response did not include an id")
				}

				if err := validateReviewDetailDemoCredentialsWithExisting(d,
					existing.Data.Attributes,
					visited,
					strings.TrimSpace(*demoAccountName),
					strings.TrimSpace(*demoAccountPassword),
				); err != nil {
					return err
				}
				handled = true
			} else {
				resp, err = client.CreateAppStoreReviewDetail(requestCtx, versionValue, attrsPtr)
			}
			if err != nil {
				var resolveErr error
				existing, handled, resolveErr = shared.ResolveIfExistsConflict(ifExistsMode, err, reviewDetailsCreateExistsCodes, func() (*asc.AppStoreReviewDetailResponse, bool, error) {
					detail, lookupErr := client.GetAppStoreReviewDetailForVersion(requestCtx, versionValue)
					if lookupErr != nil {
						return nil, false, lookupErr
					}
					return detail, strings.TrimSpace(detail.Data.ID) != "", nil
				})
				if resolveErr != nil {
					return withReviewDetailExistsHint(ifExistsMode, fmt.Errorf("review details-create: failed to create: %w", resolveErr))
				}
				if !handled {
					return fmt.Errorf("review details-create: failed to create: %w", err)
				}
			}
			if handled {
				resp = existing
				outcome := "left unchanged"
				if ifExistsMode == shared.IfExistsUpdate && attrsPtr != nil {
					updated, updateErr := client.UpdateAppStoreReviewDetail(requestCtx, existing.Data.ID, reviewDetailUpdateAttributesFromCreate(*attrsPtr))
					if updateErr != nil {
						return fmt.Errorf("review details-create: update existing review detail %s: %w", existing.Data.ID, updateErr)
					}
					resp = updated
					outcome = "updated it in place"
				}
				fmt.Fprintf(d.IO.ErrOut, "review details-create: review detail %s already exists for version %s; %s (--if-exists %s)\n", existing.Data.ID, versionValue, outcome, ifExistsMode)
			}

			shared.WarnIncludeSensitive(d.IO.ErrOut, *includeSensitive)
			return d.PrintOutput(presentableReviewDetail(resp, *includeSensitive), *output.Output, *output.Pretty)
		},
	}
}

func newReviewDetailsUpdateCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("details-update", flag.ExitOnError)

	detailID := shared.BindResourceIDFlag(fs, "id", "appStoreReviewDetails", "App Store review detail ID (required)")
	contactFirstName := fs.String("contact-first-name", "", "Contact first name")
	contactLastName := fs.String("contact-last-name", "", "Contact last name")
	contactEmail := fs.String("contact-email", "", "Contact email")
	contactPhone := fs.String("contact-phone", "", reviewDetailContactPhoneUsage)
	demoAccountName := fs.String("demo-account-name", "", reviewDetailDemoAccountNameUsage)
	demoAccountPassword := fs.String("demo-account-password", "", reviewDetailDemoAccountPasswordUsage)
	demoAccountRequired := fs.Bool("demo-account-required", false, reviewDetailDemoAccountRequiredUsage)
	notes := fs.String("notes", "", reviewDetailNotesUsage)
	includeSensitive := shared.BindIncludeSensitiveFlag(fs)
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "details-update",
		ShortUsage: "asc review details-update --id \"DETAIL_ID\" [flags]",
		ShortHelp:  "Update App Store review details.",
		LongHelp: `Update App Store review details.

` + "`--notes`" + ` allow at most 4,000 characters, and +1 phone numbers need 10 digits
after the country code. These are checked before any request.

Leave ` + "`--demo-account-required`" + ` false when ` + "`--notes`" + ` are enough for reviewer instructions.
Use ` + "`--demo-account-required=true`" + ` only when App Review needs demo credentials.
Do not use placeholder demo credentials just to satisfy the field shape.

Examples:
  asc review details-update --id "DETAIL_ID" --notes "Reviewer can use the guest flow from the welcome screen."
  asc review details-update --id "DETAIL_ID" --demo-account-required=true --demo-account-name "reviewer@example.com" --demo-account-password "rotated-password" --notes "This account has full reviewer access."`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			detailValue := strings.TrimSpace(*detailID)
			if detailValue == "" {
				return d.RequiredFlag("--id")
			}

			visited := map[string]bool{}
			fs.Visit(func(f *flag.Flag) {
				visited[f.Name] = true
			})

			if !hasReviewDetailUpdates(visited) {
				return d.UpdateFlagRequired(fs, contactFirstName, contactLastName, contactEmail, contactPhone, demoAccountName, demoAccountPassword, demoAccountRequired, notes)
			}
			if visited["demo-account-password"] {
				if err := validateReviewDetailDemoPasswordLength(d, strings.TrimSpace(*demoAccountPassword)); err != nil {
					return err
				}
			}
			if err := validateReviewDetailInputValues(d, visited, strings.TrimSpace(*notes), strings.TrimSpace(*contactPhone)); err != nil {
				return err
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("review details-update: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			if err := validateReviewDetailUpdateDemoCredentials(d,
				requestCtx,
				client,
				detailValue,
				visited,
				*demoAccountRequired,
				strings.TrimSpace(*demoAccountName),
				strings.TrimSpace(*demoAccountPassword),
			); err != nil {
				return err
			}

			attrs := asc.AppStoreReviewDetailUpdateAttributes{}
			if visited["contact-first-name"] {
				value := strings.TrimSpace(*contactFirstName)
				attrs.ContactFirstName = &value
			}
			if visited["contact-last-name"] {
				value := strings.TrimSpace(*contactLastName)
				attrs.ContactLastName = &value
			}
			if visited["contact-email"] {
				value := strings.TrimSpace(*contactEmail)
				attrs.ContactEmail = &value
			}
			if visited["contact-phone"] {
				value := strings.TrimSpace(*contactPhone)
				attrs.ContactPhone = &value
			}
			if visited["demo-account-name"] {
				value := strings.TrimSpace(*demoAccountName)
				attrs.DemoAccountName = &value
			}
			if visited["demo-account-password"] {
				value := strings.TrimSpace(*demoAccountPassword)
				attrs.DemoAccountPassword = &value
			}
			if visited["demo-account-required"] {
				value := *demoAccountRequired
				attrs.DemoAccountRequired = &value
			}
			if visited["notes"] {
				value := strings.TrimSpace(*notes)
				attrs.Notes = &value
			}

			resp, err := client.UpdateAppStoreReviewDetail(requestCtx, detailValue, attrs)
			if err != nil {
				return fmt.Errorf("review details-update: failed to update: %w", err)
			}

			shared.WarnIncludeSensitive(d.IO.ErrOut, *includeSensitive)
			return d.PrintOutput(presentableReviewDetail(resp, *includeSensitive), *output.Output, *output.Pretty)
		},
	}
}

// withReviewDetailExistsHint names the one-flag retry when the version already
// has review details and the caller left --if-exists at fail: the create flags
// can be applied to the existing detail instead.
func withReviewDetailExistsHint(mode shared.IfExistsMode, err error) error {
	apiErr, ok := errors.AsType[*asc.APIError](err)
	if mode != shared.IfExistsFail || !ok || apiErr.StatusCode != http.StatusConflict ||
		(apiErr.Code != "STATE_ERROR.ALREADY_EXISTS" && !strings.Contains(apiErr.Detail, "already has an existing review")) {
		return err
	}
	return &shared.HintedError{
		Err:  err,
		Hint: "This version already has review details. Rerun the same command with --if-exists update to apply these flags to them, or --if-exists skip to keep them as they are.",
	}
}

// reviewDetailsCreateExistsCodes lists the Apple 409 codes accepted as "an
// appStoreReviewDetail already exists for this appStoreVersion" on
// POST /v1/appStoreReviewDetails. Live against app 6759231657 on 2026-09-15
// Apple answers this conflict with STATE_ERROR.ALREADY_EXISTS ("Resource
// already exists." / "The given app version already has an existing review.");
// the relationship and duplicate-attribute codes are kept because the version
// owns at most one detail and Apple has reported the same conflict through
// them. The read-back of GET /v1/appStoreVersions/{id}/appStoreReviewDetail is
// what finally proves existence; every other STATE_ERROR.* keeps failing.
var reviewDetailsCreateExistsCodes = []string{
	"STATE_ERROR.ALREADY_EXISTS",
	"ENTITY_ERROR.RELATIONSHIP.INVALID",
	"ENTITY_ERROR.ATTRIBUTE.INVALID.DUPLICATE",
	"ENTITY_ERROR.ATTRIBUTE.INVALID.ALREADY_EXISTS",
}

// reviewDetailUpdateAttributesFromCreate carries the create flags over to the
// PATCH schema, which accepts the same attribute set.
func reviewDetailUpdateAttributesFromCreate(attrs asc.AppStoreReviewDetailCreateAttributes) asc.AppStoreReviewDetailUpdateAttributes {
	return asc.AppStoreReviewDetailUpdateAttributes(attrs)
}

// presentableReviewDetail withholds the demo account password unless the caller
// opted in for this invocation. The fetched response keeps its real value so
// validation and request construction stay unaffected.
func presentableReviewDetail(resp *asc.AppStoreReviewDetailResponse, includeSensitive bool) *asc.AppStoreReviewDetailResponse {
	if includeSensitive {
		return resp
	}
	return asc.RedactAppStoreReviewDetailResponse(resp)
}

func hasReviewDetailUpdates(visited map[string]bool) bool {
	return visited["contact-first-name"] ||
		visited["contact-last-name"] ||
		visited["contact-email"] ||
		visited["contact-phone"] ||
		visited["demo-account-name"] ||
		visited["demo-account-password"] ||
		visited["demo-account-required"] ||
		visited["notes"]
}

func validateReviewDetailUpdateDemoCredentials(d *shared.Deps,
	ctx context.Context,
	client *asc.Client,
	detailID string,
	visited map[string]bool,
	demoAccountRequired bool,
	demoAccountName string,
	demoAccountPassword string,
) error {
	if !visited["demo-account-required"] || !demoAccountRequired {
		return nil
	}

	if visited["demo-account-name"] && visited["demo-account-password"] {
		return validateReviewDetailDemoCredentialValues(d, demoAccountName, demoAccountPassword)
	}

	resp, err := client.GetAppStoreReviewDetail(ctx, detailID)
	if err != nil {
		return fmt.Errorf("review details-update: failed to fetch existing review details for demo credential validation: %w", err)
	}

	return validateReviewDetailDemoCredentialsWithExisting(d, resp.Data.Attributes, visited, demoAccountName, demoAccountPassword)
}

func validateReviewDetailDemoCredentialsWithExisting(d *shared.Deps,
	existing asc.AppStoreReviewDetailAttributes,
	visited map[string]bool,
	demoAccountName string,
	demoAccountPassword string,
) error {
	effectiveName := demoAccountName
	if !visited["demo-account-name"] {
		effectiveName = strings.TrimSpace(existing.DemoAccountName)
	}
	effectivePassword := demoAccountPassword
	if !visited["demo-account-password"] {
		effectivePassword = strings.TrimSpace(existing.DemoAccountPassword)
	}
	return validateReviewDetailDemoCredentialValues(d, effectiveName, effectivePassword)
}

func validateReviewDetailDemoCredentialValues(d *shared.Deps, demoAccountName, demoAccountPassword string) error {
	if strings.TrimSpace(demoAccountName) == "" || strings.TrimSpace(demoAccountPassword) == "" {
		fmt.Fprintln(d.IO.ErrOut, reviewDetailDemoCredentialsError)
		return shared.WithDiagnostic(flag.ErrHelp, shared.DiagnosticRequiredInputMissing, "")
	}

	return validateReviewDetailDemoPasswordLength(d, demoAccountPassword)
}

func validateReviewDetailDemoPasswordLength(d *shared.Deps, demoAccountPassword string) error {
	if utf8.RuneCountInString(strings.TrimSpace(demoAccountPassword)) <= reviewDetailDemoPasswordMaxLength {
		return nil
	}

	fmt.Fprintln(d.IO.ErrOut, reviewDetailDemoPasswordLengthError)
	return shared.WithDiagnostic(flag.ErrHelp, shared.DiagnosticInvalidInput, "--demo-account-password")
}

// reviewDetailCreateContactFlags lists the contact flags App Store Connect was
// observed rejecting a new review detail without. Last name and email are left
// to App Store Connect.
var reviewDetailCreateContactFlags = []string{
	"contact-first-name",
	"contact-phone",
}

// validateReviewDetailInputValues checks the notes and contact phone the
// command is about to send. App Store Connect otherwise rejects them only after
// the request, without saying by how much the notes are over the limit.
func validateReviewDetailInputValues(d *shared.Deps, visited map[string]bool, notes, contactPhone string) error {
	if visited["notes"] {
		if err := validateReviewDetailNotesLength(d, notes); err != nil {
			return err
		}
	}
	if visited["contact-phone"] {
		return validateReviewDetailContactPhone(d, contactPhone)
	}
	return nil
}

func validateReviewDetailNotesLength(d *shared.Deps, notes string) error {
	length := validation.ReviewNotesLength(notes)
	if length <= validation.LimitReviewNotes {
		return nil
	}
	over := length - validation.LimitReviewNotes
	unit := "characters"
	if over == 1 {
		unit = "character"
	}
	message := fmt.Sprintf(
		"--notes is %s characters; App Store review notes allow at most %s. Remove at least %s %s.",
		formatReviewDetailCount(length),
		formatReviewDetailCount(validation.LimitReviewNotes),
		formatReviewDetailCount(over),
		unit,
	)
	return reportReviewDetailUsageError(d, shared.UsageErrorInvalidValue, shared.DiagnosticInvalidInput, "--notes", message)
}

// validateReviewDetailContactPhone rejects only +1 numbers whose digit count
// cannot be a North American number. App Store Connect accepts numbers without
// a plus sign and validates other country codes against numbering data the CLI
// does not carry, so every other value is left to App Store Connect.
func validateReviewDetailContactPhone(d *shared.Deps, contactPhone string) error {
	count, ok := validation.NANPNationalDigitCount(contactPhone)
	if !ok || count == validation.NANPNationalNumberDigits {
		return nil
	}
	message := fmt.Sprintf(
		"--contact-phone has %d digits after +1; North American (+1) numbers need exactly %d (3-digit area code and 7-digit number), for example +1 408 555 0100",
		count,
		validation.NANPNationalNumberDigits,
	)
	return reportReviewDetailUsageError(d, shared.UsageErrorInvalidValue, shared.DiagnosticInvalidInput, "--contact-phone", message)
}

// validateReviewDetailCreateContacts requires the contact fields App Store
// Connect demands on a new review detail; it rejects the POST one missing field
// at a time.
func validateReviewDetailCreateContacts(d *shared.Deps, visited map[string]bool, values map[string]string) error {
	var missing []string
	for _, name := range reviewDetailCreateContactFlags {
		if !visited[name] || strings.TrimSpace(values[name]) == "" {
			missing = append(missing, "--"+name)
		}
	}
	if len(missing) == 0 {
		return nil
	}
	message := fmt.Sprintf(
		"review details-create needs %s; App Store Connect rejects a new review detail without them. To change a version that already has review details, use asc review details-update or --if-exists update.",
		strings.Join(missing, " and "),
	)
	return reportReviewDetailUsageError(d, shared.UsageErrorMissingRequired, shared.DiagnosticRequiredInputMissing, missing[0], message)
}

// reportReviewDetailUsageError prints one diagnostic line and returns a usage
// error (exit 2) without the full usage page.
func reportReviewDetailUsageError(d *shared.Deps, kind shared.UsageErrorKind, code shared.DiagnosticCode, parameter, message string) error {
	fmt.Fprintf(d.IO.ErrOut, "Error: %s\n", message)
	return shared.WithDiagnostic(shared.NewReportedUsageError(kind, message), code, parameter)
}

// formatReviewDetailCount formats a non-negative count with thousands
// separators.
func formatReviewDetailCount(value int) string {
	digits := strconv.Itoa(value)
	var formatted strings.Builder
	for i, digit := range digits {
		if i > 0 && (len(digits)-i)%3 == 0 {
			formatted.WriteByte(',')
		}
		formatted.WriteRune(digit)
	}
	return formatted.String()
}
