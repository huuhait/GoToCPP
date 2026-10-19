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
	p := d + "/m"
	must(os.WriteFile(p, nil, 0o600))
	for _, m := range []os.FileMode{0o000, 0o400, 0o640, 0o755, 0o777, 0o4755, 0o2755, 0o1755, os.ModeSetuid | 0o700, os.ModeSetgid | 0o700, os.ModeSticky | 0o700, os.ModeDir | 0o700} {
		err := os.Chmod(p, m)
		fi, _ := os.Stat(p)
		fmt.Printf("%v %s -> %v %o\n", m, e(err), fi.Mode(), uint32(fi.Mode().Perm()))
	}
	os.Chmod(p, 0o600)
}

// Output:
// ---------- <nil> -> ---------- 0
// -r-------- <nil> -> -r-------- 400
// -rw-r----- <nil> -> -rw-r----- 640
// -rwxr-xr-x <nil> -> -rwxr-xr-x 755
// -rwxrwxrwx <nil> -> -rwxrwxrwx 777
// -rwxr-xr-x <nil> -> -rwxr-xr-x 755
// -rwxr-xr-x <nil> -> -rwxr-xr-x 755
// -rwxr-xr-x <nil> -> -rwxr-xr-x 755
// urwx------ <nil> -> urwx------ 700
// grwx------ <nil> -> grwx------ 700
// trwx------ <nil> -> trwx------ 700
// drwx------ <nil> -> -rwx------ 700
