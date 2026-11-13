package web

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/rootfs"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/secureopen"
	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

var checkIndividualAPIKeyOutputPublicationFn = func(root rootfs.Root, probeName string) error {
	return root.CheckCreateNewFileAtomic(probeName, 0o600)
}

// WebAPIKeysCreateIndividualCommand creates and registers an individual API
// key for the authenticated Apple Account user.
func WebAPIKeysCreateIndividualCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web api-keys create-individual", flag.ExitOnError)

	userID := fs.String("user-id", "", "App Store Connect user UUID")
	outputDir := fs.String("output-dir", "", "Directory for ApiKey_<KEY_ID>.p8")
	confirm := fs.Bool("confirm", false, "Confirm creation of an individual API key")
	authFlags := bindWebSessionFlags(fs)
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "create-individual",
		ShortUsage: "asc web api-keys create-individual --user-id USER_UUID --output-dir DIR --confirm [flags]",
		ShortHelp:  "Create an individual API key and save its local P8.",
		LongHelp: `WEB SESSION WORKFLOWS

Create an App Store Connect individual API key for one user through the
authenticated Apple Account web session. The user UUID is verified by reading
the user resource and matching its username to the authenticated session
before any create request. An active individual key for that user also blocks
creation.

The command generates an ECDSA P-256 keypair locally, registers only the public
key with App Store Connect, and saves the PKCS#8 private key as
ApiKey_<KEY_ID>.p8 with mode 0600. Existing files are never overwritten. Key
material is never written to command output. If App Store Connect creates a key
but local private-key materialization is not confirmed, that key remains active
without a registered public key. The command reports both the staged and
canonical paths; inspect both to locate the private artifact, then inspect or
revoke the identified key before starting another create. The command does not
automatically register, retry, or revoke that uncertain operation.

Creating an individual API key requires explicit confirmation.

The private key is first persisted in a random 0600 staging file before the
remote create. After the POST succeeds, the actor-filtered list identifies the
one newly active key absent from the preflight snapshot. The staged file is
then moved to its canonical name without replacing an existing file. If list
resolution, materialization, registration, or post-read is uncertain, the
local artifact is retained for recovery and the command does not retry the
remote operation.

Examples:
  asc web api-keys create-individual --user-id USER_UUID --output-dir ~/.asc/keys --confirm
  asc web api-keys create-individual --user-id USER_UUID --output-dir ~/.asc/keys --confirm --output json

`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if _, err := shared.ValidateOutputFormat(*output.Output, *output.Pretty); err != nil {
				return shared.UsageError(err.Error())
			}
			userIDValue := strings.TrimSpace(*userID)
			if userIDValue == "" {
				return shared.UsageError("--user-id is required")
			}
			outputDirValue := strings.TrimSpace(*outputDir)
			if outputDirValue == "" {
				return shared.UsageError("--output-dir is required")
			}
			if !*confirm {
				return shared.UsageError("--confirm is required")
			}

			outputRoot, err := rootfs.New(outputDirValue)
			if err != nil {
				return shared.UsageErrorf("invalid --output-dir: %v", err)
			}
			defer outputRoot.Close()
			if err := outputRoot.MkdirAll(".", 0o700); err != nil {
				return fmt.Errorf("prepare individual API key output directory: %w", err)
			}
			probeName, err := newIndividualAPIKeyStagingName()
			if err != nil {
				return fmt.Errorf("prepare individual API key publication probe; remote create was not attempted: %w", err)
			}
			if err := checkIndividualAPIKeyOutputPublicationFn(outputRoot, probeName); err != nil {
				return fmt.Errorf("verify individual API key output directory supports atomic no-replace private-key publication; remote create was not attempted: %w", err)
			}

			session, requestCtx, cancel, err := resolveWebSessionForCommand(ctx, authFlags)
			defer cancel()
			if err != nil {
				return err
			}
			if session == nil || strings.TrimSpace(session.UserEmail) == "" {
				return fmt.Errorf("web api-keys create-individual failed: authenticated web session did not include an email; cannot establish current user identity")
			}
			client := newWebAPIKeyClientFn(session)
			sessionEmail := strings.TrimSpace(session.UserEmail)

			var user *webcore.WebUser
			err = withWebSpinner("Verifying App Store Connect user", func() error {
				var getErr error
				user, getErr = client.GetWebUser(requestCtx, userIDValue)
				return getErr
			})
			if err != nil {
				return withWebAuthHint(err, "web api-keys create-individual")
			}
			if user == nil || strings.TrimSpace(user.ID) != userIDValue {
				return fmt.Errorf("web api-keys create-individual failed: user identity could not be established for %q", userIDValue)
			}
			if !strings.EqualFold(strings.TrimSpace(user.Username), sessionEmail) {
				return fmt.Errorf("web api-keys create-individual failed: user %q username does not match authenticated web session", userIDValue)
			}

			var existing []webcore.IndividualAPIKey
			err = withWebSpinner("Checking existing individual API keys", func() error {
				var listErr error
				existing, listErr = client.ListIndividualAPIKeysForUser(requestCtx, userIDValue)
				return listErr
			})
			if err != nil {
				return withWebAuthHint(err, "web api-keys create-individual")
			}
			existingKeyIDs := make(map[string]struct{}, len(existing))
			for _, key := range existing {
				existingKeyIDs[key.KeyID] = struct{}{}
				if key.Active {
					return fmt.Errorf("web api-keys create-individual failed: active individual API key %q already exists for user %q", key.KeyID, userIDValue)
				}
			}

			privatePEM, publicPEM, err := generateIndividualAPIKeyMaterial()
			if err != nil {
				return fmt.Errorf("generate individual API key material; remote create was not attempted: %w", err)
			}
			stagedName, err := newIndividualAPIKeyStagingName()
			if err != nil {
				return fmt.Errorf("prepare individual API key staging path; remote create was not attempted: %w", err)
			}
			stagedPath := filepath.Join(outputRoot.Path(), stagedName)
			if err := stageIndividualAPIKey(outputRoot, stagedName, privatePEM); err != nil {
				return fmt.Errorf("stage individual API key private artifact at %q; remote create was not attempted: %w", stagedPath, err)
			}

			err = withWebSpinner("Creating individual API key", func() error {
				return client.CreateIndividualAPIKey(requestCtx)
			})
			if err != nil {
				return fmt.Errorf("individual API key create request was not confirmed; the remote create may have succeeded; private key artifact was retained at %q; inspect remote state before retrying: %w", stagedPath, withWebAuthHint(err, "web api-keys create-individual"))
			}

			var createdKeys []webcore.IndividualAPIKey
			err = withWebSpinner("Locating created individual API key", func() error {
				var listErr error
				createdKeys, listErr = client.ListIndividualAPIKeysForUser(requestCtx, userIDValue)
				return listErr
			})
			if err != nil {
				return fmt.Errorf("individual API key create succeeded but the created key could not be resolved; private key artifact was retained at %q; inspect remote state before retrying: %w", stagedPath, withWebAuthHint(err, "web api-keys create-individual"))
			}
			created, err := resolveCreatedIndividualAPIKey(existingKeyIDs, createdKeys)
			if err != nil {
				return fmt.Errorf("web api-keys create-individual failed: %w; private key artifact was retained at %q; inspect remote state before retrying", err, stagedPath)
			}
			if created.PublicKeyPresent {
				return fmt.Errorf("web api-keys create-individual failed: newly created individual API key %q already has a registered public key; refusing to overwrite it; private key artifact was retained at %q", created.KeyID, stagedPath)
			}
			keyID := strings.TrimSpace(created.KeyID)

			fileName := fmt.Sprintf("ApiKey_%s.p8", keyID)
			p8Path := filepath.Join(outputRoot.Path(), fileName)
			if err := materializeIndividualAPIKey(outputRoot, stagedName, fileName); err != nil {
				return fmt.Errorf("individual API key %q was created, but its public key has not been registered because final private-key materialization was not confirmed without replacing %q; the private-key artifact may be at the staged path %q or canonical path %q; inspect both paths and inspect or revoke key %q before starting another create; no automatic retry was sent: %w", keyID, p8Path, stagedPath, p8Path, keyID, err)
			}

			err = withWebSpinner("Registering individual API key public key", func() error {
				return client.RegisterIndividualAPIKey(requestCtx, keyID, string(publicPEM))
			})
			if err != nil {
				return fmt.Errorf("individual API key %q public-key registration was not confirmed; private key artifact was retained at %q; inspect the remote key before retrying: %w", keyID, p8Path, withWebAuthHint(err, "web api-keys create-individual"))
			}

			var verified []webcore.IndividualAPIKey
			err = withWebSpinner("Verifying individual API key", func() error {
				var verifyErr error
				verified, verifyErr = client.ListIndividualAPIKeysForUser(requestCtx, userIDValue)
				return verifyErr
			})
			if err != nil {
				return fmt.Errorf("individual API key %q was created and its private key artifact was saved to %q, but post-registration verification failed: %w", keyID, p8Path, withWebAuthHint(err, "web api-keys create-individual"))
			}
			var verifiedKey *webcore.IndividualAPIKey
			for index := range verified {
				if verified[index].KeyID != keyID {
					continue
				}
				if verifiedKey != nil {
					return fmt.Errorf("individual API key %q was created and its private key artifact was saved to %q, but post-registration returned duplicate resources", keyID, p8Path)
				}
				verifiedKey = &verified[index]
			}
			if verifiedKey == nil || !verifiedKey.Active || !verifiedKey.PublicKeyPresent || !verifiedKey.MatchesPublicKey(string(publicPEM)) {
				return fmt.Errorf("individual API key %q was created and its private key artifact was saved to %q, but post-registration verification did not confirm the generated public key on an active resource", keyID, p8Path)
			}

			result := &asc.WebAPIKeyCreateIndividualResult{
				KeyID:      keyID,
				UserID:     userIDValue,
				P8Path:     p8Path,
				Active:     verifiedKey.Active,
				Registered: verifiedKey.PublicKeyPresent,
			}
			if err := shared.PrintOutput(result, *output.Output, *output.Pretty); err != nil {
				return fmt.Errorf("individual API key %q was created and its public key was registered; private key artifact saved to %q; output failed; do not retry automatically: %w", keyID, p8Path, err)
			}
			return nil
		},
	}
}

func resolveCreatedIndividualAPIKey(existingIDs map[string]struct{}, keys []webcore.IndividualAPIKey) (*webcore.IndividualAPIKey, error) {
	var candidate *webcore.IndividualAPIKey
	for index := range keys {
		key := &keys[index]
		if _, existed := existingIDs[key.KeyID]; existed || !key.Active {
			continue
		}
		if candidate != nil {
			return nil, fmt.Errorf("created individual API key could not be identified unambiguously: multiple newly active keys were returned")
		}
		candidate = key
	}
	if candidate == nil {
		return nil, fmt.Errorf("created individual API key could not be identified unambiguously: no newly active key was returned")
	}
	return candidate, nil
}

func stageIndividualAPIKey(root rootfs.Root, name string, privatePEM []byte) (err error) {
	opened, err := root.OpenRoot()
	if err != nil {
		return err
	}
	defer func() {
		err = errors.Join(err, opened.Close())
	}()
	_, err = shared.SafeWriteFileNoSymlinkWithPreparationAndCreatorInRoot(
		opened, filepath.Join(root.Path(), name), name, 0o600, false,
		".asc-api-key-private-*.p8", "",
		func(file *os.File) error { return secureopen.PreparePrivateFile(file, 0o600) },
		secureopen.OpenNewPrivateFileNoFollowInRoot,
		func(file *os.File) (int64, error) {
			written, writeErr := file.Write(privatePEM)
			return int64(written), writeErr
		},
	)
	return err
}

func newIndividualAPIKeyStagingName() (string, error) {
	nonce := make([]byte, 16)
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("generate staging name: %w", err)
	}
	return fmt.Sprintf(".asc-api-key-%x.p8", nonce), nil
}

func materializeIndividualAPIKey(root rootfs.Root, stagedName, canonicalName string) (err error) {
	opened, err := root.OpenRoot()
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := opened.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("close output directory after private key materialization: %w", closeErr))
		}
	}()
	return secureopen.RenameNoReplaceInRoot(opened, stagedName, canonicalName)
}

func generateIndividualAPIKeyMaterial() ([]byte, []byte, error) {
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, fmt.Errorf("generate P-256 keypair: %w", err)
	}
	privateDER, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		return nil, nil, fmt.Errorf("marshal PKCS#8 private key: %w", err)
	}
	publicDER, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		return nil, nil, fmt.Errorf("marshal SPKI public key: %w", err)
	}
	privatePEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privateDER})
	publicPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: publicDER})
	if len(privatePEM) == 0 || len(publicPEM) == 0 {
		return nil, nil, errors.New("generated empty PEM key material")
	}
	return privatePEM, publicPEM, nil
}
