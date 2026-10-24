package server

import (
	"context"
	"fmt"
	"math/rand"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/huuhait/gocpp/internal/check"
	"github.com/huuhait/gocpp/internal/version"
)

// The differential session test of the quick publish. A server with the focused check and one
// without it (diagnostics.noFocus) get the same scripts of edits, a step at a time, and the first
// publish of every edited version must be the same: the quick publish is the answer that the editor
// shows first, and the full run that settles it (settleDelay) would hide a difference. Scripts are
// made from a seed. (lspdiff, tools/lspdiff, cannot show this: it waits after each edit, so that
// the full run has replaced the quick publish when it compares.)

// diffModule is a package lib of three files, and a package main that uses it.
func diffModule() map[string]string {
	return map[string]string{
		"gocpp.mod": "module example.com/m\n",
		"lib/a.gp": `package lib

import "fmt"

type Counter struct{ n int }

func (c *Counter) Add(k int) int {
	c.n += k
	// MARK_A
	return c.n
}

func Describe(c *Counter) string {
	// MARK_A2
	return fmt.Sprintf("counter %d", c.n)
}
`,
		"lib/b.gp": `package lib

import "strings"

var Names = []string{"a", "b"}

func Join() string {
	// MARK_B
	return strings.Join(Names, ",")
}
`,
		"lib/c.gp": `package lib

import "strconv"

const Limit = 10

type Shape interface{ Area() float64 }

func Helper(k int) int {
	v, err := strconv.Atoi("5")
	// MARK_C
	if err != nil {
		return k
	}
	return v + k + Limit
}
`,
		"main/main.gp": `package main

import "example.com/m/lib"

func main() {
	var c lib.Counter
	_ = c.Add(1)
	// MARK_M
	_ = lib.Join()
}
`,
	}
}

// diffStatements are the statements that the scripts put at a marker.
var diffStatements = []string{
	"x := 1", "_ = undefinedName", "_ = Helper(1)", "_ = Join()", "_ = strings.ToUpper(\"a\")",
	"_ = fmt.Sprint(1)", "c.n = \"s\"", "Names = nil", "_ = Limit", "var s Shape", "return", "(",
	"}", "goto L", "for {", "_ = strconv.Itoa(2)", "_ = lib.Limit", "_ = lib.Join()", "y := lib.Helper(2)",
	"go func() {}()", "ch := make(chan int)\n\t_ = ch", "defer println()", "_ = #func()", "",
}

// diffMarkers are the markers of each file.
var diffMarkers = map[string][]string{
	"lib/a.gp":     {"// MARK_A", "// MARK_A2"},
	"lib/b.gp":     {"// MARK_B"},
	"lib/c.gp":     {"// MARK_C"},
	"main/main.gp": {"// MARK_M"},
}

var diffFiles = []string{"lib/a.gp", "lib/b.gp", "lib/c.gp", "main/main.gp"}

// A diffSpec is what the scripts of a differential session work on: the module, the files they
// edit, the markers of each file, the statements they put at a marker, and the changes of a
// declaration they make (old, new; each applies to the files that have old).
type diffSpec struct {
	module     func() map[string]string
	files      []string
	markers    map[string][]string
	statements []string
	decls      [][2]string
}

// plainDiff is the spec of diffModule.
var plainDiff = &diffSpec{
	module: diffModule, files: diffFiles, markers: diffMarkers, statements: diffStatements,
	decls: [][2]string{{"func Join()", "func Join2()"}, {"Add(k int)", "Add(k, j int)"}},
}

// diffEdit returns the next text of the file: mostly a statement at a marker (now and then after a
// revert to the original), and now and then a revert, a new function, a renamed function or a new
// parameter.
func (spec *diffSpec) diffEdit(rng *rand.Rand, name, orig, text string) string {
	switch k := rng.Intn(10); {
	case k < 6:
		base := text
		if rng.Intn(2) == 0 {
			base = orig
		}
		markers := spec.markers[name]
		marker := markers[rng.Intn(len(markers))]
		stmt := spec.statements[rng.Intn(len(spec.statements))]
		return strings.Replace(base, marker, stmt+" "+marker, 1)
	case k == 6:
		return orig
	case k == 7:
		return text // the same text with a new version
	case k == 8:
		return orig + fmt.Sprintf("\nfunc Extra%d() {}\n", rng.Intn(3))
	}
	d := spec.decls[rng.Intn(len(spec.decls))]
	return strings.Replace(orig, d[0], d[1], 1)
}

// genericDiffModule is diffModule with generic declarations in lib/g.gp, a generic function and a
// method of a generic type in lib/a.gp, and their uses in main: markers in the bodies of each kind.
func genericDiffModule() map[string]string {
	files := diffModule()
	files["lib/a.gp"] += `
func First[V any](xs []V) V {
	var zero V
	// MARK_GA
	if len(xs) == 0 {
		return zero
	}
	return xs[0]
}

func (l *List[V]) Last() V {
	var zero V
	// MARK_LA
	if len(l.items) == 0 {
		return zero
	}
	return l.items[len(l.items)-1]
}
`
	files["lib/g.gp"] = `package lib

type Number interface{ ~int | ~float64 }

func Sum[V Number](xs ...V) V {
	var s V
	// MARK_G
	for _, x := range xs {
		s += x
	}
	return s
}

func Pick[V any](x, y V) V { return x }

type Box[V any] struct{ v V }

func (b Box[V]) Get() V {
	var zero V
	// MARK_GB
	_ = zero
	return b.v
}

func NewBox[V any](v V) *Box[V] { return &Box[V]{v: v} }

type Pair[K comparable, V any] struct {
	Key K
	Val V
}

type List[V any] struct{ items []V }

func (l *List[V]) Push(x V) { l.items = append(l.items, x) }
`
	files["main/main.gp"] = strings.Replace(files["main/main.gp"], "\t// MARK_M\n",
		"\t// MARK_M\n\t_ = lib.Pick(1, 2) + lib.Sum(3)\n\t_ = lib.NewBox(\"x\").Get()\n", 1)
	return files
}

// genericDiff is the spec of genericDiffModule: the statements of plainDiff and ones with the
// generics of g.gp, with the type parameter V of the generic functions and methods (an undefined
// name elsewhere), and with instantiations that make an edge of weight 1 or a cycle (First[[]V] in
// First), where the focused check declines.
var genericDiff = &diffSpec{
	module: genericDiffModule,
	files:  []string{"lib/a.gp", "lib/b.gp", "lib/c.gp", "lib/g.gp", "main/main.gp"},
	markers: map[string][]string{
		"lib/a.gp":     {"// MARK_A", "// MARK_A2", "// MARK_GA", "// MARK_LA"},
		"lib/b.gp":     {"// MARK_B"},
		"lib/c.gp":     {"// MARK_C"},
		"lib/g.gp":     {"// MARK_G", "// MARK_GB"},
		"main/main.gp": {"// MARK_M"},
	},
	statements: append(slices.Clone(diffStatements),
		"_ = Pick(1, 2)", "_ = Pick(1, \"a\")", "_ = Sum(1, 2.5)", "_ = Sum(\"a\")", "_ = Pick[int, int](1, 2)",
		"b := NewBox(1)\n\t_ = b.Get()", "var b Box[string]\n\t_ = b.Get() + 1", "var p Pair[[]int, int]\n\t_ = p",
		"_ = Pick", "_ = Box{}", "var n Number\n\t_ = n", "var l List[int]\n\tl.Push(\"x\")",
		"_ = First([]string{\"a\"})", "_ = lib.Pick(1, \"a\")", "_ = lib.Sum[string]()",
		"_ = Pick[V](zero, zero)", "_ = Pick(zero, zero)", "var b Box[V]\n\t_ = b.Get()", "_ = Sum(zero)",
		"_ = zero + 1", "_ = NewBox(zero).Get()",
		"_ = Pick[[]V](nil, nil)", "var b Box[*V]\n\t_ = b", "_ = First[[]V](nil)",
		"type local struct{ v V }\n\t_ = Pick(local{}, local{})"),
	decls: [][2]string{
		{"func Join()", "func Join2()"}, {"Add(k int)", "Add(k, j int)"},
		{"func Pick[V any]", "func Pick[V comparable]"}, {"type Box[V any]", "type Box[V comparable]"},
		{"func First[V any](xs []V) V", "func First[V any](xs []V) (V, bool)"},
	},
}

// firstPublishAt waits for the first publish of the file at the version and returns its
// diagnostics.
func (c *dclient) firstPublishAt(name string, version int) string {
	c.t.Helper()
	deadline := time.Now().Add(hangLimit)
	for {
		c.pubMu.Lock()
		for _, p := range c.publishes {
			if p.URI == c.uri(name) && int(p.Version) == version {
				out := format(p)
				c.pubMu.Unlock()
				return out
			}
		}
		c.pubMu.Unlock()
		if time.Now().After(deadline) {
			c.t.Fatalf("no diagnostics of %s at version %d before the hang limit", name, version)
		}
		time.Sleep(200 * time.Microsecond)
	}
}

// lastPublish returns the last publish of the file, as its version and diagnostics.
func (c *dclient) lastPublish(name string) string {
	c.pubMu.Lock()
	defer c.pubMu.Unlock()
	out := "none"
	for _, p := range c.publishes {
		if p.URI == c.uri(name) {
			out = fmt.Sprintf("v%d:\n%s", p.Version, format(p))
		}
	}
	return out
}

// untilIdle waits until the diagnostics of the server have nothing waiting or running.
func (s *Server) untilIdle(t testing.TB) {
	t.Helper()
	deadline := time.Now().Add(hangLimit)
	for s.diags.deb.Busy() {
		if time.Now().After(deadline) {
			t.Fatal("the diagnostics were still busy at the hang limit")
		}
		time.Sleep(200 * time.Microsecond)
	}
}

// flushFull runs the full run of each folder of dirs at once, and then of every folder that has a
// run waiting or running, until there is none. A full run whose API changed runs its importers
// again (with the focused check), and such a run arms the settle run, which the test delays by an
// hour: one pass over the folders would leave that one waiting. A round that has not settled after
// a while replaces what is left by full runs again.
func (s *Server) flushFull(t testing.TB, dirs []string) {
	t.Helper()
	deadline := time.Now().Add(time.Minute)
	for {
		for _, dir := range dirs {
			s.diags.deb.Run(dir, 0, func(ctx context.Context) { s.diags.runFull(ctx, dir) })
		}
		round := time.Now().Add(200 * time.Millisecond)
		for s.diags.deb.Busy() && time.Now().Before(round) {
			time.Sleep(200 * time.Microsecond)
		}
		if !s.diags.deb.Busy() {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("the diagnostics were busy for a minute: %v", s.diags.deb.Keys())
		}
		dirs = s.diags.deb.Keys()
	}
}

// runDiffSession runs the script of the seed on a server, and returns the first publish of each
// edited version, the last publish of each file after the full run that settles the session, and
// how many focused checks the server made.
func runDiffSession(
	t *testing.T, spec *diffSpec, seed int64, phase int, noFocus bool,
) (steps, final map[string]string, quick int64) {
	t.Helper()
	files := spec.module()
	cfg := testConfig()
	cfg.Phase = phase
	var srv *Server
	c := startDiag(t, cfg, files, func(s *Server) {
		s.diags.settle = time.Hour // the test settles the session itself
		s.diags.noFocus = noFocus
		srv = s
	})
	type doc struct {
		orig, text string
		version    int
	}
	docs := make(map[string]*doc)
	for _, name := range spec.files {
		docs[name] = &doc{orig: files[name], text: files[name], version: 1}
		c.open(name, files[name])
	}
	for _, name := range spec.files {
		c.firstPublishAt(name, 1)
	}
	srv.untilIdle(t)

	rng := rand.New(rand.NewSource(seed))
	steps = make(map[string]string)
	for step := range 18 {
		name := spec.files[rng.Intn(len(spec.files))]
		d := docs[name]
		d.text = spec.diffEdit(rng, name, d.orig, d.text)
		d.version++
		c.change(name, d.version, d.text)
		steps[fmt.Sprintf("step %02d %s v%d", step, name, d.version)] = c.firstPublishAt(name, d.version)
	}

	// The full runs that wait for the settle delay run now.
	srv.flushFull(t, append(srv.diags.deb.Keys(), filepath.Join(c.root, "lib")))
	final = make(map[string]string)
	for _, name := range spec.files {
		final[name] = c.lastPublish(name)
	}
	return steps, final, srv.pipeline.FocusedChecks()
}

func TestDiagnosticsQuickMatchesFullRun(t *testing.T) {
	diagnosticsQuickMatchesFullRun(t, plainDiff, 25)
}

// TestDiagnosticsQuickMatchesFullRunGenerics is TestDiagnosticsQuickMatchesFullRun in a package with
// generic declarations (focus.go in internal/lsp/cache, "Generics").
func TestDiagnosticsQuickMatchesFullRunGenerics(t *testing.T) {
	diagnosticsQuickMatchesFullRun(t, genericDiff, 40)
}

func diagnosticsQuickMatchesFullRun(t *testing.T, spec *diffSpec, seeds int64) {
	for _, phase := range []int{check.LastPhase, version.Phase} {
		t.Run(fmt.Sprintf("phase %d", phase), func(t *testing.T) {
			compared, focused := 0, int64(0)
			for seed := int64(1); seed <= seeds; seed++ {
				quickSteps, quickFinal, n := runDiffSession(t, spec, seed, phase, false)
				fullSteps, fullFinal, none := runDiffSession(t, spec, seed, phase, true)
				focused += n
				if none != 0 {
					t.Errorf("seed %d: %d focused checks with the focused check off", seed, none)
				}
				var keys []string
				for key := range fullSteps {
					keys = append(keys, key)
				}
				sort.Strings(keys)
				for _, key := range keys {
					compared++
					if quickSteps[key] != fullSteps[key] {
						t.Errorf("seed %d, %s, the first publish:\nwith the focused check:\n%s\nwithout:\n%s",
							seed, key, quickSteps[key], fullSteps[key])
					}
				}
				for _, name := range spec.files {
					if quickFinal[name] != fullFinal[name] {
						t.Errorf("seed %d, %s after the full run:\nwith the focused check:\n%s\nwithout:\n%s",
							seed, name, quickFinal[name], fullFinal[name])
					}
				}
			}
			if focused == 0 {
				t.Error("the focused check answered none of the edits")
			}
			t.Logf("%d first publishes compared, %d focused checks", compared, focused)
		})
	}
}
