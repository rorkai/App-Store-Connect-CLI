package cmdtest

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"hash/crc32"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

func TestImagePreflightsRejectAlphaBeforeNetwork(t *testing.T) {
	for _, kind := range []string{"alpha", "truecolor-tRNS"} {
		for _, command := range []string{"screenshots", "creative", "validate"} {
			t.Run(kind+"/"+command, func(t *testing.T) {
				setupAuth(t)
				img := image.NewNRGBA(image.Rect(0, 0, 640, 960))
				if kind == "truecolor-tRNS" {
					for i := 0; i < len(img.Pix); i += 4 {
						img.Pix[i+3] = 255
					}
				}
				var data bytes.Buffer
				if err := png.Encode(&data, img); err != nil {
					t.Fatal(err)
				}
				content := data.Bytes()
				if kind == "truecolor-tRNS" {
					chunk := make([]byte, 18)
					binary.BigEndian.PutUint32(chunk, 6)
					copy(chunk[4:8], "tRNS")
					binary.BigEndian.PutUint32(chunk[14:], crc32.ChecksumIEEE(chunk[4:14]))
					content = append(append(append([]byte{}, content[:33]...), chunk...), content[33:]...)
				}
				path := filepath.Join(t.TempDir(), "alpha.png")
				if err := os.WriteFile(path, content, 0o600); err != nil {
					t.Fatal(err)
				}
				reg := asctest.NewRegistry(t)
				reg.Install()
				args := []string{"screenshots", "upload", "--version-localization", "LOC", "--path", path, "--device-type", "IPHONE_35"}
				switch command {
				case "creative":
					args = []string{"asset-library", "images", "upload", "--library-id", "lib", "--file", path}
				case "validate":
					args = []string{"screenshots", "validate", "--path", path, "--device-type", "IPHONE_35", "--output", "json"}
				}
				stdout, stderr, err := runRootCommand(t, args)
				if command == "validate" {
					assertValidateReportsAlpha(t, stdout, stderr, err, path)
				} else if err == nil || !strings.Contains(err.Error(), "alpha channel or transparency") {
					t.Fatalf("expected alpha rejection, got %v; stdout=%s; stderr=%s", err, stdout, stderr)
				}
				if calls := len(reg.Requests()); calls != 0 {
					t.Fatalf("made %d network calls before alpha rejection", calls)
				}
			})
		}
	}
}

func assertValidateReportsAlpha(t *testing.T, stdout, stderr string, err error, path string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected validate to fail on an alpha PNG; stdout=%s; stderr=%s", stdout, stderr)
	}
	var result struct {
		ReadyFiles int `json:"readyFiles"`
		ErrorCount int `json:"errorCount"`
		Files      []struct {
			Status string `json:"status"`
		} `json:"files"`
		Issues []struct {
			Code        string `json:"code"`
			Severity    string `json:"severity"`
			FilePath    string `json:"filePath"`
			Message     string `json:"message"`
			Remediation string `json:"remediation"`
		} `json:"issues"`
	}
	if jsonErr := json.Unmarshal([]byte(stdout), &result); jsonErr != nil {
		t.Fatalf("decode validate output: %v; stdout=%s", jsonErr, stdout)
	}
	if result.ReadyFiles != 0 || result.ErrorCount != 1 || len(result.Files) != 1 || result.Files[0].Status != "error" || len(result.Issues) != 1 {
		t.Fatalf("expected one alpha error and no ready files, got %+v", result)
	}
	issue := result.Issues[0]
	if issue.Code != "alpha_channel" || issue.Severity != "error" || issue.FilePath != path ||
		!strings.Contains(issue.Message, "alpha channel or transparency") || !strings.Contains(issue.Remediation, "without an alpha channel") {
		t.Fatalf("unexpected alpha issue %+v", issue)
	}
}
