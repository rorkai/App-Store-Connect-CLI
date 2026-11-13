package snitch

import (
	"net/url"
	"regexp"
	"strings"
	"sync"
)

const (
	redactionMarker           = "[REDACTED]"
	privateKeyRedactionMarker = "[REDACTED PRIVATE KEY]"
	oversizedFieldMarker      = "[REDACTED: oversized report field omitted]"
	redactionNotice           = "Note: sensitive values were redacted from the snitch report."
	maxRedactionFieldBytes    = 64 * 1024
	maxShellRedactionDepth    = 32

	sensitiveAssignmentName     = `(?:_auth|account[_-]?key|api[_-]?key|access[_-]?token|auth[_-]?token|refresh[_-]?token|session[_-]?token|shared[_-]?access[_-]?key|client[_-]?secret|client[_-]?key[_-]?data|app[_-]?secret|webhook[_-]?secret|webhook|signing[_-]?secret|secret[_-]?access[_-]?key|secret[_-]?answer|secret[_-]?key(?:[_-]?base)?|security[_-]?key[_-]?pin|asc[_-]?private[_-]?key(?:[_-]?b64)?|private[_-]?key(?:[_-]?b64)?|password|passphrase|passwd|pwd|secret|token)`
	delimitedPassName           = `_*(?:(?:[a-z0-9]+[_-])+)?pass`
	sensitivePrefixedName       = `(?:_*(?:[a-z0-9]+[_-])*[a-z0-9]*` + sensitiveAssignmentName + `|` + delimitedPassName + `)`
	tomlQuotedSensitiveKey      = `(?:"` + sensitivePrefixedName + `"|'` + sensitivePrefixedName + `')`
	valueBearingFlagPrefix      = `(?:n(?:[a-np-z0-9][a-z0-9]*|o[a-z0-9]+)?|[a-mo-z0-9][a-z0-9]*)`
	sensitiveFlagName           = `(?:(?:` + valueBearingFlagPrefix + `[_-])+` + sensitiveAssignmentName + `|(?:` + valueBearingFlagPrefix + `[_-])+pass|oauth2-bearer|access[_-]?token|auth[_-]?token|refresh[_-]?token|session[_-]?token|client[_-]?secret|app[_-]?secret|webhook[_-]?secret|webhook[_-]?header|slack[_-]?webhook|webhook|signing[_-]?secret|secret[_-]?access[_-]?key|secret[_-]?answer|secret[_-]?key(?:[_-]?base)?|demo[_-]?account[_-]?password|two[_-]?factor[_-]?code|proxy-tlspassword|tlspassword|password|passphrase|passwd|pwd|pass|token)`
	sensitiveOrSecretFlagName   = `(?:` + sensitiveFlagName + `|secret)`
	powerShellVariableScope     = `(?:(?:env|global|local|private|script|using):)`
	powerShellSensitiveVariable = `(?:\$(?:(?:` + powerShellVariableScope + `)?` + sensitivePrefixedName + `\b|\{(?:` + powerShellVariableScope + `)?` + sensitivePrefixedName + `\}))`
	sensitiveShellFlagToken     = `(?:-{1,2}` + sensitiveFlagName + `\b|"-{1,2}` + sensitiveFlagName + `\b"|'-{1,2}` + sensitiveFlagName + `\b'|-{1,2}"` + sensitiveFlagName + `\b"|-{1,2}'` + sensitiveFlagName + `\b')`
	sensitiveOrSecretShellToken = `(?:-{1,2}` + sensitiveOrSecretFlagName + `\b|"-{1,2}` + sensitiveOrSecretFlagName + `\b"|'-{1,2}` + sensitiveOrSecretFlagName + `\b'|-{1,2}"` + sensitiveOrSecretFlagName + `\b"|-{1,2}'` + sensitiveOrSecretFlagName + `\b')`
	credentialHeaderName        = `(?:proxy-authorization|authorization|cookie|set-cookie|scnt|x-apple-id-session-id|x-apple-widget-key|csrf|csrf_ts)`
	cgiCredentialHeaderName     = `(?:proxy_authorization|authorization|cookie|set_cookie|scnt|x_apple_id_session_id|x_apple_widget_key|csrf|csrf_ts)`
	traceCredentialHeader       = `(?:cookie|set-cookie|scnt|x-apple-id-session-id|x-apple-widget-key|csrf|csrf_ts)`
	webAuthQueryCredential      = `(?:widgetkey|scnt)`
	queryCredentialName         = `(?:x-amz-(?:credential|security-token|signature)|x-goog-(?:credential|signature)|signature|sig|authorization|auth|` + webAuthQueryCredential + `|` + sensitiveAssignmentName + `)`
	webAuthStructuredCredential = `(?:authservicekey|servicekey)`
	wellKnownSecretDataName     = `(?:tls\.key|\.dockerconfigjson)`
	structuredCredentialName    = `(?:` + sensitivePrefixedName + `|` + credentialHeaderName + `|` + webAuthStructuredCredential + `|` + wellKnownSecretDataName + `)`
	yamlCredentialName          = `(?:` + sensitivePrefixedName + `|` + wellKnownSecretDataName + `)`
	yamlNodeTag                 = `(?:!<[^>\r\n]+>|![^\s#]+)`
	yamlNodeAnchor              = `(?:&[a-zA-Z0-9_-]+)`
	yamlMappingKeyProperties    = `(?:(?:` + yamlNodeTag + `|` + yamlNodeAnchor + `)[ \t]+)*`
	powerShellEscapedCharacter  = `\x60(?:\r?\n|[^\r\n])`
	singleLineQuotedValue       = `(?:"(?:\\.|` + powerShellEscapedCharacter + `|[^"\\\x60\r\n])*"|\$?'(?:\\.|[^'\\\r\n])*')`
	shellCommandSubstitution    = `(?:\x60(?:\\.|[^\x60\\\r\n])*\x60|\$\((?:\\.|[^)\\\r\n])*\))`
	fishCommandSubstitution     = `\((?:\\.|[^)\\\r\n])*\)`
	cmdEscapedCharacter         = `\^(?:\r?\n|[^\r\n])`
	singleLineUnquotedFragment  = `(?:\\[^\r\n]|[^\s\\;&|<>()"'\x60^])+`
	singleLineShellWord         = `(?:` + singleLineQuotedValue + `|` + shellCommandSubstitution + `|` + powerShellEscapedCharacter + `|` + cmdEscapedCharacter + `|` + singleLineUnquotedFragment + `)+`
	kubectlShellWord            = `(?:` + singleLineQuotedValue + `|` + shellCommandSubstitution + `|` + powerShellEscapedCharacter + `|` + cmdEscapedCharacter + `|\\\r?\n[ \t]*|` + singleLineUnquotedFragment + `)+`
	fishShellWord               = `(?:` + singleLineQuotedValue + `|` + shellCommandSubstitution + `|` + fishCommandSubstitution + `|` + powerShellEscapedCharacter + `|` + cmdEscapedCharacter + `|` + singleLineUnquotedFragment + `)+`
	singleLineShellTerminator   = `(?:[ \t;&|<>()]|\r?\n|\z)`
	escapedQuotedCharacter      = `\\(?:\r?\n|[^\r\n])`
	escapeAwareQuotedValue      = `(?:"(?:` + escapedQuotedCharacter + `|` + powerShellEscapedCharacter + `|[^"\\\x60])*"|\$?'(?:''|` + escapedQuotedCharacter + `|[^'\\])*')`
	unterminatedQuotedValue     = `(?s:".*|\$?'.*)`
	shellUnquotedValue          = `(?:\\(?:\r?\n|[^\r\n])|[^\s;&|<>()"'])+`
	commandShortSubstitution    = `(?:\x60(?:\\.|[^\x60\\\r\n])*\x60|\$\((?:\\.|[^()\\\r\n]|\((?:\\.|[^()\\\r\n])*\))*\))`
	commandShortUnquotedValue   = `(?:\\[^\r\n]|[^\s\\;&|<>()"'\x60^$])+`
	commandShortShellWord       = `(?:` + singleLineQuotedValue + `|` + commandShortSubstitution + `|` + powerShellEscapedCharacter + `|` + cmdEscapedCharacter + `|` + commandShortUnquotedValue + `|\$)+`
	structuredUnquotedValue     = `(?:\\(?:\r?\n|[^\r\n])|[^\s,;&|<>()"'{}\[\]])+`
	flagUnquotedValue           = `(?:\\[^\r\n]|-[^-\s\\;&|<>()]|[^-\s\\;&|<>()])(?:\\[^\r\n]|[^\s;&|<>()])*`
	commandFlagUnquotedValue    = `(?:\\[^\r\n]|-[^-\s\\;&|<>()"']|[^-\s\\;&|<>()"'])(?:\\[^\r\n]|[^\s;&|<>()"'])*`
	credentialPairQuoted        = `(?:"(?:` + escapedQuotedCharacter + `|[^"\\])*:(?:` + escapedQuotedCharacter + `|[^"\\])+"|\$?'(?:` + escapedQuotedCharacter + `|[^'\\])*:(?:` + escapedQuotedCharacter + `|[^'\\])+')`
	credentialPairOpen          = `(?:"[^\r\n]*:[^\r\n]+|\$?'[^\r\n]*:[^\r\n]+)`
	credentialPairShellWord     = `(?:` + singleLineQuotedValue + `|\\(?:\r?\n|[^\r\n])|[^\s:;&|<>()"'])+:(?:` + singleLineQuotedValue + `|` + shellCommandSubstitution + `|` + fishCommandSubstitution + `|` + shellUnquotedValue + `)+`
	credentialPairUnquoted      = `(?:\\(?:\r?\n|[^\r\n])|[^\s:;&|<>()])*:(?:\\(?:\r?\n|[^\r\n])|[^\s;&|<>()])+`
	credentialPairValue         = `(?:` + credentialPairQuoted + `|` + credentialPairShellWord + `|` + credentialPairOpen + `|` + credentialPairUnquoted + `)`
	cookieDataQuoted            = `(?:"(?:\\.|[^"\\\r\n])*=(?:\\.|[^"\\\r\n])*"|\$?'(?:\\.|[^'\\\r\n])*=(?:\\.|[^'\\\r\n])*')`
	cookieDataUnquoted          = `(?:\\(?:\r?\n|[^\r\n])|[^\s;&|<>()])+=(?:\\(?:\r?\n|[^\r\n])|[^\s;&|<>()])*`
	cookieDataValue             = `(?:` + cookieDataQuoted + `|` + cookieDataUnquoted + `)`
	curlOptionValueSeparator    = `(?:[ \t]+|[ \t]*=[ \t]*|[ \t]*(?:\\|\x60|\^)\r?\n[ \t]*)`
	curlCertOptionPrefix        = `(?:(?:(?-i:-E)|--(?:proxy-)?cert)\b` + curlOptionValueSeparator + `|(?-i:-E))`
	curlCertUnquotedPath        = `(?:\\(?:\r?\n|[^\r\n])|[^\s:'"])+`
	curlCertShellPath           = `(?:` + singleLineQuotedValue + `|` + curlCertUnquotedPath + `)*`
	curlHeaderOptionPrefix      = `(?:(?:-H|--header|--proxy-header)\b(?:[ \t]+|[ \t]*=[ \t]*)|-H)`
	curlFormDataOptionPrefix    = `(?:(?-i:-F)(?:[ \t]+|[ \t]*=[ \t]*)?|--(?:data-urlencode|data|form-string|form)\b(?:[ \t]+|[ \t]*=[ \t]*))`
	curlConfigSeparator         = `(?:[ \t]*[=:][ \t]*|[ \t]+)`
	curlConfigExplicitSeparator = `(?:[ \t]*[=:][ \t]*)`
	shellCommandPathSeparator   = `(?:[ \t]+(?:\\\r?\n[ \t]*)*|(?:\\\r?\n)+[ \t]+(?:\\\r?\n[ \t]*)*)`
	foldedHeaderContinuation    = `(?:\r?\n[ \t]+[^\r\n]*)*`
	passwordFileField           = `(?:\\[:\\]|[^:\\\r\n])+`
	passwordFileHostField       = `(?:\\[:\\]|[^#:\\\r\n])(?:\\[:\\]|[^:\\\r\n])*`
)

const (
	powerShellSecureStringSwitch    = `(?:(?:-AsPlainText|-Force)(?:[ \t]+|[ \t]*:[ \t]*\$?(?:true|false)[ \t]+))`
	uppercaseSessionEnvironmentName = `(?:[A-Z0-9]+_)+SESSION`
)

type redactionRule struct {
	pattern     *lazyRegexp
	replacement string
}

// lazyRegexp keeps package init from compiling patterns on every asc invocation.
type lazyRegexp struct {
	once sync.Once
	expr string
	re   *regexp.Regexp
}

var lazyPatterns []*lazyRegexp

func lazyMustCompile(expr string) *lazyRegexp {
	l := &lazyRegexp{expr: expr}
	lazyPatterns = append(lazyPatterns, l)
	return l
}

func (l *lazyRegexp) compiled() *regexp.Regexp {
	l.once.Do(func() { l.re = regexp.MustCompile(l.expr) })
	return l.re
}

func (l *lazyRegexp) MatchString(s string) bool { return l.compiled().MatchString(s) }

func (l *lazyRegexp) FindStringIndex(s string) []int { return l.compiled().FindStringIndex(s) }

func (l *lazyRegexp) FindStringSubmatch(s string) []string {
	return l.compiled().FindStringSubmatch(s)
}

func (l *lazyRegexp) FindStringSubmatchIndex(s string) []int {
	return l.compiled().FindStringSubmatchIndex(s)
}

func (l *lazyRegexp) FindAllStringIndex(s string, n int) [][]int {
	return l.compiled().FindAllStringIndex(s, n)
}

func (l *lazyRegexp) FindAllStringSubmatch(s string, n int) [][]string {
	return l.compiled().FindAllStringSubmatch(s, n)
}

func (l *lazyRegexp) FindAllStringSubmatchIndex(s string, n int) [][]int {
	return l.compiled().FindAllStringSubmatchIndex(s, n)
}

func (l *lazyRegexp) ReplaceAllString(src, repl string) string {
	return l.compiled().ReplaceAllString(src, repl)
}

var (
	secretMarkerPattern            = lazyMustCompile(`(?i)(^|[ \t])-{1,2}secret(?:` + singleLineShellTerminator + `|[ \t]*=[ \t]*(?:1|t|true)(?:` + singleLineShellTerminator + `))`)
	secretValuePattern             = lazyMustCompile(`(?i)(^|[ \t])(-{1,2}value(?:[ \t]+|[ \t]*=[ \t]*))(?:\[REDACTED(?: PRIVATE KEY)?\]|` + escapeAwareQuotedValue + `|` + unterminatedQuotedValue + `|` + flagUnquotedValue + `)`)
	kubectlFromLiteralValue        = lazyMustCompile(`(?i)(--from-literal(?:[ \t]+|[ \t]*=[ \t]*))(?:\[REDACTED(?: PRIVATE KEY)?\]|` + kubectlShellWord + `)`)
	securityCredentialFlagPatterns = map[string]*lazyRegexp{
		"create-keychain":                      newCommandShortCredentialFlagPattern("p"),
		"unlock-keychain":                      newCommandShortCredentialFlagPattern("p"),
		"set-keychain-password":                newCommandShortCredentialFlagPattern("o", "p"),
		"add-generic-password":                 newCommandShortCredentialFlagPattern("p", "w", "X"),
		"add-internet-password":                newCommandShortCredentialFlagPattern("w", "X"),
		"set-generic-password-partition-list":  newCommandShortCredentialFlagPattern("k"),
		"set-internet-password-partition-list": newCommandShortCredentialFlagPattern("k"),
		"set-key-partition-list":               newCommandShortCredentialFlagPattern("k"),
		"import":                               newCommandShortCredentialFlagPattern("P"),
		"export":                               newCommandShortCredentialFlagPattern("P"),
		"cms":                                  newCommandShortCredentialFlagPattern("p"),
		"create-filevaultmaster-keychain":      newCommandShortCredentialFlagPattern("p"),
	}
	securityAttachedCredentialFlags = map[string]string{
		"create-keychain":                      "p",
		"unlock-keychain":                      "p",
		"set-keychain-password":                "op",
		"add-generic-password":                 "pwX",
		"add-internet-password":                "wX",
		"set-generic-password-partition-list":  "k",
		"set-internet-password-partition-list": "k",
		"set-key-partition-list":               "k",
		"import":                               "P",
		"export":                               "P",
		"cms":                                  "p",
		"create-filevaultmaster-keychain":      "p",
	}
	opensslCredentialOptions            = []string{"passin", "passout", "passcerts", "pass", "k", "K"}
	opensslSubcommandCredentialPatterns = map[string]*lazyRegexp{
		"ca":   newCommandCredentialFlagValueStartPattern("key"),
		"dgst": newCommandCredentialFlagValueStartPattern("hmac"),
	}
	opensslSubcommandCredentialOptions = map[string][]string{
		"pkeyutl":  {"pkeyopt_passin"},
		"s_client": {"proxy_pass", "psk", "srppass"},
		"s_server": {"dpass", "psk"},
	}
	opensslSubcommandArgumentOptions = map[string]string{
		"pkeyutl":  "engine config in inkey passin peerkey peerform sigfile keyform out secret digest pkeyopt pkeyopt_passin kdf kdflen kemop rand writerand provider-path provider provparam propquery",
		"s_client": "engine ssl_client_engine ssl_config ctlogfile host port connect bind proxy proxy_user proxy_pass unix maxfraglen max_send_frag split_send_frag max_pipelines read_buf cert certform cert_chain key keyform pass verify nameopt CApath CAfile CAstore requestCAfile dane_tlsa_domain dane_tlsa_rrdata psk_identity psk psk_session name sess_out sess_in starttls xmpphost msgfile keymatexport keymatexportlen keylogfile servername serverinfo alpn mtu nextprotoneg early_data use_srtp srpuser srppass srp_strength rand writerand sigalgs client_sigalgs groups curves named_curve cipher ciphersuites min_protocol max_protocol record_padding policy purpose verify_name verify_depth auth_level attime verify_hostname verify_email verify_ip CRL CRLform chainCAfile chainCApath chainCAstore verifyCAfile verifyCApath verifyCAstore xkey xcert xchain xcertform xkeyform provider-path provider provparam propquery",
		"s_server": "ssl_config engine port accept unix context CAfile CApath CAstore verify Verify nameopt cert cert2 certform cert_chain serverinfo key key2 keyform pass dcert dcertform dcert_chain dkey dkeyform dpass dhparam servername id_prefix keymatexport keymatexportlen CRL CRLform chainCAfile chainCApath chainCAstore verifyCAfile verifyCApath verifyCAstore status_timeout status_url proxy no_proxy status_file msgfile max_pipelines naccept keylogfile mtu read_buf split_send_frag max_send_frag psk_identity psk_hint psk psk_session srpvfile srpuserseed max_early_data recv_max_early_data num_tickets use_srtp nextprotoneg alpn rand writerand sigalgs client_sigalgs groups curves named_curve cipher ciphersuites min_protocol max_protocol record_padding policy purpose verify_name verify_depth auth_level attime verify_hostname verify_email verify_ip xkey xcert xchain xcertform xkeyform provider-path provider provparam propquery",
	}
	opensslMACKeyOption       = openSSLSecretOption{newCommandCredentialFlagValueStartPattern("macopt"), openSSLOptionValueWithPrefix("key:", "hexkey:")}
	opensslSecretOptionValues = map[string]openSSLSecretOption{
		"dgst": opensslMACKeyOption,
		"mac":  opensslMACKeyOption,
		"kdf":  {newCommandCredentialFlagValueStartPattern("kdfopt"), openSSLOptionValueWithPrefix("key:", "hexkey:", "secret:", "hexsecret:", "pass:", "hexpass:")},
	}
	keytoolCredentialFlagPattern          = newCommandCredentialFlagPatternWithSuffix("(?::(?:env|file))?", "storepass", "keypass", "new", "srcstorepass", "deststorepass", "srckeypass", "destkeypass")
	jarsignerCredentialFlagPattern        = newCommandCredentialFlagPatternWithSuffix("(?::(?:env|file))?", "storepass", "keypass")
	dockerLoginCredentialFlagPattern      = newCommandShortCredentialFlagPattern("p")
	zipCredentialFlagPattern              = newCommandShortCredentialFlagPattern("P")
	rawCookieJarPattern                   = lazyMustCompile(`(?i)"cookies"[ \t\r\n]*:[ \t\r\n]*(?:\{|\[)`)
	escapedCookieJarPattern               = lazyMustCompile(`(?i)\\"cookies\\"[ \t\r\n]*:[ \t\r\n]*(?:\{|\[)`)
	rawRegistryAuthsPattern               = lazyMustCompile(`(?i)"auths"[ \t\r\n]*:[ \t\r\n]*\{`)
	escapedRegistryAuthsPattern           = lazyMustCompile(`(?i)\\"auths\\"[ \t\r\n]*:[ \t\r\n]*\{`)
	rawRequestHeaders                     = lazyMustCompile(`(?i)"requestHeaders"[ \t\r\n]*:[ \t\r\n]*\[`)
	escapedRequestHeaders                 = lazyMustCompile(`(?i)\\"requestHeaders\\"[ \t\r\n]*:[ \t\r\n]*\[`)
	rawStructuredValueStart               = lazyMustCompile(`(?i)"value"[ \t\r\n]*:[ \t\r\n]*"`)
	escapedValueStart                     = lazyMustCompile(`(?i)\\"value\\"[ \t\r\n]*:[ \t\r\n]*\\"`)
	rawCredentialObject                   = lazyMustCompile(`(?i)"` + structuredCredentialName + `"[ \t\r\n]*:[ \t\r\n]*\{`)
	escapedCredentialObject               = lazyMustCompile(`(?i)\\"` + structuredCredentialName + `\\"[ \t\r\n]*:[ \t\r\n]*\{`)
	rawCredentialArray                    = lazyMustCompile(`(?i)"` + structuredCredentialName + `"[ \t\r\n]*:[ \t\r\n]*\[`)
	escapedCredentialArray                = lazyMustCompile(`(?i)\\"` + structuredCredentialName + `\\"[ \t\r\n]*:[ \t\r\n]*\[`)
	credentialHeaderNamePattern           = lazyMustCompile(`(?i)^` + credentialHeaderName + `$`)
	sensitiveAssignmentHeaderName         = lazyMustCompile(`(?i)^` + sensitivePrefixedName + `$`)
	queryCredentialNamePattern            = lazyMustCompile(`(?i)^` + queryCredentialName + `$`)
	queryParameterName                    = lazyMustCompile(`[?&]([^=&#\s"'<>]+)=`)
	curlHeaderOptionStart                 = lazyMustCompile(`(?i)(^|\s)(` + curlHeaderOptionPrefix + `)`)
	completeShellWord                     = lazyMustCompile(`^(` + fishShellWord + `)(` + singleLineShellTerminator + `)`)
	netrcEntryStart                       = lazyMustCompile(`(?im)(?:^|[\r\n])[ \t]*(?:machine[ \t]+[^\s#]+|default)(?:[ \t\r\n]|\z)`)
	netrcPasswordValue                    = lazyMustCompile(`(?i)(^|[ \t\r\n])(password[ \t]+)` + singleLineShellWord + `(` + singleLineShellTerminator + `)`)
	booleanSecretMarker                   = lazyMustCompile(`(?i)(^|\s)(-{1,2}secret)([ \t]*=[ \t]*)(true|false|1|0|t|f)(` + singleLineShellTerminator + `)`)
	yamlCredentialScalar                  = lazyMustCompile(`(?i)^([ \t]*(?:-[ \t]+)?` + yamlMappingKeyProperties + `(?:["']?` + yamlCredentialName + `["']?)[ \t]*:[ \t]*)(?:(?:[!&][^\s#]+)[ \t]*)*[|>](?:[+-]?[1-9]?|[1-9][+-]?)[ \t]*(?:#[^\r\n]*)?$`)
	yamlCredentialMapping                 = lazyMustCompile(`(?i)^([ \t]*(?:-[ \t]+)?` + yamlMappingKeyProperties + `(?:["']?` + yamlCredentialName + `["']?)[ \t]*:)[ \t]*(?:(?:[!&][^\s#]+)[ \t]*)*(?:#[^\r\n]*)?$`)
	yamlCredentialPlainScalar             = lazyMustCompile(`(?i)^([ \t]*(?:-[ \t]+)?` + yamlMappingKeyProperties + `(?:["']?` + yamlCredentialName + `["']?)[ \t]*:[ \t]*)[^"'[\{\s\r\n][^\r\n]*$`)
	yamlCredentialFlowStart               = lazyMustCompile(`(?im)^([ \t]*(?:-[ \t]+)?` + yamlMappingKeyProperties + `(?:["']?` + yamlCredentialName + `["']?)[ \t]*:[ \t]*)([\[{])`)
	yamlExplicitCredentialKey             = lazyMustCompile(`(?i)^[ \t]*(?:-[ \t]+)?\?[ \t]+` + yamlMappingKeyProperties + `(?:["']?` + yamlCredentialName + `["']?)[ \t]*(?:#[^\r\n]*)?$`)
	yamlBlockScalarIndicator              = lazyMustCompile(`^[|>](?:[+-]?[1-9]?|[1-9][+-]?)$`)
	yamlCredentialAlias                   = lazyMustCompile(`(?im)^[ \t]*(?:-[ \t]+)?` + yamlMappingKeyProperties + `(?:["']?` + yamlCredentialName + `["']?)[ \t]*:[ \t]*\*([a-z0-9_-]+)[ \t]*(?:#[^\r\n]*)?$`)
	yamlDocumentBoundary                  = lazyMustCompile(`^(?:---|\.\.\.)(?:[ \t]|$)`)
	yamlValueAlias                        = lazyMustCompile(`\*([a-zA-Z0-9_-]+)\b`)
	yamlAnchor                            = lazyMustCompile(`&([a-zA-Z0-9_-]+)\b`)
	yamlSensitiveNameAnchor               = lazyMustCompile(`(?im)&([a-zA-Z0-9_-]+)[ \t]+(?:(?:` + yamlNodeTag + `)[ \t]+)*(?:["']?` + yamlCredentialName + `["']?)[ \t]*(?:#[^\r\n]*)?$`)
	yamlAliasMappingKey                   = lazyMustCompile(`(?i)^([ \t]*(?:-[ \t]+)?)(\*([a-zA-Z0-9_-]+))([ \t]*:)`)
	yamlExplicitAliasKey                  = lazyMustCompile(`(?i)^([ \t]*(?:-[ \t]+)?\?[ \t]+)(\*([a-zA-Z0-9_-]+))([ \t]*(?:#[^\r\n]*)?)$`)
	jsonQuotedScalarLine                  = lazyMustCompile(`^"(?:\\.|[^"\\])*"[ \t]*,?[ \t]*$`)
	jsonCredentialName                    = lazyMustCompile(`(?i)^(?:` + structuredCredentialName + `)$`)
	yamlCredentialNamePattern             = lazyMustCompile(`(?i)^(?:` + yamlCredentialName + `)$`)
	tomlCredentialName                    = lazyMustCompile(`(?i)^(?:` + sensitivePrefixedName + `)$`)
	tomlMultilineCredentialStart          = lazyMustCompile(`(?i)(?:^|[^-a-z0-9_])(?:` + sensitivePrefixedName + `\b|` + tomlQuotedSensitiveKey + `)[ \t]*=[ \t]*("""|''')`)
	sensitiveCommandSubstitutionStart     = lazyMustCompile(`(?i)(?:^|\s)(?:` + sensitiveShellFlagToken + `(?:[ \t]+|[ \t]*=[ \t]*)|` + sensitivePrefixedName + `\b[ \t]*[:=][ \t]*)(\$\(|\(|\x60)`)
	powerShellHereStringCredential        = lazyMustCompile(`(?i)(?:^|\s)(?:` + sensitiveShellFlagToken + `(?:` + shellCommandPathSeparator + `|[ \t]*=[ \t]*)|` + powerShellSensitiveVariable + `[ \t]*=[ \t]*)(@["']\r?\n)`)
	powerShellCollectionCredential        = lazyMustCompile(`(?i)(?:^|\s)` + powerShellSensitiveVariable + `[ \t]*=[ \t]*(@[({])`)
	commandPromptQuotedSetAssignment      = lazyMustCompile(`(?im)(?:^|[ \t;&|])set[ \t]+"` + sensitivePrefixedName + `\b[ \t]*=[ \t]*`)
	commandPromptUnquotedSetAssignment    = lazyMustCompile(`(?im)(?:^|[ \t;&|()])set[ \t]+` + sensitivePrefixedName + `\b[ \t]*=[ \t]*`)
	bareEnvironmentDumpCredential         = lazyMustCompile(`(?im)^([ \t]*(?:export[ \t]+)?(?:` + sensitivePrefixedName + `|(?-i:` + uppercaseSessionEnvironmentName + `))\b[ \t]*=[ \t]*)([^\s"'\\;&|<>()]+(?:[ \t]+[^\s"'\\;&|<>()]+)+)([ \t]*\r?)$`)
	shellAssignmentWord                   = lazyMustCompile(`^[A-Za-z_][A-Za-z0-9_]*=`)
	curlConfigCertificateEntry            = lazyMustCompile(`(?im)^([ \t]*(?:cert|proxy-cert)` + curlConfigSeparator + `)`)
	xmlCredentialElementStart             = lazyMustCompile(`(?i)<(?:[a-z_][a-z0-9_.-]*:)?` + structuredCredentialName + `(?:[ \t\r\n/>])`)
	xmlCredentialName                     = lazyMustCompile(`(?i)^(?:` + structuredCredentialName + `)$`)
	xmlElementStart                       = lazyMustCompile(`<[a-zA-Z_:][a-zA-Z0-9_.:-]*(?:[ \t\r\n/>])`)
	xmlAttribute                          = lazyMustCompile(`(?s)(?:^|[ \t\r\n])([a-zA-Z_:][a-zA-Z0-9_.:-]*)[ \t\r\n]*=[ \t\r\n]*(?:"([^"]*)"|'([^']*)')`)
	authorizationHeaderValueStart         = lazyMustCompile(`(?i)\bauthorization[ \t]*[:=][ \t]*`)
	standaloneBearerCandidate             = lazyMustCompile(`(?i)\bbearer[ \t]+([-a-z0-9._~+/=]+)`)
	envLongSplitStringOption              = lazyMustCompile(`(?i)--split-string[ \t]*=`)
	standaloneURLSafeCredentialCandidates = []*lazyRegexp{
		lazyMustCompile(`AIza[A-Za-z0-9_-]{35}`),
		lazyMustCompile(`SG\.[A-Za-z0-9_-]{22}\.[A-Za-z0-9_-]{43}`),
	}
	xcodeCloudEnvVarSetCommand = lazyMustCompile(`(?i)(?:\basc\b|"asc"|'asc')` + shellCommandPathSeparator + `web` + shellCommandPathSeparator + `xcode-cloud` + shellCommandPathSeparator + `env-vars` + shellCommandPathSeparator + `(?:shared` + shellCommandPathSeparator + `)?set\b`)
)

var structuredContainerValueRedactionRules = []redactionRule{
	{lazyMustCompile(`(?i)("value"[ \t\r\n]*:[ \t\r\n]*")(?:\\.|[^"\\\r\n])*(")([ \t\r\n]*(?:[,}\]]|\z))`), `${1}` + redactionMarker + `${2}${3}`},
	{lazyMustCompile(`(?i)(\\"value\\"[ \t\r\n]*:[ \t\r\n]*\\")(?:\\.|[^"\\\r\n])*?(\\")([ \t\r\n]*(?:[,}\]]|\z))`), `${1}` + redactionMarker + `${2}${3}`},
}

var registryAuthValueRedactionRules = []redactionRule{
	{lazyMustCompile(`(?i)("auth"[ \t\r\n]*:[ \t\r\n]*")(?:\\.|[^"\\\r\n])*(")`), `${1}` + redactionMarker + `${2}`},
	{lazyMustCompile(`(?i)(\\"auth\\"[ \t\r\n]*:[ \t\r\n]*\\")(?:\\.|[^"\\\r\n])*?(\\")`), `${1}` + redactionMarker + `${2}`},
}

var curlUserCredentialRedactionRules = []redactionRule{
	{lazyMustCompile(`(?i)(^|\s)((?:-u|--(?:proxy-)?user)\b` + curlOptionValueSeparator + `)(?:\[REDACTED(?: PRIVATE KEY)?\]|` + credentialPairValue + `)`), `${1}${2}` + redactionMarker},
	{lazyMustCompile(`(?i)(^|\s)((?:-u|--(?:proxy-)?user)\b[ \t]*=[ \t]*)(?:\[REDACTED(?: PRIVATE KEY)?\]|` + credentialPairValue + `)`), `${1}${2}` + redactionMarker},
	{lazyMustCompile(`(?i)(^|\s)(-u)(?:\[REDACTED(?: PRIVATE KEY)?\]|` + credentialPairValue + `)`), `${1}${2}` + redactionMarker},
}

var curlCertificateCredentialRedactionRules = []redactionRule{
	{lazyMustCompile(`(?i)(^|\s)(` + curlCertOptionPrefix + `)(")((?:` + escapedQuotedCharacter + `|[^"\\:\r\n])*):(?:` + escapedQuotedCharacter + `|[^"\\])+(")`), `${1}${2}${3}${4}:` + redactionMarker + `${5}`},
	{lazyMustCompile(`(?i)(^|\s)(` + curlCertOptionPrefix + `)(')((?:` + escapedQuotedCharacter + `|[^'\\:\r\n])*):(?:` + escapedQuotedCharacter + `|[^'\\])+(')`), `${1}${2}${3}${4}:` + redactionMarker + `${5}`},
	{lazyMustCompile(`(?i)(^|\s)(` + curlCertOptionPrefix + `)(` + curlCertShellPath + `):` + singleLineShellWord), `${1}${2}${3}:` + redactionMarker},
}

var curlArgumentCredentialRedactionRules = []redactionRule{
	{lazyMustCompile(`(?i)(^|\s)(` + curlHeaderOptionPrefix + `)(` + credentialHeaderName + `)[ \t]*:[ \t]*(?:\\(?:\r?\n|[^\r\n])|[^\s;&|<>()])+`), `${1}${2}${3}:` + redactionMarker},
	{lazyMustCompile(`(?i)(^|\s)((?:(?-i:-b)|--cookie)\b[ \t]+)(?:\[REDACTED(?: PRIVATE KEY)?\]|` + cookieDataValue + `)`), `${1}${2}` + redactionMarker},
	{lazyMustCompile(`(?i)(^|\s)((?:(?-i:-b)|--cookie)\b[ \t]*=[ \t]*)(?:\[REDACTED(?: PRIVATE KEY)?\]|` + cookieDataValue + `)`), `${1}${2}` + redactionMarker},
	{lazyMustCompile(`(?i)(^|\s)((?-i:-b))(?:\[REDACTED(?: PRIVATE KEY)?\]|` + cookieDataValue + `)`), `${1}${2}` + redactionMarker},
	{lazyMustCompile(`(?i)(^|\s)(` + curlFormDataOptionPrefix + `)(")((?:\\.|[^"\\])*?` + sensitivePrefixedName + `\b[ \t]*=[ \t]*)(?:\\.|[^"\\])*(")`), `${1}${2}${3}${4}` + redactionMarker + `${5}`},
	{lazyMustCompile(`(?i)(^|\s)(` + curlFormDataOptionPrefix + `)(')([^']*?` + sensitivePrefixedName + `\b[ \t]*=[ \t]*)[^']*(')`), `${1}${2}${3}${4}` + redactionMarker + `${5}`},
}

// Redact complete single-line shell words first so adjacent quoted and
// unquoted fragments cannot leak, and an unmatched quote in an earlier log
// line cannot claim a later command's opening quote as its closer.
var singleLineShellWordRedactionRules = []redactionRule{
	{lazyMustCompile(`(?m)(^|[ \t;&|])(` + uppercaseSessionEnvironmentName + `[ \t]*=[ \t]*)` + singleLineShellWord + `(` + singleLineShellTerminator + `)`), `${1}${2}` + redactionMarker + `${3}`},
	{lazyMustCompile(`(?im)(^|[ \t;&|])(` + powerShellSensitiveVariable + `[ \t]*=[ \t]*)(?:&[ \t]+)?(?:[a-z0-9_.-]+\\)?ConvertTo-SecureString[ \t]+(?:` + powerShellSecureStringSwitch + `)*(?:-String(?:[ \t]+|[ \t]*:[ \t]*))?` + fishShellWord + `(` + singleLineShellTerminator + `)`), `${1}${2}` + redactionMarker + `${3}`},
	{lazyMustCompile(`(?im)(^|[;&|][ \t]*)([ \t]*set[ \t]+(?:(?:--|-[a-z]+|--[a-z][a-z-]*)[ \t]+)*` + sensitivePrefixedName + `\b[ \t]+)` + fishShellWord + `(?:[ \t]+` + fishShellWord + `)*`), `${1}${2}` + redactionMarker},
	{lazyMustCompile(`(?i)(^|\s)(` + sensitiveOrSecretShellToken + `[ \t]*=[ \t]*)` + fishShellWord + `(` + singleLineShellTerminator + `)`), `${1}${2}` + redactionMarker + `${3}`},
	{lazyMustCompile(`(?i)(^|[^-a-z0-9_])(` + sensitivePrefixedName + `\b[ \t]*=[ \t]*)` + singleLineShellWord + `(` + singleLineShellTerminator + `)`), `${1}${2}` + redactionMarker + `${3}`},
}

var commandScopedSensitiveFlagRedactionRules = []redactionRule{
	{lazyMustCompile(`(?i)(^|\s)(` + sensitiveShellFlagToken + shellCommandPathSeparator + `)` + fishShellWord + `(` + singleLineShellTerminator + `)`), `${1}${2}` + redactionMarker + `${3}`},
	{lazyMustCompile(`(?i)(^|\s)(` + sensitiveShellFlagToken + shellCommandPathSeparator + `)(?:\[REDACTED(?: PRIVATE KEY)?\]|` + escapeAwareQuotedValue + `|` + unterminatedQuotedValue + `|` + shellUnquotedValue + `)`), `${1}${2}` + redactionMarker},
}

var sensitiveTextRedactionRules = []redactionRule{
	{lazyMustCompile(`(?s)-----BEGIN[ \t]+(?:[A-Z0-9]+[ \t]+)*PRIVATE[ \t]+KEY(?:[ \t]+BLOCK)?-----.*?-----END[ \t]+(?:[A-Z0-9]+[ \t]+)*PRIVATE[ \t]+KEY(?:[ \t]+BLOCK)?-----`), privateKeyRedactionMarker},
	{lazyMustCompile(`(?s)-----BEGIN[ \t]+(?:[A-Z0-9]+[ \t]+)*PRIVATE[ \t]+KEY(?:[ \t]+BLOCK)?-----.*\z`), privateKeyRedactionMarker},
	{lazyMustCompile(`(?im)^([ \t]*(?:redirect_)?http_` + cgiCredentialHeaderName + `\b[ \t]*=[ \t]*)[^\r\n]*(\r?)$`), `${1}` + redactionMarker + `${2}`},
	{lazyMustCompile(`(?m)^(` + passwordFileHostField + `:(?:[0-9]+|\*):` + passwordFileField + `:` + passwordFileField + `:)` + passwordFileField + `(\r?)$`), `${1}` + redactionMarker + `${2}`},
	{lazyMustCompile(`(?im)^([ \t]*(?:-[ \t]+)?` + yamlMappingKeyProperties + yamlCredentialName + `[ \t]*:[ \t]*)(?:\[[^\]\r\n]*\]|\{[^}\r\n]*\})`), `${1}` + redactionMarker},
	{lazyMustCompile(`(?im)^([ \t]*(?:-[ \t]+)?` + yamlMappingKeyProperties + `["']` + yamlCredentialName + `["'][ \t]*:[ \t]*)(?:\[[ \t]*[^"'\]\r\n][^\]\r\n]*\]|\{[ \t]*[^"'}\r\n][^}\r\n]*\})`), `${1}` + redactionMarker},
	{lazyMustCompile(`(?im)^([ \t]*` + yamlMappingKeyProperties + `(?:["']?` + yamlCredentialName + `["']?)[ \t]*:[ \t]*)(?:[^"'[{\s\r\n][^\r\n]*)(\r?)$`), `${1}` + redactionMarker + `${2}`},
	{lazyMustCompile(`(?i)(["']` + structuredCredentialName + `["'][ \t\r\n]*:[ \t\r\n]*\[)[ \t\r\n]*(?:"(?:\\.|[^"\\\r\n])*"(?:[ \t\r\n]*,[ \t\r\n]*"(?:\\.|[^"\\\r\n])*")*)[ \t\r\n]*(\])`), `${1}"` + redactionMarker + `"${2}`},
	{lazyMustCompile(`(?i)(\\"` + structuredCredentialName + `\\"[ \t\r\n]*:[ \t\r\n]*\[)[ \t\r\n]*(?:\\"(?:\\.|[^"\\\r\n])*?\\"(?:[ \t\r\n]*,[ \t\r\n]*\\"(?:\\.|[^"\\\r\n])*?\\")*)[ \t\r\n]*(\])`), `${1}\"` + redactionMarker + `\"${2}`},
	{lazyMustCompile(`(?i)("securitycode"[ \t\r\n]*:[ \t\r\n]*\{[ \t\r\n]*"code"[ \t\r\n]*:[ \t\r\n]*")(?:\\.|[^"\\\r\n])*(")`), `${1}` + redactionMarker + `${2}`},
	{lazyMustCompile(`(?i)(\\"securitycode\\"[ \t\r\n]*:[ \t\r\n]*\{[ \t\r\n]*\\"code\\"[ \t\r\n]*:[ \t\r\n]*\\")(?:\\.|[^"\\\r\n])*?(\\")`), `${1}` + redactionMarker + `${2}`},
	{lazyMustCompile(`(?i)("authorization[ \t]*:[ \t]*)(?:` + escapedQuotedCharacter + `|[^"\\])*(")`), `${1}` + redactionMarker + `${2}`},
	{lazyMustCompile(`(?i)('authorization[ \t]*:[ \t]*)(?:` + escapedQuotedCharacter + `|[^'\\])*(')`), `${1}` + redactionMarker + `${2}`},
	{lazyMustCompile(`(?i)\bauthorization[ \t]*[:=][ \t]*(?:bearer|basic|token)[ \t]+(?:` + escapeAwareQuotedValue + `|` + unterminatedQuotedValue + `|[^\s,;"']+)` + foldedHeaderContinuation), "Authorization: " + redactionMarker},
	{lazyMustCompile(`(?i)\bauthorization[ \t]*[:=][ \t]*[a-z][a-z0-9_-]*[ \t]+[^\s=,]+[ \t]*=[^\r\n]+` + foldedHeaderContinuation), "Authorization: " + redactionMarker},
	{lazyMustCompile(`(?i)("` + traceCredentialHeader + `[ \t]*:[ \t]*)(?:` + escapedQuotedCharacter + `|[^"\\])*(")`), `${1}` + redactionMarker + `${2}`},
	{lazyMustCompile(`(?i)('` + traceCredentialHeader + `[ \t]*:[ \t]*)(?:` + escapedQuotedCharacter + `|[^'\\])*(')`), `${1}` + redactionMarker + `${2}`},
	{lazyMustCompile(`(?i)\b(` + credentialHeaderName + `|` + sensitivePrefixedName + `)[ \t]*:\[[^\]\r\n]*\]`), `${1}:` + redactionMarker},
	{lazyMustCompile(`(?im)^([ \t]*(?:(?:[<>][ \t]*)?` + traceCredentialHeader + `|[<>][ \t]*` + sensitivePrefixedName + `))[ \t]*:[ \t]*[^\r\n]+` + foldedHeaderContinuation), `${1}: ` + redactionMarker},
	{lazyMustCompile(`(?i)(^|[\s"'(<>{}\[\],;])((?:cookie|set-cookie)[ \t]*:[ \t]*)(?:` + cookieDataQuoted + `|` + cookieDataUnquoted + `)(?:[ \t]*;[ \t]*` + cookieDataUnquoted + `)*` + foldedHeaderContinuation), `${1}${2}` + redactionMarker},
	{lazyMustCompile(`(?i)(^|[\s"'(<>{}\[\],;])((?:` + traceCredentialHeader + `)[ \t]*:[ \t]*)(?:\[REDACTED(?: PRIVATE KEY)?\]|` + escapeAwareQuotedValue + `|` + unterminatedQuotedValue + `|[^\s,;"'()<>{}\[\]]+)` + foldedHeaderContinuation), `${1}${2}` + redactionMarker},
	{lazyMustCompile(`(?i)\b([a-z][a-z0-9+.-]*://)[^/?#\s@]+@`), `${1}` + redactionMarker + `@`},
	{lazyMustCompile(`(?i)(^|[\s"'=])[^/?#\s@:]+:[^/?#\s@]+@([a-z0-9.-]+:[^\s"'<>]+)`), `${1}` + redactionMarker + `@${2}`},
	{lazyMustCompile(`(?i)\b(https://hooks\.slack(?:-gov)?\.com/services/)[^?#\s"'()<>{}\[\],;]+`), `${1}` + redactionMarker},
	{lazyMustCompile(`(?i)\b(https://(?:(?:canary|ptb)\.)?discord(?:app)?\.com/api(?:/v[0-9]+)?/webhooks/[0-9]+/)[^?#\s"'()<>{}\[\],;]+`), `${1}` + redactionMarker},
	{lazyMustCompile(`(?i)([?&]` + queryCredentialName + `=)[^&#\s"'<>]+`), `${1}` + redactionMarker},
	{lazyMustCompile(`(?im)^([ \t]*(?:header|proxy-header)` + curlConfigSeparator + `")(` + credentialHeaderName + `|` + sensitivePrefixedName + `)([ \t]*:[ \t]*)(?:\\.|[^"\\\r\n])+(")`), `${1}${2}${3}` + redactionMarker + `${4}`},
	{lazyMustCompile(`(?im)^([ \t]*(?:header|proxy-header)` + curlConfigSeparator + `)(` + credentialHeaderName + `|` + sensitivePrefixedName + `)([ \t]*:[ \t]*)[^\r\n]+$`), `${1}${2}${3}` + redactionMarker},
	{lazyMustCompile(`(?im)^([ \t]*(?:user|proxy-user)` + curlConfigSeparator + `)(?:\[REDACTED(?: PRIVATE KEY)?\]|` + credentialPairValue + `)`), `${1}` + redactionMarker},
	{lazyMustCompile(`(?im)^([ \t]*(?:oauth2-bearer|proxy-tlspassword|tlspassword)` + curlConfigSeparator + `)(?:\[REDACTED(?: PRIVATE KEY)?\]|` + escapeAwareQuotedValue + `|` + unterminatedQuotedValue + `|[^\s#]+)`), `${1}` + redactionMarker},
	{lazyMustCompile(`(?im)^([ \t]*pass` + curlConfigExplicitSeparator + `)(?:\[REDACTED(?: PRIVATE KEY)?\]|` + escapeAwareQuotedValue + `|` + unterminatedQuotedValue + `|[^\s#]+)`), `${1}` + redactionMarker},
	{lazyMustCompile(`(?im)^([ \t]*cookie` + curlConfigSeparator + `)(?:\[REDACTED(?: PRIVATE KEY)?\]|` + cookieDataValue + `)`), `${1}` + redactionMarker},
	{lazyMustCompile(`(?im)^((?:#HttpOnly_)?[^#\t\r\n][^\t\r\n]*\t(?:TRUE|FALSE)\t[^\t\r\n]*\t(?:TRUE|FALSE)\t[0-9]+\t[^\t\r\n]+\t)(?:\[REDACTED(?: PRIVATE KEY)?\]|[^\t\r\n]+)`), `${1}` + redactionMarker},
	{lazyMustCompile(`(?i)(^|\s)(-{1,2}secret\b` + shellCommandPathSeparator + `)(?:\[REDACTED(?: PRIVATE KEY)?\]|` + escapeAwareQuotedValue + `|` + unterminatedQuotedValue + `|` + flagUnquotedValue + `)`), `${1}${2}` + redactionMarker},
	{lazyMustCompile(`(?i)(^|\s)(` + sensitiveOrSecretShellToken + `[ \t]*=[ \t]*)(?:\[REDACTED(?: PRIVATE KEY)?\]|` + escapeAwareQuotedValue + `|` + unterminatedQuotedValue + `|` + shellUnquotedValue + `)`), `${1}${2}` + redactionMarker},
	{lazyMustCompile(`(?i)(\\"` + structuredCredentialName + `\\"[ \t\r\n]*:[ \t\r\n]*\\")(?:\\.|[^"\\\r\n])*?(\\")([ \t\r\n]*(?:[,}\]]|\z))`), `${1}` + redactionMarker + `${2}${3}`},
	{lazyMustCompile(`(?i)(["']` + structuredCredentialName + `["'][ \t\r\n]*:[ \t\r\n]*)(?:` + escapeAwareQuotedValue + `|` + unterminatedQuotedValue + `|[^\s,;}\[\]]+)`), `${1}"` + redactionMarker + `"`},
	{lazyMustCompile(`(?i)(^|[^-a-z0-9_])(` + tomlQuotedSensitiveKey + `[ \t]*=[ \t]*)(?:\[REDACTED(?: PRIVATE KEY)?\]|` + escapeAwareQuotedValue + `|` + unterminatedQuotedValue + `|` + shellUnquotedValue + `)`), `${1}${2}` + redactionMarker},
	{lazyMustCompile(`(?i)(^|[^-a-z0-9_])(` + sensitivePrefixedName + `\b[ \t]*=[ \t]*)(?:\[REDACTED(?: PRIVATE KEY)?\]|(?:(?:bearer|basic|token)[ \t]+)` + shellUnquotedValue + `|` + escapeAwareQuotedValue + `|` + unterminatedQuotedValue + `|` + shellUnquotedValue + `)`), `${1}${2}` + redactionMarker},
	{lazyMustCompile(`(?im)(^[ \t]*|[\[{(,;][ \t]*)(` + yamlMappingKeyProperties + sensitivePrefixedName + `\b[ \t]*:[ \t]*)(?:\[REDACTED(?: PRIVATE KEY)?\]|(?:(?:bearer|basic|token)[ \t]+)` + structuredUnquotedValue + `|` + escapeAwareQuotedValue + `|` + unterminatedQuotedValue + `|` + structuredUnquotedValue + `)`), `${1}${2}` + redactionMarker},
	{lazyMustCompile(`\beyJ[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\b`), redactionMarker},
	{lazyMustCompile(`\b(?:github_pat_[A-Za-z0-9_]{20,}|gh[pousr]_[A-Za-z0-9]{20,}|xoxb-[0-9]{10,}-[0-9]{10,}-[A-Za-z0-9]{20,}|xoxp-[A-Za-z0-9-]{20,}|xapp-[0-9]+-[A-Za-z0-9]{10,}(?:-[A-Za-z0-9]{6,})+|npm_[A-Za-z0-9]{36}|glpat-[A-Za-z0-9_-]{20,}|[sr]k_(?:live|test)_[A-Za-z0-9]{16,})\b`), redactionMarker},
}

func redactSensitiveText(value string) (string, bool) {
	return redactSensitiveTextDepth(value, 0)
}

// Each redaction step redacts one kind of secret. Steps run in order, and each
// step sees the output of the previous one.
var (
	commandArgumentRedactionSteps = []func(string) (string, bool){
		redactKubernetesSecretData,
		redactPowerShellHereStringCredentials,
		redactPowerShellCollectionCredentials,
		redactCommandPromptSetAssignments,
		redactCurlConfigCertificatePasswords,
		redactSecretMarkedValues,
		redactKubectlSecretLiterals,
		redactSecurityCredentialArguments,
		redactOpenSSLCredentialArguments,
		redactNamedCommandArguments("keytool", "", keytoolCredentialFlagPattern),
		redactNamedCommandArguments("jarsigner", "", jarsignerCredentialFlagPattern),
		redactDockerLoginCredentialArguments,
		redactShortOptionSecrets(sshpassSecrets),
		redactShortOptionSecrets(sshKeygenSecrets),
		redactNamedCommandArguments("zip", "P", zipCredentialFlagPattern),
		redactNamedCommandArguments("unzip", "P", zipCredentialFlagPattern),
		redactRedisCLICredentialArguments,
		redactCurlCredentialArguments,
		redactNetrcPasswords,
		redactWebAuthCodeQueryValues,
		redactEncodedQueryCredentialValues,
		redactPlistCredentialValues,
		redactXMLCredentialElements,
		redactXMLCredentialAttributes,
	}
	structuredDataRedactionSteps = []func(string) (string, bool){
		redactTOMLCredentialValues,
		redactTOMLMultilineCredentials,
		redactJSONEscapedCredentialValues,
		redactContextualJSONCredentialValues,
		redactSensitiveJSONNameValuePairs,
		redactSensitiveYAMLNameValuePairs,
		redactStructuredCookieValues,
		redactRegistryConfigurationAuthValues,
		redactStructuredUploadHeaderValues,
		redactStructuredCredentialObjects,
		redactYAMLCredentialAliases,
		redactYAMLExplicitCredentialMappings,
		redactYAMLCredentialBlocks,
		redactYAMLFlowCredentials,
		redactSensitiveCommandSubstitutions,
		redactBareEnvironmentDumpCredentials,
	}
	redactCommandScopedFlagValues = applyingRules(commandScopedSensitiveFlagRedactionRules)
	plainTextRedactionSteps       = []func(string) (string, bool){
		redactCommandScopedSensitiveFlags,
		applyingRules(singleLineShellWordRedactionRules),
		applyingRules(sensitiveTextRedactionRules),
		redactStandaloneURLSafeCredentials,
		redactHighConfidenceAuthorizationCredentials,
		redactStandaloneBearerCredentials,
	}
)

func applyingRules(rules []redactionRule) func(string) (string, bool) {
	return func(value string) (string, bool) {
		changed := false
		for _, rule := range rules {
			if next := rule.pattern.ReplaceAllString(value, rule.replacement); next != value {
				value = next
				changed = true
			}
		}
		return value, changed
	}
}

func runRedactionSteps(value string, changed bool, steps []func(string) (string, bool)) (string, bool) {
	for _, step := range steps {
		if next, stepChanged := step(value); stepChanged {
			value = next
			changed = true
		}
	}
	return value, changed
}

func redactSensitiveTextDepth(value string, depth int) (string, bool) {
	redacted, changed := boundRedactionInput(value)
	if depth > maxShellRedactionDepth {
		if redacted == redactionMarker {
			return redacted, changed
		}
		return redactionMarker, true
	}
	for _, redactNested := range [...]func(string, int) (string, bool){
		redactShellCommandSubstitutionContents,
		redactShellSubshellGroupContents,
		redactEnvSplitCommandStrings,
		redactShellCommandStrings,
		redactEvalCommandStrings,
		redactLaunchctlSubmitEmbeddedCommands,
	} {
		if next, stepChanged := redactNested(redacted, depth); stepChanged {
			redacted = next
			changed = true
		}
	}
	redacted, changed = runRedactionSteps(redacted, changed, commandArgumentRedactionSteps)
	redacted, yamlKeyRestorations := normalizeYAMLEscapedCredentialKeys(redacted)
	redacted, yamlAliasKeyRestorations := normalizeYAMLAliasCredentialKeys(redacted)
	redacted, changed = runRedactionSteps(redacted, changed, structuredDataRedactionSteps)
	redacted, booleanMarkerProtection := protectBooleanSecretMarkers(redacted)
	redacted, changed = runRedactionSteps(redacted, changed, plainTextRedactionSteps)
	if booleanMarkerProtection != "" {
		redacted = strings.ReplaceAll(redacted, booleanMarkerProtection, "")
	}
	for placeholder, original := range yamlKeyRestorations {
		redacted = strings.ReplaceAll(redacted, placeholder, original)
	}
	for placeholder, original := range yamlAliasKeyRestorations {
		redacted = strings.ReplaceAll(redacted, placeholder, original)
	}
	return redacted, changed
}

func redactBareEnvironmentDumpCredentials(value string) (string, bool) {
	matches := bareEnvironmentDumpCredential.FindAllStringSubmatchIndex(value, -1)
	if len(matches) == 0 {
		return value, false
	}

	var redacted strings.Builder
	redacted.Grow(len(value))
	last := 0
	changed := false
	for _, match := range matches {
		valueStart, valueEnd := match[4], match[5]
		words := strings.Fields(value[valueStart:valueEnd])
		// A leading assignment followed by asc is a shell command invocation,
		// not an environment dump. The shell-word pass below redacts only the
		// assignment value and preserves that useful command context.
		if len(words) > 1 && (words[1] == "asc" || shellAssignmentWord.MatchString(words[1])) {
			continue
		}

		redacted.WriteString(value[last:valueStart])
		redacted.WriteString(redactionMarker)
		last = valueEnd
		changed = true
	}
	if !changed {
		return value, false
	}
	redacted.WriteString(value[last:])
	return redacted.String(), true
}

// boundRedactionInput limits parser work before any redaction pass runs. It
// omits an oversized field in full so a multiline credential that crosses the
// byte boundary cannot leave part of its value in a report.
func boundRedactionInput(value string) (string, bool) {
	if len(value) <= maxRedactionFieldBytes {
		return value, false
	}
	return oversizedFieldMarker, true
}

// redactDelimitedValues replaces each value that starts at the first submatch
// of pattern and ends where end says, or at the end of value when end returns
// -1.
func redactDelimitedValues(value string, pattern *lazyRegexp, end func(value string, open int) int) (string, bool) {
	redacted := value
	changed := false
	for searchStart := 0; searchStart < len(redacted); {
		match := pattern.FindStringSubmatchIndex(redacted[searchStart:])
		if match == nil {
			break
		}

		open := searchStart + match[2]
		close := end(redacted, open)
		if close < 0 {
			close = len(redacted)
		}
		redacted = redacted[:open] + redactionMarker + redacted[close:]
		changed = true
		searchStart = open + len(redactionMarker)
	}
	return redacted, changed
}

func redactEncodedQueryCredentialValues(value string) (string, bool) {
	return redactQueryParameterValues(value, func(_, name string, _, _, _ int) bool {
		if !strings.Contains(name, "%") {
			return false
		}
		for decodeLayer := 0; decodeLayer < 2; decodeLayer++ {
			decoded, err := url.QueryUnescape(name)
			if err != nil {
				return false
			}
			name = decoded
			if queryCredentialNamePattern.MatchString(name) {
				return true
			}
		}
		return false
	})
}

// redactQueryParameterValues redacts each non-empty URL query parameter value
// that isSecret accepts, given the text, the parameter's raw name and offsets.
func redactQueryParameterValues(value string, isSecret func(text, name string, nameStart, valueStart, valueEnd int) bool) (string, bool) {
	redacted := value
	changed := false
	for searchStart := 0; searchStart < len(redacted); {
		match := queryParameterName.FindStringSubmatchIndex(redacted[searchStart:])
		if match == nil {
			break
		}

		nameStart := searchStart + match[2]
		valueStart := searchStart + match[1]
		name := redacted[nameStart : searchStart+match[3]]
		searchStart = valueStart
		valueEnd := valueStart
		for valueEnd < len(redacted) && !isQueryValueTerminator(redacted[valueEnd]) {
			valueEnd++
		}
		if valueEnd == valueStart || redacted[valueStart:valueEnd] == redactionMarker || !isSecret(redacted, name, nameStart, valueStart, valueEnd) {
			continue
		}

		redacted = redacted[:valueStart] + redactionMarker + redacted[valueEnd:]
		changed = true
		searchStart = valueStart + len(redactionMarker)
	}
	return redacted, changed
}

func redactWebAuthCodeQueryValues(value string) (string, bool) {
	return redactQueryParameterValues(value, func(text, name string, nameStart, valueStart, valueEnd int) bool {
		decodedName, err := url.QueryUnescape(name)
		return err == nil && strings.EqualFold(decodedName, "code") && hasWebAuthQueryContext(text, nameStart, text[valueStart:valueEnd])
	})
}

func hasWebAuthQueryContext(value string, parameterStart int, codeValue string) bool {
	queryStart := strings.LastIndex(value[:parameterStart], "?")
	if queryStart < 0 {
		return false
	}
	for cursor := queryStart + 1; cursor < parameterStart; cursor++ {
		if isURLTextBoundary(value[cursor]) {
			return false
		}
	}

	queryEnd := queryStart + 1
	for queryEnd < len(value) && !isURLTextBoundary(value[queryEnd]) {
		queryEnd++
	}
	for _, parameter := range strings.Split(value[queryStart+1:queryEnd], "&") {
		name, _, _ := strings.Cut(parameter, "=")
		decodedName, err := url.QueryUnescape(name)
		if err == nil && (strings.EqualFold(decodedName, "widgetkey") || strings.EqualFold(decodedName, "scnt")) {
			return true
		}
	}

	urlStart := queryStart
	for urlStart > 0 && !isURLTextBoundary(value[urlStart-1]) {
		urlStart--
	}
	parsed, err := url.Parse(value[urlStart:queryStart])
	if err != nil {
		return false
	}
	for _, part := range strings.FieldsFunc(strings.ToLower(parsed.Hostname()+parsed.EscapedPath()), func(character rune) bool {
		return character == '.' || character == '/'
	}) {
		switch part {
		case "auth", "oauth", "oauth2", "authorize", "authorization", "login", "signin", "sign-in":
			return looksLikeWebAuthCode(codeValue)
		}
	}
	return false
}

func looksLikeWebAuthCode(value string) bool {
	decoded, err := url.QueryUnescape(value)
	if err != nil || len(decoded) < 8 {
		return false
	}
	if len(decoded) >= 16 {
		return true
	}
	hasLetter := false
	hasDigit := false
	for _, character := range decoded {
		hasLetter = hasLetter || character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z'
		hasDigit = hasDigit || character >= '0' && character <= '9'
	}
	return hasLetter && hasDigit
}

func isURLTextBoundary(character byte) bool {
	switch character {
	case ' ', '\t', '\r', '\n', '\f', '\v', '"', '\'', '<', '>', '(', ')', '[', ']', '{', '}', '#', ';', '|', '`':
		return true
	default:
		return false
	}
}

func isQueryValueTerminator(character byte) bool {
	switch character {
	case '&', '#', ' ', '\t', '\r', '\n', '\f', '\v', '"', '\'', '<', '>':
		return true
	default:
		return false
	}
}

func redactHighConfidenceAuthorizationCredentials(value string) (string, bool) {
	redacted := value
	changed := false
	for searchStart := 0; searchStart < len(redacted); {
		match := authorizationHeaderValueStart.FindStringIndex(redacted[searchStart:])
		if match == nil {
			break
		}

		headerStart := searchStart + match[0]
		valueStart := searchStart + match[1]
		lineEnd := valueStart
		for lineEnd < len(redacted) && redacted[lineEnd] != '\r' && redacted[lineEnd] != '\n' {
			lineEnd++
		}
		line := redacted[valueStart:lineEnd]
		if strings.HasPrefix(line, redactionMarker) {
			searchStart = lineEnd
			continue
		}
		firstEnd := strings.IndexAny(line, " \t")
		if firstEnd < 0 {
			firstEnd = len(line)
		}
		scheme := line[:firstEnd]
		credential := scheme
		credentialEnd := firstEnd
		nextStart := firstEnd
		for nextStart < len(line) && (line[nextStart] == ' ' || line[nextStart] == '\t') {
			nextStart++
		}
		hasSeparateCredential := false
		if nextStart < len(line) {
			nextEnd := nextStart
			for nextEnd < len(line) && line[nextEnd] != ' ' && line[nextEnd] != '\t' {
				nextEnd++
			}
			credential = line[nextStart:nextEnd]
			credentialEnd = nextEnd
			hasSeparateCredential = strings.TrimSpace(line[nextEnd:]) == ""
		}

		if !hasSeparateCredential && !isCredentialSpecificAuthorizationScheme(scheme) && !looksLikeAuthorizationCredential(credential) {
			searchStart = lineEnd
			continue
		}

		redacted = redacted[:headerStart] + "Authorization: " + redactionMarker + redacted[valueStart+credentialEnd:]
		changed = true
		searchStart = headerStart + len("Authorization: ") + len(redactionMarker)
	}
	return redacted, changed
}

func isCredentialSpecificAuthorizationScheme(value string) bool {
	switch strings.ToLower(value) {
	case "apikey", "api-key":
		return true
	default:
		return false
	}
}

func looksLikeAuthorizationCredential(value string) bool {
	if len(value) < 8 || value == redactionMarker {
		return false
	}
	return strings.ContainsAny(value, "0123456789._~+/=-")
}

func redactStandaloneURLSafeCredentials(value string) (string, bool) {
	redacted := value
	changed := false
	for _, candidate := range standaloneURLSafeCredentialCandidates {
		matches := candidate.FindAllStringIndex(redacted, -1)
		for match := len(matches) - 1; match >= 0; match-- {
			start, end := matches[match][0], matches[match][1]
			if start > 0 && isURLSafeCredentialCharacter(redacted[start-1]) || end < len(redacted) && isURLSafeCredentialCharacter(redacted[end]) {
				continue
			}
			redacted = redacted[:start] + redactionMarker + redacted[end:]
			changed = true
		}
	}
	return redacted, changed
}

func isURLSafeCredentialCharacter(value byte) bool {
	return value >= 'a' && value <= 'z' || value >= 'A' && value <= 'Z' || value >= '0' && value <= '9' || value == '_' || value == '-'
}

func redactStandaloneBearerCredentials(value string) (string, bool) {
	redacted := value
	changed := false
	for searchStart := 0; searchStart < len(redacted); {
		match := standaloneBearerCandidate.FindStringSubmatchIndex(redacted[searchStart:])
		if match == nil {
			break
		}

		tokenStart := searchStart + match[2]
		tokenEnd := searchStart + match[3]
		token := redacted[tokenStart:tokenEnd]
		if len(token) < 8 || !strings.ContainsAny(token, "0123456789") || isBenignBearerProtocol(token) {
			searchStart = tokenEnd
			continue
		}

		redacted = redacted[:tokenStart] + redactionMarker + redacted[tokenEnd:]
		changed = true
		searchStart = tokenStart + len(redactionMarker)
	}
	return redacted, changed
}

func isBenignBearerProtocol(value string) bool {
	lower := strings.ToLower(value)
	switch lower {
	case "oauth2", "oauth2.0", "http2", "http2.0", "rfc6750":
		return true
	}
	if !strings.HasPrefix(lower, "oauth2.") {
		return false
	}
	suffix := strings.TrimPrefix(lower, "oauth2.")
	if suffix == "0-beta" {
		return true
	}
	for _, character := range suffix {
		if character < '0' || character > '9' {
			return false
		}
	}
	return suffix != ""
}

// exclusiveEnd turns the index of a closing delimiter into the end of the
// text it closes, keeping -1 for a missing delimiter.
func exclusiveEnd(close int) int {
	if close < 0 {
		return -1
	}
	return close + 1
}

func protectBooleanSecretMarkers(value string) (string, string) {
	if !booleanSecretMarker.MatchString(value) {
		return value, ""
	}

	protection := "\x00"
	for strings.Contains(value, protection) {
		protection += "\x00"
	}
	protected, changed := transformXcodeCloudEnvVarSetCommands(value, func(command string) (string, bool) {
		next := booleanSecretMarker.ReplaceAllString(command, `${1}${2}`+protection+`${3}${4}${5}`)
		return next, next != command
	})
	if !changed {
		return value, ""
	}
	return protected, protection
}

func redactSecretMarkedValues(value string) (string, bool) {
	return transformXcodeCloudEnvVarSetCommands(value, func(command string) (string, bool) {
		if secretMarkerPattern.MatchString(command) {
			redacted := secretValuePattern.ReplaceAllString(command, `${1}${2}`+redactionMarker)
			if redacted != command {
				return redacted, true
			}
		}
		return command, false
	})
}

// textReplacement replaces text[start:end]. Replacements are ordered and do not
// overlap.
type textReplacement struct {
	start, end int
	text       string
}

func applyTextReplacements(text string, replacements []textReplacement) string {
	for index := len(replacements) - 1; index >= 0; index-- {
		replacement := replacements[index]
		text = text[:replacement.start] + replacement.text + text[replacement.end:]
	}
	return text
}

func transformXcodeCloudEnvVarSetCommands(value string, transform func(string) (string, bool)) (string, bool) {
	result := value
	changed := false
	for searchStart := 0; searchStart < len(result); {
		match := xcodeCloudEnvVarSetCommand.FindStringIndex(result[searchStart:])
		if match == nil {
			break
		}

		start := searchStart + match[0]
		end := findShellCommandEnd(result, searchStart+match[1])
		command := result[start:end]
		next, commandChanged := transform(command)
		if commandChanged {
			result = result[:start] + next + result[end:]
			changed = true
		}
		searchStart = start + len(next)
	}
	return result, changed
}

func redactLogEntry(entry LogEntry) (LogEntry, bool) {
	changed := false
	redactField := func(field *string) {
		redacted, fieldChanged := redactSensitiveText(*field)
		*field = redacted
		changed = changed || fieldChanged
	}

	redactField(&entry.Description)
	redactField(&entry.Repro)
	redactField(&entry.Expected)
	redactField(&entry.Actual)
	redactField(&entry.Severity)
	redactField(&entry.ASCVersion)
	redactField(&entry.OS)
	var labelsChanged bool
	entry.Labels, labelsChanged = redactStringSlice(entry.Labels)
	changed = changed || labelsChanged

	return entry, changed
}

func redactStringSlice(values []string) ([]string, bool) {
	if len(values) == 0 {
		return values, false
	}

	redacted := make([]string, len(values))
	anyChanged := false
	for i, value := range values {
		redactedValue, valueChanged := redactSensitiveText(value)
		redacted[i] = redactedValue
		anyChanged = anyChanged || valueChanged
	}
	return redacted, anyChanged
}

func redactLogEntries(entries []LogEntry) ([]LogEntry, bool) {
	if len(entries) == 0 {
		return entries, false
	}

	redacted := make([]LogEntry, len(entries))
	changed := false
	for i, entry := range entries {
		var entryChanged bool
		redacted[i], entryChanged = redactLogEntry(entry)
		changed = changed || entryChanged
	}
	return redacted, changed
}
