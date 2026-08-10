//go:build darwin || linux

package cmdtest

import (
	"fmt"
	"io"
	"os"
	"testing"
)

func TestAppClipsAdvancedExperiencesCreateMissingSelectorDoesNotReadConfig(t *testing.T) {
	originalAppID, hadAppID := os.LookupEnv("ASC_APP_ID")
	if err := os.Unsetenv("ASC_APP_ID"); err != nil {
		t.Fatalf("unset ASC_APP_ID: %v", err)
	}
	t.Cleanup(func() {
		if hadAppID {
			if err := os.Setenv("ASC_APP_ID", originalAppID); err != nil {
				t.Errorf("restore ASC_APP_ID: %v", err)
			}
			return
		}
		if err := os.Unsetenv("ASC_APP_ID"); err != nil {
			t.Errorf("clear ASC_APP_ID: %v", err)
		}
	})

	configReader, configWriter, err := os.Pipe()
	if err != nil {
		t.Fatalf("create config sentinel: %v", err)
	}
	t.Cleanup(func() { _ = configReader.Close() })
	if _, err := configWriter.WriteString("{}"); err != nil {
		t.Fatalf("write config sentinel: %v", err)
	}
	if err := configWriter.Close(); err != nil {
		t.Fatalf("close config sentinel: %v", err)
	}
	t.Setenv("ASC_CONFIG_PATH", fmt.Sprintf("/dev/fd/%d", configReader.Fd()))

	assertAppClipAdvancedExperienceCreateUsageBeforeClient(
		t,
		nil,
		"Error: --app-clip-id or --bundle-id is required\n",
	)
	remainingConfig, err := io.ReadAll(configReader)
	if err != nil {
		t.Fatalf("read config sentinel: %v", err)
	}
	if string(remainingConfig) != "{}" {
		t.Fatal("configuration was read before selector validation")
	}
}
