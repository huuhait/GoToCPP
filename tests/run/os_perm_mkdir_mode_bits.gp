// phase: 09

//go:build linux

package main

import (
	"fmt"
	"os"
	"syscall"
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
	old := syscall.Umask(0o022)
	defer syscall.Umask(old)
	must(os.Mkdir(d+"/d1", 0o777))
	must(os.Mkdir(d+"/d2", 0o700))
	must(os.WriteFile(d+"/f1", nil, 0o666))
	must(os.WriteFile(d+"/f2", nil, 0o600))
	f, _ := os.OpenFile(d+"/f3", os.O_CREATE|os.O_WRONLY, 0o755)
	f.Close()
	for _, n := range []string{"d1", "d2", "f1", "f2", "f3"} {
		fi, _ := os.Stat(d + "/" + n)
		fmt.Println(n, fi.Mode())
	}
}

// Output:
// d1 drwxr-xr-x
// d2 drwx------
// f1 -rw-r--r--
// f2 -rw-------
// f3 -rwxr-xr-x
