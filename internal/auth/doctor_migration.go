package auth

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/config"
)

type DoctorMigrationHints struct {
	DetectedFiles     []string `json:"detectedFiles"`
	DetectedActions   []string `json:"detectedActions"`
	SuggestedCommands []string `json:"suggestedCommands"`
}

type MigrationSuggestionResolverInput struct {
	AppIdentifier    string
	AppID            string
	MarketingVersion string
	NeedAppID        bool
	NeedVersionID    bool
	NeedBuildID      bool
}

type MigrationSuggestionResolverOutput struct {
	AppID     string
	VersionID string
	BuildID   string
}

type MigrationSuggestionResolver func(input MigrationSuggestionResolverInput) MigrationSuggestionResolverOutput

type appfileSignal struct {
	path          string
	keys          []string
	appIdentifier string
}

type fastfileSignal struct {
	path    string
	actions []string
}

type migrationSignals struct {
	root                      string
	appfiles                  []appfileSignal
	fastfiles                 []fastfileSignal
	deliverfiles              []string
	bundlerFiles              []string
	detectedFiles             []string
	detectedActions           []string
	fastlaneDir               string
	appIdentifier             string
	marketingVersion          string
	marketingVersionSource    string
	marketingVersionAmbiguous bool
}

func inspectMigrationHints(resolver MigrationSuggestionResolver) (DoctorSection, *DoctorMigrationHints) {
	section := DoctorSection{Title: "Migration Hints"}

	root, err := resolveMigrationRoot()
	if err != nil {
		section.Checks = append(section.Checks, DoctorCheck{
			Status:  DoctorInfo,
			Message: fmt.Sprintf("Migration scan skipped: %v", err),
		})
		return section, buildMigrationHints(migrationSignals{}, nil)
	}

	signals := scanMigrationSignals(root)
	suggestions := buildSuggestedCommands(signals, resolver)
	section.Checks = append(section.Checks, buildMigrationChecks(signals, suggestions)...)
	hints := buildMigrationHints(signals, suggestions)
	return section, hints
}

func resolveMigrationRoot() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	root := findRepoRoot(wd)
	return root, nil
}

func findRepoRoot(start string) string {
	dir := start
	for {
		if hasGitMarker(dir) {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return start
}

func scanMigrationSignals(root string) migrationSignals {
	signals := migrationSignals{root: root}
	seenFiles := map[string]struct{}{}
	fastfileActions := map[string]struct{}{}

	for _, candidate := range migrationSearchPaths() {
		for _, name := range fastlaneFileNames() {
			path := filepath.Join(root, candidate, name)
			if !isFile(path) {
				continue
			}
			rel := relativePath(root, path)
			if _, ok := seenFiles[rel]; ok {
				continue
			}
			seenFiles[rel] = struct{}{}
			signals.detectedFiles = append(signals.detectedFiles, rel)

			switch name {
			case "Appfile":
				appfile := extractAppfileSignal(path)
				appfile.path = rel
				signals.appfiles = append(signals.appfiles, appfile)
				if signals.appIdentifier == "" {
					signals.appIdentifier = strings.TrimSpace(appfile.appIdentifier)
				}
			case "Fastfile":
				actions := extractFastfileActions(path)
				signals.fastfiles = append(signals.fastfiles, fastfileSignal{path: rel, actions: actions})
				for _, action := range actions {
					fastfileActions[action] = struct{}{}
				}
			case "Deliverfile":
				signals.deliverfiles = append(signals.deliverfiles, rel)
			}
		}
	}

	for _, bundlerFile := range []string{"Gemfile", "Gemfile.lock"} {
		path := filepath.Join(root, bundlerFile)
		if !isFile(path) {
			continue
		}
		rel := relativePath(root, path)
		if _, ok := seenFiles[rel]; ok {
			continue
		}
		seenFiles[rel] = struct{}{}
		signals.bundlerFiles = append(signals.bundlerFiles, rel)
		signals.detectedFiles = append(signals.detectedFiles, rel)
	}

	signals.detectedActions = orderDetectedActions(fastfileActions)
	signals.fastlaneDir = resolveFastlaneDir(signals)
	signals.marketingVersion, signals.marketingVersionSource, signals.marketingVersionAmbiguous = detectMarketingVersion(root)
	return signals
}

func migrationSearchPaths() []string {
	return []string{
		".",
		"fastlane",
		".fastlane",
		filepath.Join("ios", "fastlane"),
		filepath.Join("android", "fastlane"),
	}
}

func fastlaneFileNames() []string {
	return []string{"Appfile", "Fastfile", "Deliverfile"}
}

var (
	appfileIdentifierRegex = regexp.MustCompile(`^\s*app_identifier\s*(?:\(\s*)?["']([^"']+)["']`)
	marketingVersionRegex  = regexp.MustCompile(`\bMARKETING_VERSION\s*=\s*([^;]+);`)
)

func extractAppfileSignal(path string) appfileSignal {
	data, err := os.ReadFile(path)
	if err != nil {
		return appfileSignal{}
	}

	found := map[string]struct{}{}
	keys := appfileKeyOrder()
	appIdentifier := ""
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		line := stripFastlaneComment(scanner.Text())
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		for _, key := range keys {
			if strings.HasPrefix(line, key) {
				found[key] = struct{}{}
			}
		}
		if appIdentifier == "" {
			if match := appfileIdentifierRegex.FindStringSubmatch(line); len(match) > 1 {
				appIdentifier = strings.TrimSpace(match[1])
			}
		}
	}

	var ordered []string
	for _, key := range keys {
		if _, ok := found[key]; ok {
			ordered = append(ordered, key)
		}
	}
	return appfileSignal{keys: ordered, appIdentifier: appIdentifier}
}

func appfileKeyOrder() []string {
	return []string{
		"app_identifier",
		"apple_id",
		"team_id",
		"itc_team_id",
		"apple_dev_portal_id",
		"itunes_connect_id",
	}
}

func extractFastfileActions(path string) []string {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	found := map[string]struct{}{}
	actionOrder := fastfileActionOrder()
	actionRegex := fastfileActionRegexes()

	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		line := stripFastlaneComment(scanner.Text())
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		for _, action := range actionOrder {
			if actionRegex[action].MatchString(line) {
				found[action] = struct{}{}
			}
		}
	}

	var ordered []string
	for _, action := range actionOrder {
		if _, ok := found[action]; ok {
			ordered = append(ordered, action)
		}
	}
	return ordered
}

func fastfileActionOrder() []string {
	return []string{
		"app_store_connect_api_key",
		"deliver",
		"upload_to_testflight",
		"pilot",
		"upload_to_app_store",
		"precheck",
		"app_store_build_number",
		"latest_testflight_build_number",
	}
}

func fastfileActionRegexes() map[string]*regexp.Regexp {
	regexes := make(map[string]*regexp.Regexp)
	for _, action := range fastfileActionOrder() {
		regexes[action] = regexp.MustCompile(fmt.Sprintf(`\b%s\b`, regexp.QuoteMeta(action)))
	}
	return regexes
}

func stripFastlaneComment(line string) string {
	trimmed := strings.TrimSpace(line)
	if strings.HasPrefix(trimmed, "#") {
		return ""
	}
	if idx := strings.Index(line, "#"); idx != -1 {
		return line[:idx]
	}
	return line
}

func orderDetectedActions(found map[string]struct{}) []string {
	var ordered []string
	for _, action := range fastfileActionOrder() {
		if _, ok := found[action]; ok {
			ordered = append(ordered, action)
		}
	}
	return ordered
}

func resolveFastlaneDir(signals migrationSignals) string {
	var candidates []string
	for _, appfile := range signals.appfiles {
		candidates = append(candidates, appfile.path)
	}
	for _, fastfile := range signals.fastfiles {
		candidates = append(candidates, fastfile.path)
	}
	candidates = append(candidates, signals.deliverfiles...)
	for _, path := range candidates {
		dir := filepath.Dir(path)
		if dir == "" {
			continue
		}
		return dir
	}
	return ""
}

func detectMarketingVersion(root string) (version string, source string, ambiguous bool) {
	pbxprojFiles := discoverPbxprojFiles(root)
	if len(pbxprojFiles) == 0 {
		return "", "", false
	}

	counts := map[string]int{}
	firstSource := map[string]string{}
	for _, relPath := range pbxprojFiles {
		absPath := filepath.Join(root, filepath.FromSlash(relPath))
		versions := extractMarketingVersions(absPath)
		for _, value := range versions {
			counts[value]++
			if _, ok := firstSource[value]; !ok {
				firstSource[value] = relPath
			}
		}
	}
	if len(counts) == 0 {
		return "", "", false
	}
	if len(counts) == 1 {
		for value := range counts {
			return value, firstSource[value], false
		}
	}

	type candidate struct {
		version string
		count   int
	}
	candidates := make([]candidate, 0, len(counts))
	for value, count := range counts {
		candidates = append(candidates, candidate{version: value, count: count})
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].count == candidates[j].count {
			return candidates[i].version < candidates[j].version
		}
		return candidates[i].count > candidates[j].count
	})
	if len(candidates) > 1 && candidates[0].count == candidates[1].count {
		return "", "", true
	}

	best := candidates[0].version
	return best, firstSource[best], false
}

func discoverPbxprojFiles(root string) []string {
	seen := map[string]struct{}{}
	var files []string

	scanDirs := []string{root}
	entries, err := os.ReadDir(root)
	if err == nil {
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			// Bounded scan: root + first-level directories.
			scanDirs = append(scanDirs, filepath.Join(root, entry.Name()))
		}
	}

	for _, dir := range scanDirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if !entry.IsDir() || !strings.HasSuffix(entry.Name(), ".xcodeproj") {
				continue
			}
			pbxprojPath := filepath.Join(dir, entry.Name(), "project.pbxproj")
			if !isFile(pbxprojPath) {
				continue
			}
			rel := relativePath(root, pbxprojPath)
			if _, ok := seen[rel]; ok {
				continue
			}
			seen[rel] = struct{}{}
			files = append(files, rel)
		}
	}

	sort.Strings(files)
	return files
}

func extractMarketingVersions(path string) []string {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	matches := marketingVersionRegex.FindAllStringSubmatch(string(data), -1)
	versions := make([]string, 0, len(matches))
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		value := strings.TrimSpace(match[1])
		value = strings.Trim(value, `"'`)
		if value == "" {
			continue
		}
		// Skip unresolved build-setting references like $(MARKETING_VERSION).
		if strings.Contains(value, "$(") {
			continue
		}
		versions = append(versions, value)
	}
	return versions
}

func buildMigrationChecks(signals migrationSignals, suggestions []string) []DoctorCheck {
	var checks []DoctorCheck

	for _, appfile := range signals.appfiles {
		message := fmt.Sprintf("Detected Appfile at %s", appfile.path)
		if len(appfile.keys) > 0 {
			message = fmt.Sprintf("%s (keys: %s)", message, strings.Join(appfile.keys, ", "))
		}
		checks = append(checks, DoctorCheck{Status: DoctorInfo, Message: message})
	}
	for _, fastfile := range signals.fastfiles {
		message := fmt.Sprintf("Detected Fastfile at %s", fastfile.path)
		if len(fastfile.actions) > 0 {
			message = fmt.Sprintf("%s (actions: %s)", message, strings.Join(fastfile.actions, ", "))
		}
		checks = append(checks, DoctorCheck{Status: DoctorInfo, Message: message})
	}
	for _, deliverfile := range signals.deliverfiles {
		checks = append(checks, DoctorCheck{
			Status:  DoctorInfo,
			Message: fmt.Sprintf("Detected Deliverfile at %s", deliverfile),
		})
	}
	for _, bundler := range signals.bundlerFiles {
		checks = append(checks, DoctorCheck{
			Status:  DoctorInfo,
			Message: fmt.Sprintf("Detected %s", bundler),
		})
	}
	if signals.marketingVersion != "" {
		if signals.marketingVersionSource != "" {
			checks = append(checks, DoctorCheck{
				Status:  DoctorInfo,
				Message: fmt.Sprintf("Detected MARKETING_VERSION %q from %s", signals.marketingVersion, signals.marketingVersionSource),
			})
		} else {
			checks = append(checks, DoctorCheck{
				Status:  DoctorInfo,
				Message: fmt.Sprintf("Detected MARKETING_VERSION %q from Xcode project settings", signals.marketingVersion),
			})
		}
	} else if signals.marketingVersionAmbiguous {
		checks = append(checks, DoctorCheck{
			Status:  DoctorInfo,
			Message: "Multiple MARKETING_VERSION values detected; keeping generic version placeholders",
		})
	}

	if len(signals.appfiles) == 0 && len(signals.fastfiles) == 0 && len(signals.deliverfiles) == 0 {
		checks = append(checks, DoctorCheck{
			Status:  DoctorInfo,
			Message: "No Appfile/Fastfile/Deliverfile detected in common fastlane locations",
		})
	}

	if len(suggestions) == 0 {
		if len(signals.bundlerFiles) > 0 {
			checks = append(checks, DoctorCheck{
				Status:  DoctorInfo,
				Message: "No asc command suggestions matched detected Bundler files",
			})
		}
		return checks
	}
	for _, cmd := range suggestions {
		checks = append(checks, DoctorCheck{
			Status:  DoctorInfo,
			Message: fmt.Sprintf("Suggested: %s", cmd),
		})
	}
	return checks
}

func buildMigrationHints(signals migrationSignals, suggestedCommands []string) *DoctorMigrationHints {
	detectedFiles := append([]string{}, signals.detectedFiles...)
	if detectedFiles == nil {
		detectedFiles = []string{}
	}
	detectedActions := append([]string{}, signals.detectedActions...)
	if detectedActions == nil {
		detectedActions = []string{}
	}
	if suggestedCommands == nil {
		suggestedCommands = []string{}
	}

	return &DoctorMigrationHints{
		DetectedFiles:     detectedFiles,
		DetectedActions:   detectedActions,
		SuggestedCommands: suggestedCommands,
	}
}

type migrationCommandValues struct {
	appID         string
	versionString string
	versionID     string
	buildID       string
}

func buildSuggestedCommands(signals migrationSignals, resolver MigrationSuggestionResolver) []string {
	var commands []string
	seen := map[string]struct{}{}
	add := func(cmd string) {
		if _, ok := seen[cmd]; ok {
			return
		}
		seen[cmd] = struct{}{}
		commands = append(commands, cmd)
	}

	hasAuthSignal := containsAction(signals.detectedActions, "app_store_connect_api_key")
	hasMetadataSignal := len(signals.appfiles) > 0 || len(signals.deliverfiles) > 0 || containsAction(signals.detectedActions, "deliver")
	hasBuildSignal := containsAction(signals.detectedActions, "app_store_build_number") ||
		containsAction(signals.detectedActions, "latest_testflight_build_number")
	hasTestflightSignal := containsAction(signals.detectedActions, "upload_to_testflight") || containsAction(signals.detectedActions, "pilot")
	hasAppStoreSignal := containsAction(signals.detectedActions, "upload_to_app_store") || containsAction(signals.detectedActions, "precheck")
	needsAppID := hasMetadataSignal || hasBuildSignal || hasTestflightSignal || hasAppStoreSignal
	needsVersionString := hasAppStoreSignal
	needsVersionID := hasMetadataSignal
	needsBuildID := hasAppStoreSignal
	values := resolveMigrationCommandValues(signals, resolver, needsAppID, needsVersionString, needsVersionID, needsBuildID)
	values = fallbackMigrationCommandValues(values)

	if hasAuthSignal {
		add(`asc auth login --name "MyKey" --key-id "KEY_ID" --issuer-id "ISSUER_ID" --private-key /path/to/AuthKey.p8`)
	}
	if hasMetadataSignal {
		fastlaneDir := formatFastlaneDir(signals.fastlaneDir)
		add(fmt.Sprintf("asc migrate validate --fastlane-dir %s", fastlaneDir))
		add(fmt.Sprintf(`asc migrate import --app %q --version-id %q --fastlane-dir %s`, values.appID, values.versionID, fastlaneDir))
	}
	if hasBuildSignal {
		add(fmt.Sprintf(`asc builds info --app %q --latest`, values.appID))
	}
	if hasTestflightSignal {
		add(fmt.Sprintf(`asc publish testflight --app %q --ipa app.ipa --group "GROUP_ID"`, values.appID))
	}
	if hasAppStoreSignal {
		add(fmt.Sprintf(`asc publish appstore --app %q --ipa app.ipa --version %q --submit --confirm`, values.appID, values.versionString))
		add(fmt.Sprintf(`asc submit create --app %q --version %q --build %q --confirm`, values.appID, values.versionString, values.buildID))
	}

	return commands
}

func resolveMigrationCommandValues(
	signals migrationSignals,
	resolver MigrationSuggestionResolver,
	needsAppID bool,
	needsVersionString bool,
	needsVersionID bool,
	needsBuildID bool,
) migrationCommandValues {
	values := migrationCommandValues{}
	if needsAppID {
		values.appID = resolveLocalAppID()
	}
	if needsVersionString || needsVersionID || needsBuildID {
		values.versionString = strings.TrimSpace(signals.marketingVersion)
	}

	if resolver != nil && (strings.TrimSpace(values.appID) == "" || needsVersionID || needsBuildID) {
		result := resolver(MigrationSuggestionResolverInput{
			AppIdentifier:    signals.appIdentifier,
			AppID:            values.appID,
			MarketingVersion: values.versionString,
			NeedAppID:        needsAppID,
			NeedVersionID:    needsVersionID,
			NeedBuildID:      needsBuildID,
		})
		if appID := strings.TrimSpace(result.AppID); appID != "" {
			values.appID = appID
		}
		if versionID := strings.TrimSpace(result.VersionID); versionID != "" {
			values.versionID = versionID
		}
		if buildID := strings.TrimSpace(result.BuildID); buildID != "" {
			values.buildID = buildID
		}
	}

	return values
}

func fallbackMigrationCommandValues(values migrationCommandValues) migrationCommandValues {
	if strings.TrimSpace(values.appID) == "" {
		values.appID = "APP_ID"
	}
	if strings.TrimSpace(values.versionString) == "" {
		values.versionString = "1.2.3"
	}
	if strings.TrimSpace(values.versionID) == "" {
		values.versionID = "VERSION_ID"
	}
	if strings.TrimSpace(values.buildID) == "" {
		values.buildID = "BUILD_ID"
	}
	return values
}

func resolveLocalAppID() string {
	if env := strings.TrimSpace(os.Getenv("ASC_APP_ID")); env != "" {
		return env
	}

	configPath, err := config.Path()
	if err != nil {
		return ""
	}
	cfg, err := config.LoadAt(configPath)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(cfg.AppID)
}

func formatFastlaneDir(dir string) string {
	if dir == "" || dir == "." {
		return "."
	}
	if strings.HasPrefix(dir, "./") {
		return dir
	}
	return "./" + dir
}

func containsAction(actions []string, target string) bool {
	for _, action := range actions {
		if action == target {
			return true
		}
	}
	return false
}

func relativePath(root, path string) string {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return filepath.ToSlash(path)
	}
	return filepath.ToSlash(rel)
}

func isFile(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

func hasGitMarker(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, ".git"))
	return err == nil
}
