package diag

import (
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// TestGoBuild checks that Print prints exactly what go build prints.
//
// testdata/gobuild/m is a small Go module whose packages have errors.
// testdata/gobuild/want holds what the real go build printed for each case
// in testdata/gobuild/cases.txt. testdata/gobuild/capture.sh captured the
// files with the dev shell's Go (want/go-version.txt has the version).
// To check them again, run it and look at git diff:
//
//	nix develop -c sh internal/diag/testdata/gobuild/capture.sh
//	git diff internal/diag/testdata   # must show nothing
//
// The test type-checks each package with go/types, collects the errors in a
// List and prints them as go build would in the same current folder. go
// build also prints a line like "# example.com/m/two" before the errors of
// a package. The test skips that line: the gocpp command prints it.
//
// What the files show about go build (Go 1.25):
//   - File names (cwd, subdir, parent, sibling, absolute): ./main.go in the
//     current folder, sub/a.go in a sub-folder, ../main.go and
//     ../two/a.go outside it, and the absolute name when it is shorter.
//     Related places use the same names (redecl, redecl-parent,
//     redecl-absolute).
//   - Order (two, order): files come in the order of their first error,
//     not by name. In order/, b.go comes first, because the error in its
//     package-level variable is found before the error in a.go's function.
//   - Limit (nine, ten, eleven, twelve-e, reportorder): after the 10th
//     error, go build prints "<position of the 10th error>: too many
//     errors", also when there are exactly 10. The 10 are the first 10
//     errors found, not the first 10 in the file. -gcflags=-e prints all.
//   - Duplicates (dups, dups-e, interleave, samelinecount): an error with
//     the same line and message as the error just before it is dropped,
//     also with -e, and it does not count for the limit. x, y, x on one
//     line gives three errors.
//   - At the same line, errors are sorted by column (samepos), and
//     messages with more lines print as they are (multiline).
//
// The two known differences (see Print) are not in these files.
// TestPrintLongColumns has the go build output for the column.
func TestGoBuild(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("testdata", "gobuild", "m"))
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range readCases(t) {
		t.Run(c.name, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join("testdata", "gobuild", "want", c.name+".txt"))
			if err != nil {
				t.Fatal(err)
			}
			// A Windows checkout can have "\r\n" line ends.
			text := strings.ReplaceAll(string(data), "\r\n", "\n")
			var want strings.Builder
			for line := range strings.Lines(text) {
				if !strings.HasPrefix(line, "# ") {
					// The files come from Linux. On Windows, go build
					// shows the same paths with backslashes (the
					// messages of these cases have no slashes).
					want.WriteString(strings.ReplaceAll(filepath.FromSlash(line), "$ROOT", root))
				}
			}

			var l List
			typeCheck(t, filepath.Join(root, c.pkg), &l)
			dir := filepath.Join(root, c.cwd)
			if c.cwd == "DEEP" {
				// Like capture.sh: deep enough that absolute names are shorter.
				dir = filepath.Join(root, "deep") + strings.Repeat(string(filepath.Separator)+"d", len(root))
			}
			var got strings.Builder
			p := Printer{All: c.all, Dir: dir}
			if err := p.Print(&got, &l); err != nil {
				t.Fatal(err)
			}
			if got.String() != want.String() {
				t.Errorf("got:\n%s\nwant (from go build):\n%s", got.String(), want.String())
			}
		})
	}
}

// TestGoBuildFiles checks that every file in testdata/gobuild/want belongs
// to a case, so that no old file stays behind.
func TestGoBuildFiles(t *testing.T) {
	var names []string
	for _, c := range readCases(t) {
		names = append(names, c.name+".txt")
	}
	files, err := filepath.Glob(filepath.Join("testdata", "gobuild", "want", "*.txt"))
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		name := filepath.Base(f)
		if name != "go-version.txt" && !slices.Contains(names, name) {
			t.Errorf("%s is not a case in cases.txt", f)
		}
	}
}

// A goBuildCase is one line of testdata/gobuild/cases.txt.
type goBuildCase struct {
	name string // the name of the file in want/, without .txt
	cwd  string // the current folder of go build, relative to m/, or DEEP
	pkg  string // the package folder, relative to m/
	all  bool   // go build -gcflags=-e
}

func readCases(t *testing.T) []goBuildCase {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "gobuild", "cases.txt"))
	if err != nil {
		t.Fatal(err)
	}
	var cases []goBuildCase
	for line := range strings.Lines(string(data)) {
		f := strings.Fields(line)
		if len(f) == 0 || strings.HasPrefix(f[0], "#") {
			continue
		}
		if len(f) < 3 || len(f) > 4 || len(f) == 4 && f[3] != "-e" {
			t.Fatalf("cases.txt: bad line %q", line)
		}
		cases = append(cases, goBuildCase{name: f[0], cwd: f[1], pkg: f[2], all: len(f) == 4})
	}
	if len(cases) == 0 {
		t.Fatal("cases.txt has no cases")
	}
	return cases
}

// typeCheck parses the .go files in dir and type-checks them with go/types,
// with l.Collect as the error handler. Like the go command, it gives the
// parser absolute file names, sorted by name.
func typeCheck(t *testing.T, dir string, l *List) {
	t.Helper()
	names, err := filepath.Glob(filepath.Join(dir, "*.go"))
	if err != nil || len(names) == 0 {
		t.Fatalf("no .go files in %s (%v)", dir, err)
	}
	fset := token.NewFileSet()
	var files []*ast.File
	for _, name := range names {
		f, err := parser.ParseFile(fset, name, nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, f)
	}
	conf := types.Config{Error: l.Collect}
	conf.Check(files[0].Name.Name, fset, files, nil) // the errors go to l
}
