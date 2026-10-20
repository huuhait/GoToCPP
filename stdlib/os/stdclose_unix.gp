// gocpp: the close hook of the standard streams (gocpp_helpers.gp, ADR-100).

//go:build unix

package os

import (
	"internal/poll"
	"sync/atomic"
)

var stdoutDone, stdinDone atomic.Bool

// installCloseHook makes the closing of the descriptor of the original os.Stdout or os.Stdin wait for
// the prints and scans in flight (fmtDeferClose); every other close goes to syscall.Close.
func installCloseHook() {
	prev := poll.CloseFunc
	// Capture the identities before any Close can change pfd.Sysfd to -1. Other files'
	// concurrent closes must not read those mutable fields.
	stdoutFD, stdinFD := stdout0.pfd.Sysfd, stdin0.pfd.Sysfd
	poll.CloseFunc = func(fd int) error {
		if stdoutClosed.Load() && fd == stdoutFD && !stdoutDone.Swap(true) {
			if fmtDeferClose(true, func() { prev(fd) }) {
				return nil
			}
		} else if stdinClosed.Load() && fd == stdinFD && !stdinDone.Swap(true) {
			if fmtDeferClose(false, func() { prev(fd) }) {
				return nil
			}
		}
		return prev(fd)
	}
}
