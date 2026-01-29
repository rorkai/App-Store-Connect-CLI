package shared

import (
	"fmt"
	"os"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

const (
	serverKeyIDEnvVar            = "ASC_IAP_KEY_ID"
	serverIssuerIDEnvVar         = "ASC_IAP_ISSUER_ID"
	serverPrivateKeyPathEnvVar   = "ASC_IAP_PRIVATE_KEY_PATH"
	serverPrivateKeyEnvVar       = "ASC_IAP_PRIVATE_KEY"
	serverPrivateKeyBase64EnvVar = "ASC_IAP_PRIVATE_KEY_B64"
	serverBundleIDEnvVar         = "ASC_IAP_BUNDLE_ID"
	serverEnvEnvVar              = "ASC_IAP_ENV"
)

type serverAPICredentials struct {
	keyID       string
	issuerID    string
	keyPath     string
	bundleID    string
	environment asc.ServerEnvironment
}

func resolveServerAPICredentials() (serverAPICredentials, error) {
	keyID := strings.TrimSpace(os.Getenv(serverKeyIDEnvVar))
	issuerID := strings.TrimSpace(os.Getenv(serverIssuerIDEnvVar))
	bundleID := strings.TrimSpace(os.Getenv(serverBundleIDEnvVar))
	envValue := strings.TrimSpace(os.Getenv(serverEnvEnvVar))

	keyPath, err := resolveServerAPIPrivateKeyPath()
	if err != nil {
		return serverAPICredentials{}, err
	}
	if keyID == "" || issuerID == "" || bundleID == "" || envValue == "" || keyPath == "" {
		return serverAPICredentials{}, fmt.Errorf(
			"missing server API credentials. Set %s, %s, %s (or %s/%s), %s, %s",
			serverKeyIDEnvVar,
			serverIssuerIDEnvVar,
			serverPrivateKeyPathEnvVar,
			serverPrivateKeyEnvVar,
			serverPrivateKeyBase64EnvVar,
			serverBundleIDEnvVar,
			serverEnvEnvVar,
		)
	}

	env, err := asc.ParseServerEnvironment(envValue)
	if err != nil {
		return serverAPICredentials{}, err
	}

	return serverAPICredentials{
		keyID:       keyID,
		issuerID:    issuerID,
		keyPath:     keyPath,
		bundleID:    bundleID,
		environment: env,
	}, nil
}

func resolveServerAPIPrivateKeyPath() (string, error) {
	if path := strings.TrimSpace(os.Getenv(serverPrivateKeyPathEnvVar)); path != "" {
		return path, nil
	}
	if value := strings.TrimSpace(os.Getenv(serverPrivateKeyBase64EnvVar)); value != "" {
		decoded, err := decodeBase64Secret(value)
		if err != nil {
			return "", fmt.Errorf("%s: %w", serverPrivateKeyBase64EnvVar, err)
		}
		return writeTempPrivateKey(decoded)
	}
	if value := strings.TrimSpace(os.Getenv(serverPrivateKeyEnvVar)); value != "" {
		return writeTempPrivateKey([]byte(normalizePrivateKeyValue(value)))
	}
	return "", nil
}

func getServerAPIClient() (*asc.ServerAPIClient, error) {
	creds, err := resolveServerAPICredentials()
	if err != nil {
		return nil, err
	}
	return asc.NewServerAPIClient(creds.keyID, creds.issuerID, creds.keyPath, creds.bundleID, creds.environment)
}

// GetServerAPIClient returns an App Store Server API client.
func GetServerAPIClient() (*asc.ServerAPIClient, error) {
	return getServerAPIClient()
}
