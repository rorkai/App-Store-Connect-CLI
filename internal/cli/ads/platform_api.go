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

// PlatformAPICommand returns the raw Apple Ads Platform API command group.
func PlatformAPICommand() *ffcli.Command {
	fs := flag.NewFlagSet("ads api", flag.ExitOnError)
	return &ffcli.Command{
		Name:        "api",
		ShortUsage:  "asc ads api <subcommand> [flags]",
		ShortHelp:   "Make raw Apple Ads Platform API v1 requests.",
		LongHelp:    "Make raw Apple Ads Platform API v1 requests.",
		FlagSet:     fs,
		UsageFunc:   shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{PlatformAPIRequestCommand()},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// PlatformAPIRequestCommand returns the raw Apple Ads Platform API request command.
func PlatformAPIRequestCommand() *ffcli.Command {
	fs := flag.NewFlagSet("ads api request", flag.ExitOnError)
	method := fs.String("method", "GET", "HTTP method: GET, POST, PUT, DELETE")
	path := fs.String("path", "", "Relative v1 path or Apple Ads Platform API URL")
	file := fs.String("file", "", "Path to JSON request payload ('-' reads stdin)")
	confirm := fs.Bool("confirm", false, confirmFlagUsage(appleads.EndpointSpec{RiskConfirm: true}))
	common := commonFlags{
		AdsProfile: fs.String("ads-profile", "", "Use named Apple Ads authentication profile"),
		AdAccount:  fs.String("ad-account", "", "Apple Ads ad account ID (or ASC_ADS_AD_ACCOUNT_ID env)"),
	}
	output := bindAdsRawOutputFlags(fs)
	return &ffcli.Command{
		Name:       "request",
		ShortUsage: "asc ads api request --method METHOD --path v1/... [flags]",
		ShortHelp:  "Make a raw Apple Ads Platform API v1 request.",
		LongHelp: `Make a raw Apple Ads Platform API v1 request.

Examples:
  asc ads api request --method GET --path v1/me
  asc ads api request --method POST --path v1/metadata/apps/supported-languages/query --file query.json --ad-account "123"`,
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
			if methodValue == http.MethodDelete && !*confirm {
				return shared.UsageError("--confirm is required")
			}
			contextKind, err := rawPlatformRequestContextKind(methodValue, pathValue)
			if err != nil {
				return shared.UsageError(err.Error())
			}
			if contextKind == appleads.ContextNone && value(common.AdAccount) != "" {
				return shared.UsageError("--ad-account is not supported for this context-free endpoint")
			}
			pathOnly, err := platformPathOnly(pathValue)
			if err != nil {
				return shared.UsageError(err.Error())
			}
			if message := rawPlatformRequestMultipartMessage(methodValue, pathOnly); message != "" {
				return shared.UsageError(message)
			}
			if rawPlatformRequestRequiresPrePayloadConfirmation(methodValue, pathOnly) && !*confirm {
				if message := rawPlatformRequestConfirmMessage(methodValue, pathOnly, nil); message != "" {
					return shared.UsageError(message)
				}
			}
			var payload json.RawMessage
			if strings.TrimSpace(*file) != "" {
				payload, err = shared.ReadJSONFilePayloadKind(*file, shared.JSONPayloadAny)
				if err != nil {
					return fmt.Errorf("ads api request: %w", err)
				}
			}
			if message := rawPlatformRequestConfirmMessage(methodValue, pathOnly, payload); message != "" && !*confirm {
				return shared.UsageError(message)
			}
			client, effectiveAdAccountID, err := resolvePlatformClientAndAdAccountID(ctx, common, contextKind)
			if err != nil {
				return fmt.Errorf("ads api request: %w", err)
			}
			if err := validateRawPlatformAdAccountPathID(methodValue, pathOnly, effectiveAdAccountID); err != nil {
				return err
			}
			requestCtx, cancel := requestContext(ctx)
			defer cancel()
			resp, err := requestRawPlatformEndpoint(requestCtx, client, methodValue, pathValue, payload, contextKind)
			if err != nil {
				return fmt.Errorf("ads api request: %w", err)
			}
			return shared.PrintOutput(resp, outputFormat, *output.Pretty)
		},
	}
}

func rawPlatformRequestMultipartMessage(method, pathOnly string) string {
	method = strings.ToUpper(strings.TrimSpace(method))
	if method != http.MethodPost {
		return ""
	}
	spec, ok := platformEndpointSpecForRequest(method, pathOnly)
	if !ok || spec.BodyKind != appleads.BodyMultipart {
		return ""
	}
	return "v1/assets/upload requires multipart/form-data; use `asc ads assets upload --file IMAGE --brand BRAND_ID --ad-account AD_ACCOUNT_ID` instead of the JSON raw request command"
}

func rawPlatformRequestRequiresConfirm(method, pathOnly string, payload json.RawMessage) bool {
	method = strings.ToUpper(strings.TrimSpace(method))
	if spec, ok := platformEndpointSpecForRequest(method, pathOnly); ok {
		if spec.RequiresConfirm {
			return true
		}
		if spec.RiskConfirm {
			if spec.RiskConfirmBodyField == "" {
				return true
			}
			if len(payload) == 0 {
				// A body-scoped exception is safe only when the caller supplied
				// the body that proves the exception. Missing or malformed bodies
				// must not turn a mutation into a confirmation-free request.
				return true
			}
			return riskConfirmationRequired(spec, payload)
		}
		if spec.ConfirmBodyField != "" && len(payload) > 0 {
			var object map[string]json.RawMessage
			if err := json.Unmarshal(payload, &object); err == nil {
				_, present := object[spec.ConfirmBodyField]
				return present
			}
		}
		return false
	}

	resourcePath := strings.TrimPrefix(pathOnly, "v1/")
	switch {
	case method == http.MethodPost:
		switch resourcePath {
		case "recommendations/daily-budgets/apply",
			"recommendations/daily-budgets/dismiss",
			"recommendations/target-cpas/apply",
			"recommendations/target-cpas/dismiss":
			return true
		}
	case method == http.MethodPut && isSingleChildPath(resourcePath, "ad-accounts"):
		var object map[string]json.RawMessage
		if err := json.Unmarshal(payload, &object); err == nil {
			_, hasDelegations := object["delegations"]
			return hasDelegations
		}
	}
	// Unknown mutations are conservatively treated as destructive. Known
	// endpoints should declare their metadata in PlatformEndpointSpecs so raw
	// requests and generated commands share the same safety contract.
	return method == http.MethodDelete || method == http.MethodPost || method == http.MethodPut
}

func rawPlatformRequestRequiresPrePayloadConfirmation(method, pathOnly string) bool {
	method = strings.ToUpper(strings.TrimSpace(method))
	if spec, ok := platformEndpointSpecForRequest(method, pathOnly); ok {
		if spec.RequiresConfirm {
			return true
		}
		return spec.RiskConfirm && spec.RiskConfirmBodyField == ""
	}
	return method == http.MethodDelete || method == http.MethodPost || method == http.MethodPut
}

func rawPlatformRequestConfirmMessage(method, pathOnly string, payload json.RawMessage) string {
	if !rawPlatformRequestRequiresConfirm(method, pathOnly, payload) {
		return ""
	}
	if spec, ok := platformEndpointSpecForRequest(method, pathOnly); ok && spec.RiskConfirm && riskConfirmationRequired(spec, payload) {
		if spec.RiskConfirmBodyField != "" {
			if spec.Name == "platform-update-campaign" {
				return fmt.Sprintf("--confirm is required unless status is %q and only non-spend fields are changed", spec.RiskConfirmBodyValue)
			}
			return fmt.Sprintf("--confirm is required unless %s is explicitly %q; otherwise acknowledge %s", spec.RiskConfirmBodyField, spec.RiskConfirmBodyValue, riskConfirmationImpact)
		}
		return "--confirm is required to acknowledge " + riskConfirmationImpact
	}
	return "--confirm is required"
}

func platformEndpointSpecForRequest(method, pathOnly string) (appleads.EndpointSpec, bool) {
	method = strings.ToUpper(strings.TrimSpace(method))
	for _, spec := range appleads.PlatformEndpointSpecs() {
		if strings.ToUpper(spec.Method) != method || !platformEndpointPathMatches(spec.Path, pathOnly) {
			continue
		}
		return spec, true
	}
	return appleads.EndpointSpec{}, false
}

func rawPlatformRequestEndpointSpec(method, pathValue string) (appleads.EndpointSpec, bool) {
	pathOnly, err := platformPathOnly(pathValue)
	if err != nil {
		return appleads.EndpointSpec{}, false
	}
	spec, ok := platformEndpointSpecForRequest(method, pathOnly)
	if !ok {
		return appleads.EndpointSpec{}, false
	}
	// Do expands path parameters from the typed command path. A raw request
	// already supplies the concrete path, so preserve that path while retaining
	// the matched endpoint's safety metadata and context.
	spec.Path = pathValue
	return spec, true
}

func requestRawPlatformEndpoint(ctx context.Context, client *appleads.Client, method, pathValue string, payload json.RawMessage, contextKind appleads.ContextKind) (appleads.RawResponse, error) {
	if spec, ok := rawPlatformRequestEndpointSpec(method, pathValue); ok {
		// Route documented endpoints through Do so the raw escape hatch
		// preserves endpoint metadata such as RetrySafe. Unknown requests
		// continue through the conservative versioned transport below.
		return client.Do(ctx, spec, nil, nil, payload)
	}
	return client.RequestForVersion(ctx, appleads.APIVersionPlatformV1, method, pathValue, nil, payload, contextKind)
}

func platformEndpointPathMatches(pattern, path string) bool {
	patternParts := strings.Split(strings.Trim(strings.TrimSpace(pattern), "/"), "/")
	pathParts := strings.Split(strings.Trim(strings.TrimSpace(path), "/"), "/")
	if len(patternParts) != len(pathParts) {
		return false
	}
	for index, patternPart := range patternParts {
		if strings.HasPrefix(patternPart, "{") && strings.HasSuffix(patternPart, "}") {
			if pathParts[index] == "" {
				return false
			}
			continue
		}
		if patternPart != pathParts[index] {
			return false
		}
	}
	return true
}

func validateRawPlatformAdAccountPathID(method, pathOnly, effectiveAdAccountID string) error {
	if method != http.MethodGet && method != http.MethodPut && method != http.MethodDelete {
		return nil
	}
	parts := strings.Split(strings.Trim(pathOnly, "/"), "/")
	if len(parts) != 3 || parts[0] != "v1" || parts[1] != "ad-accounts" || strings.TrimSpace(parts[2]) == "" {
		return nil
	}
	if effectiveAdAccountID == "" {
		return shared.UsageError("--ad-account is required for v1/ad-accounts/{id} and must match the path ID")
	}
	if effectiveAdAccountID != parts[2] {
		return shared.UsageError(fmt.Sprintf("--ad-account %q must match the v1/ad-accounts path ID %q", effectiveAdAccountID, parts[2]))
	}
	return nil
}

func rawPlatformRequestRequiresAdAccount(method, pathValue string) (bool, error) {
	kind, err := rawPlatformRequestContextKind(method, pathValue)
	return kind == appleads.ContextAdAccount, err
}

func rawPlatformRequestContextKind(method, pathValue string) (appleads.ContextKind, error) {
	pathOnly, err := platformPathOnly(pathValue)
	if err != nil {
		return appleads.ContextNone, err
	}
	method = strings.ToUpper(strings.TrimSpace(method))
	resourcePath := strings.TrimPrefix(pathOnly, "v1/")

	switch {
	case method == http.MethodGet && (resourcePath == "me" || resourcePath == "acls" || resourcePath == "advertiser-resources"):
		return appleads.ContextNone, nil
	case method == http.MethodGet && isSingleChildPath(resourcePath, "orgs"):
		return appleads.ContextNone, nil
	case method == http.MethodPost && resourcePath == "ad-accounts":
		return appleads.ContextNone, nil
	case method == http.MethodPost && resourcePath == "shared-budgets":
		return appleads.ContextNone, nil
	case (method == http.MethodPut || method == http.MethodDelete) && isSingleChildPath(resourcePath, "shared-budgets"):
		return appleads.ContextNone, nil
	case method == http.MethodPost && resourcePath == "shared-budgets/query":
		return appleads.ContextAdAccountOptional, nil
	case method == http.MethodGet && isSingleChildPath(resourcePath, "shared-budgets"):
		return appleads.ContextAdAccountOptional, nil
	default:
		return appleads.ContextAdAccount, nil
	}
}

func platformPathOnly(pathValue string) (string, error) {
	trimmed := strings.TrimSpace(pathValue)
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return "", fmt.Errorf("--path must be a valid URL or v1 path: %w", err)
	}
	if parsed.IsAbs() {
		if parsed.Scheme != "https" || parsed.User != nil || parsed.Host != "api.ads.apple.com" || !strings.HasPrefix(parsed.Path, "/v1/") {
			return "", fmt.Errorf("--path must be an Apple Ads Platform API v1 URL")
		}
		pathOnly := strings.TrimPrefix(parsed.Path, "/")
		if err := validatePlatformRelativePath(pathOnly); err != nil {
			return "", err
		}
		return pathOnly, nil
	}
	pathOnly := strings.TrimPrefix(parsed.Path, "/")
	if !strings.HasPrefix(pathOnly, "v1/") {
		return "", fmt.Errorf("--path must start with v1/")
	}
	if err := validatePlatformRelativePath(pathOnly); err != nil {
		return "", err
	}
	return pathOnly, nil
}

func validatePlatformRelativePath(pathOnly string) error {
	if adsPathHasTraversal(pathOnly) {
		return fmt.Errorf("--path must not contain path traversal")
	}
	relative, err := url.Parse(strings.TrimPrefix(pathOnly, "v1/"))
	if err != nil || relative.IsAbs() || relative.Host != "" || relative.User != nil || strings.HasPrefix(relative.Path, "/") || adsPathHasTraversal(relative.Path) {
		return fmt.Errorf("--path must not escape the Apple Ads Platform API v1 base URL")
	}
	return nil
}

func isSingleChildPath(path, parent string) bool {
	parts := strings.Split(path, "/")
	return len(parts) == 2 && parts[0] == parent && strings.TrimSpace(parts[1]) != ""
}

func adsPathHasTraversal(path string) bool {
	for _, part := range strings.Split(path, "/") {
		if part == ".." {
			return true
		}
	}
	return false
}
