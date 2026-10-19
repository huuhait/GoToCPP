// phase: 09

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
	must(os.MkdirAll(d+"/x/y", 0o755))
	must(os.MkdirAll(d+"/z", 0o755))
	for _, n := range []string{"top", "x/one", "x/y/two", "z/three"} {
		must(os.WriteFile(d+"/"+n, []byte(n), 0o644))
	}
	err := fs.WalkDir(os.DirFS(d), ".", func(p string, de fs.DirEntry, err error) error {
		fmt.Println(p, de.IsDir(), e(err))
		if p == "z" {
			return fs.SkipDir
		}
		return nil
	})
	fmt.Println(e(err))
	err = fs.WalkDir(os.DirFS(d), "x", func(p string, de fs.DirEntry, err error) error {
		if p == "x/y/two" {
			return fs.SkipAll
		}
		fmt.Println(p)
		return nil
	})
	fmt.Println(e(err))
	err = fs.WalkDir(os.DirFS(d), "nope", func(p string, de fs.DirEntry, err error) error {
		fmt.Println(p, de == nil, e(err))
		return err
	})
	fmt.Println(e(err))
}

// Output:
// . true <nil>
// top false <nil>
// x true <nil>
// x/one false <nil>
// x/y true <nil>
// x/y/two false <nil>
// z true <nil>
// <nil>
// x
// x/one
// x/y
// <nil>
// nope true stat nope: no such file or directory
// stat nope: no such file or directory
