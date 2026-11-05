package cmdtest

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
)

func TestGameCenterVersionDefaultImage(t *testing.T) {
	for _, kind := range []struct{ group, resource string }{
		{"challenges", "Challenge"},
		{"activities", "Activity"},
	} {
		for _, present := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/present=%t", kind.group, present), func(t *testing.T) {
				setupAuth(t)
				t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))
				path := "/v1/gameCenter" + kind.resource + "Versions/ver-1/defaultImage"
				data := "null"
				if present {
					data = `{"type":"gameCenter` + kind.resource + `Images","id":"img-1","attributes":{"fileName":"image.png","fileSize":12}}`
				}
				body := `{"data":` + data + `,"links":{"self":"https://api.appstoreconnect.apple.com` + path + `"}}`
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
					if req.Method != http.MethodGet || req.URL.Path != path || req.URL.RawQuery != "" {
						t.Errorf("unexpected request: %s %s", req.Method, req.URL)
					}
					w.Header().Set("Content-Type", "application/json")
					_, _ = io.WriteString(w, body)
				}))
				t.Cleanup(server.Close)
				installDefaultTransportForServer(t, server)

				for _, output := range []string{"json", "table", "markdown"} {
					t.Run(output, func(t *testing.T) {
						args := []string{"game-center", kind.group, "versions", "default-image", "view", "--id", "ver-1", "--output", output}
						stdout, stderr := captureOutput(t, func() {
							if code := rootcmd.Run(args, "test"); code != rootcmd.ExitSuccess {
								t.Errorf("exit code = %d, want success", code)
							}
						})
						if stderr != "" {
							t.Fatalf("stderr = %q, want empty", stderr)
						}
						if output == "json" {
							var got, want any
							if err := json.Unmarshal([]byte(stdout), &got); err != nil {
								t.Fatalf("decode stdout: %v", err)
							}
							if err := json.Unmarshal([]byte(body), &want); err != nil {
								t.Fatal(err)
							}
							if !reflect.DeepEqual(got, want) {
								t.Fatalf("response changed: got %s, want %s", stdout, body)
							}
							return
						}
						if !strings.Contains(stdout, "File Name") || !strings.Contains(stdout, "File Size") {
							t.Fatalf("missing image headers: %q", stdout)
						}
						if present {
							if !strings.Contains(stdout, "img-1") || !strings.Contains(stdout, "image.png") || !strings.Contains(stdout, "12") {
								t.Fatalf("missing image row: %q", stdout)
							}
						} else if strings.ContainsAny(stdout, "0123456789") {
							t.Fatalf("invented an image row for null data: %q", stdout)
						}
					})
				}
			})
		}
	}
}
