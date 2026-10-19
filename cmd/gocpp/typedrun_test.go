package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"
)

// The programs of tests/typed (Phase 12 step 12.6, ADR-087): what typed code generation can do and
// opaque mode cannot, or does differently. Each is built and run with GOCPP_TYPED_INTEROP=1 with
// both compilers and prints what its C++ twin (tests/typed/twin/NAME.cpp, compiled with both
// compilers by TestTypedTwin) prints: the expected output is C++'s, not the lowering's own.
//
// A program starts with `// phase: 12` and, under it, the lines
//
//	// opaque: reject   the program does not build in opaque mode (what typed mode adds)
//	// opaque: same     the program builds in opaque mode and prints the same
//	// pkg-config: l    skipped without the libraries
//	// build-only       built with both compilers, not run (a window or a display)
//	// error: regexp    the typed build fails, with an error that matches (an error test; no Output)
//
// and, at the end, `// Output:` and the lines of the output behind `// `.

type typedProgram struct {
	name      string
	file      string
	opaque    string // "reject", "same" or ""
	libs      []string
	buildOnly bool
	errorRE   *regexp.Regexp
	want      string
	hasWant   bool
	windows   bool
}

var typedDir = func() string {
	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	return filepath.Join(wd, "..", "..", "tests", "typed")
}()

func loadTypedPrograms(t *testing.T) []typedProgram {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(typedDir, "*.gp"))
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(files)
	var out []typedProgram
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		p := typedProgram{name: strings.TrimSuffix(filepath.Base(f), ".gp"), file: filepath.Base(f)}
		lines := strings.Split(string(b), "\n")
		inOutput := false
		var want []string
		for _, l := range lines {
			switch {
			case inOutput:
				if rest, ok := strings.CutPrefix(l, "// "); ok {
					want = append(want, rest)
				} else if l == "//" {
					want = append(want, "")
				}
			case l == "// Output:":
				inOutput, p.hasWant = true, true
			case strings.HasPrefix(l, "// opaque: "):
				p.opaque = strings.TrimPrefix(l, "// opaque: ")
			case strings.HasPrefix(l, "// pkg-config: "):
				p.libs = strings.Fields(strings.TrimPrefix(l, "// pkg-config: "))
			case l == "// build-only":
				p.buildOnly = true
			case l == "// windows: yes":
				p.windows = true
			case strings.HasPrefix(l, "// error: "):
				re, err := regexp.Compile(strings.TrimPrefix(l, "// error: "))
				if err != nil {
					t.Fatalf("%s: %v", f, err)
				}
				p.errorRE = re
			}
		}
		p.want = strings.Join(want, "\n")
		if p.hasWant {
			p.want += "\n"
		}
		if !p.hasWant && p.errorRE == nil && !p.buildOnly {
			t.Fatalf("%s: no // Output:, // error: or // build-only", f)
		}
		out = append(out, p)
	}
	if len(out) == 0 {
		t.Fatal("no programs in tests/typed")
	}
	return out
}

// typedIn copies the programs and headers of tests/typed to a new folder and makes it the current
// folder.
func typedIn(t *testing.T) {
	t.Helper()
	files := map[string]string{}
	entries, err := os.ReadDir(typedDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		b, err := os.ReadFile(filepath.Join(typedDir, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		files[e.Name()] = string(b)
	}
	inDir(t, files)
}

func typedEnv(t *testing.T) {
	t.Helper()
	t.Setenv("GOCPP_MODCACHE", "off")
	t.Setenv("GOCPP_OFFLINE", "1")
	t.Setenv("GOCPP_PROBE_TIMEOUT", "5m")
	t.Setenv("GOCPP_NOPROBECACHE", "1")
	t.Setenv("GOCPP_CLANG", "")
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
}

func missingLibs(libs []string) string {
	if len(libs) == 0 {
		return ""
	}
	if err := exec.Command("pkg-config", append([]string{"--exists"}, libs...)...).Run(); err != nil {
		return "pkg-config does not find " + strings.Join(libs, " ")
	}
	return ""
}

// TestTypedRun builds and runs each program of tests/typed in typed mode with each compiler.
func TestTypedRun(t *testing.T) {
	progs := loadTypedPrograms(t)
	forEachCXX(t, func(t *testing.T, c testCXX) {
		typedEnv(t)
		typedIn(t)
		needClang(t)
		for _, p := range progs {
			t.Run(p.name, func(t *testing.T) {
				if why := missingLibs(p.libs); why != "" {
					t.Skip(why)
				}
				t.Setenv("GOCPP_TYPED_INTEROP", "1")
				switch {
				case p.errorRE != nil:
					r := gocppRun("", false, "build", "-o", filepath.Join(t.TempDir(), "x"), p.file)
					if r.code == 0 {
						t.Fatalf("the typed build succeeded; want an error that matches %s", p.errorRE)
					}
					if !p.errorRE.MatchString(r.stderr) {
						t.Fatalf("the typed build's errors do not match %s:\n%s", p.errorRE, r.stderr)
					}
				case p.buildOnly:
					if r := gocppRun("", false, "build", "-o", filepath.Join(t.TempDir(), "x"), p.file); r.code != 0 {
						t.Fatalf("typed build: exit %d\n%s", r.code, r.stderr)
					}
				default:
					r := gocppRun("", false, "run", p.file)
					if r.code != 0 || r.stderr != "" {
						t.Fatalf("typed run: exit %d\nstderr:\n%s\nstdout:\n%s", r.code, r.stderr, r.stdout)
					}
					if r.stdout != p.want {
						t.Fatalf("typed run: stdout differs from the // Output: block:\ngot:\n%s\nwant:\n%s", r.stdout, p.want)
					}
				}
				// What opaque mode does with the same program.
				if p.opaque == "" || p.errorRE != nil {
					return
				}
				t.Setenv("GOCPP_TYPED_INTEROP", "")
				switch p.opaque {
				case "reject":
					if r := gocppRun("", false, "build", "-o", filepath.Join(t.TempDir(), "x"), p.file); r.code == 0 {
						t.Fatalf("the opaque build succeeded: the program shows nothing that typed mode adds (remove // opaque: reject or fix the test)")
					}
				case "same":
					if p.buildOnly {
						return
					}
					r := gocppRun("", false, "run", p.file)
					if r.code != 0 || r.stdout != p.want {
						t.Fatalf("opaque run: exit %d, the output differs from typed mode's:\ngot:\n%s\nstderr:\n%s", r.code, r.stdout, r.stderr)
					}
				default:
					t.Fatalf("// opaque: %s: want reject or same", p.opaque)
				}
			})
		}
	})
}

// TestTypedTwin compiles the C++ twin of each program of tests/typed with each compiler and checks
// that it prints the program's expected output: the expectations are C++'s.
func TestTypedTwin(t *testing.T) {
	progs := loadTypedPrograms(t)
	forEachCXX(t, func(t *testing.T, c testCXX) {
		for _, p := range progs {
			twin := filepath.Join(typedDir, "twin", p.name+".cpp")
			if _, err := os.Stat(twin); err != nil {
				if p.hasWant {
					t.Errorf("%s has no C++ twin (tests/typed/twin/%s.cpp)", p.name, p.name)
				}
				continue
			}
			t.Run(p.name, func(t *testing.T) {
				if why := missingLibs(p.libs); why != "" {
					t.Skip(why)
				}
				dir := t.TempDir()
				exe := filepath.Join(dir, "twin")
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
				defer cancel()
				args := append([]string{"-std=c++20", "-Wall", "-o", exe, twin}, c.cxx.Args...)
				if len(p.libs) > 0 {
					out, err := exec.Command("pkg-config", append([]string{"--cflags", "--libs"}, p.libs...)...).Output()
					if err != nil {
						t.Skip("pkg-config: " + err.Error())
					}
					args = append(args, strings.Fields(string(out))...)
				}
				cmd := exec.CommandContext(ctx, c.cxx.Path, args...)
				var stderr bytes.Buffer
				cmd.Stderr = &stderr
				if err := cmd.Run(); err != nil {
					t.Fatalf("the twin does not compile: %v\n%s", err, stderr.String())
				}
				if stderr.Len() > 0 {
					t.Errorf("the twin has warnings:\n%s", stderr.String())
				}
				if p.buildOnly || p.errorRE != nil {
					return
				}
				out, err := exec.CommandContext(ctx, exe).Output()
				if err != nil {
					t.Fatalf("the twin fails: %v", err)
				}
				if string(out) != p.want {
					t.Fatalf("the twin prints:\n%s\nthe program's // Output: block is:\n%s", out, p.want)
				}
			})
		}
	})
}
