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
	must(os.WriteFile(d+"/a", []byte("x"), 0o644))
	must(os.WriteFile(d+"/b", []byte("x"), 0o644))
	must(os.Symlink("a", d+"/la"))
	a1, _ := os.Stat(d + "/a")
	a2, _ := os.Stat(d + "/a")
	b, _ := os.Stat(d + "/b")
	la, _ := os.Stat(d + "/la")
	ll, _ := os.Lstat(d + "/la")
	fmt.Println(os.SameFile(a1, a2), os.SameFile(a1, b), os.SameFile(a1, la), os.SameFile(a1, ll))
	f, _ := os.Open(d + "/a")
	fs, _ := f.Stat()
	fmt.Println(os.SameFile(a1, fs), os.SameFile(b, fs))
	f.Close()
	fmt.Println(os.SameFile(a1, nil) == false)
}

// Output:
// true false true false
// true false
// true
