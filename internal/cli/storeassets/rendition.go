package storeassets

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/rootfs"
)

const maxPreviewBytes int64 = 500_000_000

// Resolve only against previews fetched from this version's locale/device set.
// Delivered renditions need a content comparison because their source checksum
// describes the original upload, not necessarily the bytes returned by Apple.
func resolvePreviewMatch(ctx context.Context, client *asc.Client, preview *PreviewLayout, existing []asc.Resource[asc.AppPreviewAttributes]) error {
	for _, item := range existing {
		if strings.EqualFold(item.Attributes.SourceFileChecksum, preview.checksum) {
			preview.existingID, preview.sourceChecksum = item.ID, item.Attributes.SourceFileChecksum
			return nil
		}
	}
	for _, item := range existing {
		if item.Attributes.FileName != preview.FileName {
			continue
		}
		mediaURL := strings.TrimSpace(item.Attributes.VideoURL)
		if mediaURL == "" {
			detail, err := request(ctx, func(ctx context.Context) (*asc.AppPreviewResponse, error) {
				return client.GetAppPreview(ctx, item.ID)
			})
			if err != nil {
				return err
			}
			mediaURL = strings.TrimSpace(detail.Data.Attributes.VideoURL)
		}
		match, err := matchesDeliveredMedia(ctx, preview.stagedPath, mediaURL, maxPreviewBytes)
		if err != nil {
			return fmt.Errorf("verify delivered preview %s: %w", item.ID, err)
		}
		if match {
			if preview.existingID != "" {
				return fmt.Errorf("multiple delivered previews match %s; resolve duplicate remote assets before importing", preview.FileName)
			}
			preview.existingID, preview.sourceChecksum = item.ID, item.Attributes.SourceFileChecksum
		}
	}
	return nil
}

func (p *ImportPlan) checkPreviewMatches(ctx context.Context, client *asc.Client) error {
	checked := map[string][]asc.Resource[asc.AppPreviewAttributes]{}
	for _, preview := range p.Previews {
		if preview.existingID == "" {
			continue
		}
		group := p.previewGroups[preview.Locale+"/"+strings.ToUpper(preview.DeviceType)]
		items, ok := checked[group.SetID]
		if !ok {
			var err error
			items, err = previewItems(ctx, client, group.SetID)
			if err != nil {
				return err
			}
			checked[group.SetID] = items
		}
		found := false
		for _, item := range items {
			if item.ID == preview.existingID && strings.EqualFold(item.Attributes.SourceFileChecksum, preview.sourceChecksum) {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("preview %s changed or left set %s since preflight; retry", preview.existingID, group.SetID)
		}
	}
	return nil
}

// Stream at most the staged size plus one sentinel byte, never materializing a
// remote file. Both digests and lengths must agree; local edits remain uploads.
func matchesDeliveredMedia(ctx context.Context, stagedPath, mediaURL string, maxBytes int64) (bool, error) {
	root, err := rootfs.New(filepath.Dir(stagedPath))
	if err != nil {
		return false, err
	}
	defer root.Close()
	file, err := root.OpenFile(filepath.Base(stagedPath))
	if err != nil {
		return false, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return false, err
	}
	if !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > maxBytes {
		return false, fmt.Errorf("media comparison requires a nonempty regular file of at most %d bytes", maxBytes)
	}
	parsed, err := url.Parse(mediaURL)
	if err != nil || !validPreviewMediaURL(parsed) {
		return false, fmt.Errorf("asset has no valid HTTP media URL")
	}
	bounded, cancel := shared.ContextWithUploadTimeout(shared.ContextWithoutTimeout(ctx))
	defer cancel()
	req, err := http.NewRequestWithContext(bounded, http.MethodGet, mediaURL, nil)
	if err != nil {
		return false, fmt.Errorf("cannot request delivered media (URL omitted)")
	}
	req.Header.Set("Accept", "*/*")
	client := *http.DefaultClient
	client.Jar = nil
	previousRedirect := client.CheckRedirect
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if !validPreviewMediaURL(req.URL) || len(via) >= 10 {
			return fmt.Errorf("invalid media redirect")
		}
		if previousRedirect != nil {
			return previousRedirect(req, via)
		}
		return nil
	}
	resp, err := client.Do(req)
	if err != nil {
		return false, fmt.Errorf("delivered media request failed (URL omitted)")
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return false, fmt.Errorf("delivered media request returned HTTP %d", resp.StatusCode)
	}
	if resp.ContentLength > maxBytes {
		return false, fmt.Errorf("delivered media exceeds %d bytes", maxBytes)
	}
	remoteHash := sha256.New()
	n, err := io.Copy(remoteHash, io.LimitReader(resp.Body, info.Size()+1))
	if err != nil {
		return false, fmt.Errorf("delivered media read failed (URL omitted)")
	}
	if n != info.Size() {
		return false, nil
	}
	localHash := sha256.New()
	localN, err := io.Copy(localHash, io.LimitReader(file, info.Size()+1))
	if err != nil {
		return false, err
	}
	return localN == n && bytes.Equal(remoteHash.Sum(nil), localHash.Sum(nil)), nil
}

func validPreviewMediaURL(value *url.URL) bool {
	return value != nil && (value.Scheme == "https" || value.Scheme == "http") && value.Host != "" && value.User == nil
}
