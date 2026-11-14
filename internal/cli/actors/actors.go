package actors

import (
	"context"
	"flag"
	"fmt"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// NewActorsCommand returns the actors command with subcommands built on d.
func NewActorsCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("actors", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "actors",
		ShortUsage: "asc actors <subcommand> [flags]",
		ShortHelp:  "Lookup actors (users, API keys) by ID.",
		LongHelp: `Lookup actor records for audit fields like submittedByActor.

Examples:
  asc actors list --id "ACTOR_ID"
  asc actors view --id "ACTOR_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newActorsListCommand(d),
			newActorsViewCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newActorsListCommand returns the actors list subcommand built on d.
func newActorsListCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("list", flag.ExitOnError)

	ids := fs.String("id", "", "Actor ID(s), comma-separated")
	fields := fs.String("fields", "", "Fields to include: "+strings.Join(actorFieldsList(), ", "))
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := d.BindOutputFlags(fs)

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
			if err := shared.ValidateNextURL(*next); err != nil {
				return shared.UsageErrorf("actors list: %v", err)
			}
			if err := shared.RejectNextFlagConflicts(fs, *next, "actors list", "id", "fields", "limit"); err != nil {
				return err
			}
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return shared.UsageError("actors list: --limit must be between 1 and 200")
			}
			if strings.TrimSpace(*ids) == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(d.IO.ErrOut, "Error: --id is required")
				return shared.MissingRequiredUsageError("--id")
			}

			fieldsValue, err := normalizeActorFields(*fields)
			if err != nil {
				return shared.UsageErrorf("actors list: %v", err)
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("actors list: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			opts := []asc.ActorsOption{
				asc.WithActorsIDs(shared.SplitCSV(*ids)),
				asc.Limit[asc.ActorsOption](*limit),
				asc.NextURL[asc.ActorsOption](*next),
			}
			if len(fieldsValue) > 0 {
				opts = append(opts, asc.WithActorsFields(fieldsValue))
			}

			if *paginate {
				paginateOpts := append(opts, asc.Limit[asc.ActorsOption](200))
				firstPage, err := client.GetActors(requestCtx, paginateOpts...)
				if err != nil {
					return fmt.Errorf("actors list: failed to fetch: %w", err)
				}

				actors, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetActors(ctx, asc.NextURL[asc.ActorsOption](nextURL))
				})
				if err != nil {
					return fmt.Errorf("actors list: %w", err)
				}

				return d.PrintOutput(actors, *output.Output, *output.Pretty)
			}

			actors, err := client.GetActors(requestCtx, opts...)
			if err != nil {
				return fmt.Errorf("actors list: failed to fetch: %w", err)
			}

			return d.PrintOutput(actors, *output.Output, *output.Pretty)
		},
	}
}

// newActorsViewCommand returns the actors view subcommand built on d.
func newActorsViewCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("view", flag.ExitOnError)

	id := shared.BindResourceIDFlag(fs, "id", "actors", "Actor ID")
	fields := fs.String("fields", "", "Fields to include: "+strings.Join(actorFieldsList(), ", "))
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "view",
		ShortUsage: "asc actors view --id ACTOR_ID [flags]",
		ShortHelp:  "View an actor by ID.",
		LongHelp: `View an actor by ID.

Examples:
  asc actors view --id "ACTOR_ID"
  asc actors view --id "ACTOR_ID" --fields "actorType,userEmail"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			idValue := strings.TrimSpace(*id)
			if idValue == "" {
				fmt.Fprintln(d.IO.ErrOut, "Error: --id is required")
				return shared.MissingRequiredUsageError("--id")
			}

			fieldsValue, err := normalizeActorFields(*fields)
			if err != nil {
				return shared.UsageErrorf("actors view: %v", err)
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("actors view: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			actor, err := client.GetActor(requestCtx, idValue, fieldsValue)
			if err != nil {
				return fmt.Errorf("actors view: failed to fetch: %w", err)
			}

			return d.PrintOutput(actor, *output.Output, *output.Pretty)
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
