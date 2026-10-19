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
	if os.Geteuid() == 0 {
		fmt.Println("skipped as root")
		return
	}
	must(os.Mkdir(d+"/locked", 0o755))
	must(os.WriteFile(d+"/locked/f", []byte("x"), 0o644))
	must(os.Chmod(d+"/locked", 0o000))
	_, err := os.ReadDir(d + "/locked")
	fmt.Println(e(err), os.IsPermission(err))
	_, err = os.Stat(d + "/locked/f")
	fmt.Println(e(err), os.IsPermission(err))
	err = os.WriteFile(d+"/locked/g", nil, 0o644)
	fmt.Println(e(err))
	err = os.Remove(d + "/locked/f")
	fmt.Println(e(err))
	must(os.Chmod(d+"/locked", 0o500))
	err = os.WriteFile(d+"/locked/g", nil, 0o644)
	fmt.Println(e(err))
	err = os.Mkdir(d+"/locked/sub", 0o755)
	fmt.Println(e(err))
	must(os.Chmod(d+"/locked", 0o700))
}

// Output:
// <nil> false
// <nil> false
// <nil>
// <nil>
// <nil>
// <nil>
