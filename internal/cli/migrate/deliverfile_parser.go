package migrate

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
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
}

func parseDeliverfile(path string) (DeliverfileConfig, error) {
	file, err := os.Open(path)
	if err != nil {
		return DeliverfileConfig{}, err
	}
	defer func() { _ = file.Close() }()

	var config DeliverfileConfig
	scanner := bufio.NewScanner(file)
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = stripDeliverfileComment(line)
		if strings.TrimSpace(line) == "" {
			continue
		}
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
