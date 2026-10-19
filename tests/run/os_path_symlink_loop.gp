// phase: 09

package main

import (
	"fmt"
	"os"
)

var tmpDir string

func setup() string {
	d, err := os.MkdirTemp("", "gpos")
	if err != nil {
		panic(err)
	}
	tmpDir = d
	return d
}

func cleanup() { os.RemoveAll(tmpDir) }

func idx(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

// norm replaces the temporary directory in s by $D.
func norm(s string) string {
	for {
		i := idx(s, tmpDir)
		if i < 0 {
			return s
		}
		s = s[:i] + "$D" + s[i+len(tmpDir):]
	}
}

// e is the text of err, with the temporary directory replaced.
func e(err error) string {
	if err == nil {
		return "<nil>"
	}
	return norm(err.Error())
}

func repeat(s string, n int) string {
	r := ""
	for i := 0; i < n; i++ {
		r += s
	}
	return r
}

func itoa(n int) string { return fmt.Sprint(n) }

func sortStrings(a []string) {
	for i := 1; i < len(a); i++ {
		for j := i; j > 0 && a[j] < a[j-1]; j-- {
			a[j], a[j-1] = a[j-1], a[j]
		}
	}
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}

func main() {
	d := setup()
	defer cleanup()
	must(os.Symlink("b", d+"/a"))
	must(os.Symlink("a", d+"/b"))
	_, err := os.Stat(d + "/a")
	fmt.Println(e(err))
	_, err = os.Open(d + "/a")
	fmt.Println(e(err))
	l, err := os.Lstat(d + "/a")
	fmt.Println(e(err), l.Mode().Type() == os.ModeSymlink)
	must(os.Symlink("self", d+"/self"))
	_, err = os.ReadFile(d + "/self")
	fmt.Println(e(err))
	s, err := os.Readlink(d + "/self")
	fmt.Println(s, e(err))
}

// Output:
// stat $D/a: too many levels of symbolic links
// open $D/a: too many levels of symbolic links
// <nil> true
// open $D/self: too many levels of symbolic links
// self <nil>
