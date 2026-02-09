package asc

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	// NotaryBaseURL is the Apple Notary API base URL.
	NotaryBaseURL = "https://appstoreconnect.apple.com"
	// notarySubmissionsPath is the submissions endpoint path.
	notarySubmissionsPath = "/notary/v2/submissions"
	// notaryS3Region is the AWS region for the notary S3 bucket.
	notaryS3Region = "us-west-2"
	// notaryS3MaxSingleUploadBytes is the max size for a single PutObject upload.
	notaryS3MaxSingleUploadBytes = 5 * 1024 * 1024 * 1024
	// notaryS3MinPartSizeBytes is the minimum part size for multipart uploads.
	notaryS3MinPartSizeBytes = 5 * 1024 * 1024
	// notaryS3DefaultPartSizeBytes is the default part size for multipart uploads.
	notaryS3DefaultPartSizeBytes = 16 * 1024 * 1024
	// notaryS3MaxParts is the maximum number of parts allowed in a multipart upload.
	notaryS3MaxParts = 10000
)

// NotarySubmissionStatus represents the status of a notarization submission.
type NotarySubmissionStatus string

const (
	NotaryStatusAccepted   NotarySubmissionStatus = "Accepted"
	NotaryStatusInProgress NotarySubmissionStatus = "In Progress"
	NotaryStatusInvalid    NotarySubmissionStatus = "Invalid"
	NotaryStatusRejected   NotarySubmissionStatus = "Rejected"
)

// NotarySubmissionRequest is the request body for submitting software for notarization.
type NotarySubmissionRequest struct {
	Sha256         string `json:"sha256"`
	SubmissionName string `json:"submissionName"`
}

// NotarySubmissionResponseAttributes contains S3 upload credentials returned by the submit endpoint.
type NotarySubmissionResponseAttributes struct {
	AwsAccessKeyID     string `json:"awsAccessKeyId"`
	AwsSecretAccessKey string `json:"awsSecretAccessKey"`
	AwsSessionToken    string `json:"awsSessionToken"`
	Bucket             string `json:"bucket"`
	Object             string `json:"object"`
}

// NotarySubmissionResponseData is the data portion of a submit response.
type NotarySubmissionResponseData struct {
	Type       string                             `json:"type"`
	ID         string                             `json:"id"`
	Attributes NotarySubmissionResponseAttributes `json:"attributes"`
}

// NotarySubmissionResponse is the response from the submit endpoint.
type NotarySubmissionResponse struct {
	Data NotarySubmissionResponseData `json:"data"`
}

// NotarySubmissionStatusAttributes describes a submission's current state.
type NotarySubmissionStatusAttributes struct {
	Status      NotarySubmissionStatus `json:"status"`
	Name        string                 `json:"name"`
	CreatedDate string                 `json:"createdDate"`
}

// NotarySubmissionStatusData is the data portion of a status response.
type NotarySubmissionStatusData struct {
	ID         string                           `json:"id"`
	Type       string                           `json:"type"`
	Attributes NotarySubmissionStatusAttributes `json:"attributes"`
}

// NotarySubmissionStatusResponse is the response from the status endpoint.
type NotarySubmissionStatusResponse struct {
	Data NotarySubmissionStatusData `json:"data"`
}

// NotarySubmissionsListResponse is the response from the list submissions endpoint.
type NotarySubmissionsListResponse struct {
	Data []NotarySubmissionStatusData `json:"data"`
}

// NotarySubmissionLogsAttributes contains the developer log URL.
type NotarySubmissionLogsAttributes struct {
	DeveloperLogURL string `json:"developerLogUrl"`
}

// NotarySubmissionLogsData is the data portion of a logs response.
type NotarySubmissionLogsData struct {
	ID         string                         `json:"id"`
	Type       string                         `json:"type"`
	Attributes NotarySubmissionLogsAttributes `json:"attributes"`
}

// NotarySubmissionLogsResponse is the response from the logs endpoint.
type NotarySubmissionLogsResponse struct {
	Data NotarySubmissionLogsData `json:"data"`
}

// S3Credentials holds the temporary AWS credentials for uploading to S3.
type S3Credentials struct {
	AccessKeyID     string
	SecretAccessKey string
	SessionToken    string
	Bucket          string
	Object          string
}

// GenerateNotaryJWT generates a JWT for the Notary API.
// It is identical to GenerateJWT but includes the "scope" claim required by the Notary API.
func GenerateNotaryJWT(keyID, issuerID string, privateKey interface{}) (string, error) {
	now := time.Now()
	claims := jwt.MapClaims{
		"iss":   issuerID,
		"aud":   "appstoreconnect-v1",
		"iat":   jwt.NewNumericDate(now),
		"exp":   jwt.NewNumericDate(now.Add(tokenLifetime)),
		"scope": []string{"/notary/v2"},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodES256, claims)
	token.Header["kid"] = keyID

	signedToken, err := token.SignedString(privateKey)
	if err != nil {
		return "", fmt.Errorf("failed to sign notary token: %w", err)
	}

	return signedToken, nil
}

// SetNotaryBaseURL overrides the Notary API base URL (for testing).
func (c *Client) SetNotaryBaseURL(url string) {
	c.notaryBaseURL = url
}

// resolveNotaryBaseURL returns the effective Notary base URL.
func (c *Client) resolveNotaryBaseURL() string {
	if c.notaryBaseURL != "" {
		return c.notaryBaseURL
	}
	return NotaryBaseURL
}

// newNotaryRequest creates a new HTTP request targeting the Notary API.
func (c *Client) newNotaryRequest(ctx context.Context, method, path string, body io.Reader) (*http.Request, error) {
	token, err := GenerateNotaryJWT(c.keyID, c.issuerID, c.privateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to generate notary JWT: %w", err)
	}

	url := path
	if !strings.HasPrefix(path, "http://") && !strings.HasPrefix(path, "https://") {
		url = c.resolveNotaryBaseURL() + path
	}
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	return req, nil
}

// doNotary performs an HTTP request against the Notary API.
func (c *Client) doNotary(ctx context.Context, method, path string, body io.Reader) ([]byte, error) {
	req, err := c.newNotaryRequest(ctx, method, path, body)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("notary request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		if err := ParseErrorWithStatus(respBody, resp.StatusCode); err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("notary API request failed with status %d", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}

// SubmitNotarization creates a new notarization submission.
func (c *Client) SubmitNotarization(ctx context.Context, sha256Hash, submissionName string) (*NotarySubmissionResponse, error) {
	sha256Hash = strings.TrimSpace(sha256Hash)
	submissionName = strings.TrimSpace(submissionName)
	if sha256Hash == "" {
		return nil, fmt.Errorf("sha256 hash is required")
	}
	if submissionName == "" {
		return nil, fmt.Errorf("submission name is required")
	}

	payload := NotarySubmissionRequest{
		Sha256:         sha256Hash,
		SubmissionName: submissionName,
	}

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	data, err := c.doNotary(ctx, "POST", notarySubmissionsPath, body)
	if err != nil {
		return nil, fmt.Errorf("submit notarization: %w", err)
	}

	var response NotarySubmissionResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse submission response: %w", err)
	}

	return &response, nil
}

// GetNotarizationStatus retrieves the status of a notarization submission.
func (c *Client) GetNotarizationStatus(ctx context.Context, submissionID string) (*NotarySubmissionStatusResponse, error) {
	submissionID = strings.TrimSpace(submissionID)
	if submissionID == "" {
		return nil, fmt.Errorf("submission ID is required")
	}

	path := fmt.Sprintf("%s/%s", notarySubmissionsPath, submissionID)
	data, err := c.doNotary(ctx, "GET", path, nil)
	if err != nil {
		return nil, fmt.Errorf("get notarization status: %w", err)
	}

	var response NotarySubmissionStatusResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse status response: %w", err)
	}

	return &response, nil
}

// GetNotarizationLogs retrieves the developer log URL for a notarization submission.
func (c *Client) GetNotarizationLogs(ctx context.Context, submissionID string) (*NotarySubmissionLogsResponse, error) {
	submissionID = strings.TrimSpace(submissionID)
	if submissionID == "" {
		return nil, fmt.Errorf("submission ID is required")
	}

	path := fmt.Sprintf("%s/%s/logs", notarySubmissionsPath, submissionID)
	data, err := c.doNotary(ctx, "GET", path, nil)
	if err != nil {
		return nil, fmt.Errorf("get notarization logs: %w", err)
	}

	var response NotarySubmissionLogsResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse logs response: %w", err)
	}

	return &response, nil
}

// ListNotarizations retrieves previous notarization submissions.
func (c *Client) ListNotarizations(ctx context.Context) (*NotarySubmissionsListResponse, error) {
	data, err := c.doNotary(ctx, "GET", notarySubmissionsPath, nil)
	if err != nil {
		return nil, fmt.Errorf("list notarizations: %w", err)
	}

	var response NotarySubmissionsListResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse list response: %w", err)
	}

	return &response, nil
}

// ComputeFileSHA256 computes the SHA-256 hex digest of a file.
func ComputeFileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open file: %w", err)
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", fmt.Errorf("read file: %w", err)
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

// UploadToS3 uploads file data to the S3 bucket using AWS Signature V4 authentication.
// This is a minimal implementation for the single PutObject operation needed by the Notary API.
func UploadToS3(ctx context.Context, creds S3Credentials, data io.Reader, payloadHash string, contentLength int64, contentType string) error {
	if creds.Bucket == "" || creds.Object == "" {
		return fmt.Errorf("S3 bucket and object are required")
	}
	payloadHash = strings.TrimSpace(payloadHash)
	if payloadHash == "" {
		return fmt.Errorf("payload hash is required")
	}
	if contentLength <= 0 {
		return fmt.Errorf("content length must be positive")
	}
	if strings.TrimSpace(contentType) == "" {
		contentType = "application/octet-stream"
	}

	if contentLength > notaryS3MaxSingleUploadBytes {
		return uploadMultipartToS3(ctx, creds, data, contentLength, contentType)
	}

	return uploadSinglePartToS3(ctx, creds, data, payloadHash, contentLength, contentType)
}

func uploadSinglePartToS3(ctx context.Context, creds S3Credentials, data io.Reader, payloadHash string, contentLength int64, contentType string) error {
	encodedPath, err := encodeS3ObjectPath(creds.Object)
	if err != nil {
		return fmt.Errorf("encode S3 object key: %w", err)
	}

	// Build the S3 URL
	host := fmt.Sprintf("%s.s3.%s.amazonaws.com", creds.Bucket, notaryS3Region)
	url := fmt.Sprintf("https://%s%s", host, encodedPath)

	now := time.Now().UTC()
	amzDate := now.Format("20060102T150405Z")

	req, err := http.NewRequestWithContext(ctx, "PUT", url, data)
	if err != nil {
		return fmt.Errorf("create S3 request: %w", err)
	}

	req.ContentLength = contentLength
	req.Header.Set("Host", host)
	req.Header.Set("X-Amz-Date", amzDate)
	req.Header.Set("X-Amz-Content-Sha256", payloadHash)
	req.Header.Set("Content-Type", contentType)
	if creds.SessionToken != "" {
		req.Header.Set("X-Amz-Security-Token", creds.SessionToken)
	}

	if err := signS3Request(req, creds, payloadHash, now); err != nil {
		return err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("S3 upload failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("S3 upload failed with status %d: %s", resp.StatusCode, sanitizeErrorBody(respBody))
	}

	return nil
}

func uploadMultipartToS3(ctx context.Context, creds S3Credentials, data io.Reader, contentLength int64, contentType string) error {
	encodedPath, err := encodeS3ObjectPath(creds.Object)
	if err != nil {
		return fmt.Errorf("encode S3 object key: %w", err)
	}

	host := fmt.Sprintf("%s.s3.%s.amazonaws.com", creds.Bucket, notaryS3Region)
	uploadID, err := createMultipartUpload(ctx, host, encodedPath, creds, contentType)
	if err != nil {
		return err
	}

	parts, err := uploadMultipartParts(ctx, host, encodedPath, creds, uploadID, data, contentLength)
	if err != nil {
		if abortErr := abortMultipartUpload(ctx, host, encodedPath, creds, uploadID); abortErr != nil {
			return fmt.Errorf("%w (abort failed: %v)", err, abortErr)
		}
		return err
	}

	if err := completeMultipartUpload(ctx, host, encodedPath, creds, uploadID, parts); err != nil {
		if abortErr := abortMultipartUpload(ctx, host, encodedPath, creds, uploadID); abortErr != nil {
			return fmt.Errorf("%w (abort failed: %v)", err, abortErr)
		}
		return err
	}

	return nil
}

type s3CompletedPart struct {
	PartNumber int
	ETag       string
}

func uploadMultipartParts(ctx context.Context, host, encodedPath string, creds S3Credentials, uploadID string, data io.Reader, contentLength int64) ([]s3CompletedPart, error) {
	partSize := calculateMultipartPartSize(contentLength)
	partCount := int((contentLength + partSize - 1) / partSize)
	if partCount > notaryS3MaxParts {
		return nil, fmt.Errorf("multipart upload exceeds maximum parts (%d)", notaryS3MaxParts)
	}

	parts := make([]s3CompletedPart, 0, partCount)
	buffer := make([]byte, int(partSize))
	var offset int64

	for partNumber := 1; offset < contentLength; partNumber++ {
		remaining := contentLength - offset
		partBytes := buffer
		if remaining < int64(len(buffer)) {
			partBytes = buffer[:remaining]
		}

		if _, err := io.ReadFull(data, partBytes); err != nil {
			return nil, fmt.Errorf("read upload part %d: %w", partNumber, err)
		}

		etag, err := uploadMultipartPart(ctx, host, encodedPath, creds, uploadID, partNumber, partBytes)
		if err != nil {
			return nil, err
		}

		parts = append(parts, s3CompletedPart{
			PartNumber: partNumber,
			ETag:       normalizeETag(etag),
		})
		offset += int64(len(partBytes))
	}

	return parts, nil
}

func createMultipartUpload(ctx context.Context, host, encodedPath string, creds S3Credentials, contentType string) (string, error) {
	query := url.Values{}
	query.Set("uploads", "")
	rawQuery := encodeS3Query(query)

	req, err := newS3Request(ctx, "POST", host, encodedPath, rawQuery, nil)
	if err != nil {
		return "", fmt.Errorf("create multipart request: %w", err)
	}

	payloadHash := sha256Hex(nil)
	now := time.Now().UTC()
	amzDate := now.Format("20060102T150405Z")
	req.ContentLength = 0
	req.Header.Set("Host", host)
	req.Header.Set("X-Amz-Date", amzDate)
	req.Header.Set("X-Amz-Content-Sha256", payloadHash)
	if strings.TrimSpace(contentType) != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if creds.SessionToken != "" {
		req.Header.Set("X-Amz-Security-Token", creds.SessionToken)
	}
	if err := signS3Request(req, creds, payloadHash, now); err != nil {
		return "", err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("create multipart upload failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("create multipart upload failed with status %d: %s", resp.StatusCode, sanitizeErrorBody(respBody))
	}

	type createMultipartUploadResult struct {
		UploadID string `xml:"UploadId"`
	}
	var result createMultipartUploadResult
	if err := xml.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("parse multipart upload response: %w", err)
	}
	if strings.TrimSpace(result.UploadID) == "" {
		return "", fmt.Errorf("multipart upload response missing upload ID")
	}
	return result.UploadID, nil
}

func uploadMultipartPart(ctx context.Context, host, encodedPath string, creds S3Credentials, uploadID string, partNumber int, partBytes []byte) (string, error) {
	query := url.Values{}
	query.Set("partNumber", fmt.Sprintf("%d", partNumber))
	query.Set("uploadId", uploadID)
	rawQuery := encodeS3Query(query)

	req, err := newS3Request(ctx, "PUT", host, encodedPath, rawQuery, bytes.NewReader(partBytes))
	if err != nil {
		return "", fmt.Errorf("create multipart part request: %w", err)
	}

	payloadHash := sha256Hex(partBytes)
	now := time.Now().UTC()
	amzDate := now.Format("20060102T150405Z")
	req.ContentLength = int64(len(partBytes))
	req.Header.Set("Host", host)
	req.Header.Set("X-Amz-Date", amzDate)
	req.Header.Set("X-Amz-Content-Sha256", payloadHash)
	if creds.SessionToken != "" {
		req.Header.Set("X-Amz-Security-Token", creds.SessionToken)
	}
	if err := signS3Request(req, creds, payloadHash, now); err != nil {
		return "", err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("upload part %d failed: %w", partNumber, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("upload part %d failed with status %d: %s", partNumber, resp.StatusCode, sanitizeErrorBody(respBody))
	}

	etag := resp.Header.Get("ETag")
	if strings.TrimSpace(etag) == "" {
		return "", fmt.Errorf("upload part %d response missing ETag", partNumber)
	}

	return etag, nil
}

func completeMultipartUpload(ctx context.Context, host, encodedPath string, creds S3Credentials, uploadID string, parts []s3CompletedPart) error {
	query := url.Values{}
	query.Set("uploadId", uploadID)
	rawQuery := encodeS3Query(query)

	type completeMultipartUploadPart struct {
		PartNumber int    `xml:"PartNumber"`
		ETag       string `xml:"ETag"`
	}
	type completeMultipartUploadRequest struct {
		XMLName xml.Name                      `xml:"CompleteMultipartUpload"`
		Parts   []completeMultipartUploadPart `xml:"Part"`
	}
	payload := completeMultipartUploadRequest{Parts: make([]completeMultipartUploadPart, 0, len(parts))}
	for _, part := range parts {
		payload.Parts = append(payload.Parts, completeMultipartUploadPart(part))
	}
	bodyBytes, err := xml.Marshal(payload)
	if err != nil {
		return fmt.Errorf("build complete multipart payload: %w", err)
	}

	req, err := newS3Request(ctx, "POST", host, encodedPath, rawQuery, bytes.NewReader(bodyBytes))
	if err != nil {
		return fmt.Errorf("create complete multipart request: %w", err)
	}

	payloadHash := sha256Hex(bodyBytes)
	now := time.Now().UTC()
	amzDate := now.Format("20060102T150405Z")
	req.ContentLength = int64(len(bodyBytes))
	req.Header.Set("Host", host)
	req.Header.Set("X-Amz-Date", amzDate)
	req.Header.Set("X-Amz-Content-Sha256", payloadHash)
	req.Header.Set("Content-Type", "application/xml")
	if creds.SessionToken != "" {
		req.Header.Set("X-Amz-Security-Token", creds.SessionToken)
	}
	if err := signS3Request(req, creds, payloadHash, now); err != nil {
		return err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("complete multipart upload failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("complete multipart upload failed with status %d: %s", resp.StatusCode, sanitizeErrorBody(respBody))
	}
	return nil
}

func abortMultipartUpload(ctx context.Context, host, encodedPath string, creds S3Credentials, uploadID string) error {
	query := url.Values{}
	query.Set("uploadId", uploadID)
	rawQuery := encodeS3Query(query)

	req, err := newS3Request(ctx, "DELETE", host, encodedPath, rawQuery, nil)
	if err != nil {
		return fmt.Errorf("create abort request: %w", err)
	}

	payloadHash := sha256Hex(nil)
	now := time.Now().UTC()
	amzDate := now.Format("20060102T150405Z")
	req.ContentLength = 0
	req.Header.Set("Host", host)
	req.Header.Set("X-Amz-Date", amzDate)
	req.Header.Set("X-Amz-Content-Sha256", payloadHash)
	if creds.SessionToken != "" {
		req.Header.Set("X-Amz-Security-Token", creds.SessionToken)
	}
	if err := signS3Request(req, creds, payloadHash, now); err != nil {
		return err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("abort multipart upload failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("abort multipart upload failed with status %d: %s", resp.StatusCode, sanitizeErrorBody(respBody))
	}
	return nil
}

// sha256Hex computes the SHA-256 hex digest of data.
func sha256Hex(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

// hmacSHA256 computes HMAC-SHA256.
func hmacSHA256(key, data []byte) []byte {
	h := hmac.New(sha256.New, key)
	h.Write(data)
	return h.Sum(nil)
}

// deriveSigningKey derives the AWS Signature V4 signing key.
func deriveSigningKey(secretKey, dateStamp, region, service string) []byte {
	kDate := hmacSHA256([]byte("AWS4"+secretKey), []byte(dateStamp))
	kRegion := hmacSHA256(kDate, []byte(region))
	kService := hmacSHA256(kRegion, []byte(service))
	kSigning := hmacSHA256(kService, []byte("aws4_request"))
	return kSigning
}

func encodeS3ObjectPath(object string) (string, error) {
	object = strings.TrimPrefix(strings.TrimSpace(object), "/")
	if object == "" {
		return "", fmt.Errorf("object key is required")
	}

	segments := strings.Split(object, "/")
	for i, segment := range segments {
		segments[i] = escapePathSegment(segment)
	}

	return "/" + strings.Join(segments, "/"), nil
}

func escapePathSegment(value string) string {
	var builder strings.Builder
	for i := 0; i < len(value); i++ {
		b := value[i]
		if (b >= 'A' && b <= 'Z') ||
			(b >= 'a' && b <= 'z') ||
			(b >= '0' && b <= '9') ||
			b == '-' || b == '_' || b == '.' || b == '~' {
			builder.WriteByte(b)
			continue
		}
		builder.WriteString(fmt.Sprintf("%%%02X", b))
	}
	return builder.String()
}

func encodeS3Query(values url.Values) string {
	if len(values) == 0 {
		return ""
	}

	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	parts := make([]string, 0)
	for _, key := range keys {
		vals := values[key]
		sort.Strings(vals)
		for _, val := range vals {
			parts = append(parts, encodeQueryComponent(key)+"="+encodeQueryComponent(val))
		}
	}

	return strings.Join(parts, "&")
}

func encodeQueryComponent(value string) string {
	escaped := url.QueryEscape(value)
	escaped = strings.ReplaceAll(escaped, "+", "%20")
	escaped = strings.ReplaceAll(escaped, "%7E", "~")
	return escaped
}

func canonicalQueryString(values url.Values) string {
	return encodeS3Query(values)
}

func canonicalizeHeaders(headers map[string]string) (string, string) {
	keys := make([]string, 0, len(headers))
	for key := range headers {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	var canonicalBuilder strings.Builder
	for _, key := range keys {
		value := strings.TrimSpace(headers[key])
		canonicalBuilder.WriteString(key)
		canonicalBuilder.WriteString(":")
		canonicalBuilder.WriteString(value)
		canonicalBuilder.WriteString("\n")
	}

	signedHeaders := strings.Join(keys, ";")
	return canonicalBuilder.String(), signedHeaders
}

func signS3Request(req *http.Request, creds S3Credentials, payloadHash string, now time.Time) error {
	amzDate := now.Format("20060102T150405Z")
	dateStamp := now.Format("20060102")

	headers := map[string]string{
		"host":                 req.Host,
		"x-amz-content-sha256": payloadHash,
		"x-amz-date":           amzDate,
	}
	if contentType := req.Header.Get("Content-Type"); contentType != "" {
		headers["content-type"] = contentType
	}
	if securityToken := req.Header.Get("X-Amz-Security-Token"); securityToken != "" {
		headers["x-amz-security-token"] = securityToken
	}

	canonicalHeaders, signedHeaders := canonicalizeHeaders(headers)
	canonicalRequest := strings.Join([]string{
		req.Method,
		req.URL.EscapedPath(),
		canonicalQueryString(req.URL.Query()),
		canonicalHeaders,
		signedHeaders,
		payloadHash,
	}, "\n")

	credentialScope := fmt.Sprintf("%s/%s/s3/aws4_request", dateStamp, notaryS3Region)
	stringToSign := strings.Join([]string{
		"AWS4-HMAC-SHA256",
		amzDate,
		credentialScope,
		sha256Hex([]byte(canonicalRequest)),
	}, "\n")

	signingKey := deriveSigningKey(creds.SecretAccessKey, dateStamp, notaryS3Region, "s3")
	signature := hex.EncodeToString(hmacSHA256(signingKey, []byte(stringToSign)))

	authHeader := fmt.Sprintf("AWS4-HMAC-SHA256 Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		creds.AccessKeyID, credentialScope, signedHeaders, signature)
	req.Header.Set("Authorization", authHeader)

	return nil
}

func calculateMultipartPartSize(contentLength int64) int64 {
	partSize := int64(notaryS3DefaultPartSizeBytes)
	if contentLength <= partSize*notaryS3MaxParts {
		return partSize
	}

	partSize = (contentLength + notaryS3MaxParts - 1) / notaryS3MaxParts
	if partSize < notaryS3MinPartSizeBytes {
		partSize = notaryS3MinPartSizeBytes
	}
	return partSize
}

func normalizeETag(etag string) string {
	etag = strings.TrimSpace(etag)
	if etag == "" {
		return etag
	}
	if strings.HasPrefix(etag, "\"") && strings.HasSuffix(etag, "\"") {
		return etag
	}
	return `"` + etag + `"`
}

func newS3Request(ctx context.Context, method, host, encodedPath, rawQuery string, body io.Reader) (*http.Request, error) {
	url := fmt.Sprintf("https://%s%s", host, encodedPath)
	if rawQuery != "" {
		url = url + "?" + rawQuery
	}
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, err
	}
	req.Host = host
	return req, nil
}
