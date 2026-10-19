package cache

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"slices"
	"time"
)

// The file systems that the loads of the pipeline read through
// (load.Config.FS), so that a load sees exactly what its snapshot sees:
//
//   - stateFS reads the files, folder listings and stats of a snapshot (its
//     overlays, and the disk as the state read it once). The load of a
//     package's metadata records every read (reads), because the loader
//     resolves the imports with them (does the folder of an import exist,
//     which gocpp.mod is above it): the metadata is valid in another
//     snapshot only when the same reads give the same results there
//     (metadata.valid), and the reads are part of the key of the check.
//   - replayFS gives a check the results of the reads of its metadata, and
//     nothing else: a check reads only what the resolution of its imports
//     read (its own files come from load.Config.Overlay), so it depends
//     only on its key, also when it runs after its snapshot is gone.

// TODO(11.4, Phase 14): the names are taken as they come, not normalized:
// a folder reached through a symbolic link, and on Windows a name in other
// letter case, is another folder here than its real path, so two names of
// one package make two packages (gopls has the same rule for URIs).

// A read is one read of a load: a file, a folder listing or a stat, and
// what it gave.
type read struct {
	kind byte // 'f' ReadFile, 'd' ReadDir, 's' Stat
	path string

	exists bool     // the file or folder exists
	isDir  bool     // for a stat
	data   []byte   // for ReadFile
	names  []string // for ReadDir: the .gp files
	err    string   // the error of ReadFile or ReadDir, if it is not "does not exist"
}

// digest returns the hash of what r gave.
func (r *read) digest(h *hasher) {
	h.str(string(r.kind))
	h.str(r.path)
	h.bool(r.exists)
	h.bool(r.isDir)
	h.hash(hashOf(r.data))
	h.int(int64(len(r.names)))
	for _, n := range r.names {
		h.str(n)
	}
	h.str(r.err)
}

// same reports whether o is what r is, field by field: the equality that
// digest makes a hash of, without hashing the files' contents.
func (r *read) same(o *read) bool {
	return r.kind == o.kind && r.path == o.path && r.exists == o.exists && r.isDir == o.isDir &&
		r.err == o.err && bytes.Equal(r.data, o.data) && slices.Equal(r.names, o.names)
}

// A stateFS is a load.FileSystem over the snapshot of the state s. With
// log, it records each read.
type stateFS struct {
	s   *state
	log *[]*read
}

// do makes the read of kind and path in s.
func (s *state) do(kind byte, path string) (*read, error) {
	r := &read{kind: kind, path: path}
	switch kind {
	case 'f':
		f, err := s.file(path)
		if err != nil {
			return nil, err
		}
		switch {
		case f.err == nil:
			r.exists, r.data = true, f.src
		case errors.Is(f.err, fs.ErrNotExist):
		default:
			r.exists, r.err = true, f.err.Error()
		}
	case 'd':
		d, err := s.dir(path)
		if err != nil {
			return nil, err
		}
		r.names = d.names
		r.exists = d.isDir
		if d.isDir && d.err != nil {
			r.err = d.err.Error() // it can't be read
		}
	case 's':
		d, err := s.dir(path)
		if err != nil {
			return nil, err
		}
		if d.isDir {
			r.exists, r.isDir = true, true
			break
		}
		f, err := s.file(path)
		if err != nil {
			return nil, err
		}
		r.exists = f.err == nil || !errors.Is(f.err, fs.ErrNotExist)
	}
	return r, nil
}

func (fsys stateFS) get(kind byte, path string) (*read, error) {
	r, err := fsys.s.do(kind, path)
	if err == nil && fsys.log != nil {
		*fsys.log = append(*fsys.log, r)
	}
	return r, err
}

func (fsys stateFS) ReadFile(name string) ([]byte, error) {
	r, err := fsys.get('f', name)
	if err != nil {
		return nil, err
	}
	return r.file()
}

func (fsys stateFS) ReadDir(name string) ([]fs.DirEntry, error) {
	r, err := fsys.get('d', name)
	if err != nil {
		return nil, err
	}
	return r.dir()
}

func (fsys stateFS) Stat(name string) (fs.FileInfo, error) {
	r, err := fsys.get('s', name)
	if err != nil {
		return nil, err
	}
	return r.stat()
}

// file, dir and stat return what the read r gave, as the os functions do.
func (r *read) file() ([]byte, error) {
	if !r.exists {
		return nil, &fs.PathError{Op: "open", Path: r.path, Err: fs.ErrNotExist}
	}
	if r.err != "" {
		return nil, errors.New(r.err)
	}
	return r.data, nil
}

func (r *read) dir() ([]fs.DirEntry, error) {
	if !r.exists {
		return nil, &fs.PathError{Op: "open", Path: r.path, Err: fs.ErrNotExist}
	}
	if r.err != "" {
		return nil, errors.New(r.err)
	}
	entries := make([]fs.DirEntry, len(r.names))
	for i, n := range r.names {
		entries[i] = fileInfo{name: n}
	}
	return entries, nil
}

func (r *read) stat() (fs.FileInfo, error) {
	if !r.exists {
		return nil, &fs.PathError{Op: "stat", Path: r.path, Err: fs.ErrNotExist}
	}
	return fileInfo{name: filepath.Base(r.path), dir: r.isDir}, nil
}

// A replayFS is a load.FileSystem that gives the recorded reads, and an
// error for any other read (which would be a bug: see above).
type replayFS map[string]*read // by kind and path

func newReplayFS(reads []*read) replayFS {
	m := make(replayFS, len(reads))
	for _, r := range reads {
		m[string(r.kind)+r.path] = r
	}
	return m
}

func (fsys replayFS) get(kind byte, path string) (*read, error) {
	if r := fsys[string(kind)+path]; r != nil {
		return r, nil
	}
	return nil, fmt.Errorf("internal error: %s was not read when the imports were resolved", path)
}

func (fsys replayFS) ReadFile(name string) ([]byte, error) {
	r, err := fsys.get('f', name)
	if err != nil {
		return nil, err
	}
	return r.file()
}

func (fsys replayFS) ReadDir(name string) ([]fs.DirEntry, error) {
	r, err := fsys.get('d', name)
	if err != nil {
		return nil, err
	}
	return r.dir()
}

func (fsys replayFS) Stat(name string) (fs.FileInfo, error) {
	r, err := fsys.get('s', name)
	if err != nil {
		return nil, err
	}
	return r.stat()
}

// A fileInfo is a file or folder of a read, as an fs.FileInfo and an
// fs.DirEntry.
type fileInfo struct {
	name string
	dir  bool
}

func (fi fileInfo) Name() string { return fi.name }
func (fi fileInfo) Size() int64  { return 0 }
func (fi fileInfo) Mode() fs.FileMode {
	if fi.dir {
		return fs.ModeDir | 0o755
	}
	return 0o644
}
func (fi fileInfo) ModTime() time.Time         { return time.Time{} }
func (fi fileInfo) IsDir() bool                { return fi.dir }
func (fi fileInfo) Sys() any                   { return nil }
func (fi fileInfo) Type() fs.FileMode          { return fi.Mode().Type() }
func (fi fileInfo) Info() (fs.FileInfo, error) { return fi, nil }
