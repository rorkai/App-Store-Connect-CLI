package artifacts

import (
	"crypto/sha1" //nolint:gosec // SHA-1 matches the identity hash codesign and security print; it is not used for trust.
	"crypto/sha256"
	"crypto/x509"
	"encoding/asn1"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"go.mozilla.org/pkcs7"
)

// Signature statuses reported for a main executable or a flat package.
const (
	SignatureSigned     = "signed"
	SignatureAdHoc      = "ad-hoc"
	SignatureUnsigned   = "unsigned"
	SignatureUnreadable = "unreadable"
)

const (
	maxLoadCommandBytes   = 4 << 20
	maxFatArchitectures   = 32
	maxSuperblobSlots     = 256
	maxSignatureCMSBytes  = 256 << 10
	maxXarSignerCertCount = 32

	loadCommandCodeSignature = 0x1d

	magicEmbeddedSignature = 0xfade0cc0
	magicBlobWrapper       = 0xfade0b01
	slotCodeDirectory      = 0x0
	slotAlternateFirst     = 0x1000
	slotAlternateLast      = 0x1004
	slotCMSSignature       = 0x10000
)

// oidUserID is the subject attribute Apple repeats the team identifier in.
var oidUserID = asn1.ObjectIdentifier{0, 9, 2342, 19200300, 100, 1, 1}

// SignerIdentity describes the leaf certificate embedded in a signature. It is
// read from the artifact and is not validated against a trust store.
type SignerIdentity struct {
	CommonName               string
	TeamID                   string
	Organization             string
	IssuerCommonName         string
	IssuerOrganizationalUnit string
	// DeveloperIDAuthorityStatus is set only for Developer ID issuers.
	DeveloperIDAuthorityStatus string
	SerialNumber               string
	NotBefore                  string
	NotAfter                   string
	SHA1Fingerprint            string
	SHA256Fingerprint          string
}

// ArchitectureSignature is the code signature of one Mach-O architecture
// slice. CPUType and CPUSubtype are the raw values from the universal header,
// or from the Mach-O header of a thin executable.
type ArchitectureSignature struct {
	CPUType            uint32
	CPUSubtype         uint32
	Arch               string
	CodeSignature      string
	CodeSignatureError string
	Signer             *SignerIdentity
}

// forwardReader reads a stream in increasing offset order. Seekable sources skip
// without reading; others discard the skipped bytes. After an I/O failure the
// stream position is unknown, so every later call returns that failure.
type forwardReader struct {
	source io.Reader
	pos    int64
	size   int64
	err    error
	// capture, when set, receives the primary slice's code signature.
	capture *signatureCapture
}

func (reader *forwardReader) skipTo(offset int64) error {
	if reader.err != nil {
		return reader.err
	}
	if offset < reader.pos {
		return fmt.Errorf("Mach-O signature offsets are out of order")
	}
	if offset > reader.size {
		return fmt.Errorf("Mach-O offset %d is beyond the executable", offset)
	}
	distance := offset - reader.pos
	if seeker, ok := reader.source.(io.Seeker); ok {
		if _, err := seeker.Seek(distance, io.SeekCurrent); err != nil {
			reader.err = err
			return err
		}
	} else if _, err := io.CopyN(io.Discard, reader.source, distance); err != nil {
		reader.err = err
		return err
	}
	reader.pos = offset
	return nil
}

func (reader *forwardReader) read(length int64) ([]byte, error) {
	if reader.err != nil {
		return nil, reader.err
	}
	if length < 0 || length > reader.size-reader.pos {
		return nil, fmt.Errorf("Mach-O executable is truncated")
	}
	data := make([]byte, int(length))
	if _, err := io.ReadFull(reader.source, data); err != nil {
		reader.err = fmt.Errorf("Mach-O executable is truncated: %w", err)
		return nil, reader.err
	}
	reader.pos += length
	return data, nil
}

// readMachOSignatures classifies every architecture slice of a thin or
// universal Mach-O executable and extracts each CMS signer's leaf certificate.
// Slices are returned in universal header order; primary indexes the slice
// stored first. The source is consumed once, front to back, so slices are read
// in file order. A slice that cannot be read is reported as unreadable; an
// error means no slice could be identified.
func readMachOSignatures(source io.Reader, size int64) ([]ArchitectureSignature, int, error) {
	return readMachOSignaturesCapture(source, size, nil)
}

// readMachOSignaturesCapture is readMachOSignatures that also keeps the
// primary slice's code signature in capture when capture is non-nil.
func readMachOSignaturesCapture(source io.Reader, size int64, capture *signatureCapture) ([]ArchitectureSignature, int, error) {
	reader := &forwardReader{source: source, size: size, capture: capture}
	magic, err := reader.read(4)
	if err != nil {
		return nil, 0, err
	}
	switch binary.BigEndian.Uint32(magic) {
	case 0xcafebabe, 0xcafebabf:
		return readFatSignatures(reader, binary.BigEndian.Uint32(magic) == 0xcafebabf)
	}
	header, err := readThinHeader(reader, magic, reader.size)
	if err != nil {
		return nil, 0, err
	}
	slice := newArchitectureSignature(header.cpuType, header.cpuSubtype)
	slice.record(readThinSignature(reader, header, 0, reader.size))
	return []ArchitectureSignature{slice}, 0, nil
}

func newArchitectureSignature(cpuType, cpuSubtype uint32) ArchitectureSignature {
	return ArchitectureSignature{CPUType: cpuType, CPUSubtype: cpuSubtype, Arch: architectureName(cpuType, cpuSubtype)}
}

func (slice *ArchitectureSignature) record(status string, signer *SignerIdentity, err error) {
	if err != nil {
		slice.CodeSignature, slice.CodeSignatureError, slice.Signer = SignatureUnreadable, err.Error(), nil
		return
	}
	slice.CodeSignature, slice.Signer = status, signer
}

type fatArchitecture struct {
	cpuType, cpuSubtype uint32
	offset, size        int64
}

func readFatSignatures(reader *forwardReader, wide bool) ([]ArchitectureSignature, int, error) {
	countBytes, err := reader.read(4)
	if err != nil {
		return nil, 0, err
	}
	count := binary.BigEndian.Uint32(countBytes)
	if count == 0 || count > maxFatArchitectures {
		return nil, 0, fmt.Errorf("universal binary declares %d architectures", count)
	}
	entrySize := int64(20)
	if wide {
		entrySize = 32
	}
	table, err := reader.read(int64(count) * entrySize)
	if err != nil {
		return nil, 0, err
	}
	architectures := make([]fatArchitecture, count)
	for index := range architectures {
		entry := table[int64(index)*entrySize:]
		var offset, sliceSize uint64
		if wide {
			offset, sliceSize = binary.BigEndian.Uint64(entry[8:16]), binary.BigEndian.Uint64(entry[16:24])
		} else {
			offset, sliceSize = uint64(binary.BigEndian.Uint32(entry[8:12])), uint64(binary.BigEndian.Uint32(entry[12:16]))
		}
		if offset < uint64(reader.pos) || offset > uint64(reader.size) || sliceSize > uint64(reader.size)-offset {
			return nil, 0, fmt.Errorf("universal binary slice is outside the executable")
		}
		architectures[index] = fatArchitecture{
			cpuType:    binary.BigEndian.Uint32(entry[0:4]),
			cpuSubtype: binary.BigEndian.Uint32(entry[4:8]),
			offset:     int64(offset),
			size:       int64(sliceSize),
		}
	}
	// Read slices in file order so the source is consumed front to back.
	order := make([]int, count)
	for index := range order {
		order[index] = index
	}
	sort.SliceStable(order, func(a, b int) bool { return architectures[order[a]].offset < architectures[order[b]].offset })
	for index := 1; index < len(order); index++ {
		previous, next := architectures[order[index-1]], architectures[order[index]]
		if previous.offset+previous.size > next.offset {
			return nil, 0, fmt.Errorf("universal binary slices overlap")
		}
	}
	slices := make([]ArchitectureSignature, count)
	for position, index := range order {
		if position == 1 {
			// Only the primary slice, stored first, is captured for verification.
			reader.capture = nil
		}
		architecture := architectures[index]
		slices[index] = newArchitectureSignature(architecture.cpuType, architecture.cpuSubtype)
		slices[index].record(readFatSlice(reader, architecture))
	}
	return slices, order[0], nil
}

func readFatSlice(reader *forwardReader, architecture fatArchitecture) (string, *SignerIdentity, error) {
	if err := reader.skipTo(architecture.offset); err != nil {
		return "", nil, err
	}
	if architecture.size < 4 {
		return "", nil, fmt.Errorf("universal binary slice is truncated")
	}
	magic, err := reader.read(4)
	if err != nil {
		return "", nil, err
	}
	header, err := readThinHeader(reader, magic, architecture.size)
	if err != nil {
		return "", nil, err
	}
	return readThinSignature(reader, header, architecture.offset, architecture.size)
}

type thinHeader struct {
	order                      binary.ByteOrder
	size                       int64
	cpuType, cpuSubtype        uint32
	commandCount, commandBytes int64
}

// readThinHeader reads a Mach-O header whose magic has already been consumed.
// Every read stays inside the slice.
func readThinHeader(reader *forwardReader, magic []byte, sliceSize int64) (thinHeader, error) {
	var header thinHeader
	switch {
	case binary.LittleEndian.Uint32(magic) == 0xfeedface:
		header.order, header.size = binary.LittleEndian, 28
	case binary.LittleEndian.Uint32(magic) == 0xfeedfacf:
		header.order, header.size = binary.LittleEndian, 32
	case binary.BigEndian.Uint32(magic) == 0xfeedface:
		header.order, header.size = binary.BigEndian, 28
	case binary.BigEndian.Uint32(magic) == 0xfeedfacf:
		header.order, header.size = binary.BigEndian, 32
	default:
		return header, fmt.Errorf("main executable is not a Mach-O file")
	}
	if header.size > sliceSize {
		return header, fmt.Errorf("Mach-O executable is truncated")
	}
	raw, err := reader.read(header.size - 4)
	if err != nil {
		return header, err
	}
	header.cpuType = header.order.Uint32(raw[0:4])
	header.cpuSubtype = header.order.Uint32(raw[4:8])
	header.commandCount = int64(header.order.Uint32(raw[12:16]))
	header.commandBytes = int64(header.order.Uint32(raw[16:20]))
	return header, nil
}

func readThinSignature(reader *forwardReader, header thinHeader, base, sliceSize int64) (string, *SignerIdentity, error) {
	order, headerSize := header.order, header.size
	commandCount, commandBytes := header.commandCount, header.commandBytes
	if commandBytes > maxLoadCommandBytes || commandBytes > sliceSize-headerSize || commandCount > commandBytes/8 {
		return "", nil, fmt.Errorf("Mach-O load commands exceed the read limit")
	}
	commands, err := reader.read(commandBytes)
	if err != nil {
		return "", nil, err
	}
	var dataOffset, dataSize int64
	found := false
	for index, offset := int64(0), int64(0); index < commandCount; index++ {
		if offset+8 > commandBytes {
			return "", nil, fmt.Errorf("Mach-O load command table is truncated")
		}
		command := order.Uint32(commands[offset:])
		commandSize := int64(order.Uint32(commands[offset+4:]))
		if commandSize < 8 || commandSize > commandBytes-offset {
			return "", nil, fmt.Errorf("Mach-O load command has an invalid size")
		}
		if command == loadCommandCodeSignature {
			if found {
				return "", nil, fmt.Errorf("Mach-O has multiple code signature commands")
			}
			if commandSize < 16 {
				return "", nil, fmt.Errorf("Mach-O code signature command is truncated")
			}
			found = true
			dataOffset = int64(order.Uint32(commands[offset+8:]))
			dataSize = int64(order.Uint32(commands[offset+12:]))
		}
		offset += commandSize
	}
	if !found {
		return SignatureUnsigned, nil, nil
	}
	if dataOffset < headerSize+commandBytes || dataSize < 12 || dataOffset > sliceSize || dataSize > sliceSize-dataOffset {
		return "", nil, fmt.Errorf("Mach-O code signature is outside the executable")
	}
	if err := reader.skipTo(base + dataOffset); err != nil {
		return "", nil, err
	}
	if reader.capture != nil {
		return captureSuperblob(reader, base, sliceSize, dataSize)
	}
	return readSuperblob(reader, dataSize)
}

func readSuperblob(reader *forwardReader, limit int64) (string, *SignerIdentity, error) {
	start := reader.pos
	header, err := reader.read(12)
	if err != nil {
		return "", nil, err
	}
	if binary.BigEndian.Uint32(header) != magicEmbeddedSignature {
		return "", nil, fmt.Errorf("code signature has an unknown superblob type")
	}
	length := int64(binary.BigEndian.Uint32(header[4:8]))
	count := int64(binary.BigEndian.Uint32(header[8:12]))
	if length < 12 || length > limit {
		return "", nil, fmt.Errorf("code signature superblob length is invalid")
	}
	if count > maxSuperblobSlots || 12+8*count > length {
		return "", nil, fmt.Errorf("code signature superblob index is invalid")
	}
	index, err := reader.read(8 * count)
	if err != nil {
		return "", nil, err
	}
	hasCodeDirectory := false
	cmsOffset := int64(-1)
	for slot := range count {
		kind := binary.BigEndian.Uint32(index[slot*8:])
		offset := int64(binary.BigEndian.Uint32(index[slot*8+4:]))
		if offset < 12+8*count || offset+8 > length {
			return "", nil, fmt.Errorf("code signature slot is outside the superblob")
		}
		switch {
		case kind == slotCodeDirectory || (kind >= slotAlternateFirst && kind <= slotAlternateLast):
			hasCodeDirectory = true
		case kind == slotCMSSignature:
			if cmsOffset >= 0 {
				return "", nil, fmt.Errorf("code signature has multiple CMS slots")
			}
			cmsOffset = offset
		}
	}
	if !hasCodeDirectory {
		return "", nil, fmt.Errorf("code signature has no code directory")
	}
	if cmsOffset < 0 {
		return SignatureAdHoc, nil, nil
	}
	if err := reader.skipTo(start + cmsOffset); err != nil {
		return "", nil, err
	}
	wrapper, err := reader.read(8)
	if err != nil {
		return "", nil, err
	}
	if binary.BigEndian.Uint32(wrapper) != magicBlobWrapper {
		return "", nil, fmt.Errorf("code signature CMS slot has an unknown blob type")
	}
	blobLength := int64(binary.BigEndian.Uint32(wrapper[4:8]))
	if blobLength < 8 || blobLength > length-cmsOffset {
		return "", nil, fmt.Errorf("code signature CMS blob length is invalid")
	}
	if blobLength == 8 {
		// codesign writes an empty wrapper for ad-hoc signatures.
		return SignatureAdHoc, nil, nil
	}
	if blobLength-8 > maxSignatureCMSBytes {
		return "", nil, fmt.Errorf("code signature CMS blob exceeds %d bytes", maxSignatureCMSBytes)
	}
	cms, err := reader.read(blobLength - 8)
	if err != nil {
		return "", nil, err
	}
	signer, err := signerFromCMS(cms)
	if err != nil {
		return "", nil, err
	}
	return SignatureSigned, signer, nil
}

func signerFromCMS(data []byte) (signer *SignerIdentity, err error) {
	defer func() {
		// The CMS parser indexes untrusted BER; keep a malformed blob from
		// taking down an otherwise readable inspection.
		if recovered := recover(); recovered != nil {
			signer, err = nil, fmt.Errorf("parse code signature CMS: malformed input")
		}
	}()
	parsed, err := pkcs7.Parse(data)
	if err != nil {
		return nil, fmt.Errorf("parse code signature CMS: %w", err)
	}
	leaf := parsed.GetOnlySigner()
	if leaf == nil {
		return nil, fmt.Errorf("code signature CMS does not name exactly one signer certificate")
	}
	return signerIdentity(leaf), nil
}

func signerIdentity(certificate *x509.Certificate) *SignerIdentity {
	sha1Sum := sha1.Sum(certificate.Raw) //nolint:gosec // identity hash, not a trust decision.
	sha256Sum := sha256.Sum256(certificate.Raw)
	teamID := ""
	if len(certificate.Subject.OrganizationalUnit) > 0 {
		teamID = certificate.Subject.OrganizationalUnit[0]
	}
	if teamID == "" {
		for _, name := range certificate.Subject.Names {
			if value, ok := name.Value.(string); ok && name.Type.Equal(oidUserID) {
				teamID = value
				break
			}
		}
	}
	organization := ""
	if len(certificate.Subject.Organization) > 0 {
		organization = certificate.Subject.Organization[0]
	}
	issuerUnit := ""
	if len(certificate.Issuer.OrganizationalUnit) > 0 {
		issuerUnit = certificate.Issuer.OrganizationalUnit[0]
	}
	serial := ""
	if certificate.SerialNumber != nil {
		serial = strings.ToUpper(certificate.SerialNumber.Text(16))
	}
	return &SignerIdentity{
		CommonName:                 certificate.Subject.CommonName,
		TeamID:                     teamID,
		Organization:               organization,
		IssuerCommonName:           certificate.Issuer.CommonName,
		IssuerOrganizationalUnit:   issuerUnit,
		DeveloperIDAuthorityStatus: DeveloperIDAuthorityStatus(certificate.Issuer),
		SerialNumber:               serial,
		NotBefore:                  certificate.NotBefore.UTC().Format(time.RFC3339),
		NotAfter:                   certificate.NotAfter.UTC().Format(time.RFC3339),
		SHA1Fingerprint:            fmt.Sprintf("%X", sha1Sum),
		SHA256Fingerprint:          fmt.Sprintf("%X", sha256Sum),
	}
}

type xarSignature struct {
	xarHeapRange
	Certificates []string `xml:"KeyInfo>X509Data>X509Certificate"`
}

// xarSigner reads the leaf certificate from a xar table of contents. productsign
// writes the signing certificate first, followed by its issuers.
func xarSigner(signatures ...*xarSignature) (string, *SignerIdentity, error) {
	for _, signature := range signatures {
		if signature == nil {
			continue
		}
		if len(signature.Certificates) == 0 {
			return SignatureUnreadable, nil, fmt.Errorf("package signature has no certificates")
		}
		if len(signature.Certificates) > maxXarSignerCertCount {
			return SignatureUnreadable, nil, fmt.Errorf("package signature lists more than %d certificates", maxXarSignerCertCount)
		}
		encoded := strings.Join(strings.Fields(signature.Certificates[0]), "")
		der, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			return SignatureUnreadable, nil, fmt.Errorf("decode package signing certificate: %w", err)
		}
		certificate, err := x509.ParseCertificate(der)
		if err != nil {
			return SignatureUnreadable, nil, fmt.Errorf("parse package signing certificate: %w", err)
		}
		return SignatureSigned, signerIdentity(certificate), nil
	}
	return SignatureUnsigned, nil, nil
}
