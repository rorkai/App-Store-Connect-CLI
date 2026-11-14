package cmdtest

import (
	"path/filepath"
	"strings"
	"testing"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

// A --poll-interval below 1s would poll App Store Connect faster than the
// default schedule and burn the per-key hourly request budget, so every media
// upload that waits for processing must reject it before sending requests.
func TestMediaUploadPollIntervalRejectsSubSecondValues(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	commands := [][]string{
		{"screenshots", "upload", "--version-localization", "LOC_ID", "--path", "./shots", "--device-type", "IPHONE_65"},
		{"video-previews", "upload", "--version-localization", "LOC_ID", "--path", "./previews", "--device-type", "IPHONE_65"},
		{"asset-library", "images", "upload", "--library-id", "LIB_ID", "--file", "./image.png"},
		{"asset-library", "videos", "upload", "--library-id", "LIB_ID", "--file", "./video.mp4"},
		{"product-pages", "custom-pages", "localizations", "screenshot-sets", "upload", "--localization-id", "LOC_ID", "--path", "./shots", "--device-type", "IPHONE_65"},
		{"product-pages", "custom-pages", "localizations", "screenshot-sets", "sync", "--localization-id", "LOC_ID", "--path", "./shots", "--device-type", "IPHONE_65", "--confirm"},
		{"product-pages", "custom-pages", "localizations", "preview-sets", "upload", "--localization-id", "LOC_ID", "--path", "./previews", "--device-type", "IPHONE_65"},
		{"product-pages", "custom-pages", "localizations", "preview-sets", "sync", "--localization-id", "LOC_ID", "--path", "./previews", "--device-type", "IPHONE_65", "--confirm"},
		{"product-pages", "experiments", "treatments", "localizations", "screenshot-sets", "upload", "--localization-id", "LOC_ID", "--path", "./shots", "--device-type", "IPHONE_65"},
		{"product-pages", "experiments", "treatments", "localizations", "screenshot-sets", "sync", "--localization-id", "LOC_ID", "--path", "./shots", "--device-type", "IPHONE_65", "--confirm"},
		{"product-pages", "experiments", "treatments", "localizations", "preview-sets", "upload", "--localization-id", "LOC_ID", "--path", "./previews", "--device-type", "IPHONE_65"},
	}
	for _, command := range commands {
		for _, value := range []string{"500ms", "0s"} {
			args := append(append([]string{}, command...), "--poll-interval", value)
			name := command[0]
			for _, part := range command[1:] {
				if strings.HasPrefix(part, "--") {
					break
				}
				name += " " + part
			}
			t.Run(name+" "+value, func(t *testing.T) {
				asctest.NewRegistry(t).Install()
				_, stderr, runErr := runRootCommand(t, args)
				if got := rootcmd.ExitCodeFromError(runErr); got != rootcmd.ExitUsage {
					t.Fatalf("exit code = %d, want %d (err=%v)", got, rootcmd.ExitUsage, runErr)
				}
				if want := "Error: --poll-interval must be at least 1s"; !strings.Contains(stderr, want) {
					t.Fatalf("expected %q in stderr %q", want, stderr)
				}
			})
		}
	}
}
