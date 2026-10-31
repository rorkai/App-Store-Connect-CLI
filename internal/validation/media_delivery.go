package validation

import (
	"fmt"
	"strings"
)

// Check IDs for version media delivery findings.
const (
	CheckMediaDeliveryFailed     = "media.delivery.failed"
	CheckMediaDeliveryPending    = "media.delivery.pending"
	CheckMediaDeliveryUnverified = "media.delivery.unverified"
)

type mediaDeliveryClass int

const (
	mediaDelivered mediaDeliveryClass = iota
	mediaDeliveryFailed
	mediaDeliveryPending
	mediaDeliveryUnknown
)

// classifySetMediaState maps the assetDeliveryState and videoDeliveryState
// enums. Apple reports no timestamp, so media stuck in a pending state cannot
// be told apart from media still moving through it.
func classifySetMediaState(state string) mediaDeliveryClass {
	switch normalizeMediaState(state) {
	case "COMPLETE":
		return mediaDelivered
	case "FAILED":
		return mediaDeliveryFailed
	case "AWAITING_UPLOAD", "UPLOAD_COMPLETE", "PROCESSING":
		return mediaDeliveryPending
	default:
		return mediaDeliveryUnknown
	}
}

// classifyPlacementState maps AppAssetLibraryPlacementState. The PARENT_ states
// describe the owning version, which means the asset itself finished.
func classifyPlacementState(state string) mediaDeliveryClass {
	normalized := normalizeMediaState(state)
	switch {
	case normalized == "FAILED":
		return mediaDeliveryFailed
	case normalized == "ASSET_PROCESSING":
		return mediaDeliveryPending
	case strings.HasPrefix(normalized, "PARENT_"):
		return mediaDelivered
	default:
		return mediaDeliveryUnknown
	}
}

func normalizeMediaState(state string) string {
	return strings.ToUpper(strings.TrimSpace(state))
}

type mediaSetAsset struct {
	id     string
	name   string
	state  string
	errors []string
}

type mediaSetDescription struct {
	noun          string
	resourceType  string
	resourceID    string
	slot          string
	locale        string
	deleteCommand string
}

// mediaDeliveryChecks reports media a version cannot ship with yet. A source
// that could not be read is reported rather than treated as delivered.
func mediaDeliveryChecks(screenshots []ScreenshotSet, previews []PreviewSet, placements []MediaPlacement, gaps []MediaReadGap) []CheckResult {
	var checks []CheckResult
	for _, set := range screenshots {
		assets := make([]mediaSetAsset, 0, len(set.Screenshots))
		for _, shot := range set.Screenshots {
			assets = append(assets, mediaSetAsset{id: shot.ID, name: shot.FileName, state: shot.DeliveryState, errors: shot.DeliveryErrors})
		}
		checks = append(checks, mediaSetDeliveryChecks(mediaSetDescription{
			noun:          "screenshot",
			resourceType:  "appScreenshotSet",
			resourceID:    set.ID,
			slot:          set.DisplayType,
			locale:        set.Locale,
			deleteCommand: `asc screenshots delete --id "SCREENSHOT_ID" --confirm`,
		}, assets)...)
	}
	for _, set := range previews {
		assets := make([]mediaSetAsset, 0, len(set.Previews))
		for _, preview := range set.Previews {
			assets = append(assets, mediaSetAsset{id: preview.ID, name: preview.FileName, state: preview.DeliveryState, errors: preview.DeliveryErrors})
		}
		checks = append(checks, mediaSetDeliveryChecks(mediaSetDescription{
			noun:          "app preview",
			resourceType:  "appPreviewSet",
			resourceID:    set.ID,
			slot:          set.PreviewType,
			locale:        set.Locale,
			deleteCommand: `asc video-previews delete --id "PREVIEW_ID" --confirm`,
		}, assets)...)
	}
	for _, placement := range placements {
		if check, ok := mediaPlacementCheck(placement); ok {
			checks = append(checks, check)
		}
	}
	for _, gap := range gaps {
		checks = append(checks, CheckResult{
			ID:           CheckMediaDeliveryUnverified,
			Severity:     SeverityWarning,
			Locale:       gap.Locale,
			ResourceType: "appStoreVersionLocalization",
			ResourceID:   gap.LocalizationID,
			Message:      fmt.Sprintf("could not verify delivery of %s for this locale", gap.Media),
			Remediation:  fmt.Sprintf("%s. Confirm this locale's media finished processing in App Store Connect", gap.Reason),
		})
	}
	return checks
}

func mediaSetDeliveryChecks(set mediaSetDescription, assets []mediaSetAsset) []CheckResult {
	grouped := map[mediaDeliveryClass][]string{}
	for _, asset := range assets {
		class := classifySetMediaState(asset.state)
		if class != mediaDelivered {
			grouped[class] = append(grouped[class], mediaAssetLabel(asset, class))
		}
	}

	subject := strings.TrimSpace(set.noun + " set " + set.slot)
	summary := func(labels []string, condition string) string {
		return fmt.Sprintf("%s has %d of %d %ss %s: %s", subject, len(labels), len(assets), set.noun, condition, strings.Join(labels, ", "))
	}

	var checks []CheckResult
	appendCheck := func(id string, severity Severity, message, remediation string) {
		checks = append(checks, CheckResult{
			ID:           id,
			Severity:     severity,
			Locale:       set.locale,
			ResourceType: set.resourceType,
			ResourceID:   set.resourceID,
			Message:      message,
			Remediation:  remediation,
		})
	}
	if labels := grouped[mediaDeliveryFailed]; len(labels) > 0 {
		appendCheck(CheckMediaDeliveryFailed, SeverityError, summary(labels, "that failed processing"),
			fmt.Sprintf("Delete each failed %s with `%s`, then upload it again", set.noun, set.deleteCommand))
	}
	if labels := grouped[mediaDeliveryPending]; len(labels) > 0 {
		appendCheck(CheckMediaDeliveryPending, SeverityWarning, summary(labels, "that have not finished processing"),
			fmt.Sprintf("Wait for each %s to reach COMPLETE, then validate again. If its state stops changing, delete it with `%s` and upload it again", set.noun, set.deleteCommand))
	}
	if labels := grouped[mediaDeliveryUnknown]; len(labels) > 0 {
		appendCheck(CheckMediaDeliveryUnverified, SeverityWarning, summary(labels, "with an unconfirmed delivery state"),
			fmt.Sprintf("Confirm each %s finished processing in App Store Connect", set.noun))
	}
	return checks
}

func mediaAssetLabel(asset mediaSetAsset, class mediaDeliveryClass) string {
	name := strings.TrimSpace(asset.name)
	if name == "" {
		name = strings.TrimSpace(asset.id)
	}
	detail := normalizeMediaState(asset.state)
	switch {
	case class == mediaDeliveryFailed && len(asset.errors) > 0:
		detail = strings.Join(asset.errors, "; ")
	case class == mediaDeliveryFailed:
		return name
	case detail == "":
		detail = "state not returned"
	}
	return fmt.Sprintf("%s (%s)", name, detail)
}

func mediaPlacementCheck(placement MediaPlacement) (CheckResult, bool) {
	class := classifyPlacementState(placement.State)
	check := CheckResult{
		Locale:       placement.Locale,
		ResourceType: "appAssetLibraryPlacement",
		ResourceID:   placement.ID,
	}
	subject := strings.TrimSpace("asset library placement " + placement.PlacementType)
	if group := strings.TrimSpace(placement.PlacementGroup); group != "" {
		subject += " in group " + group
	}
	state := normalizeMediaState(placement.State)
	switch class {
	case mediaDeliveryFailed:
		check.ID = CheckMediaDeliveryFailed
		check.Severity = SeverityError
		check.Message = subject + " failed processing"
		if len(placement.StateDetails) > 0 {
			check.Message += ": " + strings.Join(placement.StateDetails, "; ")
		}
		check.Remediation = "Replace the failed placement, or remove it with `asc localizations placements delete --id \"PLACEMENT_ID\" --confirm`"
	case mediaDeliveryPending:
		check.ID = CheckMediaDeliveryPending
		check.Severity = SeverityWarning
		check.Message = fmt.Sprintf("%s has not finished processing (%s)", subject, state)
		check.Remediation = "Wait for the asset to finish processing, then validate again"
	case mediaDeliveryUnknown:
		if state == "" {
			state = "state not returned"
		}
		check.ID = CheckMediaDeliveryUnverified
		check.Severity = SeverityWarning
		check.Message = fmt.Sprintf("%s has an unconfirmed delivery state (%s)", subject, state)
		check.Remediation = "Confirm the placement finished processing in App Store Connect"
	default:
		return CheckResult{}, false
	}
	return check, true
}
