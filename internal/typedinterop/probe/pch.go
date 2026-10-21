package probe

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
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

	"github.com/huuhait/gocpp/internal/build"
	"github.com/huuhait/gocpp/internal/interop"
)

// The precompiled header of the probes' standard library block (step 12.14, ADR-117).
//
// Every probe file starts with the same library headers (stdHeaders: <string>, <vector>,
// <memory>, ...), and clang parses them again in each run: 0.3 s of the 0.35 s that a run takes
// for a package with no library headers of its own, in each of the three stages of a cold check.
// The block is the same for every package, so it is compiled once into a precompiled header
// (-x c++-header) and each probe run loads it (-include-pch: 0.06 s). The file keeps its
// #include lines, which the loaded header makes no-ops, so no line moves and no answer changes.
//
// The header lives in the probe cache folder, content-addressed:
//
//	pch/ab/<hash>.pch    the precompiled header
//	pch/ab/<hash>.hpp    the source it was made from (the block; clang names it as an input file)
//	pch/ab/<hash>.json   the files the compiler read (and the hash of their contents)
//
// <hash> is the key: the compiler, the flags and the environment of the include path, the toolchain
// and the block itself. Whether the header may be used is the hash of the contents of its files,
// as for a probe result: an edit of a library header, or a header that appears earlier on the
// search path, makes it stale, and it is made again. Nothing here changes an answer: the files
// of the dependency list of a run that uses it are the files that the same run would read without
// it (the list that the header was made with is added to the run's own), and a run whose header
// the compiler refuses (a file's time changed under it, another compiler version wrote it) is made
// again without it.
//
// Which runs. The probe runs (units.go) and the function runs (decls.go) load the header. The type runs
// (decls.go) load it for the program's own types and only trust the answer when it is whole: clang reads the
// members of a class, struct or enum that comes from a precompiled header only when something uses them, and
// its JSON dump prints only the members read so far (the dump of std::mutex has no methods, that of struct
// _IO_FILE has a tenth of its fields, and no flag makes it read them: the dump-all option does not reach the
// JSON dumper). A declaration that the compiler parsed from the text of one of the program's files has all its
// members, so a type run's answer is used when the location of every declaration it describes is a file that
// exists and that the header was not made of (pchRef.madeOf); otherwise the run is made again without the
// header, with the full library block. Names of the library (std, an underscore) and names with template
// arguments are not tried with the header at all, since their answer would be refused. This is only a speed
// choice: the full preamble stays the oracle (ADR-117, ADR-121), and tests/pchoracle_test.go compares both.
//
// There is no header without a cache folder, for a compiler that is not clang, or when the flags
// say the cache is machine-specific (-march=native): the probes then run as before.

// EnvPCH is the variable that controls the precompiled header: off, 0, false or no (any case)
// turn it off, and wait makes every run wait for a header that is being made (for the tests and
// the benchmarks that need every run to use it).
const EnvPCH = "GOCPP_PROBE_PCH"

// pchKeep is the number of precompiled headers kept in the cache folder (each is several MB).
const pchKeep = 4

// pchOff reports whether the environment turns precompiled headers off.
func pchOff() bool {
	switch strings.ToLower(os.Getenv(EnvPCH)) {
	case "off", "0", "false", "no":
		return true
	}
	return false
}

// pchForced reports whether every run waits for a header that is being made.
func pchForced() bool {
	return pchForce || strings.EqualFold(os.Getenv(EnvPCH), "wait")
}

// hasStdBlock reports whether a probe text starts with the standard block after its first line.
func hasStdBlock(text string) bool {
	_, rest, ok := strings.Cut(text, "\n")
	return ok && strings.HasPrefix(rest, stdBlock())
}

// A pchFile is the meta file of a precompiled header.
type pchFile struct {
	Format   int      `json:"format"`
	Created  int64    `json:"created"`
	Compiler string   `json:"compiler"`
	Read     []string `json:"read"` // the files the compiler read, as it named them
	Deps     []string `json:"deps"` // Read and the places where a header would hide one of them
	DepsHash string   `json:"depsHash"`
	Size     int64    `json:"size"` // the bytes of the .pch (a cut-off file is not used)
}

// A pchRef is a header that a run loads: the files, and the headers the compiler read to make it.
type pchRef struct {
	path, src string
	read      []string

	// readInfo are the files of read that exist, for comparing a file by identity, not by spelling.
	readOnce sync.Once
	readInfo []os.FileInfo
}

// madeOf reports whether file, as a declaration's location names it (relative to dir, the folder
// the compiler ran in), is one of the files that the header was made of, or cannot be told from
// them: the same spelling, the same file on disk, or a file that cannot be looked at. A
// declaration in such a file may have been loaded from the header, where the dump prints only the
// members that were read. Only a file that exists and is none of them is the program's own.
func (p *pchRef) madeOf(dir, file string) bool {
	names := append([]string{p.src}, p.read...)
	p.readOnce.Do(func() {
		for _, name := range names {
			if fi, err := os.Stat(name); err == nil {
				p.readInfo = append(p.readInfo, fi)
			}
		}
	})
	clean := filepath.Clean(file)
	for _, name := range names {
		if filepath.Clean(name) == clean {
			return true
		}
	}
	if !filepath.IsAbs(file) {
		file = filepath.Join(dir, file)
	}
	fi, err := os.Stat(file)
	if err != nil {
		return true
	}
	for _, other := range p.readInfo {
		if os.SameFile(fi, other) {
			return true
		}
	}
	return false
}

// A pchState is the precompiled header of one key in a Runner: being made, or made (path) or
// given up (path empty).
type pchState struct {
	started  time.Time
	done     chan struct{}
	cancel   context.CancelFunc // stops the build (WaitPCH, the end of the command)
	ref      *pchRef
	deps     []string
	depsHash string
}

// pchRejects counts, by header path, the runs whose compiler refused the header; at pchMaxRejects the
// process stops using it.
var (
	pchRejectMu sync.Mutex
	pchRejects  = make(map[string]int)
)

const pchMaxRejects = 2

func pchRejected(path string) int {
	pchRejectMu.Lock()
	defer pchRejectMu.Unlock()
	return pchRejects[path]
}

func pchReject(path string) int {
	pchRejectMu.Lock()
	defer pchRejectMu.Unlock()
	pchRejects[path]++
	return pchRejects[path]
}

// pchKey is the key of the header for cfg and toolchain tc. It leaves out the package folder and
// the limits of the dump (the header is for every package), and has what changes the parse of the
// library headers.
func pchKey(cfg Config, tc toolchain) string {
	cfg.Dir, cfg.MaxDecl, cfg.MaxJSON = "", 0, 0
	pre := withToolchain(PreKey(cfg, &Source{Text: "pch\n" + stdBlock()}), tc)
	sum := sha256.Sum256([]byte("gocpp-probe-pch-v1\x00" + selfID() + "\x00" + pre))
	return hex.EncodeToString(sum[:])
}

// pchBase returns the path of the files of key (without extension) under the cache folder dir.
func pchBase(dir, key string) string {
	return filepath.Join(dir, "pch", key[:2], key[2:])
}

// A run that starts while the header is being made waits for it only when the build is old enough to
// be nearly done: a run without the header takes 0.15 to 0.2 s more, a build 0.35 to 0.45 s. A young
// build is left to finish beside the run (the next stage of the check finds it ready), an old one is
// waited for at most pchMaxWait. Variables for the tests.
var (
	pchGrace   = 200 * time.Millisecond
	pchMaxWait = 400 * time.Millisecond
	// pchForce makes every run wait for a header that is being made (tests that count the runs that
	// use it).
	pchForce = false
)

// A pchMode says what a caller does about a header that is not ready.
type pchMode int

const (
	pchStart pchMode = iota // start the build, do not wait
	pchUse                  // wait when the build is old enough to be nearly done (a run)
)

// withPCH returns cfg with the precompiled header for the run of text, when the text starts with
// the standard block and a header is ready; a run that has none is not slower than before.
func (rn *Runner) withPCH(ctx context.Context, cfg Config, text string) Config {
	if cfg.pch != nil || !hasStdBlock(text) {
		return cfg
	}
	cfg.pch = rn.pchPath(ctx, cfg, pchUse)
	return cfg
}

// Prewarm starts making the precompiled header of the standard library block for cfg, if there is
// none and the cache can keep it, and returns at once. A caller that knows that probes will run
// (the kind probe had to ask the compiler) starts it so that it is made beside the work that comes
// before the probes.
func (rn *Runner) Prewarm(ctx context.Context, cfg Config) {
	rn.pchPath(ctx, cfg, pchStart)
}

// pchWaitCap is how long WaitPCH waits for the builds of a call that is done, and pchStopWait how
// long it waits for a build that it has stopped (variables for the tests).
var (
	pchWaitCap  = 6 * time.Second
	pchStopWait = 2 * time.Second
)

// WaitPCH waits for the precompiled headers that this Runner is making, so that no compiler
// outlives the command that started it. A build that takes more than pchWaitCap, or that the
// context's end finds running, is stopped (its compiler is killed) and waited for a moment: a build
// that is blocked on the lock of another process (one that is gone, on a cache that is shared)
// must not hold the command.
func (rn *Runner) WaitPCH(ctx context.Context) {
	rn.mu.Lock()
	var states []*pchState
	for _, st := range rn.pchs {
		states = append(states, st)
	}
	rn.mu.Unlock()
	limit := time.NewTimer(pchWaitCap)
	defer limit.Stop()
	for _, st := range states {
		select {
		case <-st.done:
			continue
		case <-ctx.Done():
		case <-limit.C:
		}
		// Stopped: every build that is still running is told to end, and given a moment to kill its compiler.
		for _, other := range states {
			other.cancel()
		}
		grace := time.NewTimer(pchStopWait)
		defer grace.Stop()
		for _, other := range states {
			select {
			case <-other.done:
			case <-grace.C:
				return
			}
		}
		return
	}
}

// buildContextLocked is the context a build ends with (rn.mu is held).
func (rn *Runner) buildContextLocked(ctx context.Context) context.Context {
	if rn.buildCtx != nil {
		return rn.buildCtx
	}
	return context.WithoutCancel(ctx)
}

// SetBuildContext makes the context of a command (it ends when the command is interrupted) the one that
// the builds of precompiled headers started from now on end with. Without it a build outlives the call
// that started it, up to its own time limit (a batch of probes ends and cancels its context while the
// header is still being made for the next one).
func (rn *Runner) SetBuildContext(ctx context.Context) {
	rn.mu.Lock()
	rn.buildCtx = ctx
	rn.mu.Unlock()
}

// ClearBuildContext undoes SetBuildContext: builds started from now on end with the call that starts them.
func (rn *Runner) ClearBuildContext() {
	rn.mu.Lock()
	rn.buildCtx = nil
	rn.mu.Unlock()
}

// pchPath returns the precompiled header for cfg. When
// there is none it starts making it and returns nil (or waits for it, as mode says); it
// returns nil also when there can be none: no cache folder, not clang, a header that failed.
func (rn *Runner) pchPath(ctx context.Context, cfg Config, mode pchMode) *pchRef {
	if pchOff() || cfg.CXX.Family != build.Clang || relativeSearchPath(cfg) {
		return nil
	}
	dir, write := rn.diskFor(cfg)
	if dir == "" || !write {
		return nil
	}
	tc := rn.toolchain(ctx, cfg, dir, write)
	key := pchKey(cfg, tc)
	for round := 0; round < 4; round++ {
		rn.mu.Lock()
		st := rn.pchs[key]
		if st == nil {
			// The header outlives the call that asked for it (a batch of probes ends and cancels its
			// context): it has its own time limit, and ends with the command's context if there is one.
			bctx, cancel := context.WithCancel(rn.buildContextLocked(ctx))
			st = &pchState{done: make(chan struct{}), started: time.Now(), cancel: cancel}
			if rn.pchs == nil {
				rn.pchs = make(map[string]*pchState)
			}
			rn.pchs[key] = st
			rn.mu.Unlock()
			go func() {
				defer cancel()
				st.ref, st.deps, st.depsHash = rn.obtainPCH(bctx, cfg, dir, key, tc)
				close(st.done)
			}()
		} else {
			rn.mu.Unlock()
		}
		select {
		case <-st.done:
		default:
			age := time.Since(st.started)
			if mode == pchStart || !pchForced() && age < pchGrace {
				return nil
			}
			limit := time.NewTimer(pchMaxWait)
			if pchForced() {
				limit.Reset(effTimeout(cfg))
			}
			select {
			case <-st.done:
			case <-limit.C:
				return nil
			case <-ctx.Done():
				limit.Stop()
				return nil
			}
			limit.Stop()
		}
		if st.ref == nil || pchRejected(st.ref.path) >= pchMaxRejects {
			return nil
		}
		if DepsHash(st.deps) == st.depsHash {
			return st.ref
		}
		// A library header changed while this process runs: make the header again.
		rn.mu.Lock()
		if rn.pchs[key] == st {
			delete(rn.pchs, key)
		}
		rn.mu.Unlock()
	}
	return nil
}

// relativeSearchPath reports whether the include path of the probes has a folder that is spelled relative
// to the working directory (an entry of CPATH and the like, an empty entry, a -I, -isystem, -iquote,
// -idirafter, -include or -imacros argument): the header is made in an empty folder and the runs are made in
// the package's, so such a path would find different files. There is no header then.
func relativeSearchPath(cfg Config) bool {
	for _, name := range searchPathVariables {
		v, ok := os.LookupEnv(name)
		if !ok || v == "" {
			continue
		}
		for _, e := range filepath.SplitList(v) {
			if e == "" || !filepath.IsAbs(e) {
				return true
			}
		}
	}
	withArg := []string{"-I", "-isystem", "-iquote", "-idirafter", "-include", "-imacros", "-isysroot", "-iprefix"}
	flags := cfg.Flags
	for i := 0; i < len(flags); i++ {
		f := flags[i]
		for _, w := range withArg {
			switch {
			case f == w:
				if i+1 < len(flags) && !filepath.IsAbs(flags[i+1]) {
					return true
				}
			case strings.HasPrefix(f, w) && len(f) > len(w) && !strings.HasPrefix(f, "-include-"):
				if !filepath.IsAbs(strings.TrimPrefix(strings.TrimPrefix(f, w), "=")) {
					return true
				}
			}
		}
		if strings.HasPrefix(f, "--sysroot=") && !filepath.IsAbs(strings.TrimPrefix(f, "--sysroot=")) {
			return true
		}
	}
	return false
}

// searchPathVariables are the variables whose folders the compiler adds to its include path.
var searchPathVariables = []string{
	"CPATH", "C_INCLUDE_PATH", "CPLUS_INCLUDE_PATH", "OBJC_INCLUDE_PATH", "OBJCPLUS_INCLUDE_PATH",
}

// obtainPCH finds the header on disk or makes it, under the lock of its key. It returns an empty
// path when it cannot.
func (rn *Runner) obtainPCH(ctx context.Context, cfg Config, dir, key string, tc toolchain) (
	ref *pchRef, deps []string, depsHash string) {
	base := pchBase(dir, key)
	if r, d, h, ok := lookPCH(base); ok {
		return r, d, h
	}
	release, _ := acquireLock(ctx, dir, "p"+key, effTimeout(cfg))
	if release != nil {
		defer release()
	}
	if ctx.Err() != nil {
		return nil, nil, ""
	}
	// (Always: a process that missed and then took the lock just after the builder let it go finds the
	// header the builder made.)
	if r, d, h, ok := lookPCH(base); ok {
		return r, d, h
	}
	// The question about the includes of the block does not wait for the compile.
	preamble := "// gocpp typed interop: precompiled block (generated)\n" + stdBlock()
	ictx, endInfo := rn.prefetchInfo(ctx, cfg, preamble)
	defer endInfo()
	start := time.Now()
	read, err := buildPCH(ctx, cfg, base)
	if err != nil {
		return nil, nil, ""
	}
	cacheCount.pchBuilds.Add(1)
	deps = rn.finalDeps(ictx, cfg, preamble, nil, read, nil, tc)
	if ctx.Err() != nil || racyDeps(deps, start) {
		// Cancellation or a changed library header leaves no complete dependencies to keep. Use
		// the header for nothing and ask again next time.
		os.Remove(base + ".pch")
		return nil, nil, ""
	}
	hash := DepsHash(deps)
	fi, err := os.Stat(base + ".pch")
	if err != nil {
		return nil, nil, ""
	}
	meta, err := json.Marshal(pchFile{
		Format: 1, Created: time.Now().UnixNano(), Compiler: compilerID(cfg.CXX),
		Read: read, Deps: deps, DepsHash: hash, Size: fi.Size(),
	})
	if err != nil || !writeEntry(base+".json", meta) {
		os.Remove(base + ".pch")
		return nil, nil, ""
	}
	prunePCH(filepath.Join(dir, "pch"), pchKeep, base)
	return &pchRef{path: base + ".pch", src: base + ".hpp", read: read}, deps, hash
}

// lookPCH reads the description of the header at base and checks the files: the description must be
// whole and its file lists have their hash, the .pch must be the size it had when it was made (a cut-off
// file is refused here, not by the compiler in each run), and the source must be there.
func lookPCH(base string) (ref *pchRef, deps []string, depsHash string, ok bool) {
	body, ok := readEntry(base + ".json")
	if !ok {
		return nil, nil, "", false
	}
	var pf pchFile
	if json.Unmarshal(body, &pf) != nil || pf.Format != 1 || pf.DepsHash == "" || len(pf.Read) == 0 ||
		pf.Size <= 0 || DepsHash(pf.Deps) != pf.DepsHash {
		return nil, nil, "", false
	}
	if fi, err := os.Stat(base + ".pch"); err != nil || !fi.Mode().IsRegular() || fi.Size() != pf.Size {
		return nil, nil, "", false
	}
	if _, err := os.Stat(base + ".hpp"); err != nil {
		return nil, nil, "", false
	}
	cacheCount.diskHits.Add(1)
	return &pchRef{path: base + ".pch", src: base + ".hpp", read: pf.Read}, pf.Deps, pf.DepsHash, true
}

// buildPCH compiles the standard block into base+".pch" (and writes base+".hpp"), and returns the
// headers that the compiler read, sorted, without the block's own file. The files are written
// under a temporary name and renamed, so another process sees the old header or the new one. The
// compiler runs outside the Jobs() gate (one build per key and process, and it is the only way to the
// other runs' header); its process group dies with ctx.
func buildPCH(ctx context.Context, cfg Config, base string) ([]string, error) {
	folder := filepath.Dir(base)
	if err := os.MkdirAll(folder, 0o700); err != nil {
		return nil, err
	}
	tmp, err := os.MkdirTemp("", "gocpp-typed-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmp)
	if tmp, err = filepath.Abs(tmp); err != nil {
		return nil, err
	}
	// The source is written where it stays: the compiler names it as an input file of the header.
	hpp := base + ".hpp"
	if err := writeSource(hpp, stdBlock()); err != nil {
		return nil, err
	}
	out, err := os.CreateTemp(folder, ".tmp-*.pch")
	if err != nil {
		return nil, err
	}
	outPath := out.Name()
	out.Close()
	defer os.Remove(outPath)
	depPath := filepath.Join(tmp, "gp_pch.d")
	// The package folder is not an argument: the header is for every package, and the quoted
	// includes of its own folder do not matter to a block of angle-bracket includes.
	extra := []string{"-fvalidate-ast-input-files-content", "-o", outPath}
	args, err := interop.ProbeCompilerArgs(cfg.CXX, extra, "", cfg.Flags, depPath, hpp)
	if err != nil {
		return nil, err
	}
	args = asHeader(args)
	rctx, cancel := context.WithTimeout(ctx, effTimeout(cfg))
	defer cancel()
	stderr := &limitWriter{max: maxStderr}
	cmd, err := build.Start(rctx, func() *exec.Cmd {
		c := cfg.CXX.Command(rctx, args...)
		c.Dir = tmp
		c.Env = interop.ProbeEnv(tmp, "")
		c.Stderr = stderr
		c.WaitDelay = probeWaitDelay
		build.InGroup(c)
		return c
	})
	if err != nil {
		return nil, fmt.Errorf("%s didn't start: %w", cfg.CXX.Name(), err)
	}
	defer build.KillGroup(cmd)
	werr := cmd.Wait()
	build.KillGroup(cmd)
	if ctx.Err() != nil {
		return nil, fmt.Errorf("%s was stopped: %w", cfg.CXX.Name(), ctx.Err())
	}
	if werr != nil {
		return nil, fmt.Errorf("%s could not precompile the library headers: %v%s",
			cfg.CXX.Name(), werr, excerpt(stderr.buf.Bytes()))
	}
	data, err := os.ReadFile(depPath)
	if err != nil {
		return nil, err
	}
	deps := interop.ParseDeps(data, hpp, "")
	deps = slices.DeleteFunc(deps, func(d string) bool { return d == hpp || strings.HasSuffix(d, ".pch") })
	slices.Sort(deps)
	if len(deps) == 0 {
		return nil, errors.New("the precompiled header read no file")
	}
	if err := os.Rename(outPath, base+".pch"); err != nil {
		return nil, err
	}
	return deps, nil
}

// writeSource writes the source of a header to path unless the file already has this text, and then
// by a temporary name and a rename: a process that has loaded the header names the file as an input
// and checks its size and time, so the file must not change in place. A file that is already right
// gets a new modification time (it is in use; the pruning of orphans goes by age).
func writeSource(path, text string) error {
	if old, err := os.ReadFile(path); err == nil && string(old) == text {
		now := time.Now()
		os.Chtimes(path, now, now)
		return nil
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tmp-*.hpp")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if _, err := tmp.WriteString(text); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(name, path)
}

// asHeader changes the arguments of a compile of a source (ProbeCompilerArgs ends with -x c++
// and the file) into those of a precompiled header.
func asHeader(args []string) []string {
	args = slices.Clone(args)
	for i := len(args) - 2; i >= 0; i-- {
		if args[i] == "-x" && args[i+1] == "c++" {
			args[i+1] = "c++-header"
			break
		}
	}
	return args
}

// withPCHArgs adds the header to the arguments of a probe run (before the source's -x c++).
func withPCHArgs(args []string, pch string) []string {
	for i := len(args) - 2; i >= 0; i-- {
		if args[i] == "-x" && args[i+1] == "c++" {
			return slices.Concat(args[:i:i], []string{"-include-pch", pch}, args[i:])
		}
	}
	return args
}

// pchMessage reports whether the compiler's messages are about a precompiled header that it
// would not use (a file changed, another version wrote it).
func pchMessage(stderr []byte) bool {
	s := strings.ToLower(string(stderr))
	for _, w := range []string{
		"precompiled header", "precompiled file", "pch file", "ast file", "pch ", "malformed", "corrupt",
	} {
		if strings.Contains(s, w) {
			return true
		}
	}
	return false
}

// pchOrphanAge is how old a temporary file or a file without its description must be to be removed by
// prunePCH: a build in another process writes them for a few seconds.
var pchOrphanAge = 10 * time.Minute

// prunePCH removes the oldest precompiled headers of root until keep are left; the files of keepBase
// stay. It also removes what a killed build left: temporary files (.tmp-*), and a .pch or .hpp with no
// description (.json) of the same name, when they are older than pchOrphanAge.
func prunePCH(root string, keep int, keepBase string) {
	type one struct {
		base string
		mod  time.Time
	}
	var all []one
	described := make(map[string]bool)
	var loose []string
	old := time.Now().Add(-pchOrphanAge)
	filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		fi, err := d.Info()
		if err != nil {
			return nil
		}
		name := d.Name()
		switch {
		case strings.HasPrefix(name, ".tmp-"):
			if fi.ModTime().Before(old) {
				os.Remove(p)
			}
		case strings.HasSuffix(p, ".json"):
			all = append(all, one{strings.TrimSuffix(p, ".json"), fi.ModTime()})
			described[strings.TrimSuffix(p, ".json")] = true
		case (strings.HasSuffix(p, ".pch") || strings.HasSuffix(p, ".hpp")) && fi.ModTime().Before(old):
			loose = append(loose, p)
		}
		return nil
	})
	for _, p := range loose {
		base := strings.TrimSuffix(strings.TrimSuffix(p, ".pch"), ".hpp")
		if !described[base] && base != keepBase {
			os.Remove(p)
		}
	}
	if len(all) <= keep {
		return
	}
	slices.SortFunc(all, func(a, b one) int { return a.mod.Compare(b.mod) })
	for _, o := range all[:len(all)-keep] {
		if o.base == keepBase {
			continue
		}
		for _, ext := range []string{".json", ".pch", ".hpp"} {
			os.Remove(o.base + ext)
		}
	}
}
