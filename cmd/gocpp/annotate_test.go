package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/huuhait/gocpp/internal/cppgen"
	"github.com/huuhait/gocpp/internal/explain"
	"github.com/huuhait/gocpp/internal/lower"
)

var updateAnnotate = flag.Bool("update-annotate", false, "rewrite testdata/annotate/*.golden")

// exampleDir is the folder of the programs of the explain topics: the corpus of the annotate tests.
var exampleDir = "../../internal/explain/examples" // made absolute by init

// examples returns the names (without .gp) of the example programs.
func examples(t *testing.T) []string {
	t.Helper()
	ents, err := os.ReadDir(exampleDir)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range ents {
		if n, ok := strings.CutSuffix(e.Name(), ".gp"); ok {
			names = append(names, n)
		}
	}
	return names
}

// withExample copies the example name to a folder of its own, as name.gp, and works in it with the
// gates of Phase 15 open (the examples of the later phases of the plan are emitted too).
func withExample(t *testing.T, name string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(exampleDir, name+".gp"))
	if err != nil {
		t.Fatal(err)
	}
	inDir(t, map[string]string{name + ".gp": string(data)})
	t.Setenv(gatePhaseEnv, "15")
}

// TestAnnotateGolden: gocpp emit -annotate of each example program is the golden file. The goldens
// are reviewed output: -update-annotate rewrites them.
func TestAnnotateGolden(t *testing.T) {
	names := examples(t)
	if len(names) < 15 {
		t.Fatalf("%d example programs, want at least 15", len(names))
	}
	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			withExample(t, name)
			r := gocppRun("", false, "emit", "-annotate", name+".gp")
			if r.code != 0 || r.stderr != "" {
				t.Fatalf("gocpp emit -annotate %s.gp = %+v", name, r)
			}
			// inDir changed the folder: the golden is in the package folder.
			golden := filepath.Join(pkgDir(t), "testdata", "annotate", name+".golden")
			if *updateAnnotate {
				if err := os.WriteFile(golden, []byte(r.stdout), 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}
			want, err := os.ReadFile(golden)
			if err != nil {
				t.Fatalf("%v (go test ./cmd/gocpp -run TestAnnotateGolden -update-annotate writes it)", err)
			}
			if r.stdout != string(want) {
				t.Errorf("gocpp emit -annotate %s.gp differs from %s (-update-annotate rewrites it after a review):\n%s", name, golden, firstDiff(string(want), r.stdout))
			}
		})
	}
}

var pkgDirOnce string

// pkgDir returns the folder of this package's sources.
func pkgDir(t *testing.T) string {
	t.Helper()
	if pkgDirOnce == "" {
		t.Fatal("pkgDirOnce not set")
	}
	return pkgDirOnce
}

func init() {
	pkgDirOnce, _ = os.Getwd()
	exampleDir = filepath.Join(pkgDirOnce, exampleDir)
}

// firstDiff shows the first line where a and b differ.
func firstDiff(a, b string) string {
	la, lb := strings.Split(a, "\n"), strings.Split(b, "\n")
	for i := 0; i < len(la) || i < len(lb); i++ {
		var x, y string
		if i < len(la) {
			x = la[i]
		}
		if i < len(lb) {
			y = lb[i]
		}
		if x != y {
			return "line " + strconv.Itoa(i+1) + ":\n  want " + x + "\n  got  " + y
		}
	}
	return "(no difference)"
}

// TestAnnotateDeterministic: the same program gives the same view on every run.
func TestAnnotateDeterministic(t *testing.T) {
	for _, name := range []string{"closures", "generics", "select", "interfaces"} {
		withExample(t, name)
		first := gocppRun("", false, "emit", "-annotate", name+".gp")
		for range 3 {
			if again := gocppRun("", false, "emit", "-annotate", name+".gp"); again != first {
				t.Fatalf("%s: two runs differ:\n%s", name, firstDiff(first.stdout, again.stdout))
			}
		}
	}
}

// annotationLine reports whether a line of the annotated view is the view's own.
var annotationLine = regexp.MustCompile(`^(// ---- |// gp\[|//     |//// reasons for|// \S+\.gp:\d+$|// gocpp explain |// gocpp emit -annotate|// lowering's choices)`)

// TestAnnotateKeepsTheCpp: the C++ of the annotated view is the C++ that the emitter writes: take
// the view's own lines and the blank lines away, and what is left is the emit's output without its
// #line directives and blank lines. An annotation never changes the code.
func TestAnnotateKeepsTheCpp(t *testing.T) {
	for _, name := range examples(t) {
		t.Run(name, func(t *testing.T) {
			withExample(t, name)
			ann := gocppRun("", false, "emit", "-annotate", name+".gp")
			if ann.code != 0 {
				t.Fatalf("%+v", ann)
			}
			d, set := loadForTest(t, name+".gp")
			files, code := d.emit(set, explain.GenRoot, nil)
			if files == nil {
				t.Fatalf("emit failed: %d", code)
			}
			var want []string
			for _, f := range files.Files {
				want = append(want, "//// "+f.Path)
				for _, l := range strings.Split(string(f.Data), "\n") {
					if strings.HasPrefix(l, "#line ") || strings.TrimSpace(l) == "" {
						continue
					}
					want = append(want, l)
				}
			}
			var got []string
			for _, l := range strings.Split(ann.stdout, "\n") {
				if strings.TrimSpace(l) == "" || annotationLine.MatchString(l) {
					continue
				}
				got = append(got, l)
			}
			if strings.Join(got, "\n") != strings.Join(want, "\n") {
				t.Errorf("the C++ of the view is not the emitted C++:\n%s", firstDiff(strings.Join(want, "\n"), strings.Join(got, "\n")))
			}
		})
	}
}

// loadForTest loads the files for a driver in the current folder.
func loadForTest(t *testing.T, args ...string) (*driver, *buildSet) {
	t.Helper()
	var errOut bytes.Buffer
	d, code := newDriver("emit", &buildFlags{}, &bytes.Buffer{}, &errOut)
	if d == nil {
		t.Fatalf("newDriver: %d %s", code, errOut.String())
	}
	t.Cleanup(d.stopSignals)
	set, code := d.load(args)
	if set == nil {
		t.Fatalf("load %v: %d %s", args, code, errOut.String())
	}
	return d, set
}

// emitText returns the files of the emit as one text.
func emitText(t *testing.T, d *driver, set *buildSet) string {
	t.Helper()
	files, code := d.emit(set, "", nil)
	if files == nil {
		t.Fatalf("emit failed: %d", code)
	}
	var b bytes.Buffer
	for _, f := range files.Files {
		b.WriteString("//// " + f.Path + "\n")
		b.Write(f.Data)
	}
	return b.String()
}

// TestEmitByteIdenticalWithLog: the emitter writes the same bytes with the reason log on and off, for
// each example (the byte-for-byte comparison with the compiler before 13.5 was made over the whole
// corpus of tests/ and examples/, see the step's report; TestEmitCorpusIdentical repeats it here).
func TestEmitByteIdenticalWithLog(t *testing.T) {
	for _, name := range examples(t) {
		t.Run(name, func(t *testing.T) {
			withExample(t, name)
			d, set := loadForTest(t, name+".gp")
			off := emitText(t, d, set)
			d.why = lower.NewWhy()
			on := emitText(t, d, set)
			d.why.NoChecks = true
			onNo := emitText(t, d, set)
			if on != off || onNo != off {
				t.Errorf("the C++ changes with the reason log on:\n%s", firstDiff(off, on))
			}
			if len(d.why.Reasons()) == 0 {
				t.Errorf("no reasons for %s", name)
			}
		})
	}
}

// TestEmitCorpusIdentical: the same over the programs of tests/run and tests/golden: no program's C++
// changes with the log on. It skips the programs that don't check (they are the error tests).
func TestEmitCorpusIdentical(t *testing.T) {
	if testing.Short() {
		t.Skip("the corpus takes a while")
	}
	root := filepath.Join(pkgDir(t), "..", "..", "tests")
	var progs []string
	for _, dir := range []string{"run", "golden"} {
		m, _ := filepath.Glob(filepath.Join(root, dir, "*.gp"))
		progs = append(progs, m...)
	}
	sort.Strings(progs)
	if len(progs) < 700 {
		t.Fatalf("%d programs in tests/run and tests/golden", len(progs))
	}
	t.Setenv(gatePhaseEnv, "15")
	t.Chdir(t.TempDir())
	checked := 0
	for _, p := range progs {
		var errOut bytes.Buffer
		d, code := newDriver("emit", &buildFlags{}, &bytes.Buffer{}, &errOut)
		if d == nil {
			t.Fatalf("newDriver: %d", code)
		}
		set, _ := d.load([]string{p})
		if set == nil {
			d.stopSignals()
			continue
		}
		var off, on string
		var okOff, okOn bool
		func() {
			defer func() { recover() }()
			files, _ := d.emit(set, "", nil)
			okOff = files != nil
			if okOff {
				off = filesText(files.Files)
			}
		}()
		d.why = lower.NewWhy()
		func() {
			defer func() { recover() }()
			files, _ := d.emit(set, "", nil)
			okOn = files != nil
			if okOn {
				on = filesText(files.Files)
			}
		}()
		d.stopSignals()
		if okOff != okOn || off != on {
			t.Errorf("%s: the emit changes with the reason log on", filepath.Base(p))
		}
		if okOff {
			checked++
		}
	}
	if checked < 500 {
		t.Errorf("only %d programs were emitted", checked)
	}
}

func filesText(files []cppgen.File) string {
	var b bytes.Buffer
	for _, f := range files {
		b.WriteString("//// " + f.Path + "\n")
		b.Write(f.Data)
	}
	return b.String()
}

// TestAnnotateFlags covers the flags of gocpp emit that belong to -annotate.
func TestAnnotateFlags(t *testing.T) {
	withExample(t, "boxing")
	for _, flag := range []string{"-json", "-nochecks", "-kinds=boxing", "-o=x.txt"} {
		r := gocppRun("", false, "emit", flag, "boxing.gp")
		name := strings.SplitN(flag, "=", 2)[0]
		if r.code != 2 || r.stdout != "" || r.stderr != "gocpp emit: "+name+" needs -annotate\n" {
			t.Errorf("gocpp emit %s = %+v, want exit 2 and %q", flag, r, name+" needs -annotate")
		}
	}
	r := gocppRun("", false, "emit", "-annotate", "-kinds=boxing,nope", "boxing.gp")
	if r.code != 2 || !strings.Contains(r.stderr, `unknown kind "nope"`) || !strings.Contains(r.stderr, "closures, defer") {
		t.Errorf("-kinds=nope = %+v", r)
	}
	// -kinds keeps the reasons of the kinds only.
	r = gocppRun("", false, "emit", "-annotate", "-kinds=boxing", "boxing.gp")
	if r.code != 0 || !strings.Contains(r.stdout, "// gp[boxing]") || strings.Contains(r.stdout, "// gp[methods]") || strings.Contains(r.stdout, "// gp[checks]") {
		t.Errorf("-kinds=boxing:\n%s", r.stdout)
	}
	// -nochecks changes the words of the checks.
	plain := gocppRun("", false, "emit", "-annotate", "-kinds=checks", "boxing.gp")
	no := gocppRun("", false, "emit", "-annotate", "-nochecks", "-kinds=checks", "boxing.gp")
	if !strings.Contains(plain.stdout, "gocpp build -nochecks removes it") || !strings.Contains(no.stdout, "removed by -nochecks") || strings.Contains(no.stdout, "gocpp build -nochecks removes it") {
		t.Errorf("-nochecks:\n%s\n%s", plain.stdout, no.stdout)
	}
	// -o writes the file and prints nothing.
	r = gocppRun("", false, "emit", "-annotate", "-o", "view.txt", "boxing.gp")
	data, err := os.ReadFile("view.txt")
	full := gocppRun("", false, "emit", "-annotate", "boxing.gp")
	if r.code != 0 || r.stdout != "" || err != nil || string(data) != full.stdout {
		t.Errorf("-o: %+v, %v", r, err)
	}
	// -json is valid JSON with the chunks.
	r = gocppRun("", false, "emit", "-annotate", "-json", "boxing.gp")
	var v struct {
		Files []struct {
			Path   string
			Chunks []struct {
				GP      *struct{ Line int }
				Reasons []struct{ Kind, Msg string }
				Cpp     []string
			}
		}
	}
	if err := json.Unmarshal([]byte(r.stdout), &v); err != nil || len(v.Files) < 2 {
		t.Fatalf("-json: %v\n%s", err, r.stdout)
	}
	kinds := map[string]bool{}
	for _, f := range v.Files {
		for _, c := range f.Chunks {
			for _, r := range c.Reasons {
				kinds[r.Kind] = true
			}
		}
	}
	if strings.Contains(r.stdout, os.TempDir()) {
		t.Errorf("-json has an absolute path:\n%s", r.stdout)
	}
	if !kinds["boxing"] || !kinds["refcount"] {
		t.Errorf("-json kinds %v", kinds)
	}
	// Errors of the program are the errors of gocpp emit.
	inDir(t, map[string]string{"bad.gp": "package main\n\nfunc main() { x := 1 }\n"})
	r = gocppRun("", false, "emit", "-annotate", "bad.gp")
	if r.code != 1 || !strings.Contains(r.stderr, "declared and not used") || r.stdout != "" {
		t.Errorf("a program with an error: %+v", r)
	}
}

// reasonKinds are the kinds that each example must show: a note whose decision moved or was removed
// makes its kind disappear from the view of the program that needs it.
var reasonKinds = map[string][]string{
	"boxing":         {"boxing", "refcount", "methods", "checks"},
	"closures":       {"closures", "boxing"},
	"defer":          {"defer", "closures"},
	"recover":        {"recover", "defer"},
	"checks":         {"checks"},
	"refcount":       {"refcount"},
	"interfaces":     {"interfaces", "allocation"},
	"generics":       {"generics"},
	"select":         {"select", "channels"},
	"goroutines":     {"goroutines", "channels"},
	"channels":       {"channels", "range"},
	"allocation":     {"allocation"},
	"order":          {"order"},
	"interop":        {"interop"},
	"embedding":      {"embedding"},
	"range":          {"range"},
	"names":          {"names"},
	"methods":        {"methods"},
	"panics":         {"panics"},
	"strings":        {"strings"},
	"slices":         {"slices"},
	"maps":           {"maps"},
	"initialization": {"initialization"},
	// The reference topics have no kind of their own: what their examples show anyway.
	"arrays": {"range"}, "results": {"checks"}, "complex": {"interfaces"}, "named": {"methods"},
	"types": {"allocation"}, "fmt": {"interfaces"}, "files": {"initialization"}, "linemap": {},
}

// TestAnnotateKinds: each example shows the kinds of reasons that its topic is about.
func TestAnnotateKinds(t *testing.T) {
	for _, name := range examples(t) {
		t.Run(name, func(t *testing.T) {
			want, ok := reasonKinds[name]
			if !ok {
				t.Fatalf("no kinds listed for the example %s", name)
			}
			withExample(t, name)
			r := gocppRun("", false, "emit", "-annotate", name+".gp")
			for _, k := range want {
				if !strings.Contains(r.stdout, "// gp["+k+"]") {
					t.Errorf("the view of %s.gp has no [%s] reason", name, k)
				}
			}
		})
	}
	// Every kind is shown by some example.
	shown := map[string]bool{}
	for _, ks := range reasonKinds {
		for _, k := range ks {
			shown[k] = true
		}
	}
	for _, k := range lower.AllKinds {
		if !shown[string(k)] {
			t.Errorf("no example is required to show [%s]", k)
		}
	}
}

// A truthRule says what the C++ of a chunk must have, and what neither it nor the reason may have,
// when a reason of the view has this text.
type truthRule struct {
	msg    string   // the reason's text starts with, or contains (a leading ~), this
	need   []string // alternatives: the chunk's C++ has one of them
	forbid []string // in the chunk's C++ and in the reason
}

var truthRules = []truthRule{
	{msg: "m[k]: gp::get gives", need: []string{"gp::get("}},
	{msg: "v, ok := m[k]: gp::lookup", need: []string{"gp::lookup("}},
	{msg: "m[k] = v: gp::set(m, k, v) stores", need: []string{"gp::set("}, forbid: []string{"gp::get gives", "never inserts"}},
	{msg: "m[k]++, m[k] op= v", need: []string{"gp::ref("}},
	{msg: "m[k] /= y", need: []string{"gp::set("}},
	{msg: "go: ", need: []string{"gp::go_fn(", "gp::go_call("}, forbid: []string{"gp_push", "frame"}},
	{msg: "defer: ", need: []string{"gp_push"}},
	{msg: "return assigns the results first", need: []string{"goto "}},
	{msg: "~ is boxed:", need: []string{"gp::new_", "gp::ptr<"}},
	{msg: "~into any allocates", need: []string{"to_iface"}},
	{msg: "each iteration has its own", need: []string{"gp::new_<"}},
	{msg: "shift by a count", need: []string{"gp::shl(", "gp::shr("}},
	{msg: "method value ", need: []string{"[gp_r = "}, forbid: []string{"a plain member call", "a static call"}},
	{msg: "method expression ", need: []string{"[](auto&& gp_r"}},
	{msg: "~is boxed in this case clause", need: []string{"gp::new_<"}},
	{msg: "m[k] %= y", need: []string{"gp::rem("}},
	{msg: "m[k] <<= n", need: []string{"gp::shl(", "gp::shr("}},
	{msg: "return jumps to", need: []string{"goto "}, forbid: []string{"returns the results"}},
	{msg: "range over a func:", need: []string{"rangefunc_state"}},
	{msg: "func init is the C++ function", need: []string{"gp_init_"}},
}

// TestAnnotateTruth: a reason is about the C++ of its own chunk: the reasons whose text promises a
// mechanism (gp::set for a store, gp::go_fn for go, a box for a boxed variable) are shown only on
// chunks that have it, and for a generic function's instances each instance has its own reasons.
func TestAnnotateTruth(t *testing.T) {
	type progInfo struct{ name, src string }
	var progs []progInfo
	for _, n := range examples(t) {
		data, err := os.ReadFile(filepath.Join(exampleDir, n+".gp"))
		if err != nil {
			t.Fatal(err)
		}
		progs = append(progs, progInfo{n, string(data)})
	}
	extra, _ := filepath.Glob(filepath.Join(pkgDir(t), "testdata", "truth", "*.gp"))
	if len(extra) < 4 {
		t.Fatalf("%d programs in testdata/truth", len(extra))
	}
	for _, f := range extra {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		progs = append(progs, progInfo{"truth-" + strings.TrimSuffix(filepath.Base(f), ".gp"), string(data)})
	}
	t.Setenv(gatePhaseEnv, "15")
	checked := map[string]int{}
	for _, p := range progs {
		t.Run(p.name, func(t *testing.T) {
			inDir(t, map[string]string{"p.gp": p.src})
			r := gocppRun("", false, "emit", "-annotate", "-json", "p.gp")
			if r.code != 0 {
				t.Fatalf("%+v", r)
			}
			var v struct {
				Files []struct {
					Path   string
					Chunks []struct {
						GP      *struct{ Line int }
						Inst    string `json:"instance"`
						Reasons []struct{ Kind, Msg string }
						Cpp     []string
					}
				}
			}
			if err := json.Unmarshal([]byte(r.stdout), &v); err != nil {
				t.Fatal(err)
			}
			for _, f := range v.Files {
				boxed := map[string]bool{}  // instance + "\x00" + name
				copied := map[string]bool{} // the same, for the names captured by copy
				for _, c := range f.Chunks {
					cpp := strings.Join(c.Cpp, "\n")
					line := 0
					if c.GP != nil {
						line = c.GP.Line
					}
					for _, rs := range c.Reasons {
						for _, rule := range truthRules {
							want, contains := strings.CutPrefix(rule.msg, "~")
							if !(contains && strings.Contains(rs.Msg, want) || !contains && strings.HasPrefix(rs.Msg, rule.msg)) {
								continue
							}
							checked[rule.msg]++
							ok := false
							for _, n := range rule.need {
								ok = ok || strings.Contains(cpp, n)
							}
							if !ok {
								t.Errorf("%s:%d: the reason %q needs one of %q in its chunk:\n%s", f.Path, line, rs.Msg, rule.need, cpp)
							}
							for _, bad := range rule.forbid {
								if strings.Contains(cpp, bad) || strings.Contains(rs.Msg, bad) {
									t.Errorf("%s:%d: the reason %q or its chunk has %q:\n%s", f.Path, line, rs.Msg, bad, cpp)
								}
							}
						}
						if name, _, ok := strings.Cut(rs.Msg, " is boxed:"); ok {
							boxed[c.Inst+"\x00"+name] = true
						}
						if rest, ok := strings.CutPrefix(rs.Msg, "captures "); ok {
							if name, how, ok := strings.Cut(rest, " by "); ok {
								if strings.HasPrefix(how, "copy") {
									copied[c.Inst+"\x00"+name] = true
								}
							}
						}
					}
				}
				// The same name is not both boxed and captured by copy in one instance.
				for k := range copied {
					if boxed[k] {
						inst, name, _ := strings.Cut(k, "\x00")
						t.Errorf("%s: %s is boxed and captured by copy in the same function (instance %q)", f.Path, name, inst)
					}
				}
			}
		})
	}
	for _, rule := range truthRules {
		if checked[rule.msg] == 0 {
			t.Errorf("no program exercises the rule %q", rule.msg)
		}
	}
}

// TestAnnotateInstances: the instances of a generic function share its .gp lines, and each one has
// the reasons that are true for it: Show[Big] boxes y (512 bytes captured by a closure), Show[int]
// and Show[string] copy it. The chunks are named by their instance.
func TestAnnotateInstances(t *testing.T) {
	t.Setenv(gatePhaseEnv, "15")
	data, err := os.ReadFile(filepath.Join(pkgDir(t), "testdata", "truth", "instances.gp"))
	if err != nil {
		t.Fatal(err)
	}
	inDir(t, map[string]string{"p.gp": string(data)})
	r := gocppRun("", false, "emit", "-annotate", "-json", "p.gp")
	if r.code != 0 {
		t.Fatalf("%+v", r)
	}
	var v struct {
		Files []struct {
			Chunks []struct {
				GP      *struct{ Line int }
				Inst    string `json:"instance"`
				Cont    bool   `json:"continued"`
				Reasons []struct{ Kind, Msg string }
				Cpp     []string
			}
		}
	}
	if err := json.Unmarshal([]byte(r.stdout), &v); err != nil {
		t.Fatal(err)
	}
	type key struct {
		inst string
		line int
	}
	boxed := map[string]bool{}  // the instances with a boxing reason
	copies := map[string]bool{} // the instances with a captured-by-copy reason
	lines := map[key]int{}
	for _, f := range v.Files {
		for _, c := range f.Chunks {
			if c.Inst == "" || c.GP == nil {
				continue
			}
			lines[key{c.Inst, c.GP.Line}]++
			if c.Cont {
				t.Errorf("a chunk of %s at line %d is continued: each instance shows its own reasons", c.Inst, c.GP.Line)
			}
			for _, rs := range c.Reasons {
				if rs.Kind == "boxing" {
					boxed[c.Inst] = true
				}
				if strings.Contains(rs.Msg, "by copy") {
					copies[c.Inst] = true
				}
			}
		}
	}
	insts := map[string]bool{}
	for k := range lines {
		insts[k.inst] = true
	}
	if len(insts) != 3 || len(boxed) != 1 || len(copies) != 2 {
		t.Errorf("instances %v, boxed %v, by copy %v: want 3 instances, 1 boxing, 2 by copy", insts, boxed, copies)
	}
	for inst := range boxed {
		if copies[inst] {
			t.Errorf("%s boxes y and copies it", inst)
		}
	}
	text := gocppRun("", false, "emit", "-annotate", "p.gp").stdout
	if n := strings.Count(text, "  [in gp_inst_"); n < 12 || strings.Contains(text, "(continued)") && strings.Contains(text, "[in gp_inst_") && strings.Count(text, "func Show[T any](x T) {  [in gp_inst_") != 3 {
		t.Errorf("the headers of the instance chunks: %d marks\n%s", n, text)
	}
}

// TestAnnotateStdLibrary: the view is of the program's own packages; -std adds the standard
// library's, whose generated lines are recognised (no $GOCPP/gen path as a .gp place).
func TestAnnotateStdLibrary(t *testing.T) {
	t.Setenv(gatePhaseEnv, "15")
	inDir(t, map[string]string{"e.gp": "package main\n\nimport (\n\t\"errors\"\n\t\"fmt\"\n)\n\nfunc main() {\n\tfmt.Println(errors.New(\"x\"))\n}\n"})
	own := gocppRun("", false, "emit", "-annotate", "e.gp")
	if own.code != 0 || strings.Contains(own.stdout, "//// gpkg/errors/") || strings.Contains(own.stdout, "$GOCPP/") {
		t.Errorf("the default view has the standard library:\n%.600s", own.stdout)
	}
	all := gocppRun("", false, "emit", "-annotate", "-std", "e.gp")
	if all.code != 0 || !strings.Contains(all.stdout, "//// gpkg/errors/errors.gp.cpp") || !strings.Contains(all.stdout, "$GOCPP/stdlib/errors/errors.gp:") {
		t.Errorf("-std has no errors package")
	}
	if strings.Contains(all.stdout, "$GOCPP/gen/") {
		t.Errorf("-std shows a generated line of a standard package as a .gp place:\n%s", all.stdout[strings.Index(all.stdout, "$GOCPP/gen/")-80:][:300])
	}
	if len(all.stdout) < 3*len(own.stdout) {
		t.Errorf("-std is not much more than the default: %d and %d bytes", len(all.stdout), len(own.stdout))
	}
	if r := gocppRun("", false, "emit", "-std", "e.gp"); r.code != 2 || r.stderr != "gocpp emit: -std needs -annotate\n" {
		t.Errorf("-std without -annotate = %+v", r)
	}
}

// TestEmitUsageNamesAnnotate: gocpp emit -h tells about -annotate and its flags.
func TestEmitUsageNamesAnnotate(t *testing.T) {
	r := gocppRun("", false, "emit", "-h")
	for _, want := range []string{"-annotate", "-nochecks", "-kinds", "-std", "docs/guides/explain.md"} {
		if !strings.Contains(r.stderr, want) {
			t.Errorf("emit -h does not mention %s:\n%s", want, r.stderr)
		}
	}
}

// TestAnnotateNeedsFlagOrder: the flag named when several need -annotate is always the same.
func TestAnnotateNeedsFlagOrder(t *testing.T) {
	for range 20 {
		r := gocppRun("", false, "emit", "-o", "x", "-json", "-nochecks", "x.gp")
		if r.stderr != "gocpp emit: -json needs -annotate\n" {
			t.Fatalf("%q", r.stderr)
		}
	}
}

// viewOf returns the text of gocpp emit -annotate for the source src (as p.gp).
func viewOf(t *testing.T, src string, args ...string) string {
	t.Helper()
	t.Setenv(gatePhaseEnv, "15")
	inDir(t, map[string]string{"p.gp": src})
	r := gocppRun("", false, append(append([]string{"emit", "-annotate"}, args...), "p.gp")...)
	if r.code != 0 {
		t.Fatalf("%+v", r)
	}
	return r.stdout
}

func truthSource(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(pkgDir(t), "testdata", "truth", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// TestAnnotateMultiLineStatement: a reason about the continuation line of a statement (an argument
// on the third line of a call) is shown on the chunk of the statement, with its own line number; it
// is not "a reason for a .gp line that has no C++".
func TestAnnotateMultiLineStatement(t *testing.T) {
	src := truthSource(t, "multiline.gp")
	text := viewOf(t, src)
	if strings.Contains(text, "reasons for .gp lines that have no C++") {
		t.Errorf("a reason is unplaced:\n%s", text[strings.Index(text, "//// reasons for"):])
	}
	i := strings.Index(text, "// ---- ./p.gp:13: x := f(")
	j := strings.Index(text, "// ---- ./p.gp:18: m := ")
	if i < 0 || j < i {
		t.Fatalf("no chunk for line 13:\n%s", text)
	}
	chunk := text[i:j]
	for _, want := range []string{"gp[methods] line 14 col 3: pointer method Inc", "gp[methods] line 15 col 3: pointer method Inc", "gp[checks] line 15 col 10: division of integers"} {
		if !strings.Contains(chunk, want) {
			t.Errorf("the chunk of line 13 lacks %q:\n%s", want, chunk)
		}
	}
	// The fmt.Println( with its arguments on following lines.
	if !strings.Contains(text, "gp[maps] line 24 col 4: m[k]: gp::get gives") || !strings.Contains(text, "gp[checks] line 25 col 5: p.f through a pointer") {
		t.Errorf("the map read in the arguments of fmt.Println has no note on the call:\n%s", text)
	}
	// gocpp explain prints the text of a continuation line's reasons.
	r := gocppRun("", false, "explain", "p.gp:15")
	if r.code != 0 || !strings.Contains(r.stdout, "pointer method Inc") || strings.Contains(r.stdout, "no C++ comes from") {
		t.Errorf("explain p.gp:15 = %+v", r)
	}
	if j := gocppRun("", false, "explain", "-json", "p.gp:15"); !strings.Contains(j.stdout, `"line": 15`) || !strings.Contains(j.stdout, "pointer method Inc") {
		t.Errorf("explain -json p.gp:15:\n%s", j.stdout)
	}
	// Brackets in strings and comments do not make a line a continuation.
	if got := viewOf(t, "package main\n\nfunc main() {\n\ts := \"(\" // (\n\tprintln(s, len(s))\n}\n"); strings.Contains(got, "line 5") {
		t.Errorf("a bracket in a string continued the statement:\n%s", got)
	}
}

// TestAnnotateTypeSwitchBoxing: the variable of a type switch is boxed in the clause that needs it,
// and the note is on that clause, not on the switch header.
func TestAnnotateTypeSwitchBoxing(t *testing.T) {
	text := viewOf(t, truthSource(t, "tswitch.gp"))
	hdr := strings.Index(text, "switch v := x.(type)")
	cs := strings.Index(text, ": case int:")
	cs2 := strings.Index(text, ": case string:")
	if hdr < 0 || cs < hdr || cs2 < cs {
		t.Fatalf("chunks not found:\n%s", text)
	}
	if strings.Contains(text[hdr:cs], "gp[boxing]") {
		t.Errorf("the switch header has a boxing note:\n%s", text[hdr:cs])
	}
	if !strings.Contains(text[cs:cs2], "gp[boxing] col 3: v is boxed in this case clause") {
		t.Errorf("the boxed clause has no note:\n%s", text[cs:cs2])
	}
	if strings.Contains(text[cs2:cs2+300], "gp[boxing]") {
		t.Errorf("the clause that copies v has a boxing note:\n%s", text[cs2:cs2+300])
	}
}

// TestAnnotateMethodValues: a method value binds the receiver; its notes say so, and the method
// call notes (a static call, a plain member call) are for calls.
func TestAnnotateMethodValues(t *testing.T) {
	text := viewOf(t, truthSource(t, "methodvals.gp"))
	for _, want := range []string{
		"method value Get: binds a copy of the receiver now",
		"method value Inc: binds the receiver pointer now",
		"method expression (*T).Inc: a lambda whose first parameter is the receiver",
		"method value Get of an interface: binds the interface value now",
	} {
		if !strings.Contains(strings.ReplaceAll(strings.ReplaceAll(text, "\n//     ", " "), "\n// ", "\n"), want) {
			t.Errorf("no note %q:\n%s", want, text)
		}
	}
	// The chunk of f := t.Get has no "plain member call" note.
	i := strings.Index(text, ": f := t.Get")
	j := strings.Index(text, ": g := t.Inc")
	if i < 0 || j < i || strings.Contains(text[i:j], "plain member call") {
		t.Errorf("f := t.Get:\n%s", text[i:j])
	}
}

// TestAnnotateLows covers the smaller truths of the notes: an unnamed result has a name in its
// note, int64(n) of an int is no cast, %= and <<= on a map entry are named right, a function
// without results has no "results" in its defer notes.
func TestAnnotateLows(t *testing.T) {
	text := viewOf(t, truthSource(t, "lows.gp"))
	flat := strings.ReplaceAll(text, "\n//     ", " ")
	if strings.Contains(flat, "col 14:  is boxed") || strings.Contains(flat, ": is boxed") || !strings.Contains(flat, "an unnamed result is boxed") {
		t.Errorf("the unnamed results:\n%s", text)
	}
	if n := strings.Count(flat, "integer conversion:"); n != 1 {
		t.Errorf("%d integer conversion notes, want 1 (int8(n); int64(n) of an int is no cast)", n)
	}
	for _, want := range []string{"m[k] %= y on integers: gp::set(m, k, gp::rem(", "m[k] <<= n and m[k] >>= n: gp::ref(m, k) = gp::shl("} {
		if !strings.Contains(flat, want) {
			t.Errorf("no note %q", want)
		}
	}
	q := flat[strings.Index(flat, "./p.gp:22:"):strings.Index(flat, "./p.gp:27:")]
	if strings.Contains(q, "results are variables") || strings.Contains(q, "returns the results") || !strings.Contains(q, "return jumps to") {
		t.Errorf("the defer notes of a function without results:\n%s", q)
	}
}

// TestAnnotateOutputIsNotAnInput: -o never overwrites a file that is being emitted.
func TestAnnotateOutputIsNotAnInput(t *testing.T) {
	t.Setenv(gatePhaseEnv, "15")
	const src = "package main\n\nfunc main() { println(1) }\n"
	inDir(t, map[string]string{"victim.gp": src})
	for _, out := range []string{"victim.gp", "./victim.gp"} {
		r := gocppRun("", false, "emit", "-annotate", "-o", out, "victim.gp")
		if r.code != 2 || !strings.Contains(r.stderr, "would overwrite") {
			t.Errorf("-o %s = %+v", out, r)
		}
	}
	if data, _ := os.ReadFile("victim.gp"); string(data) != src {
		t.Errorf("victim.gp was changed:\n%s", data)
	}
}

// innermostStmtLine returns the line where the innermost statement of f that contains the place
// (line, col) starts, or 0: the oracle for TestAnnotateReasonsInTheirStatement.
func innermostStmtLine(fset *token.FileSet, f *ast.File, line, col int) int {
	best := 0
	ast.Inspect(f, func(n ast.Node) bool {
		s, ok := n.(ast.Stmt)
		if !ok {
			return true
		}
		switch s.(type) {
		case *ast.BlockStmt, *ast.EmptyStmt, *ast.BadStmt:
			return true
		}
		a, b := fset.Position(s.Pos()), fset.Position(s.End())
		if (line > a.Line || line == a.Line && col >= a.Column) && (line < b.Line || line == b.Line && col < b.Column) {
			best = a.Line // the statements come outermost first: the last one found is the innermost
		}
		return true
	})
	return best
}

// TestAnnotateReasonsInTheirStatement: a reason is shown with the C++ of the statement that
// contains it, whatever the shape of the statement: operators at the end of a line, a lone "}" that
// has a chunk of its own, comments and strings with brackets. The oracle is go/parser: for each
// reason the innermost statement that contains its place must be the statement of its chunk, when
// that statement has a chunk.
func TestAnnotateReasonsInTheirStatement(t *testing.T) {
	t.Setenv(gatePhaseEnv, "15")
	var progs []string
	for _, n := range examples(t) {
		data, err := os.ReadFile(filepath.Join(exampleDir, n+".gp"))
		if err != nil {
			t.Fatal(err)
		}
		progs = append(progs, string(data))
	}
	truth, _ := filepath.Glob(filepath.Join(pkgDir(t), "testdata", "truth", "*.gp"))
	for _, f := range truth {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		progs = append(progs, string(data))
	}
	checked, moved := 0, 0
	for i, src := range progs {
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, "p.gp", src, 0)
		if err != nil {
			continue // a program with gocpp syntax: no oracle
		}
		inDir(t, map[string]string{"p.gp": src})
		r := gocppRun("", false, "emit", "-annotate", "-json", "p.gp")
		if r.code != 0 {
			t.Fatalf("program %d: %+v", i, r)
		}
		var v struct {
			Files []struct {
				Chunks []struct {
					GP *struct {
						File string
						Line int
					}
					Reasons []struct {
						Line, Col int
						Kind, Msg string
					}
				}
			}
		}
		if err := json.Unmarshal([]byte(r.stdout), &v); err != nil {
			t.Fatal(err)
		}
		have := map[int]bool{} // the .gp lines that have a chunk
		for _, fl := range v.Files {
			for _, c := range fl.Chunks {
				if c.GP != nil && strings.HasSuffix(c.GP.File, "p.gp") {
					have[c.GP.Line] = true
				}
			}
		}
		for _, fl := range v.Files {
			for _, c := range fl.Chunks {
				if c.GP == nil || !strings.HasSuffix(c.GP.File, "p.gp") {
					continue
				}
				for _, rs := range c.Reasons {
					line := rs.Line
					if line == 0 {
						line = c.GP.Line
					}
					want := innermostStmtLine(fset, f, line, rs.Col)
					if want == 0 || !have[want] {
						continue
					}
					checked++
					if rs.Line != 0 {
						moved++
					}
					if c.GP.Line != want {
						t.Errorf("program %d: the reason of line %d col %d (%s) is on the chunk of line %d, its statement starts at line %d:\n%s", i, line, rs.Col, rs.Kind, c.GP.Line, want, rs.Msg)
					}
				}
			}
		}
	}
	if checked < 300 || moved < 20 {
		t.Errorf("only %d reasons checked, %d of them on a continuation line: the oracle is not looking", checked, moved)
	}
}

// TestAnnotateStatementShapes: the shapes that misplaced reasons: an operator at the end of a line
// (the division is in the first statement, not in the loop header), a lone closing token with a
// chunk of its own, a block comment with a bracket.
func TestAnnotateStatementShapes(t *testing.T) {
	text := viewOf(t, truthSource(t, "stmts4.gp"))
	i := strings.Index(text, ": for i := 0; i < 2; i++ {")
	j := strings.Index(text, ": x := 1 +")
	k := strings.Index(text, ": if x > 0 &&")
	if i < 0 || j < i || k < j {
		t.Fatalf("chunks missing:\n%s", text)
	}
	if strings.Contains(text[i:j], "division") {
		t.Errorf("a division note is on the for header:\n%s", text[i:j])
	}
	if !strings.Contains(text[j:k], "gp[checks] line 10 col 5: division of integers") || !strings.Contains(text[k:k+500], "gp[checks] line 12 col 6: division of integers") {
		t.Errorf("the division notes are not on their statements:\n%s", text[j:k+500])
	}
	// A select case whose communication spans two lines, with a lone "}" chunk in between.
	s3 := viewOf(t, truthSource(t, "stmts3.gp"))
	if m := regexp.MustCompile(`(?s)---- \./p\.gp:(\d+): case ch <- f\(11,.*?\n// ---- \./p\.gp:\d+: [^\n]*`).FindString(s3); !strings.Contains(m, "line 28") {
		t.Errorf("the note of line 28 is not under the case on line 27:\n%.1500s", s3)
	}
}

// TestAnnotatePackageVarContinuation: a reason on a continuation line of a package variable's
// initializer (no statement; the bracket scan finds the declaration) goes to the declaration, also
// when earlier lines have a bracket in a string, a comment or a raw string.
func TestAnnotatePackageVarContinuation(t *testing.T) {
	text := viewOf(t, truthSource(t, "pkgvar.gp"))
	if strings.Contains(text, "reasons for .gp lines that have no C++") {
		t.Fatalf("a reason is unplaced:\n%s", text[strings.Index(text, "//// reasons for"):])
	}
	for _, want := range []string{"var q = f(\")\", // )\n// gp[initialization]", "gp[checks] line 8 col 4: division", "gp[checks] line 11 col 4: division", "gp[checks] line 14 col 7: division"} {
		if !strings.Contains(text, want) {
			t.Errorf("no %q in:\n%s", want, text)
		}
	}
}
