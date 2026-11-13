package web

import (
	"context"
	"flag"
	"fmt"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

var (
	listWebAppDeclarationsFn = func(ctx context.Context, client *webcore.Client, accountID, appID string) ([]webcore.AppDeclaration, error) {
		return client.ListAppDeclarations(ctx, accountID, appID)
	}
	viewWebMedicalDeviceDeclarationFn = func(ctx context.Context, client *webcore.Client, accountID, appID string) (*webcore.MedicalDeviceDeclarationState, error) {
		return client.GetMedicalDeviceDeclaration(ctx, accountID, appID)
	}
)

const declarationsLongHelp = `WEB SESSION WORKFLOWS

Read the App Store Regulations & Permits declarations App Store Connect tracks
for an app under App Information. Apple does not expose these declarations on
the public App Store Connect API, so this command uses the same web-session
compliance-form endpoint the website uses.

Requirements Apple reports here include the regulated medical device
declaration and any other declaration Apple requires for the app. A required
requirement that is still at ` + "`PENDING_COLLECTION`" + ` blocks App Store submission;
optional uncollected rows do not.

`

// WebAppsDeclarationsCommand returns the app declarations command group.
func WebAppsDeclarationsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web apps declarations", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "declarations",
		ShortUsage: "asc web apps declarations <subcommand> [flags]",
		ShortHelp:  "Read App Store Regulations & Permits declarations via web sessions.",
		LongHelp:   declarationsLongHelp,
		FlagSet:    fs,
		UsageFunc:  shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			WebAppsDeclarationsListCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// WebAppsDeclarationsListCommand lists the compliance declarations for an app.
func WebAppsDeclarationsListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web apps declarations list", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID)")
	authFlags := bindWebSessionFlags(fs)
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc web apps declarations list --app APP_ID [flags]",
		ShortHelp:  "List App Store Regulations & Permits declarations for an app.",
		LongHelp: declarationsLongHelp + `Examples:
  asc web apps declarations list --app "6748252780"
  asc web apps declarations list --app "6748252780" --output json

`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			resolvedAppID := strings.TrimSpace(shared.ResolveAppID(*appID))
			if resolvedAppID == "" {
				return shared.UsageError("--app is required (or set ASC_APP_ID)")
			}

			accountID, client, requestCtx, cancel, err := resolveWebComplianceClient(ctx, authFlags, "web apps declarations list")
			defer cancel()
			if err != nil {
				return err
			}

			declarations := []webcore.AppDeclaration{}
			err = withWebSpinner("Fetching app declarations", func() error {
				result, err := listWebAppDeclarationsFn(requestCtx, client, accountID, resolvedAppID)
				if err != nil {
					return err
				}
				declarations = append(declarations, result...)
				return nil
			})
			if err != nil {
				return withWebAuthHint(err, "web apps declarations list")
			}

			outputList := webAppDeclarationListOutput(declarations)
			return shared.PrintOutput(&outputList, *output.Output, *output.Pretty)
		},
	}
}

// WebAppsMedicalDeviceViewCommand reads the regulated medical device declaration.
func WebAppsMedicalDeviceViewCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web apps medical-device view", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID)")
	authFlags := bindWebSessionFlags(fs)
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "view",
		ShortUsage: "asc web apps medical-device view --app APP_ID [flags]",
		ShortHelp:  "Read the regulated medical device declaration via web API.",
		LongHelp: `WEB SESSION WORKFLOWS

Read the stored regulated medical device declaration for an app.

The reported declaration is "no" or "yes" once the form has been answered, and
empty while the declaration is still outstanding.

Examples:
  asc web apps medical-device view --app "6748252780"

`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			resolvedAppID := strings.TrimSpace(shared.ResolveAppID(*appID))
			if resolvedAppID == "" {
				return shared.UsageError("--app is required (or set ASC_APP_ID)")
			}

			accountID, client, requestCtx, cancel, err := resolveWebComplianceClient(ctx, authFlags, "web apps medical-device view")
			defer cancel()
			if err != nil {
				return err
			}

			var state *webcore.MedicalDeviceDeclarationState
			err = withWebSpinner("Fetching regulated medical device declaration", func() error {
				var err error
				state, err = viewWebMedicalDeviceDeclarationFn(requestCtx, client, accountID, resolvedAppID)
				return err
			})
			if err != nil {
				return withWebAuthHint(err, "web apps medical-device view")
			}
			if state == nil {
				return fmt.Errorf("web apps medical-device view failed: missing declaration state")
			}

			return shared.PrintOutput(webMedicalDeviceDeclarationStateOutput(state), *output.Output, *output.Pretty)
		},
	}
}

func webAppDeclarationListOutput(declarations []webcore.AppDeclaration) asc.WebAppDeclarationList {
	out := make(asc.WebAppDeclarationList, 0, len(declarations))
	for _, declaration := range declarations {
		out = append(out, asc.WebAppDeclaration{
			AppID:           declaration.AppID,
			RequirementID:   declaration.RequirementID,
			RequirementName: declaration.RequirementName,
			Ref:             declaration.Ref,
			Status:          declaration.Status,
			FormID:          declaration.FormID,
			DateSigned:      declaration.DateSigned,
			Required:        declaration.Required,
		})
	}
	return out
}

func webMedicalDeviceDeclarationStateOutput(state *webcore.MedicalDeviceDeclarationState) *asc.WebMedicalDeviceDeclarationState {
	if state == nil {
		return nil
	}
	return &asc.WebMedicalDeviceDeclarationState{
		AppID:              state.AppID,
		RequirementID:      state.RequirementID,
		RequirementName:    state.RequirementName,
		Status:             state.Status,
		FormID:             state.FormID,
		Required:           state.Required,
		Declaration:        state.Declaration,
		CountriesOrRegions: state.CountriesOrRegions,
	}
}

// resolveWebComplianceClient resolves the web session and account id shared by
// the compliance-form commands. Authentication uses the parent context; the
// returned request context starts only after login finishes.
func resolveWebComplianceClient(ctx context.Context, authFlags webSessionFlags, command string) (string, *webcore.Client, context.Context, context.CancelFunc, error) {
	session, requestCtx, cancel, err := resolveWebSessionForCommand(ctx, authFlags)
	if err != nil {
		return "", nil, requestCtx, cancel, err
	}

	accountID := strings.TrimSpace(session.PublicProviderID)
	if accountID == "" {
		return "", nil, requestCtx, cancel, fmt.Errorf("%s failed: web session is missing public provider/account id (run 'asc web auth login')", command)
	}
	return accountID, newWebClientFn(session), requestCtx, cancel, nil
}
