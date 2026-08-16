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

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/appleads"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// APICommand returns the raw Apple Ads API request command.
func APICommand() *ffcli.Command {
	fs := flag.NewFlagSet("ads v5 api", flag.ExitOnError)
	return &ffcli.Command{
		Name:       "api",
		ShortUsage: "asc ads v5 api <subcommand> [flags]",
		ShortHelp:  "Make raw Campaign Management API v5 requests.",
		LongHelp: `Make raw Campaign Management API v5 requests.

Examples:
  asc ads v5 api request --method GET --path v5/campaigns --org "123456"`,
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
	fs := flag.NewFlagSet("ads v5 api request", flag.ExitOnError)
	method := fs.String("method", "GET", "HTTP method: GET, POST, PUT, DELETE")
	path := fs.String("path", "", "Relative v5 path or Apple Ads API URL")
	file := fs.String("file", "", "Path to JSON request payload ('-' reads stdin)")
	confirm := fs.Bool("confirm", false, "Confirm destructive or operationally risky v5 requests")
	common := commonFlags{
		AdsProfile: fs.String("ads-profile", "", "Use named Apple Ads authentication profile"),
		Org:        fs.String("org", "", "Apple Ads organization ID (or ASC_ADS_ORG_ID env)"),
	}
	output := bindAdsRawOutputFlags(fs)
	command := &ffcli.Command{
		Name:       "request",
		ShortUsage: "asc ads v5 api request --method METHOD --path v5/... [flags]",
		ShortHelp:  "Make a raw Campaign Management API v5 request.",
		LongHelp: `Make a raw Campaign Management API v5 request.

Examples:
  asc ads v5 api request --method GET --path v5/campaigns --org "123456"
  asc ads v5 api request --method POST --path v5/campaigns/find --file selector.json --org "123456"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := rejectUnexpectedArgs(args); err != nil {
				return err
			}
			outputFormat, err := validateAdsRawOutput(output)
			if err != nil {
				return shared.UsageError(err.Error())
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
			requiresOrg, err := rawRequestRequiresOrg(pathValue)
			if err != nil {
				return shared.UsageError(err.Error())
			}
			requiresConfirm, err := rawV5RequestRequiresConfirm(methodValue, pathValue)
			if err != nil {
				return shared.UsageError(err.Error())
			}
			if requiresConfirm && !*confirm {
				return shared.UsageError("--confirm is required")
			}
			var payload json.RawMessage
			if strings.TrimSpace(*file) != "" {
				payload, err = shared.ReadJSONFilePayloadKind(*file, shared.JSONPayloadAny)
				if err != nil {
					return fmt.Errorf("ads v5 api request: %w", err)
				}
			}
			client, err := resolveClient(ctx, common, requiresOrg)
			if err != nil {
				return fmt.Errorf("ads v5 api request: %w", err)
			}
			requestCtx, cancel := requestContext(ctx)
			defer cancel()
			resp, err := client.Request(requestCtx, methodValue, pathValue, nil, payload, requiresOrg)
			if err != nil {
				return fmt.Errorf("ads v5 api request: %w", err)
			}
			return shared.PrintOutput(resp, outputFormat, *output.Pretty)
		},
	}
	return markAdsLegacyCommandDeprecated(command, []string{"v5", "api", "request"}, adsLegacyMigration{
		kind:        adsLegacyBreaking,
		replacement: []string{"api", "request"},
	})
}

func rawRequestRequiresOrg(pathValue string) (bool, error) {
	pathOnly, err := v5PathOnly(pathValue)
	if err != nil {
		return false, err
	}
	return rawPathRequiresOrg(pathOnly)
}

func v5PathOnly(pathValue string) (string, error) {
	trimmed := strings.TrimSpace(pathValue)
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return "", fmt.Errorf("--path must be a valid URL or v5 path: %w", err)
	}
	if parsed.IsAbs() {
		if parsed.Scheme != "https" || parsed.User != nil || parsed.Host != "api.searchads.apple.com" || !strings.HasPrefix(parsed.Path, "/api/v5/") {
			return "", fmt.Errorf("--path must be an Apple Ads v5 URL")
		}
		pathOnly := strings.TrimPrefix(parsed.Path, "/api/")
		if err := validateV5RelativePath(pathOnly); err != nil {
			return "", err
		}
		return pathOnly, nil
	}
	pathOnly := strings.TrimPrefix(parsed.Path, "/")
	if pathOnly == "" {
		pathOnly = strings.TrimPrefix(trimmed, "/")
	}
	if err := validateV5RelativePath(pathOnly); err != nil {
		return "", err
	}
	return pathOnly, nil
}

func validateV5RelativePath(pathOnly string) error {
	if !strings.HasPrefix(pathOnly, "v5/") {
		return fmt.Errorf("--path must start with v5/")
	}
	if adsPathHasTraversal(pathOnly) {
		return fmt.Errorf("--path must not contain path traversal")
	}
	return nil
}

func rawPathRequiresOrg(pathOnly string) (bool, error) {
	return pathOnly != "v5/me" && pathOnly != "v5/acls", nil
}

func rawV5RequestRequiresConfirm(method, pathValue string) (bool, error) {
	pathOnly, err := v5PathOnly(pathValue)
	if err != nil {
		return false, err
	}
	method = strings.ToUpper(strings.TrimSpace(method))
	if method == http.MethodDelete {
		return true, nil
	}
	if spec, ok := rawV5EndpointSpec(method, pathOnly); ok {
		return spec.RequiresConfirm || spec.RiskConfirm, nil
	}
	return method == http.MethodPost || method == http.MethodPut, nil
}

func rawV5EndpointSpec(method, pathOnly string) (appleads.EndpointSpec, bool) {
	for _, spec := range appleads.EndpointSpecs() {
		if spec.Method == method && v5EndpointPathMatches(spec.Path, pathOnly) {
			return spec, true
		}
	}
	return appleads.EndpointSpec{}, false
}

func v5EndpointPathMatches(pattern, pathOnly string) bool {
	patternParts := strings.Split(pattern, "/")
	pathParts := strings.Split(pathOnly, "/")
	if len(patternParts) != len(pathParts) {
		return false
	}
	for index, part := range patternParts {
		if strings.HasPrefix(part, "{") && strings.HasSuffix(part, "}") {
			if strings.TrimSpace(pathParts[index]) == "" {
				return false
			}
			continue
		}
		if part != pathParts[index] {
			return false
		}
	}
	return true
}
