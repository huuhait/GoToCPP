// phase: 09

//go:build windows

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

func repeat(s string, n int) string {
	r := ""
	for i := 0; i < n; i++ {
		r += s
	}
	return r
}

func itoa(n int) string { return fmt.Sprint(n) }

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
	f, err := os.OpenFile(d+"/s", os.O_CREATE|os.O_WRONLY|os.O_SYNC, 0o644)
	fmt.Println(e(err))
	n, err := f.WriteString("synchronous")
	fmt.Println(n, e(err))
	f.Close()
	b, _ := os.ReadFile(d + "/s")
	fmt.Println(string(b))
	fmt.Println(os.O_RDONLY, os.O_WRONLY, os.O_RDWR, os.O_APPEND, os.O_CREATE, os.O_EXCL, os.O_SYNC, os.O_TRUNC)
	fmt.Println(os.SEEK_SET, os.SEEK_CUR, os.SEEK_END)
}

// Output:
// <nil>
// 11 <nil>
// synchronous
// 0 1 2 1024 64 128 4096 512
// 0 1 2
