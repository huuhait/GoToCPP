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

func must(err error) {
	if err != nil {
		panic(err)
	}
}

func main() {
	d := setup()
	defer cleanup()
	f, _ := os.Create(d + "/n")
	fmt.Println(f.Fd() > 2, norm(f.Name()))
	fmt.Println(os.Stdin.Fd(), os.Stdout.Fd(), os.Stderr.Fd())
	fmt.Println(os.Stdin.Name(), os.Stdout.Name(), os.Stderr.Name())
	var nilf *os.File
	fmt.Println(nilf.Fd() == ^uintptr(0))
	g := os.NewFile(f.Fd(), "alias")
	fmt.Println(g.Name())
	nf := os.NewFile(^uintptr(0), "bad")
	fmt.Println(nf == nil)
	f.Close()
}

// Output:
// true $D/n
// 0 1 2
// /dev/stdin /dev/stdout /dev/stderr
// true
// alias
// true
