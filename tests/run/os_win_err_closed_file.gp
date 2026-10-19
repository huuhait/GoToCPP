// phase: 09

//go:build windows

package main

import (
	"errors"
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
	f, _ := os.Create(d + "/c")
	fmt.Println(e(f.Close()))
	err := f.Close()
	fmt.Println(e(err), errors.Is(err, os.ErrClosed))
	buf := make([]byte, 4)
	_, err = f.Read(buf)
	fmt.Println(e(err), err == io.EOF)
	_, err = f.Write(buf)
	fmt.Println(e(err))
	_, err = f.WriteString("x")
	fmt.Println(e(err))
	_, err = f.Seek(0, 0)
	fmt.Println(e(err))
	_, err = f.Stat()
	fmt.Println(e(err))
	_, err = f.ReadAt(buf, 0)
	fmt.Println(e(err))
	_, err = f.Readdir(1)
	fmt.Println(e(err))
	_, err = f.ReadDir(1)
	fmt.Println(e(err))
	_, err = f.Readdirnames(1)
	fmt.Println(e(err))
	fmt.Println(e(f.Chdir()))
	fmt.Println(os.IsNotExist(err), os.IsExist(err))
	pe := err.(*os.PathError)
	fmt.Println(pe.Op, pe.Err == os.ErrClosed)
}

// Output:
// <nil>
// close $D/c: file already closed true
// read $D/c: file already closed false
// write $D/c: file already closed
// write $D/c: file already closed
// seek $D/c: file already closed
// GetFileType $D/c: The handle is invalid.
// read $D/c: file already closed
// GetFileInformationByHandleEx $D/c: The handle is invalid.
// GetFileInformationByHandleEx $D/c: The handle is invalid.
// GetFileInformationByHandleEx $D/c: The handle is invalid.
// chdir $D/c: file already closed
// false false
// GetFileInformationByHandleEx false
