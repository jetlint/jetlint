package daemon

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"time"
)

// Dial connects to a daemon listening on socketPath. The deadline applies
// only to the connection attempt; the returned net.Conn does not carry a
// read or write deadline.
func Dial(socketPath string, deadline time.Duration) (net.Conn, error) {
	conn, err := net.DialTimeout("unix", socketPath, deadline)
	if err != nil {
		return nil, fmt.Errorf("dial %s: %w", socketPath, err)
	}
	return conn, nil
}

// Send writes one Request and reads one Response on conn. It applies a
// read deadline so a wedged daemon cannot hang the caller indefinitely.
func Send(conn net.Conn, req Request, replyDeadline time.Duration) (Response, error) {
	if replyDeadline > 0 {
		_ = conn.SetReadDeadline(time.Now().Add(replyDeadline))
	}
	if err := clientWrite(conn, req); err != nil {
		return Response{}, err
	}
	return clientRead(conn)
}

// Ping is a convenience wrapper for the common health-probe path.
func Ping(socketPath string, deadline time.Duration) (Response, error) {
	conn, err := Dial(socketPath, deadline)
	if err != nil {
		return Response{}, err
	}
	defer conn.Close()
	return Send(conn, Request{Kind: KindPing}, deadline)
}

// --- framing ---

func clientWrite(w io.Writer, req Request) error {
	payload, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("encode request: %w", err)
	}
	var lenBuf [4]byte
	binary.BigEndian.PutUint32(lenBuf[:], uint32(len(payload)))
	if _, err := w.Write(lenBuf[:]); err != nil {
		return fmt.Errorf("write length prefix: %w", err)
	}
	if _, err := w.Write(payload); err != nil {
		return fmt.Errorf("write payload: %w", err)
	}
	return nil
}

func clientRead(r io.Reader) (Response, error) {
	var lenBuf [4]byte
	if _, err := io.ReadFull(r, lenBuf[:]); err != nil {
		return Response{}, fmt.Errorf("read length prefix: %w", err)
	}
	length := binary.BigEndian.Uint32(lenBuf[:])
	const maxFrame = 16 << 20
	if length > maxFrame {
		return Response{}, fmt.Errorf("frame length %d exceeds %d", length, maxFrame)
	}
	payload := make([]byte, length)
	if _, err := io.ReadFull(r, payload); err != nil {
		return Response{}, fmt.Errorf("read payload: %w", err)
	}
	var resp Response
	if err := json.Unmarshal(payload, &resp); err != nil {
		return Response{}, fmt.Errorf("decode response: %w", err)
	}
	return resp, nil
}
