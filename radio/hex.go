package radio

import (
	"encoding/hex"
	"io"
)

type hexReadWriter struct {
	rw io.ReadWriter
}

func (hrw hexReadWriter) Read(p []byte) (int, error) {
	buf := make([]byte, hex.EncodedLen(len(p)))
	n, err := hrw.rw.Read(buf)
	if err != nil {
		return n, err
	}

	return hex.Decode(p, buf[0:n])
}

func (hrw hexReadWriter) Write(p []byte) (int, error) {
	buf := make([]byte, hex.EncodedLen(len(p)))
	n := hex.Encode(buf, p)

	n, err := hrw.rw.Write(buf)
	if n >= 0 {
		return hex.DecodedLen(n), err
	} else {
		return n, err
	}
}
