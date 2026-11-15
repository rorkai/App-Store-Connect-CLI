package app_events

import (
	"context"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// newAppEventsRelationshipsCommand returns the app event links command.
func newAppEventsRelationshipsCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildPaginatedListCommand(shared.PaginatedListCommandConfig{
		Deps:       d,
		Name:       "links",
		ShortUsage: "asc app-events links --event-id \"EVENT_ID\" [flags]",
		ShortHelp:  "List localization relationships for an in-app event.",
		LongHelp: `List localization relationships for an in-app event.

Examples:
  asc app-events links --event-id "EVENT_ID"
  asc app-events links --event-id "EVENT_ID" --paginate`,
		ParentFlag:           "event-id",
		ParentUsage:          "App event ID",
		ParentType:           "appEvents",
		RequiredDiagnostic:   true,
		PaginateIgnoresLimit: true,
		NoSpinner:            true,
		FetchPage: func(ctx context.Context, client *asc.Client, id string, limit int, next string) (asc.PaginatedResponse, error) {
			return client.GetAppEventLocalizationsRelationships(ctx, id, asc.Limit[asc.LinkagesOption](limit), asc.NextURL[asc.LinkagesOption](next))
		},
	})
}
