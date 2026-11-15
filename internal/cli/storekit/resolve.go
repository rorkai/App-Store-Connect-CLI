package storekit

import (
	"context"
	"encoding/base64"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	storekitapi "github.com/rudrankriyam/App-Store-Connect-CLI/internal/storekit"
)

type commonFlags struct {
	Profile     *string
	BundleID    *string
	Environment *string
}

func bindCommonFlags(fs *flag.FlagSet) commonFlags {
	return commonFlags{
		Profile:     fs.String("storekit-profile", "", "StoreKit credential profile (or ASC_STOREKIT_PROFILE)"),
		BundleID:    fs.String("bundle-id", "", "App bundle identifier (or ASC_STOREKIT_BUNDLE_ID)"),
		Environment: fs.String("environment", "", "StoreKit environment: production or sandbox"),
	}
}

func resolveClient(ctx context.Context, d *shared.Deps, flags commonFlags) (*storekitapi.Client, storekitapi.Environment, error) {
	environment, err := resolveEnvironment(flags.Environment)
	if err != nil {
		return nil, "", err
	}
	credentials, _, err := resolveCredentialsWithSource(flags)
	if err != nil {
		return nil, "", err
	}
	_ = ctx
	client, err := storekitapi.NewClient(credentials, environment, storekitapi.WithTransport(d.HTTPTransport()))
	if err != nil {
		return nil, "", err
	}
	return client, environment, nil
}

func resolveEnvironment(flagValue *string) (storekitapi.Environment, error) {
	value := stringValue(flagValue)
	if value == "" {
		value = strings.TrimSpace(os.Getenv("ASC_STOREKIT_ENVIRONMENT"))
	}
	if value == "" {
		return "", fmt.Errorf("--environment is required (or set ASC_STOREKIT_ENVIRONMENT)")
	}
	return storekitapi.ParseEnvironment(value)
}

func resolveCredentialsWithSource(flags commonFlags) (storekitapi.Credentials, string, error) {
	profile := stringValue(flags.Profile)
	profileSource := "--storekit-profile"
	if profile == "" {
		profile = strings.TrimSpace(os.Getenv("ASC_STOREKIT_PROFILE"))
		profileSource = "ASC_STOREKIT_PROFILE"
	}
	strict := parseBoolEnv("ASC_STOREKIT_STRICT_AUTH")
	if profile != "" {
		if strict && hasEnvironmentKeyCredentials() {
			return storekitapi.Credentials{}, "", fmt.Errorf("mixed StoreKit authentication sources detected: profile and ASC_STOREKIT_* key credentials")
		}
		credentials, _, err := storekitapi.GetCredentialsWithSource(profile)
		if err != nil {
			return storekitapi.Credentials{}, "", err
		}
		credentials.BundleID = resolveBundleID(flags.BundleID, credentials.BundleID)
		if credentials.BundleID == "" {
			return storekitapi.Credentials{}, "", fmt.Errorf("--bundle-id is required (or set ASC_STOREKIT_BUNDLE_ID or store it in the StoreKit profile)")
		}
		return credentials, profileSource, nil
	}
	environmentCredentials, environmentSet, err := credentialsFromEnvironment()
	if err != nil {
		return storekitapi.Credentials{}, "", err
	}
	if environmentSet {
		environmentCredentials.BundleID = resolveBundleID(flags.BundleID, environmentCredentials.BundleID)
		if environmentCredentials.BundleID == "" {
			return storekitapi.Credentials{}, "", fmt.Errorf("--bundle-id is required (or set ASC_STOREKIT_BUNDLE_ID)")
		}
		return environmentCredentials, "ASC_STOREKIT_* environment credentials", nil
	}
	credentials, source, err := storekitapi.GetCredentialsWithSource("")
	if err != nil {
		return storekitapi.Credentials{}, "", err
	}
	credentials.BundleID = resolveBundleID(flags.BundleID, credentials.BundleID)
	if credentials.BundleID == "" {
		return storekitapi.Credentials{}, "", fmt.Errorf("--bundle-id is required (or set ASC_STOREKIT_BUNDLE_ID or store it in the StoreKit profile)")
	}
	return credentials, source, nil
}

func credentialsFromEnvironment() (storekitapi.Credentials, bool, error) {
	credentials := storekitapi.Credentials{
		KeyID:          strings.TrimSpace(os.Getenv("ASC_STOREKIT_KEY_ID")),
		IssuerID:       strings.TrimSpace(os.Getenv("ASC_STOREKIT_ISSUER_ID")),
		PrivateKeyPath: strings.TrimSpace(os.Getenv("ASC_STOREKIT_PRIVATE_KEY_PATH")),
		PrivateKeyPEM:  strings.TrimSpace(os.Getenv("ASC_STOREKIT_PRIVATE_KEY")),
		BundleID:       strings.TrimSpace(os.Getenv("ASC_STOREKIT_BUNDLE_ID")),
	}
	encoded := strings.TrimSpace(os.Getenv("ASC_STOREKIT_PRIVATE_KEY_B64"))
	if encoded != "" {
		decoded, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			return storekitapi.Credentials{}, false, fmt.Errorf("ASC_STOREKIT_PRIVATE_KEY_B64 is not valid base64: %w", err)
		}
		if credentials.PrivateKeyPEM == "" {
			credentials.PrivateKeyPEM = string(decoded)
		}
	}
	anyKeyValue := credentials.KeyID != "" || credentials.IssuerID != "" || credentials.PrivateKeyPath != "" || credentials.PrivateKeyPEM != ""
	complete := credentials.KeyID != "" && credentials.IssuerID != "" && (credentials.PrivateKeyPath != "" || credentials.PrivateKeyPEM != "")
	if anyKeyValue && !complete {
		return storekitapi.Credentials{}, false, fmt.Errorf("incomplete StoreKit environment credentials: set ASC_STOREKIT_KEY_ID, ASC_STOREKIT_ISSUER_ID, and one of ASC_STOREKIT_PRIVATE_KEY_PATH, ASC_STOREKIT_PRIVATE_KEY, or ASC_STOREKIT_PRIVATE_KEY_B64")
	}
	return credentials, complete, nil
}

func hasEnvironmentKeyCredentials() bool {
	for _, name := range []string{
		"ASC_STOREKIT_KEY_ID",
		"ASC_STOREKIT_ISSUER_ID",
		"ASC_STOREKIT_PRIVATE_KEY_PATH",
		"ASC_STOREKIT_PRIVATE_KEY",
		"ASC_STOREKIT_PRIVATE_KEY_B64",
	} {
		if strings.TrimSpace(os.Getenv(name)) != "" {
			return true
		}
	}
	return false
}

func resolveBundleID(flagValue *string, stored string) string {
	if value := stringValue(flagValue); value != "" {
		return value
	}
	if value := strings.TrimSpace(os.Getenv("ASC_STOREKIT_BUNDLE_ID")); value != "" {
		return value
	}
	return strings.TrimSpace(stored)
}

func parseBoolEnv(name string) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(name))) {
	case "1", "true", "yes", "y", "on":
		return true
	default:
		return false
	}
}
