//go:build windows

package toolchain

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/huuhait/gocpp/internal/filelock"
)

// lock takes the exclusive lock of the file name (LockFileEx: it is released when the process
// dies, so a crashed download doesn't leave a lock behind), waiting for another holder until ctx
// ends. The returned function releases it.
func lock(ctx context.Context, name string) (func(), error) {
	f, err := os.OpenFile(name, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, err
	}
	for {
		ok, err := filelock.TryLock(f)
		if ok {
			return func() {
				filelock.Unlock(f)
				f.Close()
			}, nil
		}
		if err != nil {
			f.Close()
			if errors.Is(err, filelock.ErrUnsupported) {
				return func() {}, nil // a file system without locks: the rename of the folder has one winner
			}
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
