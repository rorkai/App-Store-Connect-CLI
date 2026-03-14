package workflow

import (
	"bytes"
	"encoding/json"
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strings"
)

var stepOutputPattern = regexp.MustCompile(`\$\{steps\.([a-zA-Z0-9_-]+)\.([a-zA-Z0-9_]+)\}`)

func interpolateCommand(input string, outputs map[string]map[string]string) (string, error) {
	return interpolateStepOutputs(input, outputs, true)
}

func interpolateMapValues(values map[string]string, outputs map[string]map[string]string) (map[string]string, error) {
	if len(values) == 0 {
		return nil, nil
	}
	resolved := make(map[string]string, len(values))
	for _, key := range slices.Sorted(maps.Keys(values)) {
		value, err := interpolateStepOutputs(values[key], outputs, false)
		if err != nil {
			return nil, err
		}
		resolved[key] = value
	}
	return resolved, nil
}

func interpolateStepOutputs(input string, outputs map[string]map[string]string, shellEscape bool) (string, error) {
	matches := stepOutputPattern.FindAllStringSubmatchIndex(input, -1)
	if len(matches) == 0 {
		return input, nil
	}

	var b strings.Builder
	last := 0
	for _, match := range matches {
		stepName := input[match[2]:match[3]]
		outputName := input[match[4]:match[5]]

		stepOutputs, ok := outputs[stepName]
		if !ok {
			return "", fmt.Errorf("unknown step output %q", "steps."+stepName+"."+outputName)
		}
		value, ok := stepOutputs[outputName]
		if !ok {
			return "", fmt.Errorf("unknown step output %q", "steps."+stepName+"."+outputName)
		}

		b.WriteString(input[last:match[0]])
		if shellEscape {
			b.WriteString(escapeShellValue(value, shellQuoteContextAt(input, match[0])))
		} else {
			b.WriteString(value)
		}
		last = match[1]
	}
	b.WriteString(input[last:])
	return b.String(), nil
}

type shellQuoteContext int

const (
	shellQuoteContextNone shellQuoteContext = iota
	shellQuoteContextSingle
	shellQuoteContextDouble
)

func shellQuoteContextAt(input string, pos int) shellQuoteContext {
	ctx := shellQuoteContextNone
	escaped := false

	for i := 0; i < pos; i++ {
		ch := input[i]

		switch ctx {
		case shellQuoteContextNone:
			if escaped {
				escaped = false
				continue
			}
			switch ch {
			case '\\':
				escaped = true
			case '\'':
				ctx = shellQuoteContextSingle
			case '"':
				ctx = shellQuoteContextDouble
			}
		case shellQuoteContextSingle:
			if ch == '\'' {
				ctx = shellQuoteContextNone
			}
		case shellQuoteContextDouble:
			if escaped {
				escaped = false
				continue
			}
			switch ch {
			case '\\':
				escaped = true
			case '"':
				ctx = shellQuoteContextNone
			}
		}
	}

	return ctx
}

func escapeShellValue(value string, ctx shellQuoteContext) string {
	switch ctx {
	case shellQuoteContextSingle:
		return strings.ReplaceAll(value, "'", `'\''`)
	case shellQuoteContextDouble:
		return strings.NewReplacer(
			`\`, `\\`,
			`"`, `\"`,
			`$`, `\$`,
			"`", "\\`",
		).Replace(value)
	default:
		return shellQuote(value)
	}
}

func extractDeclaredOutputs(outputDecls map[string]string, stdout []byte) (map[string]string, error) {
	trimmed := bytes.TrimSpace(stdout)
	if len(trimmed) == 0 {
		return nil, fmt.Errorf("extract outputs: command stdout was empty")
	}

	var payload any
	if err := json.Unmarshal(trimmed, &payload); err != nil {
		return nil, fmt.Errorf("extract outputs: parse command stdout as JSON: %w", err)
	}

	results := make(map[string]string, len(outputDecls))
	for _, outputName := range slices.Sorted(maps.Keys(outputDecls)) {
		value, err := evaluateJSONPath(payload, outputDecls[outputName])
		if err != nil {
			return nil, fmt.Errorf("extract outputs: %s: %w", outputName, err)
		}
		results[outputName] = value
	}
	return results, nil
}

func evaluateJSONPath(payload any, expr string) (string, error) {
	trimmed := strings.TrimSpace(expr)
	if !strings.HasPrefix(trimmed, "$.") {
		return "", fmt.Errorf("unsupported JSON path %q", expr)
	}

	current := payload
	for _, segment := range strings.Split(strings.TrimPrefix(trimmed, "$."), ".") {
		obj, ok := current.(map[string]any)
		if !ok {
			return "", fmt.Errorf("path %q traversed non-object at %q", expr, segment)
		}
		next, ok := obj[segment]
		if !ok {
			return "", fmt.Errorf("path %q missing field %q", expr, segment)
		}
		current = next
	}

	switch value := current.(type) {
	case nil:
		return "", fmt.Errorf("path %q resolved to null", expr)
	case string:
		return value, nil
	case bool:
		if value {
			return "true", nil
		}
		return "false", nil
	case float64:
		return strings.TrimSuffix(strings.TrimSuffix(fmt.Sprintf("%f", value), "0"), "."), nil
	default:
		data, err := json.Marshal(value)
		if err != nil {
			return "", fmt.Errorf("path %q: marshal value: %w", expr, err)
		}
		return string(data), nil
	}
}

func shellQuote(value string) string {
	if value == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(value, "'", `'"'"'`) + "'"
}
