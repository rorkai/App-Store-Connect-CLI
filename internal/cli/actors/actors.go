package actors

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// ActorsCommand returns the actors command with subcommands.
func ActorsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("actors", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "actors",
		ShortUsage: "asc actors <subcommand> [flags]",
		ShortHelp:  "Lookup actors (users, API keys) by ID.",
		LongHelp: `Lookup actor records for audit fields like submittedByActor.

Examples:
  asc actors list --id "ACTOR_ID"
  asc actors get --id "ACTOR_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			ActorsListCommand(),
			ActorsGetCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// ActorsListCommand returns the actors list subcommand.
func ActorsListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("list", flag.ExitOnError)

	ids := fs.String("id", "", "Actor ID(s), comma-separated")
	fields := fs.String("fields", "", "Fields to include: "+strings.Join(actorFieldsList(), ", "))
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc actors list --id ACTOR_ID[,ACTOR_ID...] [flags]",
		ShortHelp:  "List actors by ID.",
		LongHelp: `List actors by ID.

Examples:
  asc actors list --id "ACTOR_ID"
  asc actors list --id "ID1,ID2" --fields "actorType,userEmail"
  asc actors list --id "ID1,ID2" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("actors list: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("actors list: %w", err)
			}
			if strings.TrimSpace(*ids) == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			fieldsValue, err := normalizeActorFields(*fields)
			if err != nil {
				return fmt.Errorf("actors list: %w", err)
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("actors list: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.ActorsOption{
				asc.WithActorsIDs(shared.SplitCSV(*ids)),
				asc.WithActorsLimit(*limit),
				asc.WithActorsNextURL(*next),
			}
			if len(fieldsValue) > 0 {
				opts = append(opts, asc.WithActorsFields(fieldsValue))
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithActorsLimit(200))
				firstPage, err := client.GetActors(requestCtx, paginateOpts...)
				if err != nil {
					return fmt.Errorf("actors list: failed to fetch: %w", err)
				}

				actors, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetActors(ctx, asc.WithActorsNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("actors list: %w", err)
				}

				return shared.PrintOutput(actors, *output, *pretty)
			}

			actors, err := client.GetActors(requestCtx, opts...)
			if err != nil {
				return fmt.Errorf("actors list: failed to fetch: %w", err)
			}

			return shared.PrintOutput(actors, *output, *pretty)
		},
	}
}

// ActorsGetCommand returns the actors get subcommand.
func ActorsGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("get", flag.ExitOnError)

	id := fs.String("id", "", "Actor ID")
	fields := fs.String("fields", "", "Fields to include: "+strings.Join(actorFieldsList(), ", "))
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc actors get --id ACTOR_ID [flags]",
		ShortHelp:  "Get an actor by ID.",
		LongHelp: `Get an actor by ID.

Examples:
  asc actors get --id "ACTOR_ID"
  asc actors get --id "ACTOR_ID" --fields "actorType,userEmail"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			idValue := strings.TrimSpace(*id)
			if idValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			fieldsValue, err := normalizeActorFields(*fields)
			if err != nil {
				return fmt.Errorf("actors get: %w", err)
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("actors get: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			actor, err := client.GetActor(requestCtx, idValue, fieldsValue)
			if err != nil {
				return fmt.Errorf("actors get: failed to fetch: %w", err)
			}

			return shared.PrintOutput(actor, *output, *pretty)
		},
	}
}

func normalizeActorFields(value string) ([]string, error) {
	fields := shared.SplitCSV(value)
	if len(fields) == 0 {
		return nil, nil
	}

	allowed := map[string]struct{}{}
	for _, field := range actorFieldsList() {
		allowed[field] = struct{}{}
	}
	for _, field := range fields {
		if _, ok := allowed[field]; !ok {
			return nil, fmt.Errorf("--fields must be one of: %s", strings.Join(actorFieldsList(), ", "))
		}
	}

	return fields, nil
}

func actorFieldsList() []string {
	return []string{"actorType", "userFirstName", "userLastName", "userEmail", "apiKeyId"}
}
