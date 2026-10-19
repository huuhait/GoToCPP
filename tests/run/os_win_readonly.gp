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

// The read-only attribute is the only permission bit of a file: Chmod sets or clears it from the
// write bit of the owner, Mode shows 0444 or 0666, a directory is 0777, and Remove clears the
// attribute when it has to.
func main() {
	d := setup()
	defer cleanup()
	p := d + `\f.txt`
	must(os.WriteFile(p, []byte("data"), 0o644))
	fi, _ := os.Stat(p)
	fmt.Println(fi.Mode(), fi.Mode().Perm())
	fmt.Println(os.Chmod(p, 0o444))
	fi, _ = os.Stat(p)
	fmt.Println(fi.Mode(), fi.Mode().Perm())
	_, err := os.OpenFile(p, os.O_WRONLY, 0)
	fmt.Println(e(err), os.IsPermission(err))
	fmt.Println(e(os.WriteFile(p, []byte("x"), 0o644)))
	f, err := os.Open(p)
	must(err)
	fmt.Println(e(f.Chmod(0o666)))
	fi, _ = f.Stat()
	fmt.Println(fi.Mode())
	f.Close()
	fmt.Println(os.Chmod(p, 0o400))
	fmt.Println(os.Remove(p))
	_, err = os.Stat(p)
	fmt.Println(os.IsNotExist(err))

	// Any write bit of the owner means writable; the other bits are ignored.
	must(os.WriteFile(p, nil, 0o666))
	for _, m := range []os.FileMode{0o200, 0o020, 0o002, 0o100, 0o755, 0o777, 0o000} {
		must(os.Chmod(p, m))
		fi, _ = os.Stat(p)
		fmt.Printf("%04o -> %v\n", uint32(m), fi.Mode())
	}
	must(os.Chmod(p, 0o666))

	// Directories.
	fi, _ = os.Stat(d)
	fmt.Println(fi.Mode(), fi.IsDir())
	must(os.Mkdir(d+`\sub`, 0o700))
	fi, _ = os.Stat(d + `\sub`)
	fmt.Println(fi.Mode())
	fmt.Println(os.Chmod(d+`\sub`, 0o555))
	fi, _ = os.Stat(d + `\sub`)
	fmt.Println(fi.Mode())
	// Chown and friends are not implemented.
	fmt.Println(e(os.Chown(p, 1, 1)))
	fmt.Println(e(os.Lchown(p, 1, 1)))
	fmt.Println(os.Getuid(), os.Getgid(), os.Geteuid(), os.Getegid())
	g, _ := os.Getgroups()
	fmt.Println(len(g))
}

// Output:
// -rw-rw-rw- -rw-rw-rw-
// <nil>
// -r--r--r-- -r--r--r--
// open $D\f.txt: Access is denied. true
// open $D\f.txt: Access is denied.
// chmod $D\f.txt: Access is denied.
// -r--r--r--
// <nil>
// <nil>
// true
// 0200 -> -rw-rw-rw-
// 0020 -> -r--r--r--
// 0002 -> -r--r--r--
// 0100 -> -r--r--r--
// 0755 -> -rw-rw-rw-
// 0777 -> -rw-rw-rw-
// 0000 -> -r--r--r--
// drwxrwxrwx true
// drwxrwxrwx
// <nil>
// dr-xr-xr-x
// chown $D\f.txt: not supported by windows
// lchown $D\f.txt: not supported by windows
// -1 -1 -1 -1
// 0
