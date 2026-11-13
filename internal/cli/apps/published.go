package apps

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

const (
	appsPublishedAuditTimeout = 5 * time.Minute
	appsPublishedWorkers      = 4
)

var appsPublishedClientFactory = shared.GetASCClient

// AppsPublishedCommand returns the account-wide published-app audit command.
func AppsPublishedCommand() *ffcli.Command {
	fs := flag.NewFlagSet("apps published", flag.ExitOnError)
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "published",
		ShortUsage: "asc apps published [flags]",
		ShortHelp:  "List published apps and their published-territory counts.",
		LongHelp: `List published apps and their published-territory counts.

The command audits every App Store Connect app record and all of its territory
availability pages. An app is published when at least one territory reports the
AVAILABLE content status. Apps without a published territory are omitted from
the result, while the report includes the total number of audited app records.

Examples:
  asc apps published
  asc apps published --output table
  asc apps published --output markdown`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			client, err := appsPublishedClientFactory()
			if err != nil {
				return fmt.Errorf("apps published: %w", err)
			}

			requestCtx, cancel := shared.ContextWithResolvedTimeout(ctx, appsPublishedAuditTimeout)
			defer cancel()
			appsResponse, err := fetchAllAppsForPublishedAudit(requestCtx, client)
			if err != nil {
				return fmt.Errorf("apps published: fetch apps: %w", err)
			}

			report, auditErr := auditPublishedApps(requestCtx, client, appsResponse.Data)
			if strings.EqualFold(strings.TrimSpace(*output.Output), "json") {
				fmt.Fprintf(
					os.Stderr,
					"%s\n",
					asc.PublishedAppsSummary(report),
				)
			}
			if err := shared.PrintOutput(&report, *output.Output, *output.Pretty); err != nil {
				return err
			}
			if auditErr != nil {
				return fmt.Errorf("apps published: %w", auditErr)
			}
			return nil
		},
	}
}

// fetchAllAppsForPublishedAudit retrieves every app record across all result pages.
func fetchAllAppsForPublishedAudit(ctx context.Context, client *asc.Client) (*asc.AppsResponse, error) {
	firstPage, err := client.GetApps(ctx, asc.WithAppsLimit(200), asc.WithAppsSort("name"))
	if err != nil {
		return nil, err
	}
	allPages, err := asc.PaginateAll(ctx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
		return client.GetApps(ctx, asc.WithAppsNextURL(nextURL))
	})
	if err != nil {
		return nil, err
	}
	response, ok := allPages.(*asc.AppsResponse)
	if !ok {
		return nil, fmt.Errorf("unexpected apps response type %T", allPages)
	}
	return response, nil
}

type publishedAppAuditResult struct {
	App            asc.Resource[asc.AppAttributes]
	AvailabilityID string
	TerritoryCount int
	Err            error
}

// auditPublishedApps audits app availability concurrently and returns published apps.
func auditPublishedApps(ctx context.Context, client *asc.Client, appResources []asc.Resource[asc.AppAttributes]) (asc.AppsPublishedReport, error) {
	report := asc.AppsPublishedReport{
		AuditedAppCount: len(appResources),
		Apps:            make([]asc.PublishedApp, 0),
	}
	if len(appResources) == 0 {
		return report, nil
	}

	workerCount := min(appsPublishedWorkers, len(appResources))
	jobs := make(chan asc.Resource[asc.AppAttributes])
	results := make(chan publishedAppAuditResult, len(appResources))
	var workers sync.WaitGroup
	workers.Add(workerCount)
	for range workerCount {
		go func() {
			defer workers.Done()
			for app := range jobs {
				availabilityID, territoryCount, err := auditPublishedApp(ctx, client, app.ID)
				results <- publishedAppAuditResult{
					App:            app,
					AvailabilityID: availabilityID,
					TerritoryCount: territoryCount,
					Err:            err,
				}
			}
		}()
	}
	go func() {
		for _, app := range appResources {
			jobs <- app
		}
		close(jobs)
		workers.Wait()
		close(results)
	}()

	failures := make([]asc.PublishedAppFailure, 0)
	for result := range results {
		if result.Err != nil {
			failures = append(failures, asc.PublishedAppFailure{
				ID:    result.App.ID,
				Name:  result.App.Attributes.Name,
				Error: result.Err.Error(),
			})
			continue
		}
		if result.TerritoryCount == 0 {
			continue
		}
		report.Apps = append(report.Apps, asc.PublishedApp{
			ID:                      result.App.ID,
			Name:                    result.App.Attributes.Name,
			BundleID:                result.App.Attributes.BundleID,
			SKU:                     result.App.Attributes.SKU,
			PrimaryLocale:           result.App.Attributes.PrimaryLocale,
			AvailabilityID:          result.AvailabilityID,
			PublishedTerritoryCount: result.TerritoryCount,
		})
	}
	sort.Slice(report.Apps, func(i, j int) bool {
		left := strings.ToLower(strings.TrimSpace(report.Apps[i].Name))
		right := strings.ToLower(strings.TrimSpace(report.Apps[j].Name))
		if left != right {
			return left < right
		}
		return report.Apps[i].ID < report.Apps[j].ID
	})
	sortPublishedAppFailures(failures)
	report.Failures = failures
	report.PublishedAppCount = len(report.Apps)
	if len(failures) > 0 {
		details := make([]string, 0, len(failures))
		for _, failure := range failures {
			details = append(details, fmt.Sprintf("%s (%s): %s", failure.Name, failure.ID, failure.Error))
		}
		return report, fmt.Errorf("audit failed for %d of %d apps: %s", len(failures), len(appResources), strings.Join(details, "; "))
	}
	return report, nil
}

func sortPublishedAppFailures(failures []asc.PublishedAppFailure) {
	sort.Slice(failures, func(i, j int) bool {
		left := strings.ToLower(strings.TrimSpace(failures[i].Name))
		right := strings.ToLower(strings.TrimSpace(failures[j].Name))
		if left != right {
			return left < right
		}
		return failures[i].ID < failures[j].ID
	})
}

// auditPublishedApp returns an app's availability ID and published-territory count.
func auditPublishedApp(ctx context.Context, client *asc.Client, appID string) (string, int, error) {
	availability, err := client.GetAppAvailabilityV2(ctx, appID)
	if err != nil {
		if isPublishedAuditAvailabilityMissing(err) {
			return "", 0, nil
		}
		return "", 0, fmt.Errorf("fetch availability: %w", err)
	}
	availabilityID := strings.TrimSpace(availability.Data.ID)
	if availabilityID == "" {
		return "", 0, fmt.Errorf("availability response did not include an ID")
	}

	firstPage, err := client.GetTerritoryAvailabilities(ctx, availabilityID, asc.Limit[asc.TerritoryAvailabilitiesOption](200))
	if err != nil {
		return availabilityID, 0, fmt.Errorf("fetch territory availabilities: %w", err)
	}
	allPages, err := asc.PaginateAll(ctx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
		return client.GetTerritoryAvailabilities(ctx, availabilityID, asc.NextURL[asc.TerritoryAvailabilitiesOption](nextURL))
	})
	if err != nil {
		return availabilityID, 0, fmt.Errorf("fetch territory availabilities: %w", err)
	}
	territories, ok := allPages.(*asc.TerritoryAvailabilitiesResponse)
	if !ok {
		return availabilityID, 0, fmt.Errorf("unexpected territory availabilities response type %T", allPages)
	}

	publishedTerritoryCount := 0
	for _, territory := range territories.Data {
		if hasAvailableContentStatus(territory.Attributes.ContentStatuses) {
			publishedTerritoryCount++
		}
	}
	return availabilityID, publishedTerritoryCount, nil
}

// isPublishedAuditAvailabilityMissing reports resource-specific missing availability.
func isPublishedAuditAvailabilityMissing(err error) bool {
	if err == nil || !errors.Is(err, asc.ErrNotFound) {
		return false
	}
	var apiErr *asc.APIError
	if !errors.As(err, &apiErr) {
		return false
	}
	description := strings.ToLower(strings.Join([]string{apiErr.Code, apiErr.Title, apiErr.Detail}, " "))
	return strings.Contains(description, "appavailabilities") ||
		strings.Contains(description, "app availability") ||
		strings.Contains(description, "appavailabilityv2")
}

// hasAvailableContentStatus reports whether any content status is AVAILABLE.
func hasAvailableContentStatus(statuses []string) bool {
	for _, status := range statuses {
		if strings.EqualFold(strings.TrimSpace(status), "AVAILABLE") {
			return true
		}
	}
	return false
}
