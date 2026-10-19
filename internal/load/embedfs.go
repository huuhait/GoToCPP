package load

import (
	"fmt"
	"io"
	"io/fs"
	"os"
)

// What the embed resolver (embed.go) asks of a file system beyond FileSystem. A FileSystem that
// does not have them gets the plain methods, so the language server's file systems need nothing.

// A FullLister lists a folder so that the caller sees every file that comes or goes, also those
// the loader ignores by name. The build's recorder (internal/build) uses it to put the folders
// of //go:embed patterns in its stamp unfiltered: a file added to such a folder changes the
// program, and the stamp must say so.
type FullLister interface {
	ReadDirAll(name string) ([]fs.DirEntry, error)
}

// A LimitedReader reads a regular file of at most limit bytes without following a symbolic
// link, and says if the file is bigger (a *TooBigError). The embed resolver reads the files of
// the program through it.
type LimitedReader interface {
	ReadFileLimit(name string, limit int64) ([]byte, error)
}

// A TooBigError says that a file is bigger than the limit; Size is its size, or limit+1 when only
// the read found out.
type TooBigError struct{ Size int64 }

func (e *TooBigError) Error() string { return fmt.Sprintf("file is %d bytes", e.Size) }

// ReadRegularFile reads the file name, which must be a regular file of at most limit bytes: it is
// opened without following a symbolic link and without blocking (a named pipe or a device is
// refused after the open), and the size is asked of the open file, not of the name, so a file
// swapped between a look and the read (a link to /dev/zero) can't make the read grow without end.
func ReadRegularFile(name string, limit int64) ([]byte, error) {
	f, err := os.OpenFile(name, os.O_RDONLY|openNoFollow, 0)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !fi.Mode().IsRegular() {
		return nil, fmt.Errorf("cannot embed irregular file %s", name)
	}
	if fi.Size() > limit {
		return nil, &TooBigError{fi.Size()}
	}
	data, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, &TooBigError{limit + 1}
	}
	return data, nil
}

func (osFS) ReadDirAll(name string) ([]fs.DirEntry, error) { return os.ReadDir(name) }

func (osFS) ReadFileLimit(name string, limit int64) ([]byte, error) {
	return ReadRegularFile(name, limit)
}
