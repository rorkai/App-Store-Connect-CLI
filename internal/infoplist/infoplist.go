// Package infoplist bounds how much of an untrusted IPA's Info.plist the CLI
// expands. The archive controls a ZIP member's advertised uncompressed size, so
// IPA metadata readers check both that declared size and the bytes actually
// streamed against MaxBytes.
package infoplist

import (
	"fmt"
	"io"
)

// MaxBytes is the largest uncompressed size accepted for the top-level
// Payload/*.app/Info.plist member of an IPA. Even unusually large app
// Info.plists stay in the low hundreds of kilobytes, so 4 MiB leaves ample
// headroom while capping what a crafted archive can make the CLI allocate.
// There is deliberately no flag or environment override.
const MaxBytes = 4 << 20

// CheckDeclaredSize rejects an Info.plist whose ZIP metadata already advertises
// more than MaxBytes, so an oversized member is refused before it is opened or
// decompressed.
func CheckDeclaredSize(uncompressedSize uint64) error {
	if uncompressedSize > MaxBytes {
		return fmt.Errorf("declared uncompressed size %d bytes exceeds the %d byte Info.plist limit", uncompressedSize, MaxBytes)
	}
	return nil
}

// ReadBounded expands at most MaxBytes from reader and fails if more bytes are
// available, so forged or absent ZIP size metadata cannot bypass
// CheckDeclaredSize.
func ReadBounded(reader io.Reader) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(reader, MaxBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > MaxBytes {
		return nil, fmt.Errorf("expanded contents exceed the %d byte Info.plist limit", MaxBytes)
	}
	return data, nil
}
