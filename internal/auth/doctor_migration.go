package auth

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type DoctorMigrationHints struct {
	DetectedFiles     []string `json:"detectedFiles"`
	DetectedActions   []string `json:"detectedActions"`
	SuggestedCommands []string `json:"suggestedCommands"`
}

type appfileSignal struct {
	path string
	keys []string
}

type fastfileSignal struct {
	path    string
	actions []string
}

type migrationSignals struct {
	root            string
	appfiles        []appfileSignal
	fastfiles       []fastfileSignal
	deliverfiles    []string
	bundlerFiles    []string
	detectedFiles   []string
	detectedActions []string
	fastlaneDir     string
}

func inspectMigrationHints() (DoctorSection, *DoctorMigrationHints) {
	section := DoctorSection{Title: "Migration Hints"}

	root, err := resolveMigrationRoot()
	if err != nil {
		section.Checks = append(section.Checks, DoctorCheck{
			Status:  DoctorInfo,
			Message: fmt.Sprintf("Migration scan skipped: %v", err),
		})
		return section, &DoctorMigrationHints{}
	}

	signals := scanMigrationSignals(root)
	suggestions := buildSuggestedCommands(signals)
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
	for i := 0; i < 8; i++ {
		if isDirectory(filepath.Join(dir, ".git")) {
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
				keys := extractAppfileKeys(path)
				signals.appfiles = append(signals.appfiles, appfileSignal{path: rel, keys: keys})
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

func extractAppfileKeys(path string) []string {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	found := map[string]struct{}{}
	keys := appfileKeyOrder()
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		for _, key := range keys {
			if strings.HasPrefix(line, key) {
				found[key] = struct{}{}
			}
		}
	}
	var ordered []string
	for _, key := range keys {
		if _, ok := found[key]; ok {
			ordered = append(ordered, key)
		}
	}
	return ordered
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

func buildMigrationHints(signals migrationSignals, suggestions []string) *DoctorMigrationHints {
	return &DoctorMigrationHints{
		DetectedFiles:     append([]string{}, signals.detectedFiles...),
		DetectedActions:   append([]string{}, signals.detectedActions...),
		SuggestedCommands: append([]string{}, suggestions...),
	}
}

func buildSuggestedCommands(signals migrationSignals) []string {
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

	if hasAuthSignal {
		add(`asc auth login --name "MyKey" --key-id "KEY_ID" --issuer-id "ISSUER_ID" --private-key /path/to/AuthKey.p8`)
	}
	if hasMetadataSignal {
		fastlaneDir := formatFastlaneDir(signals.fastlaneDir)
		add(fmt.Sprintf("asc migrate validate --fastlane-dir %s", fastlaneDir))
		add(fmt.Sprintf(`asc migrate import --app "APP_ID" --version-id "VERSION_ID" --fastlane-dir %s`, fastlaneDir))
	}
	if hasBuildSignal {
		add(`asc builds latest --app "APP_ID"`)
	}
	if hasTestflightSignal {
		add(`asc publish testflight --app "APP_ID" --ipa app.ipa --group "GROUP_ID"`)
	}
	if hasAppStoreSignal {
		add(`asc publish appstore --app "APP_ID" --ipa app.ipa --version "1.2.3" --submit --confirm`)
		add(`asc submit create --app "APP_ID" --version "1.2.3" --build "BUILD_ID" --confirm`)
	}

	return commands
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

func isDirectory(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}
