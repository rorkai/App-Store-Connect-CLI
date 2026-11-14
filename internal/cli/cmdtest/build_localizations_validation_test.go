package cmdtest

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	cmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
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
			t.Cleanup(shared.SetASCClientFactoryForTesting(func() (*asc.Client, error) {
				clientCalls++
				return nil, errors.New("client should not be created")
			}))

			args := append(append([]string(nil), test.args...), "--whats-new", strings.Repeat("w", validation.LimitWhatsNew+1))

			stdout, stderr := captureOutput(t, func() {
				if code := cmd.Run(args, "1.2.3"); code != cmd.ExitUsage {
					t.Fatalf("expected exit code %d, got %d", cmd.ExitUsage, code)
				}
			})

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
	setupAuth(t)

	whatsNew := strings.Repeat("あ", validation.LimitWhatsNew)
	if len(whatsNew) <= validation.LimitWhatsNew {
		t.Fatalf("expected multibyte payload larger than the character limit, got %d bytes", len(whatsNew))
	}

	var sentWhatsNew string
	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/builds/build-1/appStoreVersion"),
		asctest.JSON(http.StatusOK, `{"data":{"type":"appStoreVersions","id":"version-1"}}`))
	reg.Register(asctest.POST("/v1/appStoreVersionLocalizations"),
		asctest.JSON(http.StatusCreated, `{"data":{"type":"appStoreVersionLocalizations","id":"loc-1","attributes":{"locale":"ja"}}}`))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	captureOutput(t, func() {
		if err := root.Parse([]string{
			"build-localizations", "create",
			"--build-id", "build-1",
			"--locale", "ja",
			"--whats-new", whatsNew,
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(t.Context()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

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
