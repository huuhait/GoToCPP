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

func must(err error) {
	if err != nil {
		panic(err)
	}
}

func main() {
	d := setup()
	defer cleanup()
	p := d + "/x"
	f, err := os.OpenFile(p, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	fmt.Println(e(err))
	f.WriteString("data")
	f.Close()
	_, err = os.OpenFile(p, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	fmt.Println(e(err), os.IsExist(err), errors.Is(err, fs.ErrExist))
	f, err = os.OpenFile(p, os.O_TRUNC|os.O_WRONLY, 0)
	fmt.Println(e(err))
	f.Close()
	b, _ := os.ReadFile(p)
	fmt.Println(len(b))
	_, err = os.OpenFile(d+"/y", os.O_WRONLY, 0)
	fmt.Println(e(err))
	_, err = os.OpenFile(d+"/y", os.O_RDONLY|os.O_CREATE, 0o644)
	fmt.Println(e(err))
}

// Output:
// <nil>
// open $D/x: The file exists. true true
// <nil>
// 0
// open $D/y: The system cannot find the file specified.
// <nil>
