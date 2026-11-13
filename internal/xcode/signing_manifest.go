package xcode

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	pathpkg "path"
	"sort"
	"strings"

	"github.com/google/uuid"
)

func rejectDuplicateJSONKeys(data []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	if err := scanJSONValueForDuplicateKeys(decoder); err != nil {
		return err
	}
	return nil
}

func scanJSONValueForDuplicateKeys(decoder *json.Decoder) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delimiter, ok := token.(json.Delim)
	if !ok {
		return nil
	}

	switch delimiter {
	case '{':
		seen := make(map[string]struct{})
		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return err
			}
			key, ok := keyToken.(string)
			if !ok {
				return fmt.Errorf("JSON object key is not a string")
			}
			if _, duplicate := seen[key]; duplicate {
				return fmt.Errorf("duplicate JSON object key %q", key)
			}
			seen[key] = struct{}{}
			if err := scanJSONValueForDuplicateKeys(decoder); err != nil {
				return err
			}
		}
	case '[':
		for decoder.More() {
			if err := scanJSONValueForDuplicateKeys(decoder); err != nil {
				return err
			}
		}
	default:
		return fmt.Errorf("unexpected JSON delimiter %q", delimiter)
	}

	closing, err := decoder.Token()
	if err != nil {
		return err
	}
	expectedClosing := json.Delim('}')
	if delimiter == '[' {
		expectedClosing = ']'
	}
	if closing != expectedClosing {
		return fmt.Errorf("unexpected JSON delimiter %q", closing)
	}
	return nil
}

func readSigningSettingsManifest(path string) (*signingSettingsManifest, error) {
	data, err := readSigningRegularFile(path, signingSettingsMaxBytes)
	if err != nil {
		return nil, fmt.Errorf("read settings file %s: %w", path, err)
	}
	if err := rejectDuplicateJSONKeys(data); err != nil {
		return nil, newSigningInputError(fmt.Errorf("decode settings file %s: %w", path, err))
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var manifest signingSettingsManifest
	if err := decoder.Decode(&manifest); err != nil {
		return nil, newSigningInputError(fmt.Errorf("decode settings file %s: %w", path, err))
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return nil, newSigningInputError(fmt.Errorf("decode settings file %s: multiple JSON values", path))
		}
		return nil, newSigningInputError(fmt.Errorf("decode settings file %s: %w", path, err))
	}
	if manifest.SchemaVersion != signingPlanSchemaVersion {
		return nil, newSigningInputError(fmt.Errorf("settings file schemaVersion must be %d", signingPlanSchemaVersion))
	}
	if len(manifest.Targets) == 0 {
		return nil, newSigningInputError(fmt.Errorf("settings file targets must not be empty"))
	}
	return &manifest, nil
}

func normalizeSigningRequests(manifest *signingSettingsManifest) ([]signingRequest, []SigningPlanTarget, error) {
	requests := make([]signingRequest, 0)
	desired := make([]SigningPlanTarget, 0, len(manifest.Targets))
	seenTargets := make(map[string]bool)
	for _, target := range manifest.Targets {
		name := strings.TrimSpace(target.Name)
		if err := validateSigningName(name, "target"); err != nil {
			return nil, nil, err
		}
		if seenTargets[name] {
			return nil, nil, fmt.Errorf("settings file contains duplicate target %q", name)
		}
		seenTargets[name] = true
		if len(target.Configurations) == 0 {
			return nil, nil, fmt.Errorf("target %q configurations must not be empty", name)
		}
		seenConfigurations := make(map[string]bool)
		planTarget := SigningPlanTarget{Target: name}
		for _, configuration := range target.Configurations {
			configurationName := strings.TrimSpace(configuration.Name)
			if err := validateSigningName(configurationName, "configuration"); err != nil {
				return nil, nil, fmt.Errorf("target %q: %w", name, err)
			}
			if seenConfigurations[configurationName] {
				return nil, nil, fmt.Errorf("target %q contains duplicate configuration %q", name, configurationName)
			}
			seenConfigurations[configurationName] = true
			if len(configuration.Settings) == 0 {
				return nil, nil, fmt.Errorf("target %q configuration %q settings must not be empty", name, configurationName)
			}
			settings, err := normalizeSigningSettings(configuration.Settings)
			if err != nil {
				return nil, nil, fmt.Errorf("target %q configuration %q: %w", name, configurationName, err)
			}
			request := signingRequest{target: name, configuration: configurationName, settings: settings}
			requests = append(requests, request)
			planConfiguration := SigningPlanConfiguration{Name: configurationName}
			for _, setting := range settings {
				planConfiguration.Settings = append(planConfiguration.Settings, SigningPlanSetting{Key: setting.key, Value: cloneSigningString(setting.value)})
			}
			planTarget.Configurations = append(planTarget.Configurations, planConfiguration)
		}
		desired = append(desired, planTarget)
	}
	sort.Slice(desired, func(left, right int) bool { return desired[left].Target < desired[right].Target })
	for index := range desired {
		sort.Slice(desired[index].Configurations, func(left, right int) bool {
			return desired[index].Configurations[left].Name < desired[index].Configurations[right].Name
		})
	}
	return requests, desired, nil
}

func normalizeSigningSettings(raw map[string]json.RawMessage) ([]signingDesiredSetting, error) {
	keys := make([]string, 0, len(raw))
	for key := range raw {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	settings := make([]signingDesiredSetting, 0, len(keys))
	for _, key := range keys {
		if !allowedSigningSetting(key) {
			return nil, fmt.Errorf("unsupported signing setting %q", key)
		}
		value, err := normalizeSigningValue(key, raw[key])
		if err != nil {
			return nil, err
		}
		settings = append(settings, signingDesiredSetting{key: key, value: value})
	}
	return settings, nil
}

func normalizeSigningValue(key string, raw json.RawMessage) (*string, error) {
	trimmed := bytes.TrimSpace(raw)
	if bytes.Equal(trimmed, []byte("null")) {
		if !signingSettingAllowsRemoval(key) {
			return nil, fmt.Errorf("%s does not support null removal", key)
		}
		return nil, nil
	}
	var value string
	if err := json.Unmarshal(trimmed, &value); err != nil {
		return nil, fmt.Errorf("%s must be a string or null", key)
	}
	if err := validateSigningStaticValue(key, value); err != nil {
		return nil, err
	}
	value = strings.TrimSpace(value)
	if value == "" && (key == "CODE_SIGN_STYLE" || key == "DEVELOPMENT_TEAM" || key == "PRODUCT_BUNDLE_IDENTIFIER") {
		return nil, fmt.Errorf("%s must not be empty", key)
	}
	switch key {
	case "CODE_SIGN_STYLE":
		switch strings.ToLower(value) {
		case "automatic":
			value = "Automatic"
		case "manual":
			value = "Manual"
		default:
			return nil, fmt.Errorf("CODE_SIGN_STYLE must be automatic or manual")
		}
	case "DEVELOPMENT_TEAM":
		value = strings.ToUpper(value)
		if !signingTeamIDPattern.MatchString(value) {
			return nil, fmt.Errorf("DEVELOPMENT_TEAM must be a 10-character alphanumeric team ID")
		}
	case "PROVISIONING_PROFILE":
		parsed, err := uuid.Parse(value)
		if err != nil || parsed == uuid.Nil {
			return nil, fmt.Errorf("PROVISIONING_PROFILE must be a UUID")
		}
		value = parsed.String()
	case "PRODUCT_BUNDLE_IDENTIFIER":
		if !signingBundleIDPattern.MatchString(value) {
			return nil, fmt.Errorf("PRODUCT_BUNDLE_IDENTIFIER must be a reverse-DNS bundle identifier")
		}
	case "CODE_SIGN_ENTITLEMENTS":
		if err := validateSigningRelativePath(value); err != nil {
			return nil, fmt.Errorf("CODE_SIGN_ENTITLEMENTS: %w", err)
		}
	}
	return stringPtr(value), nil
}

func validateSigningStaticValue(key, value string) error {
	if strings.ContainsRune(value, '\x00') {
		return fmt.Errorf("%s must not contain NUL", key)
	}
	if strings.ContainsAny(value, "\r\n") {
		return fmt.Errorf("%s must not contain a newline", key)
	}
	if strings.Contains(value, "//") || strings.Contains(value, "/*") || strings.Contains(value, "*/") {
		return fmt.Errorf("%s must not contain comment syntax", key)
	}
	if strings.Contains(value, "$(") || strings.Contains(value, "${") {
		return fmt.Errorf("%s must be a static value without build-setting references", key)
	}
	return nil
}

func validateSigningRelativePath(value string) error {
	if value == "" {
		return fmt.Errorf("path must not be empty")
	}
	if pathpkg.IsAbs(value) || strings.HasPrefix(value, "~") || strings.Contains(value, "\\") || isWindowsDrivePath(value) {
		return fmt.Errorf("path must be relative and use POSIX separators")
	}
	clean := pathpkg.Clean(value)
	if clean == "." || clean != value {
		return fmt.Errorf("path must not contain traversal or redundant components")
	}
	for _, component := range strings.Split(value, "/") {
		if component == ".." || component == "" {
			return fmt.Errorf("path must stay within the project")
		}
	}
	return nil
}

func isWindowsDrivePath(value string) bool {
	return len(value) >= 2 && ((value[0] >= 'A' && value[0] <= 'Z') || (value[0] >= 'a' && value[0] <= 'z')) && value[1] == ':'
}

func validateSigningName(value, label string) error {
	if value == "" {
		return fmt.Errorf("%s must not be empty", label)
	}
	if strings.ContainsRune(value, '\x00') || strings.ContainsAny(value, "\r\n") {
		return fmt.Errorf("%s must not contain control characters", label)
	}
	return nil
}

func allowedSigningSetting(key string) bool {
	switch key {
	case "CODE_SIGN_STYLE", "DEVELOPMENT_TEAM", "CODE_SIGN_IDENTITY", "PROVISIONING_PROFILE_SPECIFIER", "PROVISIONING_PROFILE", "CODE_SIGN_ENTITLEMENTS", "PRODUCT_BUNDLE_IDENTIFIER":
		return true
	default:
		return false
	}
}

func signingSettingAllowsRemoval(key string) bool {
	switch key {
	case "CODE_SIGN_IDENTITY", "PROVISIONING_PROFILE_SPECIFIER", "PROVISIONING_PROFILE", "CODE_SIGN_ENTITLEMENTS":
		return true
	default:
		return false
	}
}
