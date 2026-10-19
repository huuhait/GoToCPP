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

func main() {
	d := setup()
	defer cleanup()
	_, err := os.Open(d + "/missing")
	pe, ok := err.(*os.PathError)
	fmt.Println(ok, pe.Op, norm(pe.Path), pe.Err, pe.Err == syscall.ENOENT)
	fmt.Println(e(pe.Unwrap()), pe.Timeout())
	var pe2 *fs.PathError
	fmt.Println(errors.As(err, &pe2), pe == pe2)
	var errno syscall.Errno
	fmt.Println(errors.As(err, &errno), errno, uintptr(errno))
	wrapped := fmt.Errorf("context: %w", err)
	fmt.Println(norm(wrapped.Error()))
	fmt.Println(errors.Is(wrapped, os.ErrNotExist), errors.Is(wrapped, syscall.ENOENT), errors.Is(wrapped, os.ErrExist))
	fmt.Println(errors.Is(err, fs.ErrNotExist), os.ErrNotExist == fs.ErrNotExist)
	fmt.Printf("%T\n", err)
}

// Output:
// true open $D/missing no such file or directory true
// no such file or directory false
// true true
// true no such file or directory 2
// context: open $D/missing: no such file or directory
// true true false
// true true
// *fs.PathError
