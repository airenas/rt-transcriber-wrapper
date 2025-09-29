package service

import (
	"encoding/base64"
	"testing"
)

func Test_extractUserTxt(t *testing.T) {
	tests := []struct {
		name string // description of this test case
		// Named input parameters for target function.
		txt     string
		want    *user
		wantErr bool
	}{
		{name: "ok",
			txt: base64.StdEncoding.EncodeToString([]byte(`{"id":"123"}`)),
			want: &user{
				ID: "123",
			},
		},
		{name: "bad json",
			txt:     base64.StdEncoding.EncodeToString([]byte(`{"id":"123"`)),
			wantErr: true,
		},
		{name: "no id",
			txt:     base64.RawStdEncoding.EncodeToString([]byte(`{}`)),
			wantErr: true,
		},
		{name: "bad base64",
			txt:     "%%%$$$",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, gotErr := extractUserTxt(tt.txt)
			if gotErr != nil {
				if !tt.wantErr {
					t.Errorf("extractUserTxt() failed: %v", gotErr)
				}
				return
			}
			if tt.wantErr {
				t.Fatal("extractUserTxt() succeeded unexpectedly")
			}
			if got.ID != tt.want.ID {
				t.Errorf("extractUserTxt() = %v, want %v", got, tt.want)
			}
		})
	}
}
