package storekit

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"strings"

	"github.com/99designs/keyring"
	"github.com/peterbourgon/ff/v3/ffcli"

	authsvc "github.com/rudrankriyam/App-Store-Connect-CLI/internal/auth"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/config"
	storekitapi "github.com/rudrankriyam/App-Store-Connect-CLI/internal/storekit"
)

func authCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("storekit auth", flag.ExitOnError)
	return &ffcli.Command{
		Name:       "auth",
		ShortUsage: "asc storekit auth <subcommand> [flags]",
		ShortHelp:  "Manage In-App Purchase API credentials for StoreKit.",
		LongHelp: `Manage StoreKit In-App Purchase API credentials.

These credentials are independent of asc auth and App Store Connect API keys.

Examples:
  asc storekit auth login --name Production --key-id KEY_ID --issuer-id ISSUER_ID --private-key ./SubscriptionKey.p8 --bundle-id com.example.app
  asc storekit auth status
  asc storekit auth doctor --environment sandbox --network`,
		FlagSet: fs,
		Subcommands: []*ffcli.Command{
			authLoginCommand(d),
			authStatusCommand(d),
			authSwitchCommand(d),
			authDoctorCommand(d),
			authLogoutCommand(d),
		},
		UsageFunc: d.DefaultUsageFunc,
		Exec:      func(ctx context.Context, args []string) error { return flag.ErrHelp },
	}
}

func authLoginCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("storekit auth login", flag.ExitOnError)
	name := fs.String("name", "", "Friendly name for this StoreKit key")
	keyID := fs.String("key-id", "", "In-App Purchase API key ID")
	issuerID := fs.String("issuer-id", "", "In-App Purchase API issuer ID")
	privateKey := fs.String("private-key", "", "Path to the In-App Purchase EC private key")
	bundleID := fs.String("bundle-id", "", "App bundle identifier")
	environment := fs.String("environment", "", "Network validation environment: production or sandbox")
	bypassKeychain := fs.Bool("bypass-keychain", false, "Store the key path in config.json instead of keychain")
	local := fs.Bool("local", false, "When bypassing keychain, write to ./.asc/config.json")
	network := fs.Bool("network", false, "Validate credentials with the Retention Messaging API")
	skipValidation := fs.Bool("skip-validation", false, "Skip private key and network validation")
	return &ffcli.Command{
		Name:       "login",
		ShortUsage: "asc storekit auth login [flags]",
		ShortHelp:  "Register an In-App Purchase API key.",
		LongHelp: `Register an In-App Purchase API key for StoreKit server APIs.

Examples:
  asc storekit auth login --name Production --key-id KEY_ID --issuer-id ISSUER_ID --private-key ./SubscriptionKey.p8 --bundle-id com.example.app
  asc storekit auth login --network --environment sandbox --name Sandbox --key-id KEY_ID --issuer-id ISSUER_ID --private-key ./SubscriptionKey.p8 --bundle-id com.example.app`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			cleanName := strings.TrimSpace(*name)
			cleanKeyID := strings.TrimSpace(*keyID)
			cleanIssuerID := strings.TrimSpace(*issuerID)
			cleanPrivateKey := strings.TrimSpace(*privateKey)
			cleanBundleID := strings.TrimSpace(*bundleID)
			required := []struct{ value, name string }{
				{cleanName, "--name"},
				{cleanKeyID, "--key-id"},
				{cleanIssuerID, "--issuer-id"},
				{cleanPrivateKey, "--private-key"},
				{cleanBundleID, "--bundle-id"},
			}
			for _, item := range required {
				if strings.TrimSpace(item.value) == "" {
					return d.UsageError(item.name + " is required")
				}
			}
			if *skipValidation && *network {
				return d.UsageError("--skip-validation and --network are mutually exclusive")
			}
			if flagWasSet(fs, "environment") && !*network {
				return d.UsageError("--environment requires --network")
			}
			if *local && !*bypassKeychain && !storekitapi.ShouldBypassKeychain() {
				return d.UsageError("--local requires --bypass-keychain or ASC_STOREKIT_BYPASS_KEYCHAIN")
			}
			if !*skipValidation {
				if err := authsvc.ValidateKeyFile(cleanPrivateKey); err != nil {
					return d.InvalidInputErrorf("storekit auth login: invalid private key: %v", err)
				}
			}
			credentials := storekitapi.Credentials{
				KeyID: cleanKeyID, IssuerID: cleanIssuerID, PrivateKeyPath: cleanPrivateKey, BundleID: cleanBundleID,
			}
			if !*skipValidation {
				localClient, err := storekitapi.NewClient(credentials, storekitapi.Sandbox, storekitapi.WithTransport(d.HTTPTransport()))
				if err != nil {
					return fmt.Errorf("storekit auth login: %w", err)
				}
				if err := localClient.Validate(); err != nil {
					return fmt.Errorf("storekit auth login: invalid StoreKit signing key: %w", err)
				}
			}
			if *network {
				target, err := resolveEnvironment(environment)
				if err != nil {
					return d.UsageError(err.Error())
				}
				client, err := storekitapi.NewClient(credentials, target, storekitapi.WithTransport(d.HTTPTransport()))
				if err != nil {
					return fmt.Errorf("storekit auth login: %w", err)
				}
				requestCtx, cancel := d.Timeout(ctx)
				defer cancel()
				if _, err := client.ListMessages(requestCtx); err != nil {
					return fmt.Errorf("storekit auth login: network validation failed: %w", err)
				}
			}
			if *bypassKeychain || storekitapi.ShouldBypassKeychain() {
				if *local {
					path, err := config.LocalPath()
					if err != nil {
						return fmt.Errorf("storekit auth login: %w", err)
					}
					if err := storekitapi.StoreCredentialsConfigAt(cleanName, credentials, path); err != nil {
						return fmt.Errorf("storekit auth login: store credentials: %w", err)
					}
				} else if err := storekitapi.StoreCredentialsConfig(cleanName, credentials); err != nil {
					return fmt.Errorf("storekit auth login: store credentials: %w", err)
				}
			} else if err := storekitapi.StoreCredentials(cleanName, credentials); err != nil {
				return fmt.Errorf("storekit auth login: store credentials: %w", err)
			}
			fmt.Fprintf(d.IO.Out, "Successfully registered StoreKit API key %q for %s\n", cleanName, cleanBundleID)
			return nil
		},
	}
}

type authStatusOutput struct {
	ActiveSource string          `json:"active_source,omitempty"`
	ActiveError  string          `json:"active_error,omitempty"`
	Environment  string          `json:"environment,omitempty"`
	Credentials  []authStatusRow `json:"credentials"`
}

type authStatusRow struct {
	Name      string `json:"name"`
	KeyID     string `json:"key_id"`
	IssuerID  string `json:"issuer_id"`
	BundleID  string `json:"bundle_id"`
	IsDefault bool   `json:"default"`
	Source    string `json:"source"`
}

func authStatusCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("storekit auth status", flag.ExitOnError)
	output := d.BindOutputFlags(fs)
	validate := fs.Bool("validate", false, "Validate every stored credential via network")
	environment := fs.String("environment", "", "Validation environment: production or sandbox")
	return &ffcli.Command{
		Name:       "status",
		ShortUsage: "asc storekit auth status [flags]",
		ShortHelp:  "Show StoreKit authentication status.",
		LongHelp: `Show StoreKit authentication status.

Examples:
  asc storekit auth status
  asc storekit auth status --validate --environment sandbox --output json`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if flagWasSet(fs, "environment") && !*validate {
				return d.UsageError("--environment requires --validate")
			}
			credentials, err := storekitapi.ListCredentials()
			if err != nil && !errors.Is(err, config.ErrNotFound) {
				return fmt.Errorf("storekit auth status: %w", err)
			}
			activeCredentials, activeSource, activeErr := resolveCredentialsWithSource(commonFlags{})
			if activeErr == nil && activeSource == "ASC_STOREKIT_* environment credentials" {
				credentials = append(credentials, storekitapi.StoredCredential{
					Credentials: activeCredentials,
					Name:        "environment",
					IsDefault:   true,
					Source:      activeSource,
				})
			}
			rows := make([]authStatusRow, 0, len(credentials))
			for _, credential := range credentials {
				rows = append(rows, authStatusRow{
					Name: credential.Name, KeyID: credential.KeyID, IssuerID: credential.IssuerID,
					BundleID: credential.BundleID, IsDefault: credential.IsDefault, Source: credential.Source,
				})
			}
			result := authStatusOutput{Credentials: rows, ActiveSource: activeSource}
			if activeErr != nil {
				result.ActiveError = activeErr.Error()
				fmt.Fprintf(d.IO.ErrOut, "Warning: active StoreKit authentication could not be resolved: %v\n", activeErr)
			}
			if *validate {
				if len(credentials) == 0 {
					return fmt.Errorf("storekit auth status: no credentials available to validate: %w", activeErr)
				}
				target, err := resolveEnvironment(environment)
				if err != nil {
					return d.UsageError(err.Error())
				}
				result.Environment = string(target)
				failures := 0
				for _, credential := range credentials {
					client, clientErr := storekitapi.NewClient(credential.Credentials, target, storekitapi.WithTransport(d.HTTPTransport()))
					if clientErr == nil {
						requestCtx, cancel := d.Timeout(ctx)
						_, clientErr = client.ListMessages(requestCtx)
						cancel()
					}
					if clientErr != nil {
						failures++
						fmt.Fprintf(d.IO.ErrOut, "StoreKit profile %q validation failed: %v\n", credential.Name, clientErr)
					}
				}
				if failures > 0 {
					return shared.NewReportedError(fmt.Errorf("storekit auth status: validation failed for %d credential(s)", failures))
				}
			}
			tableRows := make([][]string, 0, len(rows))
			for _, credential := range rows {
				tableRows = append(tableRows, []string{credential.Name, credential.KeyID, credential.IssuerID, credential.BundleID, boolString(credential.IsDefault), credential.Source})
			}
			return printOutput(d, result, *output.Output, *output.Pretty,
				[]string{"Name", "Key ID", "Issuer ID", "Bundle ID", "Default", "Source"}, tableRows)
		},
	}
}

func authSwitchCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("storekit auth switch", flag.ExitOnError)
	name := fs.String("name", "", "StoreKit credential name")
	return &ffcli.Command{
		Name:       "switch",
		ShortUsage: "asc storekit auth switch --name NAME",
		ShortHelp:  "Set the default StoreKit credential.",
		FlagSet:    fs,
		UsageFunc:  d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			cleanName := strings.TrimSpace(*name)
			if cleanName == "" {
				return d.UsageError("--name is required")
			}
			if err := storekitapi.SetDefaultCredentials(cleanName); err != nil {
				return fmt.Errorf("storekit auth switch: %w", err)
			}
			fmt.Fprintf(d.IO.Out, "Default StoreKit profile set to %q\n", cleanName)
			return nil
		},
	}
}

func authDoctorCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("storekit auth doctor", flag.ExitOnError)
	common := bindCommonFlags(fs)
	network := fs.Bool("network", false, "Validate credentials with the Retention Messaging API")
	return &ffcli.Command{
		Name:       "doctor",
		ShortUsage: "asc storekit auth doctor --environment ENV [flags]",
		ShortHelp:  "Diagnose StoreKit credentials and JWT signing.",
		LongHelp: `Diagnose StoreKit credential resolution and JWT signing.

Examples:
  asc storekit auth doctor --environment sandbox
  asc storekit auth doctor --environment sandbox --network`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			client, environment, err := resolveClient(ctx, d, common)
			if err != nil {
				return usageOrWrap(d, "storekit auth doctor", err)
			}
			if err := client.Validate(); err != nil {
				return fmt.Errorf("storekit auth doctor: JWT signing failed: %w", err)
			}
			if *network {
				requestCtx, cancel := d.Timeout(ctx)
				defer cancel()
				if _, err := client.ListMessages(requestCtx); err != nil {
					return fmt.Errorf("storekit auth doctor: network validation failed: %w", err)
				}
			}
			fmt.Fprintf(d.IO.Out, "StoreKit credentials are valid for %s%s\n", environment, map[bool]string{true: " (network verified)", false: ""}[*network])
			return nil
		},
	}
}

func authLogoutCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("storekit auth logout", flag.ExitOnError)
	name := fs.String("name", "", "StoreKit credential name")
	all := fs.Bool("all", false, "Remove all StoreKit credentials")
	confirm := fs.Bool("confirm", false, "Confirm credential removal")
	return &ffcli.Command{
		Name:       "logout",
		ShortUsage: "asc storekit auth logout (--name NAME | --all) --confirm",
		ShortHelp:  "Remove StoreKit credentials.",
		FlagSet:    fs,
		UsageFunc:  d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			cleanName := strings.TrimSpace(*name)
			if *all == (cleanName != "") {
				return d.UsageError("exactly one of --name or --all is required")
			}
			if err := d.RequireConfirm(*confirm); err != nil {
				return err
			}
			if *all {
				if err := storekitapi.RemoveAllCredentials(); err != nil {
					return fmt.Errorf("storekit auth logout: %w", err)
				}
				fmt.Fprintln(d.IO.Out, "Removed all StoreKit credentials")
				return nil
			}
			if err := storekitapi.RemoveCredentials(cleanName); err != nil {
				if errors.Is(err, keyring.ErrKeyNotFound) {
					return fmt.Errorf("storekit auth logout: credentials not found for profile %q", cleanName)
				}
				return fmt.Errorf("storekit auth logout: %w", err)
			}
			fmt.Fprintf(d.IO.Out, "Removed StoreKit profile %q\n", cleanName)
			return nil
		},
	}
}
