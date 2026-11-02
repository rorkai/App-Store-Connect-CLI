package appleauth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
)

type stubSessionState struct {
	method      string
	phoneID     int
	phoneMode   string
	destination string
	requested   bool
	preference  TwoFactorPreference
}

func (s *stubSessionState) TwoFactorPreference() TwoFactorPreference { return s.preference }

func (s *stubSessionState) TwoFactorMethod() string { return s.method }

func (s *stubSessionState) TwoFactorPhoneID() int { return s.phoneID }

func (s *stubSessionState) TwoFactorPhoneMode() string { return s.phoneMode }

func (s *stubSessionState) TwoFactorDestination() string { return s.destination }

func (s *stubSessionState) TwoFactorCodeRequested() bool { return s.requested }

func (s *stubSessionState) SetPreparedTwoFactorState(method string, phoneID int, phoneMode, destination string, requested bool) {
	s.method = method
	s.phoneID = phoneID
	s.phoneMode = phoneMode
	s.destination = destination
	s.requested = requested
}

func (s *stubSessionState) SetTwoFactorCodeRequested(requested bool) {
	s.requested = requested
}

func TestAuthOptionsResponseAuthOptionsCopiesTrustedPhoneNumbers(t *testing.T) {
	var resp AuthOptionsResponse
	if err := json.Unmarshal([]byte(`{
		"noTrustedDevices": true,
		"trustedPhoneNumbers": [
			{"id": 7, "pushMode": "sms", "numberWithDialCode": "+1 (•••) •••-••66"}
		]
	}`), &resp); err != nil {
		t.Fatalf("unmarshal auth options response: %v", err)
	}

	got := resp.AuthOptions()
	if got == nil {
		t.Fatal("expected auth options response conversion result")
		return
	}
	if !got.NoTrustedDevices {
		t.Fatal("expected noTrustedDevices to be preserved")
	}
	if len(got.TrustedPhoneNumbers) != 1 {
		t.Fatalf("expected one trusted phone number, got %d", len(got.TrustedPhoneNumbers))
	}
	if got.TrustedPhoneNumbers[0].ID != 7 {
		t.Fatalf("expected trusted phone id 7, got %d", got.TrustedPhoneNumbers[0].ID)
	}

	got.TrustedPhoneNumbers[0].ID = 99
	if resp.TrustedPhoneNumbers[0].ID != 7 {
		t.Fatalf("expected trusted phone slice to be copied, got %d", resp.TrustedPhoneNumbers[0].ID)
	}
}

func TestNilAuthOptionsResponseReturnsEmptyOptions(t *testing.T) {
	var resp *AuthOptionsResponse
	got := resp.AuthOptions()
	if got == nil {
		t.Fatal("expected empty auth options for nil response")
		return
	}
	if got.NoTrustedDevices {
		t.Fatal("did not expect noTrustedDevices on nil response")
	}
	if len(got.TrustedPhoneNumbers) != 0 {
		t.Fatalf("expected no trusted phone numbers, got %d", len(got.TrustedPhoneNumbers))
	}
}

func TestPrepareTwoFactorChallengeRoutesSecurityKeys(t *testing.T) {
	for _, tt := range []struct {
		name             string
		delivery         string
		fsa              bool
		noTrustedDevices bool
		wantMethod       string
		wantError        string
	}{
		{name: "FSA overrides code metadata", fsa: true, wantMethod: TwoFactorMethodSecurityKey},
		{name: "explicit security key", delivery: TwoFactorMethodSecurityKey, fsa: true, wantMethod: TwoFactorMethodSecurityKey},
		{name: "FSA preserves explicit SMS", delivery: TwoFactorDeliverySMS, fsa: true, wantMethod: TwoFactorMethodPhone},
		{name: "FSA preserves automatic phone routing without trusted devices", fsa: true, noTrustedDevices: true, wantMethod: TwoFactorMethodPhone},
		{name: "FSA preserves explicit SMS without trusted devices", delivery: TwoFactorDeliverySMS, fsa: true, noTrustedDevices: true, wantMethod: TwoFactorMethodPhone},
		{name: "explicit key refuses phone-priority auth options", delivery: TwoFactorMethodSecurityKey, fsa: true, noTrustedDevices: true, wantError: "security-key verification is unavailable because Apple auth options prioritize phone verification; use auto or sms"},
		{name: "key mode refuses code-only challenge", delivery: TwoFactorMethodSecurityKey, wantError: "security-key challenge not offered by Apple for this account"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			payload := fmt.Sprintf(`{"noTrustedDevices":%t,"trustedPhoneNumbers":[{"id":7,"pushMode":"sms","numberWithDialCode":"+1 (•••) •••-••66"}]`, tt.noTrustedDevices)
			if !tt.noTrustedDevices {
				payload += `,"trustedDevices":[{"id":"device"}]`
			}
			if tt.fsa {
				payload += `,"fsaChallenge":{"challenge":"AQID","keyHandles":["BAUG"],"allowedCredentials":"BAUG"}`
			}
			payload += `}`
			var response AuthOptionsResponse
			if err := json.Unmarshal([]byte(payload), &response); err != nil {
				t.Fatal(err)
			}
			session := &stubSessionState{preference: TwoFactorPreference{Delivery: tt.delivery}}
			challenge, err := PrepareTwoFactorChallenge(context.Background(), session, func(context.Context) (*AuthOptions, error) {
				return response.AuthOptions(), nil
			})
			if tt.wantError != "" {
				if err == nil || err.Error() != tt.wantError {
					t.Fatalf("challenge error = %v, want %q", err, tt.wantError)
				}
				if challenge != nil || session.method != "" || session.phoneID != 0 || session.phoneMode != "" || session.destination != "" || session.requested {
					t.Fatal("rejected challenge prepared code delivery")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if challenge.Method != tt.wantMethod {
				t.Fatalf("challenge = %+v, want method %q", challenge, tt.wantMethod)
			}
			if tt.wantMethod == TwoFactorMethodPhone {
				if challenge.SecurityKey != nil || challenge.Destination != "+1 (•••) •••-••66" || session.method != TwoFactorMethodPhone || session.phoneID != 7 || session.phoneMode != TwoFactorDeliverySMS || session.destination != challenge.Destination {
					t.Fatalf("phone challenge = %+v, session = %+v, want prepared SMS delivery to phone 7", challenge, session)
				}
			} else if challenge.SecurityKey == nil || challenge.SecurityKey.Challenge != "AQID" || challenge.SecurityKey.AllowedCredentials != "BAUG" {
				t.Fatalf("security-key continuation lost: %+v", challenge.SecurityKey)
			}
			if challenge.PhoneFallbackAvailable || challenge.Requested {
				t.Fatalf("challenge unexpectedly requested code delivery or offered fallback: %+v", challenge)
			}
		})
	}
}

// Regression: only a usable challenge may replace the code flow, so an
// unexpected fsaChallenge cannot break ordinary sign-in, including explicit SMS.
func TestPrepareTwoFactorChallengeIgnoresUnusableFSAChallenge(t *testing.T) {
	for _, fsa := range []string{
		`null`, `{}`, `""`, `[]`, `false`,
		`{"challenge":123}`,
		`{"allowedCredentials":[]}`,
		`{"challenge":"AQID"}`,
		`{"allowedCredentials":"BAUG"}`,
		`{"challenge":null,"allowedCredentials":""}`,
	} {
		for _, tt := range []struct{ delivery, wantMethod string }{
			{wantMethod: TwoFactorMethodTrustedDevice},
			{delivery: TwoFactorDeliverySMS, wantMethod: TwoFactorMethodPhone},
		} {
			t.Run(fsa+"/"+tt.wantMethod, func(t *testing.T) {
				payload := fmt.Sprintf(`{"trustedDevices":[{"id":"device"}],"trustedPhoneNumbers":[{"id":7,"pushMode":"sms","numberWithDialCode":"+1 (•••) •••-••66"}],"fsaChallenge":%s}`, fsa)
				var response AuthOptionsResponse
				if err := json.Unmarshal([]byte(payload), &response); err != nil {
					t.Fatal(err)
				}
				session := &stubSessionState{preference: TwoFactorPreference{Delivery: tt.delivery}}
				challenge, err := PrepareTwoFactorChallenge(context.Background(), session, func(context.Context) (*AuthOptions, error) {
					return response.AuthOptions(), nil
				})
				if err != nil {
					t.Fatal(err)
				}
				if challenge.Method != tt.wantMethod || challenge.SecurityKey != nil {
					t.Fatalf("challenge = %+v, want code method %q", challenge, tt.wantMethod)
				}
			})
		}
	}
}

func TestSubmitTwoFactorCodeRequestsPhoneFallbackBeforeRetry(t *testing.T) {
	session := &stubSessionState{
		method:      TwoFactorMethodTrustedDevice,
		phoneID:     7,
		phoneMode:   "sms",
		destination: "+1 (•••) •••-••66",
	}

	trustedDeviceErr := errors.New("trusted-device rejected code")
	requestedPhoneCode := false
	submittedPhoneCode := false
	finalized := false

	err := SubmitTwoFactorCode(
		context.Background(),
		session,
		"123456",
		func(context.Context) (*AuthOptions, error) {
			t.Fatal("did not expect auth options lookup for prepared trusted-device session")
			return nil, nil
		},
		func(ctx context.Context, phoneID int, mode string) error {
			requestedPhoneCode = true
			if phoneID != 7 {
				t.Fatalf("expected fallback phone id 7, got %d", phoneID)
			}
			if mode != "sms" {
				t.Fatalf("expected fallback phone mode %q, got %q", "sms", mode)
			}
			return nil
		},
		func(ctx context.Context, code string) error {
			if code != "123456" {
				t.Fatalf("expected trusted-device code %q, got %q", "123456", code)
			}
			return trustedDeviceErr
		},
		func(context.Context, string, int, string) error {
			submittedPhoneCode = true
			return nil
		},
		func(context.Context) error {
			finalized = true
			return nil
		},
	)
	var requestedErr *PhoneCodeRequestedError
	if !errors.As(err, &requestedErr) {
		t.Fatalf("expected phone-code-requested error, got %v", err)
	}
	if !errors.Is(err, trustedDeviceErr) {
		t.Fatalf("expected trusted-device error to be preserved, got %v", err)
	}
	if requestedErr.Destination != "+1 (•••) •••-••66" {
		t.Fatalf("expected destination %q, got %q", "+1 (•••) •••-••66", requestedErr.Destination)
	}
	if !requestedPhoneCode {
		t.Fatal("expected fallback phone delivery request")
	}
	if submittedPhoneCode {
		t.Fatal("did not expect phone verification submission before a new code is entered")
	}
	if finalized {
		t.Fatal("did not expect finalize after requesting fallback phone delivery")
	}
	if session.method != TwoFactorMethodPhone {
		t.Fatalf("expected session method to switch to phone, got %q", session.method)
	}
	if !session.requested {
		t.Fatal("expected session to remember that phone delivery was requested")
	}
}

func TestSubmitTwoFactorCodeReturnsRetryWhenTrustedDeviceFallbackWasAlreadyRequested(t *testing.T) {
	session := &stubSessionState{
		method:      TwoFactorMethodTrustedDevice,
		phoneID:     7,
		phoneMode:   "sms",
		destination: "+1 (•••) •••-••66",
		requested:   true,
	}

	trustedDeviceErr := errors.New("trusted-device rejected code")
	requestedPhoneCode := false
	submittedPhoneCode := false
	finalized := false

	err := SubmitTwoFactorCode(
		context.Background(),
		session,
		"123456",
		func(context.Context) (*AuthOptions, error) {
			t.Fatal("did not expect auth options lookup for prepared trusted-device session")
			return nil, nil
		},
		func(context.Context, int, string) error {
			requestedPhoneCode = true
			return nil
		},
		func(ctx context.Context, code string) error {
			if code != "123456" {
				t.Fatalf("expected trusted-device code %q, got %q", "123456", code)
			}
			return trustedDeviceErr
		},
		func(context.Context, string, int, string) error {
			submittedPhoneCode = true
			return nil
		},
		func(context.Context) error {
			finalized = true
			return nil
		},
	)
	var requestedErr *PhoneCodeRequestedError
	if !errors.As(err, &requestedErr) {
		t.Fatalf("expected phone-code-requested error, got %v", err)
	}
	if !errors.Is(err, trustedDeviceErr) {
		t.Fatalf("expected trusted-device error to be preserved, got %v", err)
	}
	if requestedPhoneCode {
		t.Fatal("did not expect duplicate phone delivery request when fallback was already requested")
	}
	if submittedPhoneCode {
		t.Fatal("did not expect stale phone verification submission before a new code is entered")
	}
	if finalized {
		t.Fatal("did not expect finalize after retry signal")
	}
	if session.method != TwoFactorMethodPhone {
		t.Fatalf("expected session method to switch to phone, got %q", session.method)
	}
	if !session.requested {
		t.Fatal("expected session to keep requested phone-delivery state")
	}
}

func TestSubmitTwoFactorCodeRequestsPhoneDeliveryBeforePreparedPhoneVerification(t *testing.T) {
	session := &stubSessionState{
		method:      TwoFactorMethodPhone,
		phoneID:     7,
		phoneMode:   "sms",
		destination: "+1 (•••) •••-••66",
	}

	requestedPhoneCode := false
	submittedPhoneCode := false
	finalized := false

	err := SubmitTwoFactorCode(
		context.Background(),
		session,
		"123456",
		func(context.Context) (*AuthOptions, error) {
			t.Fatal("did not expect auth options lookup for prepared phone session")
			return nil, nil
		},
		func(ctx context.Context, phoneID int, mode string) error {
			requestedPhoneCode = true
			if phoneID != 7 {
				t.Fatalf("expected phone id 7, got %d", phoneID)
			}
			if mode != "sms" {
				t.Fatalf("expected phone mode %q, got %q", "sms", mode)
			}
			return nil
		},
		func(context.Context, string) error {
			t.Fatal("did not expect trusted-device submission for phone flow")
			return nil
		},
		func(ctx context.Context, code string, phoneID int, mode string) error {
			submittedPhoneCode = true
			if code != "123456" {
				t.Fatalf("expected phone code %q, got %q", "123456", code)
			}
			if phoneID != 7 {
				t.Fatalf("expected phone id 7, got %d", phoneID)
			}
			if mode != "sms" {
				t.Fatalf("expected phone mode %q, got %q", "sms", mode)
			}
			return nil
		},
		func(context.Context) error {
			finalized = true
			return nil
		},
	)
	if err != nil {
		t.Fatalf("expected successful phone verification, got %v", err)
	}
	if !requestedPhoneCode {
		t.Fatal("expected phone delivery request before verification")
	}
	if !submittedPhoneCode {
		t.Fatal("expected phone verification submission")
	}
	if !finalized {
		t.Fatal("expected finalize after phone verification")
	}
	if session.method != TwoFactorMethodPhone {
		t.Fatalf("expected session method to remain phone, got %q", session.method)
	}
	if !session.requested {
		t.Fatal("expected session to remember that phone delivery was requested")
	}
}

func TestSubmitTwoFactorCodeRejectsPreparedPhoneFlowWithoutPhoneID(t *testing.T) {
	session := &stubSessionState{
		method:    TwoFactorMethodPhone,
		phoneMode: "sms",
		requested: true,
	}

	requestedPhoneCode := false
	submittedPhoneCode := false
	finalized := false

	err := SubmitTwoFactorCode(
		context.Background(),
		session,
		"123456",
		func(context.Context) (*AuthOptions, error) {
			t.Fatal("did not expect auth options lookup for prepared phone session")
			return nil, nil
		},
		func(context.Context, int, string) error {
			requestedPhoneCode = true
			return nil
		},
		func(context.Context, string) error {
			t.Fatal("did not expect trusted-device submission for phone flow")
			return nil
		},
		func(context.Context, string, int, string) error {
			submittedPhoneCode = true
			return nil
		},
		func(context.Context) error {
			finalized = true
			return nil
		},
	)
	if !errors.Is(err, ErrNoTrustedPhoneNumbers) {
		t.Fatalf("expected ErrNoTrustedPhoneNumbers, got %v", err)
	}
	if requestedPhoneCode {
		t.Fatal("did not expect duplicate phone delivery request")
	}
	if submittedPhoneCode {
		t.Fatal("did not expect malformed phone verification submission without phone id")
	}
	if finalized {
		t.Fatal("did not expect finalize after rejected phone submission")
	}
}

func TestSubmitTwoFactorCodeUsesVoicePushModeFromAuthOptions(t *testing.T) {
	session := &stubSessionState{}
	requestedPhoneCode := false
	submittedPhoneCode := false
	finalized := false

	err := SubmitTwoFactorCode(
		context.Background(),
		session,
		"123456",
		func(context.Context) (*AuthOptions, error) {
			return &AuthOptions{
				NoTrustedDevices: true,
				TrustedPhoneNumbers: []TrustedPhoneNumber{
					{ID: 7, PushMode: "voice", NumberWithDialCode: "+1 (•••) •••-••66"},
				},
			}, nil
		},
		func(ctx context.Context, phoneID int, mode string) error {
			requestedPhoneCode = true
			if phoneID != 7 {
				t.Fatalf("expected phone id 7, got %d", phoneID)
			}
			if mode != "voice" {
				t.Fatalf("expected phone request mode voice, got %q", mode)
			}
			return nil
		},
		func(context.Context, string) error {
			t.Fatal("did not expect trusted-device submission for phone-only flow")
			return nil
		},
		func(ctx context.Context, code string, phoneID int, mode string) error {
			submittedPhoneCode = true
			if code != "123456" {
				t.Fatalf("expected 2fa code 123456, got %q", code)
			}
			if phoneID != 7 {
				t.Fatalf("expected phone id 7, got %d", phoneID)
			}
			if mode != "voice" {
				t.Fatalf("expected phone verification mode voice, got %q", mode)
			}
			return nil
		},
		func(context.Context) error {
			finalized = true
			return nil
		},
	)
	if err != nil {
		t.Fatalf("expected successful phone verification, got %v", err)
	}
	if !requestedPhoneCode {
		t.Fatal("expected phone delivery request")
	}
	if !submittedPhoneCode {
		t.Fatal("expected phone verification submission")
	}
	if !finalized {
		t.Fatal("expected finalize after phone verification")
	}
	if session.phoneMode != "voice" {
		t.Fatalf("expected session to remember voice mode, got %q", session.phoneMode)
	}
}

func smsPreferenceAuthOptions(phones ...TrustedPhoneNumber) func(context.Context) (*AuthOptions, error) {
	return func(context.Context) (*AuthOptions, error) {
		return &AuthOptions{TrustedPhoneNumbers: phones}, nil
	}
}

func TestPrepareTwoFactorChallengePrefersSMSWhenTrustedDevicesAreAvailable(t *testing.T) {
	session := &stubSessionState{preference: TwoFactorPreference{Delivery: TwoFactorDeliverySMS}}

	challenge, err := PrepareTwoFactorChallenge(context.Background(), session, smsPreferenceAuthOptions(
		TrustedPhoneNumber{ID: 4, PushMode: "voice", NumberWithDialCode: "+1 (•••) •••-••44"},
	))
	if err != nil {
		t.Fatalf("PrepareTwoFactorChallenge() error = %v", err)
	}
	if challenge.Method != TwoFactorMethodPhone {
		t.Fatalf("expected phone method, got %q", challenge.Method)
	}
	if challenge.PhoneFallbackAvailable || challenge.Requested {
		t.Fatalf("expected an unrequested direct phone challenge, got %+v", challenge)
	}
	if challenge.Destination != "+1 (•••) •••-••44" {
		t.Fatalf("expected destination of the trusted phone, got %q", challenge.Destination)
	}
	if session.phoneID != 4 || session.phoneMode != "sms" {
		t.Fatalf("expected phone 4 in sms mode, got id=%d mode=%q", session.phoneID, session.phoneMode)
	}
}

func TestPrepareTwoFactorChallengeSelectsRequestedSMSPhone(t *testing.T) {
	session := &stubSessionState{preference: TwoFactorPreference{Delivery: TwoFactorDeliverySMS, PhoneID: 2}}

	challenge, err := PrepareTwoFactorChallenge(context.Background(), session, smsPreferenceAuthOptions(
		TrustedPhoneNumber{ID: 1, PushMode: "sms", NumberWithDialCode: "+1 (•••) •••-••11"},
		TrustedPhoneNumber{ID: 2, PushMode: "sms", NumberWithDialCode: "+44 ••••• •••222"},
	))
	if err != nil {
		t.Fatalf("PrepareTwoFactorChallenge() error = %v", err)
	}
	if session.phoneID != 2 || challenge.Destination != "+44 ••••• •••222" {
		t.Fatalf("expected phone 2, got id=%d destination=%q", session.phoneID, challenge.Destination)
	}
}

func TestPrepareTwoFactorChallengeRejectsAmbiguousSMSPhone(t *testing.T) {
	session := &stubSessionState{preference: TwoFactorPreference{Delivery: TwoFactorDeliverySMS}}

	_, err := PrepareTwoFactorChallenge(context.Background(), session, smsPreferenceAuthOptions(
		TrustedPhoneNumber{ID: 1, NumberWithDialCode: "+1 (•••) •••-••11"},
		TrustedPhoneNumber{ID: 2, NumberWithDialCode: "+44 ••••• •••222"},
	))
	var selectionErr *TrustedPhoneSelectionError
	if !errors.As(err, &selectionErr) {
		t.Fatalf("expected TrustedPhoneSelectionError, got %v", err)
	}
	want := "multiple trusted phone numbers are available; select one by ID: 1 (+1 (•••) •••-••11), 2 (+44 ••••• •••222)"
	if err.Error() != want {
		t.Fatalf("unexpected error:\n got: %s\nwant: %s", err.Error(), want)
	}
	if session.method != "" {
		t.Fatalf("expected no prepared state after selection failure, got %q", session.method)
	}
}

func TestPrepareTwoFactorChallengeRejectsUnknownSMSPhoneID(t *testing.T) {
	session := &stubSessionState{preference: TwoFactorPreference{Delivery: TwoFactorDeliverySMS, PhoneID: 9}}

	_, err := PrepareTwoFactorChallenge(context.Background(), session, smsPreferenceAuthOptions(
		TrustedPhoneNumber{ID: 1, NumberWithDialCode: "+1 (•••) •••-••11"},
	))
	var selectionErr *TrustedPhoneSelectionError
	if !errors.As(err, &selectionErr) {
		t.Fatalf("expected TrustedPhoneSelectionError, got %v", err)
	}
	want := "trusted phone number ID 9 is not available; select one by ID: 1 (+1 (•••) •••-••11)"
	if err.Error() != want {
		t.Fatalf("unexpected error:\n got: %s\nwant: %s", err.Error(), want)
	}
}

func TestPrepareTwoFactorChallengeRejectsSMSWithoutTrustedPhones(t *testing.T) {
	session := &stubSessionState{preference: TwoFactorPreference{Delivery: TwoFactorDeliverySMS}}

	_, err := PrepareTwoFactorChallenge(context.Background(), session, smsPreferenceAuthOptions())
	if !errors.Is(err, ErrNoTrustedPhoneNumbers) {
		t.Fatalf("expected ErrNoTrustedPhoneNumbers, got %v", err)
	}
	if err.Error() != "sms verification is unavailable: no trusted phone numbers available" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestEnsureTwoFactorCodeRequestedRequestsSMSForPreferredPhone(t *testing.T) {
	session := &stubSessionState{preference: TwoFactorPreference{Delivery: TwoFactorDeliverySMS}}
	var requests []string

	challenge, err := EnsureTwoFactorCodeRequested(context.Background(), session, smsPreferenceAuthOptions(
		TrustedPhoneNumber{ID: 3, PushMode: "voice", NumberWithDialCode: "+1 (•••) •••-••33"},
	), func(_ context.Context, phoneID int, mode string) error {
		requests = append(requests, fmt.Sprintf("%d:%s", phoneID, mode))
		return nil
	})
	if err != nil {
		t.Fatalf("EnsureTwoFactorCodeRequested() error = %v", err)
	}
	if !challenge.Requested {
		t.Fatal("expected the SMS code to be requested")
	}
	if len(requests) != 1 || requests[0] != "3:sms" {
		t.Fatalf("expected one sms request for phone 3, got %v", requests)
	}
}
