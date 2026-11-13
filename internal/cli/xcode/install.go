package xcode

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	localxcode "github.com/rudrankriyam/App-Store-Connect-CLI/internal/xcode"
)

const xcodeInstallDefaultTimeout = 5 * time.Minute

var (
	runInstall         = localxcode.Install
	printInstallOutput = shared.PrintOutput
)

// XcodeInstallCommand returns the local connected-device installation command.
func XcodeInstallCommand() *ffcli.Command {
	fs := flag.NewFlagSet("xcode install", flag.ExitOnError)

	ipaPath := fs.String("ipa", "", "Path to an iOS IPA (required)")
	deviceID := fs.String("device-id", "", "Exact CoreDevice identifier from devicectl list devices (required)")
	timeout := fs.Duration("timeout", xcodeInstallDefaultTimeout, "Maximum duration for inspection, installation, and verification")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "install",
		ShortUsage: "asc xcode install --ipa FILE --device-id IDENTIFIER [flags]",
		ShortHelp:  "Install a local IPA on one connected iOS device.",
		LongHelp: `Install a local iOS IPA on one exact connected CoreDevice.

asc validates the IPA's embedded development or ad-hoc signing profile, finds
the exact physical device by its CoreDevice identifier, securely materializes
the single Payload/*.app bundle in a private temporary directory, and passes
that app bundle to the Xcode-provided devicectl tool. A successful command
also verifies the installed bundle's version and build. The original IPA is
never modified, and output never includes raw device identifiers or paths. The
default timeout is 5m; accepted values range from 5s through 10m.

Examples:
  asc xcode install --ipa .asc/artifacts/App.ipa --device-id COREDEVICE_IDENTIFIER
  asc xcode install --ipa .asc/artifacts/App.ipa --device-id COREDEVICE_IDENTIFIER --timeout 10m --output json
  asc xcode install --ipa .asc/artifacts/App.ipa --device-id COREDEVICE_IDENTIFIER --output table`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			options := localxcode.InstallOptions{
				IPAPath:  *ipaPath,
				DeviceID: *deviceID,
				Timeout:  *timeout,
			}
			if err := localxcode.ValidateInstallOptions(options); err != nil {
				return shared.UsageError(err.Error())
			}
			if _, err := shared.ValidateOutputFormat(*output.Output, *output.Pretty); err != nil {
				return shared.UsageError(err.Error())
			}

			result, installErr := runInstall(ctx, options)
			var inputErr *localxcode.InstallInputError
			if errors.As(installErr, &inputErr) {
				return shared.UsageError(inputErr.Error())
			}
			if result != nil {
				if outputErr := printInstallOutput(result, *output.Output, *output.Pretty); outputErr != nil {
					return shared.NewErrorWithCause(
						errors.New("xcode install output failed"),
						errors.Join(outputErr, installErr),
					)
				}
			}
			if installErr != nil {
				if result != nil {
					diagnostic := installFailureDiagnostic(result)
					fmt.Fprintf(os.Stderr, "Error: %s\n", diagnostic)
					return shared.NewStderrReportedError(shared.NewErrorWithCause(errors.New(diagnostic), installErr))
				}
				return shared.NewErrorWithCause(errors.New("xcode install failed"), installErr)
			}
			if result == nil {
				return fmt.Errorf("xcode install: installer returned no result")
			}
			return nil
		},
	}
}

func installFailureDiagnostic(result *asc.XcodeInstallResult) string {
	if result == nil || !knownInstallFailureStage(result.FailureStage) || !knownInstallFailureCode(result.FailureCode) {
		return "xcode install failed"
	}
	return fmt.Sprintf("xcode install failed at %s (%s)", result.FailureStage, result.FailureCode)
}

func knownInstallFailureStage(value string) bool {
	switch value {
	case "input", "profile-preflight", "device-discovery", "materialization", "install", "verification":
		return true
	default:
		return false
	}
}

func knownInstallFailureCode(value string) bool {
	switch value {
	case "unsupported_platform", "timeout", "cancelled", "operation_cancelled",
		"ipa_preflight_failed", "materialization_failed", "profile_not_installable",
		"devicectl_unavailable", "private_output_failed", "device_not_found",
		"device_unavailable", "profile_device_mismatch", "profile_device_membership_unavailable",
		"device_discovery_failed", "device_discovery_invalid_response", "install_failed",
		"install_invalid_response", "verification_failed":
		return true
	default:
		return false
	}
}
