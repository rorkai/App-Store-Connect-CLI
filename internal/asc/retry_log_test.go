package asc

import (
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/config"
)

func TestResolveRetryLogEnabled_OverrideBeatsEnvAndConfig(t *testing.T) {
	t.Setenv("ASC_RETRY_LOG", "1")
	on := true
	SetRetryLogOverride(&on)
	t.Cleanup(func() { SetRetryLogOverride(nil) })

	if !ResolveRetryLogEnabled() {
		t.Fatal("expected override=true to enable retry logging")
	}

	off := false
	SetRetryLogOverride(&off)
	if ResolveRetryLogEnabled() {
		t.Fatal("expected override=false to disable retry logging")
	}
}

func TestResolveRetryLogEnabled_EnvBeatsConfig(t *testing.T) {
	setConfigLoaderForTest(func() (*config.Config, error) {
		return &config.Config{RetryLog: "on"}, nil
	})
	t.Cleanup(resetConfigCacheForTest)
	for _, value := range []string{"", "0", " FALSE ", "no", "off"} {
		t.Setenv("ASC_RETRY_LOG", value)
		if ResolveRetryLogEnabled() {
			t.Errorf("ASC_RETRY_LOG=%q should disable logging over an enabled config", value)
		}
	}

	t.Setenv("ASC_RETRY_LOG", "1")
	if !ResolveRetryLogEnabled() {
		t.Fatal("expected ASC_RETRY_LOG to enable retry logging")
	}
}
