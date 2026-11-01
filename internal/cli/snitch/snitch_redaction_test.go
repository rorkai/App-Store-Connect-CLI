package snitch

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

func TestRedactSensitiveTextPatterns(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "authorization header",
			input: "request failed with authorization=Basic dXNlcjpwYXNzd29yZA== after retry",
			want:  "request failed with Authorization: [REDACTED] after retry",
		},
		{
			name:  "single quoted authorization header argument",
			input: `curl -H 'Authorization: Bearer opaque-secret' https://example.test`,
			want:  `curl -H 'Authorization: [REDACTED]' https://example.test`,
		},
		{
			name:  "compound quoted authorization header argument",
			input: `curl -H 'Authorization: Bearer 'opaque-secret https://example.test`,
			want:  `curl -H "Authorization: [REDACTED]" https://example.test`,
		},
		{
			name:  "compound authorization header name and value",
			input: `curl --header Auth'oriz'ation:'Bearer opaque-secret' https://example.test`,
			want:  `curl --header "Authorization: [REDACTED]" https://example.test`,
		},
		{
			name:  "quoted curl API key header",
			input: `curl -H "X-API-Key: opaque-secret" https://example.test`,
			want:  `curl -H "X-API-Key: [REDACTED]" https://example.test`,
		},
		{
			name:  "quoted curl auth token header",
			input: `curl --header 'X-Auth-Token: opaque-secret' https://example.test`,
			want:  `curl --header "X-Auth-Token: [REDACTED]" https://example.test`,
		},
		{
			name:  "outgoing HTTP trace API key header",
			input: "> X-API-Key: opaqueTraceSecret\n> Content-Type: application/json",
			want:  "> X-API-Key: [REDACTED]\n> Content-Type: application/json",
		},
		{
			name:  "incoming HTTP trace auth token header",
			input: "< X-Auth-Token: opaqueTraceSecret\n< Content-Type: application/json",
			want:  "< X-Auth-Token: [REDACTED]\n< Content-Type: application/json",
		},
		{
			name:  "parameterized authorization header",
			input: "Authorization: AWS4-HMAC-SHA256 Credential=AKIAEXAMPLE/20260819/region/service/aws4_request, SignedHeaders=host;x-amz-date, Signature=abcdef0123456789",
			want:  "Authorization: [REDACTED]",
		},
		{
			name:  "digest authorization header",
			input: `Authorization: Digest username="user", nonce="nonce-value", response="credential-value"`,
			want:  "Authorization: [REDACTED]",
		},
		{
			name:  "signature authorization header with arbitrary first parameter",
			input: `Authorization: Signature keyId="my-key",algorithm="rsa-sha256",signature="credential-value"`,
			want:  "Authorization: [REDACTED]",
		},
		{
			name:  "arbitrary authorization token scheme",
			input: `Authorization: Negotiate dG9rZW4tc2VjcmV0`,
			want:  "Authorization: [REDACTED]",
		},
		{
			name:  "alphabetic arbitrary authorization token scheme",
			input: `Authorization: Negotiate opaqueSecretValue`,
			want:  "Authorization: [REDACTED]",
		},
		{
			name:  "folded authorization header",
			input: "Authorization: Bearer opaque-head\r\n opaque-tail\r\nstatus: failed",
			want:  "Authorization: [REDACTED]\r\nstatus: failed",
		},
		{
			name:  "folded cookie header",
			input: "< Cookie: dslang=US-EN;\r\n myacinfo=opaque-tail-secret\r\nstatus: failed",
			want:  "< Cookie: [REDACTED]\r\nstatus: failed",
		},
		{
			name:  "folded embedded session header",
			input: "request failed with scnt: opaque-head\r\n opaque-tail-secret\r\nstatus: failed",
			want:  "request failed with scnt: [REDACTED]\r\nstatus: failed",
		},
		{
			name:  "cookie request header",
			input: "Cookie: myacinfo=super-session-secret; dslang=US-EN",
			want:  "Cookie: [REDACTED]",
		},
		{
			name:  "CGI authorization environment variable",
			input: "HTTP_AUTHORIZATION=Bearer opaque-bearer-secret",
			want:  "HTTP_AUTHORIZATION=[REDACTED]",
		},
		{
			name:  "redirected CGI authorization environment variable",
			input: "REDIRECT_HTTP_AUTHORIZATION=Basic dXNlcjpwYXNz",
			want:  "REDIRECT_HTTP_AUTHORIZATION=[REDACTED]",
		},
		{
			name:  "CGI cookie environment variable",
			input: "HTTP_COOKIE=sessionid=opaque-session-secret; locale=en-US",
			want:  "HTTP_COOKIE=[REDACTED]",
		},
		{
			name:  "cookie header embedded in diagnostic line",
			input: "request failed with Cookie: myacinfo=opaque-session-secret after retry",
			want:  "request failed with Cookie: [REDACTED] after retry",
		},
		{
			name:  "continuation header embedded in diagnostic line",
			input: "request failed with scnt: opaque-continuation-secret after retry",
			want:  "request failed with scnt: [REDACTED] after retry",
		},
		{
			name:  "parenthesized continuation header embedded in diagnostic line",
			input: "request failed (scnt: opaque-continuation-secret) after retry",
			want:  "request failed (scnt: [REDACTED]) after retry",
		},
		{
			name:  "set cookie response header",
			input: "< Set-Cookie: myacinfo=super-response-secret; Path=/; Secure; HttpOnly",
			want:  "< Set-Cookie: [REDACTED]",
		},
		{
			name:  "two factor continuation header",
			input: "< scnt: opaque-lowercase-continuation",
			want:  "< scnt: [REDACTED]",
		},
		{
			name:  "account session continuation header",
			input: "< X-Apple-ID-Session-Id: opaque-lowercase-session",
			want:  "< X-Apple-ID-Session-Id: [REDACTED]",
		},
		{
			name:  "quoted continuation header argument",
			input: `curl -H "scnt: opaque-lowercase-continuation" https://example.test`,
			want:  `curl -H "scnt: [REDACTED]" https://example.test`,
		},
		{
			name:  "portal csrf header",
			input: "< csrf: opaque-lowercase-token-value",
			want:  "< csrf: [REDACTED]",
		},
		{
			name:  "portal csrf timestamp header",
			input: "< csrf_ts: opaque-lowercase-timestamp-value",
			want:  "< csrf_ts: [REDACTED]",
		},
		{
			name:  "structured portal csrf headers",
			input: `{"csrf":"opaque-lowercase-token-value","csrf_ts":"opaque-lowercase-timestamp-value","status":"failed"}`,
			want:  `{"csrf":"[REDACTED]","csrf_ts":"[REDACTED]","status":"failed"}`,
		},
		{
			name:  "quoted cookie header argument",
			input: `curl -H "Cookie: myacinfo=super-session-secret; dslang=US-EN" https://example.test`,
			want:  `curl -H "Cookie: [REDACTED]" https://example.test`,
		},
		{
			name:  "continued quoted cookie header argument",
			input: "curl -H \"Cookie: myacinfo=super\\\nsecret\" https://example.test",
			want:  `curl -H "Cookie: [REDACTED]" https://example.test`,
		},
		{
			name:  "attached cookie header argument",
			input: `curl -HCookie:myacinfo=super-session-secret https://example.test`,
			want:  `curl -HCookie:[REDACTED] https://example.test`,
		},
		{
			name:  "attached service credential header argument",
			input: `curl -HX-Apple-Widget-Key:header-service-secret https://example.test`,
			want:  `curl -HX-Apple-Widget-Key:[REDACTED] https://example.test`,
		},
		{
			name:  "unquoted separated cookie header argument",
			input: `curl -H Cookie:myacinfo=super-session-secret https://example.test`,
			want:  `curl -H Cookie:[REDACTED] https://example.test`,
		},
		{
			name:  "unquoted long cookie header argument",
			input: `curl --header=Cookie:myacinfo=super-session-secret https://example.test`,
			want:  `curl --header=Cookie:[REDACTED] https://example.test`,
		},
		{
			name:  "unquoted proxy cookie header argument",
			input: `curl --proxy-header Cookie:myacinfo=super-session-secret https://example.test`,
			want:  `curl --proxy-header Cookie:[REDACTED] https://example.test`,
		},
		{
			name:  "curl long cookie data argument",
			input: `curl --cookie 'myacinfo=super-session-secret; dslang=US-EN' https://example.test`,
			want:  `curl --cookie [REDACTED] https://example.test`,
		},
		{
			name:  "curl short cookie data argument",
			input: `curl -b "myacinfo=super-session-secret" https://example.test`,
			want:  `curl -b [REDACTED] https://example.test`,
		},
		{
			name:  "curl cookie equals data argument",
			input: `curl --cookie="myacinfo=super-session-secret" https://example.test`,
			want:  `curl --cookie=[REDACTED] https://example.test`,
		},
		{
			name:  "quoted curl data credential with spaces",
			input: `curl --data 'password=opaque secret tail' https://example.test`,
			want:  `curl --data 'password=[REDACTED]' https://example.test`,
		},
		{
			name:  "quoted curl data urlencode credential with spaces",
			input: `curl --data-urlencode "client_secret=opaque secret tail" https://example.test`,
			want:  `curl --data-urlencode "client_secret=[REDACTED]" https://example.test`,
		},
		{
			name:  "quoted curl short multipart credential with spaces",
			input: `curl -F 'password=opaque secret tail' https://example.test`,
			want:  `curl -F 'password=[REDACTED]' https://example.test`,
		},
		{
			name:  "quoted curl attached short multipart credential with spaces",
			input: `curl -F'password=opaque secret tail' https://example.test`,
			want:  `curl -F'password=[REDACTED]' https://example.test`,
		},
		{
			name:  "quoted curl multipart credential with spaces",
			input: `curl --form "client_secret=opaque secret tail" https://example.test`,
			want:  `curl --form "client_secret=[REDACTED]" https://example.test`,
		},
		{
			name:  "quoted curl literal multipart credential with spaces",
			input: `curl --form-string 'password=opaque secret tail' https://example.test`,
			want:  `curl --form-string 'password=[REDACTED]' https://example.test`,
		},
		{
			name:  "quoted curl literal multipart credential equals form",
			input: `curl --form-string='password=opaque secret tail' https://example.test`,
			want:  `curl --form-string='password=[REDACTED]' https://example.test`,
		},
		{
			name:  "curl attached short cookie data argument",
			input: `curl -bmyacinfo=super-session-secret https://example.test`,
			want:  `curl -b[REDACTED] https://example.test`,
		},
		{
			name:  "PowerShell escaped quote in credential value",
			input: "asc signing sync --password \"opaque`\"suffix\" --verbose",
			want:  "asc signing sync --password [REDACTED] --verbose",
		},
		{
			name:  "PowerShell plaintext SecureString assignment",
			input: `$password = ConvertTo-SecureString "opaque secret" -AsPlainText -Force`,
			want:  `$password = [REDACTED] -AsPlainText -Force`,
		},
		{
			name:  "PowerShell explicit plaintext SecureString assignment",
			input: `$client_secret = ConvertTo-SecureString -String "opaque secret" -AsPlainText -Force`,
			want:  `$client_secret = [REDACTED] -AsPlainText -Force`,
		},
		{
			name:  "PowerShell plaintext SecureString switches before explicit input",
			input: `$password = ConvertTo-SecureString -AsPlainText -String "opaque secret" -Force`,
			want:  `$password = [REDACTED] -Force`,
		},
		{
			name:  "PowerShell plaintext SecureString switches before positional input",
			input: `$client_secret = ConvertTo-SecureString -Force "opaque secret" -AsPlainText`,
			want:  `$client_secret = [REDACTED] -AsPlainText`,
		},
		{
			name:  "Netscape cookie jar credentials",
			input: ".example.test\tTRUE\t/\tTRUE\t2147483647\tJSESSIONID\topaque-java-session\n#HttpOnly_.example.test\tFALSE\t/\tTRUE\t0\tcsrftoken\topaque-csrf-token\n.example.test\tFALSE\t/\tFALSE\t0\tlocale\ten-US",
			want:  ".example.test\tTRUE\t/\tTRUE\t2147483647\tJSESSIONID\t[REDACTED]\n#HttpOnly_.example.test\tFALSE\t/\tTRUE\t0\tcsrftoken\t[REDACTED]\n.example.test\tFALSE\t/\tFALSE\t0\tlocale\t[REDACTED]",
		},
		{
			name:  "password file credential row",
			input: `db.example.com:5432:app:alice:opaque-password`,
			want:  `db.example.com:5432:app:alice:[REDACTED]`,
		},
		{
			name:  "password file credential row with escapes",
			input: `db\:primary.example.com:5432:app:alice:opaque\:password\\tail`,
			want:  `db\:primary.example.com:5432:app:alice:[REDACTED]`,
		},
		{
			name:  "secret answer flag",
			input: `tool --secret-answer "opaque recovery answer" --verbose`,
			want:  `tool --secret-answer [REDACTED] --verbose`,
		},
		{
			name:  "secret answer equals flag",
			input: `tool --secret-answer=opaque-recovery-answer --verbose`,
			want:  `tool --secret-answer=[REDACTED] --verbose`,
		},
		{
			name:  "curl certificate password argument",
			input: `curl --cert client.p12:supersensitive https://example.test`,
			want:  `curl --cert client.p12:[REDACTED] https://example.test`,
		},
		{
			name:  "curl certificate password equals argument",
			input: `curl --cert="client cert.p12:secret with spaces" https://example.test`,
			want:  `curl --cert="client cert.p12:[REDACTED]" https://example.test`,
		},
		{
			name:  "curl attached short certificate password argument",
			input: `curl -Eclient.p12:supersensitive https://example.test`,
			want:  `curl -Eclient.p12:[REDACTED] https://example.test`,
		},
		{
			name:  "curl certificate password quoted suffix",
			input: `curl --cert client.p12:'supersensitive password' https://example.test`,
			want:  `curl --cert client.p12:[REDACTED] https://example.test`,
		},
		{
			name:  "curl certificate separately quoted path",
			input: `curl --cert "client cert.p12":supersensitive https://example.test`,
			want:  `curl --cert "client cert.p12":[REDACTED] https://example.test`,
		},
		{
			name:  "curl proxy certificate password argument",
			input: `curl --proxy-cert client.p12:opaque-proxy-secret https://example.test`,
			want:  `curl --proxy-cert client.p12:[REDACTED] https://example.test`,
		},
		{
			name:  "persisted session cookie values",
			input: `{"cookies":{"https://appstoreconnect.apple.com":[{"name":"myacinfo","value":"super-session-secret","path":"/"},{"name":"dqsid","value":"second-session-secret"}]},"version":1}`,
			want:  `{"cookies":{"https://appstoreconnect.apple.com":[{"name":"myacinfo","value":"[REDACTED]","path":"/"},{"name":"dqsid","value":"[REDACTED]"}]},"version":1}`,
		},
		{
			name:  "persisted session cookie value before name",
			input: `{"cookies":{"https://appstoreconnect.apple.com":[{"value":"super-session-secret","name":"myacinfo"}]},"version":1}`,
			want:  `{"cookies":{"https://appstoreconnect.apple.com":[{"value":"[REDACTED]","name":"myacinfo"}]},"version":1}`,
		},
		{
			name:  "escaped persisted session cookie value",
			input: `cache {\"cookies\":{\"https://appstoreconnect.apple.com\":[{\"name\":\"myacinfo\",\"value\":\"super-session-secret\",\"path\":\"/\"}]}}`,
			want:  `cache {\"cookies\":{\"https://appstoreconnect.apple.com\":[{\"name\":\"myacinfo\",\"value\":\"[REDACTED]\",\"path\":\"/\"}]}}`,
		},
		{
			name:  "browser cookie export array",
			input: `{"cookies":[{"name":"sessionid","value":"opaque-session-secret","domain":"example.test"}],"version":1}`,
			want:  `{"cookies":[{"name":"sessionid","value":"[REDACTED]","domain":"example.test"}],"version":1}`,
		},
		{
			name:  "escaped browser cookie export array",
			input: `cache {\"cookies\":[{\"name\":\"sessionid\",\"value\":\"opaque-session-secret\",\"domain\":\"example.test\"}]}`,
			want:  `cache {\"cookies\":[{\"name\":\"sessionid\",\"value\":\"[REDACTED]\",\"domain\":\"example.test\"}]}`,
		},
		{
			name:  "upload operation request header values",
			input: `{"uploadOperations":[{"method":"PUT","requestHeaders":[{"name":"Authorization","value":"opaque-upload-secret"},{"name":"x-amz-checksum-sha256","value":"checksum-capability"}],"length":12}],"status":"pending"}`,
			want:  `{"uploadOperations":[{"method":"PUT","requestHeaders":[{"name":"Authorization","value":"[REDACTED]"},{"name":"x-amz-checksum-sha256","value":"[REDACTED]"}],"length":12}],"status":"pending"}`,
		},
		{
			name:  "escaped upload operation request header value",
			input: `response {\"requestHeaders\":[{\"name\":\"x-upload-token\",\"value\":\"escaped-upload-secret\"}],\"method\":\"PUT\"}`,
			want:  `response {\"requestHeaders\":[{\"name\":\"x-upload-token\",\"value\":\"[REDACTED]\"}],\"method\":\"PUT\"}`,
		},
		{
			name:  "Kubernetes environment credential name value pair",
			input: `{"name":"DB_PASSWORD","value":"opaqueAlphabeticSecret","valueFrom":null}`,
			want:  `{"name":"DB_PASSWORD","value":"[REDACTED]","valueFrom":null}`,
		},
		{
			name:  "escaped reversed Kubernetes environment credential name value pair",
			input: `deployment {\"value\":\"escapedAlphabeticSecret\",\"name\":\"API_TOKEN\"}`,
			want:  `deployment {\"value\":\"[REDACTED]\",\"name\":\"API_TOKEN\"}`,
		},
		{
			name:  "nested Kubernetes environment credential preserves diagnostic pair",
			input: `{"env":[{"name":"DB_PASSWORD","value":"nestedAlphabeticSecret"}],"diagnostic":{"name":"failure","value":"preserve this explanation"}}`,
			want:  `{"env":[{"name":"DB_PASSWORD","value":"[REDACTED]"}],"diagnostic":{"name":"failure","value":"preserve this explanation"}}`,
		},
		{
			name: "Kubernetes YAML environment credential name value pair",
			input: `env:
  - name: DB_PASSWORD
    value: opaqueAlphabeticSecret
diagnostic:
  name: failure
  value: preserve this explanation`,
			want: `env:
  - name: DB_PASSWORD
    value: [REDACTED]
diagnostic:
  name: failure
  value: preserve this explanation`,
		},
		{
			name: "reversed Kubernetes YAML environment credential name value pair",
			input: `env:
  - value: reversedAlphabeticSecret
    name: API_TOKEN`,
			want: `env:
  - value: [REDACTED]
    name: API_TOKEN`,
		},
		{
			name: "Kubernetes YAML block scalar environment credential name",
			input: `env:
  - name: |-
      DB_PASSWORD
    value: opaqueBlockNameSecret`,
			want: `env:
  - name: |-
      DB_PASSWORD
    value: [REDACTED]`,
		},
		{
			name: "Kubernetes YAML folded tagged block scalar environment credential name",
			input: `env:
  - name: !!str >-
      API_TOKEN
    value: opaqueFoldedBlockNameSecret`,
			want: `env:
  - name: !!str >-
      API_TOKEN
    value: [REDACTED]`,
		},
		{
			name: "Kubernetes YAML block scalar credential name with blank line",
			input: `env:
  - name: |-

      DB_PASSWORD
    value: opaqueBlankBlockNameSecret`,
			want: `env:
  - name: |-

      DB_PASSWORD
    value: [REDACTED]`,
		},
		{
			name:  "Kubernetes YAML flow environment credential name value pair",
			input: `env: [{value: flowAlphabeticSecret, name: DB_PASSWORD}]`,
			want:  `env: [{value: [REDACTED], name: DB_PASSWORD}]`,
		},
		{
			name: "Kubernetes YAML block environment credential preserves public sibling",
			input: `env:
  - name: DB_PASSWORD
    value: |-
      firstSecretLine
      secondSecretLine
  - name: PUBLIC_CONFIG
    value: keep-public`,
			want: `env:
  - name: DB_PASSWORD
    value: [REDACTED]
  - name: PUBLIC_CONFIG
    value: keep-public`,
		},
		{
			name:  "quoted Kubernetes YAML flow environment credential",
			input: `env: [{name: "API_TOKEN", value: "quotedAlphabeticSecret"}]`,
			want:  `env: [{name: "API_TOKEN", value: "[REDACTED]"}]`,
		},
		{
			name: "aliased Kubernetes YAML environment credential name",
			input: `credential: &credential DB_PASSWORD
env:
  - name: *credential
    value: opaqueAliasSecret`,
			want: `credential: &credential DB_PASSWORD
env:
  - name: *credential
    value: [REDACTED]`,
		},
		{
			name: "aliased Kubernetes YAML flow environment credential name",
			input: `credential: &credential API_TOKEN
env: [{name: *credential, value: opaqueFlowAliasSecret}]`,
			want: `credential: &credential API_TOKEN
env: [{name: *credential, value: [REDACTED]}]`,
		},
		{
			name:  "truncated upload operation request header value",
			input: `{"requestHeaders":[{"name":"Authorization","value":"opaque-upload-secret`,
			want:  `{"requestHeaders":[{"name":"Authorization","value":"[REDACTED]`,
		},
		{
			name:  "escaped truncated upload operation request header value",
			input: `response {\"requestHeaders\":[{\"name\":\"Authorization\",\"value\":\"escaped-upload-secret`,
			want:  `response {\"requestHeaders\":[{\"name\":\"Authorization\",\"value\":\"[REDACTED]`,
		},
		{
			name:  "structured credential headers",
			input: `{"Authorization":"Basic c3VwZXJzZWNyZXQ=","Cookie":"myacinfo=super-session-secret","status":"failed"}`,
			want:  `{"Authorization":"[REDACTED]","Cookie":"[REDACTED]","status":"failed"}`,
		},
		{
			name:  "alphabetic API key authorization header",
			input: "Authorization: ApiKey opaqueSecretValue\nstatus: failed",
			want:  "Authorization: [REDACTED]\nstatus: failed",
		},
		{
			name:  "go formatted credential header map",
			input: `request headers: map[Cookie:[myacinfo=opaque-lowercase-secret] Content-Type:[application/json]]`,
			want:  `request headers: map[Cookie:[REDACTED] Content-Type:[application/json]]`,
		},
		{
			name:  "go formatted custom credential header map",
			input: `request headers: map[X-API-Key:[opaqueAlphabeticSecret] X-Auth-Token:[firstToken secondToken] Content-Type:[application/json]]`,
			want:  `request headers: map[X-API-Key:[REDACTED] X-Auth-Token:[REDACTED] Content-Type:[application/json]]`,
		},
		{
			name:  "object valued structured credential",
			input: `{"token":{"type":"bearer","value":"opaque-lowercase-secret"},"status":"failed"}`,
			want:  `{"token":"[REDACTED]","status":"failed"}`,
		},
		{
			name:  "escaped object valued structured credential",
			input: `trace {\"token\":{\"type\":\"bearer\",\"value\":\"opaque-lowercase-secret\"},\"status\":\"failed\"}`,
			want:  `trace {\"token\":\"[REDACTED]\",\"status\":\"failed\"}`,
		},
		{
			name:  "truncated object valued structured credential",
			input: `{"token":{"type":"bearer","value":"opaque-lowercase-secret"`,
			want:  `{"token":"[REDACTED]"`,
		},
		{
			name:  "array-valued authorization header",
			input: `{"Authorization":["Bearer opaque-lowercase-secret"],"status":"failed"}`,
			want:  `{"Authorization":["[REDACTED]"],"status":"failed"}`,
		},
		{
			name:  "array-valued proxy authorization header",
			input: `{"Proxy-Authorization":["Basic dXNlcjpzdXBlcnNlY3JldA=="],"status":"failed"}`,
			want:  `{"Proxy-Authorization":["[REDACTED]"],"status":"failed"}`,
		},
		{
			name:  "escaped array-valued authorization header",
			input: `trace {\"Authorization\":[\"Bearer first-secret\",\"Basic second-secret\"],\"status\":\"failed\"}`,
			want:  `trace {\"Authorization\":[\"[REDACTED]\"],\"status\":\"failed\"}`,
		},
		{
			name:  "heterogeneous credential array",
			input: `{"password":["first-secret",{"value":"second-secret"}],"status":"failed"}`,
			want:  `{"password":["[REDACTED]"],"status":"failed"}`,
		},
		{
			name: "web auth service credentials",
			input: `X-Apple-Widget-Key: header-service-secret
{"authServiceKey":"auth-service-secret","serviceKey":"response-service-secret","status":"failed"}`,
			want: `X-Apple-Widget-Key: [REDACTED]
{"authServiceKey":"[REDACTED]","serviceKey":"[REDACTED]","status":"failed"}`,
		},
		{
			name:  "nested two factor request code",
			input: `{"securityCode":{"code":"123456"},"mode":"sms"}`,
			want:  `{"securityCode":{"code":"[REDACTED]"},"mode":"sms"}`,
		},
		{
			name:  "escaped nested two factor request code",
			input: `trace {\"securityCode\":{\"code\":\"654321\"},\"mode\":\"sms\"}`,
			want:  `trace {\"securityCode\":{\"code\":\"[REDACTED]\"},\"mode\":\"sms\"}`,
		},
		{
			name:  "unescaped structured credential headers",
			input: "{\"Authorization\":\"Basic c3VwZXJzZWNyZXQ=\",\"Cookie\":\"myacinfo=super-session-secret\",\"status\":\"failed\"}",
			want:  "{\"Authorization\":\"[REDACTED]\",\"Cookie\":\"[REDACTED]\",\"status\":\"failed\"}",
		},
		{
			name:  "escaped structured credential headers",
			input: `response {\"Authorization\":\"Basic c3VwZXJzZWNyZXQ=\",\"Set-Cookie\":\"myacinfo=super-session-secret\",\"status\":\"failed\"}`,
			want:  `response {\"Authorization\":\"[REDACTED]\",\"Set-Cookie\":\"[REDACTED]\",\"status\":\"failed\"}`,
		},
		{
			name:  "standalone bearer credential",
			input: "server returned Bearer eyJhbGciOiJFUzI1NiJ9.fake.signature",
			want:  "server returned Bearer [REDACTED]",
		},
		{
			name:  "standalone minimum-length bearer credential",
			input: "server returned Bearer abcd1234",
			want:  "server returned Bearer [REDACTED]",
		},
		{
			name:  "standalone Google API key",
			input: "Google request failed for AIza0123456789abcdefghijklmnopqrstuvwxy",
			want:  "Google request failed for [REDACTED]",
		},
		{
			name:  "standalone Google API key ending in hyphen",
			input: "Google request failed for AIzaAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA-",
			want:  "Google request failed for [REDACTED]",
		},
		{
			name:  "standalone SendGrid API key",
			input: "request failed with SG.abcdefghijklmnopqrstuv.ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopq",
			want:  "request failed with [REDACTED]",
		},
		{
			name:  "standalone notification webhook URL",
			input: "failed webhook https://hooks.slack.com/services/T012/B034/opaque-webhook-secret after retry",
			want:  "failed webhook https://hooks.slack.com/services/[REDACTED] after retry",
		},
		{
			name:  "standalone Discord webhook URL",
			input: "failed webhook https://discord.com/api/webhooks/123456789012345678/opaqueDiscordWebhookToken?wait=true after retry",
			want:  "failed webhook https://discord.com/api/webhooks/123456789012345678/[REDACTED]?wait=true after retry",
		},
		{
			name:  "legacy Discord webhook URL",
			input: "failed webhook https://discordapp.com/api/webhooks/123456789012345678/opaqueLegacyWebhookToken after retry",
			want:  "failed webhook https://discordapp.com/api/webhooks/123456789012345678/[REDACTED] after retry",
		},
		{
			name:  "versioned Discord webhook URL",
			input: "failed webhook https://canary.discord.com/api/v10/webhooks/123456789012345678/opaqueCanaryWebhookToken after retry",
			want:  "failed webhook https://canary.discord.com/api/v10/webhooks/123456789012345678/[REDACTED] after retry",
		},
		{
			name:  "signed URL credentials",
			input: "upload https://example.test/file?part=1&X-Amz-Credential=ACCESS%2F20260819&X-Amz-Signature=abcdef0123456789#result",
			want:  "upload https://example.test/file?part=1&X-Amz-Credential=[REDACTED]&X-Amz-Signature=[REDACTED]#result",
		},
		{
			name:  "client secret URL parameter",
			input: "callback https://example.test/path?client_secret=client-value&state=ready",
			want:  "callback https://example.test/path?client_secret=[REDACTED]&state=ready",
		},
		{
			name:  "refresh token URL parameter",
			input: "callback https://example.test/path?refresh_token=refresh-value&state=ready",
			want:  "callback https://example.test/path?refresh_token=[REDACTED]&state=ready",
		},
		{
			name:  "password URL parameter",
			input: "callback https://example.test/path?password=password-value&state=ready",
			want:  "callback https://example.test/path?password=[REDACTED]&state=ready",
		},
		{
			name:  "percent encoded password URL parameter name",
			input: "callback https://example.test/path?pass%77ord=password-value&state=ready",
			want:  "callback https://example.test/path?pass%77ord=[REDACTED]&state=ready",
		},
		{
			name:  "web auth query credentials",
			input: "authenticate https://example.test/auth?widgetKey=widget-secret&code=123456&scnt=continuation-secret&flow=login",
			want:  "authenticate https://example.test/auth?widgetKey=[REDACTED]&code=[REDACTED]&scnt=[REDACTED]&flow=login",
		},
		{
			name:  "OAuth callback code query credential",
			input: "callback https://example.test/oauth/callback?c%6fde=opaque-oauth-code&state=ready",
			want:  "callback https://example.test/oauth/callback?c%6fde=[REDACTED]&state=ready",
		},
		{
			name:  "private key URL parameter",
			input: "callback https://example.test/path?private_key=private-key-value&state=ready",
			want:  "callback https://example.test/path?private_key=[REDACTED]&state=ready",
		},
		{
			name:  "URL userinfo credentials",
			input: "fetch https://user:p%40ss%2Fword@example.test/private/path?part=1",
			want:  "fetch https://[REDACTED]@example.test/private/path?part=1",
		},
		{
			name:  "URL userinfo with encoded separator",
			input: "fetch sftp://user%3Ap%2540ss@example.test/private/path",
			want:  "fetch sftp://[REDACTED]@example.test/private/path",
		},
		{
			name:  "URL username-only credential",
			input: "fetch https://access-token@example.test/private/path",
			want:  "fetch https://[REDACTED]@example.test/private/path",
		},
		{
			name:  "scp style remote credentials",
			input: "asc signing sync --repo user:supersensitive@github.com:team/certs.git",
			want:  "asc signing sync --repo [REDACTED]@github.com:team/certs.git",
		},
		{
			name:  "private key block",
			input: "before\n-----BEGIN OPENSSH PRIVATE KEY-----\nkey-material\n-----END OPENSSH PRIVATE KEY-----\nafter",
			want:  "before\n[REDACTED PRIVATE KEY]\nafter",
		},
		{
			name:  "PGP private key block",
			input: "before\n-----BEGIN PGP PRIVATE KEY BLOCK-----\nkey-material\n-----END PGP PRIVATE KEY BLOCK-----\nafter",
			want:  "before\n[REDACTED PRIVATE KEY]\nafter",
		},
		{
			name:  "unterminated private key block",
			input: "before\n-----BEGIN PRIVATE KEY-----\ntruncated-key-material",
			want:  "before\n[REDACTED PRIVATE KEY]",
		},
		{
			name:  "shell assignment",
			input: `command CLIENT_SECRET="super secret value" --verbose`,
			want:  "command CLIENT_SECRET=[REDACTED] --verbose",
		},
		{
			name:  "passphrase assignment",
			input: `passphrase = "opaque private key credential"`,
			want:  "passphrase = [REDACTED]",
		},
		{
			name:  "passphrase flag",
			input: `asc signing sync --passphrase "opaque private key credential" --verbose`,
			want:  "asc signing sync --passphrase [REDACTED] --verbose",
		},
		{
			name:  "Command Prompt quoted set assignment",
			input: `set "PASSWORD=opaque secret value" & echo done`,
			want:  `set "PASSWORD=[REDACTED]" & echo done`,
		},
		{
			name:  "Command Prompt unquoted set assignment with spaces",
			input: `set PASSWORD=opaque secret value & echo done`,
			want:  `set PASSWORD=[REDACTED] & echo done`,
		},
		{
			name:  "Command Prompt unquoted set assignment with escaped operator",
			input: `set PASSWORD=opaque ^& secret value && echo done`,
			want:  `set PASSWORD=[REDACTED] && echo done`,
		},
		{
			name:  "Command Prompt quoted set assignment with escaped quote",
			input: `echo preparing & set "PASSWORD=opaque ^" secret value" & echo done`,
			want:  `echo preparing & set "PASSWORD=[REDACTED]" & echo done`,
		},
		{
			name:  "Command Prompt continued quoted set assignment",
			input: "set \"PASSWORD=opaque^\r\nsecret value\" & echo done",
			want:  "set \"PASSWORD=[REDACTED]\" & echo done",
		},
		{
			name:  "unterminated Command Prompt quoted set assignment",
			input: "set \"PASSWORD=opaque secret value\nstatus: failed",
			want:  "set \"PASSWORD=[REDACTED]\nstatus: failed",
		},
		{
			name:  "unquoted assignment before command separator",
			input: `PASSWORD=supersecret; echo next`,
			want:  `PASSWORD=[REDACTED]; echo next`,
		},
		{
			name:  "bare environment dump value with spaces",
			input: `PASSWORD=opaque secret tail`,
			want:  `PASSWORD=[REDACTED]`,
		},
		{
			name:  "uppercase session environment credential",
			input: `AUTOMATION_SESSION=opaque-session-value`,
			want:  `AUTOMATION_SESSION=[REDACTED]`,
		},
		{
			name:  "uppercase session environment credential with spaces",
			input: `AUTOMATION_SESSION=opaque session tail`,
			want:  `AUTOMATION_SESSION=[REDACTED]`,
		},
		{
			name:  "unquoted secret flag before conditional operator",
			input: `asc deploy --password supersecret && echo next`,
			want:  `asc deploy --password [REDACTED] && echo next`,
		},
		{
			name:  "PowerShell escaped whitespace in secret flag",
			input: "asc signing sync --password opaque` secret --verbose",
			want:  "asc signing sync --password [REDACTED] --verbose",
		},
		{
			name:  "PowerShell continued secret flag",
			input: "asc signing sync --password opaque`\nsecret --verbose",
			want:  "asc signing sync --password [REDACTED] --verbose",
		},
		{
			name:  "PowerShell single quoted here string in secret flag",
			input: "asc signing sync --password @'\nopaque-head\nopaque-tail\n'@ --verbose",
			want:  "asc signing sync --password [REDACTED] --verbose",
		},
		{
			name:  "PowerShell double quoted here string in secret flag",
			input: "asc signing sync --password @\"\nopaque-head\nopaque-tail\n\"@ --verbose",
			want:  "asc signing sync --password [REDACTED] --verbose",
		},
		{
			name:  "unterminated PowerShell here string in secret flag",
			input: "asc signing sync --password @'\nopaque-head\nopaque-tail",
			want:  "asc signing sync --password [REDACTED]",
		},
		{
			name:  "PowerShell here string credential assignment",
			input: "$password = @'\nopaque-head\nopaque-tail\n'@\nWrite-Host done",
			want:  "$password = [REDACTED]\nWrite-Host done",
		},
		{
			name:  "scoped PowerShell here string credential assignment",
			input: "$env:PASSWORD = @'\nopaque-head\nopaque-tail\n'@\nWrite-Host done",
			want:  "$env:PASSWORD = [REDACTED]\nWrite-Host done",
		},
		{
			name:  "unterminated PowerShell here string credential assignment",
			input: "$password = @\"\nopaque-head\nopaque-tail",
			want:  "$password = [REDACTED]",
		},
		{
			name:  "PowerShell array credential assignment",
			input: `$password = @("opaque-first", @{ nested = "opaque-second" }) ; Write-Host done`,
			want:  `$password = [REDACTED] ; Write-Host done`,
		},
		{
			name:  "PowerShell hashtable credential assignment",
			input: `$client_secret = @{ primary = "opaque-first"; nested = @("opaque-second") }; Write-Host done`,
			want:  `$client_secret = [REDACTED]; Write-Host done`,
		},
		{
			name:  "braced scoped PowerShell collection credential assignment",
			input: `${global:client_secret} = @("opaque-first", "opaque-second"); Write-Host done`,
			want:  `${global:client_secret} = [REDACTED]; Write-Host done`,
		},
		{
			name:  "unterminated PowerShell collection credential assignment",
			input: `${client_secret} = @("opaque-first", "opaque-tail"`,
			want:  `${client_secret} = [REDACTED]`,
		},
		{
			name:  "Command Prompt continued secret flag",
			input: "asc signing sync --password opaque^\r\nsecret --verbose",
			want:  "asc signing sync --password [REDACTED] --verbose",
		},
		{
			name:  "Command Prompt escaped whitespace in secret flag",
			input: "asc signing sync --password opaque^ secret --verbose",
			want:  "asc signing sync --password [REDACTED] --verbose",
		},
		{
			name:  "XML property list credential string",
			input: `<plist><dict><key>password</key><string>opaque-plist-secret</string><key>status</key><string>failed</string></dict></plist>`,
			want:  `<plist><dict><key>password</key><string>[REDACTED]</string><key>status</key><string>failed</string></dict></plist>`,
		},
		{
			name:  "XML property list credential data",
			input: `<plist><dict><key>password</key><data>b3BhcXVlLXNlY3JldA==</data><key>status</key><string>failed</string></dict></plist>`,
			want:  `<plist><dict><key>password</key><data>[REDACTED]</data><key>status</key><string>failed</string></dict></plist>`,
		},
		{
			name:  "XML entity encoded property list credential key",
			input: `<plist><dict><key>pass&#x77;ord</key><string>opaque-entity-secret</string><key>status</key><string>failed</string></dict></plist>`,
			want:  `<plist><dict><key>pass&#x77;ord</key><string>[REDACTED]</string><key>status</key><string>failed</string></dict></plist>`,
		},
		{
			name:  "XML comment before property list credential value",
			input: `<plist><dict><key>password</key><!-- diagnostic context --><data>b3BhcXVlLXNlY3JldA==</data><key>status</key><string>failed</string></dict></plist>`,
			want:  `<plist><dict><key>password</key><!-- diagnostic context --><data>[REDACTED]</data><key>status</key><string>failed</string></dict></plist>`,
		},
		{
			name:  "XML property list credential container",
			input: `<plist><dict><key>token</key><array><string>first-secret</string><dict><key>value</key><string>second-secret</string></dict></array><key>status</key><string>failed</string></dict></plist>`,
			want:  `<plist><dict><key>token</key><array>[REDACTED]</array><key>status</key><string>failed</string></dict></plist>`,
		},
		{
			name:  "XML credential element",
			input: `<settings><servers><server><password>opaque-xml-secret</password><status>failed</status></server></servers></settings>`,
			want:  `<settings><servers><server><password>[REDACTED]</password><status>failed</status></server></servers></settings>`,
		},
		{
			name:  "XML credential element value attribute",
			input: `<settings><password value="opaque-element-attribute-secret"/><token value='opaque-token-attribute-secret'></token><status value="public"/></settings>`,
			want:  `<settings><password value="[REDACTED]"/><token value='[REDACTED]'></token><status value="public"/></settings>`,
		},
		{
			name:  "XML credential key value attributes",
			input: `<configuration><add key="ClearTextPassword" value="opaque-attribute-secret" /><add key="status" value="failed" /></configuration>`,
			want:  `<configuration><add key="ClearTextPassword" value="[REDACTED]" /><add key="status" value="failed" /></configuration>`,
		},
		{
			name:  "XML credential name and entity encoded attributes",
			input: `<configuration><entry name='pass&#x77;ord' value='opaque-entity-attribute-secret' /><entry name='status' value='failed' /></configuration>`,
			want:  `<configuration><entry name='pass&#x77;ord' value='[REDACTED]' /><entry name='status' value='failed' /></configuration>`,
		},
		{
			name:  "XML credential name attribute with text content",
			input: `<configuration><variable name="DB_PASSWORD">opaqueXmlSecret</variable><variable name="status">failed</variable></configuration>`,
			want:  `<configuration><variable name="DB_PASSWORD">[REDACTED]</variable><variable name="status">failed</variable></configuration>`,
		},
		{
			name:  "XML credential key attribute with text content",
			input: `<configuration><property key='pass&#x77;ord'>opaqueEntitySecret</property><property key='status'>failed</property></configuration>`,
			want:  `<configuration><property key='pass&#x77;ord'>[REDACTED]</property><property key='status'>failed</property></configuration>`,
		},
		{
			name:  "truncated XML credential element",
			input: `<settings><password>opaque-truncated-secret`,
			want:  `<settings><password>[REDACTED]`,
		},
		{
			name:  "truncated XML property list credential value",
			input: `<plist><dict><key>password</key><string>opaque-truncated-plist-secret`,
			want:  `<plist><dict><key>password</key><string>[REDACTED]`,
		},
		{
			name:  "multiword plain yaml scalar",
			input: "password: correct horse battery staple\nstatus: failed",
			want:  "password: [REDACTED]\nstatus: failed",
		},
		{
			name:  "inline structured credential scalar",
			input: "request failed with {password: opaque-secret, status: failed}",
			want:  "request failed with {password: [REDACTED], status: failed}",
		},
		{
			name:  "implicit YAML credential value after comment",
			input: "password:\n# context\n  opaque-secret\nstatus: failed",
			want:  "password: [REDACTED]\nstatus: failed",
		},
		{
			name:  "YAML block scalar hash prefixed content",
			input: "password: |\n  # opaque-secret\nstatus: failed",
			want:  "password: [REDACTED]\nstatus: failed",
		},
		{
			name:  "indentless YAML credential sequence",
			input: "password:\n- opaque-first-secret\n- opaque-second-secret\nstatus: failed",
			want:  "password: [REDACTED]\nstatus: failed",
		},
		{
			name: "Kubernetes Secret data keys",
			input: `apiVersion: v1
kind: Secret
data:
  tls.key: b3BhcXVlLXByaXZhdGUta2V5
  .dockerconfigjson: eyJhdXRocyI6eyJyZWdpc3RyeS5leGFtcGxlIjp7ImF1dGgiOiJvcGFxdWUifX19
status: failed`,
			want: `apiVersion: v1
kind: Secret
data:
  tls.key: [REDACTED]
  .dockerconfigjson: [REDACTED]
status: failed`,
		},
		{
			name: "Kubernetes Secret arbitrary data and stringData keys",
			input: `apiVersion: v1
kind: Secret
data: {config: b3BhcXVlLWNvbmZpZw==}
stringData:
  settings: opaque-kubernetes-string-data
status: failed`,
			want: `apiVersion: v1
kind: Secret
data: {config: [REDACTED]}
stringData:
  settings: [REDACTED]
status: failed`,
		},
		{
			name: "Kubernetes Secret block scalar kind",
			input: `apiVersion: v1
kind: |-
  Secret
data:
  config: opaqueBlockKindSecret
status: failed`,
			want: `apiVersion: v1
kind: |-
  Secret
data:
  config: [REDACTED]
status: failed`,
		},
		{
			name: "Kubernetes Secret folded tagged block scalar kind",
			input: `apiVersion: v1
kind: !!str >-
  Secret
stringData:
  config: opaqueFoldedBlockKindSecret
status: failed`,
			want: `apiVersion: v1
kind: !!str >-
  Secret
stringData:
  config: [REDACTED]
status: failed`,
		},
		{
			name: "Kubernetes Secret data before anchored block scalar kind",
			input: `apiVersion: v1
data:
  config: opaqueDataBeforeBlockKindSecret
kind: &secret-kind |2-
  Secret`,
			want: `apiVersion: v1
data:
  config: [REDACTED]
kind: &secret-kind |2-
  Secret`,
		},
		{
			name: "Kubernetes Secret aliased block scalar kind",
			input: `secretKind: &secret-kind |-
  Secret
kind: *secret-kind
data:
  config: opaqueAliasedBlockKindSecret`,
			want: `secretKind: &secret-kind |-
  Secret
kind: *secret-kind
data:
  config: [REDACTED]`,
		},
		{
			name: "commented Kubernetes YAML document start resets Secret state",
			input: `kind: Secret
data:
  token: opaque-secret
--- # next document
data:
  count: 42
kind: ConfigMap`,
			want: `kind: Secret
data:
  token: [REDACTED]
--- # next document
data:
  count: 42
kind: ConfigMap`,
		},
		{
			name: "commented Kubernetes YAML document end resets Secret state",
			input: `kind: Secret
data:
  token: opaque-secret
... # end document
data:
  count: 42`,
			want: `kind: Secret
data:
  token: [REDACTED]
... # end document
data:
  count: 42`,
		},
		{
			name: "Kubernetes Secret explicit mapping keys",
			input: `? kind
: Secret
? data
:
  config: opaque-explicit-secret`,
			want: `? kind
: Secret
? data
:
  config: [REDACTED]`,
		},
		{
			name: "Kubernetes Secret explicit flow data value",
			input: `? kind
: Secret
? data
: {config: opaque-explicit-flow-secret}`,
			want: `? kind
: Secret
? data
: {config: [REDACTED]}`,
		},
		{
			name: "Kubernetes Secret multiline explicit block scalar data key",
			input: `kind: Secret
data:
  ? >-
    config
  : opaque-multiline-explicit-secret`,
			want: `kind: Secret
data:
  ? >-
    config
  : [REDACTED]`,
		},
		{
			name: "Kubernetes Secret escaped YAML structural keys",
			input: `"k\u0069nd": Secret
"d\u0061ta":
  config: opaque-escaped-key-secret`,
			want: `"k\u0069nd": Secret
"d\u0061ta":
  config: [REDACTED]`,
		},
		{
			name: "Kubernetes Secret kind from inline YAML merge mapping",
			input: `<<: &defaults
  kind: Secret
data:
  config: opaque-merged-kind-secret`,
			want: `<<: &defaults
  kind: Secret
data:
  config: [REDACTED]`,
		},
		{
			name: "Kubernetes Secret kind from aliased YAML merge mapping",
			input: `defaults: &defaults
  kind: Secret
<<: *defaults
data:
  config: opaque-aliased-merge-secret`,
			want: `defaults: &defaults
  kind: Secret
<<: *defaults
data:
  config: [REDACTED]`,
		},
		{
			name: "Kubernetes Secret kind from flow YAML merge mapping",
			input: `defaults: &defaults {kind: Secret}
<<: [*defaults]
data:
  config: opaque-flow-merge-secret`,
			want: `defaults: &defaults {kind: Secret}
<<: [*defaults]
data:
  config: [REDACTED]`,
		},
		{
			name: "Kubernetes Secret data before YAML merge mapping",
			input: `data:
  config: opaque-merge-lookahead-secret
<<: &defaults
  kind: Secret`,
			want: `data:
  config: [REDACTED]
<<: &defaults
  kind: Secret`,
		},
		{
			name: "Kubernetes Secret list item arbitrary data key",
			input: `items:
- kind: Secret
  data:
    config: opaque-list-secret
- kind: ConfigMap
  data:
    config: public-config`,
			want: `items:
- kind: Secret
  data:
    config: [REDACTED]
- kind: ConfigMap
  data:
    config: public-config`,
		},
		{
			name: "Kubernetes Secret data before kind",
			input: `apiVersion: v1
data:
  config: opaque-order-secret
kind: Secret
status: failed`,
			want: `apiVersion: v1
data:
  config: [REDACTED]
kind: Secret
status: failed`,
		},
		{
			name: "Kubernetes Secret data before aliased kind",
			input: `secretKind: &secretKind Secret
data:
  config: opaque-alias-lookahead-secret
kind: *secretKind
status: failed`,
			want: `secretKind: &secretKind Secret
data:
  config: [REDACTED]
kind: *secretKind
status: failed`,
		},
		{
			name: "Kubernetes Secret tagged kind",
			input: `apiVersion: v1
kind: !!str Secret
data:
  config: opaque-tagged-secret
status: failed`,
			want: `apiVersion: v1
kind: !!str Secret
data:
  config: [REDACTED]
status: failed`,
		},
		{
			name: "Kubernetes Secret tagged structural keys",
			input: `apiVersion: v1
!!str kind: Secret
!!str data:
  config: opaque-tagged-key-secret
status: failed`,
			want: `apiVersion: v1
!!str kind: Secret
!!str data:
  config: [REDACTED]
status: failed`,
		},
		{
			name:  "Kubernetes Secret tagged flow structural keys",
			input: `{!!str kind: Secret, !!str data: {config: opaque-tagged-flow-key-secret}, status: failed}`,
			want:  `{!!str kind: Secret, !!str data: {config: [REDACTED]}, status: failed}`,
		},
		{
			name:  "Kubernetes Secret flow root",
			input: `{apiVersion: v1, data: {config: opaque-flow-secret}, kind: Secret, status: failed}`,
			want:  `{apiVersion: v1, data: {config: [REDACTED]}, kind: Secret, status: failed}`,
		},
		{
			name:  "Kubernetes Secret multiline flow root",
			input: "{kind: Secret,\ndata: {config: opaque-multiline-flow-secret},\nstatus: failed}",
			want:  "{kind: Secret,\ndata: {config: [REDACTED]},\nstatus: failed}",
		},
		{
			name:  "Kubernetes Secret multiline flow comment",
			input: "{kind: Secret, # public comment\n data: {config: opaque-flow-comment-secret}}",
			want:  "{kind: Secret, # public comment\n data: {config: [REDACTED]}}",
		},
		{
			name: "Kubernetes Secret anchored kind",
			input: `kind: &secret-kind Secret
data:
  config: opaque-anchor-kind-secret`,
			want: `kind: &secret-kind Secret
data:
  config: [REDACTED]`,
		},
		{
			name: "Kubernetes Secret aliased kind",
			input: `secretKind: &secretKind Secret
kind: *secretKind
data:
  config: opaque-aliased-kind-secret
status: failed`,
			want: `secretKind: &secretKind Secret
kind: *secretKind
data:
  config: [REDACTED]
status: failed`,
		},
		{
			name: "Kubernetes Secret aliased data value",
			input: `kind: Secret
metadata:
  annotations:
    source: &secretValue opaque-anchored-data-secret
data:
  config: *secretValue
status: failed`,
			want: `kind: Secret
metadata:
  annotations:
    source: &secretValue [REDACTED]
data:
  config: [REDACTED]
status: failed`,
		},
		{
			name: "Kubernetes Secret aliased multiline data value",
			input: `kind: Secret
metadata:
  annotations:
    source: &secretValue "opaque-anchor-head
      opaque-anchor-tail-secret"
data:
  config: *secretValue
status: failed`,
			want: `kind: Secret
metadata:
  annotations:
    source: &secretValue [REDACTED]
data:
  config: [REDACTED]
status: failed`,
		},
		{
			name: "Kubernetes Secret aliased data mapping",
			input: `secretData: &secretData
  config: opaque-anchored-mapping-secret
kind: Secret
data: *secretData
status: failed`,
			want: `secretData: &secretData [REDACTED]
kind: Secret
data: [REDACTED]
status: failed`,
		},
		{
			name: "Kubernetes Secret flow data alias",
			input: `kind: Secret
metadata:
  annotations:
    source: &secretValue opaque-flow-alias-secret
data: {config: *secretValue}
status: failed`,
			want: `kind: Secret
metadata:
  annotations:
    source: &secretValue [REDACTED]
data: {config: [REDACTED]}
status: failed`,
		},
		{
			name: "Kubernetes Secret multiline quoted data scalar",
			input: `kind: Secret
data:
  config: "opaque-head
    opaque-tail-secret"
status: failed`,
			want: `kind: Secret
data:
  config: [REDACTED]
status: failed`,
		},
		{
			name: "Kubernetes Secret multiline single quoted data scalar",
			input: `kind: Secret
data:
  config: 'opaque-head
    opaque-tail-secret'
status: failed`,
			want: `kind: Secret
data:
  config: [REDACTED]
status: failed`,
		},
		{
			name: "Kubernetes Secret multiline plain data scalar",
			input: `kind: Secret
data:
  config: opaque-head
    opaque-tail-secret
status: failed`,
			want: `kind: Secret
data:
  config: [REDACTED]
status: failed`,
		},
		{
			name: "Kubernetes Secret tagged data container",
			input: `kind: Secret
data: !!map
  config: opaque-map-tag-secret`,
			want: `kind: Secret
data: !!map
  config: [REDACTED]`,
		},
		{
			name: "Kubernetes Secret explicit data child",
			input: `kind: Secret
data:
  ? config
  : opaque-explicit-data-secret
status: failed`,
			want: `kind: Secret
data:
  ? config
  : [REDACTED]
status: failed`,
		},
		{
			name: "Kubernetes Secret tagged explicit stringData child block scalar",
			input: `kind: Secret
stringData:
  ? !!str config
  : |
    opaque-explicit-head
    opaque-explicit-tail
status: failed`,
			want: `kind: Secret
stringData:
  ? !!str config
  : [REDACTED]
    [REDACTED]
    [REDACTED]
status: failed`,
		},
		{
			name: "Kubernetes Secret multiline data flow map",
			input: `apiVersion: v1
kind: Secret
data: {
  config: opaque-multiline-data-flow-secret
}
status: failed`,
			want: `apiVersion: v1
kind: Secret
data: {
  config: [REDACTED]
}
status: failed`,
		},
		{
			name: "Kubernetes Secret list data before kind",
			input: `items:
- data:
    config: opaque-list-order-secret
  kind: Secret
- kind: ConfigMap
  data:
    config: public-config`,
			want: `items:
- data:
    config: [REDACTED]
  kind: Secret
- kind: ConfigMap
  data:
    config: public-config`,
		},
		{
			name:  "Kubernetes Secret JSON data keys",
			input: `{"apiVersion":"v1","kind":"Secret","data":{"tls.key":"b3BhcXVlLXByaXZhdGUta2V5",".dockerconfigjson":"eyJhdXRocyI6eyJyZWdpc3RyeS5leGFtcGxlIjp7ImF1dGgiOiJvcGFxdWUifX19"},"status":"failed"}`,
			want:  `{"apiVersion":"v1","kind":"Secret","data":{"tls.key":"[REDACTED]",".dockerconfigjson":"[REDACTED]"},"status":"failed"}`,
		},
		{
			name:  "Kubernetes Secret JSON arbitrary data and stringData keys",
			input: `{"apiVersion":"v1","kind":"Secret","data":{"config":"opaque-json-config"},"stringData":{"settings":"opaque-json-string-data"},"status":"failed"}`,
			want:  `{"apiVersion":"v1","kind":"Secret","data":{"config":"[REDACTED]"},"stringData":{"settings":"[REDACTED]"},"status":"failed"}`,
		},
		{
			name:  "Kubernetes Secret escaped JSON arbitrary data key",
			input: `trace {\"kind\":\"Secret\",\"data\":{\"config\":\"opaque-escaped-secret\"}}`,
			want:  `trace {\"kind\":\"Secret\",\"data\":{\"config\":\"[REDACTED]\"}}`,
		},
		{
			name:  "Kubernetes Secret unicode escaped JSON kind key",
			input: `{"\u006b\u0069\u006e\u0064":"Secret","data":{"config":"opaque-unicode-kind-secret"}}`,
			want:  `{"\u006b\u0069\u006e\u0064":"Secret","data":{"config":"[REDACTED]"}}`,
		},
		{
			name:  "Kubernetes Secret unicode escaped JSON data key",
			input: `{"kind":"Secret","\u0064\u0061\u0074\u0061":{"config":"opaque-unicode-data-secret"}}`,
			want:  `{"kind":"Secret","\u0064\u0061\u0074\u0061":{"config":"[REDACTED]"}}`,
		},
		{
			name:  "Kubernetes Secret truncated JSON arbitrary data key",
			input: `{"kind":"Secret","data":{"config":"opaque-truncated-secret"`,
			want:  `{"kind":"Secret","data":{"config":"[REDACTED]"`,
		},
		{
			name:  "Kubernetes Secret truncated escaped JSON arbitrary data key",
			input: `{\"kind\":\"Secret\",\"data\":{\"config\":\"opaque-truncated-escaped-secret\"`,
			want:  `{\"kind\":\"Secret\",\"data\":{\"config\":\"[REDACTED]\"`,
		},
		{
			name:  "Kubernetes Secret unterminated JSON string",
			input: `{"kind":"Secret","data":{"config":"opaque-unterminated-secret`,
			want:  `{"kind":"Secret","data":{"config":"[REDACTED]"`,
		},
		{
			name:  "Kubernetes Secret unterminated escaped JSON string",
			input: `{\"kind\":\"Secret\",\"data\":{\"config\":\"opaque-unterminated-escaped-secret`,
			want:  `{\"kind\":\"Secret\",\"data\":{\"config\":\"[REDACTED]\"`,
		},
		{
			name:  "Kubernetes Secret truncated non-string JSON value",
			input: `{"kind":"Secret","data":{"config":opaque-truncated-nonstring-secret`,
			want:  `{"kind":"Secret","data":{"config":"[REDACTED]"`,
		},
		{
			name:  "TOML multiline basic string",
			input: "password = \"\"\"opaque-head\nopaque-tail\"\"\"\nstatus = \"failed\"",
			want:  "password = [REDACTED]\nstatus = \"failed\"",
		},
		{
			name:  "TOML multiline literal string",
			input: "password = '''opaque-head\nopaque-tail'''\nstatus = \"failed\"",
			want:  "password = [REDACTED]\nstatus = \"failed\"",
		},
		{
			name:  "TOML basic quoted credential key",
			input: `"asc_private_key_b64" = "opaque-private-key-material"`,
			want:  `"asc_private_key_b64" = [REDACTED]`,
		},
		{
			name:  "TOML literal quoted credential key with multiline value",
			input: "'password' = '''opaque-head\nopaque-tail'''\nstatus = \"failed\"",
			want:  "'password' = [REDACTED]\nstatus = \"failed\"",
		},
		{
			name: "TOML credential inline table",
			input: `password = { value = "opaque-inline-secret", nested = { label = "]" } }
status = "failed"`,
			want: `password = [REDACTED]
status = "failed"`,
		},
		{
			name: "TOML credential multiline array",
			input: `password = [
  "opaque-array-secret",
  { value = "opaque-nested-secret" },
]
status = "failed"`,
			want: `password = [REDACTED]
status = "failed"`,
		},
		{
			name:  "TOML escaped basic quoted credential key",
			input: `"pass\u0077ord" = "opaque-escaped-key-secret"`,
			want:  `"pass\u0077ord" = [REDACTED]`,
		},
		{
			name:  "TOML dotted escaped credential key",
			input: `credentials."pass\u0077ord" = "opaque-dotted-key-secret"`,
			want:  `credentials."pass\u0077ord" = [REDACTED]`,
		},
		{
			name:  "TOML sensitive parent dotted key",
			input: `password.value = "opaque-parent-secret"`,
			want:  `password.value = [REDACTED]`,
		},
		{
			name:  "TOML sensitive middle dotted key",
			input: `credentials.client_secret.value = "opaque-middle-secret"`,
			want:  `credentials.client_secret.value = [REDACTED]`,
		},
		{
			name:  "TOML quoted sensitive parent dotted key",
			input: `"pass\u0077ord".value = "opaque-quoted-parent-secret"`,
			want:  `"pass\u0077ord".value = [REDACTED]`,
		},
		{
			name:  "TOML sensitive table path",
			input: "[password]\nvalue = \"opaque-table-secret\"\n[metadata]\nvalue = \"public-context\"",
			want:  "[password]\nvalue = [REDACTED]\n[metadata]\nvalue = \"public-context\"",
		},
		{
			name:  "TOML sensitive array table path",
			input: "[[token]]\nvalue = \"opaque-array-table-secret\"",
			want:  "[[token]]\nvalue = [REDACTED]",
		},
		{
			name:  "netrc inline password",
			input: `machine api.example.test login alice password opaque-netrc-secret`,
			want:  `machine api.example.test login alice password [REDACTED]`,
		},
		{
			name:  "netrc multiline password",
			input: "machine api.example.test\n  login alice\n  password opaque-netrc-secret\nmachine public.example.test\n  login guest",
			want:  "machine api.example.test\n  login alice\n  password [REDACTED]\nmachine public.example.test\n  login guest",
		},
		{
			name:  "curl config user credential",
			input: "user = \"alice:opaque-config-secret\"\nurl = \"https://example.test\"",
			want:  "user = [REDACTED]\nurl = \"https://example.test\"",
		},
		{
			name:  "curl config user credential with colon separator",
			input: "user: \"alice:opaque-config-secret\"\nurl = \"https://example.test\"",
			want:  "user: [REDACTED]\nurl = \"https://example.test\"",
		},
		{
			name:  "curl config user credential with whitespace separator",
			input: "proxy-user \"alice:opaque-config-secret\"\nurl = \"https://example.test\"",
			want:  "proxy-user [REDACTED]\nurl = \"https://example.test\"",
		},
		{
			name:  "curl config certificate password",
			input: "cert = \"client.p12:opaque-config-secret\"\nurl = \"https://example.test\"",
			want:  "cert = \"client.p12:[REDACTED]\"\nurl = \"https://example.test\"",
		},
		{
			name:  "curl config proxy certificate password with colon separator",
			input: "proxy-cert: client.p12:opaque-config-secret\nurl = \"https://example.test\"",
			want:  "proxy-cert: client.p12:[REDACTED]\nurl = \"https://example.test\"",
		},
		{
			name:  "curl config Windows certificate password",
			input: `cert = "C:\client.p12:opaque-config-secret"`,
			want:  `cert = "C:\client.p12:[REDACTED]"`,
		},
		{
			name:  "curl config bearer credential",
			input: "oauth2-bearer = \"opaque-bearer-secret\"\nurl = \"https://example.test\"",
			want:  "oauth2-bearer = [REDACTED]\nurl = \"https://example.test\"",
		},
		{
			name:  "curl config bearer credential with whitespace separator",
			input: "oauth2-bearer \"opaque-bearer-secret\"\nurl = \"https://example.test\"",
			want:  "oauth2-bearer [REDACTED]\nurl = \"https://example.test\"",
		},
		{
			name:  "curl config explicit passphrase",
			input: "pass = \"opaque-curl-passphrase\"\nurl = \"sftp://example.test\"",
			want:  "pass = [REDACTED]\nurl = \"sftp://example.test\"",
		},
		{
			name:  "curl config cookie data credential",
			input: "cookie = \"myacinfo=opaque-cookie-secret\"\nurl = \"https://example.test\"",
			want:  "cookie = [REDACTED]\nurl = \"https://example.test\"",
		},
		{
			name:  "curl config cookie data credential with colon separator",
			input: "cookie: \"myacinfo=opaque-cookie-secret\"\nurl = \"https://example.test\"",
			want:  "cookie: [REDACTED]\nurl = \"https://example.test\"",
		},
		{
			name:  "curl config custom credential header",
			input: `header = "X-API-Key: opaque-curl-config-secret"`,
			want:  `header = "X-API-Key: [REDACTED]"`,
		},
		{
			name:  "curl config proxy credential header",
			input: `proxy-header: "X-Auth-Token: opaque-curl-proxy-secret"`,
			want:  `proxy-header: "X-Auth-Token: [REDACTED]"`,
		},
		{
			name:  "curl config unquoted authorization header",
			input: `header Authorization: Bearer opaque-curl-authorization-secret`,
			want:  `header Authorization: [REDACTED]`,
		},
		{
			name: "YAML escaped double quoted credential key block scalar",
			input: `"pass\u0077ord": |
  opaque-yaml-key-secret
status: failed`,
			want: `"pass\u0077ord": [REDACTED]
status: failed`,
		},
		{
			name:  "multiline plain yaml scalar preserves sibling",
			input: "response:\n  password: opaque-first\n    opaque-second\n  status: failed",
			want:  "response:\n  password: [REDACTED]\n  status: failed",
		},
		{
			name:  "YAML single quoted scalar with doubled quote",
			input: "password: 'super''sensitive'\nstatus: failed",
			want:  "password: [REDACTED]\nstatus: failed",
		},
		{
			name:  "YAML explicit credential key",
			input: "? password\n: opaque-explicit-secret\nstatus: failed",
			want:  "? password\n: [REDACTED]\nstatus: failed",
		},
		{
			name:  "YAML explicit credential key with intervening comments",
			input: "? password\n# context\n\n: opaque-explicit-secret\nstatus: failed",
			want:  "? password\n# context\n\n: [REDACTED]\nstatus: failed",
		},
		{
			name:  "tagged YAML explicit credential key",
			input: "? !!str password\n: opaque-tagged-explicit-secret\nstatus: failed",
			want:  "? !!str password\n: [REDACTED]\nstatus: failed",
		},
		{
			name:  "quoted YAML explicit credential block scalar",
			input: "response:\n  ? \"password\"\n  : |\n    opaque-explicit-secret\n  status: failed",
			want:  "response:\n  ? \"password\"\n  : [REDACTED]\n  status: failed",
		},
		{
			name:  "sequence YAML explicit credential flow value",
			input: "items:\n  - ? token\n    : [first-secret,\n      second-secret]\n    status: failed",
			want:  "items:\n  - ? token\n    : [REDACTED]\n    status: failed",
		},
		{
			name:  "YAML credential alias definition",
			input: "shared: &credential opaque-secret\npassword: *credential\nstatus: failed",
			want:  "shared: &credential [REDACTED]\npassword: [REDACTED]\nstatus: failed",
		},
		{
			name:  "YAML credential alias block definition",
			input: "shared: &credential\n  value: opaque-secret\n  type: bearer\npassword: *credential\nstatus: failed",
			want:  "shared: &credential [REDACTED]\npassword: [REDACTED]\nstatus: failed",
		},
		{
			name:  "YAML credential alias mapping key",
			input: "key: &s api_key\n*s: opaque-api-key\nstatus: failed",
			want:  "key: &s api_key\n*s: [REDACTED]\nstatus: failed",
		},
		{
			name:  "YAML credential alias flow mapping key",
			input: "key: &s client_secret\n*s: [first-secret,\n  second-secret]\nstatus: failed",
			want:  "key: &s client_secret\n*s: [REDACTED]\nstatus: failed",
		},
		{
			name:  "YAML explicit credential alias mapping key",
			input: "key: &s token\n? *s\n: opaque-token\nstatus: failed",
			want:  "key: &s token\n? *s\n: [REDACTED]\nstatus: failed",
		},
		{
			name:  "space-separated secret flag",
			input: `asc web sandbox create --email "user@example.test" --password "Passwordtest1" --territory "USA"`,
			want:  `asc web sandbox create --email "user@example.test" --password [REDACTED] --territory "USA"`,
		},
		{
			name:  "prefixed credential flags",
			input: `tool --database-password opaque-db-secret --github-token=opaque-token --password-file ./password.txt`,
			want:  `tool --database-password [REDACTED] --github-token=[REDACTED] --password-file ./password.txt`,
		},
		{
			name:  "fully quoted sensitive flag",
			input: `asc signing sync push "--password" opaque-secret`,
			want:  `asc signing sync push "--password" [REDACTED]`,
		},
		{
			name:  "quoted sensitive flag name fragment",
			input: `asc signing sync push --"password" opaque-secret`,
			want:  `asc signing sync push --"password" [REDACTED]`,
		},
		{
			name:  "single quoted sensitive flag equals form",
			input: `asc signing sync push '--password'=opaque-secret`,
			want:  `asc signing sync push '--password'=[REDACTED]`,
		},
		{
			name:  "adjacent quoted fragments in secret flag",
			input: `asc deploy --password 'super''secret' --verbose`,
			want:  `asc deploy --password [REDACTED] --verbose`,
		},
		{
			name:  "backtick command substitution in secret flag",
			input: "asc deploy --password `printf supersecret` --verbose",
			want:  `asc deploy --password [REDACTED] --verbose`,
		},
		{
			name:  "multiline backtick command substitution in secret flag",
			input: "asc signing sync push --password `printf '%s' 'opaque-head\nopaque-tail'` --verbose",
			want:  `asc signing sync push --password [REDACTED] --verbose`,
		},
		{
			name:  "dollar command substitution in secret flag",
			input: `asc deploy --password $(printf supersecret) --verbose`,
			want:  `asc deploy --password [REDACTED] --verbose`,
		},
		{
			name:  "nested dollar command substitution in secret flag",
			input: `asc deploy --password $(printf %s $(printf prefix) nested-super-secret) --verbose`,
			want:  `asc deploy --password [REDACTED] --verbose`,
		},
		{
			name:  "fish command substitution in secret flag",
			input: `asc signing sync --password (printf opaque-secret) --verbose`,
			want:  `asc signing sync --password [REDACTED] --verbose`,
		},
		{
			name:  "nested fish command substitution in secret flag",
			input: `asc signing sync --password (printf %s (printf prefix) nested-super-secret) --verbose`,
			want:  `asc signing sync --password [REDACTED] --verbose`,
		},
		{
			name:  "mixed adjacent fragments in secret assignment",
			input: `PASSWORD=pre'super'"secret"post asc builds list`,
			want:  `PASSWORD=[REDACTED] asc builds list`,
		},
		{
			name:  "notification webhook flag",
			input: `asc notify slack --webhook https://hooks.slack.com/services/T/B/super-secret --message ready`,
			want:  `asc notify slack --webhook [REDACTED] --message ready`,
		},
		{
			name:  "notification webhook environment assignment",
			input: `ASC_SLACK_WEBHOOK=https://hooks.slack.com/services/T/B/super-secret asc notify slack --message ready`,
			want:  `ASC_SLACK_WEBHOOK=[REDACTED] asc notify slack --message ready`,
		},
		{
			name:  "Fish exported credential assignment",
			input: `set -x ASC_SIGNING_SYNC_PASSWORD opaque-lowercase-secret; asc signing sync pull`,
			want:  `set -x ASC_SIGNING_SYNC_PASSWORD [REDACTED]; asc signing sync pull`,
		},
		{
			name:  "quoted custom secret header",
			input: `asc web xcode-cloud usage alert --webhook-header "X-API-Key: supersecret" --webhook https://example.test`,
			want:  `asc web xcode-cloud usage alert --webhook-header [REDACTED] --webhook [REDACTED]`,
		},
		{
			name:  "arbitrary custom webhook header",
			input: `asc web xcode-cloud usage alert --webhook-header "X-Service-Credential: opaque-lowercase-secret"`,
			want:  `asc web xcode-cloud usage alert --webhook-header [REDACTED]`,
		},
		{
			name:  "xcode cloud slack webhook flag",
			input: `asc web xcode-cloud usage alert --slack-webhook=https://hooks.slack.com/services/T/B/super-secret --threshold 90`,
			want:  `asc web xcode-cloud usage alert --slack-webhook=[REDACTED] --threshold 90`,
		},
		{
			name:  "direct two factor code flag",
			input: `asc web auth login --two-factor-code 123456 --apple-id user@example.test`,
			want:  `asc web auth login --two-factor-code [REDACTED] --apple-id user@example.test`,
		},
		{
			name:  "direct two factor code equals flag",
			input: `asc web auth login --two-factor-code=654321 --apple-id user@example.test`,
			want:  `asc web auth login --two-factor-code=[REDACTED] --apple-id user@example.test`,
		},
		{
			name:  "escaped quote in secret flag",
			input: `asc deploy --password "pa\"ssword" --verbose`,
			want:  `asc deploy --password [REDACTED] --verbose`,
		},
		{
			name:  "multiline double quoted secret flag",
			input: "asc deploy --password \"multiline-head\nmultiline tail secret\" --verbose",
			want:  "asc deploy --password [REDACTED] --verbose",
		},
		{
			name:  "multiline single quoted assignment",
			input: "PASSWORD='multiline-head\nmultiline-tail-secret' asc builds list",
			want:  "PASSWORD=[REDACTED] asc builds list",
		},
		{
			name:  "comma in unquoted secret flag",
			input: `asc web sandbox create --password Password1,remaining-secret --territory USA`,
			want:  `asc web sandbox create --password [REDACTED] --territory USA`,
		},
		{
			name:  "compound password flag",
			input: `asc review details-create --demo-account-password "app-specific-password" --notes ready`,
			want:  `asc review details-create --demo-account-password [REDACTED] --notes ready`,
		},
		{
			name:  "single dash password flag",
			input: `asc web sandbox create -password "Passwordtest1" -territory USA`,
			want:  `asc web sandbox create -password [REDACTED] -territory USA`,
		},
		{
			name:  "password value beginning with double dash",
			input: `asc web sandbox create --password --Passwordtest1 --territory USA`,
			want:  `asc web sandbox create --password [REDACTED] --territory USA`,
		},
		{
			name:  "boolean secret marker with sensitive named value",
			input: `asc web xcode-cloud env-vars set --name MY_SECRET --value s3cret --secret --apple-id 123456789`,
			want:  `asc web xcode-cloud env-vars set --name MY_SECRET --value [REDACTED] --secret --apple-id 123456789`,
		},
		{
			name:  "boolean secret marker before value",
			input: `asc web xcode-cloud env-vars set --name PRIVATE_CONFIG --secret --value s3cret --apple-id 123456789`,
			want:  `asc web xcode-cloud env-vars set --name PRIVATE_CONFIG --secret --value [REDACTED] --apple-id 123456789`,
		},
		{
			name:  "boolean secret marker after intervening flag",
			input: `asc web xcode-cloud env-vars set --value s3cret --name MY_SECRET --secret --apple-id 123456789`,
			want:  `asc web xcode-cloud env-vars set --value [REDACTED] --name MY_SECRET --secret --apple-id 123456789`,
		},
		{
			name:  "boolean secret marker before intervening flag",
			input: `asc web xcode-cloud env-vars set --secret --name MY_SECRET --value s3cret --apple-id 123456789`,
			want:  `asc web xcode-cloud env-vars set --secret --name MY_SECRET --value [REDACTED] --apple-id 123456789`,
		},
		{
			name:  "boolean secret marker redacts duplicate values",
			input: `asc web xcode-cloud env-vars set --value old-secret --value effective-secret --secret`,
			want:  `asc web xcode-cloud env-vars set --value [REDACTED] --value [REDACTED] --secret`,
		},
		{
			name: "boolean secret marker after continued value",
			input: `asc web xcode-cloud env-vars set --value continued-secret \
  --secret`,
			want: `asc web xcode-cloud env-vars set --value [REDACTED] \
  --secret`,
		},
		{
			name: "boolean secret marker before continued value",
			input: `asc web xcode-cloud env-vars set --secret \
  --value continued-secret`,
			want: `asc web xcode-cloud env-vars set --secret \
  --value [REDACTED]`,
		},
		{
			name: "boolean secret marker after continued command path",
			input: `asc web xcode-cloud env-vars \
  set --value continued-path-secret --secret`,
			want: `asc web xcode-cloud env-vars \
  set --value [REDACTED] --secret`,
		},
		{
			name:  "boolean secret marker after literal newline in quoted value",
			input: "asc web xcode-cloud env-vars set --value \"credential-head\ncredential-tail\" --secret",
			want:  "asc web xcode-cloud env-vars set --value [REDACTED] --secret",
		},
		{
			name:  "boolean secret marker with value equals form",
			input: `asc web xcode-cloud env-vars set --value=s3cret --secret`,
			want:  `asc web xcode-cloud env-vars set --value=[REDACTED] --secret`,
		},
		{
			name:  "boolean secret marker with explicit true value",
			input: `asc web xcode-cloud env-vars set --value s3cret --secret=true`,
			want:  `asc web xcode-cloud env-vars set --value [REDACTED] --secret=true`,
		},
		{
			name:  "boolean secret marker with quoted executable",
			input: `"asc" web xcode-cloud env-vars set --value s3cret --secret=true`,
			want:  `"asc" web xcode-cloud env-vars set --value [REDACTED] --secret=true`,
		},
		{
			name:  "boolean secret marker with explicit numeric true value",
			input: `asc web xcode-cloud env-vars set --secret=1 --value s3cret`,
			want:  `asc web xcode-cloud env-vars set --secret=1 --value [REDACTED]`,
		},
		{
			name:  "boolean secret marker before command separator",
			input: `asc web xcode-cloud env-vars set --value s3cret --secret; echo done`,
			want:  `asc web xcode-cloud env-vars set --value [REDACTED] --secret; echo done`,
		},
		{
			name:  "true secret marker before conditional operator",
			input: `asc web xcode-cloud env-vars set --value s3cret --secret=true&& echo done`,
			want:  `asc web xcode-cloud env-vars set --value [REDACTED] --secret=true&& echo done`,
		},
		{
			name:  "string valued webhook secret boolean literal",
			input: `asc webhooks create --url https://example.test/hook --secret=true`,
			want:  `asc webhooks create --url https://example.test/hook --secret=[REDACTED]`,
		},
		{
			name:  "boolean secret marker does not affect another line",
			input: "asc unrelated --value public\nasc webhooks create --secret webhook-secret",
			want:  "asc unrelated --value public\nasc webhooks create --secret [REDACTED]",
		},
		{
			name:  "unterminated quoted secret flag",
			input: "asc deploy --password \"super secret value",
			want:  "asc deploy --password [REDACTED]",
		},
		{
			name:  "unterminated multiline quoted assignment",
			input: "PASSWORD=\"opaque-head\nopaque-tail-secret",
			want:  "PASSWORD=[REDACTED]",
		},
		{
			name:  "ANSI-C quoted secret flag",
			input: `asc deploy --password $'super secret value' --verbose`,
			want:  `asc deploy --password [REDACTED] --verbose`,
		},
		{
			name:  "ANSI-C quoted assignment",
			input: `PASSWORD=$'super secret value' asc builds list`,
			want:  `PASSWORD=[REDACTED] asc builds list`,
		},
		{
			name:  "backslash escaped whitespace in secret flag",
			input: `asc deploy --password super\ secret --verbose`,
			want:  `asc deploy --password [REDACTED] --verbose`,
		},
		{
			name:  "backslash escaped whitespace in assignment",
			input: `PASSWORD=super\ secret asc builds list`,
			want:  `PASSWORD=[REDACTED] asc builds list`,
		},
		{
			name:  "backslash continued secret flag",
			input: "asc deploy --password super\\\nremainingcredential --verbose",
			want:  "asc deploy --password [REDACTED] --verbose",
		},
		{
			name:  "backslash continuation between secret flag and value",
			input: "asc deploy --password \\\n  opaque-secret --verbose",
			want:  "asc deploy --password \\\n  [REDACTED] --verbose",
		},
		{
			name:  "backslash continued assignment",
			input: "PASSWORD=super\\\nremainingcredential asc builds list",
			want:  "PASSWORD=[REDACTED] asc builds list",
		},
		{
			name:  "equals form secret flag",
			input: `asc deploy --demo-account-password=super-secret --verbose`,
			want:  `asc deploy --demo-account-password=[REDACTED] --verbose`,
		},
		{
			name:  "curl long user password flag",
			input: `curl --user alice:supersensitive https://example.test`,
			want:  `curl --user [REDACTED] https://example.test`,
		},
		{
			name:  "curl user password in separate quoted shell fragment",
			input: `curl --user alice:'supersensitive password' https://example.test`,
			want:  `curl --user [REDACTED] https://example.test`,
		},
		{
			name:  "curl user with separately quoted username",
			input: `curl --user 'alice':supersensitive https://example.test`,
			want:  `curl --user [REDACTED] https://example.test`,
		},
		{
			name:  "multiline quoted curl user password flag",
			input: "curl --user \"alice:first\nsecond-secret\" https://example.test",
			want:  "curl --user [REDACTED] https://example.test",
		},
		{
			name:  "continued curl user password flag",
			input: "curl --user alice:super\\\nremainingcredential https://example.test",
			want:  "curl --user [REDACTED] https://example.test",
		},
		{
			name:  "curl short user password flag",
			input: `curl -u 'alice:super sensitive' https://example.test`,
			want:  `curl -u [REDACTED] https://example.test`,
		},
		{
			name:  "curl user after similarly named wrapper option",
			input: `sudo -u build curl -u alice:supersensitive https://example.test`,
			want:  `sudo -u build curl -u [REDACTED] https://example.test`,
		},
		{
			name:  "curl attached short user password flag",
			input: `curl -ualice:supersensitive https://example.test`,
			want:  `curl -u[REDACTED] https://example.test`,
		},
		{
			name:  "curl equals user password flag",
			input: `curl --user=alice:supersensitive https://example.test`,
			want:  `curl --user=[REDACTED] https://example.test`,
		},
		{
			name:  "curl OAuth bearer flag",
			input: `curl --oauth2-bearer supersensitive https://example.test`,
			want:  `curl --oauth2-bearer [REDACTED] https://example.test`,
		},
		{
			name:  "curl private key passphrase flag",
			input: `curl --pass superprivatephrase --key client.pem https://example.test`,
			want:  `curl --pass [REDACTED] --key client.pem https://example.test`,
		},
		{
			name:  "curl TLS password flag",
			input: `curl --tlspassword supertlsphrase https://example.test`,
			want:  `curl --tlspassword [REDACTED] https://example.test`,
		},
		{
			name:  "multiline quoted curl certificate password",
			input: "curl --cert \"client.p12:first\nsecond-secret\" https://example.test",
			want:  "curl --cert \"client.p12:[REDACTED]\" https://example.test",
		},
		{
			name:  "curl proxy TLS password flag",
			input: `curl --proxy-tlspassword superproxyphrase https://example.test`,
			want:  `curl --proxy-tlspassword [REDACTED] https://example.test`,
		},
		{
			name:  "curl long proxy user password flag",
			input: `curl --proxy-user alice:supersensitive https://example.test`,
			want:  `curl --proxy-user [REDACTED] https://example.test`,
		},
		{
			name:  "curl short proxy user password flag",
			input: `curl -U 'alice:super sensitive' https://example.test`,
			want:  `curl -U [REDACTED] https://example.test`,
		},
		{
			name:  "curl attached short proxy user password flag",
			input: `curl -Ualice:supersensitive https://example.test`,
			want:  `curl -U[REDACTED] https://example.test`,
		},
		{
			name:  "comma in unquoted assignment",
			input: `PASSWORD=Password1,remaining-secret asc builds list`,
			want:  `PASSWORD=[REDACTED] asc builds list`,
		},
		{
			name:  "prefixed environment assignment",
			input: `AWS_SECRET_ACCESS_KEY="cloud-secret" MY_CLIENT_SECRET='client-secret'`,
			want:  `AWS_SECRET_ACCESS_KEY=[REDACTED] MY_CLIENT_SECRET=[REDACTED]`,
		},
		{
			name:  "security key PIN environment assignment",
			input: `ASC_WEB_SECURITY_KEY_PIN=12345678 asc web auth login --two-factor-method security-key`,
			want:  `ASC_WEB_SECURITY_KEY_PIN=[REDACTED] asc web auth login --two-factor-method security-key`,
		},
		{
			name:  "secret key environment assignments",
			input: `SECRET_KEY=framework-secret STRIPE_SECRET_KEY=payment-secret SECRET_KEY_BASE=rails-secret`,
			want:  `SECRET_KEY=[REDACTED] STRIPE_SECRET_KEY=[REDACTED] SECRET_KEY_BASE=[REDACTED]`,
		},
		{
			name:  "prefixed pass assignments and flags",
			input: "DB_PASS=database-credential\ntool --db-pass opaque-cli-credential",
			want:  "DB_PASS=[REDACTED]\ntool --db-pass [REDACTED]",
		},
		{
			name:  "secret key flag",
			input: `tool --secret-key opaque-cli-secret --status failed`,
			want:  `tool --secret-key [REDACTED] --status failed`,
		},
		{
			name:  "kubectl secret literal",
			input: `kubectl create secret generic foo --from-literal=custom=opaque-secret --namespace demo`,
			want:  `kubectl create secret generic foo --from-literal=[REDACTED] --namespace demo`,
		},
		{
			name:  "kubectl quoted secret literal",
			input: `kubectl create secret generic foo --from-literal "custom=opaque secret" --namespace demo`,
			want:  `kubectl create secret generic foo --from-literal [REDACTED] --namespace demo`,
		},
		{
			name:  "kubectl compound quoted secret literal",
			input: `kubectl create secret generic foo --from-literal=custom="opaque secret" --from-literal=second='opaque two'`,
			want:  `kubectl create secret generic foo --from-literal=[REDACTED] --from-literal=[REDACTED]`,
		},
		{
			name:  "kubectl backslash continued secret literal",
			input: "kubectl create secret generic foo --from-literal=custom=opaque\\\n  secret --namespace demo",
			want:  "kubectl create secret generic foo --from-literal=[REDACTED] --namespace demo",
		},
		{
			name:  "kubectl continued create secret command",
			input: "kubectl create \\\n  secret generic foo --from-literal=custom=opaque-secret --namespace demo",
			want:  "kubectl create \\\n  secret generic foo --from-literal=[REDACTED] --namespace demo",
		},
		{
			name:  "kubectl global option before create secret command",
			input: `kubectl --context demo create secret generic foo --from-literal=custom=opaque-secret --namespace demo`,
			want:  `kubectl --context demo create secret generic foo --from-literal=[REDACTED] --namespace demo`,
		},
		{
			name:  "sudo kubectl secret literal",
			input: `sudo kubectl create secret generic foo --from-literal=custom=opaque-secret --namespace demo`,
			want:  `sudo kubectl create secret generic foo --from-literal=[REDACTED] --namespace demo`,
		},
		{
			name:  "kubectl secret literal through option-bearing wrappers",
			input: `sudo -u build env -v PROFILE=release kubectl create secret generic foo --from-literal=custom=opaque-secret --namespace demo`,
			want:  `sudo -u build env -v PROFILE=release kubectl create secret generic foo --from-literal=[REDACTED] --namespace demo`,
		},
		{
			name:  "security unlock keychain password",
			input: `security unlock-keychain -p "opaque credential" build.keychain`,
			want:  `security unlock-keychain -p [REDACTED] build.keychain`,
		},
		{
			name:  "security password after unindented continuation",
			input: "security unlock-keychain \\\n-popaque-credential build.keychain",
			want:  "security unlock-keychain \\\n-p[REDACTED] build.keychain",
		},
		{
			name:  "security password after similarly named wrapper option",
			input: `sudo -p public-prompt security unlock-keychain -p opaque-credential build.keychain`,
			want:  `sudo -p public-prompt security unlock-keychain -p [REDACTED] build.keychain`,
		},
		{
			name:  "security password through continued wrapper option",
			input: "sudo -p \\\n public-prompt security unlock-keychain -p opaque-credential build.keychain",
			want:  "sudo -p \\\n public-prompt security unlock-keychain -p [REDACTED] build.keychain",
		},
		{
			name:  "sudo security partition list keychain password",
			input: `sudo /usr/bin/security set-key-partition-list -S apple-tool:,apple: -s -k 'opaque credential' build.keychain`,
			want:  `sudo /usr/bin/security set-key-partition-list -S apple-tool:,apple: -s -k [REDACTED] build.keychain`,
		},
		{
			name:  "security old and new keychain passwords",
			input: `security set-keychain-password -o old-credential -p new-credential build.keychain`,
			want:  `security set-keychain-password -o [REDACTED] -p [REDACTED] build.keychain`,
		},
		{
			name:  "security generic password value",
			input: `security add-generic-password -a build -s signing -w opaque-credential build.keychain`,
			want:  `security add-generic-password -a build -s signing -w [REDACTED] build.keychain`,
		},
		{
			name:  "security import passphrase",
			input: `security import signing.p12 -k build.keychain -P opaque-passphrase`,
			want:  `security import signing.p12 -k build.keychain -P [REDACTED]`,
		},
		{
			name:  "continued security password argument",
			input: "security unlock-keychain -p \\\n  opaque-credential build.keychain",
			want:  "security unlock-keychain -p \\\n  [REDACTED] build.keychain",
		},
		{
			name:  "OpenSSL passphrase source flags",
			input: `openssl pkcs12 -export -passout pass:opaque-output -passin pass:opaque-input -passcerts pass:opaque-certs -in signing.pem`,
			want:  `openssl pkcs12 -export -passout [REDACTED] -passin [REDACTED] -passcerts [REDACTED] -in signing.pem`,
		},
		{
			name:  "OpenSSL two-dash passphrase source flags",
			input: `openssl pkeyutl --passin pass:opaque-input --passout=pass:opaque-output --passcerts pass:opaque-certs --pass pass:opaque-pass --k opaque-key --K=opaque-raw-key`,
			want:  `openssl pkeyutl --passin [REDACTED] --passout=[REDACTED] --passcerts [REDACTED] --pass [REDACTED] --k [REDACTED] --K=[REDACTED]`,
		},
		{
			name:  "OpenSSL passphrase flag after unindented continuation",
			input: "openssl pkcs12 \\\n-passout pass:opaque-output -in signing.pem",
			want:  "openssl pkcs12 \\\n-passout [REDACTED] -in signing.pem",
		},
		{
			name:  "OpenSSL enc passphrase and raw key options",
			input: `openssl enc -aes-256-cbc -k opaque-passphrase -K=0123456789abcdef -pass env:ENC_PASSWORD -in plaintext`,
			want:  `openssl enc -aes-256-cbc -k [REDACTED] -K=[REDACTED] -pass [REDACTED] -in plaintext`,
		},
		{
			name:  "OpenSSL digest HMAC key",
			input: `sudo -P openssl dgst -sha256 -hmac "opaque HMAC key" artifact`,
			want:  `sudo -P openssl dgst -sha256 -hmac [REDACTED] artifact`,
		},
		{
			name:  "OpenSSL digest HMAC key after unindented continuation",
			input: "openssl dgst \\\n-hmac=opaque-hmac-secret artifact",
			want:  "openssl dgst \\\n-hmac=[REDACTED] artifact",
		},
		{
			name:  "OpenSSL digest HMAC key from command substitution",
			input: `openssl -provider default dgst -sha256 -hmac $(printf opaque-hmac-secret) artifact`,
			want:  `openssl -provider default dgst -sha256 -hmac [REDACTED] artifact`,
		},
		{
			name:  "OpenSSL digest HMAC key with embedded substitution",
			input: `openssl dgst -hmac prefix$(printf opaque-hmac-secret)suffix artifact`,
			want:  `openssl dgst -hmac [REDACTED] artifact`,
		},
		{
			name:  "OpenSSL digest MAC option key",
			input: `openssl dgst -sha256 -mac HMAC -macopt key:opaque-mac-secret artifact`,
			want:  `openssl dgst -sha256 -mac HMAC -macopt [REDACTED] artifact`,
		},
		{
			name:  "OpenSSL MAC hex key option",
			input: `openssl -provider default mac -digest SHA256 -macopt=hexkey:0123456789abcdef HMAC`,
			want:  `openssl -provider default mac -digest SHA256 -macopt=[REDACTED] HMAC`,
		},
		{
			name:  "OpenSSL MAC option key from embedded substitution",
			input: `openssl mac -macopt "key:prefix$(printf opaque-mac-secret)suffix" HMAC`,
			want:  `openssl mac -macopt [REDACTED] HMAC`,
		},
		{
			name:  "OpenSSL KDF key options",
			input: `sudo -P openssl -provider default kdf -keylen 32 -kdfopt digest:SHA256 -kdfopt key:opaque-kdf-key -kdfopt=hexkey:0123456789abcdef HKDF`,
			want:  `sudo -P openssl -provider default kdf -keylen 32 -kdfopt digest:SHA256 -kdfopt [REDACTED] -kdfopt=[REDACTED] HKDF`,
		},
		{
			name:  "OpenSSL KDF secret options",
			input: `openssl kdf -keylen 32 -kdfopt digest:SHA256 -kdfopt secret:opaque-kdf-secret -kdfopt "hexsecret:prefix$(printf 0123456789abcdef)suffix" TLS1-PRF`,
			want:  `openssl kdf -keylen 32 -kdfopt digest:SHA256 -kdfopt [REDACTED] -kdfopt [REDACTED] TLS1-PRF`,
		},
		{
			name:  "OpenSSL KDF password options",
			input: "openssl kdf \\\n-kdfopt pass:opaque-kdf-password -kdfopt hexpass:0123456789abcdef PBKDF2",
			want:  "openssl kdf \\\n-kdfopt [REDACTED] -kdfopt [REDACTED] PBKDF2",
		},
		{
			name:  "OpenSSL CA private key passphrase",
			input: `openssl ca -config ca.cnf -key=opaque-ca-passphrase -in request.pem`,
			want:  `openssl ca -config ca.cnf -key=[REDACTED] -in request.pem`,
		},
		{
			name:  "OpenSSL CA private key passphrase after unindented continuation",
			input: "openssl ca \\\n-key opaque-ca-passphrase -in request.pem",
			want:  "openssl ca \\\n-key [REDACTED] -in request.pem",
		},
		{
			name:  "OpenSSL CA private key passphrase from arithmetic expansion",
			input: `openssl --provider default ca -key $((1000 + 95)) -in request.pem`,
			want:  `openssl --provider default ca -key [REDACTED] -in request.pem`,
		},
		{
			name:  "OpenSSL CA private key passphrase from backtick substitution",
			input: "openssl ca -key `printf opaque-ca-secret` -in request.pem",
			want:  `openssl ca -key [REDACTED] -in request.pem`,
		},
		{
			name:  "OpenSSL passwd positional password",
			input: `sudo -u build openssl passwd -6 -salt public-salt "opaque password"`,
			want:  `sudo -u build openssl passwd -6 -salt public-salt [REDACTED]`,
		},
		{
			name:  "OpenSSL passwd positional password after option terminator",
			input: `openssl passwd -- opaque-password`,
			want:  `openssl passwd -- [REDACTED]`,
		},
		{
			name:  "OpenSSL passwd positional password from command substitution",
			input: `openssl passwd -6 $(printf opaque-command-substitution)`,
			want:  `openssl passwd -6 [REDACTED]`,
		},
		{
			name:  "OpenSSL passwd positional password with embedded command substitution",
			input: `openssl passwd -6 foo$(printf opaque-command-substitution)bar`,
			want:  `openssl passwd -6 [REDACTED]`,
		},
		{
			name:  "OpenSSL passwd positional password from arithmetic expansion",
			input: `openssl passwd -6 $((1 + 2))`,
			want:  `openssl passwd -6 [REDACTED]`,
		},
		{
			name:  "OpenSSL passwd positional password from backtick substitution",
			input: "openssl passwd -6 `printf opaque-backtick-substitution`",
			want:  `openssl passwd -6 [REDACTED]`,
		},
		{
			name:  "Windows OpenSSL passwd positional password",
			input: `C:\OpenSSL\bin\openssl.exe passwd -6 opaque-windows-password`,
			want:  `C:\OpenSSL\bin\openssl.exe passwd -6 [REDACTED]`,
		},
		{
			name:  "relative Windows OpenSSL passwd positional password",
			input: `.\bin\openssl.exe passwd -6 opaque-windows-password`,
			want:  `.\bin\openssl.exe passwd -6 [REDACTED]`,
		},
		{
			name:  "OpenSSL passwd positional password after command in subshell",
			input: `(true; openssl passwd -6 opaque-subshell-password)`,
			want:  `(true; openssl passwd -6 [REDACTED])`,
		},
		{
			name:  "OpenSSL passwd positional password after conditional in subshell",
			input: `(true && openssl passwd -6 opaque-subshell-password)`,
			want:  `(true && openssl passwd -6 [REDACTED])`,
		},
		{
			name:  "OpenSSL passwd positional password after pipeline in subshell",
			input: `(true | openssl passwd -6 opaque-subshell-password)`,
			want:  `(true | openssl passwd -6 [REDACTED])`,
		},
		{
			name:  "OpenSSL passwd positional password in nested subshell",
			input: `(true; (openssl passwd -6 opaque-nested-password))`,
			want:  `(true; (openssl passwd -6 [REDACTED]))`,
		},
		{
			name:  "OpenSSL passwd positional password in subshell after quoted command",
			input: `echo "safe"; (true; openssl passwd -6 opaque-after-quoted-command)`,
			want:  `echo "safe"; (true; openssl passwd -6 [REDACTED])`,
		},
		{
			name:  "OpenSSL passwd positional password in subshell condition",
			input: `if (true; openssl passwd -6 opaque-condition-password); then :; fi`,
			want:  `if (true; openssl passwd -6 [REDACTED]); then :; fi`,
		},
		{
			name:  "OpenSSL passwd positional password in shell function",
			input: `sign_password() { openssl passwd -6 opaque-function-password; }`,
			want:  `sign_password() { openssl passwd -6 [REDACTED]; }`,
		},
		{
			name:  "OpenSSL passwd positional password in compact shell function",
			input: `sign_password(){ openssl passwd -6 opaque-function-password; }`,
			want:  `sign_password(){ openssl passwd -6 [REDACTED]; }`,
		},
		{
			name:  "OpenSSL passwd positional password in spaced shell function",
			input: `sign_password () { openssl passwd -6 opaque-function-password; }`,
			want:  `sign_password () { openssl passwd -6 [REDACTED]; }`,
		},
		{
			name:  "OpenSSL passwd positional password in function keyword body",
			input: `function sign_password { openssl passwd -6 opaque-function-password; }`,
			want:  `function sign_password { openssl passwd -6 [REDACTED]; }`,
		},
		{
			name:  "OpenSSL passwd positional password in function keyword parenthesized body",
			input: `function sign_password() { openssl passwd -6 opaque-function-password; }`,
			want:  `function sign_password() { openssl passwd -6 [REDACTED]; }`,
		},
		{
			name:  "OpenSSL passwd positional password in compact function keyword body",
			input: `function sign_password(){ openssl passwd -6 opaque-function-password; }`,
			want:  `function sign_password(){ openssl passwd -6 [REDACTED]; }`,
		},
		{
			name:  "OpenSSL passwd positional password in spaced subshell function",
			input: `sign_password () (true; openssl passwd -6 opaque-function-password)`,
			want:  `sign_password () (true; openssl passwd -6 [REDACTED])`,
		},
		{
			name:  "OpenSSL credential in if condition",
			input: `if openssl pkcs12 -export -passout pass:opaque-if-secret; then :; fi`,
			want:  `if openssl pkcs12 -export -passout [REDACTED]; then :; fi`,
		},
		{
			name:  "OpenSSL credential in brace group",
			input: `{ openssl pkcs12 -export -passout pass:opaque-group-secret; }`,
			want:  `{ openssl pkcs12 -export -passout [REDACTED]; }`,
		},
		{
			name:  "OpenSSL credential in subshell",
			input: `(openssl pkcs12 -export -passout pass:opaque-subshell-secret)`,
			want:  `(openssl pkcs12 -export -passout [REDACTED])`,
		},
		{
			name:  "OpenSSL credential in case branch",
			input: `case release in debug) : ;; *) openssl pkcs12 -export -passout pass:opaque-case-secret;; esac`,
			want:  `case release in debug) : ;; *) openssl pkcs12 -export -passout [REDACTED];; esac`,
		},
		{
			name:  "OpenSSL credential after grouped sudo options",
			input: `sudo -iu build openssl pkcs12 -export -passout pass:opaque-sudo-secret`,
			want:  `sudo -iu build openssl pkcs12 -export -passout [REDACTED]`,
		},
		{
			name:  "OpenSSL credential with escaped command name",
			input: `o\penssl pkcs12 -export -passout pass:opaque-escaped-secret`,
			want:  `o\penssl pkcs12 -export -passout [REDACTED]`,
		},
		{
			name:  "OpenSSL credential through xargs shell command",
			input: `xargs -0 sh -c 'openssl pkcs12 -export -passout pass:opaque-xargs-secret'`,
			want:  `xargs -0 sh -c 'openssl pkcs12 -export -passout [REDACTED]'`,
		},
		{
			name:  "OpenSSL credential through xargs optional EOF flag",
			input: `xargs -e openssl pkcs12 -export -passout pass:opaque-xargs-secret`,
			want:  `xargs -e openssl pkcs12 -export -passout [REDACTED]`,
		},
		{
			name:  "OpenSSL credential through xargs optional long replace flag",
			input: `xargs --replace openssl pkcs12 -export -passout pass:opaque-xargs-secret`,
			want:  `xargs --replace openssl pkcs12 -export -passout [REDACTED]`,
		},
		{
			name:  "OpenSSL credential through xargs required BSD replacement flag",
			input: `xargs -J replacement openssl pkcs12 -export -passout pass:opaque-xargs-secret`,
			want:  `xargs -J replacement openssl pkcs12 -export -passout [REDACTED]`,
		},
		{
			name:  "OpenSSL credential through find exec shell command",
			input: `find . -name '*.p12' -exec sh -c 'openssl pkcs12 -export -passout pass:opaque-find-secret' \;`,
			want:  `find . -name '*.p12' -exec sh -c 'openssl pkcs12 -export -passout [REDACTED]' \;`,
		},
		{
			name:  "OpenSSL credential through find confirmation command",
			input: `find . -name '*.p12' -ok openssl passwd -6 opaque-find-secret \;`,
			want:  `find . -name '*.p12' -ok openssl passwd -6 [REDACTED] \;`,
		},
		{
			name:  "OpenSSL credential through watch exec wrapper",
			input: `watch -n 1 -x openssl pkcs12 -export -passout pass:opaque-watch-secret`,
			want:  `watch -n 1 -x openssl pkcs12 -export -passout [REDACTED]`,
		},
		{
			name:  "OpenSSL passphrase through env wrapper",
			input: `env PROFILE=release openssl pkey -passin env:SIGNING_PASSWORD -in signing.pem`,
			want:  `env PROFILE=release openssl pkey -passin [REDACTED] -in signing.pem`,
		},
		{
			name:  "OpenSSL passphrase through env debug wrapper",
			input: `env -v openssl pkcs12 -export -passout pass:opaque-output`,
			want:  `env -v openssl pkcs12 -export -passout [REDACTED]`,
		},
		{
			name:  "OpenSSL passphrase through env long debug wrapper",
			input: `env --debug openssl pkcs12 -export -passout pass:opaque-output`,
			want:  `env --debug openssl pkcs12 -export -passout [REDACTED]`,
		},
		{
			name:  "OpenSSL passphrase through env null wrapper",
			input: `env -iv0 openssl pkcs12 -export -passout pass:opaque-output`,
			want:  `env -iv0 openssl pkcs12 -export -passout [REDACTED]`,
		},
		{
			name:  "OpenSSL passphrase through env long null wrapper",
			input: `env --null openssl pkcs12 -export -passout pass:opaque-output`,
			want:  `env --null openssl pkcs12 -export -passout [REDACTED]`,
		},
		{
			name:  "OpenSSL passphrase through env utility path wrapper",
			input: `env -P /usr/bin openssl pkcs12 -export -passout pass:opaque-output`,
			want:  `env -P /usr/bin openssl pkcs12 -export -passout [REDACTED]`,
		},
		{
			name:  "OpenSSL passphrase through env split string wrapper",
			input: `env -S "openssl pkcs12 -export -passout pass:opaque-output"`,
			want:  `env -S "openssl pkcs12 -export -passout [REDACTED]"`,
		},
		{
			name:  "OpenSSL passphrase through env long split string wrapper",
			input: `env --split-string="openssl pkcs12 -export -passout pass:opaque-output"`,
			want:  `env --split-string="openssl pkcs12 -export -passout [REDACTED]"`,
		},
		{
			name:  "OpenSSL passphrase through separate env long split string wrapper",
			input: `env --split-string "PROFILE=release openssl pkcs12 -export -passout pass:opaque-output"`,
			want:  `env --split-string "PROFILE=release openssl pkcs12 -export -passout [REDACTED]"`,
		},
		{
			name:  "OpenSSL passphrase through attached env split string wrapper",
			input: `env -iS"PROFILE=release openssl pkcs12 -export -passout pass:opaque-output"`,
			want:  `env -iS"PROFILE=release openssl pkcs12 -export -passout [REDACTED]"`,
		},
		{
			name:  "OpenSSL passphrase through timeout wrapper",
			input: `timeout 30 openssl pkcs12 -export -passout pass:opaque-timeout-secret`,
			want:  `timeout 30 openssl pkcs12 -export -passout [REDACTED]`,
		},
		{
			name:  "OpenSSL passphrase through option-bearing gtimeout wrapper",
			input: `gtimeout --preserve-status --kill-after=5s 30s sudo -u build openssl pkcs12 -export -passout pass:opaque-timeout-secret`,
			want:  `gtimeout --preserve-status --kill-after=5s 30s sudo -u build openssl pkcs12 -export -passout [REDACTED]`,
		},
		{
			name:  "OpenSSL passphrase through grouped timeout options and hex duration",
			input: `timeout -fk5s 0x1p0d openssl pkcs12 -export -passout pass:opaque-timeout-secret`,
			want:  `timeout -fk5s 0x1p0d openssl pkcs12 -export -passout [REDACTED]`,
		},
		{
			name:  "OpenSSL passphrase through fractional timeout duration",
			input: `timeout .5s openssl pkcs12 -export -passout pass:opaque-timeout-secret`,
			want:  `timeout .5s openssl pkcs12 -export -passout [REDACTED]`,
		},
		{
			name:  "OpenSSL passphrase through nohup wrapper",
			input: `nohup openssl pkcs12 -export -passout pass:opaque-nohup-secret`,
			want:  `nohup openssl pkcs12 -export -passout [REDACTED]`,
		},
		{
			name:  "OpenSSL passphrase through nohup option terminator",
			input: `/usr/bin/nohup -- env PROFILE=release openssl pkcs12 -export -passout pass:opaque-nohup-secret`,
			want:  `/usr/bin/nohup -- env PROFILE=release openssl pkcs12 -export -passout [REDACTED]`,
		},
		{
			name:  "OpenSSL passphrase through nested timeout and nohup wrappers",
			input: `timeout 30 nohup sudo -u build openssl pkcs12 -export -passout pass:opaque-nohup-secret`,
			want:  `timeout 30 nohup sudo -u build openssl pkcs12 -export -passout [REDACTED]`,
		},
		{
			name:  "OpenSSL passphrase through bash command string",
			input: `bash -c 'openssl pkcs12 -export -passout pass:opaque-shell-secret'`,
			want:  `bash -c 'openssl pkcs12 -export -passout [REDACTED]'`,
		},
		{
			name:  "keytool password through path-qualified shell command string",
			input: `/bin/sh -c "keytool -list -storepass opaque-shell-secret"`,
			want:  `/bin/sh -c "keytool -list -storepass [REDACTED]"`,
		},
		{
			name:  "OpenSSL passphrase in later shell command",
			input: `sudo -u build env PROFILE=release bash -lc 'echo preparing; timeout 30 openssl pkcs12 -export -passout pass:opaque-shell-secret'`,
			want:  `sudo -u build env PROFILE=release bash -lc 'echo preparing; timeout 30 openssl pkcs12 -export -passout [REDACTED]'`,
		},
		{
			name:  "OpenSSL passphrase through nested shell command strings",
			input: `bash -c 'sh -c "openssl pkcs12 -export -passout pass:opaque-shell-secret"'`,
			want:  `bash -c 'sh -c "openssl pkcs12 -export -passout [REDACTED]"'`,
		},
		{
			name:  "OpenSSL passphrase through shell command delimiter",
			input: `bash --noprofile -c -- 'exec openssl pkcs12 -export -passout pass:opaque-shell-secret'`,
			want:  `bash --noprofile -c -- 'exec openssl pkcs12 -export -passout [REDACTED]'`,
		},
		{
			name:  "OpenSSL passphrase through shell options and exec wrapper",
			input: `bash -O extglob -o posix -ic 'exec -a signer openssl pkcs12 -export -passout pass:opaque-shell-secret'`,
			want:  `bash -O extglob -o posix -ic 'exec -a signer openssl pkcs12 -export -passout [REDACTED]'`,
		},
		{
			name:  "OpenSSL passphrase through ANSI C quoted shell command",
			input: `bash -c $'openssl pkcs12 -export -passout pass:opaque-shell-secret'`,
			want:  `bash -c $'openssl pkcs12 -export -passout [REDACTED]'`,
		},
		{
			name:  "OpenSSL passphrase through nice wrapper",
			input: `nice -n 5 openssl pkcs12 -export -passout pass:opaque-nice-secret`,
			want:  `nice -n 5 openssl pkcs12 -export -passout [REDACTED]`,
		},
		{
			name:  "OpenSSL passphrase through bare nice wrapper",
			input: `nice openssl pkcs12 -export -passout pass:opaque-nice-secret`,
			want:  `nice openssl pkcs12 -export -passout [REDACTED]`,
		},
		{
			name:  "OpenSSL passphrase through GNU nice wrapper",
			input: `gnice --adjustment=-5 timeout 30 openssl pkcs12 -export -passout pass:opaque-nice-secret`,
			want:  `gnice --adjustment=-5 timeout 30 openssl pkcs12 -export -passout [REDACTED]`,
		},
		{
			name:  "OpenSSL passphrase through traditional nice adjustment",
			input: `/usr/bin/nice --5 sudo -u build openssl pkcs12 -export -passout pass:opaque-nice-secret`,
			want:  `/usr/bin/nice --5 sudo -u build openssl pkcs12 -export -passout [REDACTED]`,
		},
		{
			name:  "OpenSSL passphrase through time wrapper",
			input: `/usr/bin/time -al -o timing.txt openssl pkcs12 -export -passout pass:opaque-time-secret`,
			want:  `/usr/bin/time -al -o timing.txt openssl pkcs12 -export -passout [REDACTED]`,
		},
		{
			name:  "OpenSSL passphrase through GNU time wrapper",
			input: `gtime --verbose --format='%E' timeout 30 openssl pkcs12 -export -passout pass:opaque-time-secret`,
			want:  `gtime --verbose --format='%E' timeout 30 openssl pkcs12 -export -passout [REDACTED]`,
		},
		{
			name:  "OpenSSL passphrase through stdbuf wrapper",
			input: `stdbuf -oL -e 0 openssl pkcs12 -export -passout pass:opaque-stdbuf-secret`,
			want:  `stdbuf -oL -e 0 openssl pkcs12 -export -passout [REDACTED]`,
		},
		{
			name:  "OpenSSL passphrase through GNU stdbuf wrapper",
			input: `gstdbuf --output=L --error 0 nice -n5 openssl pkcs12 -export -passout pass:opaque-stdbuf-secret`,
			want:  `gstdbuf --output=L --error 0 nice -n5 openssl pkcs12 -export -passout [REDACTED]`,
		},
		{
			name:  "OpenSSL passphrase through setsid wrapper",
			input: `setsid -fw env PROFILE=release openssl pkcs12 -export -passout pass:opaque-setsid-secret`,
			want:  `setsid -fw env PROFILE=release openssl pkcs12 -export -passout [REDACTED]`,
		},
		{
			name:  "OpenSSL passphrase through ionice wrapper",
			input: `ionice -c 3 openssl pkcs12 -export -passout pass:opaque-ionice-secret`,
			want:  `ionice -c 3 openssl pkcs12 -export -passout [REDACTED]`,
		},
		{
			name:  "OpenSSL passphrase through long ionice options",
			input: `ionice --class=best-effort --classdata 4 --ignore sudo -K openssl pkcs12 -export -passout pass:opaque-ionice-secret`,
			want:  `ionice --class=best-effort --classdata 4 --ignore sudo -K openssl pkcs12 -export -passout [REDACTED]`,
		},
		{
			name:  "OpenSSL passphrase through caffeinate wrapper",
			input: `caffeinate -di -t 60 -- openssl pkcs12 -export -passout pass:opaque-caffeinate-secret`,
			want:  `caffeinate -di -t 60 -- openssl pkcs12 -export -passout [REDACTED]`,
		},
		{
			name:  "OpenSSL passphrase through caffeinate wait wrapper",
			input: `caffeinate -w123 openssl pkcs12 -export -passout pass:opaque-caffeinate-secret`,
			want:  `caffeinate -w123 openssl pkcs12 -export -passout [REDACTED]`,
		},
		{
			name:  "OpenSSL passphrase through dynamic caffeinate arguments",
			input: `caffeinate -t "$CAFFEINATE_TIMEOUT" -w "$(pgrep -n Finder)" openssl pkcs12 -export -passout pass:opaque-caffeinate-secret`,
			want:  `caffeinate -t "$CAFFEINATE_TIMEOUT" -w "$(pgrep -n Finder)" openssl pkcs12 -export -passout [REDACTED]`,
		},
		{
			name:  "OpenSSL passphrase through prefix parsed caffeinate arguments",
			input: `caffeinate -t 1foo -w123abc -t0x10 -w-1 openssl pkcs12 -export -passout pass:opaque-caffeinate-secret`,
			want:  `caffeinate -t 1foo -w123abc -t0x10 -w-1 openssl pkcs12 -export -passout [REDACTED]`,
		},
		{
			name:  "OpenSSL passphrase through arch wrapper",
			input: `env ARCHPREFERENCE=arm64 arch openssl pkcs12 -export -passout pass:opaque-arch-secret`,
			want:  `env ARCHPREFERENCE=arm64 arch openssl pkcs12 -export -passout [REDACTED]`,
		},
		{
			name:  "OpenSSL passphrase through arch options",
			input: `arch -arch x86_64 -d FOO-BAR -e =release openssl pkcs12 -export -passout pass:opaque-arch-secret`,
			want:  `arch -arch x86_64 -d FOO-BAR -e =release openssl pkcs12 -export -passout [REDACTED]`,
		},
		{
			name:  "OpenSSL passphrase through xcrun wrapper",
			input: `xcrun --sdk macosx --toolchain default -- openssl pkcs12 -export -passout pass:opaque-xcrun-secret`,
			want:  `xcrun --sdk macosx --toolchain default -- openssl pkcs12 -export -passout [REDACTED]`,
		},
		{
			name:  "OpenSSL passphrase through xcrun run mode",
			input: `xcrun -v -n -r openssl pkcs12 -export -passout pass:opaque-xcrun-secret`,
			want:  `xcrun -v -n -r openssl pkcs12 -export -passout [REDACTED]`,
		},
		{
			name:  "OpenSSL passphrase through single dash xcrun options",
			input: `xcrun -sdk macosx -toolchain default -verbose -no-cache -kill-cache -run -log openssl pkcs12 -export -passout pass:opaque-xcrun-secret`,
			want:  `xcrun -sdk macosx -toolchain default -verbose -no-cache -kill-cache -run -log openssl pkcs12 -export -passout [REDACTED]`,
		},
		{
			name:  "OpenSSL passphrase through launchctl asuser wrapper",
			input: `launchctl asuser 501 openssl pkcs12 -export -passout pass:opaque-launchctl-secret`,
			want:  `launchctl asuser 501 openssl pkcs12 -export -passout [REDACTED]`,
		},
		{
			name:  "OpenSSL passphrase through launchctl bsexec wrapper",
			input: `launchctl bsexec 123 openssl pkcs12 -export -passout pass:opaque-launchctl-secret`,
			want:  `launchctl bsexec 123 openssl pkcs12 -export -passout [REDACTED]`,
		},
		{
			name:  "OpenSSL passphrase through launchctl submit wrapper",
			input: `launchctl submit -l signer -o output.log -e error.log -- openssl pkcs12 -export -passout pass:opaque-launchctl-secret`,
			want:  `launchctl submit -l signer -o output.log -e error.log -- openssl pkcs12 -export -passout [REDACTED]`,
		},
		{
			name:  "OpenSSL passphrase through launchctl submit executable",
			input: `launchctl submit -l signer -p openssl -- signer pkcs12 -export -passout pass:opaque-launchctl-secret`,
			want:  `launchctl submit -l signer -p openssl -- signer pkcs12 -export -passout [REDACTED]`,
		},
		{
			name:  "OpenSSL passphrase through launchctl submit wrapper executable",
			input: `launchctl submit -l signer -p /usr/bin/env -- FOO=1 openssl pkcs12 -export -passout pass:opaque-launchctl-secret`,
			want:  `launchctl submit -l signer -p /usr/bin/env -- FOO=1 openssl pkcs12 -export -passout [REDACTED]`,
		},
		{
			name:  "OpenSSL passphrase through chroot wrapper",
			input: `chroot /mnt openssl pkcs12 -export -passout pass:opaque-chroot-secret`,
			want:  `chroot /mnt openssl pkcs12 -export -passout [REDACTED]`,
		},
		{
			name:  "OpenSSL passphrase through BSD chroot options",
			input: `sudo -P chroot -u build -g staff -G staff,wheel /mnt env PROFILE=release openssl pkcs12 -export -passout pass:opaque-chroot-secret`,
			want:  `sudo -P chroot -u build -g staff -G staff,wheel /mnt env PROFILE=release openssl pkcs12 -export -passout [REDACTED]`,
		},
		{
			name:  "OpenSSL passphrase through GNU chroot options",
			input: `chroot --userspec=build:staff --groups= --skip-chdir . openssl pkcs12 -export -passout pass:opaque-chroot-secret`,
			want:  `chroot --userspec=build:staff --groups= --skip-chdir . openssl pkcs12 -export -passout [REDACTED]`,
		},
		{
			name:  "OpenSSL passphrase through Homebrew GNU chroot",
			input: `/opt/homebrew/bin/gchroot --userspec=build:staff /mnt openssl pkcs12 -export -passout pass:opaque-gchroot-secret`,
			want:  `/opt/homebrew/bin/gchroot --userspec=build:staff /mnt openssl pkcs12 -export -passout [REDACTED]`,
		},
		{
			name:  "OpenSSL passphrase through MSYS chroot executable",
			input: `C:\msys64\usr\bin\chroot.exe --userspec=build:staff C:\root openssl.exe pkcs12 -export -passout pass:opaque-chroot-exe-secret`,
			want:  `C:\msys64\usr\bin\chroot.exe --userspec=build:staff C:\root openssl.exe pkcs12 -export -passout [REDACTED]`,
		},
		{
			name:  "OpenSSL passphrase through FreeBSD chroot option",
			input: `chroot -n /mnt openssl pkcs12 -export -passout pass:opaque-chroot-secret`,
			want:  `chroot -n /mnt openssl pkcs12 -export -passout [REDACTED]`,
		},
		{
			name:  "OpenSSL passphrase through chroot option terminator",
			input: `chroot -- /mnt openssl pkcs12 -export -passout pass:opaque-chroot-secret`,
			want:  `chroot -- /mnt openssl pkcs12 -export -passout [REDACTED]`,
		},
		{
			name:  "OpenSSL pkeyutl public key option passphrase",
			input: `openssl pkeyutl -decrypt -inkey key.pem -pkeyopt_passin rsa_padding_mode:opaque-pkeyutl-passphrase`,
			want:  `openssl pkeyutl -decrypt -inkey key.pem -pkeyopt_passin [REDACTED]`,
		},
		{
			name:  "OpenSSL pkeyutl public key option passphrase through wrapper",
			input: `sudo -P /usr/bin/openssl pkeyutl -pkeyopt_passin=option:prefix$(printf opaque-pkeyutl)suffix -inkey key.pem`,
			want:  `sudo -P /usr/bin/openssl pkeyutl -pkeyopt_passin=[REDACTED] -inkey key.pem`,
		},
		{
			name:  "OpenSSL client and server PSK values",
			input: `openssl s_client -connect example.com:443 -psk 001122opaqueclientaabb && C:\\OpenSSL\\bin\\openssl.exe s_server -accept 443 -psk=ffeeddopaqueserverccbb`,
			want:  `openssl s_client -connect example.com:443 -psk [REDACTED] && C:\\OpenSSL\\bin\\openssl.exe s_server -accept 443 -psk=[REDACTED]`,
		},
		{
			name:  "OpenSSL client and server password sources",
			input: `openssl s_client -proxy proxy.example:8080 -proxy_pass pass:opaque-proxy-password -srpuser build -srppass opaque-srp-password && openssl s_server -cert server.pem -dpass=pass:opaque-second-key-password`,
			want:  `openssl s_client -proxy proxy.example:8080 -proxy_pass [REDACTED] -srpuser build -srppass [REDACTED] && openssl s_server -cert server.pem -dpass=[REDACTED]`,
		},
		{
			name:  "OpenSSL double-dash passphrase and PSK options",
			input: `openssl pkeyutl --pkeyopt_passin option:opaque-passphrase -inkey key.pem && openssl s_client --psk=001122opaqueclientaabb`,
			want:  `openssl pkeyutl --pkeyopt_passin [REDACTED] -inkey key.pem && openssl s_client --psk=[REDACTED]`,
		},
		{
			name:  "OpenSSL PSK preserves shell comment prose",
			input: `openssl s_client -psk 001122opaqueclientaabb # note --psk public-comment-value`,
			want:  `openssl s_client -psk [REDACTED] # note --psk public-comment-value`,
		},
		{
			name:  "OpenSSL PSK after dash-leading option argument",
			input: `openssl s_client -connect -- -psk 001122opaqueclientaabb`,
			want:  `openssl s_client -connect -- -psk [REDACTED]`,
		},
		{
			name:  "OpenSSL escaped credential options",
			input: `openssl pkeyutl \-pkeyopt_passin option:opaque-passphrase && openssl s_server \--psk=001122opaquepskaabb`,
			want:  `openssl pkeyutl \-pkeyopt_passin [REDACTED] && openssl s_server \--psk=[REDACTED]`,
		},
		{
			name:  "OpenSSL pkeyutl passphrase from Fish substitution",
			input: `openssl pkeyutl -pkeyopt_passin option:(printf opaque-fish-passphrase) -inkey key.pem`,
			want:  `openssl pkeyutl -pkeyopt_passin [REDACTED] -inkey key.pem`,
		},
		{
			name:  "OpenSSL pkeyutl passphrase from nested substitutions",
			input: `openssl pkeyutl -pkeyopt_passin option:$(printf $(printf opaque-nested-passphrase))`,
			want:  `openssl pkeyutl -pkeyopt_passin [REDACTED]`,
		},
		{
			name:  "OpenSSL pkeyutl passphrase from process substitution",
			input: `openssl pkeyutl -pkeyopt_passin option:<(printf opaque-process-passphrase)`,
			want:  `openssl pkeyutl -pkeyopt_passin [REDACTED]`,
		},
		{
			name:  "OpenSSL pkeyutl passphrase from nested Fish substitutions",
			input: `openssl pkeyutl -pkeyopt_passin option:(printf (printf opaque-nested-fish-passphrase))`,
			want:  `openssl pkeyutl -pkeyopt_passin [REDACTED]`,
		},
		{
			name:  "OpenSSL credential options in ANSI-C quotes",
			input: `openssl s_client $'--psk=001122opaqueclientaabb' && openssl pkeyutl $'-pkeyopt_passin' option:opaque-passphrase`,
			want:  `openssl s_client $'--psk=[REDACTED]' && openssl pkeyutl $'-pkeyopt_passin' [REDACTED]`,
		},
		{
			name:  "OpenSSL credential options with ANSI-C escaped dashes",
			input: `openssl pkeyutl $'\x2dpkeyopt_passin' option:opaque-hex-passphrase && openssl pkeyutl $'\u002dpkeyopt_passin=option:opaque-unicode-passphrase'`,
			want:  `openssl pkeyutl $'\x2dpkeyopt_passin' [REDACTED] && openssl pkeyutl $'\u002dpkeyopt_passin=[REDACTED]'`,
		},
		{
			name:  "OpenSSL credential option with ANSI-C escaped separator",
			input: `openssl pkeyutl $'\x2dpkeyopt_passin\x3doption:opaque-encoded-separator-passphrase'`,
			want:  `openssl pkeyutl [REDACTED]`,
		},
		{
			name:  "OpenSSL credential options in locale quotes",
			input: `openssl s_client $"--psk=001122opaqueclientaabb" && openssl pkeyutl $"-pkeyopt_passin" option:opaque-passphrase`,
			want:  `openssl s_client $"--psk=[REDACTED]" && openssl pkeyutl $"-pkeyopt_passin" [REDACTED]`,
		},
		{
			name:  "OpenSSL credential nested in Fish option value",
			input: `openssl s_client -connect (openssl pkeyutl --pkeyopt_passin opaque-inner-passphrase)`,
			want:  `openssl s_client -connect (openssl pkeyutl --pkeyopt_passin [REDACTED])`,
		},
		{
			name:  "OpenSSL credentials in Fish variable slice substitutions",
			input: `echo $PATH[(openssl s_client --psk opaque-fish-slice-secret)]; set x $arr[(openssl pkeyutl --pkeyopt_passin opaque-fish-slice-passphrase)]; echo $PATH[1][(openssl s_client --psk opaque-chained-slice-secret)]; echo $$arr[1][(openssl s_client --psk opaque-indirect-slice-secret)]`,
			want:  `echo $PATH[(openssl s_client --psk [REDACTED])]; set x $arr[(openssl pkeyutl --pkeyopt_passin [REDACTED])]; echo $PATH[1][(openssl s_client --psk [REDACTED])]; echo $$arr[1][(openssl s_client --psk [REDACTED])]`,
		},
		{
			name:  "OpenSSL PSK inside subshell group",
			input: `(openssl s_client -psk 001122opaqueclientaabb)`,
			want:  `(openssl s_client -psk [REDACTED])`,
		},
		{
			name:  "OpenSSL PSK inside conditional subshell group",
			input: `if (openssl s_client -psk 001122opaqueclientaabb); then :; fi`,
			want:  `if (openssl s_client -psk [REDACTED]); then :; fi`,
		},
		{
			name:  "keytool keystore and key passwords",
			input: `keytool -importkeystore -srcstorepass opaque-source-store -deststorepass=opaque-destination-store -srckeypass "opaque source key" -destkeypass 'opaque destination key' -storepass opaque-store -keypass opaque-key`,
			want:  `keytool -importkeystore -srcstorepass [REDACTED] -deststorepass=[REDACTED] -srckeypass [REDACTED] -destkeypass [REDACTED] -storepass [REDACTED] -keypass [REDACTED]`,
		},
		{
			name:  "keytool new password through env wrapper",
			input: "env PROFILE=release /usr/bin/keytool -keypasswd -storepass old-store -keypass old-key -new \\\n  new-key -alias signing",
			want:  "env PROFILE=release /usr/bin/keytool -keypasswd -storepass [REDACTED] -keypass [REDACTED] -new \\\n  [REDACTED] -alias signing",
		},
		{
			name:  "Windows keytool password sources",
			input: `C:\Java\bin\keytool.exe -list -storepass:env STORE_PASSWORD -keypass:file key-password.txt`,
			want:  `C:\Java\bin\keytool.exe -list -storepass:env [REDACTED] -keypass:file [REDACTED]`,
		},
		{
			name:  "attached macOS security credentials",
			input: `security set-keychain-password -oopaque-old -popaque-new build.keychain && security add-generic-password -wopaque-generic -Xdeadbeef build.keychain && security import signing.p12 -Popaque-import`,
			want:  `security set-keychain-password -o[REDACTED] -p[REDACTED] build.keychain && security add-generic-password -w[REDACTED] -X[REDACTED] build.keychain && security import signing.p12 -P[REDACTED]`,
		},
		{
			name:  "attached short credentials beginning with hyphens",
			input: `security unlock-keychain -p--security-secret build.keychain && docker login -p=--docker-secret registry.example && zip -P---zip-secret archive.zip file.txt`,
			want:  `security unlock-keychain -p[REDACTED] build.keychain && docker login -p=[REDACTED] registry.example && zip -P[REDACTED] archive.zip file.txt`,
		},
		{
			name:  "attached short credential concatenated from shell fragments",
			input: `docker login -pfoo"bar-secret" registry.example`,
			want:  `docker login -p[REDACTED] registry.example`,
		},
		{
			name:  "attached short credential with embedded command substitution",
			input: `docker login -pfoo$(printf opaque-short-secret)bar registry.example`,
			want:  `docker login -p[REDACTED] registry.example`,
		},
		{
			name:  "attached short credential with deeply nested command substitution",
			input: `docker login -pfoo$(printf $(printf $(printf opaque-short-secret)))bar registry.example`,
			want:  `docker login -p[REDACTED] registry.example`,
		},
		{
			name:  "macOS security attached password long-form spelling",
			input: `security unlock-keychain -password build.keychain`,
			want:  `security unlock-keychain -p[REDACTED] build.keychain`,
		},
		{
			name:  "jarsigner keystore and key passwords",
			input: `jarsigner -storepass opaque-store-secret -keypass opaque-key-secret signed.jar alias`,
			want:  `jarsigner -storepass [REDACTED] -keypass [REDACTED] signed.jar alias`,
		},
		{
			name:  "Windows jarsigner password sources through wrapper",
			input: `sudo -u build C:\Java\bin\jarsigner.exe -storepass:env STORE_PASSWORD -keypass:file key-password.txt signed.jar alias`,
			want:  `sudo -u build C:\Java\bin\jarsigner.exe -storepass:env [REDACTED] -keypass:file [REDACTED] signed.jar alias`,
		},
		{
			name:  "mixed-separator Windows jarsigner path",
			input: `C:/Java/bin\jarsigner.exe -storepass opaque-store-secret signed.jar alias`,
			want:  `C:/Java/bin\jarsigner.exe -storepass [REDACTED] signed.jar alias`,
		},
		{
			name:  "relative Windows jarsigner path",
			input: `bin\jarsigner.exe -storepass opaque-store-secret signed.jar alias`,
			want:  `bin\jarsigner.exe -storepass [REDACTED] signed.jar alias`,
		},
		{
			name:  "drive-relative Windows jarsigner path",
			input: `C:Java\bin\jarsigner.exe -storepass opaque-store-secret signed.jar alias`,
			want:  `C:Java\bin\jarsigner.exe -storepass [REDACTED] signed.jar alias`,
		},
		{
			name:  "jarsigner passwords through shell command string",
			input: `bash -c 'jarsigner -storepass opaque-store-secret -keypass=opaque-key-secret signed.jar alias'`,
			want:  `bash -c 'jarsigner -storepass [REDACTED] -keypass=[REDACTED] signed.jar alias'`,
		},
		{
			name:  "attached Docker and archive passwords",
			input: `docker login -popaque-docker registry.example && zip -Popaque-zip archive.zip file.txt && unzip -Popaque-unzip archive.zip`,
			want:  `docker login -p[REDACTED] registry.example && zip -P[REDACTED] archive.zip file.txt && unzip -P[REDACTED] archive.zip`,
		},
		{
			name:  "sshpass password argument",
			input: `sshpass -p --opaque-ssh-password ssh build@example.com`,
			want:  `sshpass -p [REDACTED] ssh build@example.com`,
		},
		{
			name:  "sshpass attached grouped password argument",
			input: `sudo -P /usr/bin/sshpass -vp=--opaque-ssh-password ssh build@example.com`,
			want:  `sudo -P /usr/bin/sshpass -vp=[REDACTED] ssh build@example.com`,
		},
		{
			name:  "sshpass password from command substitution",
			input: "sshpass -v \\\n-p \"prefix$(printf opaque-ssh-password)suffix\" ssh build@example.com",
			want:  "sshpass -v \\\n-p [REDACTED] ssh build@example.com",
		},
		{
			name:  "Windows sshpass attached password argument",
			input: `.\\bin\\sshpass.exe -p--opaque-ssh-password ssh build@example.com`,
			want:  `.\\bin\\sshpass.exe -p[REDACTED] ssh build@example.com`,
		},
		{
			name:  "ssh-keygen old and new passphrases",
			input: `ssh-keygen -p -P "opaque old passphrase" -N '--opaque-new-passphrase' -f id_ed25519`,
			want:  `ssh-keygen -p -P [REDACTED] -N [REDACTED] -f id_ed25519`,
		},
		{
			name:  "Windows ssh-keygen attached passphrases through wrapper",
			input: `sudo -P C:\\Windows\\System32\\OpenSSH\\ssh-keygen.exe -p -Popaque-old-passphrase -N=opaque-new-passphrase -f id_ed25519`,
			want:  `sudo -P C:\\Windows\\System32\\OpenSSH\\ssh-keygen.exe -p -P[REDACTED] -N=[REDACTED] -f id_ed25519`,
		},
		{
			name:  "ssh-keygen passphrases through shell command string",
			input: `bash -c 'ssh-keygen -p -P prefix$(printf opaque-old)suffix -N opaque-new -f id_ed25519'`,
			want:  `bash -c 'ssh-keygen -p -P [REDACTED] -N [REDACTED] -f id_ed25519'`,
		},
		{
			name:  "ssh-keygen grouped passphrase option",
			input: `env PROFILE=release /usr/bin/ssh-keygen -qN--opaque-new-passphrase -f id_ed25519`,
			want:  `env PROFILE=release /usr/bin/ssh-keygen -qN[REDACTED] -f id_ed25519`,
		},
		{
			name:  "keytool through option-bearing wrappers",
			input: `sudo -u build env -i PROFILE=release keytool -list -storepass opaque-store`,
			want:  `sudo -u build env -i PROFILE=release keytool -list -storepass [REDACTED]`,
		},
		{
			name:  "OpenSSL through option-bearing wrapper",
			input: `sudo --user=build openssl pkcs12 -passout pass:opaque-output`,
			want:  `sudo --user=build openssl pkcs12 -passout [REDACTED]`,
		},
		{
			name:  "OpenSSL through continued wrapper options",
			input: "sudo \\\n-P openssl pkcs12 -passout pass:opaque-output",
			want:  "sudo \\\n-P openssl pkcs12 -passout [REDACTED]",
		},
		{
			name:  "OpenSSL key after similarly named wrapper option",
			input: `sudo -K openssl enc -K opaque-key -in plaintext`,
			want:  `sudo -K openssl enc -K [REDACTED] -in plaintext`,
		},
		{
			name:  "OpenSSL through wrapper with quoted option argument",
			input: `sudo -p "enter credentials: " openssl pkcs12 -export -passout pass:opaque-output`,
			want:  `sudo -p "enter credentials: " openssl pkcs12 -export -passout [REDACTED]`,
		},
		{
			name:  "OpenSSL through wrapper with empty quoted option argument",
			input: `sudo -p "" "/usr/bin/openssl" pkcs12 -export -passout pass:opaque-output`,
			want:  `sudo -p "" "/usr/bin/openssl" pkcs12 -export -passout [REDACTED]`,
		},
		{
			name:  "OpenSSL through wrapper with escaped quoted option argument",
			input: `sudo -p "enter \"credentials\": " openssl pkcs12 -export -passout pass:opaque-output`,
			want:  `sudo -p "enter \"credentials\": " openssl pkcs12 -export -passout [REDACTED]`,
		},
		{
			name:  "OpenSSL through env split string with quoted wrapper argument",
			input: `env -S "sudo -p 'enter credentials: ' openssl pkcs12 -export -passout pass:opaque-output"`,
			want:  `env -S "sudo -p 'enter credentials: ' openssl pkcs12 -export -passout [REDACTED]"`,
		},
		{
			name:  "OpenSSL through env split string shell command",
			input: `env -S "sh -c 'openssl pkcs12 -export -passout pass:opaque-output'"`,
			want:  `env -S "sh -c 'openssl pkcs12 -export -passout [REDACTED]'"`,
		},
		{
			name:  "OpenSSL through attached env split string shell command",
			input: `env --split-string="sh -c 'openssl pkcs12 -export -passout pass:opaque-output'"`,
			want:  `env --split-string="sh -c 'openssl pkcs12 -export -passout [REDACTED]'"`,
		},
		{
			name:  "OpenSSL through escaped env split string command",
			input: `env --split-string=openssl\ pkcs12\ -export\ -passout\ pass:opaque-output`,
			want:  `env --split-string=[REDACTED]`,
		},
		{
			name:  "keytool through doas and command wrappers",
			input: `doas -u build command -p keytool -list -storepass opaque-store`,
			want:  `doas -u build command -p keytool -list -storepass [REDACTED]`,
		},
		{
			name:  "Docker login short password",
			input: `docker login -u build -p opaque-docker-secret registry.example`,
			want:  `docker login -u build -p [REDACTED] registry.example`,
		},
		{
			name:  "Docker login short password with global option and wrapper",
			input: `sudo -u build docker --config /tmp/docker login --username build -p=opaque-docker-secret registry.example`,
			want:  `sudo -u build docker --config /tmp/docker login --username build -p=[REDACTED] registry.example`,
		},
		{
			name:  "Docker password after similarly named wrapper option",
			input: `sudo -p public-prompt docker login -p opaque-docker-secret registry.example`,
			want:  `sudo -p public-prompt docker login -p [REDACTED] registry.example`,
		},
		{
			name:  "Docker password after unindented continuation",
			input: "docker login \\\n-popaque-docker-secret registry.example",
			want:  "docker login \\\n-p[REDACTED] registry.example",
		},
		{
			name:  "Docker password through continued command wrapper",
			input: "command \\\n-p docker login -p opaque-docker-secret registry.example",
			want:  "command \\\n-p docker login -p [REDACTED] registry.example",
		},
		{
			name:  "Windows Docker login short password",
			input: `C:\Docker\docker.exe login -p "opaque docker secret" registry.example`,
			want:  `C:\Docker\docker.exe login -p [REDACTED] registry.example`,
		},
		{
			name:  "Zip password argument",
			input: `zip -P opaque-zip-secret archive.zip file`,
			want:  `zip -P [REDACTED] archive.zip file`,
		},
		{
			name:  "Zip password after unindented continuation",
			input: "zip \\\n-Popaque-zip-secret archive.zip file",
			want:  "zip \\\n-P[REDACTED] archive.zip file",
		},
		{
			name:  "Zip password through wrapper",
			input: `sudo -u build /usr/bin/zip -P="opaque zip secret" archive.zip file`,
			want:  `sudo -u build /usr/bin/zip -P=[REDACTED] archive.zip file`,
		},
		{
			name:  "Zip password after similarly named wrapper option",
			input: `sudo -P zip -P opaque-zip-secret archive.zip file`,
			want:  `sudo -P zip -P [REDACTED] archive.zip file`,
		},
		{
			name:  "Zip password through continued xargs wrapper",
			input: "xargs \\\n-P 4 zip -P opaque-zip-secret archive.zip file",
			want:  "xargs \\\n-P 4 zip -P [REDACTED] archive.zip file",
		},
		{
			name:  "Zip password through continued sudo wrapper",
			input: "sudo \\\n-P zip -P opaque-zip-secret archive.zip file",
			want:  "sudo \\\n-P zip -P [REDACTED] archive.zip file",
		},
		{
			name:  "Zip password through continued env wrapper",
			input: "env \\\n-P /usr/bin zip -P opaque-zip-secret archive.zip file",
			want:  "env \\\n-P /usr/bin zip -P [REDACTED] archive.zip file",
		},
		{
			name:  "Windows Zip password argument",
			input: `C:\Tools\zip.exe -P opaque-zip-secret archive.zip file`,
			want:  `C:\Tools\zip.exe -P [REDACTED] archive.zip file`,
		},
		{
			name:  "Unzip password argument",
			input: `unzip -P opaque-zip-secret archive.zip`,
			want:  `unzip -P [REDACTED] archive.zip`,
		},
		{
			name:  "scoped base64 private key assignments",
			input: `ASC_STOREKIT_PRIVATE_KEY_B64=c3RvcmVraXQtcHJpdmF0ZS1rZXk= ASC_ADS_PRIVATE_KEY_B64=YWRzLXByaXZhdGUta2V5`,
			want:  `ASC_STOREKIT_PRIVATE_KEY_B64=[REDACTED] ASC_ADS_PRIVATE_KEY_B64=[REDACTED]`,
		},
		{
			name:  "OpenSSL passphrase in command substitution",
			input: `result=$(openssl pkcs12 -export -passout pass:opaque-substitution-secret)`,
			want:  `result=$(openssl pkcs12 -export -passout [REDACTED])`,
		},
		{
			name:  "OpenSSL passphrase in backtick substitution",
			input: "echo `openssl pkcs12 -export -passout pass:opaque-substitution-secret`",
			want:  "echo `openssl pkcs12 -export -passout [REDACTED]`",
		},
		{
			name:  "OpenSSL passphrase in process substitution",
			input: `diff <(openssl pkcs12 -export -passout pass:opaque-substitution-secret) expected.txt`,
			want:  `diff <(openssl pkcs12 -export -passout [REDACTED]) expected.txt`,
		},
		{
			name:  "leading underscore token assignment",
			input: `//registry.npmjs.org/:_authToken=npm-secret`,
			want:  `//registry.npmjs.org/:_authToken=[REDACTED]`,
		},
		{
			name:  "registry base64 auth assignment",
			input: `//registry.npmjs.org/:_auth=b3BhcXVlLXNlY3JldA==`,
			want:  `//registry.npmjs.org/:_auth=[REDACTED]`,
		},
		{
			name:  "registry configuration auth value",
			input: `{"auths":{"registry.example":{"auth":"YWxpY2U6b3BhcXVlLXNlY3JldA=="}},"credsStore":"desktop"}`,
			want:  `{"auths":{"registry.example":{"auth":"[REDACTED]"}},"credsStore":"desktop"}`,
		},
		{
			name:  "escaped registry configuration auth value",
			input: `trace {\"auths\":{\"registry.example\":{\"auth\":\"YWxpY2U6b3BhcXVlLXNlY3JldA==\"}},\"status\":\"failed\"}`,
			want:  `trace {\"auths\":{\"registry.example\":{\"auth\":\"[REDACTED]\"}},\"status\":\"failed\"}`,
		},
		{
			name:  "JSON assignment",
			input: `response {"refresh_token":"refresh-value","status":"failed"}`,
			want:  `response {"refresh_token":"[REDACTED]","status":"failed"}`,
		},
		{
			name:  "JSON unicode escape in credential key",
			input: `response {"pass\u0077ord":"opaque-unicode-secret","status":"failed"}`,
			want:  `response {"pass\u0077ord":"[REDACTED]","status":"failed"}`,
		},
		{
			name:  "escaped JSON unicode escape in credential key",
			input: `trace {\"pass\\u0077ord\":\"opaque-escaped-unicode-secret\",\"status\":\"failed\"}`,
			want:  `trace {\"pass\\u0077ord\":\"[REDACTED]\",\"status\":\"failed\"}`,
		},
		{
			name:  "double escaped JSON unicode escape in credential key",
			input: `trace {\\\"pass\\\\u0077ord\\\":\\\"opaque-double-escaped-secret\\\",\\\"status\\\":\\\"failed\\\"}`,
			want:  `trace {\\\"pass\\\\u0077ord\\\":\\\"[REDACTED]\\\",\\\"status\\\":\\\"failed\\\"}`,
		},
		{
			name:  "prefixed JSON assignments",
			input: `response {"AWS_SECRET_ACCESS_KEY":"cloud-secret-value","MY_CLIENT_SECRET":"client secret value"}`,
			want:  `response {"AWS_SECRET_ACCESS_KEY":"[REDACTED]","MY_CLIENT_SECRET":"[REDACTED]"}`,
		},
		{
			name:  "secret key JSON assignments",
			input: `response {"secret_key":"opaque-json-secret","secret_key_base":"opaque-base-secret","status":"failed"}`,
			want:  `response {"secret_key":"[REDACTED]","secret_key_base":"[REDACTED]","status":"failed"}`,
		},
		{
			name:  "pretty-printed JSON assignment",
			input: "response {\n  \"client_secret\":\n    \"arbitrary secret\",\n  \"status\": \"failed\"\n}",
			want:  "response {\n  \"client_secret\":\n    \"[REDACTED]\",\n  \"status\": \"failed\"\n}",
		},
		{
			name:  "YAML literal secret block",
			input: "client_secret: |\n  super\n  sensitive\nstatus: failed",
			want:  "client_secret: [REDACTED]\nstatus: failed",
		},
		{
			name:  "tagged YAML credential key",
			input: `!!str password: opaque-tagged-key-secret`,
			want:  `!!str password: [REDACTED]`,
		},
		{
			name:  "anchored YAML credential key",
			input: `&credential password: opaque-anchored-key-secret`,
			want:  `&credential password: [REDACTED]`,
		},
		{
			name: "sequence tagged quoted YAML credential key",
			input: `items:
  - !!str "password": "opaque tagged quoted key secret"`,
			want: `items:
  - !!str "password": "[REDACTED]"`,
		},
		{
			name:  "tagged escaped YAML credential key",
			input: `!!str "pass\u0077ord": opaque-tagged-escaped-key-secret`,
			want:  `!!str "pass\u0077ord": [REDACTED]`,
		},
		{
			name:  "tagged YAML credential key block scalar",
			input: "!!str password: |\n  opaque-tagged-key-head\n  opaque-tagged-key-tail\nstatus: failed",
			want:  "!!str password: [REDACTED]\nstatus: failed",
		},
		{
			name:  "tagged YAML credential key flow value",
			input: `!!str password: {nested: opaque-tagged-key-secret, status: failed}`,
			want:  `!!str password: [REDACTED]`,
		},
		{
			name:  "tagged YAML credential alias",
			input: "source: &credential opaque-tagged-alias-secret\n!!str password: *credential",
			want:  "source: &credential [REDACTED]\n!!str password: [REDACTED]",
		},
		{
			name:  "tagged YAML credential key in flow object",
			input: `{!!str password: opaque-tagged-flow-key-secret, status: failed}`,
			want:  `{!!str password: [REDACTED], status: failed}`,
		},
		{
			name:  "YAML sequence literal secret block",
			input: "items:\n  - password: |\n      super\n      sensitive\nstatus: failed",
			want:  "items:\n  - password: [REDACTED]\nstatus: failed",
		},
		{
			name:  "YAML flow sequence credential",
			input: "password: [first-secret, second-secret]\nstatus: failed",
			want:  "password: [REDACTED]\nstatus: failed",
		},
		{
			name:  "nested single-line YAML flow sequence credential",
			input: "password: [first-secret, [second-secret], third-secret]\nstatus: failed",
			want:  "password: [REDACTED]\nstatus: failed",
		},
		{
			name:  "quoted first nested YAML flow under single quoted key",
			input: "'password': [\"first-secret\", [second-secret], third-secret]\nstatus: failed",
			want:  "'password': [REDACTED]\nstatus: failed",
		},
		{
			name:  "nested single-line YAML flow mapping credential",
			input: "token: {type: bearer, nested: {value: opaque-secret}, tail: preserved}\nstatus: failed",
			want:  "token: [REDACTED]\nstatus: failed",
		},
		{
			name:  "quoted YAML flow sequence credential",
			input: "\"password\": [first-secret, second-secret]\nstatus: failed",
			want:  "\"password\": [REDACTED]\nstatus: failed",
		},
		{
			name:  "multiline YAML flow sequence credential",
			input: "password: [first-secret,\n  second-secret]\nstatus: failed",
			want:  "password: [REDACTED]\nstatus: failed",
		},
		{
			name:  "quoted first multiline YAML flow sequence credential",
			input: "password: [\n  \"opaque-first\",\n  \"opaque-second\"\n]\nstatus: failed",
			want:  "password: [REDACTED]\nstatus: failed",
		},
		{
			name:  "multiline YAML flow mapping credential",
			input: "token: {type: bearer,\n  value: opaque-secret}\nstatus: failed",
			want:  "token: [REDACTED]\nstatus: failed",
		},
		{
			name:  "YAML block mapping credential",
			input: "token:\n  type: bearer\n  value: opaque-secret\nstatus: failed",
			want:  "token: [REDACTED]\nstatus: failed",
		},
		{
			name:  "nested YAML block mapping preserves sibling field",
			input: "response:\n  token:\n    type: bearer\n    value: opaque-secret\n  status: failed",
			want:  "response:\n  token: [REDACTED]\n  status: failed",
		},
		{
			name:  "quoted YAML block mapping credential",
			input: "response:\n  \"password\":\n    value: quoted-map-secret\n  status: failed",
			want:  "response:\n  \"password\": [REDACTED]\n  status: failed",
		},
		{
			name:  "anchored YAML block mapping credential",
			input: "response:\n  token: &auth\n    value: anchored-map-secret\n  status: failed",
			want:  "response:\n  token: [REDACTED]\n  status: failed",
		},
		{
			name:  "tagged YAML block mapping credential",
			input: "response:\n  token: !credential\n    value: tagged-map-secret\n  status: failed",
			want:  "response:\n  token: [REDACTED]\n  status: failed",
		},
		{
			name:  "sequence YAML block mapping preserves sibling field",
			input: "items:\n  - token:\n      value: opaque-secret\n    status: failed",
			want:  "items:\n  - token: [REDACTED]\n    status: failed",
		},
		{
			name:  "nested YAML block scalar preserves sibling field",
			input: "response:\n  client_secret: |\n    opaque-secret\n  status: failed",
			want:  "response:\n  client_secret: [REDACTED]\n  status: failed",
		},
		{
			name:  "quoted YAML block scalar with quoted content",
			input: "response:\n  \"password\": |\n    \"opaque-secret\"\n  status: failed",
			want:  "response:\n  \"password\": [REDACTED]\n  status: failed",
		},
		{
			name:  "anchored YAML block scalar",
			input: "response:\n  password: &credential |\n    opaque-secret\n  status: failed",
			want:  "response:\n  password: [REDACTED]\n  status: failed",
		},
		{
			name:  "tagged YAML block scalar",
			input: "response:\n  password: !!str |\n    opaque-secret\n  status: failed",
			want:  "response:\n  password: [REDACTED]\n  status: failed",
		},
		{
			name:  "YAML folded base64 private key block",
			input: "private_key_b64: >- # encoded key\n  c3VwZXI=\n\n  c2VjcmV0\nnext: preserved",
			want:  "private_key_b64: [REDACTED]\nnext: preserved",
		},
		{
			name:  "kubeconfig client key data",
			input: "client-key-data: b3BhcXVlLXByaXZhdGUta2V5\nclient-certificate-data: public-certificate",
			want:  "client-key-data: [REDACTED]\nclient-certificate-data: public-certificate",
		},
		{
			name:  "camel case JSON assignments",
			input: `response {"demoAccountPassword":"review-secret","awsSecretAccessKey":"cloud-secret"}`,
			want:  `response {"demoAccountPassword":"[REDACTED]","awsSecretAccessKey":"[REDACTED]"}`,
		},
		{
			name:  "sandbox secret answer preserves question",
			input: `{"secretQuestion":"Public question","secretAnswer":"recovery-answer-secret","status":"active"}`,
			want:  `{"secretQuestion":"Public question","secretAnswer":"[REDACTED]","status":"active"}`,
		},
		{
			name:  "escaped JSON assignment",
			input: `response {\"client_secret\":\"super\\\"sensitive\",\"status\":\"failed\"}`,
			want:  `response {\"client_secret\":\"[REDACTED]\",\"status\":\"failed\"}`,
		},
		{
			name:  "JWT",
			input: "decoded eyJhbGciOiJFUzI1NiJ9.cGF5bG9hZA.c2lnbmF0dXJl failed",
			want:  "decoded [REDACTED] failed",
		},
		{
			name:  "known opaque token",
			input: "credential ghp_abcdefghijklmnopqrstuvwxyz123456",
			want:  "credential [REDACTED]",
		},
		{
			name:  "Slack bot token",
			input: "credential xoxb-123456789012-123456789012-abcdefghijklmnopqrstuvwx",
			want:  "credential [REDACTED]",
		},
		{
			name:  "Slack user token",
			input: "credential xoxp-123456789012-123456789012-abcdefghijklmnopqrstuvwx",
			want:  "credential [REDACTED]",
		},
		{
			name:  "Slack user token documented shape",
			input: "credential xoxp-abcdef-abcdef-abcdef-abcdef",
			want:  "credential [REDACTED]",
		},
		{
			name:  "Slack app token",
			input: "credential xapp-1-123456789012-123456789012-abcdefghijklmnopqrstuvwx",
			want:  "credential [REDACTED]",
		},
		{
			name:  "Slack app token documented shape",
			input: "credential xapp-1-A0123456789-example",
			want:  "credential [REDACTED]",
		},
		{
			name:  "npm access token",
			input: "credential npm_abcdefghijklmnopqrstuvwxyz0123456789",
			want:  "credential [REDACTED]",
		},
		{
			name:  "GitLab personal access token",
			input: "credential glpat-abcdefghijklmnopqrstuvwxyz",
			want:  "credential [REDACTED]",
		},
		{
			name:  "Stripe secret API key",
			input: "request failed for " + "sk_live_" + "51ABCDEFGHIJKLMNOPQRSTUVWXYZ",
			want:  "request failed for [REDACTED]",
		},
		{
			name:  "Stripe restricted test API key",
			input: "request failed for " + "rk_test_" + "51ABCDEFGHIJKLMNOPQRSTUVWXYZ",
			want:  "request failed for [REDACTED]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, changed := redactSensitiveText(tt.input)
			if !changed {
				t.Fatalf("redactSensitiveText() did not report a change for %q", tt.input)
			}
			if got != tt.want {
				t.Fatalf("redactSensitiveText() = %q, want %q", got, tt.want)
			}
			gotAgain, changedAgain := redactSensitiveText(got)
			if changedAgain || gotAgain != got {
				t.Fatalf("redaction is not idempotent: second result %q, changed=%t", gotAgain, changedAgain)
			}
		})
	}
}

func TestRedactSensitiveTextPreservesFalseSecretMarkerAndValue(t *testing.T) {
	const publicValue = "public-value"
	for _, literal := range []string{"0", "f", "F", "false", "False", "FALSE"} {
		input := "asc web xcode-cloud env-vars set --value " + publicValue + " --secret=" + literal
		got, changed := redactSensitiveText(input)
		if changed || got != input {
			t.Errorf("redactSensitiveText(%q) = %q, changed=%t; want unchanged", input, got, changed)
		}
	}
}

func TestRedactSensitiveTextDeeplyNestedShellCommands(t *testing.T) {
	input := strings.Repeat("$( ", 12) + "docker login -popaque-deep-secret registry.example" + strings.Repeat(" )", 12)
	got, changed := redactSensitiveText(input)
	if !changed || strings.Contains(got, "opaque-deep-secret") || !strings.Contains(got, redactionMarker) {
		t.Fatalf("redactSensitiveText() = %q, changed=%t; want nested secret redacted", got, changed)
	}
	gotAgain, changedAgain := redactSensitiveText(got)
	if changedAgain || gotAgain != got {
		t.Fatalf("redaction is not idempotent: second result %q, changed=%t", gotAgain, changedAgain)
	}
}

func TestRedactSensitiveTextFailsClosedBeyondShellNestingLimit(t *testing.T) {
	boundaryInput := strings.Repeat("$( ", maxShellRedactionDepth) + "openssl pkcs12 -passout pass:opaque-boundary-secret" + strings.Repeat(" )", maxShellRedactionDepth)
	boundaryResult, boundaryChanged := redactSensitiveText(boundaryInput)
	if !boundaryChanged || strings.Contains(boundaryResult, "opaque-boundary-secret") || !strings.Contains(boundaryResult, redactionMarker) {
		t.Fatalf("boundary redaction = %q, changed=%t; want credential-specific redaction", boundaryResult, boundaryChanged)
	}

	input := strings.Repeat("$(echo ", maxShellRedactionDepth+1) + "openssl pkcs12 -passout pass:opaque-depth-limit-secret" + strings.Repeat(")", maxShellRedactionDepth+1)
	got, changed := redactSensitiveText(input)
	if !changed || strings.Contains(got, "opaque-depth-limit-secret") || !strings.Contains(got, redactionMarker) {
		t.Fatalf("redactSensitiveText() = %q, changed=%t; want content beyond nesting limit redacted", got, changed)
	}
	gotAgain, changedAgain := redactSensitiveText(got)
	if changedAgain || gotAgain != got {
		t.Fatalf("redaction is not idempotent: second result %q, changed=%t", gotAgain, changedAgain)
	}

	if got, changed := redactSensitiveTextDepth("public boundary value", maxShellRedactionDepth); changed || got != "public boundary value" {
		t.Fatalf("boundary value = %q, changed=%t; want unchanged", got, changed)
	}
	if got, changed := redactSensitiveTextDepth("unresolved nested content", maxShellRedactionDepth+1); !changed || got != redactionMarker {
		t.Fatalf("over-limit value = %q, changed=%t; want fail-closed marker", got, changed)
	}

	for _, nested := range []string{
		`$(openssl pkcs12 -passout pass:opaque-limit-secret)`,
		"`openssl pkcs12 -passout pass:opaque-limit-secret`",
		`(openssl pkcs12 -passout pass:opaque-limit-secret)`,
		`env -S "openssl pkcs12 -passout pass:opaque-limit-secret"`,
		`sh -c 'openssl pkcs12 -passout pass:opaque-limit-secret'`,
	} {
		got, changed := redactSensitiveTextDepth(nested, maxShellRedactionDepth)
		if !changed || strings.Contains(got, "opaque-limit-secret") || !strings.Contains(got, redactionMarker) {
			t.Errorf("boundary nested redaction for %q = %q, changed=%t; want fail-closed marker", nested, got, changed)
			continue
		}
		gotAgain, changedAgain := redactSensitiveText(got)
		if changedAgain || gotAgain != got {
			t.Errorf("boundary nested redaction is not idempotent for %q: second result %q, changed=%t", nested, gotAgain, changedAgain)
		}
	}
}

func TestRedactSensitiveTextPreservesBenignShellValues(t *testing.T) {
	for _, input := range []string{
		`set "STATUS=public value" & echo done`,
		`set STATUS=opaque status value & echo done`,
		`$description = ConvertTo-SecureString "public value" -AsPlainText -Force`,
		"asc signing sync --notes @'\npublic head\npublic tail\n'@ --verbose",
		`AUTOMATION_SESSION_FILE=/tmp/session.txt`,
		`DB_PASS_FILE=/tmp/database-password tool --db-pass-file /tmp/database-password`,
		`BYPASS=enabled tool --bypass enabled`,
		`COMPASS=north tool --compass north`,
		`echo openssl pkcs12 -export -passout pass:public-value`,
		`echo openssl enc -k public-value -K public-key`,
		`echo openssl passwd -6 public-value`,
		`tool -passout public-value`,
		`tool --passin public-value --passout public-value`,
		`tool -k public-value -K public-key`,
		`openssl enc -iv public-iv -S public-salt -in public-input`,
		`openssl enc -hmac public-value -in public-input`,
		`openssl dgst -mac HMAC -macopt digest:sha256 artifact`,
		`openssl enc -macopt key:public-value -in public-input`,
		`openssl kdf -kdfopt digest:sha256 -kdfopt salt:public-salt -kdfopt info:public-info HKDF`,
		`openssl enc -kdfopt key:public-value -in public-input`,
		`openssl dgst -key public-filename artifact`,
		`openssl req -key public-filename -in request.pem`,
		`openssl ca -keyfile public-filename -in request.pem`,
		`openssl pkeyutl -pkeyopt public-option:public-value -inkey public-key.pem`,
		`openssl req -pkeyopt_passin option:public-value -in request.pem`,
		`openssl s_client -psk_identity public-identity -psk_session public-session.pem`,
		`openssl s_server -psk_hint public-hint -psk_session public-session.pem`,
		`openssl ciphers -psk public-cipher-list`,
		`openssl s_client -pskpublic-value`,
		`openssl s_server --pskpublic-value`,
		`openssl s_client -- -psk public-value`,
		`openssl s_client -brief -- -psk public-value`,
		`openssl pkeyutl -decrypt -- -pkeyopt_passin public-value`,
		`openssl pkeyutl \-- -pkeyopt_passin public-value`,
		`openssl pkeyutl -- --passin public-value`,
		`openssl pkeyutl -- --passin=public-value`,
		`openssl pkeyutl -- -passin public-value`,
		`openssl pkeyutl -- -passin=public-value`,
		`openssl pkeyutl \-- --passin public-value`,
		`openssl pkeyutl -decrypt -- --passin public-value`,
		`openssl s_client -servername --psk public-value`,
		`openssl s_client -psk_identity --psk public-value`,
		`openssl s_server -psk_hint --psk public-value`,
		`openssl pkeyutl -inkey --pkeyopt_passin public-value`,
		`openssl pkeyutl -inkey $(printf $(printf x) -pkeyopt_passin public-value)`,
		`openssl s_client -connect (printf --psk public-value)`,
		`openssl s_client -connect "foo --psk public-value"`,
		`openssl s_client --psk_identity "x --psk public-value"`,
		`openssl req -psk public-value -in request.pem`,
		`echo openssl s_client -psk public-value`,
		`echo openssl dgst -hmac public-value artifact`,
		`openssl passwd -6 -salt public-salt -in passwords.txt`,
		`openssl passwd -help public-value`,
		`if echo openssl pkcs12 -passout pass:public-value; then :; fi`,
		`(echo openssl pkcs12 -passout pass:public-value)`,
		`sudo -l openssl pkcs12 -passout pass:public-value`,
		`xargs echo openssl pkcs12 -passout pass:public-value`,
		`find . -print openssl pkcs12 -passout pass:public-value`,
		`find . -path '-exec' openssl passwd -6 public-value`,
		`find . -name '-exec' openssl passwd -6 public-value`,
		`find . -fprintf report '-exec' openssl passwd -6 public-value`,
		`watch -x echo openssl pkcs12 -passout pass:public-value`,
		`watch --help openssl pkcs12 -passout pass:public-value`,
		`values=(openssl passwd -6 public-value)`,
		`values=(foo\bin\openssl.exe passwd -6 public-array-path)`,
		`values=( docker login -ppublic-value registry.example )`,
		"values=(\n docker login -ppublic-value registry.example\n)",
		`values=( zip -Ppublic-value archive.zip file.txt )`,
		`values=( security unlock-keychain -ppublic-value build.keychain )`,
		`values=( openssl passwd -6 public-value )`,
		`echo sshpass -p public-value ssh build@example.com`,
		`command -v sshpass -p public-value ssh build@example.com`,
		`sshpass -e ssh -p 2222 build@example.com`,
		`sshpass -epublic ssh -p 2222 build@example.com`,
		`sshpass -f -p ssh build@example.com`,
		`sshpass -P -p ssh build@example.com`,
		`sshpass -- ssh -p 2222 build@example.com`,
		`sshpass -h -p public-value ssh build@example.com`,
		`echo ssh-keygen -p -P public-old -N public-new -f id_ed25519`,
		`command -v ssh-keygen -P public-old -N public-new`,
		`ssh-keygen -- -Npublic-new`,
		`ssh-keygen -f -Ppublic-key-file`,
		`ssh-keygen -O -Npublic-option-value`,
		`ssh-keygen -p -n public-principals`,
		`((openssl passwd -6 public-value))`,
		`(( x = (openssl s_client --psk public-value) ))`,
		`arr[(openssl s_client --psk public-value)]`,
		`echo $((openssl passwd -6 public-value))`,
		`echo '(true; openssl passwd -6 public-value)'`,
		`foo () { echo openssl passwd -6 public-value; }`,
		`tool --no-token output.txt`,
		`tool --database-no-password output.txt`,
		`header = "X-Request-ID: public-value"`,
		`proxy-header = "@headers.txt"`,
		`Pass through values`,
		`pass through values`,
	} {
		got, changed := redactSensitiveText(input)
		if changed || got != input {
			t.Errorf("redactSensitiveText(%q) = %q, changed=%t; want unchanged", input, got, changed)
		}
	}
}

func TestRedactSensitiveTextScopesKubectlSecretLiterals(t *testing.T) {
	for _, input := range []string{
		`echo kubectl create secret generic foo --from-literal=custom=public-value`,
		`sudo echo kubectl create secret generic foo --from-literal=custom=public-value`,
		`sudo -u build echo kubectl create secret generic foo --from-literal=custom=public-value`,
		`env -v echo kubectl create secret generic foo --from-literal=custom=public-value`,
		`env -S "echo kubectl create secret generic foo --from-literal=custom=public-value"`,
		`kubectl create configmap foo --from-literal=custom=public-value`,
		`kubectl get pods create secret generic foo --from-literal=custom=public-value`,
		`kubectl run demo -- echo create secret --from-literal=custom=public-value`,
		"kubectl create\nsecret generic foo --from-literal=custom=public-value",
	} {
		got, changed := redactSensitiveText(input)
		if changed || got != input {
			t.Errorf("redactSensitiveText(%q) = %q, changed=%t; want unchanged", input, got, changed)
		}
	}
}

func TestRedactSensitiveTextScopesSecurityCredentialArguments(t *testing.T) {
	for _, input := range []string{
		`sudo -p public-prompt security unlock-keychain build.keychain`,
		`sudo -P security unlock-keychain build.keychain`,
		`echo security unlock-keychain -p public-value build.keychain`,
		`sudo echo security unlock-keychain -p public-value build.keychain`,
		`security export -p output.pem -o exported.pem`,
		`security import signing.cer -k build.keychain`,
		`security list-keychains -p public-value`,
		`tool -p public-value`,
	} {
		got, changed := redactSensitiveText(input)
		if changed || got != input {
			t.Errorf("redactSensitiveText(%q) = %q, changed=%t; want unchanged", input, got, changed)
		}
	}
}

func TestRedactSensitiveTextScopesKeytoolCredentialArguments(t *testing.T) {
	for _, input := range []string{
		`echo keytool -list -storepass public-value`,
		`sudo echo keytool -list -storepass public-value`,
		`echo keytool.exe -list -storepass:env PUBLIC_VALUE`,
		`tool -storepass public-value`,
		`keytool -list -keystore public-value`,
	} {
		got, changed := redactSensitiveText(input)
		if changed || got != input {
			t.Errorf("redactSensitiveText(%q) = %q, changed=%t; want unchanged", input, got, changed)
		}
	}
}

func TestRedactSensitiveTextScopesJarsignerCredentialArguments(t *testing.T) {
	for _, input := range []string{
		`echo jarsigner -storepass public-value signed.jar alias`,
		`sudo echo jarsigner -keypass public-value signed.jar alias`,
		`tool -storepass public-value`,
		`jarsigner -keystore public-value signed.jar alias`,
		`jarsigner -signedjar public-output.jar input.jar alias`,
	} {
		got, changed := redactSensitiveText(input)
		if changed || got != input {
			t.Errorf("redactSensitiveText(%q) = %q, changed=%t; want unchanged", input, got, changed)
		}
	}
}

func TestRedactSensitiveTextScopesDockerLoginCredentialArguments(t *testing.T) {
	for _, input := range []string{
		`sudo -p public-prompt docker login registry.example`,
		`echo docker login -p public-value registry.example`,
		`sudo echo docker login -p public-value registry.example`,
		`docker run -p 8080:80 image`,
		`docker run image login -p public-value`,
		`docker login --password-stdin registry.example`,
		`docker --version login -p public-value registry.example`,
		`tool login -p public-value`,
	} {
		got, changed := redactSensitiveText(input)
		if changed || got != input {
			t.Errorf("redactSensitiveText(%q) = %q, changed=%t; want unchanged", input, got, changed)
		}
	}
}

func TestRedactSensitiveTextScopesZipCredentialArguments(t *testing.T) {
	for _, input := range []string{
		`sudo -P zip archive.zip file`,
		`sudo -P unzip archive.zip`,
		`echo zip -P public-value archive.zip file`,
		`sudo echo zip -P public-value archive.zip file`,
		`tool -P public-value`,
		`zipinfo -P public-value archive.zip`,
	} {
		got, changed := redactSensitiveText(input)
		if changed || got != input {
			t.Errorf("redactSensitiveText(%q) = %q, changed=%t; want unchanged", input, got, changed)
		}
	}
}

func TestRedactSensitiveTextPreservesTaggedConfigMapStructuralKeys(t *testing.T) {
	input := "!!str kind: ConfigMap\n!!str data:\n  config: public-value"
	got, changed := redactSensitiveText(input)
	if changed || got != input {
		t.Fatalf("redactSensitiveText(%q) = %q, changed=%t; want unchanged", input, got, changed)
	}
}

func TestRedactSensitiveTextPreservesTaggedYAMLNonCredentialKey(t *testing.T) {
	input := "!!str status: visible-value\n&field name: visible-name"
	got, changed := redactSensitiveText(input)
	if changed || got != input {
		t.Fatalf("redactSensitiveText(%q) = %q, changed=%t; want unchanged", input, got, changed)
	}
}

func TestRedactSensitiveTextRejectsNonExecutingCredentialCommandWrappers(t *testing.T) {
	for _, input := range []string{
		`sudo -u build echo keytool -list -storepass public-value`,
		`command -v keytool -storepass public-value`,
		`doas -u build echo openssl -passout public-value`,
		`env -S "echo openssl pkcs12 -passout public-value"`,
		`env -0 echo openssl pkcs12 -passout public-value`,
		`env --null echo openssl pkcs12 -passout public-value`,
		`timeout echo openssl pkcs12 -passout public-value`,
		`timeout 30 echo openssl pkcs12 -passout public-value`,
		`timeout --help openssl pkcs12 -passout public-value`,
		`timeout --foreground=bad 30 openssl pkcs12 -passout public-value`,
		`nohup echo openssl pkcs12 -passout public-value`,
		`nohup --help openssl pkcs12 -passout public-value`,
		`nohup --version openssl pkcs12 -passout public-value`,
		`nohup --unknown openssl pkcs12 -passout public-value`,
		`nohup -- --help openssl pkcs12 -passout public-value`,
		`nohup "" openssl pkcs12 -passout public-value`,
		`bash script.sh openssl pkcs12 -passout public-value`,
		`bash -- -c 'openssl pkcs12 -passout public-value'`,
		`bash -cq 'openssl pkcs12 -passout public-value'`,
		`bash -c 'echo openssl pkcs12 -passout public-value'`,
		`bash -c '' openssl pkcs12 -passout public-value`,
		`bash -c openssl pkcs12 -passout public-value`,
		`bash -c --`,
		`bash --help -c 'openssl pkcs12 -passout public-value'`,
		`bash -z -c 'openssl pkcs12 -passout public-value'`,
		`bash -c 'echo ok' openssl pkcs12 -passout public-value`,
		`bash -c 'exec echo openssl pkcs12 -passout public-value'`,
		`echo bash -c 'openssl pkcs12 -passout public-value'`,
		`exec --help openssl pkcs12 -passout public-value`,
		`exec echo openssl pkcs12 -passout public-value`,
		`nice echo openssl pkcs12 -passout public-value`,
		`nice --help openssl pkcs12 -passout public-value`,
		`nice -q openssl pkcs12 -passout public-value`,
		`nice -n invalid openssl pkcs12 -passout public-value`,
		`nice -n=5 openssl pkcs12 -passout public-value`,
		`nice 10 openssl pkcs12 -passout public-value`,
		`nice +5 openssl pkcs12 -passout public-value`,
		`nice -n openssl pkcs12 -passout public-value`,
		`gnice --adjustment=bad openssl pkcs12 -passout public-value`,
		`time echo openssl pkcs12 -passout public-value`,
		`time --help openssl pkcs12 -passout public-value`,
		`time -V openssl pkcs12 -passout public-value`,
		`time --output= openssl pkcs12 -passout public-value`,
		`stdbuf -oL echo openssl pkcs12 -passout public-value`,
		`stdbuf --help openssl pkcs12 -passout public-value`,
		`stdbuf -o invalid openssl pkcs12 -passout public-value`,
		`setsid echo openssl pkcs12 -passout public-value`,
		`setsid --help openssl pkcs12 -passout public-value`,
		`setsid -x openssl pkcs12 -passout public-value`,
		`ionice echo openssl pkcs12 -passout public-value`,
		`ionice --help openssl pkcs12 -passout public-value`,
		`ionice -p 123 openssl pkcs12 -passout public-value`,
		`ionice --pid=123 openssl pkcs12 -passout public-value`,
		`ionice --class= openssl pkcs12 -passout public-value`,
		`caffeinate echo openssl pkcs12 -passout public-value`,
		`caffeinate -tfoo openssl pkcs12 -passout public-value`,
		`caffeinate -wfoo openssl pkcs12 -passout public-value`,
		`caffeinate -t .5foo openssl pkcs12 -passout public-value`,
		`caffeinate -t Inf openssl pkcs12 -passout public-value`,
		`caffeinate -t -Infinity openssl pkcs12 -passout public-value`,
		`caffeinate -t -i openssl pkcs12 -passout public-value`,
		`arch -- openssl pkcs12 -passout public-value`,
		`arch -arm64 -- openssl pkcs12 -passout public-value`,
		`arch -h openssl pkcs12 -passout public-value`,
		`arch -arch bogus openssl pkcs12 -passout public-value`,
		`arch -e openssl pkcs12 -passout public-value`,
		`arch -dFOO openssl pkcs12 -passout public-value`,
		`arch -eFOO=bar openssl pkcs12 -passout public-value`,
		`xcrun --find openssl pkcs12 -passout public-value`,
		`xcrun -find openssl pkcs12 -passout public-value`,
		`xcrun --show-sdk-path openssl pkcs12 -passout public-value`,
		`xcrun -show-toolchain-path openssl pkcs12 -passout public-value`,
		`xcrun --sdk=macosx openssl pkcs12 -passout public-value`,
		`xcrun -vnrl openssl pkcs12 -passout public-value`,
		`xcrun --sdk openssl pkcs12 -passout public-value`,
		`xcrun echo openssl pkcs12 -passout public-value`,
		`launchctl asuser invalid openssl pkcs12 -passout public-value`,
		`launchctl print system openssl pkcs12 -passout public-value`,
		`launchctl submit -- openssl pkcs12 -passout public-value`,
		`launchctl submit -l signer openssl pkcs12 -passout public-value`,
		`launchctl submit -l signer -p echo -- echo openssl pkcs12 -passout public-value`,
		`launchctl asuser 501 echo openssl pkcs12 -passout public-value`,
		`chroot --help openssl pkcs12 -passout public-value`,
		`chroot -x /mnt openssl pkcs12 -passout public-value`,
		`chroot -u /mnt openssl pkcs12 -passout public-value`,
		`chroot /mnt echo openssl pkcs12 -passout public-value`,
		`echo '$(openssl pkcs12 -passout public-value)'`,
		`echo \$(openssl pkcs12 -passout public-value)`,
		`echo $(echo openssl pkcs12 -passout public-value)`,
		`diff <(echo openssl pkcs12 -passout public-value) expected.txt`,
		`sudo -p "enter credentials: " echo openssl pkcs12 -passout public-value`,
	} {
		got, changed := redactSensitiveText(input)
		if changed || got != input {
			t.Errorf("redactSensitiveText(%q) = %q, changed=%t; want unchanged", input, got, changed)
		}
	}
}

func TestRedactSensitiveTextPreservesBearerProse(t *testing.T) {
	for _, input := range []string{
		"Bearer authentication fails behind proxy",
		"Bearer OAuth2 authentication fails",
		"Bearer OAuth2.0 authentication fails",
		"Bearer OAuth2.1 authentication fails",
		"Bearer OAuth2.0-beta authentication fails",
		"Bearer HTTP2 authentication fails",
		"Bearer RFC6750 flow fails",
		"Authorization: request failed because proxy unavailable",
	} {
		got, changed := redactSensitiveText(input)
		if changed || got != input {
			t.Errorf("redactSensitiveText(%q) = %q, changed=%t; want unchanged", input, got, changed)
		}
	}
}

func TestRedactSensitiveTextPreservesEmbeddedSlackTokenLikeText(t *testing.T) {
	for _, input := range []string{
		"prefixxoxp-123456789012-123456789012-abcdefghijklmnopqrstuvwx",
		"prefixxapp-1-A0123456789-example",
	} {
		got, changed := redactSensitiveText(input)
		if changed || got != input {
			t.Errorf("redactSensitiveText(%q) = %q, changed=%t; want unchanged", input, got, changed)
		}
	}
}

func TestRedactSensitiveTextPreservesEmbeddedGoogleAPIKeyLikeText(t *testing.T) {
	for _, input := range []string{
		"prefixAIza0123456789abcdefghijklmnopqrstuvwxy",
		"AIza0123456789abcdefghijklmnopqrstuvwxyz",
	} {
		got, changed := redactSensitiveText(input)
		if changed || got != input {
			t.Errorf("redactSensitiveText(%q) = %q, changed=%t; want unchanged", input, got, changed)
		}
	}
}

func TestRedactSensitiveTextPreservesDiscordWebhookWithoutToken(t *testing.T) {
	const input = "request failed for https://discord.com/api/webhooks/123456789012345678"
	got, changed := redactSensitiveText(input)
	if changed || got != input {
		t.Fatalf("redactSensitiveText(%q) = %q, changed=%t; want unchanged", input, got, changed)
	}
}

func TestRedactSensitiveTextPreservesKubeconfigCertificateData(t *testing.T) {
	const input = "client-certificate-data: public-certificate"
	got, changed := redactSensitiveText(input)
	if changed || got != input {
		t.Fatalf("redactSensitiveText(%q) = %q, changed=%t; want unchanged", input, got, changed)
	}
}

func TestRedactSensitiveTextPreservesMalformedCookieJarRow(t *testing.T) {
	const input = ".example.test\tTRUE\t/\tTRUE\tnot-an-expiry\tJSESSIONID\tpublic-diagnostic"
	got, changed := redactSensitiveText(input)
	if changed || got != input {
		t.Fatalf("redactSensitiveText(%q) = %q, changed=%t; want unchanged", input, got, changed)
	}
}

func TestRedactSensitiveTextPreservesBenignYAMLAliasMappingKey(t *testing.T) {
	input := "key: &s status\n*s: failed"
	got, changed := redactSensitiveText(input)
	if changed || got != input {
		t.Fatalf("redactSensitiveText(%q) = %q, changed=%t; want unchanged", input, got, changed)
	}
}

func TestRedactSensitiveTextPreservesEmptyYAMLCredentialMappingBeforeComment(t *testing.T) {
	input := "password:\n# context\nstatus: failed"
	got, changed := redactSensitiveText(input)
	if changed || got != input {
		t.Fatalf("redactSensitiveText(%q) = %q, changed=%t; want unchanged", input, got, changed)
	}
}

func TestRedactSensitiveTextRedactsValueForTrueSecretMarkerLiterals(t *testing.T) {
	for _, literal := range []string{"1", "t", "T", "true", "True", "TRUE"} {
		input := "asc web xcode-cloud env-vars set --value credential --secret=" + literal
		want := "asc web xcode-cloud env-vars set --value [REDACTED] --secret=" + literal
		got, changed := redactSensitiveText(input)
		if !changed || got != want {
			t.Errorf("redactSensitiveText(%q) = %q, changed=%t; want %q", input, got, changed, want)
		}
	}
}

func TestRedactSensitiveTextScopesBooleanSecretProtectionToEnvVarSetCommand(t *testing.T) {
	input := "asc web xcode-cloud env-vars set --value public-value --secret=false; " +
		"asc webhooks create --url https://example.test/hook --secret=true"
	want := "asc web xcode-cloud env-vars set --value public-value --secret=false; " +
		"asc webhooks create --url https://example.test/hook --secret=[REDACTED]"

	got, changed := redactSensitiveText(input)
	if !changed || got != want {
		t.Fatalf("redactSensitiveText(%q) = %q, changed=%t; want %q", input, got, changed, want)
	}
}

func TestRedactSensitiveTextPreservesCurlCookieFilenames(t *testing.T) {
	for _, input := range []string{
		`curl --cookie ./cookies.txt https://example.test`,
		`curl --cookie=./cookies.txt https://example.test`,
		`curl -b "$TMPDIR/cookies.jar" https://example.test`,
		`curl -b./cookies.jar https://example.test`,
		`cookie = "./cookies.txt"`,
	} {
		got, changed := redactSensitiveText(input)
		if changed || got != input {
			t.Fatalf("redactSensitiveText(%q) = %q, changed=%t; want unchanged", input, got, changed)
		}
	}
}

func TestRedactSensitiveTextPreservesCurlUseASCII(t *testing.T) {
	input := `curl -B 'https://example.test/?name=value'`
	got, changed := redactSensitiveText(input)
	if changed || got != input {
		t.Fatalf("redactSensitiveText(%q) = %q, changed=%t; want unchanged", input, got, changed)
	}
}

func TestRedactSensitiveTextPreservesNonCurlShortUArguments(t *testing.T) {
	for _, input := range []string{
		`git diff -u HEAD~1:README.md HEAD:README.md`,
		`diff -u before:file after:file`,
	} {
		got, changed := redactSensitiveText(input)
		if changed || got != input {
			t.Fatalf("redactSensitiveText(%q) = %q, changed=%t; want unchanged", input, got, changed)
		}
	}
}

func TestRedactSensitiveTextScopesCurlCertificatePasswordsToCurl(t *testing.T) {
	for _, input := range []string{
		`grep -E "host:port" logfile`,
		`grep --cert client.p12:public-pattern logfile`,
		`echo curl --cert client.p12:public-example`,
	} {
		got, changed := redactSensitiveText(input)
		if changed || got != input {
			t.Fatalf("redactSensitiveText(%q) = %q, changed=%t; want unchanged", input, got, changed)
		}
	}
}

func TestRedactSensitiveTextRedactsCombinedCurlCredentials(t *testing.T) {
	input := `curl --user alice:opaque-user --cert client.p12:opaque-cert https://example.test`
	want := `curl --user [REDACTED] --cert client.p12:[REDACTED] https://example.test`

	got, changed := redactSensitiveText(input)
	if !changed || got != want {
		t.Fatalf("redactSensitiveText(%q) = %q, changed=%t; want %q", input, got, changed, want)
	}
}

func TestRedactSensitiveTextRedactsRedisCLIPassword(t *testing.T) {
	for _, test := range []struct {
		input string
		want  string
	}{
		{input: `redis-cli -a opaque-redis-secret PING`, want: `redis-cli -a [REDACTED] PING`},
		{input: `redis-cli -a=opaque-redis-secret PING`, want: `redis-cli -a=[REDACTED] PING`},
		{input: `redis-cli '-a' opaque-redis-secret PING`, want: `redis-cli '-a' [REDACTED] PING`},
		{input: `sudo -u build /usr/local/bin/redis-cli -a opaque-redis-secret PING`, want: `sudo -u build /usr/local/bin/redis-cli -a [REDACTED] PING`},
		{input: `sh -c "redis-cli -a opaque-redis-secret PING"`, want: `sh -c "redis-cli -a [REDACTED] PING"`},
	} {
		got, changed := redactSensitiveText(test.input)
		if !changed || got != test.want {
			t.Errorf("redactSensitiveText(%q) = %q, changed=%t; want %q", test.input, got, changed, test.want)
		}
	}
}

func TestRedactSensitiveTextPreservesNonCredentialRedisCLIArguments(t *testing.T) {
	for _, input := range []string{
		`redis-cli GET -akey`,
		`redis-cli -aopaque-not-an-option PING`,
		`echo redis-cli -a public-example PING`,
	} {
		got, changed := redactSensitiveText(input)
		if changed || got != input {
			t.Fatalf("redactSensitiveText(%q) = %q, changed=%t; want unchanged", input, got, changed)
		}
	}
}

func TestRedactSensitiveTextPreservesCurlReferer(t *testing.T) {
	input := `curl -e https://example.test/page https://target.test`

	got, changed := redactSensitiveText(input)
	if changed || got != input {
		t.Fatalf("redactSensitiveText(%q) = %q, changed=%t; want unchanged", input, got, changed)
	}
}

func TestRedactSensitiveTextPreservesCurlCertificateWithoutPassword(t *testing.T) {
	for _, input := range []string{
		`curl --cert client.pem https://example.test`,
		`curl --cert="client cert.pem" https://example.test`,
		`curl -Eclient.p12 https://example.test`,
		`cert = "client cert.pem"`,
		`proxy-cert: client.p12`,
		`cert = C:\client.p12`,
	} {
		got, changed := redactSensitiveText(input)
		if changed || got != input {
			t.Fatalf("redactSensitiveText(%q) = %q, changed=%t; want unchanged", input, got, changed)
		}
	}
}

func TestRedactSensitiveTextPreservesAttachedBenignCurlHeader(t *testing.T) {
	for _, input := range []string{
		`curl -HAccept:application/json https://example.test`,
		`curl -H Accept:application/json https://example.test`,
		`curl --header=Accept:application/json https://example.test`,
	} {
		got, changed := redactSensitiveText(input)
		if changed || got != input {
			t.Fatalf("redactSensitiveText(%q) = %q, changed=%t; want unchanged", input, got, changed)
		}
	}
}

func TestRedactSensitiveTextDoesNotJoinEscapedBackslashLines(t *testing.T) {
	input := `asc unrelated --value public \\
asc web xcode-cloud env-vars set --secret`

	got, changed := redactSensitiveText(input)
	if changed || got != input {
		t.Fatalf("redactSensitiveText(%q) = %q, changed=%t; want unchanged", input, got, changed)
	}
}

func TestRedactSensitiveTextPreservesURLQueryAndFragmentAtSigns(t *testing.T) {
	tests := []string{
		"https://example.test?next=a:b@evil.test",
		"https://example.test/path#value=a:b@evil.test",
	}

	for _, input := range tests {
		got, changed := redactSensitiveText(input)
		if changed || got != input {
			t.Fatalf("redactSensitiveText(%q) = %q, changed=%t; want unchanged", input, got, changed)
		}
	}
}

func TestRedactSensitiveTextPreservesDiagnosticCodeQueryParameter(t *testing.T) {
	for _, input := range []string{
		"request failed: https://example.com/error?code=404&message=not_found",
		"login failed: https://example.com/auth/error?code=404&message=not_found",
	} {
		got, changed := redactSensitiveText(input)
		if changed || got != input {
			t.Fatalf("redactSensitiveText(%q) = (%q, %t), want unchanged", input, got, changed)
		}
	}
}

func TestRedactSensitiveTextPreservesUsernameOnlySCPRemote(t *testing.T) {
	input := "asc signing sync --repo git@github.com:team/certs.git"

	got, changed := redactSensitiveText(input)
	if changed || got != input {
		t.Fatalf("redactSensitiveText(%q) = %q, changed=%t; want unchanged", input, got, changed)
	}
}

func TestRedactSensitiveTextPreservesOrdinaryCodeFields(t *testing.T) {
	input := `{"error":{"code":"ENTITY_ERROR"},"status":400}`

	got, changed := redactSensitiveText(input)
	if changed || got != input {
		t.Fatalf("redactSensitiveText(%q) = %q, changed=%t; want unchanged", input, got, changed)
	}
}

func TestRedactSensitiveTextPreservesOrdinaryAuthField(t *testing.T) {
	input := `{"auth":"public mode","status":"ready"}`

	got, changed := redactSensitiveText(input)
	if changed || got != input {
		t.Fatalf("redactSensitiveText(%q) = %q, changed=%t; want unchanged", input, got, changed)
	}
}

func TestRedactSensitiveTextPreservesUnterminatedTOMLLiteralKey(t *testing.T) {
	for _, input := range []string{"'", "'unterminated"} {
		got, changed := redactSensitiveText(input)
		if changed || got != input {
			t.Errorf("redactSensitiveText(%q) = %q, changed=%t; want unchanged", input, got, changed)
		}
	}
}

func TestRedactSensitiveTextPreservesNameValuePairOutsideCookieJar(t *testing.T) {
	for _, input := range []string{
		`{"cookies":{},"diagnostic":{"name":"failure","value":"preserve this explanation"}}`,
		"env:\n  - name: |-\n      DB_PASSWORD\n      value: diagnostic text\n    value: preserve-public-value",
	} {
		got, changed := redactSensitiveText(input)
		if changed || got != input {
			t.Fatalf("redactSensitiveText(%q) = %q, changed=%t; want unchanged", input, got, changed)
		}
	}
}

func TestRedactSensitiveTextDoesNotResolveYAMLAliasesAcrossDocuments(t *testing.T) {
	input := "credential: &credential DB_PASSWORD\n---\nenv:\n  - name: *credential\n    value: preserve-public-value"

	got, changed := redactSensitiveText(input)
	if changed || got != input {
		t.Fatalf("redactSensitiveText(%q) = %q, changed=%t; want unchanged", input, got, changed)
	}
}

func TestRedactSensitiveTextPreservesSimilarKubernetesDataKeys(t *testing.T) {
	for _, input := range []string{
		"tls.keyUsage: digital signature\n.dockerconfigjson.backup: public metadata",
		"apiVersion: v1\nkind: ConfigMap\ndata: {config: public-config}\nstringData:\n  settings: public-settings",
		"apiVersion: v1\nkind: >-\n  Con\n  figMap\ndata:\n  config: public-config",
		"apiVersion: v1\nkind: |-\n  ConfigMap\ndata:\n  config: public-config",
		`{"apiVersion":"v1","kind":"ConfigMap","data":{"config":"public-config"},"stringData":{"settings":"public-settings"}}`,
		"data:\n  config: public-config\n# ---\nkind: ConfigMap",
		"items:\n- data:\n    config: public-config\n- kind: Secret",
		`message: "{kind: Secret, data: {config: public-diagnostic}}"`,
		"secretRef:\n  kind: Secret\nstatus:\n  data:\n    count: 42",
		"data:\n  count: 42\n... # end document\nkind: Secret",
		"? kind\n: ConfigMap\n? data\n:\n  config: public-config",
		`"k\u0069nd": ConfigMap
"d\u0061ta":
  config: public-config`,
		"<<: &defaults\n  kind: ConfigMap\ndata:\n  config: public-config",
		"defaults: &defaults {kind: ConfigMap}\n<<: [*defaults]\ndata:\n  config: public-config",
		"kind: ConfigMap\n<<: &defaults\n  kind: Secret\ndata:\n  config: public-config",
		"data:\n  config: public-config\n<<: &defaults\n  kind: Secret\nkind: ConfigMap",
	} {
		got, changed := redactSensitiveText(input)
		if changed || got != input {
			t.Fatalf("redactSensitiveText(%q) = %q, changed=%t; want unchanged", input, got, changed)
		}
	}
}

func TestRedactSensitiveTextPreservesCredentialCommandWordsInProse(t *testing.T) {
	input := "ads auth token: token request failed: dial tcp: connection refused"

	got, changed := redactSensitiveText(input)
	if changed || got != input {
		t.Fatalf("redactSensitiveText(%q) = %q, changed=%t; want unchanged", input, got, changed)
	}
}

func TestRedactSensitiveTextPreservesOrdinaryDottedTOMLKey(t *testing.T) {
	input := `metadata.value = "public context"`

	got, changed := redactSensitiveText(input)
	if changed || got != input {
		t.Fatalf("redactSensitiveText(%q) = %q, changed=%t; want unchanged", input, got, changed)
	}
}

func TestRedactSensitiveTextPreservesPasswordFileCommentsAndNonNumericPorts(t *testing.T) {
	for _, input := range []string{
		"#db.example.com:5432:app:alice:documentation",
		"db.example.com:postgres:app:alice:diagnostic context",
	} {
		got, changed := redactSensitiveText(input)
		if changed || got != input {
			t.Errorf("redactSensitiveText(%q) = %q, changed=%t; want unchanged", input, got, changed)
		}
	}
}

func TestRedactSensitiveTextRedactsDeeplyEscapedJSONCredential(t *testing.T) {
	const secret = "opaque-deeply-escaped-secret"
	encoded := `{"password":"` + secret + `","status":"failed"}`
	for range 6 {
		encodedJSON, err := json.Marshal(encoded)
		if err != nil {
			t.Fatalf("encode JSON layer: %v", err)
		}
		encoded = string(encodedJSON)
	}

	got, changed := redactSensitiveText(encoded)
	if !changed {
		t.Fatalf("redactSensitiveText() did not report a change for deeply escaped JSON")
	}
	if strings.Contains(got, secret) {
		t.Fatalf("redactSensitiveText() leaked deeply escaped credential: %q", got)
	}
	if !strings.Contains(got, redactionMarker) || !strings.Contains(got, "status") {
		t.Fatalf("redactSensitiveText() = %q, want redaction marker and surrounding context", got)
	}
}

func TestRedactSensitiveTextHandlesLargeNestedBenignJSON(t *testing.T) {
	var input strings.Builder
	input.WriteString(`{"root":`)
	for index := 0; index < 6000; index++ {
		input.WriteString(`{"level":`)
	}
	input.WriteString(`"public"`)
	for index := 0; index < 6000; index++ {
		input.WriteByte('}')
	}

	start := time.Now()
	got, changed := redactSensitiveText(input.String())
	if changed || got != input.String() {
		t.Fatalf("redactSensitiveText changed benign nested JSON: changed=%t", changed)
	}
	t.Logf("redactSensitiveText handled %d-byte benign nested JSON in %s", input.Len(), time.Since(start))
}

func TestRedactSensitiveTextRedactsDeeplyEscapedKubernetesSecretJSON(t *testing.T) {
	const secret = "opaque-deep-kubernetes-secret"
	encoded := `{"kind":"Secret","data":{"config":"` + secret + `"}}`
	for range 3 {
		encodedJSON, err := json.Marshal(encoded)
		if err != nil {
			t.Fatalf("encode JSON layer: %v", err)
		}
		encoded = string(encodedJSON)
	}

	got, changed := redactSensitiveText(encoded)
	if !changed || strings.Contains(got, secret) {
		t.Fatalf("redactSensitiveText() leaked deeply escaped Kubernetes Secret: changed=%t output=%q", changed, got)
	}
}

func TestRedactSensitiveTextRedactsDeeplyEscapedContextualJSONCredentials(t *testing.T) {
	tests := []struct {
		name   string
		secret string
		raw    string
	}{
		{
			name:   "cookie jar",
			secret: "opaque-deep-cookie-secret",
			raw:    `{"cookies":[{"name":"session","value":"opaque-deep-cookie-secret"}],"diagnostic":{"value":"preserve contextual diagnostics"},"status":"failed"}`,
		},
		{
			name:   "registry authentication map",
			secret: "opaque-deep-registry-secret",
			raw:    `{"auths":{"registry.example":{"auth":"opaque-deep-registry-secret"}},"diagnostic":{"value":"preserve contextual diagnostics"},"status":"failed"}`,
		},
		{
			name:   "upload request headers",
			secret: "opaque-deep-header-secret",
			raw:    `{"requestHeaders":[{"name":"Authorization","value":"opaque-deep-header-secret"}],"diagnostic":{"value":"preserve contextual diagnostics"},"status":"failed"}`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			encoded := test.raw
			for range 3 {
				encodedJSON, err := json.Marshal(encoded)
				if err != nil {
					t.Fatalf("encode JSON layer: %v", err)
				}
				encoded = string(encodedJSON)
			}

			got, changed := redactSensitiveText(encoded)
			if !changed {
				t.Fatalf("redactSensitiveText() did not report a change for deeply escaped %s", test.name)
			}
			if strings.Contains(got, test.secret) {
				t.Fatalf("redactSensitiveText() leaked deeply escaped %s: %q", test.name, got)
			}
			if !strings.Contains(got, redactionMarker) || !strings.Contains(got, "status") || !strings.Contains(got, "preserve contextual diagnostics") {
				t.Fatalf("redactSensitiveText() = %q, want redaction marker and surrounding context", got)
			}
		})
	}
}

func TestSnitchDryRunRedactsURLUserinfoCredentials(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "")

	const userinfo = "user:p%40ss%2Fword"
	const usernameOnly = "access-token"
	stdout, stderr, err := runSnitchCommand(
		t, "9.9.9",
		"--dry-run",
		"--actual", "requests to https://"+userinfo+"@example.test/private/path and sftp://"+usernameOnly+"@files.example.test/upload failed",
		"userinfo redaction probe",
	)
	if err != nil {
		t.Fatalf("run snitch: %v", err)
	}

	for _, secret := range []string{userinfo, usernameOnly} {
		if strings.Contains(stderr, secret) {
			t.Fatalf("stderr leaked URL userinfo credentials: %q", stderr)
		}
		if strings.Contains(stdout, secret) {
			t.Fatalf("stdout leaked URL userinfo credentials: %q", stdout)
		}
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want dry-run diagnostics on stderr only", stdout)
	}
	if !strings.Contains(stderr, "https://[REDACTED]@example.test/private/path") {
		t.Fatalf("stderr = %q, want the URL without userinfo credentials", stderr)
	}
	if !strings.Contains(stderr, "sftp://[REDACTED]@files.example.test/upload") {
		t.Fatalf("stderr = %q, want the username-only URL without credentials", stderr)
	}
	if !strings.Contains(stderr, "sensitive values were redacted") {
		t.Fatalf("stderr = %q, want a generic redaction notice", stderr)
	}
}

func TestSnitchDryRunRedactsRegistryBase64AuthCredential(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "")

	const secret = "b3BhcXVlLXNlY3JldA=="
	stdout, stderr, err := runSnitchCommand(
		t, "9.9.9",
		"--dry-run",
		"--actual", `//registry.npmjs.org/:_auth=`+secret,
		"registry credential redaction probe",
	)
	if err != nil {
		t.Fatalf("run snitch: %v", err)
	}
	if strings.Contains(stderr, secret) || strings.Contains(stdout, secret) {
		t.Fatalf("dry run leaked registry credential: stdout=%q stderr=%q", stdout, stderr)
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want dry-run diagnostics on stderr only", stdout)
	}
	if want := `//registry.npmjs.org/:_auth=[REDACTED]`; !strings.Contains(stderr, want) {
		t.Fatalf("stderr = %q, want redacted assignment %q", stderr, want)
	}
}

func TestSnitchDryRunRedactsKubernetesSecretData(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "")

	const privateKey = "b3BhcXVlLXByaXZhdGUta2V5"
	const registryConfig = "eyJhdXRocyI6eyJyZWdpc3RyeS5leGFtcGxlIjp7ImF1dGgiOiJvcGFxdWUifX19"
	actual := "apiVersion: v1\nkind: Secret\ndata:\n  tls.key: " + privateKey + "\n  .dockerconfigjson: " + registryConfig + "\nstatus: failed"
	stdout, stderr, err := runSnitchCommand(
		t, "9.9.9",
		"--dry-run",
		"--actual", actual,
		"Kubernetes Secret redaction probe",
	)
	if err != nil {
		t.Fatalf("run snitch: %v", err)
	}
	for _, secret := range []string{privateKey, registryConfig} {
		if strings.Contains(stderr, secret) || strings.Contains(stdout, secret) {
			t.Fatalf("dry run leaked Kubernetes Secret data: stdout=%q stderr=%q", stdout, stderr)
		}
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want dry-run diagnostics on stderr only", stdout)
	}
	for _, want := range []string{"tls.key: [REDACTED]", ".dockerconfigjson: [REDACTED]"} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("stderr = %q, want redacted Secret data %q", stderr, want)
		}
	}
	if !strings.Contains(stderr, "sensitive values were redacted") {
		t.Fatalf("stderr = %q, want a generic redaction notice", stderr)
	}
}

func TestSnitchDryRunRedactsCurlMultipartCredentialAndPreservesDiagnosticProse(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "")

	const secret = "opaque multipart secret tail"
	const diagnostic = "ads auth token: token request failed: dial tcp: connection refused"
	stdout, stderr, err := runSnitchCommand(
		t, "9.9.9",
		"--dry-run",
		"--actual", `curl --form-string 'password=`+secret+`' https://example.test`,
		"--repro", diagnostic,
		"curl multipart redaction probe",
	)
	if err != nil {
		t.Fatalf("run snitch: %v", err)
	}
	if strings.Contains(stderr, secret) || strings.Contains(stdout, secret) {
		t.Fatalf("dry run leaked multipart credential: stdout=%q stderr=%q", stdout, stderr)
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want dry-run diagnostics on stderr only", stdout)
	}
	if want := `curl --form-string 'password=[REDACTED]' https://example.test`; !strings.Contains(stderr, want) {
		t.Fatalf("stderr = %q, want redacted multipart form %q", stderr, want)
	}
	if !strings.Contains(stderr, diagnostic) {
		t.Fatalf("stderr = %q, want diagnostic prose preserved", stderr)
	}
}

func TestSnitchDryRunRedactsDeeplyEscapedJSONCredential(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "")

	const secret = "opaque-deeply-escaped-dry-run-secret"
	encoded := `{"password":"` + secret + `","status":"failed"}`
	for range 6 {
		encodedJSON, err := json.Marshal(encoded)
		if err != nil {
			t.Fatalf("encode JSON layer: %v", err)
		}
		encoded = string(encodedJSON)
	}
	stdout, stderr, err := runSnitchCommand(
		t, "9.9.9",
		"--dry-run",
		"--actual", encoded,
		"deeply escaped JSON redaction probe",
	)
	if err != nil {
		t.Fatalf("run snitch: %v", err)
	}
	if strings.Contains(stderr, secret) || strings.Contains(stdout, secret) {
		t.Fatalf("dry run leaked deeply escaped JSON credential: stdout=%q stderr=%q", stdout, stderr)
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want dry-run diagnostics on stderr only", stdout)
	}
	if !strings.Contains(stderr, redactionMarker) || !strings.Contains(stderr, "status") {
		t.Fatalf("stderr = %q, want redaction marker and surrounding context", stderr)
	}
}

func TestSnitchDryRunRedactsDeeplyEscapedContextualJSONCredentials(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "")

	secrets := []string{
		"opaque-deep-cookie-dry-run-secret",
		"opaque-deep-registry-dry-run-secret",
		"opaque-deep-header-dry-run-secret",
	}
	rawValues := []string{
		`{"cookies":[{"name":"session","value":"` + secrets[0] + `"}],"status":"failed"}`,
		`{"auths":{"registry.example":{"auth":"` + secrets[1] + `"}},"status":"failed"}`,
		`{"requestHeaders":[{"name":"Authorization","value":"` + secrets[2] + `"}],"status":"failed"}`,
	}
	for index, value := range rawValues {
		for range 3 {
			encodedJSON, err := json.Marshal(value)
			if err != nil {
				t.Fatalf("encode JSON layer: %v", err)
			}
			value = string(encodedJSON)
		}
		rawValues[index] = value
	}
	stdout, stderr, err := runSnitchCommand(
		t, "9.9.9",
		"--dry-run",
		"--actual", strings.Join(rawValues, "\n"),
		"deeply escaped contextual JSON redaction probe",
	)
	if err != nil {
		t.Fatalf("run snitch: %v", err)
	}
	for _, secret := range secrets {
		if strings.Contains(stderr, secret) || strings.Contains(stdout, secret) {
			t.Fatalf("dry run leaked contextual JSON credential %q: stdout=%q stderr=%q", secret, stdout, stderr)
		}
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want dry-run diagnostics on stderr only", stdout)
	}
	if count := strings.Count(stderr, redactionMarker); count < len(secrets) {
		t.Fatalf("stderr = %q, want at least %d redaction markers, got %d", stderr, len(secrets), count)
	}
	if !strings.Contains(stderr, "status") {
		t.Fatalf("stderr = %q, want surrounding context preserved", stderr)
	}
}

func TestSnitchDryRunRedactsStructuredCredentialAssignments(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "")

	secrets := []string{
		"opaque-here-string-tail",
		"opaque-array-secret",
		"opaque-hashtable-secret",
		"opaque-toml-parent-secret",
	}
	repro := "$password = @'\nopaque-head\n" + secrets[0] + "\n'@\n" +
		`$password = @("` + secrets[1] + `", "second")` + "\n" +
		`$client_secret = @{ primary = "` + secrets[2] + `"; status = "failed" }`
	stdout, stderr, err := runSnitchCommand(
		t, "9.9.9",
		"--dry-run",
		"--repro", repro,
		"--actual", `password.value = "`+secrets[3]+`"`+"\nstatus = \"failed\"",
		"structured credential assignment redaction probe",
	)
	if err != nil {
		t.Fatalf("run snitch: %v", err)
	}
	for _, secret := range secrets {
		if strings.Contains(stderr, secret) || strings.Contains(stdout, secret) {
			t.Fatalf("dry run leaked structured credential %q: stdout=%q stderr=%q", secret, stdout, stderr)
		}
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want dry-run diagnostics on stderr only", stdout)
	}
	for _, want := range []string{
		"$password = [REDACTED]",
		"$client_secret = [REDACTED]",
		`password.value = [REDACTED]`,
		`status = "failed"`,
	} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("stderr = %q, want preserved context %q", stderr, want)
		}
	}
}

func TestSnitchDryRunRedactsScopedPowerShellAndPasswordFileCredentials(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "")

	secrets := []string{
		"opaque-scoped-here-string-tail",
		"opaque-scoped-collection-secret",
		`opaque\:password\\tail`,
	}
	repro := "$env:PASSWORD = @'\nopaque-head\n" + secrets[0] + "\n'@\n" +
		`${global:client_secret} = @("` + secrets[1] + `", "second")`
	stdout, stderr, err := runSnitchCommand(
		t, "9.9.9",
		"--dry-run",
		"--repro", repro,
		"--actual", `db\:primary.example.com:5432:app:alice:`+secrets[2]+"\nstatus: failed",
		"scoped shell and password file redaction probe",
	)
	if err != nil {
		t.Fatalf("run snitch: %v", err)
	}
	for _, secret := range secrets {
		if strings.Contains(stderr, secret) || strings.Contains(stdout, secret) {
			t.Fatalf("dry run leaked credential %q: stdout=%q stderr=%q", secret, stdout, stderr)
		}
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want dry-run diagnostics on stderr only", stdout)
	}
	for _, want := range []string{
		"$env:PASSWORD = [REDACTED]",
		"${global:client_secret} = [REDACTED]",
		`db\:primary.example.com:5432:app:alice:[REDACTED]`,
		"status: failed",
	} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("stderr = %q, want preserved context %q", stderr, want)
		}
	}
}

func TestSnitchDryRunRedactsRegistryConfigurationAuthCredential(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "")

	const secret = "YWxpY2U6b3BhcXVlLXNlY3JldA=="
	stdout, stderr, err := runSnitchCommand(
		t, "9.9.9",
		"--dry-run",
		"--actual", `{"auths":{"registry.example":{"auth":"`+secret+`"}},"status":"failed"}`,
		"registry configuration redaction probe",
	)
	if err != nil {
		t.Fatalf("run snitch: %v", err)
	}
	if strings.Contains(stderr, secret) || strings.Contains(stdout, secret) {
		t.Fatalf("dry run leaked registry configuration credential: stdout=%q stderr=%q", stdout, stderr)
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want dry-run diagnostics on stderr only", stdout)
	}
	if want := `{"auths":{"registry.example":{"auth":"[REDACTED]"}},"status":"failed"}`; !strings.Contains(stderr, want) {
		t.Fatalf("stderr = %q, want redacted configuration %q", stderr, want)
	}
}

func TestSnitchDryRunPreservesLoneSingleQuote(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "")

	stdout, stderr, err := runSnitchCommand(t, "9.9.9", "--dry-run", "'")
	if err != nil {
		t.Fatalf("run snitch: %v", err)
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want dry-run diagnostics on stderr only", stdout)
	}
	if !strings.Contains(stderr, "Title: '") {
		t.Fatalf("stderr = %q, want the original report title", stderr)
	}
}

func TestRedactSensitiveTextBoundsOversizedInputBeforeScanning(t *testing.T) {
	const (
		maxFieldBytes       = 64 * 1024
		multilineTailSecret = "retained-multiline-tail-secret"
		discardedTailSecret = "discarded-oversized-tail-secret"
	)
	input := "diagnostic 🙂 line\nPASSWORD=\"opaque-head\n" + multilineTailSecret + "\n" +
		strings.Repeat("captured line\n", 5000) + "\"\npassword=" + discardedTailSecret

	got, changed := redactSensitiveText(input)
	if !changed {
		t.Fatal("redactSensitiveText() changed = false, want oversized input reported as changed")
	}
	if len(got) > maxFieldBytes {
		t.Fatalf("redactSensitiveText() returned %d bytes, want at most %d", len(got), maxFieldBytes)
	}
	if !utf8.ValidString(got) {
		t.Fatal("redactSensitiveText() returned invalid UTF-8")
	}
	for _, secret := range []string{multilineTailSecret, discardedTailSecret} {
		if strings.Contains(got, secret) {
			t.Fatalf("redactSensitiveText() retained oversized credential %q: %q", secret, got)
		}
	}
	if got != "[REDACTED: oversized report field omitted]" {
		t.Fatalf("redactSensitiveText() = %q, want the fail-closed omission marker", got)
	}

	again, changedAgain := redactSensitiveText(got)
	if changedAgain || again != got {
		t.Fatalf("second redactSensitiveText() = %q, changed=%t; want idempotent %q", again, changedAgain, got)
	}
}

func TestRedactLogEntryBoundsEveryStringField(t *testing.T) {
	const maxFieldBytes = 64 * 1024
	oversized := strings.Repeat("x", maxFieldBytes+1)
	entry := LogEntry{
		Description: oversized,
		Repro:       oversized,
		Expected:    oversized,
		Actual:      oversized,
		Labels:      []string{oversized},
		Severity:    oversized,
		ASCVersion:  oversized,
		OS:          oversized,
	}

	got, changed := redactLogEntry(entry)
	if !changed {
		t.Fatal("redactLogEntry() changed = false, want oversized fields reported as changed")
	}
	fields := append([]string{
		got.Description,
		got.Repro,
		got.Expected,
		got.Actual,
		got.Severity,
		got.ASCVersion,
		got.OS,
	}, got.Labels...)
	for index, field := range fields {
		if len(field) > maxFieldBytes {
			t.Errorf("field %d returned %d bytes, want at most %d", index, len(field), maxFieldBytes)
		}
		if !strings.Contains(field, "oversized report field omitted") {
			t.Errorf("field %d = %q, want an explicit truncation marker", index, field)
		}
	}
}

func TestSnitchDryRunBoundsOversizedReportFieldBeforePreview(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "")

	const (
		multilineTailSecret = "retained-dry-run-multiline-secret"
		discardedTailSecret = "discarded-dry-run-tail-secret"
	)
	actual := "PASSWORD=\"opaque-head\n" + multilineTailSecret + "\n" +
		strings.Repeat("captured diagnostic line\n", 3500) + "\"\npassword=" + discardedTailSecret
	stdout, stderr, err := runSnitchCommand(
		t, "9.9.9",
		"--dry-run",
		"--actual", actual,
		"oversized diagnostic redaction probe",
	)
	if err != nil {
		t.Fatalf("run snitch: %v", err)
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want dry-run diagnostics on stderr only", stdout)
	}
	for _, secret := range []string{multilineTailSecret, discardedTailSecret} {
		if strings.Contains(stderr, secret) {
			t.Fatalf("dry run leaked oversized credential %q: %q", secret, stderr)
		}
	}
	if !strings.Contains(stderr, "oversized report field omitted") {
		t.Fatalf("stderr = %q, want an explicit truncation marker", stderr)
	}
	if len(stderr) > 70*1024 {
		t.Fatalf("stderr returned %d bytes, want bounded dry-run preview", len(stderr))
	}
}

func TestSnitchDryRunRedactsKubeconfigClientKeyData(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "")

	const secret = "b3BhcXVlLXByaXZhdGUta2V5"
	stdout, stderr, err := runSnitchCommand(
		t, "9.9.9",
		"--dry-run",
		"--actual", "client-key-data: "+secret+"\nclient-certificate-data: public-certificate",
		"kubeconfig credential redaction probe",
	)
	if err != nil {
		t.Fatalf("run snitch: %v", err)
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want dry-run diagnostics on stderr only", stdout)
	}
	if strings.Contains(stderr, secret) {
		t.Fatalf("dry run leaked kubeconfig client key data: %q", stderr)
	}
	for _, want := range []string{
		"client-key-data: [REDACTED]",
		"client-certificate-data: public-certificate",
	} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("stderr = %q, want preserved context %q", stderr, want)
		}
	}
}

func TestSnitchDryRunRedactsEveryReportField(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "")

	secrets := []string{
		"eyJhbGciOiJFUzI1NiJ9.fake.signature",
		"0123456789abcdef0123456789abcdef",
		"private-key-payload",
		"client-secret-value",
		"Passwordtest1",
		"remaining-secret",
		"prefixed-secret",
		"flag-passphrase-secret",
		"assignment-passphrase-secret",
	}
	privateKey := "-----BEGIN PRIVATE KEY-----\n" + secrets[2] + "\n-----END PRIVATE KEY-----"

	stdout, stderr, err := runSnitchCommand(
		t, "9.9.9",
		"--dry-run",
		"--repro", `curl "https://uploads.example.test/file?X-Amz-Signature=`+secrets[1]+`&part=2"`+"\n"+`asc web sandbox create --password "`+secrets[4]+`"`+"\n"+`asc deploy --password "pa\"`+secrets[5]+`"`+"\n"+`asc signing sync --passphrase "`+secrets[7]+`"`,
		"--expected", "load this key\n"+privateKey+"\nthen retry",
		"--actual", `client_secret="`+secrets[3]+`" MY_CLIENT_SECRET="`+secrets[6]+`" passphrase = "`+secrets[8]+`"`,
		"Authorization: Bearer "+secrets[0]+" failed",
	)
	if err != nil {
		t.Fatalf("run snitch: %v", err)
	}

	for _, secret := range secrets {
		if strings.Contains(stderr, secret) {
			t.Fatalf("stderr leaked %q: %q", secret, stderr)
		}
		if strings.Contains(stdout, secret) {
			t.Fatalf("stdout leaked %q: %q", secret, stdout)
		}
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want dry-run diagnostics on stderr only", stdout)
	}
	for _, want := range []string{
		"Authorization: [REDACTED] failed",
		"X-Amz-Signature=[REDACTED]&part=2",
		"asc web sandbox create --password [REDACTED]",
		"asc deploy --password [REDACTED]",
		"asc signing sync --passphrase [REDACTED]",
		"load this key\n[REDACTED PRIVATE KEY]\nthen retry",
		"client_secret=[REDACTED] MY_CLIENT_SECRET=[REDACTED] passphrase = [REDACTED]",
	} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("stderr = %q, want preserved context %q", stderr, want)
		}
	}
	if got := strings.Count(stderr, "sensitive values were redacted"); got != 1 {
		t.Fatalf("stderr = %q, want exactly one redaction notice, got %d", stderr, got)
	}
}

func TestSnitchDryRunRedactsMalformedAndCompoundCLISecrets(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "")

	secrets := []string{
		"app-specific-password",
		"xcode-cloud-value",
		"unterminated secret value",
		"Password1,remaining-secret",
		"credential-value",
		"single-dash-password",
		"ANSI-C quoted password",
		"ANSI-C assigned password",
		"escaped-flag-tail",
		"escaped-assignment-tail",
		"reordered-cloud-value",
		"--DoubleDashPassword",
		"curl-password-tail",
		"curl-oauth-bearer-tail",
		"curl-attached-user-tail",
		"curl-attached-proxy-tail",
		"curl-private-key-passphrase-tail",
		"curl-tls-password-tail",
		"curl-proxy-tls-password-tail",
		"continued-flag-tail",
		"continued-assignment-tail",
	}
	repro := strings.Join([]string{
		`asc review details-create --demo-account-password "` + secrets[0] + `" --notes ready`,
		`asc web xcode-cloud env-vars set --name MY_SECRET --value ` + secrets[1] + ` --secret --apple-id 123456789`,
		`asc web xcode-cloud env-vars set --value ` + secrets[10] + ` --name MY_SECRET --secret --apple-id 123456789`,
		`asc web sandbox create -password "` + secrets[5] + `" -territory USA`,
		`asc deploy --password $'` + secrets[6] + `' --verbose`,
		`asc deploy --password prefix\ ` + secrets[8] + ` --verbose`,
		`asc web sandbox create --password ` + secrets[11] + ` --territory USA`,
		`curl -u alice:` + secrets[12] + ` https://example.test`,
		`curl --oauth2-bearer ` + secrets[13] + ` https://example.test`,
		`curl -ualice:` + secrets[14] + ` https://example.test`,
		`curl -Ualice:` + secrets[15] + ` https://example.test`,
		`curl --pass ` + secrets[16] + ` --key client.pem https://example.test`,
		`curl --tlspassword ` + secrets[17] + ` https://example.test`,
		`curl --proxy-tlspassword ` + secrets[18] + ` https://example.test`,
		"asc deploy --password prefix\\\n" + secrets[19] + " --verbose",
		`asc deploy --password "` + secrets[2],
	}, "\n")
	actual := "PASSWORD=" + secrets[3] + "\n" +
		`PASSWORD=$'` + secrets[7] + "'\n" +
		`PASSWORD=prefix\ ` + secrets[9] + "\n" +
		"PASSWORD=prefix\\\n" + secrets[20] + " asc builds list\n" +
		`Authorization: Signature keyId="my-key",algorithm="rsa-sha256",signature="` + secrets[4] + `"`

	stdout, stderr, err := runSnitchCommand(
		t, "9.9.9",
		"--dry-run",
		"--repro", repro,
		"--actual", actual,
		"compound redaction probe",
	)
	if err != nil {
		t.Fatalf("run snitch: %v", err)
	}

	for _, secret := range secrets {
		if strings.Contains(stderr, secret) {
			t.Fatalf("stderr leaked %q: %q", secret, stderr)
		}
		if strings.Contains(stdout, secret) {
			t.Fatalf("stdout leaked %q: %q", secret, stdout)
		}
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want dry-run diagnostics on stderr only", stdout)
	}
	for _, want := range []string{
		"--demo-account-password [REDACTED] --notes ready",
		"--name MY_SECRET --value [REDACTED] --secret --apple-id 123456789",
		"--value [REDACTED] --name MY_SECRET --secret --apple-id 123456789",
		"--password [REDACTED]",
		"-password [REDACTED] -territory USA",
		"--password [REDACTED] --territory USA",
		"--password [REDACTED] --verbose",
		"curl -u [REDACTED] https://example.test",
		"curl --oauth2-bearer [REDACTED] https://example.test",
		"curl -u[REDACTED] https://example.test",
		"curl -U[REDACTED] https://example.test",
		"curl --pass [REDACTED] --key client.pem https://example.test",
		"curl --tlspassword [REDACTED] https://example.test",
		"curl --proxy-tlspassword [REDACTED] https://example.test",
		"asc deploy --password [REDACTED] --verbose",
		"PASSWORD=[REDACTED]",
		"Authorization: [REDACTED]",
	} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("stderr = %q, want preserved context %q", stderr, want)
		}
	}
}

func TestSnitchDryRunRedactsCompoundHeaderAndPlistCredentials(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "")

	const headerSecret = "compound-header-secret"
	const plistSecret = "plist-credential-value"
	const xmlElementSecret = "xml-element-credential-value"
	const yamlSecret = "yaml-indentless-secret"
	const tomlSecret = "toml-dotted-secret"
	const netrcSecret = "netrc-password-secret"
	stdout, stderr, err := runSnitchCommand(
		t, "9.9.9",
		"--dry-run",
		"--repro", "curl -H 'Authorization: Bearer '"+headerSecret+" https://example.test",
		"--actual", `<plist><dict><key>pass&#x77;ord</key><!-- context --><data>`+plistSecret+`</data><key>status</key><string>failed</string></dict></plist>`+"\n"+
			`<settings><server><password>`+xmlElementSecret+`</password><status>failed</status></server></settings>`+"\n"+
			`credentials."pass\u0077ord" = "`+tomlSecret+`"`+"\n"+
			"machine api.example.test login alice password "+netrcSecret,
		"--expected", "password:\n- "+yamlSecret+"\nstatus: failed",
		"compound header and property list credential redaction probe",
	)
	if err != nil {
		t.Fatalf("run snitch: %v", err)
	}

	for _, secret := range []string{headerSecret, plistSecret, xmlElementSecret, yamlSecret, tomlSecret, netrcSecret} {
		if strings.Contains(stderr, secret) {
			t.Fatalf("stderr leaked %q: %q", secret, stderr)
		}
		if strings.Contains(stdout, secret) {
			t.Fatalf("stdout leaked %q: %q", secret, stdout)
		}
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want dry-run diagnostics on stderr only", stdout)
	}
	for _, want := range []string{
		`curl -H "Authorization: [REDACTED]" https://example.test`,
		`<key>pass&#x77;ord</key><!-- context --><data>[REDACTED]</data><key>status</key><string>failed</string>`,
		`<settings><server><password>[REDACTED]</password><status>failed</status></server></settings>`,
		"password: [REDACTED]\nstatus: failed",
		`credentials."pass\u0077ord" = [REDACTED]`,
		"machine api.example.test login alice password [REDACTED]",
	} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("stderr = %q, want preserved context %q", stderr, want)
		}
	}
}

func TestSnitchDryRunPreservesBearerProtocolProse(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "")

	const description = "Bearer OAuth2 authentication fails"
	const actual = "Bearer HTTP2 authentication fails; Bearer RFC6750 flow fails"
	stdout, stderr, err := runSnitchCommand(
		t, "9.9.9",
		"--dry-run",
		"--actual", actual,
		description,
	)
	if err != nil {
		t.Fatalf("run snitch: %v", err)
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want dry-run diagnostics on stderr only", stdout)
	}
	for _, want := range []string{description, actual} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("stderr = %q, want benign protocol prose %q", stderr, want)
		}
	}
	if strings.Contains(stderr, "Bearer [REDACTED]") {
		t.Fatalf("stderr over-redacted benign protocol prose: %q", stderr)
	}
}

func TestSnitchDryRunRedactsStandaloneWebhookAndTaggedExplicitYAML(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "")

	const webhookSecret = "standalone-webhook-secret"
	const yamlSecret = "tagged-explicit-yaml-secret"
	const flowSecret = "quoted-first-flow-secret"
	stdout, stderr, err := runSnitchCommand(
		t, "9.9.9",
		"--dry-run",
		"--repro", "failed webhook https://hooks.slack-gov.com/services/T012/B034/"+webhookSecret+" after retry",
		"--actual", "? !!str password\n: "+yamlSecret+"\nstatus: failed",
		"--expected", "password: [\n  \""+flowSecret+"\",\n  \"second-secret\"\n]\nstatus: failed",
		"standalone webhook and tagged explicit YAML credential redaction probe",
	)
	if err != nil {
		t.Fatalf("run snitch: %v", err)
	}

	for _, secret := range []string{webhookSecret, yamlSecret, flowSecret, "second-secret"} {
		if strings.Contains(stderr, secret) {
			t.Fatalf("stderr leaked %q: %q", secret, stderr)
		}
		if strings.Contains(stdout, secret) {
			t.Fatalf("stdout leaked %q: %q", secret, stdout)
		}
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want dry-run diagnostics on stderr only", stdout)
	}
	for _, want := range []string{
		"failed webhook https://hooks.slack-gov.com/services/[REDACTED] after retry",
		"? !!str password\n: [REDACTED]\nstatus: failed",
		"password: [REDACTED]\nstatus: failed",
	} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("stderr = %q, want preserved context %q", stderr, want)
		}
	}
}

func TestSnitchDryRunRedactsContinuedFlagAndCurlConfigCredentials(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "")

	secrets := []string{"continued-flag-secret", "curl-user-secret", "curl-bearer-secret", "curl-cookie-secret", "curl-colon-user-secret", "curl-config-cert-secret", "curl-config-proxy-cert-secret"}
	stdout, stderr, err := runSnitchCommand(
		t, "9.9.9",
		"--dry-run",
		"--repro", "asc deploy --password \\\n  "+secrets[0]+" --verbose",
		"--actual", "user = \"alice:"+secrets[1]+"\"\noauth2-bearer \""+secrets[2]+"\"\ncookie: \"myacinfo="+secrets[3]+"\"\nproxy-user: \"alice:"+secrets[4]+"\"\ncert \"client.p12:"+secrets[5]+"\"\nproxy-cert = client.p12:"+secrets[6]+"\nurl = \"https://example.test\"",
		"continued flag and curl config credential redaction probe",
	)
	if err != nil {
		t.Fatalf("run snitch: %v", err)
	}

	for _, secret := range secrets {
		if strings.Contains(stderr, secret) {
			t.Fatalf("stderr leaked %q: %q", secret, stderr)
		}
		if strings.Contains(stdout, secret) {
			t.Fatalf("stdout leaked %q: %q", secret, stdout)
		}
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want dry-run diagnostics on stderr only", stdout)
	}
	for _, want := range []string{
		"asc deploy --password \\\n  [REDACTED] --verbose",
		"user = [REDACTED]",
		"oauth2-bearer [REDACTED]",
		"cookie: [REDACTED]",
		"proxy-user: [REDACTED]",
		`cert "client.p12:[REDACTED]"`,
		"proxy-cert = client.p12:[REDACTED]",
		"url = \"https://example.test\"",
	} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("stderr = %q, want preserved context %q", stderr, want)
		}
	}
}

func TestSnitchDryRunRedactsPowerShellAndPlistStringCredentials(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "")

	const powerShellSecret = "powershell-secret-suffix"
	const quotedPowerShellSecret = "powershell-quoted-secret-suffix"
	const hereStringSecret = "powershell-here-string-secret"
	const commandPromptSecret = "command-prompt-set-secret"
	const unquotedCommandPromptSecret = "unquoted-command-prompt-set-secret"
	const secureStringSecret = "powershell-secure-string-secret"
	const plistSecret = "plist-string-credential-value"
	stdout, stderr, err := runSnitchCommand(
		t, "9.9.9",
		"--dry-run",
		"--repro", "asc signing sync --password opaque` "+powerShellSecret+" --verbose\n"+
			"asc signing sync --password \"opaque`\""+quotedPowerShellSecret+"\" --verbose\n"+
			"asc signing sync --password @'\nopaque-head\n"+hereStringSecret+"\n'@ --verbose\n"+
			`$password = ConvertTo-SecureString "opaque `+secureStringSecret+`" -AsPlainText -Force`+"\n"+
			`set "PASSWORD=opaque `+commandPromptSecret+`" & echo done`+"\n"+
			`set PASSWORD=opaque `+unquotedCommandPromptSecret+` value & echo done`,
		"--actual", `<plist><dict><key>password</key><string>`+plistSecret+`</string><key>status</key><string>failed</string></dict></plist>`,
		"shell and property list string credential redaction probe",
	)
	if err != nil {
		t.Fatalf("run snitch: %v", err)
	}

	for _, secret := range []string{powerShellSecret, quotedPowerShellSecret, hereStringSecret, secureStringSecret, commandPromptSecret, unquotedCommandPromptSecret, plistSecret} {
		if strings.Contains(stderr, secret) {
			t.Fatalf("stderr leaked %q: %q", secret, stderr)
		}
		if strings.Contains(stdout, secret) {
			t.Fatalf("stdout leaked %q: %q", secret, stdout)
		}
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want dry-run diagnostics on stderr only", stdout)
	}
	for _, want := range []string{
		"asc signing sync --password [REDACTED] --verbose",
		`$password = [REDACTED] -AsPlainText -Force`,
		`set "PASSWORD=[REDACTED]" & echo done`,
		`set PASSWORD=[REDACTED] & echo done`,
		`<key>password</key><string>[REDACTED]</string><key>status</key><string>failed</string>`,
	} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("stderr = %q, want preserved context %q", stderr, want)
		}
	}
}

func TestSnitchDryRunRedactsCustomCurlHeadersAndSecureStringSwitches(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "")

	secrets := []string{
		"opaque-curl-api-key-secret",
		"opaque-curl-auth-token-secret",
		"opaque-secure-string-explicit-secret",
		"opaque-secure-string-positional-secret",
	}
	repro := `curl -H "X-API-Key: ` + secrets[0] + `" https://example.test` + "\n" +
		`curl --header 'X-Auth-Token: ` + secrets[1] + `' https://example.test` + "\n" +
		`$password = ConvertTo-SecureString -AsPlainText -String "` + secrets[2] + `" -Force` + "\n" +
		`$client_secret = ConvertTo-SecureString -Force "` + secrets[3] + `" -AsPlainText`
	stdout, stderr, err := runSnitchCommand(
		t, "9.9.9",
		"--dry-run",
		"--repro", repro,
		"custom header and SecureString switch redaction probe",
	)
	if err != nil {
		t.Fatalf("run snitch: %v", err)
	}
	for _, secret := range secrets {
		if strings.Contains(stderr, secret) || strings.Contains(stdout, secret) {
			t.Fatalf("dry run leaked %q: stdout=%q stderr=%q", secret, stdout, stderr)
		}
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want dry-run diagnostics on stderr only", stdout)
	}
	for _, want := range []string{
		`curl -H "X-API-Key: [REDACTED]" https://example.test`,
		`curl --header "X-Auth-Token: [REDACTED]" https://example.test`,
		`$password = [REDACTED] -Force`,
		`$client_secret = [REDACTED] -AsPlainText`,
	} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("stderr = %q, want preserved context %q", stderr, want)
		}
	}
}

func TestSnitchDryRunRedactsPrefixedFlagsAndSessionEnvironmentCredentials(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "")

	secrets := []string{
		"opaque-database-password",
		"opaque-service-token",
		"opaque-automation-session",
	}
	repro := `tool --database-password ` + secrets[0] + ` --github-token=` + secrets[1] + ` --password-file ./password.txt` + "\n" +
		`AUTOMATION_SESSION=` + secrets[2]
	stdout, stderr, err := runSnitchCommand(
		t, "9.9.9",
		"--dry-run",
		"--repro", repro,
		"prefixed flag and session environment redaction probe",
	)
	if err != nil {
		t.Fatalf("run snitch: %v", err)
	}
	for _, secret := range secrets {
		if strings.Contains(stderr, secret) || strings.Contains(stdout, secret) {
			t.Fatalf("dry run leaked %q: stdout=%q stderr=%q", secret, stdout, stderr)
		}
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want dry-run diagnostics on stderr only", stdout)
	}
	for _, want := range []string{
		`tool --database-password [REDACTED] --github-token=[REDACTED] --password-file ./password.txt`,
		`AUTOMATION_SESSION=[REDACTED]`,
	} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("stderr = %q, want preserved context %q", stderr, want)
		}
	}
}

func TestSnitchDryRunRedactsXMLCredentialsAndPreservesDiagnosticContext(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "")

	secrets := []string{
		"opaque-xml-attribute-secret",
		"opaque-truncated-xml-secret",
		"opaque-truncated-plist-secret",
	}
	repro := `<configuration><add key="ClearTextPassword" value="` + secrets[0] + `" /><add key="status" value="failed" /></configuration>`
	actual := "Authorization: request failed because proxy unavailable\ntool --no-token output.txt\n<settings><password>" + secrets[1]
	expected := `<plist><dict><key>password</key><string>` + secrets[2]
	stdout, stderr, err := runSnitchCommand(
		t, "9.9.9",
		"--dry-run",
		"--repro", repro,
		"--actual", actual,
		"--expected", expected,
		"XML credential redaction and diagnostic preservation probe",
	)
	if err != nil {
		t.Fatalf("run snitch: %v", err)
	}
	for _, secret := range secrets {
		if strings.Contains(stderr, secret) || strings.Contains(stdout, secret) {
			t.Fatalf("dry run leaked %q: stdout=%q stderr=%q", secret, stdout, stderr)
		}
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want dry-run diagnostics on stderr only", stdout)
	}
	for _, want := range []string{
		`<add key="ClearTextPassword" value="[REDACTED]" />`,
		`<settings><password>[REDACTED]`,
		`<plist><dict><key>password</key><string>[REDACTED]`,
		"Authorization: request failed because proxy unavailable",
		"tool --no-token output.txt",
	} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("stderr = %q, want preserved context %q", stderr, want)
		}
	}
}

func TestSnitchDryRunRedactsPrefixedCredentialHeaders(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "")

	const cookieSecret = "prefixed-cookie-secret"
	const continuationSecret = "prefixed-continuation-secret"
	stdout, stderr, err := runSnitchCommand(
		t, "9.9.9",
		"--dry-run",
		"--actual", "request failed with Cookie: myacinfo="+cookieSecret+" after retry\n"+
			"request failed with scnt: "+continuationSecret+" after retry",
		"prefixed credential header redaction probe",
	)
	if err != nil {
		t.Fatalf("run snitch: %v", err)
	}

	for _, secret := range []string{cookieSecret, continuationSecret} {
		if strings.Contains(stderr, secret) {
			t.Fatalf("stderr leaked %q: %q", secret, stderr)
		}
		if strings.Contains(stdout, secret) {
			t.Fatalf("stdout leaked %q: %q", secret, stdout)
		}
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want dry-run diagnostics on stderr only", stdout)
	}
	for _, want := range []string{
		"request failed with Cookie: [REDACTED] after retry",
		"request failed with scnt: [REDACTED] after retry",
	} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("stderr = %q, want preserved context %q", stderr, want)
		}
	}
}

func TestSnitchDryRunRedactsCookieJarAndFoldedHeaderValues(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "")

	secrets := []string{
		"cookie-jar-secret",
		"folded-authorization-head-secret",
		"folded-authorization-tail-secret",
		"folded-cookie-tail-secret",
		"folded-session-head-secret",
		"folded-session-tail-secret",
	}
	stdout, stderr, err := runSnitchCommand(
		t, "9.9.9",
		"--dry-run",
		"--actual", ".appstoreconnect.apple.com\tTRUE\t/\tTRUE\t2147483647\tmyacinfo\t"+secrets[0]+"\n"+
			"Authorization: Bearer "+secrets[1]+"\r\n "+secrets[2]+"\r\nstatus: failed\r\n"+
			"< Cookie: dslang=US-EN;\r\n myacinfo="+secrets[3]+"\r\ncookie status: failed\r\n"+
			"request failed with scnt: "+secrets[4]+"\r\n "+secrets[5]+"\r\nsession status: failed",
		"cookie jar and folded header redaction probe",
	)
	if err != nil {
		t.Fatalf("run snitch: %v", err)
	}

	for _, secret := range secrets {
		if strings.Contains(stderr, secret) {
			t.Fatalf("stderr leaked %q: %q", secret, stderr)
		}
		if strings.Contains(stdout, secret) {
			t.Fatalf("stdout leaked %q: %q", secret, stdout)
		}
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want dry-run diagnostics on stderr only", stdout)
	}
	for _, want := range []string{
		".appstoreconnect.apple.com TRUE / TRUE 2147483647 myacinfo [REDACTED]",
		"Authorization: [REDACTED]",
		"status: failed",
		"< Cookie: [REDACTED]",
		"cookie status: failed",
		"request failed with scnt: [REDACTED]",
		"session status: failed",
	} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("stderr = %q, want preserved context %q", stderr, want)
		}
	}
}

func TestSnitchDryRunRedactsSecretAnswerAndEveryCookieJarValue(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "")

	secrets := []string{
		"quoted-recovery-answer",
		"equals-recovery-answer",
		"java-session-cookie",
		"csrf-cookie-value",
		"locale-cookie-value",
	}
	stdout, stderr, err := runSnitchCommand(
		t, "9.9.9",
		"--dry-run",
		"--repro", `tool --secret-answer "`+secrets[0]+`" --verbose`+"\n"+
			`tool --secret-answer=`+secrets[1]+` --verbose`,
		"--actual", ".example.test\tTRUE\t/\tTRUE\t2147483647\tJSESSIONID\t"+secrets[2]+"\n"+
			"#HttpOnly_.example.test\tFALSE\t/\tTRUE\t0\tcsrftoken\t"+secrets[3]+"\n"+
			".example.test\tFALSE\t/\tFALSE\t0\tlocale\t"+secrets[4],
		"secret answer and cookie jar redaction probe",
	)
	if err != nil {
		t.Fatalf("run snitch: %v", err)
	}

	for _, secret := range secrets {
		if strings.Contains(stderr, secret) {
			t.Fatalf("stderr leaked %q: %q", secret, stderr)
		}
		if strings.Contains(stdout, secret) {
			t.Fatalf("stdout leaked %q: %q", secret, stdout)
		}
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want dry-run diagnostics on stderr only", stdout)
	}
	for _, want := range []string{
		"--secret-answer [REDACTED] --verbose",
		"--secret-answer=[REDACTED] --verbose",
		".example.test TRUE / TRUE 2147483647 JSESSIONID [REDACTED]",
		"#HttpOnly_.example.test FALSE / TRUE 0 csrftoken [REDACTED]",
		".example.test FALSE / FALSE 0 locale [REDACTED]",
	} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("stderr = %q, want preserved context %q", stderr, want)
		}
	}
}

func TestSnitchDryRunRedactsEnvironmentDumpAndKnownServiceTokens(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "")

	secrets := []string{
		"opaque secret tail",
		"xoxb-123456789012-123456789012-abcdefghijklmnopqrstuvwx",
		"npm_abcdefghijklmnopqrstuvwxyz0123456789",
		"glpat-abcdefghijklmnopqrstuvwxyz",
	}
	stdout, stderr, err := runSnitchCommand(
		t, "9.9.9",
		"--dry-run",
		"--actual", "PASSWORD="+secrets[0]+"\ncredential "+secrets[1]+"\ncredential "+secrets[2]+"\ncredential "+secrets[3]+"\nstatus: failed",
		"environment dump and service token redaction probe",
	)
	if err != nil {
		t.Fatalf("run snitch: %v", err)
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want dry-run diagnostics on stderr only", stdout)
	}
	for _, secret := range secrets {
		if strings.Contains(stderr, secret) {
			t.Fatalf("dry run leaked %q: %q", secret, stderr)
		}
	}
	for _, want := range []string{
		"PASSWORD=[REDACTED]",
		"credential [REDACTED]",
		"status: failed",
	} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("stderr = %q, want preserved context %q", stderr, want)
		}
	}
}

func TestSnitchDryRunRedactsCommandPromptContinuedCredentials(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "")

	const secret = "command-prompt-secret-suffix"
	stdout, stderr, err := runSnitchCommand(
		t, "9.9.9",
		"--dry-run",
		"--repro", "asc signing sync --password opaque^\r\n"+secret+" --verbose",
		"Command Prompt credential redaction probe",
	)
	if err != nil {
		t.Fatalf("run snitch: %v", err)
	}

	if strings.Contains(stderr, secret) {
		t.Fatalf("stderr leaked %q: %q", secret, stderr)
	}
	if strings.Contains(stdout, secret) {
		t.Fatalf("stdout leaked %q: %q", secret, stdout)
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want dry-run diagnostics on stderr only", stdout)
	}
	if want := "asc signing sync --password [REDACTED] --verbose"; !strings.Contains(stderr, want) {
		t.Fatalf("stderr = %q, want preserved context %q", stderr, want)
	}
}

func TestSnitchDryRunRedactsTruncatedPrivateKeyAndProxyCredentials(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "")

	const keyMaterial = "truncated-key-material"
	const proxyPassword = "proxy-password-tail"
	stdout, stderr, err := runSnitchCommand(
		t, "9.9.9",
		"--dry-run",
		"--repro", "curl --proxy-user alice:"+proxyPassword+" https://example.test",
		"--actual", "failed to load\n-----BEGIN PRIVATE KEY-----\n"+keyMaterial,
		"malformed credential redaction probe",
	)
	if err != nil {
		t.Fatalf("run snitch: %v", err)
	}

	for _, secret := range []string{keyMaterial, proxyPassword} {
		if strings.Contains(stderr, secret) {
			t.Fatalf("stderr leaked %q: %q", secret, stderr)
		}
		if strings.Contains(stdout, secret) {
			t.Fatalf("stdout leaked %q: %q", secret, stdout)
		}
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want dry-run diagnostics on stderr only", stdout)
	}
	for _, want := range []string{
		"curl --proxy-user [REDACTED] https://example.test",
		"failed to load\n[REDACTED PRIVATE KEY]",
	} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("stderr = %q, want preserved context %q", stderr, want)
		}
	}
}

func TestSnitchDryRunRedactsExplicitMarkersAuthorizationAndStructuredKeys(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "")

	secrets := []string{"marked-value", "authorization-credential", "structured-secret", "continued-secret", "pretty-structured-secret", "camel-structured-secret", "escaped-structured-secret", "fish-assignment-secret"}
	stdout, stderr, err := runSnitchCommand(
		t, "9.9.9",
		"--dry-run",
		"--repro", "asc web xcode-cloud env-vars set --value "+secrets[0]+" --secret=true\n"+
			"asc web xcode-cloud env-vars set --value "+secrets[3]+" \\\n  --secret\n"+
			"set --global --export ASC_SIGNING_SYNC_PASSWORD "+secrets[7]+"; asc signing sync pull",
		"--actual", `Authorization: ApiKey `+secrets[1]+"\n"+`{"MY_CLIENT_SECRET":"`+secrets[2]+`"}`+"\n"+
			"{\n  \"client_secret\":\n    \""+secrets[4]+"\"\n}\n"+
			`{"demoAccountPassword":"`+secrets[5]+`"}`+"\n"+
			`response {\"client_secret\":\"`+secrets[6]+`\"}`,
		"explicit credential redaction probe",
	)
	if err != nil {
		t.Fatalf("run snitch: %v", err)
	}

	for _, secret := range secrets {
		if strings.Contains(stderr, secret) {
			t.Fatalf("stderr leaked %q: %q", secret, stderr)
		}
		if strings.Contains(stdout, secret) {
			t.Fatalf("stdout leaked %q: %q", secret, stdout)
		}
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want dry-run diagnostics on stderr only", stdout)
	}
	for _, want := range []string{
		"--value [REDACTED] --secret=true",
		"--value [REDACTED] \\\n  --secret",
		"set --global --export ASC_SIGNING_SYNC_PASSWORD [REDACTED]; asc signing sync pull",
		"Authorization: [REDACTED]",
		`{"MY_CLIENT_SECRET":"[REDACTED]"}`,
		"\"client_secret\":\n    \"[REDACTED]\"",
		`{"demoAccountPassword":"[REDACTED]"}`,
		`response {\"client_secret\":\"[REDACTED]\"}`,
	} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("stderr = %q, want preserved context %q", stderr, want)
		}
	}
}

func TestSnitchDryRunRedactsMultilineSecretMarkedValueAndPreservesFalseMarker(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "")

	const secret = "literal-newline-secret"
	const publicValue = "public-value"
	repro := "asc web xcode-cloud env-vars set --value \"credential-head\n" + secret + "\" --secret=T\n" +
		"asc web xcode-cloud env-vars set --value " + publicValue + " --secret=0"
	stdout, stderr, err := runSnitchCommand(
		t, "9.9.9",
		"--dry-run",
		"--repro", repro,
		"multiline secret marker redaction probe",
	)
	if err != nil {
		t.Fatalf("run snitch: %v", err)
	}

	if strings.Contains(stderr, secret) {
		t.Fatalf("stderr leaked %q: %q", secret, stderr)
	}
	if strings.Contains(stdout, secret) {
		t.Fatalf("stdout leaked %q: %q", secret, stdout)
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want dry-run diagnostics on stderr only", stdout)
	}
	for _, want := range []string{
		"--value [REDACTED] --secret=T",
		"--value " + publicValue + " --secret=0",
	} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("stderr = %q, want preserved context %q", stderr, want)
		}
	}
}

func TestSnitchDryRunRedactsShellTerminatedMarkersProxyCertificatesAndNestedSubstitutions(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "")

	secrets := []string{"terminated-marker-secret", "proxy-cert-secret", "nested-substitution-secret", "fish-substitution-secret", "quoted-executable-secret", "quoted-flag-secret", "nested-fish-secret", "yaml-continuation-secret", "multiline-backtick-secret"}
	repro := "asc web xcode-cloud env-vars set --value " + secrets[0] + " --secret=true&& echo done\n" +
		"curl --proxy-cert client.p12:" + secrets[1] + " https://example.test\n" +
		"asc deploy --password $(printf %s $(printf prefix) " + secrets[2] + ") --verbose\n" +
		"asc signing sync --password (printf " + secrets[3] + ") --verbose\n" +
		"asc webhooks create --url https://example.test/hook --secret=true\n" +
		`"asc" web xcode-cloud env-vars set --value ` + secrets[4] + " --secret=true\n" +
		`asc signing sync push "--password" ` + secrets[5] + "\n" +
		"asc signing sync --password (printf %s (printf prefix) " + secrets[6] + ") --verbose\n" +
		"password: opaque-first\n  " + secrets[7] + "\nstatus: failed\n" +
		"asc signing sync push --password `printf '%s' 'opaque-head\n" + secrets[8] + "'` --verbose"
	stdout, stderr, err := runSnitchCommand(
		t, "9.9.9",
		"--dry-run",
		"--repro", repro,
		"shell credential redaction probe",
	)
	if err != nil {
		t.Fatalf("run snitch: %v", err)
	}

	for _, secret := range secrets {
		if strings.Contains(stderr, secret) {
			t.Fatalf("stderr leaked %q: %q", secret, stderr)
		}
		if strings.Contains(stdout, secret) {
			t.Fatalf("stdout leaked %q: %q", secret, stdout)
		}
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want dry-run diagnostics on stderr only", stdout)
	}
	for _, want := range []string{
		"--value [REDACTED] --secret=true&& echo done",
		"curl --proxy-cert client.p12:[REDACTED] https://example.test",
		"asc deploy --password [REDACTED] --verbose",
		"asc signing sync --password [REDACTED] --verbose",
		"asc webhooks create --url https://example.test/hook --secret=[REDACTED]",
		`"asc" web xcode-cloud env-vars set --value [REDACTED] --secret=true`,
		`asc signing sync push "--password" [REDACTED]`,
		`asc signing sync --password [REDACTED] --verbose`,
		"password: [REDACTED]\nstatus: failed",
		`asc signing sync push --password [REDACTED] --verbose`,
	} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("stderr = %q, want preserved context %q", stderr, want)
		}
	}
}

func TestSnitchDryRunRedactsTOMLAndEscapedJSONCredentials(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "")

	secrets := []string{"toml-multiline-secret", "json-unicode-key-secret", "escaped-json-unicode-key-secret", "toml-inline-sensitive", "toml-array-sensitive", "toml-key-sensitive", "yaml-key-sensitive", "double-escaped-json-secret"}
	repro := "password = \"\"\"opaque-head\n" + secrets[0] + "\"\"\"\nstatus = \"failed\"\n" +
		`{"pass\u0077ord":"` + secrets[1] + `","status":"failed"}` + "\n" +
		`trace {\"pass\\u0077ord\":\"` + secrets[2] + `\",\"status\":\"failed\"}` + "\n" +
		`password = { value = "` + secrets[3] + `", nested = { label = "]" } }` + "\n" +
		"password = [\n  \"" + secrets[4] + "\",\n  { value = \"nested\" },\n]\n" +
		`"pass\u0077ord" = "` + secrets[5] + `"` + "\n" +
		`"pass\u0077ord": |` + "\n  " + secrets[6] + "\nstatus: failed\n" +
		`trace {\\\"pass\\\\u0077ord\\\":\\\"` + secrets[7] + `\\\",\\\"status\\\":\\\"failed\\\"}`
	stdout, stderr, err := runSnitchCommand(
		t, "9.9.9",
		"--dry-run",
		"--repro", repro,
		"structured credential redaction probe",
	)
	if err != nil {
		t.Fatalf("run snitch: %v", err)
	}

	for _, secret := range secrets {
		if strings.Contains(stderr, secret) {
			t.Fatalf("stderr leaked %q: %q", secret, stderr)
		}
		if strings.Contains(stdout, secret) {
			t.Fatalf("stdout leaked %q: %q", secret, stdout)
		}
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want dry-run diagnostics on stderr only", stdout)
	}
	for _, want := range []string{
		"password = [REDACTED]\nstatus = \"failed\"",
		`{"pass\u0077ord":"[REDACTED]","status":"failed"}`,
		`trace {\"pass\\u0077ord\":\"[REDACTED]\",\"status\":\"failed\"}`,
		`trace {\\\"pass\\\\u0077ord\\\":\\\"[REDACTED]\\\",\\\"status\\\":\\\"failed\\\"}`,
		`password = [REDACTED]`,
		`"pass\u0077ord" = [REDACTED]`,
		`"pass\u0077ord": [REDACTED]`,
	} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("stderr = %q, want preserved context %q", stderr, want)
		}
	}
}

func TestSnitchDryRunRedactsQuotedTOMLAndSplitCurlCredentials(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "")

	secrets := []string{"quoted-toml-scalar-secret", "quoted-toml-multiline-secret", "split-cert-secret", "split-user-secret"}
	repro := `"asc_private_key_b64" = "` + secrets[0] + `"` + "\n" +
		"'password' = '''opaque-head\n" + secrets[1] + "'''\n" +
		`curl --cert "client cert.p12":` + secrets[2] + " https://example.test\n" +
		`curl --user 'alice':` + secrets[3] + " https://example.test"
	stdout, stderr, err := runSnitchCommand(
		t, "9.9.9",
		"--dry-run",
		"--repro", repro,
		"quoted credential redaction probe",
	)
	if err != nil {
		t.Fatalf("run snitch: %v", err)
	}

	for _, secret := range secrets {
		if strings.Contains(stderr, secret) {
			t.Fatalf("stderr leaked %q: %q", secret, stderr)
		}
		if strings.Contains(stdout, secret) {
			t.Fatalf("stdout leaked %q: %q", secret, stdout)
		}
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want dry-run diagnostics on stderr only", stdout)
	}
	for _, want := range []string{
		`"asc_private_key_b64" = [REDACTED]`,
		`'password' = [REDACTED]`,
		`curl --cert "client cert.p12":[REDACTED] https://example.test`,
		`curl --user [REDACTED] https://example.test`,
	} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("stderr = %q, want preserved context %q", stderr, want)
		}
	}
}

func TestSnitchDryRunRedactsHeterogeneousCredentialArrayAndPreservesCurlUseASCII(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "")

	secrets := []string{"heterogeneous-first-secret", "heterogeneous-second-secret"}
	const publicURL = "https://example.test/?name=value"
	repro := `{"password":["` + secrets[0] + `",{"value":"` + secrets[1] + `"}],"status":"failed"}` + "\n" +
		`curl -B '` + publicURL + `'`
	stdout, stderr, err := runSnitchCommand(
		t, "9.9.9",
		"--dry-run",
		"--repro", repro,
		"structured array redaction probe",
	)
	if err != nil {
		t.Fatalf("run snitch: %v", err)
	}

	for _, secret := range secrets {
		if strings.Contains(stderr, secret) {
			t.Fatalf("stderr leaked %q: %q", secret, stderr)
		}
		if strings.Contains(stdout, secret) {
			t.Fatalf("stdout leaked %q: %q", secret, stdout)
		}
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want dry-run diagnostics on stderr only", stdout)
	}
	for _, want := range []string{
		`{"password":["[REDACTED]"],"status":"failed"}`,
		`curl -B '` + publicURL + `'`,
	} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("stderr = %q, want preserved context %q", stderr, want)
		}
	}
}

func TestSnitchDryRunRedactsCookieHeadersAndScopedPrivateKeys(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "")

	secrets := []string{
		"web-session-secret",
		"response-session-secret",
		"c3RvcmVraXQtcHJpdmF0ZS1rZXk=",
		"YWRzLXByaXZhdGUta2V5",
	}
	stdout, stderr, err := runSnitchCommand(
		t, "9.9.9",
		"--dry-run",
		"--repro", `curl -H "Cookie: myacinfo=`+secrets[0]+`; dslang=US-EN" https://example.test`,
		"--actual", "< Set-Cookie: myacinfo="+secrets[1]+"; Path=/; Secure\n"+
			"ASC_STOREKIT_PRIVATE_KEY_B64="+secrets[2]+"\n"+
			"ASC_ADS_PRIVATE_KEY_B64="+secrets[3],
		"session credential redaction probe",
	)
	if err != nil {
		t.Fatalf("run snitch: %v", err)
	}

	for _, secret := range secrets {
		if strings.Contains(stderr, secret) {
			t.Fatalf("stderr leaked %q: %q", secret, stderr)
		}
		if strings.Contains(stdout, secret) {
			t.Fatalf("stdout leaked %q: %q", secret, stdout)
		}
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want dry-run diagnostics on stderr only", stdout)
	}
	for _, want := range []string{
		`curl -H "Cookie: [REDACTED]" https://example.test`,
		"< Set-Cookie: [REDACTED]",
		"ASC_STOREKIT_PRIVATE_KEY_B64=[REDACTED]",
		"ASC_ADS_PRIVATE_KEY_B64=[REDACTED]",
	} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("stderr = %q, want preserved context %q", stderr, want)
		}
	}
}

func TestSnitchDryRunRedactsStructuredHeadersAndYAMLSecretBlocks(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "")

	secrets := []string{
		"structured-authorization-secret",
		"structured-cookie-secret",
		"yaml-literal-secret",
		"yaml-folded-secret",
	}
	stdout, stderr, err := runSnitchCommand(
		t, "9.9.9",
		"--dry-run",
		"--repro", "{\"Authorization\":\"Basic "+secrets[0]+"\",\"Cookie\":\"myacinfo="+secrets[1]+"\",\"status\":\"failed\"}",
		"--actual", `{"Authorization":"Basic `+secrets[0]+`","Cookie":"myacinfo=`+secrets[1]+`","status":"failed"}`+"\n"+
			"client_secret: |\n  "+secrets[2]+"\nstatus: failed\n"+
			"private_key_b64: >-\n  "+secrets[3]+"\nnext: preserved",
		"structured credential redaction probe",
	)
	if err != nil {
		t.Fatalf("run snitch: %v", err)
	}

	for _, secret := range secrets {
		if strings.Contains(stderr, secret) {
			t.Fatalf("stderr leaked %q: %q", secret, stderr)
		}
		if strings.Contains(stdout, secret) {
			t.Fatalf("stdout leaked %q: %q", secret, stdout)
		}
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want dry-run diagnostics on stderr only", stdout)
	}
	for _, want := range []string{
		"{\"Authorization\":\"[REDACTED]\",\"Cookie\":\"[REDACTED]\",\"status\":\"failed\"}",
		`{"Authorization":"[REDACTED]","Cookie":"[REDACTED]","status":"failed"}`,
		"client_secret: [REDACTED]\nstatus: failed",
		"private_key_b64: [REDACTED]\nnext: preserved",
	} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("stderr = %q, want preserved context %q", stderr, want)
		}
	}
}

func TestSnitchDryRunRedactsYAMLSingleQuotedScalarWithDoubledQuote(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "")

	const secret = "super''sensitive"
	stdout, stderr, err := runSnitchCommand(
		t, "9.9.9",
		"--dry-run",
		"--repro", "password: '"+secret+"'\nstatus: failed",
		"YAML quoted credential redaction probe",
	)
	if err != nil {
		t.Fatalf("run snitch: %v", err)
	}

	if strings.Contains(stderr, secret) {
		t.Fatalf("stderr leaked %q: %q", secret, stderr)
	}
	if strings.Contains(stdout, secret) {
		t.Fatalf("stdout leaked %q: %q", secret, stdout)
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want dry-run diagnostics on stderr only", stdout)
	}
	if want := "password: [REDACTED]\nstatus: failed"; !strings.Contains(stderr, want) {
		t.Fatalf("stderr = %q, want preserved context %q", stderr, want)
	}
}

func TestSnitchDryRunRedactsUploadOperationHeaderValues(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "")

	secrets := []string{"authorization-upload-secret", "custom-upload-secret"}
	stdout, stderr, err := runSnitchCommand(
		t, "9.9.9",
		"--dry-run",
		"--actual", `{"uploadOperations":[{"method":"PUT","requestHeaders":[{"name":"Authorization","value":"`+secrets[0]+`"},{"name":"x-upload-token","value":"`+secrets[1]+`"}],"length":12}],"diagnostic":{"name":"failure","value":"preserve this explanation"}}`,
		"upload operation credential redaction probe",
	)
	if err != nil {
		t.Fatalf("run snitch: %v", err)
	}

	for _, secret := range secrets {
		if strings.Contains(stderr, secret) {
			t.Fatalf("stderr leaked %q: %q", secret, stderr)
		}
		if strings.Contains(stdout, secret) {
			t.Fatalf("stdout leaked %q: %q", secret, stdout)
		}
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want dry-run diagnostics on stderr only", stdout)
	}
	want := `{"uploadOperations":[{"method":"PUT","requestHeaders":[{"name":"Authorization","value":"[REDACTED]"},{"name":"x-upload-token","value":"[REDACTED]"}],"length":12}],"diagnostic":{"name":"failure","value":"preserve this explanation"}}`
	if !strings.Contains(stderr, want) {
		t.Fatalf("stderr = %q, want preserved response context %q", stderr, want)
	}
}

func TestSnitchDryRunRedactsTruncatedUploadOperationHeaderValue(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "")

	const secret = "truncated-upload-secret"
	stdout, stderr, err := runSnitchCommand(
		t, "9.9.9",
		"--dry-run",
		"--actual", `{"requestHeaders":[{"name":"Authorization","value":"`+secret,
		"truncated upload credential redaction probe",
	)
	if err != nil {
		t.Fatalf("run snitch: %v", err)
	}
	if strings.Contains(stderr, secret) {
		t.Fatalf("stderr leaked %q: %q", secret, stderr)
	}
	if strings.Contains(stdout, secret) {
		t.Fatalf("stdout leaked %q: %q", secret, stdout)
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want dry-run diagnostics on stderr only", stdout)
	}
	if want := `{"requestHeaders":[{"name":"Authorization","value":"[REDACTED]`; !strings.Contains(stderr, want) {
		t.Fatalf("stderr = %q, want redacted truncated value %q", stderr, want)
	}
}

func TestSnitchDryRunRedactsYAMLCredentialAliasMappingKey(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "")

	const aliasSecret = "yaml-alias-key-secret"
	const explicitSecret = "yaml-explicit-comment-secret"
	const implicitSecret = "yaml-implicit-comment-secret"
	stdout, stderr, err := runSnitchCommand(
		t, "9.9.9",
		"--dry-run",
		"--actual", "key: &s api_key\n*s: "+aliasSecret+"\n? password\n# context\n\n: "+explicitSecret+"\npassword:\n# context\n  "+implicitSecret+"\nstatus: failed",
		"YAML alias credential key redaction probe",
	)
	if err != nil {
		t.Fatalf("run snitch: %v", err)
	}
	for _, secret := range []string{aliasSecret, explicitSecret, implicitSecret} {
		if strings.Contains(stderr, secret) || strings.Contains(stdout, secret) {
			t.Fatalf("dry run leaked %q: stdout=%q stderr=%q", secret, stdout, stderr)
		}
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want dry-run diagnostics on stderr only", stdout)
	}
	for _, want := range []string{"key: &s api_key", "*s: [REDACTED]", "? password\n# context\n\n: [REDACTED]", "password: [REDACTED]", "status: failed"} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("stderr = %q, want preserved context %q", stderr, want)
		}
	}
}

func TestSnitchDryRunRedactsNestedYAMLAndCommandSubstitutionCredentials(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "")

	secrets := []string{"yaml-sequence-secret", "backtick-substitution-secret", "dollar-substitution-secret", "certificate-suffix-secret", "unquoted-flow-credential", "quoted-flow-credential", "yaml-block-mapping-secret"}
	stdout, stderr, err := runSnitchCommand(
		t, "9.9.9",
		"--dry-run",
		"--repro", "items:\n  - password: |\n      "+secrets[0]+"\nstatus: failed\nasc deploy --password `printf "+secrets[1]+"` --verbose\nasc deploy --password $(printf "+secrets[2]+") --verbose\ncurl --cert client.p12:'"+secrets[3]+" password' https://example.test\npassword: [first-value, "+secrets[4]+"]\n\"password\": [first-value, "+secrets[5]+"]\nresponse:\n  token:\n    type: bearer\n    value: "+secrets[6]+"\n  status: failed\nnext: preserved",
		"nested credential redaction probe",
	)
	if err != nil {
		t.Fatalf("run snitch: %v", err)
	}

	for _, secret := range secrets {
		if strings.Contains(stderr, secret) {
			t.Fatalf("stderr leaked %q: %q", secret, stderr)
		}
		if strings.Contains(stdout, secret) {
			t.Fatalf("stdout leaked %q: %q", secret, stdout)
		}
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want dry-run diagnostics on stderr only", stdout)
	}
	for _, want := range []string{
		"items:\n  - password: [REDACTED]\nstatus: failed",
		"asc deploy --password [REDACTED] --verbose",
		"curl --cert client.p12:[REDACTED] https://example.test",
		"password: [REDACTED]",
		"\"password\": [REDACTED]",
		"response:\n  token: [REDACTED]\n  status: failed\nnext: preserved",
	} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("stderr = %q, want preserved context %q", stderr, want)
		}
	}
}

func TestSnitchDryRunRedactsMultilineCurlAndYAMLCredentials(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "")

	secrets := []string{"multiline-user-secret", "multiline-cert-secret", "quoted-yaml-secret", "sequence-yaml-secret", "anchored-yaml-secret", "multiline-flow-secret", "quoted-block-secret", "property-block-secret", "explicit-yaml-secret", "alias-definition-secret", "nested-single-line-flow-secret", "quoted-first-nested-flow-secret"}
	repro := "curl --user \"alice:first\n" + secrets[0] + "\" https://example.test\n" +
		"curl --cert \"client.p12:first\n" + secrets[1] + "\" https://example.test\n" +
		"response:\n  \"password\":\n    value: " + secrets[2] + "\n  status: failed\n" +
		"items:\n  - token:\n      value: " + secrets[3] + "\n    status: failed\n" +
		"auth:\n  token: &credentials\n    value: " + secrets[4] + "\n  status: failed\n" +
		"config:\n  password: [first-value,\n    " + secrets[5] + "]\n  status: failed\n" +
		"quoted-block:\n  \"password\": |\n    \"" + secrets[6] + "\"\n  status: failed\n" +
		"property-block:\n  password: &credential |\n    " + secrets[7] + "\n  status: failed\n" +
		"explicit-key:\n  ? password\n  : " + secrets[8] + "\n  status: failed\n" +
		"alias-key:\n  shared: &credential " + secrets[9] + "\n  password: *credential\n  status: failed\n" +
		"nested-flow:\n  password: [first-secret, [" + secrets[10] + "], third-secret]\n  status: failed\n" +
		"quoted-first-flow:\n  'password': [\"first-secret\", [" + secrets[11] + "], third-secret]\n  status: failed"
	stdout, stderr, err := runSnitchCommand(
		t, "9.9.9",
		"--dry-run",
		"--repro", repro,
		"multiline credential redaction probe",
	)
	if err != nil {
		t.Fatalf("run snitch: %v", err)
	}

	for _, secret := range secrets {
		if strings.Contains(stderr, secret) {
			t.Fatalf("stderr leaked %q: %q", secret, stderr)
		}
		if strings.Contains(stdout, secret) {
			t.Fatalf("stdout leaked %q: %q", secret, stdout)
		}
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want dry-run diagnostics on stderr only", stdout)
	}
	for _, want := range []string{
		"curl --user [REDACTED] https://example.test",
		"curl --cert \"client.p12:[REDACTED]\" https://example.test",
		"response:\n  \"password\": [REDACTED]\n  status: failed",
		"items:\n  - token: [REDACTED]\n    status: failed",
		"auth:\n  token: [REDACTED]\n  status: failed",
		"config:\n  password: [REDACTED]\n  status: failed",
		"quoted-block:\n  \"password\": [REDACTED]\n  status: failed",
		"property-block:\n  password: [REDACTED]\n  status: failed",
		"explicit-key:\n  ? password\n  : [REDACTED]\n  status: failed",
		"alias-key:\n  shared: &credential [REDACTED]\n  password: [REDACTED]\n  status: failed",
		"nested-flow:\n  password: [REDACTED]\n  status: failed",
		"quoted-first-flow:\n  'password': [REDACTED]\n  status: failed",
	} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("stderr = %q, want preserved context %q", stderr, want)
		}
	}
}

func TestSnitchDryRunRedactsWebAuthenticationPayloads(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "")

	secrets := []string{
		"array-authorization-secret",
		"header-service-secret",
		"auth-service-secret",
		"response-service-secret",
		"123456",
	}
	stdout, stderr, err := runSnitchCommand(
		t, "9.9.9",
		"--dry-run",
		"--repro", `{"Authorization":["Bearer `+secrets[0]+`"],"X-Apple-Widget-Key":["`+secrets[1]+`"],"status":"failed"}`,
		"--actual", `{"authServiceKey":"`+secrets[2]+`","serviceKey":"`+secrets[3]+`","securityCode":{"code":"`+secrets[4]+`"},"mode":"sms"}`,
		"web authentication payload redaction probe",
	)
	if err != nil {
		t.Fatalf("run snitch: %v", err)
	}

	for _, secret := range secrets {
		if strings.Contains(stderr, secret) {
			t.Fatalf("stderr leaked %q: %q", secret, stderr)
		}
		if strings.Contains(stdout, secret) {
			t.Fatalf("stdout leaked %q: %q", secret, stdout)
		}
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want dry-run diagnostics on stderr only", stdout)
	}
	for _, want := range []string{
		`{"Authorization":["[REDACTED]"],"X-Apple-Widget-Key":["[REDACTED]"],"status":"failed"}`,
		`{"authServiceKey":"[REDACTED]","serviceKey":"[REDACTED]","securityCode":{"code":"[REDACTED]"},"mode":"sms"}`,
	} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("stderr = %q, want preserved context %q", stderr, want)
		}
	}
}

func TestSnitchDryRunRedactsCurlCertificatePasswords(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "")

	secrets := []string{"space-secret", "equals-secret", "attached-secret"}
	stdout, stderr, err := runSnitchCommand(
		t, "9.9.9",
		"--dry-run",
		"--repro", "curl --cert client.p12:"+secrets[0]+" --cert=client.pem:"+secrets[1]+" -Eclient.pfx:"+secrets[2]+" https://example.test",
		"certificate password redaction probe",
	)
	if err != nil {
		t.Fatalf("run snitch: %v", err)
	}

	for _, secret := range secrets {
		if strings.Contains(stderr, secret) {
			t.Fatalf("stderr leaked %q: %q", secret, stderr)
		}
		if strings.Contains(stdout, secret) {
			t.Fatalf("stdout leaked %q: %q", secret, stdout)
		}
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want dry-run diagnostics on stderr only", stdout)
	}
	want := "curl --cert client.p12:[REDACTED] --cert=client.pem:[REDACTED] -Eclient.pfx:[REDACTED] https://example.test"
	if !strings.Contains(stderr, want) {
		t.Fatalf("stderr = %q, want preserved context %q", stderr, want)
	}
}

func TestSnitchDryRunRedactsAttachedCurlCredentialHeaders(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "")

	secrets := []string{"cookie-session-secret", "widget-service-secret", "separated-cookie-secret", "long-cookie-secret"}
	stdout, stderr, err := runSnitchCommand(
		t, "9.9.9",
		"--dry-run",
		"--repro", "curl -HCookie:myacinfo="+secrets[0]+" -HX-Apple-Widget-Key:"+secrets[1]+" -H Cookie:myacinfo="+secrets[2]+" --header=Cookie:myacinfo="+secrets[3]+" -HAccept:application/json https://example.test",
		"attached credential header redaction probe",
	)
	if err != nil {
		t.Fatalf("run snitch: %v", err)
	}

	for _, secret := range secrets {
		if strings.Contains(stderr, secret) {
			t.Fatalf("stderr leaked %q: %q", secret, stderr)
		}
		if strings.Contains(stdout, secret) {
			t.Fatalf("stdout leaked %q: %q", secret, stdout)
		}
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want dry-run diagnostics on stderr only", stdout)
	}
	want := "curl -HCookie:[REDACTED] -HX-Apple-Widget-Key:[REDACTED] -H Cookie:[REDACTED] --header=Cookie:[REDACTED] -HAccept:application/json https://example.test"
	if !strings.Contains(stderr, want) {
		t.Fatalf("stderr = %q, want preserved context %q", stderr, want)
	}
}

func TestSnitchDryRunRedactsQuotedAuthorizationAndSCPRemoteCredentials(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "")

	secrets := []string{"quoted-authorization-secret", "remote-password-secret"}
	stdout, stderr, err := runSnitchCommand(
		t, "9.9.9",
		"--dry-run",
		"--repro", "curl -H 'Authorization: Bearer "+secrets[0]+"' https://example.test",
		"--actual", "asc signing sync --repo user:"+secrets[1]+"@github.com:team/certs.git",
		"quoted authorization and remote credential redaction probe",
	)
	if err != nil {
		t.Fatalf("run snitch: %v", err)
	}

	for _, secret := range secrets {
		if strings.Contains(stderr, secret) {
			t.Fatalf("stderr leaked %q: %q", secret, stderr)
		}
		if strings.Contains(stdout, secret) {
			t.Fatalf("stdout leaked %q: %q", secret, stdout)
		}
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want dry-run diagnostics on stderr only", stdout)
	}
	for _, want := range []string{
		"curl -H 'Authorization: [REDACTED]' https://example.test",
		"asc signing sync --repo [REDACTED]@github.com:team/certs.git",
	} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("stderr = %q, want preserved context %q", stderr, want)
		}
	}
}

func TestSnitchDryRunRedactsCompositeYAMLAndProxyHeaderCredentials(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "")

	secrets := []string{"yaml secret tail", "nested-object-secret", "proxy-header-secret", "webhook-secret", "truncated-object-secret"}
	stdout, stderr, err := runSnitchCommand(
		t, "9.9.9",
		"--dry-run",
		"--repro", "curl --proxy-header Cookie:myacinfo="+secrets[2]+" https://example.test\nasc notify slack --webhook https://hooks.slack.com/services/T/B/"+secrets[3]+" --message ready",
		"--expected", "password: [REDACTED]",
		"--actual", "password: "+secrets[0]+"\nresponse: {\"token\":{\"type\":\"bearer\",\"value\":\""+secrets[1]+"\"},\"status\":\"failed\"}\ntruncated: {\"token\":{\"value\":\""+secrets[4]+"\"",
		"composite credential redaction probe",
	)
	if err != nil {
		t.Fatalf("run snitch: %v", err)
	}

	for _, secret := range secrets {
		if strings.Contains(stderr, secret) {
			t.Fatalf("stderr leaked %q: %q", secret, stderr)
		}
		if strings.Contains(stdout, secret) {
			t.Fatalf("stdout leaked %q: %q", secret, stdout)
		}
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want dry-run diagnostics on stderr only", stdout)
	}
	for _, want := range []string{
		"curl --proxy-header Cookie:[REDACTED] https://example.test",
		"asc notify slack --webhook [REDACTED] --message ready",
		"password: [REDACTED]",
		`response: {"token":"[REDACTED]","status":"failed"}`,
		`truncated: {"token":"[REDACTED]"`,
	} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("stderr = %q, want preserved context %q", stderr, want)
		}
	}
}

func TestSnitchDryRunRedactsGoHeaderMapsAndContinuedCurlCredentials(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "")

	secrets := []string{"continued-user-secret", "go-header-secret", "proxy-authorization-secret", "split-user-secret"}
	stdout, stderr, err := runSnitchCommand(
		t, "9.9.9",
		"--dry-run",
		"--repro", "curl --user alice:"+secrets[0]+"\\\n-tail https://example.test\n"+
			"curl --user alice:'"+secrets[3]+" password' https://example.test/split",
		"--actual", "request headers: map[Cookie:[myacinfo="+secrets[1]+"] Content-Type:[application/json]]\n{\"Proxy-Authorization\":[\"Basic "+secrets[2]+"\"],\"status\":\"failed\"}",
		"header map and continued credential redaction probe",
	)
	if err != nil {
		t.Fatalf("run snitch: %v", err)
	}

	for _, secret := range secrets {
		if strings.Contains(stderr, secret) {
			t.Fatalf("stderr leaked %q: %q", secret, stderr)
		}
		if strings.Contains(stdout, secret) {
			t.Fatalf("stdout leaked %q: %q", secret, stdout)
		}
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want dry-run diagnostics on stderr only", stdout)
	}
	for _, want := range []string{
		"curl --user [REDACTED] https://example.test",
		"curl --user [REDACTED] https://example.test/split",
		"request headers: map[Cookie:[REDACTED] Content-Type:[application/json]]",
		`{"Proxy-Authorization":["[REDACTED]"],"status":"failed"}`,
	} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("stderr = %q, want preserved context %q", stderr, want)
		}
	}
}

func TestSnitchDryRunPreservesOperatorsAroundContinuedHeaderCredentials(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "")

	secrets := []string{"continued-header-secret", "boundary-assignment-credential", "operator-flag-credential", "fragmented-flag-credential", "fragmented-env-credential", "environment-webhook-secret", "custom-header-secret"}
	stdout, stderr, err := runSnitchCommand(
		t, "9.9.9",
		"--dry-run",
		"--repro", "curl -H \"Cookie: myacinfo="+secrets[0]+"\\\n-tail\" https://example.test\nPASSWORD="+secrets[1]+"; echo next\nasc deploy --password "+secrets[2]+" && echo done\nasc deploy --password 'adjacent-'\""+secrets[3]+"\" --verbose\nPASSWORD='adjacent-'\""+secrets[4]+"\" asc builds list\nASC_SLACK_WEBHOOK=https://hooks.slack.com/services/T/B/"+secrets[5]+" asc notify slack --message ready\nasc web xcode-cloud usage alert --webhook-header \"X-API-Key: "+secrets[6]+"\" --webhook https://example.test",
		"continued header and operator preservation probe",
	)
	if err != nil {
		t.Fatalf("run snitch: %v", err)
	}

	for _, secret := range secrets {
		if strings.Contains(stderr, secret) {
			t.Fatalf("stderr leaked %q: %q", secret, stderr)
		}
		if strings.Contains(stdout, secret) {
			t.Fatalf("stdout leaked %q: %q", secret, stdout)
		}
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want dry-run diagnostics on stderr only", stdout)
	}
	for _, want := range []string{
		`curl -H "Cookie: [REDACTED]" https://example.test`,
		"PASSWORD=[REDACTED]; echo next",
		"asc deploy --password [REDACTED] && echo done",
		"asc deploy --password [REDACTED] --verbose",
		"PASSWORD=[REDACTED] asc builds list",
		"ASC_SLACK_WEBHOOK=[REDACTED] asc notify slack --message ready",
		`asc web xcode-cloud usage alert --webhook-header [REDACTED] --webhook [REDACTED]`,
	} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("stderr = %q, want preserved context %q", stderr, want)
		}
	}
}

func TestSnitchDryRunRedactsMultilineQuotedAndSecretAnswerValues(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "")

	secrets := []string{
		"multiline-head",
		"multiline-tail-secret",
		"recovery-answer-secret",
	}
	stdout, stderr, err := runSnitchCommand(
		t, "9.9.9",
		"--dry-run",
		"--repro", "asc deploy --password \""+secrets[0]+"\n"+secrets[1]+" with space\" --verbose",
		"--actual", `{"secretQuestion":"Public question","secretAnswer":"`+secrets[2]+`","status":"active"}`,
		"multiline and recovery credential redaction probe",
	)
	if err != nil {
		t.Fatalf("run snitch: %v", err)
	}

	for _, secret := range secrets {
		if strings.Contains(stderr, secret) {
			t.Fatalf("stderr leaked %q: %q", secret, stderr)
		}
		if strings.Contains(stdout, secret) {
			t.Fatalf("stdout leaked %q: %q", secret, stdout)
		}
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want dry-run diagnostics on stderr only", stdout)
	}
	for _, want := range []string{
		"asc deploy --password [REDACTED] --verbose",
		`{"secretQuestion":"Public question","secretAnswer":"[REDACTED]","status":"active"}`,
	} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("stderr = %q, want preserved context %q", stderr, want)
		}
	}
}

func TestSnitchDryRunRedactsTwoFactorContinuationCredentials(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "")

	secrets := []string{
		"123456",
		"opaque-lowercase-continuation",
		"opaque-lowercase-session",
	}
	stdout, stderr, err := runSnitchCommand(
		t, "9.9.9",
		"--dry-run",
		"--repro", "asc web auth login --two-factor-code "+secrets[0]+" --apple-id user@example.test",
		"--actual", "< scnt: "+secrets[1]+"\n< X-Apple-ID-Session-Id: "+secrets[2],
		"two factor continuation credential redaction probe",
	)
	if err != nil {
		t.Fatalf("run snitch: %v", err)
	}

	for _, secret := range secrets {
		if strings.Contains(stderr, secret) {
			t.Fatalf("stderr leaked %q: %q", secret, stderr)
		}
		if strings.Contains(stdout, secret) {
			t.Fatalf("stdout leaked %q: %q", secret, stdout)
		}
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want dry-run diagnostics on stderr only", stdout)
	}
	for _, want := range []string{
		"asc web auth login --two-factor-code [REDACTED] --apple-id user@example.test",
		"< scnt: [REDACTED]",
		"< X-Apple-ID-Session-Id: [REDACTED]",
	} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("stderr = %q, want preserved context %q", stderr, want)
		}
	}
}

func TestSnitchDryRunRedactsPortalCSRFCredentials(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "")

	secrets := []string{
		"opaque-lowercase-csrf",
		"opaque-lowercase-csrf-timestamp",
		"structured-csrf-secret",
		"structured-csrf-timestamp-secret",
	}
	stdout, stderr, err := runSnitchCommand(
		t, "9.9.9",
		"--dry-run",
		"--repro", "< csrf: "+secrets[0]+"\n< csrf_ts: "+secrets[1],
		"--actual", `{"csrf":"`+secrets[2]+`","csrf_ts":"`+secrets[3]+`","status":"failed"}`,
		"portal csrf credential redaction probe",
	)
	if err != nil {
		t.Fatalf("run snitch: %v", err)
	}

	for _, secret := range secrets {
		if strings.Contains(stderr, secret) {
			t.Fatalf("stderr leaked %q: %q", secret, stderr)
		}
		if strings.Contains(stdout, secret) {
			t.Fatalf("stdout leaked %q: %q", secret, stdout)
		}
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want dry-run diagnostics on stderr only", stdout)
	}
	for _, want := range []string{
		"< csrf: [REDACTED]",
		"< csrf_ts: [REDACTED]",
		`{"csrf":"[REDACTED]","csrf_ts":"[REDACTED]","status":"failed"}`,
	} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("stderr = %q, want preserved context %q", stderr, want)
		}
	}
}

func TestSnitchDryRunRedactsCGICredentialHeaderVariables(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "")

	secrets := []string{
		"opaque-bearer-secret",
		"dXNlcjpwYXNz",
		"opaque-session-secret",
	}
	stdout, stderr, err := runSnitchCommand(
		t, "9.9.9",
		"--dry-run",
		"--actual", "HTTP_AUTHORIZATION=Bearer "+secrets[0]+"\n"+
			"REDIRECT_HTTP_AUTHORIZATION=Basic "+secrets[1]+"\n"+
			"HTTP_COOKIE=sessionid="+secrets[2]+"; locale=en-US\n"+
			"REQUEST_METHOD=GET",
		"CGI credential header redaction probe",
	)
	if err != nil {
		t.Fatalf("run snitch: %v", err)
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want dry-run diagnostics on stderr only", stdout)
	}
	for _, secret := range secrets {
		if strings.Contains(stderr, secret) {
			t.Fatalf("dry run leaked %q: %q", secret, stderr)
		}
	}
	for _, want := range []string{
		"HTTP_AUTHORIZATION=[REDACTED]",
		"REDIRECT_HTTP_AUTHORIZATION=[REDACTED]",
		"HTTP_COOKIE=[REDACTED]",
		"REQUEST_METHOD=GET",
	} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("stderr = %q, want preserved context %q", stderr, want)
		}
	}
}

func TestSnitchDryRunRedactsQuotedFormAndUnterminatedMultilineCredentials(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "")

	secrets := []string{
		"form-password-secret",
		"form-password-tail-secret",
		"urlencoded-client-secret",
		"urlencoded-client-tail-secret",
		"opaque-head",
		"opaque-tail-secret",
	}
	stdout, stderr, err := runSnitchCommand(
		t, "9.9.9",
		"--dry-run",
		"--repro", `curl --data 'password=`+secrets[0]+` `+secrets[1]+`' https://example.test`+"\n"+
			`curl --data-urlencode "client_secret=`+secrets[2]+` `+secrets[3]+`" https://example.test`,
		"--actual", "PASSWORD=\""+secrets[4]+"\n"+secrets[5],
		"quoted form and multiline credential redaction probe",
	)
	if err != nil {
		t.Fatalf("run snitch: %v", err)
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want dry-run diagnostics on stderr only", stdout)
	}
	for _, secret := range secrets {
		if strings.Contains(stderr, secret) {
			t.Fatalf("dry run leaked %q: %q", secret, stderr)
		}
	}
	for _, want := range []string{
		`curl --data 'password=[REDACTED]' https://example.test`,
		`curl --data-urlencode "client_secret=[REDACTED]" https://example.test`,
		"PASSWORD=[REDACTED]",
	} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("stderr = %q, want preserved context %q", stderr, want)
		}
	}
}

func TestSnitchDryRunRedactsWebAuthQueryCredentials(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "")

	secrets := []string{
		"widget-query-secret",
		"123456",
		"continuation-query-secret",
		"encoded-name-query-secret",
	}
	stdout, stderr, err := runSnitchCommand(
		t, "9.9.9",
		"--dry-run",
		"--repro", "curl 'https://example.test/auth?widgetKey="+secrets[0]+"&code="+secrets[1]+"&scnt="+secrets[2]+"&flow=login'",
		"--actual", "callback https://example.test/path?pass%77ord="+secrets[3]+"&state=ready",
		"web auth query credential redaction probe",
	)
	if err != nil {
		t.Fatalf("run snitch: %v", err)
	}

	for _, secret := range secrets {
		if strings.Contains(stderr, secret) {
			t.Fatalf("stderr leaked %q: %q", secret, stderr)
		}
		if strings.Contains(stdout, secret) {
			t.Fatalf("stdout leaked %q: %q", secret, stdout)
		}
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want dry-run diagnostics on stderr only", stdout)
	}
	for _, want := range []string{
		"curl 'https://example.test/auth?widgetKey=[REDACTED]&code=[REDACTED]&scnt=[REDACTED]&flow=login'",
		"callback https://example.test/path?pass%77ord=[REDACTED]&state=ready",
	} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("stderr = %q, want preserved context %q", stderr, want)
		}
	}
}

func TestSnitchDryRunRedactsCurlCookieDataAndSessionCacheValues(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "")

	secrets := []string{
		"curl-cookie-secret",
		"cached-cookie-secret",
		"second-cached-cookie-secret",
		"reordered-cached-cookie-secret",
		"browser-cookie-secret",
	}
	stdout, stderr, err := runSnitchCommand(
		t, "9.9.9",
		"--dry-run",
		"--repro", `curl --cookie 'myacinfo=`+secrets[0]+`' --cookie ./cookies.txt https://example.test`,
		"--actual", `{"cookies":{"https://appstoreconnect.apple.com":[{"name":"myacinfo","value":"`+secrets[1]+`","path":"/"},{"name":"dqsid","value":"`+secrets[2]+`"},{"value":"`+secrets[3]+`","name":"itctx"}]},"diagnostic":{"name":"failure","value":"preserve this explanation"},"version":1}`+"\n"+
			`{"cookies":[{"name":"sessionid","value":"`+secrets[4]+`","domain":"example.test"}],"version":1}`,
		"session cookie redaction probe",
	)
	if err != nil {
		t.Fatalf("run snitch: %v", err)
	}

	for _, secret := range secrets {
		if strings.Contains(stderr, secret) {
			t.Fatalf("stderr leaked %q: %q", secret, stderr)
		}
		if strings.Contains(stdout, secret) {
			t.Fatalf("stdout leaked %q: %q", secret, stdout)
		}
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want dry-run diagnostics on stderr only", stdout)
	}
	for _, want := range []string{
		`curl --cookie [REDACTED] --cookie ./cookies.txt https://example.test`,
		`{"cookies":{"https://appstoreconnect.apple.com":[{"name":"myacinfo","value":"[REDACTED]","path":"/"},{"name":"dqsid","value":"[REDACTED]"},{"value":"[REDACTED]","name":"itctx"}]},"diagnostic":{"name":"failure","value":"preserve this explanation"},"version":1}`,
		`{"cookies":[{"name":"sessionid","value":"[REDACTED]","domain":"example.test"}],"version":1}`,
	} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("stderr = %q, want preserved context %q", stderr, want)
		}
	}
}

func TestWriteLocalLogRedactsEveryStringField(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd() error: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(origDir); err != nil {
			t.Fatalf("os.Chdir restore error: %v", err)
		}
	})
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("os.Chdir temp dir error: %v", err)
	}

	secrets := []string{
		"description-secret",
		"reproduction-secret",
		"expected-secret",
		"actual-secret",
		"label-secret",
		"version-secret",
		"os-secret",
	}
	entry := LogEntry{
		Description: "token=" + secrets[0],
		Repro:       "api_key=" + secrets[1],
		Expected:    "password=" + secrets[2],
		Actual:      "refresh_token=" + secrets[3],
		Labels:      []string{"client_secret=" + secrets[4]},
		Severity:    "bug",
		ASCVersion:  "access_token=" + secrets[5],
		OS:          "webhook_secret=" + secrets[6],
	}

	if err := writeLocalLog(entry); err != nil {
		t.Fatalf("writeLocalLog() error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(".asc", "snitch.log"))
	if err != nil {
		t.Fatalf("os.ReadFile() error: %v", err)
	}
	for _, secret := range secrets {
		if strings.Contains(string(data), secret) {
			t.Fatalf("local log leaked %q: %s", secret, data)
		}
	}
	if got := strings.Count(string(data), "[REDACTED]"); got != len(secrets) {
		t.Fatalf("local log = %s, want %d redaction markers, got %d", data, len(secrets), got)
	}
}

func TestSearchIssuesRedactsCredentialInQuery(t *testing.T) {
	const secret = "query-bearer-secret"
	var searchQuery string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		searchQuery = r.URL.Query().Get("q")
		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write([]byte(`{"items":[]}`)); err != nil {
			t.Fatalf("w.Write() error: %v", err)
		}
	}))
	defer server.Close()

	origBase := githubAPIBase
	defer func() { setGitHubAPIBase(origBase) }()
	setGitHubAPIBase(server.URL)

	if _, err := searchIssues(t.Context(), "test-token", "Authorization: Bearer "+secret); err != nil {
		t.Fatalf("searchIssues() error: %v", err)
	}
	if strings.Contains(searchQuery, secret) {
		t.Fatalf("duplicate-search query leaked the credential: %q", searchQuery)
	}
	if !strings.Contains(searchQuery, "Authorization: [REDACTED]") {
		t.Fatalf("duplicate-search query = %q, want redacted context", searchQuery)
	}
}

func TestCreateIssueRedactsCredentialPayload(t *testing.T) {
	const secret = "issue-payload-secret-123"
	var payload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("json.Decode() error: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		if _, err := w.Write([]byte(`{"number":42,"title":"redacted","html_url":"https://example.test/issues/42"}`)); err != nil {
			t.Fatalf("w.Write() error: %v", err)
		}
	}))
	defer server.Close()

	origBase := githubAPIBase
	defer func() { setGitHubAPIBase(origBase) }()
	setGitHubAPIBase(server.URL)

	entry := LogEntry{
		Description: "token=" + secret,
		Actual:      "Bearer " + secret,
		Severity:    "bug",
	}
	if _, err := createIssue(t.Context(), "test-token", entry); err != nil {
		t.Fatalf("createIssue() error: %v", err)
	}

	encoded, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("json.Marshal() error: %v", err)
	}
	if strings.Contains(string(encoded), secret) {
		t.Fatalf("issue request leaked the credential: %s", encoded)
	}
	if got := strings.Count(string(encoded), "[REDACTED]"); got < 2 {
		t.Fatalf("issue request = %s, want redacted title and body", encoded)
	}
}

func TestAddIssueLabelsRedactsSensitiveValues(t *testing.T) {
	const secret = "label-payload-secret"
	var payload map[string][]string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("json.Decode() error: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte(`{"labels":[]}`)); err != nil {
			t.Fatalf("w.Write() error: %v", err)
		}
	}))
	defer server.Close()

	origBase := githubAPIBase
	defer func() { setGitHubAPIBase(origBase) }()
	setGitHubAPIBase(server.URL)

	if err := addIssueLabels(t.Context(), "test-token", 42, []string{"token=" + secret}); err != nil {
		t.Fatalf("addIssueLabels() error: %v", err)
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("json.Marshal() error: %v", err)
	}
	if strings.Contains(string(encoded), secret) {
		t.Fatalf("label request leaked the credential: %s", encoded)
	}
	if !strings.Contains(string(encoded), "[REDACTED]") {
		t.Fatalf("label request = %s, want a redaction marker", encoded)
	}
}

func TestFormatLocalEntriesRedactsLegacyCredentials(t *testing.T) {
	const secret = "legacy-log-secret"
	formatted := formatLocalEntries([]LogEntry{{
		Description: "old entry",
		Actual:      "Authorization: Bearer " + secret,
		Severity:    "bug",
	}})

	if strings.Contains(formatted, secret) {
		t.Fatalf("formatted log leaked the credential: %q", formatted)
	}
	if !strings.Contains(formatted, "Authorization: [REDACTED]") {
		t.Fatalf("formatted log = %q, want redacted context", formatted)
	}
}

func TestIssueBodyPreservesBenignSecurityVocabulary(t *testing.T) {
	entry := LogEntry{
		Description: "Bearer authentication fails behind proxy",
		Repro:       "asc builds list --filter-key token\nasc signing sync pull --password-file /tmp/sync-password\nasc auth login --private-key /path/to/AuthKey.p8\nasc auth login --private-key=/path/to/AuthKey.p8\nasc xcode validate --api-key KEY123ABC\ncurl --user alice https://example.test\ncurl --proxy-user alice https://example.test\ngit clone https://example.test/repo",
		Expected:    "secret scanning documentation remains visible",
		Actual:      `request to https://example.test/path?signature_state=missing returned 401 with {"passwordPolicy":"strict","tokenCount":0}`,
		Severity:    "bug",
	}
	body := issueBody(entry)
	if title := issueTitle(entry); title != entry.Description {
		t.Fatalf("issueTitle() = %q, want benign description %q preserved", title, entry.Description)
	}

	for _, want := range []string{entry.Description, entry.Repro, entry.Expected, entry.Actual} {
		if !strings.Contains(body, want) {
			t.Fatalf("issue body = %q, want benign text %q preserved", body, want)
		}
	}
	if strings.Contains(body, "[REDACTED]") {
		t.Fatalf("issue body unexpectedly redacted benign diagnostics: %q", body)
	}
}
