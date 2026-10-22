package load

import (
	"bytes"
	"cmp"
	"container/heap"
	"errors"
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"github.com/huuhait/gocpp/internal/diag"
	"github.com/huuhait/gocpp/internal/load/gocppmod"
	"github.com/huuhait/gocpp/internal/syntax/parser"
	"github.com/huuhait/gocpp/internal/syntax/scanner"
)

// A Config says what Load loads, and for which target.
type Config struct {
	// Dir is the folder that relative patterns are relative to, like the
	// current folder of the go command. If it is empty, Load uses
	// os.Getwd.
	Dir string

	// Stdlib holds the standard packages: one folder for each import
	// path, with the .gp files of the package in it ("fmt/print.gp",
	// "unicode/utf8/utf8.gp"). If it is nil, there are none.
	Stdlib fs.FS

	// StdlibDir is the folder that the names of the files of standard
	// packages start with, in the FileSet and in messages:
	// StdlibDir/fmt/print.gp. If it is empty, it is "$GOCPP/stdlib".
	StdlibDir string

	// AllowStdInternal lets the packages that Load reads outside the standard library import the
	// standard library's internal packages (internal/bytealg, ...), which Go's rule for internal
	// packages forbids. It is for gocpp's own tests of those packages (cmd/gocpp sets it from
	// GOCPP_ALLOW_INTERNAL, which only the test harness sets): a program that uses it is not Go.
	AllowStdInternal bool

	// GoStd reports whether path is a package of Go's standard library.
	// For such a package that Stdlib doesn't have, the error is "package
	// net/http is not available yet (Phase 09)" instead of "package
	// net/htp is not in std". If it is nil, no path is.
	GoStd func(path string) bool

	// GOOS and GOARCH are the target of the build constraints. If they
	// are empty, they are the system that runs Load (runtime.GOOS and
	// runtime.GOARCH).
	GOOS, GOARCH string

	// Tags are more build tags that are satisfied, like go build -tags.
	Tags []string

	// Tests loads the packages that the patterns name for their tests,
	// like go test: with their _test.gp files (their dependencies don't
	// get theirs). The _test.gp files with the package clause "package
	// x_test" (external tests) become a package of their own, which comes
	// right after its package in Program.Roots. A package with test files
	// of its own package gets an import graph of its own, as a test
	// binary does in Go: in it, the package with its test files (the test
	// variant) takes the place of the package, also for the packages that
	// import it, directly or not, which are loaded again for the test
	// (their ForTest is set). The other packages are shared with the rest
	// of the program. A package without such files is the plain package.
	Tests bool

	// TestMain, with Tests, makes the main package of a test binary, like the
	// main package that go test generates: after the package under test p
	// (the test variant when it has internal tests) and its external test
	// package xtest (nil if there is none) are loaded, TestMain is called
	// (and the file set) and returns the source of a file "package main"
	// (nil: no main package) or the errors that stop the test (added to p). The file is the only one of a package that is the last
	// root, after p and xtest, with the import path p.ImportPath + ".test".
	// It can import p, the path of xtest (p.ImportPath + "_test") and
	// "testing/internal/testdeps" besides the packages that any file can
	// import, and it is loaded in the import graph of the test: p is the
	// test variant. TestMain is not called when p or xtest has load errors.
	TestMain func(fset *token.FileSet, p, xtest *Package) ([]byte, []diag.Diagnostic)

	// AllErrors parses with parser.AllErrors: all syntax errors, not only
	// the first 10 on different lines (gocpp check -e).
	AllErrors bool

	// SkipBodies, if not nil, says for the name of a file whether the parser leaves the bodies of
	// its functions out (parser.SkipFuncBodies; the body of init stays). The language server's
	// focused check uses it for the files that it does not look at the bodies of
	// (internal/lsp/cache, focus.go).
	SkipBodies func(path string) bool

	// NoEmbedFiles makes //go:embed and #embed (step 10.8) check the
	// directive and the pattern, but not read any file: the variable or the
	// macro gets the empty string, and a missing file is no error. The
	// language server sets it: a check there may read only what the
	// resolution of the imports read (internal/lsp/cache/fs.go), and the
	// content of an embedded file changes no type. gocpp check and build
	// read the files.
	NoEmbedFiles bool

	// Fset is the file set that gets the files. If it is nil, Load makes
	// a new one.
	Fset *token.FileSet

	// Overlay holds files by absolute name whose contents Load reads
	// instead of the files on disk: the unsaved buffers of an editor (the
	// language server, Phase 11 README §2.6). A .gp file of Overlay is in
	// its folder also when it is not on disk, and so is a gocpp.mod file.
	// The files of Config.Stdlib have no overlay.
	Overlay map[string][]byte

	// Deps, if not nil, gives packages that Load uses as they are instead
	// of loading them: for each package that a loaded package imports, Load
	// calls Deps once, with the import path, the folder and whether it is a
	// standard package (Package.Std), before it loads the package. If Deps
	// returns a package, which must be one that Package.Summary made, Load
	// reads none of its files: it is in the Imports of its importers, but
	// not in Program.Packages, and Load never changes it. If Deps returns
	// nil, Load loads the package itself. The language server gives the
	// summaries of the packages it has loaded and checked already, so a
	// load reads only the files of the package it is for (Phase 11
	// README §2.2). Load ignores Deps when Tests is set: in the import
	// graph of a test, a package that imports the package under test must
	// be loaded again (Package.ForTest).
	Deps func(importPath, dir string, std bool) *Package

	// FS, if not nil, is where Load reads the files and folders that are
	// not in Config.Stdlib or Config.Overlay, instead of the operating
	// system: every file, folder listing and stat that Load makes goes
	// through it. The language server gives the files of a snapshot, so
	// that a load sees what the snapshot sees, also what decides how the
	// imports resolve (which folders exist, which gocpp.mod files).
	FS FileSystem

	// ModuleDir, if not nil, gives the folder of a required module that is
	// neither replaced by a local folder nor vendored (see moddir.go). If it
	// is nil, Load uses the default, which downloads nothing yet.
	ModuleDir ModuleDirFunc
}

// A FileSystem is what Load reads the files on disk with (Config.FS). The
// names are absolute; the methods work like os.ReadFile, os.ReadDir and
// os.Stat (an error for a missing file is fs.ErrNotExist).
type FileSystem interface {
	ReadFile(name string) ([]byte, error)
	ReadDir(name string) ([]fs.DirEntry, error)
	Stat(name string) (fs.FileInfo, error)
}

// osFS is the FileSystem of the operating system.
type osFS struct{}

func (osFS) ReadDir(name string) ([]fs.DirEntry, error) { return os.ReadDir(name) }
func (osFS) Stat(name string) (fs.FileInfo, error)      { return os.Stat(name) }

// ReadFile reads a regular file (or a link to one); see ReadRegular.
func (osFS) ReadFile(name string) ([]byte, error) { return ReadRegular(name) }

// ReadRegular reads the file name like os.ReadFile, but a named pipe, a socket or a device with
// the name of a source is an error, not a read that waits for ever (see openRegular). The reads of
// the sources and of the module file by the loader, gocpp build's recorder (build.FSRecorder), the
// commands and the C header checks go through it; the language server's own reads do not.
func ReadRegular(name string) ([]byte, error) {
	f, size, err := openRegular(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	buf := bytes.NewBuffer(make([]byte, 0, size+bytes.MinRead))
	_, err = buf.ReadFrom(f)
	return buf.Bytes(), err
}

// A Program is what Load loaded: the packages that the patterns name and
// every package that they import, directly or not.
type Program struct {
	// Fset holds the positions of all files. Files on disk have absolute
	// names (so that diag.Printer can show them like the go command), and
	// the files of standard packages have names in Config.StdlibDir.
	Fset *token.FileSet

	// Roots are the packages of the patterns, in the order of the
	// patterns. An external test package comes right after its package.
	Roots []*Package

	// Packages are all the packages, each one after the packages it
	// imports. Among the packages that are ready (their imports are
	// done), the one with the smallest import path comes first: this is
	// the order in which Go initializes the packages. Packages of an
	// import cycle come at the end.
	Packages []*Package

	// Errors are the errors that belong to no package: a bad pattern, a
	// pattern for a package that can't be found, or a broken gocpp.mod
	// file.
	Errors *diag.List
}

// HasErrors reports whether the program has an error: in Errors or in a
// package.
func (p *Program) HasErrors() bool {
	if p.Errors.HasErrors() {
		return true
	}
	for _, pkg := range p.Packages {
		if pkg.Errors.HasErrors() {
			return true
		}
	}
	return false
}

// A Package is one loaded package: a folder of .gp files, or the .gp files
// named on the command line.
//
// Load runs the parse-level steps of the front-end on each file, in this
// order (Phase 01 README §2.1 and §2.5): parse (with comments, and with
// the parser's object resolution, which RewriteFile needs), the
// package-level names of all files (PackageLevelNames), then for each file
// CheckReservedC (not in standard packages) and ParseDirectives, the
// loading of the imported gocpp packages (PackagePaths), ClassifyImports
// with their real names and the exported names of the dot-imported
// packages, and RewriteFile with those names too. So after Load, the ASTs
// are ready for go/types with FakeImportC (step 1.10).
type Package struct {
	// ImportPath is the import path: "fmt", "example.com/app/geom". The
	// .gp files named on the command line, and a folder outside a module,
	// have the import path "command-line-arguments", like in the go
	// command. An external test package has the import path of its
	// package and "_test": "example.com/app/geom_test".
	ImportPath string

	// Name is the package name from the package clause: geom. It is ""
	// if no file has a package clause.
	Name string

	// Dir is the absolute folder of the package. For a standard package,
	// it is in Config.StdlibDir.
	Dir string

	// Std reports whether the package comes from Config.Stdlib.
	Std bool

	// Module is the module that the package is in, or nil for a standard
	// package and for a package outside every module.
	Module *Module

	// Files are the .gp files of the package, by name for a folder, in the
	// order they were named for files on the command line. Every file has
	// all its fields set, also when the package has errors.
	Files []*File

	// Imports maps the import path of each gocpp package that the files
	// import (FileInfo.Packages) to the package. A package that has errors
	// is there too, but an import that couldn't be resolved is not (its
	// error is in Errors), and neither is "unsafe", which go/types knows
	// itself (types.Unsafe). Imports has a cycle only after an "import
	// cycle not allowed" error.
	Imports map[string]*Package

	// CFiles are the C files (.c) of the package, and CXXFiles its C++
	// files (.cc, .cpp, .cxx): their names in Dir, by name, that the build
	// constraints keep, like go/build's CFiles and CXXFiles (see
	// sources.go). gocpp build compiles them and links them with the
	// package. A package of .gp files named on the command line has none,
	// like go run a.go, which builds only the files it names.
	CFiles, CXXFiles []string

	// Errors are the errors and warnings of the package, in the order
	// they were found. It is never nil.
	Errors *diag.List

	// Incomplete reports whether the package or a package that it
	// imports, directly or not, has an error.
	Incomplete bool

	// NoFiles says why the folder of the package has no .gp file to
	// build, when nothing else is wrong with it: no .gp files at all
	// (NoGpFiles), only _test.gp files without Config.Tests
	// (OnlyTestFiles), or only files that the build constraints exclude
	// (ExcludedFiles). Errors then holds just that error ("no .gp files
	// in", "no non-test .gp files in", "build constraints exclude all .gp
	// files in"). It is HasFiles, the zero value, otherwise. Like the go
	// command, gocpp check skips such a folder when a pattern with "..."
	// matches it (go's NoGoError, and its omitTestOnly for tests).
	NoFiles NoFilesReason

	// ForTest is set for the packages of the import graph of a test
	// (Config.Tests) that are not shared with the rest of the program: the
	// test variant of a package, its external test package, and the
	// packages that import the test variant. It is the import path of the
	// package under test, like ForTest of go list, so that a package can
	// be shown as go test shows it: "example.com/app/geom
	// [example.com/app/geom.test]". Packages can have the same ImportPath
	// when one of them has ForTest set.
	ForTest string

	// at is the place of the import that loaded the package first, or the
	// zero Position for a root. Errors about the package as a whole ("no
	// .gp files in") go there, like the go command does.
	at token.Position

	// xtestOf is the package of an external test package, and xtest the
	// external test package of a package. Else they are nil.
	xtestOf, xtest *Package

	// testMainOf is the package under test of the generated main package
	// of a test binary (Config.TestMain), and xtestMain the external test
	// package it imports, if any.
	testMainOf, xtestMain *Package

	// testMain says that the package is the generated main package of a test binary, and
	// mainUnderTest that it is a package main that a test binary imports (go test builds it
	// as a library, its main function is a function like any other): see IsProgram.
	testMain, mainUnderTest bool

	// exported holds the exported package-level names, for dot imports.
	// It is nil until dotNames needs it.
	exported map[string]bool

	// nameFile is the file that gave the package its name (read).
	nameFile string

	// cycleReported is set once the package has an import cycle error:
	// it gets only one, like in the go command.
	cycleReported bool

	// summary is set for a package that Summary made: it has no files,
	// and the loader never changes it (Config.Deps).
	summary bool

	// Macros are the macros that the package declares, by name (Phase 10, step 10.9): what a
	// package that uses one needs to rewrite the use. A summary keeps them.
	Macros map[string]*MacroSig

	// Derives are the lines #Derive(...) on top of the types of the package (Phase 10, step 10.10):
	// the loader added the stubs of the methods, and internal/check writes their bodies once the
	// types are known.
	Derives []*Derive

	// MacroUses holds the names of the functions that the rewrite of the macro uses
	// call (the callee of each #Name(...), also on top of a func), with what each use calls:
	// internal/check tells a use of a macro's function that is not one of them (a plain call, a
	// value) from the others, and finds a callee that a variable of the same name hides.
	MacroUses map[*ast.Ident]*MacroUse

	// IncludeDirs are the folders that the C++ compiler needs for the module
	// headers that the package imports (Include.File): the root folder of
	// each module with such a header, in the order of the imports, each
	// once. The build passes them as -iquote to the compiles and to the probes
	// (build.ReadPackageFlags adds them to the compile flags), so that a
	// header finds the headers of its library that it includes by their
	// names in the module ("imconfig.h", "backends/x.h").
	IncludeDirs []string

	// GOOS is the target of the build constraints that loaded the package (Config.GOOS, or the
	// system that runs Load): build.ReadPackageFlags gives a package for windows the Win32 defaults
	// (step 14.4). It is empty in a summary.
	GOOS string

	// CxxModules are the modules whose headers the package imports, with the
	// cxx recipe that says how to build the library, in the order of the
	// imports, each once (Phase 06 step 6.1d). build.ReadPackageFlags reads the
	// recipes: their include folders and defines are flags of the package's
	// compiles and probes, and the build compiles their sources.
	CxxModules []*CxxModule

	// main is the main module of the program, for a package that is not
	// in it (a package of a required module): its require and replace
	// lines say where every import goes. It is nil for a root, whose
	// module is the main module.
	main *Module
}

// mainModule returns the main module of the program that p is in: the
// module of the root packages. Only its gocpp.mod file has the require and
// replace lines that count (as in Go).
func (p *Package) mainModule() *Module {
	if p.main != nil {
		return p.main
	}
	return p.Module
}

// A MacroUse is the callee of the rewrite of a macro use (Package.MacroUses).
type MacroUse struct {
	Sig  *MacroSig
	Path string        // the import path of the package that declares the macro
	Call *ast.CallExpr // the call that the rewrite made
}

// Summary returns what a later Load needs of p when Config.Deps gives it,
// as a new package: the fields of p but no Files, no Imports (an empty map)
// and no test links, with the exported package-level names of p kept for
// dot imports. It is never changed, so the summary can be given to any
// number of loads, also at the same time. Call it before the files of p
// are changed (check.Check rewrites the ASTs; their package-level names
// stay the same).
func (p *Package) Summary() *Package {
	return &Package{
		ImportPath: p.ImportPath,
		Name:       p.Name,
		Dir:        p.Dir,
		Std:        p.Std,
		Module:     p.Module,
		Imports:    map[string]*Package{},
		Errors:     p.Errors,
		Incomplete: p.Incomplete,
		NoFiles:    p.NoFiles,
		ForTest:    p.ForTest,
		exported:   maps.Clone(p.exportedNames()),
		Macros:     summaryMacros(p.Macros),
		summary:    true,
	}
}

// summaryMacros returns the signatures of the macros without the declarations, which a summary
// must not keep alive.
func summaryMacros(m map[string]*MacroSig) map[string]*MacroSig {
	if len(m) == 0 {
		return nil
	}
	out := make(map[string]*MacroSig, len(m))
	for k, v := range m {
		c := *v
		c.decl = nil
		out[k] = &c
	}
	return out
}

// ExportedNames returns the exported package-level names of p, sorted: what
// a summary (Summary) keeps for dot imports.
func (p *Package) ExportedNames() []string {
	return slices.Sorted(maps.Keys(p.exportedNames()))
}

// NewSummary returns a summary (see Summary) made from its parts: the
// fields of p that a summary keeps (ImportPath, Name, Dir, Std, Module,
// Errors, Incomplete, NoFiles, ForTest; the others are ignored) and the
// exported package-level names (ExportedNames of the package). The
// language server keeps summaries on disk and makes them again with it.
func NewSummary(p *Package, exported []string) *Package {
	s := &Package{
		ImportPath: p.ImportPath,
		Name:       p.Name,
		Dir:        p.Dir,
		Std:        p.Std,
		Module:     p.Module,
		Imports:    map[string]*Package{},
		Errors:     p.Errors,
		Incomplete: p.Incomplete,
		NoFiles:    p.NoFiles,
		ForTest:    p.ForTest,
		exported:   make(map[string]bool, len(exported)),
		Macros:     summaryMacros(p.Macros),
		summary:    true,
	}
	if s.Errors == nil {
		s.Errors = new(diag.List)
	}
	for _, name := range exported {
		s.exported[name] = true
	}
	return s
}

// A NoFilesReason says why a package has no .gp file to build
// (Package.NoFiles).
type NoFilesReason int

const (
	// HasFiles: the package has files to build, or another problem.
	HasFiles NoFilesReason = iota
	// NoGpFiles: the folder has no .gp files.
	NoGpFiles
	// OnlyTestFiles: the folder has only _test.gp files.
	OnlyTestFiles
	// ExcludedFiles: the build constraints exclude all the .gp files.
	ExcludedFiles
)

// A File is one .gp file of a package, after the parse-level steps.
type File struct {
	// Path is the absolute name of the file, which is its name in the
	// FileSet too.
	Path string

	// AST is the parsed file, after RewriteFile: header and namespace
	// uses are rewritten, and the imports are the gocpp packages (and
	// "C" if the file uses C).
	AST *ast.File

	// Info is what ClassifyImports found out about the imports.
	Info *FileInfo

	// Directives are the //gocpp: directives of the file.
	Directives []Directive

	// Externs are the names of the valid extern functions of the file (//gocpp:extern, extern.go),
	// in source order: the checker asks the imports of a package for them.
	Externs []string

	// Rewrite is what RewriteFile changed.
	Rewrite *Rewrite

	// Test reports whether the file is a _test.gp file.
	Test bool

	// Synthetic holds the names that the rewrites of check wrote into AST (the
	// temporaries and error variables of the `!` forms of Phase 10, the
	// names of the code that stands for them), or nil. They have positions
	// of the user's text (the check needs them), so tools that look up a
	// name at a place skip them: hover, go to definition, references,
	// rename and the hints take the user's own names only.
	Synthetic map[*ast.Ident]bool
	// BodiesSkipped says that the parser left the function bodies of the file out of the tree
	// (Config.SkipBodies): the type checker sees the declarations only.
	BodiesSkipped bool

	// Src is the text of the file, for the macro #text(x) (the text of
	// an argument, step 10.8). It is set only for a file of a user package
	// (not the standard library): it is nil otherwise. The caller must not
	// change it.
	Src []byte
}

// A Module is a tree of packages with a gocpp.mod file at its top. Module.File
// has the parsed file; the build reads the module-wide settings from it
// (docs/spec/modules.md).
type Module struct {
	// Path is the module path from the module line: example.com/app. It
	// is "" if gocpp.mod has no valid module line (Program.Errors says
	// why).
	Path string

	// Dir is the absolute folder that holds gocpp.mod. For a vendored
	// module (Vendored), it is the folder in vendor/ that holds its
	// packages: a vendored module has no gocpp.mod file there.
	Dir string

	// Version is the version of a module that the main module requires,
	// such as v1.2.3. It is "" for the main module, and for a module
	// that no loaded package imports yet.
	Version string

	// Vendored is true for a module that comes from the vendor folder of
	// the main module.
	Vendored bool

	// cached is true for a module whose folder moduleDir gave (Config.ModuleDir,
	// moddir.go): like a vendored module, it needs no gocpp.mod file, and
	// the gocpp.mod files above its folder are not its own.
	cached bool

	// file is the parsed gocpp.mod, or nil: for a vendored module, when
	// the file can't be read, and for a Module that Config.Deps or a
	// disk cache made again. It is not part of the exported fields, so
	// that a Module can be encoded (gob) as before.
	file *gocppmod.File
}

// File returns the parsed gocpp.mod file of the module, or nil if the module
// has none (a vendored module) or it could not be read. The caller must not
// change it.
func (m *Module) File() *gocppmod.File {
	if m == nil {
		return nil
	}
	return m.file
}

// commandLine is the import path of the .gp files named on the command
// line, as in the go command.
const commandLine = "command-line-arguments"

// Load loads the packages that the patterns name and every package that
// they import. A pattern is one of:
//
//   - a folder: ".", "./geom", "../app", "/abs/app". The package is the
//     .gp files in it, without the files whose name starts with . or _,
//     the _test.gp files (unless Config.Tests) and the files that the
//     build constraints leave out, like the go command;
//   - a list of .gp files (then every pattern must be one), which all
//     have to be in one folder: they are one package, whatever their
//     build constraints say, like go run a.go b.go;
//   - an import path, such as "fmt" or "example.com/app/geom".
//
// No pattern means ".". Patterns with "..." are not supported yet.
//
// The import paths of a package are resolved like this: a path that has a
// folder in Config.Stdlib is a standard package. Otherwise a path in the
// module of the package (found by walking up from its folder to a
// gocpp.mod file) is the folder of that name in the module. A path in a
// module that the main module requires is in the folder of its replace line
// or in vendor/ (see modules.go; the longest module path wins). A package
// without a module can import only standard packages, and so can a
// standard package. Every package is loaded once, also when many packages
// import it. "C", headers and namespaces are not packages: ClassifyImports
// sorts them out.
//
// Load doesn't stop at errors: a package that fails to load gets its
// errors and the others go on. The errors read like the go command's
// ("package foo is not in std", "import cycle not allowed", "found
// packages a (a.gp) and b (b.gp) in /dir").
func Load(cfg Config, patterns ...string) *Program {
	l := newLoader(cfg)
	if len(patterns) == 0 {
		patterns = []string{"."}
	}
	if strings.HasSuffix(patterns[0], ".gp") {
		l.loadFiles(patterns)
	} else {
		for _, pat := range patterns {
			l.loadPattern(pat)
		}
	}
	l.prog.Packages = initOrder(l.all)
	setIncomplete(l.all)
	l.warnStrayRecipes()
	return l.prog
}

// A loader holds the state of one Load call.
type loader struct {
	cfg      Config
	fset     *token.FileSet
	prog     *Program
	match    *matcher
	byPath   map[string]*Package       // the importable packages, by import path
	modules  map[string]*Module        // the module of each folder looked at (nil: none)
	vendors  map[string]*vendorState   // the vendor folder of each main module looked at (nil: none)
	required map[string]resolvedModule // where each required module is (modules.go)
	cxxMods  map[string]*CxxModule     // the C and C++ build recipe of each module looked at (cxxrecipe.go)

	// patternMain is the main module while a root package that an import
	// path names is made (loadImportPattern), else nil.
	patternMain *Module
	stdDirs     map[string]bool // whether each path has a folder in Stdlib with .gp files
	rootDirs    map[string]bool // the folders of the root packages
	stack       []stackEntry    // the packages being loaded, from the root
	graph       *testGraph      // the import graph of the test being loaded, or nil
	state       map[*Package]loadState
	all         []*Package // the packages, in the order they started loading

	// overlayDirs holds the base names of the files of Config.Overlay, by
	// folder.
	overlayDirs map[string][]string

	// parseFile is per loader so worker tests need no process-wide parser hook.
	parseFile parseFileFunc
}

// A stackEntry is a package on the import stack, and the name of the file
// in the package below it that imports it (the "from" of the import cycle
// message).
type stackEntry struct {
	pkg  *Package
	from string
}

type loadState int

const (
	notLoaded loadState = iota
	loading
	loaded
)

func newLoader(cfg Config) *loader {
	if cfg.Dir == "" {
		cfg.Dir, _ = os.Getwd()
	}
	if abs, err := filepath.Abs(cfg.Dir); err == nil {
		cfg.Dir = abs
	}
	if cfg.StdlibDir == "" {
		cfg.StdlibDir = "$GOCPP/stdlib"
	}
	if cfg.GOOS == "" {
		cfg.GOOS = runtime.GOOS
	}
	if cfg.GOARCH == "" {
		cfg.GOARCH = runtime.GOARCH
	}
	if cfg.Fset == nil {
		cfg.Fset = token.NewFileSet()
	}
	if cfg.FS == nil {
		cfg.FS = osFS{}
	}
	l := &loader{
		cfg:      cfg,
		fset:     cfg.Fset,
		prog:     &Program{Fset: cfg.Fset, Errors: new(diag.List)},
		match:    newMatcher(cfg.GOOS, cfg.GOARCH, cfg.Tags),
		byPath:   make(map[string]*Package),
		modules:  make(map[string]*Module),
		vendors:  make(map[string]*vendorState),
		required: make(map[string]resolvedModule),
		cxxMods:  make(map[string]*CxxModule),
		stdDirs:  make(map[string]bool),
		rootDirs: make(map[string]bool),
		state:    make(map[*Package]loadState),
	}
	l.parseFile = parser.ParseFile
	if skip := cfg.SkipBodies; skip != nil {
		l.parseFile = func(
			fset *token.FileSet, name string, src any, mode parser.Mode,
		) (*ast.File, error) {
			if skip(name) {
				mode |= parser.SkipFuncBodies
			}
			return parser.ParseFile(fset, name, src, mode)
		}
	}
	l.overlayDirs = make(map[string][]string)
	for name := range cfg.Overlay {
		dir := filepath.Dir(name)
		l.overlayDirs[dir] = append(l.overlayDirs[dir], filepath.Base(name))
	}
	return l
}

// newPackage returns a new package that is not loaded yet.
func (l *loader) newPackage(importPath, dir string, std bool, mod *Module) *Package {
	var main *Module
	if l.patternMain != nil && l.patternMain != mod {
		main = l.patternMain
	}
	return &Package{
		main:       main,
		GOOS:       l.cfg.GOOS,
		ImportPath: importPath,
		Dir:        dir,
		Std:        std,
		Module:     mod,
		Imports:    make(map[string]*Package),
		Errors:     new(diag.List),
	}
}

// loadFiles loads the .gp files named on the command line as one package.
func (l *loader) loadFiles(names []string) {
	var paths, dirs []string
	for _, name := range names {
		if !strings.HasSuffix(name, ".gp") {
			l.prog.Errors.Errorf(token.Position{}, "named files must be .gp files: %s", name)
			return
		}
		abs := l.abs(name)
		if fi, err := l.cfg.FS.Stat(abs); err == nil && fi.IsDir() {
			l.prog.Errors.Errorf(token.Position{}, "%s is a directory, should be a .gp file", name)
			return
		}
		if len(paths) > 0 && filepath.Dir(abs) != filepath.Dir(paths[0]) {
			l.prog.Errors.Errorf(token.Position{}, "named files must all be in one directory; have %s and %s",
				dirs[0], filepath.Dir(name))
			return
		}
		paths = append(paths, abs)
		dirs = append(dirs, filepath.Dir(name))
	}
	dir := filepath.Dir(paths[0])
	p := l.newPackage(commandLine, dir, false, l.module(dir))
	if l.cfg.Tests {
		l.loadTest(p, paths)
		return
	}
	l.loadRoot(p, paths)
}

// loadPattern loads the package of one pattern that is not a file.
func (l *loader) loadPattern(pat string) {
	if strings.Contains(pat, "...") {
		l.prog.Errors.Errorf(token.Position{}, "pattern %s: patterns with ... are not supported yet", pat)
		return
	}
	if !isLocalPattern(pat) {
		l.loadImportPattern(pat)
		return
	}
	dir := l.abs(pat)
	fi, err := l.cfg.FS.Stat(dir)
	switch {
	case err != nil && len(l.overlayDirs[dir]) > 0:
		// A folder that is only in the overlay.
	case err != nil:
		l.prog.Errors.Errorf(token.Position{}, "stat %s: directory not found", dir)
		return
	case !fi.IsDir():
		l.prog.Errors.Errorf(token.Position{}, "%s is not a directory", dir)
		return
	}
	mod := l.module(dir)
	if vm, path := l.vendoredFolder(mod, dir); vm != nil {
		l.patternMain = mod
		l.loadRootAt(path, dir, false, vm)
		l.patternMain = nil
		return
	}
	importPath := commandLine
	if mod != nil && mod.Path != "" {
		// dir is in mod.Dir: module walked up from it.
		if rel, err := filepath.Rel(mod.Dir, dir); err == nil {
			importPath = joinImportPath(mod.Path, filepath.ToSlash(rel))
		}
	}
	l.loadRootAt(importPath, dir, false, mod)
}

// loadImportPattern loads the package of a pattern that is an import path,
// found as if the package in Config.Dir imported it.
func (l *loader) loadImportPattern(pat string) {
	if pat == "unsafe" {
		l.prog.Errors.Errorf(token.Position{}, "package unsafe has no .gp files: go/types knows it")
		return
	}
	from := &Package{Dir: l.cfg.Dir, Module: l.module(l.cfg.Dir)}
	loc, msg := l.find(from, pat)
	if msg != "" {
		l.prog.Errors.Errorf(token.Position{}, "%s", msg)
		return
	}
	// The package may be in a required module, but the main module is the
	// one of Config.Dir: its gocpp.mod says where the imports go.
	l.patternMain = from.Module
	l.loadRootAt(loc.importPath, loc.dir, loc.std, loc.mod)
	l.patternMain = nil
}

// loadRootAt loads the package in the folder dir as a root. Without Tests,
// the package is the same as the one that an import of importPath gets.
func (l *loader) loadRootAt(importPath, dir string, std bool, mod *Module) {
	if l.rootDirs[dir] {
		return // named by an earlier pattern
	}
	l.rootDirs[dir] = true
	if l.cfg.Tests {
		l.loadTest(l.newPackage(importPath, dir, std, mod), nil)
		return
	}
	importable := importPath != commandLine
	if importable {
		if p := l.byPath[importPath]; p != nil {
			if !slices.Contains(l.prog.Roots, p) {
				l.prog.Roots = append(l.prog.Roots, p)
			}
			return
		}
	}
	p := l.newPackage(importPath, dir, std, mod)
	if importable {
		l.byPath[importPath] = p
	}
	l.loadRoot(p, nil)
}

// loadRoot loads the root package p. files are the files named on the
// command line, or nil for the files of the folder.
func (l *loader) loadRoot(p *Package, files []string) {
	l.prog.Roots = append(l.prog.Roots, p)
	l.stack = []stackEntry{{pkg: p}}
	l.loadPackage(p, files, false)
	l.stack = nil
}

// A testGraph is the import graph of one test (Config.Tests), as Go builds
// one for each test binary. In it, the test variant of the package under
// test (root) takes the place of the package. A package that imports the
// test variant, directly or not, is loaded again for the graph; the other
// packages are shared with the rest of the program.
type testGraph struct {
	root *Package // the test variant

	// byPath holds the packages that were loaded for the graph, by
	// import path: the test variant, the packages that import it, and
	// the packages that don't, which are shared too once they are loaded.
	byPath map[string]*Package

	depends map[*Package]bool // whether a package loaded for the graph imports root
	reaches map[*Package]bool // whether a shared package imports a package with root's path
}

// loadTest loads the root package p for its test, with its _test.gp files,
// and its external test package if it has one. files are the files named
// on the command line, or nil for the files of the folder.
//
// When p has test files of its own package (internal tests), it is the
// test variant, in an import graph of its own (see testGraph). Otherwise
// it is the plain package, shared with the rest of the program, as in go
// test; then only the external test package is for the test.
func (l *loader) loadTest(p *Package, files []string) {
	if plain := l.byPath[p.ImportPath]; plain != nil && files == nil && len(plain.Files) > 0 {
		// Loaded already, as the import of an earlier root. Without
		// internal tests, it is the root as it is, and only the external
		// test files are read. A plain package without files is read
		// again, as when the root comes first: if the folder has test
		// files, the root is not the plain package (whose error is "no
		// non-test .gp files").
		if xt, ok := l.readXTest(plain); ok {
			l.startTest(plain, xt)
			return
		}
	}
	internalTests := l.read(p, files, true)
	xt := p.xtest
	if internalTests {
		p.ForTest = p.ImportPath
		l.graph = &testGraph{
			root:    p,
			byPath:  map[string]*Package{p.ImportPath: p},
			depends: make(map[*Package]bool),
			reaches: make(map[*Package]bool),
		}
	} else if p.ImportPath != commandLine && (len(p.Files) > 0 || xt == nil) {
		// The plain package, for the rest of the program too. With
		// external test files but no files of its own, it is not: an
		// import of it loads it again and gets "no non-test .gp files".
		if plain := l.byPath[p.ImportPath]; plain != nil {
			if len(p.Files) == 0 {
				// No files and no test files, as plain: the same package.
				l.startTest(plain, nil)
				return
			}
			// readXTest and read disagree about the internal tests. Never
			// make a second plain package for the path: keep the first one.
			l.prog.Errors.Errorf(token.Position{}, "internal error: package %s loaded twice for its test", p.ImportPath)
			l.startTest(plain, xt)
			return
		}
		l.byPath[p.ImportPath] = p
	}
	l.startTest(p, xt)
}

// startTest loads the root p of a test, unless it is loaded already, and
// its external test package xt, if it is not nil.
func (l *loader) startTest(p, xt *Package) {
	if !slices.Contains(l.prog.Roots, p) {
		l.prog.Roots = append(l.prog.Roots, p)
	}
	if l.state[p] == notLoaded {
		l.stack = []stackEntry{{pkg: p}}
		l.state[p] = loading
		l.all = append(l.all, p)
		l.process(p)
		l.state[p] = loaded
	}
	if xt != nil {
		xt.xtestOf, p.xtest = p, xt
		xt.ForTest = p.ImportPath
		l.prog.Roots = append(l.prog.Roots, xt)
		l.stack = []stackEntry{{pkg: xt}}
		l.state[xt] = loading
		l.all = append(l.all, xt)
		l.process(xt)
		l.state[xt] = loaded
	}
	l.loadTestMain(p, xt)
	l.stack, l.graph = nil, nil
}

// IsProgram reports whether the package is the main package of the program, the one with the entry
// point: a package main, except one that a test binary imports (the generated main package of the
// test binary is the program then, and it is not named main: a program has one package named main).
func (p *Package) IsProgram() bool {
	return p.testMain || p.Name == "main" && !p.mainUnderTest
}

// testMainFile is the name of the file of the generated main package of a test binary.
const testMainFile = "_testmain.gp"

// loadTestMain loads the main package that Config.TestMain makes for the test of p and its
// external test package xt (which may be nil), in the import graph of the test.
func (l *loader) loadTestMain(p, xt *Package) {
	if l.cfg.TestMain == nil || p.ImportPath == commandLine || p.Errors.HasErrors() || xt != nil && xt.Errors.HasErrors() {
		return
	}
	src, errs := l.cfg.TestMain(l.fset, p, xt)
	for _, d := range errs {
		p.Errors.Add(d)
	}
	if src == nil || len(errs) > 0 {
		return
	}
	m := l.newPackage(p.ImportPath+".test", p.Dir, p.Std, p.Module)
	m.testMainOf, m.xtestMain = p, xt
	m.testMain = true
	if p.Name == "main" {
		p.mainUnderTest = true
	}
	m.ForTest = p.ImportPath
	path := filepath.Join(p.Dir, testMainFile)
	if l.cfg.Overlay == nil {
		l.cfg.Overlay = make(map[string][]byte)
	}
	l.cfg.Overlay[path] = src
	l.read(m, []string{path}, false)
	l.prog.Roots = append(l.prog.Roots, m)
	l.stack = []stackEntry{{pkg: m}}
	l.state[m] = loading
	l.all = append(l.all, m)
	l.process(m)
	l.state[m] = loaded
}

// readXTest reads the external test files (package c_test) of the plain
// package p, which is loaded already, for a test of p, and returns their
// package, or nil if there are none. It reports false, and reads nothing,
// if p has test files of its own package (internal tests, or a test file
// whose package clause it can't read): then p needs a test variant. Like
// read, it reports a package clause that doesn't fit p ("found packages")
// in the errors of the external test package.
func (l *loader) readXTest(p *Package) (*Package, bool) {
	entries, err := l.readDir(p)
	if err != nil {
		return nil, false
	}
	type source struct {
		name, path string
		src        []byte
	}
	var (
		srcs  []source
		xerrs []diag.Diagnostic
	)
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !isTestFile(name) || strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_") {
			continue
		}
		path := filepath.Join(p.Dir, name)
		src, err := l.readFile(p, name)
		if err != nil {
			return nil, false
		}
		// The same decisions as in read: the build constraints first, so
		// an excluded file counts for nothing; a broken constraint is the
		// external test package's when the clause ends in _test.
		external := strings.HasSuffix(clauseName(src), "_test")
		ok, msg := l.match.file(name, src)
		switch {
		case msg != "" && external:
			xerrs = append(xerrs, diag.Diagnostic{Pos: token.Position{Filename: path}, Msg: msg})
		case msg != "":
			return nil, false // a broken internal test file: read keeps its error
		case !ok:
			// Left out by the build constraints.
		case !external || clauseName(src) == p.Name:
			return nil, false // an internal test
		default:
			srcs = append(srcs, source{name, path, src})
		}
	}
	if len(srcs) == 0 && len(xerrs) == 0 {
		return nil, true
	}
	mode := l.parseMode()
	var (
		files  []*File
		xnames []xtestName
	)
	paths := make([]string, len(srcs))
	texts := make([][]byte, len(srcs))
	for i, s := range srcs {
		paths[i], texts[i] = s.path, s.src
	}
	parsed, parseErrs := l.parseFiles(paths, texts, mode)
	for i, s := range srcs {
		f, err := parsed[i], parseErrs[i]
		var errs diag.List
		if err != nil {
			addParseErrors(&errs, err)
		}
		xerrs = append(xerrs, errs.Diagnostics()...)
		files = append(files, &File{Path: s.path, AST: f, Test: true, Src: l.keepSrc(p, s.src)})
		xnames = append(xnames, xtestName{s.name, strings.TrimSuffix(f.Name.Name, "_test")})
	}
	// The same check as in read, against the name of p from its own files.
	if d, ok := xtestConflict(p.Name, p.nameFile, p.Dir, token.Position{}, xnames); ok {
		xerrs = append(xerrs, d)
	}
	pkgName := p.Name
	if pkgName == "" && len(xnames) > 0 {
		pkgName = xnames[0].name
	}
	xt := l.newPackage(p.ImportPath+"_test", p.Dir, p.Std, p.Module)
	xt.Name = pkgName + "_test"
	xt.Files = files
	for _, d := range xerrs {
		xt.Errors.Add(d)
	}
	return xt, true
}

// dependsOnRoot reports whether the package p, loaded for the graph,
// imports the test variant, directly or not. A package that imports a
// package that is still loading is in an import cycle, which is an error;
// it counts as importing the test variant.
func (g *testGraph) dependsOnRoot(l *loader, p *Package) bool {
	if p == g.root {
		return true
	}
	if v, ok := g.depends[p]; ok {
		return v
	}
	g.depends[p] = true // for a cycle
	v := false
	for _, dep := range p.Imports {
		if g.byPath[dep.ImportPath] != dep {
			continue // a shared package: it doesn't
		}
		if l.state[dep] == loading || g.dependsOnRoot(l, dep) {
			v = true
			break
		}
	}
	g.depends[p] = v
	return v
}

// reachesRoot reports whether the shared package p imports a package with
// the import path of the package under test, directly or not: then the
// graph needs its own copy of p, which imports the test variant.
func (g *testGraph) reachesRoot(p *Package) bool {
	if p.ImportPath == g.root.ImportPath {
		return true
	}
	if v, ok := g.reaches[p]; ok {
		return v
	}
	g.reaches[p] = false // for a cycle
	v := false
	for _, dep := range p.Imports {
		if g.reachesRoot(dep) {
			v = true
			break
		}
	}
	g.reaches[p] = v
	return v
}

// loadPackage reads, parses and processes the package p, and loads the
// packages that it imports first.
func (l *loader) loadPackage(p *Package, files []string, tests bool) {
	l.state[p] = loading
	l.all = append(l.all, p)
	l.read(p, files, tests)
	l.process(p)
	l.state[p] = loaded
}

// read reads and parses the files of p: the files named on the command
// line, or else the .gp files in the folder of p that the build
// constraints keep. With tests, it reads the _test.gp files too, and moves
// the files of an external test package, and their errors, to p.xtest. It
// reports whether p has test files of its own package (internal tests),
// counting a test file whose package clause it can't read.
func (l *loader) read(p *Package, named []string, tests bool) (internalTests bool) {
	type source struct {
		name string // the base name
		path string // the name in the FileSet
		src  []byte
	}
	var (
		srcs                        []source
		skippedTests, excluded, bad int
		xerrs                       []diag.Diagnostic // the errors of external test files
	)
	// fileError adds the error d of the file name (with the contents src,
	// or nil if it can't be read), which is not in the package: to the
	// errors of the external test package if it is one of its files.
	fileError := func(name string, src []byte, d diag.Diagnostic) {
		bad++
		if isTestFile(name) {
			if src != nil && strings.HasSuffix(clauseName(src), "_test") {
				xerrs = append(xerrs, d)
				return
			}
			internalTests = true
		}
		p.Errors.Add(d)
	}
	if named != nil {
		for _, path := range named {
			name := filepath.Base(path)
			if isTestFile(name) && !tests {
				skippedTests++
				continue
			}
			src, err := l.readAbs(path)
			if err != nil {
				fileError(name, nil, diag.Diagnostic{Msg: err.Error()})
				continue
			}
			srcs = append(srcs, source{name, path, src})
		}
	} else {
		entries, err := l.readDir(p)
		if err != nil {
			p.Errors.Errorf(p.at, "%s", err)
			return false
		}
		for _, e := range entries {
			name := e.Name()
			if e.IsDir() || strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_") {
				continue
			}
			if !strings.HasSuffix(name, ".gp") {
				l.addSource(p, name) // a C or C++ file
				continue
			}
			if isTestFile(name) && !tests {
				skippedTests++
				continue
			}
			path := filepath.Join(p.Dir, name)
			src, err := l.readFile(p, name)
			if err != nil {
				fileError(name, nil, diag.Diagnostic{Pos: token.Position{Filename: path}, Msg: err.Error()})
				continue
			}
			ok, msg := l.match.file(name, src)
			if msg != "" {
				fileError(name, src, diag.Diagnostic{Pos: token.Position{Filename: path}, Msg: msg})
				continue
			}
			if !ok {
				excluded++
				continue
			}
			srcs = append(srcs, source{name, path, src})
		}
	}
	if len(srcs) == 0 && bad == 0 {
		dir := p.Dir
		switch {
		case skippedTests > 0:
			p.NoFiles = OnlyTestFiles
			p.Errors.Errorf(p.at, "%s: no non-test .gp files in %s", p.ImportPath, dir)
		case excluded > 0:
			p.NoFiles = ExcludedFiles
			p.Errors.Errorf(p.at, "package %s: build constraints exclude all .gp files in %s", p.ImportPath, dir)
		default:
			p.NoFiles = NoGpFiles
			p.Errors.Errorf(p.at, "no .gp files in %s", dir)
		}
		return false
	}

	mode := l.parseMode()
	var (
		reported bool // the "found packages" error of p is reported
		xfiles   []*File
		xnames   []xtestName
	)
	// Read/filter through the configured file system first; only parsing the
	// captured bytes is parallel. Commit results in source order below.
	paths := make([]string, len(srcs))
	texts := make([][]byte, len(srcs))
	for i, s := range srcs {
		paths[i] = s.path
		texts[i] = s.src
	}
	parsed, parseErrs := l.parseFiles(paths, texts, mode)
	for i, s := range srcs {
		f := parsed[i]
		err := parseErrs[i]
		var errs diag.List
		if err != nil {
			addParseErrors(&errs, err)
		}
		file := &File{Path: s.path, AST: f, Test: isTestFile(s.name), Src: l.keepSrc(p, s.src)}
		file.BodiesSkipped = l.cfg.SkipBodies != nil && l.cfg.SkipBodies(s.path)
		name := f.Name.Name
		if file.Test && strings.HasSuffix(name, "_test") && name != p.Name {
			// An external test package: package geom_test, next to
			// package geom. go/build's rule. Its files are checked against
			// the name of p at the end (xtestConflict).
			xfiles = append(xfiles, file)
			xerrs = append(xerrs, errs.Diagnostics()...)
			xnames = append(xnames, xtestName{s.name, strings.TrimSuffix(name, "_test")})
			continue
		}
		internalTests = internalTests || file.Test
		// The name of p comes from its own files (not from the external
		// test files), so it is the same whether or not the test files are
		// read.
		switch {
		case name == "":
			// No package clause: the parser has reported that.
		case p.Name == "":
			p.Name, p.nameFile = name, s.name
		case name != p.Name && !reported:
			p.Errors.Errorf(p.at, "found packages %s (%s) and %s (%s) in %s", p.Name, p.nameFile, name, s.name, p.Dir)
			reported = true
		}
		p.Files = append(p.Files, file)
		for _, d := range errs.Diagnostics() {
			p.Errors.Add(d)
		}
	}
	xname := p.Name
	if len(xnames) > 0 {
		if d, ok := xtestConflict(p.Name, p.nameFile, p.Dir, p.at, xnames); ok {
			xerrs = append(xerrs, d)
		}
		if xname == "" {
			xname = xnames[0].name
		}
	}
	if len(xfiles) > 0 || len(xerrs) > 0 {
		xt := l.newPackage(p.ImportPath+"_test", p.Dir, p.Std, p.Module)
		xt.Name = xname + "_test"
		xt.Files = xfiles
		for _, d := range xerrs {
			xt.Errors.Add(d)
		}
		xt.xtestOf, p.xtest = p, xt
	}
	return internalTests
}

// An xtestName is an external test file and its package name without
// _test.
type xtestName struct {
	file, name string
}

// xtestConflict returns the "found packages" error for the first external
// test file (of xs, sorted by file name) whose name doesn't fit the
// package name pkgName, which the file nameFile gives (or, if pkgName is
// "", the first external test file). The two files are shown in the order
// of their names, as go/build would find them. The error is at the
// position at.
func xtestConflict(pkgName, nameFile, dir string, at token.Position, xs []xtestName) (diag.Diagnostic, bool) {
	for _, x := range xs {
		switch {
		case pkgName == "":
			pkgName, nameFile = x.name, x.file
		case x.name != pkgName:
			a, af, b, bf := pkgName, nameFile, x.name, x.file
			if bf < af {
				a, af, b, bf = b, bf, a, af
			}
			return diag.Diagnostic{Pos: at, Msg: fmt.Sprintf("found packages %s (%s) and %s (%s) in %s", a, af, b, bf, dir)}, true
		}
	}
	return diag.Diagnostic{}, false
}

// clauseName returns the package name in the package clause of the file
// with the contents src, or "" if it has none.
func clauseName(src []byte) string {
	f, _ := parser.ParseFile(token.NewFileSet(), "", src, parser.PackageClauseOnly)
	return f.Name.Name
}

// readDir returns the entries of the folder of p, sorted by name, with the
// files of Config.Overlay in it.
func (l *loader) readDir(p *Package) ([]fs.DirEntry, error) {
	if p.Std {
		return fs.ReadDir(l.cfg.Stdlib, l.stdPath(p))
	}
	entries, err := l.cfg.FS.ReadDir(p.Dir)
	extra := l.overlayDirs[p.Dir]
	if len(extra) == 0 {
		return entries, err
	}
	// The folder may be only in the overlay (not saved yet).
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	for _, name := range extra {
		if !slices.ContainsFunc(entries, func(e fs.DirEntry) bool { return e.Name() == name }) {
			entries = append(entries, overlayEntry(name))
		}
	}
	slices.SortFunc(entries, func(a, b fs.DirEntry) int { return strings.Compare(a.Name(), b.Name()) })
	return entries, nil
}

// readFile reads the file name in the folder of p.
func (l *loader) readFile(p *Package, name string) ([]byte, error) {
	if p.Std {
		return fs.ReadFile(l.cfg.Stdlib, l.stdPath(p)+"/"+name)
	}
	return l.readAbs(filepath.Join(p.Dir, name))
}

// readAbs reads the file with the absolute name path, from Config.Overlay
// if it is there.
func (l *loader) readAbs(path string) ([]byte, error) {
	if src, ok := l.cfg.Overlay[path]; ok {
		return src, nil
	}
	return l.cfg.FS.ReadFile(path)
}

// isDir reports whether the absolute name dir is a folder, on disk or in
// Config.Overlay.
func (l *loader) isDir(dir string) bool {
	if fi, err := l.cfg.FS.Stat(dir); err == nil {
		return fi.IsDir()
	}
	return len(l.overlayDirs[dir]) > 0
}

// An overlayEntry is a file of Config.Overlay that is not on disk, as an
// entry of its folder.
type overlayEntry string

func (e overlayEntry) Name() string               { return string(e) }
func (e overlayEntry) IsDir() bool                { return false }
func (e overlayEntry) Type() fs.FileMode          { return 0 }
func (e overlayEntry) Info() (fs.FileInfo, error) { return nil, fs.ErrNotExist }

// stdPath returns the folder of the standard package p in Config.Stdlib.
func (l *loader) stdPath(p *Package) string {
	return strings.TrimSuffix(p.ImportPath, "_test")
}

// process runs the parse-level steps of the front-end on the files of p,
// in the order of Phase 01 README §2.5, and loads the packages that p
// imports before it classifies the imports, so that ClassifyImports gets
// their real names and RewriteFile the names of the dot imports.
func (l *loader) process(p *Package) {
	asts := make([]*ast.File, len(p.Files))
	for i, f := range p.Files {
		asts[i] = f.AST
	}
	pkgLevel := PackageLevelNames(asts)

	var paths []string              // the imported paths, in the order of their first import
	first := make(map[string]*File) // the file with the first import of each path
	// The checks of one file's tree don't depend on the other files, so they run together;
	// their errors come in the order of the files.
	forEachFile(p.Files, p.Errors, func(_ int, f *File, errs *diag.List) {
		if !p.Std {
			// The standard library is gocpp's own, and it keeps Go's
			// names: unicode.C (Phase 09 ports unicode).
			CheckReservedC(l.fset, f.AST, errs)
		}
		f.Directives = ParseDirectives(l.fset, f.AST, errs)
		f.Externs = ExternNames(f.AST)
		if !p.Std {
			checkNativeStd(l.fset, f.AST, errs)
		} else {
			checkExternStd(l.fset, f.AST, errs)
		}
	})
	for _, f := range p.Files {
		for _, path := range PackagePaths(f.AST) {
			if first[path] == nil {
				first[path] = f
				paths = append(paths, path)
			}
		}
	}

	problems := make(map[string]string) // the error of each path that can't be resolved
	for _, path := range paths {
		dep, msg := l.resolve(p, path, l.fset.Position(importPos(first[path].AST, path)))
		switch {
		case msg != "":
			problems[path] = msg
		case dep != nil:
			p.Imports[path] = dep
			l.loadDep(dep, filepath.Base(first[path].Path))
		}
	}

	packageName := func(path string) string {
		if dep := p.Imports[path]; dep != nil {
			return dep.Name
		}
		return ""
	}
	dotNames := func(path string) []string {
		return slices.Collect(maps.Keys(l.dotImported(p, path)))
	}
	for _, f := range p.Files {
		f.Info = ClassifyImports(l.fset, f.AST, pkgLevel, packageName, dotNames, p.Errors)
	}
	l.moduleHeaders(p) // moduleheader.go

	// The errors of the imports, once for each path, at its first import
	// that ClassifyImports took as a gocpp package, like the go command.
	done := make(map[string]bool)
	for _, f := range p.Files {
		for _, spec := range f.Info.Packages {
			path, _ := importPath(spec)
			// The external test of a standard package may dot-import it (Go's tests of unicode do): the
			// embedded library has no test files of its own, so only the tests of step 9.10 are affected
			// (GOCPP_TEST_STDLIB), and none of them uses a C/C++ name.
			stdTest := p.xtestOf != nil && p.xtestOf.Std
			if spec.Name != nil && spec.Name.Name == "." && !stdTest && l.exportedNames(p, path)["C"] {
				p.Errors.Errorf(l.fset.Position(spec.Pos()),
					"cannot dot-import %q: it exports C, which is reserved for C's built-in types", path)
			}
			if done[path] {
				continue
			}
			done[path] = true
			pos := l.fset.Position(spec.Pos())
			if msg, ok := problems[path]; ok {
				p.Errors.Errorf(pos, "%s", msg)
			} else if dep := p.Imports[path]; dep != nil && dep.Name == "main" && dep != p.xtestOf && dep != p.testMainOf {
				p.Errors.Errorf(pos, "import %q is a program, not an importable package", path)
			}
		}
	}
	// A path whose every import ClassifyImports rejected (C "fmt") is not
	// an import of the package.
	for path := range p.Imports {
		if !done[path] {
			delete(p.Imports, path)
		}
	}

	// The names that the dot imports bring are found first, one file after the other: they read
	// the packages that p imports. The rewrites of the files don't depend on each other.
	fileDots := make([]map[string]bool, len(p.Files))
	for i, f := range p.Files {
		fileDots[i] = l.dotNames(p, f.Info)
	}
	forEachFile(p.Files, p.Errors, func(i int, f *File, errs *diag.List) {
		addOnTopUnresolved(f.AST) // macros.go: the lines of macros are not walked by the parser's resolver
		f.Rewrite = RewriteFile(l.fset, f.AST, f.Info, pkgLevel, fileDots[i], errs)
	})
	l.builtinMacros(p) // builtins.go: #file, #line, #func, #text, #embed and //go:embed
}

// loadDep loads the package dep that the package on top of the stack
// imports, in its file from, unless it is loaded already. If dep is still
// loading, the import closes a cycle.
func (l *loader) loadDep(dep *Package, from string) {
	switch l.state[dep] {
	case loaded:
		return
	case loading:
		l.cycle(dep, from)
		return
	}
	l.stack = append(l.stack, stackEntry{pkg: dep, from: from})
	l.loadPackage(dep, nil, false)
	l.stack = l.stack[:len(l.stack)-1]
	if g := l.graph; g != nil {
		// A package loaded for a test: it is for the test only if it
		// imports the test variant. Otherwise the rest of the program can
		// share it.
		if g.dependsOnRoot(l, dep) {
			dep.ForTest = g.root.ImportPath
		} else if l.byPath[dep.ImportPath] == nil {
			l.byPath[dep.ImportPath] = dep
		}
	}
}

// cycle reports the import cycle that the import of dep in the file from
// of the package on top of the stack closes. The error goes to dep, and it
// shows the import stack from the root, like the go command:
//
//	package example.com/app
//		imports example.com/app/a from main.gp
//		imports example.com/app/b from a.gp
//		imports example.com/app/a from b.gp: import cycle not allowed
//
// A cycle through the test variant of a test ends in "import cycle not
// allowed in test". A package gets one cycle error, the first one found.
func (l *loader) cycle(dep *Package, from string) {
	if dep.cycleReported {
		return
	}
	dep.cycleReported = true
	msg := "import cycle not allowed"
	if l.graph != nil && dep == l.graph.root {
		msg += " in test"
	}
	d := diag.Diagnostic{Msg: "package " + l.stack[0].pkg.ImportPath}
	for _, e := range l.stack[1:] {
		d.Related = append(d.Related, diag.Related{Msg: "imports " + e.pkg.ImportPath + " from " + e.from})
	}
	d.Related = append(d.Related, diag.Related{
		Msg: "imports " + dep.ImportPath + " from " + from + ": " + msg,
	})
	dep.Errors.Add(d)
}

// dotNames returns the names that the dot imports of a file (with the
// FileInfo info) of the package p bring into the file: the exported
// package-level names of each dot-imported package.
func (l *loader) dotNames(p *Package, info *FileInfo) map[string]bool {
	var names map[string]bool
	for _, spec := range info.Packages {
		if spec.Name == nil || spec.Name.Name != "." {
			continue
		}
		path, _ := importPath(spec)
		for name := range l.dotImported(p, path) {
			if names == nil {
				names = make(map[string]bool)
			}
			names[name] = true
		}
	}
	return names
}

// dotImported returns the names that a dot import of path in the package p
// brings into the file: the exported names of the package, but not C,
// which can't be dot-imported (process reports the error).
func (l *loader) dotImported(p *Package, path string) map[string]bool {
	names := l.exportedNames(p, path)
	if names["C"] {
		names = maps.Clone(names)
		delete(names, "C")
	}
	return names
}

// exportedNames returns the exported package-level names of the package
// that p imports with the import path path, or nil if it is not known.
func (l *loader) exportedNames(p *Package, path string) map[string]bool {
	if path == "unsafe" {
		return unsafeNames()
	}
	if dep := p.Imports[path]; dep != nil {
		return dep.exportedNames()
	}
	return nil
}

// exportedNames returns the exported package-level names of p.
func (p *Package) exportedNames() map[string]bool {
	if p.exported == nil {
		asts := make([]*ast.File, len(p.Files))
		for i, f := range p.Files {
			asts[i] = f.AST
		}
		p.exported = make(map[string]bool)
		for name := range PackageLevelNames(asts) {
			if token.IsExported(name) {
				p.exported[name] = true
			}
		}
	}
	return p.exported
}

// unsafeNames returns the names of package unsafe.
func unsafeNames() map[string]bool {
	names := make(map[string]bool)
	for _, name := range types.Unsafe.Scope().Names() {
		names[name] = true
	}
	return names
}

// importPos returns the position of the first import of path in f.
func importPos(f *ast.File, path string) token.Pos {
	for _, spec := range fileImports(f) {
		if p, ok := importPath(spec); ok && p == path {
			return spec.Pos()
		}
	}
	return token.NoPos
}

// setIncomplete sets Package.Incomplete in the packages pkgs, which hold
// every package that they import. A package in an import cycle is
// incomplete: the loader reported the cycle at one of its packages.
func setIncomplete(pkgs []*Package) {
	state := make(map[*Package]loadState) // loading: on the way to the package being looked at
	var visit func(p *Package) bool
	visit = func(p *Package) bool {
		if p.summary {
			return p.Incomplete // Config.Deps: set by its own load
		}
		switch state[p] {
		case loading:
			return true // a cycle
		case loaded:
			return p.Incomplete
		}
		state[p] = loading
		p.Incomplete = p.Errors.HasErrors()
		for _, dep := range p.Imports {
			if visit(dep) {
				p.Incomplete = true
			}
		}
		state[p] = loaded
		return p.Incomplete
	}
	for _, p := range pkgs {
		visit(p)
	}
}

// initOrder returns the packages pkgs (in the order they started loading),
// each one after the packages it imports: of the packages whose imports
// are all done, the one with the smallest import path comes first, which
// is the order in which Go initializes packages. Packages in an import
// cycle, and the packages that import them, come at the end, in the order
// of pkgs.
func initOrder(pkgs []*Package) []*Package {
	index := make(map[*Package]int, len(pkgs))
	for i, p := range pkgs {
		index[p] = i
	}
	waiting := make(map[*Package]int)          // the imports of each package that are not done
	importers := make(map[*Package][]*Package) // the packages that import each package
	ready := &pkgHeap{index: index}
	for _, p := range pkgs {
		for _, dep := range p.Imports {
			if dep.summary {
				continue // Config.Deps: not loaded here, so done
			}
			waiting[p]++
			importers[dep] = append(importers[dep], p)
		}
		if waiting[p] == 0 {
			heap.Push(ready, p)
		}
	}
	order := make([]*Package, 0, len(pkgs))
	done := make(map[*Package]bool)
	for ready.Len() > 0 {
		p := heap.Pop(ready).(*Package)
		order = append(order, p)
		done[p] = true
		for _, user := range importers[p] {
			if waiting[user]--; waiting[user] == 0 {
				heap.Push(ready, user)
			}
		}
	}
	for _, p := range pkgs {
		if !done[p] {
			order = append(order, p)
		}
	}
	return order
}

// A pkgHeap holds packages, the smallest import path first, and for the
// same import path the package that started loading first.
type pkgHeap struct {
	pkgs  []*Package
	index map[*Package]int
}

func (h *pkgHeap) Len() int { return len(h.pkgs) }
func (h *pkgHeap) Less(i, j int) bool {
	a, b := h.pkgs[i], h.pkgs[j]
	return cmp.Or(cmp.Compare(a.ImportPath, b.ImportPath), cmp.Compare(h.index[a], h.index[b])) < 0
}
func (h *pkgHeap) Swap(i, j int) { h.pkgs[i], h.pkgs[j] = h.pkgs[j], h.pkgs[i] }
func (h *pkgHeap) Push(x any)    { h.pkgs = append(h.pkgs, x.(*Package)) }
func (h *pkgHeap) Pop() any {
	p := h.pkgs[len(h.pkgs)-1]
	h.pkgs = h.pkgs[:len(h.pkgs)-1]
	return p
}

// addParseErrors adds the errors of the parser to errs.
func addParseErrors(errs *diag.List, err error) {
	var list scanner.ErrorList
	if errors.As(err, &list) {
		for _, e := range list {
			errs.Add(diag.Diagnostic{Pos: e.Pos, Msg: e.Msg})
		}
		return
	}
	errs.Errorf(token.Position{}, "%s", err)
}

// isTestFile reports whether the file name is the name of a test file.
func isTestFile(name string) bool {
	return strings.HasSuffix(name, "_test.gp")
}

// isLocalPattern reports whether the pattern is a folder, not an import
// path: ".", "..", "./x", "../x" or an absolute path.
func isLocalPattern(pat string) bool {
	return pat == "." || pat == ".." || strings.HasPrefix(pat, "./") || strings.HasPrefix(pat, "../") ||
		filepath.IsAbs(pat) || filepath.Separator != '/' &&
		(strings.HasPrefix(pat, "."+string(filepath.Separator)) || strings.HasPrefix(pat, ".."+string(filepath.Separator)))
}

// abs returns the absolute name of the file or folder name, which is
// relative to Config.Dir.
func (l *loader) abs(name string) string {
	if filepath.IsAbs(name) {
		return filepath.Clean(name)
	}
	return filepath.Join(l.cfg.Dir, name)
}

// joinImportPath joins the module path and the slash-separated path of a
// folder in the module.
func joinImportPath(modPath, rel string) string {
	if rel == "." {
		return modPath
	}
	return modPath + "/" + rel
}

// parseMode returns the mode of the parser for the files of a package.
func (l *loader) parseMode() parser.Mode {
	mode := parser.ParseComments
	if l.cfg.AllErrors {
		mode |= parser.AllErrors
	}
	// The syntax of Phase 10 (F!(x), func f!(), #Name, macro, lazy) is always on
	// (ADR-095): the parser makes stock go/ast nodes with marks (package gpast).
	return mode | parser.Phase10Syntax
}

// keepSrc returns src for File.Src: the text of a file of a user package,
// else nil.
func (l *loader) keepSrc(p *Package, src []byte) []byte {
	if !p.Std {
		return src
	}
	return nil
}
