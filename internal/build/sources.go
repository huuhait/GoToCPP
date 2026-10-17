package build

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"sync"
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

// maxSourceJobs is how many of a package's C and C++ files compile at the
// same time, next to the compile of the generated C++. Phase 06 brings the
// real build graph and -p.
var maxSourceJobs = max(1, min(runtime.NumCPU(), 8))

// compileSources compiles the package's C and C++ files (Dir, CFiles and
// CXXFiles) into Work/cobj, at most maxSourceJobs at the same time, and
// returns their object files, in the order of the files (the C files
// first). rtDir is the runtime's folder, for gocpp/nofortify.h. A compile
// that fails is a *CompileError with Source set: the first one, in the
// order of the files.
func (b *Builder) compileSources(ctx context.Context, rtDir string) ([]string, error) {
	type source struct {
		c    CXX
		file string
	}
	var srcs []source
	for _, f := range b.CFiles {
		srcs = append(srcs, source{b.CC, f})
	}
	for _, f := range b.CXXFiles {
		srcs = append(srcs, source{b.CXX, f})
	}
	if len(srcs) == 0 {
		return nil, nil
	}
	objDir := filepath.Join(b.Work, "cobj")
	b.trace("mkdir -p " + ShellQuoteWork(objDir, b.Work))
	if err := os.Mkdir(objDir, 0o755); err != nil {
		return nil, err
	}
	nofortify := filepath.Join(rtDir, "include", "gocpp", "nofortify.h")
	objs := make([]string, len(srcs))
	errs := make([]error, len(srcs))
	sem := make(chan struct{}, maxSourceJobs)
	var wg sync.WaitGroup
	for i, s := range srcs {
		file := filepath.Join(b.Dir, s.file)
		// a.c and a.cpp get a.c.o and a.cpp.o.
		objs[i] = filepath.Join(objDir, s.file+".o")
		what, std, lang, env := "the C++ file ", "-std=c++20", "c++", b.Env.CXXCompile()
		if s.c.C {
			what, std, lang, env = "the C file ", "-std=gnu17", "c", b.Env.CFlags
		}
		args := slices.Concat(SourceFlags(s.c, b.ASan), []string{std, "-iquote", b.Dir, "-include", nofortify},
			env, b.Package.Compile, []string{"-c", "-x", lang, file, "-o", objs[i]})
		wg.Go(func() {
			sem <- struct{}{}
			defer func() { <-sem }()
			out, err := b.compileCached(ctx, s.c, file, objs[i], args, func(args []string) ([]byte, error) {
				return b.runOutput(ctx, s.c, what+s.file, false, args)
			}, nil)
			var ce *CompileError
			if errors.As(err, &ce) {
				ce.Source = file
			}
			errs[i] = err
			if len(out) > 0 {
				b.traceMu.Lock()
				defer b.traceMu.Unlock()
				switch {
				case b.SourceWarnings != nil:
					b.SourceWarnings(out)
				case b.Stderr != nil:
					b.Stderr.Write(out)
				}
			}
		})
	}
	wg.Wait()
	for _, err := range errs {
		if err != nil {
			return nil, err
		}
	}
	return objs, nil
}
