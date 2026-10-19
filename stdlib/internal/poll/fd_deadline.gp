// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build windows

package poll

import "time"

// A Windows handle is in blocking mode (the model of fd_windows.gp): it has no deadlines.

// SetDeadline sets the read and write deadlines associated with fd.
func (fd *FD) SetDeadline(t time.Time) error {
	return ErrNoDeadline
}

// SetReadDeadline sets the read deadline associated with fd.
func (fd *FD) SetReadDeadline(t time.Time) error {
	return ErrNoDeadline
}

// SetWriteDeadline sets the write deadline associated with fd.
func (fd *FD) SetWriteDeadline(t time.Time) error {
	return ErrNoDeadline
}

// IsPollDescriptor reports whether fd is the descriptor being used by the poller: never.
func IsPollDescriptor(fd uintptr) bool { return false }
