package certificates

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"math/big"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/artifacts"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

var developerIDCheckTestNow = time.Date(2026, time.October, 31, 12, 0, 0, 0, time.UTC)

func developerIDCertificateContent(t *testing.T, unit string, notAfter time.Time) string {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	issuer := pkix.Name{CommonName: artifacts.DeveloperIDAuthorityCommonName, OrganizationalUnit: []string{unit}}
	parent := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: issuer}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: "Developer ID Application: Example Corp (ABCDE12345)"},
		NotBefore:    notAfter.AddDate(-5, 0, 0),
		NotAfter:     notAfter,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, parent, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	return base64.StdEncoding.EncodeToString(der)
}

func TestClassifyDeveloperIDCertificate(t *testing.T) {
	const previous = artifacts.DeveloperIDPreviousAuthorityUnit
	const current = artifacts.DeveloperIDCurrentAuthorityUnit
	onCutoff := time.Date(2027, time.February, 1, 22, 12, 15, 0, time.UTC)
	for _, test := range []struct {
		name            string
		attributes      asc.CertificateAttributes
		wantStatus      string
		wantEvidence    string
		wantExpired     bool
		wantIssuerUnit  string
		wantExpiration  string
		wantNeedsAction bool
	}{
		{
			name:           "previous issuer",
			attributes:     asc.CertificateAttributes{CertificateType: "DEVELOPER_ID_APPLICATION", CertificateContent: developerIDCertificateContent(t, previous, onCutoff), ExpirationDate: "2027-02-01T22:12:15.000+00:00"},
			wantStatus:     developerIDStatusAffected,
			wantEvidence:   developerIDEvidenceIssuer,
			wantIssuerUnit: previous, wantExpiration: "2027-02-01T22:12:15.000+00:00", wantNeedsAction: true,
		},
		{
			name:           "issuer outranks an expiration past the cutoff",
			attributes:     asc.CertificateAttributes{CertificateType: "DEVELOPER_ID_APPLICATION", CertificateContent: developerIDCertificateContent(t, previous, onCutoff.AddDate(1, 0, 0))},
			wantStatus:     developerIDStatusAffected,
			wantEvidence:   developerIDEvidenceIssuer,
			wantIssuerUnit: previous, wantExpiration: "2028-02-01T22:12:15Z", wantNeedsAction: true,
		},
		{
			name:           "G2 issuer outranks an expiration before the cutoff",
			attributes:     asc.CertificateAttributes{CertificateType: "DEVELOPER_ID_APPLICATION", CertificateContent: developerIDCertificateContent(t, current, onCutoff.AddDate(0, -1, 0))},
			wantStatus:     developerIDStatusNotAffected,
			wantEvidence:   developerIDEvidenceIssuer,
			wantIssuerUnit: current, wantExpiration: "2027-01-01T22:12:15Z",
		},
		{
			name:         "G2 type without content",
			attributes:   asc.CertificateAttributes{CertificateType: "DEVELOPER_ID_KEXT_G2", ExpirationDate: "2027-01-15T00:00:00Z"},
			wantStatus:   developerIDStatusNotAffected,
			wantEvidence: developerIDEvidenceCertificateType, wantExpiration: "2027-01-15T00:00:00Z",
		},
		{
			name:         "last instant of the cutoff day without content",
			attributes:   asc.CertificateAttributes{CertificateType: "DEVELOPER_ID_KEXT", ExpirationDate: "2027-02-01T23:59:59Z"},
			wantStatus:   developerIDStatusLikelyAffected,
			wantEvidence: developerIDEvidenceExpirationDate, wantExpiration: "2027-02-01T23:59:59Z", wantNeedsAction: true,
		},
		{
			name:         "day after the cutoff without content",
			attributes:   asc.CertificateAttributes{CertificateType: "DEVELOPER_ID_APPLICATION", ExpirationDate: "2027-02-02T00:00:00Z"},
			wantStatus:   developerIDStatusNotAffected,
			wantEvidence: developerIDEvidenceExpirationDate, wantExpiration: "2027-02-02T00:00:00Z",
		},
		{
			name:         "unreadable content falls back to the expiration date",
			attributes:   asc.CertificateAttributes{CertificateType: "DEVELOPER_ID_APPLICATION", CertificateContent: "not a certificate", ExpirationDate: "2027-01-15T00:00:00Z"},
			wantStatus:   developerIDStatusLikelyAffected,
			wantEvidence: developerIDEvidenceExpirationDate, wantExpiration: "2027-01-15T00:00:00Z", wantNeedsAction: true,
		},
		{
			name:         "nothing to judge by",
			attributes:   asc.CertificateAttributes{CertificateType: "DEVELOPER_ID_APPLICATION", ExpirationDate: "soon"},
			wantStatus:   developerIDStatusUnknown,
			wantEvidence: developerIDEvidenceNone, wantExpiration: "soon", wantNeedsAction: true,
		},
		{
			name:         "expired certificate is reported without asking for action",
			attributes:   asc.CertificateAttributes{CertificateType: "DEVELOPER_ID_APPLICATION", ExpirationDate: "2025-01-15T00:00:00Z"},
			wantStatus:   developerIDStatusLikelyAffected,
			wantEvidence: developerIDEvidenceExpirationDate, wantExpiration: "2025-01-15T00:00:00Z", wantExpired: true,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := buildDeveloperIDCheck([]asc.Resource[asc.CertificateAttributes]{{ID: "cert", Attributes: test.attributes}}, developerIDCheckTestNow)
			item := got.Certificates[0]
			if item.Status != test.wantStatus || item.Evidence != test.wantEvidence || item.Expired != test.wantExpired {
				t.Fatalf("status=%q evidence=%q expired=%t, want %q %q %t", item.Status, item.Evidence, item.Expired, test.wantStatus, test.wantEvidence, test.wantExpired)
			}
			if item.IssuerOrganizationalUnit != test.wantIssuerUnit || item.ExpirationDate != test.wantExpiration {
				t.Fatalf("issuer unit=%q expiration=%q, want %q %q", item.IssuerOrganizationalUnit, item.ExpirationDate, test.wantIssuerUnit, test.wantExpiration)
			}
			if got.ActionRequired != test.wantNeedsAction {
				t.Fatalf("actionRequired=%t, want %t", got.ActionRequired, test.wantNeedsAction)
			}
		})
	}
}

func TestBuildDeveloperIDCheckAuthorityClock(t *testing.T) {
	for _, test := range []struct {
		name        string
		now         time.Time
		wantDays    int
		wantExpired bool
	}{
		{name: "before", now: developerIDCheckTestNow, wantDays: 92},
		{name: "day before", now: time.Date(2027, time.January, 31, 0, 0, 0, 0, time.UTC), wantDays: 1},
		{name: "on the date", now: time.Date(2027, time.February, 1, 0, 0, 0, 0, time.UTC), wantExpired: true},
		{name: "after", now: time.Date(2027, time.March, 1, 0, 0, 0, 0, time.UTC), wantExpired: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := buildDeveloperIDCheck(nil, test.now)
			if got.DaysRemaining != test.wantDays || got.AuthorityExpired != test.wantExpired {
				t.Fatalf("days=%d expired=%t, want %d %t", got.DaysRemaining, got.AuthorityExpired, test.wantDays, test.wantExpired)
			}
		})
	}
}

func captureDeveloperIDCheckOutput(t *testing.T, run func() error) (string, string, error) {
	t.Helper()
	capture := func(target **os.File) func() string {
		reader, writer, err := os.Pipe()
		if err != nil {
			t.Fatal(err)
		}
		original := *target
		*target = writer
		done := make(chan string)
		go func() {
			var buffer bytes.Buffer
			_, _ = io.Copy(&buffer, reader)
			done <- buffer.String()
		}()
		return func() string {
			*target = original
			_ = writer.Close()
			return <-done
		}
	}
	stopStdout, stopStderr := capture(&os.Stdout), capture(&os.Stderr)
	err := run()
	return stopStdout(), stopStderr(), err
}

func runDeveloperIDCheck(t *testing.T, transport http.RoundTripper, args ...string) (string, string, error) {
	t.Helper()
	client := newCertificatesTestClient(t, transport)
	originalClient, originalNow := getCertificatesASCClient, developerIDCheckNow
	getCertificatesASCClient = func() (*asc.Client, error) { return client, nil }
	developerIDCheckNow = func() time.Time { return developerIDCheckTestNow }
	t.Cleanup(func() { getCertificatesASCClient, developerIDCheckNow = originalClient, originalNow })

	command := CertificatesDeveloperIDCheckCommand()
	if err := command.FlagSet.Parse(args); err != nil {
		t.Fatal(err)
	}
	return captureDeveloperIDCheckOutput(t, func() error {
		return command.Exec(context.Background(), command.FlagSet.Args())
	})
}

func developerIDCheckTransport(t *testing.T, requests *[]*http.Request, pages ...string) roundTripFunc {
	t.Helper()
	return func(request *http.Request) (*http.Response, error) {
		*requests = append(*requests, request)
		if len(*requests) > len(pages) {
			t.Fatalf("unexpected request: %s", request.URL)
		}
		return jsonHTTPResponse(http.StatusOK, pages[len(*requests)-1]), nil
	}
}

func TestCertificatesDeveloperIDCheckReportsEveryPage(t *testing.T) {
	const nextURL = "https://api.appstoreconnect.apple.com/v1/certificates?cursor=AQ&limit=200"
	first, err := json.Marshal(map[string]any{
		"data": []any{map[string]any{"type": "certificates", "id": "old", "attributes": map[string]any{
			"name": "Developer ID Application: Example", "displayName": "Example", "certificateType": "DEVELOPER_ID_APPLICATION",
			"serialNumber": "AB12", "expirationDate": "2027-02-01T22:12:15.000+00:00", "activated": true,
			"certificateContent": developerIDCertificateContent(t, artifacts.DeveloperIDPreviousAuthorityUnit, time.Date(2027, time.February, 1, 22, 12, 15, 0, time.UTC)),
		}}},
		"links": map[string]any{"next": nextURL},
	})
	if err != nil {
		t.Fatal(err)
	}
	second := `{"data":[{"type":"certificates","id":"new","attributes":{"certificateType":"DEVELOPER_ID_APPLICATION_G2","expirationDate":"2031-01-01T00:00:00Z"}}],"links":{}}`

	var requests []*http.Request
	stdout, stderr, err := runDeveloperIDCheck(t, developerIDCheckTransport(t, &requests, string(first), second), "--output", "json")
	if err != nil {
		t.Fatalf("Exec() error = %v; stderr=%q", err, stderr)
	}

	if len(requests) != 2 {
		t.Fatalf("expected 2 requests, got %d", len(requests))
	}
	query := requests[0].URL.Query()
	if requests[0].Method != http.MethodGet || requests[0].URL.Path != "/v1/certificates" || query.Get("limit") != "200" ||
		query.Get("filter[certificateType]") != "DEVELOPER_ID_APPLICATION,DEVELOPER_ID_APPLICATION_G2,DEVELOPER_ID_KEXT,DEVELOPER_ID_KEXT_G2" {
		t.Fatalf("unexpected first request: %s %s", requests[0].Method, requests[0].URL)
	}
	if requests[1].URL.String() != nextURL {
		t.Fatalf("unexpected second request: %s", requests[1].URL)
	}

	var report struct {
		AuthorityExpiresOn   string `json:"authorityExpiresOn"`
		AuthorityExpired     *bool  `json:"authorityExpired"`
		DaysRemaining        *int   `json:"daysRemaining"`
		ReplacementAuthority string `json:"replacementAuthority"`
		ActionRequired       *bool  `json:"actionRequired"`
		Summary              struct {
			Total, Affected, LikelyAffected, NotAffected, Unknown int
		} `json:"summary"`
		CheckedTypes []string `json:"checkedTypes"`
		NotChecked   []struct {
			Certificate string `json:"certificate"`
			Reason      string `json:"reason"`
		} `json:"notChecked"`
		Certificates []struct {
			ID                       string `json:"id"`
			Name                     string `json:"name"`
			DisplayName              string `json:"displayName"`
			CertificateType          string `json:"certificateType"`
			SerialNumber             string `json:"serialNumber"`
			ExpirationDate           string `json:"expirationDate"`
			Activated                *bool  `json:"activated"`
			Expired                  *bool  `json:"expired"`
			Status                   string `json:"status"`
			Evidence                 string `json:"evidence"`
			IssuerCommonName         string `json:"issuerCommonName"`
			IssuerOrganizationalUnit string `json:"issuerOrganizationalUnit"`
		} `json:"certificates"`
	}
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatalf("parse stdout %q: %v", stdout, err)
	}
	if report.AuthorityExpiresOn != "2027-02-01" || report.AuthorityExpired == nil || *report.AuthorityExpired ||
		report.DaysRemaining == nil || *report.DaysRemaining != 92 || report.ReplacementAuthority != "Developer ID Certification Authority (G2)" ||
		report.ActionRequired == nil || !*report.ActionRequired {
		t.Fatalf("unexpected report header: %s", stdout)
	}
	if report.Summary.Total != 2 || report.Summary.Affected != 1 || report.Summary.NotAffected != 1 || report.Summary.LikelyAffected != 0 || report.Summary.Unknown != 0 {
		t.Fatalf("unexpected summary: %+v", report.Summary)
	}
	if len(report.CheckedTypes) != 4 {
		t.Fatalf("unexpected checked types: %v", report.CheckedTypes)
	}
	if len(report.NotChecked) != 1 || report.NotChecked[0].Certificate != "Developer ID Installer" || !strings.Contains(report.NotChecked[0].Reason, "asc pkg-info") {
		t.Fatalf("Installer certificates must be reported as not checked: %+v", report.NotChecked)
	}
	if len(report.Certificates) != 2 {
		t.Fatalf("expected both pages, got %+v", report.Certificates)
	}
	old := report.Certificates[0]
	if old.ID != "old" || old.Name != "Developer ID Application: Example" || old.DisplayName != "Example" ||
		old.CertificateType != "DEVELOPER_ID_APPLICATION" || old.SerialNumber != "AB12" || old.ExpirationDate != "2027-02-01T22:12:15.000+00:00" ||
		old.Activated == nil || !*old.Activated || old.Expired == nil || *old.Expired || old.Status != "affected" || old.Evidence != "issuer" ||
		old.IssuerCommonName != "Developer ID Certification Authority" || old.IssuerOrganizationalUnit != "Apple Certification Authority" {
		t.Fatalf("unexpected certificate: %+v", old)
	}
	if !strings.Contains(stderr, "Warning: certificates developer-id-check: 1 certificate(s)") {
		t.Fatalf("expected a warning on stderr, got %q", stderr)
	}
	if strings.Contains(stdout, "certificateContent") {
		t.Fatalf("certificate content must not be echoed: %s", stdout)
	}
}

func TestCertificatesDeveloperIDCheckExitBehavior(t *testing.T) {
	const affected = `{"data":[{"type":"certificates","id":"a","attributes":{"certificateType":"DEVELOPER_ID_APPLICATION","expirationDate":"2027-01-15T00:00:00Z"}}],"links":{}}`
	const clear = `{"data":[{"type":"certificates","id":"c","attributes":{"certificateType":"DEVELOPER_ID_APPLICATION_G2","expirationDate":"2031-01-15T00:00:00Z"}}],"links":{}}`
	const expired = `{"data":[{"type":"certificates","id":"e","attributes":{"certificateType":"DEVELOPER_ID_APPLICATION","expirationDate":"2025-01-15T00:00:00Z"}}],"links":{}}`
	const empty = `{"data":[],"links":{}}`
	for _, test := range []struct {
		name        string
		body        string
		args        []string
		wantErr     bool
		wantWarning bool
	}{
		{name: "findings exit zero by default", body: affected, wantWarning: true},
		{name: "strict fails on findings", body: affected, args: []string{"--strict"}, wantErr: true, wantWarning: true},
		{name: "strict passes when clear", body: clear, args: []string{"--strict"}},
		{name: "strict passes on expired certificates", body: expired, args: []string{"--strict"}},
		{name: "strict passes on an empty listing", body: empty, args: []string{"--strict"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			var requests []*http.Request
			args := append([]string{"--output", "json"}, test.args...)
			stdout, stderr, err := runDeveloperIDCheck(t, developerIDCheckTransport(t, &requests, test.body), args...)
			if (err != nil) != test.wantErr {
				t.Fatalf("Exec() error = %v, wantErr %t", err, test.wantErr)
			}
			if !json.Valid([]byte(stdout)) {
				t.Fatalf("the report must reach stdout before exiting: %q", stdout)
			}
			if strings.Contains(stderr, "Warning:") != test.wantWarning {
				t.Fatalf("stderr = %q, wantWarning %t", stderr, test.wantWarning)
			}
		})
	}
}

func TestCertificatesDeveloperIDCheckAPIFailurePrintsNoReport(t *testing.T) {
	transport := roundTripFunc(func(*http.Request) (*http.Response, error) {
		return jsonHTTPResponse(http.StatusForbidden, `{"errors":[{"status":"403","code":"FORBIDDEN","title":"Forbidden"}]}`), nil
	})
	stdout, _, err := runDeveloperIDCheck(t, transport, "--output", "json")
	if err == nil || !strings.Contains(err.Error(), "certificates developer-id-check:") {
		t.Fatalf("expected a command error, got %v", err)
	}
	if stdout != "" {
		t.Fatalf("a failed listing must not print a report: %q", stdout)
	}
}

func TestCertificatesDeveloperIDCheckTableNamesWhatItCannotSee(t *testing.T) {
	var requests []*http.Request
	stdout, _, err := runDeveloperIDCheck(t, developerIDCheckTransport(t, &requests, `{"data":[],"links":{}}`), "--output", "table")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout, "Developer ID Installer") {
		t.Fatalf("table output must name Installer certificates as not checked: %q", stdout)
	}
}

func TestCertificatesDeveloperIDCheckRejectsPositionalArguments(t *testing.T) {
	command := CertificatesDeveloperIDCheckCommand()
	if err := command.Exec(context.Background(), []string{"extra"}); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected a usage error, got %v", err)
	}
}
