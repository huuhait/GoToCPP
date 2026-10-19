package fuzz

import "testing"

func TestKnownGaps(t *testing.T) {
	for _, msg := range []string{
		"/fuzz/p/main.gp:3:2: cppgen: not supported yet: the call of strings.ToLower (gocpp lowers only the Print, Sprint, Fprint, Append and Errorf functions of fmt)",
		"cppgen: not supported yet: the call of atomic.AddInt32 (gocpp lowers only x)",
		"cppgen: not supported yet: a function without a body",
		"cppgen: not supported yet: the conversion from string to []MyByte12",
	} {
		if knownGap(msg) == "" {
			t.Errorf("not a known gap: %s", msg)
		}
	}
	// Other refusals are findings: unsafe is a gocpp bug cluster of the conformance run, a
	// package of the program and other constructs are not listed.
	for _, msg := range []string{
		"cppgen: not supported yet: the call of unsafe.Add (gocpp lowers only x)",
		"cppgen: not supported yet: the call of mypkg.F (gocpp lowers only x)",
		"cppgen: not supported yet: the conversion from []int to string",
		"cppgen: not supported yet: switch statement",
		"cppgen: internal error: x",
	} {
		if g := knownGap(msg); g != "" {
			t.Errorf("%q is a known gap (%q), it must stay a finding", msg, g)
		}
	}
}

func TestNewInternal(t *testing.T) {
	got := newInternal([]string{
		"a.gp:1:1: internal error: the type checker failed on this package: nil underlying",
		"a.gp:2:1: internal error: expression has no type: x",
	})
	if len(got) != 1 || got[0] != "a.gp:2:1: internal error: expression has no type: x" {
		t.Errorf("got %q", got)
	}
}
