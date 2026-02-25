package iris

import (
	"bytes"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/1Password/srp"
	"golang.org/x/crypto/pbkdf2"
)

func newIrisHTTPClient(jar http.CookieJar) *http.Client {
	transport, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		return &http.Client{
			Jar:     jar,
			Timeout: 60 * time.Second,
		}
	}

	cloned := transport.Clone()
	// Default is 10s; raise to reduce false failures on slow networks.
	cloned.TLSHandshakeTimeout = 30 * time.Second

	return &http.Client{
		Jar:       jar,
		Timeout:   60 * time.Second,
		Transport: cloned,
	}
}

func extractServiceErrorCodes(respBody []byte) []string {
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
		if strings.TrimSpace(serviceErr.Code) != "" {
			codes = append(codes, serviceErr.Code)
		}
	}
	return codes
}

const (
	// Apple authentication endpoints
	authServiceURL    = "https://idmsa.apple.com/appleauth/auth"
	appStoreBaseURL   = "https://appstoreconnect.apple.com"
	olympusSessionURL = "https://appstoreconnect.apple.com/olympus/v1/session"

	// Apple uses SRP RFC5054 2048 group and a 32-byte derived password.
	srpClientSecretBytes  = 256
	srpDerivedPasswordLen = 32
)

// AuthSession represents an authenticated IRIS session
type AuthSession struct {
	Client     *http.Client
	ProviderID int64
	TeamID     string
	UserEmail  string

	// Auth state needed for 2FA continuation.
	ServiceKey       string
	AppleIDSessionID string
	SCNT             string
}

// LoginCredentials holds the credentials for login
type LoginCredentials struct {
	Username string
	Password string
}

// TwoFactorRequiredError indicates the Apple account requires 2FA to proceed.
//
// This error intentionally does not include raw session tokens in its Error()
// string to avoid accidental logging.
type TwoFactorRequiredError struct {
	AppleIDSessionID string
	SCNT             string
}

func (e *TwoFactorRequiredError) Error() string {
	return "2FA required"
}

// signinInitResponse represents the response from auth/signin/init
type signinInitResponse struct {
	Iteration  int    `json:"iteration"`
	Salt       string `json:"salt"`
	Protocol   string `json:"protocol"`
	ServerPubB string `json:"b"`
	Challenge  string `json:"c"`
}

// SessionInfo represents the session information from Apple
type SessionInfo struct {
	Provider struct {
		ProviderID int64  `json:"providerId"`
		Name       string `json:"name"`
	} `json:"provider"`
	User struct {
		EmailAddress string `json:"emailAddress"`
	} `json:"user"`
	AvailableProviders []struct {
		ProviderID int64  `json:"providerId"`
		Name       string `json:"name"`
	} `json:"availableProviders"`
}

// Client is an IRIS API client
type Client struct {
	httpClient *http.Client
	baseURL    string
}

// APIError captures non-2xx responses from IRIS endpoints.
// It intentionally includes the raw body, since IRIS error bodies are often the
// only available "schema" for debugging.
type APIError struct {
	Status         int
	Body           []byte
	AppleRequestID string
	CorrelationKey string
}

func (e *APIError) Error() string {
	// Keep previous error shape for callers while enriching debug logs.
	return fmt.Sprintf("API error (status %d): %s", e.Status, string(e.Body))
}

// NewClient creates a new IRIS client with an authenticated session
func NewClient(session *AuthSession) *Client {
	return &Client{
		httpClient: session.Client,
		baseURL:    appStoreBaseURL + "/iris/v1",
	}
}

// Login authenticates with Apple using SRP and returns a session
func Login(creds LoginCredentials) (*AuthSession, error) {
	// Create cookie jar for session management
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create cookie jar: %w", err)
	}

	client := newIrisHTTPClient(jar)

	// Get auth service key
	serviceKey, err := getAuthServiceKey(client)
	if err != nil {
		return nil, fmt.Errorf("failed to get service key: %w", err)
	}

	// Perform SRP authentication
	if err := performSRPLogin(client, creds, serviceKey); err != nil {
		// Return a partially-initialized session so the caller can continue 2FA.
		var tfaErr *TwoFactorRequiredError
		if errors.As(err, &tfaErr) {
			return &AuthSession{
				Client:           client,
				ServiceKey:       serviceKey,
				AppleIDSessionID: tfaErr.AppleIDSessionID,
				SCNT:             tfaErr.SCNT,
				UserEmail:        creds.Username,
			}, fmt.Errorf("SRP login failed: %w", err)
		}
		return nil, fmt.Errorf("SRP login failed: %w", err)
	}

	// Get session info
	sessionInfo, err := getSessionInfo(client)
	if err != nil {
		return nil, fmt.Errorf("failed to get session info: %w", err)
	}

	return &AuthSession{
		Client:     client,
		ProviderID: sessionInfo.Provider.ProviderID,
		TeamID:     fmt.Sprintf("%d", sessionInfo.Provider.ProviderID),
		UserEmail:  sessionInfo.User.EmailAddress,
		ServiceKey: serviceKey,
	}, nil
}

// getAuthServiceKey fetches the Apple Widget Key for authentication
func getAuthServiceKey(client *http.Client) (string, error) {
	req, err := http.NewRequest("GET", "https://appstoreconnect.apple.com/olympus/v1/app/config?hostname=itunesconnect.apple.com", nil)
	if err != nil {
		return "", fmt.Errorf("failed to build auth service key request: %w", err)
	}
	setModifiedCookieHeader(client, req)

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to get auth service key: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("failed to get auth service key with status %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		// Known response field from olympus config endpoint.
		AuthServiceKey string `json:"authServiceKey"`

		// Back-compat: some clients/documentation mention serviceKey.
		ServiceKey string `json:"serviceKey"`

		KeyID string `json:"keyId"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode auth service key response: %w", err)
	}
	serviceKey := strings.TrimSpace(result.AuthServiceKey)
	if serviceKey == "" {
		serviceKey = strings.TrimSpace(result.ServiceKey)
	}

	if serviceKey == "" {
		return "", fmt.Errorf("auth service key is empty")
	}

	return serviceKey, nil
}

// performSRPLogin performs the SRP authentication flow
func performSRPLogin(client *http.Client, creds LoginCredentials, serviceKey string) error {
	// Use RFC 5054 2048-bit group (Apple uses 2048-bit)
	group := srp.KnownGroups[srp.RFC5054Group2048]
	n := group.N()
	g := group.Generator()

	// Generate ephemeral private key (a)
	aBytes := make([]byte, srpClientSecretBytes)
	if _, err := rand.Read(aBytes); err != nil {
		return fmt.Errorf("failed to generate ephemeral key: %w", err)
	}
	a := new(big.Int).SetBytes(aBytes)

	// Calculate A = g^a mod N (client's public ephemeral value)
	A := new(big.Int).Exp(g, a, n)
	aBase64 := base64.StdEncoding.EncodeToString(A.Bytes())

	// Step 1: Init sign in
	initResp, err := signinInit(client, creds.Username, aBase64, serviceKey)
	if err != nil {
		return fmt.Errorf("signin init failed: %w", err)
	}

	// Derive password using PBKDF2
	salt, err := base64.StdEncoding.DecodeString(initResp.Salt)
	if err != nil {
		return fmt.Errorf("failed to decode salt: %w", err)
	}

	preparedPassword, err := preparePasswordForProtocol(creds.Password, initResp.Protocol)
	if err != nil {
		return err
	}

	// Apple derives a 32-byte password via PBKDF2 from protocol-prepared password bytes.
	derivedPassword := pbkdf2.Key(preparedPassword, salt, initResp.Iteration, srpDerivedPasswordLen, sha256.New)

	// Decode server public B
	serverB, err := base64.StdEncoding.DecodeString(initResp.ServerPubB)
	if err != nil {
		return fmt.Errorf("failed to decode server public B: %w", err)
	}

	// Calculate SRP proofs using fastlane-compatible SIRP formulas.
	m1, m2, err := calculateSRPProof(creds.Username, a, A, n, g, serverB, derivedPassword, salt)
	if err != nil {
		return fmt.Errorf("failed to calculate SRP proof: %w", err)
	}

	hashcash, err := getHashcash(client, serviceKey)
	if err != nil {
		return fmt.Errorf("failed to compute hashcash: %w", err)
	}

	// Step 2: Complete sign in
	if err := signinComplete(client, creds.Username, m1, m2, initResp.Challenge, serviceKey, hashcash); err != nil {
		return fmt.Errorf("signin complete failed: %w", err)
	}

	return nil
}

// calculateSRPProof calculates M1 and M2 using fastlane-compatible SIRP formulas.
func calculateSRPProof(username string, a, A, n, g *big.Int, serverB, derivedPassword, salt []byte) (string, string, error) {
	bHex := hex.EncodeToString(serverB)
	saltHex := hex.EncodeToString(salt)
	aHex := numToHex(A)
	derivedPasswordHex := hex.EncodeToString(derivedPassword)

	// x = SHA256(salt || SHA256(":" || derivedPasswordHexBytes))
	x, err := calcXHex(derivedPasswordHex, saltHex)
	if err != nil {
		return "", "", err
	}

	k, err := calcK(n, g)
	if err != nil {
		return "", "", err
	}

	u, err := calcU(n, aHex, bHex)
	if err != nil {
		return "", "", err
	}
	if u.Sign() == 0 {
		return "", "", fmt.Errorf("invalid SRP scrambling parameter")
	}

	B := new(big.Int).SetBytes(serverB)

	// S = (B - k*g^x)^(a + ux) mod N
	gx := new(big.Int).Exp(g, x, n)
	kgx := new(big.Int).Mul(k, gx)
	kgx.Mod(kgx, n)
	base := new(big.Int).Sub(B, kgx)
	base.Mod(base, n)
	if base.Sign() < 0 {
		base.Add(base, n)
	}
	exp := new(big.Int).Add(a, new(big.Int).Mul(u, x))
	S := new(big.Int).Exp(base, exp, n)

	// K = SHA256(hex_bytes(S_hex))
	kHex, err := shaHex(numToHex(S))
	if err != nil {
		return "", "", err
	}

	// M1 = SHA256(hex_bytes(H(N)^H(g) || H(username) || s || A || B || K))
	m1Hex, err := calcM(n, g, username, saltHex, aHex, bHex, kHex)
	if err != nil {
		return "", "", err
	}

	// M2 = SHA256(hex_bytes(A || M1 || K))
	m2Hex, err := calcHAMK(aHex, m1Hex, kHex)
	if err != nil {
		return "", "", err
	}

	m1Bytes, err := hex.DecodeString(m1Hex)
	if err != nil {
		return "", "", fmt.Errorf("failed to decode m1 proof: %w", err)
	}
	m2Bytes, err := hex.DecodeString(m2Hex)
	if err != nil {
		return "", "", fmt.Errorf("failed to decode m2 proof: %w", err)
	}

	return base64.StdEncoding.EncodeToString(m1Bytes), base64.StdEncoding.EncodeToString(m2Bytes), nil
}

func preparePasswordForProtocol(password, protocol string) ([]byte, error) {
	passwordDigest := sha256.Sum256([]byte(password))

	switch protocol {
	case "s2k":
		return passwordDigest[:], nil
	case "s2k_fo":
		return []byte(hex.EncodeToString(passwordDigest[:])), nil
	default:
		return nil, fmt.Errorf("unsupported SRP protocol %q", protocol)
	}
}

func calcXHex(derivedPasswordHex, saltHex string) (*big.Int, error) {
	if _, err := hex.DecodeString(derivedPasswordHex); err != nil {
		return nil, fmt.Errorf("invalid derived password hex: %w", err)
	}
	if _, err := hex.DecodeString(saltHex); err != nil {
		return nil, fmt.Errorf("invalid salt hex: %w", err)
	}

	inner, err := shaHex("3a" + derivedPasswordHex) // ":" + xpassword
	if err != nil {
		return nil, err
	}
	outer, err := shaHex(saltHex + inner)
	if err != nil {
		return nil, err
	}

	x := new(big.Int)
	if _, ok := x.SetString(outer, 16); !ok {
		return nil, fmt.Errorf("failed to parse x value")
	}
	return x, nil
}

func calcK(n, g *big.Int) (*big.Int, error) {
	return hashWithPadding(n, numToHex(n), numToHex(g))
}

func calcU(n *big.Int, aHex, bHex string) (*big.Int, error) {
	return hashWithPadding(n, aHex, bHex)
}

func calcM(n, g *big.Int, username, saltHex, aHex, bHex, kHex string) (string, error) {
	hn, err := hashWithPadding(n, numToHex(n))
	if err != nil {
		return "", err
	}
	hg, err := hashWithPadding(n, numToHex(g))
	if err != nil {
		return "", err
	}
	hxor := new(big.Int).Xor(hn, hg)

	buf := numToHex(hxor) + shaStringHex(username) + saltHex + aHex + bHex + kHex
	raw, err := hex.DecodeString(buf)
	if err != nil {
		return "", fmt.Errorf("failed to decode M input: %w", err)
	}

	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

func calcHAMK(aHex, mHex, kHex string) (string, error) {
	raw, err := hex.DecodeString(aHex + mHex + kHex)
	if err != nil {
		return "", fmt.Errorf("failed to decode H_AMK input: %w", err)
	}
	sum := sha256.Sum256(raw)

	// Mirror fastlane-sirp behavior: digest bytes -> integer -> hex string.
	hamkInt := new(big.Int).SetBytes(sum[:])
	return numToHex(hamkInt), nil
}

func hashWithPadding(n *big.Int, values ...string) (*big.Int, error) {
	nHexLen := len(fmt.Sprintf("%x", n))
	nLen := 2 * (((nHexLen * 4) + 7) >> 3)

	var input strings.Builder
	for _, value := range values {
		if value == "" {
			continue
		}
		hexValue := strings.ToLower(value)
		if len(hexValue) > nLen {
			return nil, fmt.Errorf("bit width mismatch for value")
		}
		input.WriteString(strings.Repeat("0", nLen-len(hexValue)))
		input.WriteString(hexValue)
	}

	digestHex, err := shaHex(input.String())
	if err != nil {
		return nil, err
	}

	result := new(big.Int)
	if _, ok := result.SetString(digestHex, 16); !ok {
		return nil, fmt.Errorf("failed to parse hash result")
	}
	result.Mod(result, n)
	return result, nil
}

func shaHex(hexValue string) (string, error) {
	raw, err := hex.DecodeString(hexValue)
	if err != nil {
		return "", fmt.Errorf("invalid hex input: %w", err)
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

func shaStringHex(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func numToHex(number *big.Int) string {
	hexValue := strings.ToLower(number.Text(16))
	if len(hexValue)%2 == 1 {
		hexValue = "0" + hexValue
	}
	return hexValue
}

func getHashcash(client *http.Client, serviceKey string) (string, error) {
	endpoint := authServiceURL + "/signin?widgetKey=" + url.QueryEscape(serviceKey)
	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	setModifiedCookieHeader(client, req)

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("failed to fetch hashcash challenge with status %d: %s", resp.StatusCode, string(body))
	}

	bitsValue := strings.TrimSpace(resp.Header.Get("X-Apple-HC-Bits"))
	challenge := strings.TrimSpace(resp.Header.Get("X-Apple-HC-Challenge"))
	if bitsValue == "" || challenge == "" {
		return "", fmt.Errorf("missing hashcash headers in signin response")
	}

	bits, err := strconv.Atoi(bitsValue)
	if err != nil {
		return "", fmt.Errorf("invalid hashcash bits %q: %w", bitsValue, err)
	}

	return makeHashcash(bits, challenge, time.Now().UTC()), nil
}

func makeHashcash(bits int, challenge string, now time.Time) string {
	date := now.Format("20060102150405")

	for counter := 0; ; counter++ {
		candidate := fmt.Sprintf("1:%d:%s:%s::%d", bits, date, challenge, counter)
		sum := sha1.Sum([]byte(candidate))
		if hasLeadingZeroBits(sum[:], bits) {
			return candidate
		}
	}
}

func hasLeadingZeroBits(sum []byte, bits int) bool {
	fullBytes := bits / 8
	remainingBits := bits % 8

	for i := 0; i < fullBytes; i++ {
		if sum[i] != 0 {
			return false
		}
	}
	if remainingBits == 0 {
		return true
	}

	mask := byte(0xFF << (8 - remainingBits))
	return (sum[fullBytes] & mask) == 0
}

// setModifiedCookieHeader mirrors fastlane's workaround where DES cookies
// need explicit quotes in the Cookie header for some Apple auth flows.
func setModifiedCookieHeader(client *http.Client, req *http.Request) {
	if client == nil || client.Jar == nil || req == nil || req.URL == nil {
		return
	}

	cookies := client.Jar.Cookies(req.URL)
	if len(cookies) == 0 {
		return
	}

	parts := make([]string, 0, len(cookies))
	for _, cookie := range cookies {
		if cookie == nil {
			continue
		}
		value := cookie.Value
		if strings.Contains(cookie.Name, "DES") && !strings.HasPrefix(value, "\"") {
			value = "\"" + value + "\""
		}
		parts = append(parts, cookie.Name+"="+value)
	}
	if len(parts) == 0 {
		return
	}

	req.Header.Set("Cookie", strings.Join(parts, "; "))
}

// signinInit initiates the SRP authentication
func signinInit(client *http.Client, username, aBase64, serviceKey string) (*signinInitResponse, error) {
	reqBody := map[string]interface{}{
		"accountName": username,
		"protocols":   []string{"s2k", "s2k_fo"},
		"a":           aBase64,
	}
	body, _ := json.Marshal(reqBody)

	req, err := http.NewRequest("POST", authServiceURL+"/signin/init", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Apple-Widget-Key", serviceKey)
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	req.Header.Set("Accept", "application/json, text/javascript")
	setModifiedCookieHeader(client, req)

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("signin init failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	var result signinInitResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to decode signin init response: %w", err)
	}

	return &result, nil
}

// signinComplete completes the SRP authentication
func signinComplete(client *http.Client, username, m1, m2, challenge, serviceKey, hashcash string) error {
	reqBody := map[string]interface{}{
		"accountName": username,
		"rememberMe":  false,
		"m1":          m1,
		"m2":          m2,
		"c":           challenge,
	}
	body, _ := json.Marshal(reqBody)

	req, err := http.NewRequest("POST", authServiceURL+"/signin/complete?isRememberMeEnabled=false", bytes.NewReader(body))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Apple-Widget-Key", serviceKey)
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	req.Header.Set("Accept", "application/json, text/javascript")
	req.Header.Set("X-Apple-HC", hashcash)
	setModifiedCookieHeader(client, req)

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	// 200 = success, 409 = 2FA required
	if resp.StatusCode == 200 {
		return nil
	}

	if resp.StatusCode == 409 {
		// 2FA required - capture session headers for follow-up requests.
		return &TwoFactorRequiredError{
			AppleIDSessionID: strings.TrimSpace(resp.Header.Get("X-Apple-ID-Session-Id")),
			SCNT:             strings.TrimSpace(resp.Header.Get("scnt")),
		}
	}

	return fmt.Errorf("signin complete failed with status %d: %s", resp.StatusCode, string(respBody))
}

// getSessionInfo gets the current session information
func getSessionInfo(client *http.Client) (*SessionInfo, error) {
	resp, err := client.Get(olympusSessionURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("failed to get session info with status %d: %s", resp.StatusCode, string(respBody))
	}

	var result SessionInfo
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to decode session info: %w", err)
	}

	return &result, nil
}

type authOptionsResponse struct {
	TrustedDevices      []map[string]interface{} `json:"trustedDevices"`
	TrustedPhoneNumbers []struct {
		ID                 int    `json:"id"`
		PushMode           string `json:"pushMode"`
		NumberWithDialCode string `json:"numberWithDialCode"`
	} `json:"trustedPhoneNumbers"`
	SecurityCode struct {
		Length int `json:"length"`
	} `json:"securityCode"`
}

type twoFAVerificationFailedError struct {
	Kind   string
	Status int
	Body   []byte
}

func (e *twoFAVerificationFailedError) Error() string {
	codes := extractServiceErrorCodes(e.Body)
	if len(codes) > 0 {
		return fmt.Sprintf("%s 2FA failed (status %d, codes=%v)", e.Kind, e.Status, codes)
	}
	return fmt.Sprintf("%s 2FA failed (status %d)", e.Kind, e.Status)
}

// SubmitTwoFactorCode completes Apple 2FA for an existing SRP session.
//
// This is used when Login() returns a non-nil session with a *TwoFactorRequiredError.
func SubmitTwoFactorCode(session *AuthSession, code string) error {
	if session == nil || session.Client == nil {
		return fmt.Errorf("session is required")
	}
	code = strings.TrimSpace(code)
	if code == "" {
		return fmt.Errorf("2FA code is required")
	}
	if session.ServiceKey == "" || session.AppleIDSessionID == "" || session.SCNT == "" {
		return fmt.Errorf("session is missing 2FA continuation state")
	}

	// Fetch auth options to decide which verification endpoint to use.
	opts, err := getAuthOptions(session)
	if err != nil {
		return err
	}

	// Apple commonly shows "Enter the verification code sent to your Apple devices."
	// even when it doesn't enumerate trusted devices. Try the trusted-device endpoint first.
	if err := submitTrustedDeviceCode(session, code); err == nil {
		return finalizeTwoFactor(session)
	}

	// Fallback to phone flow if present (SMS/voice).
	if len(opts.TrustedPhoneNumbers) > 0 {
		phone := opts.TrustedPhoneNumbers[0]
		mode := strings.TrimSpace(phone.PushMode)
		if mode == "" {
			mode = "sms"
		}
		if err := submitPhoneCode(session, code, phone.ID, mode); err != nil {
			return err
		}
		return finalizeTwoFactor(session)
	}

	return fmt.Errorf("2FA failed: no supported verification method found")
}

func appleSessionHeaders(session *AuthSession) http.Header {
	header := make(http.Header)
	header.Set("X-Apple-ID-Session-Id", session.AppleIDSessionID)
	header.Set("X-Apple-Widget-Key", session.ServiceKey)
	header.Set("scnt", session.SCNT)
	return header
}

func getAuthOptions(session *AuthSession) (*authOptionsResponse, error) {
	req, err := http.NewRequest("GET", authServiceURL, nil)
	if err != nil {
		return nil, err
	}
	for key, values := range appleSessionHeaders(session) {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}
	req.Header.Set("Accept", "application/json")
	setModifiedCookieHeader(session.Client, req)

	resp, err := session.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	// Xcodes expects 201; accept any 2xx.
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("auth options failed with status %d: %s", resp.StatusCode, string(body))
	}

	var result authOptionsResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse auth options: %w", err)
	}
	return &result, nil
}

func submitTrustedDeviceCode(session *AuthSession, code string) error {
	payload := map[string]interface{}{
		"securityCode": map[string]string{"code": code},
	}
	body, _ := json.Marshal(payload)

	req, err := http.NewRequest("POST", authServiceURL+"/verify/trusteddevice/securitycode", bytes.NewReader(body))
	if err != nil {
		return err
	}
	for key, values := range appleSessionHeaders(session) {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	setModifiedCookieHeader(session.Client, req)

	resp, err := session.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	return &twoFAVerificationFailedError{Kind: "trusted-device", Status: resp.StatusCode, Body: respBody}
}

func submitPhoneCode(session *AuthSession, code string, phoneID int, mode string) error {
	payload := map[string]interface{}{
		"securityCode": map[string]string{"code": code},
		"phoneNumber":  map[string]int{"id": phoneID},
		"mode":         mode,
	}
	body, _ := json.Marshal(payload)

	req, err := http.NewRequest("POST", authServiceURL+"/verify/phone/securitycode", bytes.NewReader(body))
	if err != nil {
		return err
	}
	for key, values := range appleSessionHeaders(session) {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	setModifiedCookieHeader(session.Client, req)

	resp, err := session.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	return &twoFAVerificationFailedError{Kind: "phone", Status: resp.StatusCode, Body: respBody}
}

func finalizeTwoFactor(session *AuthSession) error {
	req, err := http.NewRequest("GET", authServiceURL+"/2sv/trust", nil)
	if err != nil {
		return err
	}
	for key, values := range appleSessionHeaders(session) {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}
	req.Header.Set("Accept", "application/json")
	setModifiedCookieHeader(session.Client, req)

	resp, err := session.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("2fa trust failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Refresh session info so callers can proceed with API calls.
	info, err := getSessionInfo(session.Client)
	if err != nil {
		return err
	}
	session.ProviderID = info.Provider.ProviderID
	session.TeamID = fmt.Sprintf("%d", info.Provider.ProviderID)
	session.UserEmail = info.User.EmailAddress

	return nil
}

// doRequest performs an HTTP request with the IRIS client
func (c *Client) doRequest(method, path string, body interface{}) ([]byte, error) {
	var reqBody io.Reader
	var jsonBody []byte
	if body != nil {
		var err error
		jsonBody, err = json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		reqBody = bytes.NewReader(jsonBody)
	}

	req, err := http.NewRequest(method, c.baseURL+path, reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	// Mirror the browser-ish shape; some internal endpoints are picky.
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	req.Header.Set("Origin", appStoreBaseURL)
	req.Header.Set("Referer", appStoreBaseURL+"/")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	appleRequestID := strings.TrimSpace(resp.Header.Get("X-Apple-Request-Uuid"))
	if appleRequestID == "" {
		// Some services use a different casing/spelling.
		appleRequestID = strings.TrimSpace(resp.Header.Get("X-Apple-Request-UUID"))
	}
	correlationKey := strings.TrimSpace(resp.Header.Get("X-Apple-Jingle-Correlation-Key"))

	if resp.StatusCode >= 400 {
		return nil, &APIError{
			Status:         resp.StatusCode,
			Body:           respBody,
			AppleRequestID: appleRequestID,
			CorrelationKey: correlationKey,
		}
	}

	return respBody, nil
}
