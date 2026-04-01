package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/apps/studio/internal/studio/ascbin"
	"github.com/rudrankriyam/App-Store-Connect-CLI/apps/studio/internal/studio/settings"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/config"
)

func (a *App) newASCCommand(ctx context.Context, ascPath string, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, ascPath, args...)
	// Inject auth env vars so we're immune to config.json wipes.
	// Read credentials once from config and pass them via env every time.
	baseEnv := os.Environ()
	env := make([]string, 0, len(baseEnv)+4)
	env = append(env, baseEnv...)
	if a.cachedKeyID != "" {
		// When we have cached credentials, bypass keychain and inject them directly
		// so we're immune to config.json wipes during the session.
		env = setEnvVar(env, "ASC_BYPASS_KEYCHAIN", "1")
		env = setEnvVar(env, "ASC_KEY_ID", a.cachedKeyID)
		env = setEnvVar(env, "ASC_ISSUER_ID", a.cachedIssuerID)
		env = setEnvVar(env, "ASC_PRIVATE_KEY_PATH", a.cachedPrivateKeyPath)
	}
	cmd.Env = env
	return cmd
}

func (a *App) runASCCombinedOutput(ctx context.Context, ascPath string, args ...string) ([]byte, error) {
	cmd := a.newASCCommand(ctx, ascPath, args...)
	cleanup := isolateASCConfig(cmd)
	defer cleanup()
	return cmd.CombinedOutput()
}

func isolateASCConfig(cmd *exec.Cmd) func() {
	activePath, err := config.Path()
	if err != nil {
		return func() {}
	}

	shadowDir, err := os.MkdirTemp("", "asc-studio-config-*")
	if err != nil {
		return func() {}
	}
	shadowPath := filepath.Join(shadowDir, "config.json")

	if data, err := os.ReadFile(activePath); err == nil {
		if err := os.WriteFile(shadowPath, data, 0o600); err != nil {
			_ = os.RemoveAll(shadowDir)
			return func() {}
		}
	} else if !os.IsNotExist(err) {
		_ = os.RemoveAll(shadowDir)
		return func() {}
	}

	cmd.Env = setEnvVar(cmd.Env, "ASC_CONFIG_PATH", shadowPath)
	return func() {
		_ = os.RemoveAll(shadowDir)
	}
}

func setEnvVar(env []string, key, value string) []string {
	prefix := key + "="
	filtered := make([]string, 0, len(env)+1)
	for _, entry := range env {
		if strings.HasPrefix(entry, prefix) {
			continue
		}
		filtered = append(filtered, entry)
	}
	return append(filtered, prefix+value)
}

func (a *App) resolveASCPath() (string, error) {
	resolution, err := a.resolveASC()
	if err != nil {
		return "", err
	}
	return resolution.Path, nil
}

func (a *App) resolveASC() (ascbin.Resolution, error) {
	cfg, err := a.settings.Load()
	if err != nil {
		return ascbin.Resolution{}, err
	}

	return ascbin.Resolve(a.ascResolveOptions(cfg))
}

func (a *App) ascResolveOptions(cfg settings.StudioSettings) ascbin.ResolveOptions {
	return ascbin.ResolveOptions{
		BundledPath:    a.bundledASCPath(),
		SystemOverride: cfg.SystemASCPath,
		PreferBundled:  cfg.PreferBundledASC,
		LookPath:       execLookPath,
	}
}

func (a *App) bundledASCPath() string {
	candidates := bundledASCCandidates()
	for _, candidate := range candidates {
		if fileExists(candidate) {
			return candidate
		}
	}
	if len(candidates) > 0 {
		return candidates[0]
	}
	return ""
}

func bundledASCCandidates() []string {
	var candidates []string
	if executable, err := osExecutableFunc(); err == nil && strings.TrimSpace(executable) != "" {
		execDir := filepath.Dir(executable)
		candidates = append(candidates,
			filepath.Clean(filepath.Join(execDir, "..", "Resources", "bin", "asc")),
			filepath.Clean(filepath.Join(execDir, "bin", "asc")),
		)
	}

	if workingDir, err := getwdFunc(); err == nil && strings.TrimSpace(workingDir) != "" {
		candidates = append(candidates,
			filepath.Join(workingDir, "bin", "asc"),
			filepath.Join(workingDir, "apps", "studio", "bin", "asc"),
		)
	}

	seen := make(map[string]struct{}, len(candidates))
	deduped := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		if _, ok := seen[candidate]; ok {
			continue
		}
		seen[candidate] = struct{}{}
		deduped = append(deduped, candidate)
	}
	return deduped
}

func base64Decode(s string) ([]byte, error) {
	// ASC API uses URL-safe base64 without padding
	return base64.RawURLEncoding.DecodeString(s)
}

func parseAppsListOutput(out []byte) ([]rawASCApp, error) {
	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(out, &envelope); err == nil && envelope.Data != nil {
		var rawApps []rawASCApp
		if err := json.Unmarshal(envelope.Data, &rawApps); err != nil {
			return nil, err
		}
		return rawApps, nil
	}

	var rawApps []rawASCApp
	if err := json.Unmarshal(out, &rawApps); err != nil {
		return nil, err
	}
	return rawApps, nil
}

func parseResourceIDOutput(out []byte) (string, error) {
	var envelope struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(out, &envelope); err != nil {
		return "", err
	}
	return strings.TrimSpace(envelope.Data.ID), nil
}

func parseAvailabilityViewOutput(out []byte) (string, bool, error) {
	var envelope struct {
		Data struct {
			ID         string `json:"id"`
			Attributes struct {
				AvailableInNewTerritories bool `json:"availableInNewTerritories"`
			} `json:"attributes"`
		} `json:"data"`
	}
	if err := json.Unmarshal(out, &envelope); err != nil {
		return "", false, err
	}
	return strings.TrimSpace(envelope.Data.ID), envelope.Data.Attributes.AvailableInNewTerritories, nil
}

func parseFirstAppPriceReference(out []byte) (appPriceReference, bool, error) {
	type rawPrice struct {
		ID string `json:"id"`
	}
	var env struct {
		Data []rawPrice `json:"data"`
	}
	if err := json.Unmarshal(out, &env); err != nil {
		return appPriceReference{}, false, err
	}
	if len(env.Data) == 0 {
		return appPriceReference{}, false, nil
	}

	decoded, err := base64Decode(env.Data[0].ID)
	if err != nil {
		return appPriceReference{}, false, err
	}

	var ref struct {
		Territory  string `json:"t"`
		PricePoint string `json:"p"`
	}
	if err := json.Unmarshal(decoded, &ref); err != nil {
		return appPriceReference{}, false, err
	}
	if strings.TrimSpace(ref.Territory) == "" || strings.TrimSpace(ref.PricePoint) == "" {
		return appPriceReference{}, false, errors.New("missing territory or price point")
	}

	return appPriceReference{
		Territory:  strings.TrimSpace(ref.Territory),
		PricePoint: strings.TrimSpace(ref.PricePoint),
	}, true, nil
}

func parseAppPricePointLookup(out []byte, territoryID, wantedPricePoint string) (appPricePointLookupResult, bool, error) {
	type rawPricePoint struct {
		ID         string `json:"id"`
		Attributes struct {
			CustomerPrice string `json:"customerPrice"`
			Proceeds      string `json:"proceeds"`
		} `json:"attributes"`
	}
	type rawIncluded struct {
		Type       string `json:"type"`
		ID         string `json:"id"`
		Attributes struct {
			Currency string `json:"currency"`
		} `json:"attributes"`
	}

	var env struct {
		Data     []rawPricePoint `json:"data"`
		Included []rawIncluded   `json:"included"`
	}
	if err := json.Unmarshal(out, &env); err != nil {
		return appPricePointLookupResult{}, false, err
	}

	currencyByTerritory := make(map[string]string, len(env.Included))
	for _, included := range env.Included {
		if included.Type != "territories" {
			continue
		}
		currencyByTerritory[included.ID] = strings.TrimSpace(included.Attributes.Currency)
	}

	for _, point := range env.Data {
		decoded, err := base64Decode(point.ID)
		if err != nil {
			continue
		}
		var ref struct {
			PricePoint string `json:"p"`
		}
		if err := json.Unmarshal(decoded, &ref); err != nil {
			continue
		}
		if strings.TrimSpace(ref.PricePoint) != wantedPricePoint {
			continue
		}
		return appPricePointLookupResult{
			Price:    point.Attributes.CustomerPrice,
			Proceeds: point.Attributes.Proceeds,
			Currency: currencyByTerritory[territoryID],
		}, true, nil
	}

	return appPricePointLookupResult{}, false, nil
}

func fileExists(path string) bool {
	if strings.TrimSpace(path) == "" {
		return false
	}
	info, err := os.Stat(filepath.Clean(path))
	return err == nil && !info.IsDir()
}

func execLookPath(file string) (string, error) {
	return execLookPathFunc(file)
}
