package cmdtest

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	cmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/validation"
)

// App Store Connect rejects release notes longer than the documented limit, so
// the CLI has to refuse them before spending a request on the resolution and
// mutation round-trip.
func TestBuildLocalizationsRejectsOverlongWhatsNewBeforeAnyRequest(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{
			name: "create",
			args: []string{"build-localizations", "create", "--build-id", "build-1", "--locale", "en-US"},
		},
		{
			name: "update",
			args: []string{"build-localizations", "update", "--id", "loc-1"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			clientCalls := 0
			client := func() (*asc.Client, error) {
				clientCalls++
				return nil, errors.New("client should not be created")
			}

			args := append(append([]string(nil), test.args...), "--whats-new", strings.Repeat("w", validation.LimitWhatsNew+1))

			stdout, stderr, code := runWithDeps(t, client, args...)
			if code != cmd.ExitUsage {
				t.Fatalf("expected exit code %d, got %d", cmd.ExitUsage, code)
			}

			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, "whatsNew exceeds 4000 characters") {
				t.Fatalf("expected length error on stderr, got %q", stderr)
			}
			if clientCalls != 0 {
				t.Fatalf("expected no client creation, got %d", clientCalls)
			}
		})
	}
}

// Release notes are limited by character count, so multibyte text at the limit
// stays valid even though it is several times that many bytes.
func TestBuildLocalizationsAcceptsMultibyteWhatsNewAtLimit(t *testing.T) {
	whatsNew := strings.Repeat("あ", validation.LimitWhatsNew)
	if len(whatsNew) <= validation.LimitWhatsNew {
		t.Fatalf("expected multibyte payload larger than the character limit, got %d bytes", len(whatsNew))
	}

	var sentWhatsNew string
	reg := asctest.NewRegistry(t)
	reg.Register(asctest.GET("/v1/builds/build-1/appStoreVersion"),
		asctest.JSON(http.StatusOK, `{"data":{"type":"appStoreVersions","id":"version-1"}}`))
	reg.Register(asctest.POST("/v1/appStoreVersionLocalizations"),
		asctest.JSON(http.StatusCreated, `{"data":{"type":"appStoreVersionLocalizations","id":"loc-1","attributes":{"locale":"ja"}}}`))

	if _, _, err := runRootWithDeps(t, stubClient(t, reg),
		"build-localizations", "create",
		"--build-id", "build-1",
		"--locale", "ja",
		"--whats-new", whatsNew,
	); err != nil {
		t.Fatalf("run error: %v", err)
	}

	for _, req := range reg.Requests() {
		if req.Method != http.MethodPost {
			continue
		}
		var payload struct {
			Data struct {
				Attributes struct {
					WhatsNew string `json:"whatsNew"`
				} `json:"attributes"`
			} `json:"data"`
		}
		if err := json.Unmarshal(req.Body, &payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		sentWhatsNew = payload.Data.Attributes.WhatsNew
	}
	if sentWhatsNew != whatsNew {
		t.Fatalf("expected operator input to reach the API unchanged, got %d characters", len([]rune(sentWhatsNew)))
	}
}
