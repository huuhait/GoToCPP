package cppgen

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/huuhait/gocpp/internal/build"
	rtfiles "github.com/huuhait/gocpp/runtime"
)

// The C++ side of the tests: the compilers, the runtime and running programs.
//
// Like the test harness (tests/harness_test.go) and the runtime's tests: the tests use g++ and
// clang++, or only the compiler in GOCPP_CXX; a compiler that isn't installed is a skip, unless
// GOCPP_REQUIRE_CXX (a comma list, "g++,clang++" in make ci) names it, which makes it a
// failure.

// cmdTimeout bounds each compile and each run.
const cmdTimeout = 3 * time.Minute

// A cxx is one C++ compiler of the test matrix.
type cxx struct {
	name    string    // "g++", "clang++"
	cc      build.CXX // as build.LookCXX found it
	missing string    // why it can't be used, or ""

	rt  [2]rtObjects // the runtime's object files: [0] as gocpp build compiles it, [1] with -nochecks
	pch [2]pchFile   // the precompiled rt.hpp, for the same two modes
}

// pchFile is the precompiled gocpp/rt.hpp for one compiler and mode, built once by pchArgs.
// Parsing rt.hpp and the standard headers it includes is nearly all of the time to compile a
// small generated program (about 1 s of 1.05 s), and the tests compile thousands of them.
type pchFile struct {
	once   sync.Once
	header string // the file that includes gocpp/rt.hpp
	pch    string // header + ".pch" (clang++) or ".gch" (g++)
	err    error
}

// rtObjects are the runtime's object files for one compiler and one mode, compiled once by
// runtimeObjectsMode.
type rtObjects struct {
	once sync.Once
	objs []string
	err  error
}

var (
	cxxOnce sync.Once
	cxxList []*cxx
	cxxErr  error
)

// compilers returns the compilers to test with.
func compilers(t *testing.T) []*cxx {
	t.Helper()
	cxxOnce.Do(func() { cxxList, cxxErr = findCompilers(os.Getenv("GOCPP_CXX"), os.Getenv("GOCPP_REQUIRE_CXX")) })
	if cxxErr != nil {
		t.Fatal(cxxErr)
	}
	return cxxList
}

func findCompilers(env, required string) ([]*cxx, error) {
	names := []string{"g++", "clang++"}
	if env != "" {
		names = []string{env}
	}
	var list []*cxx
	for _, n := range names {
		c := &cxx{name: filepath.Base(n)}
		cc, err := build.LookCXX(context.Background(), n)
		if err != nil {
			c.missing = err.Error()
		}
		c.cc = cc
		list = append(list, c)
	}
	for _, want := range strings.Split(required, ",") {
		want = strings.TrimSpace(want)
		if want == "" {
			continue
		}
		i := slices.IndexFunc(list, func(c *cxx) bool { return c.name == want || (env != "" && env == want) })
		switch {
		case i < 0 && env != "":
			return nil, fmt.Errorf("GOCPP_REQUIRE_CXX=%s needs %s, but GOCPP_CXX=%s tests only that compiler", required, want, env)
		case i < 0:
			return nil, fmt.Errorf("GOCPP_REQUIRE_CXX=%s needs %s, which the tests don't use: they use g++ and clang++, or the compiler in GOCPP_CXX", required, want)
		case list[i].missing != "":
			return nil, fmt.Errorf("GOCPP_REQUIRE_CXX=%s needs %s, but %s", required, want, list[i].missing)
		}
	}
	return list, nil
}

// bothCompilers returns g++ and clang++ if the tests use both and both are there, or nil.
func bothCompilers(t *testing.T) []*cxx {
	var both []*cxx
	for _, c := range compilers(t) {
		if c.missing == "" && (c.cc.Family == build.GCC || c.cc.Family == build.Clang) {
			both = append(both, c)
		}
	}
	if len(both) != 2 || both[0].cc.Family == both[1].cc.Family {
		return nil
	}
	return both
}

// flags returns the driver's flags for generated C++ with this compiler, without -I: the
// same as gocpp build's (build.Flags and build.GeneratedFlags), with more, and then
// GOCPP_TEST_CXXFLAGS (testFlags).
func (c *cxx) flags(more ...string) []string {
	return slices.Concat(build.Flags(c.cc, false), build.GeneratedFlags(c.cc), more, testFlags())
}

// runtimeFlags returns the driver's flags for the runtime's own files, with more, and then
// GOCPP_TEST_CXXFLAGS.
func (c *cxx) runtimeFlags(more ...string) []string {
	return slices.Concat(build.Flags(c.cc, false), more, testFlags())
}

// testFlags returns the flags of GOCPP_TEST_CXXFLAGS (split at spaces), which these tests add
// to every compile and link, after the driver's, so that GOCPP_TEST_CXXFLAGS="-O2
// -fsanitize=address,undefined -fno-sanitize-recover=undefined" runs them optimized and with
// the sanitizers.
func testFlags() []string {
	return strings.Fields(os.Getenv("GOCPP_TEST_CXXFLAGS"))
}

var (
	rtDirOnce sync.Once
	rtDir     string
	rtDirErr  error
)

// runtimeDir returns a folder with the embedded runtime (rtfiles.FS), extracted once for the
// whole test binary, as the driver will extract it.
func runtimeDir(t *testing.T) string {
	t.Helper()
	rtDirOnce.Do(func() {
		rtDir, rtDirErr = os.MkdirTemp("", "cppgen-rt-")
		if rtDirErr != nil {
			return
		}
		rtDirErr = fs.WalkDir(rtfiles.FS(), ".", func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			dst := filepath.Join(rtDir, filepath.FromSlash(p))
			if d.IsDir() {
				return os.MkdirAll(dst, 0o755)
			}
			b, err := fs.ReadFile(rtfiles.FS(), p)
			if err != nil {
				return err
			}
			return os.WriteFile(dst, b, 0o644)
		})
	})
	if rtDirErr != nil {
		t.Fatal(rtDirErr)
	}
	return rtDir
}

// TestMain removes the extracted runtime at the end.
func TestMain(m *testing.M) {
	code := m.Run()
	if rtDir != "" {
		os.RemoveAll(rtDir)
	}
	os.Exit(code)
}

// includeFlag returns -I for the runtime's headers.
func includeFlag(t *testing.T) string {
	return "-I" + filepath.Join(runtimeDir(t), "include")
}

// noChecksFlags returns the flags that gocpp build -nochecks adds to every compile with this
// compiler (build.CompileFlags).
func (c *cxx) noChecksFlags() []string {
	return build.CompileFlags(c.cc, false, true)[len(build.Flags(c.cc, false)):]
}

// runtimeObjectsMode compiles the runtime's .cpp files (rtfiles.Sources) once for this compiler
// and returns the object files; with nochecks, for a -nochecks build (compiled with
// noChecksFlags, into a folder of its own).
func (c *cxx) runtimeObjectsMode(t *testing.T, nochecks bool) []string {
	t.Helper()
	rt := runtimeDir(t)
	mode, folder, more := 0, "obj-"+c.name, []string(nil)
	if nochecks {
		mode, folder, more = 1, "obj-nochecks-"+c.name, c.noChecksFlags()
	}
	r := &c.rt[mode]
	r.once.Do(func() {
		dir := filepath.Join(rt, folder)
		if r.err = os.MkdirAll(dir, 0o755); r.err != nil {
			return
		}
		for i, src := range rtfiles.Sources(runtime.GOOS) {
			obj := filepath.Join(dir, fmt.Sprintf("%d.o", i))
			args := append(c.runtimeFlags(append([]string{includeFlag(t)}, more...)...), "-c", filepath.Join(rt, filepath.FromSlash(src)), "-o", obj)
			out, _, code, err := command(dir, true, c.cc.Path, args...)
			if err == nil && code != 0 {
				err = fmt.Errorf("exit code %d", code)
			}
			if err != nil {
				r.err = fmt.Errorf("%s %s: %v\n%s", c.cc.Path, strings.Join(args, " "), err, out)
				return
			}
			r.objs = append(r.objs, obj)
		}
	})
	if r.err != nil {
		t.Fatal(r.err)
	}
	return r.objs
}

// build compiles the C++ file src (generated code) with the runtime into a program and
// returns its path. The compiler must print nothing: no errors, and no warnings either.
func (c *cxx) build(t *testing.T, src string) string {
	t.Helper()
	return c.buildMode(t, src, false)
}

// pchArgs builds the precompiled rt.hpp for this compiler and mode once, with flags (what a
// compile of generated code gets before its source, as buildMode writes them: the compile
// must have the same flags as the header or the compiler can't use it), and returns the
// flags that make a compile use it. Without GOCPP_NOPCH.
func (c *cxx) pchArgs(t *testing.T, nochecks bool, flags []string) []string {
	t.Helper()
	if os.Getenv("GOCPP_NOPCH") != "" || (c.cc.Family != build.Clang && c.cc.Family != build.GCC) {
		return nil
	}
	mode, ext := 0, ".gch"
	if nochecks {
		mode = 1
	}
	if c.cc.Family == build.Clang {
		ext = ".pch"
	}
	p := &c.pch[mode]
	p.once.Do(func() {
		dir := filepath.Join(runtimeDir(t), fmt.Sprintf("pch-%s-%d", c.name, mode))
		if p.err = os.MkdirAll(dir, 0o755); p.err != nil {
			return
		}
		p.header = filepath.Join(dir, "rt.hpp")
		p.pch = p.header + ext
		if p.err = os.WriteFile(p.header, []byte("#include <gocpp/rt.hpp>\n"), 0o644); p.err != nil {
			return
		}
		args := append(slices.Clone(flags), "-x", "c++-header", p.header, "-o", p.pch)
		out, _, code, err := command(dir, true, c.cc.Path, args...)
		if err != nil || code != 0 || len(out) != 0 {
			p.err = fmt.Errorf("%s %s: exit code %d %v\n%s", c.cc.Path, strings.Join(args, " "), code, err, out)
		}
	})
	if p.err != nil {
		t.Fatal(p.err)
	}
	if c.cc.Family == build.Clang {
		return []string{"-include-pch", p.pch}
	}
	return []string{"-include", p.header}
}

// buildMode is build, with nochecks for a -nochecks build: the program and the runtime are
// compiled with noChecksFlags, and the program's name ends with "-nochecks".
func (c *cxx) buildMode(t *testing.T, src string, nochecks bool) string {
	t.Helper()
	objs := c.runtimeObjectsMode(t, nochecks)
	bin := strings.TrimSuffix(src, ".cpp") + "-" + c.name
	more := []string{includeFlag(t)}
	if nochecks {
		bin += "-nochecks"
		more = append(more, c.noChecksFlags()...)
	}
	args := c.flags(more...)
	args = append(args, c.pchArgs(t, nochecks, args)...)
	args = append(args, src)
	args = append(args, objs...)
	args = append(args, "-o", bin)
	out, _, code, err := command(filepath.Dir(src), true, c.cc.Path, args...)
	if err != nil || code != 0 || len(out) != 0 {
		t.Fatalf("%s %s: exit code %d %v\n%s", c.cc.Path, strings.Join(args, " "), code, err, out)
	}
	return bin
}

// syntaxOK reports whether the compiler accepts the C++ file src (-fsyntax-only), with the
// driver's flags and more, and returns what it printed. It may run in any goroutine.
func (c *cxx) syntaxOK(src string, more ...string) (bool, string, error) {
	args := append(c.flags(more...), "-fsyntax-only", src)
	out, _, code, err := command(filepath.Dir(src), true, c.cc.Path, args...)
	return code == 0, string(out), err
}

// A result is what a program printed and its exit code.
type result struct {
	stdout, stderr string
	code           int
}

// run runs the program bin in the folder dir. Its stderr ends with the goroutine line of a
// panic: the stack trace after it differs between Go and gocpp (paths, offsets, frames), so
// the tests compare the output up to it.
func run(t *testing.T, dir, bin string) result {
	t.Helper()
	return runEnv(t, dir, nil, bin)
}

// runEnv is run with the environment changes env (KEY=value) on top of the compiler's.
func runEnv(t *testing.T, dir string, env []string, bin string) result {
	t.Helper()
	stdout, stderr, code, err := commandEnv(dir, append(build.CompilerEnv(), env...), false, bin)
	if err != nil {
		t.Fatal(err)
	}
	e := string(stderr)
	if i := strings.Index(e, goStackMark); i >= 0 {
		e = e[:i+len(goStackMark)]
	}
	return result{string(stdout), e, code}
}

// command runs a command with a timeout in the folder dir, in the compiler's environment
// (build.CompilerEnv: LC_ALL=C, so that the messages are the same everywhere). combined
// merges stdout and stderr into one stream; otherwise they are separate.
func command(dir string, combined bool, name string, args ...string) (stdout, stderr []byte, code int, err error) {
	return commandEnv(dir, build.CompilerEnv(), combined, name, args...)
}

// goCommand runs the go command (goCmd) with the arguments in dir, its output combined, with
// GOAMD64=v1: from v3 on gc fuses a*b + c into one instruction, which rounds differently, and
// gocpp gives the results without fusing (docs/spec/cpp-mapping.md, "Complex numbers").
func goCommand(dir string, args ...string) (stdout, stderr []byte, code int, err error) {
	return commandEnv(dir, append(build.CompilerEnv(), "GOAMD64=v1"), true, goCmd, args...)
}

// commandEnv is command with the environment env.
func commandEnv(dir string, env []string, combined bool, name string, args ...string) (stdout, stderr []byte, code int, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	cmd.Env = env
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	if combined {
		cmd.Stderr = &out
	} else {
		cmd.Stderr = &errb
	}
	err = cmd.Run()
	var exit *exec.ExitError
	if errors.As(err, &exit) && exit.Exited() {
		return out.Bytes(), errb.Bytes(), exit.ExitCode(), nil
	}
	if err != nil {
		return out.Bytes(), errb.Bytes(), -1, fmt.Errorf("%s: %v\n%s%s", name, err, out.Bytes(), errb.Bytes())
	}
	return out.Bytes(), errb.Bytes(), 0, nil
}

// parallel runs f(i) for i in [0, n) on all CPUs.
func parallel(n int, f func(i int)) {
	sem := make(chan struct{}, runtime.NumCPU())
	var wg sync.WaitGroup
	for i := range n {
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			f(i)
		}()
	}
	wg.Wait()
}
