package build

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"
)

// The precompiled header of the runtime. Parsing gocpp/rt.hpp and the standard headers it
// includes is nearly the whole cost of compiling a small program (about 1 s of 1.05 s for
// hello.gp with clang++ -O0), so the cache keeps rt.hpp precompiled, next to the runtime's
// object files: <runtime folder>/pch/<key>/rt.hpp.pch (clang++) or rt.hpp.gch (g++), where
// the key is the runtime key (runtimeKey) of the flags of a generated compile.
//
// Only a plain compile of the generated C++ uses it: without flags of the user or the
// environment, which could change what the header means (-D, -std, -include), and with a cache
// folder. The first build with a new key does not wait for it: it builds the header while the
// program compiles and uses it from the next build on. Generated code starts with
// #include <gocpp/rt.hpp>, which a PCH makes a no-op (#pragma once), so the program means the
// same with and without it.
//
// A header that the compiler refuses (the runtime's files changed under it) is removed and the
// program compiles again without it; g++ ignores an unusable .gch by itself.
//
// g++ writes a header in place and does not check it beyond a few bytes: a .gch that is
// being written, or was written by two compilers at once, makes cc1plus crash (SIGSEGV in
// c_common_read_pch, "internal compiler error: Segmentation fault"). That happened when
// several Builders of one process (TestProgramParallel) built the same header at the same
// time: the temporary name had only the pid, so they wrote one file, and the first to finish
// renamed it into place while the others still wrote into it. (A cc1plus that has such a file
// mapped crashes at once: reproduced by hand with four g++ -c -x c++-header -o same.gch started
// together and compiles that start when the first has finished.) Each build now has a file of
// its own, and one build of a header at a time runs in a process. If a compiler still dies
// with a header (a disk that lost a write, a g++ bug), the compile runs again without it, the
// header is removed (compilerDied).

// pchFlags are the flags of the precompiled header: those of a plain compile of the generated
// C++ (Program), without the package's -iquote, which the compilers don't check.
func (b *Builder) pchFlags() []string {
	return slices.Concat(CompileFlags(b.CXX, b.ASan, b.NoChecks), GeneratedFlags(b.CXX))
}

// pchBuilds are the headers that this process is building (by file name).
var pchBuilds sync.Map

// A pch is the precompiled header for one set of flags.
type pch struct {
	b      *Builder
	rtDir  string
	header string // the file that includes gocpp/rt.hpp, in the key's folder
	file   string // header + ".pch" or ".gch"
}

// precompiled returns the precompiled header of the runtime in rtDir for a plain compile, or
// nil when there is none to use: no cache folder, a compiler of another family, flags of the
// user or the environment, or a response file or config file in the compiler's arguments (its
// content changes what the header means, and the key has only its name).
func (b *Builder) precompiled(rtDir string) *pch {
	if b.Cache == "" || !strings.HasPrefix(rtDir, b.Cache) || os.Getenv("GOCPP_NOPCH") != "" ||
		len(b.Env.CXXCompile()) > 0 || len(b.Package.Compile) > 0 || slices.ContainsFunc(b.CXX.Args, readsDriverFile) {
		return nil
	}
	var ext string
	switch b.CXX.Family {
	case Clang:
		ext = ".pch"
	case GCC:
		ext = ".gch"
	default:
		return nil
	}
	dir := filepath.Join(rtDir, "pch", b.runtimeKey([]string{"rt.hpp", "deps2"}, b.pchFlags()))
	header := filepath.Join(dir, "rt.hpp")
	return &pch{b: b, rtDir: rtDir, header: header, file: header + ext}
}

// args are the flags that make a compile use the header: before -c.
func (p *pch) args() []string {
	if p.b.CXX.Family == Clang {
		return []string{"-include-pch", p.file}
	}
	return []string{"-include", p.header} // g++ finds rt.hpp.gch next to it, else reads the text
}

// ready reports whether the precompiled header exists with its list of files (saveDeps): a
// header without the list (a process that removed or replaced them between its two steps) is
// rebuilt, not used, because a compile with it could never be cached.
func (p *pch) ready() bool {
	for _, f := range []string{p.file, p.depfile()} {
		if fi, err := os.Stat(f); err != nil || !fi.Mode().IsRegular() {
			return false
		}
	}
	return true
}

// build compiles the header, atomically: the compiler writes a temporary file, which is
// renamed into place, so that gocpp commands at the same time never see half a header. The
// header is a failure of the cache only: it returns no error, and the next build tries again.
func (p *pch) build(ctx context.Context) {
	// One build of a header at a time in this process: the others would only repeat the work.
	// (The temporary file's own name is what keeps builds of other processes apart.)
	if _, busy := pchBuilds.LoadOrStore(p.file, struct{}{}); busy {
		return
	}
	defer pchBuilds.Delete(p.file)
	p.buildOnce(ctx)
}

// buildOnce is build without the guard of this process: it must be safe to run at the same
// time as itself, in this or in another process (every run has files of its own).
func (p *pch) buildOnce(ctx context.Context) {
	dir := filepath.Dir(p.header)
	if os.MkdirAll(dir, 0o755) != nil {
		return
	}
	removeStale(dir)
	// The header file has the same text for everyone: it is written once, and never changes
	// while a PCH that was built from it exists (clang checks its size and time).
	if _, err := os.Stat(p.header); err != nil {
		tmp, err := os.CreateTemp(dir, ".tmp-hdr-")
		if err != nil {
			return
		}
		_, werr := fmt.Fprintln(tmp, "#include <gocpp/rt.hpp>")
		cerr := tmp.Close()
		if werr != nil || cerr != nil || os.Rename(tmp.Name(), p.header) != nil {
			os.Remove(tmp.Name())
			return
		}
	}
	// A name of its own for each build: builds in one process (several Builders) or in
	// containers that share the cache (equal pids) must never write the same file. See above.
	tf, err := os.CreateTemp(dir, ".tmp-pch-"+filepath.Base(p.file)+"-")
	if err != nil {
		return
	}
	tf.Close()
	out := tf.Name()
	depfile := out + ".d"
	list := out + ".deps"
	defer os.Remove(out)
	defer os.Remove(depfile)
	defer os.Remove(list)
	// The files the header is made from, for the object cache (objcache.go): g++ does not
	// list them for a compile that uses the header. They are saved with their hashes next to
	// it, as they are now, so that the cache sees what the header holds even if a system
	// header is changed in place later.
	args := slices.Concat(p.b.pchFlags(), []string{"-I", filepath.Join(p.rtDir, "include"), "-c", "-x", "c++-header", p.header, "-o", out, "-MD", "-MF", depfile})
	start := time.Now()
	if _, err := p.b.runOutput(ctx, p.b.CXX, "the precompiled header", false, args); err != nil && !errors.Is(err, errOutputOpen) {
		return
	}
	if !p.saveDeps(depfile, list, start) {
		return
	}
	// The list first, so that a header that exists has its list.
	if os.Rename(list, p.depfile()) != nil {
		return
	}
	os.Rename(out, p.file)
}

// saveDeps writes the list of files that the dependency file depfile names, with their hashes,
// to list. It is false (and the header is not kept) when a file can't be read or changed a
// moment before the compile started (start); a file that uses __DATE__ or the like makes the
// list say that nothing compiled with the header may be cached.
func (p *pch) saveDeps(depfile, list string, start time.Time) bool {
	paths, ok := p.b.depPaths([]string{depfile}, "")
	if !ok {
		return false
	}
	runtimes := filepath.Join(p.b.Cache, "rt") + string(filepath.Separator)
	var pd pchDeps
	for _, path := range paths {
		rec, _, err := recordFile(path, start, strings.HasPrefix(path, runtimes))
		if errors.Is(err, errTimeMacro) {
			pd = pchDeps{Uncacheable: true}
			break
		}
		if err != nil {
			return false
		}
		pd.Deps = append(pd.Deps, rec)
	}
	data, err := json.Marshal(pd)
	return err == nil && os.WriteFile(list, data, 0o600) == nil
}

// depfile is the file that lists what the header was made from (saveDeps).
func (p *pch) depfile() string { return p.file + ".deps" }

// remove deletes a header that the compiler refused. The list goes first, the reverse of the
// order that build creates them in, so that a header that exists without its list is never
// seen as ready (ready).
func (p *pch) remove() {
	os.Remove(p.depfile())
	os.Remove(p.file)
}

// pchRefused reports whether the output of a failed compile that used the precompiled header
// file may say that the header was the problem: it names a precompiled header in one of the
// spellings of the compilers (clang: "file ... has been modified since the precompiled header
// ... was built", "PCH file built from a different branch", "definition of macro 'X' differs
// between the precompiled file '...' and the command line") or the header's file. The caller
// compiles again without the header, and drops it only if that works: a real error of the
// program doesn't cost a good header.
func pchRefused(out []byte, file string) bool {
	for _, s := range []string{"precompiled header", "precompiled file", "PCH file", "-include-pch"} {
		if bytes.Contains(out, []byte(s)) {
			return true
		}
	}
	return file != "" && bytes.Contains(out, []byte(file))
}

// compilerDied reports whether a failed compile was the compiler itself dying, not a message
// about the program: a signal (an ICE of g++ prints "internal compiler error" and exits with
// status 4 while cc1plus died of a signal, a killed compiler prints "signal terminated
// program"; clang prints "PLEASE submit a bug report"), or the driver killed by a signal.
//
// It is a heuristic. A compiler that the OOM killer ended counts as dead: the compile runs
// again without the header, which works, and the good header is removed and rebuilt once. A
// program's own error that contains the text "internal compiler error" costs one more compile
// and keeps the header (it is removed only when the compile without it works).
func compilerDied(ce *CompileError) bool {
	var exit *exec.ExitError
	if errors.As(ce.Err, &exit) {
		if ws, ok := exit.Sys().(interface{ Signaled() bool }); ok && ws.Signaled() {
			return true
		}
	}
	for _, s := range []string{"internal compiler error", "signal terminated program", "PLEASE submit a bug report", "clang frontend command failed"} {
		if bytes.Contains(ce.Output, []byte(s)) {
			return true
		}
	}
	return false
}

// startPCH starts building the precompiled header for the runtime in rtDir, unless it exists
// or can't be used, so that it is built while the runtime's object files compile. waitPCH
// waits for it.
func (b *Builder) startPCH(ctx context.Context, rtDir string) {
	pc := b.precompiled(rtDir)
	if pc == nil || pc.ready() || b.pchDone != nil {
		return
	}
	done := make(chan struct{})
	b.pchDone = done
	go func() {
		defer close(done)
		pc.build(ctx)
	}()
}

// waitPCH waits for the header that startPCH started.
func (b *Builder) waitPCH() {
	if b.pchDone != nil {
		<-b.pchDone
		b.pchDone = nil
	}
}
