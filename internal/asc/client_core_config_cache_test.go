package asc

import (
	"fmt"
	"os"
	"sync/atomic"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/config"
)

func TestLoadConfigCachesSingleLoadAcrossResolvers(t *testing.T) {
	SetRetryLogOverride(nil)
	t.Cleanup(func() { SetRetryLogOverride(nil) })

	originalValue, hadOriginal := os.LookupEnv("ASC_RETRY_LOG")
	_ = os.Unsetenv("ASC_RETRY_LOG")
	t.Cleanup(func() {
		if hadOriginal {
			_ = os.Setenv("ASC_RETRY_LOG", originalValue)
			return
		}
		_ = os.Unsetenv("ASC_RETRY_LOG")
	})

	var calls int32
	setConfigLoaderForTest(func() (*config.Config, error) {
		atomic.AddInt32(&calls, 1)
		return &config.Config{RetryLog: "1"}, nil
	})
	t.Cleanup(resetConfigCacheForTest)
	t.Cleanup(resetConfigCacheForTest)

	if !ResolveRetryLogEnabled() {
		t.Fatal("expected retry logging enabled from config cache")
	}
	if !ResolveRetryLogEnabled() {
		t.Fatal("expected retry logging enabled from cached config")
	}

	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("expected config loader called once, got %d", got)
	}
}

func TestResetConfigCacheForTestReloadsConfig(t *testing.T) {
	SetRetryLogOverride(nil)
	t.Cleanup(func() { SetRetryLogOverride(nil) })

	originalValue, hadOriginal := os.LookupEnv("ASC_RETRY_LOG")
	_ = os.Unsetenv("ASC_RETRY_LOG")
	t.Cleanup(func() {
		if hadOriginal {
			_ = os.Setenv("ASC_RETRY_LOG", originalValue)
			return
		}
		_ = os.Unsetenv("ASC_RETRY_LOG")
	})

	var calls int32
	setConfigLoaderForTest(func() (*config.Config, error) {
		atomic.AddInt32(&calls, 1)
		return &config.Config{RetryLog: "1"}, nil
	})

	if !ResolveRetryLogEnabled() {
		t.Fatal("expected retry logging enabled from first config load")
	}

	setConfigLoaderForTest(func() (*config.Config, error) {
		atomic.AddInt32(&calls, 1)
		return &config.Config{}, nil
	})

	if ResolveRetryLogEnabled() {
		t.Fatal("expected retry logging disabled after cache reset and reload")
	}

	if got := atomic.LoadInt32(&calls); got != 2 {
		t.Fatalf("expected config loader called twice across resets, got %d", got)
	}
}

func TestLoadConfigRetriesAfterFailure(t *testing.T) {
	SetRetryLogOverride(nil)
	t.Cleanup(func() { SetRetryLogOverride(nil) })

	originalValue, hadOriginal := os.LookupEnv("ASC_RETRY_LOG")
	_ = os.Unsetenv("ASC_RETRY_LOG")
	t.Cleanup(func() {
		if hadOriginal {
			_ = os.Setenv("ASC_RETRY_LOG", originalValue)
			return
		}
		_ = os.Unsetenv("ASC_RETRY_LOG")
	})

	var calls int32
	setConfigLoaderForTest(func() (*config.Config, error) {
		if atomic.AddInt32(&calls, 1) == 1 {
			return nil, fmt.Errorf("config not found")
		}
		return &config.Config{RetryLog: "1"}, nil
	})
	t.Cleanup(resetConfigCacheForTest)

	if ResolveRetryLogEnabled() {
		t.Fatal("expected retry logging disabled when first config load fails")
	}

	if !ResolveRetryLogEnabled() {
		t.Fatal("expected retry logging enabled after subsequent successful config load")
	}

	if got := atomic.LoadInt32(&calls); got != 2 {
		t.Fatalf("expected config loader to be retried after failure, got %d calls", got)
	}
}
