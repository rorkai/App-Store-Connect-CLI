package asc

import "encoding/json"

// SigningSyncTargetResult is the deterministic summary for one target in a
// multi-target signing sync operation.
type SigningSyncTargetResult struct {
	BundleID                 string   `json:"bundleId"`
	ProfileType              string   `json:"profileType"`
	ProfilePath              string   `json:"profilePath"`
	ProfilePaths             []string `json:"profilePaths,omitempty"`
	ProfileCreated           bool     `json:"profileCreated"`
	CertificateCreationState string   `json:"certificateCreationState,omitempty"`
	ProfileCreationState     string   `json:"profileCreationState,omitempty"`
	Files                    []string `json:"files"`
	// ProfileLifecycle is set by push with --renew-expired or
	// --force-for-new-devices when an existing profile was evaluated.
	ProfileLifecycle *SigningSyncProfileLifecycle `json:"profileLifecycle,omitempty"`
}

// SigningSyncStorage identifies where a signing sync operation keeps its
// encrypted artifacts. Location is the same non-secret locator reported in
// repoUrl. Branch is set only for git storage. There is no version field
// because one push can write many objects, each with its own version.
type SigningSyncStorage struct {
	Kind     string `json:"kind"`
	Location string `json:"location"`
	Branch   string `json:"branch,omitempty"`
}

// SigningSyncSkippedEntry is a source entry that a signing sync operation did
// not use, with the reason.
type SigningSyncSkippedEntry struct {
	Path   string `json:"path"`
	Reason string `json:"reason"`
}

// SigningSyncResult is the structured output for signing sync operations.
// The singular BundleID field is retained for the historical single-target
// and pull JSON shape. Batch output marks itself explicitly so that an empty
// or partially populated computed result cannot accidentally reintroduce it.
type SigningSyncResult struct {
	Operation                string                    `json:"operation"`
	RepoURL                  string                    `json:"repoUrl"`
	Storage                  *SigningSyncStorage       `json:"storage,omitempty"`
	BundleID                 string                    `json:"bundleId"`
	ProfileType              string                    `json:"profileType"`
	Files                    []string                  `json:"files"`
	IdentityPresent          bool                      `json:"identityPresent"`
	IdentitySHA256           string                    `json:"identitySha256,omitempty"`
	SensitiveFiles           []string                  `json:"sensitiveFiles,omitempty"`
	BundleIDs                []string                  `json:"bundleIds,omitempty"`
	Targets                  []SigningSyncTargetResult `json:"targets,omitempty"`
	CertificateIDs           []string                  `json:"certificateIds,omitempty"`
	CertificateCreationState string                    `json:"certificateCreationState,omitempty"`
	ProfileCreationState     string                    `json:"profileCreationState,omitempty"`
	PublicationState         string                    `json:"publicationState,omitempty"`
	Partial                  bool                      `json:"partial,omitempty"`
	// ProfileLifecycle is set by single-target push with --renew-expired or
	// --force-for-new-devices when an existing profile was evaluated. Batch
	// results report it per target.
	ProfileLifecycle *SigningSyncProfileLifecycle `json:"profileLifecycle,omitempty"`
	SourceFormat     string                       `json:"sourceFormat,omitempty"`
	Skipped          []SigningSyncSkippedEntry    `json:"skipped,omitempty"`
	batch            bool
}

// MarkBatch marks a computed result as the multi-target shape. It is kept out
// of JSON so the public contract remains the documented result fields only.
func (result *SigningSyncResult) MarkBatch() {
	if result != nil {
		result.batch = true
	}
}

// MarshalJSON keeps the historical singular bundleId field for single-target
// and pull results while omitting it from the additive batch result.
func (result SigningSyncResult) MarshalJSON() ([]byte, error) {
	bundleID := &result.BundleID
	if result.batch {
		bundleID = nil
	}
	type signingSyncResultJSON struct {
		Operation                string                       `json:"operation"`
		RepoURL                  string                       `json:"repoUrl"`
		Storage                  *SigningSyncStorage          `json:"storage,omitempty"`
		BundleID                 *string                      `json:"bundleId,omitempty"`
		ProfileType              string                       `json:"profileType"`
		Files                    []string                     `json:"files"`
		IdentityPresent          bool                         `json:"identityPresent"`
		IdentitySHA256           string                       `json:"identitySha256,omitempty"`
		SensitiveFiles           []string                     `json:"sensitiveFiles,omitempty"`
		BundleIDs                []string                     `json:"bundleIds,omitempty"`
		Targets                  []SigningSyncTargetResult    `json:"targets,omitempty"`
		CertificateIDs           []string                     `json:"certificateIds,omitempty"`
		CertificateCreationState string                       `json:"certificateCreationState,omitempty"`
		ProfileCreationState     string                       `json:"profileCreationState,omitempty"`
		PublicationState         string                       `json:"publicationState,omitempty"`
		Partial                  bool                         `json:"partial,omitempty"`
		ProfileLifecycle         *SigningSyncProfileLifecycle `json:"profileLifecycle,omitempty"`
		SourceFormat             string                       `json:"sourceFormat,omitempty"`
		Skipped                  []SigningSyncSkippedEntry    `json:"skipped,omitempty"`
	}
	return json.Marshal(signingSyncResultJSON{
		Operation:                result.Operation,
		RepoURL:                  result.RepoURL,
		Storage:                  result.Storage,
		BundleID:                 bundleID,
		ProfileType:              result.ProfileType,
		Files:                    result.Files,
		IdentityPresent:          result.IdentityPresent,
		IdentitySHA256:           result.IdentitySHA256,
		SensitiveFiles:           result.SensitiveFiles,
		BundleIDs:                result.BundleIDs,
		Targets:                  result.Targets,
		CertificateIDs:           result.CertificateIDs,
		CertificateCreationState: result.CertificateCreationState,
		ProfileCreationState:     result.ProfileCreationState,
		PublicationState:         result.PublicationState,
		Partial:                  result.Partial,
		ProfileLifecycle:         result.ProfileLifecycle,
		SourceFormat:             result.SourceFormat,
		Skipped:                  result.Skipped,
	})
}

func signingSyncRows(result *SigningSyncResult) ([]string, [][]string) {
	summaryHeaders := []string{"Operation", "Repo URL", "Storage", "Bundle ID", "Profile Type", "Files", "Identity Present"}
	if result == nil {
		return summaryHeaders, nil
	}
	if !result.batch {
		return summaryHeaders, [][]string{{
			result.Operation,
			result.RepoURL,
			signingSyncStorageKind(result.Storage),
			result.BundleID,
			result.ProfileType,
			joinSigningList(result.Files),
			formatBool(result.IdentityPresent),
		}}
	}
	rows := make([][]string, 0, len(result.Targets))
	for _, target := range result.Targets {
		rows = append(rows, []string{
			target.BundleID,
			target.ProfileType,
			target.ProfilePath,
			formatBool(target.ProfileCreated),
			joinSigningList(target.Files),
		})
	}
	return []string{"Bundle ID", "Profile Type", "Profile Path", "Profile Created", "Files"}, rows
}

func signingSyncStorageKind(storage *SigningSyncStorage) string {
	if storage == nil {
		return ""
	}
	return storage.Kind
}
