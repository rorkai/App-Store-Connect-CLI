package validation

import (
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
)

// Check IDs for the announced App Store Connect upload requirement.
const (
	CheckUploadRequirementMinOS           = "build.upload_requirement.min_os"
	CheckUploadRequirementMinOSUnverified = "build.upload_requirement.min_os_unverified"
	CheckUploadRequirementSDK             = "build.upload_requirement.sdk"
	CheckUploadRequirementSDKUnverified   = "build.upload_requirement.sdk_unverified"
)

type uploadRequirement struct {
	// effective is the first instant of effectiveLabel's month, because Apple
	// published the month without a day.
	effective      time.Time
	effectiveLabel string
	sdkMajor       int
	iosTargetMajor int
	// platformSDKs lists the App Store version platforms Apple names. macOS is
	// absent because Apple lists no macOS requirement.
	platformSDKs map[string]string
	watchSDK     string
}

// upcomingUploadRequirement is the requirement published at
// https://developer.apple.com/app-store/submitting/.
var upcomingUploadRequirement = uploadRequirement{
	effective:      time.Date(2027, time.April, 1, 0, 0, 0, 0, time.UTC),
	effectiveLabel: "April 2027",
	sdkMajor:       27,
	iosTargetMajor: 15,
	platformSDKs: map[string]string{
		"IOS":       "iOS and iPadOS",
		"TV_OS":     "tvOS",
		"VISION_OS": "visionOS",
	},
	watchSDK: "watchOS",
}

func (requirement uploadRequirement) timing(now time.Time) string {
	if now.Before(requirement.effective) {
		return "Starting " + requirement.effectiveLabel
	}
	return "Since " + requirement.effectiveLabel
}

// UploadRequirementApplies reports whether the upload requirement names the
// App Store version platform.
func UploadRequirementApplies(platform string) bool {
	_, ok := upcomingUploadRequirement.platformSDKs[normalizePlatform(platform)]
	return ok
}

func normalizePlatform(platform string) string {
	return strings.ToUpper(strings.TrimSpace(platform))
}

// uploadRequirementChecks compares the attached build with the requirement.
//
// Build.minOsVersion proves the deployment target. The public API has no SDK
// version: BuildBundle.sdkBuild is a build identifier such as 23A339, and Apple
// publishes no mapping from it to an SDK version. sdkName is the binary's
// DTSDKName when the caller inspected one. Without it the SDK stays unverified,
// which is informational so a requirement that cannot be proven never blocks
// --strict.
func uploadRequirementChecks(platform string, build *Build, sdkName string, now time.Time) []CheckResult {
	requirement := upcomingUploadRequirement
	normalizedPlatform := normalizePlatform(platform)
	platformSDK, applies := requirement.platformSDKs[normalizedPlatform]
	if build == nil || !applies {
		return nil
	}

	var checks []CheckResult
	if normalizedPlatform == "IOS" {
		checks = append(checks, deploymentTargetChecks(requirement, build, now)...)
	}
	return append(checks, sdkChecks(requirement, normalizedPlatform, platformSDK, build, sdkName, now)...)
}

func deploymentTargetChecks(requirement uploadRequirement, build *Build, now time.Time) []CheckResult {
	target := strings.TrimSpace(build.MinOSVersion)
	rule := fmt.Sprintf("%s, iOS and iPadOS apps uploaded to App Store Connect must target iOS %d or later", requirement.timing(now), requirement.iosTargetMajor)
	check := CheckResult{Field: "minOsVersion", ResourceType: "build", ResourceID: strings.TrimSpace(build.ID)}

	major, ok := leadingVersionMajor(target)
	switch {
	case !ok:
		check.ID = CheckUploadRequirementMinOSUnverified
		check.Severity = SeverityInfo
		check.Message = rule + "; App Store Connect did not report a readable deployment target for this build"
		check.Remediation = "Confirm the build's deployment target in Xcode"
	case major < requirement.iosTargetMajor:
		check.ID = CheckUploadRequirementMinOS
		check.Severity = SeverityWarning
		check.Message = fmt.Sprintf("%s; this build targets iOS %s", rule, target)
		check.Remediation = fmt.Sprintf("Raise the deployment target to iOS %d or later before your next upload", requirement.iosTargetMajor)
	default:
		return nil
	}
	return []CheckResult{check}
}

func sdkChecks(requirement uploadRequirement, platform, platformSDK string, build *Build, sdkName string, now time.Time) []CheckResult {
	buildID := strings.TrimSpace(build.ID)
	timing := requirement.timing(now)
	required := func(sdk string) string {
		return fmt.Sprintf("the %s %d SDK", sdk, requirement.sdkMajor)
	}

	var checks []CheckResult
	var unproven []string
	sdkName = strings.TrimSpace(sdkName)
	major, proven := sdkNameMajor(sdkName)
	switch {
	case !proven:
		unproven = append(unproven, required(platformSDK))
	case major < requirement.sdkMajor:
		checks = append(checks, CheckResult{
			ID:           CheckUploadRequirementSDK,
			Severity:     SeverityWarning,
			ResourceType: "build",
			ResourceID:   buildID,
			Message:      fmt.Sprintf("%s, uploads to App Store Connect must be built with %s or later; this binary was built with %s", timing, required(platformSDK), sdkName),
			Remediation:  fmt.Sprintf("Build with Xcode %d or later before your next upload", requirement.sdkMajor),
		})
	}

	var evidence []string
	for _, bundle := range build.Bundles {
		if strings.EqualFold(strings.TrimSpace(bundle.Type), "WATCH_APP") && !slices.Contains(unproven, required(requirement.watchSDK)) {
			unproven = append(unproven, required(requirement.watchSDK))
		}
		if sdkBuild := strings.TrimSpace(bundle.SDKBuild); sdkBuild != "" {
			evidence = append(evidence, strings.TrimSpace(bundle.Type+" "+sdkBuild))
		}
	}

	unreadReason := strings.TrimSpace(build.BundlesUnreadReason)
	if platform != "IOS" {
		unreadReason = ""
	}
	if len(unproven) == 0 && unreadReason == "" {
		return checks
	}

	if unreadReason != "" && !slices.Contains(unproven, required(requirement.watchSDK)) {
		unproven = append(unproven, required(requirement.watchSDK)+" for any watchOS app")
	}
	message := fmt.Sprintf("%s, uploads to App Store Connect must be built with %s or later; the App Store Connect API reports no SDK version for this build", timing, strings.Join(unproven, " and "))
	if len(evidence) > 0 {
		message += fmt.Sprintf(" (sdkBuild %s is a build identifier, not an SDK version)", strings.Join(evidence, ", "))
	}
	if unreadReason != "" {
		message += ". " + unreadReason
	}

	remediation := "Confirm the SDK in Xcode or in the archive's DTSDKName"
	if platform == "IOS" && sdkName == "" {
		remediation += ", or rerun with --ipa PATH to read it from the binary"
	}
	return append(checks, CheckResult{
		ID:           CheckUploadRequirementSDKUnverified,
		Severity:     SeverityInfo,
		ResourceType: "build",
		ResourceID:   buildID,
		Message:      message,
		Remediation:  remediation,
	})
}

var sdkNamePattern = regexp.MustCompile(`^[A-Za-z]+([0-9]+)`)

// sdkNameMajor reads the SDK major version from a DTSDKName such as
// iphoneos26.4.
func sdkNameMajor(sdkName string) (int, bool) {
	match := sdkNamePattern.FindStringSubmatch(sdkName)
	if match == nil {
		return 0, false
	}
	major, err := strconv.Atoi(match[1])
	return major, err == nil
}

func leadingVersionMajor(version string) (int, bool) {
	head, _, _ := strings.Cut(version, ".")
	major, err := strconv.Atoi(head)
	return major, err == nil
}
