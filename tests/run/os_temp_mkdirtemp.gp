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
	p, err := os.MkdirTemp(d, "dir-*")
	fmt.Println(e(err), norm(p)[:7])
	fi, _ := os.Stat(p)
	fmt.Println(fi.IsDir(), fi.Mode().Perm())
	q, err := os.MkdirTemp(d, "x*y*z")
	fmt.Println(e(err), norm(q)[:5], norm(q)[len(norm(q))-1:])
	_, err = os.MkdirTemp(d, "a/b")
	fmt.Println(e(err))
	_, err = os.MkdirTemp(d+"/no", "x")
	fmt.Println(e(err))
	seen := map[string]bool{}
	for i := 0; i < 50; i++ {
		r, err := os.MkdirTemp(d, "u")
		must(err)
		seen[r] = true
	}
	fmt.Println(len(seen))
}

// Output:
// <nil> $D/dir-
// true -rwx------
// <nil> $D/x* z
// mkdirtemp a/b: pattern contains path separator
// stat $D/no: no such file or directory
// 50
