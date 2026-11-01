package web

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/appleauth"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/securitykey"
	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

// stubTwoFactorDeliveryLogin stubs a sign-in that reaches 2FA and records the
// delivery preference visible when the challenge is prepared.
func stubTwoFactorDeliveryLogin(t *testing.T) (*appleauth.TwoFactorPreference, *bool) {
	t.Helper()
	origLogin := webLoginFn
	origPrepare := prepareTwoFactorChallengeFn
	origEnsure := ensureTwoFactorCodeRequestedFn
	origSubmit := submitTwoFactorCodeFn
	origStatusWriter := twoFactorStatusWriter
	t.Cleanup(func() {
		webLoginFn = origLogin
		prepareTwoFactorChallengeFn = origPrepare
		ensureTwoFactorCodeRequestedFn = origEnsure
		submitTwoFactorCodeFn = origSubmit
		twoFactorStatusWriter = origStatusWriter
	})

	var (
		seen     appleauth.TwoFactorPreference
		loggedIn bool
	)
	webLoginFn = func(ctx context.Context, creds webcore.LoginCredentials) (*webcore.AuthSession, error) {
		loggedIn = true
		return &webcore.AuthSession{}, &webcore.TwoFactorRequiredError{}
	}
	prepareTwoFactorChallengeFn = func(ctx context.Context, session *webcore.AuthSession) (*webcore.TwoFactorChallenge, error) {
		seen = session.TwoFactorPreference()
		return &webcore.TwoFactorChallenge{Method: "phone", Destination: "+1 (•••) •••-••22"}, nil
	}
	ensureTwoFactorCodeRequestedFn = func(ctx context.Context, session *webcore.AuthSession) (*webcore.TwoFactorChallenge, error) {
		return &webcore.TwoFactorChallenge{Method: "phone", Destination: "+1 (•••) •••-••22", Requested: true}, nil
	}
	submitTwoFactorCodeFn = func(ctx context.Context, session *webcore.AuthSession, code string) error {
		return nil
	}
	twoFactorStatusWriter = io.Discard
	return &seen, &loggedIn
}

func readStubTwoFactorCode(context.Context, string) (string, error) {
	return "123456", nil
}

// Regression: key errors retain their stage and diagnostic, and automatic
// sign-in cannot consume an environment PIN even when Apple requires a key.
func TestSecurityKeyLoginPolicyAndFailures(t *testing.T) {
	for _, tc := range []struct {
		name                  string
		explicit              bool
		pin                   string
		automaticDiscovery    bool
		submitError           error
		wantError             string
		wantLogin, wantSubmit int
		wantRejected          bool
	}{
		{name: "explicit login honors PIN", explicit: true, pin: "secret-key-PIN", wantLogin: 1, wantSubmit: 1},
		{name: "automatic without PIN", wantLogin: 1, wantSubmit: 1},
		{name: "automatic configured PIN stops before password", pin: "secret-key-PIN", wantError: "asc web auth login"},
		{name: "discovered key needs usable backend guidance", automaticDiscovery: true, wantLogin: 1, wantError: "security key required by Apple"},
		{name: "discovered key never uses configured PIN", automaticDiscovery: true, pin: "secret-key-PIN", wantLogin: 1, wantError: "asc web auth login"},
		{name: "cancelled", explicit: true, submitError: context.Canceled, wantLogin: 1, wantSubmit: 1, wantError: "2fa verification failed: context canceled"},
		{name: "rejected assertion", explicit: true, submitError: &webcore.SecurityKeyAssertionRejectedError{Status: 401}, wantLogin: 1, wantSubmit: 1, wantRejected: true, wantError: "Apple rejected the security-key assertion"},
		{name: "accepted assertion finalization failed", explicit: true, submitError: &webcore.TwoFactorFinalizationError{Status: 401}, wantLogin: 1, wantSubmit: 1, wantError: "2fa finalization failed"},
		{name: "device failure", explicit: true, submitError: errors.New("device disconnected"), wantLogin: 1, wantSubmit: 1, wantError: "2fa verification failed: device disconnected"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("ASC_TIMEOUT", "30s")
			t.Setenv("ASC_TIMEOUT_SECONDS", "")
			t.Setenv("ASC_WEB_SECURITY_KEY_PIN", tc.pin)
			stubTwoFactorDeliveryLogin(t)
			originalSubmit, originalPromptAvailable := submitSecurityKeyFn, twoFactorPromptAvailableFn
			t.Cleanup(func() { submitSecurityKeyFn, twoFactorPromptAvailableFn = originalSubmit, originalPromptAvailable })
			twoFactorPromptAvailableFn = func() bool { return tc.automaticDiscovery }
			var signInDeadline time.Time
			loginCalls, submitCalls := 0, 0
			webLoginFn = func(ctx context.Context, _ webcore.LoginCredentials) (*webcore.AuthSession, error) {
				loginCalls++
				signInDeadline, _ = ctx.Deadline()
				return &webcore.AuthSession{}, &webcore.TwoFactorRequiredError{}
			}
			prepareTwoFactorChallengeFn = func(context.Context, *webcore.AuthSession) (*webcore.TwoFactorChallenge, error) {
				return &webcore.TwoFactorChallenge{Method: appleauth.TwoFactorMethodSecurityKey}, nil
			}
			submitSecurityKeyFn = func(ctx context.Context, _ *webcore.AuthSession, pin string) error {
				submitCalls++
				wantPIN := ""
				if tc.explicit {
					wantPIN = tc.pin
				}
				if pin != wantPIN {
					t.Fatal("signer received an unauthorized PIN")
				}
				deadline, ok := ctx.Deadline()
				if !ok || deadline.Before(signInDeadline.Add(securitykey.AssertionTimeout)) {
					t.Fatal("security-key sign-in did not reserve the hardware touch budget in addition to the HTTP request budget")
				}
				return tc.submitError
			}
			timedCtx, cancel := shared.ContextWithTimeout(context.Background())
			defer cancel()
			ctx := timedCtx
			if tc.explicit {
				ctx = contextForWebSignIn(ctx)
			}
			preference := appleauth.TwoFactorPreference{Delivery: appleauth.TwoFactorMethodSecurityKey}
			if tc.automaticDiscovery {
				preference = appleauth.TwoFactorPreference{}
			}
			ctx = contextWithTwoFactorPreference(ctx, preference)
			_, err := loginWithOptionalTwoFactor(ctx, "user@example.com", "secret", "", nil, nil)
			if tc.automaticDiscovery && !securitykey.Available() {
				if !errors.Is(err, securitykey.ErrUnavailable) || strings.Contains(err.Error(), "retry with --two-factor-method") {
					t.Fatalf("unavailable backend error = %v, want build guidance without an unusable retry", err)
				}
			} else if tc.wantError == "" {
				if err != nil {
					t.Fatal(err)
				}
			} else if err == nil || !strings.Contains(err.Error(), tc.wantError) {
				t.Fatalf("error = %v, want %q", err, tc.wantError)
			}
			if loginCalls != tc.wantLogin || submitCalls != tc.wantSubmit {
				t.Fatalf("login calls = %d, submit calls = %d; want login %d, submit %d", loginCalls, submitCalls, tc.wantLogin, tc.wantSubmit)
			}
			if tc.submitError != nil && !errors.Is(err, tc.submitError) {
				t.Fatalf("lost original error: %v", err)
			}
			diagnostic, _ := shared.DiagnosticFromError(err)
			if (diagnostic.Code == shared.DiagnosticAuthenticationRejected) != tc.wantRejected {
				t.Fatalf("diagnostic = %+v, want rejected %v", diagnostic, tc.wantRejected)
			}
			if err != nil && (strings.Contains(err.Error(), "verification code") || (tc.pin != "" && strings.Contains(err.Error(), tc.pin))) {
				t.Fatalf("key error has code guidance or leaked PIN: %v", err)
			}
		})
	}
}

// Regression: PIN policy must survive both session resolver paths, while an
// already valid session remains usable with a configured PIN.
func TestResolveSessionSecurityKeyPINPolicy(t *testing.T) {
	for _, tc := range []struct {
		name, cache           string
		explicit              bool
		promptedPassword      bool
		submitError           error
		wantSource            string
		wantLogin, wantSubmit int
	}{
		{name: "explicit fresh", explicit: true, wantSource: "fresh", wantLogin: 1, wantSubmit: 1},
		{name: "explicit expired", explicit: true, cache: "expired", wantSource: "auto-reauth", wantLogin: 1, wantSubmit: 1},
		{name: "automatic fresh"},
		{name: "automatic expired", cache: "expired"},
		{name: "automatic valid cache", cache: "valid", wantSource: "cache"},
		{name: "explicit expired finalization failure", explicit: true, cache: "expired", submitError: &webcore.TwoFactorFinalizationError{Status: 401}, wantLogin: 2, wantSubmit: 2},
		{name: "explicit expired prompted finalization failure", explicit: true, cache: "expired", promptedPassword: true, submitError: &webcore.TwoFactorFinalizationError{Status: 401}, wantLogin: 2, wantSubmit: 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			configureFreshPasswordLoginTest(t)
			t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
			t.Setenv("ASC_WEB_SESSION_CACHE_DIR", t.TempDir())
			t.Setenv("ASC_WEB_SECURITY_KEY_PIN", "secret-key-PIN")
			t.Setenv(webTwoFactorCodeCommandEnv, "")
			t.Setenv(webTwoFactorMethodEnv, "")
			t.Setenv(webTwoFactorPhoneIDEnv, "")
			oldSubmit, oldStatus := submitSecurityKeyFn, twoFactorStatusWriter
			t.Cleanup(func() { submitSecurityKeyFn, twoFactorStatusWriter = oldSubmit, oldStatus })
			var status strings.Builder
			twoFactorStatusWriter = &status
			cached := &webcore.AuthSession{Client: &http.Client{}, UserEmail: "user@example.com"}
			tryResumeSessionFn = func(context.Context, string) (*webcore.AuthSession, bool, error) {
				switch tc.cache {
				case "valid":
					return cached, true, nil
				case "expired":
					return nil, false, webcore.ErrCachedSessionExpired
				default:
					return nil, false, nil
				}
			}
			loadCachedSessionFn = func(string) (*webcore.AuthSession, bool, error) { return cached, true, nil }
			loginCalls, submitCalls := 0, 0
			webLoginFn = func(context.Context, webcore.LoginCredentials) (*webcore.AuthSession, error) {
				loginCalls++
				return cached, &webcore.TwoFactorRequiredError{}
			}
			webLoginWithClientFn = func(ctx context.Context, _ *http.Client, creds webcore.LoginCredentials) (*webcore.AuthSession, error) {
				return webLoginFn(ctx, creds)
			}
			prepareTwoFactorChallengeFn = func(context.Context, *webcore.AuthSession) (*webcore.TwoFactorChallenge, error) {
				return &webcore.TwoFactorChallenge{Method: appleauth.TwoFactorMethodSecurityKey}, nil
			}
			submitSecurityKeyFn = func(_ context.Context, _ *webcore.AuthSession, pin string) error {
				submitCalls++
				if !tc.explicit || pin != "secret-key-PIN" {
					t.Fatal("session resolution used unauthorized or missing PIN")
				}
				return tc.submitError
			}
			timedCtx, cancel := shared.ContextWithTimeout(context.Background())
			defer cancel()
			ctx := timedCtx
			if tc.explicit {
				ctx = contextForWebSignIn(ctx)
			}
			ctx = contextWithTwoFactorPreference(ctx, appleauth.TwoFactorPreference{Delivery: appleauth.TwoFactorMethodSecurityKey})
			password := "secret"
			if tc.promptedPassword {
				password = ""
				promptPasswordFn = func(context.Context) (string, error) { return "secret", nil }
			}
			_, source, err := resolveSession(ctx, "user@example.com", password, "")
			if tc.wantSource != "" {
				if err != nil || source != tc.wantSource {
					t.Fatalf("source = %q, error = %v; want %q", source, err, tc.wantSource)
				}
			} else if tc.submitError != nil {
				var finalization *webcore.TwoFactorFinalizationError
				if !errors.As(err, &finalization) || !strings.Contains(err.Error(), "2fa finalization failed") {
					t.Fatalf("lost finalization error: %v", err)
				}
			} else if err == nil || !strings.Contains(err.Error(), "asc web auth login") {
				t.Fatalf("error = %v, want explicit login guidance", err)
			}
			if loginCalls != tc.wantLogin || submitCalls != tc.wantSubmit {
				t.Fatalf("login calls = %d, submit calls = %d; want %d, %d", loginCalls, submitCalls, tc.wantLogin, tc.wantSubmit)
			}
			if strings.Contains(status.String(), "verification code") || strings.Contains(status.String(), "secret-key-PIN") || (err != nil && strings.Contains(err.Error(), "secret-key-PIN")) {
				t.Fatalf("unexpected key status %q, error %v", status.String(), err)
			}
		})
	}
}

func TestLoginWithOptionalTwoFactorAppliesPreferenceLayeredOverTimeout(t *testing.T) {
	t.Setenv(webTwoFactorMethodEnv, "")
	t.Setenv(webTwoFactorPhoneIDEnv, "")
	seen, _ := stubTwoFactorDeliveryLogin(t)

	// The sign-in flow strips request timeouts, which also drops values that
	// were layered above them; the preference must survive that.
	timedCtx, cancel := shared.ContextWithTimeout(context.Background())
	defer cancel()
	want := appleauth.TwoFactorPreference{Delivery: appleauth.TwoFactorDeliverySMS, PhoneID: 2}
	ctx := contextWithTwoFactorPreference(timedCtx, want)

	if _, err := loginWithOptionalTwoFactor(ctx, "user@example.com", "secret", "", nil, readStubTwoFactorCode, "read-code"); err != nil {
		t.Fatalf("loginWithOptionalTwoFactor returned error: %v", err)
	}
	if *seen != want {
		t.Fatalf("challenge saw preference %+v, want %+v", *seen, want)
	}
}

func TestLoginWithOptionalTwoFactorFallsBackToEnvironmentPreference(t *testing.T) {
	t.Setenv(webTwoFactorMethodEnv, "SMS")
	t.Setenv(webTwoFactorPhoneIDEnv, "7")
	seen, _ := stubTwoFactorDeliveryLogin(t)

	if _, err := loginWithOptionalTwoFactor(context.Background(), "user@example.com", "secret", "", nil, readStubTwoFactorCode, "read-code"); err != nil {
		t.Fatalf("loginWithOptionalTwoFactor returned error: %v", err)
	}
	want := appleauth.TwoFactorPreference{Delivery: appleauth.TwoFactorDeliverySMS, PhoneID: 7}
	if *seen != want {
		t.Fatalf("challenge saw preference %+v, want %+v", *seen, want)
	}
}

func TestLoginWithOptionalTwoFactorRejectsInvalidEnvironmentBeforeSignIn(t *testing.T) {
	t.Setenv(webTwoFactorMethodEnv, "push")
	t.Setenv(webTwoFactorPhoneIDEnv, "")
	_, loggedIn := stubTwoFactorDeliveryLogin(t)

	_, err := loginWithOptionalTwoFactor(context.Background(), "user@example.com", "secret", "", nil, readStubTwoFactorCode, "read-code")
	if err == nil || !strings.Contains(err.Error(), webTwoFactorMethodEnv+" must be one of: auto, sms") {
		t.Fatalf("expected invalid environment error, got %v", err)
	}
	if *loggedIn {
		t.Fatal("expected validation to fail before sending credentials")
	}
}

func TestLoginWithOptionalTwoFactorHintsPhoneIDForAmbiguousSMSPhone(t *testing.T) {
	t.Setenv(webTwoFactorMethodEnv, "")
	t.Setenv(webTwoFactorPhoneIDEnv, "")
	stubTwoFactorDeliveryLogin(t)
	prepareTwoFactorChallengeFn = func(ctx context.Context, session *webcore.AuthSession) (*webcore.TwoFactorChallenge, error) {
		return nil, &appleauth.TrustedPhoneSelectionError{Phones: []appleauth.TrustedPhoneNumber{
			{ID: 1, NumberWithDialCode: "+1 (•••) •••-••11"},
			{ID: 2, NumberWithDialCode: "+1 (•••) •••-••22"},
		}}
	}
	ctx := contextWithTwoFactorPreference(context.Background(), appleauth.TwoFactorPreference{Delivery: appleauth.TwoFactorDeliverySMS})

	_, err := loginWithOptionalTwoFactor(ctx, "user@example.com", "secret", "", nil, readStubTwoFactorCode, "read-code")
	want := "2fa challenge setup failed: multiple trusted phone numbers are available; select one by ID: 1 (+1 (•••) •••-••11), 2 (+1 (•••) •••-••22); pass --two-factor-phone-id or set " + webTwoFactorPhoneIDEnv
	if err == nil || err.Error() != want {
		t.Fatalf("unexpected error:\n got: %v\nwant: %s", err, want)
	}
}

func TestLoginWithOptionalTwoFactorRejectedPhoneCodeGuidanceFollowsDeliveryMode(t *testing.T) {
	tests := []struct {
		mode     string
		wantHint bool
	}{
		{mode: "sms", wantHint: true},
		{mode: "voice", wantHint: false},
	}
	for _, tt := range tests {
		t.Run(tt.mode, func(t *testing.T) {
			t.Setenv(webTwoFactorMethodEnv, "")
			t.Setenv(webTwoFactorPhoneIDEnv, "")
			stubTwoFactorDeliveryLogin(t)
			prepareTwoFactorChallengeFn = func(ctx context.Context, session *webcore.AuthSession) (*webcore.TwoFactorChallenge, error) {
				session.SetPreparedTwoFactorState("phone", 1, tt.mode, "+1 (•••) •••-••11", false)
				return &webcore.TwoFactorChallenge{Method: "phone", Destination: "+1 (•••) •••-••11"}, nil
			}
			submitTwoFactorCodeFn = func(ctx context.Context, session *webcore.AuthSession, code string) error {
				return fmt.Errorf("phone 2fa failed (status 400, codes=[-21669]): %w", webcore.ErrTwoFactorCodeRejected)
			}

			_, err := loginWithOptionalTwoFactor(context.Background(), "user@example.com", "secret", "", nil, readStubTwoFactorCode, "read-code")
			if err == nil {
				t.Fatal("expected rejected code error")
			}
			if !strings.Contains(err.Error(), "2fa verification failed: Apple rejected the verification code") {
				t.Fatalf("expected readable rejection, got %v", err)
			}
			if got := strings.Contains(err.Error(), "latest SMS"); got != tt.wantHint {
				t.Fatalf("SMS hint present = %v, want %v; error = %v", got, tt.wantHint, err)
			}
		})
	}
}

func TestResolveTwoFactorPreference(t *testing.T) {
	type testCase struct {
		name    string
		method  string
		phoneID int
		env     map[string]string
		want    appleauth.TwoFactorPreference
	}
	tests := []testCase{
		{name: "default"},
		{name: "explicit auto", method: "auto"},
		{name: "sms flag", method: "sms", want: appleauth.TwoFactorPreference{Delivery: "sms"}},
		{name: "sms flag with phone", method: " SMS ", phoneID: 3, want: appleauth.TwoFactorPreference{Delivery: "sms", PhoneID: 3}},
		{name: "sms env", env: map[string]string{webTwoFactorMethodEnv: "sms", webTwoFactorPhoneIDEnv: "4"}, want: appleauth.TwoFactorPreference{Delivery: "sms", PhoneID: 4}},
		{name: "flag phone overrides env phone", method: "sms", phoneID: 5, env: map[string]string{webTwoFactorPhoneIDEnv: "4"}, want: appleauth.TwoFactorPreference{Delivery: "sms", PhoneID: 5}},
		{name: "auto flag overrides env sms", method: "auto", env: map[string]string{webTwoFactorMethodEnv: "sms"}},
	}
	if securitykey.Available() {
		tests = append(tests, testCase{name: "security key flag", method: "security-key", want: appleauth.TwoFactorPreference{Delivery: "security-key"}})
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(webTwoFactorMethodEnv, "")
			t.Setenv(webTwoFactorPhoneIDEnv, "")
			for key, value := range tt.env {
				t.Setenv(key, value)
			}
			got, err := resolveTwoFactorPreference(tt.method, tt.phoneID, tt.phoneID != 0)
			if err != nil {
				t.Fatalf("resolveTwoFactorPreference() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("resolveTwoFactorPreference() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestReadTwoFactorCodeFromCommandExportsCodeRequestTime(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses a POSIX shell command")
	}
	requestedAt := time.Date(2026, 10, 17, 21, 15, 43, 123000000, time.UTC)
	ctx := contextWithTwoFactorCodeRequestedAt(context.Background(), requestedAt)

	got, err := readTwoFactorCodeFromCommand(ctx, `printf %s "$`+webTwoFactorCodeRequestedAtEnv+`"`)
	if err != nil {
		t.Fatalf("readTwoFactorCodeFromCommand() error = %v", err)
	}
	if got != "2026-10-17T21:15:43.123Z" {
		t.Fatalf("command saw %s=%q", webTwoFactorCodeRequestedAtEnv, got)
	}
}

func TestLoginWithOptionalTwoFactorPassesCodeRequestTimeToCommand(t *testing.T) {
	t.Setenv(webTwoFactorMethodEnv, "")
	t.Setenv(webTwoFactorPhoneIDEnv, "")
	stubTwoFactorDeliveryLogin(t)
	var requestedSMSAt time.Time
	ensureTwoFactorCodeRequestedFn = func(ctx context.Context, session *webcore.AuthSession) (*webcore.TwoFactorChallenge, error) {
		requestedSMSAt = time.Now()
		return &webcore.TwoFactorChallenge{Method: "phone", Requested: true}, nil
	}
	var (
		seen   time.Time
		seenOK bool
	)
	readCode := func(ctx context.Context, command string) (string, error) {
		seen, seenOK = twoFactorCodeRequestedAt(ctx)
		return "123456", nil
	}

	if _, err := loginWithOptionalTwoFactor(context.Background(), "user@example.com", "secret", "", nil, readCode, "read-code"); err != nil {
		t.Fatalf("loginWithOptionalTwoFactor returned error: %v", err)
	}
	if !seenOK {
		t.Fatal("expected the code command to receive the code request time")
	}
	if seen.After(requestedSMSAt) {
		t.Fatalf("code request time %v is after the SMS request at %v", seen, requestedSMSAt)
	}
}
