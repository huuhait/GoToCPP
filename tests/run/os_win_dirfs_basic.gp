// phase: 09

//go:build windows

package main

import (
	"fmt"
	"io/fs"
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
	must(os.MkdirAll(d+"/sub/deep", 0o755))
	must(os.WriteFile(d+"/a.txt", []byte("A"), 0o644))
	must(os.WriteFile(d+"/sub/b.txt", []byte("BB"), 0o644))
	fsys := os.DirFS(d)
	b, err := fs.ReadFile(fsys, "sub/b.txt")
	fmt.Println(string(b), e(err))
	_, err = fs.ReadFile(fsys, "missing")
	fmt.Println(e(err))
	_, err = fs.ReadFile(fsys, "../a.txt")
	fmt.Println(e(err))
	_, err = fs.ReadFile(fsys, "/a.txt")
	fmt.Println(e(err))
	ents, err := fs.ReadDir(fsys, ".")
	for _, en := range ents {
		fmt.Println(en.Name(), en.IsDir())
	}
	fmt.Println(e(err))
	fi, err := fs.Stat(fsys, "sub")
	fmt.Println(fi.IsDir(), fi.Name(), e(err))
	f, err := fsys.Open("a.txt")
	fmt.Println(e(err))
	st, _ := f.Stat()
	fmt.Println(st.Size())
	f.Close()
	fmt.Println(fs.ValidPath("a/b"), fs.ValidPath("../a"), fs.ValidPath(""), fs.ValidPath("."))
}

// Output:
// BB <nil>
// open missing: The system cannot find the file specified.
// readfile ../a.txt: invalid argument
// readfile /a.txt: invalid argument
// a.txt false
// sub true
// <nil>
// true sub <nil>
// <nil>
// 1
// true false false true
