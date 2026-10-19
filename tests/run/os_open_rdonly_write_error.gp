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
	p := d + "/r"
	must(os.WriteFile(p, []byte("abc"), 0o644))
	f, _ := os.Open(p)
	n, err := f.Write([]byte("x"))
	fmt.Println(n, e(err))
	n, err = f.WriteString("x")
	fmt.Println(n, e(err))
	n, err = f.WriteAt([]byte("x"), 0)
	fmt.Println(n, e(err))
	fmt.Println(e(f.Truncate(1)))
	f.Close()
	g, _ := os.OpenFile(p, os.O_WRONLY, 0)
	buf := make([]byte, 2)
	n, err = g.Read(buf)
	fmt.Println(n, e(err))
	n, err = g.ReadAt(buf, 0)
	fmt.Println(n, e(err))
	g.Close()
}

// Output:
// 0 write $D/r: bad file descriptor
// 0 write $D/r: bad file descriptor
// 0 write $D/r: bad file descriptor
// truncate $D/r: invalid argument
// 0 read $D/r: bad file descriptor
// 0 read $D/r: bad file descriptor
