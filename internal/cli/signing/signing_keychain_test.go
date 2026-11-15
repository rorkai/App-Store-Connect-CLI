package signing

import (
	"context"
	"errors"
	"flag"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/artifacts"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func TestSigningKeychainInstallCommandThreadsOptions(t *testing.T) {
	previous := installSigningKeychainFn
	previousContext := signingKeychainInstallContext
	t.Cleanup(func() {
		installSigningKeychainFn = previous
		signingKeychainInstallContext = previousContext
	})

	var got signingKeychainInstallOptions
	type installContextKey struct{}
	contextStopped := false
	signingKeychainInstallContext = func(ctx context.Context) (context.Context, func()) {
		return context.WithValue(ctx, installContextKey{}, true), func() { contextStopped = true }
	}
	installSigningKeychainFn = func(ctx context.Context, _ *shared.Deps, options signingKeychainInstallOptions) (*asc.SigningKeychainInstallResult, error) {
		if wrapped, _ := ctx.Value(installContextKey{}).(bool); !wrapped {
			t.Fatal("install context was not wrapped for termination signals")
		}
		got = options
		return &asc.SigningKeychainInstallResult{Action: "installed"}, nil
	}
	d, depsStdout, _ := newSigningTestDeps()
	command := newSigningKeychainInstallCommand(d)
	command.FlagSet.SetOutput(&strings.Builder{})
	if err := command.Parse([]string{
		"--identity", "App.p12",
		"--identity-password-file", "p12-password",
		"--keychain", "release.keychain-db",
		"--keychain-password-file", "keychain-password",
		"--expected-certificate-sha256", strings.Repeat("a", 64),
		"--add-to-search-list",
		"--add-apple-intermediates",
		"--confirm",
		"--output", "json",
	}); err != nil {
		t.Fatal(err)
	}
	runErr := command.Run(context.Background())
	stdout := depsStdout.String()
	if runErr != nil {
		t.Fatal(runErr)
	}
	want := signingKeychainInstallOptions{
		IdentityPath:              "App.p12",
		IdentityPasswordPath:      "p12-password",
		KeychainPath:              "release.keychain-db",
		KeychainPasswordPath:      "keychain-password",
		ExpectedCertificateSHA256: strings.Repeat("A", 64),
		AddToSearchList:           true,
		AddAppleIntermediates:     true,
		LockOnSleep:               true,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("options = %+v, want %+v", got, want)
	}
	if !strings.Contains(stdout, `"action":"installed"`) {
		t.Fatalf("stdout = %q, want JSON result", stdout)
	}
	if !contextStopped {
		t.Fatal("install signal context was not stopped")
	}
}

func TestSigningKeychainInstallCommandValidatesBeforeExecution(t *testing.T) {
	previous := installSigningKeychainFn
	t.Cleanup(func() { installSigningKeychainFn = previous })
	called := false
	installSigningKeychainFn = func(context.Context, *shared.Deps, signingKeychainInstallOptions) (*asc.SigningKeychainInstallResult, error) {
		called = true
		return nil, nil
	}

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{name: "identity required", args: []string{"--identity-password-file", "a", "--keychain", "b", "--keychain-password-file", "c", "--confirm"}, wantErr: "--identity is required"},
		{name: "identity password required", args: []string{"--identity", "a", "--keychain", "b", "--keychain-password-file", "c", "--confirm"}, wantErr: "--identity-password-file is required"},
		{name: "keychain required", args: []string{"--identity", "a", "--identity-password-file", "b", "--keychain-password-file", "c", "--confirm"}, wantErr: "--keychain is required"},
		{name: "keychain password required", args: []string{"--identity", "a", "--identity-password-file", "b", "--keychain", "c", "--confirm"}, wantErr: "--keychain-password-file is required"},
		{name: "confirm required", args: []string{"--identity", "a", "--identity-password-file", "b", "--keychain", "c", "--keychain-password-file", "d"}, wantErr: "--confirm is required"},
		{name: "invalid digest", args: []string{"--identity", "a", "--identity-password-file", "b", "--keychain", "c", "--keychain-password-file", "d", "--expected-certificate-sha256", "short", "--confirm"}, wantErr: "64-character hexadecimal"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			called = false
			d, _, stderr := newSigningTestDeps()
			command := newSigningKeychainInstallCommand(d)
			command.FlagSet.SetOutput(&strings.Builder{})
			if err := command.Parse(test.args); err != nil {
				t.Fatal(err)
			}
			err := command.Run(context.Background())
			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("error = %v, want %q", err, test.wantErr)
			}
			if line := stderr.String(); !strings.HasPrefix(line, "Error: ") || !strings.Contains(line, test.wantErr) {
				t.Fatalf("deps stderr = %q, want the %q usage error", line, test.wantErr)
			}
			if !errors.Is(err, flag.ErrHelp) || shared.ClassifyUsageError(err) == "" {
				t.Fatalf("error = %v, want classified usage error", err)
			}
			if called {
				t.Fatal("executor called after command validation failure")
			}
		})
	}
}

func TestExecuteSigningKeychainInstallCreatesDedicatedKeychain(t *testing.T) {
	fixture := newSigningRunFixture(t, signingRunFixtureOptions{})
	identityPath := filepath.Join(t.TempDir(), "App.p12")
	identityPasswordPath := filepath.Join(t.TempDir(), "identity-password")
	keychainPasswordPath := filepath.Join(t.TempDir(), "keychain-password")
	writePrivateTestFile(t, identityPath, fixture.identity)
	writePrivateTestFile(t, identityPasswordPath, []byte(fixture.password+"\n"))
	writePrivateTestFile(t, keychainPasswordPath, []byte("keychain-secret\r\n"))
	keychainPath := filepath.Join(t.TempDir(), "release.keychain-db")
	resolvedKeychainPath := canonicalSigningKeychainTestPath(t, keychainPath)

	inspection, err := inspectSigningRunIdentity(fixture.identity, []byte(fixture.password), fixture.now)
	if err != nil {
		t.Fatal(err)
	}
	var events []string
	deps := signingKeychainInstallDeps{
		GOOS:              "darwin",
		SecurityAvailable: true,
		Now:               func() time.Time { return fixture.now },
		AcquireLock: func(context.Context) (func() error, error) {
			events = append(events, "lock")
			return func() error { events = append(events, "unlock"); return nil }, nil
		},
		CreateKeychain: func(_ context.Context, path string, password []byte) (bool, error) {
			events = append(events, "create")
			if path != resolvedKeychainPath || string(password) != "keychain-secret" {
				t.Fatalf("create path=%q password=%q", path, password)
			}
			return true, nil
		},
		ImportIdentity: func(_ context.Context, path string, keychainPassword, identityData, identityPassword []byte, expectedSHA1 string) error {
			events = append(events, "import")
			if path != resolvedKeychainPath || string(keychainPassword) != "keychain-secret" || string(identityPassword) != fixture.password {
				t.Fatalf("unexpected import inputs")
			}
			if !reflect.DeepEqual(identityData, fixture.identity) || expectedSHA1 != inspection.CertificateSHA1 {
				t.Fatalf("identity import mismatch")
			}
			return nil
		},
		KeychainSearchList: func(context.Context) ([]string, error) {
			events = append(events, "list")
			return []string{"/Users/me/Library/Keychains/login.keychain-db"}, nil
		},
		SetKeychainSearchList: func(_ context.Context, paths []string) error {
			events = append(events, "set")
			want := []string{"/Users/me/Library/Keychains/login.keychain-db", resolvedKeychainPath}
			if !reflect.DeepEqual(paths, want) {
				t.Fatalf("search list = %v, want %v", paths, want)
			}
			return nil
		},
		DeleteKeychain: func(context.Context, string) error {
			events = append(events, "delete")
			return nil
		},
	}

	result, err := executeSigningKeychainInstallWith(context.Background(), signingKeychainInstallOptions{
		IdentityPath:              identityPath,
		IdentityPasswordPath:      identityPasswordPath,
		KeychainPath:              keychainPath,
		KeychainPasswordPath:      keychainPasswordPath,
		ExpectedCertificateSHA256: inspection.CertificateSHA256,
		AddToSearchList:           true,
	}, deps)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(events, []string{"lock", "list", "create", "set", "import", "unlock"}) {
		t.Fatalf("events = %v", events)
	}
	if result.Action != "installed" || result.KeychainPath != resolvedKeychainPath || !result.SearchListUpdated {
		t.Fatalf("result = %+v", result)
	}
	if result.CertificateSHA256 != inspection.CertificateSHA256 || result.CertificateSHA1 != inspection.CertificateSHA1 || result.TeamID != fixture.teamID {
		t.Fatalf("certificate result = %+v", result)
	}
}

func TestExecuteSigningKeychainInstallRollsBackWhenCreateReportsPostCreateFailure(t *testing.T) {
	fixture := newSigningRunFixture(t, signingRunFixtureOptions{})
	identityPath := filepath.Join(t.TempDir(), "App.p12")
	identityPasswordPath := filepath.Join(t.TempDir(), "identity-password")
	keychainPasswordPath := filepath.Join(t.TempDir(), "keychain-password")
	writePrivateTestFile(t, identityPath, fixture.identity)
	writePrivateTestFile(t, identityPasswordPath, []byte(fixture.password))
	writePrivateTestFile(t, keychainPasswordPath, []byte("keychain-secret"))
	keychainPath := filepath.Join(t.TempDir(), "release.keychain-db")
	resolvedKeychainPath := canonicalSigningKeychainTestPath(t, keychainPath)
	originalSearchList := []string{"login.keychain-db", "system.keychain"}
	createErr := errors.New("keychain unlock failed after creation")
	deleted := false
	var restored []string
	imported := false

	_, err := executeSigningKeychainInstallWith(context.Background(), signingKeychainInstallOptions{
		IdentityPath: identityPath, IdentityPasswordPath: identityPasswordPath,
		KeychainPath: keychainPath, KeychainPasswordPath: keychainPasswordPath,
		AddToSearchList: true,
	}, signingKeychainInstallDeps{
		GOOS:              "darwin",
		SecurityAvailable: true,
		Now:               func() time.Time { return fixture.now },
		AcquireLock:       acquireSigningKeychainTestLock,
		CreateKeychain: func(context.Context, string, []byte) (bool, error) {
			return true, createErr
		},
		ImportIdentity: func(context.Context, string, []byte, []byte, []byte, string) error {
			imported = true
			return nil
		},
		KeychainSearchList: func(context.Context) ([]string, error) {
			return append([]string(nil), originalSearchList...), nil
		},
		SetKeychainSearchList: func(_ context.Context, paths []string) error {
			restored = append([]string(nil), paths...)
			return nil
		},
		RemoveKeychainSearchEntry: func(context.Context, string) error { return nil },
		DeleteKeychain: func(_ context.Context, path string) error {
			if path != resolvedKeychainPath {
				t.Fatalf("rollback path = %q", path)
			}
			deleted = true
			return nil
		},
	})
	if !errors.Is(err, createErr) {
		t.Fatalf("error = %v, want create failure", err)
	}
	if !deleted || !reflect.DeepEqual(restored, originalSearchList) {
		t.Fatalf("rollback deleted=%t restored=%v, want deleted and %v", deleted, restored, originalSearchList)
	}
	if imported {
		t.Fatal("identity imported after keychain creation failure")
	}
}

func TestExecuteSigningKeychainInstallRejectsExistingDestinationBeforeSecrets(t *testing.T) {
	keychainPath := filepath.Join(t.TempDir(), "existing.keychain-db")
	if err := os.WriteFile(keychainPath, []byte("existing"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := executeSigningKeychainInstallWith(context.Background(), signingKeychainInstallOptions{
		IdentityPath:         filepath.Join(t.TempDir(), "missing-identity"),
		IdentityPasswordPath: filepath.Join(t.TempDir(), "missing-identity-password"),
		KeychainPath:         keychainPath,
		KeychainPasswordPath: filepath.Join(t.TempDir(), "missing-keychain-password"),
	}, signingKeychainInstallDeps{GOOS: "darwin", SecurityAvailable: true})
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("error = %v, want existing destination error", err)
	}
}

func TestExecuteSigningKeychainInstallRejectsUnsupportedPlatformBeforeSecrets(t *testing.T) {
	_, err := executeSigningKeychainInstallWith(context.Background(), signingKeychainInstallOptions{
		IdentityPath:         "missing-identity",
		IdentityPasswordPath: "missing-identity-password",
		KeychainPath:         "missing-keychain",
		KeychainPasswordPath: "missing-keychain-password",
	}, signingKeychainInstallDeps{GOOS: "linux"})
	if err == nil || !strings.Contains(err.Error(), "supported only on macOS") {
		t.Fatalf("error = %v, want platform error", err)
	}
}

func TestExecuteSigningKeychainInstallRejectsDigestMismatchBeforeCreation(t *testing.T) {
	fixture := newSigningRunFixture(t, signingRunFixtureOptions{})
	identityPath := filepath.Join(t.TempDir(), "App.p12")
	identityPasswordPath := filepath.Join(t.TempDir(), "identity-password")
	keychainPasswordPath := filepath.Join(t.TempDir(), "keychain-password")
	writePrivateTestFile(t, identityPath, fixture.identity)
	writePrivateTestFile(t, identityPasswordPath, []byte(fixture.password))
	writePrivateTestFile(t, keychainPasswordPath, []byte("keychain-secret"))

	created := false
	_, err := executeSigningKeychainInstallWith(context.Background(), signingKeychainInstallOptions{
		IdentityPath: identityPath, IdentityPasswordPath: identityPasswordPath,
		KeychainPath: filepath.Join(t.TempDir(), "release.keychain-db"), KeychainPasswordPath: keychainPasswordPath,
		ExpectedCertificateSHA256: strings.Repeat("A", 64),
	}, signingKeychainInstallDeps{
		GOOS:                      "darwin",
		SecurityAvailable:         true,
		Now:                       func() time.Time { return fixture.now },
		AcquireLock:               acquireSigningKeychainTestLock,
		CreateKeychain:            func(context.Context, string, []byte) (bool, error) { created = true; return true, nil },
		ImportIdentity:            func(context.Context, string, []byte, []byte, []byte, string) error { return nil },
		KeychainSearchList:        func(context.Context) ([]string, error) { return nil, nil },
		SetKeychainSearchList:     func(context.Context, []string) error { return nil },
		RemoveKeychainSearchEntry: func(context.Context, string) error { return nil },
		DeleteKeychain:            func(context.Context, string) error { return nil },
	})
	if err == nil || !strings.Contains(err.Error(), "does not match --expected-certificate-sha256") {
		t.Fatalf("error = %v, want digest mismatch", err)
	}
	if created {
		t.Fatal("keychain created after digest mismatch")
	}
}

func TestExecuteSigningKeychainInstallKeepsSearchListUnchangedByDefault(t *testing.T) {
	fixture := newSigningRunFixture(t, signingRunFixtureOptions{})
	identityPath := filepath.Join(t.TempDir(), "App.p12")
	identityPasswordPath := filepath.Join(t.TempDir(), "identity-password")
	keychainPasswordPath := filepath.Join(t.TempDir(), "keychain-password")
	writePrivateTestFile(t, identityPath, fixture.identity)
	writePrivateTestFile(t, identityPasswordPath, []byte(fixture.password))
	writePrivateTestFile(t, keychainPasswordPath, []byte("keychain-secret"))
	keychainPath := filepath.Join(t.TempDir(), "release.keychain-db")
	resolvedKeychainPath := canonicalSigningKeychainTestPath(t, keychainPath)

	var events []string
	result, err := executeSigningKeychainInstallWith(context.Background(), signingKeychainInstallOptions{
		IdentityPath: identityPath, IdentityPasswordPath: identityPasswordPath,
		KeychainPath: keychainPath, KeychainPasswordPath: keychainPasswordPath,
	}, signingKeychainInstallDeps{
		GOOS:              "darwin",
		SecurityAvailable: true,
		Now:               func() time.Time { return fixture.now },
		AcquireLock:       acquireSigningKeychainTestLock,
		KeychainSearchList: func(context.Context) ([]string, error) {
			events = append(events, "list")
			return nil, nil
		},
		SetKeychainSearchList: func(_ context.Context, paths []string) error {
			events = append(events, "set")
			if !reflect.DeepEqual(paths, []string{resolvedKeychainPath}) {
				t.Fatalf("staged search list = %v", paths)
			}
			return nil
		},
		CreateKeychain: func(context.Context, string, []byte) (bool, error) {
			events = append(events, "create")
			return true, nil
		},
		ImportIdentity: func(context.Context, string, []byte, []byte, []byte, string) error {
			events = append(events, "import")
			return nil
		},
		RemoveKeychainSearchEntry: func(context.Context, string) error {
			events = append(events, "remove-search-entry")
			return nil
		},
		DeleteKeychain: func(context.Context, string) error { events = append(events, "delete"); return nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(events, []string{"list", "create", "set", "import", "remove-search-entry"}) {
		t.Fatalf("events = %v", events)
	}
	if result.SearchListUpdated {
		t.Fatalf("result = %+v, want unchanged search list", result)
	}
}

func TestExecuteSigningKeychainInstallRemovesPreexistingSearchEntryByDefault(t *testing.T) {
	fixture := newSigningRunFixture(t, signingRunFixtureOptions{})
	identityPath := filepath.Join(t.TempDir(), "App.p12")
	identityPasswordPath := filepath.Join(t.TempDir(), "identity-password")
	keychainPasswordPath := filepath.Join(t.TempDir(), "keychain-password")
	writePrivateTestFile(t, identityPath, fixture.identity)
	writePrivateTestFile(t, identityPasswordPath, []byte(fixture.password))
	writePrivateTestFile(t, keychainPasswordPath, []byte("keychain-secret"))
	keychainPath := filepath.Join(t.TempDir(), "release.keychain-db")
	resolvedKeychainPath := canonicalSigningKeychainTestPath(t, keychainPath)
	originalSearchList := []string{"login.keychain-db", resolvedKeychainPath, "system.keychain"}

	var events []string
	result, err := executeSigningKeychainInstallWith(context.Background(), signingKeychainInstallOptions{
		IdentityPath: identityPath, IdentityPasswordPath: identityPasswordPath,
		KeychainPath: keychainPath, KeychainPasswordPath: keychainPasswordPath,
	}, signingKeychainInstallDeps{
		GOOS:              "darwin",
		SecurityAvailable: true,
		Now:               func() time.Time { return fixture.now },
		AcquireLock:       acquireSigningKeychainTestLock,
		KeychainSearchList: func(context.Context) ([]string, error) {
			events = append(events, "list")
			return append([]string(nil), originalSearchList...), nil
		},
		SetKeychainSearchList: func(_ context.Context, paths []string) error {
			events = append(events, "set")
			want := []string{"login.keychain-db", resolvedKeychainPath, "system.keychain"}
			if !reflect.DeepEqual(paths, want) {
				t.Fatalf("staged search list = %v, want %v", paths, want)
			}
			return nil
		},
		CreateKeychain: func(context.Context, string, []byte) (bool, error) {
			events = append(events, "create")
			return true, nil
		},
		ImportIdentity: func(context.Context, string, []byte, []byte, []byte, string) error {
			events = append(events, "import")
			return nil
		},
		RemoveKeychainSearchEntry: func(context.Context, string) error {
			events = append(events, "remove-search-entry")
			return nil
		},
		DeleteKeychain: func(context.Context, string) error { events = append(events, "delete"); return nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(events, []string{"list", "create", "remove-search-entry", "set", "import", "remove-search-entry"}) {
		t.Fatalf("events = %v", events)
	}
	if result.SearchListUpdated {
		t.Fatalf("result = %+v, want unchanged search list", result)
	}
}

func TestExecuteSigningKeychainInstallRollsBackAfterImportFailure(t *testing.T) {
	fixture := newSigningRunFixture(t, signingRunFixtureOptions{})
	identityPath := filepath.Join(t.TempDir(), "App.p12")
	identityPasswordPath := filepath.Join(t.TempDir(), "identity-password")
	keychainPasswordPath := filepath.Join(t.TempDir(), "keychain-password")
	writePrivateTestFile(t, identityPath, fixture.identity)
	writePrivateTestFile(t, identityPasswordPath, []byte(fixture.password))
	writePrivateTestFile(t, keychainPasswordPath, []byte("keychain-secret"))
	keychainPath := filepath.Join(t.TempDir(), "release.keychain-db")
	resolvedKeychainPath := canonicalSigningKeychainTestPath(t, keychainPath)

	deleted := false
	deps := signingKeychainInstallDeps{
		GOOS:              "darwin",
		SecurityAvailable: true,
		Now:               func() time.Time { return fixture.now },
		AcquireLock:       acquireSigningKeychainTestLock,
		CreateKeychain:    func(context.Context, string, []byte) (bool, error) { return true, nil },
		ImportIdentity: func(context.Context, string, []byte, []byte, []byte, string) error {
			return errors.New("import failed")
		},
		KeychainSearchList:        func(context.Context) ([]string, error) { return nil, nil },
		SetKeychainSearchList:     func(context.Context, []string) error { return nil },
		RemoveKeychainSearchEntry: func(context.Context, string) error { return nil },
		DeleteKeychain: func(_ context.Context, path string) error {
			deleted = true
			if path != resolvedKeychainPath {
				t.Fatalf("rollback path = %q", path)
			}
			return nil
		},
	}
	_, err := executeSigningKeychainInstallWith(context.Background(), signingKeychainInstallOptions{
		IdentityPath: identityPath, IdentityPasswordPath: identityPasswordPath,
		KeychainPath: keychainPath, KeychainPasswordPath: keychainPasswordPath,
	}, deps)
	if err == nil || !strings.Contains(err.Error(), "import failed") || !deleted {
		t.Fatalf("error = %v deleted=%v", err, deleted)
	}
}

func TestExecuteSigningKeychainInstallAppleIntermediates(t *testing.T) {
	fixture := newSigningRunFixture(t, signingRunFixtureOptions{})
	identityPath := filepath.Join(t.TempDir(), "App.p12")
	identityPasswordPath := filepath.Join(t.TempDir(), "identity-password")
	keychainPasswordPath := filepath.Join(t.TempDir(), "keychain-password")
	writePrivateTestFile(t, identityPath, fixture.identity)
	writePrivateTestFile(t, identityPasswordPath, []byte(fixture.password))
	writePrivateTestFile(t, keychainPasswordPath, []byte("keychain-secret"))
	inspection, err := inspectSigningRunIdentity(fixture.identity, []byte(fixture.password), fixture.now)
	if err != nil {
		t.Fatal(err)
	}
	issuer := inspection.Certificate
	expired := *issuer
	expired.NotAfter = fixture.now.Add(-time.Hour)
	_, unrelated := makeSigningRunCertificate(t, "Unrelated", "", fixture.now)
	issuerSHA256 := signingCertificateSHA256(issuer)

	tests := []struct {
		name       string
		flag       bool
		embedded   []artifacts.AppleIntermediateCertificate
		importErr  error
		wantEvents []string
		wantResult []asc.SigningKeychainIntermediate
		wantStderr string
		wantErr    string
	}{
		{name: "flag off", embedded: []artifacts.AppleIntermediateCertificate{{Name: "issuer", Certificate: issuer}}, wantEvents: []string{"create", "set", "import"}},
		{name: "issuer added before identity import", flag: true, embedded: []artifacts.AppleIntermediateCertificate{{Name: "unrelated", Certificate: unrelated}, {Name: "issuer", Certificate: issuer}}, wantEvents: []string{"create", "set", "add-certificate", "import"}, wantResult: []asc.SigningKeychainIntermediate{{Name: "issuer", SHA256: issuerSHA256, Status: "added"}}},
		{name: "expired issuer skipped", flag: true, embedded: []artifacts.AppleIntermediateCertificate{{Name: "issuer", Certificate: &expired}}, wantEvents: []string{"create", "set", "import"}, wantResult: []asc.SigningKeychainIntermediate{{Name: "issuer", SHA256: issuerSHA256, Status: "skipped-expired"}}, wantStderr: "Warning: skipping expired Apple intermediate certificate issuer"},
		{name: "no matching issuer", flag: true, embedded: []artifacts.AppleIntermediateCertificate{{Name: "unrelated", Certificate: unrelated}}, wantEvents: []string{"create", "set", "import"}, wantStderr: `Warning: no embedded Apple intermediate certificate matches the identity issuer "Distribution"`},
		{name: "failed import rolls back", flag: true, embedded: []artifacts.AppleIntermediateCertificate{{Name: "issuer", Certificate: issuer}}, importErr: errors.New("add failed"), wantEvents: []string{"create", "set", "add-certificate", "delete", "set"}, wantErr: "import Apple intermediate certificate issuer: add failed"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var events []string
			var stderrBuffer strings.Builder
			deps := signingKeychainInstallDeps{
				GOOS:              "darwin",
				SecurityAvailable: true,
				Now:               func() time.Time { return fixture.now },
				Stderr:            &stderrBuffer,
				AcquireLock:       acquireSigningKeychainTestLock,
				CreateKeychain: func(context.Context, string, []byte) (bool, error) {
					events = append(events, "create")
					return true, nil
				},
				KeychainSearchList:        func(context.Context) ([]string, error) { return nil, nil },
				SetKeychainSearchList:     func(context.Context, []string) error { events = append(events, "set"); return nil },
				RemoveKeychainSearchEntry: func(context.Context, string) error { return nil },
				DeleteKeychain:            func(context.Context, string) error { events = append(events, "delete"); return nil },
				ImportIdentity: func(context.Context, string, []byte, []byte, []byte, string) error {
					events = append(events, "import")
					return nil
				},
				AppleIntermediates: func() ([]artifacts.AppleIntermediateCertificate, error) { return test.embedded, nil },
				ImportCertificate: func(_ context.Context, path string, der []byte) error {
					events = append(events, "add-certificate")
					if filepath.Base(path) != "release.keychain-db" || !reflect.DeepEqual(der, issuer.Raw) {
						t.Fatalf("certificate import path=%q", path)
					}
					return test.importErr
				},
			}
			result, err := executeSigningKeychainInstallWith(context.Background(), signingKeychainInstallOptions{
				IdentityPath: identityPath, IdentityPasswordPath: identityPasswordPath,
				KeychainPath: filepath.Join(t.TempDir(), "release.keychain-db"), KeychainPasswordPath: keychainPasswordPath,
				AddToSearchList: true, AddAppleIntermediates: test.flag,
			}, deps)
			stderr := stderrBuffer.String()
			if test.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), test.wantErr) {
					t.Fatalf("error = %v, want %q", err, test.wantErr)
				}
			} else if err != nil {
				t.Fatal(err)
			} else if !reflect.DeepEqual(result.Intermediates, test.wantResult) {
				t.Fatalf("intermediates = %+v, want %+v", result.Intermediates, test.wantResult)
			}
			if !reflect.DeepEqual(events, test.wantEvents) {
				t.Fatalf("events = %v, want %v", events, test.wantEvents)
			}
			if !strings.Contains(stderr, test.wantStderr) || (test.wantStderr == "" && stderr != "") {
				t.Fatalf("stderr = %q, want %q", stderr, test.wantStderr)
			}
		})
	}
}

func TestExecuteSigningKeychainInstallRestoresStaleSearchListEntryAfterFailure(t *testing.T) {
	fixture := newSigningRunFixture(t, signingRunFixtureOptions{})
	identityPath := filepath.Join(t.TempDir(), "App.p12")
	identityPasswordPath := filepath.Join(t.TempDir(), "identity-password")
	keychainPasswordPath := filepath.Join(t.TempDir(), "keychain-password")
	writePrivateTestFile(t, identityPath, fixture.identity)
	writePrivateTestFile(t, identityPasswordPath, []byte(fixture.password))
	writePrivateTestFile(t, keychainPasswordPath, []byte("keychain-secret"))
	keychainPath := filepath.Join(t.TempDir(), "release.keychain-db")
	resolvedKeychainPath := canonicalSigningKeychainTestPath(t, keychainPath)
	originalSearchList := []string{"login.keychain-db", resolvedKeychainPath}

	deleted := false
	removeCalls := 0
	var restoredSearchList []string
	_, err := executeSigningKeychainInstallWith(context.Background(), signingKeychainInstallOptions{
		IdentityPath: identityPath, IdentityPasswordPath: identityPasswordPath,
		KeychainPath: keychainPath, KeychainPasswordPath: keychainPasswordPath,
	}, signingKeychainInstallDeps{
		GOOS:              "darwin",
		SecurityAvailable: true,
		Now:               func() time.Time { return fixture.now },
		AcquireLock:       acquireSigningKeychainTestLock,
		CreateKeychain:    func(context.Context, string, []byte) (bool, error) { return true, nil },
		ImportIdentity: func(context.Context, string, []byte, []byte, []byte, string) error {
			return errors.New("import failed")
		},
		KeychainSearchList: func(context.Context) ([]string, error) {
			return append([]string(nil), originalSearchList...), nil
		},
		SetKeychainSearchList: func(_ context.Context, paths []string) error {
			restoredSearchList = append([]string(nil), paths...)
			return nil
		},
		RemoveKeychainSearchEntry: func(context.Context, string) error {
			removeCalls++
			return nil
		},
		DeleteKeychain: func(context.Context, string) error {
			deleted = true
			return nil
		},
	})
	if err == nil || !strings.Contains(err.Error(), "import failed") {
		t.Fatalf("error = %v, want import failure", err)
	}
	if !deleted || removeCalls != 1 {
		t.Fatalf("deleted=%v removeCalls=%d", deleted, removeCalls)
	}
	if !reflect.DeepEqual(restoredSearchList, originalSearchList) {
		t.Fatalf("restored search list = %v, want %v", restoredSearchList, originalSearchList)
	}
}

func TestExecuteSigningKeychainInstallRollsBackAfterCancellationWithIndependentContext(t *testing.T) {
	fixture := newSigningRunFixture(t, signingRunFixtureOptions{})
	identityPath := filepath.Join(t.TempDir(), "App.p12")
	identityPasswordPath := filepath.Join(t.TempDir(), "identity-password")
	keychainPasswordPath := filepath.Join(t.TempDir(), "keychain-password")
	writePrivateTestFile(t, identityPath, fixture.identity)
	writePrivateTestFile(t, identityPasswordPath, []byte(fixture.password))
	writePrivateTestFile(t, keychainPasswordPath, []byte("keychain-secret"))
	keychainPath := filepath.Join(t.TempDir(), "release.keychain-db")

	ctx, cancel := context.WithCancel(context.Background())
	deleted := false
	restored := false
	_, err := executeSigningKeychainInstallWith(ctx, signingKeychainInstallOptions{
		IdentityPath: identityPath, IdentityPasswordPath: identityPasswordPath,
		KeychainPath: keychainPath, KeychainPasswordPath: keychainPasswordPath,
		AddToSearchList: true,
	}, signingKeychainInstallDeps{
		GOOS:              "darwin",
		SecurityAvailable: true,
		Now:               func() time.Time { return fixture.now },
		AcquireLock:       acquireSigningKeychainTestLock,
		CreateKeychain:    func(context.Context, string, []byte) (bool, error) { return true, nil },
		ImportIdentity: func(context.Context, string, []byte, []byte, []byte, string) error {
			cancel()
			return context.Canceled
		},
		KeychainSearchList: func(context.Context) ([]string, error) { return []string{"login.keychain-db"}, nil },
		SetKeychainSearchList: func(cleanupCtx context.Context, paths []string) error {
			if cleanupCtx.Err() != nil {
				t.Fatalf("rollback search-list context is canceled: %v", cleanupCtx.Err())
			}
			restored = reflect.DeepEqual(paths, []string{"login.keychain-db"})
			return nil
		},
		DeleteKeychain: func(cleanupCtx context.Context, _ string) error {
			if cleanupCtx.Err() != nil {
				t.Fatalf("rollback deletion context is canceled: %v", cleanupCtx.Err())
			}
			deleted = true
			return nil
		},
	})
	if !errors.Is(err, context.Canceled) || !deleted || !restored {
		t.Fatalf("error=%v deleted=%v restored=%v", err, deleted, restored)
	}
}

func TestExecuteSigningKeychainInstallRollsBackWhenCanceledAfterFinalMutation(t *testing.T) {
	fixture := newSigningRunFixture(t, signingRunFixtureOptions{})
	identityPath := filepath.Join(t.TempDir(), "App.p12")
	identityPasswordPath := filepath.Join(t.TempDir(), "identity-password")
	keychainPasswordPath := filepath.Join(t.TempDir(), "keychain-password")
	writePrivateTestFile(t, identityPath, fixture.identity)
	writePrivateTestFile(t, identityPasswordPath, []byte(fixture.password))
	writePrivateTestFile(t, keychainPasswordPath, []byte("keychain-secret"))
	keychainPath := filepath.Join(t.TempDir(), "release.keychain-db")

	ctx, cancel := context.WithCancel(context.Background())
	deleted := false
	restored := false
	_, err := executeSigningKeychainInstallWith(ctx, signingKeychainInstallOptions{
		IdentityPath: identityPath, IdentityPasswordPath: identityPasswordPath,
		KeychainPath: keychainPath, KeychainPasswordPath: keychainPasswordPath,
		AddToSearchList: true,
	}, signingKeychainInstallDeps{
		GOOS:               "darwin",
		SecurityAvailable:  true,
		Now:                func() time.Time { return fixture.now },
		AcquireLock:        acquireSigningKeychainTestLock,
		CreateKeychain:     func(context.Context, string, []byte) (bool, error) { return true, nil },
		ImportIdentity:     func(context.Context, string, []byte, []byte, []byte, string) error { return nil },
		KeychainSearchList: func(context.Context) ([]string, error) { return []string{"login.keychain-db"}, nil },
		SetKeychainSearchList: func(_ context.Context, paths []string) error {
			if len(paths) == 2 {
				cancel()
				return nil
			}
			restored = reflect.DeepEqual(paths, []string{"login.keychain-db"})
			return nil
		},
		DeleteKeychain: func(cleanupCtx context.Context, _ string) error {
			if cleanupCtx.Err() != nil {
				t.Fatalf("rollback deletion context is canceled: %v", cleanupCtx.Err())
			}
			deleted = true
			return nil
		},
	})
	if !errors.Is(err, context.Canceled) || !deleted || !restored {
		t.Fatalf("error=%v deleted=%v restored=%v", err, deleted, restored)
	}
}

func TestExecuteSigningKeychainInstallRollsBackAfterSearchListFailure(t *testing.T) {
	fixture := newSigningRunFixture(t, signingRunFixtureOptions{})
	identityPath := filepath.Join(t.TempDir(), "App.p12")
	identityPasswordPath := filepath.Join(t.TempDir(), "identity-password")
	keychainPasswordPath := filepath.Join(t.TempDir(), "keychain-password")
	writePrivateTestFile(t, identityPath, fixture.identity)
	writePrivateTestFile(t, identityPasswordPath, []byte(fixture.password))
	writePrivateTestFile(t, keychainPasswordPath, []byte("keychain-secret"))
	keychainPath := filepath.Join(t.TempDir(), "release.keychain-db")

	originalSearchList := []string{"login.keychain-db"}
	deleted := false
	var setCalls [][]string
	deps := signingKeychainInstallDeps{
		GOOS:              "darwin",
		SecurityAvailable: true,
		Now:               func() time.Time { return fixture.now },
		AcquireLock:       acquireSigningKeychainTestLock,
		CreateKeychain:    func(context.Context, string, []byte) (bool, error) { return true, nil },
		ImportIdentity:    func(context.Context, string, []byte, []byte, []byte, string) error { return nil },
		KeychainSearchList: func(context.Context) ([]string, error) {
			return append([]string(nil), originalSearchList...), nil
		},
		SetKeychainSearchList: func(_ context.Context, paths []string) error {
			setCalls = append(setCalls, append([]string(nil), paths...))
			if len(setCalls) == 1 {
				return errors.New("search list failed")
			}
			return nil
		},
		DeleteKeychain: func(context.Context, string) error {
			deleted = true
			return nil
		},
	}
	_, err := executeSigningKeychainInstallWith(context.Background(), signingKeychainInstallOptions{
		IdentityPath: identityPath, IdentityPasswordPath: identityPasswordPath,
		KeychainPath: keychainPath, KeychainPasswordPath: keychainPasswordPath,
		AddToSearchList: true,
	}, deps)
	if err == nil || !strings.Contains(err.Error(), "search list failed") || !deleted {
		t.Fatalf("error = %v deleted=%v", err, deleted)
	}
	if len(setCalls) != 2 || !reflect.DeepEqual(setCalls[1], originalSearchList) {
		t.Fatalf("search-list calls = %v, want failed activation followed by exact restoration", setCalls)
	}
}

func TestExecuteSigningKeychainInstallRollbackPreservesPreexistingStaleSearchEntry(t *testing.T) {
	fixture := newSigningRunFixture(t, signingRunFixtureOptions{})
	identityPath := filepath.Join(t.TempDir(), "App.p12")
	identityPasswordPath := filepath.Join(t.TempDir(), "identity-password")
	keychainPasswordPath := filepath.Join(t.TempDir(), "keychain-password")
	writePrivateTestFile(t, identityPath, fixture.identity)
	writePrivateTestFile(t, identityPasswordPath, []byte(fixture.password))
	writePrivateTestFile(t, keychainPasswordPath, []byte("keychain-secret"))
	keychainPath := filepath.Join(t.TempDir(), "release.keychain-db")
	resolvedKeychainPath := canonicalSigningKeychainTestPath(t, keychainPath)
	originalSearchList := []string{"login.keychain-db", resolvedKeychainPath}

	var restored []string
	_, err := executeSigningKeychainInstallWith(context.Background(), signingKeychainInstallOptions{
		IdentityPath: identityPath, IdentityPasswordPath: identityPasswordPath,
		KeychainPath: keychainPath, KeychainPasswordPath: keychainPasswordPath,
		AddToSearchList: true,
	}, signingKeychainInstallDeps{
		GOOS:              "darwin",
		SecurityAvailable: true,
		Now:               func() time.Time { return fixture.now },
		AcquireLock:       acquireSigningKeychainTestLock,
		CreateKeychain:    func(context.Context, string, []byte) (bool, error) { return true, nil },
		ImportIdentity: func(context.Context, string, []byte, []byte, []byte, string) error {
			return errors.New("import failed")
		},
		KeychainSearchList:    func(context.Context) ([]string, error) { return append([]string(nil), originalSearchList...), nil },
		SetKeychainSearchList: func(_ context.Context, paths []string) error { restored = append([]string(nil), paths...); return nil },
		DeleteKeychain:        func(context.Context, string) error { return nil },
	})
	if err == nil || !strings.Contains(err.Error(), "import failed") {
		t.Fatalf("error = %v, want import failure", err)
	}
	if !reflect.DeepEqual(restored, originalSearchList) {
		t.Fatalf("restored search list = %v, want %v", restored, originalSearchList)
	}
}

func canonicalSigningKeychainTestPath(t *testing.T, path string) string {
	t.Helper()
	parent, err := filepath.EvalSymlinks(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Join(parent, filepath.Base(path))
}

func acquireSigningKeychainTestLock(context.Context) (func() error, error) {
	return func() error { return nil }, nil
}
