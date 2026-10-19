package build

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

// The C and C++ sources of a package (Phase 04 README §2.9, step 4.13).
//
// Program compiles the package's .c files with the C compiler (FindCC) and
// its .cc, .cpp and .cxx files with the C++ compiler, in the work folder,
// and links their object files with the generated C++ and the runtime. Each
// compile gets, in this order:
//
//   - SourceFlags: -g and -O0 like the generated C++ (debug info, and an
//     explicit -O for the Nix wrapper), the sanitizers with -asan, and
//     -Qunused-arguments for clang;
//   - -std=gnu17 for C, -std=c++20 for C++: gnu17, not c17, so that
//     glibc declares the POSIX and GNU names that C code expects (strdup,
//     fileno, M_PI), as it does in C++ mode, where both compilers define
//     _GNU_SOURCE; and not the newest default (gcc 15 has gnu23), in which
//     bool and true are keywords and f() means f(void), which breaks old
//     C. Both compilers get the same C this way;
//   - -iquote <package folder>, so that #include "x.h" finds the headers
//     next to the file and no package folder hides a system header;
//   - -include gocpp/nofortify.h (Phase 02 README §2.7): a hardened Nix
//     compiler wrapper adds -D_FORTIFY_SOURCE after every flag, which warns
//     at -O0; the header undefines it at -O0;
//   - the trusted flags of the environment: GOCPP_CFLAGS, and for C++
//     GOCPP_CXXFLAGS after it;
//   - the package's compile flags (//gocpp:cflags, pkg-config --cflags);
//   - -c, -x c or -x c++, the file and its object file.
//
// The user's flags come after gocpp's, so they win: a package can choose
// -O2 or -std=gnu11 for its own files (a package's flags go to both
// languages, so a -std= for one of them breaks the other). gocpp's language
// flags (LanguageFlags: -fwrapv and friends) are the semantics of Go code,
// not of the user's C, and the flags of generated C++ (GeneratedFlags,
// -Wno-error) don't apply to the user's code either. -nochecks changes
// nothing here.

// sourceDebugFlags are the flags of a debug build of a package's own C
// and C++ files: debug info (with the columns, which are right for these
// files, unlike for the generated C++) and an explicit -O0.
var sourceDebugFlags = []string{"-g", "-O0"}

// SourceFlags returns the flags of every compile of a package's own C and
// C++ files with the compiler c (a C or a C++ compiler), after c.Args: the
// debug flags, the sanitizers with asan, and -Qunused-arguments for clang.
func SourceFlags(c CXX, asan bool) []string {
	flags := slices.Clone(sourceDebugFlags)
	if asan {
		flags = append(flags, asanFlags...)
	}
	if c.Family == Clang {
		flags = append(flags, "-Qunused-arguments")
	}
	return flags
}

// cxxName matches the name of a C++ compiler program and splits it into the
// parts around the name of its family: x86_64-w64-mingw32-g++,
// clang++-18, g++-13.exe.
var cxxName = regexp.MustCompile(`^(.*?)(clang\+\+|g\+\+|c\+\+)(-[0-9][0-9.]*)?((?i)\.exe)?$`)

// cNames are the C compilers that go with the C++ compilers of cxxName.
var cNames = map[string]string{"clang++": "clang", "g++": "gcc", "c++": "cc"}

// FindCC finds the C compiler for the .c files of a package, to go with the
// C++ compiler cxx: the compiler in GOCPP_CC (a program and maybe
// arguments, like GOCPP_CXX), else the C compiler of the same toolchain as
// cxx: clang for clang++, gcc for g++, x86_64-w64-mingw32-gcc for
// x86_64-w64-mingw32-g++, clang-18 for clang++-18. It is looked up next to
// cxx first, then in PATH; with a launcher (GOCPP_CXX="ccache g++"), the
// launcher gets the C compiler's name instead (ccache gcc), and the other
// arguments stay (a C++-only flag in GOCPP_CXX then reaches the C compiler
// too: GOCPP_CC is the way out). CC is not used: a Nix shell sets it to gcc
// also when GOCPP_CXX is clang++, and gocpp doesn't support mixing two
// compilers in one program. A GOCPP_CC of another family than cxx is not
// checked.
func FindCC(ctx context.Context, cxx CXX) (CXX, error) {
	if cc, ok, err := cxxFromEnv(ctx, "GOCPP_CC"); ok {
		return cc, err
	}
	words := append([]string{cxx.Path}, cxx.Args...)
	for i, w := range words {
		m := cxxName.FindStringSubmatch(filepath.Base(w))
		if m == nil {
			continue
		}
		name := m[1] + cNames[m[2]] + m[3] + m[4]
		if i > 0 {
			if strings.ContainsAny(w, `/\`) {
				name = filepath.Join(filepath.Dir(w), name) // ccache /usr/bin/g++
			}
			args := slices.Clone(cxx.Args)
			args[i-1] = name
			return lookPath(ctx, cxx.Path, args, true)
		}
		path := filepath.Join(filepath.Dir(w), name)
		if fi, err := os.Stat(path); err != nil || fi.IsDir() {
			found, err := exec.LookPath(name)
			if err != nil {
				return CXX{}, fmt.Errorf("no C compiler for the package's .c files: %s is not next to the C++ compiler %s, nor in PATH (GOCPP_CC can name the C compiler)", name, cxx.Name())
			}
			if path, err = filepath.Abs(found); err != nil {
				return CXX{}, err
			}
		}
		return lookPath(ctx, path, cxx.Args, true)
	}
	return CXX{}, fmt.Errorf("no C compiler for the package's .c files: gocpp can't tell which C compiler goes with the C++ compiler %s (GOCPP_CC can name it)", cxx.Name())
}

// planPackageSources returns the object files of the C and C++ files of the package pb (Dir,
// CFiles and CXXFiles), in the order of the files (the C files first), in Work/cobj, and the
// actions that compile them (compileCached). rtDir is the runtime's folder, for gocpp/nofortify.h.
// idx is the number of the package and multi says that the program has more than one: then the
// objects are in a folder of the package, cobj/<idx>. A compile that fails is a *CompileError with
// Source and PkgPath set: an error of the user.
func (b *Builder) planPackageSources(ctx context.Context, rtDir string, pb *PackageBuild, idx int, multi bool) (objs []string, acts []action, err error) {
	type source struct {
		c    CXX
		file string
	}
	var srcs []source
	for _, f := range pb.CFiles {
		srcs = append(srcs, source{b.CC, f})
	}
	for _, f := range pb.CXXFiles {
		srcs = append(srcs, source{b.CXX, f})
	}
	if len(srcs) == 0 {
		return nil, nil, nil
	}
	objDir := filepath.Join(b.Work, "cobj")
	if multi {
		objDir = filepath.Join(objDir, fmt.Sprint(idx))
	}
	b.trace("mkdir -p " + ShellQuoteWork(objDir, b.Work))
	if err := os.MkdirAll(objDir, 0o755); err != nil {
		return nil, nil, err
	}
	nofortify := filepath.Join(rtDir, "include", "gocpp", "nofortify.h")
	objs = make([]string, len(srcs))
	for i, s := range srcs {
		file := filepath.Join(pb.Dir, s.file)
		// a.c and a.cpp get a.c.o and a.cpp.o.
		objs[i] = filepath.Join(objDir, safeObjName(s.file, i))
		what, std, lang, env := "the C++ file ", "-std=c++20", "c++", b.Env.CXXCompile()
		if s.c.C {
			what, std, lang, env = "the C file ", "-std=gnu17", "c", b.Env.CFlags
		}
		args := slices.Concat(SourceFlags(s.c, b.ASan), []string{std, "-iquote", pb.Dir, "-include", nofortify},
			env, pb.Flags.Compile, pb.Public, []string{"-c", "-x", lang, file, "-o", objs[i]})
		cmd := Command{Kind: KindSource, Pkg: pb.Path, Lang: lang, Compiler: compiler(s.c), Args: args, Dir: pb.Dir, File: file, Output: objs[i],
			IR: "src/" + pb.Path + "/" + s.file}
		b.record(cmd)
		acts = append(acts, action{name: file, run: func() (func(), error) {
			out, err := b.compileCached(ctx, s.c, file, objs[i], args, func(args []string) ([]byte, error) {
				return b.runOutput(ctx, s.c, what+s.file, false, args)
			}, nil)
			if err == nil {
				err = b.emitIR(ctx, s.c, what+s.file, cmd)
			}
			var ce *CompileError
			if errors.As(err, &ce) {
				ce.Source, ce.PkgPath = file, pb.Path
			}
			return func() {
				if len(out) == 0 {
					return
				}
				b.traceMu.Lock()
				defer b.traceMu.Unlock()
				switch {
				case pb.SourceWarnings != nil:
					pb.SourceWarnings(out)
				case b.Stderr != nil:
					b.Stderr.Write(out)
				}
			}, err
		}})
	}
	return objs, acts, nil
}

// safeObjName returns the file name of the object of the source file name file (the i-th of its package): file with ".o",
// or, when the compiler cannot be handed that name (a source 日本.cpp on a Windows host: the object is an argument),
// an ASCII name with the same extension.
func safeObjName(file string, i int) string {
	if name := file + ".o"; CompilerSafe(name) {
		return name
	}
	ext := filepath.Ext(file)
	if !CompilerSafe(ext) {
		ext = ""
	}
	return fmt.Sprintf("gpsrc%d%s.o", i, ext)
}
