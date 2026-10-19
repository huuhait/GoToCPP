package build

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

	rtfiles "github.com/huuhait/gocpp/runtime"
)

// Build modes (Phase 06 step 6.10, docs/spec/build.md "Libraries"). gocpp build -buildmode picks what
// the build makes of the same compiles:
//
//	exe        a program (the default)
//	c-archive  a static library libx.a that C programs link, and a C header; the exported functions
//	           are the //export directives
//	c-shared   a shared library libx.so (the same functions, hidden visibility for everything else)
//	cpp-lib    a static library libx.a for C++ programs, with the packages' own headers and CMake files
//
// The library modes compile every unit, also the runtime's, with -fPIC (a static library is linked
// into position-independent programs and into shared libraries). The shared mode also hides every symbol
// but the exports: -fvisibility=hidden, so that two gocpp libraries in one process, each with its own
// copy of the runtime, share nothing (GCC would turn the inline variables of the runtime's headers into
// GNU_UNIQUE symbols that merge across libraries and block dlclose). The mode is part of the compiler
// arguments (applyMode), so the runtime's object files, the precompiled header and the object cache
// keep a copy for each set of flags, and a build in another mode never reuses an object that was
// compiled without them.

// A Mode is the kind of output of a build.
type Mode string

const (
	ModeExe      Mode = "exe"
	ModeCArchive Mode = "c-archive"
	ModeCShared  Mode = "c-shared"
	ModeCPPLib   Mode = "cpp-lib"
)

// Modes lists the build modes, for messages.
var Modes = []Mode{ModeExe, ModeCArchive, ModeCShared, ModeCPPLib}

// ParseMode returns the mode that the -buildmode value s names ("" is exe).
func ParseMode(s string) (Mode, error) {
	if s == "" {
		return ModeExe, nil
	}
	for _, m := range Modes {
		if string(m) == s {
			return m, nil
		}
	}
	var names []string
	for _, m := range Modes {
		names = append(names, string(m))
	}
	return "", fmt.Errorf("unknown build mode %q (one of: %s)", s, strings.Join(names, ", "))
}

// Library reports whether the mode makes a library.
func (m Mode) Library() bool { return m == ModeCArchive || m == ModeCShared || m == ModeCPPLib }

// Archive reports whether the mode makes a static library: the objects go into an archive, not a link.
func (m Mode) Archive() bool { return m == ModeCArchive || m == ModeCPPLib }

// CompileFlags returns the flags that every compile of the mode gets, for the compiler family f; c
// says that the compiler is a C compiler (-fvisibility-inlines-hidden is for C++ only).
func (m Mode) CompileFlags(f Family, c bool) []string {
	switch m {
	case ModeCArchive, ModeCPPLib:
		return []string{"-fPIC"}
	case ModeCShared:
		flags := []string{"-fPIC", "-fvisibility=hidden"}
		if !c {
			flags = append(flags, "-fvisibility-inlines-hidden")
		}
		if f == GCC {
			flags = append(flags, "-fno-gnu-unique")
		}
		return flags
	}
	return nil
}

// LinkFlags returns the flags that the link of the mode gets before the objects: a shared library
// with its name (soname), none of the symbols of the archives it takes in exported, and, with a
// version script (the file that names the exported functions, libgen.VersionScript), no symbol
// of the library but those: not the weak symbols of the C++ library's templates either.
func (m Mode) LinkFlags(soname, script string) []string {
	if m != ModeCShared {
		return nil
	}
	flags := []string{"-shared", "-Wl,--exclude-libs,ALL"}
	if soname != "" {
		flags = append(flags, "-Wl,-soname,"+soname)
	}
	if script != "" {
		flags = append(flags, "-Wl,--version-script="+script)
	}
	return flags
}

// applyMode adds the compile flags of the mode to the compilers of the build, once. They are compiler
// arguments (CXX.Args), which every run of a compiler passes first, so every compile and the link get
// them and every key that includes the compiler has them.
func (b *Builder) applyMode() {
	if b.modeApplied || !b.Mode.Library() {
		return
	}
	b.modeApplied = true
	b.CXX.Args = slices.Concat(b.CXX.Args, b.Mode.CompileFlags(b.CXX.Family, false))
	if b.CC.Path != "" {
		b.CC.Args = slices.Concat(b.CC.Args, b.Mode.CompileFlags(b.CC.Family, true))
	}
}

// LibraryLinkFlags returns the link flags that a program needs besides a static library that was made from
// the packages pkgs: their link directives and pkg-config libraries and GOCPP_LDFLAGS, in the order that
// the link of a program has them (the packages that import the others first, a library once).
func LibraryLinkFlags(pkgs []PackageBuild, env EnvFlags) []string {
	var link []string
	for i := len(pkgs) - 1; i >= 0; i-- {
		link = appendLinkFlags(link, pkgs[i].Flags.Link)
	}
	return slices.Concat(link, env.LDFlags)
}

// archiver returns the program that makes an archive for the compiler c: gcc-ar or llvm-ar from the
// compiler's own folder or PATH when there is one (they load the compiler's LTO plugin), else ar.
func (b *Builder) archiver() string {
	names := []string{"ar"}
	switch b.CXX.Family {
	case GCC:
		names = []string{"gcc-ar", "ar"}
	case Clang:
		names = []string{"llvm-ar", "ar"}
	}
	dir := filepath.Dir(b.CXX.Path)
	for _, n := range names {
		if p := filepath.Join(dir, n); fileExists(p) {
			return p
		}
	}
	for _, n := range names {
		if p, err := exec.LookPath(n); err == nil {
			return p
		}
	}
	return "ar"
}

func fileExists(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.Mode().IsRegular()
}

// archive makes the static library out from the objects: a deterministic archive (no times, no
// owners: ar's D), with an index. The compile flags of the build are not part of an archive, the
// objects hold them. It records the archiver for the up-to-date check.
func (b *Builder) archive(ctx context.Context, objs []string, out string) error {
	ar := b.archiver()
	b.recordArchive(ar)
	b.Stats.linked()
	if err := os.Remove(out); err != nil && !os.IsNotExist(err) {
		return err
	}
	tool := CXX{Path: ar, Family: b.CXX.Family}
	_, err := b.runOutput(ctx, tool, "the archive", true, slices.Concat([]string{"rcsD", out}, objs))
	return err
}

// recordArchive notes the archiver as the program of the link in the record of the build.
func (b *Builder) recordArchive(ar string) {
	if b.Rec == nil {
		return
	}
	lr := &LinkRec{}
	if p, err := exec.LookPath(ar); err == nil {
		if fi, err := os.Stat(p); err == nil && fi.Mode().IsRegular() {
			f := linkFile("ar", p, fi, time.Now())
			lr.Linker = &f
		}
	}
	b.Rec.mu.Lock()
	b.Rec.link = lr
	b.Rec.mu.Unlock()
}

// A RuntimeHeader is a header of gocpp's runtime, with its path below include/ (gocpp/rt.hpp).
type RuntimeHeader struct {
	Path string
	Data []byte
}

// RuntimeHeaders returns the headers of the embedded runtime in the order of their paths: a cpp-lib
// installs them, because the headers of its packages include them.
func RuntimeHeaders() ([]RuntimeHeader, error) {
	var out []RuntimeHeader
	err := fs.WalkDir(rtfiles.FS(), "include", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := fs.ReadFile(rtfiles.FS(), p)
		if err != nil {
			return err
		}
		out = append(out, RuntimeHeader{Path: strings.TrimPrefix(p, "include/"), Data: data})
		return nil
	})
	return out, err
}
