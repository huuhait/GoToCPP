// phase: 09

package main

import (
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
	must(os.WriteFile(d+"/f", []byte("0123456789"), 0o644))
	f, _ := os.Open(d + "/f")
	for _, c := range []struct{ size int; off int64 }{{4, 0}, {4, 8}, {4, 10}, {4, 100}, {10, 0}, {11, 0}, {0, 0}, {0, 10}, {0, 11}} {
		buf := make([]byte, c.size)
		n, err := f.ReadAt(buf, c.off)
		fmt.Println(c.size, c.off, n, err == io.EOF, e(err))
	}
	f.Close()
}

// Output:
// 4 0 4 false <nil>
// 4 8 2 true EOF
// 4 10 0 true EOF
// 4 100 0 true EOF
// 10 0 10 false <nil>
// 11 0 10 true EOF
// 0 0 0 false <nil>
// 0 10 0 false <nil>
// 0 11 0 false <nil>
