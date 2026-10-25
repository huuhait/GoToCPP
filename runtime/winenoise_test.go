//go:build linux

package rtfiles_test

import "testing"

// TestDropWineNoise: Wine's own client errors at the end of a process are not the program's
// output (windows_test.go, runVia).
func TestDropWineNoise(t *testing.T) {
	for _, c := range []struct{ in, want string }{
		{"only stdout\n", "only stdout\n"},
		{"only stdout\nwine client error:c544: sendmsg: Socket operation on non-socket\n", "only stdout\n"},
		{"wine client error:1: x\nok: 3 checks\n", "ok: 3 checks\n"},
		{"a\nwine client error:1: x", "a\n"},
		{"the text wine client error: is the program's own\n", "the text wine client error: is the program's own\n"},
		{"wine client error without a colon\n", "wine client error without a colon\n"},
		{"", ""},
	} {
		if got := string(dropWineNoise([]byte(c.in))); got != c.want {
			t.Errorf("dropWineNoise(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// TestWineRetry: a run is repeated only when Wine's client error was seen; a signal death without it
// is a real crash and fails on the first run.
func TestWineRetry(t *testing.T) {
	noise := []byte("ok\nwine client error:c544: sendmsg: Socket operation on non-socket\n")
	for _, c := range []struct {
		name     string
		code     int
		timedOut bool
		attempt  int
		out, err []byte
		want     bool
	}{
		{"signal with noise on stdout", -1, false, 1, noise, nil, true},
		{"signal with noise on stderr", -1, false, 1, nil, noise, true},
		{"signal without noise", -1, false, 1, []byte("ok: 3 checks\n"), nil, false},
		{"second attempt", -1, false, 2, noise, nil, false},
		{"our timeout", -1, true, 1, noise, nil, false},
		{"the program prints the text mid-line", -1, false, 1, []byte("it said: wine client error: x\n"), nil, false},
		{"the program prints the text without its colon", -1, false, 1, []byte("wine client error\n"), nil, false},
		{"noise at the start of the last line, no newline", -1, false, 1, []byte("ok\nwine client error:1: x"), nil, true},
		{"exit code 2 with noise", 2, false, 1, noise, nil, false},
		{"exit 0 with noise", 0, false, 1, noise, nil, false},
	} {
		if got := wineRetry(c.code, c.timedOut, c.attempt, c.out, c.err); got != c.want {
			t.Errorf("%s: wineRetry = %v, want %v", c.name, got, c.want)
		}
	}
}
