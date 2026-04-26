package daemon

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"
)

// Server is a per-tsconfig daemon. It listens on a Unix domain socket,
// serves Requests, and shuts itself down after a configurable idle period.
//
// One Server is created per tsconfig. A single Server is not designed to
// serve multiple tsconfigs; the architecture is "per-project daemon" by
// deliberate choice.
type Server struct {
	socketPath  string
	listener    net.Listener
	idleTimeout time.Duration

	mu          sync.Mutex
	lastRequest time.Time
	closed      atomic.Bool
}

// NewServer prepares a Server bound to socketPath with the given idle
// timeout. The parent directory of socketPath is created if it does not
// already exist. The listener is opened immediately so the socket file is
// visible to clients before Run is called; this avoids a race where a
// caller probes the socket between NewServer returning and the accept loop
// starting.
//
// idleTimeout is the duration of inactivity after which the server exits
// cleanly. A non-positive value disables idle shutdown.
func NewServer(socketPath string, idleTimeout time.Duration) (*Server, error) {
	if err := os.MkdirAll(filepath.Dir(socketPath), 0o700); err != nil {
		return nil, fmt.Errorf("prepare socket directory: %w", err)
	}
	// Remove any stale file at the socket path before listening.
	if err := os.Remove(socketPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("clear stale socket: %w", err)
	}
	ln, err := net.Listen("unix", socketPath)
	if err != nil {
		return nil, fmt.Errorf("listen on %s: %w", socketPath, err)
	}
	return &Server{
		socketPath:  socketPath,
		listener:    ln,
		idleTimeout: idleTimeout,
		lastRequest: time.Now(),
	}, nil
}

// SocketPath returns the path of the socket the server is listening on.
func (s *Server) SocketPath() string { return s.socketPath }

// Run blocks until ctx is canceled, the idle timeout elapses, or the
// listener is closed. It returns nil for any of those clean shutdown
// paths and an error only on unexpected listener failures.
func (s *Server) Run(ctx context.Context) error {
	defer s.cleanup()

	idleCh := make(chan struct{})
	if s.idleTimeout > 0 {
		go s.watchIdle(ctx, idleCh)
	}

	connCh := make(chan net.Conn)
	errCh := make(chan error, 1)
	go func() {
		for {
			conn, err := s.listener.Accept()
			if err != nil {
				if s.closed.Load() {
					close(connCh)
					return
				}
				errCh <- err
				return
			}
			connCh <- conn
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-idleCh:
			return nil
		case err := <-errCh:
			return fmt.Errorf("accept loop: %w", err)
		case conn, ok := <-connCh:
			if !ok {
				return nil
			}
			s.touchActivity()
			go s.handle(conn)
		}
	}
}

// Close stops the listener. Run will return on its next iteration.
func (s *Server) Close() error {
	s.closed.Store(true)
	return s.listener.Close()
}

func (s *Server) cleanup() {
	_ = s.listener.Close()
	// Best-effort socket file removal; if a client raced past the close, it
	// will simply see a connection refused.
	_ = os.Remove(s.socketPath)
}

func (s *Server) touchActivity() {
	s.mu.Lock()
	s.lastRequest = time.Now()
	s.mu.Unlock()
}

func (s *Server) watchIdle(ctx context.Context, fire chan<- struct{}) {
	tick := time.NewTicker(s.idleTimeout / 4)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			s.mu.Lock()
			idle := time.Since(s.lastRequest)
			s.mu.Unlock()
			if idle >= s.idleTimeout {
				select {
				case fire <- struct{}{}:
				case <-ctx.Done():
				}
				return
			}
		}
	}
}

func (s *Server) handle(conn net.Conn) {
	defer conn.Close()
	req, err := readMessage(conn)
	if err != nil {
		s.respondError(conn, err)
		return
	}
	resp := Response{Kind: req.Kind, PID: os.Getpid()}
	switch req.Kind {
	case KindPing:
		// nothing to do; the response carries the daemon's PID.
	default:
		resp.Error = fmt.Sprintf("unknown request kind: %s", req.Kind)
	}
	if err := writeMessage(conn, resp); err != nil {
		// The client likely went away; nothing further we can do.
		return
	}
}

func (s *Server) respondError(conn net.Conn, err error) {
	_ = writeMessage(conn, Response{Error: err.Error()})
}

// --- framing ---

// readMessage reads one length-prefixed JSON Request from r.
func readMessage(r io.Reader) (Request, error) {
	var lenBuf [4]byte
	if _, err := io.ReadFull(r, lenBuf[:]); err != nil {
		return Request{}, fmt.Errorf("read length prefix: %w", err)
	}
	length := binary.BigEndian.Uint32(lenBuf[:])
	const maxFrame = 16 << 20 // 16 MiB safety cap
	if length > maxFrame {
		return Request{}, fmt.Errorf("frame length %d exceeds %d", length, maxFrame)
	}
	payload := make([]byte, length)
	if _, err := io.ReadFull(r, payload); err != nil {
		return Request{}, fmt.Errorf("read payload: %w", err)
	}
	var req Request
	if err := json.Unmarshal(payload, &req); err != nil {
		return Request{}, fmt.Errorf("decode request: %w", err)
	}
	return req, nil
}

// writeMessage writes one length-prefixed JSON Response to w.
func writeMessage(w io.Writer, resp Response) error {
	payload, err := json.Marshal(resp)
	if err != nil {
		return fmt.Errorf("encode response: %w", err)
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
