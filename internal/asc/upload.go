package asc

import (
	"context"
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
)

// UploadOptions configure how upload operations are executed.
type UploadOptions struct {
	Concurrency int
	Client      *http.Client
	RetryOpts   RetryOptions
}

// UploadOption configures upload options.
type UploadOption func(*UploadOptions)

type uploadTask struct {
	index int
	op    UploadOperation
}

// WithUploadConcurrency sets the number of concurrent upload workers.
func WithUploadConcurrency(concurrency int) UploadOption {
	return func(opts *UploadOptions) {
		opts.Concurrency = concurrency
	}
}

// WithUploadHTTPClient sets the HTTP client used for upload operations.
func WithUploadHTTPClient(client *http.Client) UploadOption {
	return func(opts *UploadOptions) {
		opts.Client = client
	}
}

// newUploadClient creates a dedicated HTTP client for upload operations
// with appropriate timeouts and a cloned transport when possible to avoid
// sharing the connection pool with http.DefaultClient.
func newUploadClient() *http.Client {
	transport := http.DefaultTransport
	if base, ok := transport.(*http.Transport); ok {
		transport = base.Clone()
	}
	return &http.Client{
		Timeout:   ResolveUploadTimeout(),
		Transport: transport,
	}
}

// ExecuteUploadOperations performs the file uploads for the provided operations.
func ExecuteUploadOperations(ctx context.Context, filePath string, operations []UploadOperation, opts ...UploadOption) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if len(operations) == 0 {
		return errors.New("no upload operations provided")
	}

	uploadOpts := UploadOptions{
		Concurrency: 1,
		Client:      newUploadClient(),
		RetryOpts:   ResolveRetryOptions(),
	}
	for _, opt := range opts {
		opt(&uploadOpts)
	}
	if uploadOpts.Concurrency < 1 {
		return fmt.Errorf("upload concurrency must be at least 1")
	}
	if uploadOpts.Client == nil {
		uploadOpts.Client = newUploadClient()
	}
	if uploadOpts.Concurrency > len(operations) {
		uploadOpts.Concurrency = len(operations)
	}

	file, err := openUploadSourceFile(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return fmt.Errorf("stat file: %w", err)
	}
	if info.IsDir() {
		return fmt.Errorf("path %q is a directory", filePath)
	}
	size := info.Size()

	for i, op := range operations {
		if strings.TrimSpace(op.URL) == "" {
			return fmt.Errorf("upload operation %d has empty URL", i)
		}
		if op.Offset < 0 {
			return fmt.Errorf("upload operation %d has negative offset", i)
		}
		if op.Length <= 0 {
			return fmt.Errorf("upload operation %d has non-positive length", i)
		}
		if op.Offset+op.Length > size {
			return fmt.Errorf("upload operation %d exceeds file size", i)
		}
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var firstErr error
	var errOnce sync.Once
	setErr := func(err error) {
		errOnce.Do(func() {
			firstErr = err
			cancel()
		})
	}

	jobs := make(chan uploadTask)
	var wg sync.WaitGroup

	worker := func() {
		defer wg.Done()
		for task := range jobs {
			if ctx.Err() != nil {
				return
			}
			if err := executeUploadOperation(ctx, file, task, uploadOpts); err != nil {
				setErr(err)
				return
			}
		}
	}

	for i := 0; i < uploadOpts.Concurrency; i++ {
		wg.Add(1)
		go worker()
	}

sendLoop:
	for i, op := range operations {
		select {
		case <-ctx.Done():
			break sendLoop
		case jobs <- uploadTask{index: i, op: op}:
		}
	}
	close(jobs)

	wg.Wait()
	return firstErr
}

func openUploadSourceFile(filePath string) (*os.File, error) {
	info, err := os.Lstat(filePath)
	if err != nil {
		return nil, fmt.Errorf("open file: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("refusing to read symlink %q", filePath)
	}

	file, err := openExistingNoFollow(filePath)
	if err != nil {
		// Re-check to keep the symlink rejection error deterministic in races.
		if latestInfo, statErr := os.Lstat(filePath); statErr == nil && latestInfo.Mode()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("refusing to read symlink %q", filePath)
		}
		return nil, fmt.Errorf("open file: %w", err)
	}
	return file, nil
}

func executeUploadOperation(ctx context.Context, file *os.File, task uploadTask, uploadOpts UploadOptions) error {
	method := strings.ToUpper(strings.TrimSpace(task.op.Method))
	if method == "" {
		method = http.MethodPut
	}

	_, err := WithRetry(ctx, func() (struct{}, error) {
		reader := io.NewSectionReader(file, task.op.Offset, task.op.Length)
		req, err := http.NewRequestWithContext(ctx, method, task.op.URL, reader)
		if err != nil {
			return struct{}{}, err
		}
		req.ContentLength = task.op.Length
		for _, header := range task.op.RequestHeaders {
			req.Header.Set(header.Name, header.Value)
		}

		resp, err := uploadOpts.Client.Do(req)
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return struct{}{}, err
			}
			return struct{}{}, &RetryableError{Err: fmt.Errorf("upload request failed: %w", err)}
		}
		defer resp.Body.Close()
		_, _ = io.Copy(io.Discard, resp.Body)

		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == http.StatusServiceUnavailable {
			retryAfter := parseRetryAfterHeader(resp.Header.Get("Retry-After"))
			return struct{}{}, &RetryableError{
				Err:        buildRetryableError(resp.StatusCode, retryAfter, nil),
				RetryAfter: retryAfter,
			}
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return struct{}{}, fmt.Errorf("upload request failed with status %s", resp.Status)
		}

		return struct{}{}, nil
	}, uploadOpts.RetryOpts)
	if err != nil {
		return fmt.Errorf("upload operation %d: %w", task.index, err)
	}
	return nil
}

// VerifySourceFileChecksums computes and compares checksums provided by the API.
func VerifySourceFileChecksums(filePath string, expected *Checksums) (*Checksums, error) {
	if expected == nil {
		return nil, nil
	}

	computed := &Checksums{}
	if expected.File != nil {
		expectedHash := strings.TrimSpace(expected.File.Hash)
		if expectedHash == "" {
			return nil, errors.New("file checksum hash is missing")
		}
		sum, err := ComputeFileChecksum(filePath, expected.File.Algorithm)
		if err != nil {
			return nil, err
		}
		if !strings.EqualFold(expectedHash, sum.Hash) {
			return nil, fmt.Errorf("file checksum mismatch (expected %s, got %s)", expectedHash, sum.Hash)
		}
		computed.File = sum
	}
	if expected.Composite != nil {
		expectedHash := strings.TrimSpace(expected.Composite.Hash)
		if expectedHash == "" {
			return nil, errors.New("composite checksum hash is missing")
		}
		sum, err := ComputeFileChecksum(filePath, expected.Composite.Algorithm)
		if err != nil {
			return nil, err
		}
		if !strings.EqualFold(expectedHash, sum.Hash) {
			return nil, fmt.Errorf("composite checksum mismatch (expected %s, got %s)", expectedHash, sum.Hash)
		}
		computed.Composite = sum
	}
	if computed.File == nil && computed.Composite == nil {
		return nil, errors.New("no checksum algorithms provided")
	}

	return computed, nil
}

// ComputeFileChecksum computes the checksum for a file using the provided algorithm.
func ComputeFileChecksum(filePath string, algorithm ChecksumAlgorithm) (*Checksum, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("open file for checksum: %w", err)
	}
	defer file.Close()

	var hash hash.Hash
	switch algorithm {
	case ChecksumAlgorithmMD5:
		hash = md5.New()
	case ChecksumAlgorithmSHA256:
		hash = sha256.New()
	default:
		return nil, fmt.Errorf("unsupported checksum algorithm: %s", algorithm)
	}

	if _, err := io.Copy(hash, file); err != nil {
		return nil, fmt.Errorf("compute checksum: %w", err)
	}

	return &Checksum{
		Hash:      hex.EncodeToString(hash.Sum(nil)),
		Algorithm: algorithm,
	}, nil
}
