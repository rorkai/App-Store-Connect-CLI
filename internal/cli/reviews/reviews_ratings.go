package reviews

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared/errfmt"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/itunes"
)

const ratingsAppSearchLimit = 200

type ratingsBundleLookupError struct {
	cause error
}

func (e *ratingsBundleLookupError) Error() string {
	return "could not resolve --app by bundle ID; pass a numeric App Store ID or try again later"
}

func (e *ratingsBundleLookupError) Unwrap() error {
	return e.cause
}

func newReviewsRatingsCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("ratings", flag.ExitOnError)

	appID := fs.String("app", "", "App Store app ID, exact bundle ID, or exact app name (required)")
	country := fs.String("country", "us", "Country code (e.g., us, gb, de)")
	all := fs.Bool("all", false, "Fetch ratings from all countries")
	workers := fs.Int("workers", 10, "Number of parallel workers for --all")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "ratings",
		ShortUsage: "asc reviews ratings [flags]",
		ShortHelp:  "Show App Store rating statistics.",
		LongHelp: `Show App Store rating statistics using the public iTunes API.

This command fetches aggregate rating data (average rating, rating count,
histogram) that is not available through the App Store Connect API.

No authentication is required.

Examples:
  asc reviews ratings --app "1479784361"
  asc reviews ratings --app "com.example.app"
  asc reviews ratings --app "1479784361" --country de
  asc reviews ratings --app "1479784361" --output table
  asc reviews ratings --app "1479784361" --all
  asc reviews ratings --app "1479784361" --all --workers 20`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			app := d.ResolveAppID(*appID)
			if strings.TrimSpace(app) == "" {
				fmt.Fprintln(d.IO.ErrOut, "Error: --app is required (or set ASC_APP_ID)")
				return shared.MissingRequiredUsageError("--app")
			}

			if *workers < 1 {
				fmt.Fprintln(d.IO.ErrOut, "Error: --workers must be at least 1")
				return shared.WithDiagnostic(flag.ErrHelp, shared.DiagnosticInvalidInput, "--workers")
			}
			if !*all {
				if _, err := itunes.NormalizeCountryCode(*country); err != nil {
					fmt.Fprintln(d.IO.ErrOut, "Error: "+err.Error())
					return shared.WithDiagnostic(flag.ErrHelp, shared.DiagnosticInvalidInput, "--country")
				}
			}

			return executeRatings(ctx, d, app, *country, *all, *workers, *output.Output, *output.Pretty)
		},
	}
}

func executeRatings(ctx context.Context, d *shared.Deps, appID, country string, all bool, workers int, output string, pretty bool) error {
	client := itunes.NewClient()
	client.HTTPClient = &http.Client{Transport: d.HTTPTransport()}
	return executeRatingsWithClient(ctx, d, client, appID, country, all, workers, output, pretty)
}

func executeRatingsWithClient(ctx context.Context, d *shared.Deps, client *itunes.Client, appID, country string, all bool, workers int, output string, pretty bool) error {
	format, err := normalizeRatingsOutput(output, pretty)
	if err != nil {
		return err
	}

	resolveCtx, resolveCancel := d.Timeout(ctx)
	resolvedAppID, err := resolveRatingsAppID(resolveCtx, client, appID, ratingsAppLookupCountry(country, all))
	resolveCancel()
	if err != nil {
		return fmt.Errorf("reviews ratings: %w", err)
	}

	if all {
		return executeAllRatings(ctx, d, client, resolvedAppID, workers, format, pretty)
	}

	countryCtx, countryCancel := d.Timeout(ctx)
	defer countryCancel()
	return executeSingleRatings(countryCtx, d, client, resolvedAppID, country, format, pretty)
}

func ratingsAppLookupCountry(country string, all bool) string {
	if all {
		return "us"
	}
	return country
}

func resolveRatingsAppID(ctx context.Context, client *itunes.Client, app string, country string) (string, error) {
	app = strings.TrimSpace(app)
	if app == "" {
		return "", fmt.Errorf("app ID is required")
	}
	if parsed, err := strconv.ParseInt(app, 10, 64); err == nil {
		return strconv.FormatInt(parsed, 10), nil
	}
	if looksLikeRatingsBundleID(app) {
		result, err := client.LookupAppByBundleID(ctx, app, itunes.LookupOptions{
			Country:               country,
			IncludeSoftwareEntity: true,
		})
		if err != nil {
			return "", &ratingsBundleLookupError{cause: err}
		}
		if result != nil && result.AppID != 0 {
			return strconv.FormatInt(result.AppID, 10), nil
		}
	}

	results, err := client.SearchApps(ctx, app, country, ratingsAppSearchLimit)
	if err != nil {
		return "", err
	}

	if resolved, ok := uniqueExactRatingsAppMatch(results, app, func(result itunes.SearchResult) string {
		return result.BundleID
	}); ok {
		return resolved, nil
	}
	if resolved, ok := uniqueExactRatingsAppMatch(results, app, func(result itunes.SearchResult) string {
		return result.Name
	}); ok {
		return resolved, nil
	}

	return "", shared.NewErrorWithCause(errors.New("could not resolve --app; pass a numeric App Store ID, exact bundle ID, or exact app name"), asc.ErrNotFound)
}

func looksLikeRatingsBundleID(app string) bool {
	app = strings.TrimSpace(app)
	return strings.Contains(app, ".") && !strings.ContainsAny(app, " \t\r\n")
}

func uniqueExactRatingsAppMatch(results []itunes.SearchResult, app string, value func(itunes.SearchResult) string) (string, bool) {
	var resolved string
	for _, result := range results {
		if result.AppID == 0 || !strings.EqualFold(strings.TrimSpace(value(result)), app) {
			continue
		}
		appID := strconv.FormatInt(result.AppID, 10)
		if resolved != "" && resolved != appID {
			return "", false
		}
		resolved = appID
	}
	return resolved, resolved != ""
}

func executeSingleRatings(ctx context.Context, d *shared.Deps, client *itunes.Client, appID, country, output string, pretty bool) error {
	ratings, err := client.GetRatings(ctx, appID, country)
	if err != nil {
		err = fmt.Errorf("reviews ratings: %w", err)
		if !errors.Is(err, asc.ErrNotFound) {
			return err
		}
		fmt.Fprint(d.IO.ErrOut, errfmt.FormatStderr(err))
		fmt.Fprintf(d.IO.ErrOut, "Hint: the app must be released in the %s storefront; pass --country with another storefront or use --all to check every storefront.\n", strings.ToUpper(strings.TrimSpace(country)))
		return shared.WithDiagnostic(shared.NewStderrReportedError(err), shared.DiagnosticResourceNotFound, "--country")
	}
	return d.PrintOutputWithRenderers(
		ratings,
		output,
		pretty,
		func() error { return printRatingsTable(d, ratings) },
		func() error { return printRatingsMarkdown(d, ratings) },
	)
}

func executeAllRatings(ctx context.Context, d *shared.Deps, client *itunes.Client, appID string, workers int, output string, pretty bool) error {
	global, err := client.GetAllRatings(ctx, appID, workers, d.Timeout)
	if err != nil {
		return fmt.Errorf("reviews ratings: %w", err)
	}
	return d.PrintOutputWithRenderers(
		global,
		output,
		pretty,
		func() error { return printGlobalRatingsTable(d, global) },
		func() error { return printGlobalRatingsMarkdown(d, global) },
	)
}

func normalizeRatingsOutput(output string, pretty bool) (string, error) {
	return shared.ValidateOutputFormat(output, pretty)
}

func printRatingsTable(d *shared.Deps, r *itunes.AppRatings) error {
	fmt.Fprintf(d.IO.Out, "\n%s\n", r.AppName)
	fmt.Fprintf(d.IO.Out, "App ID: %d | Country: %s\n", r.AppID, r.Country)
	fmt.Fprintln(d.IO.Out, strings.Repeat("─", 40))

	fmt.Fprintf(d.IO.Out, "Rating: %.2f (%s ratings)\n", r.AverageRating, formatNumber(r.RatingCount))

	if r.CurrentVersionRating != r.AverageRating || r.CurrentVersionCount != r.RatingCount {
		fmt.Fprintf(d.IO.Out, "Current Version: %.2f (%s ratings)\n", r.CurrentVersionRating, formatNumber(r.CurrentVersionCount))
	}

	if len(r.Histogram) > 0 {
		printHistogram(d, r.Histogram)
	}
	fmt.Fprintln(d.IO.Out)
	return nil
}

func printGlobalRatingsTable(d *shared.Deps, g *itunes.GlobalRatings) error {
	fmt.Fprintf(d.IO.Out, "\n%s\n", g.AppName)
	fmt.Fprintf(d.IO.Out, "App ID: %d\n", g.AppID)
	fmt.Fprintln(d.IO.Out, strings.Repeat("─", 60))

	fmt.Fprintf(d.IO.Out, "GLOBAL: %.2f avg (%s total ratings across %d countries)\n",
		g.AverageRating, formatNumber(g.TotalCount), g.CountryCount)

	if len(g.Histogram) > 0 {
		fmt.Fprintln(d.IO.Out, "\nHistogram (Global):")
		printHistogramRows(d, g.Histogram)
	}

	fmt.Fprintln(d.IO.Out, strings.Repeat("─", 60))
	fmt.Fprintf(d.IO.Out, "\n%-20s %8s %8s\n", "Country", "Rating", "Count")
	fmt.Fprintln(d.IO.Out, strings.Repeat("─", 40))

	for _, r := range g.ByCountry {
		name := r.CountryName
		if name == "" {
			name = r.Country
		}
		if len(name) > 18 {
			name = name[:18] + ".."
		}
		fmt.Fprintf(d.IO.Out, "%-20s %8.2f %8s\n", name, r.AverageRating, formatNumber(r.RatingCount))
	}
	fmt.Fprintln(d.IO.Out)
	return nil
}

func printRatingsMarkdown(d *shared.Deps, r *itunes.AppRatings) error {
	fmt.Fprintf(d.IO.Out, "## %s\n\n", r.AppName)
	fmt.Fprintf(d.IO.Out, "**App ID:** %d | **Country:** %s\n\n", r.AppID, r.Country)
	fmt.Fprintf(d.IO.Out, "**Rating:** %.2f (%s ratings)\n\n", r.AverageRating, formatNumber(r.RatingCount))

	if len(r.Histogram) > 0 {
		fmt.Fprintln(d.IO.Out, "### Histogram")
		printHistogramMarkdown(d, r.Histogram)
	}
	fmt.Fprintln(d.IO.Out)
	return nil
}

func printGlobalRatingsMarkdown(d *shared.Deps, g *itunes.GlobalRatings) error {
	fmt.Fprintf(d.IO.Out, "## %s\n\n", g.AppName)
	fmt.Fprintf(d.IO.Out, "**App ID:** %d\n\n", g.AppID)
	fmt.Fprintf(d.IO.Out, "**Global Rating:** %.2f (%s total ratings across %d countries)\n\n",
		g.AverageRating, formatNumber(g.TotalCount), g.CountryCount)

	if len(g.Histogram) > 0 {
		fmt.Fprintln(d.IO.Out, "### Global Histogram")
		printHistogramMarkdown(d, g.Histogram)
	}

	fmt.Fprint(d.IO.Out, "\n### By Country\n\n")
	{
		headers := []string{"Country", "Rating", "Count"}
		rows := make([][]string, 0, len(g.ByCountry))
		for _, r := range g.ByCountry {
			name := r.CountryName
			if name == "" {
				name = r.Country
			}
			rows = append(rows, []string{name, fmt.Sprintf("%.2f", r.AverageRating), formatNumber(r.RatingCount)})
		}
		asc.RenderMarkdownTo(d.IO.Out, headers, rows)
	}
	fmt.Fprintln(d.IO.Out)
	return nil
}

func printHistogram(d *shared.Deps, histogram map[int]int64) {
	fmt.Fprintln(d.IO.Out, "\nHistogram:")
	printHistogramRows(d, histogram)
}

func printHistogramRows(d *shared.Deps, histogram map[int]int64) {
	var total int64
	for _, count := range histogram {
		total += count
	}

	headers := []string{"Stars", "Count", "Percentage", "Bar"}
	rows := make([][]string, 0, 5)
	for star := 5; star >= 1; star-- {
		count := histogram[star]
		pct := float64(0)
		if total > 0 {
			pct = float64(count) / float64(total) * 100
		}
		bar := strings.Repeat("█", int(pct/5))
		rows = append(rows, []string{
			fmt.Sprintf("%d★", star),
			formatNumber(count),
			fmt.Sprintf("%.1f%%", pct),
			bar,
		})
	}
	asc.RenderTableTo(d.IO.Out, headers, rows)
}

func printHistogramMarkdown(d *shared.Deps, histogram map[int]int64) {
	var total int64
	for _, count := range histogram {
		total += count
	}
	headers := []string{"Stars", "Count", "Percentage"}
	rows := make([][]string, 0, 5)
	for star := 5; star >= 1; star-- {
		count := histogram[star]
		pct := float64(0)
		if total > 0 {
			pct = float64(count) / float64(total) * 100
		}
		rows = append(rows, []string{
			fmt.Sprintf("%d★", star),
			formatNumber(count),
			fmt.Sprintf("%.1f%%", pct),
		})
	}
	asc.RenderMarkdownTo(d.IO.Out, headers, rows)
}

func formatNumber(n int64) string {
	s := strconv.FormatInt(n, 10)
	if len(s) <= 3 {
		return s
	}
	var result strings.Builder
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			result.WriteRune(',')
		}
		result.WriteRune(c)
	}
	return result.String()
}
