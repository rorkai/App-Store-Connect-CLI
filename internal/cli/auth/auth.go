package auth

import (
	"context"
	"crypto/ecdsa"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	authsvc "github.com/rudrankriyam/App-Store-Connect-CLI/internal/auth"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/config"
)

var (
	loginJWTGenerator        = asc.GenerateJWT
	loginNetworkValidate     = validateLoginNetwork
	statusValidateCredential = validateStoredCredential
	listStoredCredentials    = authsvc.ListCredentials
	listCredentialSummaries  = authsvc.ListCredentialSummaries
	keychainAvailable        = authsvc.KeychainAvailable
	migrateKeychainToConfig  = authsvc.MigrateKeychainToConfig
	removeStoredCredential   = authsvc.RemoveCredentialsWithOptions
	removeStoredCredentials  = authsvc.RemoveAllCredentialsWithOptions
)

// Auth command factory
func AuthCommand() *ffcli.Command {
	fs := flag.NewFlagSet("auth", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "auth",
		ShortUsage: "asc auth <subcommand> [flags]",
		ShortHelp:  "Manage authentication for the App Store Connect API.",
		LongHelp: `Manage authentication for the App Store Connect API.

Authentication is handled via App Store Connect API keys. Generate keys at:
https://appstoreconnect.apple.com/access/integrations/api

Credentials can come from the system keychain, the active config file, or environment variables.

Credential resolution:
  - --profile or ASC_PROFILE selects a stored profile and disables the env-only fast path.
  - With no profile and keychain bypass disabled, a complete environment set skips stored lookup.
  - After stored selection succeeds, environment variables can fill eligible missing fields.
  - ASC_BYPASS_KEYCHAIN skips keychain; env fallback follows only missing/default-selection config errors.

Config selection: ASC_CONFIG_PATH; otherwise nearest ancestor .asc/config.json; otherwise ~/.asc/config.json.

Use --strict-auth or ASC_STRICT_AUTH=true (also: 1, yes, y, on) to reject split-source fields.
Set ASC_BYPASS_KEYCHAIN to 1/true/yes/on to bypass keychain.

Use "asc auth status" to see which credentials/profile are currently active.

Examples:
  asc auth status
  asc auth status --verbose
  asc --profile work apps list
  asc auth switch --name work
  asc auth export-to-config --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			AuthInitCommand(),
			AuthLoginCommand(),
			AuthExportToConfigCommand(),
			AuthSwitchCommand(),
			AuthLogoutCommand(),
			AuthDoctorCommand(),
			AuthStatusCommand(),
			AuthIssuerIDCommand(),
			AuthTokenCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			if len(args) == 0 {
				return flag.ErrHelp
			}
			fmt.Fprintf(os.Stderr, "Unknown subcommand: %s\n\n", args[0])
			return flag.ErrHelp
		},
	}
}

// AuthInit command factory
func AuthInitCommand() *ffcli.Command {
	fs := flag.NewFlagSet("auth init", flag.ExitOnError)

	force := fs.Bool("force", false, "Overwrite existing config.json")
	local := fs.Bool("local", false, "Write config.json to ./.asc in the current repo")
	open := fs.Bool("open", false, "Open the App Store Connect API keys page in your browser")

	return &ffcli.Command{
		Name:       "init",
		ShortUsage: "asc auth init [flags]",
		ShortHelp:  "Create a template config.json for authentication.",
		LongHelp: `Create a template config.json for authentication.

This writes the ASC_CONFIG_PATH file when that variable is set, otherwise
~/.asc/config.json, with empty fields and secure permissions.
Use --local to write ./.asc/config.json in the current repo instead.

Examples:
  asc auth init
  asc auth init --local
  asc auth init --force
  asc auth init --open`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			var path string
			var err error
			if *local {
				path, err = config.LocalPath()
			} else {
				path, err = config.DefaultWritePath()
			}
			if err != nil {
				return fmt.Errorf("auth init: %w", err)
			}

			if !*force {
				if _, err := os.Stat(path); err == nil {
					return fmt.Errorf("auth init: config already exists at %s (use --force to overwrite)", path)
				} else if !os.IsNotExist(err) {
					return fmt.Errorf("auth init: %w", err)
				}
			}

			template := &config.Config{}
			if err := config.SaveAt(path, template); err != nil {
				return fmt.Errorf("auth init: %w", err)
			}
			if *local {
				warnLocalConfigShadowed(path)
			}

			if *open {
				if err := openURL(authsvc.APIKeysURL); err != nil {
					return fmt.Errorf("auth init: %w", err)
				}
			}

			result := struct {
				ConfigPath string         `json:"config_path"`
				Created    bool           `json:"created"`
				Config     *config.Config `json:"config"`
			}{
				ConfigPath: path,
				Created:    true,
				Config:     template,
			}
			return shared.PrintOutput(result, "json", false)
		},
	}
}

// AuthDoctor command factory
func AuthDoctorCommand() *ffcli.Command {
	fs := flag.NewFlagSet("auth doctor", flag.ExitOnError)

	output := shared.BindOutputFlagsWithAllowed(fs, "output", "text", "Output format: text (default), json", "text", "json")
	fix := fs.Bool("fix", false, "Attempt to fix issues where possible")
	confirm := fs.Bool("confirm", false, "Confirm applying fixes")

	return &ffcli.Command{
		Name:       "doctor",
		ShortUsage: "asc auth doctor [flags]",
		ShortHelp:  "Diagnose authentication configuration issues.",
		LongHelp: `Diagnose authentication configuration issues.

Runs a comprehensive health check across keychain availability, config files,
stored profiles, private key files, and environment variables.

Examples:
  asc auth doctor
  asc auth doctor --output json
  asc auth doctor --fix --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			normalizedOutput, err := shared.ValidateOutputFormatAllowed(*output.Output, *output.Pretty, "text", "json")
			if err != nil {
				return shared.UsageError(err.Error())
			}
			if *fix && !*confirm {
				return shared.UsageError("--fix requires --confirm")
			}

			report := authsvc.DoctorWithMigrationResolver(
				authsvc.DoctorOptions{
					Fix:        *fix && *confirm,
					Profile:    shared.ResolveProfileName(),
					StrictAuth: shared.StrictAuthEnabled(),
				},
				doctorMigrationSuggestionResolver(),
			)
			if normalizedOutput == "json" {
				if err := shared.PrintOutput(report, "json", *output.Pretty); err != nil {
					return err
				}
			} else {
				printDoctorReport(report)
			}

			if report.Summary.Errors > 0 {
				return shared.NewReportedError(fmt.Errorf("auth doctor: found %d error(s)", report.Summary.Errors))
			}
			return nil
		},
	}
}

func printDoctorReport(report authsvc.DoctorReport) {
	fmt.Println("Auth Doctor")
	for _, section := range report.Sections {
		if len(section.Checks) == 0 {
			continue
		}
		fmt.Printf("\n%s:\n", shared.SanitizeTerminal(section.Title))
		for _, check := range section.Checks {
			fmt.Printf("  [%s] %s\n", doctorStatusLabel(check.Status), shared.SanitizeTerminal(check.Message))
		}
	}
	if len(report.Recommendations) > 0 {
		fmt.Println("\nRecommendations:")
		for i, rec := range report.Recommendations {
			fmt.Printf("  %d. %s\n", i+1, shared.SanitizeTerminal(rec))
		}
	}

	if report.Summary.Errors == 0 && report.Summary.Warnings == 0 {
		fmt.Println("\nNo issues found.")
	} else {
		fmt.Printf("\nFound %d warning(s) and %d error(s).\n", report.Summary.Warnings, report.Summary.Errors)
	}
}

func doctorStatusLabel(status authsvc.DoctorStatus) string {
	switch status {
	case authsvc.DoctorOK:
		return "OK"
	case authsvc.DoctorWarn:
		return "WARN"
	case authsvc.DoctorFail:
		return "FAIL"
	case authsvc.DoctorInfo:
		return "INFO"
	default:
		return strings.ToUpper(string(status))
	}
}

func doctorMigrationSuggestionResolver() authsvc.MigrationSuggestionResolver {
	return func(input authsvc.MigrationSuggestionResolverInput) authsvc.MigrationSuggestionResolverOutput {
		result := authsvc.MigrationSuggestionResolverOutput{}
		appID := strings.TrimSpace(input.AppID)
		if appID != "" {
			result.AppID = appID
		}

		needsAppLookup := input.NeedAppID && appID == "" && strings.TrimSpace(input.AppIdentifier) != ""
		needsVersionLookup := input.NeedVersionID
		needsBuildLookup := input.NeedBuildID
		if !needsAppLookup && !needsVersionLookup && !needsBuildLookup {
			return result
		}
		if appID == "" && !needsAppLookup {
			return result
		}

		client, err := shared.GetASCClient()
		if err != nil {
			return result
		}

		requestCtx, cancel := shared.ContextWithTimeout(context.Background())
		defer cancel()

		if needsAppLookup {
			appIdentifier := strings.TrimSpace(input.AppIdentifier)
			appsResp, err := client.GetApps(
				requestCtx,
				asc.WithAppsBundleIDs([]string{appIdentifier}),
				asc.WithAppsLimit(1),
			)
			if err == nil && len(appsResp.Data) > 0 {
				appID = strings.TrimSpace(appsResp.Data[0].ID)
			}
		}
		if appID == "" {
			return result
		}
		result.AppID = appID

		versionString := strings.TrimSpace(input.MarketingVersion)
		if needsVersionLookup && versionString != "" {
			versionsResp, err := client.GetAppStoreVersions(
				requestCtx, appID,
				asc.WithAppStoreVersionsVersionStrings([]string{versionString}),
				asc.Limit[asc.AppStoreVersionsOption](1),
			)
			if err == nil && len(versionsResp.Data) > 0 {
				result.VersionID = strings.TrimSpace(versionsResp.Data[0].ID)
			}
		}

		if needsBuildLookup {
			buildOpts := []asc.BuildsOption{
				asc.WithBuildsSort("-uploadedDate"),
				asc.WithBuildsLimit(1),
			}
			if versionString != "" {
				preReleaseResp, err := client.GetPreReleaseVersions(
					requestCtx, appID,
					asc.WithPreReleaseVersionsVersion(versionString),
					asc.WithPreReleaseVersionsLimit(1),
				)
				if err == nil && len(preReleaseResp.Data) > 0 {
					buildOpts = append(buildOpts, asc.WithBuildsPreReleaseVersion(preReleaseResp.Data[0].ID))
				}
			}
			buildsResp, err := client.GetBuilds(requestCtx, appID, buildOpts...)
			if err == nil && len(buildsResp.Data) > 0 {
				result.BuildID = strings.TrimSpace(buildsResp.Data[0].ID)
			}
		}

		return result
	}
}

type permissionWarning struct {
	err error
}

func (p *permissionWarning) Error() string {
	return p.err.Error()
}

func (p *permissionWarning) Unwrap() error {
	return p.err
}

func validateStoredCredential(ctx context.Context, cred authsvc.Credential) error {
	var (
		privateKey *ecdsa.PrivateKey
		client     *asc.Client
		err        error
	)
	signingIssuerID := credentialSigningIssuerID(cred)
	if pemValue := strings.TrimSpace(cred.PrivateKeyPEM); pemValue != "" {
		privateKey, err = authsvc.LoadPrivateKeyFromPEM([]byte(pemValue))
		if err != nil {
			return shared.WithPrivateKeyDiagnostic(fmt.Errorf("invalid private key: %w", err), err)
		}
		client, err = asc.NewClientFromPEM(cred.KeyID, signingIssuerID, pemValue)
		if err != nil {
			return shared.WithPrivateKeyDiagnostic(err, err)
		}
	} else {
		if err := authsvc.ValidateKeyFile(cred.PrivateKeyPath); err != nil {
			return shared.WithPrivateKeyDiagnostic(fmt.Errorf("invalid private key: %w", err), err)
		}
		privateKey, err = authsvc.LoadPrivateKey(cred.PrivateKeyPath)
		if err != nil {
			return shared.WithPrivateKeyDiagnostic(fmt.Errorf("failed to load private key: %w", err), err)
		}
		client, err = asc.NewClient(cred.KeyID, signingIssuerID, cred.PrivateKeyPath)
		if err != nil {
			return shared.WithPrivateKeyDiagnostic(err, err)
		}
	}
	if _, err := asc.GenerateJWT(cred.KeyID, signingIssuerID, privateKey); err != nil {
		return fmt.Errorf("failed to generate JWT: %w", err)
	}
	if err := probeCredential(ctx, client, cred.KeyType); err != nil {
		if errors.Is(err, asc.ErrForbidden) {
			return &permissionWarning{err: err}
		}
		return withNetworkDiagnostic(err, err)
	}
	return nil
}

func credentialSigningIssuerID(cred authsvc.Credential) string {
	if config.IsIndividualCredentialKeyType(cred.KeyType) {
		return ""
	}
	return cred.IssuerID
}

func credentialProbeName(keyType string) string {
	if config.IsEnterpriseCredentialKeyType(keyType) {
		return "bundle IDs list"
	}
	return "apps list"
}

// probeCredential makes one cheap authenticated read on the key's API.
func probeCredential(ctx context.Context, client *asc.Client, keyType string) error {
	if config.IsEnterpriseCredentialKeyType(keyType) {
		client.UseEnterpriseProgramAPI()
		_, err := client.GetBundleIDs(ctx, asc.Limit[asc.BundleIDsOption](1))
		return err
	}
	_, err := client.GetApps(ctx, asc.WithAppsLimit(1))
	return err
}

func validateLoginCredentials(ctx context.Context, keyID, issuerID, keyPath, keyType string, network bool) error {
	privateKey, err := authsvc.LoadPrivateKey(keyPath)
	if err != nil {
		return shared.WithPrivateKeyDiagnostic(fmt.Errorf("failed to load private key: %w", err), err)
	}
	if _, err := loginJWTGenerator(keyID, issuerID, privateKey); err != nil {
		return shared.WithDiagnostic(fmt.Errorf("failed to generate JWT: %w", err), shared.DiagnosticInternalError, "--private-key")
	}
	if network {
		if err := loginNetworkValidate(ctx, keyID, issuerID, keyPath, keyType); err != nil {
			return withNetworkDiagnostic(fmt.Errorf("network validation failed: %w", err), err)
		}
	}
	return nil
}

func withNetworkDiagnostic(rendered, cause error) error {
	code := shared.DiagnosticRequestFailed
	if errors.Is(cause, asc.ErrUnauthorized) {
		code = shared.DiagnosticAuthenticationRejected
	}
	return shared.WithDiagnostic(rendered, code, "")
}

// printPrivateKeyPermissionRemediation follows an over-permissive key failure
// with the exact command that repairs the file and the flag that applies it, so
// a first login recovers without consulting auth doctor. Other private-key
// failures print nothing.
func printPrivateKeyPermissionRemediation(cause error, keyPath string) {
	if kind, ok := authsvc.PrivateKeyErrorKindOf(cause); !ok || kind != authsvc.PrivateKeyPermissionsInsecure {
		return
	}
	if command, safe := authsvc.FilePermissionRemediationCommand(keyPath); safe {
		fmt.Fprintf(
			os.Stderr,
			"To fix, run:\n  %s\nOr re-run with --fix-permissions to let asc change the file to 0600.\n",
			shared.SanitizeTerminal(command),
		)
		return
	}
	fmt.Fprintln(os.Stderr, "Re-run with --fix-permissions to let asc change the file to 0600.")
}

func validateLoginNetwork(ctx context.Context, keyID, issuerID, keyPath, keyType string) error {
	client, err := asc.NewClient(keyID, issuerID, keyPath)
	if err != nil {
		return err
	}
	return probeCredential(ctx, client, keyType)
}

// loginConfigPath returns the config file a keychain-bypassing login writes:
// ./.asc/config.json with --local, otherwise ASC_CONFIG_PATH when set (the only
// file reads consult then), otherwise the global config.
func loginConfigPath(local bool) (string, error) {
	if local {
		return config.LocalPath()
	}
	return config.DefaultWritePath()
}

// warnLocalConfigShadowed explains that an explicit --local write is not the
// file later commands read while ASC_CONFIG_PATH is set.
// warnRetainedGlobalCredentials tells the user when logout left credentials in
// ~/.asc/config.json because ASC_CONFIG_PATH scoped cleanup to another file.
// An empty name refers to all stored credentials.
func warnRetainedGlobalCredentials(name string) {
	globalPath, retained, err := authsvc.RetainedGlobalConfigCredentials(name)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not check ~/.asc/config.json for credentials that auth logout left in place: %s\n", shared.SanitizeTerminal(err.Error()))
		return
	}
	if !retained {
		return
	}
	held := "stored credentials"
	if name != "" {
		held = fmt.Sprintf("credentials named '%s'", shared.SanitizeTerminal(name))
	}
	fmt.Fprintf(
		os.Stderr,
		"Warning: ASC_CONFIG_PATH is set, so auth logout did not change %s, which still holds %s; to remove them, run auth logout with ASC_CONFIG_PATH unset or pass --include-global.\n",
		shared.SanitizeTerminal(globalPath),
		held,
	)
}

func warnLocalConfigShadowed(localPath string) {
	overridePath, ok, err := config.OverridePath()
	if !ok {
		return
	}
	if err != nil {
		fmt.Fprintf(
			os.Stderr,
			"Warning: other commands cannot read %s while ASC_CONFIG_PATH is invalid (%s); unset ASC_CONFIG_PATH or point it at %s to use this file.\n",
			shared.SanitizeTerminal(localPath),
			shared.SanitizeTerminal(err.Error()),
			shared.SanitizeTerminal(localPath),
		)
		return
	}
	if filepath.Clean(overridePath) == filepath.Clean(localPath) {
		return
	}
	fmt.Fprintf(
		os.Stderr,
		"Warning: ASC_CONFIG_PATH is set, so other commands read %s instead of %s; unset ASC_CONFIG_PATH or point it at %s to use this file.\n",
		shared.SanitizeTerminal(overridePath),
		shared.SanitizeTerminal(localPath),
		shared.SanitizeTerminal(localPath),
	)
}

func loginStorageMessage(bypassKeychain, local bool) (string, error) {
	if bypassKeychain {
		path, err := loginConfigPath(local)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("Storing credentials in config file at %s", path), nil
	}

	keychainAvailable, err := authsvc.KeychainAvailable()
	if err != nil {
		return "", err
	}
	if keychainAvailable {
		return "Storing credentials in system keychain", nil
	}
	path, err := config.Path()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("System keychain unavailable; storing credentials in config file at %s", path), nil
}

// reportLoginCredentialShapes fails only when the key ID is an issuer UUID and
// the issuer ID is not, which can only mean the two values were swapped.
// Everything less certain is written to stderr as a warning so valid but
// unusual credentials never block a login.
func reportLoginCredentialShapes(keyID, issuerID string, individualKey bool) error {
	if individualKey {
		return nil
	}
	findings := authsvc.InspectCredentialShapes(
		authsvc.CredentialShapeLabels{KeyID: "--key-id", IssuerID: "--issuer-id"},
		keyID,
		issuerID,
	)
	for _, finding := range findings {
		if !finding.DefiniteSwap {
			continue
		}
		return shared.WithDiagnostic(
			shared.UsageErrorf("auth login: %s. %s", finding.Message, finding.Recommendation),
			shared.DiagnosticInvalidInput,
			finding.Field,
		)
	}
	for _, finding := range findings {
		fmt.Fprintf(os.Stderr, "Warning: %s. %s\n", finding.Message, finding.Recommendation)
	}
	return nil
}

// AuthLogin command factory
func AuthLoginCommand() *ffcli.Command {
	fs := flag.NewFlagSet("auth login", flag.ExitOnError)

	name := fs.String("name", "", "Friendly name for this key (optional on first login: defaults to \"default\" when no profiles are stored)")
	keyID := fs.String("key-id", "", "App Store Connect API Key ID (read from an AuthKey_<KEY_ID>.p8 file name when omitted)")
	issuerID := fs.String("issuer-id", "", "App Store Connect Issuer ID")
	keyType := fs.String("key-type", config.CredentialKeyTypeTeam, "API key type: team, individual, or enterprise (Enterprise Program API)")
	keyPath := fs.String("private-key", "", "Path to private key (.p8) file")
	bypassKeychain := fs.Bool("bypass-keychain", false, "Store credentials in config.json instead of keychain")
	local := fs.Bool("local", false, "When bypassing keychain, write to ./.asc/config.json")
	network := fs.Bool("network", false, "Validate credentials with a lightweight API request")
	skipValidation := fs.Bool("skip-validation", false, "Skip JWT and network validation checks")
	fixPermissions := fs.Bool("fix-permissions", false, "Change an over-permissive private key file to 0600 before reading it")

	return &ffcli.Command{
		Name:       "login",
		ShortUsage: "asc auth login [flags]",
		ShortHelp:  "Register and store App Store Connect API key credentials.",
		LongHelp: `Register and store App Store Connect API key credentials.

This command stores your API credentials in the system keychain when available,
with a local config fallback (restricted permissions). Use --bypass-keychain to
explicitly bypass keychain and write credentials to a config file instead: the
ASC_CONFIG_PATH file when that variable is set, otherwise ~/.asc/config.json.
Add --local to write ./.asc/config.json for the current repo.

--name may be omitted on a first login: with no stored profiles the key is saved
as "default". Once profiles exist, --name is required and the error lists them.

The private key file must not be readable by other users. An over-permissive key
fails validation and, when its path is safe to render, prints the exact chmod
command that repairs it. Pass --fix-permissions to let asc change the file to
0600 first and report what it changed. If the operating system cannot securely
open the file for descriptor-bound repair, asc fails without changing it.

Examples:
  asc auth login --name "MyKey" --key-id "ABC123" --issuer-id "DEF456" --private-key /path/to/AuthKey.p8
  asc auth login --name "MyIndividualKey" --key-id "ABC123" --key-type individual --private-key /path/to/AuthKey.p8
  asc auth login --bypass-keychain --local --name "MyKey" --key-id "ABC123" --issuer-id "DEF456" --private-key /path/to/AuthKey.p8
  asc auth login --name "MyEnterpriseKey" --key-id "ABC123" --issuer-id "DEF456" --key-type enterprise --private-key /path/to/AuthKey.p8
  asc auth login --network --name "MyKey" --key-id "ABC123" --issuer-id "DEF456" --private-key /path/to/AuthKey.p8
  asc auth login --skip-validation --name "MyKey" --key-id "ABC123" --issuer-id "DEF456" --private-key /path/to/AuthKey.p8
  asc auth login --fix-permissions --name "MyKey" --key-id "ABC123" --issuer-id "DEF456" --private-key /path/to/AuthKey.p8

When using system keychain storage, the encrypted key material is stored in keychain
so commands continue to work even if the original .p8 file is removed.`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			bypassKeychainEnabled := *bypassKeychain || authsvc.ShouldBypassKeychain()
			if *local && !bypassKeychainEnabled {
				return shared.WithDiagnostic(shared.UsageError("--local requires --bypass-keychain or ASC_BYPASS_KEYCHAIN set to 1/true/yes/on"), shared.DiagnosticInvalidInput, "--local")
			}
			if !flagWasSet(fs, "name") {
				profileName, err := omittedLoginProfileName(bypassKeychainEnabled, *local)
				if err != nil {
					return err
				}
				*name = profileName
			}
			if strings.TrimSpace(*name) == "" {
				return shared.RequiredFlag("--name")
			}
			trimmedKeyID := strings.TrimSpace(*keyID)
			if !flagWasSet(fs, "key-id") {
				if inferred := keyIDHintFromKeyPath(*keyPath); inferred != "" {
					fmt.Fprintf(os.Stderr, "Using --key-id %s from the private key file name\n", inferred)
					trimmedKeyID = inferred
				}
			}
			if trimmedKeyID == "" {
				fmt.Fprintln(os.Stderr, "Error: --key-id is required")
				if hint := keyIDHintFromKeyPath(*keyPath); hint != "" {
					fmt.Fprintf(os.Stderr, "Hint: the key file name suggests --key-id %s\n", hint)
				} else {
					fmt.Fprintln(os.Stderr, "Hint: the key ID is listed next to the key on the API keys page and in the downloaded file name, AuthKey_<KEY_ID>.p8.")
				}
				fmt.Fprintf(os.Stderr, "Hint: create an API key at %s, then run: %s\n", authsvc.APIKeysURL, authsvc.LoginCommandExample)
				return shared.MissingRequiredUsageError("--key-id")
			}
			*keyID = trimmedKeyID
			normalizedKeyType := config.NormalizeCredentialKeyType(*keyType)
			if !config.IsValidCredentialKeyType(normalizedKeyType) {
				return shared.WithDiagnostic(shared.UsageError("--key-type must be one of: team, individual, enterprise"), shared.DiagnosticInvalidInput, "--key-type")
			}
			trimmedIssuerID := strings.TrimSpace(*issuerID)
			if normalizedKeyType != config.CredentialKeyTypeIndividual && trimmedIssuerID == "" {
				return shared.RequiredFlag("--issuer-id")
			}
			if normalizedKeyType == config.CredentialKeyTypeIndividual && trimmedIssuerID != "" {
				return shared.WithDiagnostic(shared.UsageError("--issuer-id must be omitted when --key-type individual"), shared.DiagnosticConflictingInput, "--issuer-id")
			}
			*issuerID = trimmedIssuerID
			if strings.TrimSpace(*keyPath) == "" {
				return shared.RequiredFlag("--private-key")
			}
			if *skipValidation && *network {
				return shared.WithDiagnostic(shared.UsageError("--skip-validation and --network are mutually exclusive"), shared.DiagnosticConflictingInput, "--skip-validation")
			}
			if err := reportLoginCredentialShapes(*keyID, *issuerID, normalizedKeyType == config.CredentialKeyTypeIndividual); err != nil {
				return err
			}

			if *fixPermissions {
				changed, err := authsvc.FixPrivateKeyFilePermissions(*keyPath)
				// A missing file is left to ValidateKeyFile below, which
				// reports it as a usage error naming the path.
				if kind, _ := authsvc.PrivateKeyErrorKindOf(err); err != nil && kind != authsvc.PrivateKeyNotFound {
					rendered := errors.New(shared.SanitizeTerminal(fmt.Sprintf("auth login: failed to fix private key permissions: %v", err)))
					return shared.WithPrivateKeyDiagnostic(shared.NewErrorWithCause(rendered, err), err)
				}
				if changed {
					fmt.Fprintf(os.Stderr, "Changed private key file permissions to 0600: %s\n", shared.SanitizeTerminal(*keyPath))
				}
			}

			if err := authsvc.ValidateKeyFile(*keyPath); err != nil {
				message := err.Error()
				if kind, _ := authsvc.PrivateKeyErrorKindOf(err); kind == authsvc.PrivateKeyPermissionsInsecure {
					message = "private key file is too permissive; other users can read this credential"
				}
				rendered := shared.WithPrivateKeyDiagnostic(shared.UsageErrorf("auth login: invalid private key: %s", message), err)
				printPrivateKeyPermissionRemediation(err, *keyPath)
				return rendered
			}

			if !*skipValidation {
				if err := validateLoginCredentials(ctx, *keyID, *issuerID, *keyPath, normalizedKeyType, *network); err != nil {
					return fmt.Errorf("auth login: %w", err)
				}
			}

			storageMessage, err := loginStorageMessage(bypassKeychainEnabled, *local)
			if err != nil {
				return fmt.Errorf("auth login: %w", err)
			}
			fmt.Println(storageMessage)

			// Store credentials securely
			if bypassKeychainEnabled {
				if *local {
					path, err := config.LocalPath()
					if err != nil {
						return fmt.Errorf("auth login: %w", err)
					}
					if err := authsvc.StoreCredentialsConfigAtWithKeyType(*name, *keyID, *issuerID, *keyPath, path, normalizedKeyType); err != nil {
						return fmt.Errorf("auth login: failed to store credentials: %w", err)
					}
					warnLocalConfigShadowed(path)
				} else {
					if err := authsvc.StoreCredentialsConfigWithKeyType(*name, *keyID, *issuerID, *keyPath, normalizedKeyType); err != nil {
						return fmt.Errorf("auth login: failed to store credentials: %w", err)
					}
				}
			} else {
				if err := authsvc.StoreCredentialsWithKeyType(*name, *keyID, *issuerID, *keyPath, normalizedKeyType); err != nil {
					return fmt.Errorf("auth login: failed to store credentials: %w", err)
				}
			}

			fmt.Printf("Successfully registered API key '%s'\n", strings.TrimSpace(*name))
			return nil
		},
	}
}

// defaultLoginProfileName is the name stored credentials resolve to when none
// was given, matching the name unnamed legacy credentials are listed under.
const defaultLoginProfileName = "default"

// omittedLoginProfileName chooses the profile name for `auth login` without
// --name. With nothing stored in the destination it uses the default name; once
// profiles exist it refuses to guess, so an implicit default can never overwrite
// one, and lists the names to pass explicitly.
func omittedLoginProfileName(bypassKeychain, local bool) (string, error) {
	names, err := existingLoginProfileNames(bypassKeychain, local)
	if err != nil {
		return "", fmt.Errorf("auth login: %w", err)
	}
	if len(names) == 0 {
		return defaultLoginProfileName, nil
	}
	return "", shared.WithDiagnostic(
		shared.UsageErrorf(
			"--name is required when profiles already exist (%s); re-run with --name %q to update that profile, or --name with a new name to add one",
			strings.Join(names, ", "), names[0],
		),
		shared.DiagnosticRequiredInputMissing,
		"--name",
	)
}

// existingLoginProfileNames lists the profile names in the store `auth login`
// would write to: the config file from loginConfigPath when bypassing the
// keychain, otherwise the merged keychain and config credentials.
func existingLoginProfileNames(bypassKeychain, local bool) ([]string, error) {
	if !bypassKeychain {
		credentials, err := listCredentialSummaries()
		if err != nil {
			if _, ok := errors.AsType[*authsvc.CredentialsWarning](err); !ok {
				return nil, fmt.Errorf("failed to list credentials: %w", err)
			}
		}
		names := make([]string, 0, len(credentials))
		for _, credential := range credentials {
			names = appendProfileName(names, credential.Name)
		}
		return names, nil
	}

	path, err := loginConfigPath(local)
	if err != nil {
		return nil, err
	}
	cfg, err := config.LoadAt(path)
	if err != nil {
		if errors.Is(err, config.ErrNotFound) {
			return nil, nil
		}
		return nil, err
	}
	names := make([]string, 0, len(cfg.Keys)+1)
	for _, credential := range cfg.Keys {
		names = appendProfileName(names, credential.Name)
	}
	if strings.TrimSpace(cfg.KeyID) != "" {
		legacyName := strings.TrimSpace(cfg.DefaultKeyName)
		if legacyName == "" {
			legacyName = defaultLoginProfileName
		}
		names = appendProfileName(names, legacyName)
	}
	return names, nil
}

// flagWasSet reports whether name was passed on the command line, so an
// explicitly blank value stays an error instead of reading as omitted.
func flagWasSet(fs *flag.FlagSet, name string) bool {
	set := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == name {
			set = true
		}
	})
	return set
}

func appendProfileName(names []string, name string) []string {
	name = strings.TrimSpace(name)
	if name == "" || slices.Contains(names, name) {
		return names
	}
	return append(names, name)
}

// AuthExportToConfigCommand copies keychain-backed credentials to config.json.
func AuthExportToConfigCommand() *ffcli.Command {
	fs := flag.NewFlagSet("auth export-to-config", flag.ExitOnError)

	confirm := fs.Bool("confirm", false, "Confirm writing keychain credentials to config.json")
	local := fs.Bool("local", false, "Write credentials to ./.asc/config.json in the current repo")
	configPath := fs.String("config", "", "Write credentials to this config.json path")
	privateKeyDir := fs.String("private-key-dir", "", "Directory for exported .p8 files when original key files are missing")
	removeKeychain := fs.Bool("remove-keychain", false, "Remove migrated credentials from keychain after config.json is written")
	output := shared.BindOutputFlagsWithAllowed(fs, "output", "table", "Output format: table, json", "table", "json")

	return &ffcli.Command{
		Name:       "export-to-config",
		ShortUsage: "asc auth export-to-config --confirm [flags]",
		ShortHelp:  "Copy keychain credentials into config.json.",
		LongHelp: `Copy keychain credentials into config.json.

This helps move from system keychain storage to JSON config storage. Credentials
are written to ~/.asc/config.json by default, or to ./.asc/config.json with
--local. If a keychain entry has embedded private key material and the original
.p8 file is missing, the key is exported as a secure .p8 file and config.json
references that exported file.

Use --remove-keychain to delete migrated keychain entries after config.json is
written successfully. Otherwise keychain entries are left in place.

Examples:
  asc auth export-to-config --confirm
  asc auth export-to-config --confirm --local
  asc auth export-to-config --confirm --private-key-dir ~/.asc/keys
  asc auth export-to-config --confirm --remove-keychain
  asc auth export-to-config --confirm --output json`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			normalizedOutput, err := shared.ValidateOutputFormatAllowed(*output.Output, *output.Pretty, "table", "json")
			if err != nil {
				return shared.UsageError(err.Error())
			}
			if !*confirm {
				return shared.UsageError("--confirm is required")
			}
			if *local && *configPath != "" {
				return shared.UsageError("--local and --config are mutually exclusive")
			}

			targetConfigPath := *configPath
			if targetConfigPath == "" {
				if *local {
					targetConfigPath, err = config.LocalPath()
				} else {
					targetConfigPath, err = config.Path()
				}
				if err != nil {
					return fmt.Errorf("auth export-to-config: %w", err)
				}
			} else {
				targetConfigPath, err = filepath.Abs(targetConfigPath)
				if err != nil {
					return fmt.Errorf("auth export-to-config: invalid --config: %w", err)
				}
			}

			result, err := migrateKeychainToConfig(authsvc.MigrateKeychainToConfigOptions{
				ConfigPath:     targetConfigPath,
				PrivateKeyDir:  *privateKeyDir,
				RemoveKeychain: *removeKeychain,
			})
			if err != nil {
				return fmt.Errorf("auth export-to-config: %w", err)
			}

			if normalizedOutput == "json" {
				return shared.PrintOutput(result, "json", *output.Pretty)
			}
			printMigrateToConfigResult(result)
			return nil
		},
	}
}

func printMigrateToConfigResult(result authsvc.MigrateKeychainToConfigResult) {
	fmt.Printf("Migrated %d credential(s) to %s\n", len(result.Migrated), result.ConfigPath)
	if result.PrivateKeyDir != "" {
		fmt.Printf("Private key directory: %s\n", result.PrivateKeyDir)
	}
	if len(result.Migrated) > 0 {
		fmt.Println()
		asc.RenderTable(
			[]string{"Name", "Key ID", "Private Key", "Exported"},
			buildMigrateToConfigRows(result.Migrated),
		)
	}
	if result.RemovedFromKeychain {
		fmt.Println("\nRemoved migrated credentials from keychain.")
	} else if len(result.Warnings) > 0 {
		fmt.Println("\nKeychain cleanup incomplete; inspect warnings below.")
	} else {
		fmt.Println("\nKeychain entries were left unchanged. Set ASC_BYPASS_KEYCHAIN=1 to prefer config.json.")
	}
	for _, warning := range result.Warnings {
		fmt.Printf("Warning: %s\n", warning)
	}
}

func buildMigrateToConfigRows(credentials []authsvc.MigratedCredential) [][]string {
	rows := make([][]string, 0, len(credentials))
	for _, cred := range credentials {
		exported := "no"
		if cred.ExportedPrivateKey {
			exported = "yes"
		}
		rows = append(rows, []string{
			cred.Name,
			cred.KeyID,
			cred.PrivateKeyPath,
			exported,
		})
	}
	return rows
}

// AuthSwitch command factory
func AuthSwitchCommand() *ffcli.Command {
	fs := flag.NewFlagSet("auth switch", flag.ExitOnError)

	name := fs.String("name", "", "Profile name to set as default")

	return &ffcli.Command{
		Name:       "switch",
		ShortUsage: "asc auth switch --name <profile>",
		ShortHelp:  "Switch the default authentication profile.",
		LongHelp: `Switch the default authentication profile.

This updates the default profile used for keychain or config credentials.

Examples:
  asc auth switch --name "Personal"
  asc auth switch --name "Client"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedName := strings.TrimSpace(*name)
			if trimmedName == "" {
				return shared.RequiredFlag("--name")
			}

			credentials, err := listCredentialSummaries()
			if err != nil {
				if warning, ok := errors.AsType[*authsvc.CredentialsWarning](err); ok {
					fmt.Fprintf(os.Stderr, "Warning: %s\n", warning)
				} else {
					return fmt.Errorf("auth switch: failed to list credentials: %w", err)
				}
			}
			if len(credentials) == 0 {
				return fmt.Errorf("auth switch: no credentials stored")
			}

			found := false
			for _, cred := range credentials {
				if strings.TrimSpace(cred.Name) == trimmedName {
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("auth switch: profile %q not found", trimmedName)
			}

			if err := authsvc.SetDefaultCredentials(trimmedName); err != nil {
				return fmt.Errorf("auth switch: %w", err)
			}

			fmt.Printf("Default profile set to '%s'\n", trimmedName)
			return nil
		},
	}
}

// AuthLogout command factory
func AuthLogoutCommand() *ffcli.Command {
	fs := flag.NewFlagSet("auth logout", flag.ExitOnError)
	all := fs.Bool("all", false, "Remove all stored credentials")
	name := fs.String("name", "", "Remove a named credential")
	confirm := fs.Bool("confirm", false, "Confirm credential removal (required)")
	includeGlobal := fs.Bool("include-global", false, "Also remove matching credentials from ~/.asc/config.json when ASC_CONFIG_PATH points to another file")

	return &ffcli.Command{
		Name:       "logout",
		ShortUsage: "asc auth logout [--name NAME | --all] --confirm",
		ShortHelp:  "Remove stored API credentials.",
		LongHelp: `Remove stored API credentials.

Omitting --name removes all stored credentials. --confirm is required before
any credential is removed.

Logout removes matching credentials from the keychain and the active config
file. When ASC_CONFIG_PATH is set, that file is the only config file changed;
~/.asc/config.json is left alone, and a warning names it if it still holds
matching credentials. Pass --include-global to remove them from
~/.asc/config.json as well. Without ASC_CONFIG_PATH, logout also cleans
~/.asc/config.json. When ASC_BYPASS_KEYCHAIN is set, logout changes config
files only and leaves keychain entries untouched.

Examples:
  asc auth logout --all --confirm
  asc auth logout --name "MyKey" --confirm
  asc auth logout --all --include-global --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			confirmProvided := false
			fs.Visit(func(f *flag.Flag) {
				if f.Name == "confirm" {
					confirmProvided = true
				}
			})
			if confirmProvided && !*confirm {
				return shared.UsageError("--confirm must be true when specified")
			}
			trimmedName := strings.TrimSpace(*name)
			if trimmedName == "" && *name != "" {
				return shared.UsageError("--name cannot be blank")
			}
			if trimmedName != "" && *all {
				return shared.UsageError("--all and --name are mutually exclusive")
			}
			if !*confirm {
				return shared.UsageError("--confirm is required to remove stored credentials")
			}

			if authsvc.ShouldBypassKeychain() {
				fmt.Fprintln(os.Stderr, "Note: ASC_BYPASS_KEYCHAIN is set, so auth logout changes config files only and leaves keychain entries untouched.")
			}

			opts := authsvc.RemoveOptions{IncludeGlobalConfig: *includeGlobal}
			if trimmedName != "" {
				err := removeStoredCredential(trimmedName, opts)
				if !opts.IncludeGlobalConfig {
					warnRetainedGlobalCredentials(trimmedName)
				}
				if err != nil {
					return fmt.Errorf("auth logout: failed to remove credentials: %w", err)
				}
				fmt.Printf("Successfully removed stored credential '%s'\n", trimmedName)
				return nil
			}

			err := removeStoredCredentials(opts)
			if !opts.IncludeGlobalConfig {
				warnRetainedGlobalCredentials("")
			}
			if err != nil {
				return fmt.Errorf("auth logout: failed to remove credentials: %w", err)
			}

			fmt.Println("Successfully removed stored credentials")
			return nil
		},
	}
}

// AuthStatus command factory
func AuthStatusCommand() *ffcli.Command {
	fs := flag.NewFlagSet("auth status", flag.ExitOnError)
	output := shared.BindOutputFlagsWithAllowed(fs, "output", defaultAuthStatusOutputFormat(), "Output format: table, json", "table", "json")
	verbose := fs.Bool("verbose", false, "Show detailed storage information")
	validate := fs.Bool("validate", false, "Validate stored credentials via network")

	return &ffcli.Command{
		Name:       "status",
		ShortUsage: "asc auth status",
		ShortHelp:  "Show active profile and authentication status.",
		LongHelp: `Show current authentication status.

Displays information about stored API keys and which one is currently active.
Add --validate to perform a network validation for each stored credential.

Examples:
  asc auth status
  asc auth status --output json
  asc auth status --verbose
  asc auth status --validate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			normalizedOutput, err := shared.ValidateOutputFormatAllowed(*output.Output, *output.Pretty, "table", "json")
			if err != nil {
				return shared.UsageError(err.Error())
			}

			credentialLister := listCredentialSummaries
			if *validate {
				credentialLister = listStoredCredentials
			}
			credentials, err := credentialLister()
			var listWarning *authsvc.CredentialsWarning
			if err != nil {
				warning, ok := errors.AsType[*authsvc.CredentialsWarning](err)
				if !ok {
					return fmt.Errorf("auth status: failed to list credentials: %w", err)
				}
				listWarning = warning
			}

			bypassKeychain := authsvc.ShouldBypassKeychain()
			isKeychainAvailable, keychainErr := keychainAvailable()
			configPath, configErr := config.Path()
			storageBackend := "System Keychain"
			storageLocation := "system keychain"
			var warnings []string
			if listWarning != nil {
				warnings = append(warnings, listWarning.Error())
			}

			if bypassKeychain {
				storageBackend = "Config File"
				storageLocation = "unknown"
				if configErr == nil {
					storageLocation = configPath
				}
				warnings = append(warnings, "Keychain bypassed via ASC_BYPASS_KEYCHAIN (truthy values: 1/true/yes/on).")
			} else if !isKeychainAvailable {
				storageBackend = "Config File"
				storageLocation = "unknown"
				if configErr == nil {
					storageLocation = configPath
				}
				if keychainErr != nil {
					warnings = append(warnings, fmt.Sprintf("System keychain error: %v", keychainErr))
				} else {
					warnings = append(warnings, "System keychain unavailable. Credentials stored in config file.")
				}
			}

			hasConfigCreds := false
			for _, cred := range credentials {
				if cred.Source == "config" {
					hasConfigCreds = true
					break
				}
			}
			if hasConfigCreds && isKeychainAvailable && !bypassKeychain {
				warnings = append(warnings, "Some credentials are stored in config file (less secure).")
			}

			if normalizedOutput == "table" {
				fmt.Printf("Credential storage: %s\n", storageBackend)
				fmt.Printf("Location: %s\n", storageLocation)
				for _, warning := range warnings {
					fmt.Printf("Warning: %s\n", warning)
				}
				if *verbose {
					fmt.Printf("Keychain available: %t\n", isKeychainAvailable)
					if keychainErr != nil {
						fmt.Printf("Keychain error: %v\n", keychainErr)
					}
					if configErr == nil {
						fmt.Printf("Config path: %s\n", configPath)
					}
				}
				fmt.Println()
			}

			profile := shared.ResolveProfileName()
			envKeyID := strings.TrimSpace(os.Getenv("ASC_KEY_ID"))
			envIssuerID := strings.TrimSpace(os.Getenv("ASC_ISSUER_ID"))
			envKeyTypeRaw := strings.TrimSpace(os.Getenv("ASC_KEY_TYPE"))
			envKeyType := config.NormalizeCredentialKeyType(envKeyTypeRaw)
			envKeyTypeValid := envKeyTypeRaw == "" || config.IsValidCredentialKeyType(envKeyType)
			hasKeyEnv := strings.TrimSpace(os.Getenv("ASC_PRIVATE_KEY_PATH")) != "" ||
				strings.TrimSpace(os.Getenv(shared.PrivateKeyEnvVar)) != "" ||
				strings.TrimSpace(os.Getenv(shared.PrivateKeyBase64EnvVar)) != ""
			envProvided := envKeyID != "" || envIssuerID != "" || hasKeyEnv || envKeyTypeRaw != ""
			envComplete := shared.HasCompleteEnvironmentCredentials()
			environmentNote := authStatusEnvironmentNote(profile, bypassKeychain, envProvided, envComplete, envKeyTypeValid)
			activeEnvironmentSource := profile == "" && !bypassKeychain && envComplete && envKeyTypeValid

			validationFailures := 0
			var validationDiagnostic shared.Diagnostic
			hasValidationDiagnostic := false
			validationDiagnosticConsistent := true
			credentialOutput := make([]authStatusCredentialOutput, 0, len(credentials))
			if len(credentials) == 0 {
				if normalizedOutput == "table" {
					if activeEnvironmentSource {
						fmt.Println("No stored credentials found.")
					} else {
						fmt.Println("No credentials stored. Run 'asc auth login' to get started.")
					}
				}
			} else {
				if normalizedOutput == "table" {
					fmt.Println("Stored credentials:")
					asc.RenderTable(
						[]string{"Name", "Key ID", "Default", "Stored In"},
						buildAuthStatusCredentialRows(credentials),
					)
				}
				for _, cred := range credentials {
					credentialEntry := authStatusCredentialOutput{
						Name:      cred.Name,
						KeyID:     cred.KeyID,
						IsDefault: cred.IsDefault,
						StoredIn:  credentialStorageLabel(cred),
					}
					if *validate {
						if err := statusValidateCredential(ctx, cred); err != nil {
							if _, ok := errors.AsType[*permissionWarning](err); ok {
								credentialEntry.Validation = "works"
								credentialEntry.ValidationDetail = "insufficient permissions for " + credentialProbeName(cred.KeyType)
								if normalizedOutput == "table" {
									fmt.Printf("    %s (Key ID: %s): works (%s)\n", cred.Name, cred.KeyID, credentialEntry.ValidationDetail)
								}
							} else {
								validationFailures++
								credentialEntry.Validation = "failed"
								credentialEntry.ValidationError = err.Error()
								if diagnostic, ok := shared.DiagnosticFromError(err); ok {
									if !hasValidationDiagnostic {
										validationDiagnostic = diagnostic
										hasValidationDiagnostic = true
									} else if diagnostic != validationDiagnostic {
										validationDiagnosticConsistent = false
									}
								} else {
									validationDiagnosticConsistent = false
								}
								if normalizedOutput == "table" {
									fmt.Printf("    %s (Key ID: %s): failed (%v)\n", cred.Name, cred.KeyID, err)
								}
							}
						} else {
							credentialEntry.Validation = "works"
							if normalizedOutput == "table" {
								fmt.Printf("    %s (Key ID: %s): works\n", cred.Name, cred.KeyID)
							}
						}
					}
					credentialOutput = append(credentialOutput, credentialEntry)
				}
			}

			if normalizedOutput == "table" && environmentNote != "" {
				fmt.Println(environmentNote)
			}

			if normalizedOutput == "json" {
				payload := authStatusOutput{
					StorageBackend:                 storageBackend,
					StorageLocation:                storageLocation,
					Warnings:                       warnings,
					Credentials:                    credentialOutput,
					Profile:                        profile,
					EnvironmentCredentialsProvided: envProvided,
					EnvironmentCredentialsComplete: envComplete,
					EnvironmentNote:                environmentNote,
					ValidationFailures:             validationFailures,
				}
				if *verbose {
					payload.KeychainAvailable = boolPointer(isKeychainAvailable)
					if keychainErr != nil {
						payload.KeychainError = keychainErr.Error()
					}
					if configErr == nil {
						payload.ConfigPath = configPath
					}
				}
				if err := shared.PrintOutput(payload, "json", *output.Pretty); err != nil {
					return err
				}
			}

			if *validate && validationFailures > 0 {
				validationError := error(fmt.Errorf("auth status: validation failed for %d credential(s)", validationFailures))
				if hasValidationDiagnostic && validationDiagnosticConsistent {
					validationError = shared.WithDiagnostic(validationError, validationDiagnostic.Code, validationDiagnostic.Parameter)
				}
				return shared.NewValidationReportedError(validationError)
			}
			return nil
		},
	}
}

func credentialStorageLabel(cred authsvc.Credential) string {
	if strings.TrimSpace(cred.SourcePath) != "" {
		return fmt.Sprintf("%s: %s", cred.Source, cred.SourcePath)
	}
	if strings.TrimSpace(cred.Source) != "" {
		return cred.Source
	}
	return "unknown"
}

type authStatusCredentialOutput struct {
	Name             string `json:"name"`
	KeyID            string `json:"keyId"`
	IsDefault        bool   `json:"isDefault"`
	StoredIn         string `json:"storedIn"`
	Validation       string `json:"validation,omitempty"`
	ValidationDetail string `json:"validationDetail,omitempty"`
	ValidationError  string `json:"validationError,omitempty"`
}

type authStatusOutput struct {
	StorageBackend                 string                       `json:"storageBackend"`
	StorageLocation                string                       `json:"storageLocation"`
	Warnings                       []string                     `json:"warnings,omitempty"`
	Credentials                    []authStatusCredentialOutput `json:"credentials"`
	Profile                        string                       `json:"profile,omitempty"`
	EnvironmentCredentialsProvided bool                         `json:"environmentCredentialsProvided"`
	EnvironmentCredentialsComplete bool                         `json:"environmentCredentialsComplete"`
	EnvironmentNote                string                       `json:"environmentNote,omitempty"`
	ValidationFailures             int                          `json:"validationFailures,omitempty"`
	KeychainAvailable              *bool                        `json:"keychainAvailable,omitempty"`
	KeychainError                  string                       `json:"keychainError,omitempty"`
	ConfigPath                     string                       `json:"configPath,omitempty"`
}

func buildAuthStatusCredentialRows(credentials []authsvc.Credential) [][]string {
	rows := make([][]string, 0, len(credentials))
	for _, cred := range credentials {
		defaultLabel := "no"
		if cred.IsDefault {
			defaultLabel = "yes"
		}
		rows = append(rows, []string{
			cred.Name,
			cred.KeyID,
			defaultLabel,
			credentialStorageLabel(cred),
		})
	}
	return rows
}

func defaultAuthStatusOutputFormat() string {
	if shared.DefaultOutputFormat() == "json" {
		return "json"
	}
	return "table"
}

func authStatusEnvironmentNote(profile string, bypassKeychain, envProvided, envComplete, envKeyTypeValid bool) string {
	if profile != "" && envProvided {
		return fmt.Sprintf("Profile %q selected; environment credentials will be ignored.", profile)
	}
	if !envProvided {
		return ""
	}
	if !bypassKeychain {
		if envComplete && envKeyTypeValid {
			return "Complete environment credential fields take precedence when no profile is selected; stored credential lookup is skipped."
		}
		return ""
	}
	if !envKeyTypeValid {
		return "Environment credentials are incomplete. ASC_KEY_TYPE must be one of: team, individual, enterprise."
	}
	if !envComplete {
		return "Environment credentials are incomplete. Set ASC_KEY_ID, ASC_ISSUER_ID (unless ASC_KEY_TYPE=individual), and one of ASC_PRIVATE_KEY_PATH/ASC_PRIVATE_KEY/ASC_PRIVATE_KEY_B64."
	}
	return "Environment credentials detected (ASC_KEY_ID present). In bypass mode, stored config credentials are preferred; environment credentials are only used when no stored config credential is selected."
}

func boolPointer(value bool) *bool {
	result := value
	return &result
}

// AuthIssuerIDCommand prints the active App Store Connect issuer ID.
func AuthIssuerIDCommand() *ffcli.Command {
	fs := flag.NewFlagSet("auth issuer-id", flag.ExitOnError)

	name := fs.String("name", "", "Profile name (uses default profile if omitted)")
	output := shared.BindOutputFlagsWithAllowed(fs, "output", "text", "Output format: text (raw issuer ID), json", "text", "json")

	return &ffcli.Command{
		Name:       "issuer-id",
		ShortUsage: "asc auth issuer-id [flags]",
		ShortHelp:  "Print the active App Store Connect issuer ID.",
		LongHelp: `Print the active App Store Connect issuer ID.

This reads the issuer ID from the currently resolved authentication credentials
without making a network request.

Examples:
  asc auth issuer-id
  asc auth issuer-id --name "MyKey"
  asc auth issuer-id --output json`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedName := strings.TrimSpace(*name)
			if trimmedName == "" && *name != "" {
				return shared.UsageError("--name cannot be blank")
			}
			normalizedOutput, err := shared.ValidateOutputFormatAllowed(*output.Output, *output.Pretty, "text", "json")
			if err != nil {
				return shared.UsageError(err.Error())
			}

			cred, err := shared.ResolveAuthCredentials(trimmedName)
			if err != nil {
				return fmt.Errorf("auth issuer-id: %w", err)
			}

			if normalizedOutput == "json" {
				return shared.PrintOutput(struct {
					IssuerID string `json:"issuerId"`
					Profile  string `json:"profile,omitempty"`
				}{
					IssuerID: cred.IssuerID,
					Profile:  cred.Profile,
				}, "json", *output.Pretty)
			}

			fmt.Print(cred.IssuerID)
			return nil
		},
	}
}

// authTokenConfirmRequiredMessage explains the --confirm gate in one line. It
// keeps the "required" wording so the failure stays classified as
// missing_required with usage exit code 2.
const authTokenConfirmRequiredMessage = "--confirm is required because `asc auth token` prints a live bearer token to stdout, " +
	"where it can leak into shell history, logs, or CI output"

// authTokenConfirmInvocation reconstructs the exact command that satisfies the
// --confirm gate, preserving the root --profile and --strict-auth overrides and
// every command flag the caller already supplied so agents can re-run it
// verbatim. ok is false when a supplied value cannot be rendered as a copyable
// shell argument, in which case no suggestion is printed at all.
func authTokenConfirmInvocation(fs *flag.FlagSet) (string, bool) {
	rootFlags, ok := shared.RootFlagsForReinvocation()
	if !ok {
		return "", false
	}
	parts := []string{"asc"}
	parts = append(parts, rootFlags...)
	parts = append(parts, "auth", "token")
	if fs != nil {
		// flag.Visit walks only the flags that were set, in lexical order, so
		// the rendered invocation is deterministic.
		fs.Visit(func(f *flag.Flag) {
			if !ok || f.Name == "confirm" {
				// --confirm is re-added last, including when the caller passed
				// --confirm=false.
				return
			}
			if boolFlag, isBool := f.Value.(interface{ IsBoolFlag() bool }); isBool && boolFlag.IsBoolFlag() {
				if f.Value.String() == "true" {
					parts = append(parts, "--"+f.Name)
				}
				return
			}
			quoted, quotable := shared.ShellQuote(f.Value.String())
			if !quotable {
				ok = false
				return
			}
			parts = append(parts, "--"+f.Name, quoted)
		})
	}
	if !ok {
		return "", false
	}
	parts = append(parts, "--confirm")
	return strings.Join(parts, " "), true
}

// AuthTokenCommand prints a signed JWT for direct API calls.
func AuthTokenCommand() *ffcli.Command {
	fs := flag.NewFlagSet("auth token", flag.ExitOnError)

	name := fs.String("name", "", "Profile name (uses default profile if omitted)")
	confirm := fs.Bool("confirm", false, "Confirm printing a live JWT to stdout")
	output := shared.BindOutputFlagsWithAllowed(fs, "output", "text", "Output format: text (raw token), json", "text", "json")

	return &ffcli.Command{
		Name:       "token",
		ShortUsage: "asc auth token --confirm [flags]",
		ShortHelp:  "Print a signed JWT for direct App Store Connect API calls.",
		LongHelp: `Print a signed JWT for direct App Store Connect API calls.

The token is valid for 10 minutes and printed to stdout so it can be used
in shell pipelines.

Requires --confirm because this prints a live bearer token to stdout.

Examples:
  asc auth token --confirm
  asc auth token --name "MyKey" --confirm
  asc auth token --confirm --output json
  curl -H "Authorization: Bearer $(asc auth token --confirm)" https://api.appstoreconnect.apple.com/v1/apps`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedName := strings.TrimSpace(*name)
			if trimmedName == "" && *name != "" {
				return shared.UsageError("--name cannot be blank")
			}
			normalizedOutput, err := shared.ValidateOutputFormatAllowed(*output.Output, *output.Pretty, "text", "json")
			if err != nil {
				return shared.UsageError(err.Error())
			}
			if !*confirm {
				// UsageError writes the "Error:" line first, so the exact
				// re-invocation is appended straight after it and still lands
				// ahead of the usage page ffcli renders for flag.ErrHelp.
				usageErr := shared.UsageError(authTokenConfirmRequiredMessage)
				if invocation, ok := authTokenConfirmInvocation(fs); ok {
					fmt.Fprintf(os.Stderr, "Re-run: %s\n", invocation)
				}
				return usageErr
			}

			cred, err := shared.ResolveAuthCredentials(trimmedName)
			if err != nil {
				return fmt.Errorf("auth token: %w", err)
			}

			privateKey, err := loadCredentialKey(cred)
			if err != nil {
				return fmt.Errorf("auth token: %w", err)
			}

			generateToken := asc.GenerateJWT
			if config.IsEnterpriseCredentialKeyType(cred.KeyType) {
				generateToken = asc.GenerateEnterpriseJWT
			}
			token, err := generateToken(cred.KeyID, cred.IssuerID, privateKey)
			if err != nil {
				return fmt.Errorf("auth token: failed to generate JWT: %w", err)
			}

			if normalizedOutput == "json" {
				return shared.PrintOutput(struct {
					Token   string `json:"token"`
					KeyID   string `json:"keyId"`
					Profile string `json:"profile,omitempty"`
				}{
					Token:   token,
					KeyID:   cred.KeyID,
					Profile: cred.Profile,
				}, "json", *output.Pretty)
			}

			fmt.Print(token)
			return nil
		},
	}
}

func loadCredentialKey(cred shared.ResolvedAuthCredentials) (*ecdsa.PrivateKey, error) {
	if pemValue := cred.KeyPEM; pemValue != "" {
		privateKey, err := authsvc.LoadPrivateKeyFromPEM([]byte(pemValue))
		if err != nil {
			return nil, shared.WithPrivateKeyDiagnostic(err, err)
		}
		return privateKey, nil
	}
	if err := authsvc.ValidateKeyFile(cred.KeyPath); err != nil {
		return nil, shared.WithPrivateKeyDiagnostic(fmt.Errorf("invalid private key: %w", err), err)
	}
	privateKey, err := authsvc.LoadPrivateKey(cred.KeyPath)
	if err != nil {
		return nil, shared.WithPrivateKeyDiagnostic(err, err)
	}
	return privateKey, nil
}
