package main

import (
	"context"
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/peterbourgon/ff/v3/ffcli"
	"github.com/rudrankriyam/App-Store-Connect-CLI/apps/studio/internal/studio/acp"
	"github.com/rudrankriyam/App-Store-Connect-CLI/apps/studio/internal/studio/approvals"
	"github.com/rudrankriyam/App-Store-Connect-CLI/apps/studio/internal/studio/settings"
	"github.com/rudrankriyam/App-Store-Connect-CLI/apps/studio/internal/studio/threads"
	asccmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
)

func TestParseAppsListOutputAcceptsEmptyEnvelope(t *testing.T) {
	rawApps, err := parseAppsListOutput([]byte(`{"data":[]}`))
	if err != nil {
		t.Fatalf("parseAppsListOutput() error = %v", err)
	}
	if len(rawApps) != 0 {
		t.Fatalf("len(rawApps) = %d, want 0", len(rawApps))
	}
}

func TestParseAvailabilityViewOutputReturnsResourceID(t *testing.T) {
	availabilityID, available, err := parseAvailabilityViewOutput([]byte(`{"data":{"id":"availability-123","attributes":{"availableInNewTerritories":true}}}`))
	if err != nil {
		t.Fatalf("parseAvailabilityViewOutput() error = %v", err)
	}
	if availabilityID != "availability-123" {
		t.Fatalf("availabilityID = %q, want availability-123", availabilityID)
	}
	if !available {
		t.Fatal("available = false, want true")
	}
}

func TestParseFinanceRegionsOutputAcceptsRegionsEnvelope(t *testing.T) {
	regions, err := parseFinanceRegionsOutput([]byte(`{"regions":[{"reportRegion":"Americas","reportCurrency":"USD","regionCode":"US","countriesOrRegions":"United States"}]}`))
	if err != nil {
		t.Fatalf("parseFinanceRegionsOutput() error = %v", err)
	}
	if len(regions) != 1 {
		t.Fatalf("len(regions) = %d, want 1", len(regions))
	}
	if regions[0].Code != "US" {
		t.Fatalf("regions[0].Code = %q, want US", regions[0].Code)
	}
}

func TestParseFinanceRegionsOutputAcceptsDataEnvelope(t *testing.T) {
	regions, err := parseFinanceRegionsOutput([]byte(`{"data":[{"reportRegion":"Europe","reportCurrency":"EUR","regionCode":"EU","countriesOrRegions":"Euro-Zone"}]}`))
	if err != nil {
		t.Fatalf("parseFinanceRegionsOutput() error = %v", err)
	}
	if len(regions) != 1 {
		t.Fatalf("len(regions) = %d, want 1", len(regions))
	}
	if regions[0].Code != "EU" {
		t.Fatalf("regions[0].Code = %q, want EU", regions[0].Code)
	}
}

func TestParseFinanceRegionsOutputRejectsUnknownEnvelope(t *testing.T) {
	if _, err := parseFinanceRegionsOutput([]byte(`{"unexpected":[]}`)); err == nil {
		t.Fatal("parseFinanceRegionsOutput() error = nil, want error")
	}
}

func newStudioAppWithSystemASCPath(t *testing.T, ascPath string) *App {
	t.Helper()

	rootDir := t.TempDir()
	settingsStore := settings.NewStore(rootDir)
	if err := settingsStore.Save(settings.StudioSettings{
		SystemASCPath:    ascPath,
		PreferBundledASC: false,
	}); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	return &App{settings: settingsStore}
}

func TestListAppsUsesPaginatedFetch(t *testing.T) {
	ascPath := filepath.Join(t.TempDir(), "asc")
	script := `#!/bin/sh
has_arg() {
  wanted="$1"
  shift
  for arg in "$@"; do
    if [ "$arg" = "$wanted" ]; then
      return 0
    fi
  done
  return 1
}

if [ "$1" = "apps" ] && [ "$2" = "list" ]; then
  if ! has_arg --paginate "$@"; then
    printf 'missing --paginate' >&2
    exit 1
  fi
  printf '{"data":[{"id":"app-1","attributes":{"name":"First App","bundleId":"com.example.first","sku":"FIRST"}},{"id":"app-2","attributes":{"name":"Second App","bundleId":"com.example.second","sku":"SECOND"}}]}'
  exit 0
fi

if [ "$1" = "localizations" ] && [ "$2" = "list" ]; then
  printf '{"data":[]}'
  exit 0
fi

printf 'unexpected args: %s' "$*" >&2
exit 1
`
	if err := os.WriteFile(ascPath, []byte(script), 0o755); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	app := newStudioAppWithSystemASCPath(t, ascPath)
	got, err := app.ListApps()
	if err != nil {
		t.Fatalf("ListApps() error = %v", err)
	}
	if got.Error != "" {
		t.Fatalf("ListApps().Error = %q, want empty", got.Error)
	}
	if len(got.Apps) != 2 {
		t.Fatalf("len(ListApps().Apps) = %d, want 2", len(got.Apps))
	}
	if got.Apps[1].ID != "app-2" {
		t.Fatalf("ListApps().Apps[1].ID = %q, want app-2", got.Apps[1].ID)
	}
}

func TestLoadSubscriptionsKeepsPartialResultsWhenGroupFetchFails(t *testing.T) {
	ascPath := filepath.Join(t.TempDir(), "asc")
	script := `#!/bin/sh
has_arg() {
  wanted="$1"
  shift
  for arg in "$@"; do
    if [ "$arg" = "$wanted" ]; then
      return 0
    fi
  done
  return 1
}

if [ "$1" = "subscriptions" ] && [ "$2" = "groups" ] && [ "$3" = "list" ]; then
  if ! has_arg --paginate "$@"; then
    printf 'missing group pagination' >&2
    exit 1
  fi
  printf '{"data":[{"id":"group-1","attributes":{"referenceName":"Main Group"}},{"id":"group-2","attributes":{"referenceName":"Secondary Group"}}]}'
  exit 0
fi

if [ "$1" = "subscriptions" ] && [ "$2" = "list" ]; then
  if ! has_arg --paginate "$@"; then
    printf 'missing subscription pagination' >&2
    exit 1
  fi
  if [ "$4" = "group-1" ]; then
    printf '{"data":[{"id":"sub-1","attributes":{"name":"Pro","productId":"pro.monthly","state":"READY_FOR_SUBMISSION","subscriptionPeriod":"ONE_MONTH","reviewNote":"","groupLevel":1}}]}'
    exit 0
  fi
  printf 'group fetch failed' >&2
  exit 1
fi

printf 'unexpected args: %s' "$*" >&2
exit 1
`
	if err := os.WriteFile(ascPath, []byte(script), 0o755); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	app := &App{}
	got := app.loadSubscriptions(context.Background(), ascPath, "app-1")
	if !strings.Contains(got.Error, "group fetch failed") {
		t.Fatalf("loadSubscriptions().Error = %q, want group fetch failure", got.Error)
	}
	if len(got.Subscriptions) != 1 {
		t.Fatalf("len(loadSubscriptions().Subscriptions) = %d, want 1 partial result", len(got.Subscriptions))
	}
	if got.Subscriptions[0].ProductID != "pro.monthly" {
		t.Fatalf("loadSubscriptions().Subscriptions[0].ProductID = %q, want pro.monthly", got.Subscriptions[0].ProductID)
	}
	if got.Subscriptions[0].GroupName != "Main Group" {
		t.Fatalf("loadSubscriptions().Subscriptions[0].GroupName = %q, want Main Group", got.Subscriptions[0].GroupName)
	}
}

func TestGetOfferCodesUsesPaginationAndKeepsPartialResults(t *testing.T) {
	ascPath := filepath.Join(t.TempDir(), "asc")
	script := `#!/bin/sh
has_arg() {
  wanted="$1"
  shift
  for arg in "$@"; do
    if [ "$arg" = "$wanted" ]; then
      return 0
    fi
  done
  return 1
}

if [ "$1" = "subscriptions" ] && [ "$2" = "groups" ] && [ "$3" = "list" ]; then
  if ! has_arg --paginate "$@"; then
    printf 'missing group pagination' >&2
    exit 1
  fi
  printf '{"data":[{"id":"group-1","attributes":{"referenceName":"Main Group"}}]}'
  exit 0
fi

if [ "$1" = "subscriptions" ] && [ "$2" = "list" ]; then
  if ! has_arg --paginate "$@"; then
    printf 'missing subscription pagination' >&2
    exit 1
  fi
  printf '{"data":[{"id":"sub-1","attributes":{"name":"Pro","productId":"pro.monthly","state":"READY_FOR_SUBMISSION","subscriptionPeriod":"ONE_MONTH","reviewNote":"","groupLevel":1}},{"id":"sub-2","attributes":{"name":"Plus","productId":"plus.monthly","state":"READY_FOR_SUBMISSION","subscriptionPeriod":"ONE_MONTH","reviewNote":"","groupLevel":2}}]}'
  exit 0
fi

if [ "$1" = "subscriptions" ] && [ "$2" = "offers" ] && [ "$3" = "offer-codes" ] && [ "$4" = "list" ]; then
  if ! has_arg --paginate "$@"; then
    printf 'missing offer code pagination' >&2
    exit 1
  fi
  if [ "$6" = "sub-1" ]; then
    printf '{"data":[{"attributes":{"name":"Welcome Offer","offerEligibility":"NEW","customerEligibilities":[],"duration":"ONE_MONTH","offerMode":"FREE_TRIAL","numberOfPeriods":1,"totalNumberOfCodes":100,"productionCodeCount":40}}]}'
    exit 0
  fi
  printf 'offer fetch failed' >&2
  exit 1
fi

printf 'unexpected args: %s' "$*" >&2
exit 1
`
	if err := os.WriteFile(ascPath, []byte(script), 0o755); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	app := newStudioAppWithSystemASCPath(t, ascPath)
	got, err := app.GetOfferCodes("app-1")
	if err != nil {
		t.Fatalf("GetOfferCodes() error = %v", err)
	}
	if !strings.Contains(got.Error, "offer fetch failed") {
		t.Fatalf("GetOfferCodes().Error = %q, want offer fetch failure", got.Error)
	}
	if len(got.OfferCodes) != 1 {
		t.Fatalf("len(GetOfferCodes().OfferCodes) = %d, want 1 partial result", len(got.OfferCodes))
	}
	if got.OfferCodes[0].Name != "Welcome Offer" {
		t.Fatalf("GetOfferCodes().OfferCodes[0].Name = %q, want Welcome Offer", got.OfferCodes[0].Name)
	}
}

func TestSetEnvVarDoesNotMutateInputSlice(t *testing.T) {
	original := []string{"A=1", "ASC_KEY_ID=old", "B=2"}
	snapshot := append([]string(nil), original...)

	got := setEnvVar(original, "ASC_KEY_ID", "new")
	got = setEnvVar(got, "ASC_ISSUER_ID", "issuer")

	if strings.Join(original, "\n") != strings.Join(snapshot, "\n") {
		t.Fatalf("input slice mutated:\ngot  %q\nwant %q", original, snapshot)
	}

	want := []string{"A=1", "B=2", "ASC_KEY_ID=new", "ASC_ISSUER_ID=issuer"}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("setEnvVar() result = %q, want %q", got, want)
	}
}

func TestParseFirstAppPriceReference(t *testing.T) {
	priceID := base64.RawURLEncoding.EncodeToString([]byte(`{"t":"CAN","p":"price-point-42"}`))

	ref, found, err := parseFirstAppPriceReference([]byte(`{"data":[{"id":"` + priceID + `"}]}`))
	if err != nil {
		t.Fatalf("parseFirstAppPriceReference() error = %v", err)
	}
	if !found {
		t.Fatal("found = false, want true")
	}
	if ref.Territory != "CAN" || ref.PricePoint != "price-point-42" {
		t.Fatalf("ref = %+v, want territory CAN and price point price-point-42", ref)
	}
}

func TestParseAppPricePointLookupUsesIncludedTerritoryCurrency(t *testing.T) {
	pricePointID := base64.RawURLEncoding.EncodeToString([]byte(`{"p":"price-point-42"}`))

	result, found, err := parseAppPricePointLookup([]byte(`{
		"data":[{"id":"`+pricePointID+`","attributes":{"customerPrice":"4.99","proceeds":"3.49"}}],
		"included":[{"type":"territories","id":"CAN","attributes":{"currency":"CAD"}}]
	}`), "CAN", "price-point-42")
	if err != nil {
		t.Fatalf("parseAppPricePointLookup() error = %v", err)
	}
	if !found {
		t.Fatal("found = false, want true")
	}
	if result.Price != "4.99" || result.Proceeds != "3.49" || result.Currency != "CAD" {
		t.Fatalf("result = %+v, want price 4.99, proceeds 3.49, currency CAD", result)
	}
}

func TestParseASCCommandArgsSupportsQuotedValues(t *testing.T) {
	args, err := parseASCCommandArgs(`status --app "123 456" --output json`)
	if err != nil {
		t.Fatalf("parseASCCommandArgs() error = %v", err)
	}
	want := []string{"status", "--app", "123 456", "--output", "json"}
	if len(args) != len(want) {
		t.Fatalf("len(args) = %d, want %d", len(args), len(want))
	}
	for i := range want {
		if args[i] != want[i] {
			t.Fatalf("args[%d] = %q, want %q", i, args[i], want[i])
		}
	}
}

func TestParseASCCommandArgsRejectsUnterminatedQuotes(t *testing.T) {
	if _, err := parseASCCommandArgs(`status --app "123`); err == nil {
		t.Fatal("parseASCCommandArgs() error = nil, want error")
	}
}

func TestRunASCCommandRejectsDisallowedPaths(t *testing.T) {
	app := &App{}

	got, err := app.RunASCCommand("publish appstore --app 123 --output json")
	if err != nil {
		t.Fatalf("RunASCCommand() error = %v", err)
	}
	if got.Error != "Command is not allowed in ASC Studio" {
		t.Fatalf("RunASCCommand().Error = %q, want command rejection", got.Error)
	}
}

func TestStudioCommandPathKeepsHyphenatedSubcommands(t *testing.T) {
	parts, err := parseASCCommandArgs("review submissions-list --app 123 --output json")
	if err != nil {
		t.Fatalf("parseASCCommandArgs() error = %v", err)
	}
	if got := studioCommandPath(parts); got != "review submissions-list" {
		t.Fatalf("studioCommandPath() = %q, want review submissions-list", got)
	}
}

func TestFrontendRunASCCommandsStayInStudioAllowlistAndCLIRegistry(t *testing.T) {
	commands, err := frontendRunASCCommands()
	if err != nil {
		t.Fatalf("frontendRunASCCommands() error = %v", err)
	}
	if len(commands) == 0 {
		t.Fatal("frontendRunASCCommands() returned no commands")
	}

	root := asccmd.RootCommand("test")
	var missing []string
	var unregistered []string
	for _, command := range commands {
		parts, err := parseASCCommandArgs(strings.ReplaceAll(command, "APP_ID", "app-id"))
		if err != nil {
			t.Fatalf("parseASCCommandArgs(%q) error = %v", command, err)
		}
		path := studioCommandPath(parts)
		if _, ok := allowedStudioCommandPaths[path]; !ok {
			missing = append(missing, path+" <= "+command)
		}
		if !cliCommandPathExists(root, parts) {
			unregistered = append(unregistered, path+" <= "+command)
		}
	}
	if len(missing) > 0 {
		t.Fatalf("frontend RunASCCommand paths missing from Studio allowlist:\n%s", strings.Join(missing, "\n"))
	}
	if len(unregistered) > 0 {
		t.Fatalf("frontend RunASCCommand paths missing from CLI registry:\n%s", strings.Join(unregistered, "\n"))
	}
}

func TestBundledASCPathPrefersAppBundleResources(t *testing.T) {
	tmp := t.TempDir()
	resourceDir := filepath.Join(tmp, "ASC Studio.app", "Contents", "Resources", "bin")
	if err := os.MkdirAll(resourceDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	bundled := filepath.Join(resourceDir, "asc")
	if err := os.WriteFile(bundled, []byte("binary"), 0o755); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	originalExecutable := osExecutableFunc
	originalGetwd := getwdFunc
	t.Cleanup(func() {
		osExecutableFunc = originalExecutable
		getwdFunc = originalGetwd
	})

	osExecutableFunc = func() (string, error) {
		return filepath.Join(tmp, "ASC Studio.app", "Contents", "MacOS", "ASC Studio"), nil
	}
	getwdFunc = func() (string, error) {
		return filepath.Join(tmp, "workspace"), nil
	}

	app := &App{}
	if got := app.bundledASCPath(); got != bundled {
		t.Fatalf("bundledASCPath() = %q, want %q", got, bundled)
	}
}

func TestResolveASCMatchesCommandResolution(t *testing.T) {
	tmp := t.TempDir()
	resourceDir := filepath.Join(tmp, "ASC Studio.app", "Contents", "Resources", "bin")
	if err := os.MkdirAll(resourceDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	bundled := filepath.Join(resourceDir, "asc")
	if err := os.WriteFile(bundled, []byte("binary"), 0o755); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	originalExecutable := osExecutableFunc
	originalGetwd := getwdFunc
	originalLookPath := execLookPathFunc
	t.Cleanup(func() {
		osExecutableFunc = originalExecutable
		getwdFunc = originalGetwd
		execLookPathFunc = originalLookPath
	})

	osExecutableFunc = func() (string, error) {
		return filepath.Join(tmp, "ASC Studio.app", "Contents", "MacOS", "ASC Studio"), nil
	}
	getwdFunc = func() (string, error) {
		return filepath.Join(tmp, "workspace"), nil
	}
	execLookPathFunc = func(string) (string, error) {
		return "/usr/local/bin/asc", nil
	}

	settingsStore := settings.NewStore(tmp)
	if err := settingsStore.Save(settings.StudioSettings{
		PreferBundledASC: true,
	}); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	app := &App{settings: settingsStore}
	path, err := app.resolveASCPath()
	if err != nil {
		t.Fatalf("resolveASCPath() error = %v", err)
	}
	if path != bundled {
		t.Fatalf("resolveASCPath() = %q, want %q", path, bundled)
	}

	resp, err := app.ResolveASC()
	if err != nil {
		t.Fatalf("ResolveASC() error = %v", err)
	}
	if resp.Resolution.Path != bundled {
		t.Fatalf("ResolveASC().Resolution.Path = %q, want %q", resp.Resolution.Path, bundled)
	}
	if resp.Resolution.Source != "bundled" {
		t.Fatalf("ResolveASC().Resolution.Source = %q, want bundled", resp.Resolution.Source)
	}
}

func TestRunASCCombinedOutputUsesShadowConfigPath(t *testing.T) {
	tmpHome := t.TempDir()
	configDir := filepath.Join(tmpHome, ".asc")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	configPath := filepath.Join(configDir, "config.json")
	if err := os.WriteFile(configPath, []byte(`{"key_id":"KEY123"}`), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	t.Setenv("HOME", tmpHome)
	t.Setenv("ASC_CONFIG_PATH", configPath)

	ascPath := filepath.Join(t.TempDir(), "asc")
	script := "#!/bin/sh\ncat \"$ASC_CONFIG_PATH\"\nprintf '\\n%s' \"$ASC_CONFIG_PATH\""
	if err := os.WriteFile(ascPath, []byte(script), 0o755); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	app := &App{}
	out, err := app.runASCCombinedOutput(context.Background(), ascPath, "auth", "status")
	if err != nil {
		t.Fatalf("runASCCombinedOutput() error = %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) < 2 {
		t.Fatalf("command output = %q, want config contents and shadow path", string(out))
	}
	shadowPath := lines[len(lines)-1]
	shadowConfig := strings.Join(lines[:len(lines)-1], "\n")
	if !strings.Contains(shadowConfig, `"KEY123"`) {
		t.Fatalf("shadow config contents = %q, want copied config contents", shadowConfig)
	}
	if shadowPath == "" {
		t.Fatal("shadowPath = empty, want isolated config path")
	}
	if shadowPath == configPath {
		t.Fatalf("shadowPath = %q, want path distinct from real config", shadowPath)
	}
}

func TestRunASCCombinedOutputPreservesLegitimateConfigWrites(t *testing.T) {
	tmpHome := t.TempDir()
	configDir := filepath.Join(tmpHome, ".asc")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	configPath := filepath.Join(configDir, "config.json")
	if err := os.WriteFile(configPath, []byte(`{"key_id":"OLD","issuer_id":"ISS","private_key_path":"/tmp/old.p8"}`), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	t.Setenv("HOME", tmpHome)
	t.Setenv("ASC_CONFIG_PATH", configPath)

	ascPath := filepath.Join(t.TempDir(), "asc")
	script := "#!/bin/sh\ncat <<'EOF' > \"$ASC_CONFIG_PATH\"\n{\"key_id\":\"NEW\",\"issuer_id\":\"ISS2\",\"private_key_path\":\"/tmp/new.p8\"}\nEOF\n"
	if err := os.WriteFile(ascPath, []byte(script), 0o755); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	app := &App{}
	if _, err := app.runASCCombinedOutput(context.Background(), ascPath, "auth", "status"); err != nil {
		t.Fatalf("runASCCombinedOutput() error = %v", err)
	}

	got, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(got) != `{"key_id":"OLD","issuer_id":"ISS","private_key_path":"/tmp/old.p8"}` {
		t.Fatalf("real config = %q, want original config preserved", string(got))
	}
}

func frontendSectionCommands(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	source := string(data)
	start := strings.Index(source, "export const sectionCommands")
	if start == -1 {
		return nil, errors.New("sectionCommands block not found")
	}
	block := source[start:]
	end := strings.Index(block, "\n};")
	if end == -1 {
		return nil, errors.New("sectionCommands block terminator not found")
	}
	block = block[:end]

	re := regexp.MustCompile(`(?m)^\s*"[^"]+":\s*"([^"]+)"`)
	matches := re.FindAllStringSubmatch(block, -1)
	commands := make([]string, 0, len(matches))
	for _, match := range matches {
		commands = append(commands, match[1])
	}
	return commands, nil
}

func frontendDirectRunASCCommands(paths ...string) ([]string, error) {
	re := regexp.MustCompile("RunASCCommand\\(\\s*`([^`$]+)")
	commands := make([]string, 0, len(paths))

	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		matches := re.FindAllStringSubmatch(string(data), -1)
		for _, match := range matches {
			commands = append(commands, strings.TrimSpace(match[1]))
		}
	}

	return commands, nil
}

func frontendRunASCCommands() ([]string, error) {
	commands, err := frontendSectionCommands(filepath.Join("frontend", "src", "constants.ts"))
	if err != nil {
		return nil, err
	}

	directCommands, err := frontendDirectRunASCCommands(
		filepath.Join("frontend", "src", "hooks", "appSelection", "useAppSectionData.ts"),
		filepath.Join("frontend", "src", "hooks", "useSheetForm.ts"),
	)
	if err != nil {
		return nil, err
	}

	expectedDirectPaths := map[string]struct{}{
		"status":            {},
		"reviews list":      {},
		"insights weekly":   {},
		"bundle-ids create": {},
		"devices register":  {},
	}

	foundDirectPaths := make(map[string]struct{}, len(directCommands))
	for _, command := range directCommands {
		parts, err := parseASCCommandArgs(command)
		if err != nil {
			return nil, err
		}
		foundDirectPaths[studioCommandPath(parts)] = struct{}{}
	}

	for expected := range expectedDirectPaths {
		if _, ok := foundDirectPaths[expected]; !ok {
			return nil, errors.New("frontend direct RunASCCommand path not found: " + expected)
		}
	}

	return append(commands, directCommands...), nil
}

func cliCommandPathExists(root *ffcli.Command, parts []string) bool {
	current := root
	for _, part := range parts {
		if strings.HasPrefix(part, "-") {
			return true
		}

		var next *ffcli.Command
		for _, sub := range current.Subcommands {
			if sub.Name == part {
				next = sub
				break
			}
		}
		if next == nil {
			return false
		}
		current = next
	}
	return true
}

func TestEnsureSessionSingleFlightsConcurrentCalls(t *testing.T) {
	tmp := t.TempDir()
	settingsStore := settings.NewStore(tmp)
	if err := settingsStore.Save(settings.StudioSettings{
		AgentCommand:     "fake-agent",
		WorkspaceRoot:    "/tmp/workspace",
		PreferBundledASC: true,
	}); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	var startCalls atomic.Int32
	started := make(chan struct{}, 1)
	release := make(chan struct{})

	app := &App{
		rootDir:      tmp,
		settings:     settingsStore,
		threads:      threads.NewStore(tmp),
		approvals:    approvals.NewQueue(),
		sessions:     make(map[string]*threadSession),
		sessionInits: make(map[string]*sessionInit),
		startAgent: func(context.Context, acp.LaunchSpec) (agentClient, error) {
			startCalls.Add(1)
			return &fakeAgentClient{
				bootstrapFn: func(context.Context, acp.SessionConfig) (string, error) {
					select {
					case started <- struct{}{}:
					default:
					}
					<-release
					return "session-1", nil
				},
			}, nil
		},
	}

	thread := threads.Thread{ID: "thread-1"}
	results := make(chan *threadSession, 2)
	errorsCh := make(chan error, 2)

	go func() {
		session, err := app.ensureSession(thread)
		errorsCh <- err
		results <- session
	}()

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for first agent bootstrap")
	}

	go func() {
		session, err := app.ensureSession(thread)
		errorsCh <- err
		results <- session
	}()

	close(release)

	var sessions []*threadSession
	for range 2 {
		select {
		case err := <-errorsCh:
			if err != nil {
				t.Fatalf("ensureSession() error = %v", err)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("timed out waiting for ensureSession result")
		}
		select {
		case session := <-results:
			sessions = append(sessions, session)
		case <-time.After(2 * time.Second):
			t.Fatal("timed out waiting for session")
		}
	}

	if got := startCalls.Load(); got != 1 {
		t.Fatalf("startCalls = %d, want 1", got)
	}
	if len(sessions) != 2 || sessions[0] != sessions[1] {
		t.Fatal("ensureSession() did not reuse the same session for concurrent callers")
	}
}

func TestEnsureSessionRecoversAfterPanicDuringInitialization(t *testing.T) {
	tmp := t.TempDir()
	settingsStore := settings.NewStore(tmp)
	if err := settingsStore.Save(settings.StudioSettings{
		AgentCommand:     "fake-agent",
		WorkspaceRoot:    "/tmp/workspace",
		PreferBundledASC: true,
	}); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	var startCalls atomic.Int32
	app := &App{
		rootDir:      tmp,
		settings:     settingsStore,
		threads:      threads.NewStore(tmp),
		approvals:    approvals.NewQueue(),
		sessions:     make(map[string]*threadSession),
		sessionInits: make(map[string]*sessionInit),
		startAgent: func(context.Context, acp.LaunchSpec) (agentClient, error) {
			if startCalls.Add(1) == 1 {
				panic("boom")
			}
			return &fakeAgentClient{}, nil
		},
	}

	thread := threads.Thread{ID: "thread-panic"}

	func() {
		defer func() {
			if recovered := recover(); recovered == nil {
				t.Fatal("ensureSession() did not panic on the first initialization")
			}
		}()
		_, _ = app.ensureSession(thread)
	}()

	done := make(chan *threadSession, 1)
	errCh := make(chan error, 1)
	go func() {
		session, err := app.ensureSession(thread)
		errCh <- err
		done <- session
	}()

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("ensureSession() error after panic recovery = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("ensureSession() deadlocked after panic during initialization")
	}

	select {
	case session := <-done:
		if session == nil {
			t.Fatal("ensureSession() returned nil session after panic recovery")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for recovered session")
	}
}

func TestEnsureSessionSharesInitErrorWithConcurrentWaiters(t *testing.T) {
	tmp := t.TempDir()
	settingsStore := settings.NewStore(tmp)
	if err := settingsStore.Save(settings.StudioSettings{
		AgentCommand:     "fake-agent",
		WorkspaceRoot:    "/tmp/workspace",
		PreferBundledASC: true,
	}); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	started := make(chan struct{}, 1)
	release := make(chan struct{})

	var startCalls atomic.Int32
	app := &App{
		rootDir:      tmp,
		settings:     settingsStore,
		threads:      threads.NewStore(tmp),
		approvals:    approvals.NewQueue(),
		sessions:     make(map[string]*threadSession),
		sessionInits: make(map[string]*sessionInit),
		startAgent: func(context.Context, acp.LaunchSpec) (agentClient, error) {
			call := startCalls.Add(1)
			return &fakeAgentClient{
				bootstrapFn: func(context.Context, acp.SessionConfig) (string, error) {
					if call == 1 {
						select {
						case started <- struct{}{}:
						default:
						}
						<-release
						return "", errors.New("bootstrap failed")
					}
					return "session-2", nil
				},
			}, nil
		},
	}

	thread := threads.Thread{ID: "thread-error"}
	errCh := make(chan error, 2)

	go func() {
		_, err := app.ensureSession(thread)
		errCh <- err
	}()

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for first agent bootstrap")
	}

	go func() {
		_, err := app.ensureSession(thread)
		errCh <- err
	}()

	deadline := time.Now().Add(2 * time.Second)
	for {
		app.mu.Lock()
		init := app.sessionInits[thread.ID]
		waiters := 0
		if init != nil {
			waiters = init.waiters
		}
		app.mu.Unlock()
		if waiters == 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for concurrent ensureSession waiter")
		}
		time.Sleep(10 * time.Millisecond)
	}

	close(release)

	for range 2 {
		select {
		case err := <-errCh:
			if err == nil || err.Error() != "bootstrap failed" {
				t.Fatalf("ensureSession() error = %v, want bootstrap failed", err)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("timed out waiting for ensureSession error")
		}
	}

	if got := startCalls.Load(); got != 1 {
		t.Fatalf("startCalls after shared error = %d, want 1", got)
	}

	session, err := app.ensureSession(thread)
	if err != nil {
		t.Fatalf("ensureSession() retry error = %v", err)
	}
	if session == nil {
		t.Fatal("ensureSession() retry returned nil session")
	}
	if got := startCalls.Load(); got != 2 {
		t.Fatalf("startCalls after retry = %d, want 2", got)
	}
}

func TestStartThreadSessionTimesOutBootstrap(t *testing.T) {
	tmp := t.TempDir()
	settingsStore := settings.NewStore(tmp)
	if err := settingsStore.Save(settings.StudioSettings{
		AgentCommand:     "fake-agent",
		WorkspaceRoot:    "/tmp/workspace",
		PreferBundledASC: true,
	}); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	originalTimeout := sessionInitTimeout
	sessionInitTimeout = 20 * time.Millisecond
	t.Cleanup(func() {
		sessionInitTimeout = originalTimeout
	})

	client := &fakeAgentClient{
		bootstrapFn: func(ctx context.Context, cfg acp.SessionConfig) (string, error) {
			<-ctx.Done()
			return "", ctx.Err()
		},
	}

	app := &App{
		rootDir:      tmp,
		settings:     settingsStore,
		threads:      threads.NewStore(tmp),
		approvals:    approvals.NewQueue(),
		sessions:     make(map[string]*threadSession),
		sessionInits: make(map[string]*sessionInit),
		startAgent: func(context.Context, acp.LaunchSpec) (agentClient, error) {
			return client, nil
		},
	}

	_, err := app.startThreadSession(threads.Thread{ID: "thread-timeout"})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("startThreadSession() error = %v, want deadline exceeded", err)
	}
}

func TestGetAppDetailReturnsVersionsError(t *testing.T) {
	tmp := t.TempDir()
	fakeASC := filepath.Join(tmp, "asc")
	script := `#!/bin/sh
set -eu
if [ "$1" = "apps" ] && [ "$2" = "view" ]; then
  printf '%s\n' '{"data":{"attributes":{"name":"Test App","bundleId":"com.example.test","sku":"SKU","primaryLocale":"en-US"}}}'
  exit 0
fi
if [ "$1" = "versions" ] && [ "$2" = "list" ]; then
  printf '%s\n' 'versions failed' >&2
  exit 1
fi
if [ "$1" = "apps" ] && [ "$2" = "subtitle" ]; then
  printf '%s\n' ''
  exit 0
fi
printf '%s\n' "unexpected command: $*" >&2
exit 1
`
	if err := os.WriteFile(fakeASC, []byte(script), 0o755); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	settingsStore := settings.NewStore(tmp)
	if err := settingsStore.Save(settings.StudioSettings{
		SystemASCPath:    fakeASC,
		PreferBundledASC: false,
	}); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	app := &App{
		rootDir:   tmp,
		settings:  settingsStore,
		threads:   threads.NewStore(tmp),
		approvals: approvals.NewQueue(),
	}

	got, err := app.GetAppDetail("1")
	if err != nil {
		t.Fatalf("GetAppDetail() error = %v", err)
	}
	if got.Error != "versions failed" {
		t.Fatalf("GetAppDetail().Error = %q, want versions failed", got.Error)
	}
}

type fakeAgentClient struct {
	bootstrapFn func(context.Context, acp.SessionConfig) (string, error)
	promptFn    func(context.Context, string, string) (acp.PromptResult, []acp.Event, error)
	closeFn     func() error
}

func (f *fakeAgentClient) Bootstrap(ctx context.Context, cfg acp.SessionConfig) (string, error) {
	if f.bootstrapFn != nil {
		return f.bootstrapFn(ctx, cfg)
	}
	return "session-1", nil
}

func (f *fakeAgentClient) Prompt(ctx context.Context, sessionID string, prompt string) (acp.PromptResult, []acp.Event, error) {
	if f.promptFn != nil {
		return f.promptFn(ctx, sessionID, prompt)
	}
	return acp.PromptResult{Status: "completed"}, nil, nil
}

func (f *fakeAgentClient) Close() error {
	if f.closeFn != nil {
		return f.closeFn()
	}
	return nil
}
