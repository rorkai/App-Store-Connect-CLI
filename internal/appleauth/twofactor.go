package appleauth

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const (
	TwoFactorMethodTrustedDevice = "trusted-device"
	TwoFactorMethodPhone         = "phone"
)

var ErrNoTrustedPhoneNumbers = errors.New("no trusted phone numbers available")

type UnsupportedTwoFactorMethodError struct {
	Method string
}

func (e *UnsupportedTwoFactorMethodError) Error() string {
	return fmt.Sprintf("unsupported verification method %q", e.Method)
}

type PhoneCodeRequestedError struct {
	Destination string
	Err         error
}

func (e *PhoneCodeRequestedError) Error() string {
	destination := strings.TrimSpace(e.Destination)
	if destination != "" {
		return fmt.Sprintf("trusted-device verification failed; verification code sent to %s", destination)
	}
	return "trusted-device verification failed; verification code sent to trusted phone number"
}

func (e *PhoneCodeRequestedError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

type MarshalPayloadError struct {
	Err error
}

func (e *MarshalPayloadError) Error() string {
	if e == nil || e.Err == nil {
		return ""
	}
	return e.Err.Error()
}

func (e *MarshalPayloadError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

type TwoFactorChallenge struct {
	Method                 string
	Destination            string
	Requested              bool
	PhoneFallbackAvailable bool
}

// IsPhoneMethod reports whether the challenge uses Apple phone-code delivery.
func (c *TwoFactorChallenge) IsPhoneMethod() bool {
	return c != nil && c.Method == TwoFactorMethodPhone
}

type TrustedPhoneNumber struct {
	ID                 int    `json:"id"`
	PushMode           string `json:"pushMode"`
	NumberWithDialCode string `json:"numberWithDialCode"`
}

type AuthOptions struct {
	NoTrustedDevices    bool
	TrustedPhoneNumbers []TrustedPhoneNumber
}

type AuthOptionsResponse struct {
	NoTrustedDevices    bool                 `json:"noTrustedDevices"`
	TrustedDevices      []map[string]any     `json:"trustedDevices"`
	TrustedPhoneNumbers []TrustedPhoneNumber `json:"trustedPhoneNumbers"`
	SecurityCode        struct {
		Length int `json:"length"`
	} `json:"securityCode"`
}

func (opts *AuthOptionsResponse) AuthOptions() *AuthOptions {
	if opts == nil {
		return &AuthOptions{}
	}
	shared := &AuthOptions{
		NoTrustedDevices: opts.NoTrustedDevices,
	}
	if len(opts.TrustedPhoneNumbers) == 0 {
		return shared
	}
	shared.TrustedPhoneNumbers = append([]TrustedPhoneNumber(nil), opts.TrustedPhoneNumbers...)
	return shared
}

type SessionState interface {
	TwoFactorMethod() string
	TwoFactorPhoneID() int
	TwoFactorPhoneMode() string
	TwoFactorDestination() string
	TwoFactorCodeRequested() bool
	SetPreparedTwoFactorState(method string, phoneID int, phoneMode, destination string, requested bool)
	SetTwoFactorCodeRequested(requested bool)
}

type RequestLogger func(stage string, req *http.Request, resp *http.Response, body []byte, err error)

func ExtractServiceErrorCodes(respBody []byte) []string {
	var payload struct {
		ServiceErrors []struct {
			Code string `json:"code"`
		} `json:"serviceErrors"`
	}
	if err := json.Unmarshal(respBody, &payload); err != nil {
		return nil
	}
	if len(payload.ServiceErrors) == 0 {
		return nil
	}
	codes := make([]string, 0, len(payload.ServiceErrors))
	for _, serviceErr := range payload.ServiceErrors {
		code := strings.TrimSpace(serviceErr.Code)
		if code != "" {
			codes = append(codes, code)
		}
	}
	return codes
}

func IsAppleAccountActionRequiredSigninComplete(status int, respBody []byte) bool {
	if status != http.StatusPreconditionFailed {
		return false
	}
	var payload struct {
		AuthType string `json:"authType"`
	}
	if err := json.Unmarshal(respBody, &payload); err != nil {
		return false
	}
	switch strings.TrimSpace(payload.AuthType) {
	case "sa", "hsa", "non-sa", "hsa2":
		return true
	default:
		return false
	}
}

func PrepareTwoFactorChallenge(ctx context.Context, session SessionState, getAuthOptions func(context.Context) (*AuthOptions, error)) (*TwoFactorChallenge, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if method := session.TwoFactorMethod(); method != "" {
		return &TwoFactorChallenge{
			Method:                 method,
			Destination:            session.TwoFactorDestination(),
			Requested:              session.TwoFactorCodeRequested(),
			PhoneFallbackAvailable: method == TwoFactorMethodTrustedDevice && session.TwoFactorPhoneID() != 0,
		}, nil
	}

	opts, err := getAuthOptions(ctx)
	if err != nil {
		return nil, err
	}

	phoneID := 0
	phoneMode := ""
	destination := ""
	if len(opts.TrustedPhoneNumbers) > 0 {
		phone := opts.TrustedPhoneNumbers[0]
		phoneMode = strings.TrimSpace(phone.PushMode)
		if phoneMode == "" {
			phoneMode = "sms"
		}
		phoneID = phone.ID
		destination = strings.TrimSpace(phone.NumberWithDialCode)
	}

	if opts.NoTrustedDevices {
		if phoneID == 0 {
			return nil, ErrNoTrustedPhoneNumbers
		}
		session.SetPreparedTwoFactorState(TwoFactorMethodPhone, phoneID, phoneMode, destination, false)
		return &TwoFactorChallenge{
			Method:                 TwoFactorMethodPhone,
			Destination:            destination,
			Requested:              false,
			PhoneFallbackAvailable: false,
		}, nil
	}

	session.SetPreparedTwoFactorState(TwoFactorMethodTrustedDevice, phoneID, phoneMode, destination, false)
	return &TwoFactorChallenge{
		Method:                 TwoFactorMethodTrustedDevice,
		Destination:            destination,
		Requested:              false,
		PhoneFallbackAvailable: phoneID != 0,
	}, nil
}

func EnsureTwoFactorCodeRequested(ctx context.Context, session SessionState, getAuthOptions func(context.Context) (*AuthOptions, error), requestPhoneCode func(context.Context, int, string) error) (*TwoFactorChallenge, error) {
	challenge, err := PrepareTwoFactorChallenge(ctx, session, getAuthOptions)
	if err != nil {
		return nil, err
	}
	if challenge.Method != TwoFactorMethodPhone {
		return challenge, nil
	}
	if session.TwoFactorCodeRequested() {
		challenge.Requested = true
		return challenge, nil
	}
	phoneID := session.TwoFactorPhoneID()
	if phoneID == 0 {
		return nil, ErrNoTrustedPhoneNumbers
	}
	if err := requestPhoneCode(ctx, phoneID, session.TwoFactorPhoneMode()); err != nil {
		return nil, err
	}
	session.SetTwoFactorCodeRequested(true)
	challenge.Requested = true
	return challenge, nil
}

func SubmitTwoFactorCode(ctx context.Context, session SessionState, code string, getAuthOptions func(context.Context) (*AuthOptions, error), requestPhoneCode func(context.Context, int, string) error, submitTrustedDeviceCode func(context.Context, string) error, submitPhoneCode func(context.Context, string, int, string) error, finalize func(context.Context) error) error {
	if ctx == nil {
		ctx = context.Background()
	}
	challenge, err := PrepareTwoFactorChallenge(ctx, session, getAuthOptions)
	if err != nil {
		return err
	}

	switch challenge.Method {
	case TwoFactorMethodPhone:
		phoneID := session.TwoFactorPhoneID()
		phoneMode := session.TwoFactorPhoneMode()
		destination := session.TwoFactorDestination()
		if phoneID == 0 {
			return ErrNoTrustedPhoneNumbers
		}
		if !session.TwoFactorCodeRequested() {
			if requestPhoneCode == nil {
				return ErrNoTrustedPhoneNumbers
			}
			if err := requestPhoneCode(ctx, phoneID, phoneMode); err != nil {
				return err
			}
			session.SetPreparedTwoFactorState(TwoFactorMethodPhone, phoneID, phoneMode, destination, true)
		}
		if err := submitPhoneCode(ctx, code, phoneID, phoneMode); err != nil {
			return err
		}
		session.SetPreparedTwoFactorState(TwoFactorMethodPhone, phoneID, phoneMode, destination, true)
		return finalize(ctx)
	case TwoFactorMethodTrustedDevice:
		if err := submitTrustedDeviceCode(ctx, code); err != nil {
			phoneID := session.TwoFactorPhoneID()
			if phoneID == 0 {
				return err
			}
			phoneMode := session.TwoFactorPhoneMode()
			destination := session.TwoFactorDestination()
			if !session.TwoFactorCodeRequested() {
				if requestPhoneCode == nil {
					return err
				}
				if requestErr := requestPhoneCode(ctx, phoneID, phoneMode); requestErr != nil {
					return requestErr
				}
			}
			session.SetPreparedTwoFactorState(TwoFactorMethodPhone, phoneID, phoneMode, destination, true)
			return &PhoneCodeRequestedError{Destination: destination, Err: err}
		}
		return finalize(ctx)
	default:
		return &UnsupportedTwoFactorMethodError{Method: challenge.Method}
	}
}

func DoTwoFactorJSONRequest(ctx context.Context, client *http.Client, headers http.Header, stage, method, requestURL string, payload any, marshalPayload func(any) ([]byte, error), setModifiedCookieHeader func(*http.Request), log RequestLogger) (int, []byte, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	body, err := marshalPayload(payload)
	if err != nil {
		return 0, nil, &MarshalPayloadError{Err: err}
	}

	req, err := http.NewRequestWithContext(ctx, method, requestURL, bytes.NewReader(body))
	if err != nil {
		return 0, nil, err
	}
	for key, values := range headers {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	if setModifiedCookieHeader != nil {
		setModifiedCookieHeader(req)
	}

	resp, err := client.Do(req)
	if err != nil {
		if log != nil {
			log(stage, req, nil, nil, err)
		}
		return 0, nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, _ := io.ReadAll(resp.Body)
	if log != nil {
		log(stage, req, resp, respBody, nil)
	}
	return resp.StatusCode, respBody, nil
}
