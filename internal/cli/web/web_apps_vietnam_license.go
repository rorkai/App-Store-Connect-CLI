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

func newWebAppsVietnamLicenseCommand(d *shared.Deps) *ffcli.Command {
	const command = "web apps compliance vietnam-license view"
	fs := d.NewFlagSet(command, flag.ExitOnError)
	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID)")
	authFlags := bindWebSessionFlags(fs)
	output := d.BindOutputFlags(fs)
	view := &ffcli.Command{
		Name:       "view",
		ShortUsage: "asc " + command + " --app APP_ID [flags]",
		ShortHelp:  "Read the Vietnam game license detail for an app.",
		LongHelp:   "Read the Vietnam game license detail shown in App Compliance using a web session.\n\nExamples:\n  asc " + command + " --app APP_ID\n  asc " + command + " --app APP_ID --output json",
		FlagSet:    fs,
		UsageFunc:  d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return d.UsageErrorf("unexpected argument(s): %s", strings.Join(args, " "))
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
			err = withWebSpinnerWith(d, "Fetching Vietnam game license", func() error {
				var err error
				result, err = newWebClientFn(session).GetAppVietnamGameLicense(requestCtx, resolvedAppID)
				return err
			})
			if err != nil {
				return withWebAuthHint(err, command)
			}
			var document struct {
				Data struct {
					ID         string `json:"id"`
					Attributes struct {
						License    json.RawMessage `json:"license"`
						LicenseURL json.RawMessage `json:"licenseUrl"`
					} `json:"attributes"`
				} `json:"data"`
			}
			if err := json.Unmarshal(result, &document); err != nil {
				return fmt.Errorf("failed to parse Vietnam game license response: %w", err)
			}
			row := []string{document.Data.ID, summarizeJSONValue(document.Data.Attributes.License), summarizeJSONValue(document.Data.Attributes.LicenseURL)}
			return d.PrintOutputRows(result, *output.Output, *output.Pretty, []string{"id", "license", "license_url"}, [][]string{row})
		},
	}
	return &ffcli.Command{
		Name:        "vietnam-license",
		ShortUsage:  "asc web apps compliance vietnam-license <subcommand> [flags]",
		ShortHelp:   "Read Vietnam game license information via a web session.",
		FlagSet:     d.NewFlagSet("web apps compliance vietnam-license", flag.ExitOnError),
		UsageFunc:   d.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{view},
		Exec:        func(context.Context, []string) error { return flag.ErrHelp },
	}
}
