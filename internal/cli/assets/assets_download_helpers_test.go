package assets

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/rootfs"
)

type readerThatFailsAfterFirstRead struct {
	readOnce bool
}

func (r *readerThatFailsAfterFirstRead) Read(p []byte) (int, error) {
	if !r.readOnce {
		r.readOnce = true
		return copy(p, "NEW-DATA"), nil
	}
	return 0, errors.New("simulated read failure")
}

func TestWriteDownloadedFile_Overwrite_ErrorPreservesExistingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.bin")

	if err := os.WriteFile(path, []byte("OLD-DATA"), 0o600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	_, err := writeDownloadedFile(path, &readerThatFailsAfterFirstRead{}, true)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}

	data, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatalf("ReadFile() error: %v", readErr)
	}
	if string(data) != "OLD-DATA" {
		t.Fatalf("expected existing file contents preserved, got %q", string(data))
	}
}

func TestWriteDownloadedFile_Overwrite_ReplacesExistingFileOnSuccess(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.bin")

	if err := os.WriteFile(path, []byte("OLD-DATA"), 0o600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	written, err := writeDownloadedFile(path, strings.NewReader("NEW-DATA"), true)
	if err != nil {
		t.Fatalf("writeDownloadedFile() error: %v", err)
	}
	if written != int64(len("NEW-DATA")) {
		t.Fatalf("expected written=%d, got %d", len("NEW-DATA"), written)
	}

	data, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatalf("ReadFile() error: %v", readErr)
	}
	if string(data) != "NEW-DATA" {
		t.Fatalf("expected new file contents, got %q", string(data))
	}
}

func TestEquivalentPNGFiles(t *testing.T) {
	validHeader := downloadTestPNGIHDR()
	zeroWidthHeader := append([]byte(nil), validHeader...)
	binary.BigEndian.PutUint32(zeroWidthHeader[:4], 0)

	tests := []struct {
		name      string
		existing  []byte
		candidate []byte
		want      bool
	}{
		{
			name:      "byte identical non-PNG",
			existing:  []byte("same bytes"),
			candidate: []byte("same bytes"),
			want:      true,
		},
		{
			name:      "different non-PNG",
			existing:  []byte("first"),
			candidate: []byte("second"),
		},
		{
			name: "volatile metadata differs",
			existing: downloadTestPNG(
				downloadTestPNGTextChunk(t, "iTXt", "date:modify", "first"),
				downloadTestPNGIDAT(t, "same-pixels"),
			),
			candidate: downloadTestPNG(
				downloadTestPNGTextChunk(t, "tEXt", "date:modify", "second"),
				downloadTestPNGIDAT(t, "same-pixels"),
			),
			want: true,
		},
		{
			name: "Exif metadata differs",
			existing: downloadTestPNG(
				downloadTestPNGChunk("eXIf", []byte("orientation-first")),
				downloadTestPNGIDAT(t, "same-pixels"),
			),
			candidate: downloadTestPNG(
				downloadTestPNGChunk("eXIf", []byte("orientation-second")),
				downloadTestPNGIDAT(t, "same-pixels"),
			),
		},
		{
			name: "XMP orientation metadata differs",
			existing: downloadTestPNG(
				downloadTestPNGTextChunk(t, "iTXt", "XML:com.adobe.xmp", "orientation=1"),
				downloadTestPNGIDAT(t, "same-pixels"),
			),
			candidate: downloadTestPNG(
				downloadTestPNGTextChunk(t, "iTXt", "XML:com.adobe.xmp", "orientation=6"),
				downloadTestPNGIDAT(t, "same-pixels"),
			),
		},
		{
			name: "legacy Exif profile differs",
			existing: downloadTestPNG(
				downloadTestPNGTextChunk(t, "tEXt", "Raw profile type exif", "orientation=1"),
				downloadTestPNGIDAT(t, "same-pixels"),
			),
			candidate: downloadTestPNG(
				downloadTestPNGTextChunk(t, "zTXt", "Raw profile type exif", "orientation=6"),
				downloadTestPNGIDAT(t, "same-pixels"),
			),
		},
		{
			name: "pixel data differs",
			existing: downloadTestPNG(
				downloadTestPNGTextChunk(t, "iTXt", "date:modify", "first"),
				downloadTestPNGIDAT(t, "first-pixels"),
			),
			candidate: downloadTestPNG(
				downloadTestPNGTextChunk(t, "iTXt", "date:modify", "second"),
				downloadTestPNGIDAT(t, "second-pixels"),
			),
		},
		{
			name: "stable ancillary data differs",
			existing: downloadTestPNG(
				downloadTestPNGChunk("iCCP", []byte("first-profile")),
				downloadTestPNGIDAT(t, "same-pixels"),
			),
			candidate: downloadTestPNG(
				downloadTestPNGChunk("iCCP", []byte("second-profile")),
				downloadTestPNGIDAT(t, "same-pixels"),
			),
		},
		{
			name: "invalid CRC is not equivalent",
			existing: downloadTestPNG(
				downloadTestPNGTextChunk(t, "iTXt", "date:modify", "first"),
				downloadTestPNGIDAT(t, "same-pixels"),
			),
			candidate: corruptDownloadTestPNGCRC(downloadTestPNG(
				downloadTestPNGTextChunk(t, "iTXt", "date:modify", "second"),
				downloadTestPNGIDAT(t, "same-pixels"),
			)),
		},
		{
			name: "truncated PNG is not equivalent",
			existing: downloadTestPNG(
				downloadTestPNGTextChunk(t, "iTXt", "date:modify", "first"),
				downloadTestPNGIDAT(t, "same-pixels"),
			),
			candidate: truncateDownloadTestPNG(downloadTestPNG(
				downloadTestPNGTextChunk(t, "iTXt", "date:modify", "second"),
				downloadTestPNGIDAT(t, "same-pixels"),
			)),
		},
		{
			name: "zero width is not equivalent",
			existing: downloadTestPNGWithIHDR(
				zeroWidthHeader,
				downloadTestPNGTextChunk(t, "iTXt", "date:modify", "first"),
				downloadTestPNGIDAT(t, "same-pixels"),
			),
			candidate: downloadTestPNGWithIHDR(
				zeroWidthHeader,
				downloadTestPNGTextChunk(t, "tEXt", "date:modify", "second"),
				downloadTestPNGIDAT(t, "same-pixels"),
			),
		},
		{
			name: "malformed image data is not equivalent",
			existing: downloadTestPNG(
				downloadTestPNGTextChunk(t, "iTXt", "date:modify", "first"),
				downloadTestPNGChunk("IDAT", []byte("not-a-zlib-stream")),
			),
			candidate: downloadTestPNG(
				downloadTestPNGTextChunk(t, "tEXt", "date:modify", "second"),
				downloadTestPNGChunk("IDAT", []byte("not-a-zlib-stream")),
			),
		},
		{
			name: "reserved chunk bit is not equivalent",
			existing: downloadTestPNG(
				downloadTestPNGChunk("itxt", []byte("invalid-reserved-bit")),
				downloadTestPNGTextChunk(t, "iTXt", "date:modify", "first"),
				downloadTestPNGIDAT(t, "same-pixels"),
			),
			candidate: downloadTestPNG(
				downloadTestPNGChunk("itxt", []byte("invalid-reserved-bit")),
				downloadTestPNGTextChunk(t, "tEXt", "date:modify", "second"),
				downloadTestPNGIDAT(t, "same-pixels"),
			),
		},
		{
			name: "duplicate modification time is not equivalent",
			existing: downloadTestPNG(
				downloadTestPNGChunk("tIME", []byte{0x07, 0xea, 8, 19, 12, 30, 45}),
				downloadTestPNGChunk("tIME", []byte{0x07, 0xea, 8, 19, 12, 31, 45}),
				downloadTestPNGIDAT(t, "same-pixels"),
			),
			candidate: downloadTestPNG(
				downloadTestPNGChunk("tIME", []byte{0x07, 0xea, 8, 19, 12, 32, 45}),
				downloadTestPNGIDAT(t, "same-pixels"),
			),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := equivalentPNGBytes(tt.existing, tt.candidate)
			if got != tt.want {
				t.Fatalf("equivalentPNGBytes() = %t, want %t", got, tt.want)
			}
		})
	}
}

func TestVolatilePNGChunk(t *testing.T) {
	compressedITXt := append([]byte("date:modify\x00"), 1, 0, 0, 0)
	compressedITXt = append(compressedITXt, downloadTestPNGCompressedScanlines(t, []byte("second"))...)

	tests := []struct {
		name      string
		chunkType string
		data      []byte
		want      bool
	}{
		{name: "text modification timestamp", chunkType: "tEXt", data: downloadTestPNGTextData(t, "tEXt", "date:modify", "first"), want: true},
		{name: "compressed creation timestamp", chunkType: "zTXt", data: downloadTestPNGTextData(t, "zTXt", "date:create", "first"), want: true},
		{name: "international creation time", chunkType: "iTXt", data: downloadTestPNGTextData(t, "iTXt", "Creation Time", "first"), want: true},
		{name: "compressed international timestamp", chunkType: "iTXt", data: compressedITXt, want: true},
		{name: "valid modification time", chunkType: "tIME", data: []byte{0x07, 0xea, 8, 19, 12, 30, 45}, want: true},
		{name: "XMP profile", chunkType: "iTXt", data: downloadTestPNGTextData(t, "iTXt", "XML:com.adobe.xmp", "orientation=6")},
		{name: "legacy Exif profile", chunkType: "zTXt", data: downloadTestPNGTextData(t, "zTXt", "Raw profile type exif", "orientation=6")},
		{name: "comment", chunkType: "tEXt", data: downloadTestPNGTextData(t, "tEXt", "Comment", "keep me")},
		{name: "missing keyword terminator", chunkType: "tEXt", data: []byte("date:modify")},
		{name: "invalid compressed text", chunkType: "zTXt", data: []byte("date:modify\x00\x00not-zlib")},
		{name: "invalid date", chunkType: "tIME", data: []byte{0x07, 0xea, 2, 30, 12, 30, 45}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := volatilePNGChunk(tt.chunkType, tt.data); got != tt.want {
				t.Fatalf("volatilePNGChunk() = %t, want %t", got, tt.want)
			}
		})
	}
}

func TestValidPNGIHDRRejectsInvalidFields(t *testing.T) {
	tests := []struct {
		name   string
		mutate func([]byte)
	}{
		{name: "zero width", mutate: func(header []byte) { binary.BigEndian.PutUint32(header[:4], 0) }},
		{name: "zero height", mutate: func(header []byte) { binary.BigEndian.PutUint32(header[4:8], 0) }},
		{name: "oversized width", mutate: func(header []byte) { binary.BigEndian.PutUint32(header[:4], 0x80000000) }},
		{name: "oversized height", mutate: func(header []byte) { binary.BigEndian.PutUint32(header[4:8], 0x80000000) }},
		{name: "invalid bit depth", mutate: func(header []byte) { header[8] = 4 }},
		{name: "invalid color type", mutate: func(header []byte) { header[9] = 1 }},
		{name: "invalid compression", mutate: func(header []byte) { header[10] = 1 }},
		{name: "invalid filter", mutate: func(header []byte) { header[11] = 1 }},
		{name: "invalid interlace", mutate: func(header []byte) { header[12] = 2 }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			header := downloadTestPNGIHDR()
			tt.mutate(header)
			if validPNGIHDR(header) {
				t.Fatal("validPNGIHDR() = true, want false")
			}
		})
	}
}

func TestValidPNGImageData(t *testing.T) {
	header := pngImageHeader{width: 1, height: 1, bitDepth: 8, colorType: 6}
	valid := downloadTestPNGCompressedScanlines(t, []byte{0, 1, 2, 3, 4})
	invalidFilter := downloadTestPNGCompressedScanlines(t, []byte{5, 1, 2, 3, 4})
	shortRow := downloadTestPNGCompressedScanlines(t, []byte{0, 1, 2, 3})
	extraRowData := downloadTestPNGCompressedScanlines(t, []byte{0, 1, 2, 3, 4, 5})
	invalidChecksum := append([]byte(nil), valid...)
	invalidChecksum[len(invalidChecksum)-1] ^= 0xff
	trailingData := append(append([]byte(nil), valid...), 0)

	tests := []struct {
		name   string
		header pngImageHeader
		parts  [][]byte
		want   bool
	}{
		{name: "valid", header: header, parts: [][]byte{valid}, want: true},
		{name: "split stream", header: header, parts: [][]byte{valid[:2], valid[2:]}, want: true},
		{name: "interlaced one pixel", header: pngImageHeader{width: 1, height: 1, bitDepth: 8, colorType: 6, interlace: 1}, parts: [][]byte{valid}, want: true},
		{name: "invalid filter", header: header, parts: [][]byte{invalidFilter}},
		{name: "short row", header: header, parts: [][]byte{shortRow}},
		{name: "extra row data", header: header, parts: [][]byte{extraRowData}},
		{name: "invalid checksum", header: header, parts: [][]byte{invalidChecksum}},
		{name: "trailing compressed data", header: header, parts: [][]byte{trailingData}},
		{name: "inflated data limit", header: pngImageHeader{width: 0x7fffffff, height: 0x7fffffff, bitDepth: 16, colorType: 6}, parts: [][]byte{valid}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := validPNGImageData(tt.header, tt.parts); got != tt.want {
				t.Fatalf("validPNGImageData() = %t, want %t", got, tt.want)
			}
		})
	}
}

func TestStablePNGDigestValidatesPalette(t *testing.T) {
	indexedHeader := downloadTestPNGIHDR()
	indexedHeader[8] = 1
	indexedHeader[9] = 3
	grayscaleHeader := downloadTestPNGIHDR()
	grayscaleHeader[9] = 0
	imageData := downloadTestPNGChunk("IDAT", downloadTestPNGCompressedScanlines(t, []byte{0, 0}))
	outOfRangeImageData := downloadTestPNGChunk("IDAT", downloadTestPNGCompressedScanlines(t, []byte{0, 0x80}))

	tests := []struct {
		name string
		png  []byte
		want bool
	}{
		{name: "valid indexed palette", png: downloadTestPNGWithIHDR(indexedHeader, downloadTestPNGChunk("PLTE", []byte{0, 0, 0}), imageData), want: true},
		{name: "indexed pixel exceeds palette", png: downloadTestPNGWithIHDR(indexedHeader, downloadTestPNGChunk("PLTE", []byte{0, 0, 0}), outOfRangeImageData)},
		{name: "indexed palette missing", png: downloadTestPNGWithIHDR(indexedHeader, imageData)},
		{name: "indexed palette exceeds bit depth", png: downloadTestPNGWithIHDR(indexedHeader, downloadTestPNGChunk("PLTE", make([]byte, 9)), imageData)},
		{name: "grayscale palette forbidden", png: downloadTestPNGWithIHDR(grayscaleHeader, downloadTestPNGChunk("PLTE", []byte{0, 0, 0}), imageData)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, got := stablePNGDigest(tt.png)
			if got != tt.want {
				t.Fatalf("stablePNGDigest() valid = %t, want %t", got, tt.want)
			}
		})
	}
}

func TestUnfilterPNGPaletteRow(t *testing.T) {
	tests := []struct {
		name     string
		filter   byte
		row      []byte
		previous []byte
		want     []byte
		valid    bool
	}{
		{name: "none", filter: 0, row: []byte{1, 2}, previous: []byte{2, 4}, want: []byte{1, 2}, valid: true},
		{name: "sub", filter: 1, row: []byte{1, 1}, previous: []byte{2, 4}, want: []byte{1, 2}, valid: true},
		{name: "up", filter: 2, row: []byte{255, 254}, previous: []byte{2, 4}, want: []byte{1, 2}, valid: true},
		{name: "average", filter: 3, row: []byte{2, 5}, previous: []byte{2, 4}, want: []byte{3, 8}, valid: true},
		{name: "paeth", filter: 4, row: []byte{1, 4}, previous: []byte{2, 4}, want: []byte{3, 8}, valid: true},
		{name: "invalid", filter: 5, row: []byte{1, 2}, previous: []byte{2, 4}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotValid := unfilterPNGPaletteRow(tt.row, tt.previous, tt.filter)
			if gotValid != tt.valid {
				t.Fatalf("unfilterPNGPaletteRow() valid = %t, want %t", gotValid, tt.valid)
			}
			if gotValid && !bytes.Equal(tt.row, tt.want) {
				t.Fatalf("unfilterPNGPaletteRow() = %v, want %v", tt.row, tt.want)
			}
		})
	}
}

func TestValidPNGPaletteIndices(t *testing.T) {
	tests := []struct {
		name           string
		row            []byte
		width          uint32
		bitDepth       byte
		paletteEntries uint32
		want           bool
	}{
		{name: "one bit valid", row: []byte{0x40}, width: 2, bitDepth: 1, paletteEntries: 2, want: true},
		{name: "one bit invalid", row: []byte{0x40}, width: 2, bitDepth: 1, paletteEntries: 1},
		{name: "two bit valid", row: []byte{0x1b}, width: 4, bitDepth: 2, paletteEntries: 4, want: true},
		{name: "two bit invalid", row: []byte{0x1b}, width: 4, bitDepth: 2, paletteEntries: 3},
		{name: "four bit valid", row: []byte{0x1f}, width: 2, bitDepth: 4, paletteEntries: 16, want: true},
		{name: "four bit invalid", row: []byte{0x1f}, width: 2, bitDepth: 4, paletteEntries: 15},
		{name: "eight bit valid", row: []byte{0, 2}, width: 2, bitDepth: 8, paletteEntries: 3, want: true},
		{name: "eight bit invalid", row: []byte{0, 2}, width: 2, bitDepth: 8, paletteEntries: 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := validPNGPaletteIndices(tt.row, tt.width, tt.bitDepth, tt.paletteEntries); got != tt.want {
				t.Fatalf("validPNGPaletteIndices() = %t, want %t", got, tt.want)
			}
		})
	}
}

func TestWriteScreenshotDownloadReplacesChangedPixels(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "screenshot.png")
	existing := downloadTestPNG(downloadTestPNGIDAT(t, "old-pixels"))
	candidate := downloadTestPNG(downloadTestPNGIDAT(t, "new-pixels"))
	if err := os.WriteFile(path, existing, 0o600); err != nil {
		t.Fatalf("write existing screenshot: %v", err)
	}

	written, unchanged, err := writeScreenshotDownload(path, strings.NewReader(string(candidate)))
	if err != nil {
		t.Fatalf("writeScreenshotDownload() error: %v", err)
	}
	if unchanged {
		t.Fatal("writeScreenshotDownload() marked changed pixels as unchanged")
	}
	if written != int64(len(candidate)) {
		t.Fatalf("writeScreenshotDownload() wrote %d bytes, want %d", written, len(candidate))
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read replaced screenshot: %v", err)
	}
	if string(got) != string(candidate) {
		t.Fatal("writeScreenshotDownload() did not replace changed pixels")
	}
}

func TestSameFileSnapshotRejectsInPlaceRewrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "screenshot.png")
	original := []byte("original")
	changed := []byte("modified")
	if err := os.WriteFile(path, original, 0o600); err != nil {
		t.Fatalf("write original screenshot: %v", err)
	}

	initial, err := os.Open(path)
	if err != nil {
		t.Fatalf("open original screenshot: %v", err)
	}
	initialInfo, err := initial.Stat()
	if err != nil {
		_ = initial.Close()
		t.Fatalf("stat original screenshot: %v", err)
	}
	if err := initial.Close(); err != nil {
		t.Fatalf("close original screenshot: %v", err)
	}

	if err := os.WriteFile(path, changed, 0o600); err != nil {
		t.Fatalf("rewrite screenshot in place: %v", err)
	}
	current, err := os.Open(path)
	if err != nil {
		t.Fatalf("reopen rewritten screenshot: %v", err)
	}
	defer current.Close()
	currentInfo, err := current.Stat()
	if err != nil {
		t.Fatalf("stat rewritten screenshot: %v", err)
	}
	if !os.SameFile(initialInfo, currentInfo) {
		t.Fatal("in-place rewrite unexpectedly replaced the file identity")
	}

	if sameFileSnapshot(current, initialInfo, original) {
		t.Fatal("sameFileSnapshot() = true after an in-place content rewrite")
	}
}

func TestSameRootedFileSnapshotRejectsAtomicReplacement(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not permit this open-file replacement fixture")
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "screenshot.png")
	original := []byte("original")
	changed := []byte("modified")
	if err := os.WriteFile(path, original, 0o600); err != nil {
		t.Fatalf("write original screenshot: %v", err)
	}
	current, err := os.Open(path)
	if err != nil {
		t.Fatalf("open original screenshot: %v", err)
	}
	defer current.Close()
	initialInfo, err := current.Stat()
	if err != nil {
		t.Fatalf("stat original screenshot: %v", err)
	}

	replacement := filepath.Join(dir, "replacement.png")
	if err := os.WriteFile(replacement, changed, 0o600); err != nil {
		t.Fatalf("write replacement screenshot: %v", err)
	}
	if err := os.Rename(replacement, path); err != nil {
		t.Fatalf("replace screenshot atomically: %v", err)
	}
	if !sameFileSnapshot(current, initialInfo, original) {
		t.Fatal("sameFileSnapshot() did not retain the expected open-descriptor snapshot")
	}

	root, err := rootfs.New(dir)
	if err != nil {
		t.Fatalf("open rooted screenshot directory: %v", err)
	}
	defer root.Close()
	if sameRootedFileSnapshot(root, filepath.Base(path), initialInfo, original) {
		t.Fatal("sameRootedFileSnapshot() = true after an atomic pathname replacement")
	}
}

func TestWriteScreenshotDownloadReplacesUnreadableExistingFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows file permissions do not provide a portable unreadable-file fixture")
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "screenshot.png")
	if err := os.WriteFile(path, []byte("old screenshot"), 0o600); err != nil {
		t.Fatalf("write existing screenshot: %v", err)
	}
	if err := os.Chmod(path, 0); err != nil {
		t.Fatalf("make existing screenshot unreadable: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(path, 0o600) })

	written, unchanged, err := writeScreenshotDownload(path, strings.NewReader("new screenshot"))
	if err != nil {
		t.Fatalf("writeScreenshotDownload() error: %v", err)
	}
	if unchanged {
		t.Fatal("writeScreenshotDownload() marked unreadable destination as unchanged")
	}
	if written != int64(len("new screenshot")) {
		t.Fatalf("writeScreenshotDownload() wrote %d bytes, want %d", written, len("new screenshot"))
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read replaced screenshot: %v", err)
	}
	if string(got) != "new screenshot" {
		t.Fatalf("replaced screenshot = %q, want %q", got, "new screenshot")
	}
}

func TestWriteScreenshotDownloadFailurePreservesExistingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "screenshot.png")
	existing := []byte("existing screenshot")
	if err := os.WriteFile(path, existing, 0o600); err != nil {
		t.Fatalf("write existing screenshot: %v", err)
	}

	_, _, err := writeScreenshotDownload(path, &readerThatFailsAfterFirstRead{})
	if err == nil {
		t.Fatal("writeScreenshotDownload() error = nil, want staged read failure")
	}
	got, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatalf("read preserved screenshot: %v", readErr)
	}
	if string(got) != string(existing) {
		t.Fatalf("existing screenshot = %q, want %q", got, existing)
	}
	entries, readDirErr := os.ReadDir(dir)
	if readDirErr != nil {
		t.Fatalf("read output directory: %v", readDirErr)
	}
	if len(entries) != 1 || entries[0].Name() != filepath.Base(path) {
		t.Fatalf("output directory entries = %v, want only %q", entries, filepath.Base(path))
	}
}

func downloadTestPNG(chunks ...[]byte) []byte {
	return downloadTestPNGWithIHDR(downloadTestPNGIHDR(), chunks...)
}

func downloadTestPNGWithIHDR(header []byte, chunks ...[]byte) []byte {
	png := append([]byte(nil), pngSignature...)
	png = append(png, downloadTestPNGChunk("IHDR", header)...)
	for _, chunk := range chunks {
		png = append(png, chunk...)
	}
	png = append(png, downloadTestPNGChunk("IEND", nil)...)
	return png
}

func downloadTestPNGIHDR() []byte {
	header := make([]byte, 13)
	binary.BigEndian.PutUint32(header[:4], 1)
	binary.BigEndian.PutUint32(header[4:8], 1)
	header[8] = 8
	header[9] = 6
	return header
}

func downloadTestPNGIDAT(t *testing.T, pixels string) []byte {
	t.Helper()

	row := make([]byte, 5)
	binary.BigEndian.PutUint32(row[1:], crc32.ChecksumIEEE([]byte(pixels)))
	return downloadTestPNGChunk("IDAT", downloadTestPNGCompressedScanlines(t, row))
}

func downloadTestPNGCompressedScanlines(t *testing.T, scanlines []byte) []byte {
	t.Helper()

	var compressed bytes.Buffer
	compressor := zlib.NewWriter(&compressed)
	if _, err := compressor.Write(scanlines); err != nil {
		t.Fatalf("compress PNG test scanlines: %v", err)
	}
	if err := compressor.Close(); err != nil {
		t.Fatalf("close PNG test compressor: %v", err)
	}
	return compressed.Bytes()
}

func downloadTestPNGTextChunk(t *testing.T, chunkType, keyword, text string) []byte {
	t.Helper()
	return downloadTestPNGChunk(chunkType, downloadTestPNGTextData(t, chunkType, keyword, text))
}

func downloadTestPNGTextData(t *testing.T, chunkType, keyword, text string) []byte {
	t.Helper()

	data := append([]byte(keyword), 0)
	switch chunkType {
	case "tEXt":
		data = append(data, text...)
	case "zTXt":
		data = append(data, 0)
		data = append(data, downloadTestPNGCompressedScanlines(t, []byte(text))...)
	case "iTXt":
		data = append(data, 0, 0, 0, 0)
		data = append(data, text...)
	default:
		t.Fatalf("unsupported PNG text chunk type %q", chunkType)
	}
	return data
}

func downloadTestPNGChunk(chunkType string, data []byte) []byte {
	chunk := make([]byte, 12+len(data))
	binary.BigEndian.PutUint32(chunk[:4], uint32(len(data)))
	copy(chunk[4:8], chunkType)
	copy(chunk[8:8+len(data)], data)
	binary.BigEndian.PutUint32(chunk[8+len(data):], crc32.ChecksumIEEE(chunk[4:8+len(data)]))
	return chunk
}

func corruptDownloadTestPNGCRC(png []byte) []byte {
	corrupt := append([]byte(nil), png...)
	corrupt[len(corrupt)-1] ^= 0xff
	return corrupt
}

func truncateDownloadTestPNG(png []byte) []byte {
	return append([]byte(nil), png[:len(png)-3]...)
}
