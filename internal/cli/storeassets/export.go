// Package storeassets shares the App Clip and preview folders used by metadata and migrate.
package storeassets

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/assets"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/rootfs"
)

// ExportFile is a file fetched from the selected version. URLs stay private to execution.
type ExportFile struct {
	Path string
	Text *string
	URL  string
}

// ExportPlan obtains all remote assets before touching the selected output root.
func ExportPlan(ctx context.Context, client *asc.Client, versionID, metadataPrefix string, clip, previews bool) ([]ExportFile, []string, error) {
	var files []ExportFile
	var warnings []string
	if clip {
		exp, err := request(ctx, func(c context.Context) (*asc.AppClipDefaultExperienceResponse, error) {
			return client.GetAppStoreVersionAppClipDefaultExperience(c, versionID)
		})
		if err != nil && !asc.IsNotFound(err) {
			return nil, nil, err
		}
		if exp == nil || exp.Data.ID == "" {
			warnings = append(warnings, "version has no App Clip default experience")
		} else {
			action := string(exp.Data.Attributes.Action) + "\n"
			if strings.TrimSpace(action) != "" {
				files = append(files, ExportFile{Path: filepath.Join(metadataPrefix, "app_clip", "action.txt"), Text: &action})
			}
			locs, err := clipLocalizations(ctx, client, exp.Data.ID)
			if err != nil {
				return nil, nil, err
			}
			for _, loc := range locs {
				if err := segment(loc.Attributes.Locale); err != nil {
					return nil, nil, err
				}
				base := filepath.Join(metadataPrefix, loc.Attributes.Locale, "app_clip")
				subtitle := loc.Attributes.Subtitle + "\n"
				files = append(files, ExportFile{Path: filepath.Join(base, "subtitle.txt"), Text: &subtitle})
				header, err := request(ctx, func(c context.Context) (*asc.AppClipHeaderImageResponse, error) {
					return client.GetAppClipDefaultExperienceLocalizationHeaderImage(c, loc.ID)
				})
				if err != nil && !asc.IsNotFound(err) {
					return nil, nil, err
				}
				if header != nil && header.Data.ID != "" {
					// Request PNG delivery to match the canonical filename.
					url, err := assets.ResolveImageAssetDownloadURL(header.Data.Attributes.ImageAsset, "header_image.png")
					if err != nil {
						return nil, nil, err
					}
					files = append(files, ExportFile{Path: filepath.Join(base, "header_image.png"), URL: url})
				}
			}
		}
	}
	if previews {
		locs, err := versionLocalizations(ctx, client, versionID)
		if err != nil {
			return nil, nil, err
		}
		for _, loc := range locs {
			if err := segment(loc.Attributes.Locale); err != nil {
				return nil, nil, err
			}
			sets, err := previewSets(ctx, client, loc.ID)
			if err != nil {
				return nil, nil, err
			}
			for _, set := range sets {
				device, err := assets.NormalizePreviewType(set.Attributes.PreviewType)
				if err != nil {
					return nil, nil, err
				}
				items, err := previewItems(ctx, client, set.ID)
				if err != nil {
					return nil, nil, err
				}
				ids, err := previewOrder(ctx, client, set.ID)
				if err != nil {
					return nil, nil, err
				}
				byID := map[string]asc.Resource[asc.AppPreviewAttributes]{}
				for _, item := range items {
					byID[item.ID] = item
				}
				ordered := make([]asc.Resource[asc.AppPreviewAttributes], 0, len(items))
				for _, id := range ids {
					item, ok := byID[id]
					if !ok {
						return nil, nil, fmt.Errorf("preview order references missing asset %s", id)
					}
					ordered = append(ordered, item)
				}
				if len(ordered) != len(items) {
					return nil, nil, fmt.Errorf("preview set %s changed during export; retry", set.ID)
				}
				items = ordered
				order := make([]string, 0, len(items))
				for _, item := range items {
					name := item.Attributes.FileName
					if err := segment(name); err != nil {
						return nil, nil, err
					}
					ext := strings.ToLower(filepath.Ext(name))
					if ext != ".mp4" && ext != ".mov" && ext != ".m4v" {
						return nil, nil, fmt.Errorf("preview %s has unsupported filename", item.ID)
					}
					order = append(order, name)
					url := strings.TrimSpace(item.Attributes.VideoURL)
					if url == "" {
						detail, err := request(ctx, func(c context.Context) (*asc.AppPreviewResponse, error) { return client.GetAppPreview(c, item.ID) })
						if err != nil {
							return nil, nil, err
						}
						url = strings.TrimSpace(detail.Data.Attributes.VideoURL)
						if url == "" {
							return nil, nil, fmt.Errorf("preview %s has no downloadable videoUrl", item.ID)
						}
					}
					path := filepath.Join("app_previews", loc.Attributes.Locale, strings.ToLower(device), name)
					files = append(files, ExportFile{Path: path, URL: url})
					if item.Attributes.PreviewFrameTimeCode != "" {
						frame := item.Attributes.PreviewFrameTimeCode + "\n"
						files = append(files, ExportFile{Path: strings.TrimSuffix(path, filepath.Ext(path)) + ".poster_frame.txt", Text: &frame})
					}
				}
				if len(order) > 0 {
					data, _ := json.Marshal(order)
					value := string(data) + "\n"
					files = append(files, ExportFile{Path: filepath.Join("app_previews", loc.Attributes.Locale, strings.ToLower(device), "order.json"), Text: &value})
				}
			}
		}
	}
	seen := map[string]bool{}
	for _, f := range files {
		if seen[f.Path] {
			return nil, nil, fmt.Errorf("duplicate asset export path %s", f.Path)
		}
		seen[f.Path] = true
	}
	return files, warnings, nil
}

// WriteExport honors the same no-overwrite preflight for text and media.
func WriteExport(ctx context.Context, root rootfs.Root, files []ExportFile, overwrite bool) ([]string, error) {
	if err := CheckExportTargets(root, files, overwrite); err != nil {
		return nil, err
	}
	var written []string
	for _, f := range files {
		if err := root.MkdirAll(filepath.Dir(f.Path), 0o755); err != nil {
			return written, err
		}
		var err error
		if f.Text != nil {
			if overwrite {
				err = root.WriteFile(f.Path, []byte(*f.Text), 0o644)
			} else {
				err = root.CreateNewFile(f.Path, []byte(*f.Text), 0o644)
			}
		} else {
			err = downloadIntoRoot(ctx, root, f.Path, f.URL, overwrite)
		}
		if err != nil {
			return written, fmt.Errorf("export %s: %w", f.Path, err)
		}
		written = append(written, f.Path)
	}
	return written, nil
}

func downloadIntoRoot(ctx context.Context, root rootfs.Root, path, url string, overwrite bool) error {
	dir, err := os.MkdirTemp("", "asc-store-asset-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	temp := filepath.Join(dir, "asset")
	c, cancel := shared.ContextWithUploadTimeout(shared.ContextWithoutTimeout(ctx))
	defer cancel()
	if _, _, err = assets.DownloadMediaURL(c, url, temp, false); err != nil {
		return errors.New("media download failed (URL omitted)")
	}
	file, err := os.Open(temp)
	if err != nil {
		return err
	}
	defer file.Close()
	if overwrite {
		_, err = root.WriteFrom(path, file, 0o644)
	} else {
		_, err = root.CreateNewFrom(path, file, 0o644)
	}
	return err
}

func segment(s string) error {
	if strings.ContainsAny(s, "\r\n\x00") || strings.TrimSpace(s) == "" || s == "." || s == ".." || strings.ContainsAny(s, "/\\") {
		return fmt.Errorf("invalid asset path component %q", s)
	}
	return rootfs.ValidateRelative(s)
}

func request[T any](ctx context.Context, fn func(context.Context) (T, error)) (T, error) {
	c, cancel := shared.ContextWithTimeout(shared.ContextWithoutTimeout(ctx))
	defer cancel()
	return fn(c)
}

func pages[T any](ctx context.Context, fetch func(context.Context, string) (*asc.Response[T], error)) ([]asc.Resource[T], error) {
	first, err := request(ctx, func(c context.Context) (*asc.Response[T], error) { return fetch(c, "") })
	if err != nil {
		return nil, err
	}
	all, err := asc.PaginateAll(ctx, first, func(c context.Context, next string) (asc.PaginatedResponse, error) {
		return request(c, func(r context.Context) (*asc.Response[T], error) { return fetch(r, next) })
	})
	if err != nil {
		return nil, err
	}
	response, ok := all.(*asc.Response[T])
	if !ok {
		return nil, fmt.Errorf("unexpected asset pagination response")
	}
	return response.Data, nil
}

func clipLocalizations(ctx context.Context, c *asc.Client, id string) ([]asc.Resource[asc.AppClipDefaultExperienceLocalizationAttributes], error) {
	return pages(ctx, func(ctx context.Context, next string) (*asc.AppClipDefaultExperienceLocalizationsResponse, error) {
		return c.GetAppClipDefaultExperienceLocalizations(ctx, id, asc.WithAppClipDefaultExperienceLocalizationsNextURL(next))
	})
}

func versionLocalizations(ctx context.Context, c *asc.Client, id string) ([]asc.Resource[asc.AppStoreVersionLocalizationAttributes], error) {
	return pages(ctx, func(ctx context.Context, next string) (*asc.AppStoreVersionLocalizationsResponse, error) {
		return c.GetAppStoreVersionLocalizations(ctx, id, asc.WithAppStoreVersionLocalizationsNextURL(next))
	})
}

func previewSets(ctx context.Context, c *asc.Client, id string) ([]asc.Resource[asc.AppPreviewSetAttributes], error) {
	return pages(ctx, func(ctx context.Context, next string) (*asc.AppPreviewSetsResponse, error) {
		return c.GetAppStoreVersionLocalizationPreviewSets(ctx, id, asc.WithAppStoreVersionLocalizationPreviewSetsNextURL(next))
	})
}

func previewItems(ctx context.Context, c *asc.Client, id string) ([]asc.Resource[asc.AppPreviewAttributes], error) {
	return pages(ctx, func(ctx context.Context, next string) (*asc.AppPreviewsResponse, error) {
		return c.GetAppPreviews(ctx, id, asc.WithAppPreviewsNextURL(next))
	})
}

// CheckExportTargets validates every destination before an export writes files.
func CheckExportTargets(root rootfs.Root, files []ExportFile, overwrite bool) error {
	for _, f := range files {
		if err := root.CheckParents(f.Path); err != nil {
			return err
		}
		if !overwrite {
			if err := root.CheckCreateNewFile(f.Path); err != nil {
				return err
			}
		}
	}
	return nil
}
