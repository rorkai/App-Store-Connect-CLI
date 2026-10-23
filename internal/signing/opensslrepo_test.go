package signing

import (
	"errors"
	"testing"
)

// The vectors were produced by the OpenSSL 4 command line (v1) and by Ruby's
// OpenSSL bindings (v2) for password "repo password", salt 0102030405060708,
// and plaintext "signing asset".
func TestDecryptOpenSSLRepositoryFileKnownAnswers(t *testing.T) {
	acceptAll := func([]byte) bool { return true }
	tests := []struct {
		name     string
		data     string
		password string
		accept   func([]byte) bool
		wantErr  bool
	}{
		{name: "v1 md5", data: "U2FsdGVkX18BAgMEBQYHCOASeE7VGc606dvRR5VO7wY=\n", password: "repo password", accept: acceptAll},
		{name: "v1 sha256", data: "U2FsdGVkX18BAgMEBQYHCA3n3srZWxt6+Z6UTcnBcso=", password: "repo password", accept: acceptAll},
		{name: "v2 with line breaks", data: "dnZ2dnZ2dnZ2dnZ2dnZ2dnZ2dnYBAgMEBQYHCHaC/cA5bdig0dfffgH3a/og\nqXiVlPkSXQ5IDcwE\n", password: "repo password", accept: acceptAll},
		{name: "v1 wrong password", data: "U2FsdGVkX18BAgMEBQYHCOASeE7VGc606dvRR5VO7wY=", password: "other", accept: acceptAll, wantErr: true},
		{name: "v2 wrong password", data: "dnZ2dnZ2dnZ2dnZ2dnZ2dnZ2dnYBAgMEBQYHCHaC/cA5bdig0dfffgH3a/ogqXiVlPkSXQ5IDcwE", password: "other", accept: acceptAll, wantErr: true},
		{name: "v1 plaintext rejected", data: "U2FsdGVkX18BAgMEBQYHCOASeE7VGc606dvRR5VO7wY=", password: "repo password", accept: func([]byte) bool { return false }, wantErr: true},
		{name: "not base64", data: "-----BEGIN CERTIFICATE-----", password: "repo password", accept: acceptAll, wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			plaintext, err := DecryptOpenSSLRepositoryFile([]byte(test.data), test.password, test.accept)
			if test.wantErr {
				if !errors.Is(err, ErrOpenSSLRepositoryDecrypt) {
					t.Fatalf("error = %v, want ErrOpenSSLRepositoryDecrypt", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("decrypt: %v", err)
			}
			if string(plaintext) != "signing asset" {
				t.Fatalf("plaintext = %q", plaintext)
			}
		})
	}
}
