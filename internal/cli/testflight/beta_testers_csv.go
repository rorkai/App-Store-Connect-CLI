package testflight

import (
	"context"
	"encoding/csv"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/mail"
	"os"
	"path/filepath"
	"sort"
	"strings"

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

type betaTestersExportSummary struct {
	AppID         string `json:"appId"`
	OutputFile    string `json:"outputFile"`
	Total         int    `json:"total"`
	IncludeGroups bool   `json:"includeGroups"`
}

type betaTestersImportFailure struct {
	Row   int    `json:"row"`
	Email string `json:"email,omitempty"`
	Error string `json:"error"`
}

type betaTestersImportSummary struct {
	AppID           string                     `json:"appId"`
	InputFile       string                     `json:"inputFile"`
	DryRun          bool                       `json:"dryRun"`
	Invite          bool                       `json:"invite"`
	SkipExisting    bool                       `json:"skipExisting"`
	ContinueOnError bool                       `json:"continueOnError"`
	AppliedGroup    string                     `json:"appliedGroup,omitempty"`
	Total           int                        `json:"total"`
	Created         int                        `json:"created"`
	Existed         int                        `json:"existed"`
	Updated         int                        `json:"updated"`
	Invited         int                        `json:"invited"`
	Failed          int                        `json:"failed"`
	Failures        []betaTestersImportFailure `json:"failures,omitempty"`
}

// BetaTestersExportCommand writes beta testers to a CSV file.
func BetaTestersExportCommand() *ffcli.Command {
	fs := flag.NewFlagSet("export", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)")
	outputPath := fs.String("output", "", "Output CSV file path (required)")
	group := fs.String("group", "", "Beta group name or ID to filter (optional)")
	buildID := fs.String("build", "", "Build ID to filter (optional)")
	email := fs.String("email", "", "Filter by tester email (optional)")
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

Examples:
  asc testflight beta-testers export --app "APP_ID" --output "./testflight-testers.csv"
  asc testflight beta-testers export --app "APP_ID" --group "Beta" --output "./testers.csv"
  asc testflight beta-testers export --app "APP_ID" --build "BUILD_ID" --output "./testers.csv"
  asc testflight beta-testers export --app "APP_ID" --email "tester@example.com" --output "./testers.csv"
  asc testflight beta-testers export --app "APP_ID" --output "./testers.csv" --include-groups`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			resolvedAppID := shared.ResolveAppID(*appID)
			if resolvedAppID == "" {
				fmt.Fprintf(os.Stderr, "Error: --app is required (or set ASC_APP_ID)\n\n")
				return flag.ErrHelp
			}

			outputValue := strings.TrimSpace(*outputPath)
			if outputValue == "" {
				fmt.Fprintf(os.Stderr, "Error: --output is required\n\n")
				return flag.ErrHelp
			}
			if strings.HasSuffix(outputValue, string(filepath.Separator)) {
				return shared.UsageError("--output must be a file path")
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("beta-testers export: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			var groupResolver *betaGroupResolver
			if strings.TrimSpace(*group) != "" || *includeGroups {
				groupResolver, err = newBetaGroupResolver(requestCtx, client, resolvedAppID)
				if err != nil {
					return fmt.Errorf("beta-testers export: %w", err)
				}
			}

			opts := []asc.BetaTestersOption{asc.WithBetaTestersLimit(200)}
			if trimmed := strings.TrimSpace(*buildID); trimmed != "" {
				opts = append(opts, asc.WithBetaTestersBuildID(trimmed))
			}
			if trimmed := strings.TrimSpace(*email); trimmed != "" {
				opts = append(opts, asc.WithBetaTestersEmail(trimmed))
			}
			if trimmed := strings.TrimSpace(*group); trimmed != "" {
				id, err := groupResolver.Resolve(trimmed)
				if err != nil {
					return fmt.Errorf("beta-testers export: %w", err)
				}
				opts = append(opts, asc.WithBetaTestersGroupIDs([]string{id}))
			}

			firstPage, err := client.GetBetaTesters(requestCtx, resolvedAppID, opts...)
			if err != nil {
				return fmt.Errorf("beta-testers export: failed to fetch: %w", err)
			}

			all, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
				return client.GetBetaTesters(ctx, resolvedAppID, asc.WithBetaTestersNextURL(nextURL))
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
				groupMembership, err = fetchTesterGroupMemberships(requestCtx, client, groupResolver)
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

			if err := writeCSVFileAtomicNoSymlink(outputValue, header, rows); err != nil {
				return fmt.Errorf("beta-testers export: %w", err)
			}

			summary := &betaTestersExportSummary{
				AppID:         resolvedAppID,
				OutputFile:    filepath.Clean(outputValue),
				Total:         len(rows),
				IncludeGroups: *includeGroups,
			}

			return shared.PrintOutputWithRenderers(
				summary,
				*format.Output,
				*format.Pretty,
				func() error {
					asc.RenderTable(
						[]string{"App ID", "Output File", "Total", "Include Groups"},
						[][]string{{summary.AppID, summary.OutputFile, fmt.Sprintf("%d", summary.Total), fmt.Sprintf("%t", summary.IncludeGroups)}},
					)
					return nil
				},
				func() error {
					asc.RenderMarkdown(
						[]string{"App ID", "Output File", "Total", "Include Groups"},
						[][]string{{summary.AppID, summary.OutputFile, fmt.Sprintf("%d", summary.Total), fmt.Sprintf("%t", summary.IncludeGroups)}},
					)
					return nil
				},
			)
		},
	}
}

// BetaTestersImportCommand reads testers from a CSV file and applies changes.
func BetaTestersImportCommand() *ffcli.Command {
	fs := flag.NewFlagSet("import", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)")
	inputPath := fs.String("input", "", "Input CSV file path (required)")
	dryRun := fs.Bool("dry-run", false, "Validate and print plan without mutating network state")
	invite := fs.Bool("invite", false, "Invite newly created testers (default false)")
	group := fs.String("group", "", "Beta group name or ID to apply to all rows (optional)")
	skipExisting := fs.Bool("skip-existing", false, "If tester already exists, do not modify group membership")
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

Groups are semicolon-delimited in canonical import/export files.
For compatibility, comma-delimited groups are also accepted when no semicolon is present.

Examples:
  asc testflight beta-testers import --app "APP_ID" --input "./testflight-testers.csv" --dry-run
  asc testflight beta-testers import --app "APP_ID" --input "./testflight-testers.csv"
  asc testflight beta-testers import --app "APP_ID" --input "./testflight-testers.csv" --invite
  asc testflight beta-testers import --app "APP_ID" --input "./testflight-testers.csv" --group "Beta"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			resolvedAppID := shared.ResolveAppID(*appID)
			if resolvedAppID == "" {
				fmt.Fprintf(os.Stderr, "Error: --app is required (or set ASC_APP_ID)\n\n")
				return flag.ErrHelp
			}

			inputValue := strings.TrimSpace(*inputPath)
			if inputValue == "" {
				fmt.Fprintf(os.Stderr, "Error: --input is required\n\n")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("beta-testers import: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			parsedRows, err := readBetaTestersCSV(inputValue)
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
			appliedGroupID := ""
			if needsGroups {
				groupResolver, err = newBetaGroupResolver(requestCtx, client, resolvedAppID)
				if err != nil {
					return fmt.Errorf("beta-testers import: %w", err)
				}

				if appliedGroupValue != "" {
					id, err := groupResolver.Resolve(appliedGroupValue)
					if err != nil {
						return fmt.Errorf("beta-testers import: %w", err)
					}
					appliedGroupID = id
				}
			}

			existingByEmail, err := fetchExistingTestersByEmail(requestCtx, client, resolvedAppID)
			if err != nil {
				return fmt.Errorf("beta-testers import: %w", err)
			}

			seenInput := make(map[string]int) // emailLower -> first row index seen
			summary := &betaTestersImportSummary{
				AppID:           resolvedAppID,
				InputFile:       filepath.Clean(inputValue),
				DryRun:          *dryRun,
				Invite:          *invite,
				SkipExisting:    *skipExisting,
				ContinueOnError: *continueOnError,
				AppliedGroup:    appliedGroupValue,
				Total:           len(parsedRows),
			}

			for idx, row := range parsedRows {
				rowNumber := idx + 1 // 1-based data row index (excluding header)

				emailValue := strings.TrimSpace(row.email)
				if emailValue == "" {
					summary.Failed++
					summary.Failures = append(summary.Failures, betaTestersImportFailure{
						Row:   rowNumber,
						Error: "email is required",
					})
					if !*continueOnError {
						break
					}
					continue
				}
				if !isValidTesterEmail(emailValue) {
					summary.Failed++
					summary.Failures = append(summary.Failures, betaTestersImportFailure{
						Row:   rowNumber,
						Email: emailValue,
						Error: "invalid email format",
					})
					if !*continueOnError {
						break
					}
					continue
				}

				emailLower := strings.ToLower(emailValue)
				if firstSeen, exists := seenInput[emailLower]; exists {
					summary.Failed++
					summary.Failures = append(summary.Failures, betaTestersImportFailure{
						Row:   rowNumber,
						Email: emailValue,
						Error: fmt.Sprintf("duplicate email in input (already seen at row %d)", firstSeen),
					})
					if !*continueOnError {
						break
					}
					continue
				}
				seenInput[emailLower] = rowNumber

				var groupIDs []string
				if needsGroups {
					groupIDs, err = groupResolver.ResolveAll(row.groups)
					if err != nil {
						summary.Failed++
						summary.Failures = append(summary.Failures, betaTestersImportFailure{
							Row:   rowNumber,
							Email: emailValue,
							Error: err.Error(),
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

				if testerID, ok := existingByEmail[emailLower]; ok {
					summary.Existed++

					if *skipExisting || len(groupIDs) == 0 {
						continue
					}

					if *dryRun {
						summary.Updated++
						continue
					}

					if err := client.AddBetaTesterToGroups(requestCtx, testerID, groupIDs); err != nil {
						if errors.Is(err, asc.ErrConflict) {
							// Relationship already exists; treat as idempotent success.
							summary.Updated++
							continue
						}
						summary.Failed++
						summary.Failures = append(summary.Failures, betaTestersImportFailure{
							Row:   rowNumber,
							Email: emailValue,
							Error: err.Error(),
						})
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

				created, err := client.CreateBetaTester(requestCtx, emailValue, row.firstName, row.lastName, groupIDs)
				if err != nil {
					summary.Failed++
					summary.Failures = append(summary.Failures, betaTestersImportFailure{
						Row:   rowNumber,
						Email: emailValue,
						Error: err.Error(),
					})
					if !*continueOnError {
						break
					}
					continue
				}

				testerID := strings.TrimSpace(created.Data.ID)
				if testerID == "" {
					summary.Failed++
					summary.Failures = append(summary.Failures, betaTestersImportFailure{
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
				existingByEmail[emailLower] = testerID

				if *invite {
					invitation, err := client.CreateBetaTesterInvitation(requestCtx, resolvedAppID, testerID)
					if err != nil {
						summary.Failed++
						summary.Failures = append(summary.Failures, betaTestersImportFailure{
							Row:   rowNumber,
							Email: emailValue,
							Error: err.Error(),
						})
						if !*continueOnError {
							break
						}
						continue
					}
					if invitation == nil || strings.TrimSpace(invitation.Data.ID) == "" {
						summary.Failed++
						summary.Failures = append(summary.Failures, betaTestersImportFailure{
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
			if err := shared.PrintOutputWithRenderers(
				summary,
				*format.Output,
				*format.Pretty,
				func() error { return renderImportSummaryTables(summary, false) },
				func() error { return renderImportSummaryTables(summary, true) },
			); err != nil {
				return err
			}

			if summary.Failed > 0 {
				return shared.NewReportedError(fmt.Errorf("beta-testers import: %d row(s) failed", summary.Failed))
			}
			return nil
		},
	}
}

func renderImportSummaryTables(summary *betaTestersImportSummary, markdown bool) error {
	if summary == nil {
		return fmt.Errorf("summary is nil")
	}

	render := asc.RenderTable
	if markdown {
		render = asc.RenderMarkdown
	}

	render(
		[]string{"App ID", "Input File", "Dry Run", "Total", "Created", "Existed", "Updated", "Invited", "Failed"},
		[][]string{{
			summary.AppID,
			summary.InputFile,
			fmt.Sprintf("%t", summary.DryRun),
			fmt.Sprintf("%d", summary.Total),
			fmt.Sprintf("%d", summary.Created),
			fmt.Sprintf("%d", summary.Existed),
			fmt.Sprintf("%d", summary.Updated),
			fmt.Sprintf("%d", summary.Invited),
			fmt.Sprintf("%d", summary.Failed),
		}},
	)

	if len(summary.Failures) > 0 {
		rows := make([][]string, 0, len(summary.Failures))
		for _, f := range summary.Failures {
			rows = append(rows, []string{
				fmt.Sprintf("%d", f.Row),
				f.Email,
				f.Error,
			})
		}
		render([]string{"Row", "Email", "Error"}, rows)
	}

	return nil
}

type betaGroupResolver struct {
	byID           map[string]string   // id -> name
	idsByName      map[string][]string // lower(name) -> ids (sorted)
	uniqueNameByID map[string]bool     // id -> whether its name is unique among groups
	sortedGroupIDs []string            // stable iteration order (sorted by id)
}

func newBetaGroupResolver(ctx context.Context, client *asc.Client, appID string) (*betaGroupResolver, error) {
	if client == nil {
		return nil, fmt.Errorf("client is required")
	}
	appID = strings.TrimSpace(appID)
	if appID == "" {
		return nil, fmt.Errorf("app ID is required")
	}

	firstPage, err := client.GetBetaGroups(ctx, appID, asc.WithBetaGroupsLimit(200))
	if err != nil {
		return nil, err
	}
	all, err := asc.PaginateAll(ctx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
		return client.GetBetaGroups(ctx, appID, asc.WithBetaGroupsNextURL(nextURL))
	})
	if err != nil {
		return nil, err
	}
	resp, ok := all.(*asc.BetaGroupsResponse)
	if !ok || resp == nil {
		return nil, fmt.Errorf("unexpected beta groups response type")
	}

	r := &betaGroupResolver{
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
	if _, ok := r.byID[trimmed]; ok {
		return trimmed, nil
	}

	ids := r.idsByName[strings.ToLower(trimmed)]
	switch len(ids) {
	case 0:
		return "", fmt.Errorf("beta group %q not found", trimmed)
	case 1:
		return ids[0], nil
	default:
		return "", fmt.Errorf("multiple beta groups named %q; use group ID", trimmed)
	}
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

func fetchExistingTestersByEmail(ctx context.Context, client *asc.Client, appID string) (map[string]string, error) {
	first, err := client.GetBetaTesters(ctx, appID, asc.WithBetaTestersLimit(200))
	if err != nil {
		return nil, err
	}
	all, err := asc.PaginateAll(ctx, first, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
		return client.GetBetaTesters(ctx, appID, asc.WithBetaTestersNextURL(nextURL))
	})
	if err != nil {
		return nil, err
	}
	resp, ok := all.(*asc.BetaTestersResponse)
	if !ok || resp == nil {
		return nil, fmt.Errorf("unexpected beta testers response type")
	}

	byEmail := make(map[string]string, len(resp.Data))
	for _, tester := range resp.Data {
		email := strings.ToLower(strings.TrimSpace(tester.Attributes.Email))
		if email == "" {
			continue
		}
		if _, exists := byEmail[email]; exists {
			// Shouldn't happen, but keep the first for stable behavior.
			continue
		}
		byEmail[email] = strings.TrimSpace(tester.ID)
	}
	return byEmail, nil
}

func fetchTesterGroupMemberships(ctx context.Context, client *asc.Client, resolver *betaGroupResolver) (map[string][]string, error) {
	if client == nil {
		return nil, fmt.Errorf("client is required")
	}
	if resolver == nil {
		return nil, fmt.Errorf("group resolver is required")
	}

	membership := make(map[string][]string)
	for _, groupID := range resolver.sortedGroupIDs {
		exportValue := resolver.exportValueForID(groupID)

		firstPage, err := client.GetBetaGroupTesters(ctx, groupID, asc.WithBetaGroupTestersLimit(200))
		if err != nil {
			return nil, err
		}
		all, err := asc.PaginateAll(ctx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
			return client.GetBetaGroupTesters(ctx, groupID, asc.WithBetaGroupTestersNextURL(nextURL))
		})
		if err != nil {
			return nil, err
		}
		testersResp, ok := all.(*asc.BetaTestersResponse)
		if !ok || testersResp == nil {
			return nil, fmt.Errorf("unexpected beta group testers response type")
		}

		for _, tester := range testersResp.Data {
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

func readBetaTestersCSV(path string) ([]betaTestersCSVRow, error) {
	file, err := shared.OpenExistingNoFollow(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()

	reader := csv.NewReader(file)
	reader.FieldsPerRecord = -1
	reader.LazyQuotes = true

	header, err := reader.Read()
	if err != nil {
		if errors.Is(err, io.EOF) {
			return nil, shared.UsageError("CSV file is empty")
		}
		return nil, fmt.Errorf("read header: %w", err)
	}

	rows := make([]betaTestersCSVRow, 0)
	headerIdx, hasHeader, err := parseBetaTestersCSVHeader(header)
	if err != nil {
		return nil, err
	}
	if !hasHeader {
		row, rowErr := parseLegacyBetaTesterCSVRow(header)
		if rowErr != nil {
			return nil, rowErr
		}
		rows = append(rows, row)
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
			row, rowErr := parseLegacyBetaTesterCSVRow(record)
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

func validateBetaTestersCSVHeader(header []string) (map[string]int, error) {
	if len(header) == 0 {
		return nil, shared.UsageError("CSV header row is required")
	}

	idx := make(map[string]int, len(header))
	for i, raw := range header {
		col := strings.ToLower(strings.TrimSpace(raw))
		if col == "" {
			return nil, shared.UsageError("CSV header contains an empty column name")
		}
		canonical, ok := canonicalBetaTestersCSVColumn(col)
		if !ok {
			return nil, shared.UsageErrorf("unknown CSV column %q (allowed: email, first_name, last_name, groups)", col)
		}
		col = canonical
		if _, exists := idx[col]; exists {
			return nil, shared.UsageErrorf("duplicate CSV column %q", col)
		}
		idx[col] = i
	}
	if _, ok := idx["email"]; !ok {
		return nil, shared.UsageError("CSV header must include required column \"email\"")
	}
	return idx, nil
}

func parseBetaTestersCSVHeader(firstRow []string) (map[string]int, bool, error) {
	if len(firstRow) == 0 {
		return nil, false, shared.UsageError("CSV header row is required")
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
		_, err := validateBetaTestersCSVHeader(firstRow)
		return nil, false, err
	}
	headerIdx, err := validateBetaTestersCSVHeader(firstRow)
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
	default:
		return "", false
	}
}

func parseHeaderMappedBetaTesterCSVRow(record []string, headerIdx map[string]int) betaTestersCSVRow {
	get := func(col string) string {
		i, ok := headerIdx[col]
		if !ok || i < 0 || i >= len(record) {
			return ""
		}
		return strings.TrimSpace(record[i])
	}
	groups := make([]string, 0)
	if idx, ok := headerIdx["groups"]; ok && idx >= 0 && idx < len(record) {
		groups = splitBetaTesterCSVGroups(record[idx])
	}
	return betaTestersCSVRow{
		email:     get("email"),
		firstName: get("first_name"),
		lastName:  get("last_name"),
		groups:    groups,
	}
}

func parseLegacyBetaTesterCSVRow(record []string) (betaTestersCSVRow, error) {
	if len(record) < 3 || len(record) > 4 {
		return betaTestersCSVRow{}, shared.UsageError("legacy CSV rows must have 3 or 4 columns: first_name,last_name,email[,groups]")
	}
	row := betaTestersCSVRow{
		firstName: strings.TrimSpace(record[0]),
		lastName:  strings.TrimSpace(record[1]),
		email:     strings.TrimSpace(record[2]),
	}
	if len(record) >= 4 {
		row.groups = splitBetaTesterCSVGroups(record[3])
	}
	return row, nil
}

func splitBetaTesterCSVGroups(value string) []string {
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
	if err := w.Write(header); err != nil {
		return err
	}
	for _, row := range rows {
		if err := w.Write(row); err != nil {
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
