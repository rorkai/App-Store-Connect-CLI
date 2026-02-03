package cmd

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/auth"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/config"
)

const authKeysURL = "https://appstoreconnect.apple.com/access/integrations/api"

// Auth command factory
func AuthCommand() *ffcli.Command {
	fs := flag.NewFlagSet("auth", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "auth",
		ShortUsage: "asc auth <subcommand> [flags]",
		ShortHelp:  "Manage App Store Connect API authentication.",
		LongHelp: `Manage App Store Connect API authentication.

Authentication is handled via App Store Connect API keys. Generate keys at:
https://appstoreconnect.apple.com/access/integrations/api

Credentials are stored in the system keychain when available, with a config fallback.
A repo-local ./.asc/config.json (if present) takes precedence.`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			AuthInitCommand(),
			AuthLoginCommand(),
			AuthSwitchCommand(),
			AuthLogoutCommand(),
			AuthStatusCommand(),
			AuthDoctorCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			if len(args) == 0 {
				return flag.ErrHelp
			}
			return nil
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

This writes ~/.asc/config.json with empty fields and secure permissions.
Use --local to write ./.asc/config.json in the current repo instead.

Examples:
  asc auth init
  asc auth init --local
  asc auth init --force
  asc auth init --open`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			var path string
			var err error
			if *local {
				path, err = config.LocalPath()
			} else {
				path, err = config.GlobalPath()
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

			if *open {
				if err := openURL(authKeysURL); err != nil {
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
			return asc.PrintJSON(result)
		},
	}
}

// AuthLogin command factory
func AuthLoginCommand() *ffcli.Command {
	fs := flag.NewFlagSet("auth login", flag.ExitOnError)

	name := fs.String("name", "", "Friendly name for this key")
	keyID := fs.String("key-id", "", "App Store Connect API Key ID")
	issuerID := fs.String("issuer-id", "", "App Store Connect Issuer ID")
	keyPath := fs.String("private-key", "", "Path to private key (.p8) file")
	bypassKeychain := fs.Bool("bypass-keychain", false, "Store credentials in config.json instead of keychain")
	local := fs.Bool("local", false, "When bypassing keychain, write to ./.asc/config.json")

	return &ffcli.Command{
		Name:       "login",
		ShortUsage: "asc auth login [flags]",
		ShortHelp:  "Register and store App Store Connect API key.",
		LongHelp: `Register and store App Store Connect API key.

This command stores your API credentials in the system keychain when available,
with a local config fallback (restricted permissions). Use --bypass-keychain to
explicitly bypass keychain and write credentials to ~/.asc/config.json instead.
Add --local to write ./.asc/config.json for the current repo.

Examples:
  asc auth login --name "MyKey" --key-id "ABC123" --issuer-id "DEF456" --private-key /path/to/AuthKey.p8
  asc auth login --bypass-keychain --local --name "MyKey" --key-id "ABC123" --issuer-id "DEF456" --private-key /path/to/AuthKey.p8

The private key file path is stored securely. The key content is never saved.`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *local && !*bypassKeychain {
				return fmt.Errorf("auth login: --local requires --bypass-keychain")
			}
			if *name == "" {
				fmt.Fprintln(os.Stderr, "Error: --name is required")
				return flag.ErrHelp
			}
			if *keyID == "" {
				fmt.Fprintln(os.Stderr, "Error: --key-id is required")
				return flag.ErrHelp
			}
			if *issuerID == "" {
				fmt.Fprintln(os.Stderr, "Error: --issuer-id is required")
				return flag.ErrHelp
			}
			if *keyPath == "" {
				fmt.Fprintln(os.Stderr, "Error: --private-key is required")
				return flag.ErrHelp
			}

			// Validate the key file exists and is parseable
			if err := auth.ValidateKeyFile(*keyPath); err != nil {
				return fmt.Errorf("auth login: invalid private key: %w", err)
			}

			// Store credentials securely
			if *bypassKeychain {
				if *local {
					path, err := config.LocalPath()
					if err != nil {
						return fmt.Errorf("auth login: %w", err)
					}
					if err := auth.StoreCredentialsConfigAt(*name, *keyID, *issuerID, *keyPath, path); err != nil {
						return fmt.Errorf("auth login: failed to store credentials: %w", err)
					}
				} else {
					if err := auth.StoreCredentialsConfig(*name, *keyID, *issuerID, *keyPath); err != nil {
						return fmt.Errorf("auth login: failed to store credentials: %w", err)
					}
				}
			} else {
				if err := auth.StoreCredentials(*name, *keyID, *issuerID, *keyPath); err != nil {
					return fmt.Errorf("auth login: failed to store credentials: %w", err)
				}
			}

			fmt.Printf("Successfully registered API key '%s'\n", *name)
			return nil
		},
	}
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
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedName := strings.TrimSpace(*name)
			if trimmedName == "" {
				fmt.Fprintln(os.Stderr, "Error: --name is required")
				return flag.ErrHelp
			}

			credentials, err := auth.ListCredentials()
			if err != nil {
				return fmt.Errorf("auth switch: failed to list credentials: %w", err)
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

			if err := auth.SetDefaultCredentials(trimmedName); err != nil {
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
	all := fs.Bool("all", false, "Remove all stored credentials (default)")
	name := fs.String("name", "", "Remove a named credential")

	return &ffcli.Command{
		Name:       "logout",
		ShortUsage: "asc auth logout [flags]",
		ShortHelp:  "Remove stored API credentials.",
		LongHelp: `Remove stored API credentials.

Examples:
  asc auth logout
  asc auth logout --all
  asc auth logout --name "MyKey"`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedName := strings.TrimSpace(*name)
			if trimmedName == "" && *name != "" {
				return fmt.Errorf("auth logout: --name cannot be blank")
			}
			if trimmedName != "" && *all {
				return fmt.Errorf("auth logout: --all and --name are mutually exclusive")
			}

			if trimmedName != "" {
				if err := auth.RemoveCredentials(trimmedName); err != nil {
					return fmt.Errorf("auth logout: failed to remove credentials: %w", err)
				}
				fmt.Printf("Successfully removed stored credential '%s'\n", trimmedName)
				return nil
			}

			if err := auth.RemoveAllCredentials(); err != nil {
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

	return &ffcli.Command{
		Name:       "status",
		ShortUsage: "asc auth status",
		ShortHelp:  "Show current authentication status.",
		LongHelp: `Show current authentication status.

Displays information about stored API keys and which one is currently active.

Examples:
  asc auth status`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			credentials, err := auth.ListCredentials()
			if err != nil {
				return fmt.Errorf("auth status: failed to list credentials: %w", err)
			}

			if len(credentials) == 0 {
				fmt.Println("No credentials stored. Run 'asc auth login' to get started.")
			} else {
				fmt.Println("Stored credentials:")
				for _, cred := range credentials {
					active := ""
					if cred.IsDefault {
						active = " (default)"
					}
					fmt.Printf("  - %s (Key ID: %s)%s\n", cred.Name, cred.KeyID, active)
				}
			}

			profile := resolveProfileName()
			bypassKeychain := auth.ShouldBypassKeychain()
			envKeyID := strings.TrimSpace(os.Getenv("ASC_KEY_ID"))
			envIssuerID := strings.TrimSpace(os.Getenv("ASC_ISSUER_ID"))
			hasKeyEnv := strings.TrimSpace(os.Getenv("ASC_PRIVATE_KEY_PATH")) != "" ||
				strings.TrimSpace(os.Getenv(privateKeyEnvVar)) != "" ||
				strings.TrimSpace(os.Getenv(privateKeyBase64EnvVar)) != ""
			envProvided := envKeyID != "" || envIssuerID != "" || hasKeyEnv
			envComplete := envKeyID != "" && envIssuerID != "" && hasKeyEnv

			if profile != "" && envProvided {
				fmt.Printf("Profile %q selected; environment credentials will be ignored.\n", profile)
			} else if bypassKeychain && envComplete {
				fmt.Printf("Environment credentials detected (ASC_KEY_ID: %s). With ASC_BYPASS_KEYCHAIN=1, they will be used when no profile is selected.\n", envKeyID)
			} else if bypassKeychain && envProvided && !envComplete {
				fmt.Println("Environment credentials are incomplete. Set ASC_KEY_ID, ASC_ISSUER_ID, and one of ASC_PRIVATE_KEY_PATH/ASC_PRIVATE_KEY/ASC_PRIVATE_KEY_B64.")
			}
			return nil
		},
	}
}
