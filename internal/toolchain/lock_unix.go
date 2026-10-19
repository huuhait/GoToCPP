//go:build unix

package toolchain

import (
	"context"
	"fmt"
	"os"
	"syscall"
	"time"
)

// lock takes the exclusive lock of the file name (flock: it is released when the process dies, so
// a crashed download doesn't leave a lock behind), waiting for another holder until ctx ends. The
// returned function releases it.
func lock(ctx context.Context, name string) (func(), error) {
	f, err := os.OpenFile(name, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, err
	}
	for {
		err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		if err == nil {
			return func() {
				syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
				f.Close()
			}, nil
		}
		if err != syscall.EWOULDBLOCK && err != syscall.EINTR {
			f.Close()
			return nil, fmt.Errorf("locking %s: %w", name, err)
		}
		select {
		case <-ctx.Done():
			f.Close()
			return nil, ctx.Err()
		case <-time.After(50 * time.Millisecond):
		}
	}
}
