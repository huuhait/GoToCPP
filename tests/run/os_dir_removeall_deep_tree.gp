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
	p := d + "/t"
	for i := 0; i < 60; i++ {
		p += "/level" + fmt.Sprint(i)
	}
	must(os.MkdirAll(p, 0o755))
	must(os.WriteFile(p+"/leaf", []byte("x"), 0o644))
	for i := 0; i < 20; i++ {
		must(os.WriteFile(fmt.Sprintf("%s/t/level0/f%d", d, i), nil, 0o644))
	}
	fmt.Println(len(p) > 300, e(os.RemoveAll(d+"/t")))
	_, err := os.Stat(d + "/t")
	fmt.Println(e(err))
}

// Output:
// true <nil>
// stat $D/t: no such file or directory
