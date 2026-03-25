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
