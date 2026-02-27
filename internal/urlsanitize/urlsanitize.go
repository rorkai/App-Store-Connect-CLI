package urlsanitize

import (
	"net/url"
	"strings"
)

// DefaultSignedQueryKeys identifies query params that indicate signed URLs.
var DefaultSignedQueryKeys = map[string]struct{}{
	"x-amz-signature":     {},
	"x-amz-credential":    {},
	"x-amz-algorithm":     {},
	"x-amz-signedheaders": {},
	"signature":           {},
	"key-pair-id":         {},
	"policy":              {},
	"sig":                 {},
}

// DefaultSensitiveQueryKeys identifies params that should be redacted in logs.
var DefaultSensitiveQueryKeys = map[string]struct{}{
	"x-amz-signature":      {},
	"x-amz-credential":     {},
	"x-amz-algorithm":      {},
	"x-amz-signedheaders":  {},
	"x-amz-security-token": {},
	"signature":            {},
	"key-pair-id":          {},
	"policy":               {},
	"sig":                  {},
	"token":                {},
	"access_token":         {},
	"id_token":             {},
	"refresh_token":        {},
}

// CopyKeySet returns a shallow copy of a key set map.
func CopyKeySet(source map[string]struct{}) map[string]struct{} {
	if len(source) == 0 {
		return map[string]struct{}{}
	}
	cloned := make(map[string]struct{}, len(source))
	for key := range source {
		cloned[key] = struct{}{}
	}
	return cloned
}

// MergeKeySets returns a merged copy of all provided key sets.
func MergeKeySets(sets ...map[string]struct{}) map[string]struct{} {
	merged := make(map[string]struct{})
	for _, set := range sets {
		for key := range set {
			merged[strings.ToLower(strings.TrimSpace(key))] = struct{}{}
		}
	}
	return merged
}

// HasSignedQuery returns true when query contains a non-empty signing key value.
func HasSignedQuery(values url.Values, signedKeys map[string]struct{}) bool {
	if len(values) == 0 || len(signedKeys) == 0 {
		return false
	}
	for key, vals := range values {
		if _, ok := signedKeys[strings.ToLower(key)]; ok && hasNonEmptyValue(vals) {
			return true
		}
	}
	return false
}

// SanitizeURLForLog redacts sensitive URL fields while preserving shape.
func SanitizeURLForLog(rawURL string, signedKeys, sensitiveKeys map[string]struct{}) string {
	if rawURL == "" {
		return ""
	}
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	if parsedURL.User != nil {
		parsedURL.User = url.User("[REDACTED]")
	}
	values := parsedURL.Query()
	if len(values) == 0 {
		return parsedURL.String()
	}
	redactAll := HasSignedQuery(values, signedKeys)
	for key, vals := range values {
		if redactAll || isSensitiveQueryKey(key, sensitiveKeys) {
			for i := range vals {
				vals[i] = "[REDACTED]"
			}
			values[key] = vals
		}
	}
	parsedURL.RawQuery = values.Encode()
	return parsedURL.String()
}

func isSensitiveQueryKey(key string, sensitiveKeys map[string]struct{}) bool {
	if len(sensitiveKeys) == 0 {
		return false
	}
	_, ok := sensitiveKeys[strings.ToLower(key)]
	return ok
}

func hasNonEmptyValue(values []string) bool {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return true
		}
	}
	return false
}
