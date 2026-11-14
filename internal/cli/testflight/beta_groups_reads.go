package testflight

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

type betaGroupReadConfig struct {
	resource    string
	apiResource string
	fieldsUsage string
	fieldValues []string
	// decode parses the raw envelope for table and markdown output; JSON
	// output prints Apple's response unmodified.
	decode func([]byte) (any, error)
}

// BetaGroupsBuildsCommand returns the beta-groups builds command group.
func BetaGroupsBuildsCommand() *ffcli.Command {
	return betaGroupReadGroupCommand(betaGroupReadConfig{
		resource:    "builds",
		apiResource: "builds",
		fieldsUsage: "Fields to include for builds, comma-separated",
		fieldValues: betaGroupBuildFieldsValues,
		decode:      decodeBetaGroupRead[asc.BuildsResponse],
	})
}

// BetaGroupsTestersCommand returns the beta-groups testers command group.
func BetaGroupsTestersCommand() *ffcli.Command {
	return betaGroupReadGroupCommand(betaGroupReadConfig{
		resource:    "testers",
		apiResource: "betaTesters",
		fieldsUsage: "Fields to include for testers, comma-separated",
		fieldValues: betaGroupTesterFieldsValues,
		decode:      decodeBetaGroupRead[asc.BetaTestersResponse],
	})
}

func decodeBetaGroupRead[T any](raw []byte) (any, error) {
	var response T
	if err := json.Unmarshal(raw, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return &response, nil
}

func betaGroupReadGroupCommand(config betaGroupReadConfig) *ffcli.Command {
	fs := flag.NewFlagSet(config.resource, flag.ExitOnError)

	return &ffcli.Command{
		Name:       config.resource,
		ShortUsage: "asc testflight beta-groups " + config.resource + " <subcommand> [flags]",
		ShortHelp:  "List " + config.resource + " in a beta group.",
		LongHelp: `List ` + config.resource + ` in a beta group.

Examples:
  asc testflight beta-groups ` + config.resource + ` list --group-id "GROUP_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			betaGroupReadListCommand(config),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

func betaGroupReadListCommand(config betaGroupReadConfig) *ffcli.Command {
	fs := flag.NewFlagSet(config.resource+" list", flag.ExitOnError)

	groupID := shared.BindResourceIDFlag(fs, "group-id", "betaGroups", "Beta group ID")
	fields := fs.String("fields", "", config.fieldsUsage+": "+strings.Join(config.fieldValues, ", "))
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := shared.BindOutputFlags(fs)

	command := "beta-groups " + config.resource + " list"
	// RejectNextFlagConflicts has no context to carry the tree's path
	// rewrites, so errors name the public path directly.
	errPrefix := "testflight groups " + config.resource + " list"
	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc testflight " + command + " [flags]",
		ShortHelp:  "List " + config.resource + " in a beta group.",
		LongHelp: `List ` + config.resource + ` in a beta group by group ID; no --app is needed.

--group-id, --fields, and --limit cannot be combined with --next: a links.next
URL already carries the query it was produced from. With --paginate, --limit
sets the page size (default 200).

Examples:
  asc testflight ` + command + ` --group-id "GROUP_ID"
  asc testflight ` + command + ` --group-id "GROUP_ID" --fields "` + strings.Join(config.fieldValues[:2], ",") + `"
  asc testflight ` + command + ` --group-id "GROUP_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return shared.WithDiagnostic(
					shared.UsageErrorCtx(ctx, errPrefix+": --limit must be between 1 and 200"),
					shared.DiagnosticInvalidInput,
					"--limit",
				)
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return shared.UsageErrorfCtx(ctx, "%s: %v", errPrefix, err)
			}
			if err := shared.RejectNextFlagConflicts(fs, *next, errPrefix, "group-id", "fields", "limit"); err != nil {
				return err
			}
			fieldValues, err := shared.NormalizeSelection(*fields, config.fieldValues, "--fields")
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %s\n", err.Error())
				return shared.InvalidValueUsageError("--fields")
			}
			groupValue := strings.TrimSpace(*groupID)
			nextValue := strings.TrimSpace(*next)
			if groupValue == "" && nextValue == "" {
				return shared.RequiredFlag("--group-id")
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("%s: %w", errPrefix, err)
			}

			target := nextValue
			if target == "" {
				target = "/v1/betaGroups/" + url.PathEscape(groupValue) + "/" + config.apiResource
				query := url.Values{}
				if len(fieldValues) > 0 {
					query.Set("fields["+config.apiResource+"]", strings.Join(fieldValues, ","))
				}
				pageLimit := *limit
				if *paginate && pageLimit == 0 {
					pageLimit = 200
				}
				if pageLimit > 0 {
					query.Set("limit", strconv.Itoa(pageLimit))
				}
				if encoded := query.Encode(); encoded != "" {
					target += "?" + encoded
				}
			}

			var raw []byte
			if *paginate {
				raw, err = client.RawPaginatedGET(ctx, target, shared.ContextWithTimeout)
			} else {
				requestCtx, cancel := shared.ContextWithTimeout(ctx)
				raw, err = client.RawRequest(requestCtx, "GET", target, nil)
				cancel()
			}
			if err != nil {
				return fmt.Errorf("%s: failed to fetch: %w", errPrefix, err)
			}

			if shared.NormalizeOutputFormat(*output.Output) == "json" {
				return shared.PrintOutput(json.RawMessage(raw), *output.Output, *output.Pretty)
			}
			resp, err := config.decode(raw)
			if err != nil {
				return fmt.Errorf("%s: %w", errPrefix, err)
			}
			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}
