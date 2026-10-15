package load

import (
	"bufio"
	"io/fs"
	"strings"
	"testing"
)

// TestSources checks the C and C++ files of a package (step 4.13): .c in
// CFiles, .cc, .cpp and .cxx in CXXFiles, by name, with the build
// constraints of .gp files (file name suffixes, //go:build at the top);
// headers and other files are not sources, and the sources that cgo builds
// but gocpp doesn't are errors.
func TestSources(t *testing.T) {
	files := map[string]string{
		"gocpp.mod":     "module m\n",
		"main.gp":       "package main\n\nimport \"m/lib\"\n\nfunc main() { lib.F() }\n",
		"a.c":           "int a;\n",
		"a.cpp":         "int b;\n",
		"b.cc":          "",
		"c.cxx":         "",
		"d.c++":         "", // not a cgo extension: left alone
		"e.C":           "",
		"h.h":           "", // headers are not compiled
		"h.hpp":         "",
		"x_linux.c":     "",
		"x_windows.cpp": "",
		"x_arm64.c":     "",
		"tag.c":         "// A C file for Windows only.\n//go:build windows\n\n#include <stdio.h>\n",
		"late.c":        "#include <stdio.h>\n//go:build windows\n", // after code: no constraint
		"plus.cc":       "// +build ignore\n\nint x;\n",
		"_skip.c":       "",
		".hidden.c":     "",
		"dir.c/x.gp":    "package x\n", // a folder
		"lib/lib.c":     "",
		"lib/lib.gp":    "package lib\n\nfunc F() {}\n",
	}
	root := writeTree(t, files)
	for _, tc := range []struct {
		goos, goarch string
		c, cxx       string
	}{
		{"linux", "amd64", "a.c late.c x_linux.c", "a.cpp b.cc c.cxx"},
		{"windows", "arm64", "a.c late.c tag.c x_arm64.c", "a.cpp b.cc c.cxx x_windows.cpp"},
	} {
		t.Run(tc.goos+"_"+tc.goarch, func(t *testing.T) {
			prog := loadIn(t, Config{GOOS: tc.goos, GOARCH: tc.goarch}, root, ".")
			wantErrors(t, prog, root, "")
			p := prog.Roots[0]
			if got := strings.Join(p.CFiles, " "); got != tc.c {
				t.Errorf("CFiles = %s, want %s", got, tc.c)
			}
			if got := strings.Join(p.CXXFiles, " "); got != tc.cxx {
				t.Errorf("CXXFiles = %s, want %s", got, tc.cxx)
			}
			// An imported package has its own.
			if lib := pkgOf(t, prog, "m/lib"); strings.Join(lib.CFiles, " ") != "lib.c" {
				t.Errorf("m/lib: CFiles = %q, want lib.c", lib.CFiles)
			}
		})
	}

	// Named .gp files: only those, like go run a.go.
	prog := loadIn(t, Config{}, root, "main.gp")
	wantErrors(t, prog, root, "")
	if p := prog.Roots[0]; p.CFiles != nil || p.CXXFiles != nil {
		t.Errorf("named files: CFiles %q, CXXFiles %q, want none", p.CFiles, p.CXXFiles)
	}
}

// TestSourceErrors checks the errors of C and C++ files: a broken build
// constraint (the file is left out, as for a .gp file), and the kinds of
// source that gocpp doesn't build, unless the build constraints exclude
// them.
func TestSourceErrors(t *testing.T) {
	root := writeTree(t, map[string]string{
		"gocpp.mod":     "module m\n",
		"main.gp":       "package main\n\nfunc main() {}\n",
		"two.c":         "//go:build linux\n//go:build amd64\n\nint x;\n",
		"ok.c":          "",
		"asm.S":         "",
		"asm_windows.s": "", // excluded: no error
		"objc.m":        "",
		"res.syso":      "",
		"f.f90":         "",
	})
	prog := loadIn(t, Config{}, root, ".")
	wantErrors(t, prog, root, `# m
./asm.S: assembly files (.S) are not supported: gocpp compiles only the .c, .cc, .cpp and .cxx files of a package
./f.f90: Fortran files (.f90) are not supported: gocpp compiles only the .c, .cc, .cpp and .cxx files of a package
./objc.m: Objective-C files (.m) are not supported: gocpp compiles only the .c, .cc, .cpp and .cxx files of a package
./res.syso: system object files (.syso) are not supported: gocpp compiles only the .c, .cc, .cpp and .cxx files of a package
./two.c: multiple //go:build comments
`)
	if got := strings.Join(prog.Roots[0].CFiles, " "); got != "ok.c" {
		t.Errorf("CFiles = %s, want ok.c", got)
	}
}

func TestLeadingComments(t *testing.T) {
	for _, c := range []struct{ in, want string }{
		{"", ""},
		{"int x;\n", "i"},
		{"// a\n//go:build linux\n\n#include <x.h>\n//go:build windows\n", "// a\n//go:build linux\n\n#"},
		{"/* a\n * b */ /**/ /*/ still */\n//go:build x\nint y;", "/* a\n * b */ /**/ /*/ still */\n//go:build x\ni"},
		{"\t\r\n  // only comments", "\t\r\n  // only comments"},
		{"/* not closed", "/* not closed"},
		{"/x", "/x"},
		// What go/build's readComments skips too: a byte order mark at the
		// start, \f and ;.
		{"\xef\xbb\xbf//go:build windows\n\nint x;", "\xef\xbb\xbf//go:build windows\n\ni"},
		{"\xef\xbb\xbfint x;", "\xef\xbb\xbfi"},
		{"x\xef\xbb\xbf", "x"},
		{"\f;\n//go:build windows\n;int x;", "\f;\n//go:build windows\n;i"},
	} {
		got, err := leadingComments(bufio.NewReader(strings.NewReader(c.in)))
		if err != nil || string(got) != c.want {
			t.Errorf("leadingComments(%q) = %q, %v; want %q", c.in, got, err, c.want)
		}
	}
}

// openless is the operating system's FileSystem without Open, like the
// language server's: C files are read whole.
type openless struct{ fs osFS }

func (o openless) ReadFile(name string) ([]byte, error)       { return o.fs.ReadFile(name) }
func (o openless) ReadDir(name string) ([]fs.DirEntry, error) { return o.fs.ReadDir(name) }
func (o openless) Stat(name string) (fs.FileInfo, error)      { return o.fs.Stat(name) }

// TestSourcesFileSystems checks that a FileSystem with Open (only the
// comments at the top of a C file are read) and one without it (the whole
// file is read) give the same C files, also for the bytes that go/build
// skips before a //go:build line.
func TestSourcesFileSystems(t *testing.T) {
	root := writeTree(t, map[string]string{
		"gocpp.mod":  "module m\n",
		"main.gp":    "package main\n\nfunc main() {}\n",
		"a.c":        "int a;\n//go:build windows\n",
		"bom.c":      "\xef\xbb\xbf//go:build windows\n\nint b;\n",
		"ff.c":       "\f//go:build windows\n\nint c;\n", // \f is a space
		"semi.c":     ";\n//go:build windows\nint g;\n",  // after ;, a line that is no comment: no constraint
		"block.cpp":  "/* a\n//go:build windows\n*/\nint d;\n",
		"linux.cc":   "// x\n\n//go:build linux\n\nint e;\n",
		"windows.cc": "\xef\xbb\xbf// x\n//go:build windows\nint f;\n",
	})
	if _, ok := FileSystem(osFS{}).(opener); !ok {
		t.Fatal("osFS has no Open")
	}
	if _, ok := FileSystem(openless{}).(opener); ok {
		t.Fatal("openless has Open")
	}
	var want string
	for _, fsys := range []FileSystem{osFS{}, openless{}} {
		prog := loadIn(t, Config{FS: fsys}, root, ".")
		wantErrors(t, prog, root, "")
		p := prog.Roots[0]
		got := strings.Join(p.CFiles, " ") + "; " + strings.Join(p.CXXFiles, " ")
		if want == "" {
			want = got
			if want != "a.c semi.c; block.cpp linux.cc" {
				t.Errorf("with osFS: %s, want a.c semi.c; block.cpp linux.cc", got)
			}
		} else if got != want {
			t.Errorf("without Open: %s, with Open: %s", got, want)
		}
	}
}
