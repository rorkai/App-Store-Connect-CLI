package metadata

import (
	"context"
	"flag"
	"fmt"
	"maps"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/validation"
)

// NewReviewDetailsCommand shares metadata's review-information file and reconciliation engine.
func NewReviewDetailsCommand(d *shared.Deps) *ffcli.Command {
	return &ffcli.Command{
		Name:        "details",
		ShortUsage:  "asc review details <subcommand> [flags]",
		ShortHelp:   "Plan and set review details from a JSON file.",
		FlagSet:     d.NewFlagSet("details", flag.ExitOnError),
		UsageFunc:   d.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{newReviewDetailsFileCommand(d, false), newReviewDetailsFileCommand(d, true)},
		Exec:        func(context.Context, []string) error { return flag.ErrHelp },
	}
}

func newReviewDetailsFileCommand(d *shared.Deps, apply bool) *ffcli.Command {
	name, help := "plan", "Show review detail changes without writing."
	if apply {
		name, help = "set", "Create or update review details from a JSON file."
	}
	fs := d.NewFlagSet(name, flag.ExitOnError)
	appID := fs.String("app", "", "App Store Connect app ID to constrain the version (or ASC_APP_ID env)")
	versionID := shared.BindResourceIDFlag(fs, "version-id", "appStoreVersions", "App Store version ID (required)")
	file := fs.String("file", "", "Review information JSON file (required)")
	output := d.BindOutputFlags(fs)
	return &ffcli.Command{
		Name:       name,
		ShortUsage: "asc review details " + name + " --version-id VERSION_ID --file review-information.json",
		ShortHelp:  help,
		FlagSet:    fs,
		UsageFunc:  d.DefaultUsageFunc,
		LongHelp: `The file uses the same fields as
metadata's review-information.json: contactFirstName, contactLastName,
contactPhone, contactEmail, demoAccountRequired, demoAccountName,
demoAccountPassword, and notes. Only supplied fields are managed; omitted
fields stay unchanged. Nulls, empty strings, unknown and duplicate fields fail.

Example file:
  {"contactFirstName":"Dev","contactPhone":"+1 408 555 0100","demoAccountRequired":false,"notes":"Use the guest flow."}

A new record needs contactFirstName, contactPhone and an explicit
demoAccountRequired value (true or false). When demoAccountRequired
is true, both demoAccountName and demoAccountPassword must be present in the
file or existing record. Passwords must use "@env:NAME" or "@file:relative-path";
file references stay beneath the JSON file's directory. Output redacts passwords.

plan reads Apple state and prints the difference. set validates the entire file,
reads current state, writes only changed fields, and verifies them by reading
back. An identical second set reports noop and sends no mutation. Concurrent
edits can still conflict; inspect the error and plan again.

When --app, ASC_APP_ID, or the configured default selects an app, the version
must belong to it before details are read or changed. Without a selected app,
--version-id alone selects the resource.

Examples:
  asc review details plan --version-id VERSION_ID --file review-information.json
  asc review details set --version-id VERSION_ID --file review-information.json`,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return d.UsageError("unexpected positional arguments")
			}
			id := strings.TrimSpace(*versionID)
			if id == "" {
				return d.RequiredFlag("--version-id")
			}
			if strings.TrimSpace(*file) == "" {
				return d.RequiredFlag("--file")
			}
			if _, err := shared.ValidateOutputFormat(*output.Output, *output.Pretty); err != nil {
				return d.UsageError(err.Error())
			}
			path, err := filepath.Abs(strings.TrimSpace(*file))
			if err != nil {
				return err
			}
			scope := reviewInformationSettingScope
			scope.fileName = filepath.Base(path)
			local, err := readMetadataSettingFile(d, filepath.Dir(path), scope, "--file", "--file does not exist:")
			if err != nil {
				return err
			}
			if len(local) == 0 {
				return shared.WithDiagnostic(d.UsageError("--file must contain at least one field"), shared.DiagnosticFileInvalidFormat, "--file")
			}
			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("review details %s: %w", name, err)
			}
			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()
			var versionErr error
			if selector := d.ResolveAppSelector(*appID); selector.Value != "" {
				requestCtx = asc.WithNoRedirectReads(requestCtx)
				selectedAppID, err := shared.ResolveScopedAppID(requestCtx, client, selector)
				if err != nil {
					return fmt.Errorf("review details %s: resolve app: %w", name, err)
				}
				_, versionErr = shared.ResolveOwnedAppStoreVersionByID(requestCtx, client, selectedAppID, id, "")
			} else {
				_, versionErr = client.GetAppStoreVersion(requestCtx, id)
			}
			if versionErr != nil {
				return fmt.Errorf("review details %s: read version: %w", name, versionErr)
			}
			target := metadataSettingTarget{versionID: id}
			remote, err := scope.fetch(requestCtx, d, client, target)
			if err != nil {
				return fmt.Errorf("review details %s: %w", name, err)
			}
			desired := map[string]string{}
			maps.Copy(desired, remote)
			maps.Copy(desired, local)
			if remote == nil {
				for _, field := range []string{"contactFirstName", "contactPhone", "demoAccountRequired"} {
					if strings.TrimSpace(desired[field]) == "" {
						return d.UsageErrorf("%s is required to create review details", field)
					}
				}
			}
			if err := scope.validate(desired, true); err != nil {
				return d.UsageError(err.Error())
			}
			plan, adds, updates := buildMetadataSettingPlan(scope, local, remote)
			changes := append(adds, updates...)
			sortPlanItems(changes)
			result := asc.ReviewDetailsResult{VersionID: id, Action: plan.action, Status: "planned", Changes: make([]asc.ReviewDetailsChange, 0, len(changes))}
			for _, change := range changes {
				result.Changes = append(result.Changes, asc.ReviewDetailsChange{Field: change.Field, From: change.From, To: change.To})
			}
			var applyErr error
			if len(plan.changed) == 0 {
				result.Action = "noop"
				result.Status = "unchanged"
			} else if apply {
				receipt, err := applyMetadataSettingPlan(requestCtx, d, client, target, plan)
				result.Action, result.Status, result.Error = receipt.Action, receipt.Status, receipt.Error
				applyErr = err
			}
			if err := d.PrintOutput(&result, *output.Output, *output.Pretty); err != nil {
				return err
			}
			if applyErr != nil {
				return shared.NewReportedError(applyErr)
			}
			return nil
		},
	}
}

func validateReviewInformation(values map[string]string, complete bool) error {
	if validation.ReviewNotesLength(values["notes"]) > validation.LimitReviewNotes {
		return fmt.Errorf("notes must be at most 4,000 characters")
	}
	if count, ok := validation.NANPNationalDigitCount(values["contactPhone"]); ok && count != validation.NANPNationalNumberDigits {
		return fmt.Errorf("contactPhone with +1 must have 10 digits after the country code")
	}
	password := values["demoAccountPassword"]
	if utf8.RuneCountInString(password) > 100 {
		return fmt.Errorf("demoAccountPassword must be at most 100 characters")
	}
	if err := asc.ValidateSecretMutationValue(&password); err != nil {
		return err
	}
	if complete && values["demoAccountRequired"] == "true" && (strings.TrimSpace(values["demoAccountName"]) == "" || strings.TrimSpace(password) == "") {
		return fmt.Errorf("demoAccountRequired=true requires demoAccountName and demoAccountPassword")
	}
	return nil
}
