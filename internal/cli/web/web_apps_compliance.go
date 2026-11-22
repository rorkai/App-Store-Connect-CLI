package web

import (
	"context"
	"flag"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

var listWebAppComplianceRequirementsFn = func(ctx context.Context, client *webcore.Client, accountID, appID string) ([]webcore.AppComplianceRequirement, error) {
	return client.ListAppComplianceRequirements(ctx, accountID, appID)
}

const complianceLongHelp = `WEB SESSION WORKFLOWS

Read the requirements App Store Connect tracks for an app on its App Compliance
page, such as the Security Assessment and the AI Generated or Synthetic Content
declaration for China mainland. Apple does not expose these requirements on the
public App Store Connect API, so this command uses the same web-session
endpoint the website uses.

Every requirement is listed whether or not it was answered. ` + "`PENDING_COLLECT`" + `
means it has no answer yet, ` + "`COLLECTED_VERIFY_PENDING`" + ` means Apple has not
verified the answer, and ` + "`COLLECTED_VERIFY_NOT_NECESSARY`" + ` means the answer
needs no verification. Countries and regions are ISO 3166-1 alpha-3 codes.

The regulated medical device declaration is a separate requirement; read it
with ` + "`asc web apps declarations list`" + `.

`

// newWebAppsComplianceCommand returns the App Compliance command group.
func newWebAppsComplianceCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("web apps compliance", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "compliance",
		ShortUsage: "asc web apps compliance <subcommand> [flags]",
		ShortHelp:  "Read App Compliance requirements via web sessions.",
		LongHelp:   complianceLongHelp,
		FlagSet:    fs,
		UsageFunc:  d.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newWebAppsComplianceListCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newWebAppsComplianceListCommand lists the App Compliance requirements for an app.
func newWebAppsComplianceListCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("web apps compliance list", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID)")
	authFlags := bindWebSessionFlags(fs)
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc web apps compliance list --app APP_ID [flags]",
		ShortHelp:  "List App Compliance requirements for an app.",
		LongHelp: complianceLongHelp + `Examples:
  asc web apps compliance list --app "6748252780"
  asc web apps compliance list --app "6748252780" --output json

`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			resolvedAppID := strings.TrimSpace(d.ResolveAppID(*appID))
			if resolvedAppID == "" {
				return d.UsageError("--app is required (or set ASC_APP_ID)")
			}

			accountID, client, requestCtx, cancel, err := resolveWebComplianceClient(ctx, d, authFlags, "web apps compliance list")
			defer cancel()
			if err != nil {
				return err
			}

			var requirements []webcore.AppComplianceRequirement
			err = withWebSpinnerWith(d, "Fetching app compliance requirements", func() error {
				var err error
				requirements, err = listWebAppComplianceRequirementsFn(requestCtx, client, accountID, resolvedAppID)
				return err
			})
			if err != nil {
				return withWebAuthHint(err, "web apps compliance list")
			}

			outputList := make(asc.WebAppComplianceRequirementList, 0, len(requirements))
			for _, requirement := range requirements {
				outputList = append(outputList, asc.WebAppComplianceRequirement(requirement))
			}
			return d.PrintOutput(&outputList, *output.Output, *output.Pretty)
		},
	}
}
