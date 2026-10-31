package xcode

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/bitrise-io/go-xcode/xcodeproject/serialized"
	"github.com/bitrise-io/go-xcode/xcodeproject/xcodeproj"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/rootfs"
)

const (
	pbxprojFileName = "project.pbxproj"
	xcprojFileName  = "project.xcproj"
	xcprojMaxDepth  = 256
)

var (
	errXCProjSigningUnsupported = fmt.Errorf("asc xcode signing does not support the JSON project format (%s) yet; no files were changed", xcprojFileName)
	errXCProjAgvtoolUnsupported = fmt.Errorf("agvtool cannot read or edit the JSON project format (%s); define MARKETING_VERSION and CURRENT_PROJECT_VERSION in the project or referenced xcconfig files", xcprojFileName)
	xcprojConfigCondition       = regexp.MustCompile(`\[config=([^\],]*)\]`)
)

type xcprojValue struct {
	kind       byte // '{', '[', '"', or 0 for a number or keyword
	start, end int
	text       string
	entries    []xcprojEntry
	items      []*xcprojValue
}

type xcprojEntry struct {
	key   string
	start int
	value *xcprojValue
}

func (value *xcprojValue) field(key string) *xcprojValue {
	if value == nil {
		return nil
	}
	for _, entry := range value.entries {
		if entry.key == key {
			return entry.value
		}
	}
	return nil
}

func (value *xcprojValue) stringField(key string) string {
	if field := value.field(key); field != nil && field.kind == '"' {
		return field.text
	}
	return ""
}

// xcprojParser reads the JSON5 subset Foundation accepts for project files and
// keeps source offsets so edits can leave every other byte alone. Syntax it
// does not recognize is an error rather than a guess.
type xcprojParser struct {
	data []byte
	pos  int
}

func parseXCProjDocument(data []byte) (*xcprojValue, error) {
	if !utf8.Valid(data) {
		return nil, errors.New("file is not UTF-8")
	}
	parser := &xcprojParser{data: data}
	if bytes.HasPrefix(data, []byte{0xef, 0xbb, 0xbf}) {
		parser.pos = 3
	}
	if err := parser.skip(); err != nil {
		return nil, err
	}
	root, err := parser.value(0)
	if err != nil {
		return nil, err
	}
	if err := parser.skip(); err != nil {
		return nil, err
	}
	if parser.pos != len(data) {
		return nil, parser.errorf("unexpected content after the project")
	}
	if root.kind != '{' {
		return nil, errors.New("the project root is not an object")
	}
	return root, nil
}

func (parser *xcprojParser) errorf(format string, args ...any) error {
	return fmt.Errorf("%s at offset %d", fmt.Sprintf(format, args...), parser.pos)
}

func (parser *xcprojParser) skip() error {
	for parser.pos < len(parser.data) {
		rest := parser.data[parser.pos:]
		switch {
		case strings.IndexByte(" \t\r\n\v\f", rest[0]) >= 0:
			parser.pos++
		case bytes.HasPrefix(rest, []byte("//")):
			end := bytes.IndexAny(rest, "\r\n")
			if end < 0 {
				end = len(rest)
			}
			parser.pos += end
		case bytes.HasPrefix(rest, []byte("/*")):
			end := bytes.Index(rest, []byte("*/"))
			if end < 0 {
				return parser.errorf("unterminated comment")
			}
			parser.pos += end + 2
		default:
			return nil
		}
	}
	return nil
}

func (parser *xcprojParser) peek() byte {
	if parser.pos < len(parser.data) {
		return parser.data[parser.pos]
	}
	return 0
}

func (parser *xcprojParser) value(depth int) (*xcprojValue, error) {
	if depth > xcprojMaxDepth {
		return nil, parser.errorf("nesting is too deep")
	}
	start := parser.pos
	value := &xcprojValue{start: start}
	var err error
	switch parser.peek() {
	case '{':
		value.kind = '{'
		err = parser.object(value, depth)
	case '[':
		value.kind = '['
		err = parser.array(value, depth)
	case '"', '\'':
		value.kind = '"'
		value.text, err = parser.quoted()
	default:
		value.text, err = parser.bare()
	}
	value.end = parser.pos
	return value, err
}

func (parser *xcprojParser) object(value *xcprojValue, depth int) error {
	seen := make(map[string]bool)
	parser.pos++
	for {
		if err := parser.skip(); err != nil {
			return err
		}
		if parser.peek() == '}' {
			parser.pos++
			return nil
		}
		start := parser.pos
		var key string
		var err error
		if parser.peek() == '"' || parser.peek() == '\'' {
			key, err = parser.quoted()
		} else {
			key, err = parser.bare()
		}
		if err != nil {
			return err
		}
		if seen[key] {
			return parser.errorf("duplicate key %q", key)
		}
		seen[key] = true
		if err := parser.skip(); err != nil {
			return err
		}
		if parser.peek() != ':' {
			return parser.errorf("expected ':' after %q", key)
		}
		parser.pos++
		if err := parser.skip(); err != nil {
			return err
		}
		member, err := parser.value(depth + 1)
		if err != nil {
			return err
		}
		value.entries = append(value.entries, xcprojEntry{key: key, start: start, value: member})
		if err := parser.skip(); err != nil {
			return err
		}
		switch parser.peek() {
		case ',':
			parser.pos++
		case '}':
		default:
			return parser.errorf("expected ',' or '}'")
		}
	}
}

func (parser *xcprojParser) array(value *xcprojValue, depth int) error {
	parser.pos++
	for {
		if err := parser.skip(); err != nil {
			return err
		}
		if parser.peek() == ']' {
			parser.pos++
			return nil
		}
		member, err := parser.value(depth + 1)
		if err != nil {
			return err
		}
		value.items = append(value.items, member)
		if err := parser.skip(); err != nil {
			return err
		}
		switch parser.peek() {
		case ',':
			parser.pos++
		case ']':
		default:
			return parser.errorf("expected ',' or ']'")
		}
	}
}

func (parser *xcprojParser) bare() (string, error) {
	start := parser.pos
	for parser.pos < len(parser.data) {
		character := parser.data[parser.pos]
		isWord := character == '_' || character == '$' || character == '.' || character == '+' || character == '-' ||
			character >= '0' && character <= '9' || character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z'
		if !isWord {
			break
		}
		parser.pos++
	}
	if parser.pos == start {
		return "", parser.errorf("unexpected character")
	}
	return string(parser.data[start:parser.pos]), nil
}

func (parser *xcprojParser) quoted() (string, error) {
	quote := parser.data[parser.pos]
	parser.pos++
	var text strings.Builder
	for parser.pos < len(parser.data) {
		character := parser.data[parser.pos]
		switch character {
		case quote:
			parser.pos++
			return text.String(), nil
		case '\n', '\r':
			return "", parser.errorf("unterminated string")
		case '\\':
			if err := parser.escape(&text); err != nil {
				return "", err
			}
		default:
			text.WriteByte(character)
			parser.pos++
		}
	}
	return "", parser.errorf("unterminated string")
}

func (parser *xcprojParser) escape(text *strings.Builder) error {
	parser.pos++
	escape := parser.peek()
	parser.pos++
	switch escape {
	case '"', '\'', '\\', '/':
		text.WriteByte(escape)
	case 'b':
		text.WriteByte('\b')
	case 'f':
		text.WriteByte('\f')
	case 'n':
		text.WriteByte('\n')
	case 'r':
		text.WriteByte('\r')
	case 't':
		text.WriteByte('\t')
	case 'u':
		first, err := parser.unicodeEscape()
		if err != nil {
			return err
		}
		if !utf16.IsSurrogate(first) {
			text.WriteRune(first)
			return nil
		}
		if !bytes.HasPrefix(parser.data[parser.pos:], []byte(`\u`)) {
			return parser.errorf("unpaired surrogate escape")
		}
		parser.pos += 2
		second, err := parser.unicodeEscape()
		if err != nil {
			return err
		}
		combined := utf16.DecodeRune(first, second)
		if combined == utf8.RuneError {
			return parser.errorf("unpaired surrogate escape")
		}
		text.WriteRune(combined)
	default:
		return parser.errorf("unsupported string escape")
	}
	return nil
}

func (parser *xcprojParser) unicodeEscape() (rune, error) {
	if parser.pos+4 > len(parser.data) {
		return 0, parser.errorf("truncated unicode escape")
	}
	code, err := strconv.ParseUint(string(parser.data[parser.pos:parser.pos+4]), 16, 16)
	if err != nil {
		return 0, parser.errorf("invalid unicode escape")
	}
	parser.pos += 4
	return rune(code), nil
}

// xcprojModel holds one scope per build-settings object. The format keeps
// build settings flat, so each configuration's view comes from [config=...]
// conditions on the keys of a shared object.
type xcprojModel struct {
	data    []byte
	project xcodeproj.Proj
	objects serialized.Object
	scopes  []*xcprojScope
}

type xcprojScope struct {
	target         string
	settings       *xcprojValue
	configurations []*xcprojConfiguration
}

type xcprojConfiguration struct {
	id       string
	name     string
	settings serialized.Object
	// origins maps each key in settings to the key that provides it on disk.
	origins map[string]string
}

type xcprojReference struct {
	id, name, dir string
	dirErr        error
	children      []*xcprojReference
}

func parseXCProjModel(data []byte, rootDir string) (*xcprojModel, error) {
	root, err := parseXCProjDocument(data)
	if err != nil {
		return nil, err
	}
	if capabilities := root.field("required-capabilities"); capabilities != nil && (capabilities.kind != '[' || len(capabilities.items) > 0) {
		names := make([]string, 0, len(capabilities.items))
		for _, item := range capabilities.items {
			names = append(names, item.text)
		}
		return nil, fmt.Errorf("the project requires capabilities asc does not support: %s", strings.Join(names, ", "))
	}
	model := &xcprojModel{data: data, objects: serialized.Object{}}
	files := xcprojReferences(root.field("files"), rootDir, nil, rootDir)
	defaultName := root.stringField("default-configuration")

	var names []string
	projectFiles := make(map[string]*xcprojValue)
	for _, item := range xcprojConfigurations(root.field("configurations")) {
		names = append(names, item.name)
		projectFiles[item.name] = item.file
	}
	list, err := model.addScope("", root.field("build-settings"), names, projectFiles, defaultName, files)
	if err != nil {
		return nil, err
	}
	model.project.BuildConfigurationList = list

	seenTargets := make(map[string]bool)
	targets := root.field("targets")
	if targets != nil {
		for _, target := range targets.items {
			name := target.stringField("name")
			if name == "" || seenTargets[name] {
				return nil, fmt.Errorf("target name %q is missing or used more than once", name)
			}
			seenTargets[name] = true
			targetFiles := make(map[string]*xcprojValue)
			for _, item := range xcprojConfigurations(target.field("specialized-configurations")) {
				if !slices.Contains(names, item.name) {
					return nil, fmt.Errorf("target %q specializes unknown configuration %q", name, item.name)
				}
				targetFiles[item.name] = item.file
			}
			list, err := model.addScope(name, target.field("build-settings"), names, targetFiles, defaultName, files)
			if err != nil {
				return nil, err
			}
			productType := target.stringField("full-product-type")
			if abbreviated := target.stringField("product-type"); abbreviated != "" {
				productType = "com.apple.product-type." + abbreviated
			}
			product := target.stringField("product")
			if strings.HasPrefix(productType, "com.apple.product-type.application") {
				product = name + ".app"
			}
			model.project.Targets = append(model.project.Targets, xcodeproj.Target{
				ID:                     target.stringField("id"),
				Name:                   name,
				BuildConfigurationList: list,
				ProductReference:       xcodeproj.ProductReference{Path: product},
				ProductType:            productType,
			})
		}
	}
	return model, nil
}

type xcprojConfigurationItem struct {
	name string
	file *xcprojValue
}

func xcprojConfigurations(list *xcprojValue) []xcprojConfigurationItem {
	if list == nil {
		return nil
	}
	items := make([]xcprojConfigurationItem, 0, len(list.items))
	for _, item := range list.items {
		if item.kind == '"' {
			items = append(items, xcprojConfigurationItem{name: item.text})
		} else {
			items = append(items, xcprojConfigurationItem{name: item.stringField("name"), file: item.field("file")})
		}
	}
	return items
}

func (model *xcprojModel) addScope(
	target string,
	settings *xcprojValue,
	names []string,
	files map[string]*xcprojValue,
	defaultName string,
	references []*xcprojReference,
) (xcodeproj.ConfigurationList, error) {
	scope := &xcprojScope{target: target, settings: settings}
	list := xcodeproj.ConfigurationList{DefaultConfigurationName: defaultName}
	for _, name := range names {
		resolved, origins, err := xcprojSettingsForConfiguration(settings, name)
		if err != nil {
			return list, fmt.Errorf("build settings for target %q configuration %q: %w", target, name, err)
		}
		id := "xcproj\x00" + target + "\x00" + name
		object := map[string]interface{}{}
		if file := files[name]; file != nil {
			reference := "xcconfig:" + string(model.data[file.start:file.end])
			object["baseConfigurationReference"] = reference
			if filePath, err := resolveXCProjFile(file, references); err == nil {
				model.objects[reference] = map[string]interface{}{"path": filePath}
			}
		}
		model.objects[id] = object
		scope.configurations = append(scope.configurations, &xcprojConfiguration{id: id, name: name, settings: resolved, origins: origins})
		list.BuildConfigurations = append(list.BuildConfigurations, xcodeproj.BuildConfiguration{ID: id, Name: name, BuildSettings: maps.Clone(resolved)})
	}
	model.scopes = append(model.scopes, scope)
	return list, nil
}

func xcprojSettingsForConfiguration(settings *xcprojValue, name string) (serialized.Object, map[string]string, error) {
	resolved := serialized.Object{}
	origins := make(map[string]string)
	if settings == nil {
		return resolved, origins, nil
	}
	var conditional []xcprojEntry
	for _, entry := range settings.entries {
		if xcprojConfigCondition.MatchString(entry.key) {
			conditional = append(conditional, entry)
			continue
		}
		value, err := xcprojBuildSetting(entry)
		if err != nil {
			return nil, nil, err
		}
		resolved[entry.key] = value
		origins[entry.key] = entry.key
	}
	for _, entry := range conditional {
		match := xcprojConfigCondition.FindStringSubmatchIndex(entry.key)
		matched, err := path.Match(entry.key[match[2]:match[3]], name)
		if err != nil {
			return nil, nil, fmt.Errorf("%s: %w", entry.key, err)
		}
		if !matched {
			continue
		}
		value, err := xcprojBuildSetting(entry)
		if err != nil {
			return nil, nil, err
		}
		key := entry.key[:match[0]] + entry.key[match[1]:]
		if _, taken := resolved[key]; taken {
			// Xcode 27.1 ignores the condition in this pair. Keeping the raw key leaves
			// the conflict visible to the checks that reject differing conditional values.
			key = entry.key
		}
		resolved[key] = value
		origins[key] = entry.key
	}
	return resolved, origins, nil
}

func xcprojBuildSetting(entry xcprojEntry) (any, error) {
	switch entry.value.kind {
	case '"':
		return entry.value.text, nil
	case '[':
		values := make([]any, 0, len(entry.value.items))
		for _, item := range entry.value.items {
			if item.kind != '"' {
				return nil, fmt.Errorf("build setting %s has a non-string array item", entry.key)
			}
			values = append(values, item.text)
		}
		return values, nil
	}
	return nil, fmt.Errorf("build setting %s is neither a string nor an array", entry.key)
}

func xcprojReferences(list *xcprojValue, parentDir string, parentErr error, rootDir string) []*xcprojReference {
	if list == nil {
		return nil
	}
	references := make([]*xcprojReference, 0, len(list.items))
	for _, item := range list.items {
		pathValue := item.stringField("path")
		reference := &xcprojReference{id: item.stringField("id"), name: item.stringField("name"), dirErr: parentErr}
		if reference.name == "" {
			reference.name = path.Base(pathValue)
		}
		if reference.dirErr == nil {
			reference.dir, reference.dirErr = resolveXCProjPath(parentDir, pathValue, rootDir)
		}
		reference.children = xcprojReferences(item.field("children"), reference.dir, reference.dirErr, rootDir)
		references = append(references, reference)
	}
	return references
}

func resolveXCProjPath(parentDir, value, rootDir string) (string, error) {
	switch {
	case value == "":
		return parentDir, nil
	case strings.HasPrefix(value, "<PROJECT>/"):
		return filepath.Join(rootDir, strings.TrimPrefix(value, "<PROJECT>/")), nil
	case strings.HasPrefix(value, "<"), strings.HasPrefix(value, "~"), strings.Contains(value, `\`):
		return "", fmt.Errorf("unsupported path base in %q", value)
	case filepath.IsAbs(value):
		return filepath.Clean(value), nil
	}
	return filepath.Join(parentDir, value), nil
}

func resolveXCProjFile(file *xcprojValue, references []*xcprojReference) (string, error) {
	anchor, relative := file, []string(nil)
	if file.kind == '{' {
		anchor = file.field("anchor")
		var err error
		relative, err = xcprojNamePath(file.field("relative-path"))
		if err != nil {
			return "", err
		}
	}
	if anchor == nil {
		return "", errors.New("xcconfig reference has no anchor")
	}
	reference, err := findXCProjReference(anchor, references)
	if err != nil {
		return "", err
	}
	if reference.dirErr != nil {
		return "", reference.dirErr
	}
	return filepath.Join(append([]string{reference.dir}, relative...)...), nil
}

func findXCProjReference(anchor *xcprojValue, references []*xcprojReference) (*xcprojReference, error) {
	if id, isID := strings.CutPrefix(anchor.text, "id:"); isID && anchor.kind == '"' {
		if found := findXCProjReferenceID(id, references); found != nil {
			return found, nil
		}
		return nil, fmt.Errorf("no file reference has id %q", id)
	}
	components, err := xcprojNamePath(anchor)
	if err != nil {
		return nil, err
	}
	var current *xcprojReference
	for _, component := range components {
		var matches []*xcprojReference
		for _, reference := range references {
			if reference.name == component {
				matches = append(matches, reference)
			}
		}
		if len(matches) != 1 {
			return nil, fmt.Errorf("file reference %q matches %d items", strings.Join(components, "/"), len(matches))
		}
		current = matches[0]
		references = current.children
	}
	if current == nil {
		return nil, errors.New("empty file reference")
	}
	return current, nil
}

func findXCProjReferenceID(id string, references []*xcprojReference) *xcprojReference {
	for _, reference := range references {
		if reference.id == id {
			return reference
		}
		if found := findXCProjReferenceID(id, reference.children); found != nil {
			return found
		}
	}
	return nil
}

func xcprojNamePath(value *xcprojValue) ([]string, error) {
	switch {
	case value == nil:
		return nil, errors.New("missing path")
	case value.kind == '"':
		return strings.Split(value.text, "/"), nil
	case value.kind == '[':
		components := make([]string, 0, len(value.items))
		for _, item := range value.items {
			if item.kind == '"' {
				components = append(components, item.text)
			} else {
				components = append(components, item.stringField("name"))
			}
		}
		return components, nil
	}
	return nil, errors.New("unsupported path")
}

func openXCProjStructuredProject(projectPath string) (*structuredVersionProject, error) {
	absolutePath, err := filepath.Abs(projectPath)
	if err != nil {
		return nil, fmt.Errorf("resolve Xcode project path: %w", err)
	}
	filePath := filepath.Join(absolutePath, xcprojFileName)
	rootDir := filepath.Dir(absolutePath)
	root, err := rootfs.New(rootDir)
	if err != nil {
		return nil, fmt.Errorf("open Xcode project root: %w", err)
	}
	defer root.Close()
	data, err := root.AllowingInternalSymlinks().ReadFileLimited(filePath, signingPlanMaxBytes)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", filePath, err)
	}
	model, err := parseXCProjModel(data, rootDir)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", filePath, err)
	}
	project := &structuredVersionProject{
		project:         xcodeproj.XcodeProj{Proj: model.project},
		projectPath:     absolutePath,
		projectFilePath: filePath,
		projectSource:   "xcproj",
		rootDir:         rootDir,
		objects:         model.objects,
		parentByChild:   make(map[string]string),
		xcproj:          model,
	}
	if err := project.indexConfigurations(); err != nil {
		return nil, err
	}
	return project, nil
}

// xcprojFilePresent reports whether the project uses the JSON format. Xcode
// rejects a project with both definition files, so asc does not pick one.
func xcprojFilePresent(projectPath string) (bool, error) {
	if _, err := os.Lstat(filepath.Join(projectPath, xcprojFileName)); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, fmt.Errorf("inspect Xcode project %s: %w", projectPath, err)
	}
	if _, err := os.Lstat(filepath.Join(projectPath, pbxprojFileName)); err == nil {
		return false, fmt.Errorf("project %s has both %s and %s, and Xcode rejects a project with multiple definition files; remove the stale file", projectPath, pbxprojFileName, xcprojFileName)
	} else if !errors.Is(err, os.ErrNotExist) {
		return false, fmt.Errorf("inspect Xcode project %s: %w", projectPath, err)
	}
	return true, nil
}

// editXCProjBuildSettings turns the intended per-configuration settings back
// into edits of the shared objects, then proves the result reads back exactly.
func (project *structuredVersionProject) editXCProjBuildSettings(original []byte) ([]byte, error) {
	model := project.xcproj
	if !bytes.Equal(original, model.data) {
		return nil, fmt.Errorf("%s changed after it was read", xcprojFileName)
	}
	intended := make(map[string]serialized.Object, len(project.configurations))
	for _, configuration := range project.configurations {
		intended[configuration.id] = configuration.buildSettings
	}
	var edits []pbxEdit
	for _, scope := range model.scopes {
		scopeEdits, err := scope.edits(original, intended)
		if err != nil {
			return nil, err
		}
		edits = append(edits, scopeEdits...)
	}
	updated, err := applyProjectEdits(original, edits)
	if err != nil {
		return nil, err
	}
	staged, err := parseXCProjModel(updated, project.rootDir)
	if err != nil {
		return nil, fmt.Errorf("validate staged project: %w", err)
	}
	for _, scope := range staged.scopes {
		for _, configuration := range scope.configurations {
			if !reflect.DeepEqual(configuration.settings, intended[configuration.id]) {
				return nil, errors.New("staged Xcode project differs from intended build settings")
			}
		}
	}
	return updated, nil
}

func (scope *xcprojScope) edits(data []byte, intended map[string]serialized.Object) ([]pbxEdit, error) {
	changed := make(map[string]bool)
	for _, configuration := range scope.configurations {
		next := intended[configuration.id]
		for key := range configuration.settings {
			if _, exists := next[key]; !exists {
				return nil, fmt.Errorf("removing build setting %s is not supported in %s", key, xcprojFileName)
			}
		}
		for key, value := range next {
			if !reflect.DeepEqual(value, configuration.settings[key]) {
				changed[key] = true
			}
		}
	}
	var edits []pbxEdit
	for _, setting := range slices.Sorted(maps.Keys(changed)) {
		providers := make(map[string][]*xcprojConfiguration)
		for _, configuration := range scope.configurations {
			origin := configuration.origins[setting]
			providers[origin] = append(providers[origin], configuration)
		}
		for _, origin := range slices.Sorted(maps.Keys(providers)) {
			var providerEdits []pbxEdit
			var err error
			if origin == "" {
				providerEdits, err = scope.addSetting(data, setting, providers[origin], intended)
			} else {
				providerEdits, err = scope.replaceSetting(data, setting, origin, providers[origin], intended)
			}
			if err != nil {
				return nil, err
			}
			edits = append(edits, providerEdits...)
		}
	}
	return edits, nil
}

func (scope *xcprojScope) addSetting(data []byte, setting string, group []*xcprojConfiguration, intended map[string]serialized.Object) ([]pbxEdit, error) {
	var added []*xcprojConfiguration
	for _, configuration := range group {
		if _, exists := intended[configuration.id][setting]; exists {
			added = append(added, configuration)
		}
	}
	if len(added) == 0 {
		return nil, nil
	}
	if scope.settings == nil || len(scope.settings.entries) == 0 {
		return nil, fmt.Errorf("target %q has no build-settings entries to add %s to; add the setting in Xcode", scope.target, setting)
	}
	if len(added) == len(scope.configurations) && xcprojSameValue(added, setting, intended) {
		value, ok := intended[added[0].id][setting].(string)
		if !ok {
			return nil, fmt.Errorf("unsupported value for build setting %s", setting)
		}
		edit, err := insertXCProjSetting(data, scope.settings, setting, value)
		return []pbxEdit{edit}, err
	}
	var edits []pbxEdit
	for _, configuration := range added {
		value, ok := intended[configuration.id][setting].(string)
		if !ok {
			return nil, fmt.Errorf("unsupported value for build setting %s", setting)
		}
		key, err := xcprojConditionalKey(setting, configuration.name)
		if err != nil {
			return nil, err
		}
		edit, err := insertXCProjSetting(data, scope.settings, key, value)
		if err != nil {
			return nil, err
		}
		edits = append(edits, edit)
	}
	return edits, nil
}

func (scope *xcprojScope) replaceSetting(data []byte, setting, origin string, group []*xcprojConfiguration, intended map[string]serialized.Object) ([]pbxEdit, error) {
	var entry *xcprojEntry
	for index := range scope.settings.entries {
		if scope.settings.entries[index].key == origin {
			entry = &scope.settings.entries[index]
		}
	}
	if entry == nil {
		return nil, fmt.Errorf("missing build setting %s", origin)
	}
	unchanged := true
	for _, configuration := range group {
		unchanged = unchanged && reflect.DeepEqual(intended[configuration.id][setting], configuration.settings[setting])
	}
	if unchanged {
		return nil, nil
	}
	if xcprojSameValue(group, setting, intended) {
		value, ok := intended[group[0].id][setting].(string)
		if !ok {
			return nil, fmt.Errorf("unsupported value for build setting %s", setting)
		}
		encoded, err := xcprojString(value)
		if err != nil {
			return nil, err
		}
		return []pbxEdit{{start: entry.value.start, end: entry.value.end, text: encoded}}, nil
	}
	if xcprojConfigCondition.MatchString(origin) {
		return nil, fmt.Errorf("%s applies to several configurations and cannot be changed for only some of them; edit it in Xcode", origin)
	}
	// Xcode 27.1 ignores a [config=...] key next to the same unconditional key,
	// so a value that now differs by configuration has to replace the shared key.
	separator := xcprojSeparator(data, entry.start)
	var entries []string
	for _, configuration := range group {
		key, err := xcprojConditionalKey(setting, configuration.name)
		if err != nil {
			return nil, err
		}
		encodedKey, err := xcprojString(key)
		if err != nil {
			return nil, err
		}
		encodedValue := string(data[entry.value.start:entry.value.end])
		if !reflect.DeepEqual(intended[configuration.id][setting], configuration.settings[setting]) {
			value, ok := intended[configuration.id][setting].(string)
			if !ok {
				return nil, fmt.Errorf("unsupported value for build setting %s", setting)
			}
			if encodedValue, err = xcprojString(value); err != nil {
				return nil, err
			}
		}
		entries = append(entries, encodedKey+": "+encodedValue)
	}
	return []pbxEdit{{start: entry.start, end: entry.value.end, text: strings.Join(entries, ","+separator)}}, nil
}

func xcprojSameValue(group []*xcprojConfiguration, setting string, intended map[string]serialized.Object) bool {
	for _, configuration := range group {
		if !reflect.DeepEqual(intended[configuration.id][setting], intended[group[0].id][setting]) {
			return false
		}
	}
	return true
}

func xcprojConditionalKey(setting, configuration string) (string, error) {
	if strings.ContainsAny(configuration, "[],=*?\\") {
		return "", fmt.Errorf("configuration name %q cannot be used in a build setting condition", configuration)
	}
	base, conditions := setting, ""
	if index := strings.Index(setting, "["); index >= 0 {
		base, conditions = setting[:index], setting[index:]
	}
	return base + "[config=" + configuration + "]" + conditions, nil
}

func insertXCProjSetting(data []byte, settings *xcprojValue, key, value string) (pbxEdit, error) {
	encodedKey, err := xcprojString(key)
	if err != nil {
		return pbxEdit{}, err
	}
	encodedValue, err := xcprojString(value)
	if err != nil {
		return pbxEdit{}, err
	}
	text := encodedKey + ": " + encodedValue
	for _, entry := range settings.entries {
		if entry.key > key {
			return pbxEdit{start: entry.start, end: entry.start, text: text + "," + xcprojSeparator(data, entry.start)}, nil
		}
	}
	last := settings.entries[len(settings.entries)-1]
	separator := xcprojSeparator(data, last.start)
	parser := &xcprojParser{data: data, pos: last.value.end}
	if err := parser.skip(); err != nil {
		return pbxEdit{}, err
	}
	if parser.peek() == ',' {
		return pbxEdit{start: parser.pos + 1, end: parser.pos + 1, text: separator + text + ","}, nil
	}
	return pbxEdit{start: last.value.end, end: last.value.end, text: "," + separator + text}, nil
}

func xcprojSeparator(data []byte, start int) string {
	line := bytes.LastIndexByte(data[:start], '\n') + 1
	indent := string(data[line:start])
	if line == 0 || strings.Trim(indent, " \t") != "" {
		return " "
	}
	if line >= 2 && data[line-2] == '\r' {
		return "\r\n" + indent
	}
	return "\n" + indent
}

func xcprojString(value string) (string, error) {
	var encoded bytes.Buffer
	encoder := json.NewEncoder(&encoded)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		return "", err
	}
	return strings.TrimSuffix(encoded.String(), "\n"), nil
}
