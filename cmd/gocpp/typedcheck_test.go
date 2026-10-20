package main

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// The checker's zoo (internal/typedinterop/icheck/testdata/zoo) against the real C++
// compilers: what the typed checker rejects, the opaque build fails on at the same
// lines (with clang++ and with g++), and what it accepts the build compiles.

// zooAbs is the zoo's folder, found when the test binary starts (the tests change folder).
var zooAbs = func() string {
	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	return filepath.Join(wd, "..", "..", "internal", "typedinterop", "icheck", "testdata", "zoo")
}()

func zooDir(t *testing.T) string {
	t.Helper()
	return zooAbs
}

// zooIn copies the headers and the programs of the zoo to a new folder and makes it the
// current folder.
func zooIn(t *testing.T) {
	t.Helper()
	files := map[string]string{}
	entries, err := os.ReadDir(zooDir(t))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".gp") || strings.HasSuffix(e.Name(), ".hpp") {
			b, err := os.ReadFile(filepath.Join(zooDir(t), e.Name()))
			if err != nil {
				t.Fatal(err)
			}
			files[e.Name()] = string(b)
		}
	}
	inDir(t, files)
}

var errLineRe = regexp.MustCompile(`(?m)^\./[^\s:]+\.gp:(\d+)(?::\d+)?: `)

func errorLines(stderr string) []string {
	var out []string
	seen := map[string]bool{}
	for _, m := range errLineRe.FindAllStringSubmatch(stderr, -1) {
		if !seen[m[1]] {
			seen[m[1]] = true
			out = append(out, m[1])
		}
	}
	sort.Strings(out)
	return out
}

// buildLines builds the program and returns the lines of its errors, nil when it builds.
func buildLines(t *testing.T, name string) map[string]bool {
	t.Helper()
	br := gocppRun("", false, "build", "-o", filepath.Join(t.TempDir(), "x"), name)
	if br.code == 0 {
		return nil
	}
	out := map[string]bool{}
	for _, l := range errorLines(br.stderr) {
		out[l] = true
	}
	if len(out) == 0 {
		t.Fatalf("%s: the build failed without an error at a line of the program:\n%s", name, br.stderr)
	}
	return out
}

func zooPrograms(t *testing.T, prefix string) []string {
	m, err := filepath.Glob(filepath.Join(zooDir(t), prefix+"*.gp"))
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, f := range m {
		out = append(out, filepath.Base(f))
	}
	sort.Strings(out)
	return out
}

func zooEnv(t *testing.T) {
	t.Setenv("GOCPP_MODCACHE", "off")
	t.Setenv("GOCPP_OFFLINE", "1")
	t.Setenv("GOCPP_PROBE_TIMEOUT", "5m")
	t.Setenv("GOCPP_NOPROBECACHE", "1")
	t.Setenv("GOCPP_CLANG", matrixClang)
}

// needClang skips the test when typed interop has no clang++ (it falls back to opaque).
func needClang(t *testing.T) {
	t.Helper()
	r := gocppRun("", false, "__typed-check", "-build", "g++", "-e", "check_header.gp")
	if r.code == 3 {
		t.Skip("no clang++ for the probe")
	}
}

// Every error of the typed checker is a line where the C++ build fails, and
// every program it accepts builds, with both compilers.
func TestTypedCheckZoo(t *testing.T) {
	forEachCXX(t, func(t *testing.T, c testCXX) {
		zooEnv(t)
		t.Setenv("XDG_CACHE_HOME", t.TempDir())
		zooIn(t)
		needClang(t)
		fam := "clang++"
		if strings.HasSuffix(c.name, "g++") {
			fam = "g++"
		}
		for _, name := range zooPrograms(t, "reject_") {
			t.Run(strings.TrimSuffix(name, ".gp"), func(t *testing.T) {
				// The typed checker (with the build's compiler: errors with clang++, warnings with g++).
				tr := gocppRun("", false, "__typed-check", "-build", fam, "-e", name)
				wantCode := 1
				if fam == "g++" {
					wantCode = 0 // warnings only
				}
				if tr.code != wantCode {
					t.Fatalf("__typed-check -build %s: exit %d, want %d\n%s", fam, tr.code, wantCode, tr.stderr)
				}
				typed := errorLines(tr.stderr)
				if len(typed) == 0 {
					t.Fatalf("no diagnostic:\n%s", tr.stderr)
				}
				if fam == "g++" && !strings.Contains(tr.stderr, ": warning: ") {
					t.Errorf("no warning with g++:\n%s", tr.stderr)
				}
				// The real build fails at those lines. The lowering stops at its own errors
				// (an unknown name, a value it cannot convert), before the C++ compiler runs,
				// so a line that the full build does not reach is built alone: the program
				// with the other lines of the checker blanked (each is a statement of its own).
				full := buildLines(t, name)
				if full == nil {
					t.Fatalf("the program builds with %s, the checker was wrong:\n%s", c.name, tr.stderr)
				}
				src, err := os.ReadFile(name)
				if err != nil {
					t.Fatal(err)
				}
				for _, l := range typed {
					if full[l] {
						continue
					}
					lines := strings.Split(string(src), "\n")
					for _, other := range typed {
						if other != l {
							n, _ := strconv.Atoi(other)
							lines[n-1] = ""
						}
					}
					variant := strings.TrimSuffix(name, ".gp") + "_line" + l + ".gp"
					if err := os.WriteFile(variant, []byte(strings.Join(lines, "\n")), 0o644); err != nil {
						t.Fatal(err)
					}
					if got := buildLines(t, variant); !got[l] {
						t.Errorf("the checker reports line %s of %s, the %s build does not (alone, or in the whole program):\nchecker:\n%s\nbuild of the line alone: %v", l, name, c.name, tr.stderr, got)
					}
				}
			})
		}
		for _, name := range zooPrograms(t, "accept_") {
			t.Run(strings.TrimSuffix(name, ".gp"), func(t *testing.T) {
				tr := gocppRun("", false, "__typed-check", "-build", fam, "-e", name)
				if tr.code != 0 || strings.TrimSpace(tr.stderr) != "" {
					t.Errorf("__typed-check: exit %d\n%s", tr.code, tr.stderr)
				}
				br := gocppRun("", false, "build", "-o", filepath.Join(t.TempDir(), "x"), name)
				if br.code != 0 {
					t.Errorf("the program does not build with %s:\n%s", c.name, br.stderr)
				}
			})
		}
	})
}

// A header that is not there and a program that fails the opaque check are
// reported by the check itself; the typed checker adds nothing (exit status 1, no
// typed message).
func TestTypedCheckStaysBehindTheCheck(t *testing.T) {
	zooEnv(t)
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	zooIn(t)
	r := gocppRun("", false, "__typed-check", "check_header.gp")
	if r.code != 1 || !strings.Contains(r.stderr, "cannot find header <nosuchheader.h>") {
		t.Errorf("check_header.gp: %+v", r)
	}
}
