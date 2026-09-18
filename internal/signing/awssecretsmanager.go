package signing

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager/types"
)

const (
	// awsSecretMaxEncodedBytes bounds one base64-encoded artifact so an
	// oversize artifact fails before any AWS request.
	awsSecretMaxEncodedBytes = 60 << 10
	awsSecretNameMaxLength   = 512
	awsMaxRemoteArtifacts    = 1024
	awsListPageSize          = 100
	awsMaxListPages          = 50
)

var (
	awsRegionPattern     = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,31}$`)
	awsSecretNamePattern = regexp.MustCompile(`^[A-Za-z0-9/_+=.@-]+$`)
)

// SecretsManagerAPI is the subset of AWS Secrets Manager used to transport
// encrypted signing artifacts. Tests substitute a stub so no test dials AWS.
type SecretsManagerAPI interface {
	ListSecrets(ctx context.Context, params *secretsmanager.ListSecretsInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.ListSecretsOutput, error)
	GetSecretValue(ctx context.Context, params *secretsmanager.GetSecretValueInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error)
	CreateSecret(ctx context.Context, params *secretsmanager.CreateSecretInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.CreateSecretOutput, error)
	PutSecretValue(ctx context.Context, params *secretsmanager.PutSecretValueInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.PutSecretValueOutput, error)
}

// AWSSecretsManagerOptions configures an experimental AWS Secrets Manager
// store for already-encrypted signing artifacts.
type AWSSecretsManagerOptions struct {
	// Region is the AWS region holding the secrets.
	Region string
	// Prefix scopes the artifacts this store owns inside the account.
	Prefix string
	// API overrides the SDK client in tests.
	API SecretsManagerAPI
	// MaxArtifacts bounds how many prefixed artifacts may be transported.
	MaxArtifacts int
	// RequestContext bounds one Secrets Manager request.
	RequestContext RequestContextFunc
}

// AWSSecretsManagerStore transports encrypted signing artifacts as base64
// secret strings. It never receives the sync password and never decrypts an
// artifact.
type AWSSecretsManagerStore struct {
	api            SecretsManagerAPI
	region         string
	prefix         string
	maxArtifacts   int
	requestContext RequestContextFunc
}

// NewAWSSecretsManagerStore validates the locator and resolves credentials
// from the standard AWS environment when no client is supplied.
func NewAWSSecretsManagerStore(ctx context.Context, options AWSSecretsManagerOptions) (*AWSSecretsManagerStore, error) {
	region := strings.TrimSpace(options.Region)
	if region == "" {
		return nil, errors.New("AWS region must not be empty")
	}
	if !awsRegionPattern.MatchString(region) {
		return nil, errors.New("AWS region must be a lowercase region identifier such as us-east-1")
	}
	prefix := strings.TrimSpace(options.Prefix)
	if err := ValidateArtifactPrefix(prefix); err != nil {
		return nil, err
	}

	api := options.API
	if api == nil {
		// Credential discovery can reach container or instance metadata, so
		// it shares the per-request budget used by the API calls below.
		loadCtx, cancel := boundedRequestContext(options.RequestContext, ctx)
		configuration, err := config.LoadDefaultConfig(loadCtx, config.WithRegion(region))
		cancel()
		if err != nil {
			return nil, fmt.Errorf("load AWS configuration: %w", err)
		}
		api = secretsmanager.NewFromConfig(configuration)
	}
	store := &AWSSecretsManagerStore{
		api:            api,
		region:         region,
		prefix:         prefix,
		maxArtifacts:   options.MaxArtifacts,
		requestContext: options.RequestContext,
	}
	if store.maxArtifacts <= 0 {
		store.maxArtifacts = awsMaxRemoteArtifacts
	}
	return store, nil
}

// Locator returns a description of the configured store. It contains no
// credential material.
func (s *AWSSecretsManagerStore) Locator() string {
	return "aws-secrets-manager://" + s.region + "/" + s.prefix
}

// Fetch downloads every encrypted artifact under the configured prefix into
// the local ciphertext store.
func (s *AWSSecretsManagerStore) Fetch(ctx context.Context, store ArtifactStore) error {
	names, err := s.listPrefixedSecrets(ctx)
	if err != nil {
		return err
	}
	relPaths := make([]string, 0, len(names))
	for relPath := range names {
		relPaths = append(relPaths, relPath)
	}
	sort.Strings(relPaths)
	if err := ValidateEncryptedRepositoryPaths(relPaths); err != nil {
		return fmt.Errorf("aws secrets manager: %w", err)
	}
	for _, relPath := range relPaths {
		name := names[relPath]
		output, err := s.getSecretValue(ctx, name)
		if err != nil {
			return fmt.Errorf("aws secrets manager: read secret %s: %w", name, err)
		}
		if output == nil || output.SecretString == nil {
			return fmt.Errorf("aws secrets manager: secret %s has no string value", name)
		}
		encoded := strings.TrimSpace(*output.SecretString)
		if len(encoded) > awsSecretMaxEncodedBytes {
			return fmt.Errorf("aws secrets manager: secret %s exceeds the %d-byte encoded artifact limit", name, awsSecretMaxEncodedBytes)
		}
		ciphertext, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			return fmt.Errorf("aws secrets manager: secret %s is not base64-encoded ciphertext", name)
		}
		if len(ciphertext) == 0 {
			return fmt.Errorf("aws secrets manager: secret %s is empty", name)
		}
		if len(ciphertext) > MaxEncryptedArtifactBytes {
			return fmt.Errorf("aws secrets manager: secret %s exceeds the %d-byte size limit", name, MaxEncryptedArtifactBytes)
		}
		if err := store.WriteEncryptedArtifact(relPath, ciphertext); err != nil {
			return fmt.Errorf("aws secrets manager: store %s: %w", relPath, err)
		}
	}
	return nil
}

// Publish stores every local encrypted artifact as a base64 secret string.
// Secrets outside the configured prefix are never inspected, and no secret is
// ever deleted.
func (s *AWSSecretsManagerStore) Publish(ctx context.Context, store ArtifactStore) error {
	local, err := store.ListEncryptedFiles()
	if err != nil {
		return err
	}
	sort.Strings(local)
	if len(local) > s.maxArtifacts {
		return fmt.Errorf("aws secrets manager: %d encrypted artifacts exceed the %d-artifact limit", len(local), s.maxArtifacts)
	}

	type pendingSecret struct {
		name    string
		encoded string
	}
	pending := make([]pendingSecret, 0, len(local))
	for _, relPath := range local {
		name, err := s.secretName(relPath)
		if err != nil {
			return err
		}
		ciphertext, err := store.ReadEncryptedArtifact(relPath)
		if err != nil {
			return fmt.Errorf("aws secrets manager: read %s: %w", relPath, err)
		}
		if len(ciphertext) == 0 {
			return fmt.Errorf("aws secrets manager: encrypted artifact %s is empty", relPath)
		}
		if len(ciphertext) > MaxEncryptedArtifactBytes {
			return fmt.Errorf("aws secrets manager: encrypted artifact %s exceeds the %d-byte size limit", relPath, MaxEncryptedArtifactBytes)
		}
		encoded := base64.StdEncoding.EncodeToString(ciphertext)
		if len(encoded) > awsSecretMaxEncodedBytes {
			return fmt.Errorf(
				"aws secrets manager: encrypted artifact %s is %d bytes base64-encoded, above the %d-byte secret limit",
				relPath, len(encoded), awsSecretMaxEncodedBytes,
			)
		}
		pending = append(pending, pendingSecret{name: name, encoded: encoded})
	}

	published, err := s.listPrefixedSecrets(ctx)
	if err != nil {
		return err
	}
	existing := make(map[string]struct{}, len(published))
	for _, name := range published {
		existing[name] = struct{}{}
	}

	for _, secret := range pending {
		if _, ok := existing[secret.name]; ok {
			unchanged, err := s.hasSecretValue(ctx, secret.name, secret.encoded)
			if err != nil {
				return err
			}
			// A new version per push would consume the account's secret
			// version quota even when the ciphertext did not change.
			if unchanged {
				continue
			}
			if err := s.putSecretValue(ctx, secret.name, secret.encoded); err != nil {
				return fmt.Errorf("aws secrets manager: update secret %s: %w", secret.name, err)
			}
			continue
		}
		err := s.createSecret(ctx, secret.name, secret.encoded)
		if err == nil {
			continue
		}
		var exists *types.ResourceExistsException
		if !errors.As(err, &exists) {
			return fmt.Errorf("aws secrets manager: create secret %s: %w", secret.name, err)
		}
		if err := s.putSecretValue(ctx, secret.name, secret.encoded); err != nil {
			return fmt.Errorf("aws secrets manager: update secret %s: %w", secret.name, err)
		}
	}
	return nil
}

// Each Secrets Manager call is bounded separately so a stalled request cannot
// hang a whole multi-artifact push or pull.
func (s *AWSSecretsManagerStore) getSecretValue(ctx context.Context, name string) (*secretsmanager.GetSecretValueOutput, error) {
	requestCtx, cancel := boundedRequestContext(s.requestContext, ctx)
	defer cancel()
	return s.api.GetSecretValue(requestCtx, &secretsmanager.GetSecretValueInput{SecretId: aws.String(name)})
}

func (s *AWSSecretsManagerStore) createSecret(ctx context.Context, name, encoded string) error {
	requestCtx, cancel := boundedRequestContext(s.requestContext, ctx)
	defer cancel()
	_, err := s.api.CreateSecret(requestCtx, &secretsmanager.CreateSecretInput{
		Name:         aws.String(name),
		SecretString: aws.String(encoded),
		Description:  aws.String("Encrypted App Store Connect signing artifact"),
	})
	return err
}

func (s *AWSSecretsManagerStore) putSecretValue(ctx context.Context, name, encoded string) error {
	requestCtx, cancel := boundedRequestContext(s.requestContext, ctx)
	defer cancel()
	_, err := s.api.PutSecretValue(requestCtx, &secretsmanager.PutSecretValueInput{
		SecretId:     aws.String(name),
		SecretString: aws.String(encoded),
	})
	return err
}

func (s *AWSSecretsManagerStore) hasSecretValue(ctx context.Context, name, encoded string) (bool, error) {
	output, err := s.getSecretValue(ctx, name)
	if err != nil {
		var missing *types.ResourceNotFoundException
		if errors.As(err, &missing) {
			return false, nil
		}
		return false, fmt.Errorf("aws secrets manager: read secret %s: %w", name, err)
	}
	if output == nil || output.SecretString == nil {
		return false, nil
	}
	return strings.TrimSpace(*output.SecretString) == encoded, nil
}

func (s *AWSSecretsManagerStore) secretName(relPath string) (string, error) {
	name, err := remoteArtifactName(s.prefix, relPath)
	if err != nil {
		return "", fmt.Errorf("aws secrets manager: %w", err)
	}
	if len(name) > awsSecretNameMaxLength {
		return "", fmt.Errorf("aws secrets manager: secret name for %s exceeds %d characters", relPath, awsSecretNameMaxLength)
	}
	if !awsSecretNamePattern.MatchString(name) {
		return "", fmt.Errorf(
			"aws secrets manager: %s cannot be stored because AWS secret names allow only letters, digits, and the characters /_+=.@-",
			relPath,
		)
	}
	return name, nil
}

func (s *AWSSecretsManagerStore) listPrefixedSecrets(ctx context.Context) (map[string]string, error) {
	prefixed := make(map[string]string)
	var nextToken *string
	for page := 0; page < awsMaxListPages; page++ {
		requestCtx, cancel := boundedRequestContext(s.requestContext, ctx)
		output, err := s.api.ListSecrets(requestCtx, &secretsmanager.ListSecretsInput{
			MaxResults: aws.Int32(awsListPageSize),
			NextToken:  nextToken,
			Filters: []types.Filter{{
				Key:    types.FilterNameStringTypeName,
				Values: []string{s.prefix + "/"},
			}},
		})
		cancel()
		if err != nil {
			return nil, fmt.Errorf("aws secrets manager: list secrets: %w", err)
		}
		if output == nil {
			return nil, errors.New("aws secrets manager: list secrets returned no response")
		}
		for _, secret := range output.SecretList {
			if secret.Name == nil {
				continue
			}
			name := *secret.Name
			relPath, scoped := remoteArtifactRelativePath(s.prefix, name)
			if !scoped {
				continue
			}
			if err := validateRemoteArtifactRelativePath(relPath); err != nil {
				return nil, fmt.Errorf("aws secrets manager: %w", err)
			}
			prefixed[relPath] = name
			if len(prefixed) > s.maxArtifacts {
				return nil, fmt.Errorf("aws secrets manager: prefix holds more than %d encrypted artifacts", s.maxArtifacts)
			}
		}
		if output.NextToken == nil || strings.TrimSpace(*output.NextToken) == "" {
			return prefixed, nil
		}
		nextToken = output.NextToken
	}
	return nil, fmt.Errorf("aws secrets manager: list secrets exceeded %d pages", awsMaxListPages)
}
