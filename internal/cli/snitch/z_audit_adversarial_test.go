package snitch

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAuditAdversarialCredentialBoundaries(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"curl certificate continuation", "curl --cert \\\n  client.p12:curl-continuation-secret https://example.test", "curl --cert \\\n  client.p12:[REDACTED] https://example.test"},
		{"curl empty certificate path", "curl --cert :empty-path-cert-secret https://example.test", "curl --cert :[REDACTED] https://example.test"},
		{"curl empty proxy certificate path", "curl --proxy-cert :empty-proxy-cert-secret https://example.test", "curl --proxy-cert :[REDACTED] https://example.test"},
		{"curl short certificate path", "curl -E :short-cert-secret https://example.test", "curl -E :[REDACTED] https://example.test"},
		{"curl user continuation", "curl --user \\\n  alice:continued-curl-secret https://example.test", "curl --user \\\n  [REDACTED] https://example.test"},
		{"authorization query preserves URL shape", "https://example.test/auth?authorization=authorization-query-secret&state=ok", "https://example.test/auth?authorization=[REDACTED]&state=ok"},
		{"double encoded password query", "https://example.test/auth?pass%2577ord=double-encoded-query-secret&state=ok", "https://example.test/auth?pass%2577ord=[REDACTED]&state=ok"},
		{"bare auth query", "https://example.test/auth?auth=opaque-auth-query-secret&state=ok", "https://example.test/auth?auth=[REDACTED]&state=ok"},
		{"xml authorization element", "<authorization>xml-authorization-secret</authorization><status>ok</status>", "<authorization>[REDACTED]</authorization><status>ok</status>"},
		{"redis short pass", "redis-cli -a redis-short-secret GET foo", "redis-cli -a [REDACTED] GET foo"},
		{"redis long pass", "redis-cli --pass redis-long-secret GET foo", "redis-cli --pass [REDACTED] GET foo"},
		{"redis attached short option is positional", "redis-cli -akey GET foo", "redis-cli -akey GET foo"},
		{"redis continuation", "redis-cli -a \\\n  redis-continuation-secret PING", "redis-cli -a \\\n  [REDACTED] PING"},
		{"grep password", "grep --password public-word logfile", "grep --password public-word logfile"},
		{"nested curl in echo", "echo \"$(curl --user alice:nested-curl-secret https://example.test)\"", "echo \"$(curl --user [REDACTED] https://example.test)\""},
		{"wrapped curl", "env -- curl --user alice:wrapped-curl-secret https://example.test", "env -- curl --user [REDACTED] https://example.test"},
		{"kubectl arbitrary literal", "kubectl create secret generic demo --from-literal=custom=opaque-kubectl-secret", "kubectl create secret generic demo --from-literal=[REDACTED]"},
		{"openssl pkeyutl passphrase", "openssl pkeyutl -pkeyopt_passin opt:opaque-pkey-passphrase", "openssl pkeyutl -pkeyopt_passin [REDACTED]"},
		{"openssl client PSK", "openssl s_client -psk 00112233445566778899aabbccddeeff", "openssl s_client -psk [REDACTED]"},
		{"eval nested openssl", "eval 'openssl pkcs12 -export -passout pass:opaque-eval-passphrase'", "eval 'openssl pkcs12 -export -passout [REDACTED]'"},
		{"azure connection key", "DefaultEndpointsProtocol=https;AccountName=demo;AccountKey=YWJjZGVmZ2hpamtsbW5vcA==;EndpointSuffix=core.windows.net", "DefaultEndpointsProtocol=https;AccountName=demo;AccountKey=[REDACTED];EndpointSuffix=core.windows.net"},
		{"azure service bus key", "Endpoint=sb://demo.servicebus.windows.net/;SharedAccessKeyName=RootManageSharedAccessKey;SharedAccessKey=opaque-service-bus-key", "Endpoint=sb://demo.servicebus.windows.net/;SharedAccessKeyName=RootManageSharedAccessKey;SharedAccessKey=[REDACTED]"},
		{"echo password", "echo --password public-word", "echo --password public-word"},
		{"launchctl executable boundary", "launchctl submit -l signer -p echo -- echo openssl s_client -psk launchctl-public-secret", "launchctl submit -l signer -p echo -- echo openssl s_client -psk launchctl-public-secret"},
		{"launchctl openssl passwd argv0", "launchctl submit -l signer -p openssl -- passwd -6 launchctl-passphrase", "launchctl submit -l signer -p openssl -- passwd -6 [REDACTED]"},
		{"launchctl openssl pkcs12 argv0", "launchctl submit -l signer -p openssl -- pkcs12 -export -passout pass:launchctl-pkcs12-passphrase", "launchctl submit -l signer -p openssl -- pkcs12 -export -passout [REDACTED]"},
		{"launchctl redis argv0", "launchctl submit -l signer -p redis-cli -- redis-cli -a launchctl-redis-passphrase PING", "launchctl submit -l signer -p redis-cli -- redis-cli -a [REDACTED] PING"},
		{"launchctl docker argv0", "launchctl submit -l signer -p docker -- docker login -p launchctl-docker-passphrase registry.example.test", "launchctl submit -l signer -p docker -- docker login -p [REDACTED] registry.example.test"},
		{"launchctl security argv0", "launchctl submit -l signer -p security -- security unlock-keychain -p launchctl-keychain-passphrase login.keychain", "launchctl submit -l signer -p security -- security unlock-keychain -p [REDACTED] login.keychain"},
		{"launchctl sshpass argv0", "launchctl submit -l signer -p sshpass -- sshpass -p launchctl-ssh-passphrase ssh user@example.test", "launchctl submit -l signer -p sshpass -- sshpass -p [REDACTED] ssh user@example.test"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, changed := redactSensitiveText(test.input)
			if got != test.want {
				t.Fatalf("redactSensitiveText(%q) = %q, changed=%t; want %q", test.input, got, changed, test.want)
			}
			again, changedAgain := redactSensitiveText(got)
			if again != got || changedAgain {
				t.Fatalf("redaction is not idempotent: second result %q, changed=%t", again, changedAgain)
			}
		})
	}
}

func TestAuditMalformedKubernetesSecretJSONStopsAtUnmatchedOuterContainer(t *testing.T) {
	var input strings.Builder
	input.WriteString(`{"kind":"Secret","data":`)
	for index := 0; index < 4000; index++ {
		input.WriteByte('{')
	}
	for index := 0; index < 4000; index++ {
		input.WriteByte('}')
	}

	value := input.String()
	if end := findJSONContainerEndAtDepthStrict(value, strings.IndexByte(value, '{'), 0); end >= 0 {
		t.Fatalf("unmatched outer JSON object reported close at %d", end)
	}
	got, changed := redactSensitiveText(value)
	if changed || got != value {
		t.Fatalf("malformed JSON changed unexpectedly: changed=%t, output prefix=%q", changed, got[:min(len(got), 120)])
	}
}

func TestAuditLaunchctlEmbeddedCommandFailsClosedAtDepthLimit(t *testing.T) {
	input := "launchctl submit -l signer -p openssl -- signer pkcs12 -export -passout pass:launchctl-depth-limit-passphrase"
	got, changed := redactSensitiveTextDepth(input, maxShellRedactionDepth)
	if !changed || strings.Contains(got, "launchctl-depth-limit-passphrase") || !strings.Contains(got, redactionMarker) {
		t.Fatalf("depth-limited launchctl redaction = %q, changed=%t; want fail-closed marker", got, changed)
	}
	gotAgain, changedAgain := redactSensitiveText(got)
	if changedAgain || gotAgain != got {
		t.Fatalf("depth-limited launchctl redaction is not idempotent: second result %q, changed=%t", gotAgain, changedAgain)
	}
}

func BenchmarkAuditMalformedKubernetesSecretJSON(b *testing.B) {
	var input strings.Builder
	input.WriteString(`{"kind":"Secret","data":`)
	for index := 0; index < 4000; index++ {
		input.WriteByte('{')
	}
	for index := 0; index < 4000; index++ {
		input.WriteByte('}')
	}
	value := input.String()
	b.ReportAllocs()
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		redactSensitiveText(value)
	}
}

func TestAuditDuplicateSearchSinkRedactsRedisCredential(t *testing.T) {
	const secret = "redis-sink-secret"
	var searchQuery string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		searchQuery = r.URL.Query().Get("q")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[]}`))
	}))
	defer server.Close()

	originalBase := githubAPIBase
	t.Cleanup(func() { setGitHubAPIBase(originalBase) })
	setGitHubAPIBase(server.URL)

	if _, err := searchIssues(t.Context(), newSnitchTestDeps(), "synthetic-token", "redis-cli -a "+secret+" GET foo"); err != nil {
		t.Fatalf("searchIssues() error: %v", err)
	}
	if strings.Contains(searchQuery, secret) {
		t.Fatalf("duplicate-search query leaked the credential: %q", searchQuery)
	}
	if !strings.Contains(searchQuery, "redis-cli -a [REDACTED] GET foo") {
		t.Fatalf("duplicate-search query = %q, want scoped redaction", searchQuery)
	}
}
