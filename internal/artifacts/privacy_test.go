package artifacts

import (
	"archive/zip"
	"bytes"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"

	"howett.net/plist"
)

func TestPrivacyInventoryAttributesEachContainingBundle(t *testing.T) {
	root := "Payload/Demo.app"
	binary, err := plist.Marshal(map[string]any{"NSPrivacyTrackingDomains": []string{"metrics.example"}}, plist.BinaryFormat)
	if err != nil {
		t.Fatal(err)
	}
	files := map[string][]byte{root + "/Info.plist": plistXML(t, map[string]any{"CFBundleIdentifier": "com.example.demo"})}
	owners := map[string]struct{ kind, id string }{root: {"app", "com.example.demo"}, root + "/PlugIns/Widget.appex": {"extension", "com.example.widget"}, root + "/Frameworks/Core.framework": {"framework", "com.example.core"}, root + "/Frameworks/Core.framework/Assets.bundle": {"bundle", "com.example.assets"}, root + "/Resources/Unidentified.bundle": {"bundle", ""}}
	for bundle, owner := range owners {
		if owner.id != "" {
			files[bundle+"/Info.plist"] = plistXML(t, map[string]any{"CFBundleIdentifier": owner.id})
		}
		files[bundle+"/PrivacyInfo.xcprivacy"] = plistXML(t, map[string]any{"NSPrivacyTracking": false})
	}
	resource := root + "/Frameworks/Core.framework/Assets.bundle/PrivacyInfo.xcprivacy"
	files[resource] = binary
	files["NotPackaged/PrivacyInfo.xcprivacy"] = []byte("invalid")
	ipa := zipArtifact(t, files)
	manifest, err := InspectIPAWithOptions(bytes.NewReader(ipa), int64(len(ipa)), IPAOptions{IncludePrivacyManifests: true})
	if err != nil {
		t.Fatal(err)
	}
	inventory := manifest.PrivacyManifests
	if inventory == nil || !inventory.Complete || len(inventory.Manifests) != len(owners) {
		t.Fatalf("inventory=%+v", inventory)
	}
	previous := ""
	for _, entry := range inventory.Manifests {
		owner, ok := owners[entry.BundlePath]
		if !ok || entry.BundleKind != owner.kind || entry.BundleID != owner.id || entry.Error != "" || entry.Path <= previous {
			t.Fatalf("entry=%+v previous=%s", entry, previous)
		}
		previous = entry.Path
		want := map[string]any{"NSPrivacyTracking": false}
		if entry.Path == resource {
			want = map[string]any{"NSPrivacyTrackingDomains": []any{"metrics.example"}}
		}
		if !reflect.DeepEqual(entry.Declarations, want) {
			t.Fatalf("declarations=%#v want=%#v", entry.Declarations, want)
		}
	}
}

func TestPrivacyInventoryReportsAbsentAndUnreadableManifests(t *testing.T) {
	for _, test := range []struct {
		name, path string
		data       []byte
		mode       os.FileMode
		duplicate  bool
	}{
		{name: "absent"},
		{name: "malformed", path: "PrivacyInfo.xcprivacy", data: []byte("<plist><dict>")},
		{name: "array root", path: "PrivacyInfo.xcprivacy", data: []byte("<?xml version=\"1.0\"?><plist><array/></plist>")},
		{name: "symlink", path: "PrivacyInfo.xcprivacy", data: []byte("../../outside"), mode: os.ModeSymlink | 0o777},
		{name: "traversal", path: "../PrivacyInfo.xcprivacy", data: []byte("invalid")},
		{name: "duplicate", path: "PrivacyInfo.xcprivacy", data: []byte("invalid"), duplicate: true},
		{name: "oversized", path: "PrivacyInfo.xcprivacy", data: bytes.Repeat([]byte("x"), maxPlistBytes+1)},
	} {
		t.Run(test.name, func(t *testing.T) {
			var data bytes.Buffer
			writer := zip.NewWriter(&data)
			info, err := writer.Create("Payload/Demo.app/Info.plist")
			if err != nil {
				t.Fatal(err)
			}
			if _, err := info.Write(plistXML(t, map[string]any{"CFBundleIdentifier": "com.example.demo"})); err != nil {
				t.Fatal(err)
			}
			if test.path != "" {
				names := []string{"Payload/Demo.app/" + test.path}
				if test.duplicate {
					names = append(names, strings.ReplaceAll(names[0], "/", "\\"))
				}
				for _, name := range names {
					header := &zip.FileHeader{Name: name, Method: zip.Deflate}
					if test.mode != 0 {
						header.SetMode(test.mode)
					}
					entry, err := writer.CreateHeader(header)
					if err != nil {
						t.Fatal(err)
					}
					if _, err := entry.Write(test.data); err != nil {
						t.Fatal(err)
					}
				}
			}
			if err := writer.Close(); err != nil {
				t.Fatal(err)
			}
			baseline, err := InspectIPAWithOptions(bytes.NewReader(data.Bytes()), int64(data.Len()), IPAOptions{})
			if err != nil || baseline.PrivacyManifests != nil {
				t.Fatalf("default inspection changed: %+v %v", baseline, err)
			}
			manifest, err := InspectIPAWithOptions(bytes.NewReader(data.Bytes()), int64(data.Len()), IPAOptions{IncludePrivacyManifests: true})
			inventory := manifest.PrivacyManifests
			if test.name == "absent" {
				if err != nil || inventory == nil || !inventory.Complete || inventory.Manifests == nil || len(inventory.Manifests) != 0 {
					t.Fatalf("inventory=%+v err=%v", inventory, err)
				}
				return
			}
			if !errors.Is(err, ErrInvalidArchive) || manifest.Status != "unreadable" || inventory == nil || inventory.Complete || len(inventory.Manifests) == 0 {
				t.Fatalf("manifest=%+v err=%v", manifest, err)
			}
			for _, entry := range inventory.Manifests {
				if entry.Error == "" {
					t.Fatalf("missing failure evidence: %+v", entry)
				}
			}
		})
	}
}

func TestPrivacyInventoryBoundsAggregateReadsAndRetainsValidEntries(t *testing.T) {
	manifest := plistXML(t, map[string]any{"NSPrivacyTracking": false})
	manifest = append(manifest, bytes.Repeat([]byte(" "), maxPlistBytes-len(manifest))...)
	files := map[string][]byte{"Payload/Demo.app/Info.plist": plistXML(t, map[string]any{"CFBundleIdentifier": "com.example.demo"})}
	for _, dir := range []string{"A", "B", "C", "D", "E"} {
		files["Payload/Demo.app/Resources/"+dir+"/PrivacyInfo.xcprivacy"] = manifest
	}
	ipa := zipArtifact(t, files)
	result, err := InspectIPAWithOptions(bytes.NewReader(ipa), int64(len(ipa)), IPAOptions{IncludePrivacyManifests: true})
	if err == nil || !strings.Contains(err.Error(), "aggregate limit") || result.PrivacyManifests == nil || result.PrivacyManifests.Complete {
		t.Fatalf("inventory=%+v err=%v", result.PrivacyManifests, err)
	}
	valid, failed := 0, 0
	for _, entry := range result.PrivacyManifests.Manifests {
		if entry.Error != "" {
			failed++
		} else {
			valid++
			if entry.Declarations["NSPrivacyTracking"] != false {
				t.Fatalf("valid entry lost: %+v", entry)
			}
		}
	}
	if valid != 3 || failed != 2 {
		t.Fatalf("valid=%d failed=%d", valid, failed)
	}
}
