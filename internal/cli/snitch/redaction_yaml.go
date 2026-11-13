package snitch

import (
	"slices"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

type yamlAnchorLocation struct {
	line      int
	anchorEnd int
}

type yamlExplicitMappingRestoration struct {
	keyLine                  int
	valueLine                int
	key                      string
	keyOriginal              string
	keyContinuationOriginals []string
	valueOriginal            string
	originalValueStart       int
	normalizedValueText      string
}

func redactKubernetesSecretData(value string) (string, bool) {
	redacted, yamlChanged := redactKubernetesSecretYAMLData(value)
	redacted, jsonChanged := redactAtEachJSONEscapeDepth(redacted, redactKubernetesSecretJSONData)
	return redacted, yamlChanged || jsonChanged
}

func redactKubernetesSecretYAMLData(value string) (string, bool) {
	value, flowChanged := redactKubernetesSecretYAMLFlowData(value)
	lines := strings.SplitAfter(value, "\n")
	explicitRestorations := normalizeYAMLExplicitMappings(lines)
	secretIndent := -1
	containerIndent := -1
	blockIndent := -1
	secretActive := false
	scalarAnchors := make(map[string]string)
	anchorLocations := make(map[string]yamlAnchorLocation)
	changed := flowChanged

	resetSecret := func() {
		secretIndent = -1
		containerIndent = -1
		blockIndent = -1
		secretActive = false
	}
	resetDocument := func() {
		resetSecret()
		clear(scalarAnchors)
		clear(anchorLocations)
	}

	for line := 0; line < len(lines); line++ {
		content, ending := splitLineEnding(lines[line])
		trimmed := strings.TrimSpace(content)
		if yamlDocumentBoundary.MatchString(trimmed) {
			resetDocument()
			continue
		}

		indent := leadingIndent(content)
		if blockIndent >= 0 {
			if trimmed == "" || strings.HasPrefix(trimmed, "#") {
				continue
			}
			if indent > blockIndent {
				lines[line] = content[:indent] + redactionMarker + ending
				changed = true
				continue
			}
			blockIndent = -1
		}

		if flowStart := yamlFlowMapStart(content); flowStart >= 0 {
			close := findYAMLFlowEnd(content, flowStart)
			if close >= flowStart {
				flow, flowChanged := redactKubernetesSecretYAMLFlowObject(content[flowStart : close+1])
				if flowChanged {
					lines[line] = content[:flowStart] + flow + content[close+1:] + ending
					changed = true
					continue
				}
			}
		}

		if containerIndent >= 0 {
			if trimmed == "" || strings.HasPrefix(trimmed, "#") {
				continue
			}
			if indent > containerIndent {
				if _, valueStart, ok := parseYAMLMappingLine(content); ok {
					if next, scalarChanged, blockScalar := redactKubernetesYAMLScalar(content, valueStart); scalarChanged {
						if alias := kubernetesYAMLScalar(content[valueStart:]); strings.HasPrefix(alias, "*") {
							location, exists := anchorLocations[strings.TrimPrefix(alias, "*")]
							if exists {
								redactKubernetesYAMLAnchorDefinition(lines, location)
							}
						}
						lines[line] = next + ending
						changed = true
						if blockScalar {
							blockIndent = indent
						} else if redactYAMLScalarContinuations(lines, line, indent, content, valueStart) {
							changed = true
						}
					}
				} else if isYAMLSequenceItemAtIndent(content, indent) {
					lines[line] = content[:indent] + "- " + redactionMarker + ending
					changed = true
				}
				continue
			}
			containerIndent = -1
		}

		key, valueStart, ok := parseYAMLMappingLine(content)
		if !ok {
			continue
		}
		value := strings.TrimSpace(content[valueStart:])
		keyIndent := yamlKeyIndent(content)
		if secretActive && keyIndent < secretIndent {
			resetSecret()
		}
		rawValue := content[valueStart:]
		if anchor := yamlAnchor.FindStringSubmatchIndex(rawValue); len(anchor) == 4 {
			anchorName := rawValue[anchor[2]:anchor[3]]
			anchorLocations[anchorName] = yamlAnchorLocation{
				line:      line,
				anchorEnd: valueStart + anchor[1],
			}
			if scalar := yamlMappingScalar(lines, line, content, valueStart); scalar != "" && scalar != value {
				scalarAnchors[anchorName] = scalar
			}
		}
		if key == "<<" && yamlEffectiveMergeKindIsSecret(lines, line, keyIndent, content, valueStart, anchorLocations, scalarAnchors) {
			secretIndent = keyIndent
			secretActive = true
			containerIndent = -1
			blockIndent = -1
		}
		if !secretActive && (strings.EqualFold(key, "data") || strings.EqualFold(key, "stringData")) && yamlSecretKindFollows(lines, line+1, keyIndent, anchorLocations, scalarAnchors) {
			secretIndent = keyIndent
			secretActive = true
		}
		if strings.EqualFold(key, "kind") {
			kind := resolveYAMLAlias(yamlMappingScalar(lines, line, content, valueStart), scalarAnchors)
			if strings.EqualFold(kind, "Secret") {
				if !secretActive || keyIndent <= secretIndent {
					secretIndent = keyIndent
				}
				secretActive = true
				containerIndent = -1
				blockIndent = -1
			} else if secretActive && keyIndent <= secretIndent {
				resetSecret()
			}
			continue
		}
		if !secretActive || keyIndent != secretIndent || !strings.EqualFold(key, "data") && !strings.EqualFold(key, "stringData") {
			continue
		}

		_, propertyOffset := trimYAMLNodeProperties(content[valueStart:])
		flowValueStart := valueStart + propertyOffset
		if flowValueStart < len(content) && content[flowValueStart] == '{' {
			close := findYAMLFlowEnd(content, flowValueStart)
			if close >= flowValueStart {
				originalFlow := content[flowValueStart : close+1]
				redactKubernetesYAMLAnchorAliases(lines, originalFlow, anchorLocations)
				flow, nestedFlowChanged := redactKubernetesSecretFlowMap(originalFlow)
				if nestedFlowChanged {
					lines[line] = content[:flowValueStart] + flow + content[close+1:] + ending
					changed = true
				}
				continue
			}

			remaining := strings.Join(lines[line:], "")
			close = findYAMLFlowEnd(remaining, flowValueStart)
			if close >= flowValueStart {
				originalFlow := remaining[flowValueStart : close+1]
				redactKubernetesYAMLAnchorAliases(lines, originalFlow, anchorLocations)
				flow, nestedFlowChanged := redactKubernetesSecretFlowMap(originalFlow)
				if nestedFlowChanged {
					updated := remaining[:flowValueStart] + flow + remaining[close+1:]
					lines = append(lines[:line], strings.SplitAfter(updated, "\n")...)
					changed = true
				}
				continue
			}

			lines[line] = content[:flowValueStart] + redactionMarker + ending
			blockIndent = keyIndent
			changed = true
			continue
		}
		if remainder, _ := trimYAMLNodeProperties(value); value != "" && (strings.TrimSpace(remainder) == "" || strings.HasPrefix(strings.TrimSpace(remainder), "#")) {
			containerIndent = keyIndent
			continue
		}
		if value != "" && !strings.HasPrefix(value, "#") {
			if value == redactionMarker || value == privateKeyRedactionMarker {
				continue
			}
			if alias := kubernetesYAMLScalar(value); strings.HasPrefix(alias, "*") {
				if location, exists := anchorLocations[strings.TrimPrefix(alias, "*")]; exists {
					redactKubernetesYAMLAnchorDefinition(lines, location)
				}
			}
			lines[line] = content[:valueStart] + redactionMarker + ending
			changed = true
			continue
		}
		containerIndent = keyIndent
	}
	restoreYAMLExplicitMappings(lines, explicitRestorations)
	return strings.Join(lines, ""), changed
}

func yamlMergeHasSecretKind(lines []string, line int, content string, valueStart int, locations map[string]yamlAnchorLocation, scalarAnchors map[string]string) bool {
	value := content[valueStart:]
	if anchor := yamlAnchor.FindStringSubmatchIndex(value); len(anchor) == 4 {
		anchorName := value[anchor[2]:anchor[3]]
		location, exists := locations[anchorName]
		if !exists {
			location = yamlAnchorLocation{line: line, anchorEnd: valueStart + anchor[1]}
		}
		if yamlMappingAnchorHasSecretKind(lines, location, scalarAnchors) {
			return true
		}
	}
	remainder, _ := trimYAMLNodeProperties(value)
	remainder = strings.TrimSpace(remainder)
	if strings.HasPrefix(remainder, "{") {
		return yamlFlowMappingHasSecretKind(remainder, scalarAnchors)
	}
	if !strings.HasPrefix(remainder, "*") && !strings.HasPrefix(remainder, "[") {
		return false
	}
	for _, alias := range yamlValueAlias.FindAllStringSubmatch(remainder, -1) {
		if location, exists := locations[alias[1]]; exists && yamlMappingAnchorHasSecretKind(lines, location, scalarAnchors) {
			return true
		}
	}
	return false
}

func yamlEffectiveMergeKindIsSecret(lines []string, line, keyIndent int, content string, valueStart int, locations map[string]yamlAnchorLocation, scalarAnchors map[string]string) bool {
	if explicitKind, exists := yamlExplicitKindForMapping(lines, line, keyIndent, scalarAnchors); exists {
		return strings.EqualFold(explicitKind, "Secret")
	}
	return yamlMergeHasSecretKind(lines, line, content, valueStart, locations, scalarAnchors)
}

func yamlExplicitKindForMapping(lines []string, currentLine, mappingIndent int, scalarAnchors map[string]string) (string, bool) {
	start, end := yamlMappingLines(lines, currentLine, mappingIndent)
	for line := start; line < end; line++ {
		content, _ := splitLineEnding(lines[line])
		key, valueStart, ok := parseYAMLMappingLine(content)
		if !ok || yamlKeyIndent(content) != mappingIndent || !strings.EqualFold(key, "kind") {
			continue
		}
		kind := resolveYAMLAlias(yamlMappingScalar(lines, line, content, valueStart), scalarAnchors)
		return kind, true
	}
	return "", false
}

// yamlMappingLines returns the range of lines around line that belong to the
// mapping whose keys are indented by mappingIndent.
func yamlMappingLines(lines []string, line, mappingIndent int) (int, int) {
	start := line
	for start > 0 && yamlLineBelongsToMapping(lines[start-1], mappingIndent) {
		start--
	}
	end := line + 1
	for end < len(lines) && yamlLineBelongsToMapping(lines[end], mappingIndent) {
		end++
	}
	return start, end
}

func yamlLineBelongsToMapping(line string, mappingIndent int) bool {
	content, _ := splitLineEnding(line)
	trimmed := strings.TrimSpace(content)
	if trimmed == "" || strings.HasPrefix(trimmed, "#") {
		return true
	}
	if yamlDocumentBoundary.MatchString(trimmed) || leadingIndent(content) < mappingIndent {
		return false
	}
	return !isYAMLSequenceItemAtIndent(content, leadingIndent(content)) || yamlKeyIndent(content) == mappingIndent
}

func yamlMappingAnchorHasSecretKind(lines []string, location yamlAnchorLocation, scalarAnchors map[string]string) bool {
	if location.line < 0 || location.line >= len(lines) {
		return false
	}
	content, _ := splitLineEnding(lines[location.line])
	if location.anchorEnd < len(content) {
		remainder := strings.TrimSpace(content[location.anchorEnd:])
		if strings.HasPrefix(remainder, "{") && yamlFlowMappingHasSecretKind(remainder, scalarAnchors) {
			return true
		}
	}

	parentIndent := yamlKeyIndent(content)
	childIndent := -1
	for line := location.line + 1; line < len(lines); line++ {
		candidate, _ := splitLineEnding(lines[line])
		trimmed := strings.TrimSpace(candidate)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if yamlDocumentBoundary.MatchString(trimmed) || leadingIndent(candidate) <= parentIndent {
			break
		}
		key, valueStart, ok := parseYAMLMappingLine(candidate)
		if !ok {
			continue
		}
		keyIndent := yamlKeyIndent(candidate)
		if childIndent < 0 {
			childIndent = keyIndent
		}
		if keyIndent != childIndent || !strings.EqualFold(key, "kind") {
			continue
		}
		kind := resolveYAMLAlias(yamlMappingScalar(lines, line, candidate, valueStart), scalarAnchors)
		return strings.EqualFold(kind, "Secret")
	}
	return false
}

func yamlFlowMappingHasSecretKind(flow string, scalarAnchors map[string]string) bool {
	close := findYAMLFlowEnd(flow, 0)
	if len(flow) < 2 || flow[0] != '{' || close < 1 {
		return false
	}
	secret := false
	eachYAMLFlowProperty(flow[:close+1], func(property yamlFlowProperty) bool {
		if !strings.EqualFold(property.key, "kind") {
			return true
		}
		secret = strings.EqualFold(resolveYAMLAlias(kubernetesYAMLScalar(property.value), scalarAnchors), "Secret")
		return false
	})
	return secret
}

// yamlFlowProperty is one key: value pair of a YAML flow mapping. value spans
// flow[start:end] without surrounding whitespace.
type yamlFlowProperty struct {
	key        string
	value      string
	start, end int
}

// eachYAMLFlowProperty visits the properties of a {key: value} flow mapping
// until visit returns false. It reports false for a malformed mapping.
func eachYAMLFlowProperty(flow string, visit func(yamlFlowProperty) bool) bool {
	for cursor := 1; cursor < len(flow)-1; {
		cursor = skipYAMLFlowSpaceAndComments(flow, cursor, len(flow)-1)
		if cursor >= len(flow)-1 {
			break
		}
		colon := findKubernetesFlowColon(flow, cursor)
		if colon < 0 {
			return false
		}
		valueStart := colon + 1
		for valueStart < len(flow)-1 && isYAMLFlowSpace(flow[valueStart]) {
			valueStart++
		}
		valueEnd := findKubernetesFlowValueEnd(flow, valueStart)
		if valueEnd < valueStart {
			return false
		}
		trimmedEnd := valueEnd
		for trimmedEnd > valueStart && isYAMLFlowSpace(flow[trimmedEnd-1]) {
			trimmedEnd--
		}
		if !visit(yamlFlowProperty{key: decodeYAMLMappingKey(flow[cursor:colon]), value: flow[valueStart:trimmedEnd], start: valueStart, end: trimmedEnd}) {
			return true
		}
		cursor = valueEnd
	}
	return true
}

func parseYAMLFlowMapping(flow string) ([]yamlFlowProperty, bool) {
	var properties []yamlFlowProperty
	ok := eachYAMLFlowProperty(flow, func(property yamlFlowProperty) bool {
		properties = append(properties, property)
		return true
	})
	return properties, ok
}

func isYAMLFlowSpace(character byte) bool {
	return character == ' ' || character == '\t' || character == '\r' || character == '\n'
}

// resolveYAMLAlias returns the scalar an *alias names, or scalar itself.
func resolveYAMLAlias(scalar string, scalarAnchors map[string]string) string {
	if alias, ok := strings.CutPrefix(scalar, "*"); ok {
		return scalarAnchors[alias]
	}
	return scalar
}

func redactKubernetesYAMLAnchorAliases(lines []string, value string, locations map[string]yamlAnchorLocation) {
	for _, alias := range yamlValueAlias.FindAllStringSubmatch(value, -1) {
		if location, exists := locations[alias[1]]; exists {
			redactKubernetesYAMLAnchorDefinition(lines, location)
		}
	}
}

func redactKubernetesYAMLAnchorDefinition(lines []string, location yamlAnchorLocation) {
	if location.line < 0 || location.line >= len(lines) {
		return
	}
	content, ending := splitLineEnding(lines[location.line])
	cursor := location.anchorEnd
	for cursor < len(content) && (content[cursor] == ' ' || content[cursor] == '\t') {
		cursor++
	}
	if cursor >= len(content) || content[cursor] == '#' {
		prefix := content[:cursor]
		if prefix != "" && prefix[len(prefix)-1] != ' ' && prefix[len(prefix)-1] != '\t' {
			prefix += " "
		}
		lines[location.line] = prefix + redactionMarker + ending
		indent := leadingIndent(content)
		for line := location.line + 1; line < len(lines); line++ {
			nextContent, _ := splitLineEnding(lines[line])
			trimmed := strings.TrimSpace(nextContent)
			if trimmed != "" && leadingIndent(nextContent) <= indent {
				break
			}
			if trimmed != "" {
				lines[line] = ""
			}
		}
		return
	}
	if strings.HasPrefix(content[cursor:], redactionMarker) {
		return
	}
	redactYAMLScalarContinuations(lines, location.line, leadingIndent(content), content, cursor)
	lines[location.line] = content[:cursor] + redactionMarker + ending
}

// redactYAMLScalarContinuations clears the continuation lines of the scalar
// at content[valueStart:], a multi-line plain scalar indented beyond indent or
// a quoted scalar that does not close on its first line.
func redactYAMLScalarContinuations(lines []string, line, indent int, content string, valueStart int) bool {
	value, _ := trimYAMLNodeProperties(content[valueStart:])
	value = strings.TrimSpace(value)
	if value == "" || strings.HasPrefix(value, "[") || strings.HasPrefix(value, "{") {
		return false
	}

	quote := byte(0)
	if value[0] == '\'' || value[0] == '"' {
		quote = value[0]
		if yamlQuotedScalarCloses(value[1:], quote) {
			return false
		}
	}

	changed := false
	for nextLine := line + 1; nextLine < len(lines); nextLine++ {
		nextContent, _ := splitLineEnding(lines[nextLine])
		trimmed := strings.TrimSpace(nextContent)
		if quote == 0 && trimmed != "" && leadingIndent(nextContent) <= indent {
			break
		}
		if quote != 0 && yamlQuotedScalarCloses(nextContent, quote) {
			lines[nextLine] = ""
			return true
		}
		if trimmed != "" {
			lines[nextLine] = ""
			changed = true
		}
	}
	return changed
}

func yamlQuotedScalarCloses(value string, quote byte) bool {
	for index := 0; index < len(value); index++ {
		if quote == '"' && value[index] == '\\' {
			index++
			continue
		}
		if value[index] != quote {
			continue
		}
		if quote == '\'' && index+1 < len(value) && value[index+1] == '\'' {
			index++
			continue
		}
		return true
	}
	return false
}

func normalizeYAMLExplicitMappings(lines []string) []yamlExplicitMappingRestoration {
	var restorations []yamlExplicitMappingRestoration
	for line := 0; line < len(lines); line++ {
		content, ending := splitLineEnding(lines[line])
		cursor := yamlKeyIndent(content)
		if cursor >= len(content) || content[cursor] != '?' || cursor+1 >= len(content) || content[cursor+1] != ' ' && content[cursor+1] != '\t' {
			continue
		}

		keyText := strings.TrimSpace(content[cursor+1:])
		if comment := strings.Index(keyText, " #"); comment >= 0 {
			keyText = strings.TrimSpace(keyText[:comment])
		}
		key, valueLine, blockScalarKey := decodeYAMLExplicitBlockScalarKey(lines, line, cursor, keyText)
		scalarKey := blockScalarKey
		if !blockScalarKey {
			key, scalarKey = decodeYAMLExplicitScalarKey(keyText)
			valueLine = line + 1
			for valueLine < len(lines) {
				valueContent, _ := splitLineEnding(lines[valueLine])
				trimmed := strings.TrimSpace(valueContent)
				if trimmed != "" && !strings.HasPrefix(trimmed, "#") {
					break
				}
				valueLine++
			}
		}
		if !scalarKey {
			continue
		}
		if valueLine >= len(lines) {
			continue
		}

		valueContent, valueEnding := splitLineEnding(lines[valueLine])
		indicator := leadingIndent(valueContent)
		if indicator != cursor || indicator >= len(valueContent) || valueContent[indicator] != ':' || indicator+1 < len(valueContent) && valueContent[indicator+1] != ' ' && valueContent[indicator+1] != '\t' {
			continue
		}
		originalValueStart := indicator + 1
		for originalValueStart < len(valueContent) && (valueContent[originalValueStart] == ' ' || valueContent[originalValueStart] == '\t') {
			originalValueStart++
		}
		value := valueContent[originalValueStart:]

		restorations = append(restorations, yamlExplicitMappingRestoration{
			keyLine:                  line,
			valueLine:                valueLine,
			key:                      key,
			keyOriginal:              lines[line],
			keyContinuationOriginals: append([]string(nil), lines[line+1:valueLine]...),
			valueOriginal:            lines[valueLine],
			originalValueStart:       originalValueStart,
			normalizedValueText:      value,
		})
		normalizedKey := key
		if blockScalarKey {
			normalizedKey = strconv.Quote(key)
			for continuation := line + 1; continuation < valueLine; continuation++ {
				_, continuationEnding := splitLineEnding(lines[continuation])
				lines[continuation] = continuationEnding
			}
		}
		normalized := content[:cursor] + normalizedKey + ":"
		if value != "" {
			normalized += " " + value
		}
		lines[line] = normalized + ending
		lines[valueLine] = valueEnding
	}
	return restorations
}

func restoreYAMLExplicitMappings(lines []string, restorations []yamlExplicitMappingRestoration) {
	for _, restoration := range restorations {
		valueLine := restoration.valueOriginal
		if !strings.EqualFold(restoration.key, "kind") && restoration.keyLine < len(lines) {
			normalizedContent, _ := splitLineEnding(lines[restoration.keyLine])
			_, valueStart, ok := parseYAMLMappingLine(normalizedContent)
			if ok {
				redactedValue := normalizedContent[valueStart:]
				if redactedValue != restoration.normalizedValueText {
					originalContent, originalEnding := splitLineEnding(restoration.valueOriginal)
					valueLine = originalContent[:restoration.originalValueStart] + redactedValue + originalEnding
				}
			}
		}
		if restoration.keyLine < len(lines) {
			lines[restoration.keyLine] = restoration.keyOriginal
		}
		for offset, original := range restoration.keyContinuationOriginals {
			continuation := restoration.keyLine + 1 + offset
			if continuation < len(lines) {
				lines[continuation] = original
			}
		}
		if restoration.valueLine < len(lines) {
			lines[restoration.valueLine] = valueLine
		}
	}
}

func decodeYAMLExplicitBlockScalarKey(lines []string, line, cursor int, keyText string) (string, int, bool) {
	header, _ := trimYAMLNodeProperties(keyText)
	header = strings.TrimSpace(header)
	if comment := strings.Index(header, " #"); comment >= 0 {
		header = strings.TrimSpace(header[:comment])
	}
	if !yamlBlockScalarIndicator.MatchString(header) {
		return "", 0, false
	}

	valueLine := line + 1
	for valueLine < len(lines) {
		content, _ := splitLineEnding(lines[valueLine])
		indent := leadingIndent(content)
		if indent == cursor && indent < len(content) && content[indent] == ':' &&
			(indent+1 == len(content) || content[indent+1] == ' ' || content[indent+1] == '\t') {
			break
		}
		trimmed := strings.TrimSpace(content)
		if trimmed != "" && !strings.HasPrefix(trimmed, "#") && indent <= cursor {
			return "", 0, false
		}
		valueLine++
	}
	if valueLine >= len(lines) {
		return "", 0, false
	}

	var fragment strings.Builder
	for keyLine := line; keyLine < valueLine; keyLine++ {
		content, ending := splitLineEnding(lines[keyLine])
		if len(content) < cursor {
			return "", 0, false
		}
		fragment.WriteString(content[cursor:])
		if ending == "" {
			ending = "\n"
		}
		fragment.WriteString(ending)
	}
	fragment.WriteString(": null\n")

	var document yaml.Node
	if err := yaml.Unmarshal([]byte(fragment.String()), &document); err != nil || len(document.Content) != 1 {
		return "", 0, false
	}
	mapping := document.Content[0]
	if mapping.Kind != yaml.MappingNode || len(mapping.Content) < 2 || mapping.Content[0].Kind != yaml.ScalarNode || mapping.Content[0].Value == "" {
		return "", 0, false
	}
	return mapping.Content[0].Value, valueLine, true
}

func parseYAMLMappingLine(content string) (string, int, bool) {
	cursor := leadingIndent(content)
	if cursor >= len(content) || content[cursor] == '#' {
		return "", 0, false
	}
	if content[cursor] == '-' {
		if cursor+1 >= len(content) || (content[cursor+1] != ' ' && content[cursor+1] != '\t') {
			return "", 0, false
		}
		cursor++
		for cursor < len(content) && (content[cursor] == ' ' || content[cursor] == '\t') {
			cursor++
		}
	}
	keyStart := cursor
	var quote byte
	for cursor < len(content) {
		switch content[cursor] {
		case '\'', '"':
			switch quote {
			case 0:
				quote = content[cursor]
			case content[cursor]:
				quote = 0
			}
		case ':':
			if quote == 0 {
				key := decodeYAMLMappingKey(content[keyStart:cursor])
				if key == "" {
					return "", 0, false
				}
				valueStart := cursor + 1
				for valueStart < len(content) && (content[valueStart] == ' ' || content[valueStart] == '\t') {
					valueStart++
				}
				return key, valueStart, true
			}
		}
		cursor++
	}
	return "", 0, false
}

func kubernetesYAMLScalar(value string) string {
	value = strings.TrimSpace(value)
	if comment := strings.Index(value, " #"); comment >= 0 {
		value = strings.TrimSpace(value[:comment])
	}
	value, _ = trimYAMLNodeProperties(value)
	return decodeYAMLScalar(value)
}

func decodeYAMLScalar(value string) string {
	value = strings.TrimSpace(value)
	if len(value) < 2 || value[0] != value[len(value)-1] {
		return strings.Trim(value, "\"'")
	}
	switch value[0] {
	case '"':
		if decoded, err := strconv.Unquote(value); err == nil {
			return decoded
		}
	case '\'':
		return strings.ReplaceAll(value[1:len(value)-1], "''", "'")
	}
	return strings.Trim(value, "\"'")
}

func decodeYAMLMappingKey(value string) string {
	value, _ = trimYAMLNodeProperties(value)
	return decodeYAMLScalar(value)
}

func yamlMappingScalar(lines []string, line int, content string, valueStart int) string {
	if scalar, ok := decodeYAMLBlockScalarValue(lines, line, content, valueStart); ok {
		return strings.TrimSpace(scalar)
	}
	return kubernetesYAMLScalar(content[valueStart:])
}

func decodeYAMLBlockScalarValue(lines []string, line int, content string, valueStart int) (string, bool) {
	header, _ := trimYAMLNodeProperties(content[valueStart:])
	header = strings.TrimSpace(header)
	if comment := strings.Index(header, " #"); comment >= 0 {
		header = strings.TrimSpace(header[:comment])
	}
	if !yamlBlockScalarIndicator.MatchString(header) {
		return "", false
	}

	keyIndent := yamlKeyIndent(content)
	end := line + 1
	for end < len(lines) {
		candidate, _ := splitLineEnding(lines[end])
		trimmed := strings.TrimSpace(candidate)
		if trimmed != "" && !strings.HasPrefix(trimmed, "#") && leadingIndent(candidate) <= keyIndent {
			break
		}
		end++
	}

	var fragment strings.Builder
	fragment.WriteString("value: ")
	fragment.WriteString(content[valueStart:])
	fragment.WriteByte('\n')
	for continuation := line + 1; continuation < end; continuation++ {
		candidate, ending := splitLineEnding(lines[continuation])
		trimmed := strings.TrimSpace(candidate)
		if trimmed == "" {
			if ending == "" {
				ending = "\n"
			}
			fragment.WriteString(ending)
			continue
		}
		if strings.HasPrefix(trimmed, "#") && leadingIndent(candidate) <= keyIndent {
			fragment.WriteString(candidate)
			if ending == "" {
				ending = "\n"
			}
			fragment.WriteString(ending)
			continue
		}
		if len(candidate) < keyIndent {
			return "", false
		}
		fragment.WriteString(candidate[keyIndent:])
		if ending == "" {
			ending = "\n"
		}
		fragment.WriteString(ending)
	}

	var document yaml.Node
	if err := yaml.Unmarshal([]byte(fragment.String()), &document); err != nil || len(document.Content) != 1 {
		return "", false
	}
	mapping := document.Content[0]
	if mapping.Kind != yaml.MappingNode || len(mapping.Content) != 2 || mapping.Content[1].Kind != yaml.ScalarNode {
		return "", false
	}
	return mapping.Content[1].Value, true
}

func decodeYAMLExplicitScalarKey(value string) (string, bool) {
	value, _ = trimYAMLNodeProperties(value)
	value = strings.TrimSpace(value)
	if value == "" || value[0] == '[' || value[0] == '{' {
		return "", false
	}
	key := decodeYAMLScalar(value)
	return key, key != ""
}

func trimYAMLNodeProperties(value string) (string, int) {
	offset := len(value) - len(strings.TrimLeft(value, " \t\r\n"))
	for offset < len(value) && (value[offset] == '!' || value[offset] == '&') {
		remaining := value[offset:]
		if strings.HasPrefix(remaining, "!<") {
			end := strings.IndexByte(remaining, '>')
			if end < 0 {
				break
			}
			offset += end + 1
		} else {
			end := strings.IndexAny(remaining, " \t\r\n")
			if end < 0 {
				return "", len(value)
			}
			offset += end
		}
		for offset < len(value) && (value[offset] == ' ' || value[offset] == '\t' || value[offset] == '\r' || value[offset] == '\n') {
			offset++
		}
	}
	return value[offset:], offset
}

func redactKubernetesSecretYAMLFlowData(value string) (string, bool) {
	lower := strings.ToLower(value)
	if !strings.Contains(lower, "kind") || !strings.Contains(lower, "secret") ||
		!strings.Contains(lower, "data") && !strings.Contains(lower, "stringdata") {
		return value, false
	}
	redacted := value
	changed := false
	for lineStart := 0; lineStart < len(redacted); {
		lineEnd := len(redacted)
		if relativeEnd := strings.IndexByte(redacted[lineStart:], '\n'); relativeEnd >= 0 {
			lineEnd = lineStart + relativeEnd
		}
		relative := yamlFlowMapStart(redacted[lineStart:lineEnd])
		if relative < 0 {
			if lineEnd == len(redacted) {
				break
			}
			lineStart = lineEnd + 1
			continue
		}
		open := lineStart + relative
		close := findYAMLFlowEnd(redacted, open)
		if close < open {
			break
		}
		flow, flowChanged := redactKubernetesSecretYAMLFlowObject(redacted[open : close+1])
		if flowChanged {
			redacted = redacted[:open] + flow + redacted[close+1:]
			changed = true
			close = open + len(flow) - 1
		}
		nextLine := strings.IndexByte(redacted[close+1:], '\n')
		if nextLine < 0 {
			break
		}
		lineStart = close + 1 + nextLine + 1
	}
	return redacted, changed
}

func yamlSecretKindFollows(lines []string, start, secretIndent int, anchorLocations map[string]yamlAnchorLocation, scalarAnchors map[string]string) bool {
	for line := start; line < len(lines); line++ {
		content, _ := splitLineEnding(lines[line])
		trimmed := strings.TrimSpace(content)
		if yamlDocumentBoundary.MatchString(trimmed) {
			return false
		}
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		indent := leadingIndent(content)
		if indent < secretIndent || indent == secretIndent && indent < len(content) && content[indent] == '-' {
			return false
		}
		key, valueStart, ok := parseYAMLMappingLine(content)
		if !ok {
			continue
		}
		keyIndent := yamlKeyIndent(content)
		if keyIndent < secretIndent {
			return false
		}
		if keyIndent != secretIndent {
			continue
		}
		if key == "<<" && yamlEffectiveMergeKindIsSecret(lines, line, keyIndent, content, valueStart, anchorLocations, scalarAnchors) {
			return true
		}
		if strings.EqualFold(key, "kind") {
			kind := resolveYAMLAlias(yamlMappingScalar(lines, line, content, valueStart), scalarAnchors)
			return strings.EqualFold(kind, "Secret")
		}
	}
	return false
}

func yamlFlowMapStart(content string) int {
	start := leadingIndent(content)
	sequenceItem := false
	if start < len(content) && content[start] == '-' {
		if start+1 >= len(content) || (content[start+1] != ' ' && content[start+1] != '\t') {
			return -1
		}
		sequenceItem = true
		start++
		for start < len(content) && (content[start] == ' ' || content[start] == '\t') {
			start++
		}
	}
	if !sequenceItem && leadingIndent(content) != 0 {
		return -1
	}
	if start < len(content) && content[start] == '{' {
		return start
	}
	return -1
}

func redactKubernetesSecretYAMLFlowObject(flow string) (string, bool) {
	if len(flow) < 2 || flow[0] != '{' || flow[len(flow)-1] != '}' {
		return flow, false
	}
	properties, ok := parseYAMLFlowMapping(flow)
	if !ok || !slices.ContainsFunc(properties, func(property yamlFlowProperty) bool {
		return strings.EqualFold(property.key, "kind") && strings.EqualFold(kubernetesYAMLScalar(property.value), "Secret")
	}) {
		return flow, false
	}
	var replacements []textReplacement
	for _, property := range properties {
		if !strings.EqualFold(property.key, "data") && !strings.EqualFold(property.key, "stringData") || property.value == "" {
			continue
		}
		if property.value[0] == '{' {
			if child, childChanged := redactKubernetesSecretFlowMap(property.value); childChanged {
				replacements = append(replacements, textReplacement{property.start, property.end, child})
			}
			continue
		}
		if property.value != redactionMarker {
			replacements = append(replacements, textReplacement{property.start, property.end, redactionMarker})
		}
	}
	return applyTextReplacements(flow, replacements), len(replacements) > 0
}

func skipYAMLFlowSpaceAndComments(value string, start, limit int) int {
	for start < limit {
		for start < limit && (value[start] == ' ' || value[start] == '\t' || value[start] == '\r' || value[start] == '\n' || value[start] == ',') {
			start++
		}
		if start >= limit || value[start] != '#' {
			return start
		}
		for start < limit && value[start] != '\r' && value[start] != '\n' {
			start++
		}
	}
	return start
}

func redactKubernetesYAMLScalar(content string, valueStart int) (string, bool, bool) {
	if valueStart >= len(content) {
		return content, false, false
	}
	value := strings.TrimSpace(content[valueStart:])
	if value == "" || strings.HasPrefix(value, "#") || value == redactionMarker {
		return content, false, false
	}
	blockScalar := strings.HasPrefix(value, "|") || strings.HasPrefix(value, ">")
	return content[:valueStart] + redactionMarker, true, blockScalar
}

func redactKubernetesSecretFlowMap(flow string) (string, bool) {
	if len(flow) < 2 || flow[0] != '{' || flow[len(flow)-1] != '}' {
		return flow, false
	}
	type span struct{ start, end int }
	var replacements []span
	for cursor := 1; cursor < len(flow)-1; {
		for cursor < len(flow)-1 && (flow[cursor] == ' ' || flow[cursor] == '\t' || flow[cursor] == '\r' || flow[cursor] == '\n' || flow[cursor] == ',') {
			cursor++
		}
		if cursor >= len(flow)-1 {
			break
		}
		colon := findKubernetesFlowColon(flow, cursor)
		if colon < 0 {
			break
		}
		valueStart := colon + 1
		for valueStart < len(flow)-1 && (flow[valueStart] == ' ' || flow[valueStart] == '\t' || flow[valueStart] == '\r' || flow[valueStart] == '\n') {
			valueStart++
		}
		valueEnd := findKubernetesFlowValueEnd(flow, valueStart)
		if valueEnd < valueStart {
			break
		}
		trimmedEnd := valueEnd
		for trimmedEnd > valueStart && (flow[trimmedEnd-1] == ' ' || flow[trimmedEnd-1] == '\t' || flow[trimmedEnd-1] == '\r' || flow[trimmedEnd-1] == '\n') {
			trimmedEnd--
		}
		if trimmedEnd <= valueStart || flow[valueStart:trimmedEnd] == redactionMarker {
			cursor = valueEnd
			continue
		}
		replacementStart, replacementEnd := valueStart, trimmedEnd
		if trimmedEnd-valueStart >= 2 && (flow[valueStart] == '\'' || flow[valueStart] == '"') && flow[trimmedEnd-1] == flow[valueStart] {
			if flow[valueStart+1:trimmedEnd-1] == redactionMarker {
				cursor = valueEnd
				continue
			}
			replacementStart++
			replacementEnd--
		}
		replacements = append(replacements, span{start: replacementStart, end: replacementEnd})
		cursor = valueEnd
	}
	if len(replacements) == 0 {
		return flow, false
	}
	redacted := flow
	for index := len(replacements) - 1; index >= 0; index-- {
		replacement := replacements[index]
		redacted = redacted[:replacement.start] + redactionMarker + redacted[replacement.end:]
	}
	return redacted, true
}

// findYAMLFlowDelimiter returns the index of the first stop byte or closing
// bracket outside quotes and nested brackets, or -1.
func findYAMLFlowDelimiter(value string, start int, stop byte) int {
	depth := 0
	var quote byte
	for index := start; index < len(value); index++ {
		if quote != 0 {
			index, quote = stepYAMLQuotedScalar(value, index, quote)
			continue
		}
		switch value[index] {
		case '"', '\'':
			quote = value[index]
		case '[', '{':
			depth++
		case ']', '}':
			if depth == 0 {
				return index
			}
			depth--
		case stop:
			if depth == 0 {
				return index
			}
		}
	}
	return -1
}

// stepYAMLQuotedScalar reads value[index] inside a scalar quoted by quote. It
// returns the index of the last byte read, which includes an escaped or
// doubled quote, and quote, or 0 when the scalar closed.
func stepYAMLQuotedScalar(value string, index int, quote byte) (int, byte) {
	switch {
	case quote == '"' && value[index] == '\\':
		return index + 1, quote
	case quote == '\'' && value[index] == '\'' && index+1 < len(value) && value[index+1] == '\'':
		return index + 1, quote
	case value[index] == quote:
		return index, 0
	}
	return index, quote
}

func findKubernetesFlowColon(value string, start int) int {
	if colon := findYAMLFlowDelimiter(value, start, ':'); colon >= 0 && value[colon] == ':' {
		return colon
	}
	return -1
}

func findKubernetesFlowValueEnd(value string, start int) int {
	if end := findYAMLFlowDelimiter(value, start, ','); end >= 0 {
		return end
	}
	return len(value) - 1
}

func redactSensitiveYAMLNameValuePairs(value string) (string, bool) {
	lines := strings.SplitAfter(value, "\n")
	var redacted strings.Builder
	redacted.Grow(len(value))
	changed := false
	documentStart := 0
	for line := 0; line < len(lines); line++ {
		content, _ := splitLineEnding(lines[line])
		if !yamlDocumentBoundary.MatchString(strings.TrimSpace(content)) {
			continue
		}
		document, documentChanged := redactSensitiveYAMLNameValueDocument(lines[documentStart:line])
		redacted.WriteString(document)
		redacted.WriteString(lines[line])
		changed = changed || documentChanged
		documentStart = line + 1
	}
	document, documentChanged := redactSensitiveYAMLNameValueDocument(lines[documentStart:])
	redacted.WriteString(document)
	return redacted.String(), changed || documentChanged
}

func redactSensitiveYAMLNameValueDocument(lines []string) (string, bool) {
	scalarAnchors := collectYAMLScalarAnchors(lines)
	blockChanged := redactSensitiveYAMLBlockNameValuePairs(lines, scalarAnchors)
	redacted := strings.Join(lines, "")
	redacted, flowChanged := redactSensitiveYAMLFlowNameValuePairs(redacted, scalarAnchors)
	return redacted, blockChanged || flowChanged
}

func collectYAMLScalarAnchors(lines []string) map[string]string {
	anchors := make(map[string]string)
	for line := range lines {
		content, _ := splitLineEnding(lines[line])
		_, valueStart, ok := parseYAMLMappingLine(content)
		if !ok {
			continue
		}
		rawValue := content[valueStart:]
		anchor := yamlAnchor.FindStringSubmatchIndex(rawValue)
		if len(anchor) != 4 {
			continue
		}
		scalar := yamlMappingScalar(lines, line, content, valueStart)
		if scalar == "" || strings.HasPrefix(scalar, "*") {
			continue
		}
		anchors[rawValue[anchor[2]:anchor[3]]] = scalar
	}
	return anchors
}

func redactSensitiveYAMLBlockNameValuePairs(lines []string, scalarAnchors map[string]string) bool {
	changed := false
	for line := 0; line < len(lines); line++ {
		content, _ := splitLineEnding(lines[line])
		key, valueStart, ok := parseYAMLMappingLine(content)
		if !ok || !strings.EqualFold(key, "name") {
			continue
		}
		name := resolveYAMLAlias(yamlMappingScalar(lines, line, content, valueStart), scalarAnchors)
		if !yamlCredentialNamePattern.MatchString(name) {
			continue
		}

		mappingIndent := yamlKeyIndent(content)
		start, end := yamlMappingLines(lines, line, mappingIndent)
		if start > 0 {
			previous, _ := splitLineEnding(lines[start-1])
			previousIndent := leadingIndent(previous)
			if isYAMLSequenceItemAtIndent(previous, previousIndent) && yamlKeyIndent(previous) == mappingIndent {
				start--
			}
		}
		for candidate := start; candidate < end; candidate++ {
			valueContent, ending := splitLineEnding(lines[candidate])
			valueKey, candidateValueStart, valid := parseYAMLMappingLine(valueContent)
			if !valid || yamlKeyIndent(valueContent) != mappingIndent || !strings.EqualFold(valueKey, "value") {
				continue
			}
			scalar := kubernetesYAMLScalar(valueContent[candidateValueStart:])
			if scalar == "" || scalar == redactionMarker || scalar == privateKeyRedactionMarker {
				continue
			}
			redactYAMLScalarContinuations(lines, candidate, mappingIndent, valueContent, candidateValueStart)
			lines[candidate] = valueContent[:candidateValueStart] + redactionMarker + ending
			changed = true
		}
	}
	return changed
}

func redactSensitiveYAMLFlowNameValuePairs(value string, scalarAnchors map[string]string) (string, bool) {
	return redactEachObject(value, nextYAMLFlowObjectStart, findYAMLFlowEnd, false, func(object string) (string, bool) {
		return redactSensitiveYAMLFlowNameValueObject(object, scalarAnchors)
	})
}

func nextYAMLFlowObjectStart(value string, start int) int {
	var quote byte
	inComment := false
	for index := start; index < len(value); index++ {
		if inComment {
			if value[index] == '\r' || value[index] == '\n' {
				inComment = false
			}
			continue
		}
		if quote != 0 {
			index, quote = stepYAMLQuotedScalar(value, index, quote)
			continue
		}
		switch value[index] {
		case '\'', '"':
			quote = value[index]
		case '#':
			inComment = true
		case '{':
			return index
		}
	}
	return -1
}

func redactSensitiveYAMLFlowNameValueObject(object string, scalarAnchors map[string]string) (string, bool) {
	if len(object) < 2 || object[0] != '{' || object[len(object)-1] != '}' {
		return object, false
	}
	properties, ok := parseYAMLFlowMapping(object)
	if !ok || !slices.ContainsFunc(properties, func(property yamlFlowProperty) bool {
		return strings.EqualFold(property.key, "name") && yamlCredentialNamePattern.MatchString(resolveYAMLAlias(kubernetesYAMLScalar(property.value), scalarAnchors))
	}) {
		return object, false
	}

	redacted := object
	for index := len(properties) - 1; index >= 0; index-- {
		property := properties[index]
		if !strings.EqualFold(property.key, "value") || property.value == "" {
			continue
		}
		start, end := unquotedYAMLFlowValue(object, property.start, property.end)
		if object[start:end] != redactionMarker {
			redacted = redacted[:start] + redactionMarker + redacted[end:]
		}
	}
	return redacted, redacted != object
}

// unquotedYAMLFlowValue returns the span inside the quotes of a quoted value.
func unquotedYAMLFlowValue(flow string, start, end int) (int, int) {
	if end-start >= 2 && (flow[start] == '\'' || flow[start] == '"') && flow[end-1] == flow[start] {
		return start + 1, end - 1
	}
	return start, end
}

func normalizeYAMLEscapedCredentialKeys(value string) (string, map[string]string) {
	lines := strings.SplitAfter(value, "\n")
	restorations := make(map[string]string)
	placeholderIndex := 0
	for line := range lines {
		content, ending := splitLineEnding(lines[line])
		keyStart, explicitKey := yamlQuotedKeyStart(content)
		if keyStart < 0 || keyStart >= len(content) || content[keyStart] != '"' {
			continue
		}
		keyEnd := findTOMLQuotedStringEnd(content, keyStart, '"')
		if keyEnd <= keyStart || keyEnd > len(content) {
			continue
		}
		encodedKey := content[keyStart:keyEnd]
		if !strings.Contains(encodedKey, `\`) {
			continue
		}
		decodedKey, err := strconv.Unquote(encodedKey)
		if err != nil || !yamlCredentialNamePattern.MatchString(decodedKey) || !isYAMLMappingKeySuffix(content[keyEnd:], explicitKey) {
			continue
		}

		placeholder := ""
		for placeholder == "" || strings.Contains(value, placeholder) {
			placeholder = `"_snitch_redaction_` + strconv.Itoa(placeholderIndex) + `_password"`
			placeholderIndex++
		}
		restorations[placeholder] = encodedKey
		lines[line] = content[:keyStart] + placeholder + content[keyEnd:] + ending
	}
	return strings.Join(lines, ""), restorations
}

func normalizeYAMLAliasCredentialKeys(value string) (string, map[string]string) {
	sensitiveAliases := make(map[string]struct{})
	for _, match := range yamlSensitiveNameAnchor.FindAllStringSubmatch(value, -1) {
		sensitiveAliases[match[1]] = struct{}{}
	}
	if len(sensitiveAliases) == 0 {
		return value, nil
	}

	lines := strings.SplitAfter(value, "\n")
	restorations := make(map[string]string)
	placeholderIndex := 0
	patterns := []*lazyRegexp{yamlAliasMappingKey, yamlExplicitAliasKey}
	for line := range lines {
		content, ending := splitLineEnding(lines[line])
		for _, pattern := range patterns {
			match := pattern.FindStringSubmatchIndex(content)
			if match == nil {
				continue
			}
			if _, sensitive := sensitiveAliases[content[match[6]:match[7]]]; !sensitive {
				continue
			}

			placeholder := ""
			for placeholder == "" || strings.Contains(value, placeholder) {
				placeholder = `_snitch_redaction_` + strconv.Itoa(placeholderIndex) + `_password`
				placeholderIndex++
			}
			original := content[match[4]:match[5]]
			restorations[placeholder] = original
			lines[line] = content[:match[4]] + placeholder + content[match[5]:] + ending
			break
		}
	}
	return strings.Join(lines, ""), restorations
}

func yamlQuotedKeyStart(content string) (int, bool) {
	cursor := 0
	explicitKey := false
	for cursor < len(content) && (content[cursor] == ' ' || content[cursor] == '\t') {
		cursor++
	}
	if cursor < len(content) && content[cursor] == '-' {
		cursor++
		if cursor >= len(content) || (content[cursor] != ' ' && content[cursor] != '\t') {
			return -1, false
		}
		for cursor < len(content) && (content[cursor] == ' ' || content[cursor] == '\t') {
			cursor++
		}
	}
	if cursor < len(content) && content[cursor] == '?' {
		explicitKey = true
		cursor++
		if cursor >= len(content) || (content[cursor] != ' ' && content[cursor] != '\t') {
			return -1, false
		}
		for cursor < len(content) && (content[cursor] == ' ' || content[cursor] == '\t') {
			cursor++
		}
	}
	_, propertyOffset := trimYAMLNodeProperties(content[cursor:])
	cursor += propertyOffset
	return cursor, explicitKey
}

func isYAMLMappingKeySuffix(suffix string, explicitKey bool) bool {
	suffix = strings.TrimSpace(suffix)
	if explicitKey {
		return suffix == "" || strings.HasPrefix(suffix, "#")
	}
	return strings.HasPrefix(suffix, ":")
}

func redactYAMLCredentialAliases(value string) (string, bool) {
	aliases := make(map[string]struct{})
	for _, match := range yamlCredentialAlias.FindAllStringSubmatch(value, -1) {
		aliases[match[1]] = struct{}{}
	}
	if len(aliases) == 0 {
		return value, false
	}

	lines := strings.SplitAfter(value, "\n")
	changed := false
	for line := 0; line < len(lines); line++ {
		content, _ := splitLineEnding(lines[line])
		for _, match := range yamlAnchor.FindAllStringSubmatchIndex(content, -1) {
			if _, sensitive := aliases[content[match[2]:match[3]]]; !sensitive {
				continue
			}
			valueIndent, isValue := yamlAnchorValueIndent(content[:match[0]])
			if !isValue {
				continue
			}
			var blockChanged bool
			if lines, blockChanged = redactYAMLValueBlock(lines, line, content[:match[1]]+" ", content[match[1]:], valueIndent); blockChanged {
				changed = true
				break
			}
		}
	}
	return strings.Join(lines, ""), changed
}

// redactYAMLValueBlock replaces the value inlineValue on lines[line] after
// prefix, and the following lines indented beyond indent, with the marker.
func redactYAMLValueBlock(lines []string, line int, prefix, inlineValue string, indent int) ([]string, bool) {
	inlineValue = strings.TrimSpace(inlineValue)
	end := line + 1
	hasIndentedContent := false
	for ; end < len(lines); end++ {
		child, _ := splitLineEnding(lines[end])
		if strings.TrimSpace(child) == "" {
			continue
		}
		if leadingIndent(child) <= indent {
			break
		}
		hasIndentedContent = true
	}
	if !hasIndentedContent && (inlineValue == "" || strings.HasPrefix(inlineValue, "#") || inlineValue == redactionMarker) {
		return lines, false
	}
	_, ending := splitLineEnding(lines[line])
	lines[line] = prefix + redactionMarker + ending
	return append(lines[:line+1], lines[end:]...), true
}

func yamlAnchorValueIndent(prefix string) (int, bool) {
	if colon := strings.LastIndexByte(prefix, ':'); colon >= 0 && strings.TrimSpace(prefix[colon+1:]) == "" {
		return yamlKeyIndent(prefix), true
	}
	if strings.TrimSpace(prefix) == "-" {
		return leadingIndent(prefix), true
	}
	return 0, false
}

func redactYAMLExplicitCredentialMappings(value string) (string, bool) {
	lines := strings.SplitAfter(value, "\n")
	changed := false
	for line := 0; line < len(lines)-1; line++ {
		key, _ := splitLineEnding(lines[line])
		if !yamlExplicitCredentialKey.MatchString(key) {
			continue
		}

		keyIndent := yamlKeyIndent(key)
		valueLine := nextYAMLContentLine(lines, line+1)
		if valueLine >= len(lines) {
			continue
		}
		valueContent, _ := splitLineEnding(lines[valueLine])
		if leadingIndent(valueContent) != keyIndent {
			continue
		}
		indicator := valueContent[keyIndent:]
		if indicator == "" || indicator[0] != ':' || (len(indicator) > 1 && indicator[1] != ' ' && indicator[1] != '\t' && indicator[1] != '#') {
			continue
		}
		var blockChanged bool
		if lines, blockChanged = redactYAMLValueBlock(lines, valueLine, valueContent[:keyIndent]+": ", indicator[1:], keyIndent); blockChanged {
			changed = true
			line = valueLine
		}
	}
	return strings.Join(lines, ""), changed
}

// nextYAMLContentLine returns the first line from start that is neither blank
// nor a comment, or len(lines).
func nextYAMLContentLine(lines []string, start int) int {
	for ; start < len(lines); start++ {
		trimmed := strings.TrimSpace(lines[start])
		if trimmed != "" && !strings.HasPrefix(trimmed, "#") {
			break
		}
	}
	return start
}

func redactYAMLCredentialBlocks(value string) (string, bool) {
	lines := strings.SplitAfter(value, "\n")
	changed := false
	for line := 0; line < len(lines); line++ {
		content, ending := splitLineEnding(lines[line])
		match := yamlCredentialScalar.FindStringSubmatch(content)
		blockScalar := match != nil
		plainScalar := false
		separator := ""
		if match == nil {
			match = yamlCredentialMapping.FindStringSubmatch(content)
			separator = " "
		}
		if match == nil {
			match = yamlCredentialPlainScalar.FindStringSubmatch(content)
			plainScalar = match != nil
			separator = ""
		}
		if match == nil {
			continue
		}

		keyIndent := yamlKeyIndent(content)
		end := line + 1
		hasIndentedContent := false
		firstIndentedContent := ""
		for end < len(lines) {
			child, _ := splitLineEnding(lines[end])
			trimmedChild := strings.TrimSpace(child)
			if trimmedChild == "" || !blockScalar && !plainScalar && strings.HasPrefix(trimmedChild, "#") {
				end++
				continue
			}
			childIndent := leadingIndent(child)
			indentlessSequence := !blockScalar && !plainScalar && isYAMLSequenceItemAtIndent(child, keyIndent)
			if childIndent < keyIndent || childIndent == keyIndent && !indentlessSequence {
				break
			}
			hasIndentedContent = true
			if firstIndentedContent == "" {
				firstIndentedContent = trimmedChild
			}
			end++
		}
		if !hasIndentedContent {
			continue
		}
		if !blockScalar && !plainScalar && strings.HasPrefix(strings.TrimSpace(content), `"`) && jsonQuotedScalarLine.MatchString(firstIndentedContent) {
			continue
		}

		lines[line] = match[1] + separator + redactionMarker + ending
		lines = append(lines[:line+1], lines[end:]...)
		changed = true
	}
	return strings.Join(lines, ""), changed
}

func redactYAMLFlowCredentials(value string) (string, bool) {
	redacted := value
	changed := false
	for searchStart := 0; searchStart < len(redacted); {
		match := yamlCredentialFlowStart.FindStringSubmatchIndex(redacted[searchStart:])
		if match == nil {
			break
		}

		start := searchStart + match[0]
		prefixEnd := searchStart + match[3]
		open := searchStart + match[4]
		close := findYAMLFlowEnd(redacted, open)
		if close < 0 {
			close = len(redacted) - 1
		}
		if redacted[open:close+1] == redactionMarker {
			searchStart = open + 1
			continue
		}

		replacement := redacted[start:prefixEnd] + redactionMarker
		redacted = redacted[:start] + replacement + redacted[close+1:]
		changed = true
		searchStart = start + len(replacement)
	}
	return redacted, changed
}

func findYAMLFlowEnd(value string, open int) int {
	if open < 0 || open >= len(value) || (value[open] != '[' && value[open] != '{') {
		return -1
	}

	stack := []byte{value[open]}
	var quote byte
	for i := open + 1; i < len(value); i++ {
		if quote != 0 {
			i, quote = stepYAMLQuotedScalar(value, i, quote)
			continue
		}

		switch value[i] {
		case '"', '\'':
			quote = value[i]
		case '[', '{':
			stack = append(stack, value[i])
		case ']':
			if stack[len(stack)-1] == '[' {
				stack = stack[:len(stack)-1]
			}
		case '}':
			if stack[len(stack)-1] == '{' {
				stack = stack[:len(stack)-1]
			}
		}
		if len(stack) == 0 {
			return i
		}
	}
	return -1
}

func splitLineEnding(line string) (string, string) {
	if strings.HasSuffix(line, "\r\n") {
		return strings.TrimSuffix(line, "\r\n"), "\r\n"
	}
	if strings.HasSuffix(line, "\n") {
		return strings.TrimSuffix(line, "\n"), "\n"
	}
	return line, ""
}

func leadingIndent(line string) int {
	return len(line) - len(strings.TrimLeft(line, " \t"))
}

func yamlKeyIndent(line string) int {
	indent := leadingIndent(line)
	if indent >= len(line) || line[indent] != '-' {
		return indent
	}

	key := indent + 1
	if key >= len(line) || (line[key] != ' ' && line[key] != '\t') {
		return indent
	}
	for key < len(line) && (line[key] == ' ' || line[key] == '\t') {
		key++
	}
	return key
}

func isYAMLSequenceItemAtIndent(line string, indent int) bool {
	if indent < 0 || leadingIndent(line) != indent || indent >= len(line) || line[indent] != '-' {
		return false
	}
	return indent+1 == len(line) || line[indent+1] == ' ' || line[indent+1] == '\t'
}
