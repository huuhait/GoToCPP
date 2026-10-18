package build

import (
	"context"
	"errors"
	"fmt"
	"go/token"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/huuhait/gocpp/internal/diag"
	"github.com/huuhait/gocpp/internal/load"
	"github.com/huuhait/gocpp/internal/load/gocppmod"
)

// The C and C++ libraries of the modules (Phase 06 step 6.1d, docs/spec/modules.md
// "C and C++ libraries", ADR-024).
//
// A library is a module that gocpp.mod gives a recipe for:
//
//	cxx github.com/ocornut/imgui {
//		src imgui.cpp imgui_draw.cpp backends/*.cpp
//		include . backends
//		define IMGUI_DISABLE_OBSOLETE_FUNCTIONS
//	}
//
// load finds the recipe of each module whose headers a package imports
// (load.CxxModule); ReadPackageFlags reads it here, with the same checks as
// a //gocpp:cflags line, and the result is a CxxLib in PackageFlags.Libs.
// What the recipe says splits in three:
//
//   - the PUBLIC flags (include, system-include, define, and the compile flags
//     of pkg-config) go to the library's own compiles and to every compile and
//     probe of the packages that use the library: a macro that changes the
//     layout of a header must be the same on both sides, and <x.h> lookups
//     must work for the user;
//   - the PRIVATE flags (private-define, cflags, cxxflags, std, cstd) are for
//     the compiles of the library only;
//   - the link flags (link, ldflags, the libraries of pkg-config) go to the
//     link of the program, after the objects.
//
// The sources are listed by ResolveSources (patterns and the symbolic link
// check are in there) and compiled by the Builder, once for each
// (module, version, recipe, compiler, flags) through the object cache.
//
// Everything in a recipe comes from a module that may have been downloaded,
// so it is data, never code: nothing of it is executed; every flag goes
// through the flag allow-list (security.go, refused.go), and every path stays
// inside the module's folder: a path is relative and has no "..", and the
// files it names are looked at with their symbolic links resolved, so a link
// that leaves the module is an error. The one program that runs for a recipe
// is pkg-config, with package names that can't name a file.

// A CxxLib is the library of one module, as its recipe says it for a build
// on one system.
type CxxLib struct {
	Module  string // the module path
	Version string
	Dir     string // the folder of the module (absolute)

	// File is the gocpp.mod that has the recipe, and FromLibrary says that it
	// is the library's own file, not the project's.
	File        string
	FromLibrary bool

	// Public are the compile flags that the library and the packages that use
	// it share (-I, -isystem, -D, the flags of pkg-config), in the order of the
	// recipe, with absolute paths.
	Public []string

	// Private are the flags of every compile of the library (private-define
	// and cflags), and PrivateCXX those of its C++ files only (cxxflags).
	Private, PrivateCXX []string

	// Std is the C++ standard of the library (c++20), CStd the C standard of
	// its C files (gnu17), without -std=.
	Std, CStd string

	// Link are the flags of the link of a program with the library, after
	// the objects.
	Link []string

	srcs, excludes []*gocppmod.RecipeStmt

	once    sync.Once
	sources []LibSource
	errs    []diag.Diagnostic
}

// A LibSource is one source file of a library.
type LibSource struct {
	Rel  string // the file in the module, with /
	File string // the absolute file
	C    bool   // a C file; else C++
	Line int    // the line of gocpp.mod that listed it
	Text string // the text of that line
}

// Limits on what a recipe's patterns look at: a pattern comes from a
// dependency, and a huge folder is an error, not a long wait.
var (
	maxLibSources  = 10000  // source files of a library
	maxGlobEntries = 200000 // directory entries read for the patterns of a library
	maxDirEntries  = 100000 // entries of one folder
)

// readRecipes reads the recipes of the modules whose headers lp imports and
// adds the public compile flags of each library to flags.Compile, its link
// flags to flags.Link and the library to flags.Libs. The errors are at the
// lines of the files that have the recipes. A recipe with an error adds
// nothing.
func readRecipes(ctx context.Context, lp *load.Package, pkgConfig PkgConfigFunc, flags *PackageFlags) (errs []diag.Diagnostic, pending bool) {
	seen := make(map[string]bool) // errors of a file, once
	report := func(file string, line int, msg string) {
		key := fmt.Sprintf("%s:%d:%s", file, line, msg)
		if !seen[key] {
			seen[key] = true
			errs = append(errs, diag.Diagnostic{Pos: token.Position{Filename: file, Line: line}, Msg: msg})
		}
	}
	for _, cm := range lp.CxxModules {
		for _, e := range cm.Errors {
			report(e.Filename, e.Pos.Line, e.Err)
		}
		if cm.Recipe == nil {
			continue
		}
		lib, lerrs, lpending := readRecipe(ctx, lp, cm, pkgConfig)
		for _, e := range lerrs {
			report(e.Pos.Filename, e.Pos.Line, e.Msg)
		}
		pending = pending || lpending
		if lib == nil || len(lerrs) > 0 || lpending {
			continue
		}
		flags.Compile = append(flags.Compile, lib.Public...)
		flags.Link = append(flags.Link, lib.Link...)
		flags.Libs = append(flags.Libs, lib)
	}
	return errs, pending
}

// readRecipe reads the recipe of the module cm into a library.
func readRecipe(ctx context.Context, lp *load.Package, cm *load.CxxModule, pkgConfig PkgConfigFunc) (lib *CxxLib, errs []diag.Diagnostic, pending bool) {
	mod := cm.Module
	recipe, perrs := gocppmod.ParseRecipe(cm.RecipeFile, mod.Path, cm.Recipe)
	for _, e := range perrs {
		errs = append(errs, diag.Diagnostic{Pos: token.Position{Filename: e.Filename, Line: e.Pos.Line}, Msg: e.Err})
	}
	lib = &CxxLib{Module: mod.Path, Version: mod.Version, Dir: mod.Dir, File: cm.RecipeFile, FromLibrary: cm.FromLibrary,
		Std: "c++20", CStd: "gnu17"}
	if cm.Std != "" {
		lib.Std = cm.Std
	}
	root, err := newModRoot(mod.Dir)
	if err != nil {
		errs = append(errs, diag.Diagnostic{Pos: token.Position{Filename: cm.RecipeFile, Line: cm.Recipe.Start.Line},
			Msg: fmt.Sprintf("cxx %s: the module's folder: %v", mod.Path, err)})
		return lib, errs, false
	}
	for _, s := range recipe.Stmts {
		if !s.Cond.Match(cm.GOOS) {
			continue
		}
		fail := func(format string, args ...any) {
			errs = append(errs, diag.Diagnostic{Pos: token.Position{Filename: cm.RecipeFile, Line: s.Line},
				Msg: fmt.Sprintf("cxx %s: ", mod.Path) + fmt.Sprintf(format, args...)})
		}
		source := "the cxx recipe of " + mod.Path
		switch s.Verb {
		case gocppmod.RecipeSrc:
			lib.srcs = append(lib.srcs, s)
		case gocppmod.RecipeExclude:
			lib.excludes = append(lib.excludes, s)
		case gocppmod.RecipeInclude, gocppmod.RecipeSystemInclude:
			flag := "-I"
			if s.Verb == gocppmod.RecipeSystemInclude {
				flag = "-isystem"
			}
			for _, a := range s.Args {
				abs := root.join(a)
				fi, err := os.Stat(abs)
				switch {
				case err != nil && errors.Is(err, fs.ErrNotExist):
					fail("%s %s: no such folder in the module", s.Verb, a)
					continue
				case err != nil:
					fail("%s %s: %v", s.Verb, a, err)
					continue
				case !fi.IsDir():
					fail("%s %s: not a folder", s.Verb, a)
					continue
				}
				if err := root.leaves(abs); err != nil {
					fail("%s %s: %v", s.Verb, a, err)
					continue
				}
				lib.Public = append(lib.Public, flag, abs)
			}
		case gocppmod.RecipeDefine, gocppmod.RecipePrivateDefine:
			var args []string
			for _, a := range s.Args {
				args = append(args, "-D"+a)
			}
			if err := checkCompilerFlags("CFLAGS", source, args); err != nil {
				fail("%s: %v", s.Verb, err)
				continue
			}
			if s.Verb == gocppmod.RecipeDefine {
				lib.Public = append(lib.Public, args...)
			} else {
				lib.Private = append(lib.Private, args...)
			}
		case gocppmod.RecipeCflags, gocppmod.RecipeCxxflags:
			if err := checkCompilerFlags("CFLAGS", source, s.Args); err != nil {
				fail("%s: %v", s.Verb, err)
				continue
			}
			args, err := confineFlagPaths(s.Args, root, false)
			if err != nil {
				fail("%s: %v", s.Verb, err)
				continue
			}
			if s.Verb == gocppmod.RecipeCflags {
				lib.Private = append(lib.Private, args...)
			} else {
				lib.PrivateCXX = append(lib.PrivateCXX, args...)
			}
		case gocppmod.RecipeLdflags:
			if err := checkLinkerFlags("LDFLAGS", source, s.Args); err != nil {
				fail("%s: %v", s.Verb, err)
				continue
			}
			if bad := recipeLinkRefused(s.Args); bad != "" {
				fail("%s: invalid flag in %s: %s (a recipe can't set the run-time library path, read other files at the link, name a library by a path or load a plugin)", s.Verb, source, bad)
				continue
			}
			args, err := confineFlagPaths(s.Args, root, true)
			if err != nil {
				fail("%s: %v", s.Verb, err)
				continue
			}
			lib.Link = append(lib.Link, args...)
		case gocppmod.RecipeLink:
			if err := checkLinkerFlags("LDFLAGS", source, s.Args); err != nil {
				fail("%s: %v", s.Verb, err)
				continue
			}
			lib.Link = append(lib.Link, s.Args...)
		case gocppmod.RecipePkgConfig:
			if pkgConfig == nil {
				pkgConfig = PkgConfig
			}
			cflags, libs, err := pkgConfig(ctx, lp.Dir, s.Args)
			if errors.Is(err, ErrPending) {
				pending = true
				continue
			}
			if err == nil {
				err = checkCompilerFlags("CFLAGS", "pkg-config --cflags", cflags)
			}
			if err == nil {
				err = checkLinkerFlags("LDFLAGS", "pkg-config --libs", libs)
			}
			if err != nil {
				fail("%s: %v", s.Verb, err)
				continue
			}
			lib.Public = append(lib.Public, cflags...)
			lib.Link = append(lib.Link, libs...)
		case gocppmod.RecipeStd:
			lib.Std = s.Args[0]
		case gocppmod.RecipeCstd:
			lib.CStd = s.Args[0]
		}
	}
	return lib, errs, pending
}

// recipeLinkRefused returns the first link flag of args that a recipe may
// not have, though the allow-list lets it through: the flags of -Wl that take
// a path (the run-time library path, files whose symbols are used, a section
// made of a file, the system root of the linker, libraries and library
// folders given to the linker itself), the macOS linker plugin, and a library
// named by a path: -l:../../etc/x or -l../x is looked up by the linker in the
// -L folders and the system's, so a "/" or ".." in a name leaves the module
// (-l:libfoo.a, a file name in the library folders, stays fine).
func recipeLinkRefused(args []string) string {
	badLib := func(name string) bool {
		return strings.ContainsAny(name, `/\`) || strings.Contains(name, "..")
	}
	for i, a := range args {
		for _, p := range []string{"-Wl,-R", "-Wl,-rpath", "-Wl,--just-symbols", "-Wl,-sectcreate", "-Wl,-syslibroot", "-Wl,-framework",
			"-Wl,--push-state", "-Wl,-l", "-Wl,--library", "-Wl,-L", "-Wl,-library", "-lto_library"} {
			if strings.HasPrefix(a, p) {
				return a
			}
		}
		switch {
		case a == "-l" && i+1 < len(args) && badLib(args[i+1]):
			return a + " " + args[i+1]
		case strings.HasPrefix(a, "-l") && badLib(a[2:]):
			return a
		}
	}
	return ""
}

// A modRoot is the folder of a module, for the checks that keep a recipe's
// paths inside it.
type modRoot struct {
	dir  string // the folder, as the loader gave it
	real string // the folder with its symbolic links resolved
}

func newModRoot(dir string) (*modRoot, error) {
	real, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return nil, err
	}
	return &modRoot{dir: dir, real: real}, nil
}

// join returns the absolute file of the slash path rel in the module.
func (m *modRoot) join(rel string) string {
	return filepath.Join(m.dir, filepath.FromSlash(rel))
}

// leaves returns an error when the file or folder abs (in the module's
// folder, with the names of the recipe, which have no "..") is, after its
// symbolic links are resolved, not inside the module. It returns nil when
// abs doesn't exist: the caller looks at that.
func (m *modRoot) leaves(abs string) error {
	_, err := m.resolve(abs)
	if err != nil && errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	return err
}

// resolve returns abs with its symbolic links resolved, or the error that it
// does not exist or that it leaves the module. For a file that does not exist
// it still looks at the folder it would be in: a missing file under a link
// that leads out of the module is the error of the link.
func (m *modRoot) resolve(abs string) (string, error) {
	real, err := filepath.EvalSymlinks(abs)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			for p := filepath.Dir(abs); p != abs && len(p) >= len(m.dir); p = filepath.Dir(p) {
				if r, perr := filepath.EvalSymlinks(p); perr == nil {
					if lerr := m.within(abs, r); lerr != nil {
						return "", lerr
					}
					break
				}
			}
		}
		return "", err
	}
	if err := m.within(abs, real); err != nil {
		return "", err
	}
	return real, nil
}

// within returns an error when real, the file abs with its links resolved, is
// not inside the module.
func (m *modRoot) within(abs, real string) error {
	rel, err := filepath.Rel(m.real, real)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return fmt.Errorf("a symbolic link leaves the module (%s leads to %s)", abs, real)
	}
	return nil
}

// confineFlagPaths makes the paths of the flags args (checked by the
// allow-list already) absolute, from the module's folder, like absPaths does
// for a package, and refuses a path that is not inside the module: an
// absolute one, one with "..", one that starts with = or $ (the system root)
// and one that leads out of the module through a symbolic link. The flags
// with a path are those of absPaths.
func confineFlagPaths(args []string, root *modRoot, link bool) ([]string, error) {
	args = slices.Clone(args)
	var firstErr error
	fix := func(p string, mustExist bool) string {
		if p == "" || firstErr != nil {
			return p
		}
		slash := filepath.ToSlash(p)
		clean := path.Clean(slash)
		switch {
		case filepath.IsAbs(p) || strings.HasPrefix(slash, "/") || strings.HasPrefix(p, "=") || strings.HasPrefix(p, "$") ||
			strings.HasPrefix(p, "~") || filepath.VolumeName(p) != "":
			firstErr = fmt.Errorf("the path %q is not inside the module: a recipe's paths are relative to the module's folder", p)
			return p
		case clean == ".." || strings.HasPrefix(clean, "../"):
			firstErr = fmt.Errorf("the path %q is not inside the module: a recipe's paths have no \"..\"", p)
			return p
		}
		abs := root.join(clean)
		if _, err := root.resolve(abs); err != nil && !errors.Is(err, fs.ErrNotExist) {
			firstErr = fmt.Errorf("the path %q: %v", p, err)
			return p
		} else if err != nil && mustExist {
			return p // -include x.h: looked up through the include path
		}
		return abs
	}
	joined := []string{"-I", "-F"}
	pathNext := []string{"-I", "-F", "-isystem", "-include"}
	if link {
		joined = []string{"-L", "-F"}
		pathNext = []string{"-L", "-F"}
	}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case slices.Contains(pathNext, arg) && i+1 < len(args):
			i++
			args[i] = fix(args[i], arg == "-include")
		case strings.HasPrefix(arg, "-"):
			if len(arg) > 2 && slices.Contains(joined, arg[:2]) {
				args[i] = arg[:2] + fix(arg[2:], false)
			} else if i+1 < len(args) && takesNext(arg, link) {
				i++
			}
		case link:
			args[i] = fix(arg, false)
		}
	}
	return args, firstErr
}

// Sources returns the source files of the library, from the patterns of its
// recipe, and the errors at the lines of gocpp.mod (a file that doesn't exist,
// a pattern that matches nothing, a file that is not a C or C++ source, a
// file that leaves the module by a symbolic link, too many files). The files
// are sorted by name. It looks at the file system once.
func (lib *CxxLib) Sources() ([]LibSource, []diag.Diagnostic) {
	lib.once.Do(lib.resolveSources)
	return lib.sources, lib.errs
}

func (lib *CxxLib) resolveSources() {
	fail := func(line int, format string, args ...any) {
		lib.errs = append(lib.errs, diag.Diagnostic{Pos: token.Position{Filename: lib.File, Line: line},
			Msg: fmt.Sprintf("cxx %s: ", lib.Module) + fmt.Sprintf(format, args...)})
	}
	if len(lib.srcs) == 0 {
		return
	}
	root, err := newModRoot(lib.Dir)
	if err != nil {
		fail(lib.srcs[0].Line, "the module's folder: %v", err)
		return
	}
	budget := &globBudget{entries: maxGlobEntries}
	seen := make(map[string]bool) // by the real file
	var all []LibSource
	for _, s := range lib.srcs {
		for _, pat := range s.Args {
			rels, err := root.expand(pat, budget)
			if err != nil {
				fail(s.Line, "%s %s: %v", s.Verb, pat, err)
				continue
			}
			for _, rel := range rels {
				abs := root.join(rel)
				real, err := root.resolve(abs)
				if err != nil {
					fail(s.Line, "%s %s: %s: %v", s.Verb, pat, rel, err)
					continue
				}
				if seen[real] {
					continue
				}
				seen[real] = true
				if len(all) >= maxLibSources {
					fail(s.Line, "%s %s: more than %d source files", s.Verb, pat, maxLibSources)
					lib.sources = nil
					return
				}
				all = append(all, LibSource{Rel: rel, File: abs, C: strings.HasSuffix(rel, ".c"), Line: s.Line, Text: s.Text})
			}
		}
	}
	// exclude: every pattern, in every line, applies to every source.
	for _, s := range lib.excludes {
		for _, pat := range s.Args {
			all = slices.DeleteFunc(all, func(f LibSource) bool {
				ok, _ := path.Match(pat, f.Rel)
				return ok
			})
		}
	}
	slices.SortFunc(all, func(a, b LibSource) int { return strings.Compare(a.Rel, b.Rel) })
	lib.sources = all
}

// isSourceName reports whether the file name is a C or C++ source of a
// library: the same extensions as the files of a package (.c, .cc, .cpp,
// .cxx).
func isSourceName(name string) bool {
	switch path.Ext(name) {
	case ".c", ".cc", ".cpp", ".cxx":
		return true
	}
	return false
}

// maxSourceSize is the largest source file of a library: the cache and the
// compile hold a source in memory more than once, and a library comes from a
// dependency.
var maxSourceSize int64 = 32 << 20

// checkSourceFile returns why the file fi can't be a source: it must be a
// regular file (a pipe would block the build forever, a device would never
// end) of a bounded size.
func checkSourceFile(fi fs.FileInfo) error {
	switch {
	case !fi.Mode().IsRegular():
		kind := "a special file"
		switch m := fi.Mode(); {
		case m&fs.ModeNamedPipe != 0:
			kind = "a named pipe"
		case m&fs.ModeSocket != 0:
			kind = "a socket"
		case m&fs.ModeDevice != 0:
			kind = "a device"
		}
		return fmt.Errorf("not a regular file (it is %s)", kind)
	case fi.Size() > maxSourceSize:
		return fmt.Errorf("%d bytes: a source of a library is at most %d bytes", fi.Size(), maxSourceSize)
	}
	return nil
}

// A globBudget bounds the directory entries that the patterns of a library
// read.
type globBudget struct{ entries int }

// expand returns the files of the module that the pattern pat (a clean
// slash path, from the recipe) names: the file itself when it has no
// pattern characters (it must exist, be a file and a C or C++ source), else
// the files that match, among the folders it goes through, in one folder
// each (a "*" doesn't match "/"). A pattern that matches no source is an
// error. The names that start with a dot are matched only by a pattern that
// starts with a dot.
func (m *modRoot) expand(pat string, budget *globBudget) ([]string, error) {
	if !strings.ContainsAny(pat, "*?[") {
		fi, err := os.Stat(m.join(pat))
		switch {
		case errors.Is(err, fs.ErrNotExist):
			return nil, errors.New("no such file in the module")
		case err != nil:
			return nil, err
		case fi.IsDir():
			return nil, errors.New("a folder, not a file")
		case !isSourceName(pat):
			return nil, errors.New("not a C or C++ source (.c, .cc, .cpp, .cxx)")
		}
		if err := checkSourceFile(fi); err != nil {
			return nil, err
		}
		return []string{pat}, nil
	}
	segs := strings.Split(pat, "/")
	cur := []string{""}
	for i, seg := range segs {
		last := i == len(segs)-1
		var next []string
		for _, dir := range cur {
			if !strings.ContainsAny(seg, "*?[") {
				p := path.Join(dir, seg)
				if fi, err := os.Stat(m.join(p)); err == nil && (last || fi.IsDir()) {
					if !last {
						if _, err := m.resolve(m.join(p)); err != nil {
							return nil, fmt.Errorf("%s: %v", p, err) // not even listed: it leaves the module
						}
					}
					next = append(next, p)
				}
				continue
			}
			names, err := m.readNames(dir, budget)
			if err != nil {
				return nil, err
			}
			for _, name := range names {
				if strings.HasPrefix(name, ".") && !strings.HasPrefix(seg, ".") {
					continue
				}
				if ok, err := path.Match(seg, name); err != nil {
					return nil, err
				} else if !ok {
					continue
				}
				p := path.Join(dir, name)
				if !last {
					if fi, err := os.Stat(m.join(p)); err != nil || !fi.IsDir() {
						continue
					}
					if _, err := m.resolve(m.join(p)); err != nil {
						return nil, fmt.Errorf("%s: %v", p, err) // not even listed: it leaves the module
					}
				}
				next = append(next, p)
			}
		}
		cur = next
		if len(cur) == 0 {
			break
		}
	}
	var out []string
	for _, p := range cur {
		if !isSourceName(p) {
			continue
		}
		fi, err := os.Stat(m.join(p))
		if err != nil || fi.IsDir() {
			continue
		}
		if err := checkSourceFile(fi); err != nil {
			return nil, fmt.Errorf("%s: %v", p, err)
		}
		out = append(out, p)
		if len(out) > maxLibSources {
			return nil, fmt.Errorf("the pattern matches more than %d files", maxLibSources)
		}
	}
	if len(out) == 0 {
		return nil, errors.New("the pattern matches no C or C++ source file")
	}
	return out, nil
}

// readNames returns the sorted names in the folder dir of the module,
// counted against the budget.
func (m *modRoot) readNames(dir string, budget *globBudget) ([]string, error) {
	f, err := os.Open(m.join(dir))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	ents, err := f.ReadDir(maxDirEntries + 1)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, err
	}
	if len(ents) > maxDirEntries {
		return nil, fmt.Errorf("the folder %q has more than %d entries", dir, maxDirEntries)
	}
	if budget.entries -= len(ents); budget.entries < 0 {
		return nil, fmt.Errorf("the patterns look at more than %d directory entries", maxGlobEntries)
	}
	names := make([]string, len(ents))
	for i, e := range ents {
		names[i] = e.Name()
	}
	slices.Sort(names)
	return names, nil
}
