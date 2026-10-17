package artifacts

import (
	"bytes"
	"errors"
	"io/fs"
	"syscall"
	"testing"
)

// failingReaderAt simulates a storage failure while reading a valid file.
type failingReaderAt struct{ err error }

func (reader failingReaderAt) ReadAt([]byte, int64) (int, error) { return 0, reader.err }

func TestInspectIPAMarksOnlyContentFailuresAsInvalidArchive(t *testing.T) {
	garbage := []byte("junk\n")
	tests := []struct {
		name        string
		source      failingReaderAt
		data        []byte
		wantInvalid bool
	}{
		{name: "garbage archive", data: garbage, wantInvalid: true},
		{name: "read path error", source: failingReaderAt{err: &fs.PathError{Op: "read", Path: "App.ipa", Err: syscall.EIO}}},
		{name: "raw errno", source: failingReaderAt{err: syscall.EIO}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			for _, verify := range []bool{false, true} {
				var err error
				if test.data != nil {
					_, err = InspectIPAWithOptions(bytes.NewReader(test.data), int64(len(test.data)), IPAOptions{VerifySignature: verify})
				} else {
					_, err = InspectIPAWithOptions(test.source, 64<<10, IPAOptions{VerifySignature: verify})
				}
				if err == nil {
					t.Fatalf("verify=%t: InspectIPAWithOptions() error = nil, want failure", verify)
				}
				if got := errors.Is(err, ErrInvalidArchive); got != test.wantInvalid {
					t.Fatalf("verify=%t: errors.Is(%v, ErrInvalidArchive) = %t, want %t", verify, err, got, test.wantInvalid)
				}
				if !test.wantInvalid && !errors.Is(err, syscall.EIO) {
					t.Fatalf("verify=%t: error = %v, want the storage failure preserved", verify, err)
				}
			}
		})
	}
}
