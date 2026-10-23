// phase: 09
//go:build linux

// RemoveAll is Go's removeall_at.go (openat/unlinkat relative to open directories): a tree deeper
// than PATH_MAX goes, "file/x" with file a regular file is "unlinkat ...: not a directory", the
// permission errors say "unlinkat" and "openfdat", a symlink to a directory is removed, not
// followed (dev.syscalldiffs).

package main

import (
	"fmt"
	"os"
	"strings"
)

var tmp string

func e(err error) string {
	if err == nil {
		return "<nil>"
	}
	return strings.ReplaceAll(err.Error(), tmp, "$D")
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}

func main() {
	d, err := os.MkdirTemp("", "rmall")
	must(err)
	tmp = d
	defer os.RemoveAll(d)

	// a file in the way
	must(os.WriteFile(d+"/file", []byte("x"), 0o644))
	fmt.Println("file/x:", e(os.RemoveAll(d+"/file/x")))
	fmt.Println("missing:", e(os.RemoveAll(d+"/none/x")))
	fmt.Println("dot:", e(os.RemoveAll(d+"/.")))
	fmt.Println("empty:", e(os.RemoveAll("")))

	// a tree with a symlink to a directory outside: the link goes, the target stays
	must(os.MkdirAll(d+"/keep/sub", 0o755))
	must(os.WriteFile(d+"/keep/sub/f", nil, 0o644))
	must(os.MkdirAll(d+"/t/a/b", 0o755))
	must(os.Symlink(d+"/keep", d+"/t/a/link"))
	must(os.WriteFile(d+"/t/a/b/c", nil, 0o644))
	fmt.Println("tree:", e(os.RemoveAll(d+"/t")))
	_, err = os.Stat(d + "/keep/sub/f")
	fmt.Println("kept:", e(err))
	fmt.Println("symlink itself:", e(os.RemoveAll(d+"/t")))
	must(os.Symlink(d+"/keep", d+"/ln"))
	fmt.Println("link to dir:", e(os.RemoveAll(d+"/ln")))
	_, err = os.Stat(d + "/keep/sub/f")
	fmt.Println("kept:", e(err))

	// a directory that we cannot write: its file stays, the error names unlinkat
	if os.Getuid() != 0 {
		must(os.MkdirAll(d+"/ro/x", 0o755))
		must(os.WriteFile(d+"/ro/x/f", nil, 0o644))
		must(os.WriteFile(d+"/ro/g", nil, 0o644))
		must(os.Chmod(d+"/ro/x", 0o555))
		fmt.Println("read-only:", e(os.RemoveAll(d+"/ro")))
		must(os.Chmod(d+"/ro/x", 0o755))
		// an unreadable directory
		must(os.MkdirAll(d+"/nr/y", 0o755))
		must(os.WriteFile(d+"/nr/y/f", nil, 0o644))
		must(os.Chmod(d+"/nr/y", 0o300))
		fmt.Println("unreadable:", e(os.RemoveAll(d+"/nr")))
		must(os.Chmod(d+"/nr/y", 0o755))
		fmt.Println("after chmod:", e(os.RemoveAll(d+"/ro")), e(os.RemoveAll(d+"/nr")))
	}

	// deeper than PATH_MAX: built with Chdir, removed in one call
	wd, _ := os.Getwd()
	must(os.Mkdir(d+"/deep", 0o755))
	must(os.Chdir(d + "/deep"))
	name := strings.Repeat("d", 200)
	for i := 0; i < 30; i++ {
		must(os.Mkdir(name, 0o755))
		must(os.Chdir(name))
	}
	must(os.WriteFile("leaf", nil, 0o644))
	must(os.Chdir(wd))
	fmt.Println("deep:", e(os.RemoveAll(d+"/deep")))
	_, err = os.Lstat(d + "/deep")
	fmt.Println("gone:", e(err))
}
