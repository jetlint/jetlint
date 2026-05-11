package daemon_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jetlint/jetlint/internal/daemon"
)

// startServer spins up a daemon server bound to a fresh socket under a temp
// directory, runs it in a goroutine, and returns the socket path plus a
// cleanup function. Tests use t.Cleanup to close the server.
func startServer(t *testing.T, idleTimeout time.Duration) string {
	t.Helper()
	dir := t.TempDir()
	socket := filepath.Join(dir, "ts.sock")
	srv, err := daemon.NewServer(socket, idleTimeout)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- srv.Run(ctx) }()
	t.Cleanup(func() {
		_ = srv.Close()
		cancel()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Errorf("daemon Run did not return within 2s")
		}
	})
	// Wait briefly for the listener to be ready (Run starts accept immediately,
	// but the goroutine may not have scheduled yet).
	for i := 0; i < 50; i++ {
		if _, err := os.Stat(socket); err == nil {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	return socket
}

func TestServer_PingReturnsResponseWithDaemonPID(t *testing.T) {
	socket := startServer(t, 0)
	resp, err := daemon.Ping(socket, time.Second)
	if err != nil {
		t.Fatalf("Ping: %v", err)
	}
	if resp.Kind != daemon.KindPing {
		t.Errorf("expected response kind %q, got %q", daemon.KindPing, resp.Kind)
	}
	if resp.PID != os.Getpid() {
		// Server runs in this same process for the in-process test, so PID
		// must match. A spawned-daemon test would assert PID > 0.
		t.Errorf("expected PID %d, got %d", os.Getpid(), resp.PID)
	}
	if resp.Error != "" {
		t.Errorf("unexpected error in response: %s", resp.Error)
	}
}

func TestServer_RemovesSocketOnClose(t *testing.T) {
	socket := startServer(t, 0)
	if _, err := os.Stat(socket); err != nil {
		t.Fatalf("expected socket to exist before close: %v", err)
	}
	// Triggering t.Cleanup early via a fresh server doesn't help; instead,
	// rely on t.Cleanup at end of test and assert via a separate test that
	// the file is gone after the cleanup runs.
	// Here we just verify the socket exists during the server's lifetime.
}

func TestServer_RemovesSocketAfterCleanup(t *testing.T) {
	dir := t.TempDir()
	socket := filepath.Join(dir, "ts.sock")
	srv, err := daemon.NewServer(socket, 0)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- srv.Run(ctx) }()

	// Wait until the listener is accepting.
	for i := 0; i < 50; i++ {
		if _, err := os.Stat(socket); err == nil {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	_ = srv.Close()
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("daemon Run did not return within 2s")
	}

	if _, err := os.Stat(socket); !os.IsNotExist(err) {
		t.Errorf("expected socket to be removed after cleanup, stat err: %v", err)
	}
}

func TestServer_IdleTimeoutShutsDownCleanly(t *testing.T) {
	dir := t.TempDir()
	socket := filepath.Join(dir, "ts.sock")
	srv, err := daemon.NewServer(socket, 80*time.Millisecond)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	done := make(chan error, 1)
	go func() { done <- srv.Run(context.Background()) }()
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Run returned error on idle shutdown: %v", err)
		}
	case <-time.After(2 * time.Second):
		_ = srv.Close()
		t.Fatal("daemon did not shut down within 2s of idle timeout")
	}
}

func TestServer_HandlesUnknownRequestKindWithStructuredError(t *testing.T) {
	socket := startServer(t, 0)
	conn, err := daemon.Dial(socket, time.Second)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer conn.Close()
	resp, err := daemon.Send(conn, daemon.Request{Kind: "not-a-real-kind"}, time.Second)
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if resp.Error == "" {
		t.Errorf("expected structured error for unknown request kind, got empty")
	}
}
