package validation

import (
	"strings"
	"testing"
	"time"
)

var (
	beforeUploadRequirement = time.Date(2026, time.October, 31, 12, 0, 0, 0, time.UTC)
	afterUploadRequirement  = time.Date(2027, time.April, 1, 0, 0, 0, 0, time.UTC)
)

func TestUploadRequirementChecks(t *testing.T) {
	tests := []struct {
		name     string
		platform string
		build    *Build
		sdkName  string
		now      time.Time
		want     map[string]Severity
	}{
		{name: "no build", platform: "IOS"},
		{name: "macOS has no listed requirement", platform: "MAC_OS", build: &Build{ID: "b", MinOSVersion: "11.0"}},
		{
			name:     "old target is informational before the requirement",
			platform: "IOS",
			build:    &Build{ID: "b", MinOSVersion: "14.8"},
			want:     map[string]Severity{CheckUploadRequirementMinOS: SeverityInfo, CheckUploadRequirementSDKUnverified: SeverityInfo},
		},
		{
			name:     "old target warns once the requirement applies",
			platform: "IOS",
			build:    &Build{ID: "b", MinOSVersion: "14.8"},
			now:      afterUploadRequirement,
			want:     map[string]Severity{CheckUploadRequirementMinOS: SeverityWarning, CheckUploadRequirementSDKUnverified: SeverityInfo},
		},
		{
			name:     "target at the minimum never proves the SDK",
			platform: "IOS",
			build:    &Build{ID: "b", MinOSVersion: "15.0"},
			want:     map[string]Severity{CheckUploadRequirementSDKUnverified: SeverityInfo},
		},
		{
			name:     "missing target stays unverified",
			platform: "IOS",
			build:    &Build{ID: "b"},
			want:     map[string]Severity{CheckUploadRequirementMinOSUnverified: SeverityInfo, CheckUploadRequirementSDKUnverified: SeverityInfo},
		},
		{
			name:     "unreadable target stays unverified",
			platform: "IOS",
			build:    &Build{ID: "b", MinOSVersion: "next"},
			want:     map[string]Severity{CheckUploadRequirementMinOSUnverified: SeverityInfo, CheckUploadRequirementSDKUnverified: SeverityInfo},
		},
		{
			name:     "binary built with an old SDK is informational before the requirement",
			platform: "IOS",
			build:    &Build{ID: "b", MinOSVersion: "15.0"},
			sdkName:  "iphoneos26.4",
			want:     map[string]Severity{CheckUploadRequirementSDK: SeverityInfo},
		},
		{
			name:     "binary built with an old SDK warns once the requirement applies",
			platform: "IOS",
			build:    &Build{ID: "b", MinOSVersion: "15.0"},
			sdkName:  "iphoneos26.4",
			now:      afterUploadRequirement,
			want:     map[string]Severity{CheckUploadRequirementSDK: SeverityWarning},
		},
		{
			name:     "binary built with the required SDK passes",
			platform: "IOS",
			build:    &Build{ID: "b", MinOSVersion: "15.0"},
			sdkName:  "iphoneos27.0",
		},
		{
			name:     "unreadable binary SDK stays unverified",
			platform: "IOS",
			build:    &Build{ID: "b", MinOSVersion: "15.0"},
			sdkName:  "iphoneos",
			want:     map[string]Severity{CheckUploadRequirementSDKUnverified: SeverityInfo},
		},
		{
			name:     "a proven iOS SDK says nothing about the watchOS app",
			platform: "IOS",
			build:    &Build{ID: "b", MinOSVersion: "15.0", Bundles: []BuildBundle{{Type: "APP"}, {Type: "WATCH_APP", SDKBuild: "23R351"}}},
			sdkName:  "iphoneos27.0",
			want:     map[string]Severity{CheckUploadRequirementSDKUnverified: SeverityInfo},
		},
		{
			name:     "unread bundles leave a watchOS app unverified",
			platform: "IOS",
			build:    &Build{ID: "b", MinOSVersion: "15.0", BundlesUnreadReason: "forbidden"},
			sdkName:  "iphoneos27.0",
			want:     map[string]Severity{CheckUploadRequirementSDKUnverified: SeverityInfo},
		},
		{
			name:     "tvOS has no deployment target requirement",
			platform: "TV_OS",
			build:    &Build{ID: "b", MinOSVersion: "12.0"},
			want:     map[string]Severity{CheckUploadRequirementSDKUnverified: SeverityInfo},
		},
		{
			name:     "visionOS has no deployment target requirement",
			platform: "VISION_OS",
			build:    &Build{ID: "b", MinOSVersion: "1.0"},
			want:     map[string]Severity{CheckUploadRequirementSDKUnverified: SeverityInfo},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			now := test.now
			if now.IsZero() {
				now = beforeUploadRequirement
			}
			checks := uploadRequirementChecks(test.platform, test.build, test.sdkName, now)
			if len(checks) != len(test.want) {
				t.Fatalf("checks = %+v, want %v", checks, test.want)
			}
			for _, check := range checks {
				severity, ok := test.want[check.ID]
				if !ok || severity != check.Severity {
					t.Fatalf("check %s/%s, want %v", check.ID, check.Severity, test.want)
				}
				if check.ResourceType != "build" || check.ResourceID != "b" || check.Remediation == "" {
					t.Fatalf("check is not actionable: %+v", check)
				}
			}
		})
	}
}

func TestUploadRequirementChecksNameTheEvidence(t *testing.T) {
	checks := uploadRequirementChecks("IOS", &Build{
		ID:      "b",
		Bundles: []BuildBundle{{Type: "APP", SDKBuild: "23A339"}, {Type: "WATCH_APP", SDKBuild: "23R351"}},
	}, "", beforeUploadRequirement)

	var sdk *CheckResult
	for index := range checks {
		if checks[index].ID == CheckUploadRequirementSDKUnverified {
			sdk = &checks[index]
		}
	}
	if sdk == nil {
		t.Fatalf("missing SDK finding in %+v", checks)
	}
	for _, want := range []string{"iOS and iPadOS 27 SDK", "watchOS 27 SDK", "23A339", "23R351"} {
		if !strings.Contains(sdk.Message, want) {
			t.Fatalf("message %q does not contain %q", sdk.Message, want)
		}
	}
	if !strings.Contains(sdk.Remediation, "--ipa") {
		t.Fatalf("remediation %q does not offer --ipa", sdk.Remediation)
	}

	tvChecks := uploadRequirementChecks("TV_OS", &Build{ID: "b"}, "", beforeUploadRequirement)
	if len(tvChecks) != 1 || strings.Contains(tvChecks[0].Remediation, "--ipa") {
		t.Fatalf("--ipa applies only to IOS versions: %+v", tvChecks)
	}
}

func TestUploadRequirementWordingFollowsTheClock(t *testing.T) {
	build := &Build{ID: "b", MinOSVersion: "14.0"}
	tests := []struct {
		name string
		now  time.Time
		want string
	}{
		{name: "before", now: beforeUploadRequirement, want: "Starting April 2027"},
		{name: "last moment before", now: afterUploadRequirement.Add(-time.Second), want: "Starting April 2027"},
		{name: "on the month", now: afterUploadRequirement, want: "Since April 2027"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			checks := uploadRequirementChecks("IOS", build, "", test.now)
			if len(checks) == 0 || !strings.Contains(checks[0].Message, test.want) {
				t.Fatalf("checks = %+v, want %q", checks, test.want)
			}
		})
	}
}
