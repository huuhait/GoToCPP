package build

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/huuhait/gocpp/internal/fsretry"
)

// compile_commands.json (Phase 06 step 6.11, ADR-044): the JSON compilation database that clangd
// and other tools read (https://clang.llvm.org/docs/JSONCompilationDatabase.html). It has one
// entry for each translation unit of a build with the command that compiles it: the generated C++
// of the packages, the packages' own C and C++ files, the sources of the libraries of modules,
// and the files of gocpp's runtime. The files and the include folders are in stable folders (the
// folder of the generated files of the program in the cache, the runtime's folder in the cache,
// the packages' folders), not in the work folder that a build removes, so a tool can open them
// after the build. The entries are sorted by file, so the same program gives the same bytes.
//
// The "output" field is left out (the object files are in the build's work folder, which is gone
// when the build ends), and so are -o and the object file in the arguments: no tool needs them.

// A CompDBEntry is one entry of compile_commands.json.
type CompDBEntry struct {
	Directory string   `json:"directory"`
	File      string   `json:"file"`
	Arguments []string `json:"arguments"`
}

// CompDB returns the entries of the plan, sorted by file.
func (p *Plan) CompDB() []CompDBEntry {
	var entries []CompDBEntry
	for _, c := range p.sortedCompiles() {
		dir := c.Dir
		if dir == "" {
			dir = filepath.Dir(c.File)
		}
		entries = append(entries, CompDBEntry{Directory: dir, File: c.File, Arguments: append(append([]string(nil), c.Compiler...), c.CompileArgs()...)})
	}
	return entries
}

// CompDBJSON returns compile_commands.json for the plan: an array with one entry for each
// translation unit, indented, ending in a newline.
func (p *Plan) CompDBJSON() ([]byte, error) {
	entries := p.CompDB()
	if len(entries) == 0 {
		return nil, errors.New("the build has no compile commands")
	}
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(entries); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

// WriteCompDB writes the compilation database of the plan to path (the file, or a folder that
// gets compile_commands.json), and returns the name of the file. The file is replaced only when
// its content changes, and by a rename, so that a tool that watches it never reads half of it.
//
// A symbolic link is never written through: the file is a new regular file in its place (the file
// that the link pointed to is not touched, and a dangling link creates nothing at its target). A
// repository can ship such a link, and a build must not write where it points.
func WriteCompDB(p *Plan, path string) (string, error) {
	return WriteCompDBOwned(p, path, "", false)
}

// A ForeignCompDBError says that the automatic write of compile_commands.json found a file that
// gocpp did not write, or a symbolic link, and left it alone.
type ForeignCompDBError struct {
	Path string
	Why  string
}

func (e *ForeignCompDBError) Error() string { return e.Path + " " + e.Why }

// compDBKey returns the name that the cache knows path by: the folder of the file with its symbolic
// links resolved, and the name of the file (the file itself may be a link, which is not followed).
// The same file reached through a linked folder has one record.
func compDBKey(path string) string {
	if abs, err := filepath.Abs(path); err == nil {
		path = abs
	}
	dir, base := filepath.Split(path)
	if real, err := filepath.EvalSymlinks(dir); err == nil {
		dir = real
	}
	return filepath.Join(dir, base)
}

// compDBRecord returns the file in the cache that holds the hash of the last content that gocpp
// wrote to path.
func compDBRecord(cache, path string) string {
	return filepath.Join(cache, "compdb", hashHex([]byte(compDBKey(path)))[:32])
}

// WriteCompDBOwned is WriteCompDB that remembers what it wrote: the hash of the content goes to the
// cache folder, keyed by the path with the links of its folder resolved (compDBKey). With auto (the write that a debug build does by itself)
// it replaces a file only if the file still has the bytes that gocpp wrote last (or the new ones),
// and never a symbolic link or anything that is not a regular file: a database that a person or
// another tool (CMake, Bear) made is not touched, and a *ForeignCompDBError says so. Without auto
// (-compdb, gocpp export) a regular file is replaced, and so is a symbolic link (by a regular file:
// it is never written through). cache may be
// "" (nothing is remembered; auto then only writes where nothing is).
func WriteCompDBOwned(p *Plan, path, cache string, auto bool) (string, error) {
	data, err := p.CompDBJSON()
	if err != nil {
		return "", err
	}
	if !auto {
		// The automatic write is always given a file name; a name that the person gave may be
		// a folder, and says what it is with a trailing slash.
		if path, err = compDBTarget(path); err != nil {
			return path, err
		}
	} else if abs, err := filepath.Abs(path); err == nil {
		path = abs
	}
	if auto {
		if fi, err := os.Lstat(path); err == nil {
			if !fi.Mode().IsRegular() {
				return path, &ForeignCompDBError{path, "is a symbolic link or not a regular file"}
			}
			old, err := os.ReadFile(path)
			if err != nil {
				return path, err
			}
			if bytes.Equal(old, data) {
				recordCompDB(cache, path, data)
				return path, nil
			}
			rec, _ := os.ReadFile(compDBRecord(cache, path))
			if cache == "" || string(bytes.TrimSpace(rec)) != hashHex(old) {
				return path, &ForeignCompDBError{path, "exists and was not written by gocpp"}
			}
		}
	}
	if fi, err := os.Lstat(path); err == nil && fi.Mode()&os.ModeSymlink != 0 {
		// -compdb: the link is replaced by a file (a rename over the link), never written through.
		if err := replaceFile(path, data); err != nil {
			return path, err
		}
	} else if err := WriteFileIfChanged(path, data); err != nil {
		return path, err
	}
	recordCompDB(cache, path, data)
	return path, nil
}

// compDBTarget returns the file that the name a person gave (-compdb=name, gocpp export compdb
// -o name) stands for, as an absolute name, or says why gocpp does not write there. The name is
// looked at without following a link (os.Lstat), and decided like Go's -o:
//   - a folder, or a name that ends with a slash, gets compile_commands.json; a folder that does
//     not exist yet is made when the file is written (go build -o bin/ makes bin);
//   - a file that does not exist, or a regular file, is the file;
//   - a symbolic link is replaced by a file, as for the automatic write, never written through;
//     but a link to a folder is refused unless the name ends with a slash (then the person said
//     that the name is a folder, and the link is the way to it): the link is not silently
//     replaced by a file, and nothing is written behind it by accident;
//   - a named pipe, a socket or a device is refused: opening it would wait for ever or write to
//     a device. A trailing slash on a name that is not a folder is refused too.
func compDBTarget(name string) (string, error) {
	folder := name != "" && os.IsPathSeparator(name[len(name)-1])
	path := name
	if abs, err := filepath.Abs(name); err == nil {
		path = abs
	}
	fi, err := os.Lstat(path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		if folder {
			path = filepath.Join(path, "compile_commands.json")
		}
		return path, nil
	case err != nil:
		return path, err
	}
	mode := fi.Mode()
	switch {
	case mode.IsDir():
		return filepath.Join(path, "compile_commands.json"), nil
	case mode&fs.ModeSymlink != 0:
		target, err := os.Stat(path)
		switch {
		case err == nil && target.IsDir() && folder:
			return filepath.Join(path, "compile_commands.json"), nil
		case err == nil && target.IsDir():
			return path, fmt.Errorf("%s is a symbolic link to a folder: write %s%c to put compile_commands.json in the folder, "+
				"or name a file", path, name, filepath.Separator)
		case folder:
			return path, fmt.Errorf("%s is a symbolic link that does not lead to a folder", path)
		}
		return path, nil // a link to a file, or a dangling one: replaced by the file
	case !mode.IsRegular():
		return path, fmt.Errorf("%s is not a regular file (%s): not writing the compilation database there", path, fileKind(mode))
	case folder:
		return path, fmt.Errorf("%s is a file, not a folder", path)
	}
	return path, nil
}

// fileKind says what a file that is not regular is.
func fileKind(mode fs.FileMode) string {
	switch {
	case mode&fs.ModeNamedPipe != 0:
		return "a named pipe"
	case mode&fs.ModeSocket != 0:
		return "a socket"
	case mode&fs.ModeCharDevice != 0:
		return "a character device"
	case mode&fs.ModeDevice != 0:
		return "a device"
	}
	return "an irregular file"
}

// recordCompDB remembers that gocpp wrote data to path.
func recordCompDB(cache, path string, data []byte) {
	if cache == "" {
		return
	}
	file := compDBRecord(cache, path)
	if os.MkdirAll(filepath.Dir(file), 0o700) == nil {
		writeFileAtomic(file, []byte(hashHex(data)))
	}
}

// WriteFileIfChanged writes data to the file name unless it has these bytes already: through a
// temporary file next to it and a rename.
func WriteFileIfChanged(name string, data []byte) error {
	// Only a regular file is read: a named pipe would wait for ever (a file that is not regular is
	// replaced, like a link: the rename does not open it).
	if fi, err := os.Lstat(name); err == nil && fi.Mode().IsRegular() {
		if old, err := os.ReadFile(name); err == nil && bytes.Equal(old, data) {
			return nil
		}
	}
	return replaceFile(name, data)
}

// replaceFile writes data to the file name through a temporary file next to it and a rename: a
// file or a symbolic link that is there is replaced, not written through.
func replaceFile(name string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(name), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(name), ".tmp-"+filepath.Base(name)+"-")
	if err != nil {
		return err
	}
	_, werr := tmp.Write(data)
	cerr := tmp.Close()
	if err := errors.Join(werr, cerr, os.Chmod(tmp.Name(), 0o644)); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	if err := fsretry.Rename(tmp.Name(), name); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	return nil
}
