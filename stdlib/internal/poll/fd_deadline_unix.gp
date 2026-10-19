// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unix

package poll

import "time"

// SetDeadline sets the read and write deadlines associated with fd.
func (fd *FD) SetDeadline(t time.Time) error {
	return setDeadlineImpl(fd, t, 'r'+'w')
}

// SetReadDeadline sets the read deadline associated with fd.
func (fd *FD) SetReadDeadline(t time.Time) error {
	return setDeadlineImpl(fd, t, 'r')
}

// SetWriteDeadline sets the write deadline associated with fd.
func (fd *FD) SetWriteDeadline(t time.Time) error {
	return setDeadlineImpl(fd, t, 'w')
}

func setDeadlineImpl(fd *FD, t time.Time, mode int) error {
	var d int64
	if !t.IsZero() {
		d = int64(time.Until(t))
		if d == 0 {
			d = -1 // don't confuse deadline right now with no deadline
		}
	}
	if err := fd.incref(); err != nil {
		return err
	}
	defer fd.decref()
	if !fd.pd.pollable() {
		return ErrNoDeadline
	}
	fd.pd.setDeadline(d, mode)
	return nil
}

// IsPollDescriptor reports whether fd is the descriptor being used by the poller: never (the wake
// descriptors of a pollDesc are not visible to package os).
func IsPollDescriptor(fd uintptr) bool { return false }
