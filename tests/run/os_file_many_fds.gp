// phase: 09

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
	must(os.WriteFile(d+"/f", []byte("x"), 0o644))
	var fs []*os.File
	seen := map[uintptr]bool{}
	for i := 0; i < 1000; i++ {
		f, err := os.Open(d + "/f")
		if err != nil {
			fmt.Println("open failed at", i, err)
			break
		}
		fs = append(fs, f)
		seen[f.Fd()] = true
	}
	fmt.Println(len(fs), len(seen))
	for _, f := range fs {
		if err := f.Close(); err != nil {
			fmt.Println(err)
		}
	}
	f, _ := os.Open(d + "/f")
	fmt.Println(seen[f.Fd()])
	f.Close()
	for round := 0; round < 3; round++ {
		for i := 0; i < 1000; i++ {
			f, err := os.Open(d + "/f")
			must(err)
			f.Close()
		}
	}
	fmt.Println("done")
}

// Output:
// 1000 1000
// true
// done
