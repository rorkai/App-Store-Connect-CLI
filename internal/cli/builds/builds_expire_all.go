package builds

import (
	"context"
	"flag"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

type buildExpireCandidate struct {
	resource   asc.Resource[asc.BuildAttributes]
	uploadedAt time.Time
	ageDays    int
}

// BuildsExpireAllCommand returns a command to batch expire builds.
func BuildsExpireAllCommand() *ffcli.Command {
	fs := flag.NewFlagSet("builds expire-all", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (required, or ASC_APP_ID env)")
	olderThan := fs.String("older-than", "", "Expire builds older than duration (e.g., 90d, 2w, 30d) or date (YYYY-MM-DD)")
	keepLatest := fs.Int("keep-latest", 0, "Keep the N most recent builds")
	dryRun := fs.Bool("dry-run", false, "Preview builds that would be expired without expiring")
	confirm := fs.Bool("confirm", false, "Confirm expiration (required unless --dry-run)")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "expire-all",
		ShortUsage: "asc builds expire-all [flags]",
		ShortHelp:  "Expire multiple TestFlight builds for an app.",
		LongHelp: `Expire multiple TestFlight builds for an app.

Use --older-than to expire builds older than a duration or date, and optionally
--keep-latest to preserve recent builds. Use --dry-run to preview without
expiring.

Examples:
  asc builds expire-all --app "123456789" --older-than 90d --dry-run
  asc builds expire-all --app "123456789" --older-than 30d --confirm
  asc builds expire-all --app "123456789" --keep-latest 5 --confirm
  asc builds expire-all --app "123456789" --older-than "2025-01-01" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			resolvedAppID := shared.ResolveAppID(*appID)
			if resolvedAppID == "" {
				fmt.Fprintf(os.Stderr, "Error: --app is required (or set ASC_APP_ID)\n\n")
				return flag.ErrHelp
			}

			olderThanValue := strings.TrimSpace(*olderThan)
			if olderThanValue == "" && *keepLatest == 0 {
				fmt.Fprintln(os.Stderr, "Error: --older-than or --keep-latest is required")
				return flag.ErrHelp
			}
			if *keepLatest < 0 {
				return fmt.Errorf("builds expire-all: --keep-latest must be greater than or equal to 0")
			}
			if !*dryRun && !*confirm {
				fmt.Fprintln(os.Stderr, "Error: --confirm is required to expire builds")
				return flag.ErrHelp
			}

			now := time.Now().UTC()
			var olderThanThreshold time.Time
			if olderThanValue != "" {
				threshold, err := parseOlderThanThreshold(olderThanValue, now)
				if err != nil {
					return fmt.Errorf("builds expire-all: %w", err)
				}
				olderThanThreshold = threshold
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("builds expire-all: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			firstPage, err := client.GetBuilds(requestCtx, resolvedAppID, asc.WithBuildsLimit(200), asc.WithBuildsSort("-uploadedDate"))
			if err != nil {
				return fmt.Errorf("builds expire-all: failed to fetch: %w", err)
			}

			allPages, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
				return client.GetBuilds(ctx, resolvedAppID, asc.WithBuildsNextURL(nextURL))
			})
			if err != nil {
				return fmt.Errorf("builds expire-all: %w", err)
			}

			builds, ok := allPages.(*asc.BuildsResponse)
			if !ok {
				return fmt.Errorf("builds expire-all: unexpected response type")
			}

			candidates := make([]buildExpireCandidate, 0, len(builds.Data))
			skippedExpired := 0
			skippedInvalid := 0
			for _, item := range builds.Data {
				if item.Attributes.Expired {
					skippedExpired++
					continue
				}
				uploadedAt, err := parseBuildTimestamp(item.Attributes.UploadedDate)
				if err != nil {
					skippedInvalid++
					fmt.Fprintf(os.Stderr, "Warning: build %s has invalid uploadedDate %q: %v\n", item.ID, item.Attributes.UploadedDate, err)
					continue
				}
				ageDays := int(now.Sub(uploadedAt).Hours() / 24)
				if ageDays < 0 {
					ageDays = 0
				}
				candidates = append(candidates, buildExpireCandidate{
					resource:   item,
					uploadedAt: uploadedAt,
					ageDays:    ageDays,
				})
			}

			sort.Slice(candidates, func(i, j int) bool {
				return candidates[i].uploadedAt.After(candidates[j].uploadedAt)
			})

			if *keepLatest > 0 {
				if *keepLatest >= len(candidates) {
					candidates = nil
				} else {
					candidates = candidates[*keepLatest:]
				}
			}

			if !olderThanThreshold.IsZero() {
				filtered := candidates[:0]
				for _, candidate := range candidates {
					if candidate.uploadedAt.Before(olderThanThreshold) {
						filtered = append(filtered, candidate)
					}
				}
				candidates = filtered
			}

			items := make([]asc.BuildExpireAllItem, 0, len(candidates))
			failures := make([]asc.BuildExpireAllFailure, 0)
			expiredCount := 0

			for _, candidate := range candidates {
				item := buildExpireAllItem(candidate)
				if *dryRun {
					items = append(items, item)
					continue
				}

				if _, err := client.ExpireBuild(requestCtx, candidate.resource.ID); err != nil {
					failures = append(failures, asc.BuildExpireAllFailure{
						ID:    candidate.resource.ID,
						Error: err.Error(),
					})
					continue
				}

				expiredCount++
				expired := true
				item.Expired = &expired
				items = append(items, item)
			}

			var olderThanPtr *string
			if olderThanValue != "" {
				olderThanPtr = &olderThanValue
			}

			var keepLatestPtr *int
			if *keepLatest > 0 {
				keepLatestValue := *keepLatest
				keepLatestPtr = &keepLatestValue
			}

			var skippedExpiredPtr *int
			if skippedExpired > 0 {
				skippedExpiredValue := skippedExpired
				skippedExpiredPtr = &skippedExpiredValue
			}

			var skippedInvalidPtr *int
			if skippedInvalid > 0 {
				skippedInvalidValue := skippedInvalid
				skippedInvalidPtr = &skippedInvalidValue
			}

			result := &asc.BuildExpireAllResult{
				DryRun:              *dryRun,
				AppID:               resolvedAppID,
				OlderThan:           olderThanPtr,
				KeepLatest:          keepLatestPtr,
				SelectedCount:       len(candidates),
				ExpiredCount:        expiredCount,
				SkippedExpiredCount: skippedExpiredPtr,
				SkippedInvalidCount: skippedInvalidPtr,
				Builds:              items,
				Failures:            failures,
			}

			if err := shared.PrintOutput(result, *output, *pretty); err != nil {
				return err
			}

			if len(failures) > 0 {
				return fmt.Errorf("builds expire-all: %d builds failed to expire", len(failures))
			}

			return nil
		},
	}
}

func buildExpireAllItem(candidate buildExpireCandidate) asc.BuildExpireAllItem {
	return asc.BuildExpireAllItem{
		ID:           candidate.resource.ID,
		Version:      candidate.resource.Attributes.Version,
		UploadedDate: candidate.resource.Attributes.UploadedDate,
		AgeDays:      candidate.ageDays,
	}
}

func parseBuildTimestamp(value string) (time.Time, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return time.Time{}, fmt.Errorf("uploadedDate is empty")
	}
	if parsed, err := time.Parse(time.RFC3339, trimmed); err == nil {
		return parsed, nil
	}
	if parsed, err := time.Parse(time.RFC3339Nano, trimmed); err == nil {
		return parsed, nil
	}
	return time.Time{}, fmt.Errorf("invalid time %q", trimmed)
}

func parseOlderThanThreshold(value string, now time.Time) (time.Time, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return time.Time{}, fmt.Errorf("--older-than must not be empty")
	}
	if parsed, err := time.Parse("2006-01-02", trimmed); err == nil {
		return parsed, nil
	}
	if parsed, err := time.Parse(time.RFC3339, trimmed); err == nil {
		return parsed, nil
	}
	duration, err := parseOlderThanDuration(trimmed)
	if err != nil {
		return time.Time{}, err
	}
	return now.Add(-duration), nil
}

func parseOlderThanDuration(value string) (time.Duration, error) {
	trimmed := strings.ToLower(strings.TrimSpace(value))
	if trimmed == "" {
		return 0, fmt.Errorf("--older-than must not be empty")
	}
	if len(trimmed) < 2 {
		return 0, fmt.Errorf("--older-than must be a duration like 90d, 2w, or 3m")
	}
	unit := trimmed[len(trimmed)-1]
	number := strings.TrimSpace(trimmed[:len(trimmed)-1])
	if number == "" {
		return 0, fmt.Errorf("--older-than must be a duration like 90d, 2w, or 3m")
	}
	valueInt, err := strconv.Atoi(number)
	if err != nil || valueInt <= 0 {
		return 0, fmt.Errorf("--older-than must be a duration like 90d, 2w, or 3m")
	}

	switch unit {
	case 'd':
		return time.Duration(valueInt) * 24 * time.Hour, nil
	case 'w':
		return time.Duration(valueInt) * 7 * 24 * time.Hour, nil
	case 'm':
		return time.Duration(valueInt) * 30 * 24 * time.Hour, nil
	default:
		return 0, fmt.Errorf("--older-than must be a duration like 90d, 2w, or 3m")
	}
}
