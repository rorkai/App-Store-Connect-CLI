package signing

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/readonly"
)

func enableRemoteStoreReadOnly(t *testing.T, source string) {
	t.Helper()
	t.Setenv(readonly.EnvVar, "")
	readonly.SetFlagEnabled(false)
	t.Cleanup(func() { readonly.SetFlagEnabled(false) })
	if source == "flag" {
		readonly.SetFlagEnabled(true)
	} else {
		t.Setenv(readonly.EnvVar, "1")
	}
}

func TestRemoteStoresReadOnlyAllowsFetchButRefusesPublish(t *testing.T) {
	for _, source := range []string{"env", "flag"} {
		for _, existing := range []bool{false, true} {
			t.Run(fmt.Sprintf("gitlab/%s/existing=%v", source, existing), func(t *testing.T) {
				fake, server := newFakeGitLabServer(t)
				remote := newGitLabTestStore(t, server)
				relPath := "certs/distribution/serial.cer"
				if existing {
					fake.put(gitLabTestPrefix+"/"+relPath+EncryptedArtifactSuffix, []byte("old ciphertext"))
				}
				enableRemoteStoreReadOnly(t, source)
				local := newLocalArtifactStore(t)
				if err := remote.Fetch(context.Background(), local); err != nil {
					t.Fatalf("read-only fetch: %v", err)
				}
				if err := local.ReplaceEncryptedFile(relPath, []byte("certificate"), gitLabTestPassword); err != nil {
					t.Fatal(err)
				}
				err := remote.Publish(context.Background(), local)
				if !errors.Is(err, readonly.ErrRefused) {
					t.Fatalf("Publish error = %v, want readonly.ErrRefused", err)
				}
				fake.mu.Lock()
				defer fake.mu.Unlock()
				if len(fake.creates)+len(fake.deletes) != 0 {
					t.Fatal("mutations reached GitLab in read-only mode")
				}
			})
			t.Run(fmt.Sprintf("aws/%s/existing=%v", source, existing), func(t *testing.T) {
				name := awsTestPrefix + "/certs/distribution/serial.cer.enc"
				var mutations atomic.Int32
				server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "application/x-amz-json-1.1")
					switch r.Header.Get("X-Amz-Target") {
					case "secretsmanager.ListSecrets":
						items := []map[string]string{}
						if existing {
							items = append(items, map[string]string{"Name": name})
						}
						_ = json.NewEncoder(w).Encode(map[string]any{"SecretList": items})
					case "secretsmanager.GetSecretValue":
						_ = json.NewEncoder(w).Encode(map[string]string{"SecretString": base64.StdEncoding.EncodeToString([]byte("old ciphertext"))})
					default:
						mutations.Add(1)
						_, _ = w.Write([]byte(`{}`))
					}
				}))
				t.Cleanup(server.Close)
				client := secretsmanager.NewFromConfig(aws.Config{
					Region: awsTestRegion, HTTPClient: server.Client(),
					Credentials: credentials.NewStaticCredentialsProvider("test", "test", ""),
				}, func(o *secretsmanager.Options) { o.BaseEndpoint = aws.String(server.URL) })
				remote := newAWSTestStore(t, client)
				enableRemoteStoreReadOnly(t, source)
				local := newLocalArtifactStore(t)
				if err := remote.Fetch(context.Background(), local); err != nil {
					t.Fatalf("read-only fetch: %v", err)
				}
				if err := local.ReplaceEncryptedFile("certs/distribution/serial.cer", []byte("certificate"), awsTestPassword); err != nil {
					t.Fatal(err)
				}
				err := remote.Publish(context.Background(), local)
				if !errors.Is(err, readonly.ErrRefused) {
					t.Fatalf("Publish error = %v, want readonly.ErrRefused", err)
				}
				if mutations.Load() != 0 {
					t.Fatalf("%d mutations reached AWS in read-only mode", mutations.Load())
				}
			})
		}
	}
}
