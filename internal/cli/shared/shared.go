package shared

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"net/url"
	"os"
	"slices"
	"strings"
	"sync"
	"text/tabwriter"
	"time"

	"github.com/peterbourgon/ff/v3/ffcli"
	"golang.org/x/term"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/auth"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/config"
)

// ANSI escape codes for bold text
var (
	bold  = "\033[1m"
	reset = "\033[22m"
)

const (
	privateKeyEnvVar       = "ASC_PRIVATE_KEY"
	privateKeyBase64EnvVar = "ASC_PRIVATE_KEY_B64"
	profileEnvVar          = "ASC_PROFILE"
	strictAuthEnvVar       = "ASC_STRICT_AUTH"
	defaultOutputEnvVar    = "ASC_DEFAULT_OUTPUT"
)

const (
	PrivateKeyEnvVar       = privateKeyEnvVar
	PrivateKeyBase64EnvVar = privateKeyBase64EnvVar
)

var ErrMissingAuth = errors.New("missing authentication")

type missingAuthError struct {
	msg string
}

func (e missingAuthError) Error() string {
	return e.msg
}

func (e missingAuthError) Is(target error) bool {
	return target == ErrMissingAuth
}

var (
	privateKeyTempMu    sync.Mutex
	privateKeyTempPath  string
	privateKeyTempKey   string
	privateKeyTempPaths []string
	strictAuthWarnMu    sync.Mutex
	strictAuthWarned    = map[string]struct{}{}
	selectedProfile     string
	strictAuth          bool
	retryLog            OptionalBool
	debug               OptionalBool
	apiDebug            OptionalBool

	getCredentialsWithSourceFn = auth.GetCredentialsWithSource
	listCredentialSummariesFn  = auth.ListCredentialSummaries
)

var (
	isTerminal = term.IsTerminal
	noProgress bool
)

// BindRootFlags registers root-level flags that affect shared CLI behavior.
func BindRootFlags(fs *flag.FlagSet) {
	// Keep root debug/retry flags ergonomic while command-level OptionalBool
	// flags continue to require explicit values.
	retryLog.EnableBoolFlag()
	debug.EnableBoolFlag()
	apiDebug.EnableBoolFlag()

	fs.StringVar(&selectedProfile, "profile", "", "Use named authentication profile")
	fs.BoolVar(&strictAuth, "strict-auth", false, "Fail when credentials are resolved from multiple sources")
	fs.Var(&retryLog, "retry-log", "Enable retry logging to stderr (overrides ASC_RETRY_LOG/config when set)")
	fs.Var(&debug, "debug", "Enable debug logging to stderr")
	fs.Var(&apiDebug, "api-debug", "Enable HTTP debug logging to stderr (redacts sensitive values)")
	BindCIFlags(fs)
}

// SelectedProfile returns the current profile override.
func SelectedProfile() string {
	return selectedProfile
}

// ProgressEnabled reports whether it's safe/appropriate to emit progress messages.
// Progress must be stderr-only and must not appear when stderr is non-interactive.
func ProgressEnabled() bool {
	if noProgress {
		return false
	}
	return isTerminal(int(os.Stderr.Fd()))
}

// SetNoProgress sets progress suppression (tests only).
func SetNoProgress(value bool) {
	noProgress = value
}

// SetSelectedProfile sets the current profile override (tests only).
func SetSelectedProfile(value string) {
	selectedProfile = value
}

// ResetDefaultOutputFormat clears the cached default output format so that
// DefaultOutputFormat() re-reads ASC_DEFAULT_OUTPUT on its next call. Tests only.
func ResetDefaultOutputFormat() {
	defaultOutputOnce = sync.Once{}
	defaultOutputValue = ""
}

// Bold returns the string wrapped in ANSI bold codes
func Bold(s string) string {
	if !supportsANSI() {
		return s
	}
	return bold + s + reset
}

// OrNA trims a string and returns "n/a" when empty.
func OrNA(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "n/a"
	}
	return trimmed
}

// RenderSection renders a titled section as markdown or table output.
func RenderSection(title string, headers []string, rows [][]string, markdown bool) {
	if markdown {
		fmt.Fprintf(os.Stdout, "### %s\n\n", title)
		asc.RenderMarkdown(headers, rows)
		fmt.Fprintln(os.Stdout)
		return
	}

	fmt.Fprintf(os.Stdout, "%s\n", Bold(strings.ToUpper(title)))
	asc.RenderTable(headers, rows)
	fmt.Fprintln(os.Stdout)
}

func supportsANSI() bool {
	if _, ok := os.LookupEnv("NO_COLOR"); ok {
		return false
	}
	if strings.EqualFold(os.Getenv("TERM"), "dumb") {
		return false
	}
	return isTerminal(int(os.Stderr.Fd()))
}

// DefaultUsageFunc returns a usage string with bold section headers
func DefaultUsageFunc(c *ffcli.Command) string {
	var b strings.Builder

	shortHelp := strings.TrimSpace(c.ShortHelp)
	longHelp := strings.TrimSpace(c.LongHelp)
	if shortHelp == "" && longHelp != "" {
		shortHelp = longHelp
		longHelp = ""
	}

	// DESCRIPTION
	if shortHelp != "" {
		b.WriteString(Bold("DESCRIPTION"))
		b.WriteString("\n")
		b.WriteString("  ")
		b.WriteString(shortHelp)
		b.WriteString("\n\n")
	}

	// USAGE / ShortUsage
	usage := strings.TrimSpace(c.ShortUsage)
	if usage == "" {
		usage = strings.TrimSpace(c.Name)
	}
	if usage != "" {
		b.WriteString(Bold("USAGE"))
		b.WriteString("\n")
		b.WriteString("  ")
		b.WriteString(usage)
		b.WriteString("\n\n")
	}

	// LongHelp (additional description)
	if longHelp != "" {
		if shortHelp != "" && strings.HasPrefix(longHelp, shortHelp) {
			longHelp = strings.TrimSpace(strings.TrimPrefix(longHelp, shortHelp))
		}
		if longHelp != "" {
			b.WriteString(longHelp)
			b.WriteString("\n\n")
		}
	}

	// SUBCOMMANDS
	if len(c.Subcommands) > 0 {
		b.WriteString(Bold("SUBCOMMANDS"))
		b.WriteString("\n")
		tw := tabwriter.NewWriter(&b, 0, 2, 2, ' ', 0)
		for _, sub := range c.Subcommands {
			fmt.Fprintf(tw, "  %s\t%s\n", sub.Name, sub.ShortHelp)
		}
		tw.Flush()
		b.WriteString("\n")
	}

	// FLAGS
	if c.FlagSet != nil {
		visibleFlags := VisibleHelpFlags(c.FlagSet)
		if len(visibleFlags) > 0 {
			b.WriteString(Bold("FLAGS"))
			b.WriteString("\n")
			tw := tabwriter.NewWriter(&b, 0, 2, 2, ' ', 0)
			for _, f := range visibleFlags {
				def := f.DefValue
				usage := f.Usage
				if f.Name == "output" {
					usage = strings.Replace(usage, "json (default),", "json,", 1)
				}
				if def != "" {
					fmt.Fprintf(tw, "  --%-12s %s (default: %s)\n", f.Name, usage, def)
					continue
				}
				fmt.Fprintf(tw, "  --%-12s %s\n", f.Name, usage)
			}
			tw.Flush()
			b.WriteString("\n")
		}
	}

	return b.String()
}

// DeprecatedUsageFunc returns a compact usage string for compatibility aliases.
// It intentionally omits flags and subcommands so help output only points
// callers to the canonical command path.
func DeprecatedUsageFunc(c *ffcli.Command) string {
	var b strings.Builder

	shortHelp := strings.TrimSpace(c.ShortHelp)
	longHelp := strings.TrimSpace(c.LongHelp)
	if shortHelp == "" && longHelp != "" {
		shortHelp = longHelp
		longHelp = ""
	}

	if shortHelp != "" {
		b.WriteString(Bold("DESCRIPTION"))
		b.WriteString("\n")
		b.WriteString("  ")
		b.WriteString(shortHelp)
		b.WriteString("\n\n")
	}

	usage := strings.TrimSpace(c.ShortUsage)
	if usage == "" {
		usage = strings.TrimSpace(c.Name)
	}
	if usage != "" {
		b.WriteString(Bold("USAGE"))
		b.WriteString("\n")
		b.WriteString("  ")
		b.WriteString(usage)
		b.WriteString("\n\n")
	}

	if longHelp != "" {
		if shortHelp != "" && strings.HasPrefix(longHelp, shortHelp) {
			longHelp = strings.TrimSpace(strings.TrimPrefix(longHelp, shortHelp))
		}
		if longHelp != "" {
			b.WriteString(longHelp)
			b.WriteString("\n\n")
		}
	}

	return b.String()
}

type envCredentials struct {
	keyID    string
	issuerID string
	keyPath  string
	complete bool
}

// OutputFlags stores pointers to output-related flag values.
type OutputFlags struct {
	Output *string
	Pretty *bool
}

type validatedOutputValue struct {
	value   *string
	pretty  *bool
	allowed []string
}

func (v *validatedOutputValue) String() string {
	if v == nil || v.value == nil {
		return ""
	}
	return *v.value
}

func (v *validatedOutputValue) Set(value string) error {
	if v == nil || v.value == nil {
		return fmt.Errorf("output flag is not initialized")
	}
	*v.value = value
	return nil
}

func (v *validatedOutputValue) Validate() error {
	if v == nil || v.value == nil {
		return nil
	}

	pretty := false
	if v.pretty != nil {
		pretty = *v.pretty
	}

	_, err := validateOutputFormatAllowed(*v.value, pretty, v.allowed...)
	return err
}

// MetadataOutputFlags stores pointers to metadata output-related flag values.
type MetadataOutputFlags struct {
	OutputFormat *string
	Pretty       *bool
}

// ResolvedAuthCredentials contains the concrete auth inputs selected for a command.
type ResolvedAuthCredentials struct {
	KeyID    string
	IssuerID string
	KeyPath  string
	KeyPEM   string
	Profile  string
}

type resolvedCredentials struct {
	keyID    string
	issuerID string
	keyPath  string
	keyPEM   string
	profile  string
}

type credentialSource struct {
	keyID       string
	issuerID    string
	keyMaterial string
}

func resolveEnvCredentials() (envCredentials, error) {
	keyID := strings.TrimSpace(os.Getenv("ASC_KEY_ID"))
	issuerID := strings.TrimSpace(os.Getenv("ASC_ISSUER_ID"))
	hasKeyPathEnv := strings.TrimSpace(os.Getenv("ASC_PRIVATE_KEY_PATH")) != "" ||
		strings.TrimSpace(os.Getenv(privateKeyEnvVar)) != "" ||
		strings.TrimSpace(os.Getenv(privateKeyBase64EnvVar)) != ""

	if keyID == "" && issuerID == "" && !hasKeyPathEnv {
		return envCredentials{}, nil
	}

	keyPath, err := resolvePrivateKeyPath()
	if err != nil {
		return envCredentials{}, err
	}

	creds := envCredentials{
		keyID:    keyID,
		issuerID: issuerID,
		keyPath:  keyPath,
	}
	creds.complete = keyID != "" && issuerID != "" && keyPath != ""
	return creds, nil
}

func resolveCredentials() (resolvedCredentials, error) {
	return resolveCredentialsForProfile("")
}

func resolveCredentialsForProfile(profileOverride string) (resolvedCredentials, error) {
	var actualKeyID, actualIssuerID, actualKeyPath, actualKeyPEM string
	actualProfile := ""
	profile := strings.TrimSpace(profileOverride)
	if profile == "" {
		profile = resolveProfileName()
	}
	var envCreds envCredentials
	sources := credentialSource{}

	// Priority 1: Stored credentials (keychain/config)
	cfg, storedSource, err := getCredentialsWithSourceFn(profile)
	if err != nil {
		if profile != "" {
			return resolvedCredentials{}, err
		}
		// If the user explicitly denied keychain access, fail fast instead of
		// silently falling back to env/config credentials.
		if errors.Is(err, auth.ErrKeychainAccessDenied) {
			return resolvedCredentials{}, fmt.Errorf("keychain access denied; set ASC_BYPASS_KEYCHAIN=1 to bypass: %w", err)
		}
		if !allowsEnvFallbackForStoredError(err) {
			return resolvedCredentials{}, err
		}
	} else if cfg != nil {
		actualKeyID = cfg.KeyID
		actualIssuerID = cfg.IssuerID
		actualKeyPath = cfg.PrivateKeyPath
		actualKeyPEM = strings.TrimSpace(cfg.PrivateKeyPEM)
		actualProfile = strings.TrimSpace(cfg.DefaultKeyName)
		sources.keyID = storedSource
		sources.issuerID = storedSource
		if actualKeyPath != "" || actualKeyPEM != "" {
			sources.keyMaterial = storedSource
		}
	}

	// Priority 2: Environment variables (fallback for CI/CD or when keychain unavailable)
	if actualKeyID == "" || actualIssuerID == "" || (actualKeyPath == "" && actualKeyPEM == "") {
		resolved, err := resolveEnvCredentials()
		if err != nil {
			return resolvedCredentials{}, fmt.Errorf("invalid private key environment: %w", err)
		}
		envCreds = resolved
		if actualKeyID == "" && envCreds.keyID != "" {
			actualKeyID = envCreds.keyID
			sources.keyID = "env"
		}
		if actualIssuerID == "" && envCreds.issuerID != "" {
			actualIssuerID = envCreds.issuerID
			sources.issuerID = "env"
		}
		if actualKeyPath == "" && actualKeyPEM == "" && envCreds.keyPath != "" {
			actualKeyPath = envCreds.keyPath
			sources.keyMaterial = "env"
		}
	}

	if actualKeyID == "" || actualIssuerID == "" || (actualKeyPath == "" && actualKeyPEM == "") {
		if path, err := config.Path(); err == nil {
			return resolvedCredentials{}, missingAuthError{msg: fmt.Sprintf("missing authentication. Run 'asc auth login' or create %s (see 'asc auth init')", path)}
		}
		return resolvedCredentials{}, missingAuthError{msg: "missing authentication. Run 'asc auth login' or 'asc auth init'"}
	}
	if err := checkMixedCredentialSources(sources); err != nil {
		return resolvedCredentials{}, err
	}

	return resolvedCredentials{
		keyID:    actualKeyID,
		issuerID: actualIssuerID,
		keyPath:  actualKeyPath,
		keyPEM:   actualKeyPEM,
		profile:  actualProfile,
	}, nil
}

type credentialMetadataSummary struct {
	name     string
	keyID    string
	issuerID string
}

func resolveCredentialsMetadataForProfile(profileOverride string) (ResolvedAuthCredentials, error) {
	profile := strings.TrimSpace(profileOverride)
	if profile == "" {
		profile = resolveProfileName()
	}

	resolved, err := resolveStoredCredentialMetadata(profile)
	if err == nil {
		return resolved, nil
	}
	if storedFallback, fallbackErr := resolveStoredCredentialsMetadataFallback(profile); fallbackErr == nil {
		return storedFallback, nil
	}
	if profile != "" || !allowsEnvFallbackForStoredError(err) {
		return ResolvedAuthCredentials{}, err
	}

	envKeyID := strings.TrimSpace(os.Getenv("ASC_KEY_ID"))
	if envKeyID != "" {
		return ResolvedAuthCredentials{
			KeyID:    envKeyID,
			IssuerID: strings.TrimSpace(os.Getenv("ASC_ISSUER_ID")),
		}, nil
	}

	if path, pathErr := config.Path(); pathErr == nil {
		return ResolvedAuthCredentials{}, missingAuthError{msg: fmt.Sprintf("missing authentication. Run 'asc auth login' or create %s (see 'asc auth init')", path)}
	}
	return ResolvedAuthCredentials{}, missingAuthError{msg: "missing authentication. Run 'asc auth login' or 'asc auth init'"}
}

func resolveStoredCredentialsMetadataFallback(profile string) (ResolvedAuthCredentials, error) {
	credentials, err := listCredentialSummariesFn()
	if err != nil {
		var warning *auth.CredentialsWarning
		if !errors.As(err, &warning) {
			return ResolvedAuthCredentials{}, err
		}
	}

	cred, found, selectErr := selectStoredCredentialMetadataFallback(profile, credentials)
	if selectErr != nil {
		return ResolvedAuthCredentials{}, selectErr
	}
	if !found {
		return ResolvedAuthCredentials{}, config.ErrNotFound
	}

	keyID := strings.TrimSpace(cred.KeyID)
	issuerID := strings.TrimSpace(cred.IssuerID)
	if keyID == "" {
		return ResolvedAuthCredentials{}, config.ErrNotFound
	}

	return ResolvedAuthCredentials{
		KeyID:    keyID,
		IssuerID: issuerID,
		Profile:  strings.TrimSpace(cred.Name),
	}, nil
}

func selectStoredCredentialMetadataFallback(profile string, credentials []auth.Credential) (auth.Credential, bool, error) {
	profile = strings.TrimSpace(profile)
	if profile != "" {
		for _, cred := range credentials {
			if strings.TrimSpace(cred.Name) == profile {
				return cred, true, nil
			}
		}
		return auth.Credential{}, false, fmt.Errorf("credentials not found for profile %q", profile)
	}

	for _, cred := range credentials {
		if cred.IsDefault {
			return cred, true, nil
		}
	}
	if len(credentials) > 0 {
		return auth.Credential{}, false, auth.ErrDefaultCredentialsNotFound
	}
	return auth.Credential{}, false, nil
}

func resolveStoredCredentialMetadata(profile string) (ResolvedAuthCredentials, error) {
	cfg, _, err := loadConfigForCredentialMetadata()
	if err != nil {
		return ResolvedAuthCredentials{}, err
	}

	summary, err := selectCredentialMetadataSummary(cfg, profile)
	if err != nil {
		return ResolvedAuthCredentials{}, err
	}
	return ResolvedAuthCredentials{
		KeyID:    summary.keyID,
		IssuerID: summary.issuerID,
		Profile:  summary.name,
	}, nil
}

func loadConfigForCredentialMetadata() (*config.Config, string, error) {
	path, err := config.Path()
	if err != nil {
		return nil, "", err
	}
	cfg, err := config.LoadAt(path)
	if err != nil {
		return nil, "", err
	}
	if hasConfigCredentialMetadata(cfg) || strings.TrimSpace(os.Getenv("ASC_CONFIG_PATH")) != "" {
		return cfg, path, nil
	}
	if hasConfigCredentialSelection(cfg) {
		return cfg, path, nil
	}

	globalPath, err := config.GlobalPath()
	if err != nil || globalPath == path {
		return cfg, path, nil
	}
	globalCfg, err := config.LoadAt(globalPath)
	if err != nil {
		if errors.Is(err, config.ErrNotFound) {
			return cfg, path, nil
		}
		return nil, "", err
	}
	if hasConfigCredentialMetadata(globalCfg) {
		return globalCfg, globalPath, nil
	}
	return cfg, path, nil
}

func hasConfigCredentialMetadata(cfg *config.Config) bool {
	return len(configCredentialMetadataSummaries(cfg)) > 0
}

func hasConfigCredentialSelection(cfg *config.Config) bool {
	if cfg == nil {
		return false
	}
	if strings.TrimSpace(cfg.DefaultKeyName) != "" {
		return true
	}
	if strings.TrimSpace(cfg.KeyID) != "" || strings.TrimSpace(cfg.IssuerID) != "" {
		return true
	}
	if strings.TrimSpace(cfg.PrivateKeyPath) != "" || strings.TrimSpace(cfg.PrivateKeyPEM) != "" {
		return true
	}
	return len(cfg.Keys) > 0 || len(cfg.KeychainMetadata) > 0
}

func configCredentialMetadataSummaries(cfg *config.Config) []credentialMetadataSummary {
	if cfg == nil {
		return nil
	}

	summaries := make([]credentialMetadataSummary, 0, len(cfg.Keys)+len(cfg.KeychainMetadata)+1)
	seen := make(map[string]struct{}, len(cfg.Keys)+len(cfg.KeychainMetadata)+1)

	for _, entry := range cfg.KeychainMetadata {
		name := strings.TrimSpace(entry.Name)
		keyID := strings.TrimSpace(entry.KeyID)
		if name == "" || keyID == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		summaries = append(summaries, credentialMetadataSummary{
			name:     name,
			keyID:    keyID,
			issuerID: strings.TrimSpace(entry.IssuerID),
		})
	}

	for _, cred := range cfg.Keys {
		name := strings.TrimSpace(cred.Name)
		keyID := strings.TrimSpace(cred.KeyID)
		if name == "" || keyID == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		summaries = append(summaries, credentialMetadataSummary{
			name:     name,
			keyID:    keyID,
			issuerID: strings.TrimSpace(cred.IssuerID),
		})
	}

	legacyKeyID := strings.TrimSpace(cfg.KeyID)
	if legacyKeyID != "" {
		name := strings.TrimSpace(cfg.DefaultKeyName)
		if name == "" {
			name = "default"
		}
		if _, ok := seen[name]; !ok {
			summaries = append(summaries, credentialMetadataSummary{
				name:     name,
				keyID:    legacyKeyID,
				issuerID: strings.TrimSpace(cfg.IssuerID),
			})
		}
	}

	return summaries
}

func findCredentialMetadataSummary(cfg *config.Config, name string) (credentialMetadataSummary, bool) {
	name = strings.TrimSpace(name)
	if name == "" {
		return credentialMetadataSummary{}, false
	}
	for _, summary := range configCredentialMetadataSummaries(cfg) {
		if summary.name == name {
			return summary, true
		}
	}
	return credentialMetadataSummary{}, false
}

func selectCredentialMetadataSummary(cfg *config.Config, profile string) (credentialMetadataSummary, error) {
	if cfg == nil {
		return credentialMetadataSummary{}, config.ErrNotFound
	}

	profile = strings.TrimSpace(profile)
	if profile != "" {
		summary, ok := findCredentialMetadataSummary(cfg, profile)
		if !ok {
			return credentialMetadataSummary{}, fmt.Errorf("credentials not found for profile %q", profile)
		}
		return summary, nil
	}

	defaultName := strings.TrimSpace(cfg.DefaultKeyName)
	if defaultName != "" {
		summary, ok := findCredentialMetadataSummary(cfg, defaultName)
		if !ok {
			return credentialMetadataSummary{}, auth.ErrDefaultCredentialsNotFound
		}
		return summary, nil
	}

	summaries := configCredentialMetadataSummaries(cfg)
	if len(summaries) == 1 {
		return summaries[0], nil
	}
	if len(summaries) > 0 {
		return credentialMetadataSummary{}, auth.ErrDefaultCredentialsNotFound
	}
	return credentialMetadataSummary{}, config.ErrNotFound
}

func allowsEnvFallbackForStoredError(err error) bool {
	return errors.Is(err, config.ErrNotFound) || errors.Is(err, auth.ErrDefaultCredentialsNotFound)
}

func getASCClient() (*asc.Client, error) {
	resolved, err := resolveCredentials()
	if err != nil {
		return nil, err
	}
	ApplyRootLoggingOverrides()
	if strings.TrimSpace(resolved.keyPEM) != "" {
		return asc.NewClientFromPEM(resolved.keyID, resolved.issuerID, resolved.keyPEM)
	}
	return asc.NewClient(resolved.keyID, resolved.issuerID, resolved.keyPath)
}

// ApplyRootLoggingOverrides applies root-level logging flag overrides
// (--retry-log, --debug, --api-debug) into the shared ASC runtime.
func ApplyRootLoggingOverrides() {
	if retryLog.IsSet() {
		value := retryLog.Value()
		asc.SetRetryLogOverride(&value)
	} else {
		asc.SetRetryLogOverride(nil)
	}
	if debug.IsSet() {
		value := debug.Value()
		asc.SetDebugOverride(&value)
	} else {
		asc.SetDebugOverride(nil)
	}
	if apiDebug.IsSet() {
		value := apiDebug.Value()
		asc.SetDebugHTTPOverride(&value)
	} else {
		asc.SetDebugHTTPOverride(nil)
	}
}

func checkMixedCredentialSources(sources credentialSource) error {
	keyIDSource := strings.TrimSpace(sources.keyID)
	issuerSource := strings.TrimSpace(sources.issuerID)
	keyMaterialSource := strings.TrimSpace(sources.keyMaterial)
	if keyIDSource == "" || issuerSource == "" || keyMaterialSource == "" {
		return nil
	}
	if keyIDSource == issuerSource && issuerSource == keyMaterialSource {
		return nil
	}

	message := fmt.Sprintf(
		"Warning: credentials loaded from multiple sources:\n  Key ID: %s\n  Issuer ID: %s\n  Private Key: %s\n",
		keyIDSource,
		issuerSource,
		keyMaterialSource,
	)
	if strictAuthEnabled() {
		return fmt.Errorf("mixed authentication sources detected:\n  Key ID: %s\n  Issuer ID: %s\n  Private Key: %s", keyIDSource, issuerSource, keyMaterialSource)
	}
	fmt.Fprint(os.Stderr, message)
	return nil
}

func resolvePrivateKeyPath() (string, error) {
	if path := strings.TrimSpace(os.Getenv("ASC_PRIVATE_KEY_PATH")); path != "" {
		return path, nil
	}
	if value := strings.TrimSpace(os.Getenv(privateKeyBase64EnvVar)); value != "" {
		compact := strings.Join(strings.Fields(value), "")
		cacheKey := tempPrivateKeyCacheKey("b64", compact)
		if path := cachedTempPrivateKeyPath(cacheKey); path != "" {
			return path, nil
		}
		decoded, err := decodeBase64Secret(value)
		if err != nil {
			return "", fmt.Errorf("%s: %w", privateKeyBase64EnvVar, err)
		}
		return writeTempPrivateKey(decoded, cacheKey)
	}
	if value := strings.TrimSpace(os.Getenv(privateKeyEnvVar)); value != "" {
		normalized := normalizePrivateKeyValue(value)
		cacheKey := tempPrivateKeyCacheKey("raw", normalized)
		if path := cachedTempPrivateKeyPath(cacheKey); path != "" {
			return path, nil
		}
		return writeTempPrivateKey([]byte(normalized), cacheKey)
	}
	return "", nil
}

func tempPrivateKeyCacheKey(kind, value string) string {
	sum := sha256.Sum256([]byte(kind + "\x00" + value))
	return kind + ":" + hex.EncodeToString(sum[:])
}

func cachedTempPrivateKeyPath(cacheKey string) string {
	privateKeyTempMu.Lock()
	defer privateKeyTempMu.Unlock()
	if privateKeyTempPath == "" || privateKeyTempKey != cacheKey {
		return ""
	}
	return privateKeyTempPath
}

func decodeBase64Secret(value string) ([]byte, error) {
	compact := strings.Join(strings.Fields(value), "")
	if compact == "" {
		return nil, fmt.Errorf("empty value")
	}
	decoded, err := base64.StdEncoding.DecodeString(compact)
	if err != nil {
		return nil, err
	}
	if len(decoded) == 0 {
		return nil, fmt.Errorf("decoded to empty value")
	}
	return decoded, nil
}

func normalizePrivateKeyValue(value string) string {
	if strings.Contains(value, "\\n") && !strings.Contains(value, "\n") {
		return strings.ReplaceAll(value, "\\n", "\n")
	}
	return value
}

func writeTempPrivateKey(data []byte, cacheKey string) (string, error) {
	file, err := os.CreateTemp("", "asc-key-*.p8")
	if err != nil {
		return "", err
	}
	if err := file.Chmod(0o600); err != nil {
		_ = file.Close()
		return "", err
	}
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		return "", err
	}
	if err := file.Close(); err != nil {
		return "", err
	}
	registerTempPrivateKey(file.Name(), cacheKey)
	return file.Name(), nil
}

func registerTempPrivateKey(path, cacheKey string) {
	privateKeyTempMu.Lock()
	defer privateKeyTempMu.Unlock()
	privateKeyTempPath = path
	privateKeyTempKey = cacheKey
	privateKeyTempPaths = append(privateKeyTempPaths, path)
}

// CleanupTempPrivateKeys removes any temporary private key files created during this run.
func CleanupTempPrivateKeys() {
	privateKeyTempMu.Lock()
	paths := privateKeyTempPaths
	privateKeyTempPaths = nil
	privateKeyTempPath = ""
	privateKeyTempKey = ""
	privateKeyTempMu.Unlock()

	for _, path := range paths {
		_ = os.Remove(path)
	}
}

func resolveProfileName() string {
	if strings.TrimSpace(selectedProfile) != "" {
		return strings.TrimSpace(selectedProfile)
	}
	if value := strings.TrimSpace(os.Getenv(profileEnvVar)); value != "" {
		return value
	}
	return ""
}

func strictAuthEnabled() bool {
	if strictAuth {
		return true
	}
	value := strings.TrimSpace(os.Getenv(strictAuthEnvVar))
	if value == "" {
		return false
	}
	switch strings.ToLower(value) {
	case "1", "t", "true", "yes", "y", "on":
		return true
	case "0", "f", "false", "no", "n", "off":
		return false
	default:
		warnInvalidStrictAuthValueOnce(value)
		return false
	}
}

func warnInvalidStrictAuthValueOnce(value string) {
	if value == "" {
		return
	}

	strictAuthWarnMu.Lock()
	if _, ok := strictAuthWarned[value]; ok {
		strictAuthWarnMu.Unlock()
		return
	}
	strictAuthWarned[value] = struct{}{}
	strictAuthWarnMu.Unlock()

	fmt.Fprintf(
		os.Stderr,
		"Warning: invalid %s value %q (expected true/false, 1/0, yes/no, y/n, or on/off); strict auth disabled\n",
		strictAuthEnvVar,
		value,
	)
}

func resetInvalidStrictAuthWarnings() {
	strictAuthWarnMu.Lock()
	defer strictAuthWarnMu.Unlock()
	strictAuthWarned = map[string]struct{}{}
}

func printOutput(data any, format string, pretty bool) error {
	format, err := validateOutputFormat(format, pretty)
	if err != nil {
		return err
	}
	switch format {
	case "json":
		return printJSONOutput(data, pretty)
	case "markdown":
		return asc.PrintMarkdown(data)
	case "table":
		return asc.PrintTable(data)
	default:
		return fmt.Errorf("unsupported format: %s", format)
	}
}

func printOutputWithRenderers(data any, format string, pretty bool, tableRenderer, markdownRenderer func() error) error {
	format, err := validateOutputFormat(format, pretty)
	if err != nil {
		return err
	}
	switch format {
	case "json":
		return printJSONOutput(data, pretty)
	case "table":
		if tableRenderer == nil {
			return fmt.Errorf("table renderer is required")
		}
		return tableRenderer()
	case "markdown":
		if markdownRenderer == nil {
			return fmt.Errorf("markdown renderer is required")
		}
		return markdownRenderer()
	default:
		return fmt.Errorf("unsupported format: %s", format)
	}
}

func printJSONOutput(data any, pretty bool) error {
	if pretty {
		return asc.PrintPrettyJSON(data)
	}
	return asc.PrintJSON(data)
}

// NormalizeOutputFormat lowercases format and canonicalizes aliases.
func NormalizeOutputFormat(format string) string {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "md":
		return "markdown"
	default:
		return strings.ToLower(strings.TrimSpace(format))
	}
}

func validateOutputFormat(format string, pretty bool) (string, error) {
	return validateOutputFormatAllowed(format, pretty, "json", "table", "markdown")
}

func validateOutputFormatAllowed(format string, pretty bool, allowed ...string) (string, error) {
	if len(allowed) == 0 {
		allowed = []string{"json", "table", "markdown"}
	}
	normalized := NormalizeOutputFormat(format)
	if normalized == "" {
		normalized = "json"
	}
	allowedSet := make(map[string]struct{}, len(allowed))
	for _, item := range allowed {
		if canonical := NormalizeOutputFormat(item); canonical != "" {
			allowedSet[canonical] = struct{}{}
		}
	}
	if _, ok := allowedSet[normalized]; !ok {
		return "", fmt.Errorf("unsupported format: %s", normalized)
	}
	if pretty && normalized != "json" {
		return "", fmt.Errorf("--pretty is only valid with JSON output")
	}
	return normalized, nil
}

func printStreamPage(data any) error {
	return asc.PrintJSON(data)
}

func normalizeDate(value, flagName string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", fmt.Errorf("%s is required", flagName)
	}
	parsed, err := time.Parse("2006-01-02", trimmed)
	if err != nil {
		return "", fmt.Errorf("%s must be in YYYY-MM-DD format", flagName)
	}
	return parsed.Format("2006-01-02"), nil
}

func isAppAvailabilityMissing(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, asc.ErrNotFound) {
		return true
	}
	if apiErr, ok := errors.AsType[*asc.APIError](err); ok {
		title := strings.ToLower(strings.TrimSpace(apiErr.Title))
		detail := strings.ToLower(strings.TrimSpace(apiErr.Detail))
		if strings.Contains(title, "resource does not exist") && strings.Contains(detail, "appavailabilities") {
			return true
		}
		if strings.Contains(detail, "appavailabilities") && strings.Contains(detail, "does not exist") {
			return true
		}
	}
	message := strings.ToLower(strings.TrimSpace(err.Error()))
	if strings.Contains(message, "appavailabilities") {
		if strings.Contains(message, "resource does not exist") ||
			strings.Contains(message, "does not exist") ||
			strings.Contains(message, "no resource") ||
			strings.Contains(message, "not found") {
			return true
		}
		if strings.Contains(message, "resource") {
			return true
		}
	}
	return false
}

var (
	defaultOutputOnce  sync.Once
	defaultOutputValue string
)

// DefaultOutputFormat returns the default output format for CLI commands.
// It checks ASC_DEFAULT_OUTPUT first. When unset, interactive terminals default
// to table output and non-interactive contexts default to JSON.
// Valid ASC_DEFAULT_OUTPUT values are "json", "table", "markdown", and "md".
func DefaultOutputFormat() string {
	defaultOutputOnce.Do(func() {
		defaultOutputValue = resolveDefaultOutput()
	})
	return defaultOutputValue
}

func resolveDefaultOutput() string {
	env := strings.TrimSpace(os.Getenv(defaultOutputEnvVar))
	if env == "" {
		if isTerminal(int(os.Stdout.Fd())) {
			return "table"
		}
		return "json"
	}
	normalized := strings.ToLower(env)
	switch normalized {
	case "json", "table", "markdown", "md":
		return normalized
	default:
		fmt.Fprintf(os.Stderr, "Warning: invalid %s value %q (expected json, table, markdown, or md); using json\n", defaultOutputEnvVar, env)
		return "json"
	}
}

// BindOutputFlagsWith registers a custom output-format flag and --pretty.
func BindOutputFlagsWith(fs *flag.FlagSet, flagName, defaultValue, usage string) OutputFlags {
	return BindOutputFlagsWithAllowed(fs, flagName, defaultValue, usage, "json", "table", "markdown")
}

// BindOutputFlagsWithAllowed registers a custom output-format flag and --pretty
// with an explicit allowed format set.
func BindOutputFlagsWithAllowed(fs *flag.FlagSet, flagName, defaultValue, usage string, allowed ...string) OutputFlags {
	name := strings.TrimSpace(flagName)
	if name == "" {
		name = "output"
	}

	if len(allowed) == 0 {
		allowed = []string{"json", "table", "markdown"}
	}

	outputValue := defaultValue
	prettyValue := false
	fs.Var(&validatedOutputValue{
		value:   &outputValue,
		pretty:  &prettyValue,
		allowed: slices.Clone(allowed),
	}, name, usage)

	return OutputFlags{
		Output: &outputValue,
		Pretty: bindPrettyJSONFlagWithValue(fs, &prettyValue),
	}
}

// BindPrettyJSONFlag registers a --pretty flag for JSON rendering.
func BindPrettyJSONFlag(fs *flag.FlagSet) *bool {
	value := false
	return bindPrettyJSONFlagWithValue(fs, &value)
}

func bindPrettyJSONFlagWithValue(fs *flag.FlagSet, value *bool) *bool {
	fs.BoolVar(value, "pretty", false, "Pretty-print JSON output")
	return value
}

// BindOutputFlags registers --output and --pretty flags on the provided flagset.
func BindOutputFlags(fs *flag.FlagSet) OutputFlags {
	return BindOutputFlagsWith(fs, "output", DefaultOutputFormat(), "Output format: json, table, markdown")
}

// BindMetadataOutputFlags registers --output-format and --pretty flags on the provided flagset.
func BindMetadataOutputFlags(fs *flag.FlagSet) MetadataOutputFlags {
	output := BindOutputFlagsWithAllowed(fs, "output-format", "json", "Output format for metadata: json (default), table, markdown", "json", "table", "markdown")
	return MetadataOutputFlags{
		OutputFormat: output.Output,
		Pretty:       output.Pretty,
	}
}

// ValidateBoundOutputFlags validates any output-format flags bound via the
// shared output helpers on the provided flagset.
func ValidateBoundOutputFlags(fs *flag.FlagSet) error {
	if fs == nil {
		return nil
	}

	var validationErr error
	fs.VisitAll(func(f *flag.Flag) {
		if validationErr != nil {
			return
		}

		validator, ok := f.Value.(interface{ Validate() error })
		if !ok {
			return
		}

		validationErr = validator.Validate()
	})
	return validationErr
}

func validateCommandOutputPath(commands []*ffcli.Command) error {
	for _, cmd := range commands {
		if cmd == nil {
			continue
		}
		if err := ValidateBoundOutputFlags(cmd.FlagSet); err != nil {
			return err
		}
	}
	return nil
}

// WrapCommandOutputValidation ensures shared output flags are validated before
// command execution so invalid format combinations fail before side effects.
func WrapCommandOutputValidation(cmd *ffcli.Command) {
	wrapCommandOutputValidation(cmd, nil)
}

func wrapCommandOutputValidation(cmd *ffcli.Command, parents []*ffcli.Command) {
	if cmd == nil {
		return
	}

	path := append(append([]*ffcli.Command(nil), parents...), cmd)
	for _, sub := range cmd.Subcommands {
		wrapCommandOutputValidation(sub, path)
	}

	if cmd.Exec == nil {
		return
	}

	originalExec := cmd.Exec
	cmd.Exec = func(ctx context.Context, args []string) error {
		if err := validateCommandOutputPath(path); err != nil {
			return UsageError(err.Error())
		}
		return originalExec(ctx, args)
	}
}

func resolveAppID(appID string) string {
	if appID != "" {
		return appID
	}
	if env, ok := os.LookupEnv("ASC_APP_ID"); ok {
		return strings.TrimSpace(env)
	}
	cfg, err := config.Load()
	if err != nil || cfg == nil {
		return ""
	}
	return strings.TrimSpace(cfg.AppID)
}

type timeoutParentContextKey struct{}

func withTimeoutContext(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if ctx == nil {
		ctx = context.Background()
	}
	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	return context.WithValue(timeoutCtx, timeoutParentContextKey{}, ctx), cancel
}

func contextWithoutTimeout(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	if base, ok := ctx.Value(timeoutParentContextKey{}).(context.Context); ok && base != nil {
		return contextWithoutTimeout(base)
	}
	return ctx
}

func contextWithTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	return withTimeoutContext(ctx, asc.ResolveTimeout())
}

func contextWithUploadTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	return withTimeoutContext(ctx, asc.ResolveUploadTimeout())
}

func splitCSV(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	cleaned := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		cleaned = append(cleaned, part)
	}
	return cleaned
}

func splitUniqueCSV(value string) []string {
	values := splitCSV(value)
	if len(values) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(values))
	unique := make([]string, 0, len(values))
	for _, item := range values {
		if _, exists := seen[item]; exists {
			continue
		}
		seen[item] = struct{}{}
		unique = append(unique, item)
	}
	return unique
}

func splitCSVUpper(value string) []string {
	values := splitCSV(value)
	if len(values) == 0 {
		return nil
	}
	upper := make([]string, 0, len(values))
	for _, item := range values {
		upper = append(upper, strings.ToUpper(item))
	}
	return upper
}

func validateNextURL(next string) error {
	next = strings.TrimSpace(next)
	if next == "" {
		return nil
	}
	parsed, err := url.Parse(next)
	if err != nil {
		return fmt.Errorf("--next must be a valid URL: %w", err)
	}
	if parsed.Scheme != "https" || parsed.Host != "api.appstoreconnect.apple.com" {
		return fmt.Errorf("--next must be an App Store Connect URL")
	}
	return nil
}

func validateSort(value string, allowed ...string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	if slices.Contains(allowed, value) {
		return nil
	}
	return fmt.Errorf("--sort must be one of: %s", strings.Join(allowed, ", "))
}

// Exported wrappers for shared helpers.
func GetASCClient() (*asc.Client, error) {
	// Auth resolution can block on macOS keychain prompts. Show a subtle spinner on stderr
	// (interactive runs only) so the CLI doesn’t look “stuck”.
	const authSpinnerDelay = 200 * time.Millisecond
	var client *asc.Client
	err := WithSpinnerDelayed("", authSpinnerDelay, func() error {
		var innerErr error
		client, innerErr = getASCClient()
		return innerErr
	})
	return client, err
}

// ResolveAuthCredentials resolves the signing credentials for a command.
// A non-empty profile override takes precedence over root-level profile selection.
func ResolveAuthCredentials(profile string) (ResolvedAuthCredentials, error) {
	const authSpinnerDelay = 200 * time.Millisecond
	var resolved resolvedCredentials
	err := WithSpinnerDelayed("", authSpinnerDelay, func() error {
		var innerErr error
		resolved, innerErr = resolveCredentialsForProfile(profile)
		return innerErr
	})
	if err != nil {
		return ResolvedAuthCredentials{}, err
	}
	return ResolvedAuthCredentials{
		KeyID:    resolved.keyID,
		IssuerID: resolved.issuerID,
		KeyPath:  resolved.keyPath,
		KeyPEM:   resolved.keyPEM,
		Profile:  resolved.profile,
	}, nil
}

// ResolveAuthCredentialsMetadata resolves the selected auth profile's key metadata
// without loading private key material from keychain or disk.
func ResolveAuthCredentialsMetadata(profile string) (ResolvedAuthCredentials, error) {
	return resolveCredentialsMetadataForProfile(profile)
}

func ResolveProfileName() string {
	return resolveProfileName()
}

func PrintOutput(data any, format string, pretty bool) error {
	return printOutput(data, format, pretty)
}

func PrintOutputWithRenderers(data any, format string, pretty bool, tableRenderer, markdownRenderer func() error) error {
	return printOutputWithRenderers(data, format, pretty, tableRenderer, markdownRenderer)
}

func ValidateOutputFormat(format string, pretty bool) (string, error) {
	return validateOutputFormat(format, pretty)
}

func ValidateOutputFormatAllowed(format string, pretty bool, allowed ...string) (string, error) {
	return validateOutputFormatAllowed(format, pretty, allowed...)
}

func NormalizeDate(value, flagName string) (string, error) {
	return normalizeDate(value, flagName)
}

func IsAppAvailabilityMissing(err error) bool {
	return isAppAvailabilityMissing(err)
}

func ResolveAppID(appID string) string {
	return resolveAppID(appID)
}

func ContextWithTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	return contextWithTimeout(ctx)
}

func ContextWithUploadTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	return contextWithUploadTimeout(ctx)
}

func ContextWithoutTimeout(ctx context.Context) context.Context {
	return contextWithoutTimeout(ctx)
}

func SplitCSV(value string) []string {
	return splitCSV(value)
}

func SplitUniqueCSV(value string) []string {
	return splitUniqueCSV(value)
}

func SplitCSVUpper(value string) []string {
	return splitCSVUpper(value)
}

func ValidateNextURL(next string) error {
	return validateNextURL(next)
}

func ValidateSort(value string, allowed ...string) error {
	return validateSort(value, allowed...)
}

// PrintStreamPage writes a single page of data as a JSON line to stdout.
// Used with --stream --paginate to emit results page-by-page as NDJSON
// instead of buffering all pages in memory.
func PrintStreamPage(data any) error {
	return printStreamPage(data)
}
