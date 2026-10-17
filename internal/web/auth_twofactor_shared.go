package web

import (
	"context"
	"errors"
	"fmt"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/appleauth"
)

var _ appleauth.SessionState = (*AuthSession)(nil)
var _ appleauth.TwoFactorSelectionState = (*AuthSession)(nil)

func (s *AuthSession) TwoFactorMethod() string {
	if s == nil {
		return ""
	}
	return s.twoFactorMethod
}

func (s *AuthSession) TwoFactorPhoneID() int {
	if s == nil {
		return 0
	}
	return s.twoFactorPhoneID
}

func (s *AuthSession) TwoFactorPhoneMode() string {
	if s == nil {
		return ""
	}
	return s.twoFactorPhoneMode
}

func (s *AuthSession) TwoFactorDestination() string {
	if s == nil {
		return ""
	}
	return s.twoFactorDestination
}

func (s *AuthSession) TwoFactorCodeRequested() bool {
	return s != nil && s.twoFactorCodeRequested
}

func (s *AuthSession) RequestedTwoFactorMethod() string {
	if s == nil {
		return ""
	}
	return s.requestedTwoFactorMethod
}

func (s *AuthSession) RequestedTwoFactorPhoneID() int {
	if s == nil {
		return 0
	}
	return s.requestedTwoFactorPhoneID
}

func (s *AuthSession) SetRequestedTwoFactor(method string, phoneID int) {
	if s == nil {
		return
	}
	s.requestedTwoFactorMethod = method
	s.requestedTwoFactorPhoneID = phoneID
}

func (s *AuthSession) SetSelectPhone(fn func(context.Context, []appleauth.TrustedPhoneNumber) (*appleauth.TrustedPhoneNumber, error)) {
	if s == nil {
		return
	}
	s.selectPhoneFn = fn
}

func (s *AuthSession) SetPreparedTwoFactorState(method string, phoneID int, phoneMode, destination string, requested bool) {
	if s == nil {
		return
	}
	s.twoFactorMethod = method
	s.twoFactorPhoneID = phoneID
	s.twoFactorPhoneMode = phoneMode
	s.twoFactorDestination = destination
	s.twoFactorCodeRequested = requested
}

func (s *AuthSession) SetTwoFactorCodeRequested(requested bool) {
	if s == nil {
		return
	}
	s.twoFactorCodeRequested = requested
}

func wrapWebTwoFactorFlowError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, appleauth.ErrNoTrustedPhoneNumbers) {
		return fmt.Errorf("2fa failed: %w", err)
	}
	var unsupported *appleauth.UnsupportedTwoFactorMethodError
	if errors.As(err, &unsupported) {
		return fmt.Errorf("2fa failed: %w", err)
	}
	var ambiguous *appleauth.AmbiguousTrustedPhoneNumberError
	if errors.As(err, &ambiguous) {
		return fmt.Errorf("2fa failed: %w", err)
	}
	var notFound *appleauth.TrustedPhoneNumberNotFoundError
	if errors.As(err, &notFound) {
		return fmt.Errorf("2fa failed: %w", err)
	}
	return err
}
