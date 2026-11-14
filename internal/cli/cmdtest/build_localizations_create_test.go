package cmdtest

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

func TestBuildLocalizationsCreateWarnsForIncompleteCreate(t *testing.T) {
	var seenPayload struct {
		Data struct {
			Attributes struct {
				Locale   string `json:"locale"`
				WhatsNew string `json:"whatsNew"`
			} `json:"attributes"`
			Relationships struct {
				AppStoreVersion struct {
					Data struct {
						ID string `json:"id"`
					} `json:"data"`
				} `json:"appStoreVersion"`
			} `json:"relationships"`
		} `json:"data"`
	}

	reg := asctest.NewRegistry(t)
	reg.Register(asctest.GET("/v1/builds/build-1/appStoreVersion"),
		asctest.JSON(http.StatusOK, `{"data":{"type":"appStoreVersions","id":"version-1"}}`))
	reg.Register(asctest.POST("/v1/appStoreVersionLocalizations").JSONBody(`{"data":{"type":"appStoreVersionLocalizations","attributes":{"locale":"en-US","whatsNew":"Bug fixes"},"relationships":{"appStoreVersion":{"data":{"type":"appStoreVersions","id":"version-1"}}}}}`),
		asctest.JSON(http.StatusCreated, `{"data":{"type":"appStoreVersionLocalizations","id":"loc-build-1","attributes":{"locale":"en-US","whatsNew":"Bug fixes"}}}`))

	stdout, stderr, err := runRootWithDeps(t, stubClient(t, reg),
		"build-localizations", "create",
		"--build-id", "build-1",
		"--locale", "en-US",
		"--whats-new", "Bug fixes",
	)
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if !strings.Contains(stderr, "created locale en-US now participates in submission validation") {
		t.Fatalf("expected create warning on stderr, got %q", stderr)
	}
	if !strings.Contains(stderr, "description, keywords, supportUrl") {
		t.Fatalf("expected missing submit fields in warning, got %q", stderr)
	}
	for _, req := range reg.Requests() {
		if req.Method == http.MethodPost {
			if err := json.Unmarshal(req.Body, &seenPayload); err != nil {
				t.Fatalf("decode payload: %v", err)
			}
		}
	}
	if seenPayload.Data.Attributes.Locale != "en-US" {
		t.Fatalf("expected locale en-US, got %q", seenPayload.Data.Attributes.Locale)
	}
	if seenPayload.Data.Attributes.WhatsNew != "Bug fixes" {
		t.Fatalf("expected whatsNew Bug fixes, got %q", seenPayload.Data.Attributes.WhatsNew)
	}
	if seenPayload.Data.Relationships.AppStoreVersion.Data.ID != "version-1" {
		t.Fatalf("expected version relationship version-1, got %q", seenPayload.Data.Relationships.AppStoreVersion.Data.ID)
	}

	var out struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &out); err != nil {
		t.Fatalf("stdout should be valid json: %v\nstdout=%q", err, stdout)
	}
	if out.Data.ID != "loc-build-1" {
		t.Fatalf("expected localization id loc-build-1, got %q", out.Data.ID)
	}
}
