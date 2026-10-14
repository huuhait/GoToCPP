package build

import (
	"context"
	"errors"
	"os/exec"
	"syscall"
	"time"
)

// Start starts the command that newCmd makes. When the start fails with
// ETXTBSY ("text file busy"), it waits a moment and starts a new one,
// up to three times, like go run (cmd/go's base.RunStdin): a program
// that was just written (a compiler wrapper, a program that a test
// writes) can still be open for writing in a child process that another
// goroutine forked at that moment, until that child runs exec
// (golang/go#22315). A canceled ctx ends the waiting: then Start returns
// the last command and its error.
func Start(ctx context.Context, newCmd func() *exec.Cmd) (*exec.Cmd, error) {
	for try := 0; ; try++ {
		cmd := newCmd()
		err := cmd.Start()
		if err == nil || try == 3 || !errors.Is(err, syscall.ETXTBSY) {
			return cmd, err
		}
		t := time.NewTimer(100 * time.Millisecond << try)
		select {
		case <-t.C:
		case <-ctx.Done():
			t.Stop()
			return cmd, err
		}
	}
}
