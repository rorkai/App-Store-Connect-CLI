package asc

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/golang-jwt/jwt/v5"
)

const (
	enterpriseCertificatesURL = EnterpriseBaseURL + "/v1/certificates"
	ascCertificatesURL        = BaseURL + "/v1/certificates"
)

func requestAudience(t *testing.T, req *http.Request) []string {
	t.Helper()
	token := strings.TrimPrefix(req.Header.Get("Authorization"), "Bearer ")
	parsed, _, err := jwt.NewParser().ParseUnverified(token, jwt.MapClaims{})
	if err != nil {
		t.Fatalf("ParseUnverified() error: %v", err)
	}
	audience, err := parsed.Claims.GetAudience()
	if err != nil {
		t.Fatalf("GetAudience() error: %v", err)
	}
	return audience
}

func TestEnterpriseClientUsesEnterpriseHostAudienceAndNextLinks(t *testing.T) {
	var requested []string
	client := newTestClient(t, func(req *http.Request) {
		requested = append(requested, req.URL.String())
		audience := requestAudience(t, req)
		if len(audience) != 1 || audience[0] != "apple-developer-enterprise-v1" {
			t.Fatalf("audience = %v", audience)
		}
	},
		jsonResponse(http.StatusOK, `{"data":[{"type":"certificates","id":"1"}],"links":{"next":"`+enterpriseCertificatesURL+`?cursor=two"}}`),
		jsonResponse(http.StatusOK, `{"data":[{"type":"certificates","id":"2"}]}`),
	)
	client.UseEnterpriseProgramAPI()

	ctx := context.Background()
	first, err := client.GetCertificates(ctx)
	if err != nil {
		t.Fatalf("GetCertificates() error: %v", err)
	}
	all, err := PaginateAll(ctx, first, func(ctx context.Context, nextURL string) (PaginatedResponse, error) {
		return client.GetCertificates(ctx, NextURL[CertificatesOption](nextURL))
	})
	if err != nil {
		t.Fatalf("PaginateAll() error: %v", err)
	}

	if got := len(all.(*CertificatesResponse).Data); got != 2 {
		t.Fatalf("certificates = %d, want 2", got)
	}
	want := []string{enterpriseCertificatesURL, enterpriseCertificatesURL + "?cursor=two"}
	if strings.Join(requested, " ") != strings.Join(want, " ") {
		t.Fatalf("requested = %v, want %v", requested, want)
	}
}

func TestClientRejectsAbsoluteURLOnTheOtherAPIHost(t *testing.T) {
	tests := []struct {
		name       string
		enterprise bool
		target     string
	}{
		{name: "app store connect client", target: enterpriseCertificatesURL + "?cursor=two"},
		{name: "enterprise client", enterprise: true, target: ascCertificatesURL + "?cursor=two"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := newTestClient(t, func(req *http.Request) {
				t.Fatalf("unexpected request to %s", req.URL)
			}, jsonResponse(http.StatusOK, `{"data":[]}`))
			if test.enterprise {
				client.UseEnterpriseProgramAPI()
			}

			_, err := client.GetCertificates(context.Background(), NextURL[CertificatesOption](test.target))
			if err == nil || !strings.Contains(err.Error(), "rejected request URL") {
				t.Fatalf("GetCertificates() error = %v", err)
			}
			if _, err := client.RawRequest(context.Background(), http.MethodGet, test.target, nil); err == nil || !strings.Contains(err.Error(), "rejected request URL") {
				t.Fatalf("RawRequest() error = %v", err)
			}
		})
	}
}

func TestPaginationURLIdentityTreatsEnterpriseLinksAsRelative(t *testing.T) {
	absolute := PaginationURLIdentity(enterpriseCertificatesURL + "?limit=10&cursor=two")
	relative := PaginationURLIdentity("/v1/certificates?cursor=two&limit=10")
	if absolute != relative {
		t.Fatalf("identity = %q, want %q", absolute, relative)
	}
}

func TestEnterpriseClientRejectsOperationsOutsideTheAPIBeforeSending(t *testing.T) {
	tests := []struct {
		name   string
		method string
		path   string
	}{
		{name: "unknown resource", method: http.MethodGet, path: "/v1/apps"},
		{name: "unknown method on known path", method: http.MethodPatch, path: "/v1/certificates/CERT"},
		{name: "relationship read", method: http.MethodGet, path: "/v1/profiles/PROFILE/relationships/devices"},
		{name: "visible apps", method: http.MethodGet, path: "/v1/users/USER/visibleApps"},
		{name: "absolute next link", method: http.MethodGet, path: EnterpriseBaseURL + "/v1/apps?cursor=two"},
		{name: "trailing slash", method: http.MethodGet, path: "/v1/devices/"},
		{name: "encoded separator", method: http.MethodGet, path: EnterpriseBaseURL + "/v1/devices%2FDEVICE"},
		{name: "dot segment", method: http.MethodGet, path: EnterpriseBaseURL + "/v1/profiles/../devices"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := newTestClient(t, func(req *http.Request) {
				t.Fatalf("unexpected request to %s", req.URL)
			}, jsonResponse(http.StatusOK, `{"data":[]}`))
			client.UseEnterpriseProgramAPI()

			_, err := client.do(context.Background(), test.method, test.path, nil)
			if !errors.Is(err, ErrEnterpriseProgramAPIUnsupported) {
				t.Fatalf("do() error = %v", err)
			}
		})
	}
}

func TestEnterpriseClientAllowsListedOperationsWithIDAndQuery(t *testing.T) {
	var requested string
	client := newTestClient(t, func(req *http.Request) {
		requested = req.Method + " " + req.URL.RequestURI()
	}, jsonResponse(http.StatusOK, `{"data":[]}`))
	client.UseEnterpriseProgramAPI()

	if _, err := client.do(context.Background(), http.MethodGet, "/v1/profiles/PROFILE/devices?limit=1", nil); err != nil {
		t.Fatalf("do() error: %v", err)
	}
	if requested != "GET /v1/profiles/PROFILE/devices?limit=1" {
		t.Fatalf("requested = %q", requested)
	}
}

func TestEnterpriseClientRefusesNotarization(t *testing.T) {
	client := newTestClient(t, func(req *http.Request) {
		t.Fatalf("unexpected request to %s", req.URL)
	}, jsonResponse(http.StatusOK, `{"data":[]}`))
	client.UseEnterpriseProgramAPI()

	if _, err := client.ListNotarizations(context.Background()); !errors.Is(err, ErrEnterpriseProgramAPIUnsupported) {
		t.Fatalf("ListNotarizations() error = %v", err)
	}
}

func TestEnterpriseClientDoesNotFollowRedirects(t *testing.T) {
	var requests int
	client := newTestClient(t, func(*http.Request) { requests++ }, &http.Response{
		Status:     "307 Temporary Redirect",
		StatusCode: http.StatusTemporaryRedirect,
		Header:     http.Header{"Location": []string{"http://api.enterprise.developer.apple.com/v1/certificates"}},
		Body:       http.NoBody,
	})
	client.UseEnterpriseProgramAPI()

	if _, err := client.GetCertificates(context.Background()); err == nil {
		t.Fatal("GetCertificates() succeeded on a redirect")
	}
	if requests != 1 {
		t.Fatalf("requests = %d, want 1", requests)
	}
}

func TestClientRejectsAlternateSpellingsOfTheOtherHost(t *testing.T) {
	tests := []struct {
		name       string
		enterprise bool
		target     string
	}{
		{name: "uppercase", target: "https://API.ENTERPRISE.DEVELOPER.APPLE.COM/v1/certificates"},
		{name: "trailing dot", target: "https://api.enterprise.developer.apple.com./v1/certificates"},
		{name: "explicit port", target: "https://api.enterprise.developer.apple.com:443/v1/certificates"},
		{name: "userinfo", target: "https://user@api.appstoreconnect.apple.com/v1/certificates"},
		{name: "cleartext enterprise", enterprise: true, target: "http://api.enterprise.developer.apple.com/v1/certificates"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := newTestClient(t, func(req *http.Request) {
				t.Fatalf("unexpected request to %s", req.URL)
			}, jsonResponse(http.StatusOK, `{"data":[]}`))
			if test.enterprise {
				client.UseEnterpriseProgramAPI()
			}

			if _, err := client.do(context.Background(), http.MethodGet, test.target, nil); err == nil || !strings.Contains(err.Error(), "rejected request URL") {
				t.Fatalf("do() error = %v", err)
			}
		})
	}
}
