package metadata

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

type metadataSettingFieldKind int

const (
	metadataSettingString metadataSettingFieldKind = iota
	metadataSettingBool
)

type metadataSettingField struct {
	name string
	kind metadataSettingFieldKind
}

type metadataSettingTarget struct {
	appInfoID string
	versionID string
}

// metadataSettingScope is a non-localized metadata surface stored in one
// canonical JSON file at the root of --dir and selected with --include.
type metadataSettingScope struct {
	name         string
	fileName     string
	fields       []metadataSettingField
	needsAppInfo bool
	fetch        func(ctx context.Context, client *asc.Client, target metadataSettingTarget) (map[string]string, error)
	update       func(ctx context.Context, client *asc.Client, target metadataSettingTarget, changed map[string]string) error
}

type metadataSettingPlan struct {
	scope   metadataSettingScope
	local   map[string]string
	changed map[string]string
}

func metadataSettingScopes() []metadataSettingScope {
	return []metadataSettingScope{categoriesSettingScope}
}

func metadataSettingScopeNames() []string {
	names := []string{}
	for _, scope := range metadataSettingScopes() {
		names = append(names, scope.name)
	}
	return names
}

func metadataIncludeFlagUsage() string {
	scopes := append([]string{includeLocalizations, "app-clip", "previews"}, metadataSettingScopeNames()...)
	return "Included scopes: " + strings.Join(scopes, ",") + " (comma-separated)"
}

func metadataSettingsHelp(indent string, withFields bool) string {
	lines := []string{}
	for _, scope := range metadataSettingScopes() {
		lines = append(lines, fmt.Sprintf("%s- %s: <dir>/%s", indent, scope.name, scope.fileName))
		if !withFields {
			continue
		}
		line := indent + "   "
		for i, field := range scope.fields {
			name := field.name
			if i < len(scope.fields)-1 {
				name += ","
			}
			if len(line)+len(name)+1 > 100 {
				lines = append(lines, line)
				line = indent + "   "
			}
			line += " " + name
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

func isMetadataSettingScope(name string) bool {
	for _, scope := range metadataSettingScopes() {
		if scope.name == name {
			return true
		}
	}
	return false
}

func selectedMetadataSettingScopes(includes []string) []metadataSettingScope {
	selected := []metadataSettingScope{}
	for _, scope := range metadataSettingScopes() {
		if includesScope(includes, scope.name) {
			selected = append(selected, scope)
		}
	}
	return selected
}

func metadataSettingsNeedAppInfo(scopes []metadataSettingScope) bool {
	for _, scope := range scopes {
		if scope.needsAppInfo {
			return true
		}
	}
	return false
}

func readMetadataSettingFile(dir string, scope metadataSettingScope) (map[string]string, error) {
	path := filepath.Join(dir, scope.fileName)
	data, err := readFileNoFollow(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, shared.UsageErrorf("--include %s requires %s", scope.name, path)
		}
		return nil, fmt.Errorf("failed to read %s: %w", path, err)
	}
	var raw map[string]json.RawMessage
	if err := decodeStrictJSON(data, &raw); err != nil {
		return nil, shared.UsageErrorf("invalid metadata schema in %s: %v", path, err)
	}
	values := make(map[string]string, len(raw))
	for key, rawValue := range raw {
		field, ok := scope.field(key)
		if !ok {
			return nil, shared.UsageErrorf("invalid metadata schema in %s: unknown field %q", path, key)
		}
		value, err := decodeMetadataSettingValue(field, rawValue)
		if err != nil {
			return nil, shared.UsageErrorf("invalid metadata schema in %s: %v", path, err)
		}
		values[field.name] = value
	}
	return values, nil
}

func (scope metadataSettingScope) field(name string) (metadataSettingField, bool) {
	for _, field := range scope.fields {
		if field.name == name {
			return field, true
		}
	}
	return metadataSettingField{}, false
}

func decodeMetadataSettingValue(field metadataSettingField, raw json.RawMessage) (string, error) {
	if isJSONNull(raw) {
		return "", fmt.Errorf("%s cannot be null; omit it to leave the value unchanged", field.name)
	}
	switch field.kind {
	case metadataSettingBool:
		var value bool
		if err := json.Unmarshal(raw, &value); err != nil {
			return "", fmt.Errorf("%s must be a boolean", field.name)
		}
		return strconv.FormatBool(value), nil
	default:
		var value string
		if err := json.Unmarshal(raw, &value); err != nil {
			return "", fmt.Errorf("%s must be a string", field.name)
		}
		value = strings.TrimSpace(value)
		if value == "" {
			return "", fmt.Errorf("%s cannot be empty; omit it to leave the value unchanged", field.name)
		}
		return value, nil
	}
}

func encodeMetadataSettingFile(scope metadataSettingScope, values map[string]string) ([]byte, error) {
	object := make(map[string]any, len(values))
	for _, field := range scope.fields {
		value, ok := values[field.name]
		if !ok {
			continue
		}
		if field.kind == metadataSettingBool {
			object[field.name] = value == "true"
			continue
		}
		object[field.name] = value
	}
	data, err := json.MarshalIndent(object, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

func buildMetadataSettingPlan(scope metadataSettingScope, local, remote map[string]string) (metadataSettingPlan, []PlanItem, []PlanItem) {
	plan := metadataSettingPlan{scope: scope, local: local, changed: map[string]string{}}
	adds := []PlanItem{}
	updates := []PlanItem{}
	for _, field := range scope.fields {
		value, ok := local[field.name]
		if !ok {
			continue
		}
		item := PlanItem{Key: scope.name + ":" + field.name, Scope: scope.name, Field: field.name, To: value}
		remoteValue, remoteOK := remote[field.name]
		switch {
		case !remoteOK:
			item.Reason = "field exists locally but not remotely"
			adds = append(adds, item)
		case remoteValue != value:
			item.Reason = "field value differs"
			item.From = remoteValue
			updates = append(updates, item)
		default:
			continue
		}
		plan.changed[field.name] = value
	}
	return plan, adds, updates
}

func applyMetadataSettingPlan(ctx context.Context, client *asc.Client, target metadataSettingTarget, plan metadataSettingPlan) (ApplyAction, error) {
	scope := plan.scope
	_, _, err := shared.RunReconciledMutation(ctx, func(requestCtx context.Context) (struct{}, error) {
		return struct{}{}, scope.update(requestCtx, client, target, plan.changed)
	}, func(readCtx context.Context) (struct{}, bool, error) {
		remote, err := scope.fetch(readCtx, client, target)
		return struct{}{}, err == nil && metadataFieldsMatch(remote, plan.changed), err
	})
	if err == nil {
		err = verifyMetadataSetting(ctx, client, target, plan)
	}
	if err != nil {
		err = newMetadataMutationActionError(fmt.Errorf("update %s: %w", scope.name, err))
		return failedMetadataAction(scope.name, "", "", "update", "", plan.changed, err), err
	}
	return successfulMetadataAction(scope.name, "", "", "update", ""), nil
}

func verifyMetadataSetting(ctx context.Context, client *asc.Client, target metadataSettingTarget, plan metadataSettingPlan) error {
	remote, err := plan.scope.fetch(ctx, client, target)
	if err != nil {
		return fmt.Errorf("read back: %w", err)
	}
	mismatches := []string{}
	for _, field := range plan.scope.fields {
		want, ok := plan.local[field.name]
		if !ok || remote[field.name] == want {
			continue
		}
		mismatches = append(mismatches, fmt.Sprintf("%s=%q (want %q)", field.name, remote[field.name], want))
	}
	if len(mismatches) > 0 {
		return fmt.Errorf("read back differs: %s", strings.Join(mismatches, ", "))
	}
	return nil
}
