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

// ActorsCommand returns the actors command with subcommands.
func ActorsCommand() *ffcli.Command {
	return NewActorsCommand(shared.DefaultDeps())
}

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
	var fieldsValue []string

	return shared.BuildPaginatedListCommand(shared.PaginatedListCommandConfig{
		FlagSetName: "list",
		Name:        "list",
		ShortUsage:  "asc actors list --id ACTOR_ID[,ACTOR_ID...] [flags]",
		ShortHelp:   "List actors by ID.",
		LongHelp: `List actors by ID.

Examples:
  asc actors list --id "ACTOR_ID"
  asc actors list --id "ID1,ID2" --fields "actorType,userEmail"
  asc actors list --id "ID1,ID2" --paginate`,
		ParentUsage:          "Actor ID(s), comma-separated",
		RequiredDiagnostic:   true,
		PaginateIgnoresLimit: true,
		NoSpinner:            true,
		NextConflicts:        []string{"id", "fields", "limit"},
		NextBeforeLimit:      true,
		Flags: func(fs *flag.FlagSet) func() error {
			fields := fs.String("fields", "", "Fields to include: "+strings.Join(actorFieldsList(), ", "))
			return func() (err error) {
				if fieldsValue, err = normalizeActorFields(*fields); err != nil {
					return shared.UsageErrorf("actors list: %v", err)
				}
				return nil
			}
		},
		ErrorPrefix:      "actors list",
		FetchErrorPrefix: "actors list: failed to fetch",
		Deps:             d,
		FetchPage: func(ctx context.Context, client *asc.Client, ids string, limit int, next string) (asc.PaginatedResponse, error) {
			opts := []asc.ActorsOption{
				asc.WithActorsIDs(shared.SplitCSV(ids)),
				asc.Limit[asc.ActorsOption](limit),
				asc.NextURL[asc.ActorsOption](next),
			}
			if len(fieldsValue) > 0 {
				opts = append(opts, asc.WithActorsFields(fieldsValue))
			}
			return client.GetActors(ctx, opts...)
		},
	})
}

// newActorsViewCommand returns the actors view subcommand built on d.
func newActorsViewCommand(d *shared.Deps) *ffcli.Command {
	var fieldsValue []string

	return shared.BuildIDGetCommand(shared.IDGetCommandConfig{
		FlagSetName: "view",
		Name:        "view",
		ShortUsage:  "asc actors view --id ACTOR_ID [flags]",
		ShortHelp:   "View an actor by ID.",
		LongHelp: `View an actor by ID.

Examples:
  asc actors view --id "ACTOR_ID"
  asc actors view --id "ACTOR_ID" --fields "actorType,userEmail"`,
		IDUsage:            "Actor ID",
		IDType:             "actors",
		RequiredDiagnostic: true,
		Flags: func(fs *flag.FlagSet) func() error {
			fields := fs.String("fields", "", "Fields to include: "+strings.Join(actorFieldsList(), ", "))
			return func() (err error) {
				if fieldsValue, err = normalizeActorFields(*fields); err != nil {
					return shared.UsageErrorf("actors view: %v", err)
				}
				return nil
			}
		},
		ErrorPrefix:      "actors view",
		FetchErrorPrefix: "actors view: failed to fetch",
		Deps:             d,
		Fetch: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.GetActor(ctx, id, fieldsValue)
		},
	})
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
