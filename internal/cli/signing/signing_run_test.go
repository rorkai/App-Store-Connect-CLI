package signing

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"math/big"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/bitrise-io/go-xcode/certificateutil"
	"go.mozilla.org/pkcs7"
	"howett.net/plist"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func TestSigningCommandRegistersSubcommands(t *testing.T) {
	for _, path := range [][]string{{"keychain", "install"}, {"resign"}, {"run"}, {"sync", "rotate-password"}} {
		command := SigningCommand()
		for _, name := range path {
			found := false
			for _, subcommand := range command.Subcommands {
				if subcommand != nil && subcommand.Name == name {
					command, found = subcommand, true
					break
				}
			}
			if !found {
				t.Fatalf("asc signing %s is not registered", strings.Join(path, " "))
			}
		}
	}
}

func TestInspectSigningRunInputs(t *testing.T) {
	fixture := newSigningRunFixture(t, signingRunFixtureOptions{})
	got, err := inspectSigningRunInputs(
		fixture.identity,
		[]byte(fixture.password),
		fixture.profile,
		fixture.roots,
		fixture.now,
	)
	if err != nil {
		t.Fatalf("inspectSigningRunInputs() error: %v", err)
	}
	if got.ProfileUUID != fixture.profileUUID || got.TeamID != fixture.teamID || got.BundleID != fixture.bundleID {
		t.Fatalf("unexpected inspection: %+v", got)
	}
	if len(got.ProvisionedDevices) != 1 {
		t.Fatalf("provisioned devices = %d, want 1", len(got.ProvisionedDevices))
	}
	if got.CertificateSHA256 == "" || got.ProfileSHA256 == "" {
		t.Fatalf("expected digests: %+v", got)
	}
}

func TestInspectSigningRunInputsRejectsIneligibleOrMismatchedInputs(t *testing.T) {
	tests := []struct {
		name    string
		options signingRunFixtureOptions
		mutate  func(*testing.T, *signingRunFixture)
		wantErr string
	}{
		{name: "wrong password", mutate: func(_ *testing.T, fixture *signingRunFixture) { fixture.password = "wrong" }, wantErr: "decode identity"},
		{name: "expired profile", options: signingRunFixtureOptions{profileExpired: true}, wantErr: "profile is expired"},
		{name: "development profile", options: signingRunFixtureOptions{getTaskAllow: true}, wantErr: "development profile"},
		{name: "no registered devices", options: signingRunFixtureOptions{noDevices: true}, wantErr: "registered devices"},
		{name: "enterprise profile", options: signingRunFixtureOptions{allDevices: true}, wantErr: "enterprise profile"},
		{name: "wrong platform", options: signingRunFixtureOptions{platforms: []string{"macOS"}}, wantErr: "iOS"},
		{name: "identity not embedded", options: signingRunFixtureOptions{differentEmbeddedCertificate: true}, wantErr: "not embedded"},
		{name: "identity team mismatch", options: signingRunFixtureOptions{certificateTeamID: "OTHERTEAM"}, wantErr: "organizational unit"},
		{name: "invalid wildcard", options: signingRunFixtureOptions{bundleID: "com.*.example"}, wantErr: "bundle identifier pattern"},
		{name: "untrusted cms signer", mutate: func(_ *testing.T, fixture *signingRunFixture) { fixture.roots = x509.NewCertPool() }, wantErr: "verify profile signature"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := newSigningRunFixture(t, test.options)
			if test.mutate != nil {
				test.mutate(t, fixture)
			}
			_, err := inspectSigningRunInputs(fixture.identity, []byte(fixture.password), fixture.profile, fixture.roots, fixture.now)
			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("error = %v, want %q", err, test.wantErr)
			}
		})
	}
}

func TestInspectSigningRunInputsAcceptsTerminalWildcard(t *testing.T) {
	for _, bundleID := range []string{"*", "com.example.*"} {
		t.Run(bundleID, func(t *testing.T) {
			fixture := newSigningRunFixture(t, signingRunFixtureOptions{bundleID: bundleID})
			got, err := inspectSigningRunInputs(fixture.identity, []byte(fixture.password), fixture.profile, fixture.roots, fixture.now)
			if err != nil {
				t.Fatalf("inspectSigningRunInputs() error: %v", err)
			}
			if got.BundleID != bundleID {
				t.Fatalf("bundle ID = %q, want %q", got.BundleID, bundleID)
			}
		})
	}
}

func TestSigningRunProfileIdentifiersRequireUniqueValues(t *testing.T) {
	if _, err := signingRunTeamID([]string{"TEAM12345", "TEAM12345"}); !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("duplicate teams error = %v, want duplicate rejection", err)
	}
	if _, err := signingRunApplicationIdentifierPrefixes([]string{"PREFIX", "PREFIX"}, "TEAM12345"); !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("duplicate prefixes error = %v, want duplicate rejection", err)
	}
	profile := signingRunMobileProvision{
		ApplicationIdentifierPrefix: []string{"LEGACY"},
		Entitlements:                map[string]any{"application-identifier": "LEGACY.com.example.app"},
	}
	if got, err := signingRunBundleID(profile, "TEAM12345"); err != nil || got != "com.example.app" {
		t.Fatalf("legacy prefix bundle ID = %q, error = %v, want accepted legacy prefix", got, err)
	}
	missingPrefix := profile
	missingPrefix.ApplicationIdentifierPrefix = nil
	if _, err := signingRunBundleID(missingPrefix, "TEAM12345"); !strings.Contains(err.Error(), "exactly one") {
		t.Fatalf("missing prefix error = %v, want exact-one rejection", err)
	}
	contradictory := profile
	contradictory.Entitlements = map[string]any{
		"application-identifier":           "LEGACY.com.example.app",
		"com.apple.application-identifier": "OTHER.com.example.app",
	}
	if _, err := signingRunBundleID(contradictory, "TEAM12345"); !strings.Contains(err.Error(), "contradictory") {
		t.Fatalf("contradictory identifiers error = %v, want rejection", err)
	}
	missingPrimary := profile
	missingPrimary.Entitlements = map[string]any{"com.apple.application-identifier": "LEGACY.com.example.app"}
	if _, err := signingRunBundleID(missingPrimary, "TEAM12345"); !strings.Contains(err.Error(), "application identifier is missing") {
		t.Fatalf("missing primary identifier error = %v, want rejection", err)
	}
}

func TestVerifySigningRunProfileTrustRejectsMultipleSigners(t *testing.T) {
	key, certificate := makeSigningRunCertificate(t, "Profile Signer", "", time.Now())
	signed, err := pkcs7.NewSignedData([]byte("fixture"))
	if err != nil {
		t.Fatalf("new signed data: %v", err)
	}
	if err := signed.AddSigner(certificate, key, pkcs7.SignerInfoConfig{}); err != nil {
		t.Fatalf("add first signer: %v", err)
	}
	if err := signed.AddSigner(certificate, key, pkcs7.SignerInfoConfig{}); err != nil {
		t.Fatalf("add second signer: %v", err)
	}
	data, err := signed.Finish()
	if err != nil {
		t.Fatalf("finish signed data: %v", err)
	}
	profile, err := pkcs7.Parse(data)
	if err != nil {
		t.Fatalf("parse signed data: %v", err)
	}
	if err := verifySigningRunProfileTrust(profile, nil, time.Now()); !strings.Contains(err.Error(), "exactly one CMS signer") {
		t.Fatalf("error = %v, want multiple-signer rejection", err)
	}
}

func TestVerifySigningRunProfileTrustRejectsNonAppleSignerAndUnpinnedRoot(t *testing.T) {
	fixture := newSigningRunFixture(t, signingRunFixtureOptions{})
	profile, err := pkcs7.Parse(fixture.profile)
	if err != nil {
		t.Fatalf("parse profile: %v", err)
	}
	if err := verifySigningRunProfileTrust(profile, nil, fixture.now); !strings.Contains(err.Error(), "Apple provisioning signer") {
		t.Fatalf("non-Apple signer error = %v, want signer rejection", err)
	}

	for _, certificate := range profile.Certificates {
		if certificate.Subject.CommonName != "Profile Signer" {
			continue
		}
		certificate.Subject.CommonName = "Apple iPhone OS Provisioning Profile Signing"
		certificate.Subject.Organization = []string{"Apple Inc."}
		certificate.Issuer.CommonName = "Apple iPhone Certification Authority"
		break
	}
	if err := verifySigningRunProfileTrust(profile, nil, fixture.now); !strings.Contains(err.Error(), "accepted Apple root") {
		t.Fatalf("unpinned root error = %v, want pinned-root rejection", err)
	}
}

type signingRunFixtureOptions struct {
	profileExpired               bool
	getTaskAllow                 bool
	noDevices                    bool
	allDevices                   bool
	codeSigningLeaf              bool
	selfSignedCodeSigningLeaf    bool
	platforms                    []string
	differentEmbeddedCertificate bool
	certificateTeamID            string
	bundleID                     string
}

type signingRunFixture struct {
	identity    []byte
	password    string
	profile     []byte
	roots       *x509.CertPool
	now         time.Time
	teamID      string
	bundleID    string
	profileUUID string
	trustAnchor []byte
}

func newSigningRunFixture(t *testing.T, options signingRunFixtureOptions) *signingRunFixture {
	t.Helper()
	previousTrustVerifier := signingRunProfileTrustFn
	signingRunProfileTrustFn = func(profile *pkcs7.PKCS7, roots *x509.CertPool, now time.Time) error {
		return profile.VerifyWithChainAtTime(roots, now)
	}
	t.Cleanup(func() { signingRunProfileTrustFn = previousTrustVerifier })
	now := time.Date(2026, time.August, 13, 12, 0, 0, 0, time.UTC)
	teamID := "TEAM12345"
	certificateTeamID := options.certificateTeamID
	if certificateTeamID == "" {
		certificateTeamID = teamID
	}
	bundleID := options.bundleID
	if bundleID == "" {
		bundleID = "com.example.app"
	}
	platforms := options.platforms
	if platforms == nil {
		platforms = []string{"iOS", "xrOS", "visionOS"}
	}
	profileExpiry := now.Add(24 * time.Hour)
	if options.profileExpired {
		profileExpiry = now.Add(-time.Hour)
	}

	var identityKey *rsa.PrivateKey
	var identityCert *x509.Certificate
	var trustAnchor []byte
	if options.codeSigningLeaf {
		identityKey, identityCert, trustAnchor = makeSigningRunCodeSigningLeafCertificate(t, "Distribution", certificateTeamID, now, options.selfSignedCodeSigningLeaf)
	} else {
		identityKey, identityCert = makeSigningRunCertificate(t, "Distribution", certificateTeamID, now)
	}
	identity, err := certificateutil.NewCertificateInfo(*identityCert, identityKey).EncodeToP12("secret")
	if err != nil {
		t.Fatalf("encode P12: %v", err)
	}
	embeddedCert := identityCert
	if options.differentEmbeddedCertificate {
		_, embeddedCert = makeSigningRunCertificate(t, "Other", teamID, now)
	}

	profileUUID := "A7EFEF21-3432-404F-A488-083800B570FF"
	devices := []string{"00008140-000104303633001C"}
	if options.noDevices {
		devices = nil
	}
	profilePlist, err := plist.Marshal(map[string]any{
		"UUID":                        profileUUID,
		"Name":                        "Release Testing",
		"Platform":                    platforms,
		"TeamIdentifier":              []string{teamID},
		"ApplicationIdentifierPrefix": []string{teamID},
		"CreationDate":                now.Add(-time.Hour),
		"ExpirationDate":              profileExpiry,
		"ProvisionedDevices":          devices,
		"ProvisionsAllDevices":        options.allDevices,
		"DeveloperCertificates":       [][]byte{embeddedCert.Raw},
		"Entitlements": map[string]any{
			"application-identifier":              teamID + "." + bundleID,
			"com.apple.developer.team-identifier": teamID,
			"get-task-allow":                      options.getTaskAllow,
		},
	}, plist.XMLFormat)
	if err != nil {
		t.Fatalf("marshal profile plist: %v", err)
	}

	cmsKey, cmsCert := makeSigningRunCertificate(t, "Profile Signer", "", now)
	signed, err := pkcs7.NewSignedData(profilePlist)
	if err != nil {
		t.Fatalf("new signed data: %v", err)
	}
	if err := signed.AddSigner(cmsCert, cmsKey, pkcs7.SignerInfoConfig{}); err != nil {
		t.Fatalf("add profile signer: %v", err)
	}
	profile, err := signed.Finish()
	if err != nil {
		t.Fatalf("finish profile CMS: %v", err)
	}
	roots := x509.NewCertPool()
	roots.AddCert(cmsCert)

	return &signingRunFixture{
		identity:    identity,
		password:    "secret",
		profile:     profile,
		roots:       roots,
		now:         now,
		teamID:      teamID,
		bundleID:    bundleID,
		profileUUID: profileUUID,
		trustAnchor: trustAnchor,
	}
}

func makeSigningRunCodeSigningLeafCertificate(t *testing.T, commonName, teamID string, now time.Time, selfSigned bool) (*rsa.PrivateKey, *x509.Certificate, []byte) {
	t.Helper()
	rootKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate test root key: %v", err)
	}
	rootSerial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 120))
	if err != nil {
		t.Fatalf("generate test root serial: %v", err)
	}
	rootTemplate := &x509.Certificate{
		SerialNumber:          rootSerial,
		Subject:               pkix.Name{CommonName: "ASC Signing Test Root"},
		NotBefore:             now.Add(-24 * time.Hour),
		NotAfter:              now.Add(365 * 24 * time.Hour),
		BasicConstraintsValid: true,
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageCodeSigning},
	}
	rootDER, err := x509.CreateCertificate(rand.Reader, rootTemplate, rootTemplate, &rootKey.PublicKey, rootKey)
	if err != nil {
		t.Fatalf("create test root certificate: %v", err)
	}
	rootCertificate, err := x509.ParseCertificate(rootDER)
	if err != nil {
		t.Fatalf("parse test root certificate: %v", err)
	}
	leafKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate test leaf key: %v", err)
	}
	leafSerial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 120))
	if err != nil {
		t.Fatalf("generate test leaf serial: %v", err)
	}
	leafTemplate := &x509.Certificate{
		SerialNumber:          leafSerial,
		Subject:               pkix.Name{CommonName: commonName, OrganizationalUnit: nonEmptyStrings(teamID)},
		NotBefore:             now.Add(-24 * time.Hour),
		NotAfter:              now.Add(365 * 24 * time.Hour),
		BasicConstraintsValid: true,
		IsCA:                  false,
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageCodeSigning},
	}
	if selfSigned {
		leafDER, err := x509.CreateCertificate(rand.Reader, leafTemplate, leafTemplate, &leafKey.PublicKey, leafKey)
		if err != nil {
			t.Fatalf("create self-signed test leaf certificate: %v", err)
		}
		leafCertificate, err := x509.ParseCertificate(leafDER)
		if err != nil {
			t.Fatalf("parse self-signed test leaf certificate: %v", err)
		}
		return leafKey, leafCertificate, nil
	}
	leafDER, err := x509.CreateCertificate(rand.Reader, leafTemplate, rootCertificate, &leafKey.PublicKey, rootKey)
	if err != nil {
		t.Fatalf("create test leaf certificate: %v", err)
	}
	leafCertificate, err := x509.ParseCertificate(leafDER)
	if err != nil {
		t.Fatalf("parse test leaf certificate: %v", err)
	}
	return leafKey, leafCertificate, rootDER
}

func makeSigningRunCertificate(t *testing.T, commonName, teamID string, now time.Time) (*rsa.PrivateKey, *x509.Certificate) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 120))
	if err != nil {
		t.Fatalf("serial: %v", err)
	}
	template := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: commonName, OrganizationalUnit: nonEmptyStrings(teamID)},
		NotBefore:             now.Add(-24 * time.Hour),
		NotAfter:              now.Add(365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageCodeSigning},
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create certificate: %v", err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatalf("parse certificate: %v", err)
	}
	return key, cert
}

func nonEmptyStrings(value string) []string {
	if value == "" {
		return nil
	}
	return []string{value}
}

func signingRunReceiptJSONKeys(t *testing.T, data []byte) []string {
	t.Helper()
	var object map[string]json.RawMessage
	if err := json.Unmarshal(data, &object); err != nil {
		t.Fatalf("decode receipt: %v", err)
	}
	keys := make([]string, 0, len(object))
	for key := range object {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func TestSigningRunReceiptOmitsSensitiveValues(t *testing.T) {
	receipt := signingRunReceipt{
		SchemaVersion:        1,
		Purpose:              signingRunPurposeReleaseTesting,
		Outcome:              "failed",
		ChildExitCode:        23,
		CertificateSHA256:    strings.Repeat("A", 64),
		ProfileSHA256:        strings.Repeat("B", 64),
		ProfileUUID:          "A7EFEF21-3432-404F-A488-083800B570FF",
		TeamID:               "TEAM12345",
		BundleID:             "com.example.app",
		ProfileCleanupState:  "removed",
		KeychainCleanupState: "deleted",
	}
	data, err := marshalSigningRunReceipt(receipt)
	if err != nil {
		t.Fatalf("marshalSigningRunReceipt: %v", err)
	}
	for _, forbidden := range []string{"secret", "00008140", "xcodebuild", "identityPassword", "childCommand", "devices"} {
		if strings.Contains(string(data), forbidden) {
			t.Fatalf("receipt contains %q: %s", forbidden, data)
		}
	}
	wantKeys := []string{"bundleId", "certificateSha256", "childExitCode", "keychainCleanupState", "outcome", "profileCleanupState", "profileSha256", "profileUuid", "purpose", "schemaVersion", "teamId"}
	if got := signingRunReceiptJSONKeys(t, data); !reflect.DeepEqual(got, wantKeys) {
		t.Fatalf("receipt keys = %v, want %v", got, wantKeys)
	}
}

func TestProcessExitErrorBounds(t *testing.T) {
	for _, test := range []struct {
		input int
		want  int
	}{{input: 42, want: 42}, {input: 130, want: 130}, {input: 0, want: 1}, {input: -1, want: 1}, {input: 256, want: 1}} {
		got, ok := shared.ProcessExitCode(shared.NewProcessExitError(test.input))
		if !ok {
			t.Fatalf("input %d did not return a process exit code", test.input)
		}
		if got != test.want {
			t.Fatalf("input %d returned exit %d, want %d", test.input, got, test.want)
		}
	}
}

func TestWithSigningRunInputDataClearsMutableInputs(t *testing.T) {
	wantFailure := errors.New("stop after observing inputs")
	for _, test := range []struct {
		name         string
		operationErr error
	}{{name: "success"}, {name: "operation failure", operationErr: wantFailure}} {
		t.Run(test.name, func(t *testing.T) {
			inputs := map[string][]byte{
				"identity": []byte("private-pkcs12"),
				"profile":  []byte("profile-with-device-identifiers"),
				"password": []byte("secret-password\r\n"),
			}
			err := withSigningRunInputData(
				signingRunOptions{IdentityPath: "identity", ProfilePath: "profile", IdentityPasswordPath: "password"},
				func(path string, _ int64, _ bool) ([]byte, error) { return inputs[path], nil },
				func(gotIdentity, gotPassword, gotProfile []byte) error {
					if string(gotIdentity) != "private-pkcs12" || string(gotPassword) != "secret-password" || string(gotProfile) != "profile-with-device-identifiers" {
						t.Fatalf("operation inputs were changed too early: identity=%q password=%q profile=%q", gotIdentity, gotPassword, gotProfile)
					}
					return test.operationErr
				},
			)
			if !errors.Is(err, test.operationErr) {
				t.Fatalf("error = %v, want %v", err, test.operationErr)
			}
			for name, data := range inputs {
				if !bytes.Equal(data, make([]byte, len(data))) {
					t.Fatalf("%s input was not cleared: %v", name, data)
				}
			}
		})
	}
}

func TestWithSigningRunInputDataClearsEarlierInputsOnReadFailure(t *testing.T) {
	identity := []byte("private-pkcs12")
	profileErr := errors.New("profile read failed")
	err := withSigningRunInputData(
		signingRunOptions{IdentityPath: "identity", ProfilePath: "profile"},
		func(path string, _ int64, _ bool) ([]byte, error) {
			if path == "identity" {
				return identity, nil
			}
			return nil, profileErr
		},
		func(_, _, _ []byte) error {
			t.Fatal("operation must not run after a read failure")
			return nil
		},
	)
	if !errors.Is(err, profileErr) {
		t.Fatalf("error = %v, want profile read failure", err)
	}
	if !bytes.Equal(identity, make([]byte, len(identity))) {
		t.Fatalf("identity input was not cleared after later read failure: %v", identity)
	}
}

func TestSanitizedChildEnvironmentFiltersSecretsAndInvalidEntries(t *testing.T) {
	base := []string{
		"PATH=/usr/bin",
		"ASC_PRIVATE_KEY=secret",
		"ASC_ISSUER_ID=issuer",
		"HOME=/Users/example",
		"PATH=/custom/bin",
		"MALFORMED",
		"LANG=en_US.UTF-8\x00unsafe",
		"TMPDIR=/tmp/asc",
	}

	got := SanitizedChildEnvironment(base)
	want := []string{"PATH=/custom/bin", "HOME=/Users/example", "TMPDIR=/tmp/asc"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("SanitizedChildEnvironment() = %#v, want %#v", got, want)
	}
}

func TestRunSigningEnvironmentOrdinarySetupAndCleanupFailuresRemainRootRenderable(t *testing.T) {
	fixture := newSigningRunFixture(t, signingRunFixtureOptions{})
	inspection, err := inspectSigningRunInputs(fixture.identity, []byte(fixture.password), fixture.profile, fixture.roots, fixture.now)
	if err != nil {
		t.Fatalf("inspect fixture: %v", err)
	}
	setupErr := errors.New("setup exploded")
	cleanupErr := errors.New("cleanup exploded")
	var stderr bytes.Buffer
	events := []string{}
	deps := fakeSigningRunDeps(&events)
	deps.Stderr = &stderr
	deps.CreateKeychain = func(context.Context, string, []byte) error { return setupErr }
	deps.RemoveKeychainSearchEntry = func(context.Context, string) error { return cleanupErr }
	_, runErr := runSigningEnvironment(context.Background(), deps, signingRunOptions{Child: []string{"tool"}}, fixture.profile, inspection, nil)
	if !errors.Is(runErr, setupErr) || !errors.Is(runErr, cleanupErr) {
		t.Fatalf("error = %v, want setup and cleanup causes", runErr)
	}
	if _, ok := shared.ProcessExitCode(runErr); ok {
		t.Fatalf("ordinary setup failure unexpectedly carries a child exit: %v", runErr)
	}
	var reported shared.ReportedError
	if errors.As(runErr, &reported) {
		t.Fatalf("ordinary failures must remain root-renderable: %v", runErr)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want root to render the joined error", stderr.String())
	}
	for _, text := range []string{"setup exploded", "cleanup exploded"} {
		if !strings.Contains(runErr.Error(), text) {
			t.Fatalf("error = %q, want %q", runErr, text)
		}
	}
}

func TestRunSigningEnvironmentPropagatesEarlyTempCleanupFailure(t *testing.T) {
	fixture := newSigningRunFixture(t, signingRunFixtureOptions{})
	inspection, err := inspectSigningRunInputs(fixture.identity, []byte(fixture.password), fixture.profile, fixture.roots, fixture.now)
	if err != nil {
		t.Fatalf("inspect fixture: %v", err)
	}
	setupErr := errors.New("search list unavailable")
	cleanupErr := errors.New("temporary directory cleanup failed")
	events := []string{}
	deps := fakeSigningRunDeps(&events)
	deps.KeychainSearchList = func(context.Context) ([]string, error) { return nil, setupErr }
	deps.RemoveTempDir = func(string) error { events = append(events, "remove-temp"); return cleanupErr }
	_, runErr := runSigningEnvironment(context.Background(), deps, signingRunOptions{Child: []string{"tool"}}, fixture.profile, inspection, nil)
	if !errors.Is(runErr, setupErr) || !errors.Is(runErr, cleanupErr) {
		t.Fatalf("error = %v, want setup and temp cleanup causes", runErr)
	}
	if !slices.Contains(events, "remove-temp") {
		t.Fatalf("cleanup events = %v, want temporary directory cleanup", events)
	}
}

func TestRunSigningEnvironmentRejectsMissingLockReleaseFunction(t *testing.T) {
	fixture := newSigningRunFixture(t, signingRunFixtureOptions{})
	inspection, err := inspectSigningRunInputs(fixture.identity, []byte(fixture.password), fixture.profile, fixture.roots, fixture.now)
	if err != nil {
		t.Fatalf("inspect fixture: %v", err)
	}
	events := []string{}
	deps := fakeSigningRunDeps(&events)
	deps.AcquireLock = func(context.Context) (func() error, error) { return nil, nil }

	_, err = runSigningEnvironment(context.Background(), deps, signingRunOptions{Child: []string{"tool"}}, fixture.profile, inspection, nil)
	if err == nil || !strings.Contains(err.Error(), "lock returned no release function") {
		t.Fatalf("missing lock release error = %v", err)
	}
	if slices.Contains(events, "recover") {
		t.Fatalf("recovery ran after invalid lock result: %v", events)
	}
}

func TestRunSigningEnvironmentChildAndCleanupFailureRendersCompanion(t *testing.T) {
	fixture := newSigningRunFixture(t, signingRunFixtureOptions{})
	inspection, err := inspectSigningRunInputs(fixture.identity, []byte(fixture.password), fixture.profile, fixture.roots, fixture.now)
	if err != nil {
		t.Fatalf("inspect fixture: %v", err)
	}
	cleanupErr := errors.New("cleanup exploded")
	var stderr bytes.Buffer
	events := []string{}
	deps := fakeSigningRunDeps(&events)
	deps.Stderr = &stderr
	deps.RunChild = func(context.Context, []string) error { return shared.NewProcessExitError(42) }
	removeCalls := 0
	deps.RemoveKeychainSearchEntry = func(context.Context, string) error {
		removeCalls++
		if removeCalls > 1 {
			return cleanupErr
		}
		return nil
	}
	_, runErr := runSigningEnvironment(context.Background(), deps, signingRunOptions{Child: []string{"tool"}}, fixture.profile, inspection, nil)
	if code, ok := shared.ProcessExitCode(runErr); !ok || code != 42 {
		t.Fatalf("process exit = %d, %t; want 42, true; error=%v", code, ok, runErr)
	}
	if !strings.Contains(stderr.String(), "cleanup exploded") {
		t.Fatalf("stderr = %q, want cleanup cause", stderr.String())
	}
}

func TestRunSigningEnvironmentChildAndUnlockFailureRendersCompanion(t *testing.T) {
	fixture := newSigningRunFixture(t, signingRunFixtureOptions{})
	inspection, err := inspectSigningRunInputs(fixture.identity, []byte(fixture.password), fixture.profile, fixture.roots, fixture.now)
	if err != nil {
		t.Fatalf("inspect fixture: %v", err)
	}
	unlockErr := errors.New("unlock exploded")
	var stderr bytes.Buffer
	events := []string{}
	deps := fakeSigningRunDeps(&events)
	deps.Stderr = &stderr
	deps.AcquireLock = func(context.Context) (func() error, error) { return func() error { return unlockErr }, nil }
	deps.RunChild = func(context.Context, []string) error { return shared.NewProcessExitError(42) }
	_, runErr := runSigningEnvironment(context.Background(), deps, signingRunOptions{Child: []string{"tool"}}, fixture.profile, inspection, nil)
	if code, ok := shared.ProcessExitCode(runErr); !ok || code != 42 {
		t.Fatalf("process exit = %d, %t; want 42, true; error=%v", code, ok, runErr)
	}
	if !strings.Contains(stderr.String(), "release signing environment lock: unlock exploded") {
		t.Fatalf("stderr = %q, want unlock cause", stderr.String())
	}
}

func TestFinishSigningRunReceiptReportsFailureAlongsideChildExit(t *testing.T) {
	receiptPath := filepath.Join(t.TempDir(), "receipt.json")
	if err := os.WriteFile(receiptPath, []byte("existing"), 0o600); err != nil {
		t.Fatalf("write existing receipt: %v", err)
	}
	var stderr bytes.Buffer
	err := finishSigningRunReceipt(&stderr, receiptPath, signingRunReceipt{SchemaVersion: 1}, shared.NewProcessExitError(42))
	if code, ok := shared.ProcessExitCode(err); !ok || code != 42 {
		t.Fatalf("process exit = %d, %t; want 42, true; error=%v", code, ok, err)
	}
	if !strings.Contains(stderr.String(), "Error: signing run: write receipt:") {
		t.Fatalf("stderr = %q, want separately rendered receipt failure", stderr.String())
	}
	var reported shared.ReportedError
	if !errors.As(err, &reported) {
		t.Fatalf("error = %v, want already-reported composite", err)
	}
}

func TestWithSigningRunReceiptRejectsExistingDestinationBeforeRun(t *testing.T) {
	receiptPath := filepath.Join(t.TempDir(), "receipt.json")
	if err := os.WriteFile(receiptPath, []byte("existing"), 0o600); err != nil {
		t.Fatalf("write existing receipt: %v", err)
	}
	runCalled := false

	err := withSigningRunReceipt(io.Discard, receiptPath, func() (signingRunReceipt, error) {
		runCalled = true
		return signingRunReceipt{SchemaVersion: 1}, nil
	})

	if !errors.Is(err, os.ErrExist) {
		t.Fatalf("error = %v, want os.ErrExist", err)
	}
	if runCalled {
		t.Fatal("run callback executed despite invalid receipt destination")
	}
}

func TestWithSigningRunReceiptPreflightsParentAndPublishesAfterRun(t *testing.T) {
	receiptPath := filepath.Join(t.TempDir(), "missing", "receipt.json")
	runCalled := false

	err := withSigningRunReceipt(io.Discard, receiptPath, func() (signingRunReceipt, error) {
		runCalled = true
		entries, readErr := os.ReadDir(filepath.Dir(receiptPath))
		if readErr != nil {
			return signingRunReceipt{}, readErr
		}
		if len(entries) != 0 {
			return signingRunReceipt{}, fmt.Errorf("receipt directory contains preflight residue: %v", entries)
		}
		return signingRunReceipt{SchemaVersion: 1, Outcome: "succeeded"}, nil
	})
	if err != nil {
		t.Fatalf("withSigningRunReceipt() error: %v", err)
	}
	if !runCalled {
		t.Fatal("run callback was not executed")
	}
	data, err := os.ReadFile(receiptPath)
	if err != nil {
		t.Fatalf("read receipt: %v", err)
	}
	if !bytes.Contains(data, []byte(`"outcome": "succeeded"`)) {
		t.Fatalf("receipt = %s, want successful outcome", data)
	}
}

func TestWithSigningRunReceiptDoesNotReplaceDestinationCreatedDuringRun(t *testing.T) {
	receiptPath := filepath.Join(t.TempDir(), "receipt.json")
	foreign := []byte("created during run")

	err := withSigningRunReceipt(io.Discard, receiptPath, func() (signingRunReceipt, error) {
		if writeErr := os.WriteFile(receiptPath, foreign, 0o600); writeErr != nil {
			return signingRunReceipt{}, writeErr
		}
		return signingRunReceipt{SchemaVersion: 1, Outcome: "succeeded"}, nil
	})

	if !errors.Is(err, os.ErrExist) {
		t.Fatalf("error = %v, want os.ErrExist", err)
	}
	data, readErr := os.ReadFile(receiptPath)
	if readErr != nil {
		t.Fatalf("read destination: %v", readErr)
	}
	if !bytes.Equal(data, foreign) {
		t.Fatalf("destination = %q, want preserved foreign content %q", data, foreign)
	}
}

func TestRunSigningEnvironmentRestoresStateInReverseOrder(t *testing.T) {
	fixture := newSigningRunFixture(t, signingRunFixtureOptions{})
	inspection, err := inspectSigningRunInputs(fixture.identity, []byte(fixture.password), fixture.profile, fixture.roots, fixture.now)
	if err != nil {
		t.Fatalf("inspect fixture: %v", err)
	}
	events := []string{}
	deps := signingRunDeps{
		GOOS: "darwin",
		RandomBytes: func(size int) ([]byte, error) {
			return []byte(strings.Repeat("a", size)), nil
		},
		TempDir:       func() (string, error) { events = append(events, "temp"); return "/tmp/asc-signing-run/test", nil },
		RemoveTempDir: func(path string) error { events = append(events, "remove-temp:"+path); return nil },
		AcquireLock: func(context.Context) (func() error, error) {
			events = append(events, "lock")
			return func() error { events = append(events, "unlock"); return nil }, nil
		},
		Recover: func(context.Context) error { events = append(events, "recover"); return nil },
		WriteJournal: func(_ signingRunJournal, overwrite bool) error {
			events = append(events, fmt.Sprintf("journal:%t", overwrite))
			return nil
		},
		RemoveJournal: func() error { events = append(events, "remove-journal"); return nil },
		KeychainSearchList: func(context.Context) ([]string, error) {
			events = append(events, "list")
			return []string{"/Users/me/login.keychain-db"}, nil
		},
		CreateKeychain: func(context.Context, string, []byte) error { events = append(events, "create-keychain"); return nil },
		ImportIdentity: func(context.Context, string, []byte, []byte, []byte, string) error {
			events = append(events, "import")
			return nil
		},
		SetKeychainSearchList: func(_ context.Context, paths []string) error {
			events = append(events, "set-list:"+strings.Join(paths, ","))
			return nil
		},
		RemoveKeychainSearchEntry: func(context.Context, string) error { events = append(events, "remove-search-entry"); return nil },
		DeleteKeychain:            func(context.Context, string) error { events = append(events, "delete-keychain"); return nil },
		InstallProfile: func(_ context.Context, _ string, _ []byte, _ string, beforeCreate func(signingRunProfileInstall) error) (signingRunProfileInstall, error) {
			events = append(events, "install-profile")
			planned := signingRunProfileInstall{Path: "/profiles/uuid.mobileprovision", Created: true, Digest: inspection.ProfileSHA256}
			if err := beforeCreate(planned); err != nil {
				return signingRunProfileInstall{}, err
			}
			return planned, nil
		},
		RemoveProfile: func(signingRunProfileInstall) error { events = append(events, "remove-profile"); return nil },
		RunChild:      func(context.Context, []string) error { events = append(events, "child"); return nil },
	}

	result, err := runSigningEnvironment(context.Background(), deps, signingRunOptions{Child: []string{"xcodebuild"}}, fixture.profile, inspection, nil)
	if err != nil {
		t.Fatalf("runSigningEnvironment() error: %v", err)
	}
	if result.Outcome != "succeeded" || result.ChildExitCode != 0 {
		t.Fatalf("unexpected result: %+v", result)
	}
	want := []string{
		"lock", "recover", "temp", "list", "journal:false", "create-keychain", "remove-search-entry", "import",
		"install-profile", "journal:true", "list",
		"set-list:/tmp/asc-signing-run/test/signing.keychain-db,/Users/me/login.keychain-db",
		"child", "remove-profile", "remove-search-entry", "delete-keychain",
		"remove-temp:/tmp/asc-signing-run/test", "remove-journal", "unlock",
	}
	if !reflect.DeepEqual(events, want) {
		t.Fatalf("events = %#v, want %#v", events, want)
	}
}

func TestRunSigningEnvironmentRestoresAfterEachSetupFailure(t *testing.T) {
	fixture := newSigningRunFixture(t, signingRunFixtureOptions{})
	inspection, err := inspectSigningRunInputs(fixture.identity, []byte(fixture.password), fixture.profile, fixture.roots, fixture.now)
	if err != nil {
		t.Fatalf("inspect fixture: %v", err)
	}
	for _, failStage := range []string{"create-keychain", "import", "set-list", "install-profile", "child"} {
		t.Run(failStage, func(t *testing.T) {
			events := []string{}
			fail := errors.New("boom at " + failStage)
			deps := fakeSigningRunDeps(&events)
			deps.CreateKeychain = func(context.Context, string, []byte) error {
				events = append(events, "create-keychain")
				if failStage == "create-keychain" {
					return fail
				}
				return nil
			}
			deps.ImportIdentity = func(context.Context, string, []byte, []byte, []byte, string) error {
				events = append(events, "import")
				if failStage == "import" {
					return fail
				}
				return nil
			}
			deps.SetKeychainSearchList = func(_ context.Context, paths []string) error {
				events = append(events, "set-list:"+strings.Join(paths, ","))
				if failStage == "set-list" && len(paths) > 1 {
					return fail
				}
				return nil
			}
			deps.InstallProfile = func(_ context.Context, _ string, _ []byte, _ string, beforeCreate func(signingRunProfileInstall) error) (signingRunProfileInstall, error) {
				events = append(events, "install-profile")
				if failStage == "install-profile" {
					return signingRunProfileInstall{}, fail
				}
				planned := signingRunProfileInstall{Path: "/profiles/uuid", Created: true, Digest: inspection.ProfileSHA256}
				if err := beforeCreate(planned); err != nil {
					return signingRunProfileInstall{}, err
				}
				return planned, nil
			}
			deps.RunChild = func(context.Context, []string) error {
				events = append(events, "child")
				if failStage == "child" {
					return fail
				}
				return nil
			}

			_, gotErr := runSigningEnvironment(context.Background(), deps, signingRunOptions{Child: []string{"tool"}}, fixture.profile, inspection, nil)
			if !errors.Is(gotErr, fail) {
				t.Fatalf("error = %v, want injected failure", gotErr)
			}
			if !slices.Contains(events, "unlock") || !slices.Contains(events, "remove-temp") {
				t.Fatalf("cleanup events missing: %v", events)
			}
			if failStage != "create-keychain" && !slices.Contains(events, "delete-keychain") {
				t.Fatalf("keychain cleanup missing: %v", events)
			}
		})
	}
}

func TestRunSigningEnvironmentCleansUpProfileWhenInstallerReturnsErrorAfterJournaling(t *testing.T) {
	fixture := newSigningRunFixture(t, signingRunFixtureOptions{})
	inspection, err := inspectSigningRunInputs(fixture.identity, []byte(fixture.password), fixture.profile, fixture.roots, fixture.now)
	if err != nil {
		t.Fatalf("inspect fixture: %v", err)
	}
	installErr := errors.New("profile installation failed after staging")
	profileCleanupErr := errors.New("staged profile cleanup failed")
	planned := signingRunProfileInstall{
		Path: "/profiles/uuid.mobileprovision", StagedPath: "/profiles/.staged",
		Created: true, Digest: inspection.ProfileSHA256, Device: 1, Inode: 2,
	}
	events := []string{}
	deps := fakeSigningRunDeps(&events)
	deps.InstallProfile = func(_ context.Context, _ string, _ []byte, _ string, beforeCreate func(signingRunProfileInstall) error) (signingRunProfileInstall, error) {
		events = append(events, "install-profile")
		if err := beforeCreate(planned); err != nil {
			return signingRunProfileInstall{}, err
		}
		return signingRunProfileInstall{}, installErr
	}
	var removed signingRunProfileInstall
	deps.RemoveProfile = func(install signingRunProfileInstall) error {
		events = append(events, "remove-profile")
		removed = install
		return profileCleanupErr
	}
	deps.RemoveTempDir = func(string) error { events = append(events, "remove-temp"); return nil }
	deps.RemoveJournal = func() error { events = append(events, "remove-journal"); return nil }

	_, runErr := runSigningEnvironment(context.Background(), deps, signingRunOptions{Child: []string{"tool"}}, fixture.profile, inspection, nil)
	if !errors.Is(runErr, installErr) || !errors.Is(runErr, profileCleanupErr) {
		t.Fatalf("error = %v, want install and profile cleanup causes", runErr)
	}
	if !reflect.DeepEqual(removed, planned) {
		t.Fatalf("removed profile = %+v, want journaled staged ownership %+v", removed, planned)
	}
	if slices.Contains(events, "remove-temp") || slices.Contains(events, "remove-journal") {
		t.Fatalf("cleanup removed recovery state after profile cleanup failure: %v", events)
	}
}

func fakeSigningRunDeps(events *[]string) signingRunDeps {
	return signingRunDeps{
		GOOS:          "darwin",
		Stderr:        io.Discard,
		RandomBytes:   func(size int) ([]byte, error) { return []byte(strings.Repeat("a", size)), nil },
		TempDir:       func() (string, error) { *events = append(*events, "temp"); return "/tmp/signing-run", nil },
		RemoveTempDir: func(string) error { *events = append(*events, "remove-temp"); return nil },
		AcquireLock: func(context.Context) (func() error, error) {
			*events = append(*events, "lock")
			return func() error { *events = append(*events, "unlock"); return nil }, nil
		},
		Recover:       func(context.Context) error { *events = append(*events, "recover"); return nil },
		WriteJournal:  func(signingRunJournal, bool) error { *events = append(*events, "journal"); return nil },
		RemoveJournal: func() error { *events = append(*events, "remove-journal"); return nil },
		KeychainSearchList: func(context.Context) ([]string, error) {
			*events = append(*events, "list")
			return []string{"login"}, nil
		},
		CreateKeychain: func(context.Context, string, []byte) error { *events = append(*events, "create-keychain"); return nil },
		ImportIdentity: func(context.Context, string, []byte, []byte, []byte, string) error {
			*events = append(*events, "import")
			return nil
		},
		SetKeychainSearchList:     func(context.Context, []string) error { *events = append(*events, "set-list"); return nil },
		RemoveKeychainSearchEntry: func(context.Context, string) error { *events = append(*events, "remove-search-entry"); return nil },
		DeleteKeychain:            func(context.Context, string) error { *events = append(*events, "delete-keychain"); return nil },
		InstallProfile: func(context.Context, string, []byte, string, func(signingRunProfileInstall) error) (signingRunProfileInstall, error) {
			*events = append(*events, "install-profile")
			return signingRunProfileInstall{}, nil
		},
		RemoveProfile: func(signingRunProfileInstall) error { *events = append(*events, "remove-profile"); return nil },
		RunChild:      func(context.Context, []string) error { *events = append(*events, "child"); return nil },
	}
}

func TestSigningRunCommandThreadsFlagsAndChildArgv(t *testing.T) {
	previous := executeSigningRunFn
	t.Cleanup(func() { executeSigningRunFn = previous })
	var got signingRunOptions
	executeSigningRunFn = func(_ context.Context, options signingRunOptions) error {
		got = options
		return nil
	}
	command := SigningRunCommand()
	command.FlagSet.SetOutput(&strings.Builder{})
	args := []string{
		"--identity", "App.p12",
		"--identity-password-file", "p12-password",
		"--profile", "App.mobileprovision",
		"--purpose", "release-testing",
		"--receipt", "run.json",
		"--", "xcodebuild", "-exportArchive", "--looks-like-a-flag",
	}
	if err := command.Parse(args); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if err := command.Run(context.Background()); err != nil {
		t.Fatalf("run: %v", err)
	}
	want := signingRunOptions{
		IdentityPath: "App.p12", IdentityPasswordPath: "p12-password",
		ProfilePath: "App.mobileprovision", Purpose: "release-testing",
		ReceiptPath: "run.json", Child: []string{"xcodebuild", "-exportArchive", "--looks-like-a-flag"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("options = %+v, want %+v", got, want)
	}
}

func TestSigningRunCommandPreservesChildExitAndReceipt(t *testing.T) {
	previous := executeSigningRunFn
	var got signingRunOptions
	executeSigningRunFn = func(_ context.Context, options signingRunOptions) error {
		got = options
		return withSigningRunReceipt(io.Discard, options.ReceiptPath, func() (signingRunReceipt, error) {
			return signingRunReceipt{
				SchemaVersion: 1,
				Purpose:       signingRunPurposeReleaseTesting,
				Outcome:       "failed",
				ChildExitCode: 42,
			}, shared.NewProcessExitError(42)
		})
	}
	t.Cleanup(func() { executeSigningRunFn = previous })

	receiptPath := filepath.Join(t.TempDir(), "receipt.json")
	command := SigningRunCommand()
	command.FlagSet.SetOutput(io.Discard)
	if err := command.Parse([]string{
		"--identity", "App.p12",
		"--profile", "App.mobileprovision",
		"--receipt", receiptPath,
		"--", "child-tool", "--child-flag",
	}); err != nil {
		t.Fatalf("parse: %v", err)
	}
	err := command.Run(context.Background())
	if code, ok := shared.ProcessExitCode(err); !ok || code != 42 {
		t.Fatalf("process exit = %d, %t; want 42, true; error = %v", code, ok, err)
	}
	if !reflect.DeepEqual(got.Child, []string{"child-tool", "--child-flag"}) {
		t.Fatalf("child argv = %#v, want child command and arguments", got.Child)
	}
	receiptData, readErr := os.ReadFile(receiptPath)
	if readErr != nil {
		t.Fatalf("read receipt: %v", readErr)
	}
	var receipt signingRunReceipt
	if err := json.Unmarshal(receiptData, &receipt); err != nil {
		t.Fatalf("decode receipt: %v", err)
	}
	if receipt.ChildExitCode != 42 || receipt.Outcome != "failed" {
		t.Fatalf("receipt = %+v, want failed child exit 42", receipt)
	}
}

func TestSigningRunCommandValidation(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{name: "identity required", args: []string{"--profile", "App.mobileprovision", "--", "true"}, wantErr: "--identity is required"},
		{name: "profile required", args: []string{"--identity", "App.p12", "--", "true"}, wantErr: "--profile is required"},
		{name: "child required", args: []string{"--identity", "App.p12", "--profile", "App.mobileprovision"}, wantErr: "a child command is required"},
		{name: "invalid purpose", args: []string{"--identity", "App.p12", "--profile", "App.mobileprovision", "--purpose", "development", "--", "true"}, wantErr: `--purpose must be "release-testing"`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			command := SigningRunCommand()
			command.FlagSet.SetOutput(&strings.Builder{})
			if err := command.Parse(test.args); err != nil {
				t.Fatalf("parse: %v", err)
			}
			err := command.Run(context.Background())
			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("error = %v, want %q", err, test.wantErr)
			}
			if !errors.Is(err, flag.ErrHelp) {
				t.Fatalf("error = %v, want usage error", err)
			}
			if shared.ClassifyUsageError(err) == "" {
				t.Fatalf("error = %v, want classified usage error", err)
			}
		})
	}
}
