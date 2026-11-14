package cmdtest

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

func TestCustomPagesCreateSendsTemplateRelationship(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{"app store version", []string{"--from-version", "version-1"}, `,"appStoreVersionTemplate":{"data":{"type":"appStoreVersions","id":"version-1"}}`},
		{"custom product page", []string{"--from-custom-page", "page-1"}, `,"customProductPageTemplate":{"data":{"type":"appCustomProductPages","id":"page-1"}}`},
		{"no template", nil, ``},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setupAuth(t)
			t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))
			reg := asctest.NewRegistry(t)
			reg.Register(asctest.GET("/v1/apps/app-1"),
				asctest.JSON(200, `{"data":{"type":"apps","id":"app-1","attributes":{"primaryLocale":"en-US"}}}`))
			reg.Register(asctest.POST("/v1/appCustomProductPages").JSONBody(`{"data":{"type":"appCustomProductPages","attributes":{"name":"Summer"},"relationships":{"app":{"data":{"type":"apps","id":"app-1"}},"appCustomProductPageVersions":{"data":[{"type":"appCustomProductPageVersions","id":"${new-appCustomProductPageVersion-id}"}]}`+tt.want+`}},"included":[{"type":"appCustomProductPageVersions","id":"${new-appCustomProductPageVersion-id}","relationships":{"appCustomProductPageLocalizations":{"data":[{"type":"appCustomProductPageLocalizations","id":"${new-appCustomProductPageLocalization-id}"}]}}},{"type":"appCustomProductPageLocalizations","id":"${new-appCustomProductPageLocalization-id}","attributes":{"locale":"en-US"}}]}`),
				asctest.JSON(201, `{"data":{"type":"appCustomProductPages","id":"new-page","attributes":{"name":"Summer"}}}`))
			args := append([]string{"product-pages", "custom-pages", "create", "--app", "app-1", "--name", "Summer", "--output", "json"}, tt.args...)
			stdout, stderr, err := runRootWithDeps(t, stubClient(t, reg), args...)
			if err != nil || !strings.Contains(stdout, `"id":"new-page"`) {
				t.Fatalf("err=%v stdout=%q stderr=%q", err, stdout, stderr)
			}
		})
	}
}

func TestCustomPagesCreateRejectsTwoTemplatesBeforeHTTP(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))
	stdout, stderr, err := runRootWithDeps(t, stubClient(t, asctest.NewRegistry(t)), "product-pages", "custom-pages", "create", "--app", "app-1", "--name", "Summer", "--from-version", "version-1", "--from-custom-page", "page-1")
	if !isUsageClassError(err) || stdout != "" || !strings.Contains(stderr, "--from-version and --from-custom-page are mutually exclusive") {
		t.Fatalf("err=%v stdout=%q stderr=%q", err, stdout, stderr)
	}
}
