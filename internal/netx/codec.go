package netx

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"

	"p2poker/internal/protocol"
)

// length‑prefixed JSON codec: [u32 len][json bytes]

func Encode(msg protocol.NetMessage) ([]byte, error) {
	b, err := json.Marshal(msg)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := binary.Write(&buf, binary.BigEndian, uint32(len(b))); err != nil {
		return nil, err
	}
	if _, err := buf.Write(b); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func Decode(r *bufio.Reader) (protocol.NetMessage, error) {
	var msg protocol.NetMessage
	var n uint32
	if err := binary.Read(r, binary.BigEndian, &n); err != nil {
		return msg, err
	}
	if n > 10*1024*1024 {
		return msg, fmt.Errorf("frame too large: %d", n)
	}
	buf := make([]byte, n)
	if _, err := io.ReadFull(r, buf); err != nil {
		return msg, err
	}
	if err := json.Unmarshal(buf, &msg); err != nil {
		return msg, err
	}
	return msg, nil
}
