package testflight

import (
	"context"
	"encoding/csv"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/mail"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"unicode"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

type betaTestersCSVRow struct {
	email     string
	firstName string
	lastName  string
	groups    []string // raw group values (names or IDs), trimmed
}

// newBetaTestersExportCommand writes beta testers to a CSV file.
func newBetaTestersExportCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("export", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)")
	outputPath := fs.String("output", "", "Output CSV file path (required)")
	group := shared.BindResourceIDFlag(fs, "group", "betaGroups", "Beta group name or ID to filter (optional)")
	buildID := shared.BindResourceIDFlag(fs, "build-id", "builds", "Build ID to filter (optional)")
	shared.BindFlagAlias(fs, "build", "build-id")
	email := fs.String("email", "", "Filter by tester email (optional)")
	firstName := fs.String("first-name", "", "Filter by tester first name (exact match, optional)")
	lastName := fs.String("last-name", "", "Filter by tester last name (exact match, optional)")
	includeGroups := fs.Bool("include-groups", false, "Include a groups column (requires additional API calls)")
	format := shared.BindOutputFlagsWith(fs, "format", "json", "Summary output format: json (default), table, markdown")

	return &ffcli.Command{
		Name:       "export",
		ShortUsage: "asc testflight beta-testers export --app \"APP_ID\" --output \"./testers.csv\" [flags]",
		ShortHelp:  "Export TestFlight beta testers to a CSV file.",
		LongHelp: `Export TestFlight beta testers to a CSV file.

CSV format:
  email,first_name,last_name,groups
  - groups are semicolon-delimited when present (for fastlane compatibility)
  - a cell whose first character would start a spreadsheet formula (=, +, -, @)
    is prefixed with a single quote so spreadsheet software treats it as text
  - when escaping is needed, an _asc_formula_escaping provenance column is
    included so "beta-testers import" only removes prefixes added by this export

Examples:
  asc testflight beta-testers export --app "APP_ID" --output "./testflight-testers.csv"
  asc testflight beta-testers export --app "APP_ID" --group "Beta" --output "./testers.csv"
  asc testflight beta-testers export --app "APP_ID" --build-id "BUILD_ID" --output "./testers.csv"
  asc testflight beta-testers export --app "APP_ID" --email "tester@example.com" --output "./testers.csv"
  asc testflight beta-testers export --app "APP_ID" --last-name "Lovelace" --output "./testers.csv"
  asc testflight beta-testers export --app "APP_ID" --output "./testers.csv" --include-groups`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := d.RejectOutputFormatAsPath(*outputPath, "--format"); err != nil {
				return err
			}
			if strings.TrimSpace(*group) != "" && strings.TrimSpace(*buildID) != "" {
				return shared.WithDiagnostic(
					d.UsageError("--group cannot be combined with --build-id"),
					shared.DiagnosticConflictingInput,
					"",
				)
			}
			resolvedAppID := d.ResolveAppID(*appID)
			if resolvedAppID == "" {
				fmt.Fprintf(d.IO.ErrOut, "Error: --app is required (or set ASC_APP_ID)\n\n")
				return shared.MissingRequiredUsageError("--app")
			}

			outputValue := strings.TrimSpace(*outputPath)
			if outputValue == "" {
				fmt.Fprintf(d.IO.ErrOut, "Error: --output is required\n\n")
				return shared.MissingRequiredUsageError("--output")
			}
			if strings.HasSuffix(outputValue, string(filepath.Separator)) {
				return shared.WithDiagnostic(
					d.UsageError("--output must be a file path"),
					shared.DiagnosticInvalidInput,
					"--output",
				)
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("beta-testers export: %w", err)
			}

			var groupResolver *betaGroupResolver
			if strings.TrimSpace(*group) != "" || *includeGroups {
				groupResolver, err = newBetaGroupResolver(ctx, d, client, resolvedAppID)
				if err != nil {
					return fmt.Errorf("beta-testers export: %w", err)
				}
			}

			opts := []asc.BetaTestersOption{asc.Limit[asc.BetaTestersOption](200)}
			if trimmed := strings.TrimSpace(*buildID); trimmed != "" {
				opts = append(opts, asc.WithBetaTestersBuildID(trimmed))
			}
			if trimmed := strings.TrimSpace(*email); trimmed != "" {
				opts = append(opts, asc.WithBetaTestersEmail(trimmed))
			}
			if trimmed := strings.TrimSpace(*firstName); trimmed != "" {
				opts = append(opts, asc.WithBetaTestersFirstName(trimmed))
			}
			if trimmed := strings.TrimSpace(*lastName); trimmed != "" {
				opts = append(opts, asc.WithBetaTestersLastName(trimmed))
			}
			if trimmed := strings.TrimSpace(*group); trimmed != "" {
				id, err := groupResolver.Resolve(trimmed)
				if err != nil {
					return fmt.Errorf("beta-testers export: %w", err)
				}
				opts = append(opts, asc.WithBetaTestersGroupIDs([]string{id}))
			}

			requestCtx, cancel := d.Timeout(ctx)
			firstPage, err := client.GetBetaTesters(requestCtx, resolvedAppID, opts...)
			cancel()
			if err != nil {
				return fmt.Errorf("beta-testers export: failed to fetch: %w", err)
			}

			all, err := asc.PaginateAll(ctx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
				requestCtx, cancel := d.Timeout(ctx)
				defer cancel()
				return client.GetBetaTesters(requestCtx, resolvedAppID, asc.NextURL[asc.BetaTestersOption](nextURL))
			})
			if err != nil {
				return fmt.Errorf("beta-testers export: %w", err)
			}

			testers, ok := all.(*asc.BetaTestersResponse)
			if !ok || testers == nil {
				return fmt.Errorf("beta-testers export: unexpected response type")
			}

			var groupMembership map[string][]string
			if *includeGroups {
				groupMembership, err = fetchTesterGroupMemberships(ctx, d, client, groupResolver)
				if err != nil {
					return fmt.Errorf("beta-testers export: %w", err)
				}
			}

			header := []string{"email", "first_name", "last_name"}
			if *includeGroups {
				header = append(header, "groups")
			}

			type sortable struct {
				id        string
				email     string
				firstName string
				lastName  string
				groups    []string
			}

			items := make([]sortable, 0, len(testers.Data))
			for _, item := range testers.Data {
				attrs := item.Attributes
				entry := sortable{
					id:        strings.TrimSpace(item.ID),
					email:     strings.TrimSpace(attrs.Email),
					firstName: strings.TrimSpace(attrs.FirstName),
					lastName:  strings.TrimSpace(attrs.LastName),
				}
				if *includeGroups {
					entry.groups = groupMembership[entry.id]
				}
				items = append(items, entry)
			}

			sort.Slice(items, func(i, j int) bool {
				li := strings.ToLower(items[i].email)
				lj := strings.ToLower(items[j].email)
				if li == lj {
					if items[i].email == items[j].email {
						return items[i].id < items[j].id
					}
					return items[i].email < items[j].email
				}
				return li < lj
			})

			rows := make([][]string, 0, len(items))
			for _, item := range items {
				row := []string{item.email, item.firstName, item.lastName}
				if *includeGroups {
					// Semicolon keeps CSV structure stable and matches fastlane/pilot interoperability.
					row = append(row, strings.Join(item.groups, ";"))
				}
				rows = append(rows, row)
			}

			if err := writeCSVFileAtomicNoSymlink(d.OSPath(outputValue), header, rows); err != nil {
				return fmt.Errorf("beta-testers export: %w", err)
			}

			summary := &asc.BetaTestersExportSummary{
				AppID:         resolvedAppID,
				OutputFile:    filepath.Clean(outputValue),
				Total:         len(rows),
				IncludeGroups: *includeGroups,
			}

			return d.PrintOutputWithRenderers(
				summary,
				*format.Output,
				*format.Pretty,
				func() error {
					asc.RenderTableTo(
						d.IO.Out,
						[]string{"App ID", "Output File", "Total", "Include Groups"},
						[][]string{{summary.AppID, summary.OutputFile, fmt.Sprintf("%d", summary.Total), fmt.Sprintf("%t", summary.IncludeGroups)}},
					)
					return nil
				},
				func() error {
					asc.RenderMarkdownTo(
						d.IO.Out,
						[]string{"App ID", "Output File", "Total", "Include Groups"},
						[][]string{{summary.AppID, summary.OutputFile, fmt.Sprintf("%d", summary.Total), fmt.Sprintf("%t", summary.IncludeGroups)}},
					)
					return nil
				},
			)
		},
	}
}

// newBetaTestersImportCommand reads testers from a CSV file and applies changes.
func newBetaTestersImportCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("import", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)")
	inputPath := fs.String("input", "", "Input CSV file path (required)")
	dryRun := fs.Bool("dry-run", false, "Validate and print plan without mutating network state")
	confirm := fs.Bool("confirm", false, "Confirm creating and updating beta testers (required unless --dry-run)")
	invite := fs.Bool("invite", false, "Invite newly created testers (default false)")
	group := shared.BindResourceIDFlag(fs, "group", "betaGroups", "Beta group name or ID to apply to all rows (optional)")
	skipExisting := fs.Bool("skip-existing", false, "If tester already exists, do not modify group membership")
	createMissingGroups := fs.Bool("create-missing-groups", false, "Create unresolved group names as external groups with public links disabled")
	continueOnError := fs.Bool("continue-on-error", true, "Continue processing rows after failures (default true)")
	format := shared.BindOutputFlagsWith(fs, "format", "json", "Summary output format: json (default), table, markdown")

	return &ffcli.Command{
		Name:       "import",
		ShortUsage: "asc testflight beta-testers import --app \"APP_ID\" --input \"./testers.csv\" [flags]",
		ShortHelp:  "Import TestFlight beta testers from a CSV file.",
		LongHelp: `Import TestFlight beta testers from a CSV file.

CSV formats accepted:
  1) Canonical header:
     email,first_name,last_name,groups
  2) fastlane/pilot header aliases:
     First,Last,Email,Groups
  3) Legacy headerless fastlane rows:
     First,Last,Email[,Groups]

Installed Version and Install Date columns are accepted and ignored.
Groups are semicolon-delimited in canonical import/export files.
For compatibility, comma-delimited groups are also accepted when no semicolon is present.
Exports that neutralize spreadsheet formulas include an _asc_formula_escaping
provenance column. Only rows carrying the supported marker have the export-added
single quote removed; user-authored CSV apostrophes are preserved.

All rows and group references are checked before any mutation. Existing testers
are added only to groups they do not already belong to. --continue-on-error
controls provider failures after validation, not invalid CSV input.

--create-missing-groups treats unresolved group selectors as names and creates
external groups with public links disabled. --dry-run includes those groups in
the plan without creating them. Names and emails identify existing groups and
testers; repeating the import sends no writes when membership already matches.
Testers are matched by input email within the selected app, including defaults.
A create conflict is recovered only when the tester is found in that same app.
First and last names apply only to newly created testers. A one-row CSV uses
the same behavior. Invitations remain opt-in.

Examples:
  asc testflight beta-testers import --app "APP_ID" --input "./testers.csv" --create-missing-groups --dry-run
  asc testflight beta-testers import --app "APP_ID" --input "./testers.csv" --create-missing-groups --confirm
  asc testflight beta-testers import --app "APP_ID" --input "./testflight-testers.csv" --dry-run
  asc testflight beta-testers import --app "APP_ID" --input "./testflight-testers.csv" --confirm
  asc testflight beta-testers import --app "APP_ID" --input "./testflight-testers.csv" --invite --confirm
  asc testflight beta-testers import --app "APP_ID" --input "./testflight-testers.csv" --group "Beta" --confirm`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			resolvedAppID := d.ResolveAppID(*appID)
			if resolvedAppID == "" {
				fmt.Fprintf(d.IO.ErrOut, "Error: --app is required (or set ASC_APP_ID)\n\n")
				return shared.MissingRequiredUsageError("--app")
			}

			inputValue := strings.TrimSpace(*inputPath)
			if inputValue == "" {
				fmt.Fprintf(d.IO.ErrOut, "Error: --input is required\n\n")
				return shared.MissingRequiredUsageError("--input")
			}

			if err := d.RequireConfirmUnlessDryRun(*dryRun, *confirm); err != nil {
				return err
			}

			if _, err := shared.ValidateOutputFormat(*format.Output, *format.Pretty); err != nil {
				return d.UsageError(err.Error())
			}
			parsedRows, err := readBetaTestersCSV(d, inputValue)
			if err != nil {
				return fmt.Errorf("beta-testers import: %w", err)
			}
			if err := validateBetaTesterImportRows(d, parsedRows); err != nil {
				return err
			}
			ctx = asc.WithNoRedirectReads(ctx)
			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("beta-testers import: %w", err)
			}

			appliedGroupValue := strings.TrimSpace(*group)
			needsGroups := appliedGroupValue != ""
			if !needsGroups {
				for _, r := range parsedRows {
					if len(r.groups) > 0 {
						needsGroups = true
						break
					}
				}
			}

			var groupResolver *betaGroupResolver
			if needsGroups {
				groupResolver, err = newBetaGroupResolver(ctx, d, client, resolvedAppID)
				if err != nil {
					return fmt.Errorf("beta-testers import: %w", err)
				}
			}
			var missingGroups []string
			if needsGroups {
				missingGroups, err = planBetaTesterImportGroups(groupResolver, parsedRows, []string{appliedGroupValue}, *createMissingGroups)
				if err != nil {
					return fmt.Errorf("beta-testers import: %w", err)
				}
			}

			existingByEmail, err := fetchExistingTestersByEmail(ctx, d, client, []string{resolvedAppID}, parsedRows)
			if err != nil {
				return fmt.Errorf("beta-testers import: %w", err)
			}

			summary := &asc.BetaTestersImportSummary{
				AppID:           resolvedAppID,
				InputFile:       filepath.Clean(inputValue),
				DryRun:          *dryRun,
				Invite:          *invite,
				SkipExisting:    *skipExisting,
				ContinueOnError: *continueOnError,
				AppliedGroup:    appliedGroupValue,
				Total:           len(parsedRows),
			}

			var refused error
			for _, name := range missingGroups {
				item := asc.BetaTestersImportGroup{Name: name, Status: "planned"}
				id := "planned:" + name
				if !*dryRun {
					id, err = ensureBetaTesterImportGroup(ctx, d, client, resolvedAppID, name)
					if err != nil {
						item.Status, item.Error = "failed", err.Error()
						summary.Groups = append(summary.Groups, item)
						if *continueOnError {
							refused = shared.KeepReadOnlyRefusal(refused, err)
							continue
						}
						if printErr := d.PrintOutput(summary, *format.Output, *format.Pretty); printErr != nil {
							return printErr
						}
						return shared.NewReportedError(fmt.Errorf("beta-testers import: create group %q: %w", name, err))
					}
					item.ID, item.Status = id, "succeeded"
				}
				summary.Groups = append(summary.Groups, item)
				groupResolver.addImportGroup(name, id)
			}
			var appliedGroupID string
			var appliedGroupErr error
			if appliedGroupValue != "" {
				appliedGroupID, appliedGroupErr = groupResolver.Resolve(appliedGroupValue)
			}

			for idx, row := range parsedRows {
				rowNumber := idx + 1 // 1-based data row index (excluding header)

				emailValue := strings.TrimSpace(row.email)
				emailLower := strings.ToLower(emailValue)

				var groupIDs []string
				if needsGroups {
					groupErr := appliedGroupErr
					if groupErr == nil {
						groupIDs, groupErr = groupResolver.ResolveAll(row.groups)
					}
					if groupErr != nil {
						summary.Failed++
						summary.Failures = append(summary.Failures, asc.BetaTestersImportFailure{
							Row:   rowNumber,
							Email: emailValue,
							Error: groupErr.Error(),
						})
						if !*continueOnError {
							break
						}
						continue
					}
				}
				if appliedGroupID != "" {
					groupIDs = append(groupIDs, appliedGroupID)
					groupIDs = uniqueSortedStrings(groupIDs)
				}

				if tester, ok := existingByEmail[emailLower]; ok {
					testerID := tester.Data.ID
					summary.Existed++

					if *skipExisting || len(groupIDs) == 0 {
						continue
					}

					groupIDs, err = missingBetaTesterLinkages(ctx, d, client, groupIDs, "/v1/betaTesters/"+url.PathEscape(testerID)+"/relationships/betaGroups", func(requestCtx context.Context, opts ...asc.LinkagesOption) (*asc.LinkagesResponse, error) {
						return client.GetBetaTesterBetaGroupsRelationships(requestCtx, testerID, opts...)
					})
					if err != nil {
						summary.Failed++
						summary.Failures = append(summary.Failures, asc.BetaTestersImportFailure{Row: rowNumber, Email: emailValue, Error: err.Error()})
						if !*continueOnError {
							break
						}
						continue
					}
					if len(groupIDs) == 0 {
						continue
					}

					if *dryRun {
						summary.Updated++
						continue
					}

					requestCtx, cancel := d.Timeout(ctx)
					err := client.AddBetaTesterToGroups(requestCtx, testerID, groupIDs)
					cancel()
					if err != nil {
						alreadySatisfied, verificationErr := betaTesterGroupConflictAlreadySatisfied(ctx, d, client, testerID, groupIDs, err)
						if verificationErr != nil {
							err = fmt.Errorf("%w (failed to verify beta group membership: %w)", err, verificationErr)
						} else if alreadySatisfied {
							summary.Updated++
							continue
						}
						summary.Failed++
						summary.Failures = append(summary.Failures, asc.BetaTestersImportFailure{
							Row:   rowNumber,
							Email: emailValue,
							Error: err.Error(),
						})
						refused = shared.KeepReadOnlyRefusal(refused, err)
						if !*continueOnError {
							break
						}
						continue
					}
					summary.Updated++
					continue
				}

				if *dryRun {
					summary.Created++
					continue
				}

				requestCtx, cancel := d.Timeout(ctx)
				created, err := client.CreateBetaTester(requestCtx, emailValue, row.firstName, row.lastName, groupIDs, nil)
				cancel()
				if isHTTPConflict(err) {
					recoveryGroupIDs := groupIDs
					if *skipExisting {
						recoveryGroupIDs = nil
					}
					outcome, recoveryErr := resolveBetaTesterAddConflict(ctx, d, client, []string{resolvedAppID}, emailValue, recoveryGroupIDs, nil)
					if recoveryErr != nil {
						err = fmt.Errorf("%w (failed to verify tester in selected app: %w)", err, recoveryErr)
					} else if outcome != nil && outcome.tester != nil {
						existingByEmail[emailLower] = outcome.tester
						summary.Existed++
						if len(outcome.addedGroupIDs) > 0 {
							summary.Updated++
						}
						continue
					}
				}
				if err != nil {
					summary.Failed++
					summary.Failures = append(summary.Failures, asc.BetaTestersImportFailure{
						Row:   rowNumber,
						Email: emailValue,
						Error: err.Error(),
					})
					refused = shared.KeepReadOnlyRefusal(refused, err)
					if !*continueOnError {
						break
					}
					continue
				}

				testerID := strings.TrimSpace(created.Data.ID)
				if testerID == "" {
					summary.Failed++
					summary.Failures = append(summary.Failures, asc.BetaTestersImportFailure{
						Row:   rowNumber,
						Email: emailValue,
						Error: "created tester returned empty id",
					})
					if !*continueOnError {
						break
					}
					continue
				}
				summary.Created++
				existingByEmail[emailLower] = created

				if *invite {
					requestCtx, cancel := d.Timeout(ctx)
					invitation, err := client.CreateBetaTesterInvitation(requestCtx, resolvedAppID, testerID)
					cancel()
					if err != nil {
						summary.Failed++
						summary.Failures = append(summary.Failures, asc.BetaTestersImportFailure{
							Row:   rowNumber,
							Email: emailValue,
							Error: err.Error(),
						})
						refused = shared.KeepReadOnlyRefusal(refused, err)
						if !*continueOnError {
							break
						}
						continue
					}
					if invitation == nil || strings.TrimSpace(invitation.Data.ID) == "" {
						summary.Failed++
						summary.Failures = append(summary.Failures, asc.BetaTestersImportFailure{
							Row:   rowNumber,
							Email: emailValue,
							Error: "invitation returned empty id",
						})
						if !*continueOnError {
							break
						}
						continue
					}
					summary.Invited++
				}
			}

			// Always print a machine-readable summary. If any rows failed, return an error
			// that won't be re-printed by the main entrypoint.
			if err := d.PrintOutput(summary, *format.Output, *format.Pretty); err != nil {
				return err
			}

			if summary.Failed > 0 {
				return shared.NewReportedError(shared.NewErrorWithCause(fmt.Errorf("beta-testers import: %d row(s) failed", summary.Failed), refused))
			}
			return nil
		},
	}
}

type betaGroupResolver struct {
	groups         *asc.BetaGroupsResponse
	internalByID   map[string]bool
	byID           map[string]string   // id -> name
	idsByName      map[string][]string // lower(name) -> ids (sorted)
	uniqueNameByID map[string]bool     // id -> whether its name is unique among groups
	sortedGroupIDs []string            // stable iteration order (sorted by id)
}

func newBetaGroupResolver(ctx context.Context, d *shared.Deps, client *asc.Client, appID string) (*betaGroupResolver, error) {
	if client == nil {
		return nil, fmt.Errorf("client is required")
	}
	appID = strings.TrimSpace(appID)
	if appID == "" {
		return nil, fmt.Errorf("app ID is required")
	}

	requestCtx, cancel := d.Timeout(ctx)
	firstPage, err := client.GetBetaGroups(requestCtx, appID, asc.Limit[asc.BetaGroupsOption](200))
	cancel()
	if err != nil {
		return nil, err
	}
	all, err := asc.PaginateAll(ctx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
		requestCtx, cancel := d.Timeout(ctx)
		defer cancel()
		canonical, err := client.ValidateNoRedirectReadContinuation(ctx, nextURL, "/v1/apps/"+url.PathEscape(appID)+"/betaGroups")
		if err != nil {
			return nil, err
		}
		return client.GetBetaGroups(requestCtx, appID, asc.NextURL[asc.BetaGroupsOption](canonical))
	})
	if err != nil {
		return nil, err
	}
	resp, ok := all.(*asc.BetaGroupsResponse)
	if !ok || resp == nil {
		return nil, fmt.Errorf("unexpected beta groups response type")
	}

	r := &betaGroupResolver{
		groups:         resp,
		internalByID:   make(map[string]bool),
		byID:           make(map[string]string, len(resp.Data)),
		idsByName:      make(map[string][]string),
		uniqueNameByID: make(map[string]bool, len(resp.Data)),
		sortedGroupIDs: make([]string, 0, len(resp.Data)),
	}

	for _, g := range resp.Data {
		id := strings.TrimSpace(g.ID)
		if id == "" {
			continue
		}
		name := strings.TrimSpace(g.Attributes.Name)
		r.byID[id] = name
		r.internalByID[id] = g.Attributes.IsInternalGroup
		r.sortedGroupIDs = append(r.sortedGroupIDs, id)
		r.idsByName[strings.ToLower(name)] = append(r.idsByName[strings.ToLower(name)], id)
	}

	for key := range r.idsByName {
		ids := r.idsByName[key]
		sort.Strings(ids)
		r.idsByName[key] = ids
		for _, id := range ids {
			r.uniqueNameByID[id] = len(ids) == 1
		}
	}
	sort.Strings(r.sortedGroupIDs)

	return r, nil
}

func (r *betaGroupResolver) Resolve(value string) (string, error) {
	if r == nil {
		return "", fmt.Errorf("group resolver is nil")
	}
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", fmt.Errorf("beta group name is required")
	}
	return matchBetaGroupID(r.groups, trimmed)
}

func (r *betaGroupResolver) ResolveAll(values []string) ([]string, error) {
	if r == nil {
		return nil, fmt.Errorf("group resolver is nil")
	}
	if len(values) == 0 {
		return nil, nil
	}
	out := make([]string, 0, len(values))
	for _, v := range values {
		trimmed := strings.TrimSpace(v)
		if trimmed == "" {
			continue
		}
		id, err := r.Resolve(trimmed)
		if err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return uniqueSortedStrings(out), nil
}

func (r *betaGroupResolver) exportValueForID(groupID string) string {
	groupID = strings.TrimSpace(groupID)
	if groupID == "" {
		return ""
	}
	name := strings.TrimSpace(r.byID[groupID])
	if name == "" || !r.uniqueNameByID[groupID] {
		return groupID
	}
	return name
}

func fetchExistingTestersByEmail(ctx context.Context, d *shared.Deps, client *asc.Client, appIDs []string, rows []betaTestersCSVRow, fields ...string) (map[string]*asc.BetaTesterResponse, error) {
	if len(appIDs) == 0 {
		return nil, fmt.Errorf("tester lookup requires a resolved app")
	}
	for _, appID := range appIDs {
		if strings.TrimSpace(appID) == "" {
			return nil, fmt.Errorf("tester lookup requires a resolved app")
		}
	}
	appIDs = uniqueSortedStrings(appIDs)
	if len(fields) == 0 {
		fields = []string{"email", "firstName", "lastName", "inviteType", "state"}
	}
	emails := make([]string, 0, len(rows))
	for _, row := range rows {
		emails = append(emails, strings.ToLower(strings.TrimSpace(row.email)))
	}
	sort.Strings(emails)
	byEmail := make(map[string]*asc.BetaTesterResponse, len(rows))
	for _, appID := range appIDs {
		for start := 0; start < len(emails); start += 20 {
			batch := emails[start:min(start+20, len(emails))]
			emailFilter := strings.Join(batch, ",")
			requestCtx, cancel := d.Timeout(ctx)
			first, err := client.GetBetaTesters(requestCtx, appID, asc.WithBetaTestersEmail(emailFilter), asc.WithBetaTestersFields(fields), asc.Limit[asc.BetaTestersOption](200))
			cancel()
			if err != nil {
				return nil, err
			}
			all, err := asc.PaginateAll(ctx, first, func(ctx context.Context, next string) (asc.PaginatedResponse, error) {
				canonical, err := client.ValidateNoRedirectReadContinuationQuery(ctx, next, "/v1/betaTesters", url.Values{
					"filter[apps]": {appID}, "filter[email]": {emailFilter},
				})
				if err != nil {
					return nil, err
				}
				parsed, err := url.Parse(canonical)
				if err != nil {
					return nil, err
				}
				if actual, present := parsed.Query()["fields[betaTesters]"]; present && (len(actual) != 1 || !slices.Equal(uniqueSortedStrings(strings.Split(actual[0], ",")), uniqueSortedStrings(fields))) {
					return nil, fmt.Errorf("tester pagination changed the requested fields")
				}
				requestCtx, cancel := d.Timeout(ctx)
				defer cancel()
				return client.GetBetaTesters(requestCtx, appID, asc.NextURL[asc.BetaTestersOption](canonical))
			})
			if err != nil {
				return nil, err
			}
			resp, ok := all.(*asc.BetaTestersResponse)
			if !ok || resp == nil {
				return nil, fmt.Errorf("unexpected beta testers response type")
			}
			for _, tester := range resp.Data {
				email := strings.ToLower(strings.TrimSpace(tester.Attributes.Email))
				if !slices.Contains(batch, email) {
					continue
				}
				id := strings.TrimSpace(tester.ID)
				if id == "" {
					return nil, fmt.Errorf("tester %q returned an empty ID", email)
				}
				if previous, ok := byEmail[email]; ok && previous.Data.ID != id {
					return nil, fmt.Errorf("multiple testers have email %q in the selected apps; resolve the ambiguity before continuing", email)
				}
				byEmail[email] = &asc.BetaTesterResponse{Data: tester}
			}
		}
	}
	return byEmail, nil
}

func betaTesterGroupConflictAlreadySatisfied(
	ctx context.Context,
	d *shared.Deps,
	client *asc.Client,
	testerID string,
	groupIDs []string,
	requestErr error,
) (bool, error) {
	if !isHTTPConflict(requestErr) {
		return false, nil
	}

	requestedGroupIDs := uniqueSortedStrings(groupIDs)
	if len(requestedGroupIDs) == 0 {
		return false, nil
	}

	missing, err := missingBetaTesterLinkages(ctx, d, client, requestedGroupIDs, "/v1/betaTesters/"+url.PathEscape(testerID)+"/relationships/betaGroups", func(requestCtx context.Context, opts ...asc.LinkagesOption) (*asc.LinkagesResponse, error) {
		return client.GetBetaTesterBetaGroupsRelationships(requestCtx, testerID, opts...)
	})
	return err == nil && len(missing) == 0, err
}

func fetchTesterGroupMemberships(ctx context.Context, d *shared.Deps, client *asc.Client, resolver *betaGroupResolver) (map[string][]string, error) {
	if client == nil {
		return nil, fmt.Errorf("client is required")
	}
	if resolver == nil {
		return nil, fmt.Errorf("group resolver is required")
	}

	groupTesters := make([]*asc.BetaTestersResponse, len(resolver.sortedGroupIDs))
	tasks := make([]func(context.Context) error, len(resolver.sortedGroupIDs))
	for i, groupID := range resolver.sortedGroupIDs {
		tasks[i] = func(ctx context.Context) error {
			requestCtx, cancel := d.Timeout(ctx)
			firstPage, err := client.GetBetaGroupTesters(requestCtx, groupID, asc.Limit[asc.BetaGroupTestersOption](200))
			cancel()
			if err != nil {
				return err
			}
			all, err := asc.PaginateAll(ctx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
				requestCtx, cancel := d.Timeout(ctx)
				defer cancel()
				return client.GetBetaGroupTesters(requestCtx, groupID, asc.NextURL[asc.BetaGroupTestersOption](nextURL))
			})
			if err != nil {
				return err
			}
			testersResp, ok := all.(*asc.BetaTestersResponse)
			if !ok || testersResp == nil {
				return fmt.Errorf("unexpected beta group testers response type")
			}
			groupTesters[i] = testersResp
			return nil
		}
	}
	if _, err := runTestFlightGroupFetches(ctx, tasks); err != nil {
		return nil, err
	}

	membership := make(map[string][]string)
	for i, groupID := range resolver.sortedGroupIDs {
		exportValue := resolver.exportValueForID(groupID)
		for _, tester := range groupTesters[i].Data {
			id := strings.TrimSpace(tester.ID)
			if id == "" {
				continue
			}
			membership[id] = append(membership[id], exportValue)
		}
	}

	for testerID := range membership {
		membership[testerID] = uniqueSortedStrings(membership[testerID])
	}

	return membership, nil
}

func readBetaTestersCSV(d *shared.Deps, path string) ([]betaTestersCSVRow, error) {
	file, err := shared.OpenExistingNoFollow(d.OSPath(path))
	if err != nil {
		return nil, err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.FieldsPerRecord = -1
	reader.LazyQuotes = true

	header, err := reader.Read()
	if err != nil {
		if errors.Is(err, io.EOF) {
			return nil, shared.WithDiagnostic(
				d.UsageError("CSV file is empty"),
				shared.DiagnosticFileInvalidFormat,
				"--input",
			)
		}
		return nil, fmt.Errorf("read header: %w", err)
	}

	rows := make([]betaTestersCSVRow, 0)
	headerIdx, hasHeader, err := parseBetaTestersCSVHeader(d, header)
	if err != nil {
		return nil, err
	}
	if !hasHeader {
		row, rowErr := parseLegacyBetaTesterCSVRow(d, header)
		if rowErr != nil {
			return nil, rowErr
		}
		rows = append(rows, row)
	} else {
		ignored := make([]string, 0, 2)
		for _, raw := range header {
			if canonical, _ := canonicalBetaTestersCSVColumn(raw); canonical == "installed_version" || canonical == "install_date" {
				ignored = append(ignored, strings.TrimSpace(raw))
			}
		}
		if len(ignored) > 0 {
			fmt.Fprintf(d.IO.ErrOut, "Note: ignoring CSV columns: %s\n", strings.Join(ignored, ", "))
		}
	}

	for {
		record, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read csv: %w", err)
		}
		if record == nil || isAllEmpty(record) {
			continue
		}

		if !hasHeader {
			row, rowErr := parseLegacyBetaTesterCSVRow(d, record)
			if rowErr != nil {
				return nil, rowErr
			}
			rows = append(rows, row)
			continue
		}

		rows = append(rows, parseHeaderMappedBetaTesterCSVRow(record, headerIdx))
	}

	return rows, nil
}

func isAllEmpty(record []string) bool {
	for _, v := range record {
		if strings.TrimSpace(v) != "" {
			return false
		}
	}
	return true
}

func validateBetaTestersCSVHeader(d *shared.Deps, header []string) (map[string]int, error) {
	if len(header) == 0 {
		return nil, shared.WithDiagnostic(
			d.UsageError("CSV header row is required"),
			shared.DiagnosticFileInvalidFormat,
			"--input",
		)
	}

	idx := make(map[string]int, len(header))
	for i, raw := range header {
		col := strings.ToLower(strings.TrimSpace(raw))
		if col == "" {
			return nil, shared.WithDiagnostic(
				d.UsageError("CSV header contains an empty column name"),
				shared.DiagnosticFileInvalidFormat,
				"--input",
			)
		}
		canonical, ok := canonicalBetaTestersCSVColumn(col)
		if !ok {
			return nil, shared.WithDiagnostic(
				d.UsageErrorf("unknown CSV column %q (allowed: email, first_name, last_name, groups, %s; installed_version and install_date are ignored)", col, betaTestersFormulaEscapingColumn),
				shared.DiagnosticFileInvalidFormat,
				"--input",
			)
		}
		col = canonical
		if _, exists := idx[col]; exists {
			return nil, shared.WithDiagnostic(
				d.UsageErrorf("duplicate CSV column %q", col),
				shared.DiagnosticFileInvalidFormat,
				"--input",
			)
		}
		idx[col] = i
	}
	if _, ok := idx["email"]; !ok {
		return nil, shared.WithDiagnostic(
			d.UsageError("CSV header must include required column \"email\""),
			shared.DiagnosticFileInvalidFormat,
			"--input",
		)
	}
	return idx, nil
}

func parseBetaTestersCSVHeader(d *shared.Deps, firstRow []string) (map[string]int, bool, error) {
	if len(firstRow) == 0 {
		return nil, false, shared.WithDiagnostic(
			d.UsageError("CSV header row is required"),
			shared.DiagnosticFileInvalidFormat,
			"--input",
		)
	}
	hasEmailToken := false
	hasAtSignValue := false
	unknown := false
	for _, raw := range firstRow {
		trimmed := strings.TrimSpace(raw)
		col := strings.ToLower(trimmed)
		if col == "" {
			continue
		}
		if strings.Contains(trimmed, "@") {
			hasAtSignValue = true
		}
		if _, ok := canonicalBetaTestersCSVColumn(col); ok {
			if col == "email" {
				hasEmailToken = true
			}
			continue
		}
		unknown = true
	}
	// Headerless legacy rows commonly contain data values (including @ in emails).
	// Treat the first row as a header only when it explicitly contains "email"
	// and does not look like data.
	if !hasEmailToken || hasAtSignValue {
		return nil, false, nil
	}
	if unknown {
		_, err := validateBetaTestersCSVHeader(d, firstRow)
		return nil, false, err
	}
	headerIdx, err := validateBetaTestersCSVHeader(d, firstRow)
	if err != nil {
		return nil, false, err
	}
	return headerIdx, true, nil
}

func canonicalBetaTestersCSVColumn(col string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(col)) {
	case "email":
		return "email", true
	case "first_name", "first":
		return "first_name", true
	case "last_name", "last":
		return "last_name", true
	case "groups":
		return "groups", true
	case betaTestersFormulaEscapingColumn:
		return betaTestersFormulaEscapingColumn, true
	case "installed_version", "installed version":
		return "installed_version", true
	case "install_date", "install date":
		return "install_date", true
	default:
		return "", false
	}
}

func parseHeaderMappedBetaTesterCSVRow(record []string, headerIdx map[string]int) betaTestersCSVRow {
	formulaEscaped := false
	if idx, ok := headerIdx[betaTestersFormulaEscapingColumn]; ok && idx >= 0 && idx < len(record) {
		formulaEscaped = strings.TrimSpace(record[idx]) == betaTestersFormulaEscapingVersion
	}
	get := func(col string) string {
		i, ok := headerIdx[col]
		if !ok || i < 0 || i >= len(record) {
			return ""
		}
		value := record[i]
		if formulaEscaped {
			value = denormalizeSpreadsheetFormula(value)
		}
		return strings.TrimSpace(value)
	}
	groups := make([]string, 0)
	if idx, ok := headerIdx["groups"]; ok && idx >= 0 && idx < len(record) {
		groups = splitBetaTesterCSVGroups(record[idx], formulaEscaped)
	}
	return betaTestersCSVRow{
		email:     get("email"),
		firstName: get("first_name"),
		lastName:  get("last_name"),
		groups:    groups,
	}
}

func parseLegacyBetaTesterCSVRow(d *shared.Deps, record []string) (betaTestersCSVRow, error) {
	if len(record) < 3 || len(record) > 4 {
		return betaTestersCSVRow{}, shared.WithDiagnostic(
			d.UsageError("legacy CSV rows must have 3 or 4 columns: first_name,last_name,email[,groups]"),
			shared.DiagnosticFileInvalidFormat,
			"--input",
		)
	}
	row := betaTestersCSVRow{
		firstName: strings.TrimSpace(record[0]),
		lastName:  strings.TrimSpace(record[1]),
		email:     strings.TrimSpace(record[2]),
	}
	if len(record) >= 4 {
		row.groups = splitBetaTesterCSVGroups(record[3], false)
	}
	return row, nil
}

func splitBetaTesterCSVGroups(value string, formulaEscaped bool) []string {
	if formulaEscaped {
		value = denormalizeSpreadsheetFormula(value)
	}
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	splitOn := ","
	if strings.Contains(trimmed, ";") {
		// Prefer semicolon when present to preserve commas inside group names.
		splitOn = ";"
	}
	parts := strings.Split(trimmed, splitOn)
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		item := strings.TrimSpace(part)
		if item == "" {
			continue
		}
		out = append(out, item)
	}
	return out
}

func isValidTesterEmail(value string) bool {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" || strings.ContainsAny(trimmed, " \t\r\n") {
		return false
	}
	addr, err := mail.ParseAddress(trimmed)
	if err != nil {
		return false
	}
	return strings.EqualFold(addr.Address, trimmed)
}

// spreadsheetFormulaMarkers are the characters that Excel, LibreOffice Calc, and
// Google Sheets treat as the start of a formula when they open a CSV file.
const spreadsheetFormulaMarkers = "=+-@"

// spreadsheetTextPrefix neutralizes a formula-leading cell. Spreadsheet software
// reads a leading apostrophe as "the rest of this cell is text" and does not
// display it. Exported rows carry a provenance marker so import can strip only
// prefixes written by asc and leave user-authored apostrophes unchanged.
const spreadsheetTextPrefix = "'"

const (
	betaTestersFormulaEscapingColumn  = "_asc_formula_escaping"
	betaTestersFormulaEscapingVersion = "apostrophe-v1"
)

func neutralizeSpreadsheetFormulaRow(row []string) []string {
	safe := make([]string, len(row))
	for i, cell := range row {
		safe[i] = neutralizeSpreadsheetFormula(cell)
	}
	return safe
}

func rowNeedsSpreadsheetFormulaEscaping(row []string) bool {
	for _, cell := range row {
		if neutralizeSpreadsheetFormula(cell) != cell {
			return true
		}
	}
	return false
}

// neutralizeSpreadsheetFormula prefixes externally derived cells whose first
// effective character would start a spreadsheet formula. Leading whitespace and
// control characters are skipped when looking for that character, because
// spreadsheet software ignores them too. A cell that already begins with
// literal apostrophes in front of a formula character gains one more; the
// provenance-marked import path can then remove exactly one and recover the
// original value. Safe cells are returned unchanged.
func neutralizeSpreadsheetFormula(value string) string {
	if !isSpreadsheetFormulaCell(strings.TrimLeft(value, spreadsheetTextPrefix)) {
		return value
	}
	return spreadsheetTextPrefix + value
}

// denormalizeSpreadsheetFormula reverses neutralizeSpreadsheetFormula for rows
// carrying the asc export provenance marker. Exactly one prefix apostrophe is
// removed, which also restores values whose originals begin with apostrophes.
func denormalizeSpreadsheetFormula(value string) string {
	if !strings.HasPrefix(value, spreadsheetTextPrefix) {
		return value
	}
	if !isSpreadsheetFormulaCell(strings.TrimLeft(value, spreadsheetTextPrefix)) {
		return value
	}
	return strings.TrimPrefix(value, spreadsheetTextPrefix)
}

func isSpreadsheetFormulaCell(value string) bool {
	for _, r := range value {
		if unicode.IsSpace(r) || unicode.IsControl(r) {
			continue
		}
		return strings.ContainsRune(spreadsheetFormulaMarkers, r)
	}
	return false
}

func writeCSVFileAtomicNoSymlink(outputPath string, header []string, rows [][]string) error {
	trimmed := strings.TrimSpace(outputPath)
	if trimmed == "" {
		return fmt.Errorf("output path is required")
	}

	// Best-effort: refuse a symlink path and refuse overwriting.
	if info, err := os.Lstat(trimmed); err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("refusing to follow symlink %q", trimmed)
		}
		return fmt.Errorf("output file already exists: %w", os.ErrExist)
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}

	dir := filepath.Dir(trimmed)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	tempFile, err := os.CreateTemp(dir, ".asc-beta-testers-*.csv")
	if err != nil {
		return err
	}
	tempName := tempFile.Name()
	committed := false
	defer func() {
		if tempFile != nil {
			_ = tempFile.Close()
		}
		if !committed {
			_ = os.Remove(tempName)
		}
	}()

	if err := tempFile.Chmod(0o600); err != nil {
		return err
	}

	w := csv.NewWriter(tempFile)
	formulaEscaping := false
	for _, row := range rows {
		if rowNeedsSpreadsheetFormulaEscaping(row) {
			formulaEscaping = true
			break
		}
	}

	csvHeader := append([]string(nil), header...)
	if formulaEscaping {
		csvHeader = append(csvHeader, betaTestersFormulaEscapingColumn)
	}
	if err := w.Write(csvHeader); err != nil {
		return err
	}
	for _, row := range rows {
		csvRow := row
		if formulaEscaping {
			csvRow = append(neutralizeSpreadsheetFormulaRow(row), betaTestersFormulaEscapingVersion)
		}
		if err := w.Write(csvRow); err != nil {
			return err
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return err
	}

	if err := tempFile.Sync(); err != nil {
		return err
	}
	if err := tempFile.Close(); err != nil {
		return err
	}
	tempFile = nil

	if err := os.Rename(tempName, trimmed); err != nil {
		return err
	}
	committed = true
	return nil
}
