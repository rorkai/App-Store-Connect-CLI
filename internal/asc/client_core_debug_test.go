package asc

import "testing"

func TestResolveDebugSettings_EnvAPI(t *testing.T) {
	t.Setenv("ASC_DEBUG", "api")
	SetDebugOverride(nil)
	SetDebugHTTPOverride(nil)
	t.Cleanup(func() {
		SetDebugOverride(nil)
		SetDebugHTTPOverride(nil)
	})

	settings := resolveDebugSettings()
	if !settings.enabled {
		t.Fatal("expected debug to be enabled")
	}
	if !settings.verboseHTTP {
		t.Fatal("expected HTTP debug to be enabled for ASC_DEBUG=api")
	}
}

func TestResolveDebugSettings_EnvTrue(t *testing.T) {
	t.Setenv("ASC_DEBUG", "1")
	SetDebugOverride(nil)
	SetDebugHTTPOverride(nil)
	t.Cleanup(func() {
		SetDebugOverride(nil)
		SetDebugHTTPOverride(nil)
	})

	settings := resolveDebugSettings()
	if !settings.enabled {
		t.Fatal("expected debug to be enabled")
	}
	if settings.verboseHTTP {
		t.Fatal("expected HTTP debug to be disabled for ASC_DEBUG=1")
	}
}

func TestResolveDebugSettings_EnvFalse(t *testing.T) {
	for _, value := range []string{"0", "f", "false", "no", "n", "off", "OFF"} {
		t.Run(value, func(t *testing.T) {
			t.Setenv("ASC_DEBUG", value)
			SetDebugOverride(nil)
			SetDebugHTTPOverride(nil)
			t.Cleanup(func() {
				SetDebugOverride(nil)
				SetDebugHTTPOverride(nil)
			})

			settings := resolveDebugSettings()
			if settings.enabled {
				t.Fatalf("expected debug to be disabled for ASC_DEBUG=%s", value)
			}
			if settings.verboseHTTP {
				t.Fatalf("expected HTTP debug to be disabled for ASC_DEBUG=%s", value)
			}
		})
	}
}

func TestResolveDebugSettings_DebugOverrideDisablesHTTP(t *testing.T) {
	t.Setenv("ASC_DEBUG", "api")
	SetDebugHTTPOverride(nil)
	debugEnabled := true
	SetDebugOverride(&debugEnabled)
	t.Cleanup(func() {
		SetDebugOverride(nil)
		SetDebugHTTPOverride(nil)
	})

	settings := resolveDebugSettings()
	if !settings.enabled {
		t.Fatal("expected debug to be enabled")
	}
	if settings.verboseHTTP {
		t.Fatal("expected HTTP debug to be disabled when --debug is set")
	}
}

func TestResolveDebugSettings_HTTPOverrideEnablesHTTP(t *testing.T) {
	t.Setenv("ASC_DEBUG", "")
	SetDebugOverride(nil)
	httpEnabled := true
	SetDebugHTTPOverride(&httpEnabled)
	t.Cleanup(func() {
		SetDebugOverride(nil)
		SetDebugHTTPOverride(nil)
	})

	settings := resolveDebugSettings()
	if !settings.enabled {
		t.Fatal("expected debug to be enabled when HTTP debug override is set")
	}
	if !settings.verboseHTTP {
		t.Fatal("expected HTTP debug to be enabled when override is set")
	}
}

func TestResolveDebugSettings_HTTPOverrideDisablesHTTP(t *testing.T) {
	t.Setenv("ASC_DEBUG", "api")
	SetDebugOverride(nil)
	httpEnabled := false
	SetDebugHTTPOverride(&httpEnabled)
	t.Cleanup(func() {
		SetDebugOverride(nil)
		SetDebugHTTPOverride(nil)
	})

	settings := resolveDebugSettings()
	if !settings.enabled {
		t.Fatal("expected debug to be enabled for ASC_DEBUG=api")
	}
	if settings.verboseHTTP {
		t.Fatal("expected HTTP debug to be disabled by override")
	}
}

func TestResolveDebugSettings_HTTPOverrideEnablesHTTPWithDebugEnv(t *testing.T) {
	t.Setenv("ASC_DEBUG", "1")
	SetDebugOverride(nil)
	httpEnabled := true
	SetDebugHTTPOverride(&httpEnabled)
	t.Cleanup(func() {
		SetDebugOverride(nil)
		SetDebugHTTPOverride(nil)
	})

	settings := resolveDebugSettings()
	if !settings.enabled {
		t.Fatal("expected debug to be enabled for ASC_DEBUG=1")
	}
	if !settings.verboseHTTP {
		t.Fatal("expected HTTP debug to be enabled by override")
	}
}

func TestResolveDebugSettings_DebugOverrideDisablesAll(t *testing.T) {
	t.Setenv("ASC_DEBUG", "api")
	httpEnabled := true
	SetDebugHTTPOverride(&httpEnabled)
	debugEnabled := false
	SetDebugOverride(&debugEnabled)
	t.Cleanup(func() {
		SetDebugOverride(nil)
		SetDebugHTTPOverride(nil)
	})

	settings := resolveDebugSettings()
	if settings.enabled || settings.verboseHTTP {
		t.Fatal("expected debug to be disabled by override")
	}
}
