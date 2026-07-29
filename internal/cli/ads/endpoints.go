package ads

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/url"
	"os"
	"slices"
	"strconv"
	"strings"

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

	pathStrings  map[string]*string
	queryStrings map[string]*string
	queryInts    map[string]*int
	queryBools   map[string]*bool
}

type commandNode struct {
	name     string
	children map[string]*commandNode
	spec     *appleads.EndpointSpec
}

func endpointCommands() []*ffcli.Command {
	root := &commandNode{children: map[string]*commandNode{}}
	for _, spec := range appleads.EndpointSpecs() {
		addSpec(root, spec)
	}
	commands := make([]*ffcli.Command, 0, len(root.children))
	for _, name := range sortedChildNames(root) {
		commands = append(commands, buildNodeCommand(root.children[name], nil))
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

func buildNodeCommand(node *commandNode, parentPath []string) *ffcli.Command {
	path := append(append([]string(nil), parentPath...), node.name)
	var flags endpointFlagValues
	var fs *flag.FlagSet
	if node.spec != nil {
		fs, flags = bindEndpointFlags(*node.spec, strings.Join(path, " "))
	} else {
		fs = flag.NewFlagSet(strings.Join(path, " "), flag.ExitOnError)
	}

	subcommands := []*ffcli.Command{}
	for _, name := range sortedChildNames(node) {
		subcommands = append(subcommands, buildNodeCommand(node.children[name], path))
	}
	if len(path) == 1 && path[0] == "reports" {
		subcommands = append(subcommands, ReportsPresetCommand())
	}
	subcommands = append(subcommands, workflowSubcommands(path, &flags)...)

	command := &ffcli.Command{
		Name:        node.name,
		ShortUsage:  "asc ads " + strings.Join(path, " ") + " [flags]",
		ShortHelp:   endpointShortHelp(node),
		LongHelp:    endpointLongHelp(node, path),
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
		examples[0] += fmt.Sprintf(" --%s %s", param.Flag, strings.ToUpper(param.Flag))
	}
	if node.spec.BodyKind != appleads.BodyNone {
		examples[0] += " --file payload.json"
	}
	if node.spec.RequiresConfirm {
		examples[0] += " --confirm"
	}
	if node.spec.RequiresOrg {
		examples[0] += " --org ORG_ID"
	}
	help := fmt.Sprintf("%s\n\nEndpoint: %s %s", endpointShortHelp(node), node.spec.Method, node.spec.Path)
	if node.spec.BodyType == "UpdateCampaignRequest" {
		help += "\n\nPayload:\n  Apple requires a \"campaign\" envelope for campaign updates.\n  Example: {\"campaign\":{\"status\":\"PAUSED\"}}"
	}
	return help + "\n\nExamples:\n" + strings.Join(examples, "\n")
}

func endpointGroupHelp(name string) string {
	switch name {
	case "me":
		return "View the current Apple Ads user."
	case "geo":
		return "Manage Apple Ads geographic targeting resources."
	default:
		return "Manage Apple Ads " + strings.ReplaceAll(name, "-", " ") + "."
	}
}

func sentenceFromEndpointName(name string) string {
	switch name {
	case "get-me-details":
		return "View the current Apple Ads user."
	case "get-user-acl":
		return "List Apple Ads account ACLs."
	}
	text := strings.ReplaceAll(strings.TrimSpace(name), "-", " ")
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
		output:       shared.BindOutputFlags(fs),
		flagSet:      fs,
		pathStrings:  map[string]*string{},
		queryStrings: map[string]*string{},
		queryInts:    map[string]*int{},
		queryBools:   map[string]*bool{},
	}
	if spec.RequiresOrg {
		values.common.Org = fs.String("org", "", "Apple Ads organization ID (or ASC_ADS_ORG_ID env)")
	}
	for _, param := range spec.PathParams {
		values.pathStrings[param.Name] = fs.String(param.Flag, "", flagUsage(param))
	}
	for _, param := range spec.QueryParams {
		switch param.Type {
		case appleads.ParamInt:
			values.queryInts[param.Name] = fs.Int(param.Flag, 0, flagUsage(param))
		case appleads.ParamBool:
			values.queryBools[param.Name] = fs.Bool(param.Flag, false, flagUsage(param))
		default:
			values.queryStrings[param.Name] = fs.String(param.Flag, "", flagUsage(param))
		}
	}
	if spec.BodyKind != appleads.BodyNone {
		values.file = fs.String("file", "", "Path to Apple Ads JSON payload")
	}
	if spec.RequiresConfirm {
		values.confirm = fs.Bool("confirm", false, "Confirm this destructive Apple Ads operation")
	}
	if spec.SupportsPaginate {
		values.paginate = fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	}
	return fs, values
}

func flagUsage(param appleads.ParamSpec) string {
	usage := strings.ReplaceAll(param.Name, "-", " ")
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

func executeEndpoint(ctx context.Context, spec appleads.EndpointSpec, flags endpointFlagValues) error {
	if flags.confirm != nil && !*flags.confirm {
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
	body, err := readBody(spec, flags)
	if err != nil {
		return err
	}

	client, err := resolveClient(ctx, flags.common, spec.RequiresOrg)
	if err != nil {
		return fmt.Errorf("ads: %w", err)
	}

	requestCtx, cancel := requestContext(ctx)
	defer cancel()

	var result appleads.RawResponse
	if flags.paginate != nil && *flags.paginate {
		startOffset := intValue(flags.queryInts["offset"])
		pageSize := intValue(flags.queryInts["limit"])
		result, err = client.PaginateAll(requestCtx, spec, pathParams, query, startOffset, pageSize, body)
	} else {
		result, err = client.Do(requestCtx, spec, pathParams, query, body)
	}
	if err != nil {
		return fmt.Errorf("ads %s: %w", strings.Join(spec.CommandPath, " "), err)
	}
	return shared.PrintOutput(result, *flags.output.Output, *flags.output.Pretty)
}

func collectPathParams(spec appleads.EndpointSpec, flags endpointFlagValues) (map[string]string, error) {
	params := map[string]string{}
	for _, param := range spec.PathParams {
		ptr := flags.pathStrings[param.Name]
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
				if provided && param.Name == "limit" {
					return nil, fmt.Errorf("--limit must be between 1 and %d", appleads.MaxPageLimit(spec))
				}
				continue
			}
			if raw < 0 {
				return nil, fmt.Errorf("--%s must be >= 0", param.Flag)
			}
			if param.Name == "limit" {
				maxLimit := appleads.MaxPageLimit(spec)
				if raw < 1 || raw > maxLimit {
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
	return query, nil
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

func intValue(ptr *int) int {
	if ptr == nil {
		return 0
	}
	return *ptr
}
