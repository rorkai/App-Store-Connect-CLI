package telemetry

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	spoolFileName        = "telemetry-spool.jsonl"
	maxSpoolRecords      = 128
	maxSpoolBytes        = 256 * 1024
	maxSpoolRecordBytes  = 16 * 1024
	spoolLockTimeout     = 25 * time.Millisecond
	spoolReaderBufferLen = 4 * 1024
)

var errSpoolRecordTooLarge = errors.New("telemetry spool record exceeds limit")

type spoolRecord struct {
	Event    Event  `json:"event"`
	Endpoint string `json:"endpoint"`
}

type spoolStore struct {
	path           string
	maxRecords     int
	maxBytes       int
	maxRecordBytes int
	lockWait       time.Duration
}

func defaultSpoolStore() (spoolStore, error) {
	statePath, err := StatePath()
	if err != nil {
		return spoolStore{}, err
	}
	return spoolStore{
		path:           filepath.Join(filepath.Dir(statePath), spoolFileName),
		maxRecords:     maxSpoolRecords,
		maxBytes:       maxSpoolBytes,
		maxRecordBytes: maxSpoolRecordBytes,
		lockWait:       spoolLockTimeout,
	}, nil
}

func (store spoolStore) append(record spoolRecord) error {
	_, err := store.appendAndCount(record)
	return err
}

// appendAndCount appends the record and returns how many records the spool
// holds afterwards, so callers can gate follow-up work on the backlog size
// without re-reading the spool.
func (store spoolStore) appendAndCount(record spoolRecord) (int, error) {
	encoded, err := encodeSpoolRecord(record)
	if err != nil {
		return 0, fmt.Errorf("telemetry: failed to encode spool record: %w", err)
	}
	if len(encoded) > store.maxRecordBytes || len(encoded) > store.maxBytes {
		return 0, errSpoolRecordTooLarge
	}

	unlock, err := lockTelemetryFile(store.path, store.lockWait, "spool")
	if err != nil {
		return 0, err
	}
	defer unlock()
	if err := store.removeOrphanedTempFilesUnlocked(); err != nil {
		return 0, err
	}

	records, _, err := store.readUnlocked()
	if err != nil {
		return 0, err
	}
	records = append(records, record)
	records, err = store.trimToLimits(records)
	if err != nil {
		return 0, err
	}
	if err := store.writeUnlocked(records); err != nil {
		return 0, err
	}
	return len(records), nil
}

func (store spoolStore) snapshot(limit int) ([]spoolRecord, error) {
	unlock, err := lockTelemetryFile(store.path, store.lockWait, "spool")
	if err != nil {
		return nil, err
	}
	defer unlock()
	if err := store.removeOrphanedTempFilesUnlocked(); err != nil {
		return nil, err
	}

	records, dirty, err := store.readUnlocked()
	if err != nil {
		return nil, err
	}
	trimmed, err := store.trimToLimits(records)
	if err != nil {
		return nil, err
	}
	if len(trimmed) != len(records) {
		dirty = true
	}
	if dirty {
		if err := store.writeUnlocked(trimmed); err != nil {
			return nil, err
		}
	}
	if limit > 0 && len(trimmed) > limit {
		trimmed = trimmed[:limit]
	}
	return append([]spoolRecord(nil), trimmed...), nil
}

func (store spoolStore) removeDelivered(eventIDs map[string]struct{}) error {
	if len(eventIDs) == 0 {
		return nil
	}
	unlock, err := lockTelemetryFile(store.path, store.lockWait, "spool")
	if err != nil {
		return err
	}
	defer unlock()
	if err := store.removeOrphanedTempFilesUnlocked(); err != nil {
		return err
	}

	records, dirty, err := store.readUnlocked()
	if err != nil {
		return err
	}
	kept := records[:0]
	for _, record := range records {
		if _, delivered := eventIDs[record.Event.EventID]; delivered {
			dirty = true
			continue
		}
		kept = append(kept, record)
	}
	if !dirty {
		return nil
	}
	return store.writeUnlocked(kept)
}

func (store spoolStore) purge() error {
	hasArtifacts, err := store.hasArtifacts()
	if err != nil {
		return err
	}
	if !hasArtifacts {
		return nil
	}
	unlock, err := lockTelemetryFile(store.path, store.lockWait, "spool")
	if err != nil {
		return err
	}
	defer unlock()

	if err := os.Remove(store.path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("telemetry: failed to remove spool: %w", err)
	}
	if err := store.removeOrphanedTempFilesUnlocked(); err != nil {
		return err
	}
	if err := syncTelemetryDirectory(filepath.Dir(store.path)); err != nil {
		return fmt.Errorf("telemetry: failed to sync spool directory: %w", err)
	}
	return nil
}

func (store spoolStore) hasArtifacts() (bool, error) {
	entries, err := os.ReadDir(filepath.Dir(store.path))
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("telemetry: failed to inspect spool directory: %w", err)
	}
	base := filepath.Base(store.path)
	for _, entry := range entries {
		if entry.Name() == base || isSpoolTempName(base, entry.Name()) {
			return true, nil
		}
	}
	return false, nil
}

func (store spoolStore) removeOrphanedTempFilesUnlocked() error {
	dir := filepath.Dir(store.path)
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("telemetry: failed to inspect spool temp files: %w", err)
	}
	base := filepath.Base(store.path)
	for _, entry := range entries {
		if !isSpoolTempName(base, entry.Name()) || entry.IsDir() {
			continue
		}
		if err := os.Remove(filepath.Join(dir, entry.Name())); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("telemetry: failed to remove spool temp file: %w", err)
		}
	}
	return nil
}

func isSpoolTempName(base, name string) bool {
	return strings.HasPrefix(name, base+".") && strings.HasSuffix(name, ".tmp")
}

func purgeDefaultSpool() error {
	store, err := defaultSpoolStore()
	if err != nil {
		return err
	}
	return store.purge()
}

func (store spoolStore) readUnlocked() ([]spoolRecord, bool, error) {
	file, err := openTelemetryFileForRead(store.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("telemetry: failed to read spool: %w", err)
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return nil, false, fmt.Errorf("telemetry: failed to stat spool: %w", err)
	}
	if !info.Mode().IsRegular() {
		return nil, false, fmt.Errorf("telemetry: spool is not a regular file")
	}
	if err := os.Chmod(store.path, 0o600); err != nil {
		return nil, false, fmt.Errorf("telemetry: failed to secure spool: %w", err)
	}

	dirty := false
	maxRead := int64(store.maxBytes + store.maxRecordBytes)
	if maxRead <= 0 {
		maxRead = maxSpoolBytes + maxSpoolRecordBytes
	}
	start := info.Size() - maxRead
	if start > 0 {
		dirty = true
		var previous [1]byte
		if _, err := file.ReadAt(previous[:], start-1); err != nil {
			return nil, false, fmt.Errorf("telemetry: failed to seek spool: %w", err)
		}
		if _, err := file.Seek(start, io.SeekStart); err != nil {
			return nil, false, fmt.Errorf("telemetry: failed to seek spool: %w", err)
		}
		if previous[0] != '\n' {
			reader := bufio.NewReaderSize(file, spoolReaderBufferLen)
			if err := discardSpoolLine(reader); err != nil && !errors.Is(err, io.EOF) {
				return nil, false, fmt.Errorf("telemetry: failed to recover spool boundary: %w", err)
			}
			return store.decodeSpoolReader(reader, dirty)
		}
	}

	reader := bufio.NewReaderSize(file, spoolReaderBufferLen)
	return store.decodeSpoolReader(reader, dirty)
}

func (store spoolStore) decodeSpoolReader(reader *bufio.Reader, dirty bool) ([]spoolRecord, bool, error) {
	records := make([]spoolRecord, 0)
	for {
		line, oversized, readErr := readBoundedSpoolLine(reader, store.maxRecordBytes)
		if oversized {
			dirty = true
		} else if len(strings.TrimSpace(string(line))) > 0 {
			var record spoolRecord
			if err := json.Unmarshal(line, &record); err != nil || !validSpoolRecord(record) {
				dirty = true
			} else {
				records = append(records, record)
			}
		}
		if errors.Is(readErr, io.EOF) {
			if len(line) > 0 {
				dirty = true
			}
			break
		}
		if readErr != nil {
			return nil, false, fmt.Errorf("telemetry: failed to scan spool: %w", readErr)
		}
	}
	return records, dirty, nil
}

func readBoundedSpoolLine(reader *bufio.Reader, maxBytes int) ([]byte, bool, error) {
	if maxBytes <= 0 {
		maxBytes = maxSpoolRecordBytes
	}
	var line []byte
	oversized := false
	for {
		fragment, err := reader.ReadSlice('\n')
		if !oversized {
			if len(line)+len(fragment) > maxBytes {
				line = nil
				oversized = true
			} else {
				line = append(line, fragment...)
			}
		}
		if errors.Is(err, bufio.ErrBufferFull) {
			continue
		}
		return line, oversized, err
	}
}

func discardSpoolLine(reader *bufio.Reader) error {
	for {
		_, err := reader.ReadSlice('\n')
		if errors.Is(err, bufio.ErrBufferFull) {
			continue
		}
		return err
	}
}

func (store spoolStore) trimToLimits(records []spoolRecord) ([]spoolRecord, error) {
	sizes := make([]int, len(records))
	total := 0
	for index, record := range records {
		encoded, err := encodeSpoolRecord(record)
		if err != nil {
			return nil, fmt.Errorf("telemetry: failed to encode spool record: %w", err)
		}
		sizes[index] = len(encoded)
		total += len(encoded)
	}
	first := 0
	for first < len(records) && (len(records)-first > store.maxRecords || total > store.maxBytes) {
		total -= sizes[first]
		first++
	}
	return append([]spoolRecord(nil), records[first:]...), nil
}

func (store spoolStore) writeUnlocked(records []spoolRecord) error {
	dir := filepath.Dir(store.path)
	if err := ensureSecureTelemetryDirectory(dir); err != nil {
		return fmt.Errorf("telemetry: failed to create spool directory: %w", err)
	}
	if len(records) == 0 {
		if err := os.Remove(store.path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("telemetry: failed to remove empty spool: %w", err)
		}
		if err := syncTelemetryDirectory(dir); err != nil {
			return fmt.Errorf("telemetry: failed to sync spool directory: %w", err)
		}
		return nil
	}

	tmp, err := os.CreateTemp(dir, spoolFileName+".*.tmp")
	if err != nil {
		return fmt.Errorf("telemetry: failed to create temp spool: %w", err)
	}
	tmpPath := tmp.Name()
	defer func() { _ = os.Remove(tmpPath) }()
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("telemetry: failed to secure temp spool: %w", err)
	}
	for _, record := range records {
		encoded, err := encodeSpoolRecord(record)
		if err != nil {
			_ = tmp.Close()
			return fmt.Errorf("telemetry: failed to encode spool record: %w", err)
		}
		if _, err := tmp.Write(encoded); err != nil {
			_ = tmp.Close()
			return fmt.Errorf("telemetry: failed to write spool: %w", err)
		}
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("telemetry: failed to sync spool: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("telemetry: failed to close spool: %w", err)
	}
	if err := replaceTelemetryFile(tmpPath, store.path, store.lockWait); err != nil {
		return fmt.Errorf("telemetry: failed to replace spool: %w", err)
	}
	if err := syncTelemetryDirectory(dir); err != nil {
		return fmt.Errorf("telemetry: failed to sync spool directory: %w", err)
	}
	return nil
}

func encodeSpoolRecord(record spoolRecord) ([]byte, error) {
	data, err := json.Marshal(record)
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

func validSpoolRecord(record spoolRecord) bool {
	return strings.TrimSpace(record.Event.EventID) != "" &&
		record.Event.SchemaVersion != 0 &&
		strings.TrimSpace(record.Event.CommandPath) != "" &&
		strings.TrimSpace(record.Endpoint) != ""
}
