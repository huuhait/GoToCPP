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
	must(os.WriteFile(d+"/target", []byte("T"), 0o644))
	fmt.Println(e(os.Symlink("target", d+"/rel")))
	fmt.Println(e(os.Symlink(d+"/target", d+"/abs")))
	fmt.Println(e(os.Symlink("does-not-exist", d+"/dangling")))
	for _, n := range []string{"rel", "abs", "dangling"} {
		s, err := os.Readlink(d + "/" + n)
		fmt.Println(n, norm(s), e(err))
	}
	_, err := os.Readlink(d + "/target")
	fmt.Println(e(err))
	b, err := os.ReadFile(d + "/rel")
	fmt.Println(string(b), e(err))
	_, err = os.ReadFile(d + "/dangling")
	fmt.Println(e(err))
	long := ""
	for i := 0; i < 300; i++ {
		long += "x"
	}
	fmt.Println(e(os.Symlink(long, d+"/long")))
	s, err := os.Readlink(d + "/long")
	fmt.Println(len(s), s == long, e(err))
}

// Output:
// <nil>
// <nil>
// <nil>
// rel target <nil>
// abs $D/target <nil>
// dangling does-not-exist <nil>
// readlink $D/target: invalid argument
// T <nil>
// open $D/dangling: no such file or directory
// <nil>
// 300 true <nil>
