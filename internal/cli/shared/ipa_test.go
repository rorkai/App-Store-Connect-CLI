package shared

import (
	"archive/zip"
	"bytes"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"howett.net/plist"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/infoplist"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/infoplist/infoplisttest"
)

func TestExtractBundleInfoFromIPA(t *testing.T) {
	plistData := buildInfoPlist(t, "1.2.3", "45")
	ipaPath := writeTestIPA(t, map[string][]byte{
		"Payload/Demo.app/Info.plist": plistData,
	})

	info, err := ExtractBundleInfoFromIPA(ipaPath)
	if err != nil {
		t.Fatalf("extractBundleInfoFromIPA() error: %v", err)
	}
	if info.Version != "1.2.3" {
		t.Fatalf("expected version 1.2.3, got %q", info.Version)
	}
	if info.BuildNumber != "45" {
		t.Fatalf("expected build number 45, got %q", info.BuildNumber)
	}
	if info.BundleID != "com.example.demo" {
		t.Fatalf("expected bundle ID com.example.demo, got %q", info.BundleID)
	}
}

func TestExtractBundleInfoFromIPA_PrefersTopLevelApp(t *testing.T) {
	plistData := buildInfoPlist(t, "2.0.0", "200")
	extensionPlist, err := plist.Marshal(map[string]any{
		"CFBundleIdentifier":         "com.example.demo.widget",
		"CFBundleShortVersionString": "9.9.9",
		"CFBundleVersion":            "999",
	}, plist.XMLFormat)
	if err != nil {
		t.Fatalf("marshal extension plist: %v", err)
	}
	ipaPath := writeTestIPA(t, map[string][]byte{
		"Payload/Demo.app/Info.plist":                              plistData,
		"Payload/Demo.app/PlugIns/Widget.appex/Info.plist":         extensionPlist,
		"Payload/Demo.app/PlugIns/Widget.appex/Other.plist":        []byte("ignored"),
		"Payload/Demo.app/PlugIns/Widget.appex/Info.plist.bak":     []byte("ignored"),
		"Payload/Demo.app/Frameworks/Demo.framework/Info.plist":    extensionPlist,
		"Payload/Demo.app/Frameworks/Another.framework/Info.plist": extensionPlist,
	})

	info, err := ExtractBundleInfoFromIPA(ipaPath)
	if err != nil {
		t.Fatalf("extractBundleInfoFromIPA() error: %v", err)
	}
	if info.Version != "2.0.0" {
		t.Fatalf("expected version 2.0.0, got %q", info.Version)
	}
	if info.BuildNumber != "200" {
		t.Fatalf("expected build number 200, got %q", info.BuildNumber)
	}
	if info.BundleID != "com.example.demo" {
		t.Fatalf("expected top-level bundle ID com.example.demo, got %q", info.BundleID)
	}
}

func TestExtractBundleInfoFromIPA_MissingInfoPlist(t *testing.T) {
	ipaPath := writeTestIPA(t, map[string][]byte{
		"Payload/Demo.app/README.txt": []byte("no plist"),
	})

	_, err := ExtractBundleInfoFromIPA(ipaPath)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestExtractBundleInfoFromIPA_NumericBuildVersion(t *testing.T) {
	plistData := buildInfoPlistWithValues(t, "3.1.0", 210, plist.XMLFormat)
	ipaPath := writeTestIPA(t, map[string][]byte{
		"Payload/Demo.app/Info.plist": plistData,
	})

	info, err := ExtractBundleInfoFromIPA(ipaPath)
	if err != nil {
		t.Fatalf("extractBundleInfoFromIPA() error: %v", err)
	}
	if info.Version != "3.1.0" {
		t.Fatalf("expected version 3.1.0, got %q", info.Version)
	}
	if info.BuildNumber != "210" {
		t.Fatalf("expected build number 210, got %q", info.BuildNumber)
	}
}

func TestExtractBundleInfoFromIPA_BinaryPlist(t *testing.T) {
	plistData := buildInfoPlistWithValues(t, "4.0.1", int64(7), plist.BinaryFormat)
	ipaPath := writeTestIPA(t, map[string][]byte{
		"Payload/Demo.app/Info.plist": plistData,
	})

	info, err := ExtractBundleInfoFromIPA(ipaPath)
	if err != nil {
		t.Fatalf("extractBundleInfoFromIPA() error: %v", err)
	}
	if info.Version != "4.0.1" {
		t.Fatalf("expected version 4.0.1, got %q", info.Version)
	}
	if info.BuildNumber != "7" {
		t.Fatalf("expected build number 7, got %q", info.BuildNumber)
	}
}

func TestExtractBundleInfoFromIPA_DetectsPlatform(t *testing.T) {
	tests := []struct {
		name               string
		platformName       string
		supportedPlatforms []string
		format             int
		want               string
	}{
		{name: "iOS", platformName: "iphoneos", supportedPlatforms: []string{"iPhoneOS"}, format: plist.XMLFormat, want: "IOS"},
		{name: "tvOS binary plist", platformName: "appletvos", supportedPlatforms: []string{"AppleTVOS"}, format: plist.BinaryFormat, want: "TV_OS"},
		{name: "visionOS", platformName: "xros", supportedPlatforms: []string{"XROS"}, format: plist.XMLFormat, want: "VISION_OS"},
		{name: "macOS", platformName: "macosx", supportedPlatforms: []string{"MacOSX"}, format: plist.XMLFormat, want: "MAC_OS"},
		{name: "supported platform fallback", supportedPlatforms: []string{"AppleTVOS"}, format: plist.XMLFormat, want: "TV_OS"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			plistData, err := plist.Marshal(map[string]any{
				"CFBundleIdentifier":         "com.example.demo",
				"CFBundleShortVersionString": "1.2.3",
				"CFBundleVersion":            "45",
				"DTPlatformName":             test.platformName,
				"CFBundleSupportedPlatforms": test.supportedPlatforms,
			}, test.format)
			if err != nil {
				t.Fatalf("marshal Info.plist: %v", err)
			}
			ipaPath := writeTestIPA(t, map[string][]byte{
				"Payload/Demo.app/Info.plist": plistData,
			})

			info, err := ExtractBundleInfoFromIPA(ipaPath)
			if err != nil {
				t.Fatalf("ExtractBundleInfoFromIPA() error: %v", err)
			}
			if string(info.Platform) != test.want {
				t.Fatalf("expected platform %s, got %q", test.want, info.Platform)
			}
		})
	}
}

func TestExtractBundleInfoFromIPA_RejectsConflictingPlatformMetadata(t *testing.T) {
	plistData, err := plist.Marshal(map[string]any{
		"CFBundleIdentifier":         "com.example.demo",
		"CFBundleShortVersionString": "1.2.3",
		"CFBundleVersion":            "45",
		"DTPlatformName":             "iphoneos",
		"CFBundleSupportedPlatforms": []string{"AppleTVOS"},
	}, plist.XMLFormat)
	if err != nil {
		t.Fatalf("marshal Info.plist: %v", err)
	}
	ipaPath := writeTestIPA(t, map[string][]byte{
		"Payload/Demo.app/Info.plist": plistData,
	})

	_, err = ExtractBundleInfoFromIPA(ipaPath)
	if err == nil {
		t.Fatal("expected conflicting platform metadata error")
	}
	if !strings.Contains(err.Error(), "conflicting IPA platform metadata") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestExtractBundleInfoFromIPA_InfoPlistAtSizeLimit(t *testing.T) {
	plistData := buildInfoPlistOfSize(t, infoplist.MaxBytes)
	ipaPath := writeTestIPA(t, map[string][]byte{
		"Payload/Demo.app/Info.plist": plistData,
	})

	info, err := ExtractBundleInfoFromIPA(ipaPath)
	if err != nil {
		t.Fatalf("ExtractBundleInfoFromIPA() error: %v", err)
	}
	if info.Version != "1.2.3" || info.BuildNumber != "45" {
		t.Fatalf("unexpected bundle info at size limit: %+v", info)
	}
}

func TestExtractBundleInfoFromIPA_RejectsInfoPlistOneByteOverLimit(t *testing.T) {
	plistData := buildInfoPlistOfSize(t, infoplist.MaxBytes+1)
	ipaPath := writeTestIPA(t, map[string][]byte{
		"Payload/Demo.app/Info.plist": plistData,
	})

	_, err := ExtractBundleInfoFromIPA(ipaPath)
	if err == nil {
		t.Fatal("expected oversized Info.plist rejection, got nil")
	}
	if !strings.Contains(err.Error(), "Info.plist limit") {
		t.Fatalf("expected Info.plist limit error, got %v", err)
	}
}

func TestExtractBundleInfoFromIPA_RejectsHighlyCompressibleInfoPlist(t *testing.T) {
	ipaPath := writeTestIPA(t, map[string][]byte{
		"Payload/Demo.app/Info.plist": bytes.Repeat([]byte("A"), infoplist.MaxBytes*2),
	})

	_, err := ExtractBundleInfoFromIPA(ipaPath)
	if err == nil {
		t.Fatal("expected compressible oversized Info.plist rejection, got nil")
	}
	if !strings.Contains(err.Error(), "declared uncompressed size") {
		t.Fatalf("expected declared-size rejection, got %v", err)
	}
}

// A member that understates its uncompressed size must not slip past the
// declared-size check and then expand freely. archive/zip refuses a member as
// soon as it streams past its advertised size, and the bounded read is the
// backstop if it ever does not, so the caller always gets an error instead of
// an unbounded expansion.
func TestExtractBundleInfoFromIPA_RejectsUnderstatedInfoPlistSize(t *testing.T) {
	ipaPath := writeTestIPA(t, map[string][]byte{
		"Payload/Demo.app/Info.plist": bytes.Repeat([]byte("A"), infoplist.MaxBytes*2),
	})
	infoplisttest.ForgeZipDeclaredUncompressedSize(t, ipaPath, 1024)

	if _, err := ExtractBundleInfoFromIPA(ipaPath); err == nil {
		t.Fatal("expected rejection for forged ZIP size metadata, got nil")
	}
}

func TestValidateIPAPathRejectsSymlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target.ipa")
	if err := os.WriteFile(target, []byte("payload"), 0o600); err != nil {
		t.Fatalf("write target file: %v", err)
	}

	link := filepath.Join(dir, "app.ipa")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlink not supported: %v", err)
	}

	_, err := ValidateIPAPath(link)
	if err == nil {
		t.Fatal("expected symlink rejection error")
	}
	if !strings.Contains(err.Error(), "refusing to read symlink") {
		t.Fatalf("expected symlink rejection message, got %v", err)
	}
}

func TestValidateIPAPathAllowsRegularFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "app.ipa")
	content := []byte("payload")
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatalf("write IPA file: %v", err)
	}

	info, err := ValidateIPAPath(path)
	if err != nil {
		t.Fatalf("ValidateIPAPath() error: %v", err)
	}
	if info.Size() != int64(len(content)) {
		t.Fatalf("expected size %d, got %d", len(content), info.Size())
	}
}

func TestValidateIPAPathRejectsEmptyFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "empty.ipa")
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatalf("write empty IPA: %v", err)
	}

	_, err := ValidateIPAPath(path)
	if err == nil || !strings.Contains(err.Error(), "--ipa must not be empty") {
		t.Fatalf("expected empty IPA rejection, got %v", err)
	}
}

func TestValidatePKGPathRejectsSymlinkAndEmptyFile(t *testing.T) {
	dir := t.TempDir()
	emptyPath := filepath.Join(dir, "empty.pkg")
	if err := os.WriteFile(emptyPath, nil, 0o600); err != nil {
		t.Fatalf("write empty PKG: %v", err)
	}
	if _, err := ValidatePKGPath(emptyPath); err == nil || !strings.Contains(err.Error(), "--pkg must not be empty") {
		t.Fatalf("expected empty PKG rejection, got %v", err)
	}

	targetPath := filepath.Join(dir, "target.pkg")
	if err := os.WriteFile(targetPath, []byte("payload"), 0o600); err != nil {
		t.Fatalf("write PKG target: %v", err)
	}
	linkPath := filepath.Join(dir, "link.pkg")
	if err := os.Symlink(targetPath, linkPath); err != nil {
		t.Fatalf("create PKG symlink: %v", err)
	}
	if _, err := ValidatePKGPath(linkPath); err == nil || !strings.Contains(err.Error(), "from --pkg") {
		t.Fatalf("expected PKG symlink rejection, got %v", err)
	}
}

func TestResolveBundleInfoForIPAUsesProvidedValues(t *testing.T) {
	ipaPath := writeTestIPA(t, map[string][]byte{
		"Payload/Demo.app/Info.plist": buildInfoPlist(t, "1.0", "1"),
	})
	info, err := ResolveBundleInfoForIPA(ipaPath, "1.2.3", "42")
	if err != nil {
		t.Fatalf("ResolveBundleInfoForIPA() error: %v", err)
	}
	if info.Version != "1.2.3" || info.BuildNumber != "42" {
		t.Fatalf("unexpected resolved values: version=%q build=%q", info.Version, info.BuildNumber)
	}
}

func TestResolveBundleInfoForIPAFillsMissingBuildNumber(t *testing.T) {
	plistData := buildInfoPlist(t, "1.2.3", "45")
	ipaPath := writeTestIPA(t, map[string][]byte{
		"Payload/Demo.app/Info.plist": plistData,
	})

	info, err := ResolveBundleInfoForIPA(ipaPath, "1.2.3", "")
	if err != nil {
		t.Fatalf("ResolveBundleInfoForIPA() error: %v", err)
	}
	if info.Version != "1.2.3" || info.BuildNumber != "45" {
		t.Fatalf("unexpected resolved values: version=%q build=%q", info.Version, info.BuildNumber)
	}
}

func TestResolveBundleInfoForIPAReportsMissingKeys(t *testing.T) {
	plistData := buildInfoPlistWithValues(t, "", "", plist.XMLFormat)
	ipaPath := writeTestIPA(t, map[string][]byte{
		"Payload/Demo.app/Info.plist": plistData,
	})

	_, err := ResolveBundleInfoForIPA(ipaPath, "", "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "missing Info.plist keys") {
		t.Fatalf("expected missing keys error, got %v", err)
	}
}

func writeTestIPA(t *testing.T, files map[string][]byte) string {
	t.Helper()

	ipaPath := filepath.Join(t.TempDir(), "app.ipa")
	file, err := os.Create(ipaPath)
	if err != nil {
		t.Fatalf("create IPA: %v", err)
	}
	defer file.Close()

	zipWriter := zip.NewWriter(file)
	for _, name := range slices.Sorted(maps.Keys(files)) {
		data := files[name]
		entry, err := zipWriter.Create(name)
		if err != nil {
			t.Fatalf("create zip entry %q: %v", name, err)
		}
		if _, err := entry.Write(data); err != nil {
			t.Fatalf("write zip entry %q: %v", name, err)
		}
	}
	if err := zipWriter.Close(); err != nil {
		t.Fatalf("close zip writer: %v", err)
	}

	return ipaPath
}

func buildInfoPlist(t *testing.T, version string, build string) []byte {
	t.Helper()

	return buildInfoPlistWithValues(t, version, build, plist.XMLFormat)
}

// buildInfoPlistOfSize returns a valid XML Info.plist whose serialized length
// is exactly totalSize, padded with a filler key so boundary tests can hit the
// documented limit precisely.
func buildInfoPlistOfSize(t *testing.T, totalSize int) []byte {
	t.Helper()

	padding := 0
	for range 8 {
		data, err := plist.Marshal(map[string]any{
			"CFBundleShortVersionString": "1.2.3",
			"CFBundleVersion":            "45",
			"Padding":                    strings.Repeat("a", padding),
		}, plist.XMLFormat)
		if err != nil {
			t.Fatalf("marshal plist: %v", err)
		}
		if len(data) == totalSize {
			return data
		}
		padding += totalSize - len(data)
		if padding < 0 {
			t.Fatalf("cannot build an Info.plist as small as %d bytes", totalSize)
		}
	}
	t.Fatalf("failed to build an Info.plist of exactly %d bytes", totalSize)
	return nil
}

func buildInfoPlistWithValues(t *testing.T, versionValue any, buildValue any, format int) []byte {
	t.Helper()

	payload := map[string]any{
		"CFBundleIdentifier":         "com.example.demo",
		"CFBundleShortVersionString": versionValue,
		"CFBundleVersion":            buildValue,
	}

	data, err := plist.Marshal(payload, format)
	if err != nil {
		t.Fatalf("marshal plist: %v", err)
	}
	return data
}
