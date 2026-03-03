package shared

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"path"
	"strings"

	"howett.net/plist"
)

type IPABundleInfo struct {
	Version     string
	BuildNumber string
}

// ExtractBundleInfoFromIPA reads CFBundleVersion info from an IPA.
func ExtractBundleInfoFromIPA(ipaPath string) (IPABundleInfo, error) {
	reader, err := zip.OpenReader(ipaPath)
	if err != nil {
		return IPABundleInfo{}, fmt.Errorf("open IPA: %w", err)
	}
	defer func() { _ = reader.Close() }()

	for _, file := range reader.File {
		if file.FileInfo().IsDir() {
			continue
		}
		if !isTopLevelAppInfoPlist(file.Name) {
			continue
		}
		return readBundleInfoFromInfoPlist(file)
	}

	// Keep the canonical casing for the filename, but follow Go error string style.
	return IPABundleInfo{}, fmt.Errorf("missing Info.plist in IPA")
}

func isTopLevelAppInfoPlist(name string) bool {
	cleaned := path.Clean(name)
	if !strings.HasPrefix(cleaned, "Payload/") || !strings.HasSuffix(cleaned, "/Info.plist") {
		return false
	}
	dir := path.Dir(cleaned)
	if !strings.HasSuffix(dir, ".app") {
		return false
	}
	return path.Dir(dir) == "Payload"
}

func readBundleInfoFromInfoPlist(file *zip.File) (IPABundleInfo, error) {
	reader, err := file.Open()
	if err != nil {
		return IPABundleInfo{}, fmt.Errorf("open Info.plist: %w", err)
	}
	defer func() { _ = reader.Close() }()

	data, err := io.ReadAll(reader)
	if err != nil {
		return IPABundleInfo{}, fmt.Errorf("read Info.plist: %w", err)
	}

	var info map[string]any
	decoder := plist.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(&info); err != nil {
		return IPABundleInfo{}, fmt.Errorf("decode Info.plist: %w", err)
	}

	return IPABundleInfo{
		Version:     coercePlistValueToString(info["CFBundleShortVersionString"]),
		BuildNumber: coercePlistValueToString(info["CFBundleVersion"]),
	}, nil
}

func coercePlistValueToString(value any) string {
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v)
	case []byte:
		return strings.TrimSpace(string(v))
	case int, int8, int16, int32, int64:
		return fmt.Sprint(v)
	case uint, uint8, uint16, uint32, uint64:
		return fmt.Sprint(v)
	case float32, float64:
		return strings.TrimSpace(fmt.Sprint(v))
	case fmt.Stringer:
		return strings.TrimSpace(v.String())
	default:
		return ""
	}
}
