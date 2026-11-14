package artifacts

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/artifacts/artifactstest"
)

func signedSlice(t *testing.T, chain artifactstest.Chain, cpuType, cpuSubtype uint32) artifactstest.FatSlice {
	t.Helper()
	return artifactstest.FatSlice{CPUType: cpuType, CPUSubtype: cpuSubtype, Data: chain.SignedExecutable(t)}
}

func inspectExecutable(t *testing.T, executable []byte) IPAManifest {
	t.Helper()
	ipa := signedIPA(t, executable, nil)
	manifest, err := InspectIPAWithOptions(bytes.NewReader(ipa), int64(len(ipa)), IPAOptions{IncludeEntitlements: false, IncludeProfile: false})
	if err != nil {
		t.Fatal(err)
	}
	return manifest
}

func architectureNames(architectures []ArchitectureSignature) string {
	names := make([]string, 0, len(architectures))
	for _, architecture := range architectures {
		names = append(names, architecture.Arch)
	}
	return strings.Join(names, ",")
}

func TestInspectIPAReportsEveryUniversalSliceWithIdenticalSigners(t *testing.T) {
	chain := artifactstest.NewChain(t)
	for _, wide := range []bool{false, true} {
		name := "fat32"
		if wide {
			name = "fat64"
		}
		t.Run(name, func(t *testing.T) {
			manifest := inspectExecutable(t, artifactstest.UniversalMachO(
				wide,
				signedSlice(t, chain, artifactstest.CPUTypeARM64, 0),
				signedSlice(t, chain, artifactstest.CPUTypeX86_64, 3),
			))
			if architectureNames(manifest.Architectures) != "arm64,x86_64" {
				t.Fatalf("architectures=%+v", manifest.Architectures)
			}
			for _, architecture := range manifest.Architectures {
				if architecture.CodeSignature != "signed" || architecture.Signer == nil || architecture.Signer.SHA256Fingerprint != manifest.Signer.SHA256Fingerprint || architecture.CodeSignatureError != "" {
					t.Fatalf("architecture=%+v", architecture)
				}
			}
			if manifest.Architectures[1].CPUType != artifactstest.CPUTypeX86_64 || manifest.Architectures[1].CPUSubtype != 3 {
				t.Fatalf("x86_64 slice=%+v", manifest.Architectures[1])
			}
			if manifest.SignerConsistent == nil || !*manifest.SignerConsistent || manifest.CodeSignature != "signed" {
				t.Fatalf("manifest=%+v", manifest)
			}
		})
	}
}

func TestInspectIPAFlagsUniversalSlicesWithDifferentSigners(t *testing.T) {
	first, second := artifactstest.NewChain(t), artifactstest.NewChain(t)
	manifest := inspectExecutable(t, artifactstest.UniversalMachO(
		false,
		signedSlice(t, first, artifactstest.CPUTypeARM64, 0),
		signedSlice(t, second, artifactstest.CPUTypeX86_64, 3),
	))
	if len(manifest.Architectures) != 2 || manifest.Architectures[0].Signer.SHA256Fingerprint == manifest.Architectures[1].Signer.SHA256Fingerprint {
		t.Fatalf("architectures=%+v", manifest.Architectures)
	}
	if manifest.SignerConsistent == nil || *manifest.SignerConsistent {
		t.Fatalf("signerConsistent=%v", manifest.SignerConsistent)
	}
	// The top-level signer stays the lowest-offset slice.
	if manifest.Signer == nil || manifest.Signer.SHA256Fingerprint != manifest.Architectures[0].Signer.SHA256Fingerprint {
		t.Fatalf("signer=%+v", manifest.Signer)
	}
}

func TestInspectIPAFlagsUnsignedUniversalSlice(t *testing.T) {
	chain := artifactstest.NewChain(t)
	manifest := inspectExecutable(t, artifactstest.UniversalMachO(
		true,
		signedSlice(t, chain, artifactstest.CPUTypeARM64, 0),
		artifactstest.FatSlice{CPUType: artifactstest.CPUTypeX86_64, CPUSubtype: 3, Data: artifactstest.MachO(nil)},
	))
	if len(manifest.Architectures) != 2 || manifest.Architectures[1].CodeSignature != "unsigned" || manifest.Architectures[1].Signer != nil {
		t.Fatalf("architectures=%+v", manifest.Architectures)
	}
	if manifest.SignerConsistent == nil || *manifest.SignerConsistent || manifest.CodeSignature != "signed" {
		t.Fatalf("manifest=%+v", manifest)
	}
}

func TestInspectIPAListsFatSlicesInTableOrderAndKeepsLowestOffsetTopLevel(t *testing.T) {
	cms := artifactstest.NewChain(t).CMS(t)
	// FatMachO lists the unsigned slice first but stores the signed one first.
	manifest := inspectExecutable(t, artifactstest.FatMachO(artifactstest.MachO(artifactstest.Superblob(artifactstest.CodeDirectorySlot(), artifactstest.CMSSlot(cms))), artifactstest.MachO(nil)))
	if len(manifest.Architectures) != 2 || manifest.Architectures[0].CodeSignature != "unsigned" || manifest.Architectures[1].CodeSignature != "signed" {
		t.Fatalf("architectures=%+v", manifest.Architectures)
	}
	if manifest.CodeSignature != "signed" || manifest.Signer == nil || *manifest.SignerConsistent {
		t.Fatalf("manifest=%+v", manifest)
	}
}

func TestInspectIPAReportsThinExecutableAsOneArchitecture(t *testing.T) {
	manifest := inspectExecutable(t, artifactstest.NewChain(t).SignedExecutable(t))
	if len(manifest.Architectures) != 1 {
		t.Fatalf("architectures=%+v", manifest.Architectures)
	}
	architecture := manifest.Architectures[0]
	if architecture.Arch != "arm64" || architecture.CPUType != artifactstest.CPUTypeARM64 || architecture.CodeSignature != "signed" || architecture.Signer == nil {
		t.Fatalf("architecture=%+v", architecture)
	}
	if manifest.SignerConsistent == nil || !*manifest.SignerConsistent {
		t.Fatalf("signerConsistent=%v", manifest.SignerConsistent)
	}
}

func TestInspectIPAMarksMalformedUniversalSliceUnreadable(t *testing.T) {
	chain := artifactstest.NewChain(t)
	manifest := inspectExecutable(t, artifactstest.UniversalMachO(
		false,
		signedSlice(t, chain, artifactstest.CPUTypeARM64, 0),
		artifactstest.FatSlice{CPUType: artifactstest.CPUTypeX86_64, CPUSubtype: 3, Data: []byte("not a Mach-O slice at all")},
		signedSlice(t, chain, artifactstest.CPUTypeARM64, 0x80000002),
	))
	if architectureNames(manifest.Architectures) != "arm64,x86_64,arm64e" {
		t.Fatalf("architectures=%+v", manifest.Architectures)
	}
	broken := manifest.Architectures[1]
	if broken.CodeSignature != "unreadable" || broken.Signer != nil || !strings.Contains(broken.CodeSignatureError, "not a Mach-O") {
		t.Fatalf("broken=%+v", broken)
	}
	if manifest.Architectures[2].CodeSignature != "signed" {
		t.Fatalf("slice after the malformed one=%+v", manifest.Architectures[2])
	}
	if manifest.CodeSignature != "signed" || manifest.CodeSignatureError != "" || *manifest.SignerConsistent {
		t.Fatalf("manifest=%+v", manifest)
	}
}

func TestInspectIPARejectsOverlappingUniversalSlices(t *testing.T) {
	chain := artifactstest.NewChain(t)
	executable := artifactstest.UniversalMachO(
		false,
		signedSlice(t, chain, artifactstest.CPUTypeARM64, 0),
		signedSlice(t, chain, artifactstest.CPUTypeX86_64, 3),
	)
	// Point the second slice into the first.
	binary.BigEndian.PutUint32(executable[8+20+8:], artifactstest.UniversalAlignment+16)
	manifest := inspectExecutable(t, executable)
	if manifest.CodeSignature != "unreadable" || !strings.Contains(manifest.CodeSignatureError, "overlap") || manifest.Architectures != nil || manifest.SignerConsistent != nil {
		t.Fatalf("manifest=%+v", manifest)
	}
}

func TestInspectIPAStoredUniversalExecutableSkipsCodePages(t *testing.T) {
	chain := artifactstest.NewChain(t)
	padded := func(cpuType uint32) artifactstest.FatSlice {
		executable := chain.SignedExecutable(t)
		const padding = 8 << 20
		signature := executable[4096:]
		executable = append(append(executable[:4096:4096], make([]byte, padding)...), signature...)
		binary.LittleEndian.PutUint32(executable[40:44], 4096+padding)
		return artifactstest.FatSlice{CPUType: cpuType, Data: executable}
	}
	fat := artifactstest.UniversalMachO(false, padded(artifactstest.CPUTypeARM64), padded(artifactstest.CPUTypeX86_64))
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	for name, data := range map[string][]byte{
		"Payload/Demo.app/Info.plist": plistXML(t, map[string]any{"CFBundleIdentifier": "com.example.demo", "CFBundleExecutable": "Demo"}),
		"Payload/Demo.app/Demo":       fat,
	} {
		entry, err := writer.CreateHeader(&zip.FileHeader{Name: name, Method: zip.Store})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	reader := &artifactReadBudget{ReaderAt: bytes.NewReader(buffer.Bytes()), remaining: 1 << 20}
	manifest, err := InspectIPAWithOptions(reader, int64(buffer.Len()), IPAOptions{IncludeEntitlements: false, IncludeProfile: false})
	if err != nil || len(manifest.Architectures) != 2 || manifest.Architectures[1].CodeSignature != "signed" || !*manifest.SignerConsistent {
		t.Fatalf("manifest=%+v err=%v", manifest, err)
	}
}

func TestArchitectureName(t *testing.T) {
	tests := []struct {
		cpuType, cpuSubtype uint32
		want                string
	}{
		{0x0100000c, 0, "arm64"},
		{0x0100000c, 1, "arm64"},
		{0x0100000c, 0x80000002, "arm64e"},
		{0x0100000c, 3, "arm64.x1"},
		{0x0100000c, 12, "arm64e.x1"},
		{0x0100000c, 5, "unknown"},
		{0x0200000c, 1, "arm64_32"},
		{0x01000007, 3, "x86_64"},
		{0x01000007, 8, "x86_64h"},
		{0x01000007, 4, "unknown"},
		{7, 3, "i386"},
		{12, 9, "armv7"},
		{12, 11, "armv7s"},
		{12, 12, "armv7k"},
		{12, 13, "unknown"},
		{0x12, 0, "ppc"},
		{0x01000012, 0, "unknown"},
		{99, 0, "unknown"},
	}
	for _, test := range tests {
		if got := architectureName(test.cpuType, test.cpuSubtype); got != test.want {
			t.Fatalf("architectureName(%#x, %#x)=%q want %q", test.cpuType, test.cpuSubtype, got, test.want)
		}
	}
}
