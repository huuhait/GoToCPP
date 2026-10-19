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
	src := make([]byte, 1<<20+13)
	for i := range src {
		src[i] = byte(i * 31)
	}
	must(os.WriteFile(d+"/src", src, 0o644))
	in, _ := os.Open(d + "/src")
	out, _ := os.Create(d + "/dst")
	n, err := io.Copy(out, in)
	fmt.Println(n, err)
	in.Close()
	out.Close()
	back, _ := os.ReadFile(d + "/dst")
	ok := len(back) == len(src)
	for i := range back {
		if back[i] != src[i] {
			ok = false
		}
	}
	fmt.Println(ok)
	in, _ = os.Open(d + "/src")
	n, err = io.CopyN(io.Discard, in, 100)
	fmt.Println(n, err)
	n, err = io.CopyN(io.Discard, in, 10<<20)
	fmt.Println(n, err)
	in.Close()
	sr := io.NewSectionReader(mustOpen(d+"/src"), 10, 5)
	sb, _ := io.ReadAll(sr)
	fmt.Println(sb)
}

func mustOpen(p string) *os.File {
	f, err := os.Open(p)
	must(err)
	return f
}

// Output:
// 1048589 <nil>
// true
// 100 <nil>
// 1048489 EOF
// [54 85 116 147 178]
