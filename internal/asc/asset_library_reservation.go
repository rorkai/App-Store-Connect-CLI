package asc

import (
	"context"
	"fmt"
)

type assetLibraryUploadJournalKey struct{}

// AssetLibraryUploadCheckpoint distinguishes an intended reservation from an
// acknowledged transfer of the caller's source bytes.
type AssetLibraryUploadCheckpoint string

const (
	AssetLibraryCheckpointReserved    AssetLibraryUploadCheckpoint = "reserved"
	AssetLibraryCheckpointTransferred AssetLibraryUploadCheckpoint = "transferred"
)

// WithAssetLibraryUploadJournal records reservation before transfer and records
// successful transfer before commit. The callback must durably save each record
// before returning nil. An error stops the next mutation and retains the asset
// ID in the partial result. Upload credentials must never enter the journal.
func WithAssetLibraryUploadJournal(ctx context.Context, record func(AssetLibraryUploadCheckpoint, string, string) error) context.Context {
	return context.WithValue(ctx, assetLibraryUploadJournalKey{}, record)
}

func recordAssetLibraryCheckpoint(ctx context.Context, phase AssetLibraryUploadCheckpoint, id, resourceType string) error {
	record, _ := ctx.Value(assetLibraryUploadJournalKey{}).(func(AssetLibraryUploadCheckpoint, string, string) error)
	if record == nil {
		return nil
	}
	if err := record(phase, id, resourceType); err != nil {
		return fmt.Errorf("record %s asset %s: %w", phase, id, err)
	}
	return nil
}
