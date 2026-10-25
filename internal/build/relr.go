package build

import (
	"context"
	"debug/elf"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"syscall"
	"time"
)

// Packed relative relocations (DT_RELR). A program is position-independent, so every pointer in its
// constant data (the method tables, the descriptors, the strings they name) has a relative
// relocation, 24 bytes each in .rela.dyn; DT_RELR keeps them as a bitmap, a few bytes for a run of
// them, and the dynamic loader of glibc 2.36 and later applies them (gocpp's programs need glibc
// 2.38 already). GNU ld 2.38 and lld 15 make it with -z pack-relative-relocs; an older linker warns
// about the option or refuses it, so the link has it only when the linker of the compiler made a
// DT_RELR in a probe link (relrFlags). ELF targets only: MinGW-w64's PE files have base
// relocations of their own.

// relrFlag is the link flag of DT_RELR.
const relrFlag = "-Wl,-z,pack-relative-relocs"

// relrSource is the probe's program: a pointer in data, which needs a relative relocation in a
// position-independent program, and a read through it. A loader that ignores DT_RELR (glibc before
// 2.36) leaves the pointer unrelocated, and the read crashes, so the probe also runs the program.
const relrSource = "static int x = 42;\nint* p = &x;\nint main(void) { return *p == 42 ? 0 : 1; }\n"

// A relrDep is a file that the probe's answer depends on (the linker, the C library, the loader) as
// it was when the probe ran: its path and identity (fileIdentity).
type relrDep struct {
	Path string `json:"path"`
	ID   string `json:"id"`
}

// relrEntry is an answer with the files it depends on.
type relrEntry struct {
	relr bool
	deps []relrDep
}

// relrProbes caches the probe's answer in this process, by the compiler command, its program file and
// the link flags.
var relrProbes = struct {
	sync.Mutex
	m map[string]relrEntry
}{m: make(map[string]relrEntry)}

// relrRecord is the probe's answer on disk (detectFile), kept while the compiler's file and the files
// of Deps are the same.
type relrRecord struct {
	Key   string    `json:"key"`
	Stamp fileRec   `json:"stamp"`
	Relr  bool      `json:"relr"`
	Deps  []relrDep `json:"deps"`
}

// relrFlags returns the link flag of DT_RELR for this build's compiler, or nil: the target is not
// Linux (or the compiler's program can't be looked at: a CXX made by hand), or the probe link did
// not make a working DT_RELR (relrProbe). The probe links with the build's own link flags
// (GOCPP_LDFLAGS: -fuse-ld, -B, ...), so it asks the linker that the build will use. The answer is
// asked once per compiler, flags and linker and kept on disk with the compiler detection
// (SetDetectCache).
func (b *Builder) relrFlags(ctx context.Context) []string {
	if b.relr != nil {
		return *b.relr
	}
	var flags []string
	if relrWanted(b.CXX, b.GOOS) && relrAnswer(ctx, b.CXX, b.Work, b.Env.LDFlags) {
		flags = []string{relrFlag}
	}
	if ctx.Err() == nil {
		b.relr = &flags
	}
	return flags
}

// relrWanted reports whether a link for goos with the compiler c can have DT_RELR at all: an ELF
// target (Linux) and a compiler that LookCXX found (its program file stamp is known).
func relrWanted(c CXX, goos string) bool {
	if goos == "" {
		goos = runtime.GOOS
	}
	return goos == "linux" && c.GOOS != "windows" && !c.C && c.stamp.real != ""
}

// relrKey is the key of the probe's answer: the compiler command and its program file and the link
// flags of the probe. (The linker, C library and loader that it also depends on are the answer's
// deps, looked at by their files: no compiler runs to ask for them when the answer is cached.)
func relrKey(c CXX, flags []string) string {
	return "relr\x00" + commandKey(c.Path, c.Args) + "\x00" + c.stamp.real + "\x00" + strings.Join(flags, "\x01")
}

// fileIdentity names the file p (looked up in PATH when it has no slash) with its size and time, or
// p and "?" for a file that can't be found.
func fileIdentity(p string) string {
	if p != "" && !strings.ContainsAny(p, `/\`) {
		if lp, err := exec.LookPath(p); err == nil {
			p = lp
		}
	}
	st, err := stampOf(p)
	if err != nil {
		return p + "?"
	}
	return fmt.Sprintf("%s:%d:%d", st.real, st.size, st.modTime)
}

// printed returns what the compiler c prints for the query q with flags (its first line), or "".
func printed(ctx context.Context, c CXX, flags []string, q string) string {
	pctx, cancel := context.WithTimeout(ctx, detectTimeout)
	defer cancel()
	b, err := c.Command(pctx, slices.Concat(flags, []string{q})...).Output()
	if err != nil {
		return ""
	}
	line, _, _ := strings.Cut(strings.TrimSpace(string(b)), "\n")
	return line
}

// relrDeps returns the files that the probe's answer depends on, as they are now: the linker programs
// that the compiler runs with flags and its own arguments (ld, and ld.NAME for a -fuse-ld=NAME; the file
// of a -fuse-ld=/path/ld.gold or --ld-path=FILE, in the flags or in the compiler's arguments), and the C
// library and the dynamic loader that it links a program with (the probe runs the program: a loader
// that ignores DT_RELR crashes it). It asks the compiler (4 runs), so it is for a probe, not for a
// cached answer (relrDepsValid).
func relrDeps(ctx context.Context, c CXX, flags []string) []relrDep {
	names := []string{"ld"}
	var paths []string
	for _, f := range slices.Concat(c.Args, flags) {
		if n, ok := strings.CutPrefix(f, "-fuse-ld="); ok && n != "" {
			if strings.ContainsAny(n, `/\`) {
				paths = append(paths, n)
			} else {
				names = append(names, "ld."+n)
			}
		} else if n, ok := strings.CutPrefix(f, "--ld-path="); ok && n != "" {
			paths = append(paths, n)
		}
	}
	for _, name := range names {
		paths = append(paths, printed(ctx, c, flags, "-print-prog-name="+name))
	}
	loader := "ld-linux-x86-64.so.2"
	switch runtime.GOARCH {
	case "arm64":
		loader = "ld-linux-aarch64.so.1"
	case "386":
		loader = "ld-linux.so.2"
	case "riscv64":
		loader = "ld-linux-riscv64-lp64d.so.1"
	}
	for _, lib := range []string{"libc.so.6", loader} {
		paths = append(paths, printed(ctx, c, flags, "-print-file-name="+lib))
	}
	deps := make([]relrDep, 0, len(paths))
	for _, p := range paths {
		if p != "" && !strings.ContainsAny(p, `/\`) {
			if lp, err := exec.LookPath(p); err == nil {
				p = lp
			}
		}
		deps = append(deps, relrDep{Path: p, ID: fileIdentity(p)})
	}
	return deps
}

// relrDepsValid reports whether every file of deps is still as it was (a stat each, no compiler run).
func relrDepsValid(deps []relrDep) bool {
	for _, d := range deps {
		if fileIdentity(d.Path) != d.ID {
			return false
		}
	}
	return true
}

// relrAnswer is relrProbe, from the caches when the compiler's file, the flags and the files that the
// answer depends on (the linker, the C library, the loader) are unchanged. A cached answer costs a stat
// of each of them and no compiler run.
func relrAnswer(ctx context.Context, c CXX, work string, flags []string) bool {
	key := relrKey(c, flags)
	relrProbes.Lock()
	e, ok := relrProbes.m[key]
	relrProbes.Unlock()
	if ok && relrDepsValid(e.deps) {
		return e.relr
	}
	ok = false
	file := detectFile(key)
	rec := recOfStamp(c.stamp)
	if file != "" {
		if data, err := os.ReadFile(file); err == nil {
			var r relrRecord
			if json.Unmarshal(data, &r) == nil && r.Key == key && r.Stamp == rec && relrDepsValid(r.Deps) {
				e, ok = relrEntry{r.Relr, r.Deps}, true
			}
		}
	}
	if !ok {
		deps := relrDeps(ctx, c, flags) // (before the probe: a file replaced during it is seen next time)
		v, err := relrProbe(ctx, c, work, flags)
		if err != nil {
			return false // (a cancelled probe is not an answer: not kept)
		}
		e = relrEntry{v, deps}
		if file != "" {
			if data, err := json.Marshal(relrRecord{Key: key, Stamp: rec, Relr: v, Deps: deps}); err == nil && os.MkdirAll(filepath.Dir(file), 0o700) == nil {
				writeFileAtomic(file, data)
			}
		}
	}
	relrProbes.Lock()
	relrProbes.m[key] = e
	relrProbes.Unlock()
	return e.relr
}

// relrRunnable reports whether a program that the compiler c links can run on this machine: the
// target is this system and architecture (a cross compiler's program can't, and the probe then
// trusts the ELF file).
func relrRunnable(c CXX) bool {
	return runtime.GOOS == "linux" && (c.GOOS == "" || c.GOOS == runtime.GOOS) && (c.GOARCH == "" || c.GOARCH == runtime.GOARCH)
}

// relrProbe links relrSource with the link flags flags, relrFlag and fatal warnings in a folder of
// work, and reports whether the program has a DT_RELR and, when it can run here, whether it runs:
// the read through the relocated pointer works. A linker that wrote a DT_RELR proves nothing about
// the loader (glibc before 2.36 ignores the entry and the program crashes at its start). The error
// is the context's, when it ended the probe.
func relrProbe(ctx context.Context, c CXX, work string, flags []string) (bool, error) {
	dir, err := os.MkdirTemp(work, "relr-")
	if err != nil {
		return false, nil
	}
	defer os.RemoveAll(dir)
	out := filepath.Join(dir, "a.out")
	pctx, cancel := context.WithTimeout(ctx, detectTimeout)
	defer cancel()
	args := slices.Concat([]string{"-x", "c", "-", "-o", out}, flags, []string{relrFlag, "-Wl,--fatal-warnings"})
	cmd := c.Command(pctx, args...)
	cmd.Dir = dir
	cmd.Stdin = strings.NewReader(relrSource)
	cmd.WaitDelay = 2 * time.Second
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return false, ctx.Err()
		}
		return false, nil
	}
	f, err := elf.Open(out)
	if err != nil {
		return false, nil
	}
	vals, err := f.DynValue(dtRELR)
	f.Close()
	if err != nil || len(vals) == 0 {
		return false, nil
	}
	if !relrRunnable(c) {
		return true, nil
	}
	rctx, rcancel := context.WithTimeout(ctx, detectTimeout)
	defer rcancel()
	run := exec.CommandContext(rctx, out)
	run.Dir = dir
	run.WaitDelay = 2 * time.Second
	if err := run.Run(); err != nil {
		if ctx.Err() != nil {
			return false, ctx.Err()
		}
		var ee *exec.ExitError
		if errors.As(err, &ee) || errors.Is(err, syscall.ENOEXEC) {
			// It crashed or answered wrong, or is not a program of this machine (a
			// compiler with -m32 or -march flags of another one): no flag for a crash,
			// the ELF file's word for a program that can't run here.
			return errors.Is(err, syscall.ENOEXEC), nil
		}
		return false, nil
	}
	return true, nil
}

// dtRELR is the dynamic tag of the packed relative relocations (debug/elf has no name for it).
const dtRELR = elf.DynTag(36)
