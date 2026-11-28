package assetlibrary

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/rootfs"
)

type creativeJournal struct {
	root     rootfs.Root
	name     string
	identity *rootfs.FileIdentity
	receipt  *asc.CreativeManifestApplyReceipt
}

func (j *creativeJournal) save() error {
	data, err := json.Marshal(j.receipt)
	if len(data) > 4<<20 {
		return fmt.Errorf("creative journal exceeds 4 MiB; recovery state could not be saved")
	}
	if err != nil {
		return err
	}
	var next *rootfs.FileIdentity
	if j.identity == nil {
		next, err = j.root.CreateNewFileAtomicWithIdentity(j.name, data, 0o600)
	} else {
		next, err = j.root.ReplaceFileIfSame(j.name, j.identity, data, 0o600, false)
	}
	if next != nil {
		previous := j.identity
		j.identity = next
		if previous != nil {
			_ = j.root.ReleaseFileIdentity(previous)
		}
	}
	if err != nil {
		return fmt.Errorf("save creative recovery journal: %w; stop and inspect the journal before retrying", err)
	}
	return nil
}

func openCreativeJournal(path string, plan *asc.CreativeManifestPlan) (*creativeJournal, error) {
	root, err := rootfs.New(filepath.Dir(path))
	if err != nil {
		return nil, err
	}
	j := &creativeJournal{root: root, name: filepath.Base(path)}
	identity, err := root.CaptureFileLimited(j.name, 4<<20)
	if errors.Is(err, os.ErrNotExist) {
		nonce := make([]byte, 16)
		if _, err = rand.Read(nonce); err != nil {
			root.Close()
			return nil, err
		}
		j.receipt = &asc.CreativeManifestApplyReceipt{SchemaVersion: 1, AppID: plan.AppID, VersionID: plan.VersionID, LibraryID: plan.LibraryID, TransactionID: hex.EncodeToString(nonce), Sources: map[string]*asc.CreativeManifestAppliedSource{}, Targets: map[string]*asc.CreativeManifestAppliedTarget{}}
		return j, nil
	}
	if err != nil {
		root.Close()
		return nil, err
	}
	j.identity = identity
	decoder := json.NewDecoder(bytes.NewReader(identity.Data()))
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(&j.receipt); err == nil {
		if decoder.Decode(&struct{}{}) != io.EOF {
			err = fmt.Errorf("trailing journal data")
		}
	}
	if err != nil {
		root.Close()
		return nil, fmt.Errorf("invalid creative journal: %w", err)
	}
	if j.receipt == nil {
		root.Close()
		return nil, fmt.Errorf("invalid creative journal: expected JSON object")
	}
	r := j.receipt
	if r.SchemaVersion != 1 || r.AppID != plan.AppID || r.VersionID != plan.VersionID || r.LibraryID != plan.LibraryID || len(r.TransactionID) != 32 || r.Sources == nil || r.Targets == nil {
		root.Close()
		return nil, fmt.Errorf("journal does not bind the selected app/version/library")
	}
	if _, err := hex.DecodeString(r.TransactionID); err != nil {
		root.Close()
		return nil, fmt.Errorf("invalid journal transaction ID")
	}
	for key, source := range r.Sources {
		digest, err := hex.DecodeString(strings.TrimPrefix(key, "sha256:"))
		if source == nil || !strings.HasPrefix(key, "sha256:") || err != nil || len(digest) != 32 || (source.Phase != "reserving" && source.Phase != "reserved" && source.Phase != "transferred" && source.Phase != "ready") || (source.Phase != "reserving" && !resourceIDPattern.MatchString(source.AssetID)) {
			root.Close()
			return nil, fmt.Errorf("invalid journal source %s", key)
		}
	}
	for key, target := range r.Targets {
		if target == nil || r.Sources[target.SourceKey] == nil || key != creativeSlotKey(target.Locale, target.PlacementType) || !resourceIDPattern.MatchString(target.LocalizationID) || (target.Phase != "assigning" && target.Phase != "complete") || (target.Phase == "complete" && !resourceIDPattern.MatchString(target.PlacementID)) || (target.PreviousPlacementID == "") != (target.PreviousAssetID == "") {
			root.Close()
			return nil, fmt.Errorf("invalid journal target %s", key)
		}
	}
	return j, nil
}

func manifestApplyCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("asset-library manifest apply", flag.ExitOnError)
	app := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID)")
	version := shared.BindResourceIDFlag(fs, "version-id", "appStoreVersions", "Owned draft version ID")
	path := fs.String("manifest", "", "Creative JSON manifest; files relative to its directory")
	journal := fs.String("journal", "", "Retained local recovery journal (required; do not delete between runs)")
	confirm := fs.Bool("confirm", false, "Confirm uploads and assignments")
	replace := fs.Bool("replace", false, "Permit exact expected placement/asset pairs to be atomically replaced")
	output := d.BindOutputFlags(fs)
	return &ffcli.Command{
		Name: "apply", ShortUsage: "asc asset-library manifest apply --app ID --version-id ID --manifest ./creative.json --journal ./creative.receipt.json --confirm", ShortHelp: "Upload grouped creative sources and assign them with a durable recovery journal.", FlagSet: fs, UsageFunc: d.DefaultUsageFunc,
		LongHelp: `Apply the creative manifest to a PREPARE_FOR_SUBMISSION version only. All
local files and targets are preflighted before uploads. Each digest-and-poster
source is uploaded once; all sources must finish processing before assignments.
Video assignment requires a complete poster with an observable time code; a
processed creative video's download URL may be null. Missing poster identity
leaves a partial receipt. Failed or processing placements also remain partial.

--journal retains reservation IDs and acknowledged-transfer records before commit.
It binds local SHA256 to uploaded asset IDs,
not a provider checksum. Apple does not return original uploaded checksums.
Reuse requires the same owned library, ready media and unchanged observed poster.
Never substitute filenames or edit journal asset IDs to claim content identity.

Default behavior preserves occupied slots. To replace one, the entry must name
both expectedPlacementId and expectedAssetId, and --replace --confirm is required.
The exact slot is rechecked before one atomic same-media swap. The old placement
ID is lost; its library media is retained. No fallback deletion or review submit
occurs. Expected IDs are ignored only when this journal proves the requested new
asset is already assigned in the exact slot.

A reservation alone cannot establish that the source bytes were transferred.
Interrupted operations retain a partial JSON/table receipt. A pending placement
can be reconciled to the exact assigned asset. An unknown reservation outcome or
unfinished transfer stops for inspection; it is never blindly reserved again.
Inspect/delete only your own partial resources before choosing a new journal.
Keep this journal to resume and to reuse completed sources for additional locales.`,
		Exec: func(ctx context.Context, args []string) (resultErr error) {
			if err := shared.ValidateBoundOutputFlags(fs); err != nil {
				return d.UsageErrorf("manifest apply: %v", err)
			}
			appID, versionID := d.ResolveAppID(*app), strings.TrimSpace(*version)
			for _, required := range []struct{ name, value string }{{"--app", appID}, {"--version-id", versionID}, {"--manifest", *path}, {"--journal", *journal}} {
				if strings.TrimSpace(required.value) == "" {
					return d.RequiredFlag(required.name)
				}
			}
			if !*confirm {
				return d.UsageError("manifest apply: --confirm is required")
			}
			if !shared.IsNumericAppID(appID) || strings.HasPrefix(appID, "0") {
				return d.UsageError("--app requires a canonical numeric app ID")
			}
			absolute, err := filepath.Abs(d.OSPath(*path))
			if err != nil {
				return err
			}
			journalPath, err := filepath.Abs(d.OSPath(*journal))
			if err != nil {
				return err
			}
			if journalPath == absolute {
				return d.UsageError("--journal must differ from --manifest")
			}
			root, err := rootfs.New(filepath.Dir(absolute))
			if err != nil {
				return err
			}
			defer root.Close()
			data, err := root.ReadFileLimited(filepath.Base(absolute), 1<<20)
			if err != nil {
				return err
			}
			manifest, err := decodeCreativeManifest(data)
			if err != nil {
				return d.UsageErrorf("manifest: %v", err)
			}
			for _, entry := range manifest.Entries {
				if entry.ExpectedPlacementID != "" && !*replace {
					return d.UsageError("manifest replacement IDs require --replace --confirm")
				}
			}
			client, err := d.Client()
			if err != nil {
				return err
			}
			ctx = asc.WithNoRedirectReads(ctx)
			plan, err := buildCreativeManifestPlan(ctx, d, client, appID, versionID, root, manifest)
			if err != nil {
				return err
			}
			if plan.VersionState != "PREPARE_FOR_SUBMISSION" {
				return fmt.Errorf("manifest apply currently requires PREPARE_FOR_SUBMISSION; selected version is %s", plan.VersionState)
			}
			j, err := openCreativeJournal(journalPath, plan)
			if err != nil {
				return err
			}
			defer j.root.Close()
			defer func() {
				if j.identity != nil {
					if err := d.PrintOutput(j.receipt, *output.Output, *output.Pretty); err != nil {
						resultErr = errors.Join(resultErr, err)
					}
				}
			}()
			// Every snapshot is private, and the same descriptor is later uploaded.
			private, err := os.MkdirTemp("", "asc-creative-manifest-")
			if err != nil {
				return err
			}
			defer os.RemoveAll(private)
			snapshot, err := rootfs.New(private)
			if err != nil {
				return err
			}
			defer snapshot.Close()
			reference, err := getLibraryResource(ctx, d, client, "/v1/appAssetLibraryRefData")
			if err != nil {
				return err
			}
			copied := map[string]bool{}
			for i, entry := range manifest.Entries {
				if !copied[entry.File] {
					source, err := root.OpenFile(entry.File)
					if err != nil {
						return err
					}
					if err = snapshot.MkdirAll(filepath.Dir(entry.File), 0o700); err != nil {
						source.Close()
						return err
					}
					_, err = snapshot.CreateNewFrom(entry.File, io.LimitReader(source, plan.Entries[i].FileSize+1), 0o600)
					source.Close()
					if err != nil {
						return err
					}
					copied[entry.File] = true
				}
				staged, err := planCreativeSource(ctx, d, snapshot, entry, reference)
				if err != nil {
					return err
				}
				if staged.SourceKey != plan.Entries[i].SourceKey {
					return fmt.Errorf("source changed during snapshot: %s", entry.File)
				}
			}
			if err = validateCreativeApplyTargets(plan, manifest, j.receipt, *replace); err != nil {
				return err
			}
			verifiedSources := map[string]bool{}
			for _, entry := range plan.Entries {
				source := j.receipt.Sources[entry.SourceKey]
				if source == nil || verifiedSources[entry.SourceKey] {
					continue
				}
				if source.AssetID == "" {
					return fmt.Errorf("uncertain reservation for %s; inspect transaction %s before choosing a new journal", entry.SourceKey, j.receipt.TransactionID)
				}
				if source.Phase != "transferred" && source.Phase != "ready" {
					return fmt.Errorf("asset %s has no durable transfer receipt; inspect or delete the partial asset before choosing a new journal", source.AssetID)
				}
				observed, state, err := verifyCreativeJournalSource(ctx, d, client, plan.LibraryID, entry, source)
				if err != nil {
					return err
				}
				source.ObservedPosterTimeCode, source.State, source.Phase = observed, state, "ready"
				verifiedSources[entry.SourceKey] = true
			}
			// Publish the initial binding before the first provider mutation.
			if err = j.save(); err != nil {
				return err
			}

			for _, entry := range plan.Entries {
				if verifiedSources[entry.SourceKey] {
					continue
				}
				source := j.receipt.Sources[entry.SourceKey]
				if source == nil {
					source = &asc.CreativeManifestAppliedSource{ReferenceName: "asc-manifest-" + j.receipt.TransactionID + "-" + entry.SourceKey[len("sha256:"):len("sha256:")+16], File: entry.File, ContentSHA256: entry.ContentSHA256, MediaType: entry.MediaType, FileSize: entry.FileSize, PosterFrameMode: entry.PosterFrameMode, PreviewFrameTimeCode: entry.PreviewFrameTimeCode, Phase: "reserving"}
					j.receipt.Sources[entry.SourceKey] = source
					if err = j.save(); err != nil {
						return err
					}
					file, err := snapshot.OpenFile(entry.File)
					if err != nil {
						return err
					}
					uploadCtx, cancel := libraryImageUploadContext(ctx, d, entry.FileSize)
					if entry.MediaType == "VIDEO" {
						cancel()
						uploadCtx, cancel = libraryVideoUploadContext(ctx, d)
						uploadCtx = asc.WithCreativeVideoPlacementReadiness(uploadCtx)
					}
					uploadCtx = asc.WithAssetLibraryUploadJournal(uploadCtx, func(checkpoint asc.AssetLibraryUploadCheckpoint, id, resourceType string) error {
						if !resourceIDPattern.MatchString(id) {
							return fmt.Errorf("invalid reserved asset ID")
						}
						if source.AssetID != "" && source.AssetID != id {
							return fmt.Errorf("upload checkpoint asset identity changed")
						}
						source.AssetID = id
						source.Phase = string(checkpoint)
						return j.save()
					})
					name := source.ReferenceName
					if entry.MediaType == "IMAGE" {
						result, e := client.UploadAssetLibraryImageWithCategory(uploadCtx, d.IO.ErrOut, plan.LibraryID, filepath.Base(entry.File), file, entry.FileSize, creativeAssetCategory, name, d.Timeout)
						source.AssetID, source.State = result.ImageID, result.State
						err = e
					} else {
						result, e := client.UploadAssetLibraryVideoWithCategory(uploadCtx, d.IO.ErrOut, plan.LibraryID, filepath.Base(entry.File), file, entry.FileSize, creativeAssetCategory, name, entry.PreviewFrameTimeCode, d.Timeout)
						source.AssetID, source.State = result.VideoID, result.State
						err = e
					}
					cancel()
					file.Close()
					if err != nil {
						return fmt.Errorf("source %s: %w; retained asset %q, referenceName %q; inspect it before retrying", entry.SourceKey, err, source.AssetID, source.ReferenceName)
					}
				}
				if source.AssetID == "" {
					return fmt.Errorf("uncertain reservation for %s; inspect transaction %s before choosing a new journal", entry.SourceKey, j.receipt.TransactionID)
				}
				observed, state, err := verifyCreativeJournalSource(ctx, d, client, plan.LibraryID, entry, source)
				if err != nil {
					return err
				}
				source.ObservedPosterTimeCode, source.State, source.Phase = observed, state, "ready"
				verifiedSources[entry.SourceKey] = true
				if err = j.save(); err != nil {
					return err
				}
			}
			for i, entry := range plan.Entries {
				key := creativeSlotKey(entry.Locale, entry.PlacementType)
				source := j.receipt.Sources[entry.SourceKey]
				current, err := creativeManifestPlacements(ctx, d, client, entry.LocalizationID, entry.PlacementType, entry.PlacementGroup)
				if err != nil {
					return err
				}
				target := j.receipt.Targets[key]
				if target != nil && target.SourceKey == entry.SourceKey {
					if len(current) == 1 && current[0].AssetID == source.AssetID && (target.PlacementID == "" || target.PlacementID == current[0].ID) {
						target.PlacementID, target.State = current[0].ID, current[0].State
						if stateErr := creativeManifestPlacementState(target.State); stateErr != nil {
							return errors.Join(stateErr, j.save())
						}
						target.Phase = "complete"
						if err = j.save(); err != nil {
							return err
						}
						continue
					}
					return fmt.Errorf("target %s journal outcome cannot be reconciled; inspect it before retrying", key)
				}
				if !sameCreativePlacements(current, entry.ExistingPlacements) {
					return fmt.Errorf("target %s changed after planning; no assignment sent", key)
				}
				expected := manifest.Entries[i]
				if len(current) > 0 && (!*replace || len(current) != 1 || current[0].ID != expected.ExpectedPlacementID || current[0].AssetID != expected.ExpectedAssetID || current[0].MediaType != entry.MediaType) {
					return fmt.Errorf("target %s occupied or expected identity changed", key)
				}
				target = &asc.CreativeManifestAppliedTarget{Locale: entry.Locale, LocalizationID: entry.LocalizationID, PlacementType: entry.PlacementType, SourceKey: entry.SourceKey, PreviousPlacementID: expected.ExpectedPlacementID, PreviousAssetID: expected.ExpectedAssetID, Phase: "assigning"}
				j.receipt.Targets[key] = target
				if err = j.save(); err != nil {
					return err
				}
				id, state, err := assignCreativeManifestTarget(ctx, d, client, entry, source.AssetID, target.PreviousPlacementID)
				if err != nil {
					return fmt.Errorf("target %s: %w; journal preserves uncertain assignment", key, err)
				}
				target.PlacementID, target.State = id, state
				if err = j.save(); err != nil {
					return err
				}
				verified, err := creativeManifestPlacements(ctx, d, client, entry.LocalizationID, entry.PlacementType, entry.PlacementGroup)
				if err != nil {
					return err
				}
				if len(verified) != 1 || verified[0].ID != id || verified[0].AssetID != source.AssetID {
					return fmt.Errorf("target %s assignment readback differs; inspect retained receipt", key)
				}
				target.State = verified[0].State
				if stateErr := creativeManifestPlacementState(target.State); stateErr != nil {
					return errors.Join(stateErr, j.save())
				}
				target.Phase = "complete"
				if err = j.save(); err != nil {
					return err
				}
			}
			return nil
		},
	}
}

func creativeSlotKey(locale, kind string) string { return locale + "/" + kind + "/DEFAULT_PROFILE" }

func validateCreativeApplyTargets(plan *asc.CreativeManifestPlan, manifest creativeManifest, receipt *asc.CreativeManifestApplyReceipt, replace bool) error {
	for i, entry := range plan.Entries {
		if entry.LocalizationID == "" {
			return fmt.Errorf("missing localization %s; create it explicitly before apply", entry.Locale)
		}
		if source := receipt.Sources[entry.SourceKey]; source != nil {
			if source.ContentSHA256 != entry.ContentSHA256 || source.MediaType != entry.MediaType || source.FileSize != entry.FileSize || source.PosterFrameMode != entry.PosterFrameMode || source.PreviewFrameTimeCode != entry.PreviewFrameTimeCode || (source.AssetID != "" && !resourceIDPattern.MatchString(source.AssetID)) {
				return fmt.Errorf("source journal does not match %s", entry.SourceKey)
			}
		}
		target := receipt.Targets[creativeSlotKey(entry.Locale, entry.PlacementType)]
		if target != nil && target.SourceKey == entry.SourceKey {
			if target.Locale != entry.Locale || target.LocalizationID != entry.LocalizationID || target.PlacementType != entry.PlacementType {
				return fmt.Errorf("target journal scope mismatch")
			}
			source := receipt.Sources[entry.SourceKey]
			current := entry.ExistingPlacements
			if source == nil || source.AssetID == "" || len(current) != 1 || current[0].AssetID != source.AssetID || (target.PlacementID != "" && target.PlacementID != current[0].ID) {
				return fmt.Errorf("target %s journal outcome cannot be reconciled; inspect it before retrying", entry.Locale)
			}
			continue
		}
		expected := manifest.Entries[i]
		if len(entry.ExistingPlacements) == 0 {
			if expected.ExpectedPlacementID != "" {
				return fmt.Errorf("expected placement missing for %s", entry.Locale)
			}
			continue
		}
		if !replace || len(entry.ExistingPlacements) != 1 {
			return fmt.Errorf("target %s is occupied; preserve it or specify --replace and exact expected IDs", entry.Locale)
		}
		old := entry.ExistingPlacements[0]
		if old.ID != expected.ExpectedPlacementID || old.AssetID != expected.ExpectedAssetID || old.MediaType != entry.MediaType {
			return fmt.Errorf("expected placement/asset identity or media type differs for %s", entry.Locale)
		}
	}
	return nil
}

func sameCreativePlacements(a, b []asc.CreativeManifestPlacement) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func verifyCreativeJournalSource(ctx context.Context, d *shared.Deps, client *asc.Client, libraryID string, entry asc.CreativeManifestPlanEntry, source *asc.CreativeManifestAppliedSource) (string, string, error) {
	kind, resourceType := "images", "appAssetLibraryImages"
	if entry.MediaType == "VIDEO" {
		kind, resourceType = "videos", "appAssetLibraryVideos"
	}
	items, err := creativeManifestCollection(ctx, d, client, "/v1/appAssetLibraries/"+url.PathEscape(libraryID)+"/"+kind, url.Values{"filter[id]": {source.AssetID}})
	if err != nil {
		return "", "", err
	}
	if len(items) != 1 {
		return "", "", fmt.Errorf("journal asset %s is not unique in the selected library", source.AssetID)
	}
	var item struct {
		ID, Type   string
		Attributes struct {
			Category, State, SpecID, PreviewFrameTimeCode, ReferenceName string
			VideoAsset                                                   json.RawMessage
			FileSize                                                     int64
			ImageAsset                                                   *struct{ Width, Height int }
			PreviewFrameImage                                            *struct {
				Image *struct{ Width, Height int }
				State *struct{ State string }
			}
		}
	}
	if err = json.Unmarshal(items[0], &item); err != nil {
		return "", "", err
	}
	a := item.Attributes
	if source.ReferenceName == "" || a.ReferenceName != source.ReferenceName {
		return "", "", fmt.Errorf("journal asset %s referenceName differs from the retained upload transaction", source.AssetID)
	}
	if item.ID != source.AssetID || item.Type != resourceType || a.Category != creativeAssetCategory || a.FileSize != entry.FileSize || a.State != "PREPARE_FOR_SUBMISSION" || a.SpecID == "" {
		return "", "", fmt.Errorf("journal asset %s ownership/content metadata/processing is not verified (state %s); inspect it before retrying", source.AssetID, a.State)
	}
	if entry.MediaType == "IMAGE" {
		if a.ImageAsset == nil || a.ImageAsset.Width <= 0 || a.ImageAsset.Height <= 0 {
			return "", "", fmt.Errorf("journal image %s lacks processed dimensions", source.AssetID)
		}
	} else {
		var mediaURL string
		explicitNull := bytes.Equal(bytes.TrimSpace(a.VideoAsset), []byte("null"))
		if len(a.VideoAsset) > 0 && !explicitNull {
			if err := json.Unmarshal(a.VideoAsset, &mediaURL); err != nil {
				return "", "", fmt.Errorf("invalid journal video URI: %w", err)
			}
		}
		u, err := url.Parse(mediaURL)
		if (!explicitNull && (err != nil || u.Scheme != "https" || u.Host == "")) || a.PreviewFrameImage == nil || a.PreviewFrameImage.State == nil || a.PreviewFrameImage.State.State != "COMPLETE" || a.PreviewFrameImage.Image == nil || a.PreviewFrameImage.Image.Width <= 0 || a.PreviewFrameImage.Image.Height <= 0 {
			return "", "", fmt.Errorf("journal video %s media and poster processing is incomplete", source.AssetID)
		}
		if a.PreviewFrameTimeCode == "" {
			return "", "", fmt.Errorf("journal video %s poster identity is unavailable", source.AssetID)
		}
		if (entry.PosterFrameMode == "explicit" && a.PreviewFrameTimeCode != entry.PreviewFrameTimeCode) || (source.Phase == "ready" && a.PreviewFrameTimeCode != source.ObservedPosterTimeCode) {
			return "", "", fmt.Errorf("journal video %s poster differs", source.AssetID)
		}
	}
	return a.PreviewFrameTimeCode, a.State, nil
}

func assignCreativeManifestTarget(ctx context.Context, d *shared.Deps, client *asc.Client, entry asc.CreativeManifestPlanEntry, assetID, previousID string) (string, string, error) {
	if previousID == "" {
		relationship := "image"
		if entry.MediaType == "VIDEO" {
			relationship = "video"
		}
		return d.CreateLibraryPlacement(ctx, client, "appStoreVersionLocalizations", entry.LocalizationID, relationship, assetID, entry.PlacementType, entry.PlacementGroup)
	}
	mediaRelationship, mediaType := "image", "appAssetLibraryImages"
	if entry.MediaType == "VIDEO" {
		mediaRelationship, mediaType = "video", "appAssetLibraryVideos"
	}
	relationship := func(typ, id string) map[string]any {
		return map[string]any{"data": map[string]string{"type": typ, "id": id}}
	}
	payload, _ := json.Marshal(map[string]any{"data": map[string]any{"type": "appAssetLibraryPlacements", "attributes": map[string]string{"placementType": entry.PlacementType, "placementGroup": entry.PlacementGroup}, "relationships": map[string]any{"appStoreVersionLocalization": relationship("appStoreVersionLocalizations", entry.LocalizationID), mediaRelationship: relationship(mediaType, assetID), "placementToSwapOut": relationship("appAssetLibraryPlacements", previousID)}}})
	requestCtx, cancel := d.Timeout(ctx)
	defer cancel()
	raw, err := client.RawRequest(requestCtx, "POST", "/v1/appAssetLibraryPlacements", payload)
	if err != nil {
		return "", "", err
	}
	var envelope struct {
		Data struct {
			ID, Type   string
			Attributes struct{ State string }
		}
	}
	if err = json.Unmarshal(raw, &envelope); err != nil {
		return "", "", err
	}
	if envelope.Data.Type != "appAssetLibraryPlacements" || !resourceIDPattern.MatchString(envelope.Data.ID) {
		return "", "", fmt.Errorf("invalid replacement receipt")
	}
	return envelope.Data.ID, envelope.Data.Attributes.State, nil
}

func creativeManifestPlacementState(state string) error {
	switch state {
	case "", "FAILED", "ASSET_PROCESSING":
		return fmt.Errorf("placement state %s is not ready; receipt remains partial", state)
	}
	return nil
}
