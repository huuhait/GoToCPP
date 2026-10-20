package main

import (
	"strings"
	"testing"
)

// TestStdStores: every form of Go that stores to os.Stdout, os.Stdin or os.Stderr tells fmt (ADR-100,
// internal/lower/stdstore.go): gp::fmt::store_std for the variable, gp::fmt::std_changed_after for what it
// points to. The forms are the places where the lowering makes the target of a store: an assignment, a
// multiple assignment, the init and post statements of a for, the variables of a range (over a channel, a
// slice, a map), a receive in a select (with one and two values), and a swap; the last lines are no stores
// to the streams. Behavior (the text goes to the new file) is in tests/run/fmt_stdout_*.
func TestStdStores(t *testing.T) {
	t.Setenv(gatePhaseEnv, "15")
	src := `package main

import "os"

func f() (*os.File, int) { return nil, 0 }

func main() {
	w := os.Stdout
	ch := make(chan *os.File, 1)
	var ok bool
	var n int
	var i int

	os.Stdout = w                             // 1 store_std
	os.Stdin, n = w, 1                        // 2
	os.Stderr, n = f()                        // 3
	for os.Stdout = w; n < 1; os.Stdout = w { // 4, 5
	}
	for os.Stdout = range ch { // 6
	}
	for _, os.Stdin = range []*os.File{w} { // 7
	}
	for i, os.Stdout = range map[int]*os.File{} { // 8
	}
	select {
	case os.Stdout = <-ch: // 9
	case os.Stdin, ok = <-ch: // 10
	}
	*os.Stdout = *w             // 11 std_changed_after
	os.Stdout, w = w, os.Stdout // 12
	_, _, _ = ok, n, i

	// Not stores to the streams.
	var other *os.File
	other = w
	x := os.Stdout
	_ = x
	_ = other
}
`
	r := checkIn(t, map[string]string{"main.go": src}, "emit", "main.gp")
	if r.code != 0 {
		t.Fatalf("exit %d:\n%s", r.code, show(r))
	}
	// The package os brings its own stores (its init): count the ones of package main.
	main := r.stdout[strings.LastIndex(r.stdout, "gp::ptr<::gpkg_os::File> w = "):]
	if got := strings.Count(main, "gp::fmt::store_std("); got != 11 {
		t.Errorf("%d calls of gp::fmt::store_std in main, want 11:\n%s", got, main)
	}
	if got := strings.Count(main, "gp::fmt::std_changed_after("); got != 1 {
		t.Errorf("%d calls of gp::fmt::std_changed_after in main, want 1", got)
	}
	if strings.Contains(main, "Std") && strings.Contains(strings.ReplaceAll(strings.ReplaceAll(main, "gp::fmt::store_std(::gpkg_os::Std", ""), "std_changed_after(*::gpkg_os::Stdout", ""), "::gpkg_os::Stdout = ") {
		t.Errorf("a plain store to a standard stream in main:\n%s", main)
	}
}
