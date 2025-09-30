package secure_test

import (
	"testing"

	"github.com/airenas/rt-transcriber-wrapper/internal/secure"
)

func TestCrypter_EncryptDecrypt(t *testing.T) {
	tests := []struct {
		name string
		data []byte
	}{
		{"simple", []byte("some data")},
		{"empty", []byte("")},
		{"long", []byte("some long data some long data some long data some long data some long data some long data some long data some long data")},
		{"nil", nil},
		{"non ascii", []byte("ñandú")},
		{"non writable", []byte{0xff, 0xfe, 0xfd, 0xfc, 0xfb, 0xfa, 0xf9, 0xf8, 0xf7, 0xf6, 0xf5, 0xf4, 0xf3, 0xf2, 0xf1, 0xf0}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := "testkey12345678901234567890123456"
			c, err := secure.NewCrypter(key)
			if err != nil {
				t.Fatalf("could not construct receiver type: %v", err)
			}
			encrypted, err := c.Encrypt(tt.data)
			if err != nil {
				t.Fatalf("Encrypt() failed: %v", err)
			}
			if string(encrypted) == string(tt.data) {
				t.Errorf("Not encrypted = %v, want %v", string(encrypted), string(tt.data))
			}
			decrypted, err := c.Decrypt(encrypted)
			if err != nil {
				t.Errorf("Decrypt() failed: %v", err)
				return
			}
			if string(decrypted) != string(tt.data) {
				t.Errorf("Decrypt() = %v, want %v", string(decrypted), string(tt.data))
			}
		})
	}
}

func TestNewCrypter(t *testing.T) {
	tests := []struct {
		name    string // description of this test case
		key     string
		wantErr bool
	}{
		{"valid 32", "12345678901234567890123456789012", false},
		{"valid 24", "123456789012345678901234", true},
		{"valid 16", "1234567890123456", true},
		{"too short", "1234567890", true},
		{"empty", "", true},
		{"valid >32", "12345678901234567890123456789012345", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, gotErr := secure.NewCrypter(tt.key)
			if gotErr != nil {
				if !tt.wantErr {
					t.Errorf("NewCrypter() failed: %v", gotErr)
				}
				return
			}
			if got == nil {
				t.Fatal("NewCrypter() returned nil")
			}
			if tt.wantErr {
				t.Fatal("NewCrypter() succeeded unexpectedly")
			}
		})
	}
}
