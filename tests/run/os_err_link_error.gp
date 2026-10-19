// phase: 09

package main

import (
	"errors"
	"fmt"
	"os"
	"syscall"
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
	err := os.Rename(d+"/a", d+"/b")
	le, ok := err.(*os.LinkError)
	fmt.Println(ok, le.Op, norm(le.Old), norm(le.New), le.Err)
	fmt.Println(e(err), errors.Is(err, os.ErrNotExist), errors.Is(err, syscall.ENOENT))
	err = os.Link(d+"/a", d+"/b")
	fmt.Println(e(err))
	err = os.Symlink("t", d)
	fmt.Println(e(err))
	err = os.Symlink("t", d+"/l")
	fmt.Println(e(err))
	err = os.Symlink("u", d+"/l")
	fmt.Println(e(err), errors.Is(err, os.ErrExist))
	_, err = os.Readlink(d + "/zzz")
	fmt.Println(e(err))
	_, err = os.Readlink(d)
	fmt.Println(e(err))
}

// Output:
// true rename $D/a $D/b no such file or directory
// rename $D/a $D/b: no such file or directory true true
// link $D/a $D/b: no such file or directory
// symlink t $D: file exists
// <nil>
// symlink u $D/l: file exists true
// readlink $D/zzz: no such file or directory
// readlink $D: invalid argument
