package protocol

import (
	"encoding/binary"
	"errors"
	"io"
)

// MaxMessageBytes is the Chrome Native Messaging per-message limit (1 MiB).
const MaxMessageBytes = 1024 * 1024

var ErrMessageTooLarge = errors.New("native messaging payload exceeds 1 MiB")

// ReadMessage reads one 4-byte little-endian length prefix followed by the
// UTF-8 JSON body from r.
func ReadMessage(r io.Reader) ([]byte, error) {
	var lenBuf [4]byte
	if _, err := io.ReadFull(r, lenBuf[:]); err != nil {
		return nil, err
	}
	n := binary.LittleEndian.Uint32(lenBuf[:])
	if n > MaxMessageBytes {
		return nil, ErrMessageTooLarge
	}
	buf := make([]byte, n)
	if _, err := io.ReadFull(r, buf); err != nil {
		return nil, err
	}
	return buf, nil
}

// WriteMessage writes body prefixed with its 4-byte little-endian length.
func WriteMessage(w io.Writer, body []byte) error {
	if len(body) > MaxMessageBytes {
		return ErrMessageTooLarge
	}
	var lenBuf [4]byte
	binary.LittleEndian.PutUint32(lenBuf[:], uint32(len(body)))
	if _, err := w.Write(lenBuf[:]); err != nil {
		return err
	}
	_, err := w.Write(body)
	return err
}
