//go:build linux

package rtfiles_test

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/huuhait/gocpp/internal/build"
)

// winGoPrograms are programs of tests/run that TestWindowsGoPrograms builds for Windows and runs under
// Wine: hello-style output, goroutines (many, nested, a tree of them), channels, select, a deep
// recursion on a goroutine's stack, panics (exit code 2 and Go's message), recover.
var winGoPrograms = []string{
	"printf_verbs", "go_many", "go_tree", "go_recover", "go_panic_exit", "go_stack_depth",
	"chan_fan_in_out", "chan_many_goroutines", "chan_panic_close_nil", "range_chan_pipeline",
	"select_rendezvous", "select_worker_quit", "select_1000_cases", "select_panic_send_closed",
	"panic_index", "defer_panic",
}

// A runExpect is what a tests/run program must do: the // Output: block, // ExitCode: and
// // Stderr: lines (tests/run/README.md).
type runExpect struct {
	out    string
	code   int
	stderr []*regexp.Regexp
}

func parseRunExpect(src string) runExpect {
	var e runExpect
	inOut := false
	var out []string
	for _, line := range strings.Split(strings.ReplaceAll(src, "\r\n", "\n"), "\n") {
		switch {
		case inOut:
			l := strings.TrimPrefix(line, "//")
			l = strings.TrimPrefix(l, " ")
			out = append(out, strings.TrimRight(l, " \t"))
		case strings.HasPrefix(line, "// Output:"):
			inOut = true
			if rest := strings.TrimSpace(strings.TrimPrefix(line, "// Output:")); rest != "" {
				out = append(out, rest)
			}
		case strings.HasPrefix(line, "// ExitCode:"):
			n, err := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(line, "// ExitCode:")))
			if err != nil {
				panic(err)
			}
			e.code = n
		case strings.HasPrefix(line, "// Stderr:"):
			rx, err := regexp.Compile(strings.TrimSpace(strings.TrimPrefix(line, "// Stderr:")))
			if err != nil {
				panic(err)
			}
			e.stderr = append(e.stderr, rx)
		}
	}
	e.out = strings.TrimSpace(strings.Join(out, "\n"))
	return e
}

// TestWindowsGoPrograms builds Go++ programs for Windows (gocpp emit's C++, MinGW-w64, the runtime's
// Windows layer, -static) and runs them under Wine: the output, the exit code and the stderr
// must be what the program's expectations say (the same ones that the Linux harness holds it to).
// Phase 06 step 6.13 gives the driver the same job (gocpp build with GOOS=windows); this test
// is the runtime's own check, which doesn't need the driver.
func TestWindowsGoPrograms(t *testing.T) {
	c := mingw(t)
	e := wineSetup(t)
	rt := extractRuntime(t)
	gocpp := buildGocpp(t)
	flags := winExtraFlags

	dir := t.TempDir()
	type prep struct {
		name   string
		bin    string
		expect runExpect
	}
	names := winGoPrograms
	preps := make([]*prep, len(names))
	for i, n := range names {
		preps[i] = &prep{name: n}
	}
	// Emit, compile and link all programs at the same time (the gate bounds the commands).
	err := parallel(len(preps), func(i int) error {
		p := preps[i]
		src := sourcePath2(filepath.Join("..", "tests", "run", p.name+".gp"))
		text, err := os.ReadFile(src)
		if err != nil {
			return err
		}
		p.expect = parseRunExpect(string(text))
		env := append(build.CompilerEnv(), "GOCPP_GATE_PHASE=08", "GOCPP_MODCACHE=off", "GOCPP_OFFLINE=1", "XDG_CACHE_HOME="+filepath.Join(dir, "cache"))
		out, stderr, code, err := commandPrio(prioRT, dir, env, false, gocpp, "emit", src)
		if err != nil || code != 0 {
			return fmt.Errorf("%s: gocpp emit: exit %d %v\n%s", p.name, code, err, stderr)
		}
		gen := filepath.Join(dir, p.name, "gen")
		files := splitEmit(string(out))
		var srcs []string
		for path, text := range files {
			full := filepath.Join(gen, filepath.FromSlash(path))
			if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
				return err
			}
			if err := os.WriteFile(full, []byte(text), 0o644); err != nil {
				return err
			}
			if strings.HasSuffix(path, ".cpp") {
				srcs = append(srcs, full)
			}
		}
		objs, err := c.runtimeObjects(rt, flags)
		if err != nil {
			return err
		}
		var mine []string
		for j, s := range srcs {
			obj := filepath.Join(dir, p.name, fmt.Sprintf("%d.o", j))
			args := append(slicesConcat(winDriverFlags, flags), "-I", filepath.Join(rt, "include"), "-I", gen, "-c", s, "-o", obj)
			if out, _, code, err := commandPrio(prioRT, dir, build.CompilerEnv(), true, c.cc.Path, args...); err != nil || code != 0 {
				return fmt.Errorf("%s: compile %s: exit %d %v\n%s", p.name, s, code, err, out)
			}
			mine = append(mine, obj)
		}
		p.bin = filepath.Join(dir, p.name+".exe")
		args := append(slicesConcat(winDriverFlags, flags), mine...)
		args = append(args, objs...)
		args = append(args, "-o", p.bin)
		if out, _, code, err := commandPrio(prioRun, dir, build.CompilerEnv(), true, c.cc.Path, args...); err != nil || code != 0 {
			return fmt.Errorf("%s: link: exit %d %v\n%s", p.name, code, err, out)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range preps {
		t.Run(p.name, func(t *testing.T) {
			r := e.runLong(t, p.bin, nil)
			if r.code != p.expect.code {
				t.Errorf("exit code %d, want %d\nstdout:\n%.2000s\nstderr:\n%.2000s", r.code, p.expect.code, r.stdout, r.stderr)
			}
			if got := strings.TrimSpace(string(bytes.ReplaceAll(r.stdout, []byte("\r\n"), []byte("\n")))); got != p.expect.out {
				t.Errorf("stdout:\n%.2000s\nwant:\n%.2000s", got, p.expect.out)
			}
			if bytes.Contains(r.stdout, []byte("\r")) {
				t.Errorf("stdout has a \\r: Go writes \\n only")
			}
			if len(p.expect.stderr) == 0 && len(r.stderr) != 0 {
				t.Errorf("stderr is not empty:\n%.2000s", r.stderr)
			}
			for _, rx := range p.expect.stderr {
				if !rx.Match(r.stderr) {
					t.Errorf("stderr does not match %s:\n%.2000s", rx, r.stderr)
				}
			}
		})
	}
}

// winDriverFlags are the driver's flags for generated code (no -Werror: the runtime's own files
// are built with it, the generated code is the driver's concern).
var winDriverFlags = []string{"-std=c++20", "-fwrapv", "-fno-strict-aliasing", "-ffp-contract=off", "-fstack-clash-protection", "-g"}

func slicesConcat(a, b []string) []string {
	return append(append([]string{}, a...), b...)
}

func sourcePath2(p string) string {
	abs, err := filepath.Abs(p)
	if err != nil {
		panic(err)
	}
	return abs
}

// splitEmit splits the output of gocpp emit (files one after the other, each after a line
// "//// <path>") into its files.
func splitEmit(out string) map[string]string {
	files := make(map[string]string)
	var name string
	var b strings.Builder
	flush := func() {
		if name != "" {
			files[name] = b.String()
		}
		b.Reset()
	}
	for _, line := range strings.SplitAfter(out, "\n") {
		if p, ok := strings.CutPrefix(line, "//// "); ok {
			flush()
			name = strings.TrimSpace(p)
			continue
		}
		b.WriteString(line)
	}
	flush()
	return files
}

var gocppOnce struct {
	once sync.Once
	bin  string
	err  error
}

// buildGocpp builds cmd/gocpp once for the test binary and returns the program.
func buildGocpp(t *testing.T) string {
	t.Helper()
	gocppOnce.once.Do(func() {
		dir, err := os.MkdirTemp("", "gocpp-bin-")
		if err != nil {
			gocppOnce.err = err
			return
		}
		atExit = append(atExit, func() { os.RemoveAll(dir) })
		bin := filepath.Join(dir, "gocpp")
		out, _, code, err := commandPrio(prioRT, "..", nil, true, "go", "build", "-o", bin, "./cmd/gocpp")
		if err != nil || code != 0 {
			gocppOnce.err = fmt.Errorf("go build ./cmd/gocpp: exit %d %v\n%s", code, err, out)
			return
		}
		gocppOnce.bin = bin
	})
	if gocppOnce.err != nil {
		t.Fatal(gocppOnce.err)
	}
	return gocppOnce.bin
}
