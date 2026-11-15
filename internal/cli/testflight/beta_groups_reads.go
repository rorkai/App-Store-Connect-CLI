package testflight

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/url"
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

// newBetaGroupsBuildsCommand returns the beta-groups builds command group built on d.
func newBetaGroupsBuildsCommand(d *shared.Deps) *ffcli.Command {
	return betaGroupReadGroupCommand(d, betaGroupReadConfig{
		resource:    "builds",
		apiResource: "builds",
		fieldsUsage: "Fields to include for builds, comma-separated",
		fieldValues: betaGroupBuildFieldsValues,
		decode:      decodeBetaGroupRead[asc.BuildsResponse],
	})
}

// newBetaGroupsTestersCommand returns the beta-groups testers command group built on d.
func newBetaGroupsTestersCommand(d *shared.Deps) *ffcli.Command {
	return betaGroupReadGroupCommand(d, betaGroupReadConfig{
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

func betaGroupReadGroupCommand(d *shared.Deps, config betaGroupReadConfig) *ffcli.Command {
	fs := d.NewFlagSet(config.resource, flag.ExitOnError)

	return &ffcli.Command{
		Name:       config.resource,
		ShortUsage: "asc testflight beta-groups " + config.resource + " <subcommand> [flags]",
		ShortHelp:  "List " + config.resource + " in a beta group.",
		LongHelp: `List ` + config.resource + ` in a beta group.

Examples:
  asc testflight beta-groups ` + config.resource + ` list --group-id "GROUP_ID"`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			betaGroupReadListCommand(d, config),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

func betaGroupReadListCommand(d *shared.Deps, config betaGroupReadConfig) *ffcli.Command {
	fs := d.NewFlagSet(config.resource+" list", flag.ExitOnError)

	groupID := shared.BindResourceIDFlag(fs, "group-id", "betaGroups", "Beta group ID")
	fields := fs.String("fields", "", config.fieldsUsage+": "+strings.Join(config.fieldValues, ", "))
	limit := fs.Int("limit", 0, shared.LimitFlagUsage(200))
	next := fs.String("next", "", shared.NextFlagUsage)
	paginate := fs.Bool("paginate", false, shared.PaginateFlagUsage)
	output := d.BindOutputFlags(fs)

	command := "beta-groups " + config.resource + " list"
	// RejectNextFlagConflicts has no context to carry the tree's path
	// rewrites, so errors name the public path directly.
	errPrefix := "testflight groups " + config.resource + " list"
	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc testflight " + command + " [flags]",
		ShortHelp:  "List " + config.resource + " in a beta group.",
		LongHelp: `List ` + config.resource + ` in a beta group by group ID; no --app is needed.

--fields and --limit cannot be combined with --next: a links.next
URL already carries the query it was produced from. With --paginate, --limit
sets the page size (default 200).

Examples:
  asc testflight ` + command + ` --group-id "GROUP_ID"
  asc testflight ` + command + ` --group-id "GROUP_ID" --fields "` + strings.Join(config.fieldValues[:2], ",") + `"
  asc testflight ` + command + ` --group-id "GROUP_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := d.ValidatePaginationFlags(ctx, fs, errPrefix, 200, "fields", "limit"); err != nil {
				return err
			}
			groupValue := strings.TrimSpace(*groupID)
			nextValue := strings.TrimSpace(*next)
			if groupValue == "" && nextValue == "" {
				return d.RequiredFlag("--group-id")
			}
			fieldValues, err := shared.NormalizeSelection(*fields, config.fieldValues, "--fields")
			if err != nil {
				fmt.Fprintf(d.IO.ErrOut, "Error: %s\n", err.Error())
				return shared.InvalidValueUsageError("--fields")
			}

			client, err := d.Client()
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
				raw, err = client.RawPaginatedGET(ctx, target, d.Timeout)
			} else {
				requestCtx, cancel := d.Timeout(ctx)
				raw, err = client.RawRequest(requestCtx, "GET", target, nil)
				cancel()
			}
			if err != nil {
				return fmt.Errorf("%s: failed to fetch: %w", errPrefix, err)
			}

			if shared.NormalizeOutputFormat(*output.Output) == "json" {
				return d.PrintOutput(json.RawMessage(raw), *output.Output, *output.Pretty)
			}
			resp, err := config.decode(raw)
			if err != nil {
				return fmt.Errorf("%s: %w", errPrefix, err)
			}
			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}
