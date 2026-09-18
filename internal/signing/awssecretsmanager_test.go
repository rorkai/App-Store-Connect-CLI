package signing

import (
	"bytes"
	"context"
	"encoding/base64"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager/types"
)

const (
	awsTestPrefix   = "asc-signing"
	awsTestRegion   = "us-east-1"
	awsTestPassword = "repository-password"
)

type stubSecretsManager struct {
	mu      sync.Mutex
	secrets map[string]string
	creates []string
	puts    []string
	gets    []string
	lists   int
}

func newStubSecretsManager() *stubSecretsManager {
	return &stubSecretsManager{secrets: make(map[string]string)}
}

func (s *stubSecretsManager) ListSecrets(_ context.Context, params *secretsmanager.ListSecretsInput, _ ...func(*secretsmanager.Options)) (*secretsmanager.ListSecretsOutput, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lists++
	filter := ""
	for _, entry := range params.Filters {
		if entry.Key == types.FilterNameStringTypeName && len(entry.Values) > 0 {
			filter = entry.Values[0]
		}
	}
	names := make([]string, 0, len(s.secrets))
	for name := range s.secrets {
		if filter == "" || strings.HasPrefix(name, filter) {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	list := make([]types.SecretListEntry, 0, len(names))
	for _, name := range names {
		list = append(list, types.SecretListEntry{Name: aws.String(name)})
	}
	return &secretsmanager.ListSecretsOutput{SecretList: list}, nil
}

func (s *stubSecretsManager) GetSecretValue(_ context.Context, params *secretsmanager.GetSecretValueInput, _ ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	name := aws.ToString(params.SecretId)
	s.gets = append(s.gets, name)
	value, ok := s.secrets[name]
	if !ok {
		return nil, &types.ResourceNotFoundException{Message: aws.String("not found")}
	}
	return &secretsmanager.GetSecretValueOutput{SecretString: aws.String(value)}, nil
}

func (s *stubSecretsManager) CreateSecret(_ context.Context, params *secretsmanager.CreateSecretInput, _ ...func(*secretsmanager.Options)) (*secretsmanager.CreateSecretOutput, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	name := aws.ToString(params.Name)
	if _, exists := s.secrets[name]; exists {
		return nil, &types.ResourceExistsException{Message: aws.String("already exists")}
	}
	s.creates = append(s.creates, name)
	s.secrets[name] = aws.ToString(params.SecretString)
	return &secretsmanager.CreateSecretOutput{Name: aws.String(name)}, nil
}

func (s *stubSecretsManager) PutSecretValue(_ context.Context, params *secretsmanager.PutSecretValueInput, _ ...func(*secretsmanager.Options)) (*secretsmanager.PutSecretValueOutput, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	name := aws.ToString(params.SecretId)
	if _, exists := s.secrets[name]; !exists {
		return nil, &types.ResourceNotFoundException{Message: aws.String("not found")}
	}
	s.puts = append(s.puts, name)
	s.secrets[name] = aws.ToString(params.SecretString)
	return &secretsmanager.PutSecretValueOutput{Name: aws.String(name)}, nil
}

func (s *stubSecretsManager) storedValue(name string) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	value, ok := s.secrets[name]
	return value, ok
}

func (s *stubSecretsManager) calls() (creates, puts, gets []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.creates...), append([]string(nil), s.puts...), append([]string(nil), s.gets...)
}

func newAWSTestStore(t *testing.T, api SecretsManagerAPI) *AWSSecretsManagerStore {
	t.Helper()
	store, err := NewAWSSecretsManagerStore(context.Background(), AWSSecretsManagerOptions{
		Region: awsTestRegion,
		Prefix: awsTestPrefix,
		API:    api,
	})
	if err != nil {
		t.Fatalf("NewAWSSecretsManagerStore() error: %v", err)
	}
	return store
}

func TestAWSSecretsManagerRoundTripsEncryptedArtifacts(t *testing.T) {
	api := newStubSecretsManager()
	remote := newAWSTestStore(t, api)

	source := newLocalArtifactStore(t)
	plaintext := []byte("provisioning-profile-plaintext")
	relPath := "profiles/appstore/com.example.app.mobileprovision"
	if err := source.WriteEncryptedFile(relPath, plaintext, awsTestPassword); err != nil {
		t.Fatalf("WriteEncryptedFile() error: %v", err)
	}

	if err := remote.Publish(context.Background(), source); err != nil {
		t.Fatalf("Publish() error: %v", err)
	}

	wantName := awsTestPrefix + "/" + relPath + EncryptedArtifactSuffix
	stored, ok := api.storedValue(wantName)
	if !ok {
		t.Fatalf("secret %q was not created", wantName)
	}
	if strings.Contains(stored, string(plaintext)) {
		t.Fatal("stored secret contains plaintext signing material")
	}
	decoded, err := base64.StdEncoding.DecodeString(stored)
	if err != nil {
		t.Fatalf("stored secret is not standard base64: %v", err)
	}
	if bytes.Contains(decoded, plaintext) {
		t.Fatal("decoded secret contains plaintext signing material")
	}
	local, err := source.ReadEncryptedArtifact(relPath)
	if err != nil {
		t.Fatalf("ReadEncryptedArtifact() error: %v", err)
	}
	if !bytes.Equal(decoded, local) {
		t.Fatal("stored secret does not match the local ciphertext")
	}

	destination := newLocalArtifactStore(t)
	if err := remote.Fetch(context.Background(), destination); err != nil {
		t.Fatalf("Fetch() error: %v", err)
	}
	fetched, err := destination.ReadEncryptedFile(relPath, awsTestPassword)
	if err != nil {
		t.Fatalf("ReadEncryptedFile() error: %v", err)
	}
	if !bytes.Equal(fetched, plaintext) {
		t.Fatalf("decrypted artifact = %q, want %q", fetched, plaintext)
	}

	creates, puts, _ := api.calls()
	if len(creates) != 1 || creates[0] != wantName {
		t.Fatalf("CreateSecret names = %v, want [%s]", creates, wantName)
	}
	if len(puts) != 0 {
		t.Fatalf("PutSecretValue names = %v, want none for a new artifact", puts)
	}
}

func TestAWSSecretsManagerPublishPutsExistingSecretValue(t *testing.T) {
	api := newStubSecretsManager()
	remote := newAWSTestStore(t, api)

	source := newLocalArtifactStore(t)
	relPath := "certs/distribution/serial.cer"
	if err := source.WriteEncryptedFile(relPath, []byte("certificate-one"), awsTestPassword); err != nil {
		t.Fatalf("WriteEncryptedFile() error: %v", err)
	}
	if err := remote.Publish(context.Background(), source); err != nil {
		t.Fatalf("Publish() error: %v", err)
	}
	if err := source.ReplaceEncryptedFile(relPath, []byte("certificate-two"), awsTestPassword); err != nil {
		t.Fatalf("ReplaceEncryptedFile() error: %v", err)
	}
	if err := remote.Publish(context.Background(), source); err != nil {
		t.Fatalf("second Publish() error: %v", err)
	}

	wantName := awsTestPrefix + "/" + relPath + EncryptedArtifactSuffix
	creates, puts, _ := api.calls()
	if len(creates) != 1 || creates[0] != wantName {
		t.Fatalf("CreateSecret names = %v, want [%s]", creates, wantName)
	}
	if len(puts) != 1 || puts[0] != wantName {
		t.Fatalf("PutSecretValue names = %v, want [%s]", puts, wantName)
	}

	destination := newLocalArtifactStore(t)
	if err := remote.Fetch(context.Background(), destination); err != nil {
		t.Fatalf("Fetch() error: %v", err)
	}
	plaintext, err := destination.ReadEncryptedFile(relPath, awsTestPassword)
	if err != nil {
		t.Fatalf("ReadEncryptedFile() error: %v", err)
	}
	if string(plaintext) != "certificate-two" {
		t.Fatalf("decrypted artifact = %q, want the updated content", plaintext)
	}
}

func TestAWSSecretsManagerPublishSkipsUnchangedSecretValues(t *testing.T) {
	api := newStubSecretsManager()
	remote := newAWSTestStore(t, api)

	source := newLocalArtifactStore(t)
	if err := source.WriteEncryptedFile("certs/distribution/serial.cer", []byte("certificate-one"), awsTestPassword); err != nil {
		t.Fatalf("WriteEncryptedFile() error: %v", err)
	}
	if err := remote.Publish(context.Background(), source); err != nil {
		t.Fatalf("Publish() error: %v", err)
	}
	if err := remote.Publish(context.Background(), source); err != nil {
		t.Fatalf("second Publish() error: %v", err)
	}

	creates, puts, _ := api.calls()
	if len(creates) != 1 {
		t.Fatalf("CreateSecret names = %v, want one create", creates)
	}
	if len(puts) != 0 {
		t.Fatalf("PutSecretValue names = %v, want no new version for unchanged ciphertext", puts)
	}
}

func TestAWSSecretsManagerRejectsOversizeArtifactBeforeCallingAWS(t *testing.T) {
	api := newStubSecretsManager()
	remote := newAWSTestStore(t, api)

	source := newLocalArtifactStore(t)
	oversize := bytes.Repeat([]byte("C"), awsSecretMaxEncodedBytes)
	if err := source.WriteEncryptedFile("certs/distribution/serial.cer", oversize, awsTestPassword); err != nil {
		t.Fatalf("WriteEncryptedFile() error: %v", err)
	}

	err := remote.Publish(context.Background(), source)
	if err == nil {
		t.Fatal("Publish() error = nil, want the encoded secret limit enforced")
	}
	if !strings.Contains(err.Error(), "base64-encoded") || !strings.Contains(err.Error(), "61440") {
		t.Fatalf("error = %q, want the encoded 60 KiB limit explained", err)
	}
	creates, puts, gets := api.calls()
	if len(creates) != 0 || len(puts) != 0 || len(gets) != 0 {
		t.Fatalf("AWS calls creates=%v puts=%v gets=%v, want none", creates, puts, gets)
	}
}

func TestAWSSecretsManagerFetchIgnoresOtherPrefixes(t *testing.T) {
	api := newStubSecretsManager()
	api.secrets["other-team/certs/distribution/foreign.cer"+EncryptedArtifactSuffix] = base64.StdEncoding.EncodeToString([]byte("foreign"))
	api.secrets["unrelated/database-password"] = base64.StdEncoding.EncodeToString([]byte("unrelated"))
	remote := newAWSTestStore(t, api)

	destination := newLocalArtifactStore(t)
	if err := remote.Fetch(context.Background(), destination); err != nil {
		t.Fatalf("Fetch() error: %v", err)
	}
	files, err := destination.ListEncryptedFiles()
	if err != nil {
		t.Fatalf("ListEncryptedFiles() error: %v", err)
	}
	if len(files) != 0 {
		t.Fatalf("fetched files = %v, want only the configured prefix", files)
	}
	_, _, gets := api.calls()
	if len(gets) != 0 {
		t.Fatalf("GetSecretValue names = %v, want no out-of-prefix read", gets)
	}
}

func TestAWSSecretsManagerRejectsUnsupportedSecretNameCharacters(t *testing.T) {
	api := newStubSecretsManager()
	remote := newAWSTestStore(t, api)

	source := newLocalArtifactStore(t)
	if err := source.WriteEncryptedFile("profiles/appstore/App Store Profile.mobileprovision", []byte("profile"), awsTestPassword); err != nil {
		t.Fatalf("WriteEncryptedFile() error: %v", err)
	}

	err := remote.Publish(context.Background(), source)
	if err == nil {
		t.Fatal("Publish() error = nil, want an unsupported secret name rejected")
	}
	if !strings.Contains(err.Error(), "/_+=.@-") {
		t.Fatalf("error = %q, want the documented AWS secret name characters", err)
	}
	creates, puts, _ := api.calls()
	if len(creates) != 0 || len(puts) != 0 {
		t.Fatalf("AWS calls creates=%v puts=%v, want none", creates, puts)
	}
}

func TestAWSSecretsManagerFetchRejectsNonBase64Secret(t *testing.T) {
	api := newStubSecretsManager()
	api.secrets[awsTestPrefix+"/certs/distribution/serial.cer"+EncryptedArtifactSuffix] = "not base64!!"
	remote := newAWSTestStore(t, api)

	err := remote.Fetch(context.Background(), newLocalArtifactStore(t))
	if err == nil {
		t.Fatal("Fetch() error = nil, want a decoding failure")
	}
	if !strings.Contains(err.Error(), "base64") {
		t.Fatalf("error = %q, want a base64 failure", err)
	}
}

func TestAWSSecretsManagerBoundsEachRequestWithTheSuppliedBudget(t *testing.T) {
	api := newStubSecretsManager()
	budgets := 0
	store, err := NewAWSSecretsManagerStore(context.Background(), AWSSecretsManagerOptions{
		Region: awsTestRegion,
		Prefix: awsTestPrefix,
		API:    api,
		RequestContext: func(ctx context.Context) (context.Context, context.CancelFunc) {
			budgets++
			return context.WithTimeout(ctx, time.Minute)
		},
	})
	if err != nil {
		t.Fatalf("NewAWSSecretsManagerStore() error: %v", err)
	}

	source := newLocalArtifactStore(t)
	if err := source.WriteEncryptedFile("certs/distribution/serial.cer", []byte("certificate"), awsTestPassword); err != nil {
		t.Fatalf("WriteEncryptedFile() error: %v", err)
	}
	if err := store.Publish(context.Background(), source); err != nil {
		t.Fatalf("Publish() error: %v", err)
	}
	if err := store.Fetch(context.Background(), newLocalArtifactStore(t)); err != nil {
		t.Fatalf("Fetch() error: %v", err)
	}
	if budgets < 4 {
		t.Fatalf("request budgets applied = %d, want one per list, create, and read", budgets)
	}
}

func TestAWSSecretsManagerBoundsCredentialDiscovery(t *testing.T) {
	t.Setenv("AWS_CONFIG_FILE", filepath.Join(t.TempDir(), "config"))
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", filepath.Join(t.TempDir(), "credentials"))
	t.Setenv("AWS_EC2_METADATA_DISABLED", "true")
	t.Setenv("AWS_ACCESS_KEY_ID", "AKIAEXAMPLE")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "example-secret")

	budgets := 0
	if _, err := NewAWSSecretsManagerStore(context.Background(), AWSSecretsManagerOptions{
		Region: awsTestRegion,
		Prefix: awsTestPrefix,
		RequestContext: func(ctx context.Context) (context.Context, context.CancelFunc) {
			budgets++
			return context.WithTimeout(ctx, time.Minute)
		},
	}); err != nil {
		t.Fatalf("NewAWSSecretsManagerStore() error: %v", err)
	}
	if budgets != 1 {
		t.Fatalf("request budgets applied during credential discovery = %d, want 1", budgets)
	}
}

func TestAWSSecretsManagerLocatorNamesRegionAndPrefix(t *testing.T) {
	remote := newAWSTestStore(t, newStubSecretsManager())
	locator := remote.Locator()
	if !strings.Contains(locator, awsTestRegion) || !strings.Contains(locator, awsTestPrefix) {
		t.Fatalf("locator = %q, want the region and prefix", locator)
	}
}

func TestNewAWSSecretsManagerStoreValidatesInputs(t *testing.T) {
	tests := []struct {
		name    string
		options AWSSecretsManagerOptions
		want    string
	}{
		{
			name:    "empty region",
			options: AWSSecretsManagerOptions{Prefix: awsTestPrefix},
			want:    "region",
		},
		{
			name:    "invalid region",
			options: AWSSecretsManagerOptions{Region: "US East 1", Prefix: awsTestPrefix},
			want:    "region",
		},
		{
			name:    "traversal prefix",
			options: AWSSecretsManagerOptions{Region: awsTestRegion, Prefix: "../escape"},
			want:    "prefix",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewAWSSecretsManagerStore(context.Background(), AWSSecretsManagerOptions{
				Region: tt.options.Region,
				Prefix: tt.options.Prefix,
				API:    newStubSecretsManager(),
			})
			if err == nil {
				t.Fatal("NewAWSSecretsManagerStore() error = nil, want a validation failure")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %q, want it to mention %q", err, tt.want)
			}
		})
	}
}
