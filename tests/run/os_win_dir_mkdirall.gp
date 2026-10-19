// phase: 09

//go:build windows

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
	fmt.Println(e(os.MkdirAll(d+"/a/b/c", 0o755)))
	fmt.Println(e(os.MkdirAll(d+"/a/b/c", 0o755)))
	fmt.Println(e(os.MkdirAll(d+"/a/b", 0o755)))
	must(os.WriteFile(d+"/f", nil, 0o644))
	fmt.Println(e(os.MkdirAll(d+"/f", 0o755)))
	fmt.Println(e(os.MkdirAll(d+"/f/g/h", 0o755)))
	fmt.Println(e(os.MkdirAll("", 0o755)))
	fmt.Println(e(os.MkdirAll(d, 0o755)))
	fmt.Println(e(os.MkdirAll(d+"/t/../u/./v/", 0o755)))
	fi, err := os.Stat(d + "/u/v")
	fmt.Println(fi.IsDir(), e(err))
	_, err = os.Stat(d + "/t")
	fmt.Println(e(err))
	fi, _ = os.Stat(d + "/a/b/c")
	fmt.Println(fi.Mode().Perm() | 0o022&^0o022)
}

// Output:
// <nil>
// <nil>
// <nil>
// mkdir $D/f: The system cannot find the path specified.
// mkdir $D/f: The system cannot find the path specified.
// mkdir : The system cannot find the path specified.
// <nil>
// <nil>
// true <nil>
// GetFileAttributesEx $D/t: The system cannot find the file specified.
// -rwxrwxrwx
