package signing

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"io"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go.mozilla.org/pkcs7"
	"howett.net/plist"
)

func newIOSPackageFixture(t *testing.T) (string, string) {
	t.Helper()
	directory := t.TempDir()
	app := filepath.Join(directory, "App.app")
	if err := os.Mkdir(app, 0o755); err != nil {
		t.Fatal(err)
	}
	data, err := plist.Marshal(map[string]any{"CFBundleIdentifier": "com.example.app", "CFBundleExecutable": "App", "CFBundleShortVersionString": "1.0", "CFBundleVersion": "1", "CFBundleSupportedPlatforms": []string{"iPhoneOS"}}, plist.XMLFormat)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(app, "Info.plist"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	var executable bytes.Buffer
	for _, v := range []uint32{0xfeedfacf, 0x100000c, 0, 2, 1, 24, 0, 0, 0x32, 24, 2, 0x120000, 0x1b0000, 0} {
		if err := binary.Write(&executable, binary.LittleEndian, v); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(app, "App"), executable.Bytes(), 0o755); err != nil {
		t.Fatal(err)
	}
	return directory, app
}

func TestIOSPackageAdHocStagesSourceAndPublishesCreateOnly(t *testing.T) {
	if os.PathSeparator == '\\' {
		t.Skip("POSIX signer fixture")
	}
	directory, app := newIOSPackageFixture(t)
	tools := filepath.Join(directory, "bin")
	if err := os.Mkdir(tools, 0o755); err != nil {
		t.Fatal(err)
	}
	// Fake rcodesign: the real signer is not installed in unit tests.
	script := `#!/bin/sh
test "$1" = --config-file && test "$2" = /dev/null || exit 12
test "$3" = sign && test "$4" = --timestamp-url && test "$5" = none || exit 13
test -z "$ASC_KEY_ID" && test -z "$RCODESIGN_SIGN_PEM_FILE" && test -z "$AWS_SECRET_ACCESS_KEY" && test -z "$LD_PRELOAD" || exit 14
for app; do :; done
printf 'signer ran in private stage' > "$app/signer-proof"
`
	if err := os.WriteFile(filepath.Join(tools, "rcodesign"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", tools)
	t.Setenv("ASC_KEY_ID", "not-a-real-key")
	t.Setenv("RCODESIGN_SIGN_PEM_FILE", "must-be-ignored")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "must-be-ignored")
	t.Setenv("LD_PRELOAD", "must-be-ignored")
	destination := filepath.Join(directory, "App.ipa")
	opts := IOSPackageOptions{AppPath: app, IPAPath: destination, AdHoc: true, LogWriter: io.Discard}
	result, err := PackageIOSApp(context.Background(), testSigningDeps(), opts)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Success || result.SigningType != "adHoc" || result.ProfileValidated || result.SignatureVerified || result.AppleAcceptance != "notVerified" {
		t.Fatalf("misleading receipt: %+v", result)
	}
	data, err := os.ReadFile(destination)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(data)
	if result.SHA256 != hex.EncodeToString(digest[:]) {
		t.Fatal("wrong artifact hash")
	}
	archive, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	marker := false
	for _, entry := range archive.File {
		if !strings.HasPrefix(entry.Name, "Payload/App.app/") {
			t.Fatalf("unexpected entry: %s", entry.Name)
		}
		if entry.Name == "Payload/App.app/signer-proof" {
			marker = true
		}
	}
	if !marker {
		t.Fatal("signed stage was not packaged")
	}
	if _, err := os.Stat(filepath.Join(app, "signer-proof")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("source app was modified")
	}
	if _, err := PackageIOSApp(context.Background(), testSigningDeps(), opts); err == nil {
		t.Fatal("existing output replaced")
	}
	after, err := os.ReadFile(destination)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(data, after) {
		t.Fatal("existing output changed")
	}
}

func TestIOSPackageRejectsUntrustedProfileBeforeSigning(t *testing.T) {
	directory, app := newIOSPackageFixture(t)
	fixture := newSigningRunFixture(t, signingRunFixtureOptions{})
	d := testSigningDeps()
	d.Now = func() time.Time { return fixture.now }
	profile := filepath.Join(directory, "untrusted.mobileprovision")
	if err := os.WriteFile(profile, fixture.profile, 0o600); err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(directory, "App.ipa")
	_, err := PackageIOSApp(context.Background(), d, IOSPackageOptions{AppPath: app, IPAPath: destination, IdentityPath: "not-read-before-trust", ProfilePath: profile, LogWriter: io.Discard})
	if err == nil {
		t.Fatal("non-Apple profile accepted")
	}
	if _, err := os.Stat(destination); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("untrusted profile produced an artifact")
	}
}

func TestIOSPackageAdHocRejectsExistingProfile(t *testing.T) {
	directory, app := newIOSPackageFixture(t)
	if err := os.WriteFile(filepath.Join(app, "embedded.mobileprovision"), []byte("old profile"), 0o644); err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(directory, "App.ipa")
	_, err := PackageIOSApp(context.Background(), testSigningDeps(), IOSPackageOptions{AppPath: app, IPAPath: destination, AdHoc: true, LogWriter: io.Discard})
	if err == nil || !strings.Contains(err.Error(), "must not contain a provisioning profile") {
		t.Fatalf("existing profile was not rejected: %v", err)
	}
	if _, err := os.Stat(destination); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("rejected app published an IPA: %v", err)
	}
}

func TestIOSPackageEmptyEntitlementsAreExplicit(t *testing.T) {
	if os.PathSeparator == '\\' {
		t.Skip("POSIX signer fixture")
	}
	for _, explicit := range []bool{false, true} {
		t.Run(map[bool]string{false: "omitted", true: "empty plist"}[explicit], func(t *testing.T) {
			directory, app := newIOSPackageFixture(t)
			tools := filepath.Join(directory, "bin")
			if err := os.Mkdir(tools, 0o755); err != nil {
				t.Fatal(err)
			}
			// rcodesign inherits old claims unless an explicit plist overrides them.
			script := `#!/bin/sh
previous=
entitlements=
for app; do
  if test "$previous" = --entitlements-xml-file; then entitlements="$app"; fi
  previous="$app"
done
test -n "$entitlements" && test -s "$entitlements" || exit 15
while IFS= read -r line; do printf '%s\n' "$line"; done < "$entitlements" > "$app/entitlements-proof.plist"
`
			if err := os.WriteFile(filepath.Join(tools, "rcodesign"), []byte(script), 0o755); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", tools)
			opts := IOSPackageOptions{AppPath: app, IPAPath: filepath.Join(directory, "App.ipa"), AdHoc: true, LogWriter: io.Discard}
			if explicit {
				opts.EntitlementsPath = filepath.Join(directory, "empty.plist")
				data, err := plist.Marshal(map[string]any{}, plist.XMLFormat)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(opts.EntitlementsPath, data, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := PackageIOSApp(context.Background(), testSigningDeps(), opts); err != nil {
				t.Fatal(err)
			}
			archive, err := zip.OpenReader(opts.IPAPath)
			if err != nil {
				t.Fatal(err)
			}
			defer archive.Close()
			for _, entry := range archive.File {
				if entry.Name != "Payload/App.app/entitlements-proof.plist" {
					continue
				}
				reader, err := entry.Open()
				if err != nil {
					t.Fatal(err)
				}
				data, err := io.ReadAll(reader)
				reader.Close()
				if err != nil {
					t.Fatal(err)
				}
				var claims map[string]any
				if _, err := plist.Unmarshal(data, &claims); err != nil || claims == nil || len(claims) != 0 {
					t.Fatalf("signer did not receive a valid empty dictionary: %v %v", claims, err)
				}
				return
			}
			t.Fatal("signer entitlement proof missing from IPA")
		})
	}
}

func TestIOSPackageReportsPrivateStageCleanupFailure(t *testing.T) {
	directory, app := newIOSPackageFixture(t)
	original := removeIOSPackageStage
	var retained string
	removeIOSPackageStage = func(path string) error { retained = path; return errors.New("injected cleanup failure") }
	t.Cleanup(func() {
		removeIOSPackageStage = original
		if retained != "" {
			_ = os.RemoveAll(retained)
		}
	})
	t.Setenv("PATH", t.TempDir())
	result, err := PackageIOSApp(context.Background(), testSigningDeps(), IOSPackageOptions{AppPath: app, IPAPath: filepath.Join(directory, "App.ipa"), AdHoc: true, LogWriter: io.Discard})
	if err == nil || !strings.Contains(err.Error(), "private signing stage cleanup failed") || result.Success {
		t.Fatalf("cleanup failure hidden: %+v %v", result, err)
	}
}

func TestIOSPackagePipesIdentityToSignerWithoutStagingKey(t *testing.T) {
	if os.PathSeparator == '\\' {
		t.Skip("POSIX signer fixture")
	}
	directory, app := newIOSPackageFixture(t)
	fixture := newSigningRunFixture(t, signingRunFixtureOptions{})
	d := testSigningDeps()
	d.Now = func() time.Time { return fixture.now }
	inputs := map[string][]byte{
		"App.p12":             fixture.identity,
		"password":            []byte(fixture.password + "\n"),
		"App.mobileprovision": appleShapedIOSPackageProfile(t, fixture),
	}
	for name, data := range inputs {
		if err := os.WriteFile(filepath.Join(directory, name), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	tools := filepath.Join(directory, "bin")
	if err := os.Mkdir(tools, 0o755); err != nil {
		t.Fatal(err)
	}
	proof := filepath.Join(directory, "proof")
	if err := os.Mkdir(proof, 0o700); err != nil {
		t.Fatal(err)
	}
	script := `#!/bin/sh
for arg; do printf '%s\n' "$arg"; done > ` + proof + `/args
for entry in * .*; do printf '%s\n' "$entry"; done > ` + proof + `/stage
/bin/cat /dev/fd/3 > ` + proof + `/identity
/bin/cat /dev/fd/4 > ` + proof + `/password
`
	if err := os.WriteFile(filepath.Join(tools, "rcodesign"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", tools)
	opts := IOSPackageOptions{
		AppPath:      app,
		IPAPath:      filepath.Join(directory, "App.ipa"),
		IdentityPath: filepath.Join(directory, "App.p12"),
		PasswordPath: filepath.Join(directory, "password"),
		ProfilePath:  filepath.Join(directory, "App.mobileprovision"),
		LogWriter:    io.Discard,
	}
	result, err := PackageIOSApp(context.Background(), d, opts)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Success || !result.ProfileValidated {
		t.Fatalf("unexpected receipt: %+v", result)
	}
	args, err := os.ReadFile(filepath.Join(proof, "args"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(args), "--p12-file\n/dev/fd/3\n--p12-password-file\n/dev/fd/4\n") || strings.Contains(string(args), "--pem-file") {
		t.Fatalf("signer did not receive piped identity inputs: %s", args)
	}
	for name, want := range map[string][]byte{"identity": fixture.identity, "password": []byte(fixture.password + "\n")} {
		got, err := os.ReadFile(filepath.Join(proof, name))
		if err != nil || !bytes.Equal(got, want) {
			t.Fatalf("signer %s input mismatch: %v", name, err)
		}
	}
	stage, err := os.ReadFile(filepath.Join(proof, "stage"))
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range strings.Fields(string(stage)) {
		if name != "." && name != ".." && name != "App.app" && name != "entitlements.plist" {
			t.Fatalf("signing stage contains %q", name)
		}
	}
}

func appleShapedIOSPackageProfile(t *testing.T, fixture *signingRunFixture) []byte {
	t.Helper()
	original, err := pkcs7.Parse(fixture.profile)
	if err != nil {
		t.Fatal(err)
	}
	issue := func(subject pkix.Name, isCA bool, parent *x509.Certificate, parentKey *rsa.PrivateKey) (*x509.Certificate, *rsa.PrivateKey) {
		key, err := rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			t.Fatal(err)
		}
		template := &x509.Certificate{
			SerialNumber:          big.NewInt(time.Now().UnixNano()),
			Subject:               subject,
			NotBefore:             fixture.now.Add(-24 * time.Hour),
			NotAfter:              fixture.now.Add(365 * 24 * time.Hour),
			KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
			BasicConstraintsValid: true,
			IsCA:                  isCA,
		}
		if parent == nil {
			parent, parentKey = template, key
		}
		der, err := x509.CreateCertificate(rand.Reader, template, parent, &key.PublicKey, parentKey)
		if err != nil {
			t.Fatal(err)
		}
		certificate, err := x509.ParseCertificate(der)
		if err != nil {
			t.Fatal(err)
		}
		return certificate, key
	}
	root, rootKey := issue(pkix.Name{CommonName: "Apple iPhone Certification Authority"}, true, nil, nil)
	signer, signerKey := issue(pkix.Name{CommonName: "Apple iPhone OS Provisioning Profile Signing", Organization: []string{"Apple Inc."}}, false, root, rootKey)
	digest := sha256.Sum256(root.Raw)
	fingerprint := hex.EncodeToString(digest[:])
	signingResignAppleProfileRootFingerprints[fingerprint] = struct{}{}
	t.Cleanup(func() { delete(signingResignAppleProfileRootFingerprints, fingerprint) })
	signed, err := pkcs7.NewSignedData(original.Content)
	if err != nil {
		t.Fatal(err)
	}
	if err := signed.AddSignerChain(signer, signerKey, []*x509.Certificate{root}, pkcs7.SignerInfoConfig{}); err != nil {
		t.Fatal(err)
	}
	profile, err := signed.Finish()
	if err != nil {
		t.Fatal(err)
	}
	return profile
}
