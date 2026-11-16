package cmdtest

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	webcmd "github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/web"
	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

const businessAgreementsFixture = `{"constraints":{},"messages":[],"extra":"preserved","data":{"agreements":[{"contractId":"contract-old","contractDisplayName":"Paid Applications","configId":"config-old","configVersion":12,"configIsLatestVersion":false,"availableNewContractConfigId":"config-new","availableNewConfigLocales":["en-US"],"isInEffect":true,"status":"ITC.ATB.Agmnt.Status.ActiveNewAgreementAvailable","legalEntity":{"legalEntityId":"entity-test","readyForSignAgreement":true},"applicableActions":{"view":["ITC.ATB.Agmnt.Action.view.AgreeToTerms"]}}],"messages":[{"messageKey":"agreement.updated","type":"warning"}]}}`

const businessTermsFixture = `{"constraints":{"$[0].agreeToTerms":{"attributeName":"agreeToTerms","required":true,"isEditable":true,"sizeType":"chars"}},"messages":[],"data":{"agreeToTerms":false,"agreeToFamilyShare":false,"agreementHTML":"<p>Example agreement terms</p>","contractConfigId":"config-new","providerPublicId":"provider-public","selectedLocaleCode":"en-US","availableLocales":["en-US"],"termFilePublicId":"file-test","auxStorageInfoTOs":[]}}`

const businessFreshTwoFactorFixture = `{"errors":[{"id":"private-example-marker","status":"401","code":"NOT_AUTHORIZED.FRESH_2FA_REQUIRED","title":"private-example-marker","detail":"private-example-marker"}]}`

// This transition is synthetic. The current-agreement fields are captured API
// fields; this is not an observed Paid Applications acceptance response.
const businessAcceptedFixture = `{"constraints":{},"messages":[],"data":{"agreements":[{"contractId":"contract-new","contractDisplayName":"Paid Applications","configId":"config-new","configVersion":13,"configIsLatestVersion":true,"availableNewContractConfigId":"","isInEffect":true,"status":"ITC.ATB.Agmnt.Status.Active","legalEntity":{"legalEntityId":"entity-test"}}],"messages":[]}}`

func installBusinessAgreementsSession(t *testing.T, publicProviderID string, handler http.HandlerFunc) {
	t.Helper()
	home := setCmdtestHome(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(home, "config.json"))
	t.Setenv("ASC_WEB_MIN_REQUEST_INTERVAL", "0")
	for _, name := range []string{"ASC_PROFILE", "ASC_KEY_ID", "ASC_ISSUER_ID", "ASC_PRIVATE_KEY_PATH", "ASC_PRIVATE_KEY", "ASC_PRIVATE_KEY_B64", "ASC_STRICT_AUTH"} {
		t.Setenv(name, "")
	}
	t.Cleanup(webcmd.SetResolveWebSession(func(context.Context, string, string, string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{
			ProviderID: 123, PublicProviderID: publicProviderID,
			Client: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				w := httptest.NewRecorder()
				handler.ServeHTTP(w, req)
				return w.Result(), nil
			})},
		}, "cache", nil
	}))
}

func TestWebBusinessAgreementsList(t *testing.T) {
	for _, tc := range []struct {
		name, response, output, wantError string
	}{
		{"complete JSON envelope", businessAgreementsFixture, "json", ""},
		{"pending update table", businessAgreementsFixture, "table", ""},
		{"missing agreements", `{"data":{}}`, "json", "missing agreements"},
		{"null agreements", `{"data":{"agreements":null}}`, "json", "missing agreements"},
		{"malformed agreements", `{"data":{"agreements":{}}}`, "json", "malformed response"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			installBusinessAgreementsSession(t, "provider-public", func(w http.ResponseWriter, req *http.Request) {
				calls++
				if req.Method != http.MethodGet || req.URL.String() != "https://appstoreconnect.apple.com/WebObjects/iTunesConnect.woa/ra/v1/contentProviders/provider-public/agreements" || req.Header.Get("X-CSRF-ITC") != "agreements-ui" || req.Header.Get("X-Connect-Team-ID") != "provider-public" || req.Header.Get("X-Connect-Team-Type") != "PURPLESOFTWARE" {
					t.Errorf("unexpected request: %s %s csrf=%q", req.Method, req.URL, req.Header.Get("X-CSRF-ITC"))
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(tc.response))
			})
			stdout, stderr, code := runWithDeps(t, nil, "web", "business", "agreements", "list", "--output", tc.output)
			if tc.wantError != "" {
				if code != cmd.ExitError || stdout != "" || !strings.Contains(stderr, tc.wantError) || calls != 1 {
					t.Fatalf("code=%d stdout=%q stderr=%q calls=%d", code, stdout, stderr, calls)
				}
				return
			}
			if code != cmd.ExitSuccess || stderr != "" || calls != 1 {
				t.Fatalf("code=%d stderr=%q calls=%d", code, stderr, calls)
			}
			if tc.output == "json" {
				var got, want any
				if err := json.Unmarshal([]byte(stdout), &got); err != nil {
					t.Fatal(err)
				}
				if err := json.Unmarshal([]byte(tc.response), &want); err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(got, want) {
					t.Fatalf("response envelope changed: %s", stdout)
				}
			} else if !strings.Contains(stdout, "Contract ID") || !strings.Contains(stdout, "New Configuration ID") || !strings.Contains(stdout, "ActiveNewAgreementAvailable") || !strings.Contains(stdout, "config-new") {
				t.Fatalf("table lost pending configuration or columns: %s", stdout)
			}
		})
	}
}

func TestWebBusinessAgreementsAcceptValidation(t *testing.T) {
	setCmdtestHome(t)
	t.Cleanup(webcmd.SetResolveWebSession(func(context.Context, string, string, string) (*webcore.AuthSession, string, error) {
		t.Error("session resolution before validating acceptance flags")
		return nil, "", nil
	}))
	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"--confirm"}, "--contract-config-id is required"},
		{[]string{"--contract-config-id", "config-new"}, "--confirm is required"},
		{[]string{"--contract-config-id", "config-new", "--confirm", "--pretty", "--output", "table"}, "--pretty"},
	} {
		assertUsageExitWithDeps(t, nil, append([]string{"web", "business", "agreements", "accept"}, tc.args...), tc.want)
	}
}

func TestWebBusinessAgreementsAcceptPreflightAndSingleWrite(t *testing.T) {
	var duplicate map[string]any
	if err := json.Unmarshal([]byte(businessAgreementsFixture), &duplicate); err != nil {
		t.Fatal(err)
	}
	data := duplicate["data"].(map[string]any)
	rows := data["agreements"].([]any)
	data["agreements"] = append(rows, rows[0])
	duplicateJSON, err := json.Marshal(duplicate)
	if err != nil {
		t.Fatal(err)
	}
	ambiguousAccepted := strings.NewReplacer(
		`"configId":"config-old"`, `"configId":"config-new"`,
		`"configIsLatestVersion":false`, `"configIsLatestVersion":true`,
		`"availableNewContractConfigId":"config-new"`, `"availableNewContractConfigId":""`,
	).Replace(string(duplicateJSON))
	for _, tc := range []struct {
		name, list, terms, after, want, output, postBody string
		postStatus, readStatus                           int
		readOnly, freshTwoFactor                         bool
	}{
		{name: "one source-backed POST", want: "outcome is unknown", postStatus: http.StatusServiceUnavailable},
		{name: "fresh two factor required", want: "fresh 2FA challenge", postStatus: http.StatusUnauthorized, postBody: businessFreshTwoFactorFixture, freshTwoFactor: true},
		{name: "unrelated unauthorized", want: "outcome is unknown", postStatus: http.StatusUnauthorized},
		{name: "different machine code", want: "outcome is unknown", postStatus: http.StatusUnauthorized, postBody: strings.ReplaceAll(businessFreshTwoFactorFixture, "FRESH_2FA_REQUIRED", "FRESH_2FA_REQUIRED_OTHER")},
		{name: "fresh two factor code on other status", want: "outcome is unknown", postStatus: http.StatusForbidden, postBody: businessFreshTwoFactorFixture},
		{name: "default terms locale", list: strings.ReplaceAll(businessAgreementsFixture, `"availableNewConfigLocales":["en-US"]`, `"availableNewConfigLocales":[]`), want: "outcome is unknown", postStatus: http.StatusServiceUnavailable},
		{name: "no redirect replay", want: "outcome is unknown", postStatus: http.StatusTemporaryRedirect},
		{name: "configuration not outstanding", list: strings.ReplaceAll(businessAgreementsFixture, `"availableNewContractConfigId":"config-new"`, `"availableNewContractConfigId":"other-config"`), want: "not an outstanding configuration"},
		{name: "ambiguous configuration", list: string(duplicateJSON), want: "matches multiple agreements"},
		{name: "missing assigned entity", list: strings.ReplaceAll(businessAgreementsFixture, `"legalEntityId":"entity-test"`, `"legalEntityId":""`), want: "assigned legal entity"},
		{name: "entity not ready", list: strings.ReplaceAll(businessAgreementsFixture, `"readyForSignAgreement":true`, `"readyForSignAgreement":false`), want: "not ready"},
		{name: "no acceptance action", list: strings.ReplaceAll(businessAgreementsFixture, "Action.view.AgreeToTerms", "Action.view.ViewTerms"), want: "does not offer acceptance"},
		{name: "terms provider mismatch", terms: strings.ReplaceAll(businessTermsFixture, "provider-public", "other-provider"), want: "terms do not match"},
		{name: "terms configuration mismatch", terms: strings.ReplaceAll(businessTermsFixture, "config-new", "other-config"), want: "terms do not match"},
		{name: "terms locale mismatch", terms: strings.ReplaceAll(businessTermsFixture, "en-US", "fr-FR"), want: "terms do not match"},
		{name: "unsupported consent", terms: strings.ReplaceAll(businessTermsFixture, `"$[0].agreeToTerms"`, `"$[0].anotherConsent"`), want: "unsupported consent"},
		{name: "read only refusal", readOnly: true, want: "refusing POST"},
		{name: "synthetic verified receipt with replaced contract ID", after: businessAcceptedFixture, postStatus: http.StatusOK},
		{name: "synthetic verified table", after: businessAcceptedFixture, output: "table", postStatus: http.StatusOK},
		{name: "old configuration remains current", after: businessAgreementsFixture, postStatus: http.StatusOK, want: "unverified"},
		{name: "different entity accepted", after: strings.ReplaceAll(businessAcceptedFixture, "entity-test", "other-entity"), postStatus: http.StatusOK, want: "unverified"},
		{name: "ambiguous accepted configuration", after: ambiguousAccepted, postStatus: http.StatusOK, want: "unverified"},
		{name: "not latest", after: strings.ReplaceAll(businessAcceptedFixture, `"configIsLatestVersion":true`, `"configIsLatestVersion":false`), postStatus: http.StatusOK, want: "unverified"},
		{name: "latest unknown", after: strings.ReplaceAll(businessAcceptedFixture, `"configIsLatestVersion":true,`, ""), postStatus: http.StatusOK, want: "unverified"},
		{name: "not in effect", after: strings.ReplaceAll(businessAcceptedFixture, `"isInEffect":true`, `"isInEffect":false`), postStatus: http.StatusOK, want: "unverified"},
		{name: "effect unknown", after: strings.ReplaceAll(businessAcceptedFixture, `"isInEffect":true,`, ""), postStatus: http.StatusOK, want: "unverified"},
		{name: "replacement still outstanding", after: strings.ReplaceAll(businessAcceptedFixture, `"availableNewContractConfigId":""`, `"availableNewContractConfigId":"another-config"`), postStatus: http.StatusOK, want: "unverified"},
		{name: "replacement state unknown", after: strings.ReplaceAll(businessAcceptedFixture, `"availableNewContractConfigId":"",`, ""), postStatus: http.StatusOK, want: "unverified"},
		{name: "version absent", after: strings.ReplaceAll(businessAcceptedFixture, `"configVersion":13,`, ""), postStatus: http.StatusOK, want: "unverified"},
		{name: "malformed verification read", after: `{"data":{}}`, postStatus: http.StatusOK, want: "unverified"},
		{name: "failed verification read", postStatus: http.StatusOK, readStatus: http.StatusServiceUnavailable, want: "unverified"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.list == "" {
				tc.list = businessAgreementsFixture
			}
			if tc.terms == "" {
				tc.terms = businessTermsFixture
			}
			if tc.output == "" {
				tc.output = "json"
			}
			postCalls, listCalls := 0, 0
			installBusinessAgreementsSession(t, "provider-public", func(w http.ResponseWriter, req *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				switch req.Method + " " + req.URL.Path {
				case "GET /WebObjects/iTunesConnect.woa/ra/v1/contentProviders/provider-public/agreements":
					listCalls++
					if postCalls == 0 {
						_, _ = w.Write([]byte(tc.list))
					} else {
						if tc.readStatus != 0 {
							w.WriteHeader(tc.readStatus)
						}
						_, _ = w.Write([]byte(tc.after))
					}
				case "GET /WebObjects/iTunesConnect.woa/ra/v1/contentProviders/provider-public/agreementTerms/config-new/locales/en-US":
					_, _ = w.Write([]byte(tc.terms))
				case "POST /WebObjects/iTunesConnect.woa/ra/v1/contentProviders/provider-public/legalEntities/entity-test/agreementTerms/config-new/locales/en-US":
					postCalls++
					for key, want := range map[string]string{
						"Origin": "https://appstoreconnect.apple.com", "Accept": "application/json", "Content-Type": "application/json",
						"X-CSRF-ITC": "agreements-ui", "X-Connect-Team-Type": "PURPLESOFTWARE", "X-Connect-Team-ID": "provider-public",
					} {
						if got := req.Header.Get(key); got != want {
							t.Errorf("POST header %s=%q, want %q", key, got, want)
						}
					}
					var body map[string]any
					if err := json.NewDecoder(req.Body).Decode(&body); err != nil || !reflect.DeepEqual(body, map[string]any{"agreeToTerms": true, "agreeToFamilyShare": true}) {
						t.Errorf("unexpected consent body: %v error=%v", body, err)
					}
					if tc.postStatus == 0 {
						t.Error("POST after invalid agreement or terms preflight")
						w.WriteHeader(http.StatusInternalServerError)
						return
					}
					if tc.postStatus == http.StatusTemporaryRedirect {
						w.Header().Set("Location", "https://appstoreconnect.apple.com/redirected-consent")
					}
					w.WriteHeader(tc.postStatus)
					responseBody := tc.postBody
					if responseBody == "" {
						responseBody = `{"messages":[{"messageKey":"private-example-marker"}]}`
					}
					_, _ = w.Write([]byte(responseBody))
				default:
					t.Errorf("unexpected request: %s %s", req.Method, req.URL)
					w.WriteHeader(http.StatusInternalServerError)
				}
			})
			args := []string{"web", "business", "agreements", "accept", "--contract-config-id", "config-new", "--confirm", "--output", tc.output}
			if tc.readOnly {
				args = append([]string{"--read-only"}, args...)
			}
			stdout, stderr, code := runWithDeps(t, nil, args...)
			wantPosts := 0
			wantCode := cmd.ExitError
			if tc.postStatus != 0 {
				wantPosts = 1
			}
			if tc.postStatus >= 400 {
				wantCode = cmd.HTTPStatusToExitCode(tc.postStatus)
			}
			if tc.readStatus >= 400 {
				wantCode = cmd.HTTPStatusToExitCode(tc.readStatus)
			}
			if tc.readOnly {
				wantCode = cmd.ExitReadOnly
			}
			if tc.postStatus == http.StatusOK && listCalls != 2 {
				t.Errorf("verification list reads = %d, want one preflight and one verification", listCalls)
			}
			if tc.want == "" {
				if code != cmd.ExitSuccess || stderr != "" || postCalls != 1 {
					t.Fatalf("code=%d stdout=%q stderr=%q POSTs=%d", code, stdout, stderr, postCalls)
				}
				if tc.output == "table" {
					if !strings.Contains(stdout, "Configuration ID") || !strings.Contains(stdout, "Verified") || !strings.Contains(stdout, "contract-new") || !strings.Contains(stdout, "config-new") {
						t.Fatalf("missing receipt columns or values: %s", stdout)
					}
				} else {
					var got map[string]any
					if err := json.Unmarshal([]byte(stdout), &got); err != nil {
						t.Fatal(err)
					}
					want := map[string]any{"publicProviderId": "provider-public", "legalEntityId": "entity-test", "contractId": "contract-new", "contractConfigId": "config-new", "configVersion": float64(13), "status": "ITC.ATB.Agmnt.Status.Active", "verified": true}
					if !reflect.DeepEqual(got, want) {
						t.Fatalf("receipt does not reflect verified state: %#v", got)
					}
				}
				return
			}
			if code != wantCode || stdout != "" || !strings.Contains(stderr, tc.want) || strings.Contains(stderr, "private-example-marker") || postCalls != wantPosts {
				t.Fatalf("code=%d stdout=%q stderr=%q POSTs=%d", code, stdout, stderr, postCalls)
			}
			if tc.freshTwoFactor {
				for _, recovery := range []string{"asc web auth logout --apple-id EMAIL", "asc web auth login --apple-id EMAIL --public-provider-id PROVIDER_ID", "complete 2FA"} {
					if !strings.Contains(stderr, recovery) {
						t.Errorf("missing recovery step %q: %s", recovery, stderr)
					}
				}
				if strings.Contains(stderr, "web session is unauthorized or expired") || strings.Contains(stderr, "outcome is unknown") {
					t.Fatalf("fresh-2FA refusal received generic session guidance: %s", stderr)
				}
			} else if strings.Contains(stderr, "fresh 2FA challenge") {
				t.Fatalf("incorrect fresh-2FA classification: %s", stderr)
			}
		})
	}
}

func TestWebBusinessAgreementsAcceptRetainsCancellation(t *testing.T) {
	setCmdtestHome(t)
	t.Setenv("ASC_WEB_MIN_REQUEST_INTERVAL", "0")
	for _, cause := range []error{context.Canceled, context.DeadlineExceeded} {
		t.Run(cause.Error(), func(t *testing.T) {
			posts := 0
			client := webcore.NewClient(&webcore.AuthSession{
				PublicProviderID: "provider-public",
				Client: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
					if req.Method == http.MethodPost {
						posts++
						return nil, cause
					}
					body := businessTermsFixture
					if strings.HasSuffix(req.URL.Path, "/agreements") {
						body = businessAgreementsFixture
					}
					return webAgreementsJSONResponse(http.StatusOK, body), nil
				})},
			})
			result, err := client.AcceptBusinessAgreement(context.Background(), "config-new")
			if result != nil || !errors.Is(err, cause) || !strings.Contains(err.Error(), "outcome is unknown") || posts != 1 {
				t.Fatalf("result=%+v error=%v POSTs=%d", result, err, posts)
			}
		})
	}
}
