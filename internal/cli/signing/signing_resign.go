package signing

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"runtime"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// signingResignOptions describes the local inputs for an IPA re-signing run.
// The output path is deliberately separate from the renderer format.
type signingResignOptions struct {
	IPAPath              string
	OutputPath           string
	IdentityPath         string
	IdentityPasswordPath string
	ProfilesManifestPath string
	RebaseTeamClaims     bool
}

// SigningResignCommand returns the local IPA re-signing command.
func SigningResignCommand() *ffcli.Command {
	fs := flag.NewFlagSet("resign", flag.ExitOnError)
	ipaPath := fs.String("ipa", "", "Path to the existing IPA input (required)")
	outputPath := fs.String("output", "", "Path for the newly re-signed IPA (required)")
	identityPath := fs.String("identity", "", "Path to a PKCS#12 signing identity (required)")
	identityPasswordPath := fs.String("identity-password-file", "", "Path to a file containing the PKCS#12 password")
	profilesManifestPath := fs.String("profiles-manifest", "", "Path to the strict bundle-to-profile manifest (required)")
	rebaseTeamClaims := fs.Bool("rebase-team-claims", false, "Rebase allowlisted team-prefix claims; changing KVS selects a different data namespace")
	format := shared.BindOutputFlagsWith(fs, "format", shared.DefaultOutputFormat(), "Output format: json, table, markdown")

	return &ffcli.Command{
		Name:       "resign",
		ShortUsage: "asc signing resign --ipa PATH --output PATH --identity PATH --profiles-manifest PATH [--rebase-team-claims] [flags]",
		ShortHelp:  "Re-sign an existing iOS IPA with complete nested-target profile mappings.",
		LongHelp: `Re-sign an existing iOS IPA into a new destination.

The command validates every app-like target and its exact provisioning-profile
mapping before creating an isolated temporary signing keychain. It never
overwrites the input or an existing output and never installs profiles into
the user's Xcode profile directories.

Use --format to select JSON, table, or Markdown output. The input and output
paths are separate because --output names the new IPA artifact.

Cross-team entitlement claim rebasing is disabled by default. Use
--rebase-team-claims only when the allowlisted claims should be transformed;
all transformed values remain subject to replacement-profile authorization.
Changing a KVS claim selects a different namespace and can make existing KVS
data inaccessible.

Example:
  asc signing resign --ipa ./App.ipa --output ./artifacts/App-resigned.ipa --identity ./signing/distribution.p12 --identity-password-file ./secrets/p12-password --profiles-manifest ./signing/profiles.json --format json`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := ctx.Err(); err != nil {
				return err
			}
			if runtime.GOOS != "darwin" {
				return shared.UsageError("signing resign is supported only on macOS")
			}
			required := []struct {
				name  string
				value string
			}{
				{name: "--ipa", value: *ipaPath},
				{name: "--output", value: *outputPath},
				{name: "--identity", value: *identityPath},
				{name: "--profiles-manifest", value: *profilesManifestPath},
			}
			for _, item := range required {
				if strings.TrimSpace(item.value) == "" {
					return shared.UsageError(item.name + " is required")
				}
			}
			if _, err := shared.ValidateOutputFormat(*format.Output, *format.Pretty); err != nil {
				return shared.UsageError(err.Error())
			}
			result, err := executeSigningResignFn(ctx, signingResignOptions{
				IPAPath:              *ipaPath,
				OutputPath:           *outputPath,
				IdentityPath:         *identityPath,
				IdentityPasswordPath: signingResignPathOrEmpty(*identityPasswordPath),
				ProfilesManifestPath: *profilesManifestPath,
				RebaseTeamClaims:     *rebaseTeamClaims,
			})
			if err != nil {
				if isSigningResignUsageError(err) {
					return shared.UsageError(err.Error())
				}
				// The implementation normally classifies path-bearing failures
				// before returning. Keep the public command boundary defensive for
				// injected implementations and future stages: detailed causes stay
				// available to package callers through Unwrap, while the CLI never
				// prints temporary paths, keychain names, or tool diagnostics.
				err = wrapSigningResignOperationalError(
					signingResignStagePreparation,
					signingResignCodeFilesystem,
					err,
				)
				return fmt.Errorf("signing resign: %w", err)
			}
			if err := printSigningResignResultFn(result, *format.Output, *format.Pretty); err != nil {
				if result.Output.Path == "" {
					return err
				}
				publicationErr := errors.Join(
					ErrSigningResignPublicationAmbiguous,
					fmt.Errorf("render signing resign receipt: %w", err),
				)
				return fmt.Errorf("signing resign: %w", wrapSigningResignOperationalError(
					signingResignStageArtifact,
					signingResignCodeArtifactPublish,
					publicationErr,
				))
			}
			return nil
		},
	}
}

type signingResignUsageFailure struct{ err error }

func (failure signingResignUsageFailure) Error() string { return failure.err.Error() }

func (failure signingResignUsageFailure) Unwrap() error { return failure.err }

func signingResignUsage(err error) error {
	if err == nil {
		return nil
	}
	return signingResignUsageFailure{err: err}
}

func isSigningResignUsageError(err error) bool {
	var failure signingResignUsageFailure
	return errors.As(err, &failure)
}

func signingResignPathOrEmpty(value string) string {
	if strings.TrimSpace(value) == "" {
		return ""
	}
	return value
}

// Keep implementation-local aliases while exposing the public receipt from
// internal/asc, where the shared output registry can render its exact type.
type (
	signingResignResult                   = asc.SigningResignResult
	signingResignInputResult              = asc.SigningResignInputResult
	signingResignArtifactResult           = asc.SigningResignArtifactResult
	signingResignIdentityResult           = asc.SigningResignIdentityResult
	signingResignTargetResult             = asc.SigningResignTargetResult
	signingResignVerification             = asc.SigningResignVerification
	signingResignOutputEntitlementRewrite = asc.SigningResignEntitlementRewrite
)

func printSigningResignResult(result signingResignResult, format string, pretty bool) error {
	return shared.PrintOutput(&result, format, pretty)
}

var (
	executeSigningResignFn     = executeSigningResignImplementation
	printSigningResignResultFn = printSigningResignResult
)
