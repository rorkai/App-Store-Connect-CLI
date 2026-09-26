package storeassets

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/assets"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/rootfs"
)

const (
	exportInventoryPath  = ".asc/store-assets-export.json"
	exportInventoryLimit = 1 << 20
)

// ExportOptions binds cleanup to the exact selected target and layout.
type ExportOptions struct {
	AppID, VersionID, MetadataPrefix string
	Clip, Previews, Overwrite        bool
}

type exportFingerprint struct {
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}

type exportOwnership struct {
	Path           string             `json:"path"`
	AppID          string             `json:"appId"`
	VersionID      string             `json:"versionId"`
	MetadataPrefix string             `json:"metadataPrefix"`
	Scope          string             `json:"scope"`
	Current        *exportFingerprint `json:"current,omitempty"`
	Pending        *exportFingerprint `json:"pending,omitempty"`
}

type exportInventory struct {
	Schema int               `json:"schema"`
	Files  []exportOwnership `json:"files"`
}

// ManagedExport retains the inventory identity captured before any output writes.
type ManagedExport struct {
	root          rootfs.Root
	files         []ExportFile
	options       ExportOptions
	inventory     exportInventory
	identity      *rootfs.FileIdentity
	legacyWindows bool
}

// PrepareExport rejects malformed ownership and target conflicts before callers
// write ordinary metadata. Unselected scopes retain their existing ownership.
func PrepareExport(root rootfs.Root, files []ExportFile, options ExportOptions) (*ManagedExport, error) {
	files = append([]ExportFile(nil), files...)
	for i := range files {
		files[i].Path = filepath.ToSlash(files[i].Path)
	}
	p := &ManagedExport{root: root, files: files, options: options, inventory: exportInventory{Schema: 1}}
	if !options.Clip && !options.Previews {
		return p, nil
	}
	if options.AppID == "" || options.VersionID == "" || (options.MetadataPrefix != "." && options.MetadataPrefix != "metadata") {
		return nil, fmt.Errorf("invalid asset export target")
	}
	// CheckCreateNewFile accepts a genuinely missing selected root without
	// creating it, while still rejecting symlinks and invalid parents.
	var identity *rootfs.FileIdentity
	err := root.CheckCreateNewFile(exportInventoryPath)
	if errors.Is(err, os.ErrExist) {
		identity, err = root.CaptureFileLimited(exportInventoryPath, exportInventoryLimit)
	}
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("read asset export inventory: %w", err)
	}
	if identity != nil {
		p.identity = identity
		p.inventory = exportInventory{}
		dec := json.NewDecoder(bytes.NewReader(identity.Data()))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&p.inventory); err != nil {
			return nil, fmt.Errorf("invalid asset export inventory: %w", err)
		}
		if err := dec.Decode(new(any)); err != io.EOF {
			return nil, fmt.Errorf("invalid trailing asset export inventory data")
		}
		if err := p.validateInventory(); err != nil {
			return nil, err
		}
	}
	for _, f := range files {
		scope, err := exportPathScope(f.Path, options.MetadataPrefix)
		if err != nil || !p.selected(scope) {
			return nil, fmt.Errorf("invalid selected asset export path %q", f.Path)
		}
	}
	if err := CheckExportTargets(root, files, options.Overwrite); err != nil {
		return nil, err
	}
	if runtime.GOOS == "windows" {
		if identity != nil {
			return nil, fmt.Errorf("asset export inventory updates and cleanup are unavailable on Windows; use a separate destination: %w", rootfs.ErrFileIdentityMutationUnsupported)
		}
		p.legacyWindows = true
	}
	return p, nil
}

func (p *ManagedExport) selected(scope string) bool {
	return scope == "app-clip" && p.options.Clip || scope == "previews" && p.options.Previews
}

func (p *ManagedExport) validateInventory() error {
	if p.inventory.Schema != 1 {
		return fmt.Errorf("unsupported asset export inventory schema")
	}
	seen := map[string]bool{}
	for _, item := range p.inventory.Files {
		scope, err := exportPathScope(item.Path, item.MetadataPrefix)
		if err != nil || scope != item.Scope || item.AppID == "" || item.VersionID == "" || seen[item.Path] || (item.Current == nil && item.Pending == nil) {
			return fmt.Errorf("invalid asset export inventory entry %q", item.Path)
		}
		seen[item.Path] = true
		for _, fp := range []*exportFingerprint{item.Current, item.Pending} {
			if fp == nil {
				continue
			}
			digest, err := hex.DecodeString(fp.SHA256)
			if err != nil || len(digest) != sha256.Size || fp.Size < 0 || fp.Size > exportPathLimit(item.Path) {
				return fmt.Errorf("invalid asset export inventory fingerprint for %q", item.Path)
			}
		}
		if p.selected(item.Scope) && (item.AppID != p.options.AppID || item.VersionID != p.options.VersionID || item.MetadataPrefix != p.options.MetadataPrefix) {
			return fmt.Errorf("asset export destination belongs to another app, version, or layout; use a separate destination (owned path %s)", item.Path)
		}
	}
	return nil
}

func exportPathScope(path, prefix string) (string, error) {
	if prefix != "." && prefix != "metadata" {
		return "", fmt.Errorf("invalid metadata layout")
	}
	if err := rootfs.ValidateRelative(path); err != nil {
		return "", err
	}
	if filepath.ToSlash(filepath.Clean(path)) != path || strings.Contains(path, "\\") {
		return "", fmt.Errorf("noncanonical asset path")
	}
	parts := strings.Split(path, "/")
	if len(parts) == 4 && parts[0] == "app_previews" {
		locale, err := shared.CanonicalizeAppStoreLocalizationLocale(parts[1])
		if err != nil || locale != parts[1] {
			return "", fmt.Errorf("invalid preview locale")
		}
		device, err := assets.NormalizePreviewType(parts[2])
		if err != nil || strings.ToLower(device) != parts[2] {
			return "", fmt.Errorf("invalid preview type")
		}
		name := parts[3]
		ext := strings.ToLower(filepath.Ext(name))
		if name == "order.json" || strings.HasSuffix(name, ".poster_frame.txt") || ext == ".mp4" || ext == ".mov" || ext == ".m4v" {
			return "previews", nil
		}
	}
	if prefix == "metadata" {
		if len(parts) < 2 || parts[0] != "metadata" {
			return "", fmt.Errorf("wrong metadata layout")
		}
		parts = parts[1:]
	}
	if len(parts) == 2 && parts[0] == "app_clip" && parts[1] == "action.txt" {
		return "app-clip", nil
	}
	if len(parts) == 3 && parts[1] == "app_clip" && (parts[2] == "subtitle.txt" || parts[2] == "header_image.png") {
		locale, err := shared.CanonicalizeAppStoreLocalizationLocale(parts[0])
		if err == nil && locale == parts[0] {
			return "app-clip", nil
		}
	}
	return "", fmt.Errorf("unrecognized asset path")
}

func exportPathLimit(path string) int64 {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".mp4", ".mov", ".m4v":
		return maxPreviewBytes
	case ".png":
		return 1 << 30
	default:
		return exportInventoryLimit
	}
}

func fingerprint(reader io.Reader, limit int64) (exportFingerprint, error) {
	hash := sha256.New()
	n, err := io.Copy(hash, io.LimitReader(reader, limit+1))
	if err != nil {
		return exportFingerprint{}, err
	}
	if n > limit {
		return exportFingerprint{}, fmt.Errorf("asset exceeds %d-byte export limit", limit)
	}
	return exportFingerprint{Size: n, SHA256: hex.EncodeToString(hash.Sum(nil))}, nil
}

func (p *ManagedExport) readFingerprint(path string) (*exportFingerprint, error) {
	if err := p.root.CheckCreateNewFile(path); err == nil {
		return nil, nil
	} else if !errors.Is(err, os.ErrExist) {
		return nil, err
	}
	file, err := p.root.OpenFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer file.Close()
	fp, err := fingerprint(file, exportPathLimit(path))
	return &fp, err
}

func matchesFingerprint(actual *exportFingerprint, item exportOwnership) bool {
	return actual != nil && (item.Current != nil && *actual == *item.Current || item.Pending != nil && *actual == *item.Pending)
}

func (p *ManagedExport) save() error {
	if err := p.root.MkdirAll(".asc", 0o755); err != nil {
		return err
	}
	data, err := json.Marshal(p.inventory)
	if err != nil {
		return err
	}
	if len(data) >= exportInventoryLimit {
		return fmt.Errorf("asset export inventory exceeds size limit")
	}
	data = append(data, '\n')
	previous := p.identity
	var next *rootfs.FileIdentity
	if p.identity == nil {
		next, err = p.root.CreateNewFileAtomicWithIdentity(exportInventoryPath, data, 0o600)
	} else {
		next, err = p.root.ReplaceFileIfSame(exportInventoryPath, p.identity, data, 0o600, true)
	}
	if next != nil {
		p.identity = next
	}
	if err != nil {
		return fmt.Errorf("save asset export inventory (prior and pending ownership must be retained for recovery): %w", err)
	}
	if previous != nil {
		return p.root.ReleaseFileIdentity(previous)
	}
	return nil
}

func (p *ManagedExport) index(path string) int {
	for i := range p.inventory.Files {
		if p.inventory.Files[i].Path == path {
			return i
		}
	}
	return -1
}

// Write records pending bytes before publication. Cleanup happens only after
// every desired asset has been handled; failures preserve the recovery journal.
func (p *ManagedExport) Write(ctx context.Context) ([]string, []string, error) {
	if !p.options.Clip && !p.options.Previews {
		return nil, nil, nil
	}
	if p.legacyWindows {
		paths, err := WriteExport(ctx, p.root, p.files, p.options.Overwrite)
		return paths, []string{"owned asset cleanup is unavailable on Windows; stale exported files are preserved and repeated exports are not guaranteed to round-trip"}, err
	}
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	// Root creation is deferred until execution, after inventory preflight.
	if err := p.root.MkdirAll(".", 0o755); err != nil {
		return nil, nil, err
	}
	var written, warnings []string
	desired := map[string]bool{}
	for _, f := range p.files {
		desired[f.Path] = true
	}
	for _, f := range p.files {
		if err := ctx.Err(); err != nil {
			return written, warnings, err
		}
		actual, err := p.readFingerprint(f.Path)
		if err != nil {
			return written, warnings, err
		}
		if actual != nil && !p.options.Overwrite {
			return written, warnings, fmt.Errorf("export %s: destination appeared after preflight: %w", f.Path, os.ErrExist)
		}
		index := p.index(f.Path)
		owned := index >= 0
		if index >= 0 && actual != nil && !matchesFingerprint(actual, p.inventory.Files[index]) {
			warnings = append(warnings, "preserving locally edited asset "+f.Path)
			continue
		}
		staged, fp, closeStage, err := stageExport(ctx, f)
		if err != nil {
			return written, warnings, fmt.Errorf("export %s: %w", f.Path, err)
		}
		err = func() error {
			defer closeStage()
			scope, _ := exportPathScope(f.Path, p.options.MetadataPrefix)
			if index < 0 {
				p.inventory.Files = append(p.inventory.Files, exportOwnership{Path: f.Path, AppID: p.options.AppID, VersionID: p.options.VersionID, MetadataPrefix: p.options.MetadataPrefix, Scope: scope})
				index = len(p.inventory.Files) - 1
			}
			item := &p.inventory.Files[index]
			prior := *item
			item.Pending = &fp
			if err := p.save(); err != nil {
				return err
			}
			if !owned || actual == nil || *actual != fp {
				if actual != nil {
					if err := p.remove(f.Path, *actual); err != nil {
						return fmt.Errorf("preserve asset and inventory after replacement failure: %w", err)
					}
				}
				if err := p.root.MkdirAll(filepath.Dir(f.Path), 0o755); err != nil {
					return err
				}
				if _, err := p.root.CreateNewFrom(f.Path, staged, 0o644); err != nil {
					if errors.Is(err, os.ErrExist) || errors.Is(err, rootfs.ErrFileIdentityChanged) {
						// A known publication collision/replacement is not our output, even
						// when its bytes happen to equal the staged download. Do not adopt it.
						if prior.Current == nil && prior.Pending == nil {
							p.inventory.Files = append(p.inventory.Files[:index], p.inventory.Files[index+1:]...)
						} else {
							p.inventory.Files[index] = prior
						}
						return fmt.Errorf("asset publication collided with a concurrent file; preserving it: %w", errors.Join(err, p.save()))
					}
					return fmt.Errorf("asset publication failed; pending inventory retained for retry: %w", err)
				}
			}
			published, err := p.readFingerprint(f.Path)
			if err != nil || published == nil || *published != fp {
				return fmt.Errorf("asset %s changed during publication; pending ownership retained: %w", f.Path, errors.Join(rootfs.ErrFileIdentityChanged, err))
			}
			item.Current = &fp
			item.Pending = nil
			return p.save()
		}()
		if err != nil {
			return written, warnings, err
		}
		written = append(written, f.Path)
	}
	for i := 0; i < len(p.inventory.Files); {
		item := p.inventory.Files[i]
		if !p.selected(item.Scope) || desired[item.Path] {
			i++
			continue
		}
		if err := ctx.Err(); err != nil {
			return written, warnings, err
		}
		actual, err := p.readFingerprint(item.Path)
		if err != nil {
			return written, warnings, err
		}
		if actual != nil {
			if !matchesFingerprint(actual, item) {
				warnings = append(warnings, "preserving locally edited stale asset "+item.Path)
				i++
				continue
			}
			if err := p.remove(item.Path, *actual); err != nil {
				return written, warnings, fmt.Errorf("cleanup %s failed; ownership retained: %w", item.Path, err)
			}
		}
		p.inventory.Files = append(p.inventory.Files[:i], p.inventory.Files[i+1:]...)
		if err := p.save(); err != nil {
			return written, warnings, err
		}
	}
	legacy, err := p.unrecorded(desired)
	warnings = append(warnings, legacy...)
	return written, warnings, err
}

func (p *ManagedExport) remove(path string, fp exportFingerprint) error {
	decoded, err := hex.DecodeString(fp.SHA256)
	if err != nil {
		return err
	}
	var digest [sha256.Size]byte
	copy(digest[:], decoded)
	return p.root.RemoveFileIfSHA256Same(path, fp.Size, digest)
}

func stageExport(ctx context.Context, f ExportFile) (io.Reader, exportFingerprint, func(), error) {
	if f.Text != nil {
		data := []byte(*f.Text)
		fp, err := fingerprint(bytes.NewReader(data), exportPathLimit(f.Path))
		return bytes.NewReader(data), fp, func() {}, err
	}
	dir, err := os.MkdirTemp("", "asc-store-asset-*")
	if err != nil {
		return nil, exportFingerprint{}, nil, err
	}
	cleanup := func() { _ = os.RemoveAll(dir) }
	temp := filepath.Join(dir, "asset")
	c, cancel := shared.ContextWithUploadTimeout(shared.ContextWithoutTimeout(ctx))
	defer cancel()
	if _, _, err := assets.DownloadMediaURL(c, f.URL, temp, false); err != nil {
		cleanup()
		return nil, exportFingerprint{}, nil, errors.New("media download failed (URL omitted)")
	}
	file, err := os.Open(temp)
	if err != nil {
		cleanup()
		return nil, exportFingerprint{}, nil, err
	}
	closeStage := func() { _ = file.Close(); cleanup() }
	fp, err := fingerprint(file, exportPathLimit(f.Path))
	if err != nil {
		closeStage()
		return nil, exportFingerprint{}, nil, err
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		closeStage()
		return nil, exportFingerprint{}, nil, err
	}
	return file, fp, closeStage, nil
}

func (p *ManagedExport) unrecorded(desired map[string]bool) ([]string, error) {
	var warnings []string
	var walk func(string, int) error
	walk = func(dir string, depth int) error {
		entries, err := readDir(p.root, dir)
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		if err != nil {
			return err
		}
		for _, entry := range entries {
			path := filepath.ToSlash(filepath.Join(dir, entry.Name()))
			if entry.IsDir() {
				if depth > 0 {
					if err := walk(path, depth-1); err != nil {
						return err
					}
				}
				continue
			}
			scope, err := exportPathScope(path, p.options.MetadataPrefix)
			if err == nil && p.selected(scope) && !desired[path] && p.index(path) < 0 {
				warnings = append(warnings, "preserving unrecorded legacy asset "+path)
			}
		}
		return nil
	}
	if p.options.Previews {
		if err := walk("app_previews", 2); err != nil {
			return warnings, err
		}
	}
	if p.options.Clip {
		if err := walk(filepath.Join(p.options.MetadataPrefix, "app_clip"), 0); err != nil {
			return warnings, err
		}
		entries, err := readDir(p.root, p.options.MetadataPrefix)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return warnings, err
		}
		for _, entry := range entries {
			locale, err := shared.CanonicalizeAppStoreLocalizationLocale(entry.Name())
			if entry.IsDir() && err == nil && locale == entry.Name() {
				if err := walk(filepath.Join(p.options.MetadataPrefix, locale, "app_clip"), 0); err != nil {
					return warnings, err
				}
			}
		}
	}
	sort.Strings(warnings)
	return warnings, nil
}
