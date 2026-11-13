package snitch

import (
	"maps"
	"math"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// redactCommandScopedSensitiveFlags keeps generic credential flags scoped to
// commands that may consume their values. Text-producing commands such as
// echo and grep commonly appear in diagnostics as examples; redacting their
// prose would both corrupt useful reports and make the sanitizer surprising.
// Nested substitutions are handled before this pass, so a command such as
// echo "$(curl --user ...)" still redacts the inner command's credential.
func redactCommandScopedSensitiveFlags(value string) (string, bool) {
	result := value
	changed := false
	for start := 0; start < len(result); {
		end := findScopedSensitiveCommandEnd(result, start)
		command := result[start:end]
		if !isBenignTextCommand(command) {
			redacted, commandChanged := redactCommandScopedFlagValues(command)
			changed = changed || commandChanged
			if redacted != command {
				result = result[:start] + redacted + result[end:]
			}
		}

		separator := start + len(command)
		if separator >= len(result) {
			break
		}
		start = separator + 1
		if result[separator] == '\r' && start < len(result) && result[start] == '\n' {
			start++
		}
	}
	return result, changed
}

func findScopedSensitiveCommandEnd(value string, start int) int {
	end := findShellCommandEnd(value, start)
	for end < len(value) {
		if end == 0 || (value[end-1] != '^' && value[end-1] != '`') {
			break
		}
		if value[end] == '\r' && end+1 < len(value) && value[end+1] == '\n' {
			end += 2
		} else if value[end] == '\n' {
			end++
		} else {
			break
		}
		end = findShellCommandEnd(value, end)
	}
	return end
}

func isBenignTextCommand(command string) bool {
	words := shellWordValues(splitCredentialShellWordSpans(command))
	return credentialCommandIndex(words, "echo", "printf", "grep", "rg", "ripgrep") >= 0
}

// redactShellCommands applies redact to each command of value. Commands end at
// a newline or a ;, & or | operator outside quotes and parentheses.
func redactShellCommands(value string, redact func(command string) (string, bool)) (string, bool) {
	result := value
	changed := false
	for start := 0; start < len(result); {
		end := findShellCommandEnd(result, start)
		command := result[start:end]
		if next, commandChanged := redact(command); commandChanged {
			result = result[:start] + next + result[end:]
			command = next
			changed = true
		}

		separator := start + len(command)
		if separator >= len(result) {
			break
		}
		start = separator + 1
		if result[separator] == '\r' && start < len(result) && result[start] == '\n' {
			start++
		}
	}
	return result, changed
}

// redactCommandFrom redacts command from its start-th byte, the first byte of
// the program word, so words before the program are never treated as options.
func redactCommandFrom(command string, start int, redact func(string) (string, bool)) (string, bool) {
	redacted, changed := redact(command[start:])
	if !changed {
		return command, false
	}
	return command[:start] + redacted, true
}

func shellWordValues(spans []credentialShellWord) []string {
	words := make([]string, 0, len(spans))
	for _, span := range spans {
		words = append(words, span.value)
	}
	return words
}

// credentialCommandIndex returns the index of the first word that runs one of
// names after only assignments, shell keywords and transparent wrappers, or -1.
func credentialCommandIndex(words []string, names ...string) int {
	for index, word := range words {
		if slices.Contains(names, commandBaseName(word)) && isCredentialCommandPrefix(words[:index]) {
			return index
		}
	}
	return -1
}

func newCommandCredentialFlagPatternWithSuffix(suffix string, flags ...string) *lazyRegexp {
	escapedFlags := make([]string, 0, len(flags))
	for _, flag := range flags {
		escapedFlags = append(escapedFlags, regexp.QuoteMeta(flag))
	}
	return lazyMustCompile(`(^|[ \t]|\\\r?\n)(-(?:` + strings.Join(escapedFlags, "|") + `)` + suffix + `(?:` + shellCommandPathSeparator + `|[ \t]*=[ \t]*))(?:\[REDACTED(?: PRIVATE KEY)?\]|` + escapeAwareQuotedValue + `|` + unterminatedQuotedValue + `|` + commandFlagUnquotedValue + `)`)
}

func newCommandShortCredentialFlagPattern(flags ...string) *lazyRegexp {
	escapedFlags := make([]string, 0, len(flags))
	for _, flag := range flags {
		escapedFlags = append(escapedFlags, regexp.QuoteMeta(flag))
	}
	return lazyMustCompile(`(^|[ \t]|\\\r?\n)(-(?:` + strings.Join(escapedFlags, "|") + `)(?:(?:[ \t]*=[ \t]*|` + shellCommandPathSeparator + `))?)(?:\[REDACTED(?: PRIVATE KEY)?\]|` + commandShortShellWord + `|` + unterminatedQuotedValue + `)`)
}

func newCommandCredentialFlagValueStartPattern(flags ...string) *lazyRegexp {
	escapedFlags := make([]string, 0, len(flags))
	for _, flag := range flags {
		escapedFlags = append(escapedFlags, regexp.QuoteMeta(flag))
	}
	return lazyMustCompile(`(^|[ \t]|\\\r?\n)-(?:` + strings.Join(escapedFlags, "|") + `)(?:` + shellCommandPathSeparator + `|[ \t]*=[ \t]*)`)
}

// redactCommandCredentialFlagValuesMatching redacts the shell word after each
// match of pattern, or only the words isSecret accepts when it is not nil.
func redactCommandCredentialFlagValuesMatching(command string, pattern *lazyRegexp, isSecret func(string) bool) (string, bool) {
	result := command
	changed := false
	for searchStart := 0; searchStart < len(result); {
		match := pattern.FindStringIndex(result[searchStart:])
		if match == nil {
			break
		}
		valueStart := searchStart + match[1]
		valueEnd := findCredentialShellWordEnd(result, valueStart)
		if valueEnd <= valueStart {
			searchStart = valueStart
			continue
		}
		if result[valueStart:valueEnd] == redactionMarker || isSecret != nil && !isSecret(result[valueStart:valueEnd]) {
			searchStart = valueEnd
			continue
		}
		result = result[:valueStart] + redactionMarker + result[valueEnd:]
		searchStart = valueStart + len(redactionMarker)
		changed = true
	}
	return result, changed
}

func redactCommandCredentialOptionValues(command, subcommand string, options ...string) (string, bool) {
	spans := splitCredentialShellWordSpans(command)
	replacements := make([]textReplacement, 0, 2)
	for index := 0; index < len(spans); index++ {
		span := spans[index]
		if span.start < len(command) && command[span.start] == '#' {
			break
		}
		option := normalizeCredentialShellOptionToken(command[span.start:span.end])
		if option == "--" {
			break
		}

		matchedOption := ""
		attached := false
		for _, name := range options {
			for _, dashes := range []string{"-", "--"} {
				candidate := dashes + name
				if option == candidate {
					matchedOption = candidate
					break
				}
				if strings.HasPrefix(option, candidate+"=") {
					matchedOption = candidate
					attached = true
					break
				}
			}
			if matchedOption != "" {
				break
			}
		}
		if matchedOption == "" {
			if openSSLSubcommandOptionRequiresArgument(subcommand, option) && !strings.Contains(option, "=") && index+1 < len(spans) {
				argumentSpan := spans[index+1]
				if argumentSpan.start < len(command) && command[argumentSpan.start] == '#' {
					break
				}
				argumentEnd := credentialShellOptionArgumentEnd(command, argumentSpan)
				index++
				for index+1 < len(spans) && spans[index+1].start < argumentEnd {
					index++
				}
			}
			continue
		}

		argumentSpanIndex := index
		if !attached {
			if index+1 >= len(spans) || (spans[index+1].start < len(command) && command[spans[index+1].start] == '#') {
				continue
			}
			argumentSpanIndex = index + 1
		}
		argumentEnd := credentialShellOptionArgumentEnd(command, spans[argumentSpanIndex])
		rawArgument := command[spans[argumentSpanIndex].start:argumentEnd]
		contentStart, contentEnd := credentialShellWordContentSpan(rawArgument)
		if attached {
			content := rawArgument[contentStart:contentEnd]
			separator := strings.IndexByte(content, '=')
			if separator < 0 {
				replacements = append(replacements, textReplacement{spans[argumentSpanIndex].start, argumentEnd, redactionMarker})
				continue
			}
			if content[separator+1:] == redactionMarker {
				continue
			}
			replacements = append(replacements, textReplacement{spans[argumentSpanIndex].start + contentStart + separator + 1, spans[argumentSpanIndex].start + contentEnd, redactionMarker})
		} else if rawArgument[contentStart:contentEnd] != redactionMarker {
			replacements = append(replacements, textReplacement{spans[argumentSpanIndex].start, argumentEnd, redactionMarker})
		}
		index = argumentSpanIndex
		for index+1 < len(spans) && spans[index+1].start < argumentEnd {
			index++
		}
	}

	return applyTextReplacements(command, replacements), len(replacements) > 0
}

func normalizeCredentialShellOptionToken(raw string) string {
	var result strings.Builder
	result.Grow(len(raw))
	var quote byte
	ansiCQuote := false
	for index := 0; index < len(raw); index++ {
		character := raw[index]
		if quote == '\'' {
			if ansiCQuote && character == '\\' {
				decoded, _, tail, err := strconv.UnquoteChar(raw[index:], '\'')
				if err == nil {
					result.WriteRune(decoded)
					index += len(raw[index:]) - len(tail) - 1
					continue
				}
			}
			if character == '\'' {
				quote = 0
				ansiCQuote = false
			} else {
				result.WriteByte(character)
			}
			continue
		}
		if quote == '"' {
			if character == '"' {
				quote = 0
				continue
			}
			if character == '\\' && index+1 < len(raw) && strings.ContainsRune("$`\"\\\n", rune(raw[index+1])) {
				index++
				if raw[index] != '\n' {
					result.WriteByte(raw[index])
				}
				continue
			}
			result.WriteByte(character)
			continue
		}
		if character == '$' && index+1 < len(raw) && (raw[index+1] == '\'' || raw[index+1] == '"') {
			continue
		}
		switch character {
		case '\'', '"':
			quote = character
			ansiCQuote = character == '\'' && index > 0 && raw[index-1] == '$'
		case '\\':
			if index+1 < len(raw) {
				index++
				if raw[index] != '\n' {
					result.WriteByte(raw[index])
				}
			}
		default:
			result.WriteByte(character)
		}
	}
	return result.String()
}

func credentialShellArgumentEnd(command string, span credentialShellWord) int {
	end := findCredentialShellWordEnd(command, span.start)
	if end < span.end {
		return span.end
	}
	return end
}

func credentialShellOptionArgumentEnd(command string, span credentialShellWord) int {
	return findCredentialShellWordEnd(command, span.start)
}

func findCredentialShellWordEnd(value string, start int) int {
	var quote byte
	for index := start; index < len(value); index++ {
		if quote != 0 {
			if value[index] == '\\' && quote == '"' && index+1 < len(value) {
				index++
				continue
			}
			if value[index] == quote {
				quote = 0
			}
			continue
		}
		if value[index] == '\\' && index+1 < len(value) {
			index++
			continue
		}
		if value[index] == '\'' || value[index] == '"' {
			quote = value[index]
			continue
		}

		open := -1
		switch {
		case value[index] == '$' && index+1 < len(value) && value[index+1] == '(':
			open = index
		case value[index] == '`':
			open = index
		case (value[index] == '<' || value[index] == '>') && index+1 < len(value) && value[index+1] == '(':
			open = index + 1
		case value[index] == '(' && (index == start || !strings.ContainsRune(" \t\r\n;&|<>()", rune(value[index-1]))):
			open = index
		}
		if open >= 0 {
			if close := findShellCommandSubstitutionEnd(value, open); close >= 0 {
				index = close
				continue
			}
			return len(value)
		}

		if strings.ContainsRune(" \t\r\n;&|<>()", rune(value[index])) {
			return index
		}
	}
	return len(value)
}

// redactNamedCommandArguments redacts the arguments of every command that runs
// name: first values attached to the short flags in attachedFlags (-pSECRET),
// then the values pattern matches. Each is a pass over the whole text.
func redactNamedCommandArguments(name, attachedFlags string, pattern *lazyRegexp) func(string) (string, bool) {
	return func(value string) (string, bool) {
		redacted, attachedChanged := value, false
		if attachedFlags != "" {
			redacted, attachedChanged = redactNamedCommand(value, name, func(command string) (string, bool) {
				return redactAttachedShortCredentialArguments(command, attachedFlags)
			})
		}
		redacted, patternChanged := redactNamedCommand(redacted, name, func(command string) (string, bool) {
			next := pattern.ReplaceAllString(command, `${1}${2}`+redactionMarker)
			return next, next != command
		})
		return redacted, attachedChanged || patternChanged
	}
}

func redactNamedCommand(value, name string, redact func(string) (string, bool)) (string, bool) {
	return redactShellCommands(value, func(command string) (string, bool) {
		commandStart, ok := namedCredentialCommandStart(command, name)
		if !ok {
			return command, false
		}
		return redactCommandFrom(command, commandStart, redact)
	})
}

// redactFlagArguments redacts the values attached to the short flags in
// attachedFlags (-pSECRET), then the values pattern matches.
func redactFlagArguments(command, attachedFlags string, pattern *lazyRegexp) (string, bool) {
	redacted, _ := redactAttachedShortCredentialArguments(command, attachedFlags)
	redacted = pattern.ReplaceAllString(redacted, `${1}${2}`+redactionMarker)
	return redacted, redacted != command
}

func redactAttachedShortCredentialArguments(command, flags string) (string, bool) {
	result := command
	changed := false
	for index := 0; index < len(result); {
		if strings.ContainsRune(" \t\r\n;&|<>()", rune(result[index])) {
			index++
			continue
		}
		end := findCredentialShellWordEnd(result, index)
		if end <= index {
			index++
			continue
		}
		word := result[index:end]
		if len(word) <= 2 || word[0] != '-' || word[1] == '-' || !strings.ContainsRune(flags, rune(word[1])) {
			index = end
			continue
		}
		valueStart := 2
		if word[valueStart] == '=' {
			valueStart++
		}
		if valueStart >= len(word) || word[valueStart:] == redactionMarker {
			index = end
			continue
		}
		redacted := word[:valueStart] + redactionMarker
		result = result[:index] + redacted + result[end:]
		index += len(redacted)
		changed = true
	}
	return result, changed
}

func namedCredentialCommandStart(command, commandName string) (int, bool) {
	spans := splitCredentialShellWordSpans(command)
	words := shellWordValues(spans)
	index := credentialCommandIndex(words, commandName, commandName+".exe")
	if index < 0 {
		return 0, false
	}
	return spans[index].start, true
}

func isCredentialCommandPrefix(words []string) bool {
	if isShellArrayAssignmentPrefix(words) {
		return false
	}
	for {
		for len(words) > 0 && shellAssignmentWord.MatchString(strings.Trim(words[0], `"'`)) {
			words = words[1:]
		}
		if len(words) == 0 {
			return true
		}
		if remaining, ok := consumeShellControlPrefix(words); ok {
			words = remaining
			continue
		}

		remaining, ok := consumeCredentialCommandWrapper(words)
		if !ok {
			return false
		}
		words = remaining
	}
}

func isShellArrayAssignmentPrefix(words []string) bool {
	depth := 0
	for _, word := range words {
		word = strings.Trim(word, `"'`)
		start := 0
		if depth == 0 {
			assignment := strings.Index(word, "=(")
			if assignment <= 0 || !isCredentialShellIdentifier(word[:assignment]) {
				continue
			}
			start = assignment + 1
		}
		for index := start; index < len(word); index++ {
			if word[index] == '\\' && index+1 < len(word) {
				index++
				continue
			}
			switch word[index] {
			case '(':
				depth++
			case ')':
				if depth > 0 {
					depth--
				}
			}
		}
	}
	return depth > 0
}

func consumeShellControlPrefix(words []string) ([]string, bool) {
	if len(words) == 0 {
		return nil, false
	}
	if remaining, ok := consumeShellFunctionDeclarationPrefix(words); ok {
		return remaining, true
	}
	word := strings.Trim(words[0], `"'`)
	switch word {
	case "!", "{", "if", "then", "elif", "while", "until", "do", "else":
		return words[1:], true
	case "case":
		for index := 1; index < len(words); index++ {
			if strings.Trim(words[index], `"'`) != "in" {
				continue
			}
			for patternIndex := index + 1; patternIndex < len(words); patternIndex++ {
				if strings.HasSuffix(strings.Trim(words[patternIndex], `"'`), ")") {
					return words[patternIndex+1:], true
				}
			}
			return nil, false
		}
	}
	if strings.HasSuffix(word, ")") && !strings.HasPrefix(word, "(") {
		return words[1:], true
	}
	return nil, false
}

func consumeShellFunctionDeclarationPrefix(words []string) ([]string, bool) {
	trimmed := func(index int) string { return strings.Trim(words[index], `"'`) }
	if name := strings.TrimSuffix(trimmed(0), "(){"); name != trimmed(0) && isCredentialShellIdentifier(name) {
		return words[1:], true
	}
	if len(words) >= 2 {
		name := trimmed(0)
		if strings.HasSuffix(name, "()") && isCredentialShellIdentifier(strings.TrimSuffix(name, "()")) && trimmed(1) == "{" {
			return words[2:], true
		}
		if trimmed(0) == "function" {
			name = strings.TrimSuffix(trimmed(1), "(){")
			if name != trimmed(1) && isCredentialShellIdentifier(name) {
				return words[2:], true
			}
		}
	}
	if len(words) >= 3 && isCredentialShellIdentifier(trimmed(0)) && trimmed(1) == "()" && trimmed(2) == "{" {
		return words[3:], true
	}
	if len(words) >= 3 && trimmed(0) == "function" {
		name := strings.TrimSuffix(trimmed(1), "()")
		if isCredentialShellIdentifier(name) && trimmed(2) == "{" {
			return words[3:], true
		}
	}
	return nil, false
}

func isCredentialShellIdentifier(value string) bool {
	if value == "" || !isCredentialShellIdentifierStart(value[0]) {
		return false
	}
	for index := 1; index < len(value); index++ {
		if !isCredentialShellIdentifierStart(value[index]) && (value[index] < '0' || value[index] > '9') {
			return false
		}
	}
	return true
}

func isCredentialShellIdentifierStart(character byte) bool {
	return (character >= 'a' && character <= 'z') || (character >= 'A' && character <= 'Z') || character == '_'
}

// consumeCredentialCommandWrapper skips a program that runs a command given in
// its arguments, and the wrapper's own arguments.
func consumeCredentialCommandWrapper(words []string) ([]string, bool) {
	if len(words) == 0 {
		return nil, false
	}
	arguments := words[1:]
	switch commandBaseName(words[0]) {
	case "sudo":
		return sudoOptions.consumeOptions(arguments)
	case "doas":
		return doasOptions.consumeOptions(arguments)
	case "command":
		return commandOptions.consumeOptions(arguments)
	case "env":
		return envOptions.consumeOptions(arguments)
	case "timeout", "gtimeout":
		return consumeTimeoutWrapper(arguments)
	case "nohup":
		return consumeNohupCredentialWrapper(arguments)
	case "exec":
		return consumeExecCredentialWrapper(arguments)
	case "nice", "gnice":
		return consumeNiceCredentialWrapper(arguments)
	case "time", "gtime":
		return timeOptions.consumeOptions(arguments)
	case "stdbuf", "gstdbuf":
		return stdbufOptions.consumeOptions(arguments)
	case "setsid":
		return setsidOptions.consumeOptions(arguments)
	case "ionice":
		return ioniceOptions.consumeOptions(arguments)
	case "caffeinate":
		return caffeinateOptions.consumeOptions(arguments)
	case "arch":
		return consumeArchCredentialWrapper(arguments)
	case "xcrun":
		return consumeXcrunCredentialWrapper(arguments)
	case "launchctl":
		return consumeLaunchctlCredentialWrapper(arguments)
	case "chroot", "gchroot", "chroot.exe", "gchroot.exe":
		return consumeChrootWrapper(arguments)
	case "xargs", "gxargs":
		return xargsOptions.consumeOptions(arguments)
	case "find", "gfind":
		return consumeFindExecCredentialWrapper(arguments)
	case "watch":
		return watchOptions.consumeOptions(arguments)
	}
	return nil, false
}

// optionKind says how a long option takes a value.
type optionKind int

const (
	longFlag       optionKind = iota // --name; --name=VALUE is rejected
	longValue                        // --name VALUE, or --name=VALUE with a non-empty value
	longLooseValue                   // --name VALUE or --name=VALUE
	longAny                          // --name or --name=VALUE
	longNonEmpty                     // --name, or --name=VALUE with a non-empty value
)

// optionSpec describes the options a program accepts before its operands.
type optionSpec struct {
	flags    string // short options without a value; they can be grouped
	alone    string // short options without a value that cannot be grouped
	optional string // short options whose value can only be attached
	values   string // short options with a value, attached or in the next word
	long     map[string]optionKind
	// equalsValue attaches VALUE rather than =VALUE in -oVALUE and -o=VALUE.
	equalsValue bool
	// valid checks the value of each option, named by its letter or --name.
	valid func(option, value string) bool
}

func longOptions(kind optionKind, names string, more map[string]optionKind) map[string]optionKind {
	options := make(map[string]optionKind)
	for _, name := range strings.Fields(names) {
		options[name] = kind
	}
	maps.Copy(options, more)
	return options
}

var (
	sudoOptions = optionSpec{flags: "AbEHiKknPSs", values: "CDghpRrTtUu", long: longOptions(longLooseValue,
		"--chdir --close-from --command-timeout --group --host --other-user --prompt --role --type --user",
		longOptions(longAny, "--askpass --background --non-interactive --preserve-env --remove-timestamp --reset-timestamp --stdin", nil))}
	doasOptions    = optionSpec{flags: "n", values: "au"}
	commandOptions = optionSpec{flags: "p"}
	// env -S is transparent so the command words inside its split string
	// remain available to the command-scoped redactors.
	envOptions = optionSpec{flags: "0Siv", values: "CPu", long: longOptions(longLooseValue, "--argv0 --chdir --unset",
		longOptions(longAny, "--debug --ignore-environment --null --split-string", nil))}
	timeoutOptions = optionSpec{flags: "fpv", values: "ks", long: longOptions(longValue, "--kill-after --signal",
		longOptions(longFlag, "--foreground --preserve-status --verbose", nil))}
	timeOptions = optionSpec{flags: "ahlpqv", values: "fo", long: longOptions(longValue, "--format --output",
		longOptions(longFlag, "--append --portability --quiet --verbose", nil))}
	stdbufOptions = optionSpec{
		values: "eio", equalsValue: true, long: longOptions(longValue, "--error --input --output", nil),
		valid: func(_, mode string) bool { return isStdbufMode(mode) },
	}
	setsidOptions = optionSpec{flags: "cfw", long: longOptions(longFlag, "--ctty --fork --wait", nil)}
	ioniceOptions = optionSpec{flags: "t", values: "cn", long: longOptions(longValue, "--class --classdata",
		map[string]optionKind{"--ignore": longFlag})}
	caffeinateOptions = optionSpec{flags: "dimsu", values: "tw", valid: func(option, value string) bool {
		switch option {
		case "t":
			return isCaffeinateTimeoutArgument(value)
		case "w":
			return isCaffeinatePIDArgument(value)
		}
		return true
	}}
	chrootOptions = optionSpec{flags: "n", values: "Ggu", long: map[string]optionKind{"--groups": longLooseValue, "--userspec": longValue, "--skip-chdir": longFlag}}
	// GNU xargs -e, -i and -l take an optional attached value, so a
	// following word is the command.
	xargsOptions = optionSpec{flags: "0oprtx", optional: "eil", values: "adEIJLnPRSs", long: longOptions(longValue,
		"--arg-file --delimiter --max-args --max-procs --max-chars --process-slot-var",
		longOptions(longFlag, "--null --interactive --verbose --no-run-if-empty --exit --open-tty --show-limits",
			map[string]optionKind{"--eof": longAny, "--replace": longAny, "--max-lines": longNonEmpty}))}
	// watch -d takes an optional attached cumulative marker.
	watchOptions = optionSpec{flags: "bcCefgprtwx", optional: "d", values: "nqs", long: longOptions(longValue, "--interval --equexit --shotsdir",
		longOptions(longFlag, "--beep --color --no-color --errexit --chgexit --exec --precise --no-title --no-rerun --no-wrap",
			map[string]optionKind{"--differences": longNonEmpty}))}
	dockerOptions = optionSpec{alone: "D", values: "cHl", long: longOptions(longLooseValue,
		"--config --context --host --log-level --tlscacert --tlscert --tlskey", longOptions(longAny, "--debug --tls --tlsverify", nil))}
	kubectlOptions = optionSpec{values: "nsv", long: longOptions(longLooseValue,
		"--as --as-group --as-uid --cache-dir --certificate-authority --client-certificate --client-key --cluster --context --kubeconfig --namespace --password --profile --profile-output --request-timeout --server --tls-server-name --token --user --username --v --vmodule",
		longOptions(longAny, "--disable-compression --insecure-skip-tls-verify --match-server-version --warnings-as-errors", nil))}
	shellOptions = optionSpec{long: longOptions(longValue, "--init-file --rcfile",
		longOptions(longFlag, "--login --noediting --noprofile --norc --posix --restricted --verbose", nil))}
)

// option parses one option word. It returns the option's letter or --name, the
// value attached to it, and whether the value is the next word instead.
func (spec optionSpec) option(option string) (name, value string, valueFollows, ok bool) {
	if strings.HasPrefix(option, "--") {
		name, value, attached := strings.Cut(option, "=")
		kind, known := spec.long[name]
		emptyValue := attached && value == ""
		if !known || kind == longFlag && attached || (kind == longValue || kind == longNonEmpty) && emptyValue {
			return "", "", false, false
		}
		return name, value, !attached && (kind == longValue || kind == longLooseValue), true
	}
	for index := 1; index < len(option); index++ {
		flag := option[index]
		switch {
		case strings.IndexByte(spec.flags, flag) >= 0, strings.IndexByte(spec.alone, flag) >= 0 && len(option) == 2:
			continue
		case strings.IndexByte(spec.optional, flag) >= 0:
			return option[index : index+1], option[index+1:], false, true
		case strings.IndexByte(spec.values, flag) >= 0:
			value := option[index+1:]
			if spec.equalsValue {
				value = strings.TrimPrefix(value, "=")
			}
			return option[index : index+1], value, value == "", true
		}
		return "", "", false, false
	}
	return "", "", false, true
}

// consumeOptions skips the options before the first operand in words.
func (spec optionSpec) consumeOptions(words []string) ([]string, bool) {
	for len(words) > 0 {
		option := strings.Trim(words[0], `"'`)
		if option == "--" {
			return words[1:], true
		}
		if len(option) < 2 || option[0] != '-' {
			return words, true
		}
		name, value, valueFollows, ok := spec.option(option)
		words = words[1:]
		if ok && valueFollows {
			if len(words) == 0 {
				return nil, false
			}
			value = strings.Trim(words[0], `"'`)
			words = words[1:]
		}
		if !ok || spec.valid != nil && !spec.valid(name, value) {
			return nil, false
		}
	}
	return words, true
}

// consumeTimeoutWrapper skips timeout's options and its duration.
func consumeTimeoutWrapper(words []string) ([]string, bool) {
	words, ok := timeoutOptions.consumeOptions(words)
	if !ok || len(words) == 0 || !isTimeoutDurationWord(strings.Trim(words[0], `"'`)) {
		return nil, false
	}
	return words[1:], true
}

// consumeChrootWrapper skips chroot's options and its new root.
func consumeChrootWrapper(words []string) ([]string, bool) {
	words, ok := chrootOptions.consumeOptions(words)
	if !ok || len(words) == 0 || strings.Trim(words[0], `"'`) == "" {
		return nil, false
	}
	return words[1:], true
}

func consumeFindExecCredentialWrapper(words []string) ([]string, bool) {
	for index := 0; index < len(words); index++ {
		word := strings.Trim(words[index], `"'`)
		switch word {
		case "-exec", "-execdir", "-ok", "-okdir":
			return words[index+1:], true
		}
		argumentCount := findExpressionArgumentCount(word)
		if argumentCount > 0 {
			index += argumentCount
		}
	}
	return nil, false
}

func findExpressionArgumentCount(word string) int {
	switch word {
	case "-name", "-iname", "-path", "-ipath", "-wholename", "-iwholename", "-lname", "-ilname",
		"-regex", "-iregex", "-type", "-xtype", "-user", "-group", "-uid", "-gid", "-perm", "-size",
		"-Bmin", "-Bnewer", "-Btime", "-atime", "-amin", "-anewer", "-ctime", "-cmin", "-cnewer", "-mtime", "-mmin", "-mnewer", "-newer", "-newermt",
		"-fstype", "-links", "-inum", "-samefile", "-maxdepth", "-mindepth", "-regextype", "-printf",
		"-fprint", "-fprint0", "-fls", "-flags", "-used", "-xattrname", "-context", "-f", "-D", "-O", "-files0-from":
		return 1
	case "-fprintf":
		return 2
	}
	if len(word) == len("-newerXY") && strings.HasPrefix(word, "-newer") {
		return 1
	}
	return 0
}

func consumeNohupCredentialWrapper(words []string) ([]string, bool) {
	if len(words) == 0 {
		return words, true
	}
	if strings.Trim(words[0], `"'`) == "--" {
		return words[1:], true
	}
	if strings.HasPrefix(strings.Trim(words[0], `"'`), "-") {
		return nil, false
	}
	return words, true
}

func consumeExecCredentialWrapper(words []string) ([]string, bool) {
	for len(words) > 0 {
		option := strings.Trim(words[0], `"'`)
		if option == "--" {
			return words[1:], true
		}
		if option == "-a" {
			if len(words) < 2 {
				return nil, false
			}
			words = words[2:]
			continue
		}
		if option == "-c" || option == "-l" || option == "-cl" || option == "-lc" {
			words = words[1:]
			continue
		}
		if strings.HasPrefix(option, "-") && option != "-" {
			return nil, false
		}
		return words, true
	}
	return words, true
}

func consumeNiceCredentialWrapper(words []string) ([]string, bool) {
	for len(words) > 0 {
		option := strings.Trim(words[0], `"'`)
		if option == "--" {
			return words[1:], true
		}
		if option == "-n" || option == "--adjustment" {
			if len(words) < 2 || !isNiceAdjustment(strings.Trim(words[1], `"'`)) {
				return nil, false
			}
			words = words[2:]
			continue
		}
		if strings.HasPrefix(option, "--adjustment=") {
			if !isNiceAdjustment(strings.TrimPrefix(option, "--adjustment=")) {
				return nil, false
			}
			words = words[1:]
			continue
		}
		if strings.HasPrefix(option, "-n") && len(option) > 2 {
			if !isNiceAdjustment(option[2:]) {
				return nil, false
			}
			words = words[1:]
			continue
		}
		if strings.HasPrefix(option, "-") && len(option) > 1 {
			if !isNiceAdjustment(option[1:]) {
				return nil, false
			}
			words = words[1:]
			continue
		}
		return words, true
	}
	return words, true
}

func isNiceAdjustment(value string) bool {
	if value == "" {
		return false
	}
	_, err := strconv.Atoi(value)
	return err == nil
}

func isStdbufMode(value string) bool {
	if value == "0" || value == "L" || value == "B" {
		return true
	}
	value = strings.TrimSuffix(value, "B")
	if len(value) > 0 && strings.ContainsRune("KMGTPEZY", rune(value[len(value)-1])) {
		value = value[:len(value)-1]
	}
	if value == "" {
		return false
	}
	_, err := strconv.ParseUint(value, 10, 64)
	return err == nil
}

func isCaffeinateTimeoutArgument(value string) bool {
	if isCredentialDynamicShellArgument(value) {
		return true
	}
	timeout, err := strconv.ParseFloat(value, 64)
	return (err == nil && !math.IsNaN(timeout) && !math.IsInf(timeout, 0)) || hasCredentialNumericPrefix(value)
}

func isCaffeinatePIDArgument(value string) bool {
	if isCredentialDynamicShellArgument(value) {
		return true
	}
	_, err := strconv.ParseUint(value, 10, 64)
	return err == nil || hasCredentialNumericPrefix(value)
}

func isCredentialDynamicShellArgument(value string) bool {
	return strings.ContainsAny(value, "$`")
}

func hasCredentialNumericPrefix(value string) bool {
	value = strings.TrimPrefix(strings.TrimPrefix(value, "+"), "-")
	return value != "" && value[0] >= '0' && value[0] <= '9'
}

func consumeArchCredentialWrapper(words []string) ([]string, bool) {
	for len(words) > 0 {
		option := strings.Trim(words[0], `"'`)
		if option == "--" {
			return nil, false
		}
		if len(option) < 2 || option[0] != '-' || option == "-" {
			return words, true
		}
		switch option {
		case "-32", "-64", "-arm64", "-arm64e", "-x86_64", "-x86_64h", "-i386":
			words = words[1:]
		case "-arch":
			if len(words) < 2 || !isCredentialArchName(strings.Trim(words[1], `"'`)) {
				return nil, false
			}
			words = words[2:]
		case "-c":
			words = words[1:]
		case "-d":
			if len(words) < 2 || strings.Trim(words[1], `"'`) == "" {
				return nil, false
			}
			words = words[2:]
		case "-e":
			if len(words) < 2 || !isCredentialArchEnvironmentAssignment(strings.Trim(words[1], `"'`)) {
				return nil, false
			}
			words = words[2:]
		default:
			return nil, false
		}
	}
	return words, true
}

func isCredentialArchName(value string) bool {
	switch value {
	case "i386", "x86_64", "x86_64h", "arm64", "arm64e":
		return true
	default:
		return false
	}
}

func isCredentialArchEnvironmentAssignment(value string) bool {
	_, _, found := strings.Cut(value, "=")
	return found
}

func consumeXcrunCredentialWrapper(words []string) ([]string, bool) {
	for len(words) > 0 {
		option := strings.Trim(words[0], `"'`)
		if option == "--" {
			return words[1:], true
		}
		if len(option) < 2 || option[0] != '-' || option == "-" {
			return words, true
		}
		switch option {
		case "--sdk", "--toolchain", "-sdk", "-toolchain":
			if len(words) < 2 || strings.HasPrefix(strings.Trim(words[1], `"'`), "-") {
				return nil, false
			}
			words = words[2:]
		case "--log", "--verbose", "--no-cache", "--kill-cache", "--run", "-log", "-verbose", "-no-cache", "-kill-cache", "-run", "-l", "-v", "-n", "-k", "-r":
			words = words[1:]
		default:
			return nil, false
		}
	}
	return words, true
}

func consumeLaunchctlCredentialWrapper(words []string) ([]string, bool) {
	if len(words) == 0 {
		return nil, false
	}
	mode := strings.Trim(words[0], `"'`)
	if mode == "submit" {
		return consumeLaunchctlSubmitCredentialWrapper(words[1:])
	}
	if len(words) < 2 || (mode != "asuser" && mode != "bsexec") {
		return nil, false
	}
	uid := strings.Trim(words[1], `"'`)
	if _, err := strconv.ParseUint(uid, 10, 64); err != nil {
		return nil, false
	}
	return words[2:], true
}

func consumeLaunchctlSubmitCredentialWrapper(words []string) ([]string, bool) {
	hasLabel := false
	executable := ""
	for len(words) > 0 {
		option := strings.Trim(words[0], `"'`)
		switch option {
		case "-l", "-o", "-e":
			if len(words) < 2 {
				return nil, false
			}
			if option == "-l" {
				hasLabel = true
			}
			words = words[2:]
		case "-p":
			if !hasLabel {
				return nil, false
			}
			if len(words) == 1 {
				return nil, true
			}
			executable = words[1]
			words = words[2:]
		case "--":
			if !hasLabel {
				return nil, false
			}
			if executable != "" {
				return append([]string{executable}, words[1:]...), true
			}
			return words[1:], true
		default:
			return nil, false
		}
	}
	return nil, false
}

func isTimeoutDurationWord(value string) bool {
	if len(value) > 1 && strings.ContainsRune("smhdSMHD", rune(value[len(value)-1])) {
		value = value[:len(value)-1]
	}
	duration, err := strconv.ParseFloat(value, 64)
	return err == nil && duration >= 0 && !math.IsNaN(duration)
}

func commandBaseName(word string) string {
	prefix := strings.ToLower(strings.Trim(word, `"'`))
	if strings.HasPrefix(prefix, "((") {
		return prefix
	}
	if assignment := strings.Index(prefix, "=("); assignment > 0 && isCredentialShellIdentifier(prefix[:assignment]) {
		return prefix
	}
	prefix = strings.TrimLeft(prefix, "(")
	prefix = strings.TrimRight(prefix, ")")
	windowsPath := (len(prefix) >= 3 && prefix[0] >= 'a' && prefix[0] <= 'z' && prefix[1] == ':' && (prefix[2] == '\\' || prefix[2] == '/')) ||
		strings.HasPrefix(prefix, `\\`) || (strings.HasSuffix(prefix, ".exe") && strings.Contains(prefix, `\`))
	if windowsPath {
		if separator := strings.LastIndexAny(prefix, `/\\`); separator >= 0 {
			prefix = prefix[separator+1:]
		}
	} else if separator := strings.LastIndex(prefix, "/"); separator >= 0 {
		prefix = prefix[separator+1:]
	}
	if !windowsPath {
		var unescaped strings.Builder
		unescaped.Grow(len(prefix))
		for index := 0; index < len(prefix); index++ {
			if prefix[index] == '\\' && index+1 < len(prefix) {
				index++
			}
			unescaped.WriteByte(prefix[index])
		}
		prefix = unescaped.String()
	}
	return prefix
}

func normalizeCredentialCommand(command string) string {
	normalized := strings.NewReplacer("\\\r\n", " ", "\\\n", " ").Replace(command)
	return envLongSplitStringOption.ReplaceAllString(normalized, "-S ")
}

func credentialCommandWords(command string) []string {
	words := splitCredentialShellWords(normalizeCredentialCommand(command))
	for index := 0; index < len(words); index++ {
		if commandBaseName(words[index]) != "env" || !isCredentialCommandPrefix(words[:index]) {
			continue
		}
		for optionIndex := index + 1; optionIndex < len(words); {
			word := strings.Trim(words[optionIndex], `"'`)
			if shellAssignmentWord.MatchString(word) {
				optionIndex++
				continue
			}
			if option, inner, attached := envSplitStringOption(word); option != "" {
				words[optionIndex] = option
				if attached {
					innerWords := splitCredentialShellWords(inner)
					words = append(words[:optionIndex+1], append(innerWords, words[optionIndex+1:]...)...)
				} else if optionIndex+1 < len(words) {
					innerWords := splitCredentialShellWords(words[optionIndex+1])
					words = append(words[:optionIndex+1], append(innerWords, words[optionIndex+2:]...)...)
				}
				break
			}
			if len(word) < 2 || word[0] != '-' || word == "-" {
				break
			}
			_, _, requiresArgument, allowed := envOptions.option(word)
			if !allowed {
				break
			}
			optionIndex++
			if requiresArgument {
				optionIndex++
			}
		}
	}
	return words
}

func envSplitStringOption(option string) (string, string, bool) {
	if option == "--split-string" {
		return "-S", "", false
	}
	if len(option) < 2 || option[0] != '-' || option[1] == '-' {
		return "", "", false
	}
	for index := 1; index < len(option); index++ {
		switch option[index] {
		case 'i', 'v':
			continue
		case 'S':
			return option[:index+1], option[index+1:], index+1 < len(option)
		default:
			return "", "", false
		}
	}
	return "", "", false
}

type credentialShellWord struct {
	value        string
	start        int
	end          int
	contentStart int
	contentEnd   int
}

func splitCredentialShellWords(value string) []string {
	return shellWordValues(splitCredentialShellWordSpans(value))
}

func splitCredentialShellWordSpans(value string) []credentialShellWord {
	words := make([]credentialShellWord, 0, len(strings.Fields(value)))
	var word strings.Builder
	var quote byte
	started := false
	wordStart := 0
	flush := func(end int) {
		if !started {
			return
		}
		contentStart, contentEnd := credentialShellWordContentSpan(value[wordStart:end])
		words = append(words, credentialShellWord{
			value:        word.String(),
			start:        wordStart,
			end:          end,
			contentStart: wordStart + contentStart,
			contentEnd:   wordStart + contentEnd,
		})
		word.Reset()
		started = false
	}
	for index := 0; index < len(value); index++ {
		character := value[index]
		if !started && character != ' ' && character != '\t' && character != '\r' && character != '\n' {
			wordStart = index
		}
		switch {
		case character == '\\' && quote != '\'' && index+1 < len(value) && value[index+1] == '\n':
			index++
		case character == '\\' && quote != '\'' && index+2 < len(value) && value[index+1] == '\r' && value[index+2] == '\n':
			index += 2
		case character == '\\' && quote != '\'' && index+1 < len(value):
			started = true
			word.WriteByte(character)
			index++
			word.WriteByte(value[index])
		case quote == '\'' && character == '\'':
			quote = 0
		case quote == '"' && character == '"':
			quote = 0
		case quote == 0 && (character == '\'' || character == '"'):
			started = true
			quote = character
		case quote == 0 && (character == ' ' || character == '\t' || character == '\r' || character == '\n'):
			flush(index)
		default:
			started = true
			word.WriteByte(character)
		}
	}
	flush(len(value))
	return words
}

func credentialShellWordContentSpan(raw string) (int, int) {
	quoteStart := 0
	if len(raw) >= 3 && raw[0] == '$' && (raw[1] == '\'' || raw[1] == '"') {
		quoteStart = 1
	}
	if len(raw)-quoteStart < 2 || (raw[quoteStart] != '\'' && raw[quoteStart] != '"') {
		return 0, len(raw)
	}
	quote := raw[quoteStart]
	for index := quoteStart + 1; index < len(raw); index++ {
		if quote == '"' && raw[index] == '\\' && index+1 < len(raw) {
			index++
			continue
		}
		if raw[index] != quote {
			continue
		}
		if index == len(raw)-1 {
			return quoteStart + 1, index
		}
		return 0, len(raw)
	}
	return 0, len(raw)
}

func redactShellCommandSubstitutionContents(value string, depth int) (string, bool) {
	return redactNestedShellCommands(value, depth, true, shellCommandSubstitutionStart)
}

// shellCommandSubstitutionStart returns the opening parenthesis or backtick of
// a command or process substitution at index and the start of its command,
// or -1 and -1.
func shellCommandSubstitutionStart(value string, index int) (int, int) {
	switch {
	case value[index] == '$' && index+1 < len(value) && value[index+1] == '(' &&
		(index+2 >= len(value) || value[index+2] != '('):
		return index, index + 2
	case value[index] == '`' && isShellBacktickSubstitutionStart(value, index):
		return index, index + 1
	case (value[index] == '<' || value[index] == '>') && index+1 < len(value) && value[index+1] == '(':
		return index + 1, index + 2
	case value[index] == '(' && (index+1 >= len(value) || value[index+1] != '(') &&
		(index == 0 || !strings.ContainsRune("$<>=()", rune(value[index-1]))) &&
		!isWithinShellArithmeticExpression(value, index) && !isWithinShellArraySubscript(value, index):
		return index, index + 1
	}
	return -1, -1
}

// readShellQuoting reads value[index] while quote is the open quote, or 0. It
// returns the index of the last byte read, which includes an escaped byte, the
// open quote, and whether the byte was quoted, escaped or a quote.
func readShellQuoting(value string, index int, quote byte) (int, byte, bool) {
	switch {
	case quote == '\'':
		if value[index] == '\'' {
			quote = 0
		}
		return index, quote, true
	case value[index] == '\\':
		return index + 1, quote, true
	case quote != 0:
		if value[index] == quote {
			quote = 0
		}
		return index, quote, true
	case value[index] == '\'' || value[index] == '"' || value[index] == '`':
		return index, value[index], true
	}
	return index, quote, false
}

func isWithinShellArithmeticExpression(value string, position int) bool {
	var quote byte
	var quoting bool
	for index := 0; index < position; index++ {
		if index, quote, quoting = readShellQuoting(value, index, quote); quoting {
			continue
		}

		open := -1
		if value[index] == '$' && index+2 < position && value[index+1] == '(' && value[index+2] == '(' {
			open = index
		} else if value[index] == '(' && index+1 < position && value[index+1] == '(' {
			open = index
		}
		if open < 0 {
			continue
		}
		close := findShellCommandSubstitutionEnd(value, open)
		if close >= position {
			return true
		}
		if close >= 0 {
			index = close
		}
	}
	return false
}

func isWithinShellArraySubscript(value string, position int) bool {
	type subscript struct {
		fishSlice bool
	}
	openers := make([]subscript, 0, 2)
	closedFishSlice := make(map[int]bool)
	var quote byte
	var quoting bool
	for index := 0; index < position; index++ {
		if index, quote, quoting = readShellQuoting(value, index, quote); quoting {
			continue
		}
		switch value[index] {
		case '[':
			identifierStart := index
			for identifierStart > 0 {
				character := value[identifierStart-1]
				if !isCredentialShellIdentifierStart(character) && (character < '0' || character > '9') {
					break
				}
				identifierStart--
			}
			fishSlice := identifierStart > 0 && value[identifierStart-1] == '$'
			if !fishSlice && index > 0 && value[index-1] == ']' {
				fishSlice = closedFishSlice[index-1]
			}
			openers = append(openers, subscript{fishSlice: fishSlice})
		case ']':
			if len(openers) > 0 {
				closedFishSlice[index] = openers[len(openers)-1].fishSlice
				openers = openers[:len(openers)-1]
			}
		}
	}
	if len(openers) == 0 {
		return false
	}
	return !openers[len(openers)-1].fishSlice
}

func redactShellSubshellGroupContents(value string, depth int) (string, bool) {
	return redactNestedShellCommands(value, depth, false, func(value string, index int) (int, int) {
		if value[index] != '(' || !isShellSubshellGroupStart(value, index) {
			return -1, -1
		}
		return index, index + 1
	})
}

// redactNestedShellCommands redacts, one nesting level deeper, the command
// inside each construct that start finds outside single quotes, and inside
// double quotes when inDoubleQuotes is set.
func redactNestedShellCommands(value string, depth int, inDoubleQuotes bool, start func(value string, index int) (open, contentStart int)) (string, bool) {
	result := value
	changed := false
	var quote byte
	for index := 0; index < len(result); index++ {
		switch quote {
		case '\'':
			if result[index] == '\'' {
				quote = 0
			}
			continue
		case '"':
			if result[index] == '"' {
				quote = 0
				continue
			}
			if !inDoubleQuotes {
				continue
			}
		}
		if result[index] == '\\' {
			index++
			continue
		}
		if result[index] == '"' {
			quote = '"'
			continue
		}
		if quote == 0 && result[index] == '\'' {
			quote = '\''
			continue
		}
		open, contentStart := start(result, index)
		if open < 0 {
			continue
		}
		close := findShellCommandSubstitutionEnd(result, open)
		if close < contentStart {
			continue
		}
		redacted, innerChanged := redactSensitiveTextDepth(result[contentStart:close], depth+1)
		if innerChanged {
			result = result[:contentStart] + redacted + result[close:]
			close = contentStart + len(redacted)
			changed = true
		}
		index = close
	}
	return result, changed
}

func isShellSubshellGroupStart(value string, index int) bool {
	if index < 0 || index >= len(value) || value[index] != '(' {
		return false
	}
	if index+1 < len(value) && value[index+1] == '(' {
		return false
	}
	if index > 0 {
		previous := value[index-1]
		if previous == '$' || previous == '<' || previous == '>' || previous == '(' {
			return false
		}
		if strings.ContainsRune(";&|{!", rune(previous)) {
			return true
		}
		if !strings.ContainsRune(" \t\r\n", rune(previous)) {
			return false
		}
		if previous == '\r' || previous == '\n' {
			return true
		}
		wordEnd := index - 1
		for wordEnd >= 0 && (value[wordEnd] == ' ' || value[wordEnd] == '\t') {
			wordEnd--
		}
		if wordEnd < 0 || value[wordEnd] == '\r' || value[wordEnd] == '\n' || strings.ContainsRune(";&|{!", rune(value[wordEnd])) {
			return true
		}
		wordStart := wordEnd
		for wordStart > 0 && !strings.ContainsRune(" \t\r\n;&|{!", rune(value[wordStart-1])) {
			wordStart--
		}
		switch value[wordStart : wordEnd+1] {
		case "if", "then", "elif", "while", "until", "do", "else":
			return true
		default:
			return isShellFunctionSubshellBodyPrefix(value[:index])
		}
	}
	return true
}

func isShellFunctionSubshellBodyPrefix(value string) bool {
	words := splitCredentialShellWords(value)
	if len(words) == 0 {
		return false
	}
	last := strings.Trim(words[len(words)-1], `"'`)
	if strings.HasSuffix(last, "()") && isCredentialShellIdentifier(strings.TrimSuffix(last, "()")) {
		return true
	}
	if len(words) >= 2 && last == "()" && isCredentialShellIdentifier(strings.Trim(words[len(words)-2], `"'`)) {
		return true
	}
	return len(words) >= 2 && strings.Trim(words[len(words)-2], `"'`) == "function" && isCredentialShellIdentifier(last)
}

func isShellBacktickSubstitutionStart(value string, index int) bool {
	if index == 0 {
		return true
	}
	return strings.ContainsRune(" \t\r\n=([{,:;|&\"", rune(value[index-1]))
}

func redactEnvSplitCommandStrings(value string, depth int) (string, bool) {
	return redactShellCommands(value, func(command string) (string, bool) {
		contentStart, contentEnd, ok := envSplitCommandStringSpan(command)
		if !ok {
			return command, false
		}
		inner := command[contentStart:contentEnd]
		redacted, innerChanged := redactSensitiveTextDepth(inner, depth+1)
		if !innerChanged && strings.Contains(inner, `\`) {
			if _, innerChanged = redactSensitiveTextDepth(decodeEnvSplitStringForRedaction(inner), depth+1); innerChanged {
				redacted = redactionMarker
			}
		}
		return command[:contentStart] + redacted + command[contentEnd:], innerChanged
	})
}

func envSplitCommandStringSpan(command string) (int, int, bool) {
	spans := splitCredentialShellWordSpans(command)
	words := shellWordValues(spans)
	index := credentialCommandIndex(words, "env")
	if index < 0 {
		return 0, 0, false
	}
	for optionIndex := index + 1; optionIndex < len(words); {
		option := strings.Trim(words[optionIndex], `"'`)
		if shellAssignmentWord.MatchString(option) {
			optionIndex++
			continue
		}
		if option == "--split-string" {
			if optionIndex+1 >= len(spans) {
				return 0, 0, false
			}
			span := spans[optionIndex+1]
			return span.contentStart, span.contentEnd, true
		}
		if strings.HasPrefix(option, "--split-string=") {
			return attachedEnvSplitStringSpan(command, spans[optionIndex], "--split-string=")
		}
		if splitOption, _, attached := envSplitStringOption(option); splitOption != "" {
			if attached {
				return attachedEnvSplitStringSpan(command, spans[optionIndex], "S")
			}
			if optionIndex+1 >= len(spans) {
				return 0, 0, false
			}
			span := spans[optionIndex+1]
			return span.contentStart, span.contentEnd, true
		}
		if len(option) < 2 || option[0] != '-' || option == "-" {
			break
		}
		_, _, requiresArgument, allowed := envOptions.option(option)
		if !allowed {
			break
		}
		optionIndex++
		if requiresArgument {
			optionIndex++
		}
	}
	return 0, 0, false
}

func attachedEnvSplitStringSpan(command string, span credentialShellWord, marker string) (int, int, bool) {
	raw := command[span.start:span.end]
	markerIndex := strings.Index(raw, marker)
	if markerIndex < 0 {
		return 0, 0, false
	}
	payloadStart := markerIndex + len(marker)
	if payloadStart >= len(raw) {
		return 0, 0, false
	}
	contentStart, contentEnd := credentialShellWordContentSpan(raw[payloadStart:])
	return span.start + payloadStart + contentStart, span.start + payloadStart + contentEnd, true
}

func decodeEnvSplitStringForRedaction(value string) string {
	var decoded strings.Builder
	decoded.Grow(len(value))
	for index := 0; index < len(value); index++ {
		if value[index] != '\\' || index+1 >= len(value) {
			decoded.WriteByte(value[index])
			continue
		}
		index++
		switch value[index] {
		case '_':
			decoded.WriteByte(' ')
		case 'n':
			decoded.WriteByte('\n')
		case 't':
			decoded.WriteByte('\t')
		default:
			decoded.WriteByte(value[index])
		}
	}
	return decoded.String()
}

func redactShellCommandStrings(value string, depth int) (string, bool) {
	return redactShellCommands(value, func(command string) (string, bool) {
		contentStart, contentEnd, ok := shellCommandStringSpan(command)
		if !ok {
			return command, false
		}
		redacted, changed := redactSensitiveTextDepth(command[contentStart:contentEnd], depth+1)
		return command[:contentStart] + redacted + command[contentEnd:], changed
	})
}

// eval is a shell builtin whose arguments are joined and executed as a new
// command. Unlike sh -c, its command text is often quoted, so the ordinary
// quote-aware substitution pass intentionally leaves it untouched. Redact
// each eval argument recursively while preserving its quoting and spacing.
func redactEvalCommandStrings(value string, depth int) (string, bool) {
	return redactShellCommands(value, func(command string) (string, bool) {
		spans := splitCredentialShellWordSpans(command)
		words := shellWordValues(spans)
		evalIndex := credentialCommandIndex(words, "eval")
		if evalIndex < 0 {
			return command, false
		}
		var replacements []textReplacement
		for _, span := range spans[evalIndex+1:] {
			inner := command[span.contentStart:span.contentEnd]
			if redacted, changed := redactSensitiveTextDepth(inner, depth+1); changed && redacted != inner {
				replacements = append(replacements, textReplacement{span.contentStart, span.contentEnd, redacted})
			}
		}
		return applyTextReplacements(command, replacements), len(replacements) > 0
	})
}

// launchctl submit -p supplies the executable separately from the argv passed
// after --. The first word after -- is argv[0], not an argument to the
// executable. Command-specific redactors therefore cannot see the real
// command boundary in the original text (for example, docker's login flags
// appear after a second `docker` word). Redact a synthetic command containing
// the executable and its arguments, then splice only the argument suffix back
// into the original text so formatting and argv[0] remain unchanged.
func redactLaunchctlSubmitEmbeddedCommands(value string, depth int) (string, bool) {
	return redactShellCommands(value, func(command string) (string, bool) {
		return redactLaunchctlSubmitEmbeddedCommand(command, depth)
	})
}

func redactLaunchctlSubmitEmbeddedCommand(command string, depth int) (string, bool) {
	spans := splitCredentialShellWordSpans(command)
	words := shellWordValues(spans)

	for launchctlIndex, word := range words {
		if commandBaseName(word) != "launchctl" || !isCredentialCommandPrefix(words[:launchctlIndex]) {
			continue
		}
		if launchctlIndex+1 >= len(words) || strings.Trim(words[launchctlIndex+1], `"'`) != "submit" {
			continue
		}

		executableIndex := -1
		separatorIndex := -1
		for index := launchctlIndex + 2; index < len(words); index++ {
			option := strings.Trim(words[index], `"'`)
			switch option {
			case "-l", "-o", "-e":
				if index+1 >= len(words) {
					return command, false
				}
				index++
			case "-p":
				if index+1 >= len(words) {
					return command, false
				}
				executableIndex = index + 1
				index++
			case "--":
				separatorIndex = index
				index = len(words)
			default:
				return command, false
			}
		}
		if executableIndex < 0 || separatorIndex < 0 || separatorIndex+1 >= len(spans) {
			continue
		}

		argv0Index := separatorIndex + 1
		payloadStart := spans[argv0Index].end
		if payloadStart >= len(command) || strings.TrimSpace(command[payloadStart:]) == "" {
			continue
		}
		rawExecutable := command[spans[executableIndex].start:spans[executableIndex].end]
		effectivePrefix := rawExecutable + " "
		effective := effectivePrefix + command[payloadStart:]
		// OpenSSL supports argv[0]-selected commands. When launchctl's argv[0]
		// is itself a known OpenSSL subcommand, retain it in the synthetic
		// command; arbitrary argv[0] values are skipped as normal process names.
		if commandBaseName(words[executableIndex]) == "openssl" || commandBaseName(words[executableIndex]) == "openssl.exe" {
			if isOpenSSLSubcommandName(words[argv0Index]) {
				rawArgv0 := command[spans[argv0Index].start:spans[argv0Index].end]
				effectivePrefix += rawArgv0 + " "
				effective = effectivePrefix + command[payloadStart:]
			}
		}
		redactedEffective, effectiveChanged := redactSensitiveTextDepth(effective, depth+1)
		if !effectiveChanged {
			continue
		}
		if redactedEffective == redactionMarker {
			return command[:payloadStart] + redactionMarker, true
		}
		if len(redactedEffective) < len(effectivePrefix) {
			continue
		}
		redactedSuffix := redactedEffective[len(effectivePrefix):]
		return command[:payloadStart] + redactedSuffix, true
	}
	return command, false
}

func shellCommandStringSpan(command string) (int, int, bool) {
	spans := splitCredentialShellWordSpans(command)
	words := shellWordValues(spans)
	for index, word := range words {
		if !isCredentialShell(commandBaseName(word)) || !isCredentialCommandPrefix(words[:index]) {
			continue
		}
		commandIndex, ok := shellCommandStringWordIndex(words[index+1:])
		if !ok {
			return 0, 0, false
		}
		span := spans[index+1+commandIndex]
		return span.contentStart, span.contentEnd, true
	}
	return 0, 0, false
}

func isCredentialShell(name string) bool {
	name = strings.TrimSuffix(name, ".exe")
	switch name {
	case "ash", "bash", "dash", "ksh", "mksh", "sh", "zsh":
		return true
	default:
		return false
	}
}

func shellCommandStringWordIndex(words []string) (int, bool) {
	for index := 0; index < len(words); index++ {
		raw := words[index]
		option := strings.Trim(raw, `"'`)
		if option == "--" || len(option) < 2 || option[0] != '-' || option == "-" {
			return 0, false
		}
		if strings.HasPrefix(option, "--") {
			_, _, requiresArgument, allowed := shellOptions.option(option)
			if !allowed {
				return 0, false
			}
			if requiresArgument {
				index++
				if index >= len(words) {
					return 0, false
				}
			}
			continue
		}
		if option == "-o" || option == "-O" {
			index++
			if index >= len(words) {
				return 0, false
			}
			continue
		}
		hasCommandString := false
		for _, flag := range option[1:] {
			if !strings.ContainsRune("abefhiklmnprstuvxBCDEHPTc", flag) {
				return 0, false
			}
			if flag == 'c' {
				hasCommandString = true
			}
		}
		if hasCommandString {
			commandIndex := index + 1
			if commandIndex < len(words) && strings.Trim(words[commandIndex], `"'`) == "--" {
				commandIndex++
			}
			if commandIndex >= len(words) {
				return 0, false
			}
			return commandIndex, true
		}
	}
	return 0, false
}

func findShellCommandEnd(value string, start int) int {
	var quote byte
	var quoting bool
	parenDepth := 0
	for i := start; i < len(value); i++ {
		if i, quote, quoting = readShellQuoting(value, i, quote); quoting {
			continue
		}
		switch value[i] {
		case '(':
			parenDepth++
		case ')':
			if parenDepth > 0 {
				parenDepth--
			}
		case '\r':
			if parenDepth == 0 && i+1 < len(value) && value[i+1] == '\n' {
				return i
			}
		case '\n', ';', '&', '|':
			if parenDepth == 0 {
				return i
			}
		}
	}
	return len(value)
}

func redactSensitiveCommandSubstitutions(value string) (string, bool) {
	return redactDelimitedValues(value, sensitiveCommandSubstitutionStart, func(value string, open int) int {
		return exclusiveEnd(findShellCommandSubstitutionEnd(value, open))
	})
}

func findShellCommandSubstitutionEnd(value string, open int) int {
	if open < 0 || open >= len(value) {
		return -1
	}
	if value[open] == '`' {
		for index := open + 1; index < len(value); index++ {
			if value[index] == '\\' {
				index++
				continue
			}
			if value[index] == '`' {
				return index
			}
		}
		return -1
	}
	contentStart := open + 1
	if value[open] == '$' && open+1 < len(value) && value[open+1] == '(' {
		contentStart++
	} else if value[open] != '(' {
		return -1
	}

	depth := 1
	resumeQuotes := []byte{0}
	var quote byte
	for i := contentStart; i < len(value); i++ {
		if quote == '\'' {
			if value[i] == '\'' {
				quote = 0
			}
			continue
		}
		if value[i] == '\\' {
			i++
			continue
		}
		if value[i] == '$' && i+1 < len(value) && value[i+1] == '(' {
			resumeQuotes = append(resumeQuotes, quote)
			quote = 0
			depth++
			i++
			continue
		}
		if quote != 0 {
			if value[i] == quote {
				quote = 0
			}
			continue
		}

		switch value[i] {
		case '\'', '"', '`':
			quote = value[i]
		case '(':
			resumeQuotes = append(resumeQuotes, 0)
			depth++
		case ')':
			depth--
			if depth == 0 {
				return i
			}
			quote = resumeQuotes[len(resumeQuotes)-1]
			resumeQuotes = resumeQuotes[:len(resumeQuotes)-1]
		}
	}
	return -1
}
