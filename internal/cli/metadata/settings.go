package metadata

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/rootfs"
)

type metadataSettingFieldKind int

const (
	metadataSettingString metadataSettingFieldKind = iota
	metadataSettingBool
)

// A secret field is never written in clear text: pull writes
// "@env:<secretEnv>" in its place, the local file must reference the value
// with @env:NAME or @file:PATH, and plan output and receipts redact it.
type metadataSettingField struct {
	name      string
	kind      metadataSettingFieldKind
	values    []string
	secretEnv string
	nullUnset bool
}

const metadataRedactedValue = "<redacted>"

// metadataSecretReferenceSuffix keys the @env:/@file: reference of a secret
// field in the local values, so the plan and its approval bind the reference
// without exposing the resolved secret.
const metadataSecretReferenceSuffix = ":reference"

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
	validate     func(map[string]string, bool) error
	// fetch returns nil values only when the resource does not exist.
	fetch  func(ctx context.Context, d *shared.Deps, client *asc.Client, target metadataSettingTarget) (map[string]string, error)
	update func(ctx context.Context, d *shared.Deps, client *asc.Client, target metadataSettingTarget, changed map[string]string) (string, error)
}

type metadataSettingPlan struct {
	action  string
	scope   metadataSettingScope
	local   map[string]string
	changed map[string]string
}

func metadataSettingScopes() []metadataSettingScope {
	return []metadataSettingScope{categoriesSettingScope, ageRatingSettingScope, reviewInformationSettingScope}
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

func readMetadataSettingFile(d *shared.Deps, dir string, scope metadataSettingScope, parameter, missingFilePrefix string) (map[string]string, error) {
	path := filepath.Join(dir, scope.fileName)
	root, err := rootfs.New(dir)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	data, err := root.ReadFileLimited(scope.fileName, shared.MaxIndirectFileValueSize)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, shared.WithDiagnostic(d.UsageErrorf("%s %s", missingFilePrefix, path), shared.DiagnosticFileNotFound, parameter)
		}
		return nil, fmt.Errorf("failed to read %s: %w", path, err)
	}
	invalidSchema := func(format string, args ...any) error {
		err := d.UsageErrorf("invalid metadata schema in %s: %s", path, fmt.Sprintf(format, args...))
		return shared.WithDiagnostic(err, shared.DiagnosticFileInvalidFormat, parameter)
	}
	var raw map[string]json.RawMessage
	if err := decodeMetadataSettingFields(data, &raw); err != nil {
		return nil, invalidSchema("%v", err)
	}
	values := make(map[string]string, len(raw))
	for key, rawValue := range raw {
		field, ok := scope.field(key)
		if !ok {
			names := make([]string, 0, len(scope.fields))
			for _, known := range scope.fields {
				names = append(names, known.name)
			}
			return nil, invalidSchema("unknown field %q; %s fields: %s", key, scope.name, strings.Join(names, ", "))
		}
		if field.nullUnset && isJSONNull(rawValue) {
			continue
		}
		value, err := decodeMetadataSettingValue(d, dir, field, rawValue)
		if err != nil {
			return nil, invalidSchema("%v", err)
		}
		values[field.name] = value
		if field.secretEnv != "" {
			var reference string
			_ = json.Unmarshal(rawValue, &reference)
			values[field.name+metadataSecretReferenceSuffix] = reference
		}
	}
	if scope.validate != nil {
		if err := scope.validate(values, false); err != nil {
			return nil, invalidSchema("%v", err)
		}
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

func decodeMetadataSettingValue(d *shared.Deps, dir string, field metadataSettingField, raw json.RawMessage) (string, error) {
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
		if field.secretEnv != "" {
			return resolveMetadataSecretValue(d, dir, field, value)
		}
		value = strings.TrimSpace(value)
		if value == "" {
			return "", fmt.Errorf("%s cannot be empty; omit it to leave the value unchanged", field.name)
		}
		if len(field.values) > 0 && !slices.Contains(field.values, value) {
			return "", fmt.Errorf("%s must be one of %s", field.name, strings.Join(field.values, ", "))
		}
		return value, nil
	}
}

func resolveMetadataSecretValue(d *shared.Deps, dir string, field metadataSettingField, value string) (string, error) {
	var resolved string
	switch {
	case strings.HasPrefix(value, "@env:"):
		envValue, err := d.ResolveFlagValueIndirection(field.name, value)
		if err != nil {
			return "", err
		}
		resolved = envValue
	case strings.HasPrefix(value, "@file:"):
		root, err := rootfs.New(dir)
		if err != nil {
			return "", err
		}
		defer root.Close()
		data, err := root.ReadFileLimited(strings.TrimPrefix(value, "@file:"), shared.MaxIndirectFileValueSize)
		if err != nil {
			return "", fmt.Errorf("%s: cannot read the referenced file below --dir: %w", field.name, err)
		}
		resolved = string(data)
		if strings.HasSuffix(resolved, "\r\n") {
			resolved = strings.TrimSuffix(resolved, "\r\n")
		} else {
			resolved = strings.TrimSuffix(resolved, "\n")
		}
	default:
		return "", fmt.Errorf("%s must reference the secret with @env:NAME or @file:PATH instead of storing it in the file", field.name)
	}
	if strings.TrimSpace(resolved) == "" {
		return "", fmt.Errorf("%s resolved to an empty value", field.name)
	}
	return resolved, nil
}

func metadataSettingObject(fields []metadataSettingField, values map[string]string) map[string]any {
	object := make(map[string]any, len(values))
	for _, field := range fields {
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
	return object
}

func encodeMetadataSettingFile(scope metadataSettingScope, values map[string]string) ([]byte, error) {
	object := metadataSettingObject(scope.fields, values)
	for _, field := range scope.fields {
		if _, ok := object[field.name]; ok && field.secretEnv != "" {
			object[field.name] = "@env:" + field.secretEnv
		}
	}
	data, err := json.MarshalIndent(object, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

func buildMetadataSettingPlan(scope metadataSettingScope, local, remote map[string]string) (metadataSettingPlan, []PlanItem, []PlanItem) {
	plan := metadataSettingPlan{action: "update", scope: scope, local: local, changed: map[string]string{}}
	if remote == nil {
		plan.action = "create"
	}
	adds := []PlanItem{}
	updates := []PlanItem{}
	for _, field := range scope.fields {
		value, ok := local[field.name]
		if !ok {
			continue
		}
		item := PlanItem{Key: scope.name + ":" + field.name, Scope: scope.name, Field: field.name, To: value}
		if field.secretEnv != "" {
			item.To = metadataRedactedValue + " " + local[field.name+metadataSecretReferenceSuffix]
		}
		remoteValue, remoteOK := remote[field.name]
		switch {
		case !remoteOK:
			item.Reason = "field exists locally but not remotely"
			adds = append(adds, item)
		case remoteValue != value:
			item.Reason = "field value differs"
			item.From = redactMetadataSetting(field, remoteValue)
			updates = append(updates, item)
		default:
			continue
		}
		plan.changed[field.name] = value
	}
	return plan, adds, updates
}

func applyMetadataSettingPlan(ctx context.Context, d *shared.Deps, client *asc.Client, target metadataSettingTarget, plan metadataSettingPlan) (ApplyAction, error) {
	scope := plan.scope
	action := plan.action
	_, _, err := shared.RunReconciledMutationWith(ctx, d, func(requestCtx context.Context) (struct{}, error) {
		actualAction, err := scope.update(requestCtx, d, client, target, plan.changed)
		if actualAction != "" {
			action = actualAction
		}
		return struct{}{}, err
	}, func(readCtx context.Context) (struct{}, bool, error) {
		remote, err := scope.fetch(readCtx, d, client, target)
		return struct{}{}, err == nil && metadataFieldsMatch(remote, plan.changed), err
	})
	if err == nil {
		err = verifyMetadataSetting(ctx, d, client, target, plan)
	}
	if err != nil {
		err = newMetadataMutationActionError(fmt.Errorf("%s %s: %w", action, scope.name, err))
		desired := make(map[string]string, len(plan.changed))
		for _, field := range scope.fields {
			if value, ok := plan.changed[field.name]; ok {
				desired[field.name] = redactMetadataSetting(field, value)
			}
		}
		return failedMetadataAction(scope.name, "", "", action, "", desired, err), err
	}
	return successfulMetadataAction(scope.name, "", "", action, ""), nil
}

func verifyMetadataSetting(ctx context.Context, d *shared.Deps, client *asc.Client, target metadataSettingTarget, plan metadataSettingPlan) error {
	remote, err := plan.scope.fetch(ctx, d, client, target)
	if err != nil {
		return fmt.Errorf("read back: %w", err)
	}
	mismatches := []string{}
	for _, field := range plan.scope.fields {
		want, ok := plan.local[field.name]
		if !ok || remote[field.name] == want {
			continue
		}
		if field.secretEnv != "" {
			mismatches = append(mismatches, field.name+" differs")
			continue
		}
		mismatches = append(mismatches, fmt.Sprintf("%s=%q (want %q)", field.name, remote[field.name], want))
	}
	if len(mismatches) > 0 {
		return fmt.Errorf("read back differs: %s", strings.Join(mismatches, ", "))
	}
	return nil
}

func redactMetadataSetting(field metadataSettingField, value string) string {
	if field.secretEnv != "" {
		return metadataRedactedValue
	}
	return value
}

func decodeMetadataSettingFields(data []byte, target *map[string]json.RawMessage) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	token, err := decoder.Token()
	if err != nil || token != json.Delim('{') {
		return fmt.Errorf("expected a JSON object")
	}
	fields := map[string]json.RawMessage{}
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return err
		}
		key, ok := token.(string)
		if !ok {
			return fmt.Errorf("expected field name")
		}
		if _, exists := fields[key]; exists {
			return fmt.Errorf("duplicate field %q", key)
		}
		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil {
			return err
		}
		fields[key] = value
	}
	if _, err := decoder.Token(); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return fmt.Errorf("trailing data")
	}
	*target = fields
	return nil
}
