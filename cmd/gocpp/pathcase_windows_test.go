//go:build windows

package main

import "testing"

func TestCanonPath(t *testing.T) {
	for _, c := range []struct{ in, want string }{
		{`c:\windows\SYSTEM32`, `C:\Windows\System32`},
		{`C:\WINDOWS\system32\\.`, `C:\Windows\System32`},
		{`c:\PROGRA~1`, `C:\Program Files`},
		{`c:\gocpp-no-such-folder\Mixed`, `C:\gocpp-no-such-folder\Mixed`}, // not there: only the drive letter and the clean
		{``, ``},
	} {
		if got := canonPath(c.in); got != c.want {
			t.Errorf("canonPath(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
