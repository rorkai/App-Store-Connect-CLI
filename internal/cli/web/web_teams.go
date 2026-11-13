package web

import (
	"context"
	"flag"
	"fmt"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

var listDeveloperTeamsFn = func(ctx context.Context, client *webcore.Client) (*asc.WebDeveloperTeamsResult, error) {
	return client.ListDeveloperPortalTeams(ctx)
}

// WebTeamsCommand returns the Developer Portal teams command group.
func WebTeamsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web teams", flag.ExitOnError)
	return &ffcli.Command{
		Name:       "teams",
		ShortUsage: "asc web teams <subcommand> [flags]",
		ShortHelp:  "List Developer Portal teams via a web session.",
		LongHelp: `List the Apple Developer teams that the web session's Apple Account belongs to.

Examples:
  asc web teams list
`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			WebTeamsListCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// WebTeamsListCommand lists every Developer Portal team on the web session.
func WebTeamsListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web teams list", flag.ExitOnError)
	authFlags := bindWebSessionFlags(fs)
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc web teams list [flags]",
		ShortHelp:  "List Developer Portal teams via a web session.",
		LongHelp: `List every Apple Developer team that the web session's Apple Account belongs to,
with each team's ID, name, and status as Developer Portal reports them.

Use a listed team ID or exact team name as --developer-team on other asc web
commands. This command only reads the team list; it does not select a team.

Examples:
  asc web teams list
  asc web teams list --output json
`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if _, err := shared.ValidateOutputFormat(*output.Output, *output.Pretty); err != nil {
				return shared.UsageError(err.Error())
			}

			session, requestCtx, cancel, err := resolveWebSessionForCommand(ctx, authFlags)
			defer cancel()
			if err != nil {
				return withWebAuthHint(err, "web teams list")
			}

			var result *asc.WebDeveloperTeamsResult
			err = withWebSpinner("Loading Developer Portal teams", func() error {
				var listErr error
				result, listErr = listDeveloperTeamsFn(requestCtx, newWebClientFn(session))
				return listErr
			})
			if err != nil {
				return withWebAuthHint(err, "web teams list")
			}
			if result == nil {
				return fmt.Errorf("web teams list failed: missing list result")
			}
			persistDeveloperPortalSession(session)

			return shared.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}
