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

func must(err error) {
	if err != nil {
		panic(err)
	}
}

func main() {
	d := setup()
	defer cleanup()
	f, err := os.Create(d + "/a.txt")
	fmt.Println(e(err), f.Name() == d+"/a.txt")
	n, err := f.Write([]byte("hello, world"))
	fmt.Println(n, e(err))
	n, err = f.WriteString("!\n")
	fmt.Println(n, e(err))
	fmt.Println(e(f.Close()))
	b, err := os.ReadFile(d + "/a.txt")
	fmt.Printf("%q %s\n", b, e(err))
	g, err := os.Open(d + "/a.txt")
	must(err)
	buf := make([]byte, 5)
	n, err = g.Read(buf)
	fmt.Println(n, e(err), string(buf))
	rest, err := io.ReadAll(g)
	fmt.Printf("%q %s\n", rest, e(err))
	n, err = g.Read(buf)
	fmt.Println(n, err == io.EOF, e(err))
	g.Close()
}

// Output:
// <nil> true
// 12 <nil>
// 2 <nil>
// <nil>
// "hello, world!\n" <nil>
// 5 <nil> hello
// ", world!\n" <nil>
// 0 true EOF
