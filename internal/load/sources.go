package load

import (
	"bufio"
	"errors"
	"fmt"
	"go/token"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/huuhait/gocpp/internal/diag"
)

// The C and C++ sources of a package (Phase 04 README §2.9, step 4.13):
// gocpp build compiles the .c files of a package folder with the C compiler
// and its .cc, .cpp and .cxx files with the C++ compiler, and links them with
// the package, like cgo does. These are cgo's extensions; .c++ and .C are not
// (go/build doesn't know them either), and like every other file they are
// left alone. Headers are not compiled. The other sources that cgo
// compiles (assembly, Objective-C, Fortran, SWIG, .syso objects) are an
// error: a package that has them would be linked without them.

// unsupportedSources are the extensions of the files that cgo compiles or
// links but gocpp doesn't, with the kind of file for the message.
var unsupportedSources = map[string]string{
	".s": "assembly", ".S": "assembly", ".sx": "assembly",
	".m": "Objective-C",
	".f": "Fortran", ".F": "Fortran", ".for": "Fortran", ".f90": "Fortran",
	".swig": "SWIG", ".swigcxx": "SWIG",
	".syso": "system object",
}

// addSource adds the file name of the folder of p to p.CFiles or
// p.CXXFiles when it is a C or C++ source that the build constraints keep
// (the file name suffixes and //go:build lines of go/build, which reads the
// comments at the top of a C file as it does for a .go file). It reports a
// broken build constraint, and a kind of source that gocpp doesn't compile,
// as an error of the file.
func (l *loader) addSource(p *Package, name string) {
	ext := filepath.Ext(name)
	var list *[]string
	switch ext {
	case ".c":
		list = &p.CFiles
	case ".cc", ".cpp", ".cxx":
		list = &p.CXXFiles
	default:
		if unsupportedSources[ext] == "" {
			return
		}
	}
	path := filepath.Join(p.Dir, name)
	var src []byte
	if ext != ".syso" { // a binary file: only its name counts, as in go/build
		var err error
		if src, err = l.readHeader(p, name); err != nil {
			p.Errors.Add(diag.Diagnostic{Pos: token.Position{Filename: path}, Msg: err.Error()})
			return
		}
	}
	ok, msg := l.match.file(name, src)
	switch {
	case msg != "":
		p.Errors.Add(diag.Diagnostic{Pos: token.Position{Filename: path}, Msg: msg})
	case !ok:
	case list == nil:
		p.Errors.Add(diag.Diagnostic{Pos: token.Position{Filename: path}, Msg: fmt.Sprintf(
			"%s files (%s) are not supported: gocpp compiles only the .c, .cc, .cpp and .cxx files of a package", unsupportedSources[ext], ext)})
	default:
		*list = append(*list, name)
	}
}

// An opener is a FileSystem that can also open a file for reading (the
// operating system's): then readHeader reads only the start of a C file.
// The language server's file systems read whole files, which they have in
// memory, and record what was read. A FileSystem that records or replays
// what ReadFile reads must not get an Open method, for example by
// embedding osFS: the reads of C files would then bypass it.
type opener interface {
	Open(name string) (io.ReadCloser, error)
}

// readHeader returns the start of the file name in the folder of p, up to
// the end of the comments at its top: what go/build reads of a C file for
// its build constraints. A big C file (a vendored sqlite3.c) is then not
// read whole at each load. Without a FileSystem that can open files, or
// for a file of Config.Overlay or of the standard library, it reads the
// whole file.
func (l *loader) readHeader(p *Package, name string) ([]byte, error) {
	path := filepath.Join(p.Dir, name)
	o, ok := l.cfg.FS.(opener)
	if _, overlay := l.cfg.Overlay[path]; !ok || overlay || p.Std {
		return l.readFile(p, name)
	}
	f, err := o.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return leadingComments(bufio.NewReader(f))
}

// Open opens a regular file (or a link to one) for reading; see openRegular.
func (osFS) Open(name string) (io.ReadCloser, error) {
	f, _, err := openRegular(name)
	if err != nil {
		return nil, err
	}
	return f, nil
}

// afterOpenHook, if not nil, runs between the open and the look at the open file in openRegular:
// a test swaps the name there for a named pipe, to see that the kind is asked of the open file.
// It is a plain variable: the test that sets it must not run in parallel with other tests of the
// package that read files (it does not call t.Parallel and restores it when it is done).
var afterOpenHook func(name string)

// openRegular opens the file name, following a link, and returns it with its size. It does not
// wait for a writer when the name is a named pipe, and it refuses what is not a regular file once
// it is open (a named pipe, a socket, a device, a folder) with an error that says what it is: a
// file with the name of a source that is one of these must not make the load hang (mkfifo
// pipe.gp) or read for ever (a link to /dev/zero). The kind is asked of the open file, not of
// the name, so a file swapped for a pipe in between is caught too.
func openRegular(name string) (*os.File, int64, error) {
	f, err := os.OpenFile(name, os.O_RDONLY|openNonblock, 0)
	if err != nil {
		return nil, 0, openFailed(name, err)
	}
	if afterOpenHook != nil {
		afterOpenHook(name)
	}
	fi, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, 0, err
	}
	if !fi.Mode().IsRegular() {
		f.Close()
		return nil, 0, NotRegular(name, fi.Mode())
	}
	return f, fi.Size(), nil
}

// NotRegular returns the error for the file name, whose mode is mode, not being a regular file:
// what ReadRegular says, for the code that finds out by a look at the name.
func NotRegular(name string, mode fs.FileMode) error {
	return &fs.PathError{Op: "open", Path: name, Err: irregularError(mode)}
}

// openFailed turns the error of the open of name into the one that says what the file is, where
// the system's text does not: a socket cannot be opened, and the open says "no such device or
// address".
func openFailed(name string, err error) error {
	if !isNoDevice(err) {
		return err
	}
	if fi, serr := os.Stat(name); serr == nil && !fi.Mode().IsRegular() {
		return NotRegular(name, fi.Mode())
	}
	return err
}

// IsNotRegular reports whether err says that a file is not a regular file (a named pipe, a socket,
// a device, a folder), like the errors of ReadRegular and OpenRegular. A file that cannot be
// opened for another reason (it is not there, its permissions) is not one.
func IsNotRegular(err error) bool {
	var kind irregularError
	return errors.As(err, &kind)
}

// irregularError says what kind of file a file that is not regular is.
type irregularError fs.FileMode

func (e irregularError) Error() string {
	mode := fs.FileMode(e)
	kind := "an irregular file"
	switch {
	case mode.IsDir():
		kind = "a folder"
	case mode&fs.ModeNamedPipe != 0:
		kind = "a named pipe"
	case mode&fs.ModeSocket != 0:
		kind = "a socket"
	case mode&fs.ModeCharDevice != 0:
		kind = "a character device"
	case mode&fs.ModeDevice != 0:
		kind = "a device"
	}
	return "not a regular file: it is " + kind
}

// leadingComments reads the spaces, // comments and /* */ comments at the
// start of r, and returns them with the byte after them: what go/build's
// readComments reads, which then finds the //go:build lines in them. Like
// it, it skips a UTF-8 byte order mark at the start (and keeps it in the
// result, where readComments drops it again), and it counts \f and ; as
// spaces.
func leadingComments(r *bufio.Reader) ([]byte, error) {
	var buf []byte
	if bom, _ := r.Peek(3); string(bom) == "\xef\xbb\xbf" {
		buf = append(buf, bom...) // before Discard, which ends the life of bom
		r.Discard(3)
	}
	next := func() (byte, error) {
		c, err := r.ReadByte()
		if err == nil {
			buf = append(buf, c)
		}
		return c, err
	}
	for {
		c, err := next()
		if err == nil && c == '/' {
			c, err = next()
			switch {
			case err == nil && c == '/':
				for err == nil && c != '\n' {
					c, err = next()
				}
				c = ' '
			case err == nil && c == '*':
				var prev byte
				for c, err = next(); err == nil && (prev != '*' || c != '/'); c, err = next() {
					prev = c
				}
				c = ' '
			}
		}
		switch {
		case errors.Is(err, io.EOF):
			return buf, nil
		case err != nil:
			return nil, err
		case c != ' ' && c != '\t' && c != '\n' && c != '\r' && c != '\f' && c != ';':
			return buf, nil
		}
	}
}

// OpenRegular opens the file name like openRegular, for the code outside the package that reads
// a file by name (gocpp fmt): a named pipe or a device is an error, not a wait.
func OpenRegular(name string) (*os.File, error) {
	f, _, err := openRegular(name)
	return f, err
}

// DirFS returns a file system of the folder dir like os.DirFS, except that a file that is not a
// regular file (a named pipe, a socket, a device) cannot be opened: the error says what it is.
// The kind is asked of the open file, so there is no window between a look and the open. The
// reads of gocpp doc go through it. The names are os.DirFS's (slash separated; on Windows a name
// with a backslash or a colon is invalid), and so are the errors' operations and names. Two things
// differ: ReadFile of a folder says "not a regular file: it is a folder" (os: "is a directory"),
// and the error of a name that cannot be opened says the system's text for the full path inside
// the PathError's Err only when it is one of the system's.
func DirFS(dir string) fs.FS { return regularDirFS(dir) }

type regularDirFS string

// path returns the file of the name in the folder, or the invalid-argument error that os.DirFS
// gives for a name that is not a valid slash path, has a NUL, or (Windows) a separator or a
// drive of the system.
func (d regularDirFS) path(op, name string) (string, error) {
	invalid := !fs.ValidPath(name) || strings.IndexByte(name, 0) >= 0
	if filepath.Separator != '/' && strings.ContainsAny(name, `\:`) {
		invalid = true
	}
	if invalid {
		return "", &fs.PathError{Op: op, Path: name, Err: fs.ErrInvalid}
	}
	return filepath.Join(string(d), filepath.FromSlash(name)), nil
}

// named gives err the operation and the name of the fs call, as os.DirFS does, not the full path.
func named(op, name string, err error) error {
	var pathErr *fs.PathError
	if errors.As(err, &pathErr) {
		return &fs.PathError{Op: op, Path: name, Err: pathErr.Err}
	}
	return err
}

func (d regularDirFS) Open(name string) (fs.File, error) {
	path, err := d.path("open", name)
	if err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_RDONLY|openNonblock, 0)
	if err != nil {
		return nil, named("open", name, openFailed(path, err))
	}
	fi, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, named("open", name, err)
	}
	if !fi.Mode().IsRegular() && !fi.IsDir() {
		f.Close()
		return nil, named("open", name, NotRegular(path, fi.Mode()))
	}
	return f, nil
}

func (d regularDirFS) ReadFile(name string) ([]byte, error) {
	path, err := d.path("readfile", name)
	if err != nil {
		return nil, err
	}
	data, err := ReadRegular(path)
	return data, named("readfile", name, err)
}

func (d regularDirFS) ReadDir(name string) ([]fs.DirEntry, error) {
	path, err := d.path("readdir", name)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(path)
	return entries, named("readdir", name, err)
}

func (d regularDirFS) Stat(name string) (fs.FileInfo, error) {
	path, err := d.path("stat", name)
	if err != nil {
		return nil, err
	}
	fi, err := os.Stat(path)
	if err != nil {
		return nil, named("stat", name, err)
	}
	if name == "." {
		return dotInfo{fi}, nil
	}
	return fi, nil
}

// dotInfo is the information about the root folder: its name is ".", as in os.DirFS.
type dotInfo struct{ fs.FileInfo }

func (dotInfo) Name() string { return "." }
