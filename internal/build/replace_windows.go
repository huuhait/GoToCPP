//go:build windows

package build

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
)

// makeWritable clears the read-only attribute of the file name, if it is there: Windows refuses a rename onto a read-only
// file ("Access is denied"), where Linux only looks at the folder.
func makeWritable(name string) {
	if fi, err := os.Lstat(name); err == nil && fi.Mode().IsRegular() && fi.Mode().Perm()&0o200 == 0 {
		os.Chmod(name, fi.Mode().Perm()|0o200)
	}
}

// replaceError explains a refused replacement: after the retries of fsretry, and with the file no longer read-only,
// the file is in use. A running .exe cannot be replaced on Windows (Go's own build says the same).
func replaceError(err error, name string) error {
	if err != nil && errors.Is(err, fs.ErrPermission) {
		return fmt.Errorf("%w (is %s running? Windows cannot replace a program that runs: stop it, or write the build to another name)", err, name)
	}
	return err
}
