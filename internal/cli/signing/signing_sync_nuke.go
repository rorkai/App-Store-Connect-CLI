package signing

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path"
	"sort"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	signingpkg "github.com/rudrankriyam/App-Store-Connect-CLI/internal/signing"
)

// SyncNukeResult is the receipt printed by signing sync nuke.
type SyncNukeResult = asc.SigningSyncNukeResult

func syncNukeCommand() *ffcli.Command {
	fs := flag.NewFlagSet("nuke", flag.ExitOnError)

	profileType := fs.String("profile-type", "", "Profile type whose profiles are deleted, such as IOS_APP_DEVELOPMENT (required)")
	certType := fs.String("certificate-type", "", "Certificate type(s) to revoke, comma-separated (default: inferred from --profile-type)")
	repoURL := fs.String("repo", "", "Git repo URL of the encrypted signing repository (required)")
	branch := fs.String("branch", "main", "Git branch")
	passwordFile := fs.String("password-file", "", "Protected file containing the repository encryption password (or set ASC_SIGNING_SYNC_PASSWORD)")
	confirm := fs.Bool("confirm", false, "Confirm deleting the profiles, revoking the certificates, and removing their encrypted artifacts")
	dryRun := fs.Bool("dry-run", false, "Print the plan without deleting, revoking, removing, or publishing anything")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "nuke",
		ShortUsage: "asc signing sync nuke --profile-type TYPE --repo URL [--password-file PATH] (--confirm | --dry-run)",
		ShortHelp:  "Revoke certificates and delete profiles of one type, then remove them from the repo.",
		LongHelp: `Delete every App Store Connect profile of one profile type, revoke every
certificate of the matching certificate type, and remove their encrypted
artifacts from the signing Git repository.

The plan lists every profile whose type is --profile-type and every
certificate whose type is --certificate-type (inferred from --profile-type
when omitted), for the team of the active API key. Profiles of other types are
never deleted. Revoking a certificate also invalidates any other profile that
embeds it, including Apple Development and Apple Distribution certificates
shared with other profile types; narrow the scope with --certificate-type.

The repository is cloned and every artifact is authenticated with the sync
password before App Store Connect is changed. With --confirm, profiles are
deleted first, then certificates are revoked, and then only the artifacts of
the profiles Apple confirmed deleted and the certificates it confirmed revoked
are removed in one commit. A failure never stops the remaining operations;
the receipt lists each failure, keeps the matching artifacts, and the command
exits 1.

--dry-run prints the same plan and exits 0 without deleting, revoking,
removing, or publishing anything.

Examples:
  asc signing sync nuke --profile-type IOS_APP_DEVELOPMENT \
    --repo git@github.com:team/certs.git --password-file ~/.config/asc/signing-sync-password --dry-run

  asc signing sync nuke --profile-type IOS_APP_DEVELOPMENT \
    --repo git@github.com:team/certs.git --password-file ~/.config/asc/signing-sync-password --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return shared.UsageErrorf("unexpected argument(s): %s", strings.Join(args, " "))
			}
			if _, err := shared.ValidateOutputFormat(*output.Output, *output.Pretty); err != nil {
				return shared.UsageError(err.Error())
			}
			rawType := strings.TrimSpace(*profileType)
			if rawType == "" {
				return shared.UsageError("--profile-type is required")
			}
			profType, err := normalizeSigningPullProfileType(rawType)
			if err != nil {
				return shared.UsageError(err.Error())
			}
			repo := strings.TrimSpace(*repoURL)
			if repo == "" {
				return shared.UsageError("--repo is required")
			}
			selectedBranch := strings.TrimSpace(*branch)
			if selectedBranch == "" {
				return shared.UsageError("--branch must not be empty")
			}
			if *confirm && *dryRun {
				return shared.UsageError("--confirm and --dry-run are mutually exclusive")
			}
			if !*confirm && !*dryRun {
				return shared.UsageError("--confirm is required to delete profiles and revoke certificates (or pass --dry-run to preview)")
			}
			certificateTypes, err := resolveSigningCertificateTypes(profType, *certType)
			if err != nil {
				return shared.UsageErrorf("--certificate-type: %v", err)
			}
			password, err := resolveSyncPassword(*passwordFile)
			if err != nil {
				return err
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("signing sync nuke: %w", err)
			}
			certificateTypes = signingCertificateTypesForClient(client, *certType, certificateTypes)

			result := SyncNukeResult{
				Operation:        "nuke",
				RepoURL:          sanitizeRepoURLForOutput(repo),
				ProfileType:      profType,
				CertificateTypes: shared.SplitCSV(certificateTypes),
				DryRun:           *dryRun,
				Profiles:         asc.SigningSyncNukeProfiles{Planned: []asc.SigningSyncNukeResource{}, Deleted: []asc.SigningSyncNukeResource{}},
				Certificates:     asc.SigningSyncNukeCertificates{Planned: []asc.SigningSyncNukeResource{}, Revoked: []asc.SigningSyncNukeResource{}},
				RepositoryFiles:  asc.SigningSyncNukeFiles{Planned: []string{}, Removed: []string{}},
			}

			fmt.Fprintln(os.Stderr, "Listing signing assets in App Store Connect...")
			profiles, err := listSigningNukeProfiles(ctx, client, profType)
			if err != nil {
				return fmt.Errorf("signing sync nuke: list profiles: %w", err)
			}
			certificates, err := listSigningNukeCertificates(ctx, client, certificateTypes)
			if err != nil {
				return fmt.Errorf("signing sync nuke: list certificates: %w", err)
			}

			tmpDir, err := os.MkdirTemp("", "asc-signing-sync-nuke-*")
			if err != nil {
				return fmt.Errorf("signing sync nuke: create temp dir: %w", err)
			}
			store := &signingpkg.GitStore{RepoURL: repo, LocalDir: tmpDir, Branch: selectedBranch}
			defer func() { _ = store.Cleanup() }()
			fmt.Fprintln(os.Stderr, "Cloning signing repo...")
			if err := store.Clone(ctx, false); err != nil {
				return fmt.Errorf("signing sync nuke: %w", err)
			}
			files, err := readSigningNukeFiles(store, password)
			if err != nil {
				return fmt.Errorf("signing sync nuke: %w", err)
			}

			plan := newSigningNukePlan(profType, profiles, certificates)
			result.Profiles.Planned = signingNukeProfileResources(profiles)
			result.Certificates.Planned = signingNukeCertificateResources(certificates)
			result.RepositoryFiles.Planned = planSigningNukeFiles(files, plan)

			if *dryRun {
				result.PublicationState = asc.SigningSyncNukePublicationSkipped
				fmt.Fprintf(os.Stderr, "Dry run: would delete %d profile(s), revoke %d certificate(s), and remove %d encrypted file(s); nothing was changed\n",
					len(result.Profiles.Planned), len(result.Certificates.Planned), len(result.RepositoryFiles.Planned))
				return shared.PrintOutput(&result, *output.Output, *output.Pretty)
			}

			deleted := make(map[string]struct{}, len(profiles))
			for _, profile := range profiles {
				requestCtx, cancel := shared.ContextWithTimeout(ctx)
				err := client.DeleteProfile(requestCtx, profile.ID)
				cancel()
				resource := signingNukeProfileResource(profile)
				if err != nil {
					result.Profiles.Failed = append(result.Profiles.Failed, asc.SigningSyncNukeFailure{ID: profile.ID, Name: profile.Attributes.Name, Error: err.Error()})
					fmt.Fprintf(os.Stderr, "  Failed to delete profile %s: %v\n", profile.ID, err)
					continue
				}
				deleted[profile.ID] = struct{}{}
				result.Profiles.Deleted = append(result.Profiles.Deleted, resource)
				fmt.Fprintf(os.Stderr, "  Deleted profile %s\n", profile.ID)
			}
			revoked := make([]asc.Resource[asc.CertificateAttributes], 0, len(certificates))
			for _, certificate := range certificates {
				requestCtx, cancel := shared.ContextWithTimeout(ctx)
				err := client.RevokeCertificate(requestCtx, certificate.ID)
				cancel()
				if err != nil {
					result.Certificates.Failed = append(result.Certificates.Failed, asc.SigningSyncNukeFailure{ID: certificate.ID, Name: certificate.Attributes.Name, Error: err.Error()})
					fmt.Fprintf(os.Stderr, "  Failed to revoke certificate %s: %v\n", certificate.ID, err)
					continue
				}
				revoked = append(revoked, certificate)
				result.Certificates.Revoked = append(result.Certificates.Revoked, signingNukeCertificateResource(certificate))
				fmt.Fprintf(os.Stderr, "  Revoked certificate %s\n", certificate.ID)
			}

			completed := signingNukePlan{
				ProfileType:  profType,
				ProfileIDs:   deleted,
				ProfileUUIDs: signingNukeProfileUUIDs(profiles, deleted),
				Certificates: revoked,
			}
			removable := planSigningNukeFiles(files, completed)
			result.RepositoryFiles.Kept = signingNukeDifference(result.RepositoryFiles.Planned, removable)
			failures := len(result.Profiles.Failed) + len(result.Certificates.Failed)
			result.Partial = failures > 0

			var publishErr error
			if len(removable) == 0 {
				result.PublicationState = asc.SigningSyncNukePublicationNotNeeded
			} else if err := removeEncryptedSigningFiles(store, removable); err != nil {
				result.PublicationState = asc.SigningSyncNukePublicationFailed
				publishErr = fmt.Errorf("remove encrypted artifacts: %w", err)
			} else {
				fmt.Fprintln(os.Stderr, "Pushing to git...")
				if err := store.CommitAndPush(ctx, fmt.Sprintf("Remove %s signing assets", profType)); err != nil {
					result.PublicationState = asc.SigningSyncNukePublicationFailed
					publishErr = err
				} else {
					result.PublicationState = asc.SigningSyncNukePublicationSucceeded
					result.RepositoryFiles.Removed = removable
				}
			}
			if publishErr != nil {
				result.Partial = true
				result.RepositoryFiles.Kept = append([]string(nil), result.RepositoryFiles.Planned...)
			}

			if err := shared.PrintOutput(&result, *output.Output, *output.Pretty); err != nil {
				return err
			}
			switch {
			case publishErr != nil && failures > 0:
				return fmt.Errorf("signing sync nuke: failed to delete %d profile(s) and revoke %d certificate(s); repository publication failed: %w", len(result.Profiles.Failed), len(result.Certificates.Failed), publishErr)
			case publishErr != nil:
				return fmt.Errorf("signing sync nuke: App Store Connect changes completed but repository publication failed: %w", publishErr)
			case failures > 0:
				return fmt.Errorf("signing sync nuke: failed to delete %d profile(s) and revoke %d certificate(s); their repository artifacts were kept", len(result.Profiles.Failed), len(result.Certificates.Failed))
			}
			fmt.Fprintln(os.Stderr, "Done")
			return nil
		},
	}
}

// listSigningNukeProfiles lists every profile of one type, sorted by ID. The
// type is checked again client side so nuke can never delete another type.
func listSigningNukeProfiles(ctx context.Context, client *asc.Client, profileType string) ([]asc.Resource[asc.ProfileAttributes], error) {
	var profiles []asc.Resource[asc.ProfileAttributes]
	next := ""
	page := 1
	seenNext := make(map[string]struct{})
	for {
		requestCtx, cancel := shared.ContextWithTimeout(ctx)
		options := []asc.ProfilesOption{asc.WithProfilesNextURL(next)}
		if next == "" {
			options = append(options, asc.WithProfilesFilterType(profileType), asc.WithProfilesLimit(200))
		}
		response, err := client.GetProfiles(requestCtx, options...)
		cancel()
		if err != nil {
			return nil, err
		}
		for _, profile := range response.Data {
			if strings.EqualFold(strings.TrimSpace(profile.Attributes.ProfileType), profileType) {
				profiles = append(profiles, profile)
			}
		}
		if strings.TrimSpace(response.Links.Next) == "" {
			break
		}
		if _, ok := seenNext[response.Links.Next]; ok {
			return nil, fmt.Errorf("page %d: %w", page+1, asc.ErrRepeatedPaginationURL)
		}
		seenNext[response.Links.Next] = struct{}{}
		page++
		next = response.Links.Next
	}
	sort.SliceStable(profiles, func(i, j int) bool { return profiles[i].ID < profiles[j].ID })
	return profiles, nil
}

// listSigningNukeCertificates lists every certificate of the selected types,
// sorted by ID, and checks each type again client side.
func listSigningNukeCertificates(ctx context.Context, client *asc.Client, certificateTypes string) ([]asc.Resource[asc.CertificateAttributes], error) {
	allowed := make(map[string]struct{})
	for _, certificateType := range shared.SplitCSVUpper(certificateTypes) {
		allowed[certificateType] = struct{}{}
	}
	var certificates []asc.Resource[asc.CertificateAttributes]
	next := ""
	page := 1
	seenNext := make(map[string]struct{})
	for {
		requestCtx, cancel := shared.ContextWithTimeout(ctx)
		options := []asc.CertificatesOption{asc.WithCertificatesNextURL(next)}
		if next == "" {
			options = append(options, asc.WithCertificatesFilterType(certificateTypes), asc.WithCertificatesLimit(200))
		}
		response, err := client.GetCertificates(requestCtx, options...)
		cancel()
		if err != nil {
			return nil, err
		}
		for _, certificate := range response.Data {
			if _, ok := allowed[strings.ToUpper(strings.TrimSpace(certificate.Attributes.CertificateType))]; ok {
				certificates = append(certificates, certificate)
			}
		}
		if strings.TrimSpace(response.Links.Next) == "" {
			break
		}
		if _, ok := seenNext[response.Links.Next]; ok {
			return nil, fmt.Errorf("page %d: %w", page+1, asc.ErrRepeatedPaginationURL)
		}
		seenNext[response.Links.Next] = struct{}{}
		page++
		next = response.Links.Next
	}
	sort.SliceStable(certificates, func(i, j int) bool { return certificates[i].ID < certificates[j].ID })
	return certificates, nil
}

// readSigningNukeFiles decrypts every artifact with the sync password so a
// wrong password or tampered repository fails before App Store Connect is
// changed. Unlike pull, it does not require identities to be currently valid:
// expired identities are exactly what nuke cleans up.
func readSigningNukeFiles(store *signingpkg.GitStore, password string) ([]decryptedSigningFile, error) {
	relPaths, err := store.ListEncryptedFiles()
	if err != nil {
		return nil, fmt.Errorf("list files: %w", err)
	}
	sort.Strings(relPaths)
	if len(relPaths) > maxEncryptedSigningFiles {
		return nil, fmt.Errorf("encrypted signing repository contains %d files; limit is %d", len(relPaths), maxEncryptedSigningFiles)
	}
	var cumulativeSize int64
	for _, relPath := range relPaths {
		size, err := store.EncryptedFileSize(relPath)
		if err != nil {
			return nil, fmt.Errorf("inspect encrypted artifact %s: %w", relPath, err)
		}
		if size < 0 || size > maxEncryptedSigningBytes-cumulativeSize {
			return nil, fmt.Errorf("encrypted signing repository exceeds the %d-byte cumulative size limit", maxEncryptedSigningBytes)
		}
		cumulativeSize += size
	}
	files := make([]decryptedSigningFile, 0, len(relPaths))
	for _, relPath := range relPaths {
		plaintext, metadata, err := store.ReadEncryptedFileWithMetadata(relPath, password)
		if err != nil {
			return nil, fmt.Errorf("decrypt %s: %w", relPath, err)
		}
		files = append(files, decryptedSigningFile{RelativePath: relPath, Plaintext: plaintext, Metadata: metadata})
	}
	return files, nil
}

// signingNukePlan is the set of App Store Connect resources whose encrypted
// artifacts nuke may remove.
type signingNukePlan struct {
	ProfileType  string
	ProfileIDs   map[string]struct{}
	ProfileUUIDs map[string]struct{}
	Certificates []asc.Resource[asc.CertificateAttributes]
}

func newSigningNukePlan(profileType string, profiles []asc.Resource[asc.ProfileAttributes], certificates []asc.Resource[asc.CertificateAttributes]) signingNukePlan {
	ids := make(map[string]struct{}, len(profiles))
	for _, profile := range profiles {
		ids[profile.ID] = struct{}{}
	}
	return signingNukePlan{
		ProfileType:  profileType,
		ProfileIDs:   ids,
		ProfileUUIDs: signingNukeProfileUUIDs(profiles, ids),
		Certificates: certificates,
	}
}

func signingNukeProfileUUIDs(profiles []asc.Resource[asc.ProfileAttributes], selected map[string]struct{}) map[string]struct{} {
	uuids := make(map[string]struct{})
	for _, profile := range profiles {
		if _, ok := selected[profile.ID]; !ok {
			continue
		}
		if uuid, err := normalizeIdentityProfileUUID(profile.Attributes.UUID); err == nil && uuid != "" {
			uuids[uuid] = struct{}{}
		}
	}
	return uuids
}

// planSigningNukeFiles selects the encrypted artifacts that belong to the
// planned resources. Profile artifacts and identity contexts match by their
// authenticated profile type and resource ID, legacy profiles by the UUID
// App Store Connect reports, certificates by content or serial-number path,
// and identity cores only when their certificate is planned and no retained
// identity context still references them.
func planSigningNukeFiles(files []decryptedSigningFile, plan signingNukePlan) []string {
	certificateContents := make([][]byte, 0, len(plan.Certificates))
	certificateNames := make(map[string]struct{}, len(plan.Certificates))
	certificateSerials := make(map[string]struct{}, len(plan.Certificates))
	certificateFingerprints := make(map[string]struct{}, len(plan.Certificates))
	for _, certificate := range plan.Certificates {
		certificateNames[safeFileName(certificate.Attributes.SerialNumber, certificate.ID)+".cer"] = struct{}{}
		certificateSerials[asc.CertificateSerialKey(certificate.Attributes.SerialNumber)] = struct{}{}
		content, err := base64.StdEncoding.DecodeString(strings.TrimSpace(certificate.Attributes.CertificateContent))
		if err != nil || len(content) == 0 {
			continue
		}
		certificateContents = append(certificateContents, content)
		digest := sha256.Sum256(content)
		certificateFingerprints[strings.ToUpper(hex.EncodeToString(digest[:]))] = struct{}{}
	}

	selected := make(map[string]struct{})
	retainedCoreReferences := make(map[string]struct{})
	for _, file := range files {
		canonical := canonicalSigningPullPath(file.RelativePath)
		switch {
		case file.Metadata.Kind == signingProfileArtifactKind:
			if strings.EqualFold(file.Metadata.ProfileType, plan.ProfileType) && hasSigningNukeKey(plan.ProfileIDs, file.Metadata.ProfileResourceID) {
				selected[canonical] = struct{}{}
			}
		case file.Metadata.Kind == "identity-context":
			var binding identityContextBinding
			if err := json.Unmarshal(file.Plaintext, &binding); err != nil {
				continue
			}
			if strings.EqualFold(binding.ProfileType, plan.ProfileType) && hasSigningNukeKey(plan.ProfileIDs, binding.ProfileResourceID) {
				selected[canonical] = struct{}{}
				continue
			}
			retainedCoreReferences[strings.ToUpper(binding.CertificateSHA256)] = struct{}{}
		case file.Metadata.Version == 0 && strings.HasPrefix(canonical, "profiles/"):
			profile, err := parseIdentityMobileProvision(file.Plaintext)
			if err != nil {
				continue
			}
			uuid, err := normalizeIdentityProfileUUID(profile.UUID)
			if err == nil && hasSigningNukeKey(plan.ProfileUUIDs, uuid) {
				selected[canonical] = struct{}{}
			}
		case file.Metadata.Version == 0 && strings.HasPrefix(canonical, "certs/") && strings.HasSuffix(strings.ToLower(canonical), ".cer"):
			base := path.Base(canonical)
			if hasSigningNukeKey(certificateNames, base) || hasSigningNukeKey(certificateSerials, asc.CertificateSerialKey(base[:len(base)-len(".cer")])) {
				selected[canonical] = struct{}{}
				continue
			}
			for _, content := range certificateContents {
				if bytes.Equal(content, file.Plaintext) {
					selected[canonical] = struct{}{}
					break
				}
			}
		}
	}
	for _, file := range files {
		if file.Metadata.Kind != "pkcs12-identity" {
			continue
		}
		fingerprint := strings.ToUpper(file.Metadata.CertificateSHA256)
		if !hasSigningNukeKey(certificateFingerprints, fingerprint) || hasSigningNukeKey(retainedCoreReferences, fingerprint) {
			continue
		}
		selected[canonicalSigningPullPath(file.RelativePath)] = struct{}{}
	}

	paths := make([]string, 0, len(selected))
	for _, file := range files {
		if _, ok := selected[canonicalSigningPullPath(file.RelativePath)]; ok {
			paths = append(paths, file.RelativePath)
		}
	}
	sort.Strings(paths)
	return paths
}

func hasSigningNukeKey(set map[string]struct{}, key string) bool {
	key = strings.TrimSpace(key)
	if key == "" {
		return false
	}
	_, ok := set[key]
	return ok
}

func signingNukeDifference(all, removed []string) []string {
	removedSet := make(map[string]struct{}, len(removed))
	for _, value := range removed {
		removedSet[value] = struct{}{}
	}
	var kept []string
	for _, value := range all {
		if _, ok := removedSet[value]; !ok {
			kept = append(kept, value)
		}
	}
	return kept
}

func signingNukeProfileResource(profile asc.Resource[asc.ProfileAttributes]) asc.SigningSyncNukeResource {
	return asc.SigningSyncNukeResource{
		ID:             profile.ID,
		Name:           profile.Attributes.Name,
		Type:           profile.Attributes.ProfileType,
		ExpirationDate: profile.Attributes.ExpirationDate,
	}
}

func signingNukeProfileResources(profiles []asc.Resource[asc.ProfileAttributes]) []asc.SigningSyncNukeResource {
	resources := make([]asc.SigningSyncNukeResource, 0, len(profiles))
	for _, profile := range profiles {
		resources = append(resources, signingNukeProfileResource(profile))
	}
	return resources
}

func signingNukeCertificateResource(certificate asc.Resource[asc.CertificateAttributes]) asc.SigningSyncNukeResource {
	return asc.SigningSyncNukeResource{
		ID:             certificate.ID,
		Name:           certificate.Attributes.Name,
		Type:           certificate.Attributes.CertificateType,
		SerialNumber:   certificate.Attributes.SerialNumber,
		ExpirationDate: certificate.Attributes.ExpirationDate,
	}
}

func signingNukeCertificateResources(certificates []asc.Resource[asc.CertificateAttributes]) []asc.SigningSyncNukeResource {
	resources := make([]asc.SigningSyncNukeResource, 0, len(certificates))
	for _, certificate := range certificates {
		resources = append(resources, signingNukeCertificateResource(certificate))
	}
	return resources
}
