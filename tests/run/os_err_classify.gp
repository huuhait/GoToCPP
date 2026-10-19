// phase: 09

package main

import (
	"errors"
	"fmt"
	"io/fs"
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

func show(name string, err error) {
	fmt.Println(name, os.IsExist(err), os.IsNotExist(err), os.IsPermission(err), os.IsTimeout(err),
		errors.Is(err, fs.ErrExist), errors.Is(err, fs.ErrNotExist), errors.Is(err, fs.ErrPermission))
}

func main() {
	d := setup()
	defer cleanup()
	_, err := os.Open(d + "/no")
	show("notexist", err)
	must(os.Mkdir(d+"/dir", 0o755))
	err = os.Mkdir(d+"/dir", 0o755)
	show("exist", err)
	show("nil", nil)
	show("plain", errors.New("x"))
	show("EACCES", &os.PathError{Op: "x", Path: "y", Err: syscall.EACCES})
	show("EPERM", os.NewSyscallError("x", syscall.EPERM))
	show("ENOTEMPTY", &os.LinkError{Op: "x", Old: "a", New: "b", Err: syscall.ENOTEMPTY})
	show("ETIMEDOUT", &os.PathError{Err: syscall.ETIMEDOUT})
	show("EAGAIN", syscall.EAGAIN)
	show("wrapped", fmt.Errorf("w: %w", err))
}

// Output:
// notexist false true false false false true false
// exist true false false false true false false
// nil false false false false false false false
// plain false false false false false false false
// EACCES false false true false false false true
// EPERM false false true false false false true
// ENOTEMPTY true false false false true false false
// ETIMEDOUT false false false true false false false
// EAGAIN false false false true false false false
// wrapped false false false false true false false
