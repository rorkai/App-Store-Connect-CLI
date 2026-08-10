package routingcoverage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/rootfs"
)

// PreparedRoutingCoverageFile is a validated routing coverage upload source.
type PreparedRoutingCoverageFile struct {
	Path     string
	FileName string
	FileSize int64
	Checksum string

	rootPath     string
	relativePath string
}

type routingCoverageGeoJSON struct {
	Type        string           `json:"type"`
	Coordinates [][][][]*float64 `json:"coordinates"`
}

var errRoutingCoverageSourceChanged = errors.New("routing coverage source changed while reading")

// PrepareRoutingCoverageFile validates and fingerprints a routing coverage file.
func PrepareRoutingCoverageFile(path string) (PreparedRoutingCoverageFile, error) {
	return prepareRoutingCoverageFile(path, asc.ComputeChecksumFromReader)
}

func prepareRoutingCoverageFile(path string, checksumFunc func(io.Reader, asc.ChecksumAlgorithm) (*asc.Checksum, error)) (PreparedRoutingCoverageFile, error) {
	rootPath, relativePath, absolutePath, err := resolveRoutingCoverageSource(path)
	if err != nil {
		return PreparedRoutingCoverageFile{}, err
	}
	fileName := filepath.Base(absolutePath)
	if !strings.EqualFold(filepath.Ext(fileName), ".geojson") {
		return PreparedRoutingCoverageFile{}, fmt.Errorf("file must use the .geojson extension: %q", absolutePath)
	}
	root, err := rootfs.New(rootPath)
	if err != nil {
		return PreparedRoutingCoverageFile{}, err
	}
	file, err := root.OpenFile(relativePath)
	if err != nil {
		return PreparedRoutingCoverageFile{}, err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return PreparedRoutingCoverageFile{}, fmt.Errorf("stat file: %w", err)
	}
	if info.Size() <= 0 {
		return PreparedRoutingCoverageFile{}, fmt.Errorf("file size must be greater than 0")
	}
	snapshot, err := snapshotOpenedRoutingCoverageSource(file, info.Size())
	if err != nil {
		if errors.Is(err, errRoutingCoverageSourceChanged) {
			return PreparedRoutingCoverageFile{}, fmt.Errorf("file changed while reading: %q", absolutePath)
		}
		return PreparedRoutingCoverageFile{}, err
	}
	defer discardRoutingCoverageSnapshot(snapshot)
	if err := validateRoutingCoverageGeoJSON(io.NewSectionReader(snapshot, 0, info.Size())); err != nil {
		return PreparedRoutingCoverageFile{}, fmt.Errorf("invalid routing coverage GeoJSON: %w", err)
	}

	checksum, err := checksumFunc(io.NewSectionReader(snapshot, 0, info.Size()), asc.ChecksumAlgorithmMD5)
	if err != nil {
		return PreparedRoutingCoverageFile{}, fmt.Errorf("checksum failed: %w", err)
	}
	return PreparedRoutingCoverageFile{
		Path:         absolutePath,
		FileName:     fileName,
		FileSize:     info.Size(),
		Checksum:     checksum.Hash,
		rootPath:     root.Path(),
		relativePath: relativePath,
	}, nil
}

func resolveRoutingCoverageSource(path string) (string, string, string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", "", "", fmt.Errorf("file is required")
	}

	absolutePath, err := filepath.Abs(path)
	if err != nil {
		return "", "", "", fmt.Errorf("resolve file path: %w", err)
	}
	workingDir, err := os.Getwd()
	if err != nil {
		return "", "", "", fmt.Errorf("resolve current directory: %w", err)
	}
	rootPath, err := routingCoverageSourceRoot(workingDir, absolutePath)
	if err != nil {
		return "", "", "", err
	}
	root, err := rootfs.New(rootPath)
	if err != nil {
		return "", "", "", err
	}
	relativePath, err := filepath.Rel(root.Path(), absolutePath)
	if err != nil {
		return "", "", "", fmt.Errorf("resolve file relative to trusted root: %w", err)
	}
	resolvedPath, err := root.Resolve(relativePath)
	if err != nil {
		return "", "", "", err
	}
	return root.Path(), relativePath, resolvedPath, nil
}

func routingCoverageSourceRoot(workingDir, absolutePath string) (string, error) {
	rootPath, err := filepath.Abs(workingDir)
	if err != nil {
		return "", fmt.Errorf("resolve current directory: %w", err)
	}
	absolutePath = filepath.Clean(absolutePath)
	volumeRoot := filepath.Clean(filepath.VolumeName(absolutePath) + string(filepath.Separator))
	for {
		if routingCoveragePathWithinRoot(rootPath, absolutePath) {
			return routingCoverageTrustedRoot(rootPath, volumeRoot, absolutePath)
		}
		parent := filepath.Dir(rootPath)
		if parent == rootPath {
			break
		}
		rootPath = parent
	}

	if !routingCoveragePathWithinRoot(volumeRoot, absolutePath) {
		return "", fmt.Errorf("resolve trusted root for file %q", absolutePath)
	}
	return routingCoverageTrustedRoot(volumeRoot, volumeRoot, absolutePath)
}

func routingCoverageTrustedRoot(commonRoot, volumeRoot, absolutePath string) (string, error) {
	commonRoot = filepath.Clean(commonRoot)
	if commonRoot != volumeRoot {
		return commonRoot, nil
	}

	relativePath, err := filepath.Rel(volumeRoot, absolutePath)
	if err != nil {
		return "", fmt.Errorf("resolve file relative to volume root: %w", err)
	}
	firstSeparator := strings.IndexRune(relativePath, filepath.Separator)
	if firstSeparator < 0 {
		return volumeRoot, nil
	}
	topLevelRoot := filepath.Join(volumeRoot, relativePath[:firstSeparator])
	if runtime.GOOS == "darwin" {
		// macOS exposes protected volume-root aliases such as /tmp and /var.
		// Trust only that immediate alias as the root; nested and final symlinks
		// remain below it and are still rejected by rootfs.OpenFile.
		volumeInfo, volumeErr := os.Stat(volumeRoot)
		aliasInfo, aliasErr := os.Lstat(topLevelRoot)
		targetInfo, targetErr := os.Stat(topLevelRoot)
		if volumeErr == nil && volumeInfo.IsDir() && volumeInfo.Mode().Perm()&0o022 == 0 &&
			aliasErr == nil && aliasInfo.Mode()&os.ModeSymlink != 0 &&
			targetErr == nil && targetInfo.IsDir() {
			return topLevelRoot, nil
		}
	}
	return volumeRoot, nil
}

func routingCoveragePathWithinRoot(rootPath, path string) bool {
	relative, err := filepath.Rel(rootPath, path)
	if err != nil || filepath.IsAbs(relative) {
		return false
	}
	return relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func validateRoutingCoverageGeoJSON(reader io.Reader) error {
	decoder := json.NewDecoder(reader)
	var document routingCoverageGeoJSON
	if err := decoder.Decode(&document); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return fmt.Errorf("invalid JSON: multiple top-level values")
		}
		return fmt.Errorf("invalid JSON: %w", err)
	}
	if document.Type != "MultiPolygon" {
		return fmt.Errorf("top-level type must be MultiPolygon")
	}
	if len(document.Coordinates) == 0 {
		return fmt.Errorf("MultiPolygon must contain at least one Polygon")
	}
	for polygonIndex, polygon := range document.Coordinates {
		if len(polygon) == 0 {
			return fmt.Errorf("polygon %d must contain at least one linear ring", polygonIndex)
		}
		for ringIndex, ring := range polygon {
			if len(ring) < 4 {
				return fmt.Errorf("polygon %d ring %d must contain at least four coordinate points", polygonIndex, ringIndex)
			}
			for pointIndex, point := range ring {
				if len(point) < 2 {
					return fmt.Errorf("polygon %d ring %d point %d must contain longitude and latitude", polygonIndex, ringIndex, pointIndex)
				}
				for componentIndex, component := range point {
					if component == nil {
						return fmt.Errorf("polygon %d ring %d point %d coordinate component %d must be a number", polygonIndex, ringIndex, pointIndex, componentIndex)
					}
				}
				longitude := *point[0]
				if longitude < -180 || longitude > 180 {
					return fmt.Errorf("polygon %d ring %d point %d longitude must be between -180 and 180 (got %g)", polygonIndex, ringIndex, pointIndex, longitude)
				}
				latitude := *point[1]
				if latitude < -90 || latitude > 90 {
					return fmt.Errorf("polygon %d ring %d point %d latitude must be between -90 and 90 (got %g)", polygonIndex, ringIndex, pointIndex, latitude)
				}
			}
			if !equalCoordinates(ring[0], ring[len(ring)-1]) {
				return fmt.Errorf("polygon %d ring %d start and end coordinate points must be the same", polygonIndex, ringIndex)
			}
		}
	}
	return nil
}

func equalCoordinates(left, right []*float64) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if *left[i] != *right[i] {
			return false
		}
	}
	return true
}

func (file PreparedRoutingCoverageFile) openSource() (*os.File, error) {
	rootPath := file.rootPath
	relativePath := file.relativePath
	if rootPath == "" || relativePath == "" {
		var err error
		rootPath, relativePath, _, err = resolveRoutingCoverageSource(file.Path)
		if err != nil {
			return nil, err
		}
	}
	root, err := rootfs.New(rootPath)
	if err != nil {
		return nil, err
	}
	return root.OpenFile(relativePath)
}

func checksumOpenedFile(file *os.File, size int64) (*asc.Checksum, error) {
	return asc.ComputeChecksumFromReader(io.NewSectionReader(file, 0, size), asc.ChecksumAlgorithmMD5)
}

func snapshotOpenedRoutingCoverageSource(source *os.File, expectedSize int64) (*os.File, error) {
	snapshot, err := os.CreateTemp("", "asc-routing-coverage-*.geojson")
	if err != nil {
		return nil, fmt.Errorf("create upload snapshot: %w", err)
	}
	snapshotPath := snapshot.Name()
	_ = os.Remove(snapshotPath)
	cleanup := func() {
		_ = snapshot.Close()
		_ = os.Remove(snapshotPath)
	}

	if expectedSize < 0 {
		cleanup()
		return nil, errRoutingCoverageSourceChanged
	}
	if _, err := io.CopyN(snapshot, source, expectedSize); err != nil {
		cleanup()
		if errors.Is(err, io.EOF) {
			return nil, errRoutingCoverageSourceChanged
		}
		return nil, fmt.Errorf("snapshot file: %w", err)
	}
	if _, err := io.CopyN(io.Discard, source, 1); err == nil {
		cleanup()
		return nil, errRoutingCoverageSourceChanged
	} else if !errors.Is(err, io.EOF) {
		cleanup()
		return nil, fmt.Errorf("check snapshot size: %w", err)
	}
	return snapshot, nil
}

func discardRoutingCoverageSnapshot(snapshot *os.File) {
	if snapshot == nil {
		return
	}
	path := snapshot.Name()
	_ = snapshot.Close()
	_ = os.Remove(path)
}

func verifyPreparedRoutingCoverageSource(source *os.File, file PreparedRoutingCoverageFile) error {
	return verifyPreparedRoutingCoverageSourceWithChecksum(source, file, checksumOpenedFile)
}

func verifyPreparedRoutingCoverageSourceWithChecksum(
	source *os.File,
	file PreparedRoutingCoverageFile,
	checksumFunc func(*os.File, int64) (*asc.Checksum, error),
) error {
	info, err := source.Stat()
	if err != nil {
		return fmt.Errorf("stat file: %w", err)
	}
	if info.Size() != file.FileSize {
		return fmt.Errorf("file changed after validation: %q", file.Path)
	}
	currentChecksum, err := checksumFunc(source, info.Size())
	if err != nil {
		return fmt.Errorf("checksum failed: %w", err)
	}
	postChecksumInfo, err := source.Stat()
	if err != nil {
		return fmt.Errorf("stat file after checksum: %w", err)
	}
	if postChecksumInfo.Size() != info.Size() {
		return fmt.Errorf("file changed after validation: %q", file.Path)
	}
	var trailing [1]byte
	read, readErr := source.ReadAt(trailing[:], info.Size())
	if read > 0 || readErr == nil {
		return fmt.Errorf("file changed after validation: %q", file.Path)
	}
	if !errors.Is(readErr, io.EOF) {
		return fmt.Errorf("check file size after checksum: %w", readErr)
	}
	if !strings.EqualFold(strings.TrimSpace(currentChecksum.Hash), strings.TrimSpace(file.Checksum)) {
		return fmt.Errorf("file changed after validation: %q", file.Path)
	}
	return nil
}

// RevalidatePreparedRoutingCoverageFile verifies that the prepared source has
// not changed since it was initially validated.
func RevalidatePreparedRoutingCoverageFile(file PreparedRoutingCoverageFile) error {
	source, err := file.openSource()
	if err != nil {
		return fmt.Errorf("open file: %w", err)
	}
	defer source.Close()
	return verifyPreparedRoutingCoverageSource(source, file)
}

func snapshotPreparedRoutingCoverageFile(file PreparedRoutingCoverageFile) (*os.File, error) {
	source, err := file.openSource()
	if err != nil {
		return nil, fmt.Errorf("open file: %w", err)
	}
	defer source.Close()

	snapshot, err := snapshotOpenedRoutingCoverageSource(source, file.FileSize)
	if err != nil {
		if errors.Is(err, errRoutingCoverageSourceChanged) {
			return nil, fmt.Errorf("file changed after validation: %q", file.Path)
		}
		return nil, err
	}
	if err := verifyPreparedRoutingCoverageSource(snapshot, file); err != nil {
		discardRoutingCoverageSnapshot(snapshot)
		return nil, err
	}
	return snapshot, nil
}

// UploadPreparedRoutingCoverageFile creates, uploads, and commits routing coverage.
// A post-create error returns the reservation response alongside the error when
// cleanup cannot remove it, so callers can report the retained reservation ID.
func UploadPreparedRoutingCoverageFile(ctx context.Context, client *asc.Client, versionID string, file PreparedRoutingCoverageFile) (*asc.RoutingAppCoverageResponse, error) {
	return uploadPreparedRoutingCoverageFile(ctx, client, versionID, "", file)
}

// ReplaceRoutingCoverageWithPreparedFile revalidates the upload source before
// deleting the current routing coverage, then creates, uploads, and commits its
// replacement from the same open source handle. A post-create error returns the
// replacement response alongside the error when cleanup cannot remove it.
func ReplaceRoutingCoverageWithPreparedFile(ctx context.Context, client *asc.Client, versionID, currentCoverageID string, file PreparedRoutingCoverageFile) (*asc.RoutingAppCoverageResponse, error) {
	currentCoverageID = strings.TrimSpace(currentCoverageID)
	if currentCoverageID == "" {
		return nil, fmt.Errorf("current routing coverage ID is required")
	}
	return uploadPreparedRoutingCoverageFile(ctx, client, versionID, currentCoverageID, file)
}

func uploadPreparedRoutingCoverageFile(ctx context.Context, client *asc.Client, versionID, currentCoverageID string, file PreparedRoutingCoverageFile) (*asc.RoutingAppCoverageResponse, error) {
	if client == nil {
		return nil, fmt.Errorf("client is required")
	}
	versionID = strings.TrimSpace(versionID)
	if versionID == "" {
		return nil, fmt.Errorf("version ID is required")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	source, err := snapshotPreparedRoutingCoverageFile(file)
	if err != nil {
		return nil, err
	}
	defer discardRoutingCoverageSnapshot(source)
	if currentCoverageID != "" {
		deleteCtx, deleteCancel := shared.ContextWithTimeout(ctx)
		deleteErr := client.DeleteRoutingAppCoverage(deleteCtx, currentCoverageID)
		deleteCancel()
		if deleteErr != nil {
			return nil, fmt.Errorf("delete current routing coverage %s: %w", currentCoverageID, deleteErr)
		}
	}

	requestCtx, cancel := shared.ContextWithTimeout(ctx)
	response, err := client.CreateRoutingAppCoverage(requestCtx, versionID, file.FileName, file.FileSize)
	cancel()
	if err != nil {
		return nil, fmt.Errorf("failed to create: %w", err)
	}
	if response == nil {
		return nil, fmt.Errorf("created routing coverage response is empty")
	}
	coverageID := strings.TrimSpace(response.Data.ID)
	if coverageID == "" {
		return nil, fmt.Errorf("created routing coverage response is missing an ID")
	}
	cleanupFailure := func(original error) (*asc.RoutingAppCoverageResponse, error) {
		cleanupCtx, cleanupCancel := shared.ContextWithTimeout(context.WithoutCancel(ctx))
		cleanupErr := client.DeleteRoutingAppCoverage(cleanupCtx, coverageID)
		cleanupCancel()
		if cleanupErr != nil && !asc.IsNotFound(cleanupErr) {
			return response, fmt.Errorf("%w (also failed to delete routing coverage reservation %s: %w)", original, coverageID, cleanupErr)
		}
		return nil, original
	}
	if len(response.Data.Attributes.UploadOperations) == 0 {
		return cleanupFailure(fmt.Errorf("no upload operations returned"))
	}

	uploadCtx, uploadCancel := shared.ContextWithUploadTimeout(ctx)
	err = asc.ExecuteUploadOperationsFromFile(uploadCtx, source, response.Data.Attributes.UploadOperations)
	uploadCancel()
	if err != nil {
		return cleanupFailure(fmt.Errorf("upload failed: %w", err))
	}

	uploadedInfo, err := source.Stat()
	if err != nil {
		return cleanupFailure(fmt.Errorf("stat file: %w", err))
	}
	if uploadedInfo.Size() != file.FileSize {
		return cleanupFailure(fmt.Errorf("file changed during upload: %q", file.Path))
	}
	uploadedChecksum, err := checksumOpenedFile(source, uploadedInfo.Size())
	if err != nil {
		return cleanupFailure(fmt.Errorf("checksum failed: %w", err))
	}
	if !strings.EqualFold(strings.TrimSpace(uploadedChecksum.Hash), strings.TrimSpace(file.Checksum)) {
		return cleanupFailure(fmt.Errorf("file changed during upload: %q", file.Path))
	}

	uploaded := true
	attributes := asc.RoutingAppCoverageUpdateAttributes{
		SourceFileChecksum: &file.Checksum,
		Uploaded:           &uploaded,
	}
	commitCtx, commitCancel := shared.ContextWithUploadTimeout(ctx)
	committed, err := client.UpdateRoutingAppCoverage(commitCtx, coverageID, attributes)
	commitCancel()
	if err != nil {
		return response, fmt.Errorf("failed to commit upload: %w", err)
	}
	if committed == nil || strings.TrimSpace(committed.Data.ID) == "" {
		return response, fmt.Errorf("committed routing coverage response is missing an ID")
	}
	return committed, nil
}
