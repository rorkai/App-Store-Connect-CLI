package artifacts

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type artifactReadBudget struct {
	io.ReaderAt
	remaining int
}

func (reader *artifactReadBudget) ReadAt(data []byte, offset int64) (int, error) {
	if len(data) > reader.remaining {
		return 0, fmt.Errorf("artifact payload was read")
	}
	reader.remaining -= len(data)
	return reader.ReaderAt.ReadAt(data, offset)
}

func TestInspectLargeArtifactsReadOnlyMetadata(t *testing.T) {
	for _, kind := range []string{"ipa", "pkg"} {
		t.Run(kind, func(t *testing.T) {
			file, err := os.Create(filepath.Join(t.TempDir(), "large."+kind))
			if err != nil {
				t.Fatal(err)
			}
			defer file.Close()
			const offset = 5 << 30
			if kind == "ipa" {
				if _, err := file.Seek(offset, io.SeekStart); err != nil {
					t.Fatal(err)
				}
				writer := zip.NewWriter(file)
				writer.SetOffset(offset)
				entry, err := writer.Create("Payload/Large.app/Info.plist")
				if err != nil {
					t.Fatal(err)
				}
				if _, err := entry.Write(plistXML(t, map[string]any{"CFBundleIdentifier": "com.example.large"})); err != nil {
					t.Fatal(err)
				}
				if err := writer.Close(); err != nil {
					t.Fatal(err)
				}
			} else {
				data := writeXar(t, map[string][]byte{"PackageInfo": []byte(`<pkg-info identifier="com.example.large"/>`)})
				if _, err := file.Write(data); err != nil {
					t.Fatal(err)
				}
				if err := file.Truncate(offset); err != nil {
					t.Fatal(err)
				}
			}
			stat, err := file.Stat()
			if err != nil {
				t.Fatal(err)
			}
			reader := &artifactReadBudget{ReaderAt: file, remaining: 256 << 10}
			if kind == "ipa" {
				manifest, err := InspectIPA(reader, stat.Size(), false, false)
				if err != nil || manifest.BundleID != "com.example.large" || manifest.Status != "unsigned" {
					t.Fatalf("manifest=%+v err=%v", manifest, err)
				}
			} else {
				manifest, err := InspectPKG(reader, stat.Size())
				if err != nil || manifest.ProductID != "com.example.large" || manifest.Status != "readable" {
					t.Fatalf("manifest=%+v err=%v", manifest, err)
				}
			}
		})
	}
}

func TestInspectIPARejectsExcessiveDirectoryBeforeAllocation(t *testing.T) {
	for _, field := range []string{"entries", "bytes"} {
		t.Run(field, func(t *testing.T) {
			data := storedZip(t, map[string][]byte{"Payload/Demo.app/Info.plist": []byte("unused")})
			end := data[len(data)-22:]
			if field == "entries" {
				binary.LittleEndian.PutUint16(end[10:12], maxZipEntries+1)
			} else {
				binary.LittleEndian.PutUint32(end[12:16], maxZIPDirectoryBytes+1)
			}
			manifest, err := InspectIPA(bytes.NewReader(data), int64(len(data)), false, false)
			if err == nil || !strings.Contains(err.Error(), "limit") || manifest.Status != "unreadable" {
				t.Fatalf("manifest=%+v err=%v", manifest, err)
			}
		})
	}
}

func TestInspectPKGRejectsFileTruncatedAfterStat(t *testing.T) {
	data := writeXar(t, map[string][]byte{"PackageInfo": []byte(`<pkg-info identifier="com.example.large"/>`)})
	manifest, err := InspectPKG(bytes.NewReader(data[:len(data)-1]), int64(len(data)))
	if err == nil || manifest.Status != "unreadable" {
		t.Fatalf("manifest=%+v err=%v", manifest, err)
	}
}

func TestZIPDirectoryBoundsZIP64CountWithLegacySizeMarker(t *testing.T) {
	data := make([]byte, 56+20+22)
	binary.LittleEndian.PutUint32(data, 0x06064b50)
	binary.LittleEndian.PutUint64(data[32:40], maxZipEntries+1)
	locator := data[56:76]
	binary.LittleEndian.PutUint32(locator, 0x07064b50)
	binary.LittleEndian.PutUint32(locator[16:20], 1)
	end := data[76:]
	binary.LittleEndian.PutUint32(end, 0x06054b50)
	binary.LittleEndian.PutUint16(end[10:12], 1)
	// archive/zip also recognizes this legacy 16-bit directory-size marker.
	binary.LittleEndian.PutUint32(end[12:16], 0xffff)
	if err := validateZIPDirectory(bytes.NewReader(data), int64(len(data))); err == nil || !strings.Contains(err.Error(), "entries") {
		t.Fatalf("err=%v", err)
	}
}

func TestInspectIPABoundsDirectoryWithFalseSize(t *testing.T) {
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	for index := 0; index < 280; index++ {
		if _, err := writer.Create(fmt.Sprintf("%s%d", strings.Repeat("x", 65500), index)); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	data := buffer.Bytes()
	end := data[len(data)-22:]
	binary.LittleEndian.PutUint32(end[12:16], 1)
	manifest, err := InspectIPA(bytes.NewReader(data), int64(len(data)), false, false)
	if err == nil || !strings.Contains(err.Error(), "metadata read limit") || manifest.Status != "unreadable" {
		t.Fatalf("manifest=%+v err=%v", manifest, err)
	}
}

func TestZIPDirectoryRejectsActualCountAboveLimit(t *testing.T) {
	// A false count of one matches 65,537 modulo 65,536 in archive/zip.
	const count = 65537
	data := make([]byte, count*46+22)
	for offset := 0; offset < count*46; offset += 46 {
		binary.LittleEndian.PutUint32(data[offset:], 0x02014b50)
	}
	end := data[count*46:]
	binary.LittleEndian.PutUint32(end, 0x06054b50)
	binary.LittleEndian.PutUint16(end[10:12], 1)
	for _, directorySize := range []uint32{count * 46, 46} {
		// The smaller size makes the physical start point at only the last
		// record; archive/zip's raw-offset fallback still sees every record.
		binary.LittleEndian.PutUint32(end[12:16], directorySize)
		if err := validateZIPDirectory(bytes.NewReader(data), int64(len(data))); err == nil || !strings.Contains(err.Error(), "entries") {
			t.Fatalf("directorySize=%d err=%v", directorySize, err)
		}
	}
}
