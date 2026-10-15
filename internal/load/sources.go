package load

import (
	"bufio"
	"errors"
	"fmt"
	"go/token"
	"io"
	"os"
	"path/filepath"

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

func (osFS) Open(name string) (io.ReadCloser, error) { return os.Open(name) }

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
