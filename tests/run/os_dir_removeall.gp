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
	must(os.MkdirAll(d+"/t/a/b/c", 0o755))
	must(os.WriteFile(d+"/t/a/1", []byte("x"), 0o644))
	must(os.WriteFile(d+"/t/a/b/2", []byte("x"), 0o644))
	must(os.WriteFile(d+"/t/a/b/c/3", []byte("x"), 0o644))
	must(os.Symlink(d+"/t/a", d+"/t/link"))
	fmt.Println(e(os.RemoveAll(d + "/t")))
	_, err := os.Stat(d + "/t")
	fmt.Println(e(err))
	fmt.Println(e(os.RemoveAll(d + "/t")))
	fmt.Println(e(os.RemoveAll(d + "/never/existed")))
	fmt.Println(e(os.RemoveAll("")))
	must(os.WriteFile(d+"/plain", []byte("x"), 0o644))
	fmt.Println(e(os.RemoveAll(d + "/plain")))
	fmt.Println(e(os.RemoveAll(d + "/plain/sub")))
	fmt.Println(e(os.RemoveAll(".")))
	fmt.Println(e(os.RemoveAll(d + "/x/.")))
}

// Output:
// <nil>
// stat $D/t: no such file or directory
// <nil>
// <nil>
// <nil>
// <nil>
// <nil>
// RemoveAll .: invalid argument
// RemoveAll $D/x/.: invalid argument
