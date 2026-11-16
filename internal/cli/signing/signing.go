package signing

import (
	"context"
	"flag"

	"github.com/peterbourgon/ff/v3/ffcli"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// NewSigningCommand returns the signing command with subcommands.
func NewSigningCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("signing", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "signing",
		ShortUsage: "asc signing <subcommand> [flags]",
		ShortHelp:  "Manage signing certificates and profiles.",
		LongHelp: `Manage signing assets in App Store Connect.

Examples:
  asc signing fetch --bundle-id com.example.app --profile-type IOS_APP_STORE --output ./signing
  asc signing keychain install --identity ./signing/App.p12 --identity-password-file ./secrets/p12-password --keychain ./keychains/release.keychain-db --keychain-password-file ./secrets/keychain-password --confirm
  asc signing reconcile plan --archive-path .asc/artifacts/App.xcarchive --devices-file .asc/distribution/devices.json
  asc signing resign --ipa ./App.ipa --output ./artifacts/App-resigned.ipa --identity ./signing/App.p12 --profiles-manifest ./signing/profiles.json --format json
  asc signing run --identity ./signing/App.p12 --profile ./signing/App.mobileprovision -- xcodebuild -exportArchive
  asc signing sync push --bundle-id com.example.app --profile-type IOS_APP_STORE --repo git@github.com:team/certs.git
  asc signing sync pull --repo git@github.com:team/certs.git --output-dir ./signing`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newSigningFetchCommand(d),
			newSigningKeychainCommand(d),
			SigningReconcileCommand(),
			SigningResignCommand(),
			newSigningRunCommand(d),
			SigningSyncCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}
