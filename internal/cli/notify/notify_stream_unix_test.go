//go:build darwin || linux

package notify

import (
	"fmt"
	"os"
	"strings"
	"testing"
)

func TestSlackMessageFilesPreservePipeInputs(t *testing.T) {
	for _, blocks := range []bool{true, false} {
		t.Run(fmt.Sprint("blocks=", blocks), func(t *testing.T) {
			reader, writer, err := os.Pipe()
			if err != nil {
				t.Fatal(err)
			}
			defer reader.Close()
			body := `{"status":"submitted"}`
			if blocks {
				body = `[{"type":"divider"}]`
			}
			if _, err := writer.WriteString(body); err != nil {
				t.Fatal(err)
			}
			if err := writer.Close(); err != nil {
				t.Fatal(err)
			}
			path := fmt.Sprintf("/dev/fd/%d", reader.Fd())
			d, stderr := newNotifyTestDeps(nil)
			if blocks {
				value, err := parseSlackBlocks(d, "", path)
				if err != nil || len(value) != 1 || string(value[0]) != `{"type":"divider"}` {
					t.Fatalf("blocks=%s error=%v", value, err)
				}
			} else {
				value, err := parseSlackPayload(d, "", path)
				if err != nil || value["status"] != "submitted" {
					t.Fatalf("payload=%v error=%v", value, err)
				}
			}
			flagName := "--payload-file"
			if blocks {
				flagName = "--blocks-file"
			}
			if !strings.Contains(stderr.String(), "Warning: "+flagName+" non-regular or descriptor input is deprecated") {
				t.Fatalf("missing stream deprecation warning: %q", stderr.String())
			}
		})
	}
}
