package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/huuhait/gocpp/internal/build"
	"github.com/huuhait/gocpp/internal/check"
	"github.com/huuhait/gocpp/internal/cppgen"
	"github.com/huuhait/gocpp/internal/diag"
	"github.com/huuhait/gocpp/internal/libgen"
	"github.com/huuhait/gocpp/internal/load"
	"github.com/huuhait/gocpp/internal/lower"
	"github.com/huuhait/gocpp/internal/version"
)

// The build modes of gocpp build (Phase 06 step 6.10, docs/spec/build.md "Libraries", ADR-043):
// -buildmode=c-archive, c-shared and cpp-lib make a library instead of a program. The packages are
// lowered and compiled as for a program, with -fPIC (internal/build/modes.go), the program has no
// main(), and the library's own glue starts it lazily (runtime/include/gocpp/lib.hpp). The glue of the
// C modes (internal/libgen) is the C++ file gocpp_export.cpp, one extern "C" function for each
// `//export`, and the C header; a cpp-lib has the C++ headers of the packages and CMake files.

// A libState is what one library build knows besides the program's.
type libState struct {
	mode build.Mode
	name string // the library's name: libx without .a/.so for the C modes; the program's name for a cpp-lib
	file string // the file name of the library: libx.a, libx.so, x.dll
	// implib is the file name of a Windows DLL's import library (x.dll.a), which installLibrary puts next to the DLL
	// with a copy named x.lib for linkers of the other kind.
	implib string
	// workFile and workImplib are the names the linker writes them under in the work folder: those of file and implib, or ASCII ones
	// when the compiler cannot be handed the real name (workFileName); installLibrary moves them to the real names.
	workFile, workImplib string

	header     string // c-archive, c-shared: the header's file name (libx.h)
	headerData []byte
	tree       []libgen.File // cpp-lib: the files of the installed tree besides the archive
	headers    []libgen.File // cpp-lib: the include/ files that come from the packages and the runtime
	script     []byte        // c-shared: the linker's version script

	extra []string // the files written besides the library, for the up-to-date check
}

// registerMode registers -buildmode.
func (f *buildFlags) registerMode(flags *flag.FlagSet) {
	flags.Func("buildmode", "what to build: exe (a program, the default), c-archive, c-shared or cpp-lib (libraries; see docs/spec/build.md)", func(v string) error {
		m, err := build.ParseMode(v)
		if err != nil {
			return err
		}
		if m == build.ModeExe {
			m = ""
		}
		f.mode = m
		return nil
	})
}

// libMains returns the packages of a cpp-lib: the roots, in the order in which Go initializes
// them. The last one is the entry of the program (it holds gp_init_all).
func (d *driver) libMains(prog *load.Program, roots []*load.Package) []*load.Package {
	var out []*load.Package
	for _, p := range prog.Packages {
		if slices.Contains(roots, p) {
			out = append(out, p)
		}
	}
	return out
}

// libraryNames returns the name of the library and the file name of its file. A C library is named
// after -o when it names a file (libx.a: the header is libx.h), else lib and the program's name; a
// cpp-lib is named after the program, and -o is its folder.
//
// For windows a DLL is x.dll and a static library x.a, like a program is x.exe: an -o without an extension gets it, and a DLL
// asked for as x.so (or any extension but .dll) is an error that says so.
func (d *driver) libraryNames(pkg *check.Package) (name, file string, err error) {
	ext := ".a"
	if d.flags.mode == build.ModeCShared {
		ext = build.SharedSuffix(d.check.goos)
	}
	prog := d.programName(pkg)
	if d.flags.mode == build.ModeCPPLib {
		// One package is named after itself, several after their module.
		if m := pkg.Load.Module; len(d.libRoots) > 1 && m != nil && m.Path != "" {
			prog = path.Base(m.Path)
			if majorSuffix.MatchString(prog) && strings.Contains(m.Path, "/") {
				prog = path.Base(path.Dir(m.Path))
			}
		}
		prog = libgen.Name(prog)
		return prog, "lib" + prog + ext, nil
	}
	out := d.flags.output
	if out != "" && !strings.HasSuffix(out, "/") && !strings.HasSuffix(out, string(filepath.Separator)) && !isDir(out) {
		base := filepath.Base(out)
		if d.check.goos == "windows" {
			switch have := strings.ToLower(filepath.Ext(base)); {
			case have == "":
				base += ext // -o mylib names mylib.dll (mylib.a)
			case d.flags.mode == build.ModeCShared && have != ".dll":
				suggest := strings.TrimSuffix(base, filepath.Ext(base))
				if !strings.HasSuffix(strings.ToLower(suggest), ext) {
					suggest += ext // -o z.dll.a: the import library is z.dll.a, the DLL z.dll
				}
				return "", "", fmt.Errorf("-o %s: a shared library for windows is a .dll (name it %s, or leave the extension out)", out, suggest)
			}
		}
		stem := strings.TrimSuffix(base, filepath.Ext(base))
		return stem, base, nil
	}
	if d.flags.mode == build.ModeCShared && d.check.goos == "windows" {
		return prog, prog + ext, nil // a DLL is named like a program: x.dll (its import library is x.dll.a)
	}
	return "lib" + prog, "lib" + prog + ext, nil
}

// prepareLibrary adds the glue of the library mode to the generated files of the program: for the
// C modes the header and gocpp_export.cpp, for a cpp-lib gocpp_init.cpp, and adds their units to the
// last package's. It returns the exit status.
func (d *driver) prepareLibrary(set *buildSet, files *cppgen.ProgramFiles, cxx build.CXX, env build.EnvFlags) int {
	mode := d.flags.mode
	st := &libState{mode: mode}
	var nameErr error
	st.name, st.file, nameErr = d.libraryNames(set.main)
	if nameErr != nil {
		fmt.Fprintf(d.stderr, "gocpp build: %v\n", nameErr)
		return exitErrors
	}
	st.workFile = workFileName(st.file, "gplib")
	if mode == build.ModeCShared && d.check.goos == "windows" {
		st.implib = st.file + ".a"
		st.workImplib = st.workFile + ".a"
	}
	d.lib = st
	last := len(set.pkgs) - 1
	addUnit := func(rel string, data []byte) {
		files.Files = append(files.Files, cppgen.File{Path: rel, Data: data})
		files.Packages[last].Compile = append(files.Packages[last].Compile, rel)
	}
	info := make(map[string]libgen.PkgInfo)
	for i, p := range set.pkgs {
		imp := p.Load.ImportPath
		info[imp] = libgen.PkgInfo{Header: lower.PkgDir(imp) + "/pkg.hpp", Namespace: files.Packages[i].Namespace, Funcs: files.Packages[i].Funcs}
	}
	var pbs []build.PackageBuild
	for _, p := range set.pkgs {
		pbs = append(pbs, build.PackageBuild{Flags: p.Flags})
	}
	libFlags := build.LibraryLinkFlags(pbs, env)

	if mode == build.ModeCPPLib {
		d.cppLibFiles(set, files, st, cxx, libFlags, info)
		addUnit("gpkg_export/gocpp_init.cpp", libgen.InitUnit(st.name))
		return 0
	}

	var user []*check.Package
	for _, p := range set.pkgs {
		if !p.Load.Std {
			user = append(user, p)
		}
	}
	exports, errs := libgen.Collect(d.check.fset, user)
	if len(errs) > 0 {
		d.printLibErrors(errs)
		return exitErrors
	}
	inLibc := libcSymbolSet
	if d.check.goos == "windows" {
		// The list is read from the Linux C library's files (libc.so.6 ...): a Windows target has none, and a (fake or cross) Windows
		// compiler must not fill the set that the process keeps for the Linux ones.
		inLibc = func(build.CXX) func(string) bool { return func(string) bool { return false } }
	}
	if errs := libgen.CheckSymbols(exports, inLibc(cxx)); len(errs) > 0 {
		d.printLibErrors(errs)
		return exitErrors
	}
	if len(exports) == 0 {
		fmt.Fprintf(d.stderr, "gocpp build: -buildmode=%s: the program has no //export functions: mark the functions that C calls with //export Name in their doc comment\n", mode)
		return exitErrors
	}
	stem := strings.TrimSuffix(st.file, path.Ext(st.file))
	st.header = strings.TrimSuffix(st.workFile, path.Ext(st.workFile)) + ".h" // the compiler reads this one (gocpp_export.cpp includes it)
	link := []string{"-lstdc++", "-pthread", "-lm", "-ldl"}
	if d.check.goos == "windows" {
		link = []string{"-lstdc++", "-pthread", "-lm"} // MinGW-w64 has no libdl
	}
	if mode == build.ModeCShared {
		link = []string{"-l" + strings.TrimPrefix(stem, "lib")}
		if d.check.goos == "windows" {
			link = []string{"-l" + stem} // a DLL is named like a program; x.dll.a is found by -lx, also for a name that starts with lib
		}
	} else {
		link = append(link, libFlags...)
	}
	st.headerData = libgen.Header(libgen.HeaderInfo{Name: stem, Mode: string(mode), Link: link, Exports: exports})
	files.Files = append(files.Files, cppgen.File{Path: "gpkg_export/" + st.header, Data: st.headerData})
	glue := libgen.Glue(libgen.GlueInfo{
		Name:    st.name,
		Header:  "gpkg_export/" + st.header,
		Exports: exports,
		Pkg:     func(p *check.Package) libgen.PkgInfo { return info[p.Load.ImportPath] },
	})
	addUnit("gpkg_export/gocpp_export.cpp", glue)
	if mode == build.ModeCShared && d.check.goos != "windows" {
		st.script = libgen.VersionScript(exports) // a DLL's exports are the dllexport functions
	}
	return 0
}

// cppLibFiles makes the files of the installed tree of a cpp-lib: the headers of the packages and
// of the runtime, the convenience header, the CMake package and the pkg-config file.
func (d *driver) cppLibFiles(set *buildSet, files *cppgen.ProgramFiles, st *libState, cxx build.CXX, libFlags []string, info map[string]libgen.PkgInfo) {
	for _, f := range files.Files {
		if strings.HasPrefix(f.Path, "gpkg/") && (strings.HasSuffix(f.Path, ".hpp") || strings.HasSuffix(f.Path, ".h")) {
			st.headers = append(st.headers, libgen.File{Path: path.Join("include", f.Path), Data: f.Data})
		}
	}
	// The runtime's headers (extracted again from the binary: the cache folder may not exist).
	if rt, err := build.RuntimeHeaders(); err == nil {
		for _, h := range rt {
			st.headers = append(st.headers, libgen.File{Path: path.Join("include", h.Path), Data: h.Data})
		}
	}
	// The flags that every consumer needs: gocpp's language flags, the build's own -D and
	// sanitizers, and the public flags of the packages whose headers include C or C++ headers.
	compile := []string{"-std=c++20", "-fwrapv", "-fno-strict-aliasing", "-ffp-contract=off", "-fexceptions"}
	if d.flags.nochecks {
		compile = append(compile, "-DGOCPP_NOCHECKS")
	}
	if d.flags.asan {
		compile = append(compile, "-fsanitize=address,undefined", "-fno-omit-frame-pointer")
	}
	for i, p := range set.pkgs {
		if files.Packages[i].HeaderC {
			compile = append(compile, p.Flags.Compile...)
			if importsLocalHeader(p) {
				compile = append(compile, "-iquote", p.Load.Dir)
			}
		}
	}
	family := "GNU"
	if cxx.Family == build.Clang {
		family = "Clang"
	}
	var roots []libgen.RootPackage
	for _, p := range set.pkgs {
		if d.isLibRoot(p) {
			pi := info[p.Load.ImportPath]
			roots = append(roots, libgen.RootPackage{ImportPath: p.Load.ImportPath, Name: p.Types.Name(), Header: pi.Header, Namespace: pi.Namespace})
		}
	}
	st.tree = libgen.CPPLibFiles(libgen.CPPLibInfo{
		Name: st.name, Version: version.Version, Roots: roots,
		Compile: libgen.Dedup(compile), Link: libFlags, Compiler: family,
	})
}

// isLibRoot reports whether p is one of the packages that a cpp-lib offers (the packages the
// patterns matched).
func (d *driver) isLibRoot(p *check.Package) bool {
	return slices.Contains(d.libRoots, p.Load.ImportPath)
}

// installLibrary moves the library that Link wrote to its place, with its other files, and returns
// the path of the main output for the stamp. out is the file or folder of -o, or "" for the default.
func (d *driver) installLibrary(p *program, out string) (string, int) {
	st := d.lib
	name := st.file
	if st.mode == build.ModeCPPLib {
		prefix := out
		if prefix == "" {
			prefix = st.name + "-cpp"
		}
		if !filepath.IsAbs(prefix) {
			prefix = filepath.Join(d.check.dir, prefix)
		}
		lib := filepath.Join(prefix, "lib", name)
		if err := os.MkdirAll(filepath.Dir(lib), 0o755); err != nil {
			fmt.Fprintf(d.stderr, "gocpp build: %v\n", err)
			return "", exitErrors
		}
		if d.flags.x {
			fmt.Fprintf(d.stderr, "mv %s %s\n", build.ShellQuoteWork(p.exe, p.work), build.ShellQuoteWork(lib, ""))
		}
		if err := build.MoveFile(p.exe, lib); err != nil {
			fmt.Fprintf(d.stderr, "gocpp build: %v\n", err)
			return "", exitErrors
		}
		var extra []string
		for _, f := range slices.Concat(d.lib.tree, d.lib.headers) {
			dst := filepath.Join(prefix, filepath.FromSlash(f.Path))
			if _, err := writeIfChanged(dst, f.Data); err != nil {
				fmt.Fprintf(d.stderr, "gocpp build: %v\n", err)
				return "", exitErrors
			}
			extra = append(extra, dst)
		}
		st.extra = extra
		return lib, 0
	}
	if out == "" {
		out = name
	} else if strings.HasSuffix(out, "/") || strings.HasSuffix(out, string(filepath.Separator)) || isDir(out) {
		out = filepath.Join(out, name)
	}
	if !filepath.IsAbs(out) {
		out = filepath.Join(d.check.dir, out)
	}
	out = filepath.Join(filepath.Dir(out), st.file) // -o mylib names mylib.dll for a Windows target (libraryNames)
	if d.flags.x {
		fmt.Fprintf(d.stderr, "mv %s %s\n", build.ShellQuoteWork(p.exe, p.work), build.ShellQuoteWork(out, ""))
	}
	if err := build.MoveFile(p.exe, out); err != nil {
		fmt.Fprintf(d.stderr, "gocpp build: %v\n", err)
		return "", exitErrors
	}
	hdr := filepath.Join(filepath.Dir(out), strings.TrimSuffix(filepath.Base(out), filepath.Ext(out))+".h")
	if _, err := writeIfChanged(hdr, st.headerData); err != nil {
		fmt.Fprintf(d.stderr, "gocpp build: %v\n", err)
		return "", exitErrors
	}
	st.extra = []string{hdr}
	if st.implib != "" {
		// The import library next to the DLL: x.dll.a (GNU ld finds -lx in it) and a copy x.lib (MSVC's link.exe, lld).
		imp := out + ".a"
		if err := build.MoveFile(filepath.Join(p.work, st.workImplib), imp); err != nil {
			fmt.Fprintf(d.stderr, "gocpp build: %v\n", err)
			return "", exitErrors
		}
		lib := strings.TrimSuffix(out, filepath.Ext(out)) + ".lib"
		if err := copyFile(imp, lib); err != nil {
			fmt.Fprintf(d.stderr, "gocpp build: %v\n", err)
			return "", exitErrors
		}
		st.extra = append(st.extra, imp, lib)
	}
	return out, 0
}

// printLibErrors prints the mistakes of the exports, grouped by package, at the .gp positions.
func (d *driver) printLibErrors(errs []*libgen.Error) {
	lastPkg := ""
	for _, e := range errs {
		if imp := e.Pkg.Load.ImportPath; imp != lastPkg {
			fmt.Fprintf(d.stderr, "# %s\n", imp)
			lastPkg = imp
		}
		fmt.Fprintf(d.stderr, "%s:%d:%d: %s\n", diag.ShortFileName(d.check.dir, e.Pos.Filename), e.Pos.Line, e.Pos.Column, e.Msg)
	}
}

var (
	libcOnce sync.Once
	libcSyms map[string]bool
)

// libcSymbolSet returns a function that says whether a name is a symbol that the C library defines
// (libc and libm of the compiler's target, read with nm -D). When the files or nm are not there it
// says no for every name (the list in libgen and the compiler's errors remain).
func libcSymbolSet(cxx build.CXX) func(string) bool {
	libcOnce.Do(func() {
		libcSyms = map[string]bool{}
		nm, err := exec.LookPath("nm")
		if err != nil {
			return
		}
		for _, lib := range []string{"libc.so.6", "libm.so.6", "libpthread.so.0", "libdl.so.2"} {
			cmd := exec.Command(cxx.Path, append(slices.Clone(cxx.Args), "-print-file-name="+lib)...)
			out, err := cmd.Output()
			path := strings.TrimSpace(string(out))
			if err != nil || !filepath.IsAbs(path) {
				continue
			}
			syms, err := exec.Command(nm, "-D", "--defined-only", path).Output()
			if err != nil {
				continue
			}
			for _, line := range strings.Split(string(syms), "\n") {
				f := strings.Fields(line)
				if len(f) < 3 {
					continue
				}
				name, _, _ := strings.Cut(f[len(f)-1], "@")
				libcSyms[name] = true
			}
		}
	})
	return func(n string) bool { return libcSyms[n] }
}

// copyFile writes a copy of the file from to the file to (replacing it).
func copyFile(from, to string) error {
	data, err := os.ReadFile(from)
	if err != nil {
		return err
	}
	_, err = writeIfChanged(to, data)
	return err
}
