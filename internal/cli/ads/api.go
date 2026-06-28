package ads

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// APICommand returns the raw Apple Ads API request command.
func APICommand() *ffcli.Command {
	fs := flag.NewFlagSet("ads api", flag.ExitOnError)
	return &ffcli.Command{
		Name:       "api",
		ShortUsage: "asc ads api <subcommand> [flags]",
		ShortHelp:  "Make raw Apple Ads API requests.",
		LongHelp: `Make raw Apple Ads API requests.

Examples:
  asc ads api request --method GET --path v5/campaigns --org "123456"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			APIRequestCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

func APIRequestCommand() *ffcli.Command {
	fs := flag.NewFlagSet("ads api request", flag.ExitOnError)
	method := fs.String("method", "GET", "HTTP method: GET, POST, PUT, DELETE")
	path := fs.String("path", "", "Relative v5 path or Apple Ads API URL")
	file := fs.String("file", "", "Path to JSON request payload")
	confirm := fs.Bool("confirm", false, "Confirm destructive DELETE requests")
	common := commonFlags{
		AdsProfile: fs.String("ads-profile", "", "Use named Apple Ads authentication profile"),
		Org:        fs.String("org", "", "Apple Ads organization ID (or ASC_ADS_ORG_ID env)"),
	}
	output := shared.BindOutputFlags(fs)
	return &ffcli.Command{
		Name:       "request",
		ShortUsage: "asc ads api request --method METHOD --path v5/... [flags]",
		ShortHelp:  "Make a raw Apple Ads API request.",
		LongHelp: `Make a raw Apple Ads API request.

Examples:
  asc ads api request --method GET --path v5/campaigns --org "123456"
  asc ads api request --method POST --path v5/campaigns/find --file selector.json --org "123456"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := rejectUnexpectedArgs(args); err != nil {
				return err
			}
			methodValue := strings.ToUpper(strings.TrimSpace(*method))
			switch methodValue {
			case http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete:
			default:
				return shared.UsageError("--method must be one of: GET, POST, PUT, DELETE")
			}
			pathValue := strings.TrimSpace(*path)
			if pathValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --path is required")
				return shared.MissingRequiredUsageError()
			}
			if methodValue == http.MethodDelete && !*confirm {
				return shared.UsageError("--confirm is required")
			}
			requiresOrg, err := rawRequestRequiresOrg(pathValue)
			if err != nil {
				return shared.UsageError(err.Error())
			}
			var payload json.RawMessage
			if strings.TrimSpace(*file) != "" {
				payload, err = shared.ReadJSONFilePayloadKind(*file, shared.JSONPayloadAny)
				if err != nil {
					return fmt.Errorf("ads api request: %w", err)
				}
			}
			client, err := resolveClient(ctx, common, requiresOrg)
			if err != nil {
				return fmt.Errorf("ads api request: %w", err)
			}
			requestCtx, cancel := requestContext(ctx)
			defer cancel()
			resp, err := client.Request(requestCtx, methodValue, pathValue, nil, payload, requiresOrg)
			if err != nil {
				return fmt.Errorf("ads api request: %w", err)
			}
			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

func rawRequestRequiresOrg(pathValue string) (bool, error) {
	trimmed := strings.TrimSpace(pathValue)
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return false, fmt.Errorf("--path must be a valid URL or v5 path: %w", err)
	}
	if parsed.IsAbs() {
		if parsed.Scheme != "https" || parsed.Host != "api.searchads.apple.com" || !strings.HasPrefix(parsed.Path, "/api/v5/") {
			return false, fmt.Errorf("--path must be an Apple Ads v5 URL")
		}
		pathOnly := strings.TrimPrefix(parsed.Path, "/api/")
		return rawPathRequiresOrg(pathOnly)
	}
	pathOnly := strings.TrimPrefix(parsed.Path, "/")
	if pathOnly == "" {
		pathOnly = strings.TrimPrefix(trimmed, "/")
	}
	return rawPathRequiresOrg(pathOnly)
}

func rawPathRequiresOrg(pathOnly string) (bool, error) {
	if !strings.HasPrefix(pathOnly, "v5/") {
		return false, fmt.Errorf("--path must start with v5/")
	}
	if strings.Contains(pathOnly, "..") {
		return false, fmt.Errorf("--path must not contain path traversal")
	}
	return pathOnly != "v5/me" && pathOnly != "v5/acls", nil
}
