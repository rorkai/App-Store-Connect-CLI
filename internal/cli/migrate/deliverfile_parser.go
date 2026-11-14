package migrate

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

type DeliverfileConfig struct {
	MetadataPath    string
	ScreenshotsPath string
	AppIdentifier   string
	AppVersion      string
	Platform        string
	SkipMetadata    bool
	SkipScreenshots bool
	IgnoredKeys     []IgnoredConfigKey
}

// IgnoredConfigKey is a configuration file key that import does not apply.
type IgnoredConfigKey struct {
	Key   string
	Value string
	Line  int
}

func parseDeliverfile(path string) (DeliverfileConfig, error) {
	file, err := os.Open(path)
	if err != nil {
		return DeliverfileConfig{}, err
	}
	defer file.Close()
	return parseDeliverfileReader(path, file)
}

func parseDeliverfileReader(path string, reader io.Reader) (DeliverfileConfig, error) {
	var config DeliverfileConfig
	scanner := bufio.NewScanner(reader)
	lineNumber := 0
	heredocEnd := ""
	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSpace(scanner.Text())
		if heredocEnd != "" {
			if line == heredocEnd {
				heredocEnd = ""
			}
			continue
		}
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = stripDeliverfileComment(line)
		if strings.TrimSpace(line) == "" {
			continue
		}
		heredocEnd = heredocTerminator(line)
		key, value, ok := parseDeliverfileLine(line)
		if !ok {
			continue
		}
		switch key {
		case "metadata_path":
			if value == "" {
				return DeliverfileConfig{}, deliverfileLineError(path, lineNumber, "metadata_path requires a value")
			}
			config.MetadataPath = value
		case "screenshots_path":
			if value == "" {
				return DeliverfileConfig{}, deliverfileLineError(path, lineNumber, "screenshots_path requires a value")
			}
			config.ScreenshotsPath = value
		case "app_identifier":
			if value == "" {
				return DeliverfileConfig{}, deliverfileLineError(path, lineNumber, "app_identifier requires a value")
			}
			config.AppIdentifier = value
		case "app_version":
			if value == "" {
				return DeliverfileConfig{}, deliverfileLineError(path, lineNumber, "app_version requires a value")
			}
			config.AppVersion = value
		case "platform":
			if value == "" {
				return DeliverfileConfig{}, deliverfileLineError(path, lineNumber, "platform requires a value")
			}
			config.Platform = value
		case "skip_metadata":
			parsed, err := parseDeliverfileBool(path, lineNumber, "skip_metadata", value)
			if err != nil {
				return DeliverfileConfig{}, err
			}
			config.SkipMetadata = parsed
		case "skip_screenshots":
			parsed, err := parseDeliverfileBool(path, lineNumber, "skip_screenshots", value)
			if err != nil {
				return DeliverfileConfig{}, err
			}
			config.SkipScreenshots = parsed
		default:
			if slices.Contains(unappliedConfigKeys, key) && isConfigOptionCall(strings.TrimPrefix(strings.TrimSpace(line), key)) {
				config.IgnoredKeys = append(config.IgnoredKeys, IgnoredConfigKey{Key: key, Value: value, Line: lineNumber})
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return DeliverfileConfig{}, fmt.Errorf("deliverfile read error %s: %w", filepath.Base(path), err)
	}
	return config, nil
}

func parseDeliverfileLine(line string) (string, string, bool) {
	key, rest := splitDeliverfileKey(line)
	if key == "" {
		return "", "", false
	}
	value := parseDeliverfileValue(rest)
	return key, value, true
}

func splitDeliverfileKey(line string) (string, string) {
	line = strings.TrimSpace(line)
	if line == "" {
		return "", ""
	}
	var keyBuilder strings.Builder
	for _, ch := range line {
		if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '_' {
			keyBuilder.WriteRune(ch)
			continue
		}
		break
	}
	key := keyBuilder.String()
	if key == "" {
		return "", ""
	}
	rest := strings.TrimSpace(line[len(key):])
	if strings.HasPrefix(rest, "=") {
		rest = strings.TrimSpace(strings.TrimPrefix(rest, "="))
	}
	if strings.HasPrefix(rest, "(") && strings.HasSuffix(rest, ")") {
		rest = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(rest, "("), ")"))
	}
	return key, rest
}

func parseDeliverfileValue(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if strings.HasPrefix(value, "\"") {
		return readQuotedDeliverfileValue(value, '"')
	}
	if strings.HasPrefix(value, "'") {
		return readQuotedDeliverfileValue(value, '\'')
	}
	fields := strings.Fields(value)
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

func readQuotedDeliverfileValue(value string, quote rune) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	if rune(trimmed[0]) != quote {
		return ""
	}
	var b strings.Builder
	escaped := false
	for i, ch := range trimmed[1:] {
		if escaped {
			b.WriteRune(ch)
			escaped = false
			continue
		}
		if ch == '\\' {
			escaped = true
			continue
		}
		if ch == quote {
			_ = i
			return b.String()
		}
		b.WriteRune(ch)
	}
	return b.String()
}

func stripDeliverfileComment(line string) string {
	var b strings.Builder
	inSingle := false
	inDouble := false
	escaped := false
	for _, ch := range line {
		if escaped {
			b.WriteRune(ch)
			escaped = false
			continue
		}
		if ch == '\\' {
			escaped = true
			b.WriteRune(ch)
			continue
		}
		switch ch {
		case '\'':
			if !inDouble {
				inSingle = !inSingle
			}
		case '"':
			if !inSingle {
				inDouble = !inDouble
			}
		case '#':
			if !inSingle && !inDouble {
				return strings.TrimSpace(b.String())
			}
		}
		b.WriteRune(ch)
	}
	return strings.TrimSpace(b.String())
}

func parseDeliverfileBool(path string, line int, key, value string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "true":
		return true, nil
	case "false":
		return false, nil
	default:
		return false, deliverfileLineError(path, line, fmt.Sprintf("%s must be true or false", key))
	}
}

func deliverfileLineError(path string, line int, message string) error {
	return fmt.Errorf("deliverfile %s line %d: %s", filepath.Base(path), line, message)
}

// unappliedConfigKeys are the configuration file options that import
// recognises and does not apply. Any other line is ordinary script syntax
// (a condition, an assignment, a field inside a multi-line value) and is not
// reported.
var unappliedConfigKeys = []string{
	"api_key_path", "api_key", "username", "ipa", "pkg", "build_number", "edit_live", "use_live_version",
	"app_clip_header_images_path", "app_clip_default_experience_metadata_path", "app_previews_path",
	"preview_frame_time_code", "overwrite_preview_videos", "skip_binary_upload", "skip_app_version_update",
	"force", "overwrite_screenshots", "screenshot_processing_timeout", "sync_screenshots", "submit_for_review",
	"verify_only", "reject_if_possible", "version_check_wait_retry_limit", "automatic_release",
	"auto_release_date", "phased_release", "reset_ratings", "price_tier", "app_rating_config_path",
	"submission_information", "team_id", "team_name", "dev_portal_team_id", "dev_portal_team_name",
	"itc_provider", "provider_public_id", "run_precheck_before_submit", "precheck_default_rule_level",
	"individual_metadata_items", "app_icon", "apple_watch_app_icon", "copyright", "primary_category",
	"secondary_category", "primary_first_sub_category", "primary_second_sub_category",
	"secondary_first_sub_category", "secondary_second_sub_category", "trade_representative_contact_information",
	"app_review_information", "app_clip_review_information", "app_review_attachment_file",
	"routing_app_coverage_file", "description", "name", "subtitle", "keywords", "promotional_text",
	"release_notes", "privacy_url", "apple_tv_privacy_policy", "support_url", "marketing_url", "languages",
	"ignore_language_directory_validation", "precheck_include_in_app_purchases",
	"app_clip_default_experience_subtitle", "app_clip_default_experience_action",
}

// heredocStart matches the marker that opens a multi-line text, such as
// "<<~TEXT". The lines up to the closing word are text, not options.
var heredocStart = regexp.MustCompile(`^<<[~-]?["']?([A-Za-z_][A-Za-z0-9_]*)["']?`)

// heredocTerminator returns the closing word of a multi-line text that line
// opens, or "". A marker inside a quoted string does not count.
func heredocTerminator(line string) string {
	var quote byte
	for i := 0; i < len(line); i++ {
		switch c := line[i]; {
		case quote != 0 && c == '\\':
			i++
		case quote != 0:
			if c == quote {
				quote = 0
			}
		case c == '"' || c == '\'':
			quote = c
		case c == '<':
			if match := heredocStart.FindStringSubmatch(line[i:]); match != nil {
				return match[1]
			}
		}
	}
	return ""
}

// isConfigOptionCall reports whether rest, the text that follows an option
// name, sets the option: "name value" or "name(value)". Text that starts with
// an operator ("name = x", "name ||= x", "name += 1") is an assignment or an
// expression, and "name: x" is a field inside a multi-line value.
func isConfigOptionCall(rest string) bool {
	if rest == "" || rest[0] == '(' {
		return true
	}
	if rest[0] != ' ' && rest[0] != '\t' {
		return false
	}
	value := strings.TrimSpace(rest)
	if value == "" {
		return true
	}
	switch first := value[0]; {
	case strings.ContainsRune("\"'[{:(_", rune(first)):
		return true
	case first >= '0' && first <= '9', first >= 'a' && first <= 'z', first >= 'A' && first <= 'Z':
		return true
	case first == '-' && len(value) > 1 && value[1] >= '0' && value[1] <= '9':
		return true
	}
	return heredocStart.MatchString(value)
}

func ignoredConfigKeyItems(path string, keys []IgnoredConfigKey) []SkippedItem {
	items := make([]SkippedItem, 0, len(keys))
	scheduled := slices.ContainsFunc(keys, func(key IgnoredConfigKey) bool { return key.Key == "auto_release_date" })
	for _, ignored := range keys {
		reason := fmt.Sprintf("line %d: %s is not applied by import", ignored.Line, ignored.Key)
		if hint := ignoredConfigKeyHint(ignored, scheduled); hint != "" {
			reason += "; to apply it, run: " + hint
		}
		items = append(items, SkippedItem{Path: path, Reason: reason})
	}
	return items
}

// ignoredConfigKeyHint returns the command that applies an ignored option, or
// "" when no single command does. scheduled means the file also sets a release
// date, which decides the release type, so automatic_release gets no hint.
func ignoredConfigKeyHint(ignored IgnoredConfigKey, scheduled bool) string {
	value := strings.ToLower(strings.TrimSpace(ignored.Value))
	switch {
	case ignored.Key == "auto_release_date":
		return "asc versions update --version-id VERSION_ID --release-type SCHEDULED --earliest-release-date DATE"
	case ignored.Key == "automatic_release" && scheduled:
		return ""
	case ignored.Key == "submit_for_review" && value == "true":
		return "asc review submit --app APP_ID --version-id VERSION_ID --build-id BUILD_ID --confirm"
	case ignored.Key == "phased_release" && value == "true":
		return "asc versions phased-release create --version-id VERSION_ID"
	case ignored.Key == "automatic_release" && value == "true":
		return "asc versions update --version-id VERSION_ID --release-type AFTER_APPROVAL"
	case ignored.Key == "automatic_release" && value == "false":
		return "asc versions update --version-id VERSION_ID --release-type MANUAL"
	}
	return ""
}
