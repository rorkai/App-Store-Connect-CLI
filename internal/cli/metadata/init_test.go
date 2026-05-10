package metadata

import (
	"strings"
	"testing"
)

func TestMetadataInitCommandFlags(t *testing.T) {
	cmd := MetadataInitCommand()
	for _, name := range []string{"dir", "version", "locale", "force"} {
		if cmd.FlagSet.Lookup(name) == nil {
			t.Fatalf("expected --%s flag to be defined", name)
		}
	}
}

func TestMetadataInitCommandDefaultsLocale(t *testing.T) {
	cmd := MetadataInitCommand()
	f := cmd.FlagSet.Lookup("locale")
	if f == nil {
		t.Fatal("expected --locale flag to be defined")
	}
	if f.DefValue != "en-US" {
		t.Fatalf("expected --locale default en-US, got %q", f.DefValue)
	}
}

func TestMetadataInitCommandUsageMentionsVersionAndLocale(t *testing.T) {
	cmd := MetadataInitCommand()
	for _, want := range []string{"metadata init", "--version", "--locale"} {
		if !strings.Contains(cmd.ShortUsage, want) {
			t.Fatalf("expected ShortUsage to mention %q, got %q", want, cmd.ShortUsage)
		}
	}
}

func TestBuildInitWritePlansWritesBlankTemplates(t *testing.T) {
	plans, err := BuildInitWritePlans("/tmp/metadata", "en-US", "1.2.3")
	if err != nil {
		t.Fatalf("BuildInitWritePlans() error: %v", err)
	}
	if len(plans) != 2 {
		t.Fatalf("expected 2 plans, got %d", len(plans))
	}

	if got, want := plans[0].Path, "/tmp/metadata/app-info/en-US.json"; got != want {
		t.Fatalf("app-info path = %q, want %q", got, want)
	}
	wantAppInfo := `{"name":"","subtitle":"","privacyPolicyUrl":"","privacyChoicesUrl":"","privacyPolicyText":""}`
	if string(plans[0].Contents) != wantAppInfo {
		t.Fatalf("app-info template = %q, want %q", string(plans[0].Contents), wantAppInfo)
	}

	if got, want := plans[1].Path, "/tmp/metadata/version/1.2.3/en-US.json"; got != want {
		t.Fatalf("version path = %q, want %q", got, want)
	}
	wantVersion := `{"description":"","keywords":"","marketingUrl":"","promotionalText":"","supportUrl":"","whatsNew":""}`
	if string(plans[1].Contents) != wantVersion {
		t.Fatalf("version template = %q, want %q", string(plans[1].Contents), wantVersion)
	}
}

func TestBuildInitWritePlansCanWriteAppInfoOnly(t *testing.T) {
	plans, err := BuildInitWritePlans("/tmp/metadata", "en-US", "")
	if err != nil {
		t.Fatalf("BuildInitWritePlans() error: %v", err)
	}
	if len(plans) != 1 {
		t.Fatalf("expected 1 plan, got %d", len(plans))
	}
	if got, want := plans[0].Path, "/tmp/metadata/app-info/en-US.json"; got != want {
		t.Fatalf("app-info path = %q, want %q", got, want)
	}
}
