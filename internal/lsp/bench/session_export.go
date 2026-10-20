package bench

import (
	"context"
	"encoding/json"
	"time"
)

// StartSession starts the server of t on the project tree (written for t's
// extension) with its caches in cacheDir, and initializes it: for tools
// that drive a server the way the benchmarks do (tools/lspdiff).
func StartSession(ctx context.Context, timeout time.Duration, proj *Project, t Target, tree, cacheDir string) (*Session, error) {
	return startSession(ctx, Config{Timeout: timeout}, proj, t, tree, cacheDir)
}

// OpenAndWait opens the file rel and waits for its first diagnostics.
func (s *Session) OpenAndWait(ctx context.Context, rel string) error {
	_, err := s.openAndWait(ctx, rel)
	return err
}

// CallRaw sends the request and returns its raw result, including JSON null.
// A server refusal wraps *protocol.ResponseError; stream failures and timeouts
// are errors, not answers that can establish equivalence.
func (s *Session) CallRaw(ctx context.Context, method string, params any) (json.RawMessage, error) {
	_, raw, err := s.call(ctx, method, params)
	return raw, err
}

// Params returns the params of a request at the byte offset off of the open
// document rel.
func (s *Session) Params(rel string, off int) any { return s.at(rel, off) }
