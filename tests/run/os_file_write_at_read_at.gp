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
	f, err := os.OpenFile(d+"/w", os.O_RDWR|os.O_CREATE, 0o644)
	must(err)
	n, err := f.WriteAt([]byte("world"), 6)
	fmt.Println(n, e(err))
	n, err = f.WriteAt([]byte("hello"), 0)
	fmt.Println(n, e(err))
	b, _ := os.ReadFile(d + "/w")
	fmt.Printf("%q\n", b)
	buf := make([]byte, 20)
	n, err = f.ReadAt(buf, 0)
	fmt.Println(n, err == io.EOF, e(err))
	n, err = f.ReadAt(buf[:3], 100)
	fmt.Println(n, err == io.EOF)
	n, err = f.ReadAt(buf, -1)
	fmt.Println(n, e(err))
	n, err = f.WriteAt([]byte("x"), -5)
	fmt.Println(n, e(err))
	off, _ := f.Seek(0, io.SeekCurrent)
	fmt.Println("offset unchanged by *At:", off)
	f.Close()
	n, err = f.WriteAt([]byte("x"), 0)
	fmt.Println(n, e(err))
}

// Output:
// 5 <nil>
// 5 <nil>
// "hello\x00world"
// 11 true EOF
// 0 true
// 0 readat $D/w: negative offset
// 0 writeat $D/w: negative offset
// offset unchanged by *At: 0
// 0 write $D/w: file already closed
