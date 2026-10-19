package server

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"sync"
	"time"

	"github.com/huuhait/gocpp/internal/lsp/cache"
	"github.com/huuhait/gocpp/internal/lsp/protocol"
)

// A file that cannot be read for longer than the reads are repeated (cache.retryRead: about 0.6 s) leaves the snapshot
// without it, and no event comes when the other program lets go: on Windows a formatter, a build or a virus scanner can
// have the file open without sharing for seconds. So the server tells the user, once for the file, and reads the file
// again in the background with growing waits until it can (or it is gone); then it makes a new snapshot, which reads the
// file, and diagnoses the open packages again (Phase 14 step 14.5, ADR-096).

// A readRetry holds the files that are being read again.
type readRetry struct {
	mu     sync.Mutex
	active map[string]bool
	stop   chan struct{}
	once   sync.Once
	base   time.Duration // the first wait
	max    time.Duration // the longest wait
}

func newReadRetry() *readRetry {
	return &readRetry{active: map[string]bool{}, stop: make(chan struct{}), base: 500 * time.Millisecond, max: 8 * time.Second}
}

func (r *readRetry) close() { r.once.Do(func() { close(r.stop) }) }

// readFailed is Config.OnReadFailure.
func (s *Server) readFailed(path string, err error) {
	r := s.retry
	r.mu.Lock()
	if r.active[path] {
		r.mu.Unlock()
		return
	}
	r.active[path] = true
	r.mu.Unlock()
	reason := "it is in use by another program"
	if !cache.IsBusy(err) {
		reason = err.Error()
	}
	msg := fmt.Sprintf("gocpp lsp: cannot read %s (%s). The server tries again in the background and diagnoses the packages when it can.", path, reason)
	s.log.Print(msg)
	if s.conn != nil {
		if nerr := s.conn.Notify("window/logMessage", &protocol.LogMessageParams{Type: protocol.Warning, Message: msg}); nerr != nil && !errors.Is(nerr, protocol.ErrClosed) {
			s.log.Printf("window/logMessage: %v", nerr)
		}
	}
	go s.readAgain(path)
}

// readAgain reads path until it can, then makes a new snapshot in which the file is read, and diagnoses.
func (s *Server) readAgain(path string) {
	r := s.retry
	defer func() {
		r.mu.Lock()
		delete(r.active, path)
		r.mu.Unlock()
	}()
	read := s.readFile
	if read == nil {
		read = os.ReadFile
	}
	for wait := r.base; ; wait = min(wait*2, r.max) {
		select {
		case <-time.After(wait):
		case <-r.stop:
			return
		}
		_, err := read(path)
		if err != nil && cache.IsBusy(err) {
			continue
		}
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			s.log.Printf("reading %s again: %v", path, err)
		}
		s.log.Printf("%s can be read again", path)
		if err := s.session.Invalidate([]protocol.DocumentURI{protocol.URIFromPath(path)}); err != nil {
			return // the session is shut down
		}
		s.diags.scheduleOpen("", s.diags.delay)
		return
	}
}
