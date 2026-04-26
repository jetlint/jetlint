// Package daemon implements the per-project tsgolint daemon and the
// CLI-side client that talks to it. Each daemon instance binds to one
// tsconfig and serves linting requests for files governed by that
// tsconfig, holding the warm TypeScript program in memory across
// invocations.
//
// The wire protocol is deliberately simple for v0.1: each connection
// carries exactly one request and one response, both as length-prefixed
// JSON. The framing is a four-byte big-endian length followed by the
// payload. This keeps the framing testable without bringing in a full
// JSON-RPC library before the surface is well understood.
package daemon

// RequestKind identifies the operation a request asks the daemon to perform.
type RequestKind string

const (
	// KindPing is a no-op probe used for health checks. The daemon replies
	// with the kind it received and its own process identifier so callers
	// can confirm the daemon is responsive.
	KindPing RequestKind = "ping"
)

// Request is the envelope exchanged from CLI to daemon over a single
// connection.
type Request struct {
	Kind RequestKind `json:"kind"`
}

// Response is the envelope exchanged from daemon to CLI in reply to a
// Request. PID is the daemon process identifier; callers can use it to
// distinguish responses from different daemons across reconnects.
type Response struct {
	Kind RequestKind `json:"kind"`
	PID  int         `json:"pid"`
	// Error, when non-empty, indicates a structured failure handling the
	// request. Lint diagnostics are not reported through this field.
	Error string `json:"error,omitempty"`
}
