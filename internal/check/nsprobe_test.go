package check

import (
	"context"
	"errors"
	"go/ast"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/huuhait/gocpp/internal/build"
	"github.com/huuhait/gocpp/internal/interop"
	"github.com/huuhait/gocpp/internal/load"
	"github.com/huuhait/gocpp/stdlib"
)

// The tests of the namespace probe in Check (nsprobe.go) run the real
// compilers, g++ and clang++, by the rules of the test harness
// (tests/README.md, "Compilers"), like internal/interop's tests:
// GOCPP_CXX tests with that compiler only, a compiler that isn't installed
// is a skip, and a compiler that GOCPP_REQUIRE_CXX names must be tested
// ("make ci" sets g++,clang++), so there a missing one fails. The
// programs avoid what g++ and clang++ answer differently (std with only C
// headers, interop.ProbeNamespaces).

// forEachCompiler runs fn once per compiler of the matrix, as subtests
// named after the compilers.
func forEachCompiler(t *testing.T, fn func(t *testing.T, cxx build.CXX)) {
	t.Helper()
	matrix := []string{"g++", "clang++"}
	env := os.Getenv("GOCPP_CXX")
	if env != "" {
		matrix = []string{env}
	}
	var required []string
	for want := range strings.SplitSeq(os.Getenv("GOCPP_REQUIRE_CXX"), ",") {
		if want = strings.TrimSpace(want); want == "" {
			continue
		}
		if !slices.ContainsFunc(matrix, func(prog string) bool { return prog == want || filepath.Base(prog) == want }) {
			if env != "" {
				t.Fatalf("GOCPP_REQUIRE_CXX needs %s, but GOCPP_CXX=%s tests only that compiler", want, env)
			}
			t.Fatalf("GOCPP_REQUIRE_CXX needs %s, which the tests don't use: they use g++ and clang++, or the compiler in GOCPP_CXX", want)
		}
		required = append(required, want)
	}
	for _, prog := range matrix {
		name := filepath.Base(prog)
		t.Run(name, func(t *testing.T) {
			if _, err := exec.LookPath(prog); err != nil {
				if slices.Contains(required, prog) || slices.Contains(required, name) {
					t.Fatalf("GOCPP_REQUIRE_CXX needs %s, but it is not installed: %v", prog, err)
				}
				t.Skipf("%s is not installed: %v", prog, err)
			}
			cxx, err := build.LookCXX(context.Background(), prog)
			if err != nil {
				t.Fatal(err)
			}
			fn(t, cxx)
		})
	}
}

// writeScript writes the shell script src (which starts with #!/bin/sh) to
// path, executable, and runs it once, so that a test can run it without
// "text file busy". A test that forks a process (a compiler) while another
// one writes a script can give that child the script's write fd for a
// moment (between fork and exec, golang/go#22315), and exec fails with
// ETXTBSY while it has it. The run retries that; once one exec succeeded,
// no process has the fd any more. For the run, the line after #!/bin/sh
// exits at once when GOCPP_TEST_WARMUP is 1.
func writeScript(t testing.TB, path, src string) {
	t.Helper()
	body, ok := strings.CutPrefix(src, "#!/bin/sh\n")
	if !ok {
		t.Fatalf("script %s doesn't start with #!/bin/sh", path)
	}
	src = "#!/bin/sh\n[ \"$GOCPP_TEST_WARMUP\" = 1 ] && exit 0\n" + body
	if err := os.WriteFile(path, []byte(src), 0o755); err != nil {
		t.Fatal(err)
	}
	for try := 0; ; try++ {
		cmd := exec.Command(path)
		cmd.Env = append(os.Environ(), "GOCPP_TEST_WARMUP=1")
		err := cmd.Run()
		if err == nil {
			return
		}
		if !errors.Is(err, syscall.ETXTBSY) || try == 50 {
			t.Fatalf("running %s: %v", path, err)
		}
		time.Sleep(time.Duration(try+1) * 10 * time.Millisecond)
	}
}

// probeConfig returns the Config for the phase that probes with the
// compiler cxx.
func probeConfig(cxx build.CXX, phase int) Config {
	return Config{Phase: phase, CXX: func(context.Context) (build.CXX, error) { return cxx, nil }}
}

// noCXX is a Config.CXX that fails the test when Check calls it.
func noCXX(t *testing.T) func(context.Context) (build.CXX, error) {
	return func(context.Context) (build.CXX, error) {
		t.Error("Check looked for a C++ compiler")
		return build.CXX{}, errors.New("no compiler in this test")
	}
}

// A probeRecorder is a Config.Probe that records the calls and passes them
// on to interop.ProbeNamespaces.
type probeRecorder struct {
	mu    sync.Mutex
	calls []probeCall
}

type probeCall struct {
	includes, candidates []string
}

func (r *probeRecorder) probe(ctx context.Context, cxx build.CXX, dir string, flags, includes, candidates []string) (interop.Result, error) {
	r.mu.Lock()
	r.calls = append(r.calls, probeCall{includes, candidates})
	r.mu.Unlock()
	return interop.ProbeNamespaces(ctx, cxx, dir, flags, includes, candidates)
}

// loadTree writes the files into a new module example.com/m and loads it
// (for linux/amd64, with the standard library stubs).
func loadTree(t *testing.T, files map[string]string) (*load.Program, string) {
	t.Helper()
	files["gocpp.mod"] = "module example.com/m\n"
	root := writeTree(t, files)
	prog := load.Load(load.Config{Dir: root, Stdlib: stdlib.FS(), GoStd: stdlib.IsStd, GOOS: "linux", GOARCH: "amd64"}, ".")
	return prog, root
}

// probeErrors loads the files and checks them with cfg; it returns what
// gocpp check -e would print.
func probeErrors(t *testing.T, cfg Config, files map[string]string) string {
	t.Helper()
	prog, root := loadTree(t, files)
	return errorText(t, prog, Check(prog, cfg), root, true)
}

// TestProbeTypo checks a namespace typo: undefined at each use, as go
// build says it for the Go twin where stdd is an undefined name; std is a
// namespace. The uses of stdd are not opaque and not namespace uses any
// more, and the gates and the opaque pass report nothing about them.
func TestProbeTypo(t *testing.T) {
	t.Parallel()
	const src = `package main

import (
	"fmt"
	"<vector>"
)

func main() {
	v := stdd.vector[int]{1, 2}
	w := std.vector[int]{}
	fmt.Println(v, w, stdd.size(v),
		stdd.chrono.seconds(1), stdd.x) // the same error on the same line is shown once, like go build
}
`
	forEachCompiler(t, func(t *testing.T, cxx build.CXX) {
		t.Parallel()
		prog, root := loadTree(t, map[string]string{"main.gp": src})
		r := Check(prog, probeConfig(cxx, LastPhase))
		want := `# example.com/m
./main.gp:9:7: undefined: stdd
./main.gp:11:20: undefined: stdd
./main.gp:12:3: undefined: stdd
`
		if got := errorText(t, prog, r, root, true); got != want {
			t.Errorf("errors:\n%s\nwant:\n%s", got, want)
		}
		f := prog.Roots[0].Files[0]
		if len(f.Rewrite.Namespaces) != 1 || f.Rewrite.Namespaces[0].Name != "std" {
			t.Errorf("namespace candidates after the probe: %v, want only std", f.Rewrite.Namespaces)
		}
		for _, u := range f.Rewrite.Uses {
			if u.Ident.Name != "std" {
				t.Errorf("a use of %s is left in Rewrite.Uses", u.Ident.Name)
			}
		}
		p := mainPkg(t, prog, r)
		lit := func(line int) ast.Expr { return compositeLitAt(t, prog, f, line) }
		if p.Opaque.IsOpaque(lit(9)) || !p.Opaque.IsOpaque(lit(10)) {
			t.Errorf("opaque: stdd.vector[int]{1, 2} %v, std.vector[int]{} %v; want false, true",
				p.Opaque.IsOpaque(lit(9)), p.Opaque.IsOpaque(lit(10)))
		}

		// With the gates of Phase 01: the gate error of the header import,
		// and the same undefined errors. stdd.vector[int]{1, 2} is Go code
		// now, a composite literal of an undefined type, so it gets the
		// gate error of a composite literal, as in the Go twin. (That
		// error's place hides the namespace gate at std.vector[int]{}: one
		// gate error per place, then one per message and file.)
		r = Check(prog, probeConfig(cxx, 1))
		want = `# example.com/m
./main.gp:5:2: header imports are not supported yet (Phase 04)
./main.gp:9:7: undefined: stdd
./main.gp:9:7: composite types are not supported yet (Phase 03)
./main.gp:11:20: undefined: stdd
./main.gp:12:3: undefined: stdd
`
		if got := errorText(t, prog, r, root, true); got != want {
			t.Errorf("Phase 01 errors:\n%s\nwant:\n%s", got, want)
		}
	})
}

// compositeLitAt returns the first composite literal on the line of the
// file f.
func compositeLitAt(t *testing.T, prog *load.Program, f *load.File, line int) ast.Expr {
	t.Helper()
	var lit ast.Expr
	ast.Inspect(f.AST, func(n ast.Node) bool {
		if cl, ok := n.(*ast.CompositeLit); ok && lit == nil && prog.Fset.Position(cl.Pos()).Line == line {
			lit = cl
		}
		return lit == nil
	})
	if lit == nil {
		t.Fatalf("no composite literal on line %d", line)
	}
	return lit
}

// TestProbeTwins checks programs with namespace typos and other errors
// against go build's output for their Go twins (each "<vector>" commented
// out, std.vector[int]{} replaced by []int{}): the same errors, in the
// same order, also with the limit of 10 errors. The want strings come
// from go build 1.25.14 (with and without -gcflags=-e).
func TestProbeTwins(t *testing.T) {
	t.Parallel()
	const src1 = `package main

import (
	"fmt"
	"<vector>"
)

func main() {
	v := stdd.vector[int]{1, 2}
	var n int = "x"
	w := std.vector[int]{}
	fmt.Println(v, n, w, stdd.size(v))
	for _, x := range stdd.view(w) {
		_ = x + "a"
	}
	var m string = 1
	_ = m
}
`
	const want1 = `# example.com/m
./main.gp:9:7: undefined: stdd
./main.gp:10:14: cannot use "x" (untyped string constant) as int value in variable declaration
./main.gp:12:23: undefined: stdd
./main.gp:13:20: undefined: stdd
./main.gp:16:17: cannot use 1 (untyped int constant) as string value in variable declaration
`
	const src2 = `package main

import (
	"fmt"
	"<vector>"
)

func main() {
	var a int = "a"
	_ = stdd.a
	var b int = "b"
	_ = stdd.b
	var c int = "c"
	_ = stdd.c
	var d int = "d"
	_ = stdd.d
	var e int = "e"
	_ = stdd.e
	var f int = "f"
	_ = stdd.f
	_ = stdd.g
	fmt.Println(a, b, c, d, e, f)
}
`
	const want2 = `# example.com/m
./main.gp:9:14: cannot use "a" (untyped string constant) as int value in variable declaration
./main.gp:10:6: undefined: stdd
./main.gp:11:14: cannot use "b" (untyped string constant) as int value in variable declaration
./main.gp:12:6: undefined: stdd
./main.gp:13:14: cannot use "c" (untyped string constant) as int value in variable declaration
./main.gp:14:6: undefined: stdd
./main.gp:15:14: cannot use "d" (untyped string constant) as int value in variable declaration
./main.gp:16:6: undefined: stdd
./main.gp:17:14: cannot use "e" (untyped string constant) as int value in variable declaration
./main.gp:18:6: undefined: stdd
./main.gp:18:6: too many errors
`
	const want2All = `# example.com/m
./main.gp:9:14: cannot use "a" (untyped string constant) as int value in variable declaration
./main.gp:10:6: undefined: stdd
./main.gp:11:14: cannot use "b" (untyped string constant) as int value in variable declaration
./main.gp:12:6: undefined: stdd
./main.gp:13:14: cannot use "c" (untyped string constant) as int value in variable declaration
./main.gp:14:6: undefined: stdd
./main.gp:15:14: cannot use "d" (untyped string constant) as int value in variable declaration
./main.gp:16:6: undefined: stdd
./main.gp:17:14: cannot use "e" (untyped string constant) as int value in variable declaration
./main.gp:18:6: undefined: stdd
./main.gp:19:14: cannot use "f" (untyped string constant) as int value in variable declaration
./main.gp:20:6: undefined: stdd
./main.gp:21:6: undefined: stdd
`
	twin := func(src string) string {
		src = strings.ReplaceAll(src, `	"<vector>"`, `	// "<vector>"`)
		return strings.ReplaceAll(src, "std.vector[int]{}", "[]int{}")
	}
	forEachCompiler(t, func(t *testing.T, cxx build.CXX) {
		t.Parallel()
		for _, c := range []struct {
			src, want string
			all       bool
		}{{src1, want1, true}, {src2, want2, false}, {src2, want2All, true}} {
			for _, src := range []string{c.src, twin(c.src)} {
				prog, root := loadTree(t, map[string]string{"main.gp": src})
				if got := errorText(t, prog, Check(prog, probeConfig(cxx, LastPhase)), root, c.all); got != c.want {
					t.Errorf("errors of\n%s\n%s\nwant:\n%s", src, got, c.want)
				}
			}
		}
	})
}

// TestProbeConversions checks that a conversion of a value from a name
// that is not a namespace is not rewritten (step 1.10b): int(stdd.size())
// is a Go error, as in the Go twin, and n is invalid, so n + "a" is not
// reported. The conversion of an opaque value is. The errors of lines 9
// and 12 are go build's for the twin.
func TestProbeConversions(t *testing.T) {
	t.Parallel()
	const src = `package main

import (
	"fmt"
	"<vector>"
)

func main() {
	n := int(stdd.size())
	_ = n + "a"
	m := int(len("ab"))
	_ = m + "b"
	var w = std.vector[int]{}
	k := int(w.size())
	_ = k + "c"
	fmt.Println(n, m, k)
}
`
	forEachCompiler(t, func(t *testing.T, cxx build.CXX) {
		t.Parallel()
		prog, root := loadTree(t, map[string]string{"main.gp": src})
		r := Check(prog, probeConfig(cxx, LastPhase))
		want := `# example.com/m
./main.gp:9:11: undefined: stdd
./main.gp:12:6: invalid operation: m + "b" (mismatched types int and untyped string)
./main.gp:15:6: invalid operation: k + "c" (mismatched types int and untyped string)
`
		if got := errorText(t, prog, r, root, true); got != want {
			t.Errorf("errors:\n%s\nwant:\n%s", got, want)
		}
		if got, want := conversionList(prog.Fset, mainPkg(t, prog, r)), "14:7 int(w.size())\n"; got != want {
			t.Errorf("conversions:\n%s\nwant:\n%s", got, want)
		}
	})
}

// TestProbeNames checks the names that are no candidates or are
// namespaces: a local variable named like a namespace, a namespace rename,
// and a namespace that a local header declares. The probe gets exactly
// the candidates.
func TestProbeNames(t *testing.T) {
	t.Parallel()
	forEachCompiler(t, func(t *testing.T, cxx build.CXX) {
		t.Parallel()
		rec := new(probeRecorder)
		cfg := probeConfig(cxx, LastPhase)
		cfg.Probe = rec.probe
		got := probeErrors(t, cfg, map[string]string{
			"main.gp": `package main

import (
	fs "::std::filesystem"
	"<filesystem>"
	"<vector>"
	"./shapes.h"
)

func main() {
	std := struct{ size int }{}
	_ = std.size
	p := fs.path("/tmp/x.txt")
	v := geo.Point{}
	_, _ = p, v
}
`,
			"shapes.h": "#pragma once\nnamespace geo { struct Point { int x, y; }; }\n",
		})
		if got != "" {
			t.Errorf("errors:\n%s", got)
		}
		want := []probeCall{{[]string{"<filesystem>", "<vector>", `"./shapes.h"`}, []string{"geo", "::std::filesystem"}}}
		if !slices.EqualFunc(rec.calls, want, func(a, b probeCall) bool {
			return slices.Equal(a.includes, b.includes) && slices.Equal(a.candidates, b.candidates)
		}) {
			t.Errorf("probe calls %q, want %q", rec.calls, want)
		}
	})
}

// TestProbeRenames checks namespace renames whose namespace doesn't exist:
// an error at the import, and no more about their uses (they stay opaque,
// like the uses of a Go import that fails).
func TestProbeRenames(t *testing.T) {
	t.Parallel()
	forEachCompiler(t, func(t *testing.T, cxx build.CXX) {
		t.Parallel()
		got := probeErrors(t, probeConfig(cxx, LastPhase), map[string]string{
			"main.gp": `package main

import (
	fs "::std::filesystm"
	vec "::std::vector"
	"<vector>"
)

func main() {
	_ = fs.path("/tmp")
	_ = vec.x
}
`,
			// A file without header imports: no compiler run for it.
			"other.gp": `package main

import chrono "::std::chrono"

var d = chrono.seconds(1)
`,
		})
		want := `# example.com/m
./main.gp:4:5: ::std::filesystm is not a namespace of the imported headers
./main.gp:5:6: ::std::vector is not a namespace of the imported headers
./other.gp:3:15: cannot rename ::std::chrono: the file imports no header that could declare it
`
		if got != want {
			t.Errorf("errors:\n%s\nwant:\n%s", got, want)
		}
	})
}

// TestProbeNoIncludes checks a file without header imports: std.vector is
// an ordinary undefined name there (the loader doesn't take it as a
// namespace), and nothing runs a compiler, or even looks for one.
func TestProbeNoIncludes(t *testing.T) {
	t.Parallel()
	rec := new(probeRecorder)
	cfg := Config{Phase: LastPhase, CXX: noCXX(t), Probe: rec.probe}
	got := probeErrors(t, cfg, map[string]string{"main.gp": `package main

func main() {
	v := std.vector[int]{}
	_ = v
}
`})
	want := `# example.com/m
./main.gp:4:7: undefined: std
`
	if got != want || len(rec.calls) != 0 {
		t.Errorf("errors:\n%s\nwant:\n%s\nprobe calls: %q", got, want, rec.calls)
	}
	// A pure Go program needs no compiler either.
	if got := probeErrors(t, cfg, map[string]string{"main.gp": "package main\n\nfunc main() {}\n"}); got != "" || len(rec.calls) != 0 {
		t.Errorf("pure Go: errors:\n%s\nprobe calls: %q", got, rec.calls)
	}
}

// TestProbeIncludeErrors checks headers that aren't found or don't
// compile: an error at the import that brought them in, with a message
// that doesn't depend on the compiler (except the compiler's own message
// in a header), and nothing about the namespace uses of that file (the
// probe can't tell; they stay opaque). The other file of the package is
// probed as usual.
func TestProbeIncludeErrors(t *testing.T) {
	t.Parallel()
	forEachCompiler(t, func(t *testing.T, cxx build.CXX) {
		t.Parallel()
		got := probeErrors(t, probeConfig(cxx, LastPhase), map[string]string{
			"main.gp": `package main

import (
	"<vector>"
	"nosuch.h"
	"<gocpp_no_such_header.h>"
)

func main() {
	_ = std.vector[int]{}
	_ = stdd.vector[int]{}
}
`,
			"bad.gp": `package main

import (
	"./bad.h"
	"<vector>"
	"./open.h"
)

var v = stdd.vector[int]{}
`,
			"angled.gp": `package main

import "<physics.h>"

var _ = std.x
`,
			"physics.h": "#pragma once\n",
			"angled2.gp": `package main

import "<sub/s.h>"
`,
			"sub/s.h": "#pragma once\n",
			"ok.gp": `package main

import "<vector>"

var w = stdd.vector[int]{}
`,
			"bad.h":  "struct S { int x; }\nint y;\n",
			"open.h": "extern \"C\" {\nint f(void);\n",
		})
		// A missing header stops the compiler: gcc reports only the first
		// one, clang both.
		wantRx := regexp.MustCompile(`^# example\.com/m
\./angled\.gp:3:8: cannot find header <physics\.h> \(the package folder has it: import "\./physics\.h"\)
\./angled2\.gp:3:8: cannot find header <sub/s\.h> \(the package folder has it: import "\./sub/s\.h"\)
\./bad\.gp:4:2: header "\./bad\.h" doesn't compile: bad\.h:1:20: expected ';' after struct( definition)?
\./bad\.gp:6:2: header "\./open\.h" ends inside a declaration or a block that it doesn't close
\./main\.gp:5:2: cannot find header "nosuch\.h"
(\./main\.gp:6:2: cannot find header <gocpp_no_such_header\.h>
)?\./ok\.gp:5:9: undefined: stdd
$`)
		if !wantRx.MatchString(got) {
			t.Errorf("errors:\n%s\nwant:\n%s", got, wantRx)
		}
	})
}

// TestProbeUnreadableHeader checks a header that the compiler finds but
// can't read: the compiler's message, without the package folder.
func TestProbeUnreadableHeader(t *testing.T) {
	t.Parallel()
	forEachCompiler(t, func(t *testing.T, cxx build.CXX) {
		t.Parallel()
		prog, root := loadTree(t, map[string]string{
			"main.gp":  "package main\n\nimport \"./noperm.h\"\n\nvar v = std.vector[int]{}\n",
			"noperm.h": "#pragma once\n",
		})
		path := filepath.Join(root, "noperm.h")
		if err := os.Chmod(path, 0); err != nil {
			t.Fatal(err)
		}
		if f, err := os.Open(path); err == nil {
			f.Close()
			t.Skip("the file can be read without permission (root?)")
		}
		got := errorText(t, prog, Check(prog, probeConfig(cxx, LastPhase)), root, true)
		// gcc: "./noperm.h: Permission denied"; clang: "cannot open file
		// './noperm.h': Permission denied".
		wantRx := regexp.MustCompile(`^# example\.com/m
\./main\.gp:3:8: header "\./noperm\.h" can't be read: (cannot open file ')?(\./)?noperm\.h'?: Permission denied
$`)
		if !wantRx.MatchString(got) {
			t.Errorf("errors:\n%s\nwant:\n%s", got, wantRx)
		}
	})
}

// TestProbeFlagDirectives checks that a package with a //gocpp:pkg-config
// or //gocpp:cflags directive (in any of its files) is not probed, because
// the probe can't use their flags yet (Phase 04): a missing header and a
// typo give no errors there, and the uses stay opaque. The same files
// without the directive get the errors.
func TestProbeFlagDirectives(t *testing.T) {
	t.Parallel()
	const main = `package main

import (
	"<vector>"
	"<gocpp_no_such_header.h>"
)

func main() { _ = std.vector[int]{}; _ = stdd.x }
`
	forEachCompiler(t, func(t *testing.T, cxx build.CXX) {
		t.Parallel()
		for _, dir := range []string{"//gocpp:pkg-config gocpp-no-such-lib\n", "//gocpp:cflags -I/nowhere\n"} {
			rec := new(probeRecorder)
			cfg := probeConfig(cxx, LastPhase)
			cfg.Probe = rec.probe
			prog, root := loadTree(t, map[string]string{"main.gp": main, "flags.gp": dir + "package main\n"})
			if got := errorText(t, prog, Check(prog, cfg), root, true); got != "" || len(rec.calls) != 0 {
				t.Errorf("with %q: errors:\n%s\nprobe calls: %q; want none", dir, got, rec.calls)
			}
			p := mainPkg(t, prog, Check(prog, cfg))
			f := prog.Roots[0].Files[slices.IndexFunc(prog.Roots[0].Files, func(f *load.File) bool { return strings.HasSuffix(f.Path, "main.gp") })]
			if !p.Opaque.IsOpaque(compositeLitAt(t, prog, f, 8)) || len(f.Rewrite.Namespaces) != 2 {
				t.Errorf("with %q: the uses are not opaque namespace uses any more", dir)
			}
		}
		got := probeErrors(t, probeConfig(cxx, LastPhase), map[string]string{"main.gp": main, "flags.gp": "package main\n"})
		if want := "# example.com/m\n./main.gp:5:2: cannot find header <gocpp_no_such_header.h>\n"; got != want {
			t.Errorf("without the directive: errors:\n%s\nwant:\n%s", got, want)
		}
	})
}

// TestProbeTarget checks the default compiler of Config.CXX: the one for
// Config.GOOS and GOARCH (build.FindCXXFor). windows/arm64 has no known
// cross compiler, so without GOCPP_CXX that is the error at the first
// header import; for the host, FindCXX's compilers are used.
func TestProbeTarget(t *testing.T) {
	t.Setenv("GOCPP_CXX", "")
	files := map[string]string{"main.gp": "package main\n\nimport \"<vector>\"\n\nvar v = std.vector[int]{}\n"}
	got := probeErrors(t, Config{Phase: LastPhase, GOOS: "windows", GOARCH: "arm64"}, files)
	want := "# example.com/m\n./main.gp:3:8: cannot check the header imports: no C++ compiler for windows/arm64 (set GOCPP_CXX)\n"
	if got != want {
		t.Errorf("windows/arm64: errors:\n%s\nwant:\n%s", got, want)
	}
	if _, err := build.FindCXX(context.Background()); err == nil {
		if got := probeErrors(t, Config{Phase: LastPhase, GOOS: runtime.GOOS, GOARCH: runtime.GOARCH}, files); got != "" {
			t.Errorf("the host: errors:\n%s", got)
		}
	}
}

// TestProbeNoCompiler checks a program with header imports and no C++
// compiler: one error per package, at its first header import, and the
// namespace uses stay opaque.
func TestProbeNoCompiler(t *testing.T) {
	// FindCXX finds nothing in an empty PATH.
	t.Setenv("PATH", t.TempDir())
	t.Setenv("GOCPP_CXX", "")
	t.Setenv("CXX", "")
	files := map[string]string{
		"a.gp": "package main\n\nfunc main() { f() }\n",
		"b.gp": `package main

import (
	"fmt"
	"<vector>"
)

func f() { fmt.Println(std.vector[int]{}, stdd.x) }
`,
		"c.gp": "package main\n\nimport \"<cstdio>\"\n\nvar _ = cstdio.puts\n",
	}
	want := `# example.com/m
./b.gp:5:2: cannot check the header imports: no C++ compiler found: clang++ and g++ are not in PATH (GOCPP_CXX can name the compiler to use)
`
	if got := probeErrors(t, Config{Phase: LastPhase}, files); got != want {
		t.Errorf("errors:\n%s\nwant:\n%s", got, want)
	}
	t.Setenv("GOCPP_CXX", "no-such-c++")
	want = `# example.com/m
./b.gp:5:2: cannot check the header imports: GOCPP_CXX: the C++ compiler "no-such-c++" was not found: exec: "no-such-c++": executable file not found in $PATH
`
	if got := probeErrors(t, Config{Phase: LastPhase}, files); got != want {
		t.Errorf("errors:\n%s\nwant:\n%s", got, want)
	}
}

// TestProbeCanceled checks that a canceled Config.Context stops Check
// after the probe: a Result without packages.
func TestProbeCanceled(t *testing.T) {
	t.Parallel()
	forEachCompiler(t, func(t *testing.T, cxx build.CXX) {
		t.Parallel()
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		cfg := probeConfig(cxx, LastPhase)
		cfg.Context = ctx
		prog, _ := loadTree(t, map[string]string{
			"main.gp": "package main\n\nimport \"<vector>\"\n\nvar v = std.vector[int]{}\n",
		})
		if r := Check(prog, cfg); len(r.Packages) != 0 || r.Package(prog.Roots[0]) != nil {
			t.Errorf("Check with a canceled context returned %d packages, want none", len(r.Packages))
		}
	})
}

// TestProbeCache checks that a second Check of the same program (loaded
// again) runs no compiler, and that the files of a package with the same
// header imports share one compiler run. It counts the runs with a
// wrapper script around the compiler.
func TestProbeCache(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the wrapper is a shell script")
	}
	t.Parallel()
	forEachCompiler(t, func(t *testing.T, cxx build.CXX) {
		// The wrapper is written before t.Parallel (writeScript also
		// copes with parallel tests).
		dir := t.TempDir()
		log := filepath.Join(dir, "runs.log")
		wrapper := filepath.Join(dir, "c++")
		writeScript(t, wrapper, "#!/bin/sh\necho \"$*\" >> '"+log+"'\nexec '"+cxx.Path+"' \"$@\"\n")
		t.Parallel()
		wcxx, err := build.LookCXX(context.Background(), wrapper)
		if err != nil {
			t.Fatal(err)
		}
		runs := func() int {
			data, _ := os.ReadFile(log)
			return strings.Count(string(data), "-fsyntax-only")
		}
		files := map[string]string{
			"a.gp": "package main\n\nimport \"<vector>\"\n\nfunc main() { _ = std.vector[int]{} }\n",
			"b.gp": "package main\n\nimport \"<vector>\"\n\nvar _ = stdd.vector[int]{}\n",
		}
		want := "# example.com/m\n./b.gp:5:9: undefined: stdd\n"
		prog, root := loadTree(t, files)
		if got := errorText(t, prog, Check(prog, probeConfig(wcxx, LastPhase)), root, true); got != want {
			t.Errorf("errors:\n%s\nwant:\n%s", got, want)
		}
		if n := runs(); n != 1 {
			t.Errorf("%d compiler runs for two files with the same header imports, want 1", n)
		}
		prog = load.Load(load.Config{Dir: root, Stdlib: stdlib.FS(), GoStd: stdlib.IsStd, GOOS: "linux", GOARCH: "amd64"}, ".")
		if got := errorText(t, prog, Check(prog, probeConfig(wcxx, LastPhase)), root, true); got != want {
			t.Errorf("errors the second time:\n%s\nwant:\n%s", got, want)
		}
		if n := runs(); n != 1 {
			t.Errorf("%d compiler runs after the second Check, want 1 (cached)", n)
		}
	})
}

// TestProbeExamples checks the 17 examples that type-check in Phase 01
// with the real probe: exactly the errors that they give with a probe that
// takes every name as a namespace (their gate errors at Phase 01, none
// without the gates), so the probe finds no false errors in them. (raylib
// and sqlite have //gocpp:pkg-config lines, so they are not probed: see
// hasFlagDirectives.) For windows, the probe runs the MinGW-w64 cross
// compiler (the host compilers have no <windows.h>), if there is one.
func TestProbeExamples(t *testing.T) {
	t.Parallel()
	mains, err := filepath.Glob("../../examples/*/main.gp")
	if err != nil {
		t.Fatal(err)
	}
	var mingw *build.CXX
	if _, err := exec.LookPath("x86_64-w64-mingw32-g++"); err == nil {
		c, err := build.LookCXX(context.Background(), "x86_64-w64-mingw32-g++")
		if err != nil {
			t.Fatal(err)
		}
		mingw = &c
	}
	forEachCompiler(t, func(t *testing.T, cxx build.CXX) {
		t.Parallel()
		n := 0
		for _, mainFile := range mains {
			dir, err := filepath.Abs(filepath.Dir(mainFile))
			if err != nil {
				t.Fatal(err)
			}
			name := filepath.Base(dir)
			if laterSyntax[name] {
				continue
			}
			n++
			t.Run(name, func(t *testing.T) {
				for _, goos := range []string{"linux", "windows"} {
					target := cxx
					if goos == "windows" {
						if mingw == nil {
							t.Logf("windows: skipped, x86_64-w64-mingw32-g++ is not installed")
							continue
						}
						target = *mingw
					}
					for _, tests := range []bool{false, true} {
						for _, phase := range []int{0, LastPhase} {
							cfg := load.Config{Dir: dir, Stdlib: stdlib.FS(), GoStd: stdlib.IsStd, GOOS: goos, GOARCH: "amd64", Tests: tests}
							fake := load.Load(cfg, ".")
							want := errorText(t, fake, Check(fake, fakeProbeConfig(phase)), dir, true)
							prog := load.Load(cfg, ".")
							if got := errorText(t, prog, Check(prog, probeConfig(target, phase)), dir, true); got != want {
								t.Errorf("%s, Tests %v, Phase %d: errors with the probe:\n%s\nwant:\n%s", goos, tests, phase, got, want)
							}
						}
					}
				}
			})
		}
		if n != 17 {
			t.Errorf("checked %d examples, want 17", n)
		}
	})
}
