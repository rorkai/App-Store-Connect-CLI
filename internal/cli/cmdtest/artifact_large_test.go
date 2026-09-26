package cmdtest

import (
	"archive/zip"
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
)

func TestRunArtifactInfoLargeFiles(t *testing.T) {
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_TELEMETRY_DISABLED", "1")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "no-config.json"))
	for _, kind := range []string{"ipa", "pkg"} {
		t.Run(kind, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "large."+kind)
			file, err := os.Create(path)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { file.Close() })
			const offset = 600 << 20
			if kind == "ipa" {
				// A sparse prefix keeps this valid ZIP large without storing its padding.
				if _, err := file.Seek(offset, io.SeekStart); err != nil {
					t.Fatal(err)
				}
				writer := zip.NewWriter(file)
				writer.SetOffset(offset)
				for name, data := range map[string]string{
					"Payload/Large.app/Info.plist":               `<plist version="1.0"><dict><key>CFBundleIdentifier</key><string>com.example.large</string></dict></plist>`,
					"Payload/Large.app/embedded.mobileprovision": `<plist version="1.0"><dict><key>Entitlements</key><dict/></dict></plist>`,
				} {
					entry, err := writer.Create(name)
					if err != nil {
						t.Fatal(err)
					}
					if _, err := io.WriteString(entry, data); err != nil {
						t.Fatal(err)
					}
				}
				if err := writer.Close(); err != nil {
					t.Fatal(err)
				}
			} else {
				info := `<pkg-info identifier="com.example.large" version="1.0"/>`
				toc := fmt.Sprintf(`<xar><toc><file><name>Payload</name><type>file</type><data><offset>0</offset><length>%d</length><size>%d</size></data></file><file><name>PackageInfo</name><type>file</type><data><offset>%d</offset><length>%d</length><size>%d</size></data></file></toc></xar>`, offset, offset, offset, len(info), len(info))
				var compressed bytes.Buffer
				writer := zlib.NewWriter(&compressed)
				if _, err := io.WriteString(writer, toc); err != nil {
					t.Fatal(err)
				}
				if err := writer.Close(); err != nil {
					t.Fatal(err)
				}
				header := make([]byte, 28)
				copy(header, "xar!")
				binary.BigEndian.PutUint16(header[4:6], 28)
				binary.BigEndian.PutUint16(header[6:8], 1)
				binary.BigEndian.PutUint64(header[8:16], uint64(compressed.Len()))
				binary.BigEndian.PutUint64(header[16:24], uint64(len(toc)))
				if _, err := file.Write(append(header, compressed.Bytes()...)); err != nil {
					t.Fatal(err)
				}
				if _, err := file.Seek(offset, io.SeekCurrent); err != nil {
					t.Fatal(err)
				}
				if _, err := io.WriteString(file, info); err != nil {
					t.Fatal(err)
				}
			}
			if err := file.Close(); err != nil {
				t.Fatal(err)
			}
			var exit int
			stdout, stderr := captureOutput(t, func() {
				exit = cmd.Run([]string{kind + "-info", "--path", path, "--output", "json"}, "test")
			})
			if exit != cmd.ExitSuccess {
				t.Fatalf("exit=%d stderr=%q stdout=%q", exit, stderr, stdout)
			}
			var receipt map[string]any
			if err := json.Unmarshal([]byte(stdout), &receipt); err != nil {
				t.Fatal(err)
			}
			identity := "bundleId"
			if kind == "pkg" {
				identity = "productId"
			}
			if receipt[identity] != "com.example.large" || receipt["status"] != "readable" || receipt["signatureVerification"] != "not-verified" {
				t.Fatalf("receipt=%v", receipt)
			}
		})
	}
}
