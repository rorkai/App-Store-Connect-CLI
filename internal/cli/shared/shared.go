package shared

import (
	"context"
	"encoding/base64"
	"errors"
	"flag"
	"fmt"
	"net/url"
	"os"
	"strconv"
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
	privateKeyTempPaths []string
	selectedProfile     string
	strictAuth          bool
	retryLog            OptionalBool
	debug               OptionalBool
	apiDebug            OptionalBool
	noUpdate            bool
)

var (
	isTerminal = term.IsTerminal
	noProgress bool
)

// BindRootFlags registers root-level flags that affect shared CLI behavior.
func BindRootFlags(fs *flag.FlagSet) {
	fs.StringVar(&selectedProfile, "profile", "", "Use named authentication profile")
	fs.BoolVar(&strictAuth, "strict-auth", false, "Fail when credentials are resolved from multiple sources")
	fs.Var(&retryLog, "retry-log", "Enable retry logging to stderr (overrides ASC_RETRY_LOG/config when set)")
	fs.Var(&debug, "debug", "Enable debug logging to stderr")
	fs.Var(&apiDebug, "api-debug", "Enable HTTP debug logging to stderr (redacts sensitive values)")
	fs.BoolVar(&noUpdate, "no-update", false, "Skip update checks and auto-update")
	BindCIFlags(fs)
}

// SelectedProfile returns the current profile override.
func SelectedProfile() string {
	return selectedProfile
}

// NoUpdate reports whether update checks are disabled via flag.
func NoUpdate() bool {
	return noUpdate
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

// CleanupTempPrivateKey removes any temporary private key created from env values.
// Deprecated: use CleanupTempPrivateKeys to remove all tracked temp keys.
func CleanupTempPrivateKey() {
	CleanupTempPrivateKeys()
}

// Bold returns the string wrapped in ANSI bold codes
func Bold(s string) string {
	if !supportsANSI() {
		return s
	}
	return bold + s + reset
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
			fmt.Fprintf(tw, "  %-12s %s\n", sub.Name, sub.ShortHelp)
		}
		tw.Flush()
		b.WriteString("\n")
	}

	// FLAGS
	if c.FlagSet != nil {
		hasFlags := false
		c.FlagSet.VisitAll(func(*flag.Flag) {
			hasFlags = true
		})
		if hasFlags {
			b.WriteString(Bold("FLAGS"))
			b.WriteString("\n")
			tw := tabwriter.NewWriter(&b, 0, 2, 2, ' ', 0)
			c.FlagSet.VisitAll(func(f *flag.Flag) {
				def := f.DefValue
				usage := f.Usage
				if f.Name == "output" {
					usage = strings.Replace(usage, "json (default),", "json,", 1)
				}
				if def != "" {
					fmt.Fprintf(tw, "  --%-12s %s (default: %s)\n", f.Name, usage, def)
					return
				}
				fmt.Fprintf(tw, "  --%-12s %s\n", f.Name, usage)
			})
			tw.Flush()
			b.WriteString("\n")
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

type resolvedCredentials struct {
	keyID    string
	issuerID string
	keyPath  string
}

type credentialSource struct {
	keyID    string
	issuerID string
	keyPath  string
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
	var actualKeyID, actualIssuerID, actualKeyPath string
	profile := resolveProfileName()
	var envCreds envCredentials
	envResolved := false
	sources := credentialSource{}

	if profile == "" && auth.ShouldBypassKeychain() {
		resolved, err := resolveEnvCredentials()
		if err != nil {
			return resolvedCredentials{}, fmt.Errorf("invalid private key environment: %w", err)
		}
		envCreds = resolved
		envResolved = true
		if envCreds.complete {
			sources.keyID = "env"
			sources.issuerID = "env"
			sources.keyPath = "env"
			return resolvedCredentials{
				keyID:    envCreds.keyID,
				issuerID: envCreds.issuerID,
				keyPath:  envCreds.keyPath,
			}, nil
		}
	}

	// Priority 1: Stored credentials (keychain/config)
	cfg, storedSource, err := auth.GetCredentialsWithSource(profile)
	if err != nil {
		if profile != "" {
			return resolvedCredentials{}, err
		}
	} else if cfg != nil {
		actualKeyID = cfg.KeyID
		actualIssuerID = cfg.IssuerID
		actualKeyPath = cfg.PrivateKeyPath
		sources.keyID = storedSource
		sources.issuerID = storedSource
		sources.keyPath = storedSource
	}

	// Priority 2: Environment variables (fallback for CI/CD or when keychain unavailable)
	if actualKeyID == "" || actualIssuerID == "" || actualKeyPath == "" {
		if !envResolved {
			resolved, err := resolveEnvCredentials()
			if err != nil {
				return resolvedCredentials{}, fmt.Errorf("invalid private key environment: %w", err)
			}
			envCreds = resolved
		}
		if actualKeyID == "" && envCreds.keyID != "" {
			actualKeyID = envCreds.keyID
			sources.keyID = "env"
		}
		if actualIssuerID == "" && envCreds.issuerID != "" {
			actualIssuerID = envCreds.issuerID
			sources.issuerID = "env"
		}
		if actualKeyPath == "" && envCreds.keyPath != "" {
			actualKeyPath = envCreds.keyPath
			sources.keyPath = "env"
		}
	}

	if actualKeyID == "" || actualIssuerID == "" || actualKeyPath == "" {
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
	}, nil
}

func getASCClient() (*asc.Client, error) {
	resolved, err := resolveCredentials()
	if err != nil {
		return nil, err
	}
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
	return asc.NewClient(resolved.keyID, resolved.issuerID, resolved.keyPath)
}

func checkMixedCredentialSources(sources credentialSource) error {
	keyIDSource := strings.TrimSpace(sources.keyID)
	issuerSource := strings.TrimSpace(sources.issuerID)
	keyPathSource := strings.TrimSpace(sources.keyPath)
	if keyIDSource == "" || issuerSource == "" || keyPathSource == "" {
		return nil
	}
	if keyIDSource == issuerSource && issuerSource == keyPathSource {
		return nil
	}

	message := fmt.Sprintf(
		"Warning: credentials loaded from multiple sources:\n  Key ID: %s\n  Issuer ID: %s\n  Private Key: %s\n",
		keyIDSource,
		issuerSource,
		keyPathSource,
	)
	if strictAuthEnabled() {
		return fmt.Errorf("mixed authentication sources detected:\n  Key ID: %s\n  Issuer ID: %s\n  Private Key: %s", keyIDSource, issuerSource, keyPathSource)
	}
	fmt.Fprint(os.Stderr, message)
	return nil
}

func resolvePrivateKeyPath() (string, error) {
	if path := strings.TrimSpace(os.Getenv("ASC_PRIVATE_KEY_PATH")); path != "" {
		return path, nil
	}
	if privateKeyTempPath != "" {
		return privateKeyTempPath, nil
	}
	if value := strings.TrimSpace(os.Getenv(privateKeyBase64EnvVar)); value != "" {
		decoded, err := decodeBase64Secret(value)
		if err != nil {
			return "", fmt.Errorf("%s: %w", privateKeyBase64EnvVar, err)
		}
		return writeTempPrivateKey(decoded)
	}
	if value := strings.TrimSpace(os.Getenv(privateKeyEnvVar)); value != "" {
		return writeTempPrivateKey([]byte(normalizePrivateKeyValue(value)))
	}
	return "", nil
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

func writeTempPrivateKey(data []byte) (string, error) {
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
	registerTempPrivateKey(file.Name())
	return file.Name(), nil
}

func registerTempPrivateKey(path string) {
	privateKeyTempMu.Lock()
	defer privateKeyTempMu.Unlock()
	privateKeyTempPath = path
	privateKeyTempPaths = append(privateKeyTempPaths, path)
}

// CleanupTempPrivateKeys removes any temporary private key files created during this run.
func CleanupTempPrivateKeys() {
	privateKeyTempMu.Lock()
	paths := privateKeyTempPaths
	privateKeyTempPaths = nil
	privateKeyTempPath = ""
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
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return false
	}
	return parsed
}

func printOutput(data interface{}, format string, pretty bool) error {
	format = strings.ToLower(format)
	switch format {
	case "json":
		if pretty {
			return asc.PrintPrettyJSON(data)
		}
		return asc.PrintJSON(data)
	case "markdown", "md":
		if pretty {
			return fmt.Errorf("--pretty is only valid with JSON output")
		}
		return asc.PrintMarkdown(data)
	case "table":
		if pretty {
			return fmt.Errorf("--pretty is only valid with JSON output")
		}
		return asc.PrintTable(data)
	default:
		return fmt.Errorf("unsupported format: %s", format)
	}
}

func printStreamPage(data interface{}) error {
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
	var apiErr *asc.APIError
	if errors.As(err, &apiErr) {
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
// It checks the ASC_DEFAULT_OUTPUT environment variable first, falling back to "json".
// Valid values are "json", "table", "markdown", and "md".
func DefaultOutputFormat() string {
	defaultOutputOnce.Do(func() {
		defaultOutputValue = resolveDefaultOutput()
	})
	return defaultOutputValue
}

func resolveDefaultOutput() string {
	env := strings.TrimSpace(os.Getenv(defaultOutputEnvVar))
	if env == "" {
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

func contextWithTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithTimeout(ctx, asc.ResolveTimeout())
}

func contextWithUploadTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithTimeout(ctx, asc.ResolveUploadTimeout())
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
	for _, option := range allowed {
		if value == option {
			return nil
		}
	}
	return fmt.Errorf("--sort must be one of: %s", strings.Join(allowed, ", "))
}

// Exported wrappers for shared helpers.
func GetASCClient() (*asc.Client, error) {
	return getASCClient()
}

func ResolveProfileName() string {
	return resolveProfileName()
}

func ResolvePrivateKeyPath() (string, error) {
	return resolvePrivateKeyPath()
}

func PrintOutput(data interface{}, format string, pretty bool) error {
	return printOutput(data, format, pretty)
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

func SplitCSV(value string) []string {
	return splitCSV(value)
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
func PrintStreamPage(data interface{}) error {
	return printStreamPage(data)
}
