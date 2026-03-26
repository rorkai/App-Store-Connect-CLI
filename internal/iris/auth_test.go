package iris

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"math/big"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/1Password/srp"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

type sessionInfoContextKey struct{}

func TestEnsureTwoFactorCodeRequestedRequestsPhoneCodeWhenNoTrustedDevicesHasMultipleNumbers(t *testing.T) {
	session := &AuthSession{
		Client: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				switch {
				case req.Method == http.MethodGet && req.URL.String() == authServiceURL:
					return &http.Response{
						StatusCode: http.StatusOK,
						Header:     make(http.Header),
						Body: io.NopCloser(strings.NewReader(`{
							"noTrustedDevices": true,
							"trustedPhoneNumbers": [
								{"id": 7, "pushMode": "sms", "numberWithDialCode": "+1 (•••) •••-••66"},
								{"id": 9, "pushMode": "sms", "numberWithDialCode": "+1 (•••) •••-••88"}
							]
						}`)),
					}, nil
				case req.Method == http.MethodPut && req.URL.String() == authServiceURL+"/verify/phone":
					var payload struct {
						PhoneNumber struct {
							ID int `json:"id"`
						} `json:"phoneNumber"`
						Mode string `json:"mode"`
					}
					if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
						t.Fatalf("decode phone request payload: %v", err)
					}
					if payload.PhoneNumber.ID != 7 {
						t.Fatalf("expected phone id 7, got %d", payload.PhoneNumber.ID)
					}
					if payload.Mode != "sms" {
						t.Fatalf("expected sms mode, got %q", payload.Mode)
					}
					return &http.Response{
						StatusCode: http.StatusOK,
						Header:     make(http.Header),
						Body:       io.NopCloser(strings.NewReader(`{}`)),
					}, nil
				default:
					t.Fatalf("unexpected request %s %s", req.Method, req.URL.String())
					return nil, nil
				}
			}),
		},
		ServiceKey:       "service-key",
		AppleIDSessionID: "session-id",
		SCNT:             "scnt-token",
	}

	challenge, err := EnsureTwoFactorCodeRequested(context.Background(), session)
	if err != nil {
		t.Fatalf("EnsureTwoFactorCodeRequested returned error: %v", err)
	}
	if challenge.Method != twoFactorMethodPhone {
		t.Fatalf("expected phone method, got %q", challenge.Method)
	}
	if challenge.Destination != "+1 (•••) •••-••66" {
		t.Fatalf("expected first phone destination, got %q", challenge.Destination)
	}
	if !challenge.Requested {
		t.Fatal("expected phone code request for multiple fallback numbers")
	}
	if session.twoFactorPhoneID != 7 {
		t.Fatalf("expected stored phone id 7, got %d", session.twoFactorPhoneID)
	}
}

func TestEnsureTwoFactorCodeRequestedRequestsPhoneCodeForSingleTrustedPhone(t *testing.T) {
	session := &AuthSession{
		Client: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				switch {
				case req.Method == http.MethodGet && req.URL.String() == authServiceURL:
					return &http.Response{
						StatusCode: http.StatusOK,
						Header:     make(http.Header),
						Body: io.NopCloser(strings.NewReader(`{
							"noTrustedDevices": true,
							"trustedPhoneNumbers": [
								{"id": 7, "pushMode": "sms", "numberWithDialCode": "+1 (•••) •••-••66"}
							]
						}`)),
					}, nil
				case req.Method == http.MethodPut && req.URL.String() == authServiceURL+"/verify/phone":
					return &http.Response{
						StatusCode: http.StatusOK,
						Header:     make(http.Header),
						Body:       io.NopCloser(strings.NewReader(`{}`)),
					}, nil
				default:
					t.Fatalf("unexpected request %s %s", req.Method, req.URL.String())
					return nil, nil
				}
			}),
		},
		ServiceKey:       "service-key",
		AppleIDSessionID: "session-id",
		SCNT:             "scnt-token",
	}

	challenge, err := EnsureTwoFactorCodeRequested(context.Background(), session)
	if err != nil {
		t.Fatalf("EnsureTwoFactorCodeRequested returned error: %v", err)
	}
	if challenge.Method != twoFactorMethodPhone {
		t.Fatalf("expected phone method, got %q", challenge.Method)
	}
	if !challenge.Requested {
		t.Fatal("expected single trusted phone to trigger a code request before prompting")
	}
}

func TestRequestPhoneCodeReturnsMarshalError(t *testing.T) {
	sentinel := errors.New("marshal boom")
	previousMarshal := marshalAuthPayload
	marshalAuthPayload = func(v any) ([]byte, error) {
		return nil, sentinel
	}
	t.Cleanup(func() {
		marshalAuthPayload = previousMarshal
	})

	session := &AuthSession{
		Client: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				t.Fatalf("unexpected request %s %s", req.Method, req.URL.String())
				return nil, nil
			}),
		},
	}

	err := requestPhoneCode(context.Background(), session, 7, "sms")
	if !errors.Is(err, sentinel) {
		t.Fatalf("expected marshal error, got %v", err)
	}
	if got := err.Error(); !strings.Contains(got, "failed to marshal phone request payload") {
		t.Fatalf("expected wrapped marshal error, got %q", got)
	}
}

func TestEnsureTwoFactorCodeRequestedHonorsContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	session := &AuthSession{
		Client: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if got := req.Context().Err(); !errors.Is(got, context.Canceled) {
					t.Fatalf("expected canceled request context, got %v", got)
				}
				return nil, req.Context().Err()
			}),
		},
		ServiceKey:       "service-key",
		AppleIDSessionID: "session-id",
		SCNT:             "scnt-token",
	}

	if _, err := EnsureTwoFactorCodeRequested(ctx, session); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context cancellation, got %v", err)
	}
}

func TestPrepareTwoFactorChallengeKeepsPhoneFallbackForTrustedDeviceFlow(t *testing.T) {
	session := &AuthSession{
		Client: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if req.Method != http.MethodGet || req.URL.String() != authServiceURL {
					t.Fatalf("unexpected request %s %s", req.Method, req.URL.String())
				}
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body: io.NopCloser(strings.NewReader(`{
						"trustedDevices": [{"id":"device-1"}],
						"trustedPhoneNumbers": [
							{"id": 7, "pushMode": "sms", "numberWithDialCode": "+1 (•••) •••-••66"}
						]
					}`)),
				}, nil
			}),
		},
		ServiceKey:       "service-key",
		AppleIDSessionID: "session-id",
		SCNT:             "scnt-token",
	}

	challenge, err := PrepareTwoFactorChallenge(context.Background(), session)
	if err != nil {
		t.Fatalf("PrepareTwoFactorChallenge returned error: %v", err)
	}
	if challenge.Method != twoFactorMethodTrustedDevice {
		t.Fatalf("expected trusted-device method, got %q", challenge.Method)
	}
	if !challenge.PhoneFallbackAvailable {
		t.Fatal("expected phone fallback to remain available")
	}
	if session.twoFactorPhoneID != 7 {
		t.Fatalf("expected stored fallback phone id 7, got %d", session.twoFactorPhoneID)
	}
	if challenge.Destination != "+1 (•••) •••-••66" {
		t.Fatalf("expected fallback destination on initial trusted-device challenge, got %q", challenge.Destination)
	}

	cachedChallenge, err := PrepareTwoFactorChallenge(context.Background(), session)
	if err != nil {
		t.Fatalf("PrepareTwoFactorChallenge (cached) returned error: %v", err)
	}
	if cachedChallenge.Destination != challenge.Destination {
		t.Fatalf("expected cached challenge destination %q, got %q", challenge.Destination, cachedChallenge.Destination)
	}
}

func TestFinalizeTwoFactorPassesContextToSessionInfo(t *testing.T) {
	ctx, cancel := context.WithCancel(context.WithValue(context.Background(), sessionInfoContextKey{}, "marker"))
	defer cancel()

	timer := time.AfterFunc(10*time.Millisecond, cancel)
	defer timer.Stop()

	session := &AuthSession{
		Client: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				switch req.URL.String() {
				case authServiceURL + "/2sv/trust":
					return &http.Response{
						StatusCode: http.StatusOK,
						Header:     make(http.Header),
						Body:       io.NopCloser(strings.NewReader(`{}`)),
					}, nil
				case olympusSessionURL:
					if req.Context().Value(sessionInfoContextKey{}) != "marker" {
						return nil, errors.New("missing caller context on session info request")
					}
					<-req.Context().Done()
					return nil, req.Context().Err()
				default:
					t.Fatalf("unexpected request %s %s", req.Method, req.URL.String())
					return nil, nil
				}
			}),
		},
		ServiceKey:       "service-key",
		AppleIDSessionID: "session-id",
		SCNT:             "scnt-token",
	}

	err := finalizeTwoFactor(ctx, session)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected finalizeTwoFactor to return context cancellation, got %v", err)
	}
}

func TestTwoFactorChallengeIsPhoneMethod(t *testing.T) {
	if challenge := (*TwoFactorChallenge)(nil); challenge.IsPhoneMethod() {
		t.Fatal("expected nil challenge not to report phone method")
	}
	if challenge := (&TwoFactorChallenge{Method: twoFactorMethodPhone}); !challenge.IsPhoneMethod() {
		t.Fatal("expected phone challenge to report phone method")
	}
	if challenge := (&TwoFactorChallenge{Method: twoFactorMethodTrustedDevice}); challenge.IsPhoneMethod() {
		t.Fatal("expected trusted-device challenge not to report phone method")
	}
}

func TestSubmitTrustedDeviceCodeReturnsMarshalError(t *testing.T) {
	sentinel := errors.New("marshal boom")
	previousMarshal := marshalAuthPayload
	marshalAuthPayload = func(v any) ([]byte, error) {
		return nil, sentinel
	}
	t.Cleanup(func() {
		marshalAuthPayload = previousMarshal
	})

	session := &AuthSession{
		Client: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				t.Fatalf("unexpected request %s %s", req.Method, req.URL.String())
				return nil, nil
			}),
		},
	}

	err := submitTrustedDeviceCode(context.Background(), session, "123456")
	if !errors.Is(err, sentinel) {
		t.Fatalf("expected marshal error, got %v", err)
	}
	if got := err.Error(); !strings.Contains(got, "failed to marshal trusted-device payload") {
		t.Fatalf("expected wrapped marshal error, got %q", got)
	}
}

func TestSubmitPhoneCodeReturnsMarshalError(t *testing.T) {
	sentinel := errors.New("marshal boom")
	previousMarshal := marshalAuthPayload
	marshalAuthPayload = func(v any) ([]byte, error) {
		return nil, sentinel
	}
	t.Cleanup(func() {
		marshalAuthPayload = previousMarshal
	})

	session := &AuthSession{
		Client: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				t.Fatalf("unexpected request %s %s", req.Method, req.URL.String())
				return nil, nil
			}),
		},
	}

	err := submitPhoneCode(context.Background(), session, "123456", 7, "sms")
	if !errors.Is(err, sentinel) {
		t.Fatalf("expected marshal error, got %v", err)
	}
	if got := err.Error(); !strings.Contains(got, "failed to marshal phone payload") {
		t.Fatalf("expected wrapped marshal error, got %q", got)
	}
}

func TestSubmitTwoFactorCodeUsesPreparedPhoneFlow(t *testing.T) {
	session := &AuthSession{
		Client: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				switch {
				case req.Method == http.MethodPut && req.URL.String() == authServiceURL+"/verify/phone":
					var payload struct {
						PhoneNumber struct {
							ID int `json:"id"`
						} `json:"phoneNumber"`
					}
					if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
						t.Fatalf("decode phone delivery payload: %v", err)
					}
					if payload.PhoneNumber.ID != 7 {
						t.Fatalf("expected delivery phone id 7, got %d", payload.PhoneNumber.ID)
					}
					return &http.Response{
						StatusCode: http.StatusNoContent,
						Header:     make(http.Header),
						Body:       io.NopCloser(strings.NewReader("")),
					}, nil
				case req.Method == http.MethodPost && req.URL.String() == authServiceURL+"/verify/phone/securitycode":
					var payload struct {
						PhoneNumber struct {
							ID int `json:"id"`
						} `json:"phoneNumber"`
						SecurityCode struct {
							Code string `json:"code"`
						} `json:"securityCode"`
						Mode string `json:"mode"`
					}
					if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
						t.Fatalf("decode phone verification payload: %v", err)
					}
					if payload.PhoneNumber.ID != 7 {
						t.Fatalf("expected phone id 7, got %d", payload.PhoneNumber.ID)
					}
					if payload.SecurityCode.Code != "123456" {
						t.Fatalf("expected 2fa code 123456, got %q", payload.SecurityCode.Code)
					}
					if payload.Mode != "sms" {
						t.Fatalf("expected sms mode, got %q", payload.Mode)
					}
					return &http.Response{
						StatusCode: http.StatusOK,
						Header:     make(http.Header),
						Body:       io.NopCloser(strings.NewReader(`{}`)),
					}, nil
				case req.Method == http.MethodGet && req.URL.String() == authServiceURL+"/2sv/trust":
					return &http.Response{
						StatusCode: http.StatusOK,
						Header:     make(http.Header),
						Body:       io.NopCloser(strings.NewReader(`{}`)),
					}, nil
				case req.Method == http.MethodGet && req.URL.String() == olympusSessionURL:
					return &http.Response{
						StatusCode: http.StatusOK,
						Header:     make(http.Header),
						Body: io.NopCloser(strings.NewReader(`{
							"provider": {"providerId": 123},
							"user": {"emailAddress": "user@example.com"}
						}`)),
					}, nil
				default:
					t.Fatalf("unexpected request %s %s", req.Method, req.URL.String())
					return nil, nil
				}
			}),
		},
		ServiceKey:           "service-key",
		AppleIDSessionID:     "session-id",
		SCNT:                 "scnt-token",
		twoFactorMethod:      twoFactorMethodPhone,
		twoFactorPhoneID:     7,
		twoFactorPhoneMode:   "sms",
		twoFactorDestination: "+1 (•••) •••-••66",
	}

	if err := SubmitTwoFactorCode(context.Background(), session, "123456"); err != nil {
		t.Fatalf("SubmitTwoFactorCode returned error: %v", err)
	}
	if session.ProviderID != 123 {
		t.Fatalf("expected provider id 123, got %d", session.ProviderID)
	}
	if session.UserEmail != "user@example.com" {
		t.Fatalf("expected user email to be refreshed, got %q", session.UserEmail)
	}
}

func TestSubmitTwoFactorCodeSubmitsPreparedPhoneFallbackAfterTrustedDeviceFailure(t *testing.T) {
	session := &AuthSession{
		Client: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				switch {
				case req.Method == http.MethodPost && req.URL.String() == authServiceURL+"/verify/trusteddevice/securitycode":
					return &http.Response{
						StatusCode: http.StatusBadRequest,
						Header:     make(http.Header),
						Body:       io.NopCloser(strings.NewReader(`{"serviceErrors":[{"code":"-21669"}]}`)),
					}, nil
				case req.Method == http.MethodPost && req.URL.String() == authServiceURL+"/verify/phone/securitycode":
					var payload struct {
						PhoneNumber struct {
							ID int `json:"id"`
						} `json:"phoneNumber"`
						SecurityCode struct {
							Code string `json:"code"`
						} `json:"securityCode"`
					}
					if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
						t.Fatalf("decode phone verification payload: %v", err)
					}
					if payload.PhoneNumber.ID != 7 {
						t.Fatalf("expected fallback phone id 7, got %d", payload.PhoneNumber.ID)
					}
					if payload.SecurityCode.Code != "123456" {
						t.Fatalf("expected 2fa code 123456, got %q", payload.SecurityCode.Code)
					}
					return &http.Response{
						StatusCode: http.StatusOK,
						Header:     make(http.Header),
						Body:       io.NopCloser(strings.NewReader(`{}`)),
					}, nil
				case req.Method == http.MethodGet && req.URL.String() == authServiceURL+"/2sv/trust":
					return &http.Response{
						StatusCode: http.StatusOK,
						Header:     make(http.Header),
						Body:       io.NopCloser(strings.NewReader(`{}`)),
					}, nil
				case req.Method == http.MethodGet && req.URL.String() == olympusSessionURL:
					return &http.Response{
						StatusCode: http.StatusOK,
						Header:     make(http.Header),
						Body: io.NopCloser(strings.NewReader(`{
							"provider": {"providerId": 123},
							"user": {"emailAddress": "user@example.com"}
						}`)),
					}, nil
				default:
					t.Fatalf("unexpected request %s %s", req.Method, req.URL.String())
					return nil, nil
				}
			}),
		},
		ServiceKey:             "service-key",
		AppleIDSessionID:       "session-id",
		SCNT:                   "scnt-token",
		twoFactorMethod:        twoFactorMethodPhone,
		twoFactorPhoneID:       7,
		twoFactorPhoneMode:     "sms",
		twoFactorCodeRequested: true,
	}

	if err := SubmitTwoFactorCode(context.Background(), session, "123456"); err != nil {
		t.Fatalf("SubmitTwoFactorCode returned error: %v", err)
	}
}

func TestSubmitTwoFactorCodeRequestsPhoneDeliveryBeforeVerification(t *testing.T) {
	session := &AuthSession{
		Client: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				switch {
				case req.Method == http.MethodGet && req.URL.String() == authServiceURL:
					return &http.Response{
						StatusCode: http.StatusOK,
						Header:     make(http.Header),
						Body: io.NopCloser(strings.NewReader(`{
							"noTrustedDevices": true,
							"trustedPhoneNumbers": [
								{"id": 7, "pushMode": "sms", "numberWithDialCode": "+1 (•••) •••-••66"},
								{"id": 9, "pushMode": "sms", "numberWithDialCode": "+1 (•••) •••-••88"}
							]
						}`)),
					}, nil
				case req.Method == http.MethodPut && req.URL.String() == authServiceURL+"/verify/phone":
					var payload struct {
						PhoneNumber struct {
							ID int `json:"id"`
						} `json:"phoneNumber"`
					}
					if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
						t.Fatalf("decode phone delivery payload: %v", err)
					}
					if payload.PhoneNumber.ID != 7 {
						t.Fatalf("expected delivery phone id 7, got %d", payload.PhoneNumber.ID)
					}
					return &http.Response{
						StatusCode: http.StatusNoContent,
						Header:     make(http.Header),
						Body:       io.NopCloser(strings.NewReader("")),
					}, nil
				case req.Method == http.MethodPost && req.URL.String() == authServiceURL+"/verify/phone/securitycode":
					var payload struct {
						PhoneNumber struct {
							ID int `json:"id"`
						} `json:"phoneNumber"`
						SecurityCode struct {
							Code string `json:"code"`
						} `json:"securityCode"`
					}
					if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
						t.Fatalf("decode phone verification payload: %v", err)
					}
					if payload.PhoneNumber.ID != 7 {
						t.Fatalf("expected phone id 7, got %d", payload.PhoneNumber.ID)
					}
					if payload.SecurityCode.Code != "123456" {
						t.Fatalf("expected 2fa code 123456, got %q", payload.SecurityCode.Code)
					}
					return &http.Response{
						StatusCode: http.StatusOK,
						Header:     make(http.Header),
						Body:       io.NopCloser(strings.NewReader(`{}`)),
					}, nil
				case req.Method == http.MethodGet && req.URL.String() == authServiceURL+"/2sv/trust":
					return &http.Response{
						StatusCode: http.StatusOK,
						Header:     make(http.Header),
						Body:       io.NopCloser(strings.NewReader(`{}`)),
					}, nil
				case req.Method == http.MethodGet && req.URL.String() == olympusSessionURL:
					return &http.Response{
						StatusCode: http.StatusOK,
						Header:     make(http.Header),
						Body: io.NopCloser(strings.NewReader(`{
							"provider": {"providerId": 123},
							"user": {"emailAddress": "user@example.com"}
						}`)),
					}, nil
				default:
					t.Fatalf("unexpected request %s %s", req.Method, req.URL.String())
					return nil, nil
				}
			}),
		},
		ServiceKey:       "service-key",
		AppleIDSessionID: "session-id",
		SCNT:             "scnt-token",
	}

	if err := SubmitTwoFactorCode(context.Background(), session, "123456"); err != nil {
		t.Fatalf("SubmitTwoFactorCode returned error: %v", err)
	}
}

func TestSubmitTwoFactorCodeHonorsContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	session := &AuthSession{
		Client: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if got := req.Context().Err(); !errors.Is(got, context.Canceled) {
					t.Fatalf("expected canceled request context, got %v", got)
				}
				return nil, req.Context().Err()
			}),
		},
		ServiceKey:       "service-key",
		AppleIDSessionID: "session-id",
		SCNT:             "scnt-token",
	}

	if err := SubmitTwoFactorCode(ctx, session, "123456"); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context cancellation, got %v", err)
	}
}

func TestSubmitTwoFactorCodeHonorsContextCancellationDuringFinalize(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	session := &AuthSession{
		Client: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				switch {
				case req.Method == http.MethodPost && req.URL.String() == authServiceURL+"/verify/trusteddevice/securitycode":
					cancel()
					return &http.Response{
						StatusCode: http.StatusNoContent,
						Header:     make(http.Header),
						Body:       io.NopCloser(strings.NewReader("")),
					}, nil
				case req.Method == http.MethodGet && req.URL.String() == authServiceURL+"/2sv/trust":
					if got := req.Context().Err(); !errors.Is(got, context.Canceled) {
						t.Fatalf("expected canceled finalize request context, got %v", got)
					}
					return nil, req.Context().Err()
				default:
					t.Fatalf("unexpected request %s %s", req.Method, req.URL.String())
					return nil, nil
				}
			}),
		},
		ServiceKey:       "service-key",
		AppleIDSessionID: "session-id",
		SCNT:             "scnt-token",
		twoFactorMethod:  twoFactorMethodTrustedDevice,
	}

	if err := SubmitTwoFactorCode(ctx, session, "123456"); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected finalize cancellation, got %v", err)
	}
}

func TestSigninCompleteReturnsAppleAccountActionRequiredForPreconditionFailed(t *testing.T) {
	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusPreconditionFailed,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"authType":"hsa2"}`)),
			}, nil
		}),
	}

	err := signinComplete(client, "user@example.com", "m1-proof", "m2-proof", `{"v":1}`, "service-key", "hashcash-token")
	if !errors.Is(err, errAppleAccountActionRequired) {
		t.Fatalf("expected account-action-required error, got %v", err)
	}
}

func TestGetHashcashAllowsMissingChallengeHeaders(t *testing.T) {
	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if got, want := req.URL.String(), authServiceURL+"/signin?widgetKey=service-key"; got != want {
				t.Fatalf("expected request URL %q, got %q", want, got)
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader("")),
			}, nil
		}),
	}

	hashcash, err := getHashcash(client, "service-key")
	if err != nil {
		t.Fatalf("expected missing-hashcash headers to be tolerated, got %v", err)
	}
	if hashcash != "" {
		t.Fatalf("expected empty hashcash when headers are absent, got %q", hashcash)
	}
}

func TestPreparePasswordForProtocol(t *testing.T) {
	t.Run("s2k", func(t *testing.T) {
		prepared, err := preparePasswordForProtocol("example", "s2k")
		if err != nil {
			t.Fatalf("preparePasswordForProtocol returned error: %v", err)
		}
		if len(prepared) != 32 {
			t.Fatalf("expected 32-byte digest for s2k, got %d", len(prepared))
		}
	})

	t.Run("s2k_fo", func(t *testing.T) {
		prepared, err := preparePasswordForProtocol("example", "s2k_fo")
		if err != nil {
			t.Fatalf("preparePasswordForProtocol returned error: %v", err)
		}
		if len(prepared) != 64 {
			t.Fatalf("expected 64-byte hex digest for s2k_fo, got %d", len(prepared))
		}
		if _, err := hex.DecodeString(string(prepared)); err != nil {
			t.Fatalf("expected valid hex digest for s2k_fo: %v", err)
		}
	})

	t.Run("unsupported protocol", func(t *testing.T) {
		if _, err := preparePasswordForProtocol("example", "unknown"); err == nil {
			t.Fatal("expected error for unsupported protocol")
		}
	})
}

func TestMakeHashcash(t *testing.T) {
	now := time.Date(2026, 2, 24, 18, 0, 0, 0, time.UTC)
	hashcash := makeHashcash(10, "4d74fb15eb23f465f1f6fcbf534e5877", now)

	parts := strings.Split(hashcash, ":")
	if len(parts) != 6 {
		t.Fatalf("expected 6 hashcash fields, got %d (%q)", len(parts), hashcash)
	}
	if parts[0] != "1" {
		t.Fatalf("unexpected hashcash version: %q", parts[0])
	}
	if parts[1] != "10" {
		t.Fatalf("unexpected bits field: %q", parts[1])
	}
	if parts[2] != "20260224180000" {
		t.Fatalf("unexpected date field: %q", parts[2])
	}
	if parts[3] != "4d74fb15eb23f465f1f6fcbf534e5877" {
		t.Fatalf("unexpected challenge field: %q", parts[3])
	}
	if parts[4] != "" {
		t.Fatalf("expected empty extension field, got %q", parts[4])
	}

	sum := sha1.Sum([]byte(hashcash))
	if !hasLeadingZeroBits(sum[:], 10) {
		t.Fatalf("hashcash does not satisfy required leading-zero bits: %q", hashcash)
	}
}

func TestCalculateSRPProof_MatchesFastlaneSIRPFixture(t *testing.T) {
	// Generated from fastlane-sirp 1.0.0 (public SRP test vector):
	// user=user@example.com, protocol=s2k_fo, iterations=1000, salt=0a, server B=02.
	//
	// These values are not credentials; they're deterministic fixtures.
	// Keep them as byte slices to avoid secret-scanner false positives on long,
	// high-entropy string literals.
	aPublicBytes := []byte{
		0x23, 0xe0, 0x7e, 0xb6, 0xab, 0xd9, 0x7e, 0x06, 0x6a, 0x98, 0x13, 0x33,
		0x66, 0x61, 0xf4, 0x6c, 0xd6, 0x8f, 0x85, 0xf3, 0x21, 0xe0, 0x74, 0x68,
		0x88, 0xb2, 0x42, 0x1b, 0xed, 0x99, 0x74, 0x56, 0x0c, 0xb3, 0xa8, 0x8d,
		0x70, 0x73, 0x3d, 0x1d, 0x6e, 0x87, 0x8e, 0x95, 0x8e, 0x8b, 0x53, 0x5c,
		0x16, 0x01, 0x2b, 0x8e, 0x5b, 0x68, 0xf2, 0x82, 0xb9, 0xda, 0x1f, 0xd6,
		0x98, 0xa1, 0x03, 0xd8, 0x5d, 0x27, 0xf1, 0x7c, 0x0f, 0x6b, 0x21, 0xf1,
		0x02, 0xc1, 0x90, 0xdd, 0xc1, 0x41, 0x27, 0x1d, 0xa1, 0x8e, 0x1b, 0x09,
		0xe0, 0x1f, 0x6b, 0xf5, 0xf1, 0xe3, 0x30, 0xe6, 0x64, 0x3c, 0x78, 0xeb,
		0x38, 0xf0, 0x8a, 0xb7, 0x32, 0x62, 0xe1, 0xb2, 0x38, 0xd9, 0x8a, 0x41,
		0x1a, 0x9b, 0x32, 0xf9, 0xdf, 0xaa, 0xa8, 0xfa, 0xe7, 0xef, 0xef, 0x4e,
		0x73, 0x4b, 0xa9, 0x4e, 0xfb, 0x74, 0x7d, 0x30, 0x59, 0x57, 0x85, 0x95,
		0xf6, 0xa7, 0x89, 0x00, 0x22, 0x95, 0x63, 0x2e, 0xbd, 0xdd, 0x56, 0x68,
		0x02, 0x11, 0x58, 0x3c, 0xbc, 0x25, 0x97, 0x0d, 0x9b, 0xae, 0x67, 0x29,
		0xb5, 0x8d, 0x53, 0x6f, 0x0e, 0x05, 0x4d, 0xd6, 0xfa, 0xc2, 0x9a, 0xce,
		0xc1, 0x0f, 0xbc, 0xfd, 0xae, 0xfc, 0x96, 0x10, 0x30, 0x62, 0x1f, 0xc8,
		0xfb, 0xfe, 0xa6, 0xf0, 0xc5, 0xcd, 0x0a, 0x1e, 0x95, 0x2a, 0xcc, 0xfc,
		0xdd, 0xf1, 0x14, 0x88, 0xa1, 0x06, 0x32, 0x5b, 0xf4, 0x9c, 0x87, 0x8f,
		0x9d, 0x8c, 0xf9, 0x19, 0x8d, 0x3e, 0x35, 0x32, 0xa6, 0x02, 0xcc, 0xd0,
		0x93, 0x36, 0x7f, 0xec, 0x5a, 0xd7, 0x22, 0x52, 0x72, 0x47, 0xd1, 0x60,
		0xc8, 0x16, 0xd9, 0xc9, 0x38, 0x05, 0x67, 0x8c, 0xc2, 0x35, 0x79, 0xb8,
		0xfc, 0x13, 0xf2, 0x64, 0x38, 0x89, 0x73, 0x1e, 0xf4, 0x66, 0xe9, 0x4a,
		0xde, 0x02, 0x31, 0xfd,
	}

	aSecretBytes := []byte{
		0xfb, 0x08, 0x98, 0x04, 0x6d, 0x81, 0x72, 0x31, 0x8d, 0x83, 0xae, 0xbc,
		0x9b, 0x29, 0x1d, 0x52, 0x59, 0xdb, 0x83, 0x89, 0x02, 0x2c, 0xa8, 0x1b,
		0x07, 0x69, 0xd7, 0xc2, 0x7f, 0xea, 0x11, 0x9a, 0xaf, 0x74, 0x8d, 0x31,
		0x92, 0xd5, 0xed, 0x5e, 0x87, 0xea, 0x07, 0x20, 0x63, 0x02, 0x75, 0x25,
		0x3c, 0xc4, 0xda, 0x4c, 0x29, 0xdf, 0xd6, 0x60, 0xa7, 0xde, 0xb6, 0xda,
		0x03, 0x80, 0xf4, 0xb7, 0xf5, 0xb7, 0x7c, 0xe1, 0x21, 0xb7, 0xe4, 0x2b,
		0x0b, 0x8d, 0xb8, 0xd5, 0x2b, 0x99, 0xef, 0xf9, 0xf1, 0x5b, 0x34, 0x65,
		0xb8, 0xbe, 0xed, 0xb0, 0x84, 0x5a, 0x25, 0x90, 0xb3, 0xbd, 0xa8, 0x52,
		0xcf, 0x96, 0xcd, 0x85, 0x36, 0x32, 0xcd, 0x76, 0xeb, 0xa7, 0x9a, 0x0e,
		0xd0, 0xdb, 0xe7, 0xfb, 0x38, 0xde, 0x63, 0x86, 0x00, 0x84, 0x1c, 0x59,
		0x47, 0xfa, 0xfe, 0x0f, 0x3f, 0xa3, 0x06, 0x97, 0x57, 0x6b, 0x14, 0x8c,
		0x0d, 0xa0, 0xfc, 0x87, 0x29, 0xe2, 0x8f, 0x5f, 0x4b, 0x9a, 0xbe, 0x7b,
		0xff, 0x2e, 0xbf, 0x34, 0x1b, 0x91, 0xa5, 0x4f, 0x0d, 0x8b, 0x78, 0x7b,
		0xfd, 0x42, 0x63, 0xff, 0xc4, 0x8a, 0x47, 0x86, 0xce, 0x7f, 0x21, 0x36,
		0x0f, 0x85, 0x09, 0xcb, 0x60, 0x51, 0xd1, 0x15, 0x54, 0xaf, 0x4f, 0xb9,
		0x9b, 0x74, 0xa1, 0x77, 0xe7, 0x38, 0xb8, 0xeb, 0x48, 0xa0, 0x7c, 0xbe,
		0x99, 0x32, 0x02, 0x0b, 0x62, 0x53, 0xea, 0x61, 0xf2, 0x8f, 0x0e, 0xeb,
		0xb6, 0x4f, 0x81, 0xb9, 0xd9, 0x9b, 0x05, 0x0a, 0x53, 0xd3, 0xc6, 0x67,
		0xea, 0x6c, 0x01, 0x16, 0xe3, 0xaa, 0x99, 0x5d, 0xb8, 0x68, 0x56, 0x4b,
		0x61, 0x8a, 0xd4, 0xcd, 0xa0, 0xcf, 0x3f, 0x78, 0xfe, 0xcd, 0xe1, 0xb9,
		0x12, 0xc1, 0x1e, 0xbe, 0x54, 0x1d, 0xdc, 0x6e, 0xf8, 0x39, 0xbe, 0x63,
		0x87, 0x6c, 0x28, 0xd7,
	}

	salt := []byte{0x0a}
	serverB := []byte{0x02}

	derivedPassword := []byte{
		0x06, 0x26, 0x3e, 0xf8, 0x37, 0xed, 0xf6, 0x2d, 0xa9, 0xb9, 0xe5, 0xe3,
		0xd6, 0x9d, 0xaf, 0x2d, 0xf1, 0x1e, 0xd1, 0xed, 0x6f, 0x16, 0x04, 0x18,
		0x16, 0x58, 0xb6, 0x3f, 0x16, 0xd1, 0x93, 0x07,
	}
	expectedM1 := []byte{
		0xfd, 0xd8, 0x86, 0xd5, 0x2f, 0x02, 0x8f, 0x7d, 0x20, 0xb5, 0x80, 0xc8,
		0x9e, 0x79, 0x48, 0xb5, 0x32, 0xee, 0x4a, 0xe8, 0x4e, 0x51, 0xef, 0x4e,
		0x49, 0x3e, 0x63, 0x67, 0x67, 0x27, 0x0d, 0x39,
	}
	expectedM2 := []byte{
		0xdc, 0xd5, 0x6d, 0x31, 0x2e, 0x7b, 0x04, 0xae, 0x67, 0x62, 0xa7, 0x58,
		0x78, 0xf2, 0x3e, 0x5c, 0x2e, 0x1c, 0x3c, 0x7d, 0x4f, 0xbd, 0xa3, 0x42,
		0x50, 0x7f, 0xd0, 0x98, 0x2b, 0x56, 0x55, 0xe1,
	}

	aPublic := new(big.Int).SetBytes(aPublicBytes)
	aSecret := new(big.Int).SetBytes(aSecretBytes)

	group := srp.KnownGroups[srp.RFC5054Group2048]
	m1B64, m2B64, err := calculateSRPProof(
		"user@example.com",
		aSecret,
		aPublic,
		group.N(),
		group.Generator(),
		serverB,
		derivedPassword,
		salt,
	)
	if err != nil {
		t.Fatalf("calculateSRPProof returned error: %v", err)
	}

	m1Bytes, err := base64.StdEncoding.DecodeString(m1B64)
	if err != nil {
		t.Fatalf("failed to decode m1 base64: %v", err)
	}
	m2Bytes, err := base64.StdEncoding.DecodeString(m2B64)
	if err != nil {
		t.Fatalf("failed to decode m2 base64: %v", err)
	}

	if !bytes.Equal(m1Bytes, expectedM1) {
		t.Fatalf("m1 mismatch\nexpected: %s\ngot:      %s", hex.EncodeToString(expectedM1), hex.EncodeToString(m1Bytes))
	}
	if !bytes.Equal(m2Bytes, expectedM2) {
		t.Fatalf("m2 mismatch\nexpected: %s\ngot:      %s", hex.EncodeToString(expectedM2), hex.EncodeToString(m2Bytes))
	}
}
