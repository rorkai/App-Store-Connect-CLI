package snitch

import (
	"slices"
	"strings"
)

// openSSLSecretOption redacts the values of an option such as -macopt that
// carries a secret only for some keys.
type openSSLSecretOption struct {
	pattern  *lazyRegexp
	isSecret func(string) bool
}

func redactPowerShellHereStringCredentials(value string) (string, bool) {
	return redactDelimitedValues(value, powerShellHereStringCredential, func(value string, open int) int {
		contentStart := open + len(`@"`) + 1
		if value[contentStart-1] == '\r' {
			contentStart++
		}
		return findPowerShellHereStringEnd(value, contentStart, value[open+1])
	})
}

func findPowerShellHereStringEnd(value string, contentStart int, quote byte) int {
	for lineStart := contentStart; lineStart < len(value); {
		marker := lineStart
		for marker < len(value) && (value[marker] == ' ' || value[marker] == '\t') {
			marker++
		}
		if marker+1 < len(value) && value[marker] == quote && value[marker+1] == '@' &&
			(marker+2 == len(value) || strings.ContainsRune(" \t\r\n;&|)", rune(value[marker+2]))) {
			return marker + 2
		}
		lineBreak := strings.IndexByte(value[lineStart:], '\n')
		if lineBreak < 0 {
			break
		}
		lineStart += lineBreak + 1
	}
	return -1
}

func redactPowerShellCollectionCredentials(value string) (string, bool) {
	return redactDelimitedValues(value, powerShellCollectionCredential, findPowerShellCollectionEnd)
}

func findPowerShellCollectionEnd(value string, open int) int {
	if open < 0 || open+1 >= len(value) || value[open] != '@' ||
		(value[open+1] != '(' && value[open+1] != '{') {
		return -1
	}

	stack := []byte{value[open+1]}
	var quote byte
	for index := open + 2; index < len(value); index++ {
		if quote != 0 {
			if quote == '"' && value[index] == '`' {
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
			quote = 0
			continue
		}
		if value[index] == '`' {
			index++
			continue
		}

		switch value[index] {
		case '\'', '"':
			quote = value[index]
		case '(', '{', '[':
			stack = append(stack, value[index])
		case ')', '}', ']':
			if !powerShellDelimitersMatch(stack[len(stack)-1], value[index]) {
				continue
			}
			stack = stack[:len(stack)-1]
			if len(stack) == 0 {
				return index + 1
			}
		}
	}
	return -1
}

func powerShellDelimitersMatch(open, close byte) bool {
	return open == '(' && close == ')' || open == '{' && close == '}' || open == '[' && close == ']'
}

func redactCommandPromptSetAssignments(value string) (string, bool) {
	redacted, changed := redactCommandPromptSetAssignmentValues(value, commandPromptQuotedSetAssignment, findCommandPromptQuotedSetValueEnd)
	if next, unquotedChanged := redactCommandPromptSetAssignmentValues(redacted, commandPromptUnquotedSetAssignment, findCommandPromptUnquotedSetValueEnd); unquotedChanged {
		redacted = next
		changed = true
	}
	return redacted, changed
}

func redactCommandPromptSetAssignmentValues(value string, pattern *lazyRegexp, findValueEnd func(string, int) int) (string, bool) {
	redacted := value
	changed := false
	for searchStart := 0; searchStart < len(redacted); {
		match := pattern.FindStringIndex(redacted[searchStart:])
		if match == nil {
			break
		}

		valueStart := searchStart + match[1]
		valueEnd := findValueEnd(redacted, valueStart)
		currentValue := redacted[valueStart:valueEnd]
		if currentValue == "" || currentValue == redactionMarker || currentValue == privateKeyRedactionMarker {
			searchStart = valueEnd + 1
			continue
		}
		redacted = redacted[:valueStart] + redactionMarker + redacted[valueEnd:]
		changed = true
		searchStart = valueStart + len(redactionMarker)
	}
	return redacted, changed
}

func findCommandPromptQuotedSetValueEnd(value string, start int) int {
	for index := start; index < len(value); index++ {
		switch value[index] {
		case '^':
			if index+2 < len(value) && value[index+1] == '\r' && value[index+2] == '\n' {
				index += 2
			} else if index+1 < len(value) {
				index++
			}
		case '"', '\r', '\n':
			return index
		}
	}
	return len(value)
}

func findCommandPromptUnquotedSetValueEnd(value string, start int) int {
	inQuotes := false
	for index := start; index < len(value); index++ {
		switch value[index] {
		case '^':
			if index+2 < len(value) && value[index+1] == '\r' && value[index+2] == '\n' {
				index += 2
			} else if index+1 < len(value) {
				index++
			}
		case '"':
			inQuotes = !inQuotes
		case '&', '|', '<', '>', '(', ')', '\r', '\n':
			if !inQuotes {
				for index > start && (value[index-1] == ' ' || value[index-1] == '\t') {
					index--
				}
				return index
			}
		}
	}
	return len(value)
}

func redactCurlConfigCertificatePasswords(value string) (string, bool) {
	redacted := value
	changed := false
	for searchStart := 0; searchStart < len(redacted); {
		match := curlConfigCertificateEntry.FindStringSubmatchIndex(redacted[searchStart:])
		if match == nil {
			break
		}

		valueStart := searchStart + match[1]
		lineEnd := valueStart + strings.IndexAny(redacted[valueStart:], "\r\n")
		if lineEnd < valueStart {
			lineEnd = len(redacted)
		}
		contentStart, contentEnd := curlConfigValueBounds(redacted, valueStart, lineEnd)
		separator := curlCertificatePasswordSeparator(redacted[contentStart:contentEnd])
		if separator < 0 {
			searchStart = lineEnd + 1
			continue
		}

		passwordStart := contentStart + separator + 1
		if passwordStart == contentEnd || redacted[passwordStart:contentEnd] == redactionMarker {
			searchStart = lineEnd + 1
			continue
		}
		redacted = redacted[:passwordStart] + redactionMarker + redacted[contentEnd:]
		changed = true
		searchStart = passwordStart + len(redactionMarker)
	}
	return redacted, changed
}

func curlConfigValueBounds(value string, start, lineEnd int) (int, int) {
	if start >= lineEnd || (value[start] != '"' && value[start] != '\'') {
		end := start
		for end < lineEnd && value[end] != ' ' && value[end] != '\t' {
			end++
		}
		return start, end
	}

	quote := value[start]
	for index := start + 1; index < lineEnd; index++ {
		if value[index] == '\\' && index+1 < lineEnd {
			index++
			continue
		}
		if value[index] == quote {
			return start + 1, index
		}
	}
	return start + 1, lineEnd
}

func curlCertificatePasswordSeparator(value string) int {
	searchStart := 0
	if len(value) >= 3 && isASCIIAlpha(value[0]) && value[1] == ':' && (value[2] == '\\' || value[2] == '/') {
		searchStart = 3
	}
	for index := searchStart; index < len(value); index++ {
		if value[index] == '\\' && index+1 < len(value) {
			index++
			continue
		}
		if value[index] == ':' {
			return index
		}
	}
	return -1
}

func isASCIIAlpha(character byte) bool {
	return character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z'
}

func redactNetrcPasswords(value string) (string, bool) {
	redacted := value
	entries := netrcEntryStart.FindAllStringIndex(redacted, -1)
	changed := false
	for index := len(entries) - 1; index >= 0; index-- {
		start := entries[index][0]
		end := len(redacted)
		if index+1 < len(entries) {
			end = entries[index+1][0]
		}
		entry := redacted[start:end]
		next := netrcPasswordValue.ReplaceAllString(entry, `${1}${2}`+redactionMarker+`${3}`)
		if next == entry {
			continue
		}
		redacted = redacted[:start] + next + redacted[end:]
		changed = true
	}
	return redacted, changed
}

func redactCompoundCurlHeaderWords(value string) (string, bool) {
	redacted := value
	changed := false
	for searchStart := 0; searchStart < len(redacted); {
		option := curlHeaderOptionStart.FindStringSubmatchIndex(redacted[searchStart:])
		if option == nil {
			break
		}

		valueStart := searchStart + option[1]
		wordMatch := completeShellWord.FindStringSubmatchIndex(redacted[valueStart:])
		if wordMatch == nil {
			searchStart = valueStart
			continue
		}
		wordEnd := valueStart + wordMatch[3]
		word := redacted[valueStart:wordEnd]
		headerName, valid := decodeShellHeaderName(word)
		assignmentHeader := valid && sensitiveAssignmentHeaderName.MatchString(headerName)
		if !valid || (!credentialHeaderNamePattern.MatchString(headerName) && !assignmentHeader) {
			searchStart = wordEnd
			continue
		}
		if !assignmentHeader && !isCompoundQuotedShellWord(word) {
			searchStart = wordEnd
			continue
		}

		replacement := `"` + headerName + `: ` + redactionMarker + `"`
		if word == replacement {
			searchStart = wordEnd
			continue
		}
		redacted = redacted[:valueStart] + replacement + redacted[wordEnd:]
		changed = true
		searchStart = valueStart + len(replacement)
	}
	return redacted, changed
}

func isCompoundQuotedShellWord(word string) bool {
	if word == "" || !strings.ContainsAny(word, `"'`) {
		return false
	}
	if word[0] != '\'' && word[0] != '"' {
		return true
	}

	quote := word[0]
	for index := 1; index < len(word); index++ {
		if quote == '"' && (word[index] == '\\' || word[index] == '`') {
			index++
			continue
		}
		if word[index] == quote {
			return index != len(word)-1
		}
	}
	return false
}

func decodeShellHeaderName(word string) (string, bool) {
	var decoded strings.Builder
	var quote byte
	for index := 0; index < len(word); index++ {
		character := word[index]
		if quote != 0 {
			if character == quote {
				quote = 0
				continue
			}
			if quote != '"' || (character != '\\' && character != '`') {
				if character == ':' {
					return decoded.String(), decoded.Len() > 0
				}
				decoded.WriteByte(character)
				continue
			}
		} else {
			switch character {
			case '\'', '"':
				quote = character
				continue
			case ':':
				return decoded.String(), decoded.Len() > 0
			case '$', '(', ')':
				return "", false
			case '\\', '`', '^':
			default:
				decoded.WriteByte(character)
				continue
			}
		}

		index++
		if index >= len(word) {
			return "", false
		}
		if word[index] == '\r' && index+1 < len(word) && word[index+1] == '\n' {
			index++
			continue
		}
		if word[index] != '\n' {
			decoded.WriteByte(word[index])
		}
	}
	return "", false
}

func redactKubectlSecretLiterals(value string) (string, bool) {
	return redactShellCommands(value, func(command string) (string, bool) {
		if !isKubectlCreateSecretCommand(command) {
			return command, false
		}
		redacted := kubectlFromLiteralValue.ReplaceAllString(command, `${1}`+redactionMarker)
		return redacted, redacted != command
	})
}

func redactOpenSSLCredentialArguments(value string) (string, bool) {
	redacted, subcommandChanged := redactOpenSSLSubcommandCredentialArguments(value)
	redacted, positionalChanged := redactOpenSSLPasswdPositionalArguments(redacted)
	return redacted, subcommandChanged || positionalChanged
}

func redactOpenSSLSubcommandCredentialArguments(value string) (string, bool) {
	return redactShellCommands(value, func(command string) (string, bool) {
		commandStart, subcommand, ok := openSSLCommand(command)
		commandSuffix := command[commandStart:]
		commandChanged := false
		if pattern := opensslSubcommandCredentialPatterns[subcommand]; ok && pattern != nil {
			commandSuffix, commandChanged = redactCommandCredentialFlagValuesMatching(commandSuffix, pattern, nil)
		}
		if ok {
			options := append(slices.Clone(opensslCredentialOptions), opensslSubcommandCredentialOptions[subcommand]...)
			var optionChanged bool
			commandSuffix, optionChanged = redactCommandCredentialOptionValues(commandSuffix, subcommand, options...)
			commandChanged = commandChanged || optionChanged
		}
		if matches, found := opensslSecretOptionValues[subcommand]; found {
			var optionChanged bool
			commandSuffix, optionChanged = redactCommandCredentialFlagValuesMatching(commandSuffix, matches.pattern, matches.isSecret)
			commandChanged = commandChanged || optionChanged
		}
		return command[:commandStart] + commandSuffix, commandChanged
	})
}

func openSSLSubcommandOptionRequiresArgument(subcommand, option string) bool {
	if option == "" || option == "-" || !strings.HasPrefix(option, "-") || strings.Contains(option, "=") {
		return false
	}
	name := strings.TrimLeft(option, "-")
	if name == "" {
		return false
	}
	options := opensslSubcommandArgumentOptions[subcommand]
	return strings.Contains(" "+options+" ", " "+name+" ")
}

// openSSLOptionValueWithPrefix accepts an option value such as key:SECRET whose
// first word starts with one of prefixes.
func openSSLOptionValueWithPrefix(prefixes ...string) func(string) bool {
	return func(value string) bool {
		spans := splitCredentialShellWordSpans(value)
		if len(spans) == 0 {
			return false
		}
		option := strings.ToLower(strings.TrimPrefix(strings.Trim(spans[0].value, `"'`), "$"))
		return slices.ContainsFunc(prefixes, func(prefix string) bool { return strings.HasPrefix(option, prefix) })
	}
}

func openSSLCommand(command string) (int, string, bool) {
	spans := splitCredentialShellWordSpans(command)
	words := shellWordValues(spans)
	index := credentialCommandIndex(words, "openssl", "openssl.exe")
	if index < 0 {
		return 0, "", false
	}
	if index+1 >= len(words) {
		return 0, "", false
	}
	subcommandStart := index + 1
	if index+3 < len(words) && strings.Trim(words[index+1], `"'`) == "--" && isLaunchctlSubmitExecutablePrefix(words[:index]) {
		subcommandStart = index + 3
	}
	for subcommandIndex := subcommandStart; subcommandIndex < len(words); subcommandIndex++ {
		option := strings.ToLower(strings.Trim(words[subcommandIndex], `"'`))
		if option == "--" {
			continue
		}
		if !strings.HasPrefix(option, "-") || option == "-" {
			return spans[subcommandIndex].start, option, true
		}
		requiresArgument, allowed := openSSLGlobalOption(option)
		if !allowed {
			return 0, "", false
		}
		if requiresArgument {
			subcommandIndex++
			if subcommandIndex >= len(words) {
				return 0, "", false
			}
		}
	}
	return 0, "", false
}

func isLaunchctlSubmitExecutablePrefix(words []string) bool {
	if len(words) == 0 || strings.Trim(words[len(words)-1], `"'`) != "-p" {
		return false
	}
	for index := 0; index+1 < len(words); index++ {
		if commandBaseName(words[index]) == "launchctl" && strings.Trim(words[index+1], `"'`) == "submit" && isCredentialCommandPrefix(words[:index]) {
			return true
		}
	}
	return false
}

func openSSLGlobalOption(option string) (bool, bool) {
	name, _, attached := strings.Cut(option, "=")
	name = strings.TrimLeft(name, "-")
	switch name {
	case "config", "provider", "provider-path", "propquery":
		return !attached, true
	default:
		return false, false
	}
}

func redactOpenSSLPasswdPositionalArguments(value string) (string, bool) {
	return redactShellCommands(value, func(command string) (string, bool) {
		spans := splitCredentialShellWordSpans(command)
		words := shellWordValues(spans)
		passwordIndex, ok := openSSLPasswdPasswordWordIndex(words)
		if !ok {
			return command, false
		}
		span := spans[passwordIndex]
		return command[:span.start] + redactionMarker + command[credentialShellArgumentEnd(command, span):], true
	})
}

func openSSLPasswdPasswordWordIndex(words []string) (int, bool) {
	opensslIndex := credentialCommandIndex(words, "openssl", "openssl.exe")
	if opensslIndex < 0 {
		return 0, false
	}
	if opensslIndex+1 >= len(words) || strings.Trim(words[opensslIndex+1], `"'`) != "passwd" {
		return 0, false
	}
	for index := opensslIndex + 2; index < len(words); index++ {
		option := strings.Trim(words[index], `"'`)
		if option == "--" {
			if index+1 < len(words) && strings.Trim(words[index+1], `"'`) == redactionMarker {
				return 0, false
			}
			return index + 1, index+1 < len(words)
		}
		if !strings.HasPrefix(option, "-") || option == "-" {
			if strings.TrimRight(option, ")") == redactionMarker {
				return 0, false
			}
			return index, true
		}

		name, _, attached := strings.Cut(option, "=")
		switch name {
		case "-help":
			return 0, false
		case "-noverify", "-stdin", "-quiet", "-table", "-reverse", "-6", "-5", "-apr1", "-1", "-aixmd5":
			if attached {
				return 0, false
			}
		case "-in", "-salt", "-rand", "-writerand", "-provider-path", "-provider", "-provparam", "-propquery":
			if !attached {
				index++
				if index >= len(words) {
					return 0, false
				}
			}
		default:
			return 0, false
		}
	}
	return 0, false
}

// shortOptionSecrets describes a command whose short options can be grouped
// (-vp SECRET) and which stops reading options at its first operand.
type shortOptionSecrets struct {
	names    []string
	flags    string // options without a value
	values   string // options with a value, attached or in the next word
	optional string // options whose value can only be attached
	secrets  string // value options whose value is a secret
	// skipValueWords skips the words a value spans when it contains a
	// command substitution with spaces.
	skipValueWords bool
}

var (
	sshpassSecrets   = shortOptionSecrets{names: []string{"sshpass", "sshpass.exe"}, flags: "v", values: "fdPp", optional: "e", secrets: "p"}
	sshKeygenSecrets = shortOptionSecrets{names: []string{"ssh-keygen", "ssh-keygen.exe"}, flags: "ABHKLQUXceghiklopquvy", values: "CDEFIMNOPRVYZabfgmnrstwz", secrets: "NP", skipValueWords: true}
)

func redactShortOptionSecrets(spec shortOptionSecrets) func(string) (string, bool) {
	return func(value string) (string, bool) {
		return redactShellCommands(value, func(command string) (string, bool) {
			spans := splitCredentialShellWordSpans(command)
			words := shellWordValues(spans)
			commandIndex := credentialCommandIndex(words, spec.names...)
			if commandIndex < 0 {
				return command, false
			}
			var replacements []textReplacement
		options:
			for optionIndex := commandIndex + 1; optionIndex < len(words); optionIndex++ {
				option := strings.Trim(words[optionIndex], `"'`)
				if option == "--" || len(option) < 2 || option[0] != '-' || option[1] == '-' {
					break
				}
				for flagIndex := 1; flagIndex < len(option); flagIndex++ {
					flag := option[flagIndex]
					switch {
					case strings.IndexByte(spec.flags, flag) >= 0:
						continue
					case strings.IndexByte(spec.optional, flag) >= 0:
						continue options
					case strings.IndexByte(spec.values, flag) < 0:
						break options
					}
					attached := flagIndex+1 < len(option)
					valueIndex := optionIndex
					if !attached {
						if optionIndex+1 >= len(words) {
							break options
						}
						valueIndex = optionIndex + 1
					}
					valueEnd := credentialShellArgumentEnd(command, spans[valueIndex])
					if strings.IndexByte(spec.secrets, flag) >= 0 {
						if attached {
							valueStart := flagIndex + 1
							if option[valueStart] == '=' {
								valueStart++
							}
							if option[valueStart:] != redactionMarker {
								replacements = append(replacements, textReplacement{spans[optionIndex].start, valueEnd, option[:valueStart] + redactionMarker})
							}
						} else if strings.Trim(words[valueIndex], `"'`) != redactionMarker {
							replacements = append(replacements, textReplacement{spans[valueIndex].start, valueEnd, redactionMarker})
						}
					}
					optionIndex = valueIndex
					for spec.skipValueWords && optionIndex+1 < len(spans) && spans[optionIndex+1].start < valueEnd {
						optionIndex++
					}
					continue options
				}
			}
			return applyTextReplacements(command, replacements), len(replacements) > 0
		})
	}
}

func redactCurlCredentialArguments(value string) (string, bool) {
	return redactNamedCommand(value, "curl", func(command string) (string, bool) {
		redacted := command
		for _, rules := range [][]redactionRule{curlUserCredentialRedactionRules, curlCertificateCredentialRedactionRules, curlArgumentCredentialRedactionRules} {
			for _, rule := range rules {
				redacted = rule.pattern.ReplaceAllString(redacted, rule.replacement)
			}
		}
		redacted, _ = redactCompoundCurlHeaderWords(redacted)
		return redacted, redacted != command
	})
}

func redactRedisCLICredentialArguments(value string) (string, bool) {
	return redactNamedCommand(value, "redis-cli", func(command string) (string, bool) {
		return redactCommandCredentialOptionValues(command, "", "a", "pass")
	})
}

func redactDockerLoginCredentialArguments(value string) (string, bool) {
	return redactShellCommands(value, func(command string) (string, bool) {
		commandStart, ok := dockerLoginCommandStart(command)
		if !ok {
			return command, false
		}
		return redactCommandFrom(command, commandStart, func(command string) (string, bool) {
			return redactFlagArguments(command, "p", dockerLoginCredentialFlagPattern)
		})
	})
}

func dockerLoginCommandStart(command string) (int, bool) {
	spans := splitCredentialShellWordSpans(command)
	words := shellWordValues(spans)
	index := credentialCommandIndex(words, "docker", "docker.exe")
	if index < 0 {
		return 0, false
	}
	subcommand, ok := dockerSubcommand(words[index+1:])
	return spans[index].start, ok && subcommand == "login"
}

func dockerSubcommand(words []string) (string, bool) {
	words, ok := dockerOptions.consumeOptions(words)
	if !ok || len(words) == 0 {
		return "", false
	}
	return strings.Trim(words[0], `"'`), true
}

func redactSecurityCredentialArguments(value string) (string, bool) {
	return redactShellCommands(value, func(command string) (string, bool) {
		commandStart, subcommand, ok := securityCommand(command)
		pattern := securityCredentialFlagPatterns[subcommand]
		if !ok || pattern == nil {
			return command, false
		}
		return redactCommandFrom(command, commandStart, func(command string) (string, bool) {
			return redactFlagArguments(command, securityAttachedCredentialFlags[subcommand], pattern)
		})
	})
}

func securityCommand(command string) (int, string, bool) {
	spans := splitCredentialShellWordSpans(command)
	words := shellWordValues(spans)
	for index, word := range words {
		word = strings.Trim(word, `"'`)
		if separator := strings.LastIndexAny(word, `/\\`); separator >= 0 {
			word = word[separator+1:]
		}
		if word != "security" || !isCredentialCommandPrefix(words[:index]) {
			continue
		}
		for _, candidate := range words[index+1:] {
			candidate = strings.Trim(candidate, `"'`)
			if candidate == "--" || strings.HasPrefix(candidate, "-") {
				continue
			}
			_, known := securityCredentialFlagPatterns[candidate]
			return spans[index].start, candidate, known
		}
		return 0, "", false
	}
	return 0, "", false
}

func isOpenSSLSubcommandName(value string) bool {
	switch strings.ToLower(strings.Trim(value, `"'`)) {
	case "asn1parse", "ca", "ciphers", "cmp", "cms", "configutl", "crl", "crl2pkcs7", "dgst", "dhparam", "dsa", "dsaparam", "ec", "ecparam", "enc", "engine", "errstr", "fipsinstall", "gendsa", "genpkey", "genrsa", "help", "info", "kdf", "list", "mac", "nseq", "ocsp", "passwd", "pkcs12", "pkcs7", "pkcs8", "pkey", "pkeyparam", "pkeyutl", "prime", "provider", "rand", "rehash", "req", "rsa", "rsautl", "s_client", "s_server", "s_time", "sess_id", "skeyutl", "smime", "speed", "spkac", "srp", "storeutl", "ts", "verify", "version", "x509":
		return true
	default:
		return false
	}
}

func isKubectlCreateSecretCommand(command string) bool {
	words := credentialCommandWords(command)
	if len(words) < 3 {
		return false
	}

	kubectlIndex := -1
	for index, word := range words {
		word = strings.Trim(word, `"'`)
		if separator := strings.LastIndexAny(word, `/\\`); separator >= 0 {
			word = word[separator+1:]
		}
		if strings.EqualFold(word, "kubectl") {
			kubectlIndex = index
			break
		}
	}
	if kubectlIndex < 0 || !isCredentialCommandPrefix(words[:kubectlIndex]) {
		return false
	}

	return isKubectlCreateSecretSubcommand(words[kubectlIndex+1:])
}

func isKubectlCreateSecretSubcommand(words []string) bool {
	words, ok := kubectlOptions.consumeOptions(words)
	return ok && len(words) >= 2 &&
		strings.EqualFold(strings.Trim(words[0], `"'`), "create") &&
		strings.EqualFold(strings.Trim(words[1], `"'`), "secret")
}
