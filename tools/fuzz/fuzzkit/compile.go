package fuzzkit

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"hash/fnv"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/huuhait/gocpp/internal/build"
	"github.com/huuhait/gocpp/internal/cppgen"
	rtfiles "github.com/huuhait/gocpp/runtime"
)

// The sampled C++ compile stage (Phase 15 step 15.8). The emitter's output for a program that
// checks cleanly must compile: with every compiler gocpp supports, without an error and without a
// warning (the flags of the driver, build.Flags and build.GeneratedFlags). Compiling takes about
// 0.3 s a unit even with the precompiled runtime header, a thousand times the time of the rest of
// the pipeline, so only one program in N is compiled (SampleEvery), and only with -fsyntax-only:
// nothing is linked, and nothing runs.
//
// The runtime's headers and the precompiled rt.hpp are kept in a folder that all the processes
// of a campaign share (GOCPP_FUZZ_CACHE, else the system's temporary folder), written to a name
// of its own and renamed into place, because g++ crashes on a half-written .gch.

// SampleEvery returns N: one program in N is compiled by the fuzz targets. GOCPP_FUZZ_COMPILE_EVERY
// sets it; 0 (the default) turns the stage off, for the plain "go test" run of the seeds.
func SampleEvery() int {
	if s := os.Getenv("GOCPP_FUZZ_COMPILE_EVERY"); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n >= 0 {
			return n
		}
	}
	return 0
}

// forceCompile is set after the first compile failure of the process: the engine minimizes the
// failing input in the same process, and every candidate must be compiled to tell whether it
// still fails, not only the sampled ones.
var forceCompile atomic.Bool

// Sampled reports whether the program src is one of the 1 in N that are compiled.
func Sampled(src []byte) bool {
	if forceCompile.Load() {
		return true
	}
	n := SampleEvery()
	if n == 0 {
		return false
	}
	h := fnv.New64a()
	h.Write(src)
	return h.Sum64()%uint64(n) == 0
}

// A Compiler is one C++ compiler with the runtime's headers and precompiled header for it.
type Compiler struct {
	Name string
	cc   build.CXX
	args []string // the flags before the source, up to the precompiled header
}

// A CompileFailure is a compile that did not succeed.
type CompileFailure struct {
	Compiler string
	Unit     string // the path of the translation unit in Files
	Output   string // what the compiler printed (cut)
	Err      error  // the compiler could not be run, or timed out
}

func (f *CompileFailure) Error() string {
	if f.Err != nil {
		return fmt.Sprintf("%s -fsyntax-only %s: %v\n%s", f.Compiler, f.Unit, f.Err, f.Output)
	}
	return fmt.Sprintf("%s -fsyntax-only %s failed:\n%s", f.Compiler, f.Unit, f.Output)
}

var (
	compilersOnce sync.Once
	compilers     []*Compiler
	compilersErr  error
)

// Compilers returns g++ and clang++ (or GOCPP_CXX alone) as far as they are installed, and an error
// when GOCPP_REQUIRE_CXX names one that is not or when there is none.
func Compilers() ([]*Compiler, error) {
	compilersOnce.Do(func() {
		names := []string{"g++", "clang++"}
		if env := os.Getenv("GOCPP_CXX"); env != "" {
			names = []string{env}
		}
		var missing []string
		for _, n := range names {
			cc, err := build.LookCXX(context.Background(), n)
			if err != nil {
				missing = append(missing, err.Error())
				continue
			}
			c := &Compiler{Name: filepath.Base(n), cc: cc}
			if cc.Family == build.Clang {
				c.Name = "clang++"
			} else if cc.Family == build.GCC {
				c.Name = "g++"
			}
			compilers = append(compilers, c)
		}
		for _, want := range strings.Split(os.Getenv("GOCPP_REQUIRE_CXX"), ",") {
			if want = strings.TrimSpace(want); want != "" && !slices.ContainsFunc(compilers, func(c *Compiler) bool { return c.Name == want }) {
				compilersErr = fmt.Errorf("GOCPP_REQUIRE_CXX=%s needs %s: %s", os.Getenv("GOCPP_REQUIRE_CXX"), want, strings.Join(missing, "; "))
			}
		}
		if len(compilers) == 0 && compilersErr == nil {
			compilersErr = errors.New("no C++ compiler: " + strings.Join(missing, "; "))
		}
	})
	return compilers, compilersErr
}

// cacheDir returns the folder of the shared files.
func cacheDir() string {
	if d := os.Getenv("GOCPP_FUZZ_CACHE"); d != "" {
		return d
	}
	return filepath.Join(os.TempDir(), fmt.Sprintf("gocpp-fuzz-cache-%d", os.Getuid()))
}

var (
	rtOnce sync.Once
	rtDir  string
	rtErr  error
)

// runtimeInclude returns the folder with the runtime's headers (include/gocpp/...), extracted once;
// its name has a hash of their contents, so a changed runtime gets a folder of its own.
func runtimeInclude() (string, error) {
	rtOnce.Do(func() {
		h := sha256.New()
		efs := rtfiles.FS()
		err := fs.WalkDir(efs, "include", func(p string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			b, err := fs.ReadFile(efs, p)
			h.Write([]byte(p))
			h.Write(b)
			return err
		})
		if err != nil {
			rtErr = err
			return
		}
		name := "rt-" + hex.EncodeToString(h.Sum(nil))[:16]
		cleanOnce.Do(func() { cleanStale(os.TempDir(), cacheDir(), name, time.Now()) })
		dir := filepath.Join(cacheDir(), name)
		if _, err := os.Stat(filepath.Join(dir, "include")); err == nil {
			rtDir = dir
			return
		}
		tmp, err := os.MkdirTemp(cacheDirMade(), "rt-tmp-")
		if err != nil {
			rtErr = err
			return
		}
		err = fs.WalkDir(efs, "include", func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			dst := filepath.Join(tmp, filepath.FromSlash(p))
			if d.IsDir() {
				return os.MkdirAll(dst, 0o755)
			}
			b, err := fs.ReadFile(efs, p)
			if err != nil {
				return err
			}
			return os.WriteFile(dst, b, 0o644)
		})
		if err == nil {
			err = os.Rename(tmp, dir)
			if err != nil {
				if _, serr := os.Stat(filepath.Join(dir, "include")); serr == nil {
					err = nil // another process was first
				}
			}
		}
		if err != nil {
			os.RemoveAll(tmp)
			rtErr = err
			return
		}
		rtDir = dir
	})
	return rtDir, rtErr
}

// A worker that the engine kills (a hang, a limit) leaves its folder of emitted files in the
// temporary folder, and every runtime header change leaves its precompiled headers in the cache:
// the first use in a process removes what is older than an hour and not the current runtime's.
var cleanOnce sync.Once

func cleanStale(tmp, cache, keep string, now time.Time) {
	old := func(e fs.DirEntry) bool {
		info, err := e.Info()
		return err == nil && now.Sub(info.ModTime()) > time.Hour
	}
	if es, err := os.ReadDir(tmp); err == nil {
		for _, e := range es {
			if e.IsDir() && strings.HasPrefix(e.Name(), "gocpp-fuzz-gen-") && old(e) {
				os.RemoveAll(filepath.Join(tmp, e.Name()))
			}
		}
	}
	if es, err := os.ReadDir(cache); err == nil {
		for _, e := range es {
			if e.IsDir() && (strings.HasPrefix(e.Name(), "rt-") || strings.HasPrefix(e.Name(), "pch-")) && e.Name() != keep && old(e) {
				os.RemoveAll(filepath.Join(cache, e.Name()))
			}
		}
	}
}

// cacheDirMade creates the cache folder and returns it.
func cacheDirMade() string {
	d := cacheDir()
	os.MkdirAll(d, 0o755)
	return d
}

var pchMu sync.Mutex

// prepare makes the flags of the compiler: the driver's flags, the runtime's headers, and the
// precompiled rt.hpp.
func (c *Compiler) prepare() error {
	pchMu.Lock()
	defer pchMu.Unlock()
	if c.args != nil {
		return nil
	}
	rt, err := runtimeInclude()
	if err != nil {
		return err
	}
	flags := slices.Concat(build.Flags(c.cc, false), build.GeneratedFlags(c.cc), []string{"-I" + filepath.Join(rt, "include")})
	id, _ := c.cc.Identity()
	sum := sha256.Sum256([]byte(c.cc.Path + "\x00" + id + "\x00" + strings.Join(flags, "\x00")))
	dir := filepath.Join(rt, "pch-"+c.Name+"-"+hex.EncodeToString(sum[:])[:12])
	header := filepath.Join(dir, "rt.hpp")
	ext, use := ".gch", []string{"-include", header}
	if c.cc.Family == build.Clang {
		ext, use = ".pch", []string{"-include-pch", header + ".pch"}
	}
	if _, err := os.Stat(header + ext); err != nil {
		// The header is the same text everywhere and stays at its final place (clang records the
		// path of the header in the .pch); the compiled file gets a name of its own and is moved
		// into place whole.
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
		// Created once, whole, and never written again: clang refuses a .pch when the header has a
		// newer modification time than the .pch, and the workers of a campaign get here together.
		// (A hard link is created whole or not at all.)
		hdrTmp := fmt.Sprintf("%s.hdr-%d", header, os.Getpid())
		if err := os.WriteFile(hdrTmp, []byte("#include <gocpp/rt.hpp>\n"), 0o644); err != nil {
			return err
		}
		err := os.Link(hdrTmp, header)
		os.Remove(hdrTmp)
		if err != nil && !errors.Is(err, fs.ErrExist) {
			return err
		}
		tmp := fmt.Sprintf("%s.tmp-%d", header+ext, os.Getpid())
		defer os.Remove(tmp)
		args := slices.Concat(flags, []string{"-x", "c++-header", header, "-o", tmp})
		out, err := c.run(dir, 5*time.Minute, args)
		if err != nil || len(out) != 0 {
			return fmt.Errorf("%s: precompiling rt.hpp: %v\n%s", c.Name, err, out)
		}
		if err := os.Rename(tmp, header+ext); err != nil {
			if _, serr := os.Stat(header + ext); serr != nil {
				return err
			}
		}
	}
	c.args = slices.Concat(flags, use)
	return nil
}

// run runs the compiler with args in dir, in a process group of its own that is killed with it
// at the end of the time limit. It returns the combined output; the error is non-nil when the
// compiler failed to run, or exited with a nonzero status (the output says why).
func (c *Compiler) run(dir string, limit time.Duration, args []string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), limit)
	defer cancel()
	name, argv := c.cc.Path, slices.Concat(c.cc.Args, args)
	if pl, err := exec.LookPath("prlimit"); err == nil {
		// 16 GiB of address space and 5 minutes of CPU for one compile.
		argv = slices.Concat([]string{"--as=17179869184", "--cpu=300", "--", name}, argv)
		name = pl
	}
	cmd := exec.CommandContext(ctx, name, argv...)
	cmd.Dir = dir
	cmd.Env = build.CompilerEnv()
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	build.InGroup(cmd)
	err := cmd.Run()
	build.KillGroup(cmd)
	if ctx.Err() != nil {
		err = fmt.Errorf("timed out after %v", limit)
	}
	return out.Bytes(), err
}

// SyntaxOnly compiles the translation units of files whose path starts with prefix with -fsyntax-only
// and returns the first failure: a nonzero exit status, or anything printed (a warning counts).
// The files are written to a new folder under the system's temporary folder, which is removed.
func (c *Compiler) SyntaxOnly(files []cppgen.File, prefix string) *CompileFailure {
	if err := c.prepare(); err != nil {
		return &CompileFailure{Compiler: c.Name, Err: err}
	}
	root, err := os.MkdirTemp("", "gocpp-fuzz-gen-")
	if err != nil {
		return &CompileFailure{Compiler: c.Name, Err: err}
	}
	defer os.RemoveAll(root)
	total := 0
	for _, f := range files {
		total += len(f.Data)
	}
	if total > 64<<20 {
		return nil // not a program to compile
	}
	var units []string
	for _, f := range files {
		p := filepath.Join(root, filepath.FromSlash(f.Path))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			return &CompileFailure{Compiler: c.Name, Err: err}
		}
		if err := os.WriteFile(p, f.Data, 0o644); err != nil {
			return &CompileFailure{Compiler: c.Name, Err: err}
		}
		if path.Ext(f.Path) == ".cpp" && strings.HasPrefix(f.Path, prefix) {
			units = append(units, f.Path)
		}
	}
	for _, u := range units {
		args := slices.Concat(c.args, []string{"-iquote", root, "-fsyntax-only", filepath.Join(root, filepath.FromSlash(u))})
		out, err := c.run(root, 3*time.Minute, args)
		if err != nil || len(out) != 0 {
			s := strings.ReplaceAll(string(out), root, "$GEN")
			if len(s) > 16<<10 {
				s = s[:16<<10] + "\n..."
			}
			forceCompile.Store(true)
			return &CompileFailure{Compiler: c.Name, Unit: u, Output: s, Err: errIfNotExit(err)}
		}
	}
	return nil
}

// errIfNotExit returns err unless it is only a nonzero exit status (the output tells the rest).
func errIfNotExit(err error) error {
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		return nil
	}
	return err
}

// CompileAll compiles the main package's units of files with every compiler and returns the first
// failure, or nil. The error is set when no compiler can be used.
func CompileAll(r *Result) (*CompileFailure, error) {
	cs, err := Compilers()
	if err != nil {
		return nil, err
	}
	for _, c := range cs {
		if f := c.SyntaxOnly(r.Files, r.MainPkg); f != nil {
			return f, nil
		}
	}
	return nil, nil
}
