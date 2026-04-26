package protocol

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"strings"
	"testing"
)

func TestWriteMessage(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		body    []byte
		wantErr error
	}{
		{name: "empty body", body: []byte{}},
		{name: "small body", body: []byte(`{"a":1}`)},
		{name: "exact max size", body: bytes.Repeat([]byte{'x'}, MaxMessageBytes)},
		{name: "exceeds max", body: bytes.Repeat([]byte{'x'}, MaxMessageBytes+1), wantErr: ErrMessageTooLarge},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var buf bytes.Buffer
			err := WriteMessage(&buf, tt.body)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if tt.wantErr != nil {
				return
			}
			if buf.Len() != 4+len(tt.body) {
				t.Fatalf("buf len = %d, want %d", buf.Len(), 4+len(tt.body))
			}
			gotLen := binary.LittleEndian.Uint32(buf.Bytes()[:4])
			if gotLen != uint32(len(tt.body)) {
				t.Fatalf("prefix = %d, want %d", gotLen, len(tt.body))
			}
			if !bytes.Equal(buf.Bytes()[4:], tt.body) {
				t.Fatalf("body mismatch")
			}
		})
	}
}

func TestReadMessage(t *testing.T) {
	t.Parallel()

	makeFrame := func(n uint32, body []byte) []byte {
		var lenBuf [4]byte
		binary.LittleEndian.PutUint32(lenBuf[:], n)
		return append(lenBuf[:], body...)
	}

	tests := []struct {
		name    string
		input   []byte
		want    []byte
		wantErr error
	}{
		{name: "zero length", input: makeFrame(0, nil), want: []byte{}},
		{name: "small", input: makeFrame(3, []byte("abc")), want: []byte("abc")},
		{name: "missing prefix", input: []byte{0x01, 0x02}, wantErr: io.ErrUnexpectedEOF},
		{name: "empty stream", input: nil, wantErr: io.EOF},
		{
			name:    "body short",
			input:   makeFrame(10, []byte("abc")),
			wantErr: io.ErrUnexpectedEOF,
		},
		{
			name:    "exceeds max",
			input:   makeFrame(MaxMessageBytes+1, nil),
			wantErr: ErrMessageTooLarge,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := ReadMessage(bytes.NewReader(tt.input))
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("err = %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
			if !bytes.Equal(got, tt.want) {
				t.Fatalf("got %q want %q", got, tt.want)
			}
		})
	}
}

func TestReadWriteRoundTrip(t *testing.T) {
	t.Parallel()
	body := []byte(strings.Repeat("hello", 100))
	var buf bytes.Buffer
	if err := WriteMessage(&buf, body); err != nil {
		t.Fatal(err)
	}
	got, err := ReadMessage(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, body) {
		t.Fatalf("round trip mismatch")
	}
}
