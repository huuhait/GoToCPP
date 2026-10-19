// phase: 09

//go:build windows

package main

import (
	"fmt"
	"os"
	"time"
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
	p := d + "/t"
	must(os.WriteFile(p, []byte("x"), 0o644))
	mt := time.Date(2020, 5, 17, 12, 30, 45, 123456789, time.UTC)
	at := time.Date(2019, 1, 2, 3, 4, 5, 0, time.UTC)
	fmt.Println(e(os.Chtimes(p, at, mt)))
	fi, _ := os.Stat(p)
	fmt.Println(fi.ModTime().UTC(), fi.ModTime().Equal(mt))
	fmt.Println(e(os.Chtimes(p, time.Time{}, at)))
	fi, _ = os.Stat(p)
	fmt.Println(fi.ModTime().UTC())
	fmt.Println(e(os.Chtimes(p, mt, time.Time{})))
	fi, _ = os.Stat(p)
	fmt.Println(fi.ModTime().UTC())
	fmt.Println(e(os.Chtimes(d+"/none", at, mt)))
	fmt.Println(fi.ModTime().Location() == time.Local)
}

// Output:
// <nil>
// 2020-05-17 12:30:45.1234567 +0000 UTC false
// <nil>
// 2019-01-02 03:04:05 +0000 UTC
// <nil>
// 2019-01-02 03:04:05 +0000 UTC
// chtimes $D/none: The system cannot find the file specified.
// true
