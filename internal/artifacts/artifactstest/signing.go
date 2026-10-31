// Package artifactstest builds synthetic signed IPA and flat package fixtures.
// Keys are generated per test and discarded; fixtures carry only certificates.
package artifactstest

import (
	"bytes"
	"compress/zlib"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/binary"
	"math/big"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"go.mozilla.org/pkcs7"
)

// TeamID is the team identifier in the synthetic signing certificate.
const TeamID = "ABCDE12345"

// SignerCommonName is the subject common name of the synthetic leaf.
const SignerCommonName = "Apple Distribution: Example Corp (" + TeamID + ")"

// IssuerCommonName is the subject common name of the synthetic intermediate.
const IssuerCommonName = "Test Worldwide Developer Relations"

// NotBefore is the start of the synthetic certificates' validity.
var NotBefore = time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)

// Chain is a throwaway leaf certificate, its key, and its issuer.
type Chain struct {
	Leaf         *x509.Certificate
	LeafKey      *ecdsa.PrivateKey
	Intermediate *x509.Certificate
}

// NewChain creates an intermediate and a code-signing leaf shaped like an
// Apple Distribution certificate.
func NewChain(t testing.TB) Chain {
	t.Helper()
	return NewChainWithIssuer(t, pkix.Name{CommonName: IssuerCommonName, Organization: []string{"Test Authority"}})
}

// NewChainWithIssuer is NewChain with the intermediate's subject chosen by the
// caller, which becomes the leaf's issuer name.
func NewChainWithIssuer(t testing.TB, issuer pkix.Name) Chain {
	t.Helper()
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	caTemplate := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               issuer,
		NotBefore:             NotBefore,
		NotAfter:              NotBefore.AddDate(5, 0, 0),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign,
	}
	ca := createCertificate(t, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	leafKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	leafTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(0x1234abcd),
		Subject: pkix.Name{
			CommonName:         SignerCommonName,
			OrganizationalUnit: []string{TeamID},
			Organization:       []string{"Example Corp"},
		},
		NotBefore:   NotBefore,
		NotAfter:    NotBefore.AddDate(1, 0, 0),
		KeyUsage:    x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageCodeSigning},
	}
	leaf := createCertificate(t, leafTemplate, ca, &leafKey.PublicKey, caKey)
	return Chain{Leaf: leaf, LeafKey: leafKey, Intermediate: ca}
}

func createCertificate(t testing.TB, template, parent *x509.Certificate, public, private any) *x509.Certificate {
	t.Helper()
	der, err := x509.CreateCertificate(rand.Reader, template, parent, public, private)
	if err != nil {
		t.Fatal(err)
	}
	certificate, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	return certificate
}

// CMS returns a detached SignedData blob like the one codesign embeds. The
// intermediate is listed first so leaf selection must follow the signer info.
func (chain Chain) CMS(t testing.TB) []byte {
	t.Helper()
	signed, err := pkcs7.NewSignedData([]byte("code directory hash"))
	if err != nil {
		t.Fatal(err)
	}
	signed.AddCertificate(chain.Intermediate)
	if err := signed.AddSigner(chain.Leaf, chain.LeafKey, pkcs7.SignerInfoConfig{}); err != nil {
		t.Fatal(err)
	}
	signed.Detach()
	data, err := signed.Finish()
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// SignedExecutable returns a thin Mach-O whose code signature carries CMS.
func (chain Chain) SignedExecutable(t testing.TB) []byte {
	t.Helper()
	return MachO(Superblob(CodeDirectorySlot(), CMSSlot(chain.CMS(t))))
}

// XarSignature returns a TOC signature element listing the leaf then its issuer.
func (chain Chain) XarSignature(element string) string {
	return `<` + element + ` style="RSA"><offset>0</offset><size>256</size><KeyInfo xmlns="http://www.w3.org/2000/09/xmldsig#"><X509Data><X509Certificate>` +
		wrapBase64(chain.Leaf.Raw) + `</X509Certificate><X509Certificate>` + wrapBase64(chain.Intermediate.Raw) +
		`</X509Certificate></X509Data></KeyInfo></` + element + `>`
}

// Slot is one entry in an embedded signature superblob.
type Slot struct {
	Kind uint32
	Blob []byte
}

// Superblob encodes an embedded signature superblob with the given slots.
func Superblob(slots ...Slot) []byte {
	header := 12 + 8*len(slots)
	var body bytes.Buffer
	index := make([]byte, 0, 8*len(slots))
	for _, slot := range slots {
		index = binary.BigEndian.AppendUint32(index, slot.Kind)
		index = binary.BigEndian.AppendUint32(index, uint32(header+body.Len()))
		body.Write(slot.Blob)
	}
	out := binary.BigEndian.AppendUint32(nil, 0xfade0cc0)
	out = binary.BigEndian.AppendUint32(out, uint32(header+body.Len()))
	out = binary.BigEndian.AppendUint32(out, uint32(len(slots)))
	out = append(out, index...)
	return append(out, body.Bytes()...)
}

// Blob encodes a generic code signing blob.
func Blob(magic uint32, payload []byte) []byte {
	out := binary.BigEndian.AppendUint32(nil, magic)
	out = binary.BigEndian.AppendUint32(out, uint32(8+len(payload)))
	return append(out, payload...)
}

// CodeDirectorySlot returns a placeholder code directory slot.
func CodeDirectorySlot() Slot {
	return Slot{Kind: 0, Blob: Blob(0xfade0c02, make([]byte, 36))}
}

// CMSSlot wraps CMS data in the signature slot. Nil produces the empty wrapper
// codesign writes for ad-hoc signatures.
func CMSSlot(cms []byte) Slot {
	return Slot{Kind: 0x10000, Blob: Blob(0xfade0b01, cms)}
}

// MachOSignatureOffset is where MachO places the code signature.
const MachOSignatureOffset = 4096

// MachO builds a thin little-endian arm64 executable. A nil signature omits
// LC_CODE_SIGNATURE and records an LC_UUID command instead.
func MachO(signature []byte) []byte {
	var out []byte
	out = binary.LittleEndian.AppendUint32(out, 0xfeedfacf)
	out = binary.LittleEndian.AppendUint32(out, 0x0100000c)
	out = binary.LittleEndian.AppendUint32(out, 0)
	out = binary.LittleEndian.AppendUint32(out, 2)
	out = binary.LittleEndian.AppendUint32(out, 1)
	if signature == nil {
		out = binary.LittleEndian.AppendUint32(out, 24)
		out = binary.LittleEndian.AppendUint32(out, 0)
		out = binary.LittleEndian.AppendUint32(out, 0)
		out = binary.LittleEndian.AppendUint32(out, 0x1b)
		out = binary.LittleEndian.AppendUint32(out, 24)
		return append(out, make([]byte, 16)...)
	}
	out = binary.LittleEndian.AppendUint32(out, 16)
	out = binary.LittleEndian.AppendUint32(out, 0)
	out = binary.LittleEndian.AppendUint32(out, 0)
	out = binary.LittleEndian.AppendUint32(out, 0x1d)
	out = binary.LittleEndian.AppendUint32(out, 16)
	out = binary.LittleEndian.AppendUint32(out, MachOSignatureOffset)
	out = binary.LittleEndian.AppendUint32(out, uint32(len(signature)))
	out = append(out, make([]byte, MachOSignatureOffset-len(out))...)
	return append(out, signature...)
}

// FatMachO places first before second in the file but lists second first in
// the architecture table, so readers must choose slices by file offset.
func FatMachO(first, second []byte) []byte {
	const firstOffset, secondOffset = 4096, 65536
	var out []byte
	out = binary.BigEndian.AppendUint32(out, 0xcafebabe)
	out = binary.BigEndian.AppendUint32(out, 2)
	for _, arch := range []struct {
		offset int
		data   []byte
	}{{secondOffset, second}, {firstOffset, first}} {
		out = binary.BigEndian.AppendUint32(out, 0x0100000c)
		out = binary.BigEndian.AppendUint32(out, 0)
		out = binary.BigEndian.AppendUint32(out, uint32(arch.offset))
		out = binary.BigEndian.AppendUint32(out, uint32(len(arch.data)))
		out = binary.BigEndian.AppendUint32(out, 12)
	}
	out = append(out, make([]byte, firstOffset-len(out))...)
	out = append(out, first...)
	out = append(out, make([]byte, secondOffset-len(out))...)
	return append(out, second...)
}

// CPU types written into universal fixture architecture tables.
const (
	CPUTypeARM64  uint32 = 0x0100000c
	CPUTypeX86_64 uint32 = 0x01000007
)

// FatSlice is one architecture of a universal Mach-O fixture.
type FatSlice struct {
	CPUType    uint32
	CPUSubtype uint32
	Data       []byte
}

// UniversalAlignment is the file alignment UniversalMachO uses for slices.
const UniversalAlignment = 16 << 10

// UniversalMachO lists slices in the architecture table in the given order and
// stores them in that order at UniversalAlignment boundaries. Wide selects the
// 64-bit fat header (FAT_MAGIC_64).
func UniversalMachO(wide bool, slices ...FatSlice) []byte {
	magic, entrySize := uint32(0xcafebabe), 20
	if wide {
		magic, entrySize = 0xcafebabf, 32
	}
	offsets := make([]int, len(slices))
	next := UniversalAlignment
	for index, slice := range slices {
		offsets[index] = next
		next += (len(slice.Data) + UniversalAlignment - 1) / UniversalAlignment * UniversalAlignment
	}
	var out []byte
	out = binary.BigEndian.AppendUint32(out, magic)
	out = binary.BigEndian.AppendUint32(out, uint32(len(slices)))
	for index, slice := range slices {
		entry := binary.BigEndian.AppendUint32(nil, slice.CPUType)
		entry = binary.BigEndian.AppendUint32(entry, slice.CPUSubtype)
		if wide {
			entry = binary.BigEndian.AppendUint64(entry, uint64(offsets[index]))
			entry = binary.BigEndian.AppendUint64(entry, uint64(len(slice.Data)))
			entry = binary.BigEndian.AppendUint32(entry, 14)
			entry = binary.BigEndian.AppendUint32(entry, 0)
		} else {
			entry = binary.BigEndian.AppendUint32(entry, uint32(offsets[index]))
			entry = binary.BigEndian.AppendUint32(entry, uint32(len(slice.Data)))
			entry = binary.BigEndian.AppendUint32(entry, 14)
		}
		out = append(out, entry[:entrySize]...)
	}
	for index, slice := range slices {
		out = append(out, make([]byte, offsets[index]-len(out))...)
		out = append(out, slice.Data...)
	}
	return out
}

// Xar builds a flat xar archive with uncompressed members. Extra raw XML, such
// as a signature element, is placed at the start of the table of contents.
func Xar(t testing.TB, files map[string][]byte, extraTOC string) []byte {
	t.Helper()
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	var heap, filesXML bytes.Buffer
	for id, name := range names {
		data := files[name]
		offset := heap.Len()
		heap.Write(data)
		filesXML.WriteString(`<file id="` + strconv.Itoa(id+1) + `"><name>` + name + `</name><type>file</type><data><length>` + strconv.Itoa(len(data)) + `</length><offset>` + strconv.Itoa(offset) + `</offset><size>` + strconv.Itoa(len(data)) + `</size><encoding style="application/octet-stream"/></data></file>`)
	}
	toc := []byte(`<xar><toc>` + extraTOC + filesXML.String() + `</toc></xar>`)
	var compressed bytes.Buffer
	encoder := zlib.NewWriter(&compressed)
	if _, err := encoder.Write(toc); err != nil {
		t.Fatal(err)
	}
	if err := encoder.Close(); err != nil {
		t.Fatal(err)
	}
	header := make([]byte, 28)
	copy(header[:4], "xar!")
	binary.BigEndian.PutUint16(header[4:6], 28)
	binary.BigEndian.PutUint16(header[6:8], 1)
	binary.BigEndian.PutUint64(header[8:16], uint64(compressed.Len()))
	binary.BigEndian.PutUint64(header[16:24], uint64(len(toc)))
	return append(append(header, compressed.Bytes()...), heap.Bytes()...)
}

func wrapBase64(data []byte) string {
	encoded := base64.StdEncoding.EncodeToString(data)
	var out strings.Builder
	for len(encoded) > 64 {
		out.WriteString(encoded[:64])
		out.WriteString("\n")
		encoded = encoded[64:]
	}
	out.WriteString(encoded)
	return out.String()
}
