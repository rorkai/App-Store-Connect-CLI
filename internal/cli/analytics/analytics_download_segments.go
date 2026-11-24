package analytics

import (
	"compress/gzip"
	"context"
	"crypto/md5"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"hash"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/rootfs"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/secureopen"
)

func validateAnalyticsSegmentDownloadFlags(d *shared.Deps, fs *flag.FlagSet, allSegments bool, outputDir string) error {
	provided := make(map[string]bool)
	fs.Visit(func(f *flag.Flag) { provided[f.Name] = true })
	if !allSegments {
		if provided["output-dir"] {
			return d.UsageError("--output-dir requires --all-segments")
		}
		return nil
	}
	for _, name := range []string{"segment-id", "output"} {
		if provided[name] {
			return shared.WithDiagnostic(d.UsageErrorf("--all-segments cannot be combined with --%s", name), shared.DiagnosticConflictingInput, "--"+name)
		}
	}
	if outputDir == "" {
		return d.RequiredFlag("--output-dir")
	}
	return nil
}

func analyticsDownloadSegments(ctx context.Context, d *shared.Deps, client *asc.Client, instanceID, segmentID string) ([]asc.Resource[asc.AnalyticsReportSegmentAttributes], error) {
	segments, err := fetchAnalyticsReportSegmentsUntilMatch(ctx, d, client, instanceID, segmentID)
	if asc.IsNotFound(err) {
		return nil, analyticsNotFoundError(fmt.Errorf("instance %q not found", instanceID), "--instance-id")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to fetch segments: %w", err)
	}
	if len(segments) == 0 {
		return nil, analyticsNotReadyError(fmt.Errorf("no segments available for instance %q", instanceID))
	}
	return segments, nil
}

func downloadAnalyticsInstance(ctx context.Context, d *shared.Deps, requestID, instanceID, outputDir string, decompress bool) (_ *asc.AnalyticsReportInstanceDownloadResult, resultErr error) {
	outputDir = filepath.Clean(outputDir)
	absolute, err := d.Abs(outputDir)
	if err != nil {
		return nil, fmt.Errorf("resolve output directory: %w", err)
	}
	parentPath, destination := filepath.Dir(absolute), filepath.Base(absolute)
	if absolute == parentPath {
		return nil, analyticsOutputDirExists(outputDir)
	}
	root, err := rootfs.New(parentPath)
	if err != nil {
		return nil, shared.ClassifyLocalFileError(fmt.Errorf("open output directory parent: %w", err), "--output-dir", "")
	}
	defer root.Close()
	parent, err := root.OpenRoot()
	if err != nil {
		if _, statErr := os.Lstat(parentPath); errors.Is(statErr, os.ErrNotExist) {
			missing := fmt.Errorf("parent directory of --output-dir does not exist: %q", filepath.Dir(outputDir))
			return nil, shared.ClassifyLocalFileError(shared.NewErrorWithCause(missing, statErr), "--output-dir", "")
		}
		return nil, shared.ClassifyLocalFileError(fmt.Errorf("open output directory parent: %w", err), "--output-dir", "")
	}
	defer parent.Close()
	if _, err := parent.Lstat(destination); err == nil {
		return nil, analyticsOutputDirExists(outputDir)
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("inspect output directory %q: %w", outputDir, err)
	}

	client, err := d.Client()
	if err != nil {
		return nil, err
	}
	segments, err := analyticsDownloadSegments(ctx, d, client, instanceID, "")
	if err != nil {
		return nil, err
	}
	if err := validateAnalyticsInstanceSegments(segments); err != nil {
		return nil, err
	}

	stagingName := ".asc-analytics-" + rand.Text() + ".tmp"
	if err := parent.Mkdir(stagingName, 0o700); err != nil {
		return nil, fmt.Errorf("create staging directory: %w", err)
	}
	stagingInfo, err := parent.Lstat(stagingName)
	if err != nil {
		return nil, fmt.Errorf("inspect staging directory: %w", err)
	}
	if !stagingInfo.IsDir() {
		return nil, errors.New("staging directory changed during creation")
	}
	published := false
	defer func() {
		if published {
			return
		}
		err := checkAnalyticsStagingDirectory(parent, stagingName, stagingInfo)
		if err == nil {
			err = parent.RemoveAll(stagingName)
		}
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			resultErr = errors.Join(resultErr, fmt.Errorf("remove staging directory %q: %w", filepath.Join(filepath.Dir(outputDir), stagingName), err))
		}
	}()
	staging, err := parent.OpenRoot(stagingName)
	if err != nil {
		return nil, fmt.Errorf("open staging directory: %w", err)
	}
	defer staging.Close()
	opened, err := staging.Stat(".")
	if err != nil {
		return nil, fmt.Errorf("inspect opened staging directory: %w", err)
	}
	if !os.SameFile(stagingInfo, opened) {
		return nil, errors.New("staging directory changed during creation")
	}

	result := &asc.AnalyticsReportInstanceDownloadResult{
		RequestID: requestID, InstanceID: instanceID, OutputDir: outputDir,
		Segments: make([]asc.AnalyticsReportSegmentDownload, 0, len(segments)),
	}
	for i, segment := range segments {
		name := fmt.Sprintf("segment-%03d.txt", i+1)
		file, err := stageAnalyticsSegment(ctx, d, client, staging, outputDir, name, segment, decompress)
		if err != nil {
			return nil, fmt.Errorf("segment %q: %w", segment.ID, err)
		}
		result.Segments = append(result.Segments, file)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := staging.Close(); err != nil {
		return nil, fmt.Errorf("close staging directory: %w", err)
	}
	anchored, err := root.ContainsAnchoredPath(parentPath, parent)
	if err != nil {
		return nil, fmt.Errorf("verify output directory parent: %w", err)
	}
	if !anchored {
		return nil, errors.New("output directory parent changed during download")
	}
	if err := checkAnalyticsStagingDirectory(parent, stagingName, stagingInfo); err != nil {
		return nil, err
	}
	if err := renameNoReplaceInRoot(parent, stagingName, destination); err != nil {
		if errors.Is(err, os.ErrExist) {
			return nil, analyticsOutputDirExists(outputDir)
		}
		if errors.Is(err, secureopen.ErrRenameNoReplaceUnsupported) {
			unsupported := fmt.Errorf("publish output directory %q: this filesystem has no atomic no-replace rename, which --all-segments needs; nothing was saved", outputDir)
			return nil, &shared.HintedError{
				Err:  shared.NewErrorWithCause(unsupported, err),
				Hint: "Choose an --output-dir on a local filesystem, or download one segment at a time with --segment-id SEGMENT_ID.",
			}
		}
		return nil, fmt.Errorf("publish output directory %q: %w", outputDir, err)
	}
	published = true
	return result, nil
}

var renameNoReplaceInRoot = secureopen.RenameNoReplaceInRoot

func checkAnalyticsStagingDirectory(parent *os.Root, name string, expected os.FileInfo) error {
	current, err := parent.Lstat(name)
	if err != nil {
		return fmt.Errorf("inspect staging directory: %w", err)
	}
	if !os.SameFile(expected, current) {
		return errors.New("staging directory changed during download")
	}
	return nil
}

func analyticsOutputDirExists(path string) error {
	err := shared.NewValidationError(fmt.Errorf("output directory already exists: %q", path))
	return shared.WithDiagnostic(err, shared.DiagnosticResourceConflict, "--output-dir")
}

func validateAnalyticsInstanceSegments(segments []asc.Resource[asc.AnalyticsReportSegmentAttributes]) error {
	seen := make(map[string]bool, len(segments))
	for _, segment := range segments {
		id, attributes := segment.ID, segment.Attributes
		if strings.TrimSpace(id) == "" {
			return errors.New("segment ID is empty")
		}
		if seen[id] {
			return fmt.Errorf("segment %q is listed more than once", id)
		}
		seen[id] = true
		if strings.TrimSpace(attributes.URL) == "" {
			return analyticsNotReadyError(fmt.Errorf("segment %q download URL is empty", id))
		}
		if attributes.SizeInBytes < 0 {
			return fmt.Errorf("segment %q has an invalid size", id)
		}
		if expected := strings.TrimSpace(attributes.Checksum); expected != "" {
			decoded, err := hex.DecodeString(expected)
			if err != nil || len(decoded) != md5.Size {
				return fmt.Errorf("segment %q has an invalid MD5 checksum", id)
			}
		}
	}
	return nil
}

func stageAnalyticsSegment(ctx context.Context, d *shared.Deps, client *asc.Client, staging *os.Root, outputDir, name string, segment asc.Resource[asc.AnalyticsReportSegmentAttributes], decompress bool) (asc.AnalyticsReportSegmentDownload, error) {
	compressed := name + ".gz"
	expected := strings.TrimSpace(segment.Attributes.Checksum)
	var checksum hash.Hash
	if expected != "" {
		checksum = md5.New()
	}
	size, err := streamAnalyticsReport(ctx, d, client, strings.TrimSpace(segment.Attributes.URL), func(reader io.Reader) (int64, error) {
		if checksum != nil {
			reader = io.TeeReader(reader, checksum)
		}
		return writeStagedAnalyticsFile(staging, compressed, reader)
	})
	if err != nil {
		return asc.AnalyticsReportSegmentDownload{}, err
	}
	if expectedSize := segment.Attributes.SizeInBytes; expectedSize > 0 && size != expectedSize {
		return asc.AnalyticsReportSegmentDownload{}, fmt.Errorf("size mismatch: downloaded %d bytes, expected %d", size, expectedSize)
	}
	if checksum != nil && !strings.EqualFold(hex.EncodeToString(checksum.Sum(nil)), expected) {
		return asc.AnalyticsReportSegmentDownload{}, errors.New("MD5 checksum mismatch")
	}
	result := asc.AnalyticsReportSegmentDownload{SegmentID: segment.ID, FilePath: filepath.Join(outputDir, compressed), FileSize: size}
	if !decompress {
		return result, nil
	}
	source, err := secureopen.OpenExistingNoFollowInRoot(staging, compressed)
	if err != nil {
		return asc.AnalyticsReportSegmentDownload{}, fmt.Errorf("open gzip file: %w", err)
	}
	defer source.Close()
	gzipReader, err := gzip.NewReader(source)
	if err != nil {
		return asc.AnalyticsReportSegmentDownload{}, fmt.Errorf("decompress file: %w", err)
	}
	defer gzipReader.Close()
	result.DecompressedSize, err = writeStagedAnalyticsFile(staging, name, gzipReader)
	if err != nil {
		return asc.AnalyticsReportSegmentDownload{}, fmt.Errorf("decompress file: %w", err)
	}
	result.DecompressedPath = filepath.Join(outputDir, name)
	return result, nil
}

func writeStagedAnalyticsFile(staging *os.Root, name string, reader io.Reader) (int64, error) {
	file, err := secureopen.OpenNewPrivateFileNoFollowInRoot(staging, name, 0o600)
	if err != nil {
		return 0, err
	}
	size, err := io.Copy(file, reader)
	if err == nil {
		err = file.Sync()
	}
	return size, errors.Join(err, file.Close())
}
