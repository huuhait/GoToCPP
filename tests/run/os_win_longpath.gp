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

func rep(s string, n int) string {
	b := make([]byte, 0, len(s)*n)
	for i := 0; i < n; i++ {
		b = append(b, s...)
	}
	return string(b)
}

// A path of more than MAX_PATH (260) characters works in package os: it prefixes \\?\ when the
// path is long (fixLongPath), for every operation. (Chdir to such a path needs the long-path-aware
// manifest that Go's linker writes and gocpp's does not: stdlib/os/PORT.md.)
func main() {
	d := setup()
	defer cleanup()
	dir := d
	seg := rep("x", 100)
	for i := 0; i < 4; i++ {
		dir += `\` + seg
	}
	fmt.Println(len(dir) > 300)
	fmt.Println(e(os.Mkdir(dir, 0o777)) != "<nil>") // the parents do not exist
	must(os.MkdirAll(dir, 0o777))
	p := dir + `\file.txt`
	must(os.WriteFile(p, []byte("long"), 0o666))
	b, err := os.ReadFile(p)
	fmt.Println(string(b), err)
	fi, err := os.Stat(p)
	fmt.Println(fi.Name(), fi.Size(), err)
	ents, err := os.ReadDir(dir)
	fmt.Println(len(ents), ents[0].Name(), err)
	fmt.Println(os.Rename(p, dir+`\renamed.txt`))
	fmt.Println(os.Link(dir+`\renamed.txt`, dir+`\hard.txt`))
	fmt.Println(os.Truncate(dir+`\hard.txt`, 2))
	b, _ = os.ReadFile(dir + `\renamed.txt`)
	fmt.Println(string(b))
	fmt.Println(os.Remove(dir + `\hard.txt`))
	fmt.Println(os.RemoveAll(d + `\` + seg))
	_, err = os.Stat(d + `\` + seg)
	fmt.Println(os.IsNotExist(err))
	// A name longer than the file system allows is the system's error.
	_, err = os.Create(d + `\` + rep("n", 300))
	fmt.Println(e(err) != "<nil>")
}

// Output:
// true
// true
// long <nil>
// file.txt 4 <nil>
// 1 file.txt <nil>
// <nil>
// <nil>
// <nil>
// lo
// <nil>
// <nil>
// true
// true
