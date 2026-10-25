package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"

	"github.com/huuhait/gocpp/internal/build"
	"github.com/huuhait/gocpp/internal/check"
	"github.com/huuhait/gocpp/internal/cppgen"
	"github.com/huuhait/gocpp/internal/load"
	rtfiles "github.com/huuhait/gocpp/runtime"
)

// The GC engine's walking skeleton (phase-16 README §1.3, §2.2, §2.3 rules 1-2 and 7, §2.4,
// §2.7; job 16.1c).
//
// The switch is the hidden flag -gc-engine, which gocpp build and gocpp run accept and gocpp
// help does not show: gcEngineSwitch removes it from the arguments before the FlagSet parses
// them (build.go), so it is in no help text and in no flag list. Without it, every command is
// exactly the command it was: the ARC engine's pipeline below is untouched, its C++ is
// unchanged, and its programs link no gp::gc symbol.
//
// With it, the command takes this separate, small path instead of internal/load, internal/
// check and internal/lower:
//
//	internal/load.LoadGoDir   the .go files of a folder, with the stock go/parser
//	internal/check.CheckGo    go/types, Go's sizes, imports refused
//	internal/cppgen.EmitGo    the C++ of the subset, through internal/cppir
//	here                      the runtime's gp::gc sources, one compile, and the run
//
// A separate path, not an engine threaded through internal/lower: that is the multi-day
// refactor of steps 16.7/16.8, and it is reported as a design question for the ADR of 16.2.
// This path is throwaway: steps 16.7 and 16.8 replace each of its stages with the real one.

// gcEngineSwitch removes the hidden -gc-engine flag from args and reports whether it was given.
// It accepts -gc-engine, --gc-engine, and =true/=false forms. flags is the command's FlagSet,
// already registered: the scan uses it to know which flags take a separate value, so a value
// that does not start with "-" (gocpp build -o out -gc-engine dir) is copied with its flag and
// is not mistaken for the program. The scan stops at the first positional argument and at "--":
// only the flags before the program belong to gocpp, so a program that takes -gc-engine as its
// own argument still gets it (gocpp run main.gp -gc-engine runs the ARC program, unchanged).
func gcEngineSwitch(args []string, flags *flag.FlagSet) (rest []string, on bool) {
	rest = make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" || !strings.HasPrefix(a, "-") || a == "-" {
			// The program and its arguments, unchanged.
			rest = append(rest, args[i:]...)
			break
		}
		switch a {
		case "-gc-engine", "--gc-engine", "-gc-engine=true", "--gc-engine=true":
			on = true
			continue
		case "-gc-engine=false", "--gc-engine=false":
			on = false
			continue
		}
		rest = append(rest, a)
		// A flag that takes a separate value gets its value copied too, whatever the value looks
		// like: -o out, -p 4, -tags foo. A bool flag (and a flag with =value, and an unknown
		// flag, which the FlagSet reports) takes none.
		if strings.Contains(a, "=") {
			continue
		}
		name := strings.TrimLeft(a, "-")
		fl := flags.Lookup(name)
		if fl == nil {
			continue
		}
		if bf, ok := fl.Value.(interface{ IsBoolFlag() bool }); ok && bf.IsBoolFlag() {
			continue
		}
		if i+1 < len(args) {
			i++
			rest = append(rest, args[i])
		}
	}
	return rest, on
}

// gcPatternDir returns the absolute folder that the GC engine builds from the arguments of
// gocpp build or run: the current folder without one, else the one folder named.
func gcPatternDir(patterns []string) (string, error) {
	switch len(patterns) {
	case 0:
		return filepath.Abs(".")
	case 1:
		return filepath.Abs(patterns[0])
	}
	return "", fmt.Errorf("the GC engine builds one folder at a time (job 16.1c), not %d", len(patterns))
}

// gcError prints err the way gocpp reports errors and returns the exit status for a program
// error.
func (d *driver) gcError(err error) int {
	fmt.Fprintf(d.stderr, "gocpp %s: %v\n", d.name, err)
	return exitErrors
}

// gcRemoveWork removes the work folder of a GC build, unless -work asked to keep it. Only a
// folder that makeWorkFolder made is ever passed.
func (d *driver) gcRemoveWork(work string) {
	if work == "" || d.flags.work {
		return
	}
	os.RemoveAll(work)
}

// gcBuild is gocpp build -gc-engine: it builds the folder's package main to the output path
// (-o, or the folder's name in the current folder, as go build names it).
func (d *driver) gcBuild(patterns []string) int {
	dir, err := gcPatternDir(patterns)
	if err != nil {
		return d.gcError(err)
	}
	out := d.outputPath(filepath.Base(dir))
	exe, work, code := d.gcCompileDir(dir, out)
	if exe == "" {
		return code
	}
	d.gcRemoveWork(work)
	return 0
}

// gcRun is gocpp run -gc-engine: it builds the folder's package main and runs it with args.
func (d *driver) gcRun(patterns, args []string) int {
	dir, err := gcPatternDir(patterns)
	if err != nil {
		return d.gcError(err)
	}
	exe, work, code := d.gcCompileDir(dir, "")
	if exe == "" {
		return code
	}
	defer d.gcRemoveWork(work)
	if d.flags.x {
		words := []string{build.ShellQuoteWork(exe, work)}
		for _, a := range args {
			words = append(words, build.ShellQuoteWork(a, ""))
		}
		fmt.Fprintln(d.stderr, strings.Join(words, " "))
	}
	cmd := exec.CommandContext(d.ctx, exe, args...)
	cmd.Dir = d.check.dir
	cmd.Env = runProgramEnv(cmd.Dir, runtime.GOOS) // the user's TEMP and LOCALAPPDATA, not the compilers' (envpaths.go)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = d.stdin, d.stdout, d.stderr
	d.mu.Lock()
	d.running = cmd
	d.mu.Unlock()
	err = cmd.Run()
	d.mu.Lock()
	d.running = nil
	d.mu.Unlock()
	if err != nil {
		if sig := d.interrupted(); sig != nil {
			return stopStatus(sig)
		}
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) || cmd.ProcessState == nil {
			fmt.Fprintf(d.stderr, "gocpp run: %v\n", err)
			return exitErrors
		}
		if ws, ok := cmd.ProcessState.Sys().(syscall.WaitStatus); ok && ws.Signaled() {
			return stopStatus(ws.Signal())
		}
		return cmd.ProcessState.ExitCode()
	}
	return 0
}

// gcCompileDir loads, checks and emits the folder dir's package main and compiles it with the
// GC engine's runtime sources. out is the program's path, or "" to put it in the work folder
// (gocpp run). It returns the program's path and the work folder, or "" and the exit status.
func (d *driver) gcCompileDir(dir, out string) (exe, work string, code int) {
	if !isHostTarget(d.check.goos, d.check.goarch) {
		return "", "", d.gcError(fmt.Errorf("the GC engine does not cross-compile yet (job 16.1c): %s/%s, but gocpp runs on %s/%s",
			d.check.goos, d.check.goarch, runtime.GOOS, runtime.GOARCH))
	}
	gpkg, err := load.LoadGoDir(dir)
	if err != nil {
		return "", "", d.gcError(err)
	}
	pkg, info, err := check.CheckGo(gpkg.Fset, gpkg.Files)
	if err != nil {
		return "", "", d.gcError(err)
	}
	work, err = makeWorkFolder()
	if err != nil {
		return "", "", d.gcError(err)
	}
	// folder is the work folder for the cleanup: the named result work is set to "" by the
	// error returns below, and the defer must still remove the folder.
	folder := work
	keep := false
	defer func() {
		if !keep {
			d.gcRemoveWork(folder)
		}
	}()
	if d.flags.x || d.flags.work {
		fmt.Fprintf(d.stderr, "WORK=%s\n", work)
	}
	genDir := filepath.Join(work, "gopkg_main")
	if err := os.MkdirAll(genDir, 0o755); err != nil {
		return "", "", d.gcError(err)
	}
	genFile := filepath.Join(genDir, "main.cpp")
	data, err := cppgen.EmitGo(gpkg.Fset, pkg, info, gpkg.Files, cppgen.GCConfig{OutputPath: genFile})
	if err != nil {
		return "", "", d.gcError(err)
	}
	if err := os.WriteFile(genFile, data, 0o644); err != nil {
		return "", "", d.gcError(err)
	}
	rt, err := d.gcRuntime(work)
	if err != nil {
		return "", "", d.gcError(err)
	}
	cxx, err := build.FindCXXFor(d.ctx, d.check.goos, d.check.goarch)
	if err != nil {
		if sig := d.interrupted(); sig != nil {
			return "", "", stopStatus(sig)
		}
		return "", "", d.gcError(err)
	}
	if out == "" {
		out = filepath.Join(work, "prog"+exeSuffixFor(d.check.goos))
	}
	args := build.CompileFlags(cxx, d.flags.asan, false)
	args = append(args, build.GeneratedFlags(cxx)...)
	args = append(args, "-I", filepath.Join(rt, "include"), genFile)
	for _, src := range rtfiles.GCSources(d.check.goos) {
		args = append(args, filepath.Join(rt, filepath.FromSlash(src)))
	}
	args = append(args, "-o", out)
	if d.flags.x {
		words := []string{cxx.Name()}
		for _, a := range args {
			words = append(words, build.ShellQuoteWork(a, work))
		}
		fmt.Fprintln(d.stderr, strings.Join(words, " "))
	}
	cmd := cxx.Command(d.ctx, args...)
	cmd.Dir = work
	cmd.Stdout, cmd.Stderr = d.stderr, d.stderr // the compiler's diagnostics, like a build's
	if err := cmd.Run(); err != nil {
		if sig := d.interrupted(); sig != nil {
			return "", "", stopStatus(sig)
		}
		// The compiler printed its own errors. Say what failed, not why: it did.
		fmt.Fprintf(d.stderr, "gocpp %s: the C++ compiler failed on the GC engine's C++\n", d.name)
		return "", "", exitErrors
	}
	keep = true
	return out, work, 0
}

// gcRuntime returns the folder of the embedded runtime that the GC engine compiles against.
// Like the ARC engine's builds, it is extracted once into the cache; without a cache folder it
// is written into the work folder.
func (d *driver) gcRuntime(work string) (string, error) {
	if cache := cacheDir(); cache != "" {
		return build.ExtractRuntime(cache)
	}
	dir := filepath.Join(work, "rt")
	return dir, build.WriteRuntime(dir)
}
