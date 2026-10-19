package cache

import (
	"errors"
	"fmt"
	"io/fs"
	"runtime"
	"syscall"
	"testing"
	"time"
)

func TestTransientRead(t *testing.T) {
	for _, c := range []struct {
		err  error
		want int
	}{
		{nil, 0},
		{errors.New("bad"), 0},
		{fs.ErrNotExist, 0},
		{syscall.EMFILE, transientResources},
		{&fs.PathError{Op: "open", Path: "x", Err: syscall.ENFILE}, transientResources},
		{syscall.ENOMEM, transientResources},
	} {
		if got := transientRead(c.err); got != c.want {
			t.Errorf("transientRead(%v) = %d, want %d", c.err, got, c.want)
		}
	}
	// Errno 32 and 33 are sharing and lock violations on Windows only (on Linux, 32 is EPIPE: not a reason to read again).
	want := 0
	if runtime.GOOS == "windows" {
		want = transientSharing
	}
	for _, en := range []syscall.Errno{32, 33} {
		if got := transientRead(&fs.PathError{Op: "open", Path: "x", Err: en}); got != want {
			t.Errorf("transientRead(errno %d) = %d, want %d on %s", en, got, want, runtime.GOOS)
		}
	}
}

// The waits between the repeats: the total is what the user waits for a file that stays busy.
func TestRetryReadWaits(t *testing.T) {
	var waits []time.Duration
	old := retrySleep
	retrySleep = func(d time.Duration) { waits = append(waits, d) }
	defer func() { retrySleep = old }()
	for _, c := range []struct {
		name  string
		err   error
		want  []time.Duration
		total time.Duration
	}{
		{"resources", syscall.EMFILE, []time.Duration{5e6, 20e6, 80e6}, 105e6},
		{"busy", fmt.Errorf("open x: %w", ErrBusy), []time.Duration{5e6, 10e6, 20e6, 40e6, 80e6, 160e6, 320e6}, 635e6},
	} {
		waits = nil
		calls := 0
		_, err := retryRead(func() ([]byte, error) { calls++; return nil, c.err })
		if err == nil || calls != len(c.want)+1 {
			t.Errorf("%s: %d calls, err %v; want %d calls and the error", c.name, calls, err, len(c.want)+1)
		}
		var total time.Duration
		for _, d := range waits {
			total += d
		}
		if fmt.Sprint(waits) != fmt.Sprint(c.want) || total != c.total {
			t.Errorf("%s: waits %v (total %v), want %v (total %v)", c.name, waits, total, c.want, c.total)
		}
	}
	// A busy file that is free on the fourth try is read.
	waits = nil
	n := 0
	src, err := retryRead(func() ([]byte, error) {
		n++
		if n < 4 {
			return nil, ErrBusy
		}
		return []byte("ok"), nil
	})
	if err != nil || string(src) != "ok" || len(waits) != 3 {
		t.Errorf("busy for three tries: %q %v after %d waits", src, err, len(waits))
	}
}

func TestIsBusy(t *testing.T) {
	if !IsBusy(fmt.Errorf("open: %w", ErrBusy)) || IsBusy(syscall.EMFILE) || IsBusy(nil) || IsBusy(errors.New("x")) {
		t.Error("IsBusy")
	}
}
