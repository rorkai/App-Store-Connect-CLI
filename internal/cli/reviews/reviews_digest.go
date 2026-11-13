package reviews

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/rootfs"
)

const reviewDigestStateFileLimit = 64 * 1024

var (
	errReviewDigestReachedCursor    = errors.New("reached digest cursor")
	errReviewDigestVersionsComplete = errors.New("all digest reviews have a version")
)

type reviewDigestState struct {
	Cursor string `json:"cursor"`
}

type reviewDigestRelationships struct {
	Response *struct {
		Data *struct {
			ID string `json:"id"`
		} `json:"data"`
	} `json:"response"`
}

// ReviewsDigestCommand returns the reviews digest subcommand.
func ReviewsDigestCommand() *ffcli.Command {
	fs := flag.NewFlagSet("digest", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)")
	since := fs.String("since", "", "Cursor: include reviews created after this RFC3339 timestamp or YYYY-MM-DD date (UTC); with --state-file, used only when the file does not exist")
	stateFile := fs.String("state-file", "", "Path to a JSON state file holding the cursor ({\"cursor\":\"<RFC3339>\"})")
	writeState := fs.Bool("write-state", false, "After the digest prints, write the newest review date to --state-file")
	lowest := fs.Int("lowest", 5, "Number of lowest-rated reviews to include")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "digest",
		ShortUsage: "asc reviews digest --app APP_ID (--since DATE | --state-file PATH) [flags]",
		ShortHelp:  "Summarize customer reviews created after a cursor.",
		LongHelp: `Summarize customer reviews created after a cursor.

Reports the review count, average rating, star distribution, counts and
average rating by territory and app version, the lowest-rated reviews, and
how many reviews have no response. Version groups distinguish platform and
App Store version ID, even when version numbers match. Reviews that do not
appear in any returned version remain in the overall totals and are counted
as unassignedVersionReviews ("Reviews without version" in human output).
Version lookup stops once every digest review is assigned. Otherwise it reads
all version pages; raise ASC_TIMEOUT for apps with long version histories.

The cursor is exclusive: only reviews created after it are included. Reviews
are read newest first and pagination stops at the first review at or before
the cursor.

The cursor comes from --since or from --state-file. When --state-file exists,
its cursor takes precedence over --since, which must still be valid;
cursorSource in the output reports which one applied. Nothing is saved unless --write-state is set, which writes
the newest review date (or the unchanged cursor) back to --state-file after
the digest prints.

Examples:
  asc reviews digest --app "123456789" --since 2026-10-01
  asc reviews digest --app "123456789" --since 2026-10-01T00:00:00Z --output table
  asc reviews digest --app "123456789" --state-file reviews-cursor.json --since 2026-10-01 --write-state

Send to Slack and advance the cursor only after Slack accepts the digest.
Each attempt starts from the committed cursor, or the explicit initial date:
  (
    set -e
    if [ -e reviews-cursor.json ]; then
      cp reviews-cursor.json next-cursor.json
    else
      printf '%s\n' '{"cursor":"2026-10-01T00:00:00Z"}' > next-cursor.json
    fi
    asc reviews digest --app "123456789" --state-file next-cursor.json --write-state --output json > digest.json
    asc notify slack --message "New customer reviews" --payload-file digest.json
    mv next-cursor.json reviews-cursor.json
  )`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			resolvedAppID := shared.ResolveAppID(*appID)
			if resolvedAppID == "" {
				fmt.Fprintf(os.Stderr, "Error: --app is required (or set ASC_APP_ID)\n\n")
				return shared.MissingRequiredUsageError("--app")
			}
			if *lowest < 0 {
				return shared.WithDiagnostic(shared.UsageError("--lowest must be 0 or greater"), shared.DiagnosticInvalidInput, "--lowest")
			}
			path := strings.TrimSpace(*stateFile)
			if *writeState && path == "" {
				return shared.WithDiagnostic(shared.UsageError("--write-state requires --state-file"), shared.DiagnosticRequiredInputMissing, "--state-file")
			}
			var sinceTime time.Time
			if strings.TrimSpace(*since) != "" {
				parsed, err := parseReviewDigestSince(*since)
				if err != nil {
					return shared.WithDiagnostic(shared.UsageError(err.Error()), shared.DiagnosticInvalidInput, "--since")
				}
				sinceTime = parsed
			}

			cursor, source, err := resolveReviewDigestCursor(path, sinceTime)
			if err != nil {
				return err
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("reviews digest: %w", err)
			}
			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			result, err := buildReviewDigest(requestCtx, client, resolvedAppID, cursor, *lowest)
			if err != nil {
				return fmt.Errorf("reviews digest: %w", err)
			}
			result.CursorSource = source

			if err := shared.PrintOutput(result, *output.Output, *output.Pretty); err != nil {
				return err
			}
			if *writeState {
				if err := writeReviewDigestState(path, result.Cursor); err != nil {
					return fmt.Errorf("reviews digest: write --state-file: %w", err)
				}
			}
			return nil
		},
	}
}

func parseReviewDigestSince(value string) (time.Time, error) {
	trimmed := strings.TrimSpace(value)
	if parsed, ok := shared.ParseRFC3339Date(trimmed); ok {
		return parsed, nil
	}
	if parsed, err := time.Parse(time.DateOnly, trimmed); err == nil {
		return parsed, nil
	}
	return time.Time{}, fmt.Errorf("--since must be an RFC3339 timestamp or YYYY-MM-DD date")
}

func resolveReviewDigestCursor(path string, since time.Time) (time.Time, string, error) {
	if path != "" {
		data, exists, err := readReviewDigestState(path)
		if err != nil {
			return time.Time{}, "", fmt.Errorf("reviews digest: read --state-file: %w", err)
		}
		if exists {
			var state reviewDigestState
			if err := json.Unmarshal(data, &state); err != nil {
				return time.Time{}, "", fmt.Errorf("reviews digest: --state-file %q is not valid JSON: %w", path, err)
			}
			cursor, ok := shared.ParseRFC3339Date(state.Cursor)
			if !ok {
				return time.Time{}, "", fmt.Errorf("reviews digest: --state-file %q has no valid cursor", path)
			}
			return cursor, "stateFile", nil
		}
		if since.IsZero() {
			return time.Time{}, "", shared.WithDiagnostic(shared.UsageError("--since is required when --state-file does not exist"), shared.DiagnosticRequiredInputMissing, "--since")
		}
	}
	if since.IsZero() {
		return time.Time{}, "", shared.WithDiagnostic(shared.UsageError("--since or --state-file is required"), shared.DiagnosticRequiredInputMissing, "--since")
	}
	return since, "since", nil
}

func reviewDigestStateRoot(path string) (rootfs.Root, string, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return rootfs.Root{}, "", err
	}
	root, err := rootfs.New(filepath.Dir(absolute))
	if err != nil {
		return rootfs.Root{}, "", err
	}
	return root, filepath.Base(absolute), nil
}

func readReviewDigestState(path string) ([]byte, bool, error) {
	root, name, err := reviewDigestStateRoot(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, false, nil
		}
		return nil, false, err
	}
	defer root.Close()
	data, err := root.ReadFileLimited(name, reviewDigestStateFileLimit)
	if errors.Is(err, os.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return data, true, nil
}

func writeReviewDigestState(path, cursor string) error {
	data, err := json.Marshal(reviewDigestState{Cursor: cursor})
	if err != nil {
		return err
	}
	root, name, err := reviewDigestStateRoot(path)
	if err != nil {
		return err
	}
	defer root.Close()
	return root.WriteFile(name, append(data, '\n'), 0o600)
}

func buildReviewDigest(ctx context.Context, client *asc.Client, appID string, cursor time.Time, lowest int) (*asc.CustomerReviewDigestResult, error) {
	firstPage, err := client.GetReviews(ctx, appID,
		asc.WithReviewSort("-createdDate"),
		asc.WithReviewIncludeResponse(),
		asc.Limit[asc.ReviewOption](200),
	)
	if err != nil {
		return nil, err
	}

	since := cursor.Format(time.RFC3339Nano)
	result := &asc.CustomerReviewDigestResult{
		AppID:       appID,
		Since:       since,
		Cursor:      since,
		Territories: []asc.CustomerReviewDigestGroup{},
		LowestRated: []asc.CustomerReviewDigestReview{},
	}
	newest := cursor
	ratingCounts := make([]int, 6)
	ratingTotal := 0
	territoryIndex := map[string]int{}
	territorySums := []int{}
	reviews := []asc.CustomerReviewDigestReview{}
	seenReviewIDs := map[string]struct{}{}

	consume := func(page asc.PaginatedResponse) error {
		response, ok := page.(*asc.ReviewsResponse)
		if !ok {
			return fmt.Errorf("unexpected response type %T", page)
		}
		for _, item := range response.Data {
			if _, seen := seenReviewIDs[item.ID]; seen {
				continue
			}
			seenReviewIDs[item.ID] = struct{}{}
			created, ok := shared.ParseRFC3339Date(item.Attributes.CreatedDate)
			if !ok {
				return fmt.Errorf("review %q has invalid createdDate %q", item.ID, item.Attributes.CreatedDate)
			}
			if !created.After(cursor) {
				return errReviewDigestReachedCursor
			}
			if created.After(newest) {
				newest = created
				result.Cursor = item.Attributes.CreatedDate
			}

			answered := false
			if len(item.Relationships) > 0 {
				var relationships reviewDigestRelationships
				if err := json.Unmarshal(item.Relationships, &relationships); err != nil {
					return fmt.Errorf("review %q has invalid relationships: %w", item.ID, err)
				}
				answered = relationships.Response != nil && relationships.Response.Data != nil
			}

			rating := item.Attributes.Rating
			result.Total++
			ratingTotal += rating
			if rating >= 1 && rating <= 5 {
				ratingCounts[rating]++
			}
			if !answered {
				result.Unanswered++
			}

			territory := item.Attributes.Territory
			index, seen := territoryIndex[territory]
			if !seen {
				index = len(result.Territories)
				territoryIndex[territory] = index
				result.Territories = append(result.Territories, asc.CustomerReviewDigestGroup{Territory: territory})
				territorySums = append(territorySums, 0)
			}
			group := &result.Territories[index]
			group.Count++
			territorySums[index] += rating
			if !answered {
				group.Unanswered++
			}

			reviews = append(reviews, asc.CustomerReviewDigestReview{
				ID:          item.ID,
				Rating:      rating,
				Title:       item.Attributes.Title,
				Body:        item.Attributes.Body,
				Territory:   territory,
				CreatedDate: item.Attributes.CreatedDate,
				Answered:    answered,
			})
		}
		return nil
	}
	fetchNext := func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
		return client.GetReviews(ctx, appID, asc.NextURL[asc.ReviewOption](nextURL))
	}
	if err := asc.PaginateEach(ctx, firstPage, fetchNext, consume); err != nil && !errors.Is(err, errReviewDigestReachedCursor) {
		return nil, err
	}

	result.Versions, result.UnassignedVersionReviews, err = collectReviewDigestVersions(ctx, client, appID, cursor, reviews)
	if err != nil {
		return nil, fmt.Errorf("version groups: %w", err)
	}

	result.AverageRating = roundReviewDigestAverage(ratingTotal, result.Total)
	for rating := 5; rating >= 1; rating-- {
		result.Ratings = append(result.Ratings, asc.CustomerReviewDigestRating{Rating: rating, Count: ratingCounts[rating]})
	}
	for index := range result.Territories {
		result.Territories[index].AverageRating = roundReviewDigestAverage(territorySums[index], result.Territories[index].Count)
	}
	sort.SliceStable(result.Territories, func(i, j int) bool {
		if result.Territories[i].Count != result.Territories[j].Count {
			return result.Territories[i].Count > result.Territories[j].Count
		}
		return result.Territories[i].Territory < result.Territories[j].Territory
	})
	sort.SliceStable(reviews, func(i, j int) bool {
		return reviews[i].Rating < reviews[j].Rating
	})
	if len(reviews) > lowest {
		reviews = reviews[:lowest]
	}
	result.LowestRated = reviews
	return result, nil
}

func collectReviewDigestVersions(ctx context.Context, client *asc.Client, appID string, cursor time.Time, reviews []asc.CustomerReviewDigestReview) ([]asc.CustomerReviewDigestVersion, int, error) {
	groups := []asc.CustomerReviewDigestVersion{}
	if len(reviews) == 0 {
		return groups, 0, nil
	}
	byID := make(map[string]asc.CustomerReviewDigestReview, len(reviews))
	assigned := make(map[string]bool, len(reviews))
	for _, review := range reviews {
		byID[review.ID] = review
	}
	first, err := client.GetAppStoreVersions(ctx, appID, asc.Limit[asc.AppStoreVersionsOption](200))
	if err != nil {
		return nil, 0, err
	}
	consumeVersions := func(page asc.PaginatedResponse) error {
		response, ok := page.(*asc.AppStoreVersionsResponse)
		if !ok {
			return fmt.Errorf("unexpected versions response type %T", page)
		}
		for _, version := range response.Data {
			firstReviews, err := client.GetAppStoreVersionCustomerReviews(ctx, version.ID, asc.WithReviewSort("-createdDate"), asc.Limit[asc.ReviewOption](200))
			if err != nil {
				return fmt.Errorf("version %q: %w", version.ID, err)
			}
			group := asc.CustomerReviewDigestVersion{VersionID: version.ID, Version: version.Attributes.VersionString, Platform: string(version.Attributes.Platform)}
			ratingSum := 0
			consumeReviews := func(page asc.PaginatedResponse) error {
				response, ok := page.(*asc.ReviewsResponse)
				if !ok {
					return fmt.Errorf("unexpected reviews response type %T", page)
				}
				for _, item := range response.Data {
					created, ok := shared.ParseRFC3339Date(item.Attributes.CreatedDate)
					if !ok {
						return fmt.Errorf("review %q has invalid createdDate %q", item.ID, item.Attributes.CreatedDate)
					}
					if !created.After(cursor) {
						return errReviewDigestReachedCursor
					}
					if review, included := byID[item.ID]; included && !assigned[item.ID] {
						assigned[item.ID] = true
						group.Count++
						ratingSum += review.Rating
						if !review.Answered {
							group.Unanswered++
						}
						if len(assigned) == len(byID) {
							return errReviewDigestVersionsComplete
						}
					}
				}
				return nil
			}
			fetchReviews := func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
				return client.GetAppStoreVersionCustomerReviews(ctx, version.ID, asc.NextURL[asc.ReviewOption](nextURL))
			}
			pageErr := asc.PaginateEach(ctx, firstReviews, fetchReviews, consumeReviews)
			if pageErr != nil && !errors.Is(pageErr, errReviewDigestReachedCursor) && !errors.Is(pageErr, errReviewDigestVersionsComplete) {
				return fmt.Errorf("version %q: %w", version.ID, pageErr)
			}
			if group.Count > 0 {
				group.AverageRating = roundReviewDigestAverage(ratingSum, group.Count)
				groups = append(groups, group)
			}
			if errors.Is(pageErr, errReviewDigestVersionsComplete) {
				return pageErr
			}
		}
		return nil
	}
	fetchVersions := func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
		return client.GetAppStoreVersions(ctx, appID, asc.NextURL[asc.AppStoreVersionsOption](nextURL))
	}
	if err := asc.PaginateEach(ctx, first, fetchVersions, consumeVersions); err != nil && !errors.Is(err, errReviewDigestVersionsComplete) {
		return nil, 0, err
	}
	sort.Slice(groups, func(i, j int) bool {
		if groups[i].Count != groups[j].Count {
			return groups[i].Count > groups[j].Count
		}
		if groups[i].Platform != groups[j].Platform {
			return groups[i].Platform < groups[j].Platform
		}
		if groups[i].Version != groups[j].Version {
			return groups[i].Version < groups[j].Version
		}
		return groups[i].VersionID < groups[j].VersionID
	})
	unassigned := 0
	for _, review := range reviews {
		if !assigned[review.ID] {
			unassigned++
		}
	}
	return groups, unassigned, nil
}

func roundReviewDigestAverage(sum, count int) float64 {
	if count == 0 {
		return 0
	}
	return math.Round(float64(sum)/float64(count)*100) / 100
}
