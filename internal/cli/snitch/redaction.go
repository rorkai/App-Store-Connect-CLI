package snitch

import (
	"encoding/json"
	"encoding/xml"
	"errors"
	"io"
	"math"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
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
	opensslMACOptionValueStartPattern     = newCommandCredentialFlagValueStartPattern("macopt")
	opensslKDFOptionValueStartPattern     = newCommandCredentialFlagValueStartPattern("kdfopt")
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
	tomlMultilineCredentialStart          = lazyMustCompile(`(?i)(?:^|[^-a-z0-9_])(?:` + sensitivePrefixedName + `\b|` + tomlQuotedSensitiveKey + `)[ \t]*=[ \t]*(?:"""|''')`)
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
	{
		pattern:     lazyMustCompile(`(?i)("value"[ \t\r\n]*:[ \t\r\n]*")(?:\\.|[^"\\\r\n])*(")([ \t\r\n]*(?:[,}\]]|\z))`),
		replacement: `${1}` + redactionMarker + `${2}${3}`,
	},
	{
		pattern:     lazyMustCompile(`(?i)(\\"value\\"[ \t\r\n]*:[ \t\r\n]*\\")(?:\\.|[^"\\\r\n])*?(\\")([ \t\r\n]*(?:[,}\]]|\z))`),
		replacement: `${1}` + redactionMarker + `${2}${3}`,
	},
}

var registryAuthValueRedactionRules = []redactionRule{
	{
		pattern:     lazyMustCompile(`(?i)("auth"[ \t\r\n]*:[ \t\r\n]*")(?:\\.|[^"\\\r\n])*(")`),
		replacement: `${1}` + redactionMarker + `${2}`,
	},
	{
		pattern:     lazyMustCompile(`(?i)(\\"auth\\"[ \t\r\n]*:[ \t\r\n]*\\")(?:\\.|[^"\\\r\n])*?(\\")`),
		replacement: `${1}` + redactionMarker + `${2}`,
	},
}

var curlUserCredentialRedactionRules = []redactionRule{
	{
		pattern:     lazyMustCompile(`(?i)(^|\s)((?:-u|--(?:proxy-)?user)\b` + curlOptionValueSeparator + `)(?:\[REDACTED(?: PRIVATE KEY)?\]|` + credentialPairValue + `)`),
		replacement: `${1}${2}` + redactionMarker,
	},
	{
		pattern:     lazyMustCompile(`(?i)(^|\s)((?:-u|--(?:proxy-)?user)\b[ \t]*=[ \t]*)(?:\[REDACTED(?: PRIVATE KEY)?\]|` + credentialPairValue + `)`),
		replacement: `${1}${2}` + redactionMarker,
	},
	{
		pattern:     lazyMustCompile(`(?i)(^|\s)(-u)(?:\[REDACTED(?: PRIVATE KEY)?\]|` + credentialPairValue + `)`),
		replacement: `${1}${2}` + redactionMarker,
	},
}

var curlCertificateCredentialRedactionRules = []redactionRule{
	{
		pattern:     lazyMustCompile(`(?i)(^|\s)(` + curlCertOptionPrefix + `)(")((?:` + escapedQuotedCharacter + `|[^"\\:\r\n])*):(?:` + escapedQuotedCharacter + `|[^"\\])+(")`),
		replacement: `${1}${2}${3}${4}:` + redactionMarker + `${5}`,
	},
	{
		pattern:     lazyMustCompile(`(?i)(^|\s)(` + curlCertOptionPrefix + `)(')((?:` + escapedQuotedCharacter + `|[^'\\:\r\n])*):(?:` + escapedQuotedCharacter + `|[^'\\])+(')`),
		replacement: `${1}${2}${3}${4}:` + redactionMarker + `${5}`,
	},
	{
		pattern:     lazyMustCompile(`(?i)(^|\s)(` + curlCertOptionPrefix + `)(` + curlCertShellPath + `):` + singleLineShellWord),
		replacement: `${1}${2}${3}:` + redactionMarker,
	},
}

var curlArgumentCredentialRedactionRules = []redactionRule{
	{
		pattern:     lazyMustCompile(`(?i)(^|\s)(` + curlHeaderOptionPrefix + `)(` + credentialHeaderName + `)[ \t]*:[ \t]*(?:\\(?:\r?\n|[^\r\n])|[^\s;&|<>()])+`),
		replacement: `${1}${2}${3}:` + redactionMarker,
	},
	{
		pattern:     lazyMustCompile(`(?i)(^|\s)((?:(?-i:-b)|--cookie)\b[ \t]+)(?:\[REDACTED(?: PRIVATE KEY)?\]|` + cookieDataValue + `)`),
		replacement: `${1}${2}` + redactionMarker,
	},
	{
		pattern:     lazyMustCompile(`(?i)(^|\s)((?:(?-i:-b)|--cookie)\b[ \t]*=[ \t]*)(?:\[REDACTED(?: PRIVATE KEY)?\]|` + cookieDataValue + `)`),
		replacement: `${1}${2}` + redactionMarker,
	},
	{
		pattern:     lazyMustCompile(`(?i)(^|\s)((?-i:-b))(?:\[REDACTED(?: PRIVATE KEY)?\]|` + cookieDataValue + `)`),
		replacement: `${1}${2}` + redactionMarker,
	},
	{
		pattern:     lazyMustCompile(`(?i)(^|\s)(` + curlFormDataOptionPrefix + `)(")((?:\\.|[^"\\])*?` + sensitivePrefixedName + `\b[ \t]*=[ \t]*)(?:\\.|[^"\\])*(")`),
		replacement: `${1}${2}${3}${4}` + redactionMarker + `${5}`,
	},
	{
		pattern:     lazyMustCompile(`(?i)(^|\s)(` + curlFormDataOptionPrefix + `)(')([^']*?` + sensitivePrefixedName + `\b[ \t]*=[ \t]*)[^']*(')`),
		replacement: `${1}${2}${3}${4}` + redactionMarker + `${5}`,
	},
}

// Redact complete single-line shell words first so adjacent quoted and
// unquoted fragments cannot leak, and an unmatched quote in an earlier log
// line cannot claim a later command's opening quote as its closer.
var singleLineShellWordRedactionRules = []redactionRule{
	{
		pattern:     lazyMustCompile(`(?m)(^|[ \t;&|])(` + uppercaseSessionEnvironmentName + `[ \t]*=[ \t]*)` + singleLineShellWord + `(` + singleLineShellTerminator + `)`),
		replacement: `${1}${2}` + redactionMarker + `${3}`,
	},
	{
		pattern:     lazyMustCompile(`(?im)(^|[ \t;&|])(` + powerShellSensitiveVariable + `[ \t]*=[ \t]*)(?:&[ \t]+)?(?:[a-z0-9_.-]+\\)?ConvertTo-SecureString[ \t]+(?:` + powerShellSecureStringSwitch + `)*(?:-String(?:[ \t]+|[ \t]*:[ \t]*))?` + fishShellWord + `(` + singleLineShellTerminator + `)`),
		replacement: `${1}${2}` + redactionMarker + `${3}`,
	},
	{
		pattern:     lazyMustCompile(`(?im)(^|[;&|][ \t]*)([ \t]*set[ \t]+(?:(?:--|-[a-z]+|--[a-z][a-z-]*)[ \t]+)*` + sensitivePrefixedName + `\b[ \t]+)` + fishShellWord + `(?:[ \t]+` + fishShellWord + `)*`),
		replacement: `${1}${2}` + redactionMarker,
	},
	{
		pattern:     lazyMustCompile(`(?i)(^|\s)(` + sensitiveOrSecretShellToken + `[ \t]*=[ \t]*)` + fishShellWord + `(` + singleLineShellTerminator + `)`),
		replacement: `${1}${2}` + redactionMarker + `${3}`,
	},
	{
		pattern:     lazyMustCompile(`(?i)(^|[^-a-z0-9_])(` + sensitivePrefixedName + `\b[ \t]*=[ \t]*)` + singleLineShellWord + `(` + singleLineShellTerminator + `)`),
		replacement: `${1}${2}` + redactionMarker + `${3}`,
	},
}

var commandScopedSensitiveFlagRedactionRules = []redactionRule{
	{
		pattern:     lazyMustCompile(`(?i)(^|\s)(` + sensitiveShellFlagToken + shellCommandPathSeparator + `)` + fishShellWord + `(` + singleLineShellTerminator + `)`),
		replacement: `${1}${2}` + redactionMarker + `${3}`,
	},
	{
		pattern:     lazyMustCompile(`(?i)(^|\s)(` + sensitiveShellFlagToken + shellCommandPathSeparator + `)(?:\[REDACTED(?: PRIVATE KEY)?\]|` + escapeAwareQuotedValue + `|` + unterminatedQuotedValue + `|` + shellUnquotedValue + `)`),
		replacement: `${1}${2}` + redactionMarker,
	},
}

var sensitiveTextRedactionRules = []redactionRule{
	{
		pattern:     lazyMustCompile(`(?s)-----BEGIN[ \t]+(?:[A-Z0-9]+[ \t]+)*PRIVATE[ \t]+KEY(?:[ \t]+BLOCK)?-----.*?-----END[ \t]+(?:[A-Z0-9]+[ \t]+)*PRIVATE[ \t]+KEY(?:[ \t]+BLOCK)?-----`),
		replacement: privateKeyRedactionMarker,
	},
	{
		pattern:     lazyMustCompile(`(?s)-----BEGIN[ \t]+(?:[A-Z0-9]+[ \t]+)*PRIVATE[ \t]+KEY(?:[ \t]+BLOCK)?-----.*\z`),
		replacement: privateKeyRedactionMarker,
	},
	{
		pattern:     lazyMustCompile(`(?im)^([ \t]*(?:redirect_)?http_` + cgiCredentialHeaderName + `\b[ \t]*=[ \t]*)[^\r\n]*(\r?)$`),
		replacement: `${1}` + redactionMarker + `${2}`,
	},
	{
		pattern:     lazyMustCompile(`(?m)^(` + passwordFileHostField + `:(?:[0-9]+|\*):` + passwordFileField + `:` + passwordFileField + `:)` + passwordFileField + `(\r?)$`),
		replacement: `${1}` + redactionMarker + `${2}`,
	},
	{
		pattern:     lazyMustCompile(`(?im)^([ \t]*(?:-[ \t]+)?` + yamlMappingKeyProperties + yamlCredentialName + `[ \t]*:[ \t]*)(?:\[[^\]\r\n]*\]|\{[^}\r\n]*\})`),
		replacement: `${1}` + redactionMarker,
	},
	{
		pattern:     lazyMustCompile(`(?im)^([ \t]*(?:-[ \t]+)?` + yamlMappingKeyProperties + `["']` + yamlCredentialName + `["'][ \t]*:[ \t]*)(?:\[[ \t]*[^"'\]\r\n][^\]\r\n]*\]|\{[ \t]*[^"'}\r\n][^}\r\n]*\})`),
		replacement: `${1}` + redactionMarker,
	},
	{
		pattern:     lazyMustCompile(`(?im)^([ \t]*` + yamlMappingKeyProperties + `(?:["']?` + yamlCredentialName + `["']?)[ \t]*:[ \t]*)(?:[^"'[{\s\r\n][^\r\n]*)(\r?)$`),
		replacement: `${1}` + redactionMarker + `${2}`,
	},
	{
		pattern:     lazyMustCompile(`(?i)(["']` + structuredCredentialName + `["'][ \t\r\n]*:[ \t\r\n]*\[)[ \t\r\n]*(?:"(?:\\.|[^"\\\r\n])*"(?:[ \t\r\n]*,[ \t\r\n]*"(?:\\.|[^"\\\r\n])*")*)[ \t\r\n]*(\])`),
		replacement: `${1}"` + redactionMarker + `"${2}`,
	},
	{
		pattern:     lazyMustCompile(`(?i)(\\"` + structuredCredentialName + `\\"[ \t\r\n]*:[ \t\r\n]*\[)[ \t\r\n]*(?:\\"(?:\\.|[^"\\\r\n])*?\\"(?:[ \t\r\n]*,[ \t\r\n]*\\"(?:\\.|[^"\\\r\n])*?\\")*)[ \t\r\n]*(\])`),
		replacement: `${1}\"` + redactionMarker + `\"${2}`,
	},
	{
		pattern:     lazyMustCompile(`(?i)("securitycode"[ \t\r\n]*:[ \t\r\n]*\{[ \t\r\n]*"code"[ \t\r\n]*:[ \t\r\n]*")(?:\\.|[^"\\\r\n])*(")`),
		replacement: `${1}` + redactionMarker + `${2}`,
	},
	{
		pattern:     lazyMustCompile(`(?i)(\\"securitycode\\"[ \t\r\n]*:[ \t\r\n]*\{[ \t\r\n]*\\"code\\"[ \t\r\n]*:[ \t\r\n]*\\")(?:\\.|[^"\\\r\n])*?(\\")`),
		replacement: `${1}` + redactionMarker + `${2}`,
	},
	{
		pattern:     lazyMustCompile(`(?i)("authorization[ \t]*:[ \t]*)(?:` + escapedQuotedCharacter + `|[^"\\])*(")`),
		replacement: `${1}` + redactionMarker + `${2}`,
	},
	{
		pattern:     lazyMustCompile(`(?i)('authorization[ \t]*:[ \t]*)(?:` + escapedQuotedCharacter + `|[^'\\])*(')`),
		replacement: `${1}` + redactionMarker + `${2}`,
	},
	{
		pattern:     lazyMustCompile(`(?i)\bauthorization[ \t]*[:=][ \t]*(?:bearer|basic|token)[ \t]+(?:` + escapeAwareQuotedValue + `|` + unterminatedQuotedValue + `|[^\s,;"']+)` + foldedHeaderContinuation),
		replacement: "Authorization: " + redactionMarker,
	},
	{
		pattern:     lazyMustCompile(`(?i)\bauthorization[ \t]*[:=][ \t]*[a-z][a-z0-9_-]*[ \t]+[^\s=,]+[ \t]*=[^\r\n]+` + foldedHeaderContinuation),
		replacement: "Authorization: " + redactionMarker,
	},
	{
		pattern:     lazyMustCompile(`(?i)("` + traceCredentialHeader + `[ \t]*:[ \t]*)(?:` + escapedQuotedCharacter + `|[^"\\])*(")`),
		replacement: `${1}` + redactionMarker + `${2}`,
	},
	{
		pattern:     lazyMustCompile(`(?i)('` + traceCredentialHeader + `[ \t]*:[ \t]*)(?:` + escapedQuotedCharacter + `|[^'\\])*(')`),
		replacement: `${1}` + redactionMarker + `${2}`,
	},
	{
		pattern:     lazyMustCompile(`(?i)\b(` + credentialHeaderName + `|` + sensitivePrefixedName + `)[ \t]*:\[[^\]\r\n]*\]`),
		replacement: `${1}:` + redactionMarker,
	},
	{
		pattern:     lazyMustCompile(`(?im)^([ \t]*(?:(?:[<>][ \t]*)?` + traceCredentialHeader + `|[<>][ \t]*` + sensitivePrefixedName + `))[ \t]*:[ \t]*[^\r\n]+` + foldedHeaderContinuation),
		replacement: `${1}: ` + redactionMarker,
	},
	{
		pattern:     lazyMustCompile(`(?i)(^|[\s"'(<>{}\[\],;])((?:cookie|set-cookie)[ \t]*:[ \t]*)(?:` + cookieDataQuoted + `|` + cookieDataUnquoted + `)(?:[ \t]*;[ \t]*` + cookieDataUnquoted + `)*` + foldedHeaderContinuation),
		replacement: `${1}${2}` + redactionMarker,
	},
	{
		pattern:     lazyMustCompile(`(?i)(^|[\s"'(<>{}\[\],;])((?:` + traceCredentialHeader + `)[ \t]*:[ \t]*)(?:\[REDACTED(?: PRIVATE KEY)?\]|` + escapeAwareQuotedValue + `|` + unterminatedQuotedValue + `|[^\s,;"'()<>{}\[\]]+)` + foldedHeaderContinuation),
		replacement: `${1}${2}` + redactionMarker,
	},
	{
		pattern:     lazyMustCompile(`(?i)\b([a-z][a-z0-9+.-]*://)[^/?#\s@]+@`),
		replacement: `${1}` + redactionMarker + `@`,
	},
	{
		pattern:     lazyMustCompile(`(?i)(^|[\s"'=])[^/?#\s@:]+:[^/?#\s@]+@([a-z0-9.-]+:[^\s"'<>]+)`),
		replacement: `${1}` + redactionMarker + `@${2}`,
	},
	{
		pattern:     lazyMustCompile(`(?i)\b(https://hooks\.slack(?:-gov)?\.com/services/)[^?#\s"'()<>{}\[\],;]+`),
		replacement: `${1}` + redactionMarker,
	},
	{
		pattern:     lazyMustCompile(`(?i)\b(https://(?:(?:canary|ptb)\.)?discord(?:app)?\.com/api(?:/v[0-9]+)?/webhooks/[0-9]+/)[^?#\s"'()<>{}\[\],;]+`),
		replacement: `${1}` + redactionMarker,
	},
	{
		pattern:     lazyMustCompile(`(?i)([?&]` + queryCredentialName + `=)[^&#\s"'<>]+`),
		replacement: `${1}` + redactionMarker,
	},
	{
		pattern:     lazyMustCompile(`(?im)^([ \t]*(?:header|proxy-header)` + curlConfigSeparator + `")(` + credentialHeaderName + `|` + sensitivePrefixedName + `)([ \t]*:[ \t]*)(?:\\.|[^"\\\r\n])+(")`),
		replacement: `${1}${2}${3}` + redactionMarker + `${4}`,
	},
	{
		pattern:     lazyMustCompile(`(?im)^([ \t]*(?:header|proxy-header)` + curlConfigSeparator + `)(` + credentialHeaderName + `|` + sensitivePrefixedName + `)([ \t]*:[ \t]*)[^\r\n]+$`),
		replacement: `${1}${2}${3}` + redactionMarker,
	},
	{
		pattern:     lazyMustCompile(`(?im)^([ \t]*(?:user|proxy-user)` + curlConfigSeparator + `)(?:\[REDACTED(?: PRIVATE KEY)?\]|` + credentialPairValue + `)`),
		replacement: `${1}` + redactionMarker,
	},
	{
		pattern:     lazyMustCompile(`(?im)^([ \t]*(?:oauth2-bearer|proxy-tlspassword|tlspassword)` + curlConfigSeparator + `)(?:\[REDACTED(?: PRIVATE KEY)?\]|` + escapeAwareQuotedValue + `|` + unterminatedQuotedValue + `|[^\s#]+)`),
		replacement: `${1}` + redactionMarker,
	},
	{
		pattern:     lazyMustCompile(`(?im)^([ \t]*pass` + curlConfigExplicitSeparator + `)(?:\[REDACTED(?: PRIVATE KEY)?\]|` + escapeAwareQuotedValue + `|` + unterminatedQuotedValue + `|[^\s#]+)`),
		replacement: `${1}` + redactionMarker,
	},
	{
		pattern:     lazyMustCompile(`(?im)^([ \t]*cookie` + curlConfigSeparator + `)(?:\[REDACTED(?: PRIVATE KEY)?\]|` + cookieDataValue + `)`),
		replacement: `${1}` + redactionMarker,
	},
	{
		pattern:     lazyMustCompile(`(?im)^((?:#HttpOnly_)?[^#\t\r\n][^\t\r\n]*\t(?:TRUE|FALSE)\t[^\t\r\n]*\t(?:TRUE|FALSE)\t[0-9]+\t[^\t\r\n]+\t)(?:\[REDACTED(?: PRIVATE KEY)?\]|[^\t\r\n]+)`),
		replacement: `${1}` + redactionMarker,
	},
	{
		pattern:     lazyMustCompile(`(?i)(^|\s)(-{1,2}secret\b` + shellCommandPathSeparator + `)(?:\[REDACTED(?: PRIVATE KEY)?\]|` + escapeAwareQuotedValue + `|` + unterminatedQuotedValue + `|` + flagUnquotedValue + `)`),
		replacement: `${1}${2}` + redactionMarker,
	},
	{
		pattern:     lazyMustCompile(`(?i)(^|\s)(` + sensitiveOrSecretShellToken + `[ \t]*=[ \t]*)(?:\[REDACTED(?: PRIVATE KEY)?\]|` + escapeAwareQuotedValue + `|` + unterminatedQuotedValue + `|` + shellUnquotedValue + `)`),
		replacement: `${1}${2}` + redactionMarker,
	},
	{
		pattern:     lazyMustCompile(`(?i)(\\"` + structuredCredentialName + `\\"[ \t\r\n]*:[ \t\r\n]*\\")(?:\\.|[^"\\\r\n])*?(\\")([ \t\r\n]*(?:[,}\]]|\z))`),
		replacement: `${1}` + redactionMarker + `${2}${3}`,
	},
	{
		pattern:     lazyMustCompile(`(?i)(["']` + structuredCredentialName + `["'][ \t\r\n]*:[ \t\r\n]*)(?:` + escapeAwareQuotedValue + `|` + unterminatedQuotedValue + `|[^\s,;}\[\]]+)`),
		replacement: `${1}"` + redactionMarker + `"`,
	},
	{
		pattern:     lazyMustCompile(`(?i)(^|[^-a-z0-9_])(` + tomlQuotedSensitiveKey + `[ \t]*=[ \t]*)(?:\[REDACTED(?: PRIVATE KEY)?\]|` + escapeAwareQuotedValue + `|` + unterminatedQuotedValue + `|` + shellUnquotedValue + `)`),
		replacement: `${1}${2}` + redactionMarker,
	},
	{
		pattern:     lazyMustCompile(`(?i)(^|[^-a-z0-9_])(` + sensitivePrefixedName + `\b[ \t]*=[ \t]*)(?:\[REDACTED(?: PRIVATE KEY)?\]|(?:(?:bearer|basic|token)[ \t]+)` + shellUnquotedValue + `|` + escapeAwareQuotedValue + `|` + unterminatedQuotedValue + `|` + shellUnquotedValue + `)`),
		replacement: `${1}${2}` + redactionMarker,
	},
	{
		pattern:     lazyMustCompile(`(?im)(^[ \t]*|[\[{(,;][ \t]*)(` + yamlMappingKeyProperties + sensitivePrefixedName + `\b[ \t]*:[ \t]*)(?:\[REDACTED(?: PRIVATE KEY)?\]|(?:(?:bearer|basic|token)[ \t]+)` + structuredUnquotedValue + `|` + escapeAwareQuotedValue + `|` + unterminatedQuotedValue + `|` + structuredUnquotedValue + `)`),
		replacement: `${1}${2}` + redactionMarker,
	},
	{
		pattern:     lazyMustCompile(`\beyJ[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\b`),
		replacement: redactionMarker,
	},
	{
		pattern:     lazyMustCompile(`\b(?:github_pat_[A-Za-z0-9_]{20,}|gh[pousr]_[A-Za-z0-9]{20,}|xoxb-[0-9]{10,}-[0-9]{10,}-[A-Za-z0-9]{20,}|xoxp-[A-Za-z0-9-]{20,}|xapp-[0-9]+-[A-Za-z0-9]{10,}(?:-[A-Za-z0-9]{6,})+|npm_[A-Za-z0-9]{36}|glpat-[A-Za-z0-9_-]{20,}|[sr]k_(?:live|test)_[A-Za-z0-9]{16,})\b`),
		replacement: redactionMarker,
	},
}

func redactSensitiveText(value string) (string, bool) {
	return redactSensitiveTextDepth(value, 0)
}

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
			redacted := command
			for _, rule := range commandScopedSensitiveFlagRedactionRules {
				next := rule.pattern.ReplaceAllString(redacted, rule.replacement)
				if next != redacted {
					redacted = next
					changed = true
				}
			}
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
	spans := splitCredentialShellWordSpans(command)
	if len(spans) == 0 {
		return false
	}
	words := make([]string, 0, len(spans))
	for _, span := range spans {
		words = append(words, span.value)
	}
	for index, word := range words {
		if !isCredentialCommandPrefix(words[:index]) {
			continue
		}
		switch commandBaseName(word) {
		case "echo", "printf", "grep", "rg", "ripgrep":
			return true
		}
	}
	return false
}

func redactSensitiveTextDepth(value string, depth int) (string, bool) {
	redacted, changed := boundRedactionInput(value)
	if depth > maxShellRedactionDepth {
		if redacted == redactionMarker {
			return redacted, changed
		}
		return redactionMarker, true
	}
	if next, substitutionChanged := redactShellCommandSubstitutionContents(redacted, depth); substitutionChanged {
		redacted = next
		changed = true
	}
	if next, subshellChanged := redactShellSubshellGroupContents(redacted, depth); subshellChanged {
		redacted = next
		changed = true
	}
	if next, envSplitChanged := redactEnvSplitCommandStrings(redacted, depth); envSplitChanged {
		redacted = next
		changed = true
	}
	if next, shellChanged := redactShellCommandStrings(redacted, depth); shellChanged {
		redacted = next
		changed = true
	}
	if next, evalChanged := redactEvalCommandStrings(redacted, depth); evalChanged {
		redacted = next
		changed = true
	}
	if next, launchctlChanged := redactLaunchctlSubmitEmbeddedCommands(redacted, depth); launchctlChanged {
		redacted = next
		changed = true
	}
	if next, kubernetesChanged := redactKubernetesSecretData(redacted); kubernetesChanged {
		redacted = next
		changed = true
	}
	if next, powerShellChanged := redactPowerShellHereStringCredentials(redacted); powerShellChanged {
		redacted = next
		changed = true
	}
	if next, powerShellChanged := redactPowerShellCollectionCredentials(redacted); powerShellChanged {
		redacted = next
		changed = true
	}
	if next, commandPromptChanged := redactCommandPromptSetAssignments(redacted); commandPromptChanged {
		redacted = next
		changed = true
	}
	if next, curlConfigChanged := redactCurlConfigCertificatePasswords(redacted); curlConfigChanged {
		redacted = next
		changed = true
	}
	if next, secretChanged := redactSecretMarkedValues(redacted); secretChanged {
		redacted = next
		changed = true
	}
	if next, kubectlChanged := redactKubectlSecretLiterals(redacted); kubectlChanged {
		redacted = next
		changed = true
	}
	if next, securityChanged := redactSecurityCredentialArguments(redacted); securityChanged {
		redacted = next
		changed = true
	}
	if next, opensslChanged := redactOpenSSLCredentialArguments(redacted); opensslChanged {
		redacted = next
		changed = true
	}
	if next, keytoolChanged := redactKeytoolCredentialArguments(redacted); keytoolChanged {
		redacted = next
		changed = true
	}
	if next, jarsignerChanged := redactJarsignerCredentialArguments(redacted); jarsignerChanged {
		redacted = next
		changed = true
	}
	if next, dockerChanged := redactDockerLoginCredentialArguments(redacted); dockerChanged {
		redacted = next
		changed = true
	}
	if next, sshpassChanged := redactSSHPassCredentialArguments(redacted); sshpassChanged {
		redacted = next
		changed = true
	}
	if next, sshKeygenChanged := redactSSHKeygenCredentialArguments(redacted); sshKeygenChanged {
		redacted = next
		changed = true
	}
	if next, zipChanged := redactZipCredentialArguments(redacted); zipChanged {
		redacted = next
		changed = true
	}
	if next, unzipChanged := redactUnzipCredentialArguments(redacted); unzipChanged {
		redacted = next
		changed = true
	}
	if next, redisChanged := redactRedisCLICredentialArguments(redacted); redisChanged {
		redacted = next
		changed = true
	}
	if next, curlChanged := redactCurlCredentialArguments(redacted); curlChanged {
		redacted = next
		changed = true
	}
	if next, netrcChanged := redactNetrcPasswords(redacted); netrcChanged {
		redacted = next
		changed = true
	}
	if next, queryChanged := redactWebAuthCodeQueryValues(redacted); queryChanged {
		redacted = next
		changed = true
	}
	if next, queryChanged := redactEncodedQueryCredentialValues(redacted); queryChanged {
		redacted = next
		changed = true
	}
	if next, plistChanged := redactPlistCredentialValues(redacted); plistChanged {
		redacted = next
		changed = true
	}
	if next, xmlChanged := redactXMLCredentialElements(redacted); xmlChanged {
		redacted = next
		changed = true
	}
	if next, xmlChanged := redactXMLCredentialAttributes(redacted); xmlChanged {
		redacted = next
		changed = true
	}
	redacted, yamlKeyRestorations := normalizeYAMLEscapedCredentialKeys(redacted)
	redacted, yamlAliasKeyRestorations := normalizeYAMLAliasCredentialKeys(redacted)
	if next, tomlValueChanged := redactTOMLCredentialValues(redacted); tomlValueChanged {
		redacted = next
		changed = true
	}
	if next, tomlChanged := redactTOMLMultilineCredentials(redacted); tomlChanged {
		redacted = next
		changed = true
	}
	if next, jsonKeyChanged := redactJSONEscapedCredentialValues(redacted); jsonKeyChanged {
		redacted = next
		changed = true
	}
	if next, jsonContextChanged := redactContextualJSONCredentialValues(redacted); jsonContextChanged {
		redacted = next
		changed = true
	}
	if next, jsonPairChanged := redactSensitiveJSONNameValuePairs(redacted); jsonPairChanged {
		redacted = next
		changed = true
	}
	if next, yamlPairChanged := redactSensitiveYAMLNameValuePairs(redacted); yamlPairChanged {
		redacted = next
		changed = true
	}
	if next, cookieChanged := redactStructuredCookieValues(redacted); cookieChanged {
		redacted = next
		changed = true
	}
	if next, registryChanged := redactRegistryConfigurationAuthValues(redacted); registryChanged {
		redacted = next
		changed = true
	}
	if next, headerChanged := redactStructuredUploadHeaderValues(redacted); headerChanged {
		redacted = next
		changed = true
	}
	if next, objectChanged := redactStructuredCredentialObjects(redacted); objectChanged {
		redacted = next
		changed = true
	}
	if next, yamlAliasChanged := redactYAMLCredentialAliases(redacted); yamlAliasChanged {
		redacted = next
		changed = true
	}
	if next, yamlExplicitChanged := redactYAMLExplicitCredentialMappings(redacted); yamlExplicitChanged {
		redacted = next
		changed = true
	}
	if next, yamlChanged := redactYAMLCredentialBlocks(redacted); yamlChanged {
		redacted = next
		changed = true
	}
	if next, yamlFlowChanged := redactYAMLFlowCredentials(redacted); yamlFlowChanged {
		redacted = next
		changed = true
	}
	if next, substitutionChanged := redactSensitiveCommandSubstitutions(redacted); substitutionChanged {
		redacted = next
		changed = true
	}
	if next, environmentChanged := redactBareEnvironmentDumpCredentials(redacted); environmentChanged {
		redacted = next
		changed = true
	}
	redacted, booleanMarkerProtection := protectBooleanSecretMarkers(redacted)
	if next, flagChanged := redactCommandScopedSensitiveFlags(redacted); flagChanged {
		redacted = next
		changed = true
	}
	for _, rule := range singleLineShellWordRedactionRules {
		next := rule.pattern.ReplaceAllString(redacted, rule.replacement)
		if next != redacted {
			changed = true
			redacted = next
		}
	}
	for _, rule := range sensitiveTextRedactionRules {
		next := rule.pattern.ReplaceAllString(redacted, rule.replacement)
		if next != redacted {
			changed = true
			redacted = next
		}
	}
	if next, standaloneChanged := redactStandaloneURLSafeCredentials(redacted); standaloneChanged {
		redacted = next
		changed = true
	}
	if next, authorizationChanged := redactHighConfidenceAuthorizationCredentials(redacted); authorizationChanged {
		redacted = next
		changed = true
	}
	if next, bearerChanged := redactStandaloneBearerCredentials(redacted); bearerChanged {
		redacted = next
		changed = true
	}
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

func redactKubernetesSecretData(value string) (string, bool) {
	redacted, changed := redactKubernetesSecretYAMLData(value)
	maxEscapeDepth := maxJSONEscapeDepthForLength(len(redacted))
	for escapeDepth := 0; escapeDepth <= maxEscapeDepth; escapeDepth++ {
		if next, jsonChanged := redactKubernetesSecretJSONData(redacted, escapeDepth); jsonChanged {
			redacted = next
			changed = true
		}
	}
	return redacted, changed
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
						} else if redactKubernetesYAMLScalarContinuations(lines, line, content, valueStart) {
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
			kind := yamlMappingScalar(lines, line, content, valueStart)
			if strings.HasPrefix(kind, "*") {
				kind = scalarAnchors[strings.TrimPrefix(kind, "*")]
			}
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
	start := currentLine
	for start > 0 && yamlLineBelongsToMapping(lines[start-1], mappingIndent) {
		start--
	}
	end := currentLine + 1
	for end < len(lines) && yamlLineBelongsToMapping(lines[end], mappingIndent) {
		end++
	}
	for line := start; line < end; line++ {
		content, _ := splitLineEnding(lines[line])
		key, valueStart, ok := parseYAMLMappingLine(content)
		if !ok || yamlKeyIndent(content) != mappingIndent || !strings.EqualFold(key, "kind") {
			continue
		}
		kind := yamlMappingScalar(lines, line, content, valueStart)
		if strings.HasPrefix(kind, "*") {
			kind = scalarAnchors[strings.TrimPrefix(kind, "*")]
		}
		return kind, true
	}
	return "", false
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
		kind := yamlMappingScalar(lines, line, candidate, valueStart)
		if strings.HasPrefix(kind, "*") {
			kind = scalarAnchors[strings.TrimPrefix(kind, "*")]
		}
		return strings.EqualFold(kind, "Secret")
	}
	return false
}

func yamlFlowMappingHasSecretKind(flow string, scalarAnchors map[string]string) bool {
	close := findYAMLFlowEnd(flow, 0)
	if len(flow) < 2 || flow[0] != '{' || close < 1 {
		return false
	}
	flow = flow[:close+1]
	for cursor := 1; cursor < len(flow)-1; {
		cursor = skipYAMLFlowSpaceAndComments(flow, cursor, len(flow)-1)
		if cursor >= len(flow)-1 {
			break
		}
		colon := findKubernetesFlowColon(flow, cursor)
		if colon < 0 {
			return false
		}
		key := decodeYAMLMappingKey(flow[cursor:colon])
		valueStart := colon + 1
		for valueStart < len(flow)-1 && (flow[valueStart] == ' ' || flow[valueStart] == '\t' || flow[valueStart] == '\r' || flow[valueStart] == '\n') {
			valueStart++
		}
		valueEnd := findKubernetesFlowValueEnd(flow, valueStart)
		if valueEnd < valueStart {
			return false
		}
		if strings.EqualFold(key, "kind") {
			kind := kubernetesYAMLScalar(flow[valueStart:valueEnd])
			if strings.HasPrefix(kind, "*") {
				kind = scalarAnchors[strings.TrimPrefix(kind, "*")]
			}
			return strings.EqualFold(kind, "Secret")
		}
		cursor = valueEnd
	}
	return false
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
	redactKubernetesYAMLScalarContinuations(lines, location.line, content, cursor)
	lines[location.line] = content[:cursor] + redactionMarker + ending
}

func redactKubernetesYAMLScalarContinuations(lines []string, line int, content string, valueStart int) bool {
	value, _ := trimYAMLNodeProperties(content[valueStart:])
	value = strings.TrimSpace(value)
	if value == "" || strings.HasPrefix(value, "[") || strings.HasPrefix(value, "{") {
		return false
	}

	indent := leadingIndent(content)
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
			kind := yamlMappingScalar(lines, line, content, valueStart)
			if strings.HasPrefix(kind, "*") {
				kind = scalarAnchors[strings.TrimPrefix(kind, "*")]
			}
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
	type property struct {
		key             string
		valueStart      int
		trimmedValueEnd int
	}
	var properties []property
	for cursor := 1; cursor < len(flow)-1; {
		cursor = skipYAMLFlowSpaceAndComments(flow, cursor, len(flow)-1)
		if cursor >= len(flow)-1 {
			break
		}
		colon := findKubernetesFlowColon(flow, cursor)
		if colon < 0 {
			return flow, false
		}
		key := decodeYAMLMappingKey(flow[cursor:colon])
		valueStart := colon + 1
		for valueStart < len(flow)-1 && (flow[valueStart] == ' ' || flow[valueStart] == '\t' || flow[valueStart] == '\r' || flow[valueStart] == '\n') {
			valueStart++
		}
		valueEnd := findKubernetesFlowValueEnd(flow, valueStart)
		if valueEnd < valueStart {
			return flow, false
		}
		trimmedEnd := valueEnd
		for trimmedEnd > valueStart && (flow[trimmedEnd-1] == ' ' || flow[trimmedEnd-1] == '\t' || flow[trimmedEnd-1] == '\r' || flow[trimmedEnd-1] == '\n') {
			trimmedEnd--
		}
		properties = append(properties, property{key: key, valueStart: valueStart, trimmedValueEnd: trimmedEnd})
		cursor = valueEnd
	}
	secret := false
	for _, property := range properties {
		if strings.EqualFold(property.key, "kind") && strings.EqualFold(kubernetesYAMLScalar(flow[property.valueStart:property.trimmedValueEnd]), "Secret") {
			secret = true
			break
		}
	}
	if !secret {
		return flow, false
	}
	type replacement struct {
		start, end int
		value      string
	}
	var replacements []replacement
	for _, property := range properties {
		if !strings.EqualFold(property.key, "data") && !strings.EqualFold(property.key, "stringData") || property.valueStart >= property.trimmedValueEnd {
			continue
		}
		if flow[property.valueStart] == '{' {
			child, childChanged := redactKubernetesSecretFlowMap(flow[property.valueStart:property.trimmedValueEnd])
			if childChanged {
				replacements = append(replacements, replacement{start: property.valueStart, end: property.trimmedValueEnd, value: child})
			}
			continue
		}
		if flow[property.valueStart:property.trimmedValueEnd] != redactionMarker {
			replacements = append(replacements, replacement{start: property.valueStart, end: property.trimmedValueEnd, value: redactionMarker})
		}
	}
	if len(replacements) == 0 {
		return flow, false
	}
	redacted := flow
	for index := len(replacements) - 1; index >= 0; index-- {
		replacement := replacements[index]
		redacted = redacted[:replacement.start] + replacement.value + redacted[replacement.end:]
	}
	return redacted, true
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

func findKubernetesFlowColon(value string, start int) int {
	depth := 0
	var quote byte
	for index := start; index < len(value); index++ {
		if quote != 0 {
			if quote == '"' && value[index] == '\\' {
				index++
				continue
			}
			if quote == '\'' && value[index] == '\'' && index+1 < len(value) && value[index+1] == '\'' {
				index++
				continue
			}
			if value[index] == quote {
				quote = 0
			}
			continue
		}
		switch value[index] {
		case '"', '\'':
			quote = value[index]
		case '[', '{':
			depth++
		case ']', '}':
			if depth == 0 {
				return -1
			}
			depth--
		case ':':
			if depth == 0 {
				return index
			}
		}
	}
	return -1
}

func findKubernetesFlowValueEnd(value string, start int) int {
	depth := 0
	var quote byte
	for index := start; index < len(value); index++ {
		if quote != 0 {
			if quote == '"' && value[index] == '\\' {
				index++
				continue
			}
			if quote == '\'' && value[index] == '\'' && index+1 < len(value) && value[index+1] == '\'' {
				index++
				continue
			}
			if value[index] == quote {
				quote = 0
			}
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
		case ',':
			if depth == 0 {
				return index
			}
		}
	}
	return len(value) - 1
}

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
	redacted := value
	changed := false
	for searchStart := 0; searchStart < len(redacted); {
		open := nextJSONObjectStart(redacted, searchStart, escapeDepth)
		if open < 0 {
			break
		}
		close := findJSONContainerEndAtDepthStrict(redacted, open, escapeDepth)
		truncated := close < open
		if truncated {
			// A malformed outer object used to make every nested opening brace
			// rescan the remaining suffix. Keep the existing truncated-object
			// redaction behavior, but stop after the first unmatched container.
			close = len(redacted) - 1
		}
		if close < open {
			break
		}
		object := redacted[open : close+1]
		redactedObject, objectChanged := redactKubernetesSecretJSONObject(object, escapeDepth)
		if objectChanged {
			redacted = redacted[:open] + redactedObject + redacted[close+1:]
			changed = true
		}
		if truncated {
			break
		}
		searchStart = open + 1
	}
	return redacted, changed
}

func containsJSONKeyAtDepth(value, key string, escapeDepth int) bool {
	delimiter := jsonQuoteDelimiter(escapeDepth)
	for searchStart := 0; searchStart < len(value); {
		relative := strings.Index(value[searchStart:], delimiter)
		if relative < 0 {
			return false
		}
		open := searchStart + relative
		quote := open + len(delimiter) - 1
		if !isJSONStringDelimiterAtDepth(value, quote, escapeDepth) {
			searchStart = open + len(delimiter)
			continue
		}
		close := findJSONStringEndAtDepth(value, open, escapeDepth)
		if close < 0 {
			return false
		}
		cursor := close + 1
		for cursor < len(value) && (value[cursor] == ' ' || value[cursor] == '\t' || value[cursor] == '\r' || value[cursor] == '\n') {
			cursor++
		}
		decoded, valid := decodeJSONCredentialKey(value[open:close+1], escapeDepth)
		if valid && cursor < len(value) && value[cursor] == ':' && strings.EqualFold(decoded, key) {
			return true
		}
		searchStart = close + 1
	}
	return false
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
	if len(properties) == 0 {
		return object, false
	}
	isSecret := false
	for _, property := range properties {
		if !strings.EqualFold(property.key, "kind") || property.valueStart >= len(object) || !strings.HasPrefix(object[property.valueStart:], jsonQuoteDelimiter(escapeDepth)) {
			continue
		}
		kind, valid := decodeJSONCredentialKey(object[property.valueStart:property.valueEnd], escapeDepth)
		if valid && strings.EqualFold(kind, "Secret") {
			isSecret = true
			break
		}
	}
	if !isSecret {
		return object, false
	}

	type replacement struct{ start, end int }
	var replacements []replacement
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
			replacements = append(replacements, replacement{
				start: property.valueStart + child.valueStart,
				end:   property.valueStart + child.valueEnd,
			})
		}
	}
	if len(replacements) == 0 {
		return object, false
	}
	redacted := object
	for index := len(replacements) - 1; index >= 0; index-- {
		span := replacements[index]
		redacted = redacted[:span.start] + quotedMarker + redacted[span.end:]
	}
	return redacted, true
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
		valueStart := keyEnd + 1
		for valueStart < len(object) && (object[valueStart] == ' ' || object[valueStart] == '\t' || object[valueStart] == '\r' || object[valueStart] == '\n') {
			valueStart++
		}
		if valueStart >= len(object) || object[valueStart] != ':' {
			return properties
		}
		valueStart++
		for valueStart < len(object) && (object[valueStart] == ' ' || object[valueStart] == '\t' || object[valueStart] == '\r' || object[valueStart] == '\n') {
			valueStart++
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

func redactPowerShellHereStringCredentials(value string) (string, bool) {
	redacted := value
	changed := false
	for searchStart := 0; searchStart < len(redacted); {
		match := powerShellHereStringCredential.FindStringSubmatchIndex(redacted[searchStart:])
		if match == nil {
			break
		}

		open := searchStart + match[2]
		contentStart := searchStart + match[3]
		quote := redacted[open+1]
		end := findPowerShellHereStringEnd(redacted, contentStart, quote)
		if end < 0 {
			end = len(redacted)
		}
		redacted = redacted[:open] + redactionMarker + redacted[end:]
		changed = true
		searchStart = open + len(redactionMarker)
	}
	return redacted, changed
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
	redacted := value
	changed := false
	for searchStart := 0; searchStart < len(redacted); {
		match := powerShellCollectionCredential.FindStringSubmatchIndex(redacted[searchStart:])
		if match == nil {
			break
		}

		open := searchStart + match[2]
		end := findPowerShellCollectionEnd(redacted, open)
		if end < 0 {
			end = len(redacted)
		}
		redacted = redacted[:open] + redactionMarker + redacted[end:]
		changed = true
		searchStart = open + len(redactionMarker)
	}
	return redacted, changed
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

func redactEncodedQueryCredentialValues(value string) (string, bool) {
	redacted := value
	changed := false
	for searchStart := 0; searchStart < len(redacted); {
		match := queryParameterName.FindStringSubmatchIndex(redacted[searchStart:])
		if match == nil {
			break
		}

		nameStart := searchStart + match[2]
		nameEnd := searchStart + match[3]
		valueStart := searchStart + match[1]
		searchStart = valueStart
		encodedName := redacted[nameStart:nameEnd]
		if !strings.Contains(encodedName, "%") {
			continue
		}
		decodedName := encodedName
		matchedCredentialName := false
		for decodeLayer := 0; decodeLayer < 2; decodeLayer++ {
			decoded, err := url.QueryUnescape(decodedName)
			if err != nil {
				break
			}
			decodedName = decoded
			if queryCredentialNamePattern.MatchString(decodedName) {
				matchedCredentialName = true
				break
			}
		}
		if !matchedCredentialName {
			continue
		}

		valueEnd := valueStart
		for valueEnd < len(redacted) && !isQueryValueTerminator(redacted[valueEnd]) {
			valueEnd++
		}
		if valueEnd == valueStart || redacted[valueStart:valueEnd] == redactionMarker {
			continue
		}

		redacted = redacted[:valueStart] + redactionMarker + redacted[valueEnd:]
		changed = true
		searchStart = valueStart + len(redactionMarker)
	}
	return redacted, changed
}

func redactWebAuthCodeQueryValues(value string) (string, bool) {
	redacted := value
	changed := false
	for searchStart := 0; searchStart < len(redacted); {
		match := queryParameterName.FindStringSubmatchIndex(redacted[searchStart:])
		if match == nil {
			break
		}

		nameStart := searchStart + match[2]
		nameEnd := searchStart + match[3]
		valueStart := searchStart + match[1]
		searchStart = valueStart
		decodedName, err := url.QueryUnescape(redacted[nameStart:nameEnd])
		if err != nil || !strings.EqualFold(decodedName, "code") {
			continue
		}

		valueEnd := valueStart
		for valueEnd < len(redacted) && !isQueryValueTerminator(redacted[valueEnd]) {
			valueEnd++
		}
		if valueEnd == valueStart || redacted[valueStart:valueEnd] == redactionMarker || !hasWebAuthQueryContext(redacted, nameStart, redacted[valueStart:valueEnd]) {
			continue
		}

		redacted = redacted[:valueStart] + redactionMarker + redacted[valueEnd:]
		changed = true
		searchStart = valueStart + len(redactionMarker)
	}
	return redacted, changed
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
		attributeMatch := attributeMatches[index]
		valueStart, valueEnd := attributeMatch[4], attributeMatch[5]
		if valueStart < 0 {
			valueStart, valueEnd = attributeMatch[6], attributeMatch[7]
		}
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

		attributeMatch := attributeMatches[valueAttribute]
		valueStart, valueEnd := attributeMatch[4], attributeMatch[5]
		if valueStart < 0 {
			valueStart, valueEnd = attributeMatch[6], attributeMatch[7]
		}
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
	redacted := value
	changed := false
	for searchStart := 0; searchStart < len(redacted); {
		match := tomlMultilineCredentialStart.FindStringIndex(redacted[searchStart:])
		if match == nil {
			break
		}

		open := searchStart + match[1] - 3
		close := findTOMLMultilineStringEnd(redacted, open)
		if close < 0 {
			close = len(redacted) - 1
		}
		redacted = redacted[:open] + redactionMarker + redacted[close+1:]
		changed = true
		searchStart = open + len(redactionMarker)
	}
	return redacted, changed
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

func redactJSONEscapedCredentialValues(value string) (string, bool) {
	redacted := value
	changed := false
	maxEscapeDepth := maxJSONEscapeDepthForLength(len(value))
	for escapeDepth := 0; escapeDepth <= maxEscapeDepth; escapeDepth++ {
		if next, layerChanged := redactJSONCredentialValues(redacted, escapeDepth); layerChanged {
			redacted = next
			changed = true
		}
	}
	return redacted, changed
}

func maxJSONEscapeDepthForLength(length int) int {
	maxDepth := 0
	for length > 1 {
		maxDepth++
		length >>= 1
	}
	return maxDepth
}

func redactJSONCredentialValues(value string, escapeDepth int) (string, bool) {
	return redactJSONValuesMatchingName(value, escapeDepth, jsonCredentialName.MatchString, false, false)
}

func redactJSONValuesMatchingName(value string, escapeDepth int, nameMatches func(string) bool, includePlainKeys, preserveUnterminatedValues bool) (string, bool) {
	redacted := value
	changed := false
	quoteDelimiter := jsonQuoteDelimiter(escapeDepth)
	for searchStart := 0; searchStart < len(redacted); {
		relativeOpen := strings.Index(redacted[searchStart:], quoteDelimiter)
		if relativeOpen < 0 {
			break
		}
		open := searchStart + relativeOpen
		quote := open + len(quoteDelimiter) - 1
		if !isJSONStringDelimiterAtDepth(redacted, quote, escapeDepth) {
			searchStart = open + len(quoteDelimiter)
			continue
		}
		close := findJSONStringEndAtDepth(redacted, open, escapeDepth)
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

		colon := close + 1
		for colon < len(redacted) && strings.ContainsRune(" \t\r\n", rune(redacted[colon])) {
			colon++
		}
		if colon >= len(redacted) || redacted[colon] != ':' {
			searchStart = close + 1
			continue
		}
		valueStart := colon + 1
		for valueStart < len(redacted) && strings.ContainsRune(" \t\r\n", rune(redacted[valueStart])) {
			valueStart++
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
	redacted := value
	changed := false
	maxEscapeDepth := maxJSONEscapeDepthForLength(len(value))
	for escapeDepth := 0; escapeDepth <= maxEscapeDepth; escapeDepth++ {
		for _, rule := range contextualJSONContainerRules {
			if next, layerChanged := redactJSONValuesInNamedContainer(redacted, escapeDepth, rule); layerChanged {
				redacted = next
				changed = true
			}
		}
	}
	return redacted, changed
}

func redactSensitiveJSONNameValuePairs(value string) (string, bool) {
	redacted := value
	changed := false
	maxEscapeDepth := maxJSONEscapeDepthForLength(len(value))
	for escapeDepth := 0; escapeDepth <= maxEscapeDepth; escapeDepth++ {
		if next, layerChanged := redactSensitiveJSONNameValuePairsAtDepth(redacted, escapeDepth); layerChanged {
			redacted = next
			changed = true
		}
	}
	return redacted, changed
}

func redactSensitiveJSONNameValuePairsAtDepth(value string, escapeDepth int) (string, bool) {
	redacted := value
	changed := false
	for searchStart := 0; searchStart < len(redacted); {
		open := nextJSONObjectStart(redacted, searchStart, escapeDepth)
		if open < 0 {
			break
		}
		close := findJSONContainerEndAtDepthStrict(redacted, open, escapeDepth)
		if close < open {
			// Do not rescan every nested object when the outer JSON container is
			// malformed. Other JSON passes still handle credential pairs in the
			// truncated text, while this object-oriented pass has no safe bound.
			break
		}
		object := redacted[open : close+1]
		redactedObject, objectChanged := redactSensitiveJSONNameValueObject(object, escapeDepth)
		if objectChanged {
			redacted = redacted[:open] + redactedObject + redacted[close+1:]
			changed = true
		}
		searchStart = open + 1
	}
	return redacted, changed
}

func redactSensitiveJSONNameValueObject(object string, escapeDepth int) (string, bool) {
	properties := parseJSONObjectProperties(object, escapeDepth)
	delimiter := jsonQuoteDelimiter(escapeDepth)
	sensitiveName := false
	for _, property := range properties {
		if !strings.EqualFold(property.key, "name") || property.valueStart >= len(object) || !strings.HasPrefix(object[property.valueStart:], delimiter) {
			continue
		}
		name, valid := decodeJSONCredentialKey(object[property.valueStart:property.valueEnd], escapeDepth)
		if valid && jsonCredentialName.MatchString(name) {
			sensitiveName = true
			break
		}
	}
	if !sensitiveName {
		return object, false
	}

	type replacement struct {
		start int
		end   int
		value string
	}
	var replacements []replacement
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
		replacements = append(replacements, replacement{start: property.valueStart, end: valueEnd, value: replacementValue})
	}
	if len(replacements) == 0 {
		return object, false
	}

	redacted := object
	for index := len(replacements) - 1; index >= 0; index-- {
		span := replacements[index]
		redacted = redacted[:span.start] + span.value + redacted[span.end:]
	}
	return redacted, true
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
		name := yamlMappingScalar(lines, line, content, valueStart)
		if strings.HasPrefix(name, "*") {
			name = scalarAnchors[strings.TrimPrefix(name, "*")]
		}
		if !yamlCredentialNamePattern.MatchString(name) {
			continue
		}

		mappingIndent := yamlKeyIndent(content)
		start := line
		for start > 0 && yamlLineBelongsToMapping(lines[start-1], mappingIndent) {
			start--
		}
		if start > 0 {
			previous, _ := splitLineEnding(lines[start-1])
			previousIndent := leadingIndent(previous)
			if isYAMLSequenceItemAtIndent(previous, previousIndent) && yamlKeyIndent(previous) == mappingIndent {
				start--
			}
		}
		end := line + 1
		for end < len(lines) && yamlLineBelongsToMapping(lines[end], mappingIndent) {
			end++
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
			redactSensitiveYAMLValueContinuations(lines, candidate, mappingIndent, valueContent, candidateValueStart)
			lines[candidate] = valueContent[:candidateValueStart] + redactionMarker + ending
			changed = true
		}
	}
	return changed
}

func redactSensitiveYAMLValueContinuations(lines []string, line, mappingIndent int, content string, valueStart int) {
	value, _ := trimYAMLNodeProperties(content[valueStart:])
	value = strings.TrimSpace(value)
	if value == "" || strings.HasPrefix(value, "[") || strings.HasPrefix(value, "{") {
		return
	}

	quote := byte(0)
	if value[0] == '\'' || value[0] == '"' {
		quote = value[0]
		if yamlQuotedScalarCloses(value[1:], quote) {
			return
		}
	}
	for nextLine := line + 1; nextLine < len(lines); nextLine++ {
		nextContent, _ := splitLineEnding(lines[nextLine])
		trimmed := strings.TrimSpace(nextContent)
		if quote == 0 && trimmed != "" && leadingIndent(nextContent) <= mappingIndent {
			break
		}
		if quote != 0 && yamlQuotedScalarCloses(nextContent, quote) {
			lines[nextLine] = ""
			return
		}
		if trimmed != "" {
			lines[nextLine] = ""
		}
	}
}

func redactSensitiveYAMLFlowNameValuePairs(value string, scalarAnchors map[string]string) (string, bool) {
	redacted := value
	changed := false
	for searchStart := 0; searchStart < len(redacted); {
		open := nextYAMLFlowObjectStart(redacted, searchStart)
		if open < 0 {
			break
		}
		close := findYAMLFlowEnd(redacted, open)
		if close < open {
			break
		}
		object := redacted[open : close+1]
		redactedObject, objectChanged := redactSensitiveYAMLFlowNameValueObject(object, scalarAnchors)
		if objectChanged {
			redacted = redacted[:open] + redactedObject + redacted[close+1:]
			changed = true
		}
		searchStart = open + 1
	}
	return redacted, changed
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
			if quote == '"' && value[index] == '\\' {
				index++
				continue
			}
			if quote == '\'' && value[index] == '\'' && index+1 < len(value) && value[index+1] == '\'' {
				index++
				continue
			}
			if value[index] == quote {
				quote = 0
			}
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
	type property struct {
		key             string
		valueStart      int
		trimmedValueEnd int
	}
	var properties []property
	for cursor := 1; cursor < len(object)-1; {
		cursor = skipYAMLFlowSpaceAndComments(object, cursor, len(object)-1)
		if cursor >= len(object)-1 {
			break
		}
		colon := findKubernetesFlowColon(object, cursor)
		if colon < 0 {
			return object, false
		}
		key := decodeYAMLMappingKey(object[cursor:colon])
		valueStart := colon + 1
		for valueStart < len(object)-1 && strings.ContainsRune(" \t\r\n", rune(object[valueStart])) {
			valueStart++
		}
		valueEnd := findKubernetesFlowValueEnd(object, valueStart)
		if valueEnd < valueStart {
			return object, false
		}
		trimmedEnd := valueEnd
		for trimmedEnd > valueStart && strings.ContainsRune(" \t\r\n", rune(object[trimmedEnd-1])) {
			trimmedEnd--
		}
		properties = append(properties, property{key: key, valueStart: valueStart, trimmedValueEnd: trimmedEnd})
		cursor = valueEnd
	}

	sensitiveName := false
	for _, property := range properties {
		if !strings.EqualFold(property.key, "name") {
			continue
		}
		name := kubernetesYAMLScalar(object[property.valueStart:property.trimmedValueEnd])
		if strings.HasPrefix(name, "*") {
			name = scalarAnchors[strings.TrimPrefix(name, "*")]
		}
		if yamlCredentialNamePattern.MatchString(name) {
			sensitiveName = true
			break
		}
	}
	if !sensitiveName {
		return object, false
	}

	redacted := object
	for index := len(properties) - 1; index >= 0; index-- {
		property := properties[index]
		if !strings.EqualFold(property.key, "value") || property.valueStart >= property.trimmedValueEnd {
			continue
		}
		replacementStart, replacementEnd := property.valueStart, property.trimmedValueEnd
		if replacementEnd-replacementStart >= 2 && (object[replacementStart] == '\'' || object[replacementStart] == '"') && object[replacementEnd-1] == object[replacementStart] {
			replacementStart++
			replacementEnd--
		}
		if object[replacementStart:replacementEnd] == redactionMarker {
			continue
		}
		redacted = redacted[:replacementStart] + redactionMarker + redacted[replacementEnd:]
	}
	return redacted, redacted != object
}

func redactJSONValuesInNamedContainer(value string, escapeDepth int, rule contextualJSONContainerRule) (string, bool) {
	redacted := value
	changed := false
	quoteDelimiter := jsonQuoteDelimiter(escapeDepth)
	for searchStart := 0; searchStart < len(redacted); {
		relativeOpen := strings.Index(redacted[searchStart:], quoteDelimiter)
		if relativeOpen < 0 {
			break
		}
		keyOpen := searchStart + relativeOpen
		quote := keyOpen + len(quoteDelimiter) - 1
		if !isJSONStringDelimiterAtDepth(redacted, quote, escapeDepth) {
			searchStart = keyOpen + len(quoteDelimiter)
			continue
		}
		keyClose := findJSONStringEndAtDepth(redacted, keyOpen, escapeDepth)
		if keyClose < 0 {
			break
		}

		decodedKey, valid := decodeJSONCredentialKey(redacted[keyOpen:keyClose+1], escapeDepth)
		if !valid || !strings.EqualFold(decodedKey, rule.name) {
			searchStart = keyClose + 1
			continue
		}

		colon := keyClose + 1
		for colon < len(redacted) && strings.ContainsRune(" \t\r\n", rune(redacted[colon])) {
			colon++
		}
		if colon >= len(redacted) || redacted[colon] != ':' {
			searchStart = keyClose + 1
			continue
		}
		containerOpen := colon + 1
		for containerOpen < len(redacted) && strings.ContainsRune(" \t\r\n", rune(redacted[containerOpen])) {
			containerOpen++
		}
		if containerOpen >= len(redacted) || !strings.ContainsRune(rule.openers, rune(redacted[containerOpen])) {
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
		content, ending := splitLineEnding(lines[line])
		for _, match := range yamlAnchor.FindAllStringSubmatchIndex(content, -1) {
			if _, sensitive := aliases[content[match[2]:match[3]]]; !sensitive {
				continue
			}

			prefix := content[:match[0]]
			valueIndent, isValue := yamlAnchorValueIndent(prefix)
			if !isValue {
				continue
			}

			inlineValue := strings.TrimSpace(content[match[1]:])
			hasInlineValue := inlineValue != "" && !strings.HasPrefix(inlineValue, "#")
			end := line + 1
			hasIndentedContent := false
			for end < len(lines) {
				child, _ := splitLineEnding(lines[end])
				if strings.TrimSpace(child) == "" {
					end++
					continue
				}
				if leadingIndent(child) <= valueIndent {
					break
				}
				hasIndentedContent = true
				end++
			}
			if !hasInlineValue && !hasIndentedContent {
				continue
			}
			if inlineValue == redactionMarker && !hasIndentedContent {
				continue
			}

			lines[line] = content[:match[1]] + " " + redactionMarker + ending
			lines = append(lines[:line+1], lines[end:]...)
			changed = true
			break
		}
	}
	return strings.Join(lines, ""), changed
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
		valueLine := line + 1
		for valueLine < len(lines) {
			candidate, _ := splitLineEnding(lines[valueLine])
			trimmed := strings.TrimSpace(candidate)
			if trimmed != "" && !strings.HasPrefix(trimmed, "#") {
				break
			}
			valueLine++
		}
		if valueLine >= len(lines) {
			continue
		}
		valueContent, ending := splitLineEnding(lines[valueLine])
		if leadingIndent(valueContent) != keyIndent {
			continue
		}
		indicator := valueContent[keyIndent:]
		if indicator == "" || indicator[0] != ':' || (len(indicator) > 1 && indicator[1] != ' ' && indicator[1] != '\t' && indicator[1] != '#') {
			continue
		}

		inlineValue := strings.TrimSpace(indicator[1:])
		hasInlineValue := inlineValue != "" && !strings.HasPrefix(inlineValue, "#")
		end := valueLine + 1
		hasIndentedContent := false
		for end < len(lines) {
			child, _ := splitLineEnding(lines[end])
			if strings.TrimSpace(child) == "" {
				end++
				continue
			}
			if leadingIndent(child) <= keyIndent {
				break
			}
			hasIndentedContent = true
			end++
		}
		if !hasInlineValue && !hasIndentedContent {
			continue
		}
		if inlineValue == redactionMarker && !hasIndentedContent {
			continue
		}

		lines[valueLine] = valueContent[:keyIndent] + ": " + redactionMarker + ending
		lines = append(lines[:valueLine+1], lines[end:]...)
		changed = true
		line = valueLine
	}
	return strings.Join(lines, ""), changed
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
			if quote == '"' && value[i] == '\\' {
				i++
				continue
			}
			if quote == '\'' && value[i] == '\'' && i+1 < len(value) && value[i+1] == '\'' {
				i++
				continue
			}
			if value[i] == quote {
				quote = 0
			}
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

func redactStructuredCredentialObjects(value string) (string, bool) {
	type objectPattern struct {
		pattern       *lazyRegexp
		escapedQuotes bool
		replacement   string
		array         bool
	}

	patterns := []objectPattern{
		{pattern: rawCredentialObject, replacement: `"` + redactionMarker + `"`},
		{pattern: escapedCredentialObject, escapedQuotes: true, replacement: `\"` + redactionMarker + `\"`},
		{pattern: rawCredentialArray, replacement: `["` + redactionMarker + `"]`, array: true},
		{pattern: escapedCredentialArray, escapedQuotes: true, replacement: `[\"` + redactionMarker + `\"]`, array: true},
	}
	redacted := value
	changed := false
	for _, candidate := range patterns {
		searchStart := 0
		for searchStart < len(redacted) {
			match := candidate.pattern.FindStringIndex(redacted[searchStart:])
			if match == nil {
				break
			}

			open := searchStart + match[1] - 1
			if redacted[open] == '[' && (strings.HasPrefix(redacted[open:], redactionMarker) || strings.HasPrefix(redacted[open:], candidate.replacement)) {
				searchStart = open + 1
				continue
			}
			close := findJSONObjectEnd(redacted, open, candidate.escapedQuotes)
			if candidate.array && !looksLikeJSONCredentialArray(redacted, open, close, candidate.escapedQuotes) {
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

func findJSONObjectEnd(value string, open int, escapedQuotes bool) int {
	return findJSONContainerEnd(value, open, escapedQuotes)
}

func findJSONContainerEnd(value string, open int, escapedQuotes bool) int {
	escapeDepth := 0
	if escapedQuotes {
		escapeDepth = 1
	}
	return findJSONContainerEndAtDepth(value, open, escapeDepth)
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

func isJSONStringDelimiter(value string, quote int, escapedQuotes bool) bool {
	escapeDepth := 0
	if escapedQuotes {
		escapeDepth = 1
	}
	return isJSONStringDelimiterAtDepth(value, quote, escapeDepth)
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
	if !rawCookieJarPattern.MatchString(value) && !escapedCookieJarPattern.MatchString(value) {
		return value, false
	}

	type cookieObjectPattern struct {
		pattern       *lazyRegexp
		escapedQuotes bool
	}
	patterns := []cookieObjectPattern{
		{pattern: rawCookieJarPattern},
		{pattern: escapedCookieJarPattern, escapedQuotes: true},
	}
	redacted := value
	changed := false
	for _, candidate := range patterns {
		searchStart := 0
		for searchStart < len(redacted) {
			match := candidate.pattern.FindStringIndex(redacted[searchStart:])
			if match == nil {
				break
			}

			open := searchStart + match[1] - 1
			close := findJSONObjectEnd(redacted, open, candidate.escapedQuotes)
			object := redacted[open : close+1]
			redactedObject := object
			for _, rule := range structuredContainerValueRedactionRules {
				redactedObject = rule.pattern.ReplaceAllString(redactedObject, rule.replacement)
			}
			if next, truncatedChanged := redactTruncatedStructuredValues(redactedObject, candidate.escapedQuotes); truncatedChanged {
				redactedObject = next
			}
			if redactedObject != object {
				redacted = redacted[:open] + redactedObject + redacted[close+1:]
				changed = true
			}
			searchStart = open + len(redactedObject)
		}
	}
	return redacted, changed
}

// Registry configuration auth values are base64-encoded username/password
// pairs. Scope the generic "auth" key to the auths container so unrelated
// diagnostic fields with that name remain useful.
func redactRegistryConfigurationAuthValues(value string) (string, bool) {
	type authsContainerPattern struct {
		pattern       *lazyRegexp
		escapedQuotes bool
	}
	patterns := []authsContainerPattern{
		{pattern: rawRegistryAuthsPattern},
		{pattern: escapedRegistryAuthsPattern, escapedQuotes: true},
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
			close := findJSONObjectEnd(redacted, open, candidate.escapedQuotes)
			container := redacted[open : close+1]
			redactedContainer := container
			for _, rule := range registryAuthValueRedactionRules {
				redactedContainer = rule.pattern.ReplaceAllString(redactedContainer, rule.replacement)
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

// Upload-operation request-header values are capabilities regardless of their
// names, matching asc.RedactUploadOperations while preserving useful metadata.
func redactStructuredUploadHeaderValues(value string) (string, bool) {
	type headerContainerPattern struct {
		pattern       *lazyRegexp
		escapedQuotes bool
	}
	patterns := []headerContainerPattern{
		{pattern: rawRequestHeaders},
		{pattern: escapedRequestHeaders, escapedQuotes: true},
	}

	redacted := value
	changed := false
	for _, candidate := range patterns {
		searchStart := 0
		for searchStart < len(redacted) {
			match := candidate.pattern.FindStringIndex(redacted[searchStart:])
			if match == nil {
				break
			}

			open := searchStart + match[1] - 1
			close := findJSONContainerEnd(redacted, open, candidate.escapedQuotes)
			container := redacted[open : close+1]
			redactedContainer := container
			for _, rule := range structuredContainerValueRedactionRules {
				redactedContainer = rule.pattern.ReplaceAllString(redactedContainer, rule.replacement)
			}
			if next, truncatedChanged := redactTruncatedStructuredValues(redactedContainer, candidate.escapedQuotes); truncatedChanged {
				redactedContainer = next
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

func redactTruncatedStructuredValues(value string, escapedQuotes bool) (string, bool) {
	pattern := rawStructuredValueStart
	if escapedQuotes {
		pattern = escapedValueStart
	}

	searchStart := 0
	for searchStart < len(value) {
		match := pattern.FindStringIndex(value[searchStart:])
		if match == nil {
			return value, false
		}

		openQuote := searchStart + match[1] - 1
		for quote := openQuote + 1; quote < len(value); quote++ {
			if value[quote] == '"' && isJSONStringDelimiter(value, quote, escapedQuotes) {
				searchStart = quote + 1
				break
			}
			if quote == len(value)-1 {
				return value[:openQuote+1] + redactionMarker, true
			}
		}
		if openQuote == len(value)-1 {
			return value + redactionMarker, true
		}
	}
	return value, false
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

func redactKubectlSecretLiterals(value string) (string, bool) {
	result := value
	changed := false
	for start := 0; start < len(result); {
		end := findShellCommandEnd(result, start)
		command := result[start:end]
		redacted := command
		if isKubectlCreateSecretCommand(command) {
			redacted = kubectlFromLiteralValue.ReplaceAllString(command, `${1}`+redactionMarker)
		}
		if redacted != command {
			result = result[:start] + redacted + result[end:]
			changed = true
		}

		separator := start + len(redacted)
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

func redactOpenSSLCredentialArguments(value string) (string, bool) {
	redacted, subcommandChanged := redactOpenSSLSubcommandCredentialArguments(value)
	redacted, positionalChanged := redactOpenSSLPasswdPositionalArguments(redacted)
	return redacted, subcommandChanged || positionalChanged
}

func redactOpenSSLSubcommandCredentialArguments(value string) (string, bool) {
	result := value
	changed := false
	for start := 0; start < len(result); {
		end := findShellCommandEnd(result, start)
		command := result[start:end]
		commandStart, subcommand, ok := openSSLCommand(command)
		commandSuffix := command[commandStart:]
		commandChanged := false
		if pattern := opensslSubcommandCredentialPatterns[subcommand]; ok && pattern != nil {
			commandSuffix, commandChanged = redactCommandCredentialFlagValues(commandSuffix, pattern)
		}
		if ok {
			options := append([]string{}, opensslCredentialOptions...)
			options = append(options, opensslSubcommandCredentialOptions[subcommand]...)
			var optionChanged bool
			commandSuffix, optionChanged = redactCommandCredentialOptionValues(commandSuffix, subcommand, options...)
			commandChanged = commandChanged || optionChanged
		}
		if subcommand == "dgst" || subcommand == "mac" {
			var macChanged bool
			commandSuffix, macChanged = redactCommandCredentialFlagValuesMatching(commandSuffix, opensslMACOptionValueStartPattern, isOpenSSLMACKeyOption)
			commandChanged = commandChanged || macChanged
		}
		if subcommand == "kdf" {
			var kdfChanged bool
			commandSuffix, kdfChanged = redactCommandCredentialFlagValuesMatching(commandSuffix, opensslKDFOptionValueStartPattern, isOpenSSLKDFSecretOption)
			commandChanged = commandChanged || kdfChanged
		}
		if commandChanged {
			command = command[:commandStart] + commandSuffix
			result = result[:start] + command + result[end:]
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

func redactCommandCredentialFlagValues(command string, pattern *lazyRegexp) (string, bool) {
	return redactCommandCredentialFlagValuesMatching(command, pattern, func(string) bool { return true })
}

func redactCommandCredentialFlagValuesMatching(command string, pattern *lazyRegexp, shouldRedact func(string) bool) (string, bool) {
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
		if result[valueStart:valueEnd] == redactionMarker {
			searchStart = valueEnd
			continue
		}
		if !shouldRedact(result[valueStart:valueEnd]) {
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
	type replacement struct {
		start int
		end   int
		text  string
	}

	spans := splitCredentialShellWordSpans(command)
	replacements := make([]replacement, 0, 2)
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
				replacements = append(replacements, replacement{
					start: spans[argumentSpanIndex].start,
					end:   argumentEnd,
					text:  redactionMarker,
				})
				continue
			}
			if content[separator+1:] == redactionMarker {
				continue
			}
			replacements = append(replacements, replacement{
				start: spans[argumentSpanIndex].start + contentStart + separator + 1,
				end:   spans[argumentSpanIndex].start + contentEnd,
				text:  redactionMarker,
			})
		} else if rawArgument[contentStart:contentEnd] != redactionMarker {
			replacements = append(replacements, replacement{
				start: spans[argumentSpanIndex].start,
				end:   argumentEnd,
				text:  redactionMarker,
			})
		}
		index = argumentSpanIndex
		for index+1 < len(spans) && spans[index+1].start < argumentEnd {
			index++
		}
	}

	result := command
	for index := len(replacements) - 1; index >= 0; index-- {
		replacement := replacements[index]
		result = result[:replacement.start] + replacement.text + result[replacement.end:]
	}
	return result, len(replacements) > 0
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

func isOpenSSLMACKeyOption(value string) bool {
	spans := splitCredentialShellWordSpans(value)
	if len(spans) == 0 {
		return false
	}
	option := strings.ToLower(strings.TrimPrefix(strings.Trim(spans[0].value, `"'`), "$"))
	return strings.HasPrefix(option, "key:") || strings.HasPrefix(option, "hexkey:")
}

func isOpenSSLKDFSecretOption(value string) bool {
	spans := splitCredentialShellWordSpans(value)
	if len(spans) == 0 {
		return false
	}
	option := strings.ToLower(strings.TrimPrefix(strings.Trim(spans[0].value, `"'`), "$"))
	for _, name := range []string{"key:", "hexkey:", "secret:", "hexsecret:", "pass:", "hexpass:"} {
		if strings.HasPrefix(option, name) {
			return true
		}
	}
	return false
}

func openSSLCommand(command string) (int, string, bool) {
	spans := splitCredentialShellWordSpans(command)
	words := make([]string, 0, len(spans))
	for _, span := range spans {
		words = append(words, span.value)
	}
	for index, word := range words {
		baseName := commandBaseName(word)
		if (baseName != "openssl" && baseName != "openssl.exe") || !isCredentialCommandPrefix(words[:index]) {
			continue
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
	result := value
	changed := false
	for start := 0; start < len(result); {
		end := findShellCommandEnd(result, start)
		command := result[start:end]
		spans := splitCredentialShellWordSpans(command)
		words := make([]string, 0, len(spans))
		for _, span := range spans {
			words = append(words, span.value)
		}

		if passwordIndex, ok := openSSLPasswdPasswordWordIndex(words); ok {
			span := spans[passwordIndex]
			redactionEnd := credentialShellArgumentEnd(command, span)
			command = command[:span.start] + redactionMarker + command[redactionEnd:]
			result = result[:start] + command + result[end:]
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

func openSSLPasswdPasswordWordIndex(words []string) (int, bool) {
	for opensslIndex, word := range words {
		baseName := commandBaseName(word)
		if (baseName != "openssl" && baseName != "openssl.exe") || !isCredentialCommandPrefix(words[:opensslIndex]) {
			continue
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
	return 0, false
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

func redactKeytoolCredentialArguments(value string) (string, bool) {
	return redactNamedCommandCredentialArguments(value, "keytool", keytoolCredentialFlagPattern)
}

func redactJarsignerCredentialArguments(value string) (string, bool) {
	return redactNamedCommandCredentialArguments(value, "jarsigner", jarsignerCredentialFlagPattern)
}

func redactZipCredentialArguments(value string) (string, bool) {
	redacted, attachedChanged := redactNamedCommandAttachedShortCredentialArguments(value, "zip", "P")
	redacted, patternChanged := redactNamedCommandCredentialArguments(redacted, "zip", zipCredentialFlagPattern)
	return redacted, attachedChanged || patternChanged
}

func redactUnzipCredentialArguments(value string) (string, bool) {
	redacted, attachedChanged := redactNamedCommandAttachedShortCredentialArguments(value, "unzip", "P")
	redacted, patternChanged := redactNamedCommandCredentialArguments(redacted, "unzip", zipCredentialFlagPattern)
	return redacted, attachedChanged || patternChanged
}

func redactCurlCredentialArguments(value string) (string, bool) {
	result := value
	changed := false
	for start := 0; start < len(result); {
		end := findShellCommandEnd(result, start)
		command := result[start:end]
		if commandStart, ok := namedCredentialCommandStart(command, "curl"); ok {
			redacted := command[commandStart:]
			for _, rules := range [][]redactionRule{curlUserCredentialRedactionRules, curlCertificateCredentialRedactionRules, curlArgumentCredentialRedactionRules} {
				for _, rule := range rules {
					redacted = rule.pattern.ReplaceAllString(redacted, rule.replacement)
				}
			}
			redacted, _ = redactCompoundCurlHeaderWords(redacted)
			if redacted != command[commandStart:] {
				command = command[:commandStart] + redacted
				result = result[:start] + command + result[end:]
				changed = true
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

func redactRedisCLICredentialArguments(value string) (string, bool) {
	result := value
	changed := false
	for start := 0; start < len(result); {
		end := findShellCommandEnd(result, start)
		command := result[start:end]
		if commandStart, ok := namedCredentialCommandStart(command, "redis-cli"); ok {
			redacted, commandChanged := redactCommandCredentialOptionValues(command[commandStart:], "", "a", "pass")
			if commandChanged {
				command = command[:commandStart] + redacted
				result = result[:start] + command + result[end:]
				changed = true
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

func redactDockerLoginCredentialArguments(value string) (string, bool) {
	result := value
	changed := false
	for start := 0; start < len(result); {
		end := findShellCommandEnd(result, start)
		command := result[start:end]
		if commandStart, ok := dockerLoginCommandStart(command); ok {
			redacted, _ := redactAttachedShortCredentialArguments(command[commandStart:], "p")
			redacted = dockerLoginCredentialFlagPattern.ReplaceAllString(redacted, `${1}${2}`+redactionMarker)
			if redacted != command[commandStart:] {
				command = command[:commandStart] + redacted
				result = result[:start] + command + result[end:]
				changed = true
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

func redactSSHPassCredentialArguments(value string) (string, bool) {
	type replacement struct {
		start int
		end   int
		text  string
	}

	result := value
	changed := false
	for start := 0; start < len(result); {
		end := findShellCommandEnd(result, start)
		command := result[start:end]
		spans := splitCredentialShellWordSpans(command)
		words := make([]string, 0, len(spans))
		for _, span := range spans {
			words = append(words, span.value)
		}

		replacements := make([]replacement, 0, 1)
		for sshpassIndex, word := range words {
			baseName := commandBaseName(word)
			if (baseName != "sshpass" && baseName != "sshpass.exe") || !isCredentialCommandPrefix(words[:sshpassIndex]) {
				continue
			}
		options:
			for optionIndex := sshpassIndex + 1; optionIndex < len(words); optionIndex++ {
				option := strings.Trim(words[optionIndex], `"'`)
				if option == "--" || len(option) < 2 || option[0] != '-' || option == "-" {
					break
				}
				if option[1] == '-' {
					break
				}

				for flagIndex := 1; flagIndex < len(option); flagIndex++ {
					flag := option[flagIndex]
					switch flag {
					case 'v':
						continue
					case 'h', 'V':
						break options
					case 'e':
						if flagIndex+1 < len(option) {
							continue options
						}
					case 'f', 'd', 'P', 'p':
						attached := flagIndex+1 < len(option)
						if flag == 'p' {
							if attached {
								valueStart := flagIndex + 1
								if option[valueStart] == '=' {
									valueStart++
								}
								if option[valueStart:] != redactionMarker {
									prefix := option[:valueStart]
									replacements = append(replacements, replacement{start: spans[optionIndex].start, end: credentialShellArgumentEnd(command, spans[optionIndex]), text: prefix + redactionMarker})
								}
							} else if optionIndex+1 < len(words) {
								passwordSpan := spans[optionIndex+1]
								if strings.Trim(words[optionIndex+1], `"'`) != redactionMarker {
									replacements = append(replacements, replacement{start: passwordSpan.start, end: credentialShellArgumentEnd(command, passwordSpan), text: redactionMarker})
								}
							}
						}
						if !attached {
							if optionIndex+1 >= len(words) {
								break options
							}
							optionIndex++
						}
						continue options
					default:
						break options
					}
				}
			}
			break
		}

		for index := len(replacements) - 1; index >= 0; index-- {
			replacement := replacements[index]
			command = command[:replacement.start] + replacement.text + command[replacement.end:]
			changed = true
		}
		if len(replacements) > 0 {
			result = result[:start] + command + result[end:]
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

func dockerLoginCommandStart(command string) (int, bool) {
	spans := splitCredentialShellWordSpans(command)
	words := make([]string, 0, len(spans))
	for _, span := range spans {
		words = append(words, span.value)
	}
	for index, word := range words {
		baseName := commandBaseName(word)
		if (baseName != "docker" && baseName != "docker.exe") || !isCredentialCommandPrefix(words[:index]) {
			continue
		}
		subcommand, ok := dockerSubcommand(words[index+1:])
		return spans[index].start, ok && subcommand == "login"
	}
	return 0, false
}

func dockerSubcommand(words []string) (string, bool) {
	for len(words) > 0 {
		word := strings.Trim(words[0], `"'`)
		if word == "--" {
			words = words[1:]
			break
		}
		if !strings.HasPrefix(word, "-") || word == "-" {
			return word, true
		}

		requiresArgument, allowed := dockerGlobalOption(word)
		if !allowed {
			return "", false
		}
		words = words[1:]
		if requiresArgument {
			if len(words) == 0 {
				return "", false
			}
			words = words[1:]
		}
	}
	if len(words) == 0 {
		return "", false
	}
	return strings.Trim(words[0], `"'`), true
}

func dockerGlobalOption(option string) (bool, bool) {
	if strings.HasPrefix(option, "--") {
		name, _, attached := strings.Cut(option, "=")
		switch name {
		case "--config", "--context", "--host", "--log-level", "--tlscacert", "--tlscert", "--tlskey":
			return !attached, true
		case "--debug", "--tls", "--tlsverify":
			return false, true
		default:
			return false, false
		}
	}
	if len(option) < 2 {
		return false, false
	}
	switch option[1] {
	case 'c', 'H', 'l':
		return len(option) == 2, true
	case 'D':
		return false, len(option) == 2
	default:
		return false, false
	}
}

func redactSSHKeygenCredentialArguments(value string) (string, bool) {
	type replacement struct {
		start int
		end   int
		text  string
	}

	result := value
	changed := false
	for start := 0; start < len(result); {
		end := findShellCommandEnd(result, start)
		command := result[start:end]
		spans := splitCredentialShellWordSpans(command)
		words := make([]string, 0, len(spans))
		for _, span := range spans {
			words = append(words, span.value)
		}

		replacements := make([]replacement, 0, 2)
		for commandIndex, word := range words {
			baseName := commandBaseName(word)
			if (baseName != "ssh-keygen" && baseName != "ssh-keygen.exe") || !isCredentialCommandPrefix(words[:commandIndex]) {
				continue
			}
		options:
			for optionIndex := commandIndex + 1; optionIndex < len(words); optionIndex++ {
				option := strings.Trim(words[optionIndex], `"'`)
				if option == "--" || len(option) < 2 || option[0] != '-' || option == "-" || option[1] == '-' {
					break
				}

				for flagIndex := 1; flagIndex < len(option); flagIndex++ {
					flag := option[flagIndex]
					if strings.ContainsRune("ABHKLQUXceghiklopquvy", rune(flag)) {
						continue
					}
					if !strings.ContainsRune("CDEFIMNOPRVYZabfgmnrstwz", rune(flag)) {
						break options
					}

					attached := flagIndex+1 < len(option)
					argumentSpanIndex := optionIndex
					if !attached {
						if optionIndex+1 >= len(words) {
							break options
						}
						argumentSpanIndex = optionIndex + 1
					}
					argumentEnd := credentialShellArgumentEnd(command, spans[argumentSpanIndex])
					if flag == 'N' || flag == 'P' {
						if attached {
							valueStart := flagIndex + 1
							if option[valueStart] == '=' {
								valueStart++
							}
							if option[valueStart:] != redactionMarker {
								prefix := option[:valueStart]
								replacements = append(replacements, replacement{start: spans[optionIndex].start, end: argumentEnd, text: prefix + redactionMarker})
							}
						} else {
							passphraseSpan := spans[argumentSpanIndex]
							if strings.Trim(words[argumentSpanIndex], `"'`) != redactionMarker {
								replacements = append(replacements, replacement{start: passphraseSpan.start, end: argumentEnd, text: redactionMarker})
							}
						}
					}
					optionIndex = argumentSpanIndex
					for optionIndex+1 < len(spans) && spans[optionIndex+1].start < argumentEnd {
						optionIndex++
					}
					continue options
				}
			}
			break
		}

		for index := len(replacements) - 1; index >= 0; index-- {
			replacement := replacements[index]
			command = command[:replacement.start] + replacement.text + command[replacement.end:]
			changed = true
		}
		if len(replacements) > 0 {
			result = result[:start] + command + result[end:]
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

func redactNamedCommandCredentialArguments(value, commandName string, pattern *lazyRegexp) (string, bool) {
	result := value
	changed := false
	for start := 0; start < len(result); {
		end := findShellCommandEnd(result, start)
		command := result[start:end]
		if commandStart, ok := namedCredentialCommandStart(command, commandName); ok {
			redacted := pattern.ReplaceAllString(command[commandStart:], `${1}${2}`+redactionMarker)
			if redacted != command[commandStart:] {
				command = command[:commandStart] + redacted
				result = result[:start] + command + result[end:]
				changed = true
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

func redactNamedCommandAttachedShortCredentialArguments(value, commandName, flags string) (string, bool) {
	result := value
	changed := false
	for start := 0; start < len(result); {
		end := findShellCommandEnd(result, start)
		command := result[start:end]
		if commandStart, ok := namedCredentialCommandStart(command, commandName); ok {
			redacted, commandChanged := redactAttachedShortCredentialArguments(command[commandStart:], flags)
			if commandChanged {
				command = command[:commandStart] + redacted
				result = result[:start] + command + result[end:]
				changed = true
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
	words := make([]string, 0, len(spans))
	for _, span := range spans {
		words = append(words, span.value)
	}
	for index, word := range words {
		baseName := commandBaseName(word)
		if (baseName == commandName || baseName == commandName+".exe") && isCredentialCommandPrefix(words[:index]) {
			return spans[index].start, true
		}
	}
	return 0, false
}

func redactSecurityCredentialArguments(value string) (string, bool) {
	result := value
	changed := false
	for start := 0; start < len(result); {
		end := findShellCommandEnd(result, start)
		command := result[start:end]
		if commandStart, subcommand, ok := securityCommand(command); ok {
			if pattern := securityCredentialFlagPatterns[subcommand]; pattern != nil {
				redacted, _ := redactAttachedShortCredentialArguments(command[commandStart:], securityAttachedCredentialFlags[subcommand])
				redacted = pattern.ReplaceAllString(redacted, `${1}${2}`+redactionMarker)
				if redacted != command[commandStart:] {
					command = command[:commandStart] + redacted
					result = result[:start] + command + result[end:]
					changed = true
				}
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

func securityCommand(command string) (int, string, bool) {
	spans := splitCredentialShellWordSpans(command)
	words := make([]string, 0, len(spans))
	for _, span := range spans {
		words = append(words, span.value)
	}
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

func consumeCredentialCommandWrapper(words []string) ([]string, bool) {
	if len(words) == 0 {
		return nil, false
	}
	wrapper := commandBaseName(words[0])
	if wrapper == "timeout" || wrapper == "gtimeout" {
		return consumeTimeoutCredentialWrapper(words[1:])
	}
	if wrapper == "nohup" {
		return consumeNohupCredentialWrapper(words[1:])
	}
	if wrapper == "exec" {
		return consumeExecCredentialWrapper(words[1:])
	}
	if wrapper == "nice" || wrapper == "gnice" {
		return consumeNiceCredentialWrapper(words[1:])
	}
	if wrapper == "time" || wrapper == "gtime" {
		return consumeTimeCredentialWrapper(words[1:])
	}
	if wrapper == "stdbuf" || wrapper == "gstdbuf" {
		return consumeStdbufCredentialWrapper(words[1:])
	}
	if wrapper == "setsid" {
		return consumeSetsidCredentialWrapper(words[1:])
	}
	if wrapper == "ionice" {
		return consumeIoniceCredentialWrapper(words[1:])
	}
	if wrapper == "caffeinate" {
		return consumeCaffeinateCredentialWrapper(words[1:])
	}
	if wrapper == "arch" {
		return consumeArchCredentialWrapper(words[1:])
	}
	if wrapper == "xcrun" {
		return consumeXcrunCredentialWrapper(words[1:])
	}
	if wrapper == "launchctl" {
		return consumeLaunchctlCredentialWrapper(words[1:])
	}
	if wrapper == "chroot" || wrapper == "gchroot" || wrapper == "chroot.exe" || wrapper == "gchroot.exe" {
		return consumeChrootCredentialWrapper(words[1:])
	}
	if wrapper == "xargs" || wrapper == "gxargs" {
		return consumeXargsCredentialWrapper(words[1:])
	}
	if wrapper == "find" || wrapper == "gfind" {
		return consumeFindExecCredentialWrapper(words[1:])
	}
	if wrapper == "watch" {
		return consumeWatchCredentialWrapper(words[1:])
	}
	if wrapper != "sudo" && wrapper != "doas" && wrapper != "command" && wrapper != "env" {
		return nil, false
	}

	words = words[1:]
	for len(words) > 0 {
		option := strings.Trim(words[0], `"'`)
		if option == "--" {
			return words[1:], true
		}
		if len(option) < 2 || option[0] != '-' || option == "-" {
			return words, true
		}

		requiresArgument, allowed := credentialWrapperOption(wrapper, option)
		if !allowed {
			return nil, false
		}
		words = words[1:]
		if requiresArgument {
			if len(words) == 0 {
				return nil, false
			}
			words = words[1:]
		}
	}
	return words, true
}

func consumeXargsCredentialWrapper(words []string) ([]string, bool) {
	for len(words) > 0 {
		option := strings.Trim(words[0], `"'`)
		if option == "--" {
			return words[1:], true
		}
		if len(option) < 2 || option[0] != '-' || option == "-" {
			return words, true
		}
		if option == "--help" || option == "--version" {
			return nil, false
		}
		if strings.HasPrefix(option, "--") {
			name, value, attached := strings.Cut(option, "=")
			switch name {
			case "--eof", "--replace":
				words = words[1:]
				continue
			case "--max-lines":
				words = words[1:]
				if attached && value == "" {
					return nil, false
				}
				continue
			case "--arg-file", "--delimiter", "--max-args", "--max-procs", "--max-chars", "--process-slot-var":
				words = words[1:]
				if !attached {
					if len(words) == 0 {
						return nil, false
					}
					words = words[1:]
				} else if value == "" {
					return nil, false
				}
				continue
			case "--null", "--interactive", "--verbose", "--no-run-if-empty", "--exit", "--open-tty", "--show-limits":
				if attached {
					return nil, false
				}
				words = words[1:]
				continue
			default:
				return nil, false
			}
		}

		requiresArgument := false
		for index := 1; index < len(option); index++ {
			switch option[index] {
			case '0', 'o', 'p', 'r', 't', 'x':
				continue
			case 'e', 'i', 'l':
				// GNU xargs treats these compatibility options as having an
				// optional attached value. A following word is the command.
				index = len(option)
			case 'a', 'd', 'E', 'I', 'J', 'L', 'n', 'P', 'R', 'S', 's':
				requiresArgument = index == len(option)-1
				index = len(option)
			default:
				return nil, false
			}
		}
		words = words[1:]
		if requiresArgument {
			if len(words) == 0 {
				return nil, false
			}
			words = words[1:]
		}
	}
	return words, true
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

func consumeWatchCredentialWrapper(words []string) ([]string, bool) {
	for len(words) > 0 {
		option := strings.Trim(words[0], `"'`)
		if option == "--" {
			return words[1:], true
		}
		if len(option) < 2 || option[0] != '-' || option == "-" {
			return words, true
		}
		if strings.HasPrefix(option, "--") {
			name, value, attached := strings.Cut(option, "=")
			switch name {
			case "--interval":
				words = words[1:]
				if attached {
					if value == "" {
						return nil, false
					}
					continue
				}
				if len(words) == 0 {
					return nil, false
				}
				words = words[1:]
				continue
			case "--differences":
				if attached && value == "" {
					return nil, false
				}
				words = words[1:]
				continue
			case "--equexit", "--shotsdir":
				words = words[1:]
				if attached {
					if value == "" {
						return nil, false
					}
					continue
				}
				if len(words) == 0 {
					return nil, false
				}
				words = words[1:]
				continue
			case "--beep", "--color", "--no-color", "--errexit", "--chgexit", "--exec", "--precise", "--no-title", "--no-rerun", "--no-wrap":
				if attached {
					return nil, false
				}
				words = words[1:]
				continue
			default:
				return nil, false
			}
		}

		requiresArgument := false
		for index := 1; index < len(option); index++ {
			switch option[index] {
			case 'b', 'c', 'C', 'e', 'f', 'g', 'p', 'r', 't', 'w', 'x':
				continue
			case 'd':
				// The cumulative marker is an optional attached value.
				index = len(option)
			case 'n', 'q', 's':
				requiresArgument = index == len(option)-1
				index = len(option)
			default:
				return nil, false
			}
		}
		words = words[1:]
		if requiresArgument {
			if len(words) == 0 {
				return nil, false
			}
			words = words[1:]
		}
	}
	return words, true
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

func consumeTimeCredentialWrapper(words []string) ([]string, bool) {
	for len(words) > 0 {
		option := strings.Trim(words[0], `"'`)
		if option == "--" {
			return words[1:], true
		}
		if strings.HasPrefix(option, "--") {
			requiresArgument, allowed := timeCredentialWrapperLongOption(option)
			if !allowed {
				return nil, false
			}
			words = words[1:]
			if requiresArgument {
				if len(words) == 0 {
					return nil, false
				}
				words = words[1:]
			}
			continue
		}
		if len(option) < 2 || option[0] != '-' || option == "-" {
			return words, true
		}
		requiresArgument := false
		for index := 1; index < len(option); index++ {
			switch option[index] {
			case 'a', 'h', 'l', 'p', 'q', 'v':
				continue
			case 'f', 'o':
				requiresArgument = index == len(option)-1
				index = len(option)
			default:
				return nil, false
			}
		}
		words = words[1:]
		if requiresArgument {
			if len(words) == 0 {
				return nil, false
			}
			words = words[1:]
		}
	}
	return words, true
}

func timeCredentialWrapperLongOption(option string) (bool, bool) {
	name, value, attached := strings.Cut(option, "=")
	switch name {
	case "--format", "--output":
		return !attached, !attached || value != ""
	case "--append", "--portability", "--quiet", "--verbose":
		return false, !attached
	default:
		return false, false
	}
}

func consumeStdbufCredentialWrapper(words []string) ([]string, bool) {
	for len(words) > 0 {
		option := strings.Trim(words[0], `"'`)
		if option == "--" {
			return words[1:], true
		}
		if strings.HasPrefix(option, "--") {
			requiresArgument, mode, allowed := stdbufCredentialWrapperLongOption(option)
			if !allowed {
				return nil, false
			}
			words = words[1:]
			if requiresArgument {
				if len(words) == 0 {
					return nil, false
				}
				mode = strings.Trim(words[0], `"'`)
				words = words[1:]
			}
			if !isStdbufMode(mode) {
				return nil, false
			}
			continue
		}
		if len(option) < 2 || option[0] != '-' || option == "-" {
			return words, true
		}
		if option[1] != 'e' && option[1] != 'i' && option[1] != 'o' {
			return nil, false
		}
		mode := strings.TrimPrefix(option[2:], "=")
		words = words[1:]
		if mode == "" {
			if len(words) == 0 {
				return nil, false
			}
			mode = strings.Trim(words[0], `"'`)
			words = words[1:]
		}
		if !isStdbufMode(mode) {
			return nil, false
		}
	}
	return words, true
}

func stdbufCredentialWrapperLongOption(option string) (bool, string, bool) {
	name, value, attached := strings.Cut(option, "=")
	switch name {
	case "--error", "--input", "--output":
		return !attached, value, !attached || value != ""
	default:
		return false, "", false
	}
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

func consumeSetsidCredentialWrapper(words []string) ([]string, bool) {
	for len(words) > 0 {
		option := strings.Trim(words[0], `"'`)
		if option == "--" {
			return words[1:], true
		}
		if len(option) < 2 || option[0] != '-' || option == "-" {
			return words, true
		}
		if strings.HasPrefix(option, "--") {
			switch option {
			case "--ctty", "--fork", "--wait":
				words = words[1:]
				continue
			default:
				return nil, false
			}
		}
		for _, flag := range option[1:] {
			if !strings.ContainsRune("cfw", flag) {
				return nil, false
			}
		}
		words = words[1:]
	}
	return words, true
}

func consumeIoniceCredentialWrapper(words []string) ([]string, bool) {
	for len(words) > 0 {
		option := strings.Trim(words[0], `"'`)
		if option == "--" {
			return words[1:], true
		}
		if len(option) < 2 || option[0] != '-' || option == "-" {
			return words, true
		}
		if strings.HasPrefix(option, "--") {
			name, value, attached := strings.Cut(option, "=")
			switch name {
			case "--class", "--classdata":
				words = words[1:]
				if attached {
					if value == "" {
						return nil, false
					}
					continue
				}
				if len(words) == 0 {
					return nil, false
				}
				words = words[1:]
				continue
			case "--ignore":
				if attached {
					return nil, false
				}
				words = words[1:]
				continue
			case "--pid", "--pgid", "--uid", "--help", "--version":
				return nil, false
			default:
				return nil, false
			}
		}

		requiresArgument := false
		for index := 1; index < len(option); index++ {
			switch option[index] {
			case 't':
				continue
			case 'c', 'n':
				requiresArgument = index == len(option)-1
				index = len(option)
			case 'p', 'P', 'u', 'h', 'V':
				return nil, false
			default:
				return nil, false
			}
		}
		words = words[1:]
		if requiresArgument {
			if len(words) == 0 {
				return nil, false
			}
			words = words[1:]
		}
	}
	return words, true
}

func consumeCaffeinateCredentialWrapper(words []string) ([]string, bool) {
	for len(words) > 0 {
		option := strings.Trim(words[0], `"'`)
		if option == "--" {
			return words[1:], true
		}
		if len(option) < 2 || option[0] != '-' || option == "-" {
			return words, true
		}
		argumentFlag := byte(0)
		argument := ""
		for index := 1; index < len(option); index++ {
			switch option[index] {
			case 'd', 'i', 'm', 's', 'u':
				continue
			case 't', 'w':
				argumentFlag = option[index]
				argument = option[index+1:]
				index = len(option)
			default:
				return nil, false
			}
		}
		words = words[1:]
		if argumentFlag != 0 {
			if argument == "" {
				if len(words) == 0 {
					return nil, false
				}
				argument = strings.Trim(words[0], `"'`)
				words = words[1:]
			}
			if argumentFlag == 't' && !isCaffeinateTimeoutArgument(argument) {
				return nil, false
			}
			if argumentFlag == 'w' && !isCaffeinatePIDArgument(argument) {
				return nil, false
			}
		}
	}
	return words, true
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
		case "--find", "--help", "--version", "-find", "-help", "-version", "-f", "-h":
			return nil, false
		default:
			if strings.HasPrefix(option, "--show-sdk-") || strings.HasPrefix(option, "-show-sdk-") || option == "--show-toolchain-path" || option == "-show-toolchain-path" {
				return nil, false
			}
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

func consumeChrootCredentialWrapper(words []string) ([]string, bool) {
	for len(words) > 0 {
		option := strings.Trim(words[0], `"'`)
		if option == "--" {
			words = words[1:]
			break
		}
		if len(option) < 2 || option[0] != '-' || option == "-" {
			break
		}
		if option[1] == '-' {
			name, value, attached := strings.Cut(option, "=")
			switch name {
			case "--groups":
				if attached {
					words = words[1:]
					continue
				}
				if len(words) < 2 {
					return nil, false
				}
				words = words[2:]
			case "--userspec":
				if attached {
					if value == "" {
						return nil, false
					}
					words = words[1:]
					continue
				}
				if len(words) < 2 {
					return nil, false
				}
				words = words[2:]
			case "--skip-chdir":
				if attached {
					return nil, false
				}
				words = words[1:]
			case "--help", "--version":
				return nil, false
			default:
				return nil, false
			}
			continue
		}

		requiresArgument := false
		for index := 1; index < len(option); index++ {
			switch option[index] {
			case 'n':
				continue
			case 'G', 'g', 'u':
				requiresArgument = index == len(option)-1
				index = len(option)
			default:
				return nil, false
			}
		}
		words = words[1:]
		if requiresArgument {
			if len(words) == 0 {
				return nil, false
			}
			words = words[1:]
		}
	}
	if len(words) == 0 || strings.Trim(words[0], `"'`) == "" {
		return nil, false
	}
	return words[1:], true
}

func consumeTimeoutCredentialWrapper(words []string) ([]string, bool) {
	for len(words) > 0 {
		option := strings.Trim(words[0], `"'`)
		if option == "--" {
			words = words[1:]
			break
		}
		if len(option) < 2 || option[0] != '-' || option == "-" {
			break
		}

		requiresArgument, allowed := timeoutCredentialWrapperOption(option)
		if !allowed {
			return nil, false
		}
		words = words[1:]
		if requiresArgument {
			if len(words) == 0 {
				return nil, false
			}
			words = words[1:]
		}
	}
	if len(words) == 0 || !isTimeoutDurationWord(strings.Trim(words[0], `"'`)) {
		return nil, false
	}
	return words[1:], true
}

func isTimeoutDurationWord(value string) bool {
	if len(value) > 1 && strings.ContainsRune("smhdSMHD", rune(value[len(value)-1])) {
		value = value[:len(value)-1]
	}
	duration, err := strconv.ParseFloat(value, 64)
	return err == nil && duration >= 0 && !math.IsNaN(duration)
}

func timeoutCredentialWrapperOption(option string) (bool, bool) {
	if strings.HasPrefix(option, "--") {
		name, value, attached := strings.Cut(option, "=")
		switch name {
		case "--kill-after", "--signal":
			return !attached, !attached || value != ""
		case "--foreground", "--preserve-status", "--verbose":
			return false, !attached
		default:
			return false, false
		}
	}
	if len(option) < 2 {
		return false, false
	}
	for index := 1; index < len(option); index++ {
		switch option[index] {
		case 'f', 'p', 'v':
			continue
		case 'k', 's':
			return index == len(option)-1, true
		default:
			return false, false
		}
	}
	return false, true
}

func credentialWrapperOption(wrapper, option string) (bool, bool) {
	if strings.HasPrefix(option, "--") {
		name, _, attached := strings.Cut(option, "=")
		requiresArgument, allowed := credentialWrapperLongOption(wrapper, name)
		return requiresArgument && !attached, allowed
	}

	requiresArgument := false
	for index := 1; index < len(option); index++ {
		requiresValue, allowed := credentialWrapperShortOption(wrapper, option[index])
		if !allowed {
			return false, false
		}
		if requiresValue {
			requiresArgument = index == len(option)-1
			break
		}
	}
	return requiresArgument, true
}

func credentialWrapperShortOption(wrapper string, option byte) (bool, bool) {
	switch wrapper {
	case "sudo":
		if strings.ContainsRune("CDghpRrTtUu", rune(option)) {
			return true, true
		}
		return false, strings.ContainsRune("AbEHiKknPSs", rune(option))
	case "doas":
		if option == 'a' || option == 'u' {
			return true, true
		}
		return false, option == 'n'
	case "command":
		return false, option == 'p'
	case "env":
		if option == 'C' || option == 'P' || option == 'u' {
			return true, true
		}
		// Treat -S as transparent here so the command words inside its split
		// string remain available to the command-scoped redactors.
		return false, option == '0' || option == 'S' || option == 'i' || option == 'v'
	default:
		return false, false
	}
}

func credentialWrapperLongOption(wrapper, option string) (bool, bool) {
	switch wrapper {
	case "sudo":
		switch option {
		case "--chdir", "--close-from", "--command-timeout", "--group", "--host", "--other-user", "--prompt", "--role", "--type", "--user":
			return true, true
		case "--askpass", "--background", "--non-interactive", "--preserve-env", "--remove-timestamp", "--reset-timestamp", "--stdin":
			return false, true
		}
	case "doas", "command":
		return false, false
	case "env":
		switch option {
		case "--argv0", "--chdir", "--unset":
			return true, true
		case "--debug", "--ignore-environment", "--null", "--split-string":
			return false, true
		}
	}
	return false, false
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
			requiresArgument, allowed := credentialWrapperOption("env", word)
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
	spans := splitCredentialShellWordSpans(value)
	words := make([]string, 0, len(spans))
	for _, span := range spans {
		words = append(words, span.value)
	}
	return words
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

		open := -1
		contentStart := -1
		switch {
		case result[index] == '$' && index+1 < len(result) && result[index+1] == '(' &&
			(index+2 >= len(result) || result[index+2] != '('):
			open = index
			contentStart = index + 2
		case result[index] == '`' && isShellBacktickSubstitutionStart(result, index):
			open = index
			contentStart = index + 1
		case (result[index] == '<' || result[index] == '>') && index+1 < len(result) && result[index+1] == '(':
			open = index + 1
			contentStart = index + 2
		case result[index] == '(' && (index+1 >= len(result) || result[index+1] != '(') &&
			(index == 0 || !strings.ContainsRune("$<>=()", rune(result[index-1]))) &&
			!isWithinShellArithmeticExpression(result, index) && !isWithinShellArraySubscript(result, index):
			open = index
			contentStart = index + 1
		}
		if open < 0 {
			continue
		}

		close := findShellCommandSubstitutionEnd(result, open)
		if close < contentStart {
			continue
		}
		inner := result[contentStart:close]
		redacted, innerChanged := redactSensitiveTextDepth(inner, depth+1)
		if innerChanged {
			result = result[:contentStart] + redacted + result[close:]
			close = contentStart + len(redacted)
			changed = true
		}
		index = close
	}
	return result, changed
}

func isWithinShellArithmeticExpression(value string, position int) bool {
	var quote byte
	for index := 0; index < position; index++ {
		if quote == '\'' {
			if value[index] == '\'' {
				quote = 0
			}
			continue
		}
		if value[index] == '\\' {
			index++
			continue
		}
		if quote != 0 {
			if value[index] == quote {
				quote = 0
			}
			continue
		}
		if value[index] == '\'' || value[index] == '"' || value[index] == '`' {
			quote = value[index]
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
	for index := 0; index < position; index++ {
		if quote == '\'' {
			if value[index] == '\'' {
				quote = 0
			}
			continue
		}
		if value[index] == '\\' {
			index++
			continue
		}
		if quote != 0 {
			if value[index] == quote {
				quote = 0
			}
			continue
		}
		if value[index] == '\'' || value[index] == '"' || value[index] == '`' {
			quote = value[index]
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
			}
			continue
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
		if quote != 0 || result[index] != '(' || !isShellSubshellGroupStart(result, index) {
			continue
		}

		close := findShellCommandSubstitutionEnd(result, index)
		if close <= index {
			continue
		}
		contentStart := index + 1
		inner := result[contentStart:close]
		redacted, innerChanged := redactSensitiveTextDepth(inner, depth+1)
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
	result := value
	changed := false
	for start := 0; start < len(result); {
		end := findShellCommandEnd(result, start)
		command := result[start:end]
		contentStart, contentEnd, ok := envSplitCommandStringSpan(command)
		if ok {
			inner := command[contentStart:contentEnd]
			redacted, innerChanged := redactSensitiveTextDepth(inner, depth+1)
			if !innerChanged && strings.Contains(inner, `\`) {
				decoded := decodeEnvSplitStringForRedaction(inner)
				_, innerChanged = redactSensitiveTextDepth(decoded, depth+1)
				if innerChanged {
					redacted = redactionMarker
				}
			}
			if innerChanged {
				command = command[:contentStart] + redacted + command[contentEnd:]
				result = result[:start] + command + result[end:]
				changed = true
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

func envSplitCommandStringSpan(command string) (int, int, bool) {
	spans := splitCredentialShellWordSpans(command)
	words := make([]string, 0, len(spans))
	for _, span := range spans {
		words = append(words, span.value)
	}
	for index, word := range words {
		if commandBaseName(word) != "env" || !isCredentialCommandPrefix(words[:index]) {
			continue
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
			requiresArgument, allowed := credentialWrapperOption("env", option)
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
	result := value
	changed := false
	for start := 0; start < len(result); {
		end := findShellCommandEnd(result, start)
		command := result[start:end]
		contentStart, contentEnd, ok := shellCommandStringSpan(command)
		if ok {
			inner := command[contentStart:contentEnd]
			redacted, innerChanged := redactSensitiveTextDepth(inner, depth+1)
			if innerChanged {
				command = command[:contentStart] + redacted + command[contentEnd:]
				result = result[:start] + command + result[end:]
				changed = true
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

// eval is a shell builtin whose arguments are joined and executed as a new
// command. Unlike sh -c, its command text is often quoted, so the ordinary
// quote-aware substitution pass intentionally leaves it untouched. Redact
// each eval argument recursively while preserving its quoting and spacing.
func redactEvalCommandStrings(value string, depth int) (string, bool) {
	result := value
	changed := false
	for start := 0; start < len(result); {
		end := findShellCommandEnd(result, start)
		command := result[start:end]
		spans := splitCredentialShellWordSpans(command)
		words := make([]string, 0, len(spans))
		for _, span := range spans {
			words = append(words, span.value)
		}
		var replacements []struct {
			start int
			end   int
			text  string
		}
		for index, word := range words {
			if commandBaseName(word) != "eval" || !isCredentialCommandPrefix(words[:index]) {
				continue
			}
			for argumentIndex := index + 1; argumentIndex < len(spans); argumentIndex++ {
				span := spans[argumentIndex]
				inner := command[span.contentStart:span.contentEnd]
				redacted, argumentChanged := redactSensitiveTextDepth(inner, depth+1)
				if argumentChanged && redacted != inner {
					replacements = append(replacements, struct {
						start int
						end   int
						text  string
					}{start: span.contentStart, end: span.contentEnd, text: redacted})
				}
			}
			break
		}
		for index := len(replacements) - 1; index >= 0; index-- {
			replacement := replacements[index]
			command = command[:replacement.start] + replacement.text + command[replacement.end:]
			changed = true
		}
		if len(replacements) > 0 {
			result = result[:start] + command + result[end:]
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

// launchctl submit -p supplies the executable separately from the argv passed
// after --. The first word after -- is argv[0], not an argument to the
// executable. Command-specific redactors therefore cannot see the real
// command boundary in the original text (for example, docker's login flags
// appear after a second `docker` word). Redact a synthetic command containing
// the executable and its arguments, then splice only the argument suffix back
// into the original text so formatting and argv[0] remain unchanged.
func redactLaunchctlSubmitEmbeddedCommands(value string, depth int) (string, bool) {
	result := value
	changed := false
	for start := 0; start < len(result); {
		end := findShellCommandEnd(result, start)
		command := result[start:end]
		redacted, commandChanged := redactLaunchctlSubmitEmbeddedCommand(command, depth)
		if commandChanged {
			result = result[:start] + redacted + result[end:]
			command = redacted
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

func redactLaunchctlSubmitEmbeddedCommand(command string, depth int) (string, bool) {
	spans := splitCredentialShellWordSpans(command)
	words := make([]string, 0, len(spans))
	for _, span := range spans {
		words = append(words, span.value)
	}

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

func isOpenSSLSubcommandName(value string) bool {
	switch strings.ToLower(strings.Trim(value, `"'`)) {
	case "asn1parse", "ca", "ciphers", "cmp", "cms", "configutl", "crl", "crl2pkcs7", "dgst", "dhparam", "dsa", "dsaparam", "ec", "ecparam", "enc", "engine", "errstr", "fipsinstall", "gendsa", "genpkey", "genrsa", "help", "info", "kdf", "list", "mac", "nseq", "ocsp", "passwd", "pkcs12", "pkcs7", "pkcs8", "pkey", "pkeyparam", "pkeyutl", "prime", "provider", "rand", "rehash", "req", "rsa", "rsautl", "s_client", "s_server", "s_time", "sess_id", "skeyutl", "smime", "speed", "spkac", "srp", "storeutl", "ts", "verify", "version", "x509":
		return true
	default:
		return false
	}
}

func shellCommandStringSpan(command string) (int, int, bool) {
	spans := splitCredentialShellWordSpans(command)
	words := make([]string, 0, len(spans))
	for _, span := range spans {
		words = append(words, span.value)
	}
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
			requiresArgument, allowed := shellCommandStringLongOption(option)
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

func shellCommandStringLongOption(option string) (bool, bool) {
	name, value, attached := strings.Cut(option, "=")
	switch name {
	case "--init-file", "--rcfile":
		return !attached, !attached || value != ""
	case "--login", "--noediting", "--noprofile", "--norc", "--posix", "--restricted", "--verbose":
		return false, !attached
	default:
		return false, false
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
	if kubectlIndex < 0 || !isKubectlCommandPrefix(words[:kubectlIndex]) {
		return false
	}

	return isKubectlCreateSecretSubcommand(words[kubectlIndex+1:])
}

func isKubectlCommandPrefix(words []string) bool {
	return isCredentialCommandPrefix(words)
}

func isKubectlCreateSecretSubcommand(words []string) bool {
	for len(words) > 0 {
		word := strings.Trim(words[0], `"'`)
		if word == "--" {
			words = words[1:]
			break
		}
		if !strings.HasPrefix(word, "-") || word == "-" {
			break
		}

		requiresArgument, allowed := kubectlGlobalOption(word)
		if !allowed {
			return false
		}
		words = words[1:]
		if requiresArgument {
			if len(words) == 0 {
				return false
			}
			words = words[1:]
		}
	}

	return len(words) >= 2 &&
		strings.EqualFold(strings.Trim(words[0], `"'`), "create") &&
		strings.EqualFold(strings.Trim(words[1], `"'`), "secret")
}

func kubectlGlobalOption(option string) (bool, bool) {
	if strings.HasPrefix(option, "--") {
		name, _, attached := strings.Cut(option, "=")
		switch name {
		case "--as", "--as-group", "--as-uid", "--cache-dir", "--certificate-authority", "--client-certificate", "--client-key", "--cluster", "--context", "--kubeconfig", "--namespace", "--password", "--profile", "--profile-output", "--request-timeout", "--server", "--tls-server-name", "--token", "--user", "--username", "--v", "--vmodule":
			return !attached, true
		case "--disable-compression", "--insecure-skip-tls-verify", "--match-server-version", "--warnings-as-errors":
			return false, true
		default:
			return false, false
		}
	}
	if len(option) < 2 {
		return false, false
	}
	switch option[1] {
	case 'n', 's', 'v':
		return len(option) == 2, true
	default:
		return false, false
	}
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

func findShellCommandEnd(value string, start int) int {
	var quote byte
	parenDepth := 0
	for i := start; i < len(value); i++ {
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
	redacted := value
	changed := false
	for searchStart := 0; searchStart < len(redacted); {
		match := sensitiveCommandSubstitutionStart.FindStringSubmatchIndex(redacted[searchStart:])
		if match == nil {
			break
		}

		open := searchStart + match[2]
		close := findShellCommandSubstitutionEnd(redacted, open)
		if close < 0 {
			close = len(redacted) - 1
		}
		redacted = redacted[:open] + redactionMarker + redacted[close+1:]
		changed = true
		searchStart = open + len(redactionMarker)
	}
	return redacted, changed
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
