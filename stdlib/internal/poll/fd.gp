// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package poll is the file descriptor layer of package os (Go's internal/poll, reduced for
// gocpp: stdlib/internal/poll/PORT.md).
//
// It keeps what package os needs of Go's package: the FD type with its lock and reference
// count (a closed file is not used again, Close during a Read is safe), and the read, write
// and metadata calls with Go's error rules (EINTR, io.EOF, short writes). A goroutine is an
// OS thread (ADR-007), so every descriptor is used in blocking mode and there is no poller:
// no deadlines, and Close does not wake a Read that is blocked in the kernel.
package poll

import (
	"errors"
)

// errNetClosing is the type of the variable ErrNetClosing.
// This is used to implement the net.Error interface.
type errNetClosing struct{}

// Error returns the error message for ErrNetClosing.
// Keep this string consistent because of issue #4373:
// since historically programs have not been able to detect
// this error, they look for the string.
func (e errNetClosing) Error() string { return "use of closed network connection" }

func (e errNetClosing) Timeout() bool   { return false }
func (e errNetClosing) Temporary() bool { return false }

// ErrNetClosing is returned when a network descriptor is used after
// it has been closed.
var ErrNetClosing = errNetClosing{}

// ErrFileClosing is returned when a file descriptor is used after it
// has been closed.
var ErrFileClosing = errors.New("use of closed file")

// ErrNoDeadline is returned when a request is made to set a deadline
// on a file type that does not use the poller.
var ErrNoDeadline = errors.New("file type does not support deadline")

// Return the appropriate closing error based on isFile.
func errClosing(isFile bool) error {
	if isFile {
		return ErrFileClosing
	}
	return ErrNetClosing
}

// ErrDeadlineExceeded is returned for an expired deadline.
// This is exported by the os package as os.ErrDeadlineExceeded.
var ErrDeadlineExceeded error = &DeadlineExceededError{}

// DeadlineExceededError is returned for an expired deadline.
type DeadlineExceededError struct{}

// Implement the net.Error interface.
// The string is "i/o timeout" because that is what was returned
// by earlier Go versions. Changing it may break programs that
// match on error strings.
func (e *DeadlineExceededError) Error() string   { return "i/o timeout" }
func (e *DeadlineExceededError) Timeout() bool   { return true }
func (e *DeadlineExceededError) Temporary() bool { return true }

// ErrNotPollable is returned when the file or socket is not suitable
// for event notification.
var ErrNotPollable = errors.New("not pollable")
