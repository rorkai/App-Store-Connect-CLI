package xcode

import (
	"bytes"
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/bitrise-io/go-plist"
	"github.com/bitrise-io/go-xcode/xcodeproject/serialized"
	"golang.org/x/text/encoding/unicode"
)

type pbxToken struct {
	start, end, close int
	kind              byte
}

type pbxEntry struct {
	key, value, last, semicolon int
}

type pbxEdit struct {
	start, end int
	text       string
}

// Index lexical boundaries only; plist.Unmarshal remains the semantic parser.
func lexPBXProj(data []byte) ([]pbxToken, error) {
	var tokens []pbxToken
	var stack []int
	start := 0
	if bytes.HasPrefix(data, []byte{0xef, 0xbb, 0xbf}) {
		start = 3
	}
	for i := start; i < len(data); {
		if strings.ContainsRune(" \t\r\n\v\f", rune(data[i])) {
			i++
			continue
		}
		if i+1 < len(data) && data[i] == '/' {
			if data[i+1] == '/' {
				i += 2
				for i < len(data) && data[i] != '\n' && data[i] != '\r' {
					i++
				}
				continue
			}
			if data[i+1] == '*' {
				end := bytes.Index(data[i+2:], []byte("*/"))
				if end < 0 {
					return nil, fmt.Errorf("unterminated pbxproj comment")
				}
				i += end + 4
				continue
			}
		}
		start, kind := i, byte(0)
		switch data[i] {
		case '"':
			i++
			for i < len(data) && data[i] != '"' {
				if data[i] == '\\' {
					i++
				}
				i++
			}
			if i >= len(data) {
				return nil, fmt.Errorf("unterminated pbxproj string")
			}
			i++
		case '<':
			end := bytes.IndexByte(data[i:], '>')
			if end < 0 {
				return nil, fmt.Errorf("unterminated pbxproj data")
			}
			i += end + 1
		case '{', '}', '(', ')', '=', ';', ',':
			kind = data[i]
			i++
		default:
			for i < len(data) && !strings.ContainsRune(" \t\r\n\v\f{}()=;,\"<>", rune(data[i])) {
				i++
			}
			if i == start {
				return nil, fmt.Errorf("unexpected pbxproj byte at %d", i)
			}
		}
		token := pbxToken{start: start, end: i, close: len(tokens), kind: kind}
		switch kind {
		case '{', '(':
			stack = append(stack, len(tokens))
		case '}', ')':
			if len(stack) == 0 {
				return nil, fmt.Errorf("unbalanced pbxproj delimiters")
			}
			opening := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			if tokens[opening].kind == '{' && kind != '}' || tokens[opening].kind == '(' && kind != ')' {
				return nil, fmt.Errorf("mismatched pbxproj delimiters")
			}
			tokens[opening].close = len(tokens)
		}
		tokens = append(tokens, token)
	}
	if len(stack) != 0 {
		return nil, fmt.Errorf("unbalanced pbxproj delimiters")
	}
	return tokens, nil
}

func pbxDictionary(data []byte, tokens []pbxToken, opening int) (map[string]pbxEntry, error) {
	if opening < 0 || opening >= len(tokens) || tokens[opening].kind != '{' {
		return nil, fmt.Errorf("expected pbxproj dictionary")
	}
	entries := make(map[string]pbxEntry)
	for i := opening + 1; i < tokens[opening].close; {
		key := i
		var name string
		if _, err := plist.Unmarshal(data[tokens[key].start:tokens[key].end], &name); err != nil {
			return nil, err
		}
		i++
		value := key
		if i < len(tokens) && tokens[i].kind == '=' {
			value = i + 1
			i = value
		}
		if i >= len(tokens) {
			return nil, fmt.Errorf("missing pbxproj value for %s", name)
		}
		last := tokens[value].close
		if value != key {
			i = last + 1
		}
		if i >= len(tokens) || tokens[i].kind != ';' {
			return nil, fmt.Errorf("missing pbxproj semicolon for %s", name)
		}
		if _, exists := entries[name]; exists {
			return nil, fmt.Errorf("ambiguous duplicate pbxproj key %q", name)
		}
		entries[name] = pbxEntry{key: key, value: value, last: last, semicolon: i}
		i++
	}
	return entries, nil
}

func pbxString(value string, quoted bool) (string, error) {
	if value != "" && strings.IndexFunc(value, func(r rune) bool {
		return (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') && (r < '0' || r > '9') && !strings.ContainsRune("_./", r)
	}) < 0 && !strings.Contains(value, "//") {
		if quoted {
			return `"` + value + `"`, nil
		}
		return value, nil
	}
	encoded, err := plist.Marshal(value, plist.OpenStepFormat)
	return string(encoded), err
}

func editPBXProjBuildSettings(original []byte, project *structuredVersionProject) ([]byte, error) {
	if len(original) < 2 {
		return editUTF8PBXProjBuildSettings(original, project)
	}
	order, policy := unicode.LittleEndian, unicode.IgnoreBOM
	switch {
	case bytes.HasPrefix(original, []byte{0xfe, 0xff}):
		order, policy = unicode.BigEndian, unicode.UseBOM
	case bytes.HasPrefix(original, []byte{0xff, 0xfe}):
		policy = unicode.UseBOM
	case original[0] == 0 && original[1] != 0:
		order = unicode.BigEndian
	case original[0] != 0 && original[1] == 0:
	default:
		return editUTF8PBXProjBuildSettings(original, project)
	}
	codec := unicode.UTF16(order, policy)
	decoded, err := codec.NewDecoder().Bytes(original)
	if err != nil {
		return nil, fmt.Errorf("decode UTF-16 pbxproj: %w", err)
	}
	roundTrip, err := codec.NewEncoder().Bytes(decoded)
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(roundTrip, original) {
		return nil, fmt.Errorf("cannot preserve invalid UTF-16 pbxproj bytes")
	}
	updated, err := editUTF8PBXProjBuildSettings(decoded, project)
	if err != nil {
		return nil, err
	}
	return codec.NewEncoder().Bytes(updated)
}

func editUTF8PBXProjBuildSettings(original []byte, project *structuredVersionProject) ([]byte, error) {
	var raw serialized.Object
	if _, err := plist.Unmarshal(original, &raw); err != nil {
		return nil, err
	}
	objects, err := raw.Object("objects")
	if err != nil {
		return nil, err
	}
	tokens, err := lexPBXProj(original)
	if err != nil {
		return nil, err
	}
	root, err := pbxDictionary(original, tokens, 0)
	if err != nil {
		return nil, err
	}
	objectsEntry, ok := root["objects"]
	if !ok {
		return nil, fmt.Errorf("missing pbxproj objects")
	}
	objectEntries, err := pbxDictionary(original, tokens, objectsEntry.value)
	if err != nil {
		return nil, err
	}
	var edits []pbxEdit
	seen := make(map[string]bool)
	for _, configuration := range project.configurations {
		if seen[configuration.id] {
			continue
		}
		seen[configuration.id] = true
		object, err := objects.Object(configuration.id)
		if err != nil {
			return nil, err
		}
		old, err := object.Object("buildSettings")
		if err != nil {
			return nil, err
		}
		if reflect.DeepEqual(old, configuration.buildSettings) {
			continue
		}
		configEntry, ok := objectEntries[configuration.id]
		if !ok {
			return nil, fmt.Errorf("missing pbxproj configuration %s", configuration.id)
		}
		configEntries, err := pbxDictionary(original, tokens, configEntry.value)
		if err != nil {
			return nil, err
		}
		settingsEntry, ok := configEntries["buildSettings"]
		if !ok {
			return nil, fmt.Errorf("missing buildSettings for %s", configuration.id)
		}
		entries, err := pbxDictionary(original, tokens, settingsEntry.value)
		if err != nil {
			return nil, err
		}
		for key, oldValue := range old {
			newValue, exists := configuration.buildSettings[key]
			if exists && reflect.DeepEqual(oldValue, newValue) {
				continue
			}
			entry, ok := entries[key]
			if !ok {
				return nil, fmt.Errorf("missing pbxproj setting %q", key)
			}
			if !exists {
				// Keep surrounding whitespace and comments, including trailing comments.
				edits = append(edits, pbxEdit{start: tokens[entry.key].start, end: tokens[entry.semicolon].end})
				continue
			}
			value, ok := newValue.(string)
			if !ok {
				return nil, fmt.Errorf("unsupported pbxproj setting value for %q", key)
			}
			encoded, err := pbxString(value, original[tokens[entry.value].start] == '"')
			if err != nil {
				return nil, err
			}
			start, end := tokens[entry.value].start, tokens[entry.last].end
			if entry.key == entry.value {
				start, end = tokens[entry.key].end, tokens[entry.key].end
				encoded = " = " + encoded
			}
			edits = append(edits, pbxEdit{start: start, end: end, text: encoded})
		}
		var added []string
		for key := range configuration.buildSettings {
			if _, exists := old[key]; !exists {
				added = append(added, key)
			}
		}
		sort.Strings(added)
		if len(added) == 0 {
			continue
		}
		closing := tokens[tokens[settingsEntry.value].close].start
		position, separator, suffix := pbxInsertionStyle(original, tokens[settingsEntry.value].end, closing, entries, tokens)
		var text strings.Builder
		for _, key := range added {
			value, ok := configuration.buildSettings[key].(string)
			if !ok {
				return nil, fmt.Errorf("unsupported pbxproj setting value for %q", key)
			}
			encodedKey, err := pbxString(key, false)
			if err != nil {
				return nil, err
			}
			encodedValue, err := pbxString(value, false)
			if err != nil {
				return nil, err
			}
			text.WriteString(separator + encodedKey + " = " + encodedValue + ";" + suffix)
		}
		edits = append(edits, pbxEdit{start: position, end: position, text: text.String()})
	}
	sort.Slice(edits, func(i, j int) bool { return edits[i].start < edits[j].start })
	var updated bytes.Buffer
	position := 0
	for _, edit := range edits {
		if edit.start < position {
			return nil, fmt.Errorf("overlapping pbxproj edits")
		}
		updated.Write(original[position:edit.start])
		updated.WriteString(edit.text)
		position = edit.end
	}
	updated.Write(original[position:])
	return updated.Bytes(), nil
}

func pbxInsertionStyle(data []byte, opening, closing int, entries map[string]pbxEntry, tokens []pbxToken) (int, string, string) {
	line := bytes.LastIndexByte(data[:closing], '\n') + 1
	indent := string(data[line:closing])
	if line >= opening && strings.Trim(indent, " \t\r") == "" {
		newline := "\n"
		if line >= 2 && data[line-2] == '\r' {
			newline = "\r\n"
		}
		entryIndent := indent + "\t"
		first := closing
		for _, entry := range entries {
			if tokens[entry.key].start < first {
				first = tokens[entry.key].start
			}
		}
		if first < closing {
			entryLine := bytes.LastIndexByte(data[:first], '\n') + 1
			candidate := string(data[entryLine:first])
			if strings.Trim(candidate, " \t") == "" {
				entryIndent = candidate
			}
		}
		return line, entryIndent, newline
	}
	prefix := ""
	if closing == opening || !strings.ContainsRune(" \t\r\n", rune(data[closing-1])) {
		prefix = " "
	}
	return closing, prefix, " "
}
