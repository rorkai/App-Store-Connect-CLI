package api

import (
	"regexp"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/schema"
)

// TestNativeCommandsAreUnindexed keeps nativeCommands to operations the
// embedded schema index lacks: `asc api` consults the table only after the
// index rejects a request, so an indexed key would never be reached, and a
// snapshot refresh that adds one should move its advice elsewhere.
func TestNativeCommandsAreUnindexed(t *testing.T) {
	for key := range nativeCommands {
		method, template, ok := strings.Cut(key, " ")
		if !ok {
			t.Fatalf("key %q is not METHOD PATH", key)
		}
		path := placeholderSegment.ReplaceAllString(template, "ID")
		if _, indexed, err := schema.MatchOperation(method, path); err != nil || indexed {
			t.Errorf("%s is in the embedded schema index (err %v), so `asc api` never reaches its native command", key, err)
		}
	}
}

var placeholderSegment = regexp.MustCompile(`\{[^/}]+\}`)
