package web

import (
	"context"
	"flag"
	"os"
	"strconv"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/appleauth"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/securitykey"
)

const (
	webTwoFactorMethodEnv  = "ASC_WEB_2FA_METHOD"
	webTwoFactorPhoneIDEnv = "ASC_WEB_2FA_PHONE_ID"
	twoFactorMethodAuto    = "auto"
)

// twoFactorDeliveryFlags selects how Apple delivers the verification code.
type twoFactorDeliveryFlags struct {
	flagSet *flag.FlagSet
	method  *string
	phoneID *int
}

func bindTwoFactorDeliveryFlags(fs *flag.FlagSet) twoFactorDeliveryFlags {
	return twoFactorDeliveryFlags{
		flagSet: fs,
		method:  fs.String("two-factor-method", "", "2FA method: auto, sms, or security-key (requires a macOS fido2 build); defaults to "+webTwoFactorMethodEnv+", then auto"),
		phoneID: fs.Int("two-factor-phone-id", 0, "Trusted phone number ID for --two-factor-method sms when the account has several; defaults to "+webTwoFactorPhoneIDEnv),
	}
}

// resolve merges the flags with their environment fallbacks and validates the
// result before any sign-in side effect.
func (f twoFactorDeliveryFlags) resolve() (appleauth.TwoFactorPreference, error) {
	method := ""
	if f.method != nil {
		method = *f.method
	}
	phoneID := 0
	if f.phoneID != nil {
		phoneID = *f.phoneID
	}
	// An explicit --two-factor-phone-id 0 must fail rather than fall back to
	// the environment or the only trusted number.
	phoneIDSet := false
	if f.flagSet != nil {
		f.flagSet.Visit(func(fl *flag.Flag) {
			if fl.Name == "two-factor-phone-id" {
				phoneIDSet = true
			}
		})
	}
	return resolveTwoFactorPreference(method, phoneID, phoneIDSet)
}

func resolveTwoFactorPreference(methodFlag string, phoneIDFlag int, phoneIDFlagSet bool) (appleauth.TwoFactorPreference, error) {
	method := strings.ToLower(strings.TrimSpace(methodFlag))
	methodSource := "--two-factor-method"
	if method == "" {
		method = strings.ToLower(strings.TrimSpace(os.Getenv(webTwoFactorMethodEnv)))
		methodSource = webTwoFactorMethodEnv
	}
	switch method {
	case "", twoFactorMethodAuto, appleauth.TwoFactorDeliverySMS, appleauth.TwoFactorMethodSecurityKey:
	default:
		return appleauth.TwoFactorPreference{}, shared.UsageError(methodSource + " must be one of: auto, sms, security-key")
	}

	phoneID := phoneIDFlag
	phoneIDSource := "--two-factor-phone-id"
	if phoneID < 0 || (phoneIDFlagSet && phoneID == 0) {
		return appleauth.TwoFactorPreference{}, shared.UsageError(phoneIDSource + " must be a positive trusted phone number ID")
	}
	if phoneID == 0 {
		if value := strings.TrimSpace(os.Getenv(webTwoFactorPhoneIDEnv)); value != "" {
			phoneIDSource = webTwoFactorPhoneIDEnv
			parsed, err := strconv.Atoi(value)
			if err != nil || parsed <= 0 {
				return appleauth.TwoFactorPreference{}, shared.UsageError(phoneIDSource + " must be a positive trusted phone number ID")
			}
			phoneID = parsed
		}
	}

	if method != appleauth.TwoFactorDeliverySMS {
		if phoneID != 0 {
			return appleauth.TwoFactorPreference{}, shared.UsageError(phoneIDSource + " requires --two-factor-method sms")
		}
		if method == appleauth.TwoFactorMethodSecurityKey {
			if !securitykey.Available() {
				return appleauth.TwoFactorPreference{}, shared.UsageError(securitykey.ErrUnavailable.Error())
			}
			return appleauth.TwoFactorPreference{Delivery: method}, nil
		}
		return appleauth.TwoFactorPreference{}, nil
	}
	return appleauth.TwoFactorPreference{Delivery: appleauth.TwoFactorDeliverySMS, PhoneID: phoneID}, nil
}

type twoFactorPreferenceContextKey struct{}

// contextWithTwoFactorPreference carries a validated delivery preference from
// command flags to the sign-in flow.
func contextWithTwoFactorPreference(ctx context.Context, preference appleauth.TwoFactorPreference) context.Context {
	return context.WithValue(ctx, twoFactorPreferenceContextKey{}, preference)
}

// twoFactorPreferenceForSignIn returns the flag-resolved preference, falling
// back to the environment for callers without delivery flags.
func twoFactorPreferenceForSignIn(ctx context.Context) (appleauth.TwoFactorPreference, error) {
	if preference, ok := ctx.Value(twoFactorPreferenceContextKey{}).(appleauth.TwoFactorPreference); ok {
		return preference, nil
	}
	return resolveTwoFactorPreference("", 0, false)
}
