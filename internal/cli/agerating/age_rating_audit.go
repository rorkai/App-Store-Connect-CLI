package agerating

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

const ageRatingAuditWorkers = 5

// AgeRatingAuditCommand returns the age-rating audit subcommand.
func AgeRatingAuditCommand() *ffcli.Command {
	fs := flag.NewFlagSet("age-rating audit", flag.ExitOnError)

	appIDs := shared.BindOnceCSVFlag(fs, "app", "Restrict the audit to specific app IDs (comma-separated)")
	paginate := fs.Bool("paginate", false, "Fetch all app pages (default: first page only)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "audit",
		ShortUsage: "asc age-rating audit [--app \"APP_ID,APP_ID\"] [--paginate] [flags]",
		ShortHelp:  "Audit social-media age rating responses across apps.",
		LongHelp: `Audit social-media age rating responses across apps.

Starting September 2026, Apple requires responses to the social-media
capability questions in the age rating questionnaire for every new submission
and app update. By default, this command audits the first app page (up to 200
apps). Every active AppInfo for each app is audited. Pass --paginate to audit
every app page, or --app to audit specific IDs.

A response counts as missing when:
  - socialMedia is unset, or false while socialMediaAgeRestricted is true
  - messagingAndChat is unset
  - socialMediaAgeRestricted is unset while socialMedia is true
  - ageAssurance is unset or false while socialMediaAgeRestricted is true
  - userGeneratedContent is unset or false while socialMedia or socialMediaAgeRestricted is true

Use "asc age-rating edit" to fill gaps.

Examples:
  asc age-rating audit
  asc age-rating audit --paginate
  asc age-rating audit --app "123456789,987654321"
  asc age-rating audit --output table`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			appProvided := false
			fs.Visit(func(parsed *flag.Flag) {
				appProvided = appProvided || parsed.Name == "app"
			})
			only := shared.SplitUniqueCSV(appIDs.String())
			if appProvided && len(only) == 0 {
				return shared.UsageError("--app must include at least one app ID")
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("age-rating audit: %w", err)
			}
			apps, moreApps, err := auditTargetApps(ctx, client, only, *paginate)
			if err != nil {
				return fmt.Errorf("age-rating audit: %w", err)
			}
			if len(apps) == 0 {
				return fmt.Errorf("age-rating audit: no apps matched")
			}

			rows := auditDeclarations(ctx, client, apps)

			result := asc.AgeRatingAuditResult{Apps: rows}
			for _, row := range rows {
				switch {
				case row.Error != "":
					result.ErrorCount++
				case row.Ready:
					result.ReadyCount++
				default:
					result.MissingCount++
				}
			}

			if result.MissingCount > 0 {
				fmt.Fprintf(
					os.Stderr,
					"%d of %d active app info records are missing social-media age rating responses; Apple requires them for new submissions and updates starting September 2026.\n",
					result.MissingCount,
					len(rows),
				)
			} else if result.ErrorCount == 0 {
				fmt.Fprintf(os.Stderr, "All %d active app info records have the social-media age rating responses set.\n", len(rows))
			}
			if len(only) == 0 && moreApps {
				fmt.Fprintln(os.Stderr, "Warning: more apps exist; use --paginate to audit every app.")
			}

			if err := shared.PrintOutput(&result, *output.Output, *output.Pretty); err != nil {
				return err
			}
			if result.ErrorCount > 0 {
				noun := "app info records"
				if result.ErrorCount == 1 {
					noun = "app info record"
				}
				err := fmt.Errorf("age-rating audit: %d %s could not be audited; see row errors in the output", result.ErrorCount, noun)
				fmt.Fprintln(os.Stderr, err)
				return shared.NewStderrReportedError(err)
			}
			return nil
		},
	}
}

type auditApp struct {
	id       string
	name     string
	bundleID string
}

func auditTargetApps(ctx context.Context, client *asc.Client, only []string, paginate bool) ([]auditApp, bool, error) {
	requestCtx, cancel := shared.ContextWithTimeout(ctx)
	defer cancel()

	wanted := map[string]bool{}
	for _, id := range only {
		wanted[strings.TrimSpace(id)] = true
	}

	apps := []auditApp{}
	next := ""
	seenNext := map[string]struct{}{}
	page := 1
	for {
		opts := []asc.AppsOption{asc.Limit[asc.AppsOption](200)}
		if next != "" {
			opts = append(opts, asc.NextURL[asc.AppsOption](next))
		}
		resp, err := client.GetApps(requestCtx, opts...)
		if err != nil {
			return nil, false, err
		}
		for _, app := range resp.Data {
			if len(wanted) > 0 && !wanted[app.ID] {
				continue
			}
			apps = append(apps, auditApp{id: app.ID, name: app.Attributes.Name, bundleID: app.Attributes.BundleID})
		}
		next = strings.TrimSpace(resp.Links.Next)
		if next == "" || !paginate {
			break
		}
		if _, seen := seenNext[next]; seen {
			return nil, false, fmt.Errorf("page %d: %w", page+1, asc.ErrRepeatedPaginationURL)
		}
		seenNext[next] = struct{}{}
		page++
	}

	for _, id := range only {
		found := false
		for _, app := range apps {
			if app.id == id {
				found = true
				break
			}
		}
		if !found {
			apps = append(apps, auditApp{id: id})
		}
	}
	return apps, next != "", nil
}

func auditDeclarations(ctx context.Context, client *asc.Client, apps []auditApp) []asc.AgeRatingAuditRow {
	rowsByApp := make([][]asc.AgeRatingAuditRow, len(apps))
	jobs := make(chan int)

	var workers sync.WaitGroup
	workerCount := min(ageRatingAuditWorkers, len(apps))
	workers.Add(workerCount)
	for range workerCount {
		go func() {
			defer workers.Done()
			for index := range jobs {
				rowsByApp[index] = auditAppDeclarations(ctx, client, apps[index])
			}
		}()
	}
	for index := range apps {
		jobs <- index
	}
	close(jobs)
	workers.Wait()

	rows := make([]asc.AgeRatingAuditRow, 0, len(apps))
	for _, appRows := range rowsByApp {
		rows = append(rows, appRows...)
	}
	return rows
}

func auditAppDeclarations(ctx context.Context, client *asc.Client, app auditApp) []asc.AgeRatingAuditRow {
	requestCtx, cancel := shared.ContextWithTimeout(ctx)
	candidates, err := client.ListAppInfoCandidatesForApp(requestCtx, app.id)
	cancel()
	if err != nil {
		return []asc.AgeRatingAuditRow{newAgeRatingAuditErrorRow(app, asc.AppInfoCandidate{}, err)}
	}

	current := asc.CurrentAppInfoCandidates(candidates)
	if len(current) == 0 {
		err := fmt.Errorf(
			"no current app info found for app %q (%s); run `asc apps info list --app %q` to inspect candidates",
			app.id,
			asc.FormatAppInfoCandidates(candidates),
			app.id,
		)
		return []asc.AgeRatingAuditRow{newAgeRatingAuditErrorRow(app, asc.AppInfoCandidate{}, err)}
	}

	rows := make([]asc.AgeRatingAuditRow, 0, len(current))
	for _, candidate := range current {
		rows = append(rows, auditDeclaration(ctx, client, app, candidate))
	}
	return rows
}

func auditDeclaration(ctx context.Context, client *asc.Client, app auditApp, appInfo asc.AppInfoCandidate) asc.AgeRatingAuditRow {
	row := asc.AgeRatingAuditRow{
		AppID:            app.id,
		AppInfoID:        appInfo.ID,
		AppInfoState:     appInfo.State,
		Name:             app.name,
		BundleID:         app.bundleID,
		MissingResponses: []string{},
	}

	requestCtx, cancel := shared.ContextWithTimeout(ctx)
	defer cancel()

	resp, err := fetchAgeRatingDeclaration(requestCtx, client, "", appInfo.ID, "", nil)
	if err != nil {
		return newAgeRatingAuditErrorRow(app, appInfo, err)
	}

	attrs := resp.Data.Attributes
	socialMedia := nullableBoolValue(attrs.SocialMedia)
	socialMediaAgeRestricted := nullableBoolValue(attrs.SocialMediaAgeRestricted)
	row.SocialMedia = auditBoolStatus(socialMedia)
	row.SocialMediaAgeRestricted = auditBoolStatus(socialMediaAgeRestricted)
	row.MessagingAndChat = auditBoolStatus(attrs.MessagingAndChat)
	row.UserGeneratedContent = auditBoolStatus(attrs.UserGeneratedContent)
	row.AgeAssurance = auditBoolStatus(attrs.AgeAssurance)

	if socialMedia == nil {
		row.MissingResponses = append(row.MissingResponses, "socialMedia")
	}
	if attrs.MessagingAndChat == nil {
		row.MissingResponses = append(row.MissingResponses, "messagingAndChat")
	}
	if boolIsTrue(socialMedia) && socialMediaAgeRestricted == nil {
		row.MissingResponses = append(row.MissingResponses, "socialMediaAgeRestricted")
	}
	if boolIsTrue(socialMediaAgeRestricted) && boolIsFalse(socialMedia) {
		row.MissingResponses = append(row.MissingResponses, "socialMedia")
	}
	if (boolIsTrue(socialMedia) || boolIsTrue(socialMediaAgeRestricted)) && !boolIsTrue(attrs.UserGeneratedContent) {
		row.MissingResponses = append(row.MissingResponses, "userGeneratedContent")
	}
	if boolIsTrue(socialMediaAgeRestricted) && !boolIsTrue(attrs.AgeAssurance) {
		row.MissingResponses = append(row.MissingResponses, "ageAssurance")
	}
	row.Ready = len(row.MissingResponses) == 0
	return row
}

func newAgeRatingAuditErrorRow(app auditApp, appInfo asc.AppInfoCandidate, err error) asc.AgeRatingAuditRow {
	return asc.AgeRatingAuditRow{
		AppID:                    app.id,
		AppInfoID:                appInfo.ID,
		AppInfoState:             appInfo.State,
		Name:                     app.name,
		BundleID:                 app.bundleID,
		SocialMedia:              "-",
		SocialMediaAgeRestricted: "-",
		MessagingAndChat:         "-",
		UserGeneratedContent:     "-",
		AgeAssurance:             "-",
		MissingResponses:         []string{},
		Error:                    err.Error(),
	}
}

func auditBoolStatus(value *bool) string {
	if value == nil {
		return "UNSET"
	}
	return fmt.Sprintf("%t", *value)
}
