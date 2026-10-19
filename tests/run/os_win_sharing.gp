// phase: 09

//go:build windows

package main

import (
	"fmt"
	"os"
)

var tmpDir string

func setup() string {
	d, err := os.MkdirTemp("", "gpwin")
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

// Files are opened with sharing for reading and writing but not for deleting (Go's syscall.Open),
// as on every Windows program: a file that is open cannot be removed or renamed over.
func main() {
	d := setup()
	defer cleanup()
	p := d + `\a.txt`
	must(os.WriteFile(p, []byte("hello"), 0o666))

	f, err := os.Open(p)
	must(err)
	fmt.Println("remove while open:", e(os.Remove(p)))
	fmt.Println("rename while open:", e(os.Rename(p, d+`\b.txt`)))
	must(os.WriteFile(d+`\c.txt`, []byte("c"), 0o666))
	fmt.Println("rename over open:", e(os.Rename(d+`\c.txt`, p)))

	// Reading and writing through other handles is fine.
	g, err := os.OpenFile(p, os.O_RDWR, 0)
	must(err)
	_, err = g.WriteAt([]byte("J"), 0)
	must(err)
	buf := make([]byte, 5)
	n, err := f.ReadAt(buf, 0)
	fmt.Println(n, err, string(buf))
	fmt.Println(g.Close(), f.Close())

	fmt.Println("remove after close:", e(os.Remove(p)))
	_, err = os.Stat(p)
	fmt.Println(os.IsNotExist(err))

	// A directory that is the current directory cannot be removed.
	sub := d + `\cwd`
	must(os.Mkdir(sub, 0o777))
	old, _ := os.Getwd()
	must(os.Chdir(sub))
	fmt.Println("remove cwd:", e(os.Remove(sub)))
	must(os.Chdir(old))
	fmt.Println("remove after chdir:", e(os.Remove(sub)))
}

// Output:
// remove while open: remove $D\a.txt: The process cannot access the file because it is being used by another process.
// rename while open: rename $D\a.txt $D\b.txt: The process cannot access the file because it is being used by another process.
// rename over open: rename $D\c.txt $D\a.txt: Access is denied.
// 5 <nil> Jello
// <nil> <nil>
// remove after close: <nil>
// true
// remove cwd: remove $D\cwd: The process cannot access the file because it is being used by another process.
// remove after chdir: <nil>
