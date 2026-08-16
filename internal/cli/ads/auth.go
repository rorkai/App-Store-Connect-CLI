package ads

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/appleads"
	authsvc "github.com/rudrankriyam/App-Store-Connect-CLI/internal/auth"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/config"
)

// AuthCommand returns the Apple Ads auth command group.
func AuthCommand() *ffcli.Command {
	fs := flag.NewFlagSet("ads auth", flag.ExitOnError)
	return &ffcli.Command{
		Name:       "auth",
		ShortUsage: "asc ads auth <subcommand> [flags]",
		ShortHelp:  "Manage Apple Ads API credentials.",
		LongHelp: `Manage Apple Ads API credentials.

Apple Ads uses OAuth client credentials and separate Apple Ads API keys.

Examples:
  asc ads auth login --name "Ads" --client-id "SEARCHADS..." --team-id "SEARCHADS..." --key-id "KEY_ID" --private-key ./private-key.pem
  asc ads auth status
  asc ads auth discover --output json
  asc ads auth token --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			AuthLoginCommand(),
			AuthStatusCommand(),
			AuthDiscoverCommand(),
			AuthSwitchCommand(),
			AuthTokenCommand(),
			AuthDoctorCommand(),
			AuthLogoutCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

func AuthLoginCommand() *ffcli.Command {
	fs := flag.NewFlagSet("ads auth login", flag.ExitOnError)
	name := fs.String("name", "", "Friendly name for this Apple Ads key")
	clientID := fs.String("client-id", "", "Apple Ads OAuth client ID")
	teamID := fs.String("team-id", "", "Apple Ads OAuth team ID")
	keyID := fs.String("key-id", "", "Apple Ads API key ID")
	privateKey := fs.String("private-key", "", "Path to Apple Ads EC private key PEM")
	org := fs.String("org", "", "Default Apple Ads organization ID")
	adAccount := fs.String("ad-account", "", "Default Apple Ads ad account ID")
	bypassKeychain := fs.Bool("bypass-keychain", false, "Store credentials in config.json instead of keychain")
	local := fs.Bool("local", false, "When bypassing keychain, write to ./.asc/config.json")
	network := fs.Bool("network", false, "Validate credentials with Apple Ads API")
	skipValidation := fs.Bool("skip-validation", false, "Skip private key and network validation checks")

	return &ffcli.Command{
		Name:       "login",
		ShortUsage: "asc ads auth login [flags]",
		ShortHelp:  "Register and store Apple Ads API credentials.",
		LongHelp: `Register and store Apple Ads API credentials.

Examples:
  asc ads auth login --name "Ads" --client-id "SEARCHADS..." --team-id "SEARCHADS..." --key-id "KEY_ID" --private-key ./private-key.pem --org "123456" --ad-account "987654"
  asc ads auth login --bypass-keychain --local --name "Ads" --client-id "SEARCHADS..." --team-id "SEARCHADS..." --key-id "KEY_ID" --private-key ./private-key.pem`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := rejectUnexpectedArgs(args); err != nil {
				return err
			}
			if strings.TrimSpace(*name) == "" {
				fmt.Fprintln(os.Stderr, "Error: --name is required")
				return shared.MissingRequiredUsageError()
			}
			if strings.TrimSpace(*clientID) == "" {
				fmt.Fprintln(os.Stderr, "Error: --client-id is required")
				return shared.MissingRequiredUsageError()
			}
			if strings.TrimSpace(*teamID) == "" {
				fmt.Fprintln(os.Stderr, "Error: --team-id is required")
				return shared.MissingRequiredUsageError()
			}
			if strings.TrimSpace(*keyID) == "" {
				fmt.Fprintln(os.Stderr, "Error: --key-id is required")
				return shared.MissingRequiredUsageError()
			}
			if strings.TrimSpace(*privateKey) == "" {
				fmt.Fprintln(os.Stderr, "Error: --private-key is required")
				return shared.MissingRequiredUsageError()
			}
			if *skipValidation && *network {
				return shared.UsageError("--skip-validation and --network are mutually exclusive")
			}
			if *local && !*bypassKeychain && !appleads.ShouldBypassKeychain() {
				return shared.UsageError("--local requires --bypass-keychain or ASC_ADS_BYPASS_KEYCHAIN set to 1/true/yes/y/on")
			}
			if !*skipValidation {
				if err := authsvc.ValidateKeyFile(*privateKey); err != nil {
					return shared.UsageErrorf("ads auth login: invalid private key: %v", err)
				}
			}

			credentials := appleads.Credentials{
				ClientID:       *clientID,
				TeamID:         *teamID,
				KeyID:          *keyID,
				PrivateKeyPath: *privateKey,
				OrgID:          *org,
				AdAccountID:    *adAccount,
			}
			if *network {
				client, err := appleads.NewClient(credentials)
				if err != nil {
					return fmt.Errorf("ads auth login: %w", err)
				}
				requestCtx, cancel := requestContext(ctx)
				defer cancel()
				spec, err := authPlatformEndpointSpec("me", "view")
				if err != nil {
					return fmt.Errorf("ads auth login: %w", err)
				}
				if _, err := client.Do(requestCtx, spec, nil, nil, nil); err != nil {
					return fmt.Errorf("ads auth login: network validation failed: %w", err)
				}
			}

			if *bypassKeychain || appleads.ShouldBypassKeychain() {
				if *local {
					path, err := config.LocalPath()
					if err != nil {
						return fmt.Errorf("ads auth login: %w", err)
					}
					if err := appleads.StoreCredentialsConfigAt(*name, credentials, path); err != nil {
						return fmt.Errorf("ads auth login: failed to store credentials: %w", err)
					}
				} else if err := appleads.StoreCredentialsConfig(*name, credentials); err != nil {
					return fmt.Errorf("ads auth login: failed to store credentials: %w", err)
				}
			} else if err := appleads.StoreCredentials(*name, credentials); err != nil {
				return fmt.Errorf("ads auth login: failed to store credentials: %w", err)
			}
			fmt.Printf("Successfully registered Apple Ads API key '%s'\n", strings.TrimSpace(*name))
			return nil
		},
	}
}

func AuthStatusCommand() *ffcli.Command {
	fs := flag.NewFlagSet("ads auth status", flag.ExitOnError)
	output := shared.BindOutputFlagsWithAllowed(fs, "output", "table", "Output format: table, json", "table", "json")
	verbose := fs.Bool("verbose", false, "Show detailed storage information")
	validate := fs.Bool("validate", false, "Validate stored credentials via network")

	return &ffcli.Command{
		Name:       "status",
		ShortUsage: "asc ads auth status [flags]",
		ShortHelp:  "Show Apple Ads authentication status.",
		LongHelp: `Show Apple Ads authentication status.

Examples:
  asc ads auth status
  asc ads auth status --output json
  asc ads auth status --verbose
  asc ads auth status --validate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := rejectUnexpectedArgs(args); err != nil {
				return err
			}
			normalized, err := shared.ValidateOutputFormatAllowed(*output.Output, *output.Pretty, "table", "json")
			if err != nil {
				return shared.UsageError(err.Error())
			}
			active := statusActiveContext()
			credentials, err := appleads.ListCredentials()
			credentialsError := ""
			if err != nil {
				credentialsError = err.Error()
			}
			rows := make([]adsAuthStatusRow, 0, len(credentials))
			failures := 0
			for _, cred := range credentials {
				row := adsAuthStatusRow{
					Name:        cred.Name,
					ClientID:    cred.ClientID,
					TeamID:      cred.TeamID,
					KeyID:       cred.KeyID,
					OrgID:       cred.OrgID,
					AdAccountID: cred.AdAccountID,
					Default:     cred.IsDefault,
					Source:      cred.Source,
					Validated:   !*validate,
				}
				if *verbose {
					row.SourcePath = cred.SourcePath
				}
				if *validate {
					client, err := appleads.NewClient(cred.Credentials)
					if err == nil {
						requestCtx, cancel := requestContext(ctx)
						spec, specErr := authPlatformEndpointSpec("me", "view")
						if specErr != nil {
							err = specErr
						} else {
							_, err = client.Do(requestCtx, spec, nil, nil, nil)
						}
						cancel()
					}
					if err != nil {
						failures++
						row.Validated = false
						row.Error = err.Error()
					} else {
						row.Validated = true
					}
				}
				rows = append(rows, row)
			}
			result := adsAuthStatusOutput{
				Storage:          storageDescription(rows),
				Active:           active,
				Credentials:      rows,
				CredentialsError: credentialsError,
			}
			if normalized == "json" {
				if err := shared.PrintOutput(result, "json", *output.Pretty); err != nil {
					return err
				}
			} else {
				printStatusTable(result)
			}
			if *validate && credentialsError != "" {
				return shared.NewReportedError(fmt.Errorf("ads auth status: validation skipped because credentials could not be listed: %s", credentialsError))
			}
			if failures > 0 {
				return shared.NewReportedError(fmt.Errorf("ads auth status: validation failed for %d credential(s)", failures))
			}
			return nil
		},
	}
}

type adsAuthStatusOutput struct {
	Storage          string             `json:"storage"`
	Active           adsAuthContext     `json:"active"`
	Credentials      []adsAuthStatusRow `json:"credentials"`
	CredentialsError string             `json:"credentials_error,omitempty"`
}

type adsAuthContext struct {
	Profile           string `json:"profile,omitempty"`
	Source            string `json:"source,omitempty"`
	OrgID             string `json:"org_id,omitempty"`
	OrgIDSource       string `json:"org_id_source,omitempty"`
	OrgError          string `json:"org_error,omitempty"`
	AdAccountID       string `json:"ad_account_id,omitempty"`
	AdAccountIDSource string `json:"ad_account_id_source,omitempty"`
	AdAccountError    string `json:"ad_account_error,omitempty"`
	Error             string `json:"error,omitempty"`
}

type adsAuthStatusRow struct {
	Name        string `json:"name"`
	ClientID    string `json:"client_id"`
	TeamID      string `json:"team_id"`
	KeyID       string `json:"key_id"`
	OrgID       string `json:"org_id,omitempty"`
	AdAccountID string `json:"ad_account_id,omitempty"`
	Default     bool   `json:"default"`
	Source      string `json:"source"`
	SourcePath  string `json:"source_path,omitempty"`
	Validated   bool   `json:"validated"`
	Error       string `json:"error,omitempty"`
}

func printStatusTable(result adsAuthStatusOutput) {
	fmt.Printf("Credential storage: %s\n\n", result.Storage)
	printActiveContext(result.Active)
	fmt.Println()
	if result.CredentialsError != "" {
		fmt.Printf("Stored credentials: unavailable (%s)\n", result.CredentialsError)
		return
	}
	if len(result.Credentials) == 0 {
		fmt.Println("No Apple Ads credentials stored. Run 'asc ads auth login' to get started.")
		return
	}
	for _, cred := range result.Credentials {
		defaultMarker := ""
		if cred.Default {
			defaultMarker = " (default)"
		}
		fmt.Printf("%s%s\n", cred.Name, defaultMarker)
		fmt.Printf("  Client ID: %s\n", cred.ClientID)
		fmt.Printf("  Team ID: %s\n", cred.TeamID)
		fmt.Printf("  Key ID: %s\n", cred.KeyID)
		if cred.OrgID != "" {
			fmt.Printf("  Org ID: %s\n", cred.OrgID)
		}
		if cred.AdAccountID != "" {
			fmt.Printf("  Ad account ID: %s\n", cred.AdAccountID)
		}
		fmt.Printf("  Source: %s\n", cred.Source)
		if cred.SourcePath != "" {
			fmt.Printf("  Source path: %s\n", cred.SourcePath)
		}
		if cred.Error != "" {
			fmt.Printf("  Validation: failed: %s\n", cred.Error)
		} else if cred.Validated {
			fmt.Println("  Validation: ok")
		}
	}
}

func printActiveContext(active adsAuthContext) {
	if active.Error != "" && active.Source == "" {
		fmt.Printf("Active auth: unavailable (%s)\n", active.Error)
		return
	}
	if active.Source == "" {
		fmt.Println("Active auth: none")
		return
	}
	fmt.Printf("Active auth: %s\n", active.Source)
	if active.Profile != "" {
		fmt.Printf("  Profile: %s\n", active.Profile)
	}
	if active.OrgError != "" {
		fmt.Printf("  Org ID: unavailable (%s)\n", active.OrgError)
	} else if active.OrgID != "" {
		if active.OrgIDSource != "" {
			fmt.Printf("  Org ID: %s (%s)\n", active.OrgID, active.OrgIDSource)
		} else {
			fmt.Printf("  Org ID: %s\n", active.OrgID)
		}
	} else {
		fmt.Println("  Org ID: not selected")
	}
	if active.AdAccountError != "" {
		fmt.Printf("  Ad account ID: unavailable (%s)\n", active.AdAccountError)
	} else if active.AdAccountID != "" {
		if active.AdAccountIDSource != "" {
			fmt.Printf("  Ad account ID: %s (%s)\n", active.AdAccountID, active.AdAccountIDSource)
		} else {
			fmt.Printf("  Ad account ID: %s\n", active.AdAccountID)
		}
	} else {
		fmt.Println("  Ad account ID: not selected")
	}
}

func statusActiveContext() adsAuthContext {
	credentials, source, err := resolveCredentialsWithSource(commonFlags{})
	if err != nil {
		if isNoAdsCredentialError(err) {
			return adsAuthContext{}
		}
		return adsAuthContext{Error: err.Error()}
	}
	orgID, orgSource, orgErr := resolveOrgIDWithSource(commonFlags{}, credentials)
	adAccountID, adAccountSource, adAccountErr := resolveAdAccountIDWithSource(commonFlags{}, credentials)
	active := adsAuthContext{
		Profile:           credentials.Profile,
		Source:            source,
		OrgID:             orgID,
		OrgIDSource:       orgSource,
		AdAccountID:       adAccountID,
		AdAccountIDSource: adAccountSource,
	}
	if orgErr != nil {
		// Keep the pre-4.4 JSON error field for existing status consumers while
		// also exposing the context-specific field used by new consumers.
		active.Error = orgErr.Error()
		active.OrgError = orgErr.Error()
	}
	if adAccountErr != nil {
		active.AdAccountError = adAccountErr.Error()
	}
	return active
}

func isNoAdsCredentialError(err error) bool {
	return errors.Is(err, appleads.ErrDefaultCredentialsNotFound)
}

func storageDescription(rows []adsAuthStatusRow) string {
	if appleads.ShouldBypassKeychain() {
		return "Config File"
	}
	hasConfig := false
	hasKeychain := false
	for _, row := range rows {
		switch row.Source {
		case "config":
			hasConfig = true
		case "keychain":
			hasKeychain = true
		}
	}
	switch {
	case hasConfig && hasKeychain:
		return "System Keychain + Config File"
	case hasConfig:
		return "Config File"
	}
	return "System Keychain"
}

func AuthDiscoverCommand() *ffcli.Command {
	fs := flag.NewFlagSet("ads auth discover", flag.ExitOnError)
	common := commonFlags{
		AdsProfile: fs.String("ads-profile", "", "Use named Apple Ads authentication profile"),
		Org:        fs.String("org", "", "Apple Ads organization ID to mark active"),
		AdAccount:  fs.String("ad-account", "", "Apple Ads ad account ID to mark active"),
	}
	output := shared.BindOutputFlagsWithAllowed(fs, "output", "table", "Output format: table, json", "table", "json")
	return &ffcli.Command{
		Name:       "discover",
		ShortUsage: "asc ads auth discover [flags]",
		ShortHelp:  "Discover Apple Ads user and ad account access.",
		LongHelp: `Discover Apple Ads user and ad account access.

This read-only command calls GET v1/me and GET v1/acls. It does not print access tokens.

Examples:
  asc ads auth discover
  asc ads auth discover --output json
  asc ads auth discover --ads-profile "Ads"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := rejectUnexpectedArgs(args); err != nil {
				return err
			}
			normalized, err := shared.ValidateOutputFormatAllowed(*output.Output, *output.Pretty, "table", "json")
			if err != nil {
				return shared.UsageError(err.Error())
			}
			credentials, source, err := resolveCredentialsWithSource(common)
			if err != nil {
				return fmt.Errorf("ads auth discover: %w", err)
			}
			orgID, orgSource := discoverOrgIDWithSource(common, credentials)
			adAccountID, adAccountSource, err := discoverAdAccountIDWithSource(common, credentials)
			if err != nil {
				return shared.UsageErrorf("ads auth discover: %v", err)
			}
			client, err := appleads.NewClient(credentials)
			if err != nil {
				return fmt.Errorf("ads auth discover: %w", err)
			}
			requestCtx, cancel := requestContext(ctx)
			defer cancel()

			meSpec, err := authPlatformEndpointSpec("me", "view")
			if err != nil {
				return fmt.Errorf("ads auth discover: %w", err)
			}
			meRaw, err := client.Do(requestCtx, meSpec, nil, nil, nil)
			if err != nil {
				return fmt.Errorf("ads auth discover: me lookup failed: %w", err)
			}
			aclsSpec, err := authPlatformEndpointSpec("acls", "list")
			if err != nil {
				return fmt.Errorf("ads auth discover: %w", err)
			}
			aclsRaw, err := client.Do(requestCtx, aclsSpec, nil, nil, nil)
			if err != nil {
				return fmt.Errorf("ads auth discover: acl lookup failed: %w", err)
			}

			me, err := platformEnvelopeResult(meRaw)
			if err != nil {
				return fmt.Errorf("ads auth discover: me response parse failed: %w", err)
			}
			me, err = normalizePlatformDiscoveryMe(me)
			if err != nil {
				return fmt.Errorf("ads auth discover: me response parse failed: %w", err)
			}
			accounts, err := summarizePlatformACLAccounts(aclsRaw, adAccountID)
			if err != nil {
				return fmt.Errorf("ads auth discover: acl response parse failed: %w", err)
			}

			result := adsAuthDiscoveryOutput{
				AuthSource:        source,
				Profile:           credentials.Profile,
				OrgID:             orgID,
				OrgIDSource:       orgSource,
				AdAccountID:       adAccountID,
				AdAccountIDSource: adAccountSource,
				Me:                me,
				Accounts:          accounts,
			}
			if normalized == "json" {
				return shared.PrintOutput(result, "json", *output.Pretty)
			}
			printDiscoveryTable(result)
			return nil
		},
	}
}

func discoverAdAccountIDWithSource(flags commonFlags, credentials appleads.Credentials) (string, string, error) {
	adAccountID, source, err := resolveAdAccountIDWithSource(flags, credentials)
	if err == nil {
		return adAccountID, source, nil
	}

	// Discovery can proceed without account context when the optional root
	// configuration is unreadable. Explicit account selectors must never be
	// discarded, though: doing so would make a malformed user choice look as if
	// no account was selected.
	if flags.AdAccount != nil && *flags.AdAccount != "" ||
		os.Getenv("ASC_ADS_AD_ACCOUNT_ID") != "" ||
		credentials.AdAccountID != "" {
		return "", "", err
	}
	return "", "", nil
}

func authPlatformEndpointSpec(path ...string) (appleads.EndpointSpec, error) {
	spec, ok := appleads.PlatformEndpointByCommandPath(path...)
	if !ok {
		return appleads.EndpointSpec{}, fmt.Errorf("internal error: missing Apple Ads Platform endpoint spec for command %q", strings.Join(path, " "))
	}
	return spec, nil
}

func discoverOrgIDWithSource(flags commonFlags, credentials appleads.Credentials) (string, string) {
	orgID, orgSource, err := resolveOrgIDWithSource(flags, credentials)
	if err != nil {
		return "", ""
	}
	return orgID, orgSource
}

type adsAuthDiscoveryOutput struct {
	AuthSource        string                  `json:"auth_source"`
	Profile           string                  `json:"profile,omitempty"`
	OrgID             string                  `json:"org_id,omitempty"`
	OrgIDSource       string                  `json:"org_id_source,omitempty"`
	AdAccountID       string                  `json:"ad_account_id,omitempty"`
	AdAccountIDSource string                  `json:"ad_account_id_source,omitempty"`
	Me                json.RawMessage         `json:"me"`
	Accounts          []adsAuthAccountSummary `json:"accounts"`
}

type adsAuthAccountSummary struct {
	AdAccountID string   `json:"ad_account_id,omitempty"`
	OrgID       string   `json:"org_id,omitempty"`
	Name        string   `json:"name,omitempty"`
	Roles       []string `json:"roles,omitempty"`
	Active      bool     `json:"active"`
}

func printDiscoveryTable(result adsAuthDiscoveryOutput) {
	fmt.Printf("Auth source: %s\n", result.AuthSource)
	if result.Profile != "" {
		fmt.Printf("Profile: %s\n", result.Profile)
	}
	if user := discoveryUserSummary(result.Me); user != "" {
		fmt.Printf("User: %s\n", user)
	}
	if result.OrgID != "" {
		if result.OrgIDSource != "" {
			fmt.Printf("Selected org: %s (%s)\n", result.OrgID, result.OrgIDSource)
		} else {
			fmt.Printf("Selected org: %s\n", result.OrgID)
		}
	} else {
		fmt.Println("Selected org: none")
	}
	if result.AdAccountID != "" {
		if result.AdAccountIDSource != "" {
			fmt.Printf("Selected ad account: %s (%s)\n", result.AdAccountID, result.AdAccountIDSource)
		} else {
			fmt.Printf("Selected ad account: %s\n", result.AdAccountID)
		}
	} else {
		fmt.Println("Selected ad account: none")
	}
	if len(result.Accounts) == 0 {
		fmt.Println("Accounts: none returned")
		return
	}
	fmt.Println("Accounts:")
	for _, account := range result.Accounts {
		marker := ""
		if account.Active {
			marker = " (active)"
		}
		label := account.AdAccountID
		if label == "" {
			label = account.OrgID
		}
		if account.Name != "" {
			label += " - " + account.Name
		}
		fmt.Printf("  %s%s\n", label, marker)
		if len(account.Roles) > 0 {
			fmt.Printf("    Roles: %s\n", strings.Join(account.Roles, ", "))
		}
	}
}

func discoveryUserSummary(me json.RawMessage) string {
	var user map[string]any
	if err := unmarshalJSONPreservingNumbers(me, &user); err != nil {
		return ""
	}
	id := jsonScalarString(firstMapValue(user, "userId", "id"))
	name := jsonScalarString(firstMapValue(user, "name"))
	email := jsonScalarString(firstMapValue(user, "email"))
	switch {
	case name != "" && id != "":
		return name + " (" + id + ")"
	case name != "":
		return name
	case email != "":
		return email
	case id != "":
		return id
	default:
		return jsonScalarString(firstMapValue(user, "parentOrgId"))
	}
}

func platformEnvelopeResult(raw appleads.RawResponse) (json.RawMessage, error) {
	var envelope struct {
		Result json.RawMessage `json:"result"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, err
	}
	result := bytes.TrimSpace(envelope.Result)
	if len(result) == 0 || bytes.Equal(result, []byte("null")) {
		return nil, fmt.Errorf("response is missing a non-null result")
	}
	return json.RawMessage(result), nil
}

func normalizePlatformDiscoveryMe(raw json.RawMessage) (json.RawMessage, error) {
	var value map[string]any
	if err := unmarshalJSONPreservingNumbers(raw, &value); err != nil {
		return nil, err
	}
	userID := jsonScalarString(firstMapValue(value, "userId", "id"))
	name := jsonScalarString(firstMapValue(value, "name"))
	orgID := jsonScalarString(firstMapValue(value, "orgId", "parentOrgId"))
	return json.Marshal(struct {
		ID          string `json:"id"`
		Name        string `json:"name"`
		UserID      string `json:"userId,omitempty"`
		ParentOrgID string `json:"parentOrgId,omitempty"`
		OrgID       string `json:"orgId,omitempty"`
	}{
		ID:          userID,
		Name:        name,
		UserID:      userID,
		ParentOrgID: orgID,
		OrgID:       orgID,
	})
}

func summarizePlatformACLAccounts(raw appleads.RawResponse, activeAdAccountID string) ([]adsAuthAccountSummary, error) {
	result, err := platformEnvelopeResult(raw)
	if err != nil {
		return nil, err
	}
	var payload struct {
		ACLs []struct {
			AdAccount map[string]any `json:"adAccount"`
			Roles     []string       `json:"roles"`
		} `json:"acls"`
	}
	if err := unmarshalJSONPreservingNumbers(result, &payload); err != nil {
		return nil, err
	}
	accounts := make([]adsAuthAccountSummary, 0, len(payload.ACLs))
	for _, item := range payload.ACLs {
		adAccountID := jsonScalarString(firstMapValue(item.AdAccount, "id"))
		orgID := jsonScalarString(firstMapValue(item.AdAccount, "orgId", "orgID", "organizationId"))
		account := adsAuthAccountSummary{
			AdAccountID: adAccountID,
			OrgID:       orgID,
			Name:        jsonScalarString(firstMapValue(item.AdAccount, "name")),
			Roles:       append([]string(nil), item.Roles...),
			Active:      adAccountID != "" && activeAdAccountID != "" && adAccountID == activeAdAccountID,
		}
		accounts = append(accounts, account)
	}
	return accounts, nil
}

func unmarshalJSONPreservingNumbers(data []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	return decoder.Decode(target)
}

func firstMapValue(item map[string]any, keys ...string) any {
	for _, key := range keys {
		if value, ok := item[key]; ok {
			return value
		}
	}
	return nil
}

func jsonScalarString(value any) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case float64:
		return strings.TrimSuffix(strings.TrimSuffix(fmt.Sprintf("%.0f", typed), ".0"), ".")
	case json.Number:
		return typed.String()
	default:
		return ""
	}
}

func AuthSwitchCommand() *ffcli.Command {
	fs := flag.NewFlagSet("ads auth switch", flag.ExitOnError)
	name := fs.String("name", "", "Apple Ads profile name")
	return &ffcli.Command{
		Name:       "switch",
		ShortUsage: "asc ads auth switch --name NAME",
		ShortHelp:  "Switch the default Apple Ads profile.",
		LongHelp: `Switch the default Apple Ads profile.

Examples:
  asc ads auth switch --name "Ads"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := rejectUnexpectedArgs(args); err != nil {
				return err
			}
			if strings.TrimSpace(*name) == "" {
				fmt.Fprintln(os.Stderr, "Error: --name is required")
				return shared.MissingRequiredUsageError()
			}
			if err := appleads.SetDefaultCredentials(*name); err != nil {
				return fmt.Errorf("ads auth switch: %w", err)
			}
			fmt.Printf("Default Apple Ads profile set to '%s'\n", strings.TrimSpace(*name))
			return nil
		},
	}
}

func AuthTokenCommand() *ffcli.Command {
	fs := flag.NewFlagSet("ads auth token", flag.ExitOnError)
	common := commonFlags{AdsProfile: fs.String("ads-profile", "", "Use named Apple Ads authentication profile")}
	output := shared.BindOutputFlagsWithAllowed(fs, "output", "text", "Output format: text, json", "text", "json")
	confirm := fs.Bool("confirm", false, "Confirm printing a sensitive access token")
	return &ffcli.Command{
		Name:       "token",
		ShortUsage: "asc ads auth token --confirm [flags]",
		ShortHelp:  "Print an Apple Ads access token.",
		LongHelp: `Print an Apple Ads access token.

Examples:
  asc ads auth token --confirm
  asc ads auth token --confirm --output json`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := rejectUnexpectedArgs(args); err != nil {
				return err
			}
			if !*confirm {
				return shared.UsageError("--confirm is required")
			}
			normalized, err := shared.ValidateOutputFormatAllowed(*output.Output, *output.Pretty, "text", "json")
			if err != nil {
				return shared.UsageError(err.Error())
			}
			credentials, err := resolveCredentials(common)
			if err != nil {
				return fmt.Errorf("ads auth token: %w", err)
			}
			client, err := appleads.NewClient(credentials)
			if err != nil {
				return fmt.Errorf("ads auth token: %w", err)
			}
			requestCtx, cancel := requestContext(ctx)
			defer cancel()
			token, err := client.AccessToken(requestCtx)
			if err != nil {
				return fmt.Errorf("ads auth token: %w", err)
			}
			if normalized == "json" {
				return shared.PrintOutput(struct {
					AccessToken string `json:"access_token"`
				}{AccessToken: token}, "json", *output.Pretty)
			}
			fmt.Println(token)
			return nil
		},
	}
}

func AuthDoctorCommand() *ffcli.Command {
	fs := flag.NewFlagSet("ads auth doctor", flag.ExitOnError)
	output := shared.BindOutputFlagsWithAllowed(fs, "output", "text", "Output format: text, json", "text", "json")
	return &ffcli.Command{
		Name:       "doctor",
		ShortUsage: "asc ads auth doctor [flags]",
		ShortHelp:  "Diagnose Apple Ads authentication configuration.",
		LongHelp: `Diagnose Apple Ads authentication configuration.

Examples:
  asc ads auth doctor
  asc ads auth doctor --output json`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := rejectUnexpectedArgs(args); err != nil {
				return err
			}
			normalized, err := shared.ValidateOutputFormatAllowed(*output.Output, *output.Pretty, "text", "json")
			if err != nil {
				return shared.UsageError(err.Error())
			}
			credentials, err := appleads.ListCredentials()
			checks := []doctorCheck{}
			if err != nil {
				checks = append(checks, doctorCheck{Status: "fail", Message: err.Error()})
			} else if len(credentials) == 0 {
				checks = append(checks, doctorCheck{Status: "warn", Message: "No Apple Ads credentials stored"})
			} else {
				checks = append(checks, doctorCheck{Status: "ok", Message: fmt.Sprintf("%d Apple Ads credential(s) found", len(credentials))})
			}
			if os.Getenv("ASC_ADS_ACCESS_TOKEN") != "" {
				checks = append(checks, doctorCheck{Status: "info", Message: "ASC_ADS_ACCESS_TOKEN is set"})
			}
			result := doctorReport{Checks: checks}
			if normalized == "json" {
				return shared.PrintOutput(result, "json", *output.Pretty)
			}
			fmt.Println("Apple Ads Auth Doctor")
			for _, check := range checks {
				fmt.Printf("  [%s] %s\n", strings.ToUpper(check.Status), check.Message)
			}
			return nil
		},
	}
}

type doctorReport struct {
	Checks []doctorCheck `json:"checks"`
}

type doctorCheck struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

func AuthLogoutCommand() *ffcli.Command {
	fs := flag.NewFlagSet("ads auth logout", flag.ExitOnError)
	all := fs.Bool("all", false, "Remove all stored Apple Ads credentials")
	confirm := fs.Bool("confirm", false, "Confirm removal of all Apple Ads credentials")
	name := fs.String("name", "", "Remove a named Apple Ads credential")
	return &ffcli.Command{
		Name:       "logout",
		ShortUsage: "asc ads auth logout [flags]",
		ShortHelp:  "Remove stored Apple Ads credentials.",
		LongHelp: `Remove stored Apple Ads credentials.

Examples:
  asc ads auth logout --all --confirm
  asc ads auth logout --name "Ads"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := rejectUnexpectedArgs(args); err != nil {
				return err
			}
			trimmedName := strings.TrimSpace(*name)
			if trimmedName == "" && *name != "" {
				return shared.UsageError("--name cannot be blank")
			}
			if trimmedName != "" && *all {
				return shared.UsageError("--all and --name are mutually exclusive")
			}
			if trimmedName == "" && !*all {
				return shared.UsageError("provide --name or --all")
			}
			if *all && !*confirm {
				return shared.UsageError("--all requires --confirm")
			}
			if trimmedName != "" {
				if err := appleads.RemoveCredentials(trimmedName); err != nil {
					return fmt.Errorf("ads auth logout: %w", err)
				}
				fmt.Printf("Successfully removed Apple Ads credential '%s'\n", trimmedName)
				return nil
			}
			if err := appleads.RemoveAllCredentials(); err != nil {
				return fmt.Errorf("ads auth logout: %w", err)
			}
			fmt.Println("Successfully removed Apple Ads credentials")
			return nil
		},
	}
}
