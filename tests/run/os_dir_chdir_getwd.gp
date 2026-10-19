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
	old, err := os.Getwd()
	fmt.Println(old != "", err)
	must(os.Mkdir(d+"/sub", 0o755))
	fmt.Println(e(os.Chdir(d + "/sub")))
	wd, err := os.Getwd()
	a, _ := os.Stat(wd)
	b0, _ := os.Stat(d + "/sub")
	fmt.Println(os.SameFile(a, b0), e(err))
	must(os.WriteFile("rel.txt", []byte("hi"), 0o644))
	b, err := os.ReadFile(d + "/sub/rel.txt")
	fmt.Println(string(b), e(err))
	fmt.Println(e(os.Chdir("..")))
	b, err = os.ReadFile("sub/rel.txt")
	fmt.Println(string(b), e(err))
	fmt.Println(e(os.Chdir(d + "/missing")))
	fmt.Println(e(os.Chdir(d + "/sub/rel.txt")))
	f, _ := os.Open(d + "/sub")
	fmt.Println(e(f.Chdir()))
	b, _ = os.ReadFile("rel.txt")
	fmt.Println(string(b))
	f.Close()
	os.Chdir(old)
}

// Output:
// true <nil>
// <nil>
// true <nil>
// hi <nil>
// <nil>
// hi <nil>
// chdir $D/missing: no such file or directory
// chdir $D/sub/rel.txt: not a directory
// <nil>
// hi
