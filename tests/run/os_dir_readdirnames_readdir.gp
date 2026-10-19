// phase: 09

package main

import (
	"fmt"
	"io"
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
	for _, n := range []string{"b", "a", "c"} {
		must(os.WriteFile(d+"/"+n, []byte(n+n), 0o644))
	}
	must(os.Mkdir(d+"/sub", 0o755))
	f, _ := os.Open(d)
	names, err := f.Readdirnames(-1)
	sortStrings(names)
	fmt.Println(names, e(err))
	names, err = f.Readdirnames(-1)
	fmt.Println(names, e(err))
	names, err = f.Readdirnames(1)
	fmt.Println(names, err == io.EOF)
	f.Close()
	f, _ = os.Open(d)
	infos, err := f.Readdir(0)
	for i := 1; i < len(infos); i++ {
		for j := i; j > 0 && infos[j].Name() < infos[j-1].Name(); j-- {
			infos[j], infos[j-1] = infos[j-1], infos[j]
		}
	}
	for _, fi := range infos {
		fmt.Println(fi.Name(), fi.Size() > 0 || fi.IsDir(), fi.IsDir())
	}
	fmt.Println(e(err))
	f.Close()
	g, _ := os.Open(d + "/a")
	_, err = g.Readdirnames(-1)
	fmt.Println(e(err))
	_, err = g.Readdir(-1)
	fmt.Println(e(err))
	g.Close()
}

// Output:
// [a b c sub] <nil>
// [] <nil>
// [] true
// a true false
// b true false
// c true false
// sub true true
// <nil>
// readdirent $D/a: not a directory
// readdirent $D/a: not a directory
