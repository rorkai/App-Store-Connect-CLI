package artifacts

import (
	"bytes"
	"reflect"
	"strings"
	"testing"
)

func TestInspectIPAInfoPlistReadsDeviceFamilies(t *testing.T) {
	tests := []struct {
		name  string
		value any
		want  []int
	}{
		{name: "universal integers", value: []any{1, 2}, want: []int{1, 2}},
		{name: "iphone only", value: []any{1}, want: []int{1}},
		{name: "numeric strings", value: []any{"1", "2"}, want: []int{1, 2}},
		{name: "single integer", value: 2, want: []int{2}},
		{name: "absent", value: nil, want: nil},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			info := map[string]any{"CFBundleIdentifier": "com.example.demo"}
			if test.value != nil {
				info["UIDeviceFamily"] = test.value
			}
			ipa := zipArtifact(t, map[string][]byte{"Payload/Demo.app/Info.plist": plistXML(t, info)})

			manifest, err := InspectIPAInfoPlist(bytes.NewReader(ipa), int64(len(ipa)))
			if err != nil {
				t.Fatal(err)
			}
			if manifest.BundleID != "com.example.demo" {
				t.Fatalf("bundle ID = %q", manifest.BundleID)
			}
			if !reflect.DeepEqual(manifest.DeviceFamilies, test.want) {
				t.Fatalf("device families = %#v, want %#v", manifest.DeviceFamilies, test.want)
			}
		})
	}
}

func TestInspectIPAInfoPlistRejectsMalformedDeviceFamily(t *testing.T) {
	tests := []struct {
		name  string
		value any
	}{
		{name: "non-numeric entry", value: []any{"iphone"}},
		{name: "empty array", value: []any{}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ipa := zipArtifact(t, map[string][]byte{
				"Payload/Demo.app/Info.plist": plistXML(t, map[string]any{
					"CFBundleIdentifier": "com.example.demo",
					"UIDeviceFamily":     test.value,
				}),
			})

			_, err := InspectIPAInfoPlist(bytes.NewReader(ipa), int64(len(ipa)))
			if err == nil || !strings.Contains(err.Error(), "UIDeviceFamily") {
				t.Fatalf("expected UIDeviceFamily error, got %v", err)
			}
		})
	}
}

func TestInspectIPAInfoPlistRequiresTopLevelApp(t *testing.T) {
	ipa := zipArtifact(t, map[string][]byte{"Payload/readme.txt": []byte("no app")})

	_, err := InspectIPAInfoPlist(bytes.NewReader(ipa), int64(len(ipa)))
	if err == nil || !strings.Contains(err.Error(), "no top-level app Info.plist") {
		t.Fatalf("expected missing Info.plist error, got %v", err)
	}
}

func TestInspectIPAToleratesMalformedDeviceFamily(t *testing.T) {
	ipa := zipArtifact(t, map[string][]byte{
		"Payload/Demo.app/Info.plist": plistXML(t, map[string]any{
			"CFBundleIdentifier": "com.example.demo",
			"UIDeviceFamily":     map[string]any{"bad": true},
		}),
	})

	manifest, err := InspectIPAWithOptions(bytes.NewReader(ipa), int64(len(ipa)), IPAOptions{IncludeEntitlements: false, IncludeProfile: false})
	if err != nil {
		t.Fatal(err)
	}
	if manifest.BundleID != "com.example.demo" || manifest.DeviceFamilies != nil {
		t.Fatalf("manifest = %+v", manifest)
	}
}
