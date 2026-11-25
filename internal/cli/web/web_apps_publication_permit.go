package web

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func newWebAppsChinaPublicationPermitCommand(d *shared.Deps) *ffcli.Command {
	return &ffcli.Command{
		Name:        "china-publication-permit",
		ShortUsage:  "asc web apps compliance china-publication-permit <subcommand> [flags]",
		ShortHelp:   "Read the China mainland publication permit relationship.",
		FlagSet:     d.NewFlagSet("web apps compliance china-publication-permit", flag.ExitOnError),
		UsageFunc:   d.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{newWebAppsChinaPublicationPermitViewCommand(d)},
		Exec:        func(context.Context, []string) error { return flag.ErrHelp },
	}
}

func newWebAppsChinaPublicationPermitViewCommand(d *shared.Deps) *ffcli.Command {
	const command = "web apps compliance china-publication-permit view"
	fs := d.NewFlagSet(command, flag.ExitOnError)
	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID)")
	authFlags := bindWebSessionFlags(fs)
	output := d.BindOutputFlags(fs)
	return &ffcli.Command{
		Name:       "view",
		ShortUsage: "asc " + command + " --app APP_ID [flags]",
		ShortHelp:  "Read the China mainland publication permit for an app.",
		LongHelp:   "JSON output preserves the app resource, its publication permit relationship, and any included permit data returned by Apple. Table and Markdown output report whether a permit is present.\n",
		FlagSet:    fs,
		UsageFunc:  d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return d.UsageError("unexpected positional arguments")
			}
			resolvedAppID := strings.TrimSpace(d.ResolveAppID(*appID))
			if resolvedAppID == "" {
				return d.UsageError("--app is required (or set ASC_APP_ID)")
			}
			if _, err := shared.ValidateOutputFormat(*output.Output, *output.Pretty); err != nil {
				return d.UsageError(err.Error())
			}
			session, requestCtx, cancel, err := resolveWebSessionForCommandWith(ctx, d, authFlags)
			defer cancel()
			if err != nil {
				return err
			}
			var result json.RawMessage
			err = withWebSpinnerWith(d, "Fetching publication permit", func() error {
				var err error
				result, err = newWebClientFn(session).GetAppPublicationPermit(requestCtx, resolvedAppID)
				return err
			})
			if err != nil {
				return withWebAuthHint(err, command)
			}
			var document struct {
				Data struct {
					ID            string `json:"id"`
					Relationships struct {
						Permit *struct {
							Data json.RawMessage `json:"data"`
						} `json:"appPublicationPermit"`
					} `json:"relationships"`
				} `json:"data"`
			}
			if err := json.Unmarshal(result, &document); err != nil {
				return fmt.Errorf("failed to parse publication permit response: %w", err)
			}
			permit := document.Data.Relationships.Permit
			if document.Data.ID == "" || permit == nil || len(permit.Data) == 0 {
				return fmt.Errorf("invalid publication permit response: missing app or permit relationship")
			}
			status := "present"
			if string(permit.Data) == "null" {
				status = "absent"
			}
			return d.PrintOutputRows(result, *output.Output, *output.Pretty, []string{"app_id", "publication_permit"}, [][]string{{document.Data.ID, status}})
		},
	}
}
