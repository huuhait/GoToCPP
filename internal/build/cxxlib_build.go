package build

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
)

// Building the C and C++ libraries of the modules (Phase 06 step 6.1d,
// cxxlib.go).
//
// Each source file of each library (PackageFlags.Libs) is compiled to an
// object file through the object cache, and the objects are linked into the
// program. A compile gets, in this order:
//
//   - SourceFlags (-g -O0, the sanitizers with -asan, -Qunused-arguments for
//     clang) and the standard of the recipe: -std=gnu17 (cstd) for C,
//     -std=c++20 (std; or the `cxx` line of the library's gocpp.mod) for C++;
//   - -w: the warnings of a library are not the user's, and a compile that
//     printed something is never kept in the cache, so a library with a
//     warning would compile again in every build;
//   - -iquote <the module's folder>, as the packages that use the library
//     have it;
//   - -include gocpp/nofortify.h, as for a package's own sources;
//   - the trusted flags of the environment, GOCPP_CFLAGS (and GOCPP_CXXFLAGS
//     for C++): choices for the whole program (-stdlib, sanitizers);
//   - the public flags of the recipe (-I, -isystem, -D, pkg-config), the same
//     that the packages that use the library get;
//   - the private flags of the recipe (private-define, cflags, and cxxflags for
//     C++ files);
//   - -c, -x c or -x c++, the file and its object file.
//
// The flags of the package that uses the library (//gocpp:cflags) are not
// part of it: a library is compiled once for (module, version, recipe,
// compiler, flags) and every project reuses the object. The key of the cache
// has the text of the source, all the flags, the compiler and the module's
// path and version (compileCachedSalt); the headers it read are checked by
// the dependency file.
//
// The compiles run at the same time, at most maxLibJobs of them, and an error
// is the first one in the order of the libraries and their files. An error is
// a *CompileError with Lib set: an error of the user's build (the recipe or
// the library), not a bug of gocpp.

// planLibs returns the object files of the sources of the libraries libs in Work/libobj, in the
// order of the libraries and of their files (sorted by name), and the actions that compile them.
func (b *Builder) planLibs(ctx context.Context, rtDir string, libs []*CxxLib) (objs []string, acts []action, err error) {
	type job struct {
		lib *CxxLib
		src LibSource
		c   CXX
		obj string
	}
	var jobs []job
	for li, lib := range libs {
		srcs, errs := lib.Sources()
		if len(errs) > 0 {
			return nil, nil, errors.New(errs[0].Pos.Filename + ":" + fmt.Sprint(errs[0].Pos.Line) + ": " + errs[0].Msg)
		}
		for si, s := range srcs {
			c := b.CXX
			if s.C {
				if b.CC.Path == "" {
					return nil, nil, fmt.Errorf("cxx %s: no C compiler for %s", lib.Module, s.Rel)
				}
				c = b.CC
			}
			// 003-b.c.o: the library, the file and its name; the folder of the file is not in it.
			objName := fmt.Sprintf("%d-%d-%s.o", li, si, filepath.Base(s.Rel))
			if !CompilerSafe(objName) {
				objName = fmt.Sprintf("%d-%d%s.o", li, si, filepath.Ext(s.Rel)) // the compiler cannot be handed the file's name (safeargs.go)
			}
			obj := filepath.Join(b.Work, "libobj", objName)
			jobs = append(jobs, job{lib, s, c, obj})
		}
	}
	if len(jobs) == 0 {
		return nil, nil, nil
	}
	objDir := filepath.Join(b.Work, "libobj")
	b.trace("mkdir -p " + ShellQuoteWork(objDir, b.Work))
	if err := os.Mkdir(objDir, 0o755); err != nil {
		return nil, nil, err
	}
	objs = make([]string, len(jobs))
	for i, j := range jobs {
		objs[i] = j.obj
		std, lang, env := "-std="+j.lib.Std, "c++", b.Env.CXXCompile()
		private := slices.Concat(j.lib.Private, j.lib.PrivateCXX)
		what := "the C++ file "
		if j.src.C {
			std, lang, env, private, what = "-std="+j.lib.CStd, "c", b.Env.CFlags, j.lib.Private, "the C file "
		}
		args := slices.Concat(SourceFlags(j.c, b.ASan), []string{std, "-w", "-iquote", j.lib.Dir}, nofortifyFlags(j.c, rtDir),
			env, j.lib.Public, private, []string{"-c", "-x", lang, j.src.File, "-o", j.obj})
		cmd := Command{Kind: KindLibrary, Lang: lang, Compiler: compiler(j.c), Args: args, Dir: j.lib.Dir, File: j.src.File, Output: j.obj,
			IR: "lib/" + j.lib.Module + "/" + filepath.ToSlash(j.src.Rel)}
		b.record(cmd)
		acts = append(acts, action{name: j.src.File, run: func() (func(), error) {
			out, err := b.compileCachedSalt(ctx, j.c, j.src.File, j.obj, args, func(args []string) ([]byte, error) {
				return b.runOutput(ctx, j.c, what+j.src.Rel, false, args)
			}, nil, []string{"cxx", j.lib.Module, j.lib.Version})
			if err == nil {
				err = b.emitIR(ctx, j.c, what+j.src.Rel, cmd)
			}
			var ce *CompileError
			if errors.As(err, &ce) {
				ce.Source, ce.Lib, ce.LibLine, ce.LibText = j.src.File, j.lib, j.src.Line, j.src.Text
			}
			return func() {
				if len(out) > 0 && b.Stderr != nil {
					b.traceMu.Lock()
					defer b.traceMu.Unlock()
					b.Stderr.Write(out)
				}
			}, err
		}})
	}
	return objs, acts, nil
}

// compileLibsOf compiles the sources of the libraries libs (planLibs) and returns their object
// files, or the first error in the order of the sources.
func (b *Builder) compileLibsOf(ctx context.Context, rtDir string, libs []*CxxLib) ([]string, error) {
	objs, acts, err := b.planLibs(ctx, rtDir, libs)
	if err != nil {
		return nil, err
	}
	errs, _ := b.runActions(ctx, acts)
	for _, err := range errs {
		if err != nil {
			return nil, err
		}
	}
	return objs, nil
}
