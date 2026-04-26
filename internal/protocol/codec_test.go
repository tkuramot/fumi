package protocol

import (
	"bytes"
	"encoding/binary"
	"io"
	"strings"
	"testing"
)

// makeFrame builds a properly-framed native messaging message.
func makeFrame(body []byte) []byte {
	var buf bytes.Buffer
	var lenBuf [4]byte
	binary.LittleEndian.PutUint32(lenBuf[:], uint32(len(body)))
	buf.Write(lenBuf[:])
	buf.Write(body)
	return buf.Bytes()
}

func TestReadMessage_success(t *testing.T) {
	body := []byte(`{"jsonrpc":"2.0"}`)
	frame := makeFrame(body)

	got, err := ReadMessage(bytes.NewReader(frame))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bytes.Equal(got, body) {
		t.Errorf("got %q, want %q", got, body)
	}
}

func TestReadMessage_empty(t *testing.T) {
	body := []byte{}
	frame := makeFrame(body)

	got, err := ReadMessage(bytes.NewReader(frame))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty body, got %q", got)
	}
}

func TestReadMessage_eof(t *testing.T) {
	_, err := ReadMessage(bytes.NewReader(nil))
	if err != io.EOF {
		t.Errorf("expected io.EOF, got %v", err)
	}
}

func TestReadMessage_partialLength(t *testing.T) {
	// Only 2 bytes of the 4-byte length prefix.
	_, err := ReadMessage(bytes.NewReader([]byte{0x00, 0x01}))
	if err == nil {
		t.Error("expected error for partial length prefix, got nil")
	}
}

func TestReadMessage_tooLarge(t *testing.T) {
	// Encode a length just over the limit.
	var lenBuf [4]byte
	binary.LittleEndian.PutUint32(lenBuf[:], MaxMessageBytes+1)
	_, err := ReadMessage(bytes.NewReader(lenBuf[:]))
	if err != ErrMessageTooLarge {
		t.Errorf("expected ErrMessageTooLarge, got %v", err)
	}
}

func TestReadMessage_atLimit(t *testing.T) {
	// A message exactly at the limit (just the header; body can be zeros).
	body := make([]byte, MaxMessageBytes)
	frame := makeFrame(body)

	got, err := ReadMessage(bytes.NewReader(frame))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != MaxMessageBytes {
		t.Errorf("expected %d bytes, got %d", MaxMessageBytes, len(got))
	}
}

func TestReadMessage_bodyTruncated(t *testing.T) {
	// Length prefix says 10 bytes but only 4 are provided.
	var lenBuf [4]byte
	binary.LittleEndian.PutUint32(lenBuf[:], 10)
	frame := append(lenBuf[:], []byte("abcd")...)

	_, err := ReadMessage(bytes.NewReader(frame))
	if err == nil {
		t.Error("expected error for truncated body, got nil")
	}
}

func TestWriteMessage_success(t *testing.T) {
	body := []byte(`{"jsonrpc":"2.0","id":"1"}`)
	var buf bytes.Buffer
	if err := WriteMessage(&buf, body); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	written := buf.Bytes()
	if len(written) != 4+len(body) {
		t.Fatalf("expected %d bytes, got %d", 4+len(body), len(written))
	}
	gotLen := binary.LittleEndian.Uint32(written[:4])
	if int(gotLen) != len(body) {
		t.Errorf("length prefix %d != body length %d", gotLen, len(body))
	}
	if !bytes.Equal(written[4:], body) {
		t.Errorf("body mismatch: got %q, want %q", written[4:], body)
	}
}

func TestWriteMessage_tooLarge(t *testing.T) {
	body := make([]byte, MaxMessageBytes+1)
	var buf bytes.Buffer
	err := WriteMessage(&buf, body)
	if err != ErrMessageTooLarge {
		t.Errorf("expected ErrMessageTooLarge, got %v", err)
	}
}

func TestWriteMessage_atLimit(t *testing.T) {
	body := make([]byte, MaxMessageBytes)
	var buf bytes.Buffer
	if err := WriteMessage(&buf, body); err != nil {
		t.Errorf("unexpected error writing max-size message: %v", err)
	}
}

func TestRoundTrip(t *testing.T) {
	cases := []string{
		`{"jsonrpc":"2.0","id":"abc","method":"actions/list"}`,
		`{}`,
		`{"key":"` + strings.Repeat("x", 1000) + `"}`,
	}
	for _, tc := range cases {
		body := []byte(tc)
		var buf bytes.Buffer
		if err := WriteMessage(&buf, body); err != nil {
			t.Fatalf("WriteMessage: %v", err)
		}
		got, err := ReadMessage(&buf)
		if err != nil {
			t.Fatalf("ReadMessage: %v", err)
		}
		if !bytes.Equal(got, body) {
			t.Errorf("round-trip mismatch for %q: got %q", tc, got)
		}
	}
}
