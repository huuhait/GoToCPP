// phase: 09

//go:build linux

package main

import (
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
	for _, um := range []int{0o000, 0o022, 0o077} {
		old := syscall.Umask(um)
		name := fmt.Sprintf("%s/u%03o", d, um)
		f, err := os.OpenFile(name, os.O_CREATE|os.O_WRONLY, 0o666)
		must(err)
		f.Close()
		must(os.Mkdir(name+"d", 0o777))
		a, _ := os.Stat(name)
		b, _ := os.Stat(name + "d")
		fmt.Printf("%03o %v %v\n", um, a.Mode(), b.Mode())
		syscall.Umask(old)
	}
}

// Output:
// 000 -rw-rw-rw- drwxrwxrwx
// 022 -rw-r--r-- drwxr-xr-x
// 077 -rw------- drwx------
