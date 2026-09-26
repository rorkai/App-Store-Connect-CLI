package artifacts

import (
	"encoding/binary"
	"fmt"
	"io"
)

const maxZIPDirectoryBytes = 16 << 20

// Check the directory before archive/zip allocates its file list. Its allocation
// is based on the untrusted record count and the size of the entire archive.
func validateZIPDirectory(source io.ReaderAt, size int64) error {
	if size < 22 {
		return fmt.Errorf("truncated ZIP directory")
	}
	tailSize := min(size, 22+65535)
	tail := make([]byte, int(tailSize))
	if _, err := source.ReadAt(tail, size-tailSize); err != nil {
		return fmt.Errorf("read ZIP directory: %w", err)
	}
	for index := len(tail) - 22; index >= 0; index-- {
		if binary.LittleEndian.Uint32(tail[index:]) != 0x06054b50 {
			continue
		}
		end := tail[index:]
		if int(binary.LittleEndian.Uint16(end[20:22])) > len(end)-22 {
			continue
		}
		records := uint64(binary.LittleEndian.Uint16(end[10:12]))
		directorySize := uint64(binary.LittleEndian.Uint32(end[12:16]))
		directoryOffset := uint64(binary.LittleEndian.Uint32(end[16:20]))
		endOffset := size - tailSize + int64(index)
		if records == 0xffff || directorySize == 0xffff || directorySize == 0xffffffff || directoryOffset == 0xffffffff {
			locator := make([]byte, 20)
			if endOffset < 20 {
				return fmt.Errorf("truncated ZIP64 directory locator")
			}
			if _, err := source.ReadAt(locator, endOffset-20); err != nil {
				return fmt.Errorf("read ZIP64 directory locator: %w", err)
			}
			if binary.LittleEndian.Uint32(locator) == 0x07064b50 {
				if binary.LittleEndian.Uint32(locator[4:8]) != 0 || binary.LittleEndian.Uint32(locator[16:20]) != 1 {
					return fmt.Errorf("unsupported multi-disk ZIP64 directory")
				}
				offset := binary.LittleEndian.Uint64(locator[8:16])
				if size < 56 || offset > uint64(size-56) {
					return fmt.Errorf("ZIP64 directory offset is outside the archive")
				}
				zip64 := make([]byte, 56)
				if _, err := source.ReadAt(zip64, int64(offset)); err != nil {
					return fmt.Errorf("read ZIP64 directory: %w", err)
				}
				if binary.LittleEndian.Uint32(zip64) != 0x06064b50 {
					return fmt.Errorf("invalid ZIP64 directory")
				}
				records = binary.LittleEndian.Uint64(zip64[32:40])
				directorySize = binary.LittleEndian.Uint64(zip64[40:48])
				directoryOffset = binary.LittleEndian.Uint64(zip64[48:56])
				endOffset = int64(offset)
			}
		}
		if records > maxZipEntries {
			return fmt.Errorf("IPA contains %d entries; limit is %d", records, maxZipEntries)
		}
		if directorySize > maxZIPDirectoryBytes {
			return fmt.Errorf("ZIP directory exceeds the metadata read limit")
		}
		if directorySize > uint64(endOffset) || directoryOffset > uint64(size) {
			return fmt.Errorf("ZIP directory offset is outside the archive")
		}
		start := endOffset - int64(directorySize)
		if err := countZIPDirectory(source, size, start); err != nil {
			return err
		}
		// archive/zip can prefer the raw offset over the prefix-adjusted start
		// after probing its first header. Bound both possible allocation paths.
		if directoryOffset < uint64(start) {
			if err := countZIPDirectory(source, size, int64(directoryOffset)); err != nil {
				return err
			}
		}
		return nil
	}
	return fmt.Errorf("missing ZIP directory")
}

func countZIPDirectory(source io.ReaderAt, size, start int64) error {
	var header [46]byte
	for offset, count := start, 0; offset < size; {
		n, err := source.ReadAt(header[:], offset)
		if err != nil && err != io.EOF {
			return fmt.Errorf("read ZIP directory header: %w", err)
		}
		if n < 4 || binary.LittleEndian.Uint32(header[:4]) != 0x02014b50 {
			return nil
		}
		if n < len(header) {
			return fmt.Errorf("truncated ZIP directory header")
		}
		count++
		if count > maxZipEntries {
			return fmt.Errorf("IPA contains more than %d entries", maxZipEntries)
		}
		length := int64(len(header)) + int64(binary.LittleEndian.Uint16(header[28:30])) +
			int64(binary.LittleEndian.Uint16(header[30:32])) + int64(binary.LittleEndian.Uint16(header[32:34]))
		if length > size-offset {
			return fmt.Errorf("truncated ZIP directory entry")
		}
		if length > maxZIPDirectoryBytes-(offset-start) {
			return fmt.Errorf("ZIP directory exceeds the metadata read limit")
		}
		offset += length
	}
	return nil
}

// The advertised directory size can lie. Bound actual directory reads too,
// until zip.NewReader has finished parsing it; member reads have their own caps.
type zipDirectoryReader struct {
	io.ReaderAt
	remaining int64
}

func (reader *zipDirectoryReader) ReadAt(data []byte, offset int64) (int, error) {
	if reader.remaining >= 0 {
		if int64(len(data)) > reader.remaining {
			return 0, fmt.Errorf("ZIP directory exceeds the metadata read limit")
		}
		reader.remaining -= int64(len(data))
	}
	return reader.ReaderAt.ReadAt(data, offset)
}
