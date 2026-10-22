// The command line, the folder walk and processFile in this file follow
// gofmtMain, isGoFile and processFile in cmd/gofmt/gofmt.go of Go 1.25.14
// closely: Copyright 2009 The Go Authors. All rights reserved. Use of
// that source code is governed by a BSD-style license that can be found
// in internal/format/LICENSE.
//
// gocpp: one change, on purpose. When the folder walk reaches a file
// through a symbolic link, its fs.FileInfo describes the link, not the
// file. gofmt -w takes the size and the mode of the link from it, so it
// cuts the file only down to the length of the link's text: when the
// formatted text is shorter, the end of the old text can stay in the
// file, and the backup gets the mode of the link (0777: with the usual
// umask, a 0600 file gets a 0755 backup). gocpp fmt -w uses the size of
// what it read and the mode of the file itself (writePerm). What it
// prints and its exit status stay the same as gofmt's.

package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/huuhait/gocpp/internal/diff"
	"github.com/huuhait/gocpp/internal/format"
	"github.com/huuhait/gocpp/internal/syntax/scanner"
)

// fmtUsage is the help text of "gocpp fmt". "gocpp fmt -h" prints it,
// followed by the flags.
const fmtUsage = `usage: gocpp fmt [-l] [-w] [-d] [path ...]

gocpp fmt formats .gp files the way gofmt formats Go files. A path is a
file or a folder. In a folder, gocpp fmt formats every .gp file, also in
the folders inside it, but not the files whose names start with a dot.
A file named on the command line is always formatted; if it is a named
pipe (gocpp fmt /dev/stdin, gocpp fmt <(...)), it is read as gofmt reads it,
which waits for a writer of the pipe, and -w refuses it. In a folder, a
named pipe with the name of a source is an error. With no path, it
formats standard input, which may also be a part of a file, and prints
the result.

Like gofmt, it sorts the imports. But an import group that contains a
header or a namespace import (like "<windows.h>" or fs "::std::filesystem")
stays as you wrote it, because the order of #include lines can matter.
A file with syntax errors or //gocpp: directive errors is not changed:
gocpp fmt prints the errors and exits with status 2.

The flags are:
`

// runFmt is "gocpp fmt" (phase 01 README §2.11): it formats .gp files with
// internal/format. Its flags, its output and its exit status are gofmt's,
// and the steps are those of cmd/gofmt (gofmtMain and processFile), but it
// formats one file at a time, where gofmt formats several at the same
// time.
func runFmt(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	c := &fmtCommand{stdout: stdout, stderr: stderr}
	flags := flag.NewFlagSet("gocpp fmt", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.BoolVar(&c.list, "l", false, "list the files whose formatting differs from gocpp fmt's")
	flags.BoolVar(&c.write, "w", false, "write the result back to the file, not to standard output")
	flags.BoolVar(&c.diff, "d", false, "print diffs, not the formatted source")
	flags.Usage = func() {
		fmt.Fprint(stderr, fmtUsage)
		flags.PrintDefaults()
	}
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0 // like gofmt -h
		}
		return 2 // flag has printed the error and the usage
	}

	paths := flags.Args()
	if len(paths) == 0 {
		if c.write {
			c.report(errors.New("error: cannot use -w with standard input"))
			return c.exit
		}
		c.process("<standard input>", nil, stdin)
		return c.exit
	}
	for _, arg := range paths {
		switch info, err := os.Stat(arg); {
		case err != nil:
			c.report(err)
		case !info.IsDir():
			// A file named on the command line is always formatted.
			c.explicit = true
			c.process(arg, info, nil)
			c.explicit = false
		default:
			// A folder is walked, and only its .gp files are formatted. Like
			// gofmt, the walk goes into every folder (testdata, vendor and
			// folders whose names start with a dot too), and it stops at
			// the first folder that it can't read.
			err := filepath.WalkDir(arg, func(path string, d fs.DirEntry, err error) error {
				if err != nil || !isGpFile(d) {
					return err
				}
				info, err := d.Info()
				if err != nil {
					c.report(err)
					return nil
				}
				c.process(path, info, nil)
				return nil
			})
			if err != nil {
				c.report(err)
			}
		}
	}
	return c.exit
}

// isGpFile reports whether gocpp fmt formats the folder entry d when it
// walks a folder: a .gp file whose name doesn't start with a dot, like
// isGoFile in gofmt for .go files.
func isGpFile(d fs.DirEntry) bool {
	name := d.Name()
	return !strings.HasPrefix(name, ".") && strings.HasSuffix(name, ".gp") && !d.IsDir()
}

// A fmtCommand is one run of "gocpp fmt": its flags, where it prints, and
// its exit status.
type fmtCommand struct {
	list, write, diff bool
	stdout, stderr    io.Writer
	exit              int
	explicit          bool // the file is named on the command line (not found by the walk)
}

// report prints err the way gofmt prints an error: a list of syntax or
// directive errors one per line, with their positions. Then gocpp fmt
// exits with status 2.
func (c *fmtCommand) report(err error) {
	scanner.PrintError(c.stderr, err)
	c.exit = 2
}

// process formats one file and reports its error, if any. info is nil for
// standard input, which is in.
func (c *fmtCommand) process(filename string, info fs.FileInfo, in io.Reader) {
	if err := c.processFile(filename, info, in); err != nil {
		c.report(err)
	}
}

// processFile formats one file, like processFile of gofmt. Without -l, -w
// and -d, it prints the result. With them, it does something only when
// the result is not the same as the file: -l prints the file name, -w
// writes the result back into the file, and -d prints a diff.
func (c *fmtCommand) processFile(filename string, info fs.FileInfo, in io.Reader) error {
	var src, res []byte
	var err error
	if info == nil {
		// Standard input may be a part of a file, as in gofmt.
		if src, err = io.ReadAll(in); err == nil {
			res, err = format.Fragment(filename, src)
		}
	} else if src, err = c.readSource(filename, info); err == nil {
		res, err = format.Source(filename, src)
	}
	if err != nil {
		return err
	}

	if !bytes.Equal(src, res) {
		if c.list {
			fmt.Fprintln(c.stdout, filename)
		}
		if c.write {
			// gocpp: not info.Size() and info.Mode().Perm(), which describe a
			// symbolic link when the walk reaches the file through one (see
			// the note at the top). For a regular file, ReadFile checked that
			// len(src) is info.Size().
			perm, err := writePerm(filename, info)
			if err != nil {
				return err
			}
			if err := format.WriteFile(filename, src, res, perm, int64(len(src))); err != nil {
				return err
			}
		}
		if c.diff {
			newName := filepath.ToSlash(filename)
			c.stdout.Write(diff.Diff(newName+".orig", src, newName, res))
		}
	}
	if !c.list && !c.write && !c.diff {
		_, err = c.stdout.Write(res)
	}
	return err
}

// readSource reads the file filename, which info describes. A file named on the command line
// that is not a regular file (a named pipe from a shell's process substitution, /dev/stdin) is
// opened as gofmt opens it: it waits for a writer. A file that the walk found is read by
// format.ReadFile, which refuses it: a pipe with the name of a source must not stop the run.
func (c *fmtCommand) readSource(filename string, info fs.FileInfo) ([]byte, error) {
	if !c.explicit || info.Mode().IsRegular() {
		return format.ReadFile(filename, info)
	}
	if c.write {
		return nil, fmt.Errorf("error: cannot use -w with %s: it is not a regular file", filename)
	}
	in, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer in.Close()
	return io.ReadAll(in)
}

// writePerm returns the mode bits of the file filename, for gocpp fmt -w:
// format.WriteFile gives them to the backup of the file. info describes
// the file, except when the folder walk reaches the file through a
// symbolic link: then info describes the link, whose mode is 0777, so
// writePerm asks for the mode of the file itself. (gofmt uses the mode of
// the link.)
func writePerm(filename string, info fs.FileInfo) (fs.FileMode, error) {
	if info.Mode()&fs.ModeSymlink == 0 {
		return info.Mode().Perm(), nil
	}
	fi, err := os.Stat(filename)
	if err != nil {
		return 0, err
	}
	return fi.Mode().Perm(), nil
}
