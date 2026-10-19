// phase: 09

//go:build windows

package main

import (
	"fmt"
	"io/fs"
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
	must(os.WriteFile(d+"/f", []byte("hello"), 0o640))
	must(os.Mkdir(d+"/d", 0o750))
	fi, _ := os.Stat(d + "/f")
	de, _ := os.Stat(d + "/d")
	fmt.Println(fs.FormatFileInfo(fi)[:11], fs.FormatFileInfo(de)[:12])
	ents, _ := os.ReadDir(d)
	for _, en := range ents {
		fmt.Println(fs.FormatDirEntry(en), en)
	}
	for _, m := range []os.FileMode{0, 0o644, os.ModeDir | 0o755, os.ModeSymlink | 0o777, os.ModeNamedPipe | 0o600, os.ModeSocket | 0o600, os.ModeDevice | os.ModeCharDevice | 0o600, os.ModeSetuid | 0o755, os.ModeSticky | os.ModeDir | 0o777, os.ModeAppend | 0o600, os.ModeExclusive | 0o600, os.ModeIrregular} {
		fmt.Printf("%s %o %v %v %v\n", m, uint32(m.Perm()), m.IsDir(), m.IsRegular(), m.Type())
	}
}

// Output:
// -rw-rw-rw-  drwxrwxrwx 0
// d d/ d d/
// - f - f
// ---------- 0 false true ----------
// -rw-r--r-- 644 false true ----------
// drwxr-xr-x 755 true false d---------
// Lrwxrwxrwx 777 false false L---------
// prw------- 600 false false p---------
// Srw------- 600 false false S---------
// Dcrw------- 600 false false Dc---------
// urwxr-xr-x 755 false true ----------
// dtrwxrwxrwx 777 true false d---------
// arw------- 600 false true ----------
// lrw------- 600 false true ----------
// ?--------- 0 false false ?---------
