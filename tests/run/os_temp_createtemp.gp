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
	f, err := os.CreateTemp(d, "pre-*-suf.txt")
	fmt.Println(e(err))
	name := norm(f.Name())
	fmt.Println(len(name) > len("$D/pre--suf.txt"), name[:7], name[len(name)-8:])
	fi, _ := f.Stat()
	fmt.Println(fi.Mode(), fi.Size())
	f.WriteString("temp")
	f.Close()
	g, err := os.CreateTemp(d, "plain")
	fmt.Println(e(err), len(norm(g.Name())) > len("$D/plain"), norm(g.Name())[:8])
	h, err := os.CreateTemp(d, "a*b*c")
	fmt.Println(e(err), norm(h.Name())[:5], norm(h.Name())[len(norm(h.Name()))-1:])
	_, err = os.CreateTemp(d, "bad/pattern")
	fmt.Println(e(err))
	_, err = os.CreateTemp(d+"/none", "x")
	fmt.Println(os.IsNotExist(err), err.(*os.PathError).Op)
	i, err := os.CreateTemp(d, "")
	fmt.Println(e(err), len(norm(i.Name())) > 4)
}

// Output:
// <nil>
// true $D/pre- -suf.txt
// -rw------- 0
// <nil> true $D/plain
// <nil> $D/a* c
// createtemp bad/pattern: pattern contains path separator
// true open
// <nil> true
