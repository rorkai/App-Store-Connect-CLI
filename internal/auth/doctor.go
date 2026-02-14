package auth

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/config"
)

type DoctorStatus string

const (
	DoctorOK   DoctorStatus = "ok"
	DoctorWarn DoctorStatus = "warn"
	DoctorFail DoctorStatus = "fail"
	DoctorInfo DoctorStatus = "info"
)

type DoctorCheck struct {
	Status         DoctorStatus `json:"status"`
	Message        string       `json:"message"`
	Recommendation string       `json:"recommendation,omitempty"`
	FixApplied     bool         `json:"fix_applied,omitempty"`
}

type DoctorSection struct {
	Title  string        `json:"title"`
	Checks []DoctorCheck `json:"checks"`
}

type DoctorSummary struct {
	OK       int `json:"ok"`
	Info     int `json:"info"`
	Warnings int `json:"warnings"`
	Errors   int `json:"errors"`
}

type DoctorReport struct {
	Sections        []DoctorSection       `json:"sections"`
	Summary         DoctorSummary         `json:"summary"`
	Recommendations []string              `json:"recommendations,omitempty"`
	Migration       *DoctorMigrationHints `json:"migration,omitempty"`
}

type DoctorOptions struct {
	Fix bool
}

func Doctor(options DoctorOptions) DoctorReport {
	return doctor(options, nil)
}

func DoctorWithMigrationResolver(options DoctorOptions, resolver MigrationSuggestionResolver) DoctorReport {
	return doctor(options, resolver)
}

func doctor(options DoctorOptions, resolver MigrationSuggestionResolver) DoctorReport {
	migrationSection, migrationHints := inspectMigrationHints(resolver)
	sections := []DoctorSection{
		inspectStorage(options),
		inspectProfiles(),
		inspectPrivateKeys(options),
		inspectEnvironment(),
		inspectTempKeys(options),
		migrationSection,
	}

	report := DoctorReport{Sections: sections, Migration: migrationHints}
	report.Summary, report.Recommendations = summarizeDoctorReport(sections)
	return report
}

func inspectStorage(options DoctorOptions) DoctorSection {
	checks := []DoctorCheck{}

	if shouldBypassKeychain() {
		checks = append(checks, DoctorCheck{
			Status:  DoctorInfo,
			Message: "Keychain is bypassed via ASC_BYPASS_KEYCHAIN (truthy values: 1/true/yes/on)",
		})
	} else if _, err := keyringOpener(); err != nil {
		status := DoctorFail
		message := fmt.Sprintf("System keychain unavailable: %v", err)
		if isKeyringUnavailable(err) {
			status = DoctorWarn
			message = "System keychain is unavailable"
		}
		checks = append(checks, DoctorCheck{
			Status:         status,
			Message:        message,
			Recommendation: "Consider using --bypass-keychain or setting ASC_BYPASS_KEYCHAIN to 1/true/yes/on",
		})
	} else {
		checks = append(checks, DoctorCheck{
			Status:  DoctorOK,
			Message: "System keychain is available",
		})
	}

	configPath, err := config.Path()
	if err != nil {
		checks = append(checks, DoctorCheck{
			Status:  DoctorFail,
			Message: fmt.Sprintf("Failed to resolve config path: %v", err),
		})
		return DoctorSection{Title: "Storage", Checks: checks}
	}

	info, err := os.Stat(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			checks = append(checks, DoctorCheck{
				Status:  DoctorInfo,
				Message: fmt.Sprintf("Config file not found at %s", configPath),
			})
		} else {
			checks = append(checks, DoctorCheck{
				Status:  DoctorFail,
				Message: fmt.Sprintf("Failed to stat config file: %v", err),
			})
		}
		return DoctorSection{Title: "Storage", Checks: checks}
	}

	checks = append(checks, DoctorCheck{
		Status:  DoctorOK,
		Message: fmt.Sprintf("Config file exists at %s", configPath),
	})

	if info.Mode().Perm()&0o077 != 0 {
		check := DoctorCheck{
			Status:         DoctorWarn,
			Message:        fmt.Sprintf("Config file permissions are too permissive (%#o)", info.Mode().Perm()),
			Recommendation: fmt.Sprintf("Run: chmod 600 %q", configPath),
		}
		if options.Fix {
			if err := os.Chmod(configPath, 0o600); err == nil {
				check.Status = DoctorOK
				check.Message = fmt.Sprintf("Config file permissions fixed to 0600 (%s)", configPath)
				check.FixApplied = true
				check.Recommendation = ""
			}
		}
		checks = append(checks, check)
	}

	cfg, err := config.LoadAt(configPath)
	if err == nil && hasCompleteCredentials(cfg) && !shouldBypassKeychain() {
		if _, err := keyringOpener(); err == nil {
			checks = append(checks, DoctorCheck{
				Status:         DoctorWarn,
				Message:        "Config file contains credentials while keychain is available",
				Recommendation: "Prefer storing credentials in keychain (re-run auth login without --bypass-keychain)",
			})
		}
	}

	return DoctorSection{Title: "Storage", Checks: checks}
}

func inspectProfiles() DoctorSection {
	checks := []DoctorCheck{}

	credentials, err := ListCredentials()
	if err != nil {
		if warning, ok := errors.AsType[*CredentialsWarning](err); ok {
			checks = append(checks, DoctorCheck{
				Status:  DoctorWarn,
				Message: warning.Error(),
			})
		} else {
			return DoctorSection{Title: "Profiles", Checks: []DoctorCheck{{
				Status:  DoctorFail,
				Message: fmt.Sprintf("Failed to list stored credentials: %v", err),
			}}}
		}
	}

	if len(credentials) == 0 {
		checks = append(checks, DoctorCheck{
			Status:  DoctorInfo,
			Message: "No stored credentials found",
		})
	} else {
		for _, cred := range credentials {
			source := cred.Source
			if cred.SourcePath != "" {
				source = fmt.Sprintf("%s: %s", cred.Source, cred.SourcePath)
			}
			message := fmt.Sprintf("%s - complete (%s)", cred.Name, source)
			if cred.IsDefault {
				message += " [default]"
			}
			checks = append(checks, DoctorCheck{
				Status:  DoctorOK,
				Message: message,
			})
		}
	}

	configPath, err := config.Path()
	if err != nil {
		return DoctorSection{Title: "Profiles", Checks: checks}
	}
	cfg, err := config.LoadAt(configPath)
	if err != nil {
		return DoctorSection{Title: "Profiles", Checks: checks}
	}

	seen := map[string]int{}
	for _, cred := range cfg.Keys {
		name := strings.TrimSpace(cred.Name)
		if name == "" {
			continue
		}
		seen[name]++
		if !isCompleteConfigCredential(cred) {
			checks = append(checks, DoctorCheck{
				Status:         DoctorWarn,
				Message:        fmt.Sprintf("%s - incomplete (missing key ID, issuer ID, or private key path)", name),
				Recommendation: fmt.Sprintf("Re-run auth login for %q", name),
			})
		}
	}

	var duplicates []string
	for name, count := range seen {
		if count > 1 {
			duplicates = append(duplicates, name)
		}
	}
	if len(duplicates) > 0 {
		sort.Strings(duplicates)
		checks = append(checks, DoctorCheck{
			Status:         DoctorWarn,
			Message:        fmt.Sprintf("Duplicate profiles in config: %s", strings.Join(duplicates, ", ")),
			Recommendation: fmt.Sprintf("Clean up duplicates in %s", configPath),
		})
	}

	return DoctorSection{Title: "Profiles", Checks: checks}
}

func inspectPrivateKeys(options DoctorOptions) DoctorSection {
	checks := []DoctorCheck{}
	credentials, err := ListCredentials()
	if err != nil {
		if warning, ok := errors.AsType[*CredentialsWarning](err); ok {
			checks = append(checks, DoctorCheck{
				Status:  DoctorWarn,
				Message: warning.Error(),
			})
		} else {
			return DoctorSection{Title: "Private Keys", Checks: []DoctorCheck{{
				Status:  DoctorFail,
				Message: fmt.Sprintf("Failed to list stored credentials: %v", err),
			}}}
		}
	}

	if len(credentials) == 0 {
		checks = append(checks, DoctorCheck{
			Status:  DoctorInfo,
			Message: "No private keys to validate",
		})
		return DoctorSection{Title: "Private Keys", Checks: checks}
	}

	seen := map[string]struct{}{}
	for _, cred := range credentials {
		path := strings.TrimSpace(cred.PrivateKeyPath)
		if path == "" {
			checks = append(checks, DoctorCheck{
				Status:  DoctorFail,
				Message: fmt.Sprintf("%s - missing private key path", cred.Name),
			})
			continue
		}
		if _, ok := seen[path]; ok {
			continue
		}
		seen[path] = struct{}{}
		checks = append(checks, inspectPrivateKeyPath(path, options))
	}

	return DoctorSection{Title: "Private Keys", Checks: checks}
}

func inspectPrivateKeyPath(path string, options DoctorOptions) DoctorCheck {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return DoctorCheck{
				Status:  DoctorFail,
				Message: fmt.Sprintf("%s - file not found", path),
			}
		}
		return DoctorCheck{
			Status:  DoctorFail,
			Message: fmt.Sprintf("%s - failed to stat file: %v", path, err),
		}
	}
	if info.IsDir() {
		return DoctorCheck{
			Status:  DoctorFail,
			Message: fmt.Sprintf("%s - path is a directory", path),
		}
	}

	check := DoctorCheck{
		Status:  DoctorOK,
		Message: fmt.Sprintf("%s - permissions %#o", path, info.Mode().Perm()),
	}

	if info.Mode().Perm()&0o077 != 0 {
		check.Status = DoctorWarn
		check.Message = fmt.Sprintf("%s - permissions %#o (expected 0600)", path, info.Mode().Perm())
		check.Recommendation = fmt.Sprintf("Run: chmod 600 %q", path)
		if options.Fix {
			if err := os.Chmod(path, 0o600); err == nil {
				check.Status = DoctorOK
				check.Message = fmt.Sprintf("%s - permissions fixed to 0600", path)
				check.FixApplied = true
				check.Recommendation = ""
			}
		}
	}

	if _, err := LoadPrivateKey(path); err != nil {
		return DoctorCheck{
			Status:  DoctorFail,
			Message: fmt.Sprintf("%s - invalid private key: %v", path, err),
		}
	}

	if check.Status == DoctorOK && check.Message != "" {
		check.Message = fmt.Sprintf("%s - valid ECDSA key, %s", path, strings.TrimPrefix(check.Message, path+" - "))
	}

	return check
}

func inspectEnvironment() DoctorSection {
	checks := []DoctorCheck{}

	envVars := []string{
		"ASC_KEY_ID",
		"ASC_ISSUER_ID",
		"ASC_PRIVATE_KEY_PATH",
		"ASC_PRIVATE_KEY",
		"ASC_PRIVATE_KEY_B64",
		"ASC_PROFILE",
		"ASC_BYPASS_KEYCHAIN",
		"ASC_STRICT_AUTH",
	}
	for _, name := range envVars {
		if value := strings.TrimSpace(os.Getenv(name)); value != "" {
			message := fmt.Sprintf("%s is set", name)
			if name == "ASC_PROFILE" {
				message = fmt.Sprintf("%s is set (%s)", name, value)
			}
			checks = append(checks, DoctorCheck{
				Status:  DoctorInfo,
				Message: message,
			})
		}
	}

	keyID := strings.TrimSpace(os.Getenv("ASC_KEY_ID"))
	issuerID := strings.TrimSpace(os.Getenv("ASC_ISSUER_ID"))
	hasKeyPath := strings.TrimSpace(os.Getenv("ASC_PRIVATE_KEY_PATH")) != "" ||
		strings.TrimSpace(os.Getenv("ASC_PRIVATE_KEY")) != "" ||
		strings.TrimSpace(os.Getenv("ASC_PRIVATE_KEY_B64")) != ""
	envProvided := keyID != "" || issuerID != "" || hasKeyPath
	envComplete := keyID != "" && issuerID != "" && hasKeyPath
	if envProvided && !envComplete {
		checks = append(checks, DoctorCheck{
			Status:         DoctorWarn,
			Message:        "Environment credentials are incomplete (set ASC_KEY_ID, ASC_ISSUER_ID, and a private key)",
			Recommendation: "Set missing ASC_* variables or clear partial values",
		})
	}

	if envProvided {
		defaultCreds, err := GetDefaultCredentials()
		if err == nil && defaultCreds != nil {
			if keyID != "" && defaultCreds.KeyID != "" && keyID != defaultCreds.KeyID {
				checks = append(checks, DoctorCheck{
					Status:         DoctorWarn,
					Message:        "ASC_KEY_ID differs from default stored credentials",
					Recommendation: "Use --profile or clear conflicting env vars",
				})
			}
			if issuerID != "" && defaultCreds.IssuerID != "" && issuerID != defaultCreds.IssuerID {
				checks = append(checks, DoctorCheck{
					Status:         DoctorWarn,
					Message:        "ASC_ISSUER_ID differs from default stored credentials",
					Recommendation: "Use --profile or clear conflicting env vars",
				})
			}
		}
	}

	return DoctorSection{Title: "Environment", Checks: checks}
}

func inspectTempKeys(options DoctorOptions) DoctorSection {
	tempDir := os.TempDir()
	entries, err := os.ReadDir(tempDir)
	if err != nil {
		return DoctorSection{Title: "Temp Files", Checks: []DoctorCheck{{
			Status:  DoctorWarn,
			Message: fmt.Sprintf("Failed to read temp directory: %v", err),
		}}}
	}

	var matches []string
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, "asc-key-") && strings.HasSuffix(name, ".p8") {
			matches = append(matches, filepath.Join(tempDir, name))
		}
	}

	if len(matches) == 0 {
		return DoctorSection{Title: "Temp Files", Checks: []DoctorCheck{{
			Status:  DoctorOK,
			Message: "No orphaned temp key files found",
		}}}
	}

	check := DoctorCheck{
		Status:         DoctorWarn,
		Message:        fmt.Sprintf("Found %d orphaned temp key file(s)", len(matches)),
		Recommendation: "Remove orphaned temp key files from your temp directory",
	}
	if options.Fix {
		for _, path := range matches {
			_ = os.Remove(path)
		}
		check.Status = DoctorOK
		check.Message = fmt.Sprintf("Removed %d orphaned temp key file(s)", len(matches))
		check.FixApplied = true
		check.Recommendation = ""
	}

	return DoctorSection{Title: "Temp Files", Checks: []DoctorCheck{check}}
}

func summarizeDoctorReport(sections []DoctorSection) (DoctorSummary, []string) {
	var summary DoctorSummary
	recommendations := map[string]struct{}{}
	for _, section := range sections {
		for _, check := range section.Checks {
			switch check.Status {
			case DoctorOK:
				summary.OK++
			case DoctorInfo:
				summary.Info++
			case DoctorWarn:
				summary.Warnings++
			case DoctorFail:
				summary.Errors++
			}
			if check.Recommendation != "" && check.Status != DoctorOK {
				recommendations[check.Recommendation] = struct{}{}
			}
		}
	}
	unique := make([]string, 0, len(recommendations))
	for rec := range recommendations {
		unique = append(unique, rec)
	}
	sort.Strings(unique)
	return summary, unique
}
