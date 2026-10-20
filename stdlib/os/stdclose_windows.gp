// gocpp: the close hook of the standard streams (gocpp_helpers.gp, ADR-100).

//go:build windows

package os

import (
	"internal/poll"
	"sync/atomic"
	"syscall"
)

var stdoutDone, stdinDone atomic.Bool

// installCloseHook makes the closing of the handle of the original os.Stdout or os.Stdin wait for
// the prints and scans in flight (fmtDeferClose); every other close goes to syscall.CloseHandle.
func installCloseHook() {
	prev := poll.CloseFunc
	// Capture the identities before any Close can change pfd.Sysfd to InvalidHandle.
	// Other files' concurrent closes must not read those mutable fields.
	stdoutFD, stdinFD := stdout0.pfd.Sysfd, stdin0.pfd.Sysfd
	poll.CloseFunc = func(h syscall.Handle) error {
		if stdoutClosed.Load() && h == stdoutFD && !stdoutDone.Swap(true) {
			if fmtDeferClose(true, func() { prev(h) }) {
				return nil
			}
		} else if stdinClosed.Load() && h == stdinFD && !stdinDone.Swap(true) {
			if fmtDeferClose(false, func() { prev(h) }) {
				return nil
			}
		}
		return prev(h)
	}
}
