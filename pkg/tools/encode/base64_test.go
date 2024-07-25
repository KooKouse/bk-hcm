package encode

import (
	"bytes"
	"io"
	"testing"
)

func TestReaderToBase64Str(t *testing.T) {
	type args struct {
		reader io.Reader
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		// TODO: Add test cases.
		{
			name: "Happy Path",
			args: args{
				bytes.NewBufferString("Hello, World!"),
			},
			want:    "SGVsbG8sIHdvcmxkIQ==",
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ReaderToBase64Str(tt.args.reader)
			if (err != nil) != tt.wantErr {
				t.Errorf("ReaderToBase64Str() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ReaderToBase64Str() got = %v, want %v", got, tt.want)
			}
		})
	}
}
