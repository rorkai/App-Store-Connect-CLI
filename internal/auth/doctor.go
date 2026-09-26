package auth

import (
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/config"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/rootfs"
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
	Fix        bool
	Profile    string
	StrictAuth bool
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
		inspectEnvironment(options),
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

	info, err := os.Lstat(configPath)
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
	if info.Mode()&os.ModeSymlink != 0 {
		checks = append(checks, DoctorCheck{
			Status:         DoctorFail,
			Message:        fmt.Sprintf("Config path is a symbolic link at %s", configPath),
			Recommendation: "Configure a regular config file instead of a symbolic link",
		})
		return DoctorSection{Title: "Storage", Checks: checks}
	}
	if err := rootfs.CheckContainedPath(configPath); err != nil {
		checks = append(checks, DoctorCheck{
			Status:         DoctorFail,
			Message:        fmt.Sprintf("Config path cannot be inspected safely at %s: %v", configPath, err),
			Recommendation: "Configure a regular config file without symbolic links",
		})
		return DoctorSection{Title: "Storage", Checks: checks}
	}
	if !info.Mode().IsRegular() {
		checks = append(checks, DoctorCheck{
			Status:         DoctorFail,
			Message:        fmt.Sprintf("Config path is not a regular file at %s", configPath),
			Recommendation: "Configure a regular config file",
		})
		return DoctorSection{Title: "Storage", Checks: checks}
	}

	checks = append(checks, DoctorCheck{
		Status:  DoctorOK,
		Message: fmt.Sprintf("Config file exists at %s", configPath),
	})

	if filePermissionsTooPermissive(info.Mode()) {
		check := DoctorCheck{
			Status:  DoctorWarn,
			Message: fmt.Sprintf("Config file permissions are too permissive (%#o)", info.Mode().Perm()),
		}
		if command, safe := FilePermissionRemediationCommand(configPath); safe {
			check.Recommendation = fmt.Sprintf("Run: %s", command)
		} else {
			check.Recommendation = "Run: asc auth doctor --fix --confirm"
		}
		if options.Fix {
			if err := rootfs.ChmodFileIfSame(configPath, info, 0o600); err == nil {
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

	credentials, err := ListCredentialSummaries()
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
				Message:        fmt.Sprintf("%s - incomplete (missing key ID, issuer ID for team keys, or private key path)", name),
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
		if pemValue := strings.TrimSpace(cred.PrivateKeyPEM); pemValue != "" {
			if _, err := LoadPrivateKeyFromPEM([]byte(pemValue)); err != nil {
				checks = append(checks, DoctorCheck{
					Status:  DoctorFail,
					Message: fmt.Sprintf("%s - invalid keychain private key: %v", cred.Name, err),
				})
				continue
			}
			checks = append(checks, DoctorCheck{
				Status:  DoctorOK,
				Message: fmt.Sprintf("%s - valid private key stored in keychain", cred.Name),
			})
			continue
		}

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
	info, err := os.Lstat(path)
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
	if info.Mode()&os.ModeSymlink != 0 {
		return DoctorCheck{
			Status:         DoctorFail,
			Message:        fmt.Sprintf("%s - path is a symbolic link", path),
			Recommendation: "Configure a regular private key file instead of a symbolic link",
		}
	}
	if err := rootfs.CheckContainedPath(path); err != nil {
		return DoctorCheck{
			Status:         DoctorFail,
			Message:        fmt.Sprintf("%s - path cannot be inspected safely: %v", path, err),
			Recommendation: "Configure a regular private key file without symbolic links",
		}
	}
	if info.IsDir() {
		return DoctorCheck{
			Status:  DoctorFail,
			Message: fmt.Sprintf("%s - path is a directory", path),
		}
	}
	if !info.Mode().IsRegular() {
		return DoctorCheck{
			Status:  DoctorFail,
			Message: fmt.Sprintf("%s - not a regular file", path),
		}
	}

	check := DoctorCheck{
		Status:  DoctorOK,
		Message: fmt.Sprintf("%s - permissions %#o", path, info.Mode().Perm()),
	}

	if filePermissionsTooPermissive(info.Mode()) {
		check.Status = DoctorWarn
		check.Message = fmt.Sprintf("%s - permissions %#o (expected 0600)", path, info.Mode().Perm())
		if command, safe := FilePermissionRemediationCommand(path); safe {
			check.Recommendation = fmt.Sprintf("Run: %s", command)
		} else {
			check.Recommendation = "Run: asc auth doctor --fix --confirm"
		}
		if options.Fix {
			if changed, err := FixPrivateKeyFilePermissions(path); err == nil && changed {
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

func inspectEnvironment(options DoctorOptions) DoctorSection {
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
		"ASC_KEY_TYPE",
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
	if selectedProfileCheck := inspectSelectedProfile(options); selectedProfileCheck != nil {
		checks = append(checks, *selectedProfileCheck)
	}
	if defaultCredentialCheck := inspectDefaultCredentialFallback(options); defaultCredentialCheck != nil {
		checks = append(checks, *defaultCredentialCheck)
	}

	keyID := strings.TrimSpace(os.Getenv("ASC_KEY_ID"))
	issuerID := strings.TrimSpace(os.Getenv("ASC_ISSUER_ID"))
	keyTypeRaw := strings.TrimSpace(os.Getenv("ASC_KEY_TYPE"))
	keyType := config.NormalizeCredentialKeyType(keyTypeRaw)
	keyTypeValid := keyTypeRaw == "" || config.IsValidCredentialKeyType(keyType)
	individualKey := keyTypeValid && config.IsIndividualCredentialKeyType(keyType)
	hasKeyPath := hasEnvironmentPrivateKey()
	envProvided := keyID != "" || issuerID != "" || hasKeyPath || keyTypeRaw != ""
	envComplete := keyID != "" && hasKeyPath &&
		keyTypeValid &&
		(issuerID != "" || individualKey)
	if keyTypeRaw != "" && !keyTypeValid {
		checks = append(checks, DoctorCheck{
			Status:         DoctorWarn,
			Message:        "ASC_KEY_TYPE is invalid (expected team or individual)",
			Recommendation: "Set ASC_KEY_TYPE to team or individual, or clear it",
		})
	}
	if envProvided && !envComplete {
		checks = append(checks, DoctorCheck{
			Status:         DoctorWarn,
			Message:        "Environment credentials are incomplete (set ASC_KEY_ID, ASC_ISSUER_ID unless ASC_KEY_TYPE=individual, and a private key)",
			Recommendation: "Set missing ASC_* variables or clear partial values",
		})
	}
	if ignoredReason := ignoredEnvironmentPrivateKeyReason(options); hasKeyPath && ignoredReason != "" {
		checks = append(checks, DoctorCheck{
			Status:  DoctorInfo,
			Message: fmt.Sprintf("Environment private key is set but ignored because %s; key material was not validated", ignoredReason),
		})
	} else if privateKeyCheck := inspectEnvironmentPrivateKey(options); privateKeyCheck != nil {
		checks = append(checks, *privateKeyCheck)
	}

	shapeLabels := CredentialShapeLabels{KeyID: "ASC_KEY_ID", IssuerID: "ASC_ISSUER_ID"}
	if !individualKey {
		for _, finding := range InspectCredentialShapes(shapeLabels, keyID, issuerID) {
			checks = append(checks, DoctorCheck{
				Status:         DoctorWarn,
				Message:        finding.Message,
				Recommendation: finding.Recommendation,
			})
		}
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
			if !individualKey && issuerID != "" && defaultCreds.IssuerID != "" && issuerID != defaultCreds.IssuerID {
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

func ignoredEnvironmentPrivateKeyReason(options DoctorOptions) string {
	profile := selectedDoctorProfile(options)
	if profile != "" {
		credentials, err := GetCredentials(profile)
		if err != nil || credentials == nil {
			return fmt.Sprintf("profile %q is selected", profile)
		}
		if strings.TrimSpace(credentials.PrivateKeyPath) != "" || strings.TrimSpace(credentials.PrivateKeyPEM) != "" {
			return fmt.Sprintf("profile %q provides stored private key material", profile)
		}
		return ""
	}
	if !shouldBypassKeychain() && completeEnvironmentCredentialsPreemptStored() {
		return ""
	}

	credentials, err := GetDefaultCredentials()
	if err != nil || credentials == nil {
		return ""
	}
	hasKeyID := strings.TrimSpace(credentials.KeyID) != ""
	hasIssuer := strings.TrimSpace(credentials.IssuerID) != "" || config.IsIndividualCredentialKeyType(credentials.KeyType)
	hasPrivateKey := strings.TrimSpace(credentials.PrivateKeyPath) != "" || strings.TrimSpace(credentials.PrivateKeyPEM) != ""
	if !hasPrivateKey {
		return ""
	}
	complete := hasKeyID && hasIssuer
	if shouldBypassKeychain() {
		return "complete stored config credentials are selected in keychain bypass mode"
	}
	if !complete {
		return "default stored private key is selected"
	}
	return "complete default stored credentials are selected"
}

func selectedDoctorProfile(options DoctorOptions) string {
	if profile := strings.TrimSpace(options.Profile); profile != "" {
		return profile
	}
	return strings.TrimSpace(os.Getenv("ASC_PROFILE"))
}

func inspectSelectedProfile(options DoctorOptions) *DoctorCheck {
	profile := selectedDoctorProfile(options)
	if profile == "" {
		return nil
	}
	credentials, err := GetCredentials(profile)
	if err != nil {
		return &DoctorCheck{
			Status:         DoctorFail,
			Message:        fmt.Sprintf("Selected profile %q could not be resolved: %v", profile, err),
			Recommendation: "Choose an existing complete profile or update the selected profile credentials",
		}
	}
	if credentials == nil {
		return &DoctorCheck{
			Status:         DoctorFail,
			Message:        fmt.Sprintf("Selected profile %q could not be resolved", profile),
			Recommendation: "Choose an existing complete profile or update the selected profile credentials",
		}
	}
	shape := effectiveCredentialShape(credentials)
	if shape.invalidEnvironmentKeyType {
		return &DoctorCheck{
			Status:         DoctorFail,
			Message:        fmt.Sprintf("Selected profile %q cannot use environment fallback: ASC_KEY_TYPE must be team or individual", profile),
			Recommendation: "Set ASC_KEY_TYPE to team or individual, or update the selected profile so fallback is unnecessary",
		}
	}
	if len(shape.missing) > 0 {
		return &DoctorCheck{
			Status:         DoctorFail,
			Message:        fmt.Sprintf("Selected profile %q is incomplete after environment fallback (missing %s)", profile, strings.Join(shape.missing, ", ")),
			Recommendation: "Update the selected profile or set the missing ASC_* environment fields",
		}
	}
	if options.StrictAuth && shape.mixedSources {
		return &DoctorCheck{
			Status:         DoctorFail,
			Message:        fmt.Sprintf("Selected profile %q requires mixed stored and environment credential sources while strict authentication is enabled", profile),
			Recommendation: "Store a complete credential profile or clear ASC_STRICT_AUTH",
		}
	}
	return nil
}

func inspectDefaultCredentialFallback(options DoctorOptions) *DoctorCheck {
	if selectedDoctorProfile(options) != "" {
		return nil
	}
	if !shouldBypassKeychain() && completeEnvironmentCredentialsPreemptStored() {
		return nil
	}
	credentials, err := GetDefaultCredentials()
	if err != nil || credentials == nil {
		return nil
	}
	shape := effectiveCredentialShape(credentials)
	if shape.invalidEnvironmentKeyType {
		return &DoctorCheck{
			Status:         DoctorFail,
			Message:        "Default stored credentials cannot use environment fallback: ASC_KEY_TYPE must be team or individual",
			Recommendation: "Set ASC_KEY_TYPE to team or individual, or complete the default stored credentials",
		}
	}
	if len(shape.missing) > 0 {
		return &DoctorCheck{
			Status:         DoctorFail,
			Message:        fmt.Sprintf("Default stored credentials are incomplete after environment fallback (missing %s)", strings.Join(shape.missing, ", ")),
			Recommendation: "Complete the default stored credentials or set the missing ASC_* environment fields",
		}
	}
	if !options.StrictAuth || !shape.mixedSources {
		return nil
	}
	return &DoctorCheck{
		Status:         DoctorFail,
		Message:        "Default stored credentials require mixed stored and environment credential sources while strict authentication is enabled",
		Recommendation: "Store complete default credentials or clear ASC_STRICT_AUTH",
	}
}

type credentialShape struct {
	missing                   []string
	invalidEnvironmentKeyType bool
	mixedSources              bool
}

func effectiveCredentialShape(credentials *config.Config) credentialShape {
	storedKeyID := strings.TrimSpace(credentials.KeyID) != ""
	storedIssuer := strings.TrimSpace(credentials.IssuerID) != ""
	storedPrivateKey := strings.TrimSpace(credentials.PrivateKeyPath) != "" || strings.TrimSpace(credentials.PrivateKeyPEM) != ""
	storedKeyType := config.NormalizeCredentialKeyType(credentials.KeyType)
	storedIndividual := config.IsIndividualCredentialKeyType(credentials.KeyType)
	needsFallback := !storedKeyID || (!storedIssuer && !storedIndividual) || !storedPrivateKey

	shape := credentialShape{}
	effectiveIndividual := storedIndividual
	if needsFallback {
		environmentKeyType := strings.TrimSpace(os.Getenv("ASC_KEY_TYPE"))
		if environmentKeyType != "" && !config.IsValidCredentialKeyType(environmentKeyType) {
			shape.invalidEnvironmentKeyType = true
			return shape
		}
		if storedKeyType == config.CredentialKeyTypeTeam && config.IsIndividualCredentialKeyType(environmentKeyType) {
			effectiveIndividual = true
		}
	}

	sources := map[string]struct{}{}
	if storedKeyID {
		sources["stored"] = struct{}{}
	} else if strings.TrimSpace(os.Getenv("ASC_KEY_ID")) != "" {
		sources["environment"] = struct{}{}
	} else {
		shape.missing = append(shape.missing, "key ID")
	}
	if !effectiveIndividual {
		if storedIssuer {
			sources["stored"] = struct{}{}
		} else if strings.TrimSpace(os.Getenv("ASC_ISSUER_ID")) != "" {
			sources["environment"] = struct{}{}
		} else {
			shape.missing = append(shape.missing, "issuer ID")
		}
	}
	if storedPrivateKey {
		sources["stored"] = struct{}{}
	} else if hasEnvironmentPrivateKey() {
		sources["environment"] = struct{}{}
	} else {
		shape.missing = append(shape.missing, "private key")
	}
	shape.mixedSources = len(sources) > 1
	return shape
}

func hasEnvironmentPrivateKey() bool {
	return strings.TrimSpace(os.Getenv("ASC_PRIVATE_KEY_PATH")) != "" ||
		strings.TrimSpace(os.Getenv("ASC_PRIVATE_KEY")) != "" ||
		strings.TrimSpace(os.Getenv("ASC_PRIVATE_KEY_B64")) != ""
}

func completeEnvironmentCredentialsPreemptStored() bool {
	keyID := strings.TrimSpace(os.Getenv("ASC_KEY_ID"))
	issuerID := strings.TrimSpace(os.Getenv("ASC_ISSUER_ID"))
	keyType := config.NormalizeCredentialKeyType(os.Getenv("ASC_KEY_TYPE"))
	if keyID == "" || !config.IsValidCredentialKeyType(keyType) ||
		(issuerID == "" && !config.IsIndividualCredentialKeyType(keyType)) {
		return false
	}

	switch {
	case strings.TrimSpace(os.Getenv("ASC_PRIVATE_KEY_PATH")) != "":
		return true
	case strings.TrimSpace(os.Getenv("ASC_PRIVATE_KEY_B64")) != "":
		compact := strings.Join(strings.Fields(os.Getenv("ASC_PRIVATE_KEY_B64")), "")
		decoded, err := base64.StdEncoding.DecodeString(compact)
		return err == nil && len(decoded) > 0 && environmentPrivateKeyCanMaterialize(len(decoded))
	case strings.TrimSpace(os.Getenv("ASC_PRIVATE_KEY")) != "":
		normalized := strings.ReplaceAll(strings.TrimSpace(os.Getenv("ASC_PRIVATE_KEY")), `\n`, "\n")
		return environmentPrivateKeyCanMaterialize(len(normalized))
	default:
		return false
	}
}

func environmentPrivateKeyCanMaterialize(size int) bool {
	file, err := os.CreateTemp("", "asc-doctor-key-check-*.p8")
	if err != nil {
		return false
	}
	path := file.Name()
	defer func() {
		_ = file.Close()
		_ = os.Remove(path)
	}()
	if err := file.Chmod(0o600); err != nil {
		return false
	}
	if _, err := file.Write(make([]byte, size)); err != nil {
		return false
	}
	return file.Close() == nil
}

func inspectEnvironmentPrivateKey(options DoctorOptions) *DoctorCheck {
	if path := strings.TrimSpace(os.Getenv("ASC_PRIVATE_KEY_PATH")); path != "" {
		check := inspectPrivateKeyPath(path, options)
		redactEnvironmentPrivateKeyPath(&check, path)
		return &check
	}

	if value := strings.TrimSpace(os.Getenv("ASC_PRIVATE_KEY_B64")); value != "" {
		compact := strings.Join(strings.Fields(value), "")
		decoded, err := base64.StdEncoding.DecodeString(compact)
		if err != nil || len(decoded) == 0 {
			return &DoctorCheck{
				Status:         DoctorFail,
				Message:        "ASC_PRIVATE_KEY_B64 is not valid base64",
				Recommendation: "Set ASC_PRIVATE_KEY_B64 to a base64-encoded ECDSA P-256 private key",
			}
		}
		if _, err := LoadPrivateKeyFromPEM(decoded); err != nil {
			return &DoctorCheck{
				Status:         DoctorFail,
				Message:        "ASC_PRIVATE_KEY_B64 does not contain a valid private key",
				Recommendation: "Set ASC_PRIVATE_KEY_B64 to a base64-encoded ECDSA P-256 private key",
			}
		}
		if !environmentPrivateKeyCanMaterialize(len(decoded)) {
			return &DoctorCheck{
				Status:         DoctorFail,
				Message:        "ASC_PRIVATE_KEY_B64 cannot be materialized as a temporary private key",
				Recommendation: "Set TMPDIR to a writable directory or use ASC_PRIVATE_KEY_PATH",
			}
		}
		return &DoctorCheck{
			Status:  DoctorOK,
			Message: "ASC_PRIVATE_KEY_B64 contains a valid ECDSA private key",
		}
	}

	if value := strings.TrimSpace(os.Getenv("ASC_PRIVATE_KEY")); value != "" {
		value = strings.ReplaceAll(value, `\n`, "\n")
		if _, err := LoadPrivateKeyFromPEM([]byte(value)); err != nil {
			return &DoctorCheck{
				Status:         DoctorFail,
				Message:        "ASC_PRIVATE_KEY is not a valid private key",
				Recommendation: "Set ASC_PRIVATE_KEY to an ECDSA P-256 private key in PEM format",
			}
		}
		if !environmentPrivateKeyCanMaterialize(len(value)) {
			return &DoctorCheck{
				Status:         DoctorFail,
				Message:        "ASC_PRIVATE_KEY cannot be materialized as a temporary private key",
				Recommendation: "Set TMPDIR to a writable directory or use ASC_PRIVATE_KEY_PATH",
			}
		}
		return &DoctorCheck{
			Status:  DoctorOK,
			Message: "ASC_PRIVATE_KEY contains a valid ECDSA private key",
		}
	}

	return nil
}

func redactEnvironmentPrivateKeyPath(check *DoctorCheck, path string) {
	if check == nil || path == "" {
		return
	}
	check.Message = strings.ReplaceAll(check.Message, path, "ASC_PRIVATE_KEY_PATH")
	check.Recommendation = strings.ReplaceAll(check.Recommendation, strconv.Quote(path), `"$ASC_PRIVATE_KEY_PATH"`)
	check.Recommendation = strings.ReplaceAll(check.Recommendation, path, "$ASC_PRIVATE_KEY_PATH")
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
