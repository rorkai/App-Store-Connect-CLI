package builds

import (
	"context"
	"flag"
	"fmt"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// BuildsFindCommand resolves a build by build number.
func BuildsFindCommand() *ffcli.Command {
	fs := flag.NewFlagSet("find", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID, bundle ID, or exact app name (required, or ASC_APP_ID env)")
	buildNumber := fs.String("build-number", "", "Build number (CFBundleVersion) to find")
	platform := fs.String("platform", "IOS", "Platform filter: IOS, MAC_OS, TV_OS, VISION_OS")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "find",
		ShortUsage: "asc builds find --app APP_ID --build-number BUILD_NUMBER [flags]",
		ShortHelp:  "Find a build by build number.",
		LongHelp: `Find a build by build number.

This command resolves a build by app + CFBundleVersion and returns the latest
matching build for the selected platform.

Examples:
  asc builds find --app "123456789" --build-number "42"
  asc builds find --app "123456789" --build-number "42" --platform IOS
  asc builds find --app "123456789" --build-number "42" --output table`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			resolvedAppID := shared.ResolveAppID(*appID)
			if resolvedAppID == "" {
				return shared.UsageError("--app is required (or set ASC_APP_ID)")
			}

			buildNumberValue := strings.TrimSpace(*buildNumber)
			if buildNumberValue == "" {
				return shared.UsageError("--build-number is required")
			}

			normalizedPlatform, err := shared.NormalizeAppStoreVersionPlatform(*platform)
			if err != nil {
				return shared.UsageError(err.Error())
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("builds find: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			buildResp, err := resolveBuildForWait(requestCtx, client, "", resolvedAppID, buildNumberValue, normalizedPlatform)
			if err != nil {
				return fmt.Errorf("builds find: %w", err)
			}

			return shared.PrintOutput(buildResp, *output.Output, *output.Pretty)
		},
	}
}
