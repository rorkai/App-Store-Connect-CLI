package apps

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/itunes"
)

var newPublicItunesClient = itunes.NewClient

type publicAppPrice struct {
	AppID          int64   `json:"appId"`
	Name           string  `json:"name"`
	Country        string  `json:"country"`
	CountryName    string  `json:"countryName"`
	Price          float64 `json:"price"`
	FormattedPrice string  `json:"formattedPrice"`
	Currency       string  `json:"currency"`
	IsFree         bool    `json:"isFree"`
}

type publicAppDescription struct {
	AppID       int64  `json:"appId"`
	Name        string `json:"name"`
	Country     string `json:"country"`
	CountryName string `json:"countryName"`
	Version     string `json:"version"`
	Description string `json:"description"`
}

type publicSearchOutput struct {
	Term    string                `json:"term"`
	Country string                `json:"country"`
	Limit   int                   `json:"limit"`
	Results []itunes.SearchResult `json:"results"`
}

// AppsPublicCommand returns the public App Store subtree.
func AppsPublicCommand() *ffcli.Command {
	fs := flag.NewFlagSet("apps public", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "public",
		ShortUsage: "asc apps public <subcommand> [flags]",
		ShortHelp:  "Inspect public App Store storefront data.",
		LongHelp: `Inspect public App Store storefront data.

No authentication is required.

Examples:
  asc apps public view --app "1479784361"
  asc apps public search --term "focus" --country us
  asc apps public prices --app "1479784361"
  asc apps public descriptions --app "1479784361"
  asc apps public storefronts list`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			AppsPublicViewCommand(),
			AppsPublicSearchCommand(),
			AppsPublicPricesCommand(),
			AppsPublicDescriptionsCommand(),
			AppsPublicStorefrontsCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// AppsPublicViewCommand returns the public metadata view command.
func AppsPublicViewCommand() *ffcli.Command {
	fs := flag.NewFlagSet("apps public view", flag.ExitOnError)

	appID := fs.String("app", "", "Public App Store app ID")
	legacyAppID := fs.String("id", "", "Deprecated alias for --app")
	country := fs.String("country", "us", "Storefront country code (ISO alpha-2, e.g. us, gb, de)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "view",
		ShortUsage: "asc apps public view --app APP_ID [--country CODE]",
		ShortHelp:  "View public App Store metadata for an app.",
		LongHelp: `View public App Store metadata for a public App Store app.

No authentication is required.

Examples:
  asc apps public view --app "1479784361"
  asc apps public view --app "1479784361" --country de
  asc apps public view --id "1479784361" --output markdown`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				fmt.Fprintln(os.Stderr, "Error: public view does not accept positional arguments")
				return flag.ErrHelp
			}

			resolvedAppID, err := resolvePublicAppID(*appID, *legacyAppID)
			if err != nil {
				fmt.Fprintln(os.Stderr, "Error: "+err.Error())
				return flag.ErrHelp
			}

			normalizedCountry, err := normalizePublicCountry(*country)
			if err != nil {
				fmt.Fprintln(os.Stderr, "Error: "+err.Error())
				return flag.ErrHelp
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			client := newPublicItunesClient()
			app, err := client.LookupApp(requestCtx, resolvedAppID, itunes.LookupOptions{
				Country:               normalizedCountry,
				IncludeSoftwareEntity: true,
			})
			if err != nil {
				return fmt.Errorf("apps public view: %w", err)
			}

			return shared.PrintOutputWithRenderers(app, *output.Output, *output.Pretty, func() error {
				return renderPublicAppFieldTable(appFields(app))
			}, func() error {
				return renderPublicAppFieldMarkdown(appFields(app))
			})
		},
	}
}

// AppsPublicSearchCommand returns the public search command.
func AppsPublicSearchCommand() *ffcli.Command {
	fs := flag.NewFlagSet("apps public search", flag.ExitOnError)

	term := fs.String("term", "", "Search term")
	country := fs.String("country", "us", "Storefront country code (ISO alpha-2, e.g. us, gb, de)")
	limit := fs.Int("limit", 20, "Maximum results (1-200)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "search",
		ShortUsage: "asc apps public search --term QUERY [--country CODE] [--limit 20]",
		ShortHelp:  "Search public App Store storefronts.",
		LongHelp: `Search public App Store storefronts.

No authentication is required.

Examples:
  asc apps public search --term "focus" --country us
  asc apps public search --term "focus" --country de --limit 10
  asc apps public search --term "habit tracker" --output table`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				fmt.Fprintln(os.Stderr, "Error: public search does not accept positional arguments")
				return flag.ErrHelp
			}
			termValue := strings.TrimSpace(*term)
			if termValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --term is required")
				return flag.ErrHelp
			}
			if *limit < 1 || *limit > 200 {
				fmt.Fprintln(os.Stderr, "Error: --limit must be between 1 and 200")
				return flag.ErrHelp
			}

			normalizedCountry, err := normalizePublicCountry(*country)
			if err != nil {
				fmt.Fprintln(os.Stderr, "Error: "+err.Error())
				return flag.ErrHelp
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			client := newPublicItunesClient()
			results, err := client.SearchApps(requestCtx, termValue, normalizedCountry, *limit)
			if err != nil {
				return fmt.Errorf("apps public search: %w", err)
			}

			payload := publicSearchOutput{
				Term:    termValue,
				Country: strings.ToUpper(normalizedCountry),
				Limit:   *limit,
				Results: results,
			}

			return shared.PrintOutputWithRenderers(payload, *output.Output, *output.Pretty, func() error {
				return renderPublicSearchTable(payload)
			}, func() error {
				return renderPublicSearchMarkdown(payload)
			})
		},
	}
}

// AppsPublicPricesCommand returns the public price command.
func AppsPublicPricesCommand() *ffcli.Command {
	fs := flag.NewFlagSet("apps public prices", flag.ExitOnError)

	appID := fs.String("app", "", "Public App Store app ID")
	legacyAppID := fs.String("id", "", "Deprecated alias for --app")
	country := fs.String("country", "us", "Storefront country code (ISO alpha-2, e.g. us, gb, de)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "prices",
		ShortUsage: "asc apps public prices --app APP_ID [--country CODE]",
		ShortHelp:  "Show the public App Store price for an app.",
		LongHelp: `Show the public App Store price for a public App Store app.

No authentication is required.

Examples:
  asc apps public prices --app "1479784361"
  asc apps public prices --app "1479784361" --country jp
  asc apps public prices --id "1479784361" --output markdown`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				fmt.Fprintln(os.Stderr, "Error: public prices does not accept positional arguments")
				return flag.ErrHelp
			}

			resolvedAppID, err := resolvePublicAppID(*appID, *legacyAppID)
			if err != nil {
				fmt.Fprintln(os.Stderr, "Error: "+err.Error())
				return flag.ErrHelp
			}
			normalizedCountry, err := normalizePublicCountry(*country)
			if err != nil {
				fmt.Fprintln(os.Stderr, "Error: "+err.Error())
				return flag.ErrHelp
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			client := newPublicItunesClient()
			app, err := client.LookupApp(requestCtx, resolvedAppID, itunes.LookupOptions{
				Country:               normalizedCountry,
				IncludeSoftwareEntity: true,
			})
			if err != nil {
				return fmt.Errorf("apps public prices: %w", err)
			}

			payload := publicAppPrice{
				AppID:          app.AppID,
				Name:           app.Name,
				Country:        app.Country,
				CountryName:    app.CountryName,
				Price:          app.Price,
				FormattedPrice: app.FormattedPrice,
				Currency:       app.Currency,
				IsFree:         app.Price == 0,
			}

			return shared.PrintOutputWithRenderers(payload, *output.Output, *output.Pretty, func() error {
				return renderPublicAppFieldTable(appFieldsFromPrice(payload))
			}, func() error {
				return renderPublicAppFieldMarkdown(appFieldsFromPrice(payload))
			})
		},
	}
}

// AppsPublicDescriptionsCommand returns the public description command.
func AppsPublicDescriptionsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("apps public descriptions", flag.ExitOnError)

	appID := fs.String("app", "", "Public App Store app ID")
	legacyAppID := fs.String("id", "", "Deprecated alias for --app")
	country := fs.String("country", "us", "Storefront country code (ISO alpha-2, e.g. us, gb, de)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "descriptions",
		ShortUsage: "asc apps public descriptions --app APP_ID [--country CODE]",
		ShortHelp:  "Show the public App Store description for an app.",
		LongHelp: `Show the public App Store description for a public App Store app.

No authentication is required.

Examples:
  asc apps public descriptions --app "1479784361"
  asc apps public descriptions --app "1479784361" --country fr
  asc apps public descriptions --id "1479784361" --output table`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				fmt.Fprintln(os.Stderr, "Error: public descriptions does not accept positional arguments")
				return flag.ErrHelp
			}

			resolvedAppID, err := resolvePublicAppID(*appID, *legacyAppID)
			if err != nil {
				fmt.Fprintln(os.Stderr, "Error: "+err.Error())
				return flag.ErrHelp
			}
			normalizedCountry, err := normalizePublicCountry(*country)
			if err != nil {
				fmt.Fprintln(os.Stderr, "Error: "+err.Error())
				return flag.ErrHelp
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			client := newPublicItunesClient()
			app, err := client.LookupApp(requestCtx, resolvedAppID, itunes.LookupOptions{
				Country:               normalizedCountry,
				IncludeSoftwareEntity: true,
			})
			if err != nil {
				return fmt.Errorf("apps public descriptions: %w", err)
			}

			payload := publicAppDescription{
				AppID:       app.AppID,
				Name:        app.Name,
				Country:     app.Country,
				CountryName: app.CountryName,
				Version:     app.Version,
				Description: app.Description,
			}

			return shared.PrintOutputWithRenderers(payload, *output.Output, *output.Pretty, func() error {
				return renderPublicAppFieldTable(appFieldsFromDescription(payload))
			}, func() error {
				return renderPublicAppFieldMarkdown(appFieldsFromDescription(payload))
			})
		},
	}
}

// AppsPublicStorefrontsCommand returns the storefronts command group.
func AppsPublicStorefrontsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("apps public storefronts", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "storefronts",
		ShortUsage: "asc apps public storefronts <subcommand> [flags]",
		ShortHelp:  "Inspect App Store storefront metadata.",
		LongHelp: `Inspect public App Store storefront metadata.

No authentication is required.

Examples:
  asc apps public storefronts list
  asc apps public storefronts list --output markdown`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			AppsPublicStorefrontsListCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// AppsPublicStorefrontsListCommand returns the storefront list command.
func AppsPublicStorefrontsListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("apps public storefronts list", flag.ExitOnError)
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc apps public storefronts list [flags]",
		ShortHelp:  "List public App Store storefronts.",
		LongHelp: `List the public App Store storefronts.

No authentication is required.

Examples:
  asc apps public storefronts list
  asc apps public storefronts list --output table`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				fmt.Fprintln(os.Stderr, "Error: public storefronts list does not accept positional arguments")
				return flag.ErrHelp
			}

			storefronts := itunes.ListStorefronts()
			return shared.PrintOutputWithRenderers(storefronts, *output.Output, *output.Pretty, func() error {
				return renderPublicStorefrontsTable(storefronts)
			}, func() error {
				return renderPublicStorefrontsMarkdown(storefronts)
			})
		},
	}
}

func resolvePublicAppID(appID, alias string) (string, error) {
	appID = strings.TrimSpace(appID)
	alias = strings.TrimSpace(alias)
	if appID == "" && alias == "" {
		return "", fmt.Errorf("--app is required")
	}
	if appID != "" && alias != "" && appID != alias {
		return "", fmt.Errorf("--app and --id are mutually exclusive")
	}
	if appID != "" {
		if err := validatePublicAppID(appID); err != nil {
			return "", fmt.Errorf("--app must be a numeric App Store app ID")
		}
		return appID, nil
	}
	if err := validatePublicAppID(alias); err != nil {
		return "", fmt.Errorf("--app must be a numeric App Store app ID")
	}
	return alias, nil
}

func validatePublicAppID(value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return fmt.Errorf("app ID is required")
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return fmt.Errorf("app ID must contain only digits")
		}
	}
	if _, err := strconv.ParseInt(value, 10, 64); err != nil {
		return err
	}
	return nil
}

func normalizePublicCountry(country string) (string, error) {
	normalized, err := itunes.NormalizeCountryCode(country)
	if err != nil {
		return "", err
	}
	if normalized == "" {
		normalized = "us"
	}
	return normalized, nil
}

func appFields(app *itunes.App) [][]string {
	if app == nil {
		return nil
	}

	return [][]string{
		{"App ID", strconv.FormatInt(app.AppID, 10)},
		{"Name", app.Name},
		{"Bundle ID", app.BundleID},
		{"Country", app.Country},
		{"Country Name", app.CountryName},
		{"URL", app.URL},
		{"Artwork URL", app.ArtworkURL},
		{"Seller Name", app.SellerName},
		{"Primary Genre Name", app.PrimaryGenreName},
		{"Genres", strings.Join(app.Genres, ", ")},
		{"Version", app.Version},
		{"Description", app.Description},
		{"Price", fmt.Sprintf("%.2f", app.Price)},
		{"Formatted Price", app.FormattedPrice},
		{"Currency", app.Currency},
		{"Average Rating", fmt.Sprintf("%.2f", app.AverageRating)},
		{"Rating Count", strconv.FormatInt(app.RatingCount, 10)},
		{"Current Version Rating", fmt.Sprintf("%.2f", app.CurrentVersionRating)},
		{"Current Version Count", strconv.FormatInt(app.CurrentVersionCount, 10)},
	}
}

func appFieldsFromPrice(app publicAppPrice) [][]string {
	return [][]string{
		{"App ID", strconv.FormatInt(app.AppID, 10)},
		{"Name", app.Name},
		{"Country", app.Country},
		{"Country Name", app.CountryName},
		{"Price", fmt.Sprintf("%.2f", app.Price)},
		{"Formatted Price", app.FormattedPrice},
		{"Currency", app.Currency},
		{"Is Free", strconv.FormatBool(app.IsFree)},
	}
}

func appFieldsFromDescription(app publicAppDescription) [][]string {
	return [][]string{
		{"App ID", strconv.FormatInt(app.AppID, 10)},
		{"Name", app.Name},
		{"Country", app.Country},
		{"Country Name", app.CountryName},
		{"Version", app.Version},
		{"Description", app.Description},
	}
}

func renderPublicAppFieldTable(rows [][]string) error {
	asc.RenderTable([]string{"Field", "Value"}, rows)
	return nil
}

func renderPublicAppFieldMarkdown(rows [][]string) error {
	asc.RenderMarkdown([]string{"Field", "Value"}, rows)
	return nil
}

func renderPublicSearchTable(payload publicSearchOutput) error {
	fmt.Printf("\nTerm: %s\n", payload.Term)
	fmt.Printf("Country: %s\n", payload.Country)
	fmt.Printf("Limit: %d\n\n", payload.Limit)

	headers := []string{
		"App ID",
		"Name",
		"Bundle ID",
		"Seller Name",
		"Country",
		"Country Name",
		"URL",
		"Artwork URL",
		"Primary Genre Name",
		"Formatted Price",
		"Currency",
		"Average Rating",
		"Rating Count",
	}
	rows := make([][]string, 0, len(payload.Results))
	for _, result := range payload.Results {
		rows = append(rows, searchResultRows(result))
	}
	asc.RenderTable(headers, rows)
	return nil
}

func renderPublicSearchMarkdown(payload publicSearchOutput) error {
	fmt.Printf("## Search Results\n\n")
	asc.RenderMarkdown([]string{"Field", "Value"}, [][]string{
		{"Term", payload.Term},
		{"Country", payload.Country},
		{"Limit", strconv.Itoa(payload.Limit)},
	})
	fmt.Println()

	headers := []string{
		"App ID",
		"Name",
		"Bundle ID",
		"Seller Name",
		"Country",
		"Country Name",
		"URL",
		"Artwork URL",
		"Primary Genre Name",
		"Formatted Price",
		"Currency",
		"Average Rating",
		"Rating Count",
	}
	rows := make([][]string, 0, len(payload.Results))
	for _, result := range payload.Results {
		rows = append(rows, searchResultRows(result))
	}
	asc.RenderMarkdown(headers, rows)
	return nil
}

func searchResultRows(result itunes.SearchResult) []string {
	return []string{
		strconv.FormatInt(result.AppID, 10),
		result.Name,
		result.BundleID,
		result.SellerName,
		result.Country,
		result.CountryName,
		result.URL,
		result.ArtworkURL,
		result.PrimaryGenreName,
		result.FormattedPrice,
		result.Currency,
		fmt.Sprintf("%.2f", result.AverageRating),
		strconv.FormatInt(result.RatingCount, 10),
	}
}

func renderPublicStorefrontsTable(storefronts []itunes.Storefront) error {
	headers := []string{"Country", "Country Name", "Storefront ID"}
	rows := make([][]string, 0, len(storefronts))
	for _, storefront := range storefronts {
		rows = append(rows, []string{storefront.Country, storefront.CountryName, storefront.StorefrontID})
	}
	asc.RenderTable(headers, rows)
	return nil
}

func renderPublicStorefrontsMarkdown(storefronts []itunes.Storefront) error {
	headers := []string{"Country", "Country Name", "Storefront ID"}
	rows := make([][]string, 0, len(storefronts))
	for _, storefront := range storefronts {
		rows = append(rows, []string{storefront.Country, storefront.CountryName, storefront.StorefrontID})
	}
	asc.RenderMarkdown(headers, rows)
	return nil
}
