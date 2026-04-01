package main

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/config"
)

func (a *App) CheckAuthStatus() (AuthStatus, error) {
	ascPath, err := a.resolveASCPath()
	if err != nil {
		return AuthStatus{RawOutput: "Could not find asc binary: " + err.Error()}, nil
	}

	ctx, cancel := context.WithTimeout(a.contextOrBackground(), 10*time.Second)
	defer cancel()

	out, err := a.runASCCombinedOutput(ctx, ascPath, "auth", "status", "--output", "json")
	output := strings.TrimSpace(string(out))

	status := AuthStatus{RawOutput: output}

	if err != nil {
		status.Authenticated = false
		return status, nil
	}

	var jsonStatus struct {
		StorageBackend                 string `json:"storageBackend"`
		StorageLocation                string `json:"storageLocation"`
		Profile                        string `json:"profile"`
		EnvironmentCredentialsComplete bool   `json:"environmentCredentialsComplete"`
		Credentials                    []struct {
			Name      string `json:"name"`
			KeyID     string `json:"keyId"`
			IsDefault bool   `json:"isDefault"`
		} `json:"credentials"`
	}
	if json.Unmarshal([]byte(output), &jsonStatus) == nil {
		status.Storage = jsonStatus.StorageBackend
		status.Profile = jsonStatus.Profile
		status.Authenticated = len(jsonStatus.Credentials) > 0 || jsonStatus.EnvironmentCredentialsComplete
		for _, cred := range jsonStatus.Credentials {
			if cred.IsDefault {
				status.Profile = cred.Name
				break
			}
		}
		if status.Profile == "" && len(jsonStatus.Credentials) > 0 {
			status.Profile = jsonStatus.Credentials[0].Name
		}
		return status, nil
	}

	// Older asc builds may still emit non-JSON output for auth status; preserve
	// the prior success-path behavior in that fallback case.
	status.Authenticated = true
	return status, nil
}

// cacheAuthFromConfig reads auth credentials from config once and caches them
// so that subsequent asc commands don't depend on config.json staying intact.
func (a *App) cacheAuthFromConfig() {
	cfg, err := config.Load()
	if err != nil {
		return
	}
	// Prefer named key matching default_key_name
	for _, k := range cfg.Keys {
		if strings.TrimSpace(k.Name) == strings.TrimSpace(cfg.DefaultKeyName) && k.KeyID != "" {
			a.cachedKeyID = k.KeyID
			a.cachedIssuerID = k.IssuerID
			a.cachedPrivateKeyPath = k.PrivateKeyPath
			return
		}
	}
	// Fallback to top-level fields
	if cfg.KeyID != "" {
		a.cachedKeyID = cfg.KeyID
		a.cachedIssuerID = cfg.IssuerID
		a.cachedPrivateKeyPath = cfg.PrivateKeyPath
	}
}
