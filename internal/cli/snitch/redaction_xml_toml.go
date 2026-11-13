package snitch

import (
	"encoding/xml"
	"errors"
	"io"
	"strconv"
	"strings"
)

func redactPlistCredentialValues(value string) (string, bool) {
	redacted := value
	changed := false
	for searchStart := 0; searchStart < len(redacted); {
		relativeKey := strings.Index(redacted[searchStart:], "<key")
		if relativeKey < 0 {
			break
		}
		keyStart := searchStart + relativeKey
		contentStart, contentEnd, elementEnd, valid := findPlistCredentialValue(redacted, keyStart)
		if !valid || contentStart == contentEnd || redacted[contentStart:contentEnd] == redactionMarker {
			searchStart = keyStart + len("<key")
			continue
		}

		redacted = redacted[:contentStart] + redactionMarker + redacted[contentEnd:]
		changed = true
		searchStart = elementEnd - (contentEnd - contentStart) + len(redactionMarker)
	}
	return redacted, changed
}

func redactXMLCredentialElements(value string) (string, bool) {
	redacted := value
	changed := false
	for searchStart := 0; searchStart < len(redacted); {
		match := xmlCredentialElementStart.FindStringIndex(redacted[searchStart:])
		if match == nil {
			break
		}

		elementStart := searchStart + match[0]
		decoder := xml.NewDecoder(strings.NewReader(redacted[elementStart:]))
		first, err := decoder.Token()
		element, validElement := first.(xml.StartElement)
		if err != nil || !validElement || !xmlCredentialName.MatchString(element.Name.Local) {
			searchStart = elementStart + 1
			continue
		}

		contentStart := elementStart + int(decoder.InputOffset())
		_, contentEnd, elementEnd, valid := findPlistElementEnd(decoder, elementStart, contentStart)
		if !valid {
			searchStart = elementStart + 1
			continue
		}
		if next, attributeChanged, delta := redactXMLElementValueAttribute(redacted, elementStart, contentStart, element); attributeChanged {
			redacted = next
			contentStart += delta
			contentEnd += delta
			elementEnd += delta
			changed = true
		}
		if contentStart == contentEnd || redacted[contentStart:contentEnd] == redactionMarker {
			searchStart = elementEnd
			continue
		}

		redacted = redacted[:contentStart] + redactionMarker + redacted[contentEnd:]
		changed = true
		searchStart = elementEnd - (contentEnd - contentStart) + len(redactionMarker)
	}
	return redacted, changed
}

func redactXMLElementValueAttribute(value string, elementStart, elementEnd int, element xml.StartElement) (string, bool, int) {
	attributeMatches := xmlAttribute.FindAllStringSubmatchIndex(value[elementStart:elementEnd], -1)
	if len(attributeMatches) != len(element.Attr) {
		return value, false, 0
	}
	for index, attribute := range element.Attr {
		if !strings.EqualFold(attribute.Name.Local, "value") {
			continue
		}
		valueStart, valueEnd := xmlAttributeValueSpan(attributeMatches[index])
		if valueStart < 0 || valueEnd < valueStart {
			return value, false, 0
		}
		valueStart += elementStart
		valueEnd += elementStart
		if value[valueStart:valueEnd] == "" || value[valueStart:valueEnd] == redactionMarker {
			return value, false, 0
		}
		return value[:valueStart] + redactionMarker + value[valueEnd:], true, len(redactionMarker) - (valueEnd - valueStart)
	}
	return value, false, 0
}

// xmlAttributeValueSpan returns the bounds of the double- or single-quoted
// value in a match of xmlAttribute.
func xmlAttributeValueSpan(match []int) (int, int) {
	if match[4] >= 0 {
		return match[4], match[5]
	}
	return match[6], match[7]
}

func redactXMLCredentialAttributes(value string) (string, bool) {
	redacted := value
	changed := false
	for searchStart := 0; searchStart < len(redacted); {
		match := xmlElementStart.FindStringIndex(redacted[searchStart:])
		if match == nil {
			break
		}

		elementStart := searchStart + match[0]
		decoder := xml.NewDecoder(strings.NewReader(redacted[elementStart:]))
		first, err := decoder.Token()
		element, validElement := first.(xml.StartElement)
		if err != nil || !validElement {
			searchStart = elementStart + 1
			continue
		}

		elementEnd := elementStart + int(decoder.InputOffset())
		attributeMatches := xmlAttribute.FindAllStringSubmatchIndex(redacted[elementStart:elementEnd], -1)
		if len(attributeMatches) != len(element.Attr) {
			searchStart = elementEnd
			continue
		}

		sensitiveKey := false
		valueAttribute := -1
		for index, attribute := range element.Attr {
			switch strings.ToLower(attribute.Name.Local) {
			case "key", "name":
				if tomlCredentialName.MatchString(attribute.Value) {
					sensitiveKey = true
				}
			case "value":
				valueAttribute = index
			}
		}
		if !sensitiveKey {
			searchStart = elementEnd
			continue
		}
		if valueAttribute < 0 {
			contentStart := elementEnd
			_, contentEnd, fullElementEnd, valid := findPlistElementEnd(decoder, elementStart, contentStart)
			if !valid || contentEnd <= contentStart || strings.TrimSpace(redacted[contentStart:contentEnd]) == "" || redacted[contentStart:contentEnd] == redactionMarker {
				searchStart = elementEnd
				continue
			}
			redacted = redacted[:contentStart] + redactionMarker + redacted[contentEnd:]
			changed = true
			searchStart = fullElementEnd - (contentEnd - contentStart) + len(redactionMarker)
			continue
		}

		valueStart, valueEnd := xmlAttributeValueSpan(attributeMatches[valueAttribute])
		valueStart += elementStart
		valueEnd += elementStart
		if redacted[valueStart:valueEnd] == redactionMarker {
			searchStart = elementEnd
			continue
		}

		redacted = redacted[:valueStart] + redactionMarker + redacted[valueEnd:]
		changed = true
		searchStart = elementEnd - (valueEnd - valueStart) + len(redactionMarker)
	}
	return redacted, changed
}

func findPlistCredentialValue(value string, keyStart int) (int, int, int, bool) {
	decoder := xml.NewDecoder(strings.NewReader(value[keyStart:]))
	first, err := decoder.Token()
	keyElement, validKeyElement := first.(xml.StartElement)
	if err != nil || !validKeyElement || keyElement.Name.Local != "key" {
		return 0, 0, 0, false
	}

	var keyText strings.Builder
	for {
		token, tokenErr := decoder.Token()
		if tokenErr != nil {
			return 0, 0, 0, false
		}
		switch typed := token.(type) {
		case xml.CharData:
			keyText.Write(typed)
		case xml.Comment, xml.ProcInst, xml.Directive:
		case xml.EndElement:
			if typed.Name != keyElement.Name {
				return 0, 0, 0, false
			}
			if !tomlCredentialName.MatchString(strings.TrimSpace(keyText.String())) {
				return 0, 0, 0, false
			}
			goto findValue
		default:
			return 0, 0, 0, false
		}
	}

findValue:
	for {
		token, tokenErr := decoder.Token()
		if tokenErr != nil {
			return 0, 0, 0, false
		}
		switch typed := token.(type) {
		case xml.CharData:
			if strings.TrimSpace(string(typed)) != "" {
				return 0, 0, 0, false
			}
		case xml.Comment, xml.ProcInst, xml.Directive:
		case xml.StartElement:
			contentStart := keyStart + int(decoder.InputOffset())
			return findPlistElementEnd(decoder, keyStart, contentStart)
		default:
			return 0, 0, 0, false
		}
	}
}

func findPlistElementEnd(decoder *xml.Decoder, offsetBase, contentStart int) (int, int, int, bool) {
	for depth := 1; depth > 0; {
		tokenStart := offsetBase + int(decoder.InputOffset())
		token, tokenErr := decoder.Token()
		if tokenErr != nil {
			if isTruncatedXML(tokenErr) {
				contentEnd := offsetBase + int(decoder.InputOffset())
				return contentStart, contentEnd, contentEnd, true
			}
			return 0, 0, 0, false
		}
		switch token.(type) {
		case xml.StartElement:
			depth++
		case xml.EndElement:
			depth--
			if depth == 0 {
				return contentStart, tokenStart, offsetBase + int(decoder.InputOffset()), true
			}
		}
	}
	return 0, 0, 0, false
}

func isTruncatedXML(err error) bool {
	if errors.Is(err, io.EOF) {
		return true
	}
	var syntaxError *xml.SyntaxError
	return errors.As(err, &syntaxError) && strings.EqualFold(syntaxError.Msg, "unexpected EOF")
}

func redactTOMLCredentialValues(value string) (string, bool) {
	redacted := value
	changed := false
	tableSensitive := false
	for lineStart := 0; lineStart < len(redacted); {
		if sensitive, header := parseTOMLTableHeader(redacted, lineStart); header {
			tableSensitive = sensitive
			lineStart = nextTOMLLineStart(redacted, lineStart)
			continue
		}

		keyStart := lineStart
		for keyStart < len(redacted) && (redacted[keyStart] == ' ' || redacted[keyStart] == '\t') {
			keyStart++
		}
		keyEnd, sensitiveKey, validKey := parseTOMLKeyPath(redacted, keyStart)
		sensitiveKey = sensitiveKey || tableSensitive
		if !validKey {
			lineStart = nextTOMLLineStart(redacted, lineStart)
			continue
		}

		equals := keyEnd
		for equals < len(redacted) && (redacted[equals] == ' ' || redacted[equals] == '\t') {
			equals++
		}
		if equals >= len(redacted) || redacted[equals] != '=' || !sensitiveKey {
			lineStart = nextTOMLLineStart(redacted, lineStart)
			continue
		}

		valueStart := equals + 1
		for valueStart < len(redacted) && (redacted[valueStart] == ' ' || redacted[valueStart] == '\t') {
			valueStart++
		}
		if valueStart >= len(redacted) || redacted[valueStart] == '\r' || redacted[valueStart] == '\n' {
			lineStart = nextTOMLLineStart(redacted, lineStart)
			continue
		}
		compositeValue := redacted[valueStart] == '{' || redacted[valueStart] == '['
		structuredKey := strings.ContainsAny(redacted[keyStart:keyEnd], `.\`)
		if !compositeValue && !structuredKey && !tableSensitive {
			lineStart = nextTOMLLineStart(redacted, lineStart)
			continue
		}
		valueEnd := findTOMLValueEnd(redacted, valueStart)
		if valueEnd <= valueStart {
			lineStart = nextTOMLLineStart(redacted, lineStart)
			continue
		}
		if redacted[valueStart:valueEnd] != redactionMarker {
			redacted = redacted[:valueStart] + redactionMarker + redacted[valueEnd:]
			changed = true
		}
		lineStart = nextTOMLLineStart(redacted, valueStart+len(redactionMarker))
	}
	return redacted, changed
}

func parseTOMLTableHeader(value string, lineStart int) (bool, bool) {
	if lineStart < 0 || lineStart >= len(value) {
		return false, false
	}
	lineEnd := lineStart + strings.IndexByte(value[lineStart:], '\n')
	if lineEnd < lineStart {
		lineEnd = len(value)
	}
	start := lineStart
	for start < lineEnd && (value[start] == ' ' || value[start] == '\t') {
		start++
	}
	arrayTable := strings.HasPrefix(value[start:lineEnd], "[[")
	if !arrayTable && (start >= lineEnd || value[start] != '[') {
		return false, false
	}
	keyStart := start + 1
	if arrayTable {
		keyStart++
	}
	keyEnd, sensitive, valid := parseTOMLKeyPath(value, keyStart)
	if !valid {
		return false, false
	}
	close := "]"
	if arrayTable {
		close = "]]"
	}
	rest := strings.TrimSpace(value[keyEnd:lineEnd])
	if !strings.HasPrefix(rest, close) {
		return false, false
	}
	if trailing := strings.TrimSpace(rest[len(close):]); trailing != "" && !strings.HasPrefix(trailing, "#") {
		return false, false
	}
	return sensitive, true
}

func parseTOMLKeyPath(value string, start int) (int, bool, bool) {
	component, componentEnd, valid := parseTOMLKey(value, start)
	if !valid {
		return start, false, false
	}
	sensitive := tomlCredentialName.MatchString(component)
	for {
		dot := componentEnd
		for dot < len(value) && (value[dot] == ' ' || value[dot] == '\t') {
			dot++
		}
		if dot >= len(value) || value[dot] != '.' {
			return componentEnd, sensitive, true
		}

		next := dot + 1
		for next < len(value) && (value[next] == ' ' || value[next] == '\t') {
			next++
		}
		component, componentEnd, valid = parseTOMLKey(value, next)
		if !valid {
			return start, false, false
		}
		sensitive = sensitive || tomlCredentialName.MatchString(component)
	}
}

func parseTOMLKey(value string, start int) (string, int, bool) {
	if start < 0 || start >= len(value) {
		return "", start, false
	}
	switch value[start] {
	case '"':
		end := findTOMLQuotedStringEnd(value, start, '"')
		if end <= start {
			return "", start, false
		}
		key, err := strconv.Unquote(value[start:end])
		return key, end, err == nil
	case '\'':
		end := findTOMLQuotedStringEnd(value, start, '\'')
		if end <= start+1 || end > len(value) || value[end-1] != '\'' {
			return "", start, false
		}
		return value[start+1 : end-1], end, true
	default:
		end := start
		for end < len(value) && isTOMLBareKeyCharacter(value[end]) {
			end++
		}
		if end == start {
			return "", start, false
		}
		return value[start:end], end, true
	}
}

func isTOMLBareKeyCharacter(character byte) bool {
	return character == '_' || character == '-' || character >= 'a' && character <= 'z' ||
		character >= 'A' && character <= 'Z' || character >= '0' && character <= '9'
}

func nextTOMLLineStart(value string, start int) int {
	if start < 0 || start >= len(value) {
		return len(value)
	}
	newline := strings.IndexByte(value[start:], '\n')
	if newline < 0 {
		return len(value)
	}
	return start + newline + 1
}

func findTOMLValueEnd(value string, start int) int {
	if strings.HasPrefix(value[start:], `"""`) || strings.HasPrefix(value[start:], `'''`) {
		end := findTOMLMultilineStringEnd(value, start)
		if end < 0 {
			return len(value)
		}
		return end + 1
	}
	switch value[start] {
	case '"', '\'':
		return findTOMLQuotedStringEnd(value, start, value[start])
	case '{', '[':
		return findTOMLCompositeEnd(value, start)
	default:
		end := start
		for end < len(value) && value[end] != '#' && value[end] != '\r' && value[end] != '\n' {
			end++
		}
		for end > start && (value[end-1] == ' ' || value[end-1] == '\t') {
			end--
		}
		return end
	}
}

func findTOMLQuotedStringEnd(value string, start int, quote byte) int {
	for index := start + 1; index < len(value); index++ {
		if value[index] == '\r' || value[index] == '\n' {
			return index
		}
		if quote == '"' && value[index] == '\\' {
			index++
			continue
		}
		if value[index] == quote {
			return index + 1
		}
	}
	return len(value)
}

func findTOMLCompositeEnd(value string, start int) int {
	stack := make([]byte, 0, 4)
	stringDelimiter := ""
	for index := start; index < len(value); {
		if stringDelimiter != "" {
			if stringDelimiter[0] == '"' && value[index] == '\\' {
				index += 2
				continue
			}
			if strings.HasPrefix(value[index:], stringDelimiter) {
				index += len(stringDelimiter)
				stringDelimiter = ""
				continue
			}
			index++
			continue
		}

		if value[index] == '#' {
			for index < len(value) && value[index] != '\n' {
				index++
			}
			continue
		}
		if strings.HasPrefix(value[index:], `"""`) || strings.HasPrefix(value[index:], `'''`) {
			stringDelimiter = value[index : index+3]
			index += 3
			continue
		}
		switch value[index] {
		case '"', '\'':
			stringDelimiter = value[index : index+1]
			index++
		case '{', '[':
			stack = append(stack, value[index])
			index++
		case '}':
			if len(stack) > 0 && stack[len(stack)-1] == '{' {
				stack = stack[:len(stack)-1]
			}
			index++
			if len(stack) == 0 {
				return index
			}
		case ']':
			if len(stack) > 0 && stack[len(stack)-1] == '[' {
				stack = stack[:len(stack)-1]
			}
			index++
			if len(stack) == 0 {
				return index
			}
		default:
			index++
		}
	}
	return len(value)
}

func redactTOMLMultilineCredentials(value string) (string, bool) {
	return redactDelimitedValues(value, tomlMultilineCredentialStart, func(value string, open int) int {
		return exclusiveEnd(findTOMLMultilineStringEnd(value, open))
	})
}

func findTOMLMultilineStringEnd(value string, open int) int {
	if open < 0 || open+3 > len(value) {
		return -1
	}
	delimiter := value[open : open+3]
	if delimiter != `"""` && delimiter != `'''` {
		return -1
	}

	for index := open + 3; index+3 <= len(value); index++ {
		if delimiter == `"""` && value[index] == '\\' {
			index++
			continue
		}
		if value[index:index+3] == delimiter {
			return index + 2
		}
	}
	return -1
}
