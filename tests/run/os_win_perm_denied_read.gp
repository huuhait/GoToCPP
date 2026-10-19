// phase: 09

//go:build windows

package main

import (
	"errors"
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
	if os.Geteuid() == 0 {
		fmt.Println("skipped as root")
		return
	}
	p := d + "/secret"
	must(os.WriteFile(p, []byte("x"), 0o600))
	must(os.Chmod(p, 0))
	_, err := os.Open(p)
	fmt.Println(e(err), os.IsPermission(err), errors.Is(err, fs.ErrPermission))
	_, err = os.ReadFile(p)
	fmt.Println(e(err))
	err = os.WriteFile(p, []byte("y"), 0o600)
	fmt.Println(e(err), os.IsPermission(err))
	_, err = os.OpenFile(p, os.O_RDWR, 0)
	fmt.Println(e(err))
	fi, err := os.Stat(p)
	fmt.Println(fi.Mode(), e(err))
	must(os.Chmod(p, 0o600))
}

// Output:
// <nil> false false
// <nil>
// open $D/secret: Access is denied. true
// open $D/secret: Access is denied.
// -r--r--r-- <nil>
