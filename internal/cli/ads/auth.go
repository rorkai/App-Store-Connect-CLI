package ads

import (
	"bytes"
	"context"
	"encoding/json"
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
  asc ads auth login --name "Ads" --client-id "SEARCHADS..." --team-id "SEARCHADS..." --key-id "KEY_ID" --private-key ./private-key.pem --org "123456"
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
					return fmt.Errorf("ads auth login: invalid private key: %w", err)
				}
			}

			credentials := appleads.Credentials{
				ClientID:       *clientID,
				TeamID:         *teamID,
				KeyID:          *keyID,
				PrivateKeyPath: *privateKey,
				OrgID:          *org,
			}
			if *network {
				client, err := appleads.NewClient(credentials)
				if err != nil {
					return fmt.Errorf("ads auth login: %w", err)
				}
				requestCtx, cancel := requestContext(ctx)
				defer cancel()
				spec, _ := appleads.EndpointByCommandPath("me", "view")
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
					Name:      cred.Name,
					ClientID:  cred.ClientID,
					TeamID:    cred.TeamID,
					KeyID:     cred.KeyID,
					OrgID:     cred.OrgID,
					Default:   cred.IsDefault,
					Source:    cred.Source,
					Validated: !*validate,
				}
				if *verbose {
					row.SourcePath = cred.SourcePath
				}
				if *validate {
					client, err := appleads.NewClient(cred.Credentials)
					if err == nil {
						requestCtx, cancel := requestContext(ctx)
						spec, _ := appleads.EndpointByCommandPath("me", "view")
						_, err = client.Do(requestCtx, spec, nil, nil, nil)
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
	Profile     string `json:"profile,omitempty"`
	Source      string `json:"source,omitempty"`
	OrgID       string `json:"org_id,omitempty"`
	OrgIDSource string `json:"org_id_source,omitempty"`
	Error       string `json:"error,omitempty"`
}

type adsAuthStatusRow struct {
	Name       string `json:"name"`
	ClientID   string `json:"client_id"`
	TeamID     string `json:"team_id"`
	KeyID      string `json:"key_id"`
	OrgID      string `json:"org_id,omitempty"`
	Default    bool   `json:"default"`
	Source     string `json:"source"`
	SourcePath string `json:"source_path,omitempty"`
	Validated  bool   `json:"validated"`
	Error      string `json:"error,omitempty"`
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
	if active.Error != "" {
		if active.Source == "" {
			fmt.Printf("Active auth: unavailable (%s)\n", active.Error)
			return
		}
		fmt.Printf("Active auth: %s\n", active.Source)
		if active.Profile != "" {
			fmt.Printf("  Profile: %s\n", active.Profile)
		}
		fmt.Printf("  Org ID: unavailable (%s)\n", active.Error)
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
	if active.OrgID != "" {
		if active.OrgIDSource != "" {
			fmt.Printf("  Org ID: %s (%s)\n", active.OrgID, active.OrgIDSource)
		} else {
			fmt.Printf("  Org ID: %s\n", active.OrgID)
		}
	} else {
		fmt.Println("  Org ID: not selected")
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
	orgID, orgSource, err := resolveOrgIDWithSource(commonFlags{}, credentials)
	if err != nil {
		return adsAuthContext{
			Profile: credentials.Profile,
			Source:  source,
			Error:   err.Error(),
		}
	}
	return adsAuthContext{
		Profile:     credentials.Profile,
		Source:      source,
		OrgID:       orgID,
		OrgIDSource: orgSource,
	}
}

func isNoAdsCredentialError(err error) bool {
	if err == nil {
		return false
	}
	return err.Error() == "default credentials not found"
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
	}
	output := shared.BindOutputFlagsWithAllowed(fs, "output", "table", "Output format: table, json", "table", "json")
	return &ffcli.Command{
		Name:       "discover",
		ShortUsage: "asc ads auth discover [flags]",
		ShortHelp:  "Discover Apple Ads user and organization access.",
		LongHelp: `Discover Apple Ads user and organization access.

This read-only command calls GET v5/me and GET v5/acls. It does not print access tokens.

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
			client, err := appleads.NewClient(credentials)
			if err != nil {
				return fmt.Errorf("ads auth discover: %w", err)
			}
			requestCtx, cancel := requestContext(ctx)
			defer cancel()

			meSpec, _ := appleads.EndpointByCommandPath("me", "view")
			meRaw, err := client.Do(requestCtx, meSpec, nil, nil, nil)
			if err != nil {
				return fmt.Errorf("ads auth discover: me lookup failed: %w", err)
			}
			aclsSpec, _ := appleads.EndpointByCommandPath("acls", "list")
			aclsRaw, err := client.Do(requestCtx, aclsSpec, nil, nil, nil)
			if err != nil {
				return fmt.Errorf("ads auth discover: acl lookup failed: %w", err)
			}

			me, err := envelopeData(meRaw)
			if err != nil {
				return fmt.Errorf("ads auth discover: me response parse failed: %w", err)
			}
			accounts, err := summarizeACLAccounts(aclsRaw, orgID)
			if err != nil {
				return fmt.Errorf("ads auth discover: acl response parse failed: %w", err)
			}

			result := adsAuthDiscoveryOutput{
				AuthSource:  source,
				Profile:     credentials.Profile,
				OrgID:       orgID,
				OrgIDSource: orgSource,
				Me:          me,
				Accounts:    accounts,
			}
			if normalized == "json" {
				return shared.PrintOutput(result, "json", *output.Pretty)
			}
			printDiscoveryTable(result)
			return nil
		},
	}
}

func discoverOrgIDWithSource(flags commonFlags, credentials appleads.Credentials) (string, string) {
	orgID, orgSource, err := resolveOrgIDWithSource(flags, credentials)
	if err != nil {
		return "", ""
	}
	return orgID, orgSource
}

type adsAuthDiscoveryOutput struct {
	AuthSource  string                  `json:"auth_source"`
	Profile     string                  `json:"profile,omitempty"`
	OrgID       string                  `json:"org_id,omitempty"`
	OrgIDSource string                  `json:"org_id_source,omitempty"`
	Me          json.RawMessage         `json:"me"`
	Accounts    []adsAuthAccountSummary `json:"accounts"`
}

type adsAuthAccountSummary struct {
	OrgID  string   `json:"org_id,omitempty"`
	Name   string   `json:"name,omitempty"`
	Roles  []string `json:"roles,omitempty"`
	Active bool     `json:"active"`
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
		label := account.OrgID
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
	if err := json.Unmarshal(me, &user); err != nil {
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

func envelopeData(raw appleads.RawResponse) (json.RawMessage, error) {
	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, err
	}
	if len(envelope.Data) == 0 {
		return json.RawMessage("null"), nil
	}
	return envelope.Data, nil
}

func summarizeACLAccounts(raw appleads.RawResponse, activeOrgID string) ([]adsAuthAccountSummary, error) {
	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, err
	}
	items, err := aclDataItems(envelope.Data)
	if err != nil {
		return nil, err
	}
	accounts := make([]adsAuthAccountSummary, 0, len(items))
	for _, item := range items {
		orgID := jsonScalarString(firstMapValue(item, "orgId", "orgID", "organizationId", "id"))
		account := adsAuthAccountSummary{
			OrgID:  orgID,
			Name:   jsonScalarString(firstMapValue(item, "orgName", "organizationName", "name")),
			Roles:  jsonStringList(firstMapValue(item, "roleNames", "roles")),
			Active: orgID != "" && activeOrgID != "" && orgID == activeOrgID,
		}
		accounts = append(accounts, account)
	}
	return accounts, nil
}

func aclDataItems(data json.RawMessage) ([]map[string]any, error) {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return nil, nil
	}
	if trimmed[0] == '[' {
		var items []map[string]any
		if err := json.Unmarshal(trimmed, &items); err != nil {
			return nil, err
		}
		return items, nil
	}
	var item map[string]any
	if err := json.Unmarshal(trimmed, &item); err != nil {
		return nil, err
	}
	return []map[string]any{item}, nil
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

func jsonStringList(value any) []string {
	items, ok := value.([]any)
	if !ok {
		return nil
	}
	result := make([]string, 0, len(items))
	for _, item := range items {
		if text := jsonScalarString(item); text != "" {
			result = append(result, text)
		}
	}
	return result
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
			normalized, err := shared.ValidateOutputFormatAllowed(*output.Output, *output.Pretty, "text", "json")
			if err != nil {
				return shared.UsageError(err.Error())
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
			normalized, err := shared.ValidateOutputFormatAllowed(*output.Output, *output.Pretty, "text", "json")
			if err != nil {
				return shared.UsageError(err.Error())
			}
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
