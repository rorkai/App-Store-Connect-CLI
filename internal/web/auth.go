package web

import (
	"bytes"
	"context"
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

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

const (
	// Apple auth endpoints used by the web session flow.
	authServiceURL    = "https://idmsa.apple.com/appleauth/auth"
	appStoreBaseURL   = "https://appstoreconnect.apple.com"
	olympusSessionURL = "https://appstoreconnect.apple.com/olympus/v1/session"

	// Apple currently uses RFC5054 group 2048 + 32-byte derived password.
	srpClientSecretBytes  = 256
	srpDerivedPasswordLen = 32
)

var errTwoFactorRequired = errors.New("two-factor authentication required")

// AuthSession holds authenticated web-session state for internal API calls.
type AuthSession struct {
	Client     *http.Client
	ProviderID int64
	TeamID     string
	UserEmail  string

	// Continuation state needed after a 409 SRP completion response.
	ServiceKey       string
	AppleIDSessionID string
	SCNT             string
}

// LoginCredentials holds Apple ID credentials.
type LoginCredentials struct {
	Username string
	Password string
}

// TwoFactorRequiredError signals that the caller must submit a 2FA code.
type TwoFactorRequiredError struct {
	AppleIDSessionID string
	SCNT             string
}

func (e *TwoFactorRequiredError) Error() string {
	return errTwoFactorRequired.Error()
}

// Client is an internal web API client using a web session cookie jar.
type Client struct {
	httpClient *http.Client
	baseURL    string
}

// APIError wraps non-2xx internal web API responses.
//
// The raw body is retained for internal classification and tests, but Error()
// intentionally avoids dumping response bodies that may contain sensitive data.
type APIError struct {
	Status         int
	AppleRequestID string
	CorrelationKey string
	rawBody        []byte
}

func (e *APIError) Error() string {
	parts := []string{fmt.Sprintf("web api error (status %d)", e.Status)}
	if e.AppleRequestID != "" {
		parts = append(parts, fmt.Sprintf("request_id=%s", e.AppleRequestID))
	}
	if e.CorrelationKey != "" {
		parts = append(parts, fmt.Sprintf("correlation_key=%s", e.CorrelationKey))
	}
	if codes := extractServiceErrorCodes(e.rawBody); len(codes) > 0 {
		parts = append(parts, fmt.Sprintf("codes=%v", codes))
	}
	return strings.Join(parts, ", ")
}

// rawResponseBody exposes the body to package-internal helpers only.
func (e *APIError) rawResponseBody() []byte {
	return e.rawBody
}

type signinInitResponse struct {
	Iteration  int             `json:"iteration"`
	Salt       string          `json:"salt"`
	Protocol   string          `json:"protocol"`
	ServerPubB string          `json:"b"`
	Challenge  json.RawMessage `json:"c"`
}

type sessionInfo struct {
	Provider struct {
		ProviderID int64  `json:"providerId"`
		Name       string `json:"name"`
	} `json:"provider"`
	User struct {
		EmailAddress string `json:"emailAddress"`
	} `json:"user"`
}

type authOptionsResponse struct {
	TrustedDevices      []map[string]any `json:"trustedDevices"`
	TrustedPhoneNumbers []struct {
		ID                 int    `json:"id"`
		PushMode           string `json:"pushMode"`
		NumberWithDialCode string `json:"numberWithDialCode"`
	} `json:"trustedPhoneNumbers"`
}

type twoFAVerificationFailedError struct {
	Kind   string
	Status int
	Body   []byte
}

func (e *twoFAVerificationFailedError) Error() string {
	codes := extractServiceErrorCodes(e.Body)
	if len(codes) > 0 {
		return fmt.Sprintf("%s 2fa failed (status %d, codes=%v)", e.Kind, e.Status, codes)
	}
	return fmt.Sprintf("%s 2fa failed (status %d)", e.Kind, e.Status)
}

func newWebHTTPClient(jar http.CookieJar) *http.Client {
	transport, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		return &http.Client{
			Jar:     jar,
			Timeout: asc.ResolveTimeout(),
		}
	}

	cloned := transport.Clone()
	cloned.TLSHandshakeTimeout = 30 * time.Second

	return &http.Client{
		Jar:       jar,
		Timeout:   asc.ResolveTimeout(),
		Transport: cloned,
	}
}

func parseSigninInitResponse(data []byte) (*signinInitResponse, error) {
	var result signinInitResponse
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to decode signin init response: %w", err)
	}
	if len(result.Challenge) == 0 || bytes.Equal(result.Challenge, []byte("null")) {
		return nil, fmt.Errorf("signin init response missing challenge")
	}
	return &result, nil
}

// NewClient creates an internal web API client from an authenticated session.
func NewClient(session *AuthSession) *Client {
	return &Client{
		httpClient: session.Client,
		baseURL:    appStoreBaseURL + "/iris/v1",
	}
}

// Login performs Apple ID SRP authentication and returns a web session.
//
// If 2FA is required, Login returns a non-nil partial session and an error
// wrapping *TwoFactorRequiredError. The caller can continue with SubmitTwoFactorCode.
func Login(ctx context.Context, creds LoginCredentials) (*AuthSession, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if strings.TrimSpace(creds.Username) == "" {
		return nil, fmt.Errorf("apple id is required")
	}
	if strings.TrimSpace(creds.Password) == "" {
		return nil, fmt.Errorf("password is required")
	}

	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create cookie jar: %w", err)
	}
	client := newWebHTTPClient(jar)

	serviceKey, err := getAuthServiceKey(ctx, client)
	if err != nil {
		return nil, fmt.Errorf("failed to get auth service key: %w", err)
	}

	if err := performSRPLogin(ctx, client, creds, serviceKey); err != nil {
		var tfaErr *TwoFactorRequiredError
		if errors.As(err, &tfaErr) {
			partial := &AuthSession{
				Client:           client,
				ServiceKey:       serviceKey,
				AppleIDSessionID: tfaErr.AppleIDSessionID,
				SCNT:             tfaErr.SCNT,
				UserEmail:        strings.TrimSpace(creds.Username),
			}
			return partial, fmt.Errorf("srp login failed: %w", err)
		}
		return nil, fmt.Errorf("srp login failed: %w", err)
	}

	info, err := getSessionInfo(ctx, client)
	if err != nil {
		return nil, fmt.Errorf("failed to get session info: %w", err)
	}

	return &AuthSession{
		Client:     client,
		ProviderID: info.Provider.ProviderID,
		TeamID:     fmt.Sprintf("%d", info.Provider.ProviderID),
		UserEmail:  strings.TrimSpace(info.User.EmailAddress),
		ServiceKey: serviceKey,
	}, nil
}

func getAuthServiceKey(ctx context.Context, client *http.Client) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", "https://appstoreconnect.apple.com/olympus/v1/app/config?hostname=itunesconnect.apple.com", nil)
	if err != nil {
		return "", fmt.Errorf("failed to build auth service key request: %w", err)
	}
	setModifiedCookieHeader(client, req)

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to fetch auth service key: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read auth service key response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to fetch auth service key (status %d)", resp.StatusCode)
	}

	var payload struct {
		AuthServiceKey string `json:"authServiceKey"`
		ServiceKey     string `json:"serviceKey"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", fmt.Errorf("failed to decode auth service key response: %w", err)
	}
	serviceKey := strings.TrimSpace(payload.AuthServiceKey)
	if serviceKey == "" {
		serviceKey = strings.TrimSpace(payload.ServiceKey)
	}
	if serviceKey == "" {
		return "", fmt.Errorf("auth service key is empty")
	}
	return serviceKey, nil
}

func performSRPLogin(ctx context.Context, client *http.Client, creds LoginCredentials, serviceKey string) error {
	group := srp.KnownGroups[srp.RFC5054Group2048]
	n := group.N()
	g := group.Generator()

	aBytes := make([]byte, srpClientSecretBytes)
	if _, err := rand.Read(aBytes); err != nil {
		return fmt.Errorf("failed to generate ephemeral key: %w", err)
	}
	a := new(big.Int).SetBytes(aBytes)
	A := new(big.Int).Exp(g, a, n)
	aBase64 := base64.StdEncoding.EncodeToString(A.Bytes())

	initResp, err := signinInit(ctx, client, strings.TrimSpace(creds.Username), aBase64, serviceKey)
	if err != nil {
		return fmt.Errorf("signin init failed: %w", err)
	}

	salt, err := base64.StdEncoding.DecodeString(initResp.Salt)
	if err != nil {
		return fmt.Errorf("failed to decode salt: %w", err)
	}

	preparedPassword, err := preparePasswordForProtocol(creds.Password, initResp.Protocol)
	if err != nil {
		return err
	}
	derivedPassword := pbkdf2.Key(preparedPassword, salt, initResp.Iteration, srpDerivedPasswordLen, sha256.New)

	serverB, err := base64.StdEncoding.DecodeString(initResp.ServerPubB)
	if err != nil {
		return fmt.Errorf("failed to decode server public B: %w", err)
	}

	m1, m2, err := calculateSRPProof(strings.TrimSpace(creds.Username), a, A, n, g, serverB, derivedPassword, salt)
	if err != nil {
		return fmt.Errorf("failed to calculate SRP proof: %w", err)
	}

	hashcash, err := getHashcash(ctx, client, serviceKey)
	if err != nil {
		return fmt.Errorf("failed to compute hashcash: %w", err)
	}

	if err := signinComplete(ctx, client, strings.TrimSpace(creds.Username), m1, m2, initResp.Challenge, serviceKey, hashcash); err != nil {
		return fmt.Errorf("signin complete failed: %w", err)
	}

	return nil
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

func calculateSRPProof(username string, a, A, n, g *big.Int, serverB, derivedPassword, salt []byte) (string, string, error) {
	bHex := hex.EncodeToString(serverB)
	saltHex := hex.EncodeToString(salt)
	aHex := numToHex(A)
	derivedPasswordHex := hex.EncodeToString(derivedPassword)

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

	kHex, err := shaHex(numToHex(S))
	if err != nil {
		return "", "", err
	}

	m1Hex, err := calcM(n, g, username, saltHex, aHex, bHex, kHex)
	if err != nil {
		return "", "", err
	}

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

func calcXHex(derivedPasswordHex, saltHex string) (*big.Int, error) {
	if _, err := hex.DecodeString(derivedPasswordHex); err != nil {
		return nil, fmt.Errorf("invalid derived password hex: %w", err)
	}
	if _, err := hex.DecodeString(saltHex); err != nil {
		return nil, fmt.Errorf("invalid salt hex: %w", err)
	}

	inner, err := shaHex("3a" + derivedPasswordHex)
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

	input := numToHex(hxor) + shaStringHex(username) + saltHex + aHex + bHex + kHex
	raw, err := hex.DecodeString(input)
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
	return numToHex(new(big.Int).SetBytes(sum[:])), nil
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

func getHashcash(ctx context.Context, client *http.Client, serviceKey string) (string, error) {
	endpoint := authServiceURL + "/signin?widgetKey=" + url.QueryEscape(serviceKey)
	req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	setModifiedCookieHeader(client, req)

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to fetch hashcash challenge (status %d)", resp.StatusCode)
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
// require explicit quotes for some Apple auth endpoints.
func setModifiedCookieHeader(client *http.Client, req *http.Request) {
	if client == nil || client.Jar == nil || req == nil || req.URL == nil {
		return
	}
	cookies := client.Jar.Cookies(req.URL)
	if len(cookies) == 0 {
		return
	}

	parts := make([]string, 0, len(cookies))
	for _, c := range cookies {
		if c == nil {
			continue
		}
		value := c.Value
		if strings.Contains(c.Name, "DES") && !strings.HasPrefix(value, "\"") {
			value = "\"" + value + "\""
		}
		parts = append(parts, c.Name+"="+value)
	}
	if len(parts) > 0 {
		req.Header.Set("Cookie", strings.Join(parts, "; "))
	}
}

func signinInit(ctx context.Context, client *http.Client, username, aBase64, serviceKey string) (*signinInitResponse, error) {
	reqBody := map[string]any{
		"accountName": username,
		"protocols":   []string{"s2k", "s2k_fo"},
		"a":           aBase64,
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal signin init payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", authServiceURL+"/signin/init", bytes.NewReader(body))
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
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read signin init response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("signin init failed with status %d", resp.StatusCode)
	}
	return parseSigninInitResponse(respBody)
}

func signinComplete(ctx context.Context, client *http.Client, username, m1, m2 string, challenge json.RawMessage, serviceKey, hashcash string) error {
	reqBody := struct {
		AccountName string          `json:"accountName"`
		RememberMe  bool            `json:"rememberMe"`
		M1          string          `json:"m1"`
		M2          string          `json:"m2"`
		C           json.RawMessage `json:"c"`
	}{
		AccountName: username,
		RememberMe:  false,
		M1:          m1,
		M2:          m2,
		C:           challenge,
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal signin complete payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", authServiceURL+"/signin/complete?isRememberMeEnabled=false", bytes.NewReader(body))
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
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read signin complete response: %w", err)
	}
	_ = respBody

	if resp.StatusCode == http.StatusOK {
		return nil
	}
	if resp.StatusCode == http.StatusConflict {
		return &TwoFactorRequiredError{
			AppleIDSessionID: strings.TrimSpace(resp.Header.Get("X-Apple-ID-Session-Id")),
			SCNT:             strings.TrimSpace(resp.Header.Get("scnt")),
		}
	}
	return fmt.Errorf("signin complete failed with status %d", resp.StatusCode)
}

func getSessionInfo(ctx context.Context, client *http.Client) (*sessionInfo, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", olympusSessionURL, nil)
	if err != nil {
		return nil, err
	}
	setModifiedCookieHeader(client, req)

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read session info response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get session info with status %d", resp.StatusCode)
	}

	var result sessionInfo
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to decode session info: %w", err)
	}
	return &result, nil
}

func appleSessionHeaders(session *AuthSession) http.Header {
	header := make(http.Header)
	header.Set("X-Apple-ID-Session-Id", session.AppleIDSessionID)
	header.Set("X-Apple-Widget-Key", session.ServiceKey)
	header.Set("scnt", session.SCNT)
	return header
}

func getAuthOptions(ctx context.Context, session *AuthSession) (*authOptionsResponse, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", authServiceURL, nil)
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
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read auth options response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("auth options failed with status %d", resp.StatusCode)
	}

	var result authOptionsResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse auth options: %w", err)
	}
	return &result, nil
}

func submitTrustedDeviceCode(ctx context.Context, session *AuthSession, code string) error {
	payload := map[string]any{
		"securityCode": map[string]string{"code": code},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal trusted-device payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", authServiceURL+"/verify/trusteddevice/securitycode", bytes.NewReader(body))
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
	defer func() { _ = resp.Body.Close() }()
	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	return &twoFAVerificationFailedError{Kind: "trusted-device", Status: resp.StatusCode, Body: respBody}
}

func submitPhoneCode(ctx context.Context, session *AuthSession, code string, phoneID int, mode string) error {
	payload := map[string]any{
		"securityCode": map[string]string{"code": code},
		"phoneNumber":  map[string]int{"id": phoneID},
		"mode":         mode,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal phone payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", authServiceURL+"/verify/phone/securitycode", bytes.NewReader(body))
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
	defer func() { _ = resp.Body.Close() }()
	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	return &twoFAVerificationFailedError{Kind: "phone", Status: resp.StatusCode, Body: respBody}
}

func finalizeTwoFactor(ctx context.Context, session *AuthSession) error {
	req, err := http.NewRequestWithContext(ctx, "GET", authServiceURL+"/2sv/trust", nil)
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
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read 2fa trust response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("2fa trust failed with status %d", resp.StatusCode)
	}
	_ = body

	info, err := getSessionInfo(ctx, session.Client)
	if err != nil {
		return err
	}
	session.ProviderID = info.Provider.ProviderID
	session.TeamID = fmt.Sprintf("%d", info.Provider.ProviderID)
	session.UserEmail = strings.TrimSpace(info.User.EmailAddress)
	return nil
}

// SubmitTwoFactorCode completes a pending 2FA challenge for an existing session.
func SubmitTwoFactorCode(ctx context.Context, session *AuthSession, code string) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if session == nil || session.Client == nil {
		return fmt.Errorf("session is required")
	}
	code = strings.TrimSpace(code)
	if code == "" {
		return fmt.Errorf("2fa code is required")
	}
	if session.ServiceKey == "" || session.AppleIDSessionID == "" || session.SCNT == "" {
		return fmt.Errorf("session is missing 2fa continuation state")
	}

	opts, err := getAuthOptions(ctx, session)
	if err != nil {
		return err
	}

	if err := submitTrustedDeviceCode(ctx, session, code); err == nil {
		return finalizeTwoFactor(ctx, session)
	}

	if len(opts.TrustedPhoneNumbers) > 0 {
		phone := opts.TrustedPhoneNumbers[0]
		mode := strings.TrimSpace(phone.PushMode)
		if mode == "" {
			mode = "sms"
		}
		if err := submitPhoneCode(ctx, session, code, phone.ID, mode); err != nil {
			return err
		}
		return finalizeTwoFactor(ctx, session)
	}

	return fmt.Errorf("2fa failed: no supported verification method found")
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
			codes = append(codes, strings.TrimSpace(serviceErr.Code))
		}
	}
	return codes
}

func (c *Client) doRequest(ctx context.Context, method, path string, body any) ([]byte, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	var reqBody io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		reqBody = bytes.NewReader(jsonBody)
	}

	fullURL := c.baseURL + path
	req, err := http.NewRequestWithContext(ctx, method, fullURL, reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	req.Header.Set("Origin", appStoreBaseURL)
	req.Header.Set("Referer", appStoreBaseURL+"/")
	setModifiedCookieHeader(c.httpClient, req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	appleRequestID := strings.TrimSpace(resp.Header.Get("X-Apple-Request-Uuid"))
	if appleRequestID == "" {
		appleRequestID = strings.TrimSpace(resp.Header.Get("X-Apple-Request-UUID"))
	}
	correlationKey := strings.TrimSpace(resp.Header.Get("X-Apple-Jingle-Correlation-Key"))

	if resp.StatusCode >= 400 {
		return nil, &APIError{
			Status:         resp.StatusCode,
			AppleRequestID: appleRequestID,
			CorrelationKey: correlationKey,
			rawBody:        respBody,
		}
	}
	return respBody, nil
}
