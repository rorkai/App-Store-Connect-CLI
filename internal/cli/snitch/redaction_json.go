package snitch

import (
	"encoding/json"
	"strings"
)

type jsonObjectProperty struct {
	key        string
	valueStart int
	valueEnd   int
}

func redactKubernetesSecretJSONData(value string, escapeDepth int) (string, bool) {
	if !containsJSONKeyAtDepth(value, "kind", escapeDepth) ||
		!containsJSONKeyAtDepth(value, "data", escapeDepth) && !containsJSONKeyAtDepth(value, "stringData", escapeDepth) {
		return value, false
	}
	return redactEachJSONObject(value, escapeDepth, true, func(object string) (string, bool) {
		return redactKubernetesSecretJSONObject(object, escapeDepth)
	})
}

func redactEachJSONObject(value string, escapeDepth int, redactUnclosed bool, redact func(object string) (string, bool)) (string, bool) {
	return redactEachObject(value, func(value string, start int) int {
		return nextJSONObjectStart(value, start, escapeDepth)
	}, func(value string, open int) int {
		return findJSONContainerEndAtDepthStrict(value, open, escapeDepth)
	}, redactUnclosed, redact)
}

// redactEachObject redacts each object that next finds and end closes,
// including nested objects. An object without a close ends the scan, so a
// malformed outer object does not make every nested object rescan the rest of
// value; with redactUnclosed, the rest of value is redacted as that object.
func redactEachObject(value string, next, end func(value string, start int) int, redactUnclosed bool, redact func(object string) (string, bool)) (string, bool) {
	redacted := value
	changed := false
	for searchStart := 0; searchStart < len(redacted); {
		open := next(redacted, searchStart)
		if open < 0 {
			break
		}
		close := end(redacted, open)
		unclosed := close < open
		if unclosed {
			if !redactUnclosed {
				break
			}
			close = len(redacted) - 1
		}
		if redactedObject, objectChanged := redact(redacted[open : close+1]); objectChanged {
			redacted = redacted[:open] + redactedObject + redacted[close+1:]
			changed = true
		}
		if unclosed {
			break
		}
		searchStart = open + 1
	}
	return redacted, changed
}

func containsJSONKeyAtDepth(value, key string, escapeDepth int) bool {
	delimiter := jsonQuoteDelimiter(escapeDepth)
	for searchStart := 0; searchStart < len(value); {
		open, close := nextJSONStringAtDepth(value, searchStart, delimiter, escapeDepth)
		if close < 0 {
			return false
		}
		decoded, valid := decodeJSONCredentialKey(value[open:close+1], escapeDepth)
		if valid && jsonValueAfterKey(value, close) >= 0 && strings.EqualFold(decoded, key) {
			return true
		}
		searchStart = close + 1
	}
	return false
}

// nextJSONStringAtDepth returns the bounds of the next string from start that
// is quoted by delimiter, the jsonQuoteDelimiter of escapeDepth, the close
// being -1 when it or its end is missing.
func nextJSONStringAtDepth(value string, start int, delimiter string, escapeDepth int) (int, int) {
	for start < len(value) {
		relative := strings.Index(value[start:], delimiter)
		if relative < 0 {
			break
		}
		open := start + relative
		if isJSONStringDelimiterAtDepth(value, open+len(delimiter)-1, escapeDepth) {
			return open, findJSONStringEndAtDepth(value, open, escapeDepth)
		}
		start = open + len(delimiter)
	}
	return -1, -1
}

// jsonValueAfterKey returns where the value of the key that ends at keyClose
// starts, or -1 when no colon follows the key.
func jsonValueAfterKey(value string, keyClose int) int {
	colon := skipJSONWhitespace(value, keyClose+1)
	if colon >= len(value) || value[colon] != ':' {
		return -1
	}
	return skipJSONWhitespace(value, colon+1)
}

func skipJSONWhitespace(value string, index int) int {
	for index < len(value) && (value[index] == ' ' || value[index] == '\t' || value[index] == '\r' || value[index] == '\n') {
		index++
	}
	return index
}

func nextJSONObjectStart(value string, start, escapeDepth int) int {
	inString := false
	for index := start; index < len(value); index++ {
		if value[index] == '"' && isJSONStringDelimiterAtDepth(value, index, escapeDepth) {
			inString = !inString
			continue
		}
		if !inString && value[index] == '{' {
			return index
		}
	}
	return -1
}

func redactKubernetesSecretJSONObject(object string, escapeDepth int) (string, bool) {
	properties := parseJSONObjectProperties(object, escapeDepth)
	if !hasJSONStringProperty(object, properties, escapeDepth, "kind", func(kind string) bool { return strings.EqualFold(kind, "Secret") }) {
		return object, false
	}

	var replacements []textReplacement
	quotedMarker := jsonQuoteDelimiter(escapeDepth) + redactionMarker + jsonQuoteDelimiter(escapeDepth)
	for _, property := range properties {
		if !strings.EqualFold(property.key, "data") && !strings.EqualFold(property.key, "stringData") || property.valueStart >= len(object) || object[property.valueStart] != '{' {
			continue
		}
		container := object[property.valueStart:property.valueEnd]
		containerProperties := parseJSONObjectProperties(container, escapeDepth)
		for _, child := range containerProperties {
			if child.valueStart >= len(container) || container[child.valueStart:child.valueEnd] == quotedMarker {
				continue
			}
			replacements = append(replacements, textReplacement{property.valueStart + child.valueStart, property.valueStart + child.valueEnd, quotedMarker})
		}
	}
	return applyTextReplacements(object, replacements), len(replacements) > 0
}

// hasJSONStringProperty reports whether object has a property named key, in
// any case, whose string value matches.
func hasJSONStringProperty(object string, properties []jsonObjectProperty, escapeDepth int, key string, matches func(string) bool) bool {
	for _, property := range properties {
		if !strings.EqualFold(property.key, key) || property.valueStart >= len(object) || !strings.HasPrefix(object[property.valueStart:], jsonQuoteDelimiter(escapeDepth)) {
			continue
		}
		if decoded, valid := decodeJSONCredentialKey(object[property.valueStart:property.valueEnd], escapeDepth); valid && matches(decoded) {
			return true
		}
	}
	return false
}

func parseJSONObjectProperties(object string, escapeDepth int) []jsonObjectProperty {
	if len(object) < 2 || object[0] != '{' {
		return nil
	}
	objectEnd := len(object)
	if object[len(object)-1] == '}' {
		objectEnd--
	}
	var properties []jsonObjectProperty
	for cursor := 1; cursor < objectEnd; {
		for cursor < objectEnd && (object[cursor] == ' ' || object[cursor] == '\t' || object[cursor] == '\r' || object[cursor] == '\n' || object[cursor] == ',') {
			cursor++
		}
		delimiter := jsonQuoteDelimiter(escapeDepth)
		if cursor >= objectEnd || !strings.HasPrefix(object[cursor:], delimiter) || !isJSONStringDelimiterAtDepth(object, cursor+len(delimiter)-1, escapeDepth) {
			return properties
		}
		keyEnd := findJSONStringEndAtDepth(object, cursor, escapeDepth)
		if keyEnd < 0 {
			return properties
		}
		key, valid := decodeJSONCredentialKey(object[cursor:keyEnd+1], escapeDepth)
		if !valid {
			return properties
		}
		valueStart := jsonValueAfterKey(object, keyEnd)
		if valueStart < 0 {
			return properties
		}
		valueEnd := jsonPropertyValueEnd(object, valueStart, escapeDepth)
		if valueEnd <= valueStart {
			return properties
		}
		properties = append(properties, jsonObjectProperty{key: key, valueStart: valueStart, valueEnd: valueEnd})
		cursor = valueEnd
	}
	return properties
}

func jsonPropertyValueEnd(object string, start, escapeDepth int) int {
	end, _ := jsonCredentialValueReplacement(object, start, escapeDepth)
	return end
}

func redactJSONEscapedCredentialValues(value string) (string, bool) {
	return redactAtEachJSONEscapeDepth(value, redactJSONCredentialValues)
}

// redactAtEachJSONEscapeDepth runs redact once for each level of JSON string
// escaping that value is long enough to hold, from unescaped JSON upwards.
func redactAtEachJSONEscapeDepth(value string, redact func(value string, escapeDepth int) (string, bool)) (string, bool) {
	maxEscapeDepth := 0
	for length := len(value); length > 1; length >>= 1 {
		maxEscapeDepth++
	}
	redacted := value
	changed := false
	for escapeDepth := 0; escapeDepth <= maxEscapeDepth; escapeDepth++ {
		if next, layerChanged := redact(redacted, escapeDepth); layerChanged {
			redacted = next
			changed = true
		}
	}
	return redacted, changed
}

func redactJSONCredentialValues(value string, escapeDepth int) (string, bool) {
	return redactJSONValuesMatchingName(value, escapeDepth, jsonCredentialName.MatchString, false, false)
}

func redactJSONValuesMatchingName(value string, escapeDepth int, nameMatches func(string) bool, includePlainKeys, preserveUnterminatedValues bool) (string, bool) {
	redacted := value
	changed := false
	quoteDelimiter := jsonQuoteDelimiter(escapeDepth)
	for searchStart := 0; searchStart < len(redacted); {
		open, close := nextJSONStringAtDepth(redacted, searchStart, quoteDelimiter, escapeDepth)
		if close < 0 {
			break
		}

		encodedKey := redacted[open : close+1]
		if !includePlainKeys && escapeDepth == 0 && !strings.Contains(encodedKey, `\`) {
			searchStart = close + 1
			continue
		}
		decodedKey, valid := decodeJSONCredentialKey(encodedKey, escapeDepth)
		if !valid || !nameMatches(decodedKey) {
			searchStart = close + 1
			continue
		}

		valueStart := jsonValueAfterKey(redacted, close)
		if valueStart < 0 {
			searchStart = close + 1
			continue
		}
		valueEnd, replacement := jsonCredentialValueReplacement(redacted, valueStart, escapeDepth)
		if preserveUnterminatedValues && strings.HasPrefix(redacted[valueStart:], quoteDelimiter) && findJSONStringEndAtDepth(redacted, valueStart, escapeDepth) < 0 {
			valueEnd = len(redacted)
			replacement = quoteDelimiter + redactionMarker
		}
		if valueEnd <= valueStart {
			searchStart = close + 1
			continue
		}
		if redacted[valueStart:valueEnd] == replacement {
			searchStart = valueEnd
			continue
		}

		redacted = redacted[:valueStart] + replacement + redacted[valueEnd:]
		changed = true
		searchStart = valueStart + len(replacement)
	}
	return redacted, changed
}

type contextualJSONContainerRule struct {
	name      string
	openers   string
	valueName string
}

var contextualJSONContainerRules = []contextualJSONContainerRule{
	{name: "cookies", openers: "[{", valueName: "value"},
	{name: "auths", openers: "{", valueName: "auth"},
	{name: "requestHeaders", openers: "[", valueName: "value"},
}

func redactContextualJSONCredentialValues(value string) (string, bool) {
	return redactAtEachJSONEscapeDepth(value, func(redacted string, escapeDepth int) (string, bool) {
		changed := false
		for _, rule := range contextualJSONContainerRules {
			if next, ruleChanged := redactJSONValuesInNamedContainer(redacted, escapeDepth, rule); ruleChanged {
				redacted = next
				changed = true
			}
		}
		return redacted, changed
	})
}

func redactSensitiveJSONNameValuePairs(value string) (string, bool) {
	return redactAtEachJSONEscapeDepth(value, redactSensitiveJSONNameValuePairsAtDepth)
}

func redactSensitiveJSONNameValuePairsAtDepth(value string, escapeDepth int) (string, bool) {
	return redactEachJSONObject(value, escapeDepth, false, func(object string) (string, bool) {
		return redactSensitiveJSONNameValueObject(object, escapeDepth)
	})
}

func redactSensitiveJSONNameValueObject(object string, escapeDepth int) (string, bool) {
	properties := parseJSONObjectProperties(object, escapeDepth)
	if !hasJSONStringProperty(object, properties, escapeDepth, "name", jsonCredentialName.MatchString) {
		return object, false
	}

	delimiter := jsonQuoteDelimiter(escapeDepth)
	var replacements []textReplacement
	for _, property := range properties {
		if !strings.EqualFold(property.key, "value") || property.valueStart >= len(object) {
			continue
		}
		valueEnd, replacementValue := jsonCredentialValueReplacement(object, property.valueStart, escapeDepth)
		if strings.HasPrefix(object[property.valueStart:], delimiter) && findJSONStringEndAtDepth(object, property.valueStart, escapeDepth) < 0 {
			replacementValue = delimiter + redactionMarker
		}
		if valueEnd <= property.valueStart || object[property.valueStart:valueEnd] == replacementValue {
			continue
		}
		replacements = append(replacements, textReplacement{property.valueStart, valueEnd, replacementValue})
	}
	return applyTextReplacements(object, replacements), len(replacements) > 0
}

func redactJSONValuesInNamedContainer(value string, escapeDepth int, rule contextualJSONContainerRule) (string, bool) {
	redacted := value
	changed := false
	quoteDelimiter := jsonQuoteDelimiter(escapeDepth)
	for searchStart := 0; searchStart < len(redacted); {
		keyOpen, keyClose := nextJSONStringAtDepth(redacted, searchStart, quoteDelimiter, escapeDepth)
		if keyClose < 0 {
			break
		}

		decodedKey, valid := decodeJSONCredentialKey(redacted[keyOpen:keyClose+1], escapeDepth)
		if !valid || !strings.EqualFold(decodedKey, rule.name) {
			searchStart = keyClose + 1
			continue
		}

		containerOpen := jsonValueAfterKey(redacted, keyClose)
		if containerOpen < 0 || containerOpen >= len(redacted) || !strings.ContainsRune(rule.openers, rune(redacted[containerOpen])) {
			searchStart = keyClose + 1
			continue
		}

		containerClose := findJSONContainerEndAtDepth(redacted, containerOpen, escapeDepth)
		container := redacted[containerOpen : containerClose+1]
		redactedContainer, containerChanged := redactJSONValuesMatchingName(container, escapeDepth, func(name string) bool {
			return strings.EqualFold(name, rule.valueName)
		}, true, true)
		if containerChanged {
			redacted = redacted[:containerOpen] + redactedContainer + redacted[containerClose+1:]
			changed = true
		}
		searchStart = containerOpen + len(redactedContainer)
	}
	return redacted, changed
}

func decodeJSONCredentialKey(encodedKey string, escapeDepth int) (string, bool) {
	for layer := 0; layer < escapeDepth; layer++ {
		var rawKey string
		if json.Unmarshal([]byte(`"`+encodedKey+`"`), &rawKey) != nil {
			return "", false
		}
		encodedKey = rawKey
	}

	var decodedKey string
	if json.Unmarshal([]byte(encodedKey), &decodedKey) != nil {
		return "", false
	}
	return decodedKey, true
}

func findJSONStringEndAtDepth(value string, open, escapeDepth int) int {
	delimiter := jsonQuoteDelimiter(escapeDepth)
	if open < 0 || open+len(delimiter) > len(value) || value[open:open+len(delimiter)] != delimiter {
		return -1
	}
	if !isJSONStringDelimiterAtDepth(value, open+len(delimiter)-1, escapeDepth) {
		return -1
	}
	for quote := open + len(delimiter); quote < len(value); quote++ {
		if value[quote] == '"' && isJSONStringDelimiterAtDepth(value, quote, escapeDepth) {
			return quote
		}
	}
	return -1
}

func jsonCredentialValueReplacement(value string, start, escapeDepth int) (int, string) {
	if start < 0 || start >= len(value) {
		return -1, ""
	}
	delimiter := jsonQuoteDelimiter(escapeDepth)
	if strings.HasPrefix(value[start:], delimiter) && isJSONStringDelimiterAtDepth(value, start+len(delimiter)-1, escapeDepth) {
		close := findJSONStringEndAtDepth(value, start, escapeDepth)
		if close < 0 {
			return len(value), delimiter + redactionMarker + delimiter
		}
		return close + 1, delimiter + redactionMarker + delimiter
	}
	switch value[start] {
	case '{':
		return findJSONContainerEndAtDepth(value, start, escapeDepth) + 1, delimiter + redactionMarker + delimiter
	case '[':
		return findJSONContainerEndAtDepth(value, start, escapeDepth) + 1, `[` + delimiter + redactionMarker + delimiter + `]`
	default:
		end := start
		for end < len(value) && !strings.ContainsRune(",}]\r\n", rune(value[end])) {
			end++
		}
		return end, delimiter + redactionMarker + delimiter
	}
}

func jsonQuoteDelimiter(escapeDepth int) string {
	return strings.Repeat(`\`, 1<<escapeDepth-1) + `"`
}

func redactStructuredCredentialObjects(value string) (string, bool) {
	patterns := []struct {
		pattern     *lazyRegexp
		escapeDepth int
		replacement string
		array       bool
	}{
		{pattern: rawCredentialObject, replacement: `"` + redactionMarker + `"`},
		{pattern: escapedCredentialObject, escapeDepth: 1, replacement: `\"` + redactionMarker + `\"`},
		{pattern: rawCredentialArray, replacement: `["` + redactionMarker + `"]`, array: true},
		{pattern: escapedCredentialArray, escapeDepth: 1, replacement: `[\"` + redactionMarker + `\"]`, array: true},
	}
	redacted := value
	changed := false
	for _, candidate := range patterns {
		for searchStart := 0; searchStart < len(redacted); {
			match := candidate.pattern.FindStringIndex(redacted[searchStart:])
			if match == nil {
				break
			}

			open := searchStart + match[1] - 1
			if redacted[open] == '[' && (strings.HasPrefix(redacted[open:], redactionMarker) || strings.HasPrefix(redacted[open:], candidate.replacement)) {
				searchStart = open + 1
				continue
			}
			close := findJSONContainerEndAtDepth(redacted, open, candidate.escapeDepth)
			if candidate.array && !looksLikeJSONCredentialArray(redacted, open, close, candidate.escapeDepth == 1) {
				searchStart = open + 1
				continue
			}

			redacted = redacted[:open] + candidate.replacement + redacted[close+1:]
			changed = true
			searchStart = open + len(candidate.replacement)
		}
	}
	return redacted, changed
}

func looksLikeJSONCredentialArray(value string, open, close int, escapedQuotes bool) bool {
	for index := open + 1; index <= close && index < len(value); index++ {
		switch value[index] {
		case ' ', '\t', '\r', '\n':
			continue
		case '"', '{', '[', '-', '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
			return true
		case '\\':
			return escapedQuotes && index+1 < len(value) && value[index+1] == '"'
		default:
			remaining := value[index : close+1]
			return strings.HasPrefix(remaining, "true") || strings.HasPrefix(remaining, "false") || strings.HasPrefix(remaining, "null")
		}
	}
	return false
}

func findJSONContainerEndAtDepth(value string, open, escapeDepth int) int {
	if end := findJSONContainerEndAtDepthStrict(value, open, escapeDepth); end >= 0 {
		return end
	}
	return len(value) - 1
}

func findJSONContainerEndAtDepthStrict(value string, open, escapeDepth int) int {
	if open < 0 || open >= len(value) || (value[open] != '{' && value[open] != '[') {
		return -1
	}

	stack := []byte{value[open]}
	inString := false
	for i := open + 1; i < len(value); i++ {
		if value[i] == '"' && isJSONStringDelimiterAtDepth(value, i, escapeDepth) {
			inString = !inString
			continue
		}
		if inString {
			continue
		}

		switch value[i] {
		case '{', '[':
			stack = append(stack, value[i])
		case '}':
			if len(stack) == 0 || stack[len(stack)-1] != '{' {
				return -1
			}
			stack = stack[:len(stack)-1]
			if len(stack) == 0 {
				return i
			}
		case ']':
			if len(stack) == 0 || stack[len(stack)-1] != '[' {
				return -1
			}
			stack = stack[:len(stack)-1]
			if len(stack) == 0 {
				return i
			}
		}
	}
	return -1
}

func isJSONStringDelimiterAtDepth(value string, quote, escapeDepth int) bool {
	backslashes := 0
	for i := quote - 1; i >= 0 && value[i] == '\\'; i-- {
		backslashes++
	}
	period := 1 << (escapeDepth + 1)
	want := 1<<escapeDepth - 1
	return backslashes%period == want
}

func redactStructuredCookieValues(value string) (string, bool) {
	return redactJSONContainerValues(value, rawCookieJarPattern, escapedCookieJarPattern, structuredContainerValueRedactionRules, true)
}

// redactJSONContainerValues applies rules inside each JSON container that raw
// (plain quotes) or escaped (backslash-escaped quotes) finds, ending with the
// opening bracket. With truncated, it also redacts a value cut off by the end
// of the container.
func redactJSONContainerValues(value string, raw, escaped *lazyRegexp, rules []redactionRule, truncated bool) (string, bool) {
	redacted := value
	changed := false
	for escapeDepth, pattern := range []*lazyRegexp{raw, escaped} {
		for searchStart := 0; searchStart < len(redacted); {
			match := pattern.FindStringIndex(redacted[searchStart:])
			if match == nil {
				break
			}

			open := searchStart + match[1] - 1
			close := findJSONContainerEndAtDepth(redacted, open, escapeDepth)
			container := redacted[open : close+1]
			redactedContainer := container
			for _, rule := range rules {
				redactedContainer = rule.pattern.ReplaceAllString(redactedContainer, rule.replacement)
			}
			if truncated {
				redactedContainer = redactTruncatedStructuredValue(redactedContainer, escapeDepth)
			}
			if redactedContainer != container {
				redacted = redacted[:open] + redactedContainer + redacted[close+1:]
				changed = true
			}
			searchStart = open + len(redactedContainer)
		}
	}
	return redacted, changed
}

// Registry configuration auth values are base64-encoded username/password
// pairs. Scope the generic "auth" key to the auths container so unrelated
// diagnostic fields with that name remain useful.
func redactRegistryConfigurationAuthValues(value string) (string, bool) {
	return redactJSONContainerValues(value, rawRegistryAuthsPattern, escapedRegistryAuthsPattern, registryAuthValueRedactionRules, false)
}

// Upload-operation request-header values are capabilities regardless of their
// names, matching asc.RedactUploadOperations while preserving useful metadata.
func redactStructuredUploadHeaderValues(value string) (string, bool) {
	return redactJSONContainerValues(value, rawRequestHeaders, escapedRequestHeaders, structuredContainerValueRedactionRules, true)
}

func redactTruncatedStructuredValue(value string, escapeDepth int) string {
	pattern := rawStructuredValueStart
	if escapeDepth == 1 {
		pattern = escapedValueStart
	}

	for searchStart := 0; searchStart < len(value); {
		match := pattern.FindStringIndex(value[searchStart:])
		if match == nil {
			return value
		}

		openQuote := searchStart + match[1] - 1
		for quote := openQuote + 1; quote < len(value); quote++ {
			if value[quote] == '"' && isJSONStringDelimiterAtDepth(value, quote, escapeDepth) {
				searchStart = quote + 1
				break
			}
			if quote == len(value)-1 {
				return value[:openQuote+1] + redactionMarker
			}
		}
		if openQuote == len(value)-1 {
			return value + redactionMarker
		}
	}
	return value
}
