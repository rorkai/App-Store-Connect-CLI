package storeassets

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/rootfs"
)

func TestManagedExportRecoveryAndTargetIsolation(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("strict inventory updates unsupported")
	}
	root, err := rootfs.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	options := ExportOptions{AppID: "APP", VersionID: "VERSION", MetadataPrefix: ".", Clip: true, Overwrite: true}
	action, subtitle := "PLAY\n", "Original\n"
	export := func(files []ExportFile) ([]string, error) {
		t.Helper()
		p, err := PrepareExport(root, files, options)
		if err != nil {
			return nil, err
		}
		paths, _, err := p.Write(context.Background())
		return paths, err
	}
	initial := []ExportFile{{Path: "app_clip/action.txt", Text: &action}, {Path: "en-US/app_clip/subtitle.txt", Text: &subtitle}}
	if _, err := export(initial); err != nil {
		t.Fatal(err)
	}
	// A partial refresh publishes one known file but must not prune the old subtitle.
	updated := "VIEW\n"
	paths, err := export([]ExportFile{{Path: "app_clip/action.txt", Text: &updated}, {Path: "en-US/app_clip/header_image.png", URL: "invalid://media"}})
	if err == nil || len(paths) != 1 {
		t.Fatalf("want partial publication, got %v,%v", paths, err)
	}
	if data, err := root.ReadFile("en-US/app_clip/subtitle.txt"); err != nil || string(data) != subtitle {
		t.Fatalf("failed export lost prior asset: %q,%v", data, err)
	}
	inventoryBefore, err := root.ReadFile(exportInventoryPath)
	if err != nil {
		t.Fatal(err)
	}
	other := options
	other.VersionID = "OTHER"
	if _, err := PrepareExport(root, nil, other); err == nil || !strings.Contains(err.Error(), "separate destination") {
		t.Fatalf("target switch accepted: %v", err)
	}
	if data, err := root.ReadFile(exportInventoryPath); err != nil || string(data) != string(inventoryBefore) {
		t.Fatal("target conflict mutated inventory")
	}
	// Recovery removes both old and refreshed ownership after a successful empty export.
	if _, err := export(nil); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"app_clip/action.txt", "en-US/app_clip/subtitle.txt"} {
		if _, err := root.OpenFile(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("stale %s remains: %v", path, err)
		}
	}
	clip, present, err := ReadAppClip(root.Path())
	if err != nil || present {
		t.Fatalf("empty exported folders imply App Clip import: present=%v err=%v", present, err)
	}
	defer clip.Close()
}

func TestManagedExportRejectsInvalidInventoryBeforeWrites(t *testing.T) {
	for _, scenario := range []string{"schema", "traversal", "unknown", "duplicate", "bad-hash", "missing-schema", "null"} {
		t.Run(scenario, func(t *testing.T) {
			root, err := rootfs.New(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			defer root.Close()
			if err := root.MkdirAll(".asc", 0o755); err != nil {
				t.Fatal(err)
			}
			value := "PLAY\n"
			fp, _ := fingerprint(strings.NewReader(value), 100)
			entry := exportOwnership{Path: "app_clip/action.txt", AppID: "APP", VersionID: "VERSION", MetadataPrefix: ".", Scope: "app-clip", Current: &fp}
			inventory := exportInventory{Schema: 1, Files: []exportOwnership{entry}}
			switch scenario {
			case "schema":
				inventory.Schema = 2
			case "traversal":
				inventory.Files[0].Path = "../action.txt"
			case "unknown":
				inventory.Files[0].Path = "secrets.txt"
			case "duplicate":
				inventory.Files = append(inventory.Files, entry)
			case "bad-hash":
				inventory.Files[0].Current.SHA256 = "bad"
			}
			data, _ := json.Marshal(inventory)
			if scenario == "missing-schema" {
				data = []byte(`{"files":[]}`)
			}
			if scenario == "null" {
				data = []byte(`null`)
			}
			if err := root.WriteFile(exportInventoryPath, data, 0o600); err != nil {
				t.Fatal(err)
			}
			_, err = PrepareExport(root, []ExportFile{{Path: "app_clip/action.txt", Text: &value}}, ExportOptions{AppID: "APP", VersionID: "VERSION", MetadataPrefix: ".", Clip: true, Overwrite: true})
			if err == nil {
				t.Fatal("invalid inventory accepted")
			}
			if _, err := os.Stat(filepath.Join(root.Path(), "app_clip")); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("preflight created asset directory: %v", err)
			}
			after, _ := root.ReadFile(exportInventoryPath)
			if string(after) != string(data) {
				t.Fatal("invalid inventory was replaced")
			}
		})
	}
}

func TestManagedExportRecoversPendingOwnership(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("strict inventory updates unsupported")
	}
	for _, state := range []string{"prior", "published", "missing", "edited"} {
		t.Run(state, func(t *testing.T) {
			root, err := rootfs.New(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			defer root.Close()
			value := "PLAY\n"
			next := "VIEW\n"
			options := ExportOptions{AppID: "APP", VersionID: "VERSION", MetadataPrefix: ".", Clip: true, Overwrite: true}
			p, err := PrepareExport(root, []ExportFile{{Path: "app_clip/action.txt", Text: &value}}, options)
			if err != nil {
				t.Fatal(err)
			}
			if _, _, err := p.Write(context.Background()); err != nil {
				t.Fatal(err)
			}
			pending, _ := fingerprint(strings.NewReader(next), 100)
			p.inventory.Files[0].Pending = &pending
			if err := p.save(); err != nil {
				t.Fatal(err)
			}
			// These are the durable states after interruption before removal, after
			// publication, in the publication gap, and following an operator edit.
			switch state {
			case "published":
				if err := root.WriteFile("app_clip/action.txt", []byte(next), 0o600); err != nil {
					t.Fatal(err)
				}
			case "missing":
				if err := os.Remove(filepath.Join(root.Path(), "app_clip/action.txt")); err != nil {
					t.Fatal(err)
				}
			case "edited":
				if err := root.WriteFile("app_clip/action.txt", []byte("Local edit"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			retry, err := PrepareExport(root, nil, options)
			if err != nil {
				t.Fatal(err)
			}
			_, warnings, err := retry.Write(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			if state == "edited" {
				data, err := root.ReadFile("app_clip/action.txt")
				if err != nil || string(data) != "Local edit" || len(warnings) != 1 || len(retry.inventory.Files) != 1 {
					t.Fatalf("edit/recovery ownership lost: %q,%v,%v", data, err, warnings)
				}
			} else if _, err := root.ReadFile("app_clip/action.txt"); !errors.Is(err, os.ErrNotExist) || len(retry.inventory.Files) != 0 {
				t.Fatalf("pending export did not recover: %v", err)
			}
		})
	}
}

func TestReadAppClipRequiresCanonicalFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "en-US", "app_clip"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "app_clip"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "app_clip", "notes.txt"), []byte("unrelated"), 0o600); err != nil {
		t.Fatal(err)
	}
	clip, present, err := ReadAppClip(dir)
	clip.Close()
	if err != nil || present {
		t.Fatalf("empty/unrelated folders imply App Clip: %v,%v", present, err)
	}
	if err := os.WriteFile(filepath.Join(dir, "en-US", "app_clip", "subtitle.txt"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	clip, present, err = ReadAppClip(dir)
	if err != nil || !present {
		t.Fatalf("explicit empty subtitle lost intent: %v,%v", present, err)
	}
	defer clip.Close()
	if _, exists := clip.Subtitles["en-US"]; !exists {
		t.Fatal("empty subtitle missing")
	}
}

func TestManagedExportPublicationFailureRetainsRecoveryAndConcurrentEdits(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("strict inventory updates unsupported")
	}
	for _, scenario := range []string{"publication-gap", "edited-asset", "inventory-replaced"} {
		t.Run(scenario, func(t *testing.T) {
			root, err := rootfs.New(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			defer root.Close()
			options := ExportOptions{AppID: "APP", VersionID: "VERSION", MetadataPrefix: ".", Clip: true, Overwrite: true}
			path := "en-US/app_clip/header_image.png"
			original := "original"
			if scenario != "publication-gap" {
				first, err := PrepareExport(root, []ExportFile{{Path: path, Text: &original}}, options)
				if err != nil {
					t.Fatal(err)
				}
				if _, _, err := first.Write(context.Background()); err != nil {
					t.Fatal(err)
				}
			}
			p, err := PrepareExport(root, []ExportFile{{Path: path, URL: "https://media.example/header.png"}}, options)
			if err != nil {
				t.Fatal(err)
			}
			oldTransport := http.DefaultTransport
			t.Cleanup(func() { http.DefaultTransport = oldTransport })
			http.DefaultTransport = exportRoundTripper(func(req *http.Request) (*http.Response, error) {
				switch scenario {
				case "publication-gap":
					if err := root.WriteFile("en-US", []byte("unrelated parent"), 0o600); err != nil {
						t.Fatal(err)
					}
				case "edited-asset":
					if err := root.WriteFile(path, []byte("operator edit"), 0o600); err != nil {
						t.Fatal(err)
					}
				case "inventory-replaced":
					bytes, err := root.ReadFile(exportInventoryPath)
					if err != nil {
						t.Fatal(err)
					}
					if err := root.WriteFile(exportInventoryPath, bytes, 0o600); err != nil {
						t.Fatal(err)
					}
				}
				return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader("new delivered bytes")), Header: http.Header{}}, nil
			})
			if _, _, err := p.Write(context.Background()); err == nil {
				t.Fatal("concurrent change/publication failure reported success")
			}
			data, err := root.ReadFile(exportInventoryPath)
			if err != nil {
				t.Fatal(err)
			}
			var inventory exportInventory
			if err := json.Unmarshal(data, &inventory); err != nil {
				t.Fatal(err)
			}
			if len(inventory.Files) != 1 {
				t.Fatalf("recovery ownership missing: %s", data)
			}
			item := inventory.Files[0]
			if scenario == "inventory-replaced" {
				if item.Pending != nil {
					t.Fatal("concurrent inventory was overwritten")
				}
				if got, _ := root.ReadFile(path); string(got) != original {
					t.Fatal("asset changed after inventory conflict")
				}
			} else {
				if item.Pending == nil {
					t.Fatalf("pending publication lost: %s", data)
				}
				if scenario == "edited-asset" {
					if item.Current == nil {
						t.Fatal("prior ownership lost")
					}
					if got, _ := root.ReadFile(path); string(got) != "operator edit" {
						t.Fatal("concurrent asset edit overwritten")
					}
				} else if got, _ := root.ReadFile("en-US"); string(got) != "unrelated parent" {
					t.Fatal("unrelated parent lost")
				}
			}
		})
	}
}

type exportRoundTripper func(*http.Request) (*http.Response, error)

func (f exportRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func TestManagedExportWindowsLimit(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("native Windows contract")
	}
	root, err := rootfs.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	options := ExportOptions{AppID: "APP", VersionID: "VERSION", MetadataPrefix: ".", Clip: true, Overwrite: true}
	value := "PLAY\n"
	p, err := PrepareExport(root, []ExportFile{{Path: filepath.Join("app_clip", "action.txt"), Text: &value}}, options)
	if err != nil {
		t.Fatal(err)
	}
	_, warnings, err := p.Write(context.Background())
	if err != nil || len(warnings) != 1 || !strings.Contains(warnings[0], "unavailable on Windows") {
		t.Fatalf("missing explicit platform warning: %v,%v", warnings, err)
	}
	if err := root.MkdirAll(".asc", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := root.WriteFile(exportInventoryPath, []byte(`{"schema":1,"files":[]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := PrepareExport(root, nil, options); !errors.Is(err, rootfs.ErrFileIdentityMutationUnsupported) {
		t.Fatalf("existing inventory not protected: %v", err)
	}
	if got, _ := root.ReadFile("app_clip/action.txt"); string(got) != value {
		t.Fatal("unsupported cleanup changed asset")
	}
}

func TestManagedExportInventoryDescriptorBound(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("descriptor inventory requires /dev/fd")
	}
	count := func() int {
		t.Helper()
		entries, err := os.ReadDir("/dev/fd")
		if err != nil {
			t.Fatal(err)
		}
		return len(entries)
	}
	root, err := rootfs.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	p, err := PrepareExport(root, nil, ExportOptions{AppID: "APP", VersionID: "VERSION", MetadataPrefix: ".", Clip: true, Overwrite: true})
	if err != nil {
		t.Fatal(err)
	}
	before := count()
	for i := 0; i < 120; i++ {
		if err := p.save(); err != nil {
			t.Fatal(err)
		}
	}
	if delta := count() - before; delta > 4 {
		t.Fatalf("120 inventory revisions retained %d descriptors; want bounded <=4", delta)
	}
}

func TestManagedExportEmptyMissingDestination(t *testing.T) {
	root, err := rootfs.New(filepath.Join(t.TempDir(), "new-output"))
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	p, err := PrepareExport(root, nil, ExportOptions{AppID: "APP", VersionID: "VERSION", MetadataPrefix: ".", Clip: true, Previews: true, Overwrite: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(root.Path()); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("preflight created destination")
	}
	if _, _, err := p.Write(context.Background()); err != nil {
		t.Fatalf("empty export to new destination: %v", err)
	}
}

func TestManagedExportMissingDestinationAndSymlinkInventory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "missing", "output")
	root, err := rootfs.New(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	value := "PLAY\n"
	options := ExportOptions{AppID: "APP", VersionID: "VERSION", MetadataPrefix: ".", Clip: true, Overwrite: true}
	p, err := PrepareExport(root, []ExportFile{{Path: "app_clip/action.txt", Text: &value}}, options)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dir); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("preflight created missing output root")
	}
	if _, _, err := p.Write(context.Background()); err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS == "windows" {
		return
	}
	other := t.TempDir()
	if err := os.Remove(filepath.Join(dir, exportInventoryPath)); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(other, "inventory.json"), filepath.Join(dir, exportInventoryPath)); err != nil {
		t.Fatal(err)
	}
	if _, err := PrepareExport(root, nil, options); !errors.Is(err, rootfs.ErrSymlink) {
		t.Fatalf("symlink inventory accepted: %v", err)
	}
	if _, err := os.Stat(filepath.Join(other, "inventory.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("symlink target touched")
	}
}

func TestManagedExportNoForcePreservesFileCreatedAfterPreflight(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("strict managed export path")
	}
	root, err := rootfs.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	value := "PLAY\n"
	p, err := PrepareExport(root, []ExportFile{{Path: "app_clip/action.txt", Text: &value}}, ExportOptions{AppID: "APP", VersionID: "VERSION", MetadataPrefix: ".", Clip: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := root.MkdirAll("app_clip", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := root.WriteFile("app_clip/action.txt", []byte("concurrent file"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := p.Write(context.Background()); !errors.Is(err, os.ErrExist) {
		t.Fatalf("no-force overwrite must reject concurrent file: %v", err)
	}
	if got, _ := root.ReadFile("app_clip/action.txt"); string(got) != "concurrent file" {
		t.Fatalf("concurrent file overwritten: %q", got)
	}
	if _, err := root.ReadFile(exportInventoryPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("failed no-force export recorded ownership: %v", err)
	}
}

func TestManagedExportDoesNotAdoptConcurrentPublicationCollision(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("strict managed export path")
	}
	root, err := rootfs.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	path := "en-US/app_clip/header_image.png"
	content := "same delivered bytes"
	options := ExportOptions{AppID: "APP", VersionID: "VERSION", MetadataPrefix: ".", Clip: true}
	p, err := PrepareExport(root, []ExportFile{{Path: path, URL: "https://media.example/header.png"}}, options)
	if err != nil {
		t.Fatal(err)
	}
	oldTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = oldTransport })
	http.DefaultTransport = exportRoundTripper(func(req *http.Request) (*http.Response, error) {
		if err := root.MkdirAll("en-US/app_clip", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := root.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(content)), Header: http.Header{}}, nil
	})
	if _, _, err := p.Write(context.Background()); !errors.Is(err, os.ErrExist) {
		t.Fatalf("exclusive publication did not reject concurrent file: %v", err)
	}
	retry, err := PrepareExport(root, nil, options)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := retry.Write(context.Background()); err != nil {
		t.Fatal(err)
	}
	if data, err := root.ReadFile(path); err != nil || string(data) != content {
		t.Fatalf("failed publication adopted then pruned concurrent bytes: %q,%v", data, err)
	}
	if len(retry.inventory.Files) != 0 {
		t.Fatal("concurrent file retained as owned")
	}
}
