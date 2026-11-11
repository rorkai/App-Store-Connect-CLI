package asc

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

// ErrAppStoreImageAlpha reports a PNG that stores an alpha channel or
// transparency, which App Store screenshots and creative images must not have.
var ErrAppStoreImageAlpha = errors.New("image has an alpha channel or transparency; re-export it as RGB PNG or JPEG without alpha")

// ReadAppStoreImageFormatFrom checks the encoded format and PNG alpha metadata
// for screenshot and creative-image uploads. It does not decode pixel buffers.
// The source must be positioned at its beginning; its position is advanced.
func ReadAppStoreImageFormatFrom(source io.ReadSeeker) (string, error) {
	format, err := ReadImageFormatFrom(source)
	if err != nil {
		return "", err
	}
	if format != "png" {
		return format, nil
	}
	if _, err := source.Seek(0, io.SeekStart); err != nil {
		return "", err
	}
	// Bound metadata work even when the file declares oversized ancillary chunks.
	metadata := &io.LimitedReader{R: source, N: 16 << 20}
	var signature [8]byte
	if _, err := io.ReadFull(metadata, signature[:]); err != nil {
		return "", err
	}
	for {
		if metadata.N < 8 {
			return "", fmt.Errorf("PNG metadata exceeds 16 MiB limit")
		}
		var header [8]byte
		if _, err := io.ReadFull(metadata, header[:]); err != nil {
			return "", fmt.Errorf("read PNG metadata: %w", err)
		}
		length := binary.BigEndian.Uint32(header[:4])
		if length > 0x7fffffff {
			return "", fmt.Errorf("invalid PNG chunk length %d", length)
		}
		chunkType := string(header[4:])
		if chunkType != "IDAT" && chunkType != "IEND" && int64(length)+4 > metadata.N {
			return "", fmt.Errorf("PNG metadata exceeds 16 MiB limit")
		}
		switch chunkType {
		case "IHDR":
			if length != 13 {
				return "", fmt.Errorf("invalid PNG header length %d", length)
			}
			var ihdr [13]byte
			if _, err := io.ReadFull(metadata, ihdr[:]); err != nil {
				return "", err
			}
			if ihdr[9] == 4 || ihdr[9] == 6 {
				return "", ErrAppStoreImageAlpha
			}
			length = 0
		case "tRNS":
			return "", ErrAppStoreImageAlpha
		case "IDAT", "IEND":
			return format, nil
		}
		if _, err := io.CopyN(io.Discard, metadata, int64(length)+4); err != nil {
			return "", fmt.Errorf("read PNG metadata: %w", err)
		}
	}
}
