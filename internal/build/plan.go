package build

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
)

// The plan of a build: the commands that it runs or would run (Phase 06 steps 6.11 and 6.12,
// ADR-044).
//
// A Builder with a Plan records every compiler command of a build and its link, whether the
// compile ran, was answered by the object cache, or (DryRun) was only listed. The record is what
// compile_commands.json (compdb.go) and gocpp export cmake (cmake.go) are made from, and what
// gocpp build -n prints. DryRun runs nothing: the commands are shown (Trace) and recorded; the
// object cache, the precompiled header and the link are skipped.

// A CommandKind says what a Command is for.
type CommandKind string

const (
	// KindUnit is the compile of a translation unit that gocpp generated.
	KindUnit CommandKind = "unit"
	// KindSource is the compile of a package's own C or C++ file.
	KindSource CommandKind = "source"
	// KindLibrary is the compile of a source of a library of a module (a cxx recipe).
	KindLibrary CommandKind = "library"
	// KindRuntime is the compile of a file of gocpp's runtime.
	KindRuntime CommandKind = "runtime"
	// KindLink is the link of the program.
	KindLink CommandKind = "link"
)

// A Command is one compiler or linker run of a build.
type Command struct {
	Kind CommandKind

	// Pkg is the import path of the package that the unit or the source belongs to; it is "" for
	// the runtime and for libraries.
	Pkg string

	// Lang is "c" or "c++" for a compile.
	Lang string

	// Compiler is the program and the arguments that come with it (CXX.Path, CXX.Args); Args are
	// the arguments after them, as the build gives them.
	Compiler []string
	Args     []string

	// Dir is a stable folder for the command: the folder of the generated files, the folder of the
	// package, of the library or of the runtime. (The compiles run in a work folder that is removed
	// at the end, so it can't be the directory of a compile_commands.json entry.)
	Dir string

	// File is the source of a compile (absolute) and Output its object file (in the work folder).
	File, Output string

	// IR is the name of the LLVM IR file of the compile under -emit-llvm, relative to the folder of
	// the IR files, with slashes and without the extension; "" for a command that has none.
	IR string

	// For the link: the flags before the object files (the build's flags and --gc-sections), the
	// object files, and the flags after them (libraries, the packages' and the user's link flags).
	Pre, Objs, Post []string
}

// CompileArgs returns the arguments of a compile without its -o and the object file.
func (c Command) CompileArgs() []string {
	out := make([]string, 0, len(c.Args))
	for i := 0; i < len(c.Args); i++ {
		if c.Args[i] == "-o" && i+1 < len(c.Args) {
			i++
			continue
		}
		out = append(out, c.Args[i])
	}
	return out
}

// A Plan collects the commands of a build (Builder.Plan).
type Plan struct {
	mu sync.Mutex

	// Compiles are the compiles, in the order in which they were planned.
	Compiles []Command

	// Link is the link, or nil before it.
	Link *Command

	// GenRoot is the folder of the generated files and RuntimeDir the folder of the runtime, which
	// the commands name in their arguments.
	GenRoot, RuntimeDir string

	// CXX and CC are the compilers of the build (CC is zero without C files).
	CXX, CC CXX

	// GOOS is the operating system of the build.
	GOOS string
}

func (p *Plan) add(c Command) {
	p.mu.Lock()
	p.Compiles = append(p.Compiles, c)
	p.mu.Unlock()
}

// setRuntime tells the plan the folder of the runtime (nil-safe).
func (p *Plan) setRuntime(dir string) {
	if p == nil {
		return
	}
	p.mu.Lock()
	p.RuntimeDir = dir
	p.mu.Unlock()
}

// rtNoTSan are the runtime sources that a -race build (ThreadSanitizer) compiles without it
// (Phase 08 step 8.12): the race reporter and the code that it uses to name the frames of a report
// (the program's file, its symbols and its line table). The reporter is a process of its own (a copy
// of the program), where ThreadSanitizer's instrumentation does not matter for what the program
// does, but it matters for how long the program waits for the reporter at its start: reading a 7 MB
// file is thousands of function calls and tens of megabytes of memory traffic, ten times as slow
// under ThreadSanitizer. The same files also run in the program for the trace of a panic.
var rtNoTSan = map[string]bool{
	"src/trace.cpp":          true,
	"src/frames.cpp":         true,
	"src/os/linux/trace.cpp": true,
	"src/os/linux/race.cpp":  true,
}

// runtimeArgs returns the arguments of the compile of the runtime source rel (a path of
// rtfiles.Sources) in the runtime folder dir with the flags flags, and the object file obj.
func runtimeArgs(dir, rel string, flags []string, obj string) []string {
	if rtNoTSan[rel] && slices.Contains(flags, "-fsanitize=thread") {
		flags = append(slices.Clone(flags), "-fno-sanitize=thread")
	}
	return slices.Concat(flags, []string{"-I", filepath.Join(dir, "include"), "-c", filepath.Join(dir, filepath.FromSlash(rel)), "-o", obj})
}

// recordRuntime records the compiles of the runtime's sources srcs (they are compiled once into
// the cache, so the build may not run them, but compile_commands.json and the CMake export need
// them).
func (b *Builder) recordRuntime(dir string, srcs, flags []string) {
	if b.Plan == nil {
		return
	}
	for _, s := range srcs {
		b.record(Command{Kind: KindRuntime, Lang: "c++", Compiler: compiler(b.CXX), Args: runtimeArgs(dir, s, flags, ""),
			Dir: dir, File: filepath.Join(dir, filepath.FromSlash(s))})
	}
}

// record adds the compile c to the plan of the builder, if it has one.
func (b *Builder) record(c Command) {
	if b.Plan == nil {
		return
	}
	b.Plan.add(c)
}

// compiler returns the words of the compiler cx for a Command.
func compiler(cx CXX) []string {
	return append([]string{cx.Path}, cx.Args...)
}

// ---- LLVM IR (-emit-llvm)

// irArgs returns the arguments of the compile that writes the LLVM IR of a translation unit: those
// of the compile args with -c turned into -S -emit-llvm (text, .ll) or -c -emit-llvm (bitcode,
// .bc), and the output of the IR file.
func irArgs(args []string, bitcode bool, ir string) []string {
	out := make([]string, 0, len(args)+3)
	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "-o" && i+1 < len(args):
			i++
		case args[i] == "-c":
			if bitcode {
				out = append(out, "-c", "-emit-llvm")
			} else {
				out = append(out, "-S", "-emit-llvm")
			}
		default:
			out = append(out, args[i])
		}
	}
	return append(out, "-o", ir)
}

// irPath returns the IR file of the command c: the name c.IR in the folder LLVMDir, with the
// extension of the format.
func (b *Builder) irPath(c Command) string {
	ext := ".ll"
	if b.EmitLLVM == "bc" {
		ext = ".bc"
	}
	return filepath.Join(b.LLVMDir, filepath.FromSlash(c.IR)+ext)
}

// emitIR writes the LLVM IR of the translation unit of the compile c with the compiler cx, when
// the build was asked for it (EmitLLVM), after the compile that makes the object. The IR comes
// from a second run of the compiler with the same flags, so a plugin that the flags load runs in
// both. It returns a *CompileError like a compile.
func (b *Builder) emitIR(ctx context.Context, cx CXX, what string, c Command) error {
	if b.EmitLLVM == "" || c.IR == "" {
		return nil
	}
	file := b.irPath(c)
	if !b.DryRun {
		if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
			return err
		}
	}
	out, err := b.runOutput(ctx, cx, what, false, irArgs(c.Args, b.EmitLLVM == "bc", file))
	_ = out // the warnings of the compile that made the object were shown already
	return err
}

// irName returns the IR name (Command.IR) of a generated unit: its path in the folder of the
// generated files without .cpp (gpkg/example.com/m/a/a.gp), or the base name outside it.
func (b *Builder) irName(src string) string {
	rel := filepath.Base(src)
	if b.GenRoot != "" {
		if r, err := filepath.Rel(b.GenRoot, src); err == nil && !strings.HasPrefix(r, "..") {
			rel = r
		}
	}
	return "gen/" + strings.TrimSuffix(filepath.ToSlash(rel), ".cpp")
}

// ---- the markers of an IR folder

// IRMarker is the file that tells a folder of IR files that gocpp build -emit-llvm made: the
// build empties only such a folder before it writes the new files.
const IRMarker = ".gocpp-llvm"

// PrepareIRDir makes the folder of the IR files empty and marks it. A folder that exists and is not
// one of gocpp's (no IRMarker, not empty) is an error: gocpp does not remove what it did not make.
func PrepareIRDir(dir string) error {
	entries, err := os.ReadDir(dir)
	switch {
	case err == nil:
		if len(entries) > 0 {
			if _, err := os.Stat(filepath.Join(dir, IRMarker)); err != nil {
				return fmt.Errorf("%s exists and is not a folder of LLVM IR files that gocpp made (it has no %s file): remove it, or name the program with -o", dir, IRMarker)
			}
			if err := os.RemoveAll(dir); err != nil {
				return err
			}
		}
	case !os.IsNotExist(err):
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, IRMarker), []byte("LLVM IR files of gocpp build -emit-llvm: gocpp empties this folder before the next build.\n"), 0o644)
}

// sortedCompiles returns the compiles of the plan sorted by file, then by kind, without a second
// command for a file that has the same arguments, so the files made from it are the same whatever
// the order in which the actions were planned.
func (p *Plan) sortedCompiles() []Command {
	p.mu.Lock()
	defer p.mu.Unlock()
	list := slices.Clone(p.Compiles)
	slices.SortStableFunc(list, func(a, b Command) int {
		if c := strings.Compare(a.File, b.File); c != 0 {
			return c
		}
		return strings.Compare(string(a.Kind), string(b.Kind))
	})
	return slices.CompactFunc(list, func(a, b Command) bool {
		return a.File == b.File && a.Kind == b.Kind && slices.Equal(a.CompileArgs(), b.CompileArgs())
	})
}
