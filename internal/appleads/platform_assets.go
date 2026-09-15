package appleads

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/readonly"
)

const platformAssetPromotedObjectType = "BUSINESS_BRAND"

// UploadPlatformAsset uploads one already-open image to the Platform API v1.
// The caller owns file and must keep it open until this method returns.
func (c *Client) UploadPlatformAsset(ctx context.Context, file *os.File, fileSize int64, fileName, contentType, brandID string) (RawResponse, error) {
	if file == nil {
		return nil, fmt.Errorf("asset file is required")
	}
	if fileSize <= 0 {
		return nil, fmt.Errorf("asset file must not be empty")
	}
	fileName = filepath.Base(strings.TrimSpace(fileName))
	if fileName == "" || fileName == "." {
		return nil, fmt.Errorf("asset filename is required")
	}
	contentType = strings.TrimSpace(contentType)
	if contentType == "" {
		return nil, fmt.Errorf("asset content type is required")
	}
	brandID = strings.TrimSpace(brandID)
	if brandID == "" {
		return nil, fmt.Errorf("brand ID is required")
	}

	prefix, suffix, multipartContentType, err := platformAssetMultipartFraming(fileName, contentType, brandID)
	if err != nil {
		return nil, err
	}
	contentLength := int64(len(prefix)) + fileSize + int64(len(suffix))
	if contentLength < fileSize {
		return nil, fmt.Errorf("asset multipart content length overflow")
	}

	contextHeader, err := c.contextHeader(ContextAdAccount)
	if err != nil {
		return nil, err
	}
	requestURL, err := c.requestURLForVersion(APIVersionPlatformV1, "v1/assets/upload", nil)
	if err != nil {
		return nil, err
	}
	if err := readonly.Check(ctx, http.MethodPost, readonly.Target(requestURL)); err != nil {
		return nil, err
	}
	token, err := c.bearerToken(ctx)
	if err != nil {
		return nil, err
	}
	body := io.MultiReader(bytes.NewReader(prefix), io.LimitReader(file, fileSize), bytes.NewReader(suffix))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, requestURL, body)
	if err != nil {
		return nil, err
	}
	req.ContentLength = contentLength
	req.Header.Set("Content-Length", strconv.FormatInt(contentLength, 10))
	req.Header.Set("Content-Type", multipartContentType)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-AP-Context", contextHeader)

	httpClient := *c.httpClient
	httpClient.Timeout = asc.ResolveUploadTimeout()
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	c.rateLimitMu.Lock()
	c.lastRateLimit = rateLimitFromHeaders(resp.Header)
	c.rateLimitMu.Unlock()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, parseErrorForVersion(responseBody, resp.StatusCode, resp.Header, APIVersionPlatformV1)
	}
	if len(strings.TrimSpace(string(responseBody))) == 0 {
		return RawResponse(`{}`), nil
	}
	return RawResponse(responseBody), nil
}

func platformAssetMultipartFraming(fileName, contentType, brandID string) ([]byte, []byte, string, error) {
	var framing bytes.Buffer
	writer := multipart.NewWriter(&framing)
	disposition := mime.FormatMediaType("form-data", map[string]string{
		"name":     "file",
		"filename": filepath.Base(fileName),
	})
	header := make(textproto.MIMEHeader)
	header.Set("Content-Disposition", disposition)
	header.Set("Content-Type", contentType)
	if _, err := writer.CreatePart(header); err != nil {
		return nil, nil, "", fmt.Errorf("create asset multipart file part: %w", err)
	}
	prefix := append([]byte(nil), framing.Bytes()...)
	framing.Reset()
	if err := writer.WriteField("promotedObjectId", brandID); err != nil {
		return nil, nil, "", fmt.Errorf("create asset multipart brand part: %w", err)
	}
	if err := writer.WriteField("promotedObjectType", platformAssetPromotedObjectType); err != nil {
		return nil, nil, "", fmt.Errorf("create asset multipart type part: %w", err)
	}
	if err := writer.Close(); err != nil {
		return nil, nil, "", fmt.Errorf("close asset multipart body: %w", err)
	}
	suffix := append([]byte(nil), framing.Bytes()...)
	return prefix, suffix, writer.FormDataContentType(), nil
}
