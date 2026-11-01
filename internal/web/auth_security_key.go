package web

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/appleauth"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/securitykey"
)

var (
	securityKeyAvailableFn = securitykey.Available
	signSecurityKeyFn      = securitykey.Sign
)

type securityKeyClientData struct {
	Type        string `json:"type"`
	Challenge   string `json:"challenge"`
	Origin      string `json:"origin"`
	CrossOrigin bool   `json:"crossOrigin"`
}

// Apple's private verification endpoint expects a flat response with standard
// base64 fields, rather than the browser WebAuthn response envelope.
type securityKeyResponse struct {
	Challenge         string `json:"challenge"`
	ClientData        string `json:"clientData"`
	SignatureData     string `json:"signatureData"`
	AuthenticatorData string `json:"authenticatorData"`
	UserHandle        string `json:"userHandle"`
	CredentialID      string `json:"credentialID"`
	RPID              string `json:"rpId"`
}

// SecurityKeyAssertionRejectedError reports that Apple rejected an assertion,
// rather than a device, transport, or session-finalization failure.
type SecurityKeyAssertionRejectedError struct{ Status int }

func (e *SecurityKeyAssertionRejectedError) Error() string {
	return fmt.Sprintf("security-key verification rejected by Apple (HTTP %d)", e.Status)
}

// SubmitSecurityKey completes an already prepared Apple FSA challenge. The PIN
// is only passed to the authenticator and is never sent to Apple or cached.
func SubmitSecurityKey(ctx context.Context, session *AuthSession, pin string) error {
	if session == nil || session.Client == nil || session.securityKeyChallenge == nil {
		return fmt.Errorf("a prepared security-key challenge is required")
	}
	if !securityKeyAvailableFn() {
		return securitykey.ErrUnavailable
	}
	fsa := session.securityKeyChallenge
	challenge, err := decodeSecurityKeyBase64(fsa.Challenge)
	if err != nil || len(challenge) == 0 {
		return fmt.Errorf("invalid security-key challenge returned by Apple")
	}
	var credentials [][]byte
	for encoded := range strings.SplitSeq(fsa.AllowedCredentials, ",") {
		credential, err := decodeSecurityKeyBase64(strings.TrimSpace(encoded))
		if err != nil || len(credential) == 0 {
			return fmt.Errorf("invalid security-key credential allow-list returned by Apple")
		}
		credentials = append(credentials, credential)
	}
	clientData, err := json.Marshal(securityKeyClientData{
		Type: "webauthn.get", Challenge: base64.RawURLEncoding.EncodeToString(challenge),
		Origin: "https://idmsa.apple.com", CrossOrigin: true,
	})
	if err != nil {
		return fmt.Errorf("encode security-key client data: %w", err)
	}
	assertion, err := signSecurityKeyFn(ctx, securitykey.Request{
		RelyingParty: "apple.com", ClientDataHash: sha256.Sum256(clientData), CredentialIDs: credentials,
	}, pin)
	if err != nil {
		return err
	}
	allowed := false
	for _, credential := range credentials {
		if bytes.Equal(credential, assertion.CredentialID) {
			allowed = true
			break
		}
	}
	if !allowed || len(assertion.AuthenticatorData) < 37 || len(assertion.Signature) == 0 {
		return fmt.Errorf("security key returned an invalid assertion")
	}
	// Match the XcodesLoginKit protocol: absent user handles are empty, while
	// present handles must be UTF-8. Do not silently corrupt a binary handle.
	if !utf8.Valid(assertion.UserID) {
		return fmt.Errorf("security key returned a binary user handle unsupported by Apple's sign-in protocol")
	}
	encode := base64.StdEncoding.EncodeToString
	payload := securityKeyResponse{
		Challenge: encode(challenge), ClientData: encode(clientData),
		SignatureData: encode(assertion.Signature), AuthenticatorData: encode(assertion.AuthenticatorData),
		UserHandle: string(assertion.UserID), CredentialID: encode(assertion.CredentialID), RPID: "apple.com",
	}
	// Consume continuation state before sending. An interrupted request may
	// already have been accepted, so this assertion must never be replayed.
	session.securityKeyChallenge = nil
	status, _, err := appleauth.DoTwoFactorJSONRequest(ctx, session.Client, appleSessionHeaders(session),
		"verify_security_key", http.MethodPost, authServiceURL+"/verify/security/key", payload,
		json.Marshal, nil, logWebAuthHTTP)
	if err != nil {
		return fmt.Errorf("security-key verification request failed; start a new sign-in before retrying: %w", err)
	}
	if status < 200 || status >= 300 {
		if status == http.StatusPreconditionFailed {
			return fmt.Errorf("account acknowledgement required by Apple; sign in at https://appleid.apple.com before retrying")
		}
		if status == http.StatusBadRequest || status == http.StatusUnauthorized {
			return &SecurityKeyAssertionRejectedError{Status: status}
		}
		return fmt.Errorf("security-key verification failed (HTTP %d)", status)
	}
	// Once Apple accepts the assertion, trust and session-bootstrap failures
	// must not be reported as rejected authentication, including transport errors.
	err = finalizeTwoFactor(ctx, session)
	var finalization *TwoFactorFinalizationError
	if err != nil && !errors.As(err, &finalization) {
		return &TwoFactorFinalizationError{Err: err}
	}
	return err
}

func decodeSecurityKeyBase64(value string) ([]byte, error) {
	for _, encoding := range []*base64.Encoding{base64.RawURLEncoding, base64.URLEncoding, base64.RawStdEncoding, base64.StdEncoding} {
		if decoded, err := encoding.Strict().DecodeString(value); err == nil {
			return decoded, nil
		}
	}
	return nil, fmt.Errorf("invalid base64")
}
