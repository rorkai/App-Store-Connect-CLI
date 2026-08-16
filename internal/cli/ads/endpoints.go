package ads

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"slices"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/appleads"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

type endpointFlagValues struct {
	common  commonFlags
	output  shared.OutputFlags
	flagSet *flag.FlagSet

	file     *string
	confirm  *bool
	paginate *bool

	pathStrings   map[string]*string
	pathAliases   map[string][]*shared.DeprecatedStringFlagAlias
	queryStrings  map[string]*string
	queryRepeated map[string]*repeatedFlagValue
	queryInts     map[string]*int
	queryBools    map[string]*bool
}

// repeatedFlagValue preserves each occurrence of a repeated CLI flag. A
// single occurrence may still contain comma-separated values for compatibility
// with the API's existing examples.
type repeatedFlagValue struct {
	values []string
}

func (v *repeatedFlagValue) String() string {
	if v == nil {
		return ""
	}
	return strings.Join(v.values, ",")
}

func (v *repeatedFlagValue) Set(value string) error {
	v.values = append(v.values, value)
	return nil
}

type commandNode struct {
	name     string
	children map[string]*commandNode
	spec     *appleads.EndpointSpec
}

func legacyEndpointCommands() []*ffcli.Command {
	return commandsForEndpointSpecs(appleads.EndpointSpecs(), []string{"v5"})
}

func platformEndpointCommands() []*ffcli.Command {
	return commandsForEndpointSpecs(appleads.PlatformEndpointSpecs(), nil)
}

func commandsForEndpointSpecs(specs []appleads.EndpointSpec, commandPrefix []string) []*ffcli.Command {
	root := &commandNode{children: map[string]*commandNode{}}
	for _, spec := range specs {
		addSpec(root, spec)
	}
	commands := make([]*ffcli.Command, 0, len(root.children))
	for _, name := range sortedChildNames(root) {
		commands = append(commands, buildNodeCommand(root.children[name], nil, commandPrefix))
	}
	return commands
}

func addSpec(root *commandNode, spec appleads.EndpointSpec) {
	current := root
	for index, part := range spec.CommandPath {
		if current.children == nil {
			current.children = map[string]*commandNode{}
		}
		child := current.children[part]
		if child == nil {
			child = &commandNode{name: part, children: map[string]*commandNode{}}
			current.children[part] = child
		}
		current = child
		if spec.DefaultListAlias && index == 0 {
			specCopy := spec
			current.spec = &specCopy
		}
	}
	specCopy := spec
	current.spec = &specCopy
}

func buildNodeCommand(node *commandNode, parentPath, commandPrefix []string) *ffcli.Command {
	path := append(append([]string(nil), parentPath...), node.name)
	displayPath := append(append([]string(nil), commandPrefix...), path...)
	if node.spec != nil && node.spec.Name == "platform-upload-asset" {
		return PlatformAssetUploadCommand()
	}
	var flags endpointFlagValues
	var fs *flag.FlagSet
	if node.spec != nil {
		fs, flags = bindEndpointFlags(*node.spec, strings.Join(path, " "))
	} else {
		fs = flag.NewFlagSet(strings.Join(path, " "), flag.ExitOnError)
	}

	subcommands := []*ffcli.Command{}
	for _, name := range sortedChildNames(node) {
		subcommands = append(subcommands, buildNodeCommand(node.children[name], path, commandPrefix))
	}
	if slices.Equal(commandPrefix, []string{"v5"}) && len(path) == 1 && path[0] == "reports" {
		subcommands = append(subcommands, ReportsPresetCommand())
	}
	if slices.Equal(commandPrefix, []string{"v5"}) {
		subcommands = append(subcommands, workflowSubcommands(path, &flags)...)
	}
	if len(commandPrefix) == 0 {
		subcommands = append(subcommands, platformWorkflowSubcommands(path)...)
	}

	command := &ffcli.Command{
		Name:        node.name,
		ShortUsage:  "asc ads " + strings.Join(displayPath, " ") + " [flags]",
		ShortHelp:   endpointShortHelp(node),
		LongHelp:    endpointLongHelp(node, displayPath),
		FlagSet:     fs,
		UsageFunc:   shared.DefaultUsageFunc,
		Subcommands: subcommands,
	}
	if node.spec != nil {
		spec := *node.spec
		command.Exec = func(ctx context.Context, args []string) error {
			if err := rejectUnexpectedArgs(args); err != nil {
				return err
			}
			return executeEndpoint(ctx, spec, flags)
		}
		if migration, ok := adsLegacyMigrationForSpec(spec); ok {
			command = markAdsLegacyCommandDeprecated(command, displayPath, migration)
		}
	}
	return command
}

func sortedChildNames(node *commandNode) []string {
	names := make([]string, 0, len(node.children))
	for name := range node.children {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

func endpointShortHelp(node *commandNode) string {
	if node.spec == nil {
		return endpointGroupHelp(node.name)
	}
	switch node.spec.Name {
	case "get-me-details", "get-user-acl":
		return sentenceFromEndpointName(node.spec.Name)
	}
	if len(node.children) > 0 {
		return "Manage Apple Ads " + strings.ReplaceAll(node.name, "-", " ") + "."
	}
	return sentenceFromEndpointName(node.spec.Name)
}

func endpointLongHelp(node *commandNode, path []string) string {
	if node.spec == nil {
		return fmt.Sprintf("%s\n\nExamples:\n  asc ads %s --help", endpointGroupHelp(node.name), strings.Join(path, " "))
	}
	examples := []string{"  asc ads " + strings.Join(path, " ")}
	for _, param := range node.spec.PathParams {
		if param.ContextValue {
			continue
		}
		examples[0] += fmt.Sprintf(" --%s %s", param.Flag, strings.ToUpper(param.Flag))
	}
	for _, param := range node.spec.QueryParams {
		if !param.Required {
			continue
		}
		if param.Type == appleads.ParamBool {
			examples[0] += fmt.Sprintf(" --%s", param.Flag)
			continue
		}
		examples[0] += fmt.Sprintf(" --%s %s", param.Flag, strings.ToUpper(strings.ReplaceAll(param.Flag, "-", "_")))
	}
	if node.spec.BodyKind != appleads.BodyNone {
		bodyFile := node.spec.BodyFileExample
		if bodyFile == "" {
			bodyFile = "payload.json"
		}
		if node.spec.BodyOptional && !node.spec.CLIRequiresBody {
			examples[0] += " [--file " + bodyFile + "]"
		} else {
			examples[0] += " --file " + bodyFile
		}
	}
	switch {
	case node.spec.RequiresConfirm || node.spec.RiskConfirm:
		if node.spec.RiskConfirm && node.spec.RiskConfirmBodyField != "" && !node.spec.RequiresConfirm {
			examples[0] += " [--confirm]"
		} else {
			examples[0] += " --confirm"
		}
	case node.spec.ConfirmBodyField != "":
		examples[0] += " [--confirm]"
	}
	if node.spec.RequiresOrg {
		examples[0] += " --org ORG_ID"
	}
	switch node.spec.Context {
	case appleads.ContextAdAccount:
		examples[0] += " --ad-account AD_ACCOUNT_ID"
	case appleads.ContextAdAccountOptional:
		examples[0] += " [--ad-account AD_ACCOUNT_ID]"
	}
	help := fmt.Sprintf("%s\n\nEndpoint: %s %s", endpointShortHelp(node), node.spec.Method, node.spec.Path)
	if node.spec.BodyType == "UpdateCampaignRequest" {
		help += "\n\nPayload:\n  Apple requires a \"campaign\" envelope for campaign updates.\n  Example: {\"campaign\":{\"status\":\"PAUSED\"}}"
	}
	if node.spec.Name == "platform-search-apps" {
		examples = []string{
			`  asc ads apps search --ad-account AD_ACCOUNT_ID --query "Example"`,
			`  asc ads apps search --ad-account AD_ACCOUNT_ID --cpids "123456,789012"`,
			"  asc ads apps search --ad-account AD_ACCOUNT_ID --return-owned-apps",
		}
		help += `

Search modes:
  At least one of --query, --cpids, or --return-owned-apps is required.
  --query must contain at least 3 alphanumeric characters (2 for CJK text);
  punctuation-only and shorter values are rejected before authentication.
  These selectors can be combined. --query searches app and developer names;
  --cpids scopes results to content providers; --return-owned-apps returns
  apps owned by the current organization.`
	}
	if node.spec.BodyKind != appleads.BodyNone {
		help += endpointBodyHelp(*node.spec)
	}
	return help + "\n\nExamples:\n" + strings.Join(examples, "\n")
}

func endpointBodyHelp(spec appleads.EndpointSpec) string {
	required := "yes"
	if spec.BodyOptional && !spec.CLIRequiresBody {
		required = "no"
	}
	help := fmt.Sprintf("\n\nRequest body:\n  Schema: %s\n  Shape: %s\n  Required: %s", spec.BodyType, endpointBodyShape(spec.BodyKind), required)
	if strings.TrimSpace(spec.BodyHint) != "" {
		hint := strings.TrimSpace(spec.BodyHint)
		hint = strings.ReplaceAll(hint, "\n", "\n  ")
		help += "\n  Guidance: " + hint
	}
	if strings.TrimSpace(spec.BodyExample) != "" {
		fileName := strings.TrimSpace(spec.BodyFileExample)
		if fileName == "" {
			fileName = "payload.json"
		}
		example := strings.ReplaceAll(strings.TrimSpace(spec.BodyExample), "\n", "\n    ")
		help += fmt.Sprintf("\n  Starter payload (%s):\n    %s", fileName, example)
	}
	return help
}

func endpointBodyShape(kind appleads.BodyKind) string {
	switch kind {
	case appleads.BodyObject:
		return "JSON object"
	case appleads.BodyArray:
		return "JSON array"
	case appleads.BodyMultipart:
		return "multipart/form-data"
	default:
		return string(kind)
	}
}

func endpointGroupHelp(name string) string {
	switch name {
	case "acls":
		return "List Apple Ads account ACLs."
	case "advertiser-resources":
		return "List Apple Ads advertiser resources."
	case "me":
		return "View the current Apple Ads user."
	case "orgs":
		return "View Apple Ads organizations."
	case "geo":
		return "Manage Apple Ads geographic targeting resources."
	default:
		return "Manage Apple Ads " + strings.ReplaceAll(name, "-", " ") + "."
	}
}

func sentenceFromEndpointName(name string) string {
	switch name {
	case "get-me-details", "platform-get-me-details":
		return "View the current Apple Ads user."
	case "get-user-acl", "platform-get-user-acls":
		return "List Apple Ads account ACLs."
	}
	text := strings.ReplaceAll(strings.TrimPrefix(strings.TrimSpace(name), "platform-"), "-", " ")
	replacements := []struct {
		old string
		new string
	}{
		{"get all ", "List all "},
		{"get a ", "View a "},
		{"get an ", "View an "},
		{"get ", "View "},
		{"gets a ", "View a "},
		{"search for ", "Search for "},
		{"search ", "Search "},
		{"query ", "Find "},
		{"find ", "Find "},
		{"create a ", "Create a "},
		{"create an ", "Create an "},
		{"create ", "Create "},
		{"update a ", "Update a "},
		{"update an ", "Update an "},
		{"update ", "Update "},
		{"delete a ", "Delete a "},
		{"delete an ", "Delete an "},
		{"delete ", "Delete "},
		{"apply ", "Apply "},
		{"dismiss ", "Dismiss "},
		{"impression share report", "Create impression share report"},
	}
	for _, replacement := range replacements {
		if strings.HasPrefix(text, replacement.old) {
			text = replacement.new + strings.TrimPrefix(text, replacement.old)
			break
		}
	}
	if text == "" {
		text = name
	}
	return strings.TrimSuffix(text, ".") + "."
}

func bindEndpointFlags(spec appleads.EndpointSpec, flagSetName string) (*flag.FlagSet, endpointFlagValues) {
	fs := flag.NewFlagSet(flagSetName, flag.ExitOnError)
	values := endpointFlagValues{
		common: commonFlags{
			AdsProfile: fs.String("ads-profile", "", "Use named Apple Ads authentication profile"),
		},
		output:        bindAdsRawOutputFlags(fs),
		flagSet:       fs,
		pathStrings:   map[string]*string{},
		pathAliases:   map[string][]*shared.DeprecatedStringFlagAlias{},
		queryStrings:  map[string]*string{},
		queryRepeated: map[string]*repeatedFlagValue{},
		queryInts:     map[string]*int{},
		queryBools:    map[string]*bool{},
	}
	if spec.RequiresOrg {
		values.common.Org = fs.String("org", "", "Apple Ads organization ID (or ASC_ADS_ORG_ID env)")
	}
	if spec.Context == appleads.ContextAdAccount || spec.Context == appleads.ContextAdAccountOptional {
		values.common.AdAccount = fs.String("ad-account", "", "Apple Ads ad account ID (or ASC_ADS_AD_ACCOUNT_ID env)")
	}
	for _, param := range spec.PathParams {
		if param.ContextValue {
			continue
		}
		values.pathStrings[param.Name] = fs.String(param.Flag, "", flagUsage(param))
		for _, alias := range param.Aliases {
			values.pathAliases[param.Name] = append(values.pathAliases[param.Name], shared.BindDeprecatedStringFlagAlias(fs, alias, param.Flag))
		}
	}
	for _, param := range spec.QueryParams {
		switch param.Type {
		case appleads.ParamInt:
			values.queryInts[param.Name] = fs.Int(param.Flag, intParamDefault(param), flagUsage(param))
		case appleads.ParamBool:
			values.queryBools[param.Name] = fs.Bool(param.Flag, false, flagUsage(param))
		default:
			if param.Repeated {
				repeated := &repeatedFlagValue{}
				values.queryRepeated[param.Name] = repeated
				fs.Var(repeated, param.Flag, flagUsage(param))
				continue
			}
			values.queryStrings[param.Name] = fs.String(param.Flag, "", flagUsage(param))
		}
	}
	if spec.BodyKind != appleads.BodyNone {
		values.file = fs.String("file", "", "Path to Apple Ads JSON payload ('-' reads stdin)")
	}
	if spec.RequiresConfirm || spec.RiskConfirm {
		values.confirm = fs.Bool("confirm", false, confirmFlagUsage(spec))
	}
	if spec.ConfirmBodyField != "" && values.confirm == nil {
		values.confirm = fs.Bool("confirm", false, "Confirm an Apple Ads update that replaces delegations")
	}
	if spec.SupportsPaginate {
		values.paginate = fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	}
	return fs, values
}

func flagUsage(param appleads.ParamSpec) string {
	usage := param.Description
	if usage == "" {
		usage = strings.ReplaceAll(param.Flag, "-", " ")
	}
	if param.Required {
		usage += " (required)"
	}
	if param.Max > 0 {
		usage += fmt.Sprintf(" (max %d)", param.Max)
	}
	if len(param.Allowed) > 0 {
		usage += " (" + strings.Join(param.Allowed, ", ") + ")"
	}
	return usage
}

const riskConfirmationImpact = "potential Apple Ads spend, billing, delivery, targeting, or access impact"

func confirmFlagUsage(spec appleads.EndpointSpec) string {
	switch {
	case spec.RiskConfirm:
		return "Acknowledge " + riskConfirmationImpact
	case spec.RequiresConfirm:
		return "Confirm deletion of this Apple Ads resource"
	case spec.ConfirmBodyField != "":
		return "Confirm an Apple Ads update that replaces delegations"
	default:
		return "Confirm this Apple Ads operation"
	}
}

func intParamDefault(param appleads.ParamSpec) int {
	return param.Default
}

func executeEndpoint(ctx context.Context, spec appleads.EndpointSpec, flags endpointFlagValues) error {
	outputFormat, err := validateAdsRawOutput(flags.output)
	if err != nil {
		return shared.UsageError(err.Error())
	}
	if spec.RequiresConfirm && flags.confirm != nil && !*flags.confirm {
		return shared.UsageError("--confirm is required")
	}
	pathParams, err := collectPathParams(spec, flags)
	if err != nil {
		return shared.UsageError(err.Error())
	}
	query, err := collectQuery(spec, flags)
	if err != nil {
		return shared.UsageError(err.Error())
	}
	if spec.RiskConfirm && spec.RiskConfirmBodyField == "" && flags.confirm != nil && !*flags.confirm {
		return shared.UsageError("--confirm is required to acknowledge " + riskConfirmationImpact)
	}
	body, err := readBody(spec, flags)
	if err != nil {
		return err
	}
	if err := validateEndpointBody(spec, body, flags.confirm != nil && *flags.confirm); err != nil {
		return shared.UsageError(err.Error())
	}

	var client *appleads.Client
	if spec.Version == appleads.APIVersionPlatformV1 {
		var adAccountID string
		client, adAccountID, err = resolvePlatformClientAndAdAccountID(ctx, flags.common, spec.Context)
		if err != nil {
			return fmt.Errorf("ads: %w", err)
		}
		for _, param := range spec.PathParams {
			if param.ContextValue {
				pathParams[param.Name] = adAccountID
			}
		}
	}
	if spec.Version != appleads.APIVersionPlatformV1 {
		client, err = resolveClient(ctx, flags.common, spec.RequiresOrg)
	}
	if err != nil {
		return fmt.Errorf("ads: %w", err)
	}

	requestCtx, cancel := requestContext(ctx)
	defer cancel()

	var result appleads.RawResponse
	if flags.paginate != nil && *flags.paginate {
		startOffset := intValue(flags.queryInts["offset"])
		pageSize := intValue(flags.queryInts["limit"])
		if pageSize == 0 {
			// Geo search uses the Platform API's pageSize spelling instead of
			// the limit used by apps and the legacy API.
			pageSize = intValue(flags.queryInts["pageSize"])
		}
		result, err = client.PaginateAll(requestCtx, spec, pathParams, query, startOffset, pageSize, body)
	} else {
		result, err = client.Do(requestCtx, spec, pathParams, query, body)
	}
	if err != nil {
		return fmt.Errorf("ads %s: %w", strings.Join(spec.CommandPath, " "), err)
	}
	return shared.PrintOutput(result, outputFormat, *flags.output.Pretty)
}

func collectPathParams(spec appleads.EndpointSpec, flags endpointFlagValues) (map[string]string, error) {
	params := map[string]string{}
	for _, param := range spec.PathParams {
		if param.ContextValue {
			continue
		}
		ptr := flags.pathStrings[param.Name]
		for _, alias := range flags.pathAliases[param.Name] {
			if err := alias.Apply(ptr); err != nil {
				return nil, err
			}
		}
		value := value(ptr)
		if param.Required && value == "" {
			return nil, fmt.Errorf("--%s is required", param.Flag)
		}
		if value != "" && param.Type == appleads.ParamInt {
			if parsed, err := strconv.ParseInt(value, 10, 64); err != nil {
				return nil, fmt.Errorf("--%s must be an integer", param.Flag)
			} else if parsed < 0 {
				return nil, fmt.Errorf("--%s must be >= 0", param.Flag)
			}
		}
		params[param.Name] = value
	}
	return params, nil
}

func collectQuery(spec appleads.EndpointSpec, flags endpointFlagValues) (url.Values, error) {
	query := url.Values{}
	for _, param := range spec.QueryParams {
		switch param.Type {
		case appleads.ParamInt:
			raw := intValue(flags.queryInts[param.Name])
			provided := flagProvided(flags.flagSet, param.Flag)
			if raw == 0 {
				if param.Required {
					return nil, fmt.Errorf("--%s is required", param.Flag)
				}
				if provided && (param.Name == "limit" || param.Name == "pageSize") {
					maxLimit := appleads.MaxPageLimit(spec)
					if maxLimit > 0 {
						return nil, fmt.Errorf("--%s must be between 1 and %d", param.Flag, maxLimit)
					}
					return nil, fmt.Errorf("--%s must be greater than 0", param.Flag)
				}
				continue
			}
			if raw < 0 {
				return nil, fmt.Errorf("--%s must be >= 0", param.Flag)
			}
			if param.Max > 0 && param.Name != "limit" && raw > param.Max {
				return nil, fmt.Errorf("--%s must be at most %d", param.Flag, param.Max)
			}
			if param.Name == "limit" {
				maxLimit := appleads.MaxPageLimit(spec)
				if raw < 1 || (maxLimit > 0 && raw > maxLimit) {
					if maxLimit == 0 {
						return nil, fmt.Errorf("--limit must be greater than 0")
					}
					return nil, fmt.Errorf("--limit must be between 1 and %d", maxLimit)
				}
			}
			query.Set(param.Name, strconv.Itoa(raw))
		case appleads.ParamBool:
			if ptr := flags.queryBools[param.Name]; ptr != nil && *ptr {
				query.Set(param.Name, "true")
			}
		default:
			raw := value(flags.queryStrings[param.Name])
			if spec.Name == "platform-search-geo-locations" && (param.Name == "supplySource" || param.Name == "countrycode") {
				raw = strings.ToUpper(raw)
			}
			if param.Repeated {
				rawValues := []string(nil)
				if repeated := flags.queryRepeated[param.Name]; repeated != nil {
					rawValues = repeated.values
				} else if raw != "" {
					rawValues = []string{raw}
				}
				if len(rawValues) == 0 {
					if param.Required {
						return nil, fmt.Errorf("--%s is required", param.Flag)
					}
					continue
				}
				for _, occurrence := range rawValues {
					for _, part := range strings.Split(occurrence, ",") {
						part = strings.TrimSpace(part)
						if part == "" {
							return nil, fmt.Errorf("--%s must not contain empty values", param.Flag)
						}
						if err := validateAllowed(param, part); err != nil {
							return nil, err
						}
						if param.Name == "storeFronts" {
							if len(part) != 2 || !isASCIIAlpha(part[0]) || !isASCIIAlpha(part[1]) {
								return nil, fmt.Errorf("--%s values must be ISO 3166-1 alpha-2 country or region codes", param.Flag)
							}
							part = strings.ToUpper(part)
						}
						query.Add(param.Name, part)
					}
				}
				continue
			}
			if raw == "" {
				if param.Required {
					return nil, fmt.Errorf("--%s is required", param.Flag)
				}
				continue
			}
			if err := validateAllowed(param, raw); err != nil {
				return nil, err
			}
			query.Set(param.Name, raw)
		}
	}
	if spec.Name == "platform-search-apps" {
		if err := validatePlatformAppSearch(query); err != nil {
			return nil, err
		}
	}
	if spec.Name == "platform-search-geo-locations" {
		if err := validatePlatformGeoSearch(query); err != nil {
			return nil, err
		}
	}
	return query, nil
}

func isASCIIAlpha(value byte) bool {
	return value >= 'A' && value <= 'Z' || value >= 'a' && value <= 'z'
}

func validatePlatformAppSearch(query url.Values) error {
	text := strings.TrimSpace(query.Get("query"))
	if text == "" && strings.TrimSpace(query.Get("cpids")) == "" && query.Get("returnOwnedApps") != "true" {
		return fmt.Errorf("at least one of --query, --cpids, or --return-owned-apps is required")
	}
	if text == "" {
		return nil
	}
	if !strings.ContainsFunc(text, unicode.IsLetter) && !strings.ContainsFunc(text, unicode.IsDigit) {
		return fmt.Errorf("--query must contain at least one alphanumeric character")
	}
	minimum := 3
	if strings.ContainsFunc(text, func(r rune) bool {
		return unicode.In(r, unicode.Han, unicode.Hiragana, unicode.Katakana, unicode.Hangul)
	}) {
		minimum = 2
	}
	if utf8.RuneCountInString(text) < minimum {
		return fmt.Errorf("--query must contain at least %d characters", minimum)
	}
	return nil
}

func validatePlatformGeoSearch(query url.Values) error {
	text := strings.TrimSpace(query.Get("query"))
	if text != "" && text != "*" && utf8.RuneCountInString(text) < 2 {
		return fmt.Errorf("--query must contain at least 2 characters")
	}
	countryCode := query.Get("countrycode")
	if countryCode != "" && (len(countryCode) != 2 || !isASCIIAlpha(countryCode[0]) || !isASCIIAlpha(countryCode[1])) {
		return fmt.Errorf("--country-code must be an ISO 3166-1 alpha-2 country or region code")
	}
	return nil
}

func flagProvided(fs *flag.FlagSet, name string) bool {
	if fs == nil {
		return false
	}
	provided := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == name {
			provided = true
		}
	})
	return provided
}

func validateAllowed(param appleads.ParamSpec, raw string) error {
	if len(param.Allowed) == 0 {
		return nil
	}
	allowed := map[string]struct{}{}
	for _, item := range param.Allowed {
		allowed[item] = struct{}{}
	}
	for _, part := range strings.Split(raw, ",") {
		value := strings.TrimSpace(part)
		if _, ok := allowed[value]; !ok {
			return fmt.Errorf("--%s must be one of: %s", param.Flag, strings.Join(param.Allowed, ", "))
		}
	}
	return nil
}

func readBody(spec appleads.EndpointSpec, flags endpointFlagValues) (json.RawMessage, error) {
	if spec.BodyKind == appleads.BodyNone {
		return nil, nil
	}
	fileValue := value(flags.file)
	if fileValue == "" {
		if spec.BodyOptional && !spec.CLIRequiresBody {
			return nil, nil
		}
		fmt.Fprintln(os.Stderr, "Error: --file is required")
		return nil, shared.MissingRequiredUsageError()
	}
	kind := shared.JSONPayloadObject
	if spec.BodyKind == appleads.BodyArray {
		kind = shared.JSONPayloadArray
	}
	payload, err := shared.ReadJSONFilePayloadKind(fileValue, kind)
	if err != nil {
		return nil, fmt.Errorf("ads %s: %w", strings.Join(spec.CommandPath, " "), err)
	}
	return payload, nil
}

func validateEndpointBody(spec appleads.EndpointSpec, body json.RawMessage, confirmed bool) error {
	if len(body) == 0 || spec.Version != appleads.APIVersionPlatformV1 {
		return nil
	}
	if spec.RiskConfirm && !confirmed && riskConfirmationRequired(spec, body) {
		if spec.RiskConfirmBodyField != "" {
			if spec.Name == "platform-update-campaign" {
				return fmt.Errorf("--confirm is required unless status is %q and only non-spend fields are changed", spec.RiskConfirmBodyValue)
			}
			return fmt.Errorf("--confirm is required unless %s is explicitly %q; otherwise acknowledge %s", spec.RiskConfirmBodyField, spec.RiskConfirmBodyValue, riskConfirmationImpact)
		}
		return fmt.Errorf("--confirm is required to acknowledge %s", riskConfirmationImpact)
	}
	if spec.Method == http.MethodPost && strings.HasPrefix(spec.Path, "v1/") && strings.HasSuffix(spec.Path, "/query") {
		if err := validatePlatformQueryMigration(spec, body); err != nil {
			return err
		}
	}
	switch spec.Name {
	case "platform-query-keywords", "platform-query-negative-keywords":
		if err := validateKeywordQuerySelector(spec.Name, body); err != nil {
			return err
		}
	}
	if spec.Name != "platform-create-ad-account" && spec.Name != "platform-update-ad-account" {
		return nil
	}

	var payload map[string]json.RawMessage
	if err := json.Unmarshal(body, &payload); err != nil {
		return fmt.Errorf("invalid JSON object: %w", err)
	}
	if spec.Name == "platform-create-ad-account" {
		if err := requireNonEmptyJSONString(payload, "name"); err != nil {
			return err
		}
		features, ok := payload["productFeatures"]
		if !ok {
			return fmt.Errorf("productFeatures is required")
		}
		if err := validateProductFeatures(features); err != nil {
			return err
		}
	}
	if spec.Name == "platform-update-ad-account" {
		if _, present := payload["productFeatures"]; present {
			return fmt.Errorf("productFeatures is immutable and is not supported by ad-account update")
		}
		if rawName, present := payload["name"]; present {
			var name string
			if err := json.Unmarshal(rawName, &name); err != nil || strings.TrimSpace(name) == "" {
				return fmt.Errorf("name must be a non-empty string")
			}
		}
	}
	if delegations, present := payload["delegations"]; present {
		if spec.ConfirmBodyField == "delegations" && !confirmed {
			return fmt.Errorf("--confirm is required when replacing delegations")
		}
		if err := validateDelegations(delegations); err != nil {
			return err
		}
	}
	return nil
}

type querySelectorFilter struct {
	Field    string `json:"field"`
	Operator string `json:"operator"`
}

type legacyPlatformQueryMembers struct {
	selector                   bool
	conditions                 bool
	values                     bool
	orderBy                    bool
	sortOrder                  bool
	limit                      bool
	selectorFields             bool
	topLevelFields             bool
	legacyOperator             bool
	legacyOrder                bool
	startTime                  bool
	endTime                    bool
	timeZone                   bool
	granularity                bool
	returnRecordsWithNoMetrics bool
	returnRowTotals            bool
	returnGrandTotals          bool
	name                       bool
	dateRange                  bool
}

func validatePlatformQueryMigration(spec appleads.EndpointSpec, body json.RawMessage) error {
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(body, &payload); err != nil {
		return fmt.Errorf("invalid Platform API query body: %w", err)
	}

	legacy := legacyPlatformQueryMembers{}
	inspectLegacyPlatformQueryObject(payload, &legacy, false)
	if rawSelector, present := payload["selector"]; present {
		legacy.selector = true
		var selector map[string]json.RawMessage
		if json.Unmarshal(rawSelector, &selector) == nil {
			inspectLegacyPlatformQueryObject(selector, &legacy, true)
		}
	}

	var migrations []string
	if legacy.selector {
		migrations = append(migrations, `remove the v5 "selector" wrapper; v1 query members are top level`)
	}
	if legacy.conditions {
		migrations = append(migrations, `"conditions" -> "filters"`)
	}
	if legacy.values {
		migrations = append(migrations, `filter "values" -> "value"`)
	}
	if legacy.orderBy {
		migrations = append(migrations, `"orderBy" -> "sorting"`)
	}
	if legacy.sortOrder {
		migrations = append(migrations, `sorting "sortOrder" -> "order"`)
	}
	if legacy.limit {
		migrations = append(migrations, `"pagination.limit" -> "pagination.pageSize"`)
	}
	if legacy.legacyOperator {
		migrations = append(migrations, `filter operator "STARTSWITH"/"ENDSWITH" -> "STARTS_WITH"/"ENDS_WITH"`)
	}
	if legacy.legacyOrder {
		migrations = append(migrations, `sort order "ASCENDING"/"DESCENDING" -> "ASC"/"DESC"`)
	}
	if legacy.selectorFields {
		if platformQuerySupportsFields(spec.BodyType) {
			migrations = append(migrations, `"selector.fields" -> top-level "fields"`)
		} else {
			migrations = append(migrations, fmt.Sprintf(`remove "selector.fields"; %s has no field-projection member`, spec.BodyType))
		}
	}
	if legacy.topLevelFields && !platformQuerySupportsFields(spec.BodyType) {
		migrations = append(migrations, fmt.Sprintf(`remove top-level "fields"; %s has no field-projection member`, spec.BodyType))
	}
	if platformQuerySupportsTimeRange(spec.BodyType) {
		if legacy.startTime {
			migrations = append(migrations, `"startTime" -> "timeRange.start"`)
		}
		if legacy.endTime {
			migrations = append(migrations, `"endTime" -> "timeRange.end"`)
		}
		if legacy.timeZone {
			migrations = append(migrations, `"timeZone" -> "timeRange.timeZone"`)
		}
		if legacy.granularity {
			migrations = append(migrations, `"granularity" -> "timeRange.granularity"`)
		}
	}
	if platformQueryIsReport(spec.BodyType) {
		if legacy.returnRecordsWithNoMetrics {
			if spec.BodyType == "AppsReportingRequest" {
				migrations = append(migrations, `"returnRecordsWithNoMetrics": true -> add "EMPTY_METRICS" to "options.includeRows"; false -> omit`)
			} else {
				migrations = append(migrations, `remove "returnRecordsWithNoMetrics"; BrandsOptions doesn't support EMPTY_METRICS`)
			}
		}
		if legacy.returnRowTotals {
			migrations = append(migrations, `remove "returnRowTotals"; no v1 report request field exists`)
		}
		if legacy.returnGrandTotals {
			migrations = append(migrations, `"returnGrandTotals": true -> add "GRAND_TOTAL" to "options.includeRows"; false -> omit`)
		}
	}
	if spec.BodyType == "ImpressionShareQueryRequest" {
		if legacy.dateRange {
			migrations = append(migrations, `"dateRange" -> explicit "timeRange.start" and "timeRange.end"`)
		}
		if legacy.name {
			migrations = append(migrations, `remove "name"; ImpressionShareQueryRequest has no saved-report name`)
		}
	}
	if len(migrations) > 0 {
		return fmt.Errorf("platform API v1 query payload uses legacy v5 fields: %s", strings.Join(migrations, "; "))
	}
	return nil
}

func inspectLegacyPlatformQueryObject(payload map[string]json.RawMessage, legacy *legacyPlatformQueryMembers, selector bool) {
	if _, present := payload["conditions"]; present {
		legacy.conditions = true
	}
	if _, present := payload["orderBy"]; present {
		legacy.orderBy = true
	}
	if _, present := payload["fields"]; present {
		if selector {
			legacy.selectorFields = true
		} else {
			legacy.topLevelFields = true
		}
	}
	for _, key := range []string{"filters", "conditions"} {
		inspectLegacyPlatformFilters(payload[key], legacy)
	}
	for _, key := range []string{"sorting", "orderBy"} {
		inspectLegacyPlatformSorting(payload[key], legacy)
	}
	var pagination map[string]json.RawMessage
	if json.Unmarshal(payload["pagination"], &pagination) == nil {
		if _, present := pagination["limit"]; present {
			legacy.limit = true
		}
	}
	for key, target := range map[string]*bool{
		"startTime":                  &legacy.startTime,
		"endTime":                    &legacy.endTime,
		"timeZone":                   &legacy.timeZone,
		"granularity":                &legacy.granularity,
		"returnRecordsWithNoMetrics": &legacy.returnRecordsWithNoMetrics,
		"returnRowTotals":            &legacy.returnRowTotals,
		"returnGrandTotals":          &legacy.returnGrandTotals,
		"name":                       &legacy.name,
		"dateRange":                  &legacy.dateRange,
	} {
		if _, present := payload[key]; present {
			*target = true
		}
	}
}

func inspectLegacyPlatformFilters(raw json.RawMessage, legacy *legacyPlatformQueryMembers) {
	var entries []map[string]json.RawMessage
	if json.Unmarshal(raw, &entries) != nil {
		return
	}
	for _, entry := range entries {
		if _, present := entry["values"]; present {
			legacy.values = true
		}
		var operator string
		if json.Unmarshal(entry["operator"], &operator) == nil && (operator == "STARTSWITH" || operator == "ENDSWITH") {
			legacy.legacyOperator = true
		}
	}
}

func inspectLegacyPlatformSorting(raw json.RawMessage, legacy *legacyPlatformQueryMembers) {
	var entries []map[string]json.RawMessage
	if json.Unmarshal(raw, &entries) != nil {
		return
	}
	for _, entry := range entries {
		if _, present := entry["sortOrder"]; present {
			legacy.sortOrder = true
		}
		for _, key := range []string{"order", "sortOrder"} {
			var order string
			if json.Unmarshal(entry[key], &order) == nil && (order == "ASCENDING" || order == "DESCENDING") {
				legacy.legacyOrder = true
			}
		}
	}
}

func platformQuerySupportsFields(bodySchema string) bool {
	return bodySchema == "AppsReportingRequest" || bodySchema == "BrandsReportingRequest"
}

func platformQuerySupportsTimeRange(bodySchema string) bool {
	switch bodySchema {
	case "AppsReportingRequest", "BrandsReportingRequest", "ImpressionShareQueryRequest", "SearchTermPopularityQueryRequest":
		return true
	default:
		return false
	}
}

func platformQueryIsReport(bodySchema string) bool {
	return bodySchema == "AppsReportingRequest" || bodySchema == "BrandsReportingRequest"
}

func validateKeywordQuerySelector(specName string, body json.RawMessage) error {
	var payload struct {
		Filters []querySelectorFilter `json:"filters"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return fmt.Errorf("invalid QueryRequest selector filters: %w", err)
	}
	switch specName {
	case "platform-query-keywords":
		for _, filter := range payload.Filters {
			switch filter.Field {
			case "id", "adGroupId", "campaignId":
				return nil
			}
		}
		return fmt.Errorf("targeting-keywords find requires at least one filter field id, adGroupId, or campaignId")
	case "platform-query-negative-keywords":
		var hasID, hasAdGroup, hasCampaign, hasAdGroupIsNull bool
		for _, filter := range payload.Filters {
			switch filter.Field {
			case "id":
				hasID = true
			case "adGroupId":
				hasAdGroup = true
				if filter.Operator == "IS_NULL" {
					hasAdGroupIsNull = true
				}
			case "campaignId":
				hasCampaign = true
			}
		}
		if hasID || (hasAdGroup && !hasAdGroupIsNull) || (hasCampaign && hasAdGroupIsNull) {
			return nil
		}
		return fmt.Errorf("negative-keywords find requires a filter for id or adGroupId; campaign-level queries require campaignId plus an adGroupId filter with operator IS_NULL")
	}
	return nil
}

func riskConfirmationRequired(spec appleads.EndpointSpec, body json.RawMessage) bool {
	if !spec.RiskConfirm {
		return false
	}
	if spec.RiskConfirmBodyField == "" {
		return true
	}
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(body, &payload); err != nil {
		return true
	}
	checkPayload, field, ok := nestedRiskConfirmationPayload(payload, spec.RiskConfirmBodyField)
	if !ok {
		return true
	}
	if len(spec.RiskConfirmAllowedBodyFields) > 0 {
		allowed := make(map[string]struct{}, len(spec.RiskConfirmAllowedBodyFields))
		for _, field := range spec.RiskConfirmAllowedBodyFields {
			allowed[field] = struct{}{}
		}
		for field := range checkPayload {
			if _, ok := allowed[field]; !ok {
				return true
			}
		}
	}
	rawValue, ok := checkPayload[field]
	if !ok {
		return true
	}
	var value string
	if err := json.Unmarshal(rawValue, &value); err != nil {
		return true
	}
	return value != spec.RiskConfirmBodyValue
}

func nestedRiskConfirmationPayload(payload map[string]json.RawMessage, fieldPath string) (map[string]json.RawMessage, string, bool) {
	parts := strings.Split(fieldPath, ".")
	if len(parts) == 0 {
		return nil, "", false
	}
	current := payload
	for _, part := range parts[:len(parts)-1] {
		raw, ok := current[part]
		if !ok {
			return nil, "", false
		}
		var nested map[string]json.RawMessage
		if err := json.Unmarshal(raw, &nested); err != nil {
			return nil, "", false
		}
		current = nested
	}
	return current, parts[len(parts)-1], true
}

func requireNonEmptyJSONString(payload map[string]json.RawMessage, field string) error {
	raw, ok := payload[field]
	if !ok {
		return fmt.Errorf("%s is required", field)
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil || strings.TrimSpace(value) == "" {
		return fmt.Errorf("%s must be a non-empty string", field)
	}
	return nil
}

func validateProductFeatures(raw json.RawMessage) error {
	var values []string
	if err := json.Unmarshal(raw, &values); err != nil || len(values) != 1 {
		return fmt.Errorf("productFeatures must contain exactly one value")
	}
	for _, value := range values {
		if value != "APPSTORE_APP_MANUAL" && value != "BUSINESS_BRAND_MANUAL" {
			return fmt.Errorf("productFeatures values must be APPSTORE_APP_MANUAL or BUSINESS_BRAND_MANUAL")
		}
	}
	return nil
}

func validateDelegations(raw json.RawMessage) error {
	var values []map[string]json.RawMessage
	if err := json.Unmarshal(raw, &values); err != nil {
		return fmt.Errorf("delegations must be an array")
	}
	for index, delegation := range values {
		if err := requireNonEmptyJSONString(delegation, "resourceId"); err != nil {
			return fmt.Errorf("delegations[%d]: %w", index, err)
		}
		rawType, ok := delegation["resourceType"]
		if !ok {
			return fmt.Errorf("delegations[%d].resourceType is required", index)
		}
		var resourceType string
		if err := json.Unmarshal(rawType, &resourceType); err != nil || (resourceType != "CONTENT_PROVIDER" && resourceType != "BUSINESS_BRAND") {
			return fmt.Errorf("delegations[%d].resourceType must be CONTENT_PROVIDER or BUSINESS_BRAND", index)
		}
	}
	return nil
}

func intValue(ptr *int) int {
	if ptr == nil {
		return 0
	}
	return *ptr
}
