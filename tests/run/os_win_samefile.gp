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

// SameFile compares the volume serial number and the file index of the system, not the names.
func main() {
	d := setup()
	defer cleanup()
	must(os.WriteFile(d+`\a.txt`, []byte("a"), 0o666))
	must(os.WriteFile(d+`\b.txt`, []byte("b"), 0o666))
	must(os.Link(d+`\a.txt`, d+`\hard.txt`))
	a1, _ := os.Stat(d + `\a.txt`)
	a2, _ := os.Stat(d + `\A.TXT`)
	a3, _ := os.Stat(d + `\.\a.txt`)
	b, _ := os.Stat(d + `\b.txt`)
	h, _ := os.Stat(d + `\hard.txt`)
	fmt.Println(os.SameFile(a1, a2), os.SameFile(a1, a3), os.SameFile(a1, b), os.SameFile(a1, h), os.SameFile(b, h))
	f, err := os.Open(d + `\a.txt`)
	must(err)
	fi, _ := f.Stat()
	fmt.Println(os.SameFile(a1, fi), fi.Name())
	f.Close()
	dir1, _ := os.Stat(d)
	dir2, _ := os.Stat(d + `\.`)
	fmt.Println(os.SameFile(dir1, dir2), os.SameFile(dir1, a1))
	ents, err := os.ReadDir(d)
	must(err)
	for _, ent := range ents {
		i, _ := ent.Info()
		fmt.Println(ent.Name(), ent.IsDir(), ent.Type(), os.SameFile(i, a1))
	}
	// Names and times.
	fmt.Println(a1.Name(), a1.Size(), a1.Mode(), a1.IsDir(), a1.ModTime().IsZero())
	fmt.Println(e(nil), a1.Sys() != nil)
}

// Output:
// true true false true false
// true a.txt
// true false
// a.txt false ---------- true
// b.txt false ---------- false
// hard.txt false ---------- true
// a.txt 1 -rw-rw-rw- false false
// <nil> true
