package artifacts

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func readPrimaryMachOSignature(t *testing.T, data []byte) (string, *SignerIdentity) {
	t.Helper()
	slices, primary, err := readMachOSignaturesCapture(bytes.NewReader(data), int64(len(data)), nil)
	if err != nil || slices[primary].CodeSignatureError != "" {
		t.Fatalf("slices=%+v err=%v", slices, err)
	}
	return slices[primary].CodeSignature, slices[primary].Signer
}

// Apple's platform binaries carry a real BER-encoded code-signing CMS and a
// universal (fat) layout, which the synthetic fixtures only approximate.
func TestReadMachOSignatureParsesAppleSignedSystemBinary(t *testing.T) {
	data, err := os.ReadFile("/bin/ls")
	if err != nil {
		t.Skipf("system binary unavailable: %v", err)
	}
	status, signer := readPrimaryMachOSignature(t, data)
	if status != "signed" || signer == nil {
		t.Fatalf("status=%s signer=%+v", status, signer)
	}
	if !strings.Contains(signer.CommonName, "Software Signing") || !strings.Contains(signer.IssuerCommonName, "Apple") {
		t.Fatalf("signer=%+v", signer)
	}
}

// Every slice of a universal system binary must match lipo's architecture list
// and the leaf authority codesign reports for that slice.
func TestReadMachOSignaturesMatchesLipoAndCodesignPerSlice(t *testing.T) {
	lipo, lipoErr := exec.LookPath("lipo")
	codesign, codesignErr := exec.LookPath("codesign")
	if lipoErr != nil || codesignErr != nil {
		t.Skip("lipo or codesign is not installed")
	}
	const path = "/bin/ls"
	data, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("system binary unavailable: %v", err)
	}
	output, err := exec.Command(lipo, "-archs", path).Output()
	if err != nil {
		t.Skipf("lipo -archs: %v", err)
	}
	want := strings.Fields(string(output))
	if len(want) < 2 {
		t.Skipf("%s is not universal: %v", path, want)
	}
	slices, _, err := readMachOSignaturesCapture(bytes.NewReader(data), int64(len(data)), nil)
	if err != nil {
		t.Fatal(err)
	}
	got := make([]string, 0, len(slices))
	for _, slice := range slices {
		got = append(got, slice.Arch)
	}
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Fatalf("architectures=%v lipo=%v", got, want)
	}
	for _, slice := range slices {
		details, err := exec.Command(codesign, "-dvv", "--arch", slice.Arch, path).CombinedOutput()
		if err != nil {
			t.Fatalf("codesign --arch %s: %v: %s", slice.Arch, err, details)
		}
		authority := ""
		for line := range strings.SplitSeq(string(details), "\n") {
			if value, ok := strings.CutPrefix(line, "Authority="); ok {
				authority = value
				break
			}
		}
		if slice.CodeSignature != "signed" || slice.Signer == nil || slice.Signer.CommonName != authority {
			t.Fatalf("slice %s: %+v signer=%+v codesign authority=%q", slice.Arch, slice, slice.Signer, authority)
		}
	}
	if !signersConsistent(slices) {
		t.Fatalf("slices=%+v", slices)
	}
}

func TestReadMachOSignatureClassifiesCodesignOutput(t *testing.T) {
	codesign, err := exec.LookPath("codesign")
	if err != nil {
		t.Skip("codesign is not installed")
	}
	binary := filepath.Join(t.TempDir(), "tool")
	data, err := os.ReadFile("/bin/ls")
	if err != nil {
		t.Skipf("system binary unavailable: %v", err)
	}
	if err := os.WriteFile(binary, data, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, step := range []struct {
		args []string
		want string
	}{
		{[]string{"--remove-signature", binary}, "unsigned"},
		{[]string{"--force", "--sign", "-", binary}, "ad-hoc"},
	} {
		if output, err := exec.Command(codesign, step.args...).CombinedOutput(); err != nil {
			t.Skipf("codesign %v: %v: %s", step.args, err, output)
		}
		signed, err := os.ReadFile(binary)
		if err != nil {
			t.Fatal(err)
		}
		slices, _, err := readMachOSignaturesCapture(bytes.NewReader(signed), int64(len(signed)), nil)
		if err != nil {
			t.Fatalf("after %v: %v", step.args, err)
		}
		for _, slice := range slices {
			if slice.CodeSignature != step.want || slice.Signer != nil {
				t.Fatalf("after %v: slice=%+v", step.args, slice)
			}
		}
	}
}
