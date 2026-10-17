package appleauth

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

type stubSessionState struct {
	method      string
	phoneID     int
	phoneMode   string
	destination string
	requested   bool
}

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

type stubSelectionSessionState struct {
	stubSessionState
	requestedMethod  string
	requestedPhoneID int
}

func (s *stubSelectionSessionState) RequestedTwoFactorMethod() string { return s.requestedMethod }
func (s *stubSelectionSessionState) RequestedTwoFactorPhoneID() int { return s.requestedPhoneID }

func TestPrepareTwoFactorChallengeExplicitSMS(t *testing.T) {
	session := &stubSelectionSessionState{}
	challenge, err := PrepareTwoFactorChallengeWithOptions(
		context.Background(),
		session,
		func(context.Context) (*AuthOptions, error) {
			return &AuthOptions{
				NoTrustedDevices: false,
				TrustedPhoneNumbers: []TrustedPhoneNumber{
					{ID: 1, PushMode: "sms", NumberWithDialCode: "+1 (•••) •••-••11"},
				},
			}, nil
		},
		TwoFactorOptions{Method: "sms"},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if challenge.Method != TwoFactorMethodPhone {
		t.Fatalf("expected method %q, got %q", TwoFactorMethodPhone, challenge.Method)
	}
	if !challenge.IsPhoneMethod() {
		t.Fatal("expected IsPhoneMethod() to be true")
	}
	if challenge.PhoneFallbackAvailable {
		t.Fatal("expected PhoneFallbackAvailable to be false when phone already selected")
	}
	if challenge.Destination != "+1 (•••) •••-••11" {
		t.Fatalf("expected destination +1 (•••) •••-••11, got %q", challenge.Destination)
	}
}

func TestPrepareTwoFactorChallengeExplicitPhoneID(t *testing.T) {
	session := &stubSelectionSessionState{}
	challenge, err := PrepareTwoFactorChallengeWithOptions(
		context.Background(),
		session,
		func(context.Context) (*AuthOptions, error) {
			return &AuthOptions{
				TrustedPhoneNumbers: []TrustedPhoneNumber{
					{ID: 1, PushMode: "sms", NumberWithDialCode: "+1 (•••) •••-••11"},
					{ID: 2, PushMode: "sms", NumberWithDialCode: "+1 (•••) •••-••22"},
				},
			}, nil
		},
		TwoFactorOptions{Method: "sms", PhoneID: 2},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if challenge.Destination != "+1 (•••) •••-••22" {
		t.Fatalf("expected destination for phone 2, got %q", challenge.Destination)
	}
	if session.phoneID != 2 {
		t.Fatalf("expected session phoneID 2, got %d", session.phoneID)
	}
}

func TestPrepareTwoFactorChallengeAmbiguousPhoneError(t *testing.T) {
	session := &stubSelectionSessionState{}
	_, err := PrepareTwoFactorChallengeWithOptions(
		context.Background(),
		session,
		func(context.Context) (*AuthOptions, error) {
			return &AuthOptions{
				TrustedPhoneNumbers: []TrustedPhoneNumber{
					{ID: 1, PushMode: "sms", NumberWithDialCode: "+1 (•••) •••-••11"},
					{ID: 2, PushMode: "sms", NumberWithDialCode: "+1 (•••) •••-••22"},
				},
			}, nil
		},
		TwoFactorOptions{Method: "sms"},
	)
	if err == nil {
		t.Fatal("expected error for ambiguous phones without selector")
	}
	var ambiguous *AmbiguousTrustedPhoneNumberError
	if !errors.As(err, &ambiguous) {
		t.Fatalf("expected AmbiguousTrustedPhoneNumberError, got %T: %v", err, err)
	}
	if len(ambiguous.Available) != 2 {
		t.Fatalf("expected 2 available phones, got %d", len(ambiguous.Available))
	}
}

func TestPrepareTwoFactorChallengeSelectPhoneCallback(t *testing.T) {
	session := &stubSelectionSessionState{}
	called := false
	challenge, err := PrepareTwoFactorChallengeWithOptions(
		context.Background(),
		session,
		func(context.Context) (*AuthOptions, error) {
			return &AuthOptions{
				TrustedPhoneNumbers: []TrustedPhoneNumber{
					{ID: 1, PushMode: "sms", NumberWithDialCode: "+1 (•••) •••-••11"},
					{ID: 2, PushMode: "sms", NumberWithDialCode: "+1 (•••) •••-••22"},
				},
			}, nil
		},
		TwoFactorOptions{
			Method: "sms",
			SelectPhone: func(ctx context.Context, phones []TrustedPhoneNumber) (*TrustedPhoneNumber, error) {
				called = true
				return &phones[1], nil
			},
		},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatal("expected SelectPhone callback to be invoked")
	}
	if challenge.Destination != "+1 (•••) •••-••22" {
		t.Fatalf("expected destination for phone 2, got %q", challenge.Destination)
	}
}

func TestPrepareTwoFactorChallengeUnknownPhoneIDError(t *testing.T) {
	session := &stubSelectionSessionState{}
	_, err := PrepareTwoFactorChallengeWithOptions(
		context.Background(),
		session,
		func(context.Context) (*AuthOptions, error) {
			return &AuthOptions{
				TrustedPhoneNumbers: []TrustedPhoneNumber{
					{ID: 1, PushMode: "sms", NumberWithDialCode: "+1 (•••) •••-••11"},
				},
			}, nil
		},
		TwoFactorOptions{Method: "sms", PhoneID: 99},
	)
	if err == nil {
		t.Fatal("expected error for unknown phone ID")
	}
	var notFound *TrustedPhoneNumberNotFoundError
	if !errors.As(err, &notFound) {
		t.Fatalf("expected TrustedPhoneNumberNotFoundError, got %T: %v", err, err)
	}
	if notFound.ID != 99 {
		t.Fatalf("expected ID 99, got %d", notFound.ID)
	}
}

func TestPrepareTwoFactorChallengeNoTrustedPhoneNumbers(t *testing.T) {
	session := &stubSelectionSessionState{}
	_, err := PrepareTwoFactorChallengeWithOptions(
		context.Background(),
		session,
		func(context.Context) (*AuthOptions, error) {
			return &AuthOptions{
				TrustedPhoneNumbers: nil,
			}, nil
		},
		TwoFactorOptions{Method: "sms"},
	)
	if !errors.Is(err, ErrNoTrustedPhoneNumbers) {
		t.Fatalf("expected ErrNoTrustedPhoneNumbers, got %v", err)
	}
}

func TestPrepareTwoFactorChallengeUnsupportedMethod(t *testing.T) {
	session := &stubSelectionSessionState{}
	_, err := PrepareTwoFactorChallengeWithOptions(
		context.Background(),
		session,
		func(context.Context) (*AuthOptions, error) {
			return &AuthOptions{}, nil
		},
		TwoFactorOptions{Method: "email"},
	)
	if err == nil {
		t.Fatal("expected error for unsupported method")
	}
	var unsupported *UnsupportedTwoFactorMethodError
	if !errors.As(err, &unsupported) {
		t.Fatalf("expected UnsupportedTwoFactorMethodError, got %T: %v", err, err)
	}
	if unsupported.Method != "email" {
		t.Fatalf("expected method email, got %q", unsupported.Method)
	}
}
