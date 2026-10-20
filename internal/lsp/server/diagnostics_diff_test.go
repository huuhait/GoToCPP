package server

import (
	"context"
	"fmt"
	"math/rand"
	"path/filepath"
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

// diffEdit returns the next text of the file: mostly a statement at a marker (now and then after a
// revert to the original), and now and then a revert, a new function, a renamed function or a new
// parameter.
func diffEdit(rng *rand.Rand, name, orig, text string) string {
	switch k := rng.Intn(10); {
	case k < 6:
		base := text
		if rng.Intn(2) == 0 {
			base = orig
		}
		markers := diffMarkers[name]
		marker := markers[rng.Intn(len(markers))]
		stmt := diffStatements[rng.Intn(len(diffStatements))]
		return strings.Replace(base, marker, stmt+" "+marker, 1)
	case k == 6:
		return orig
	case k == 7:
		return text // the same text with a new version
	case k == 8:
		return orig + fmt.Sprintf("\nfunc Extra%d() {}\n", rng.Intn(3))
	}
	switch rng.Intn(2) {
	case 0:
		return strings.Replace(orig, "func Join()", "func Join2()", 1)
	}
	return strings.Replace(orig, "Add(k int)", "Add(k, j int)", 1)
}

// firstPublishAt waits for the first publish of the file at the version and returns its
// diagnostics.
func (c *dclient) firstPublishAt(name string, version int) string {
	c.t.Helper()
	deadline := time.Now().Add(10 * time.Second)
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
			c.t.Fatalf("no diagnostics of %s at version %d in 10 s", name, version)
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
	deadline := time.Now().Add(10 * time.Second)
	for s.diags.deb.Busy() {
		if time.Now().After(deadline) {
			t.Fatal("the diagnostics were busy for 10 s")
		}
		time.Sleep(200 * time.Microsecond)
	}
}

// runDiffSession runs the script of the seed on a server, and returns the first publish of each
// edited version, the last publish of each file after the full run that settles the session, and
// how many focused checks the server made.
func runDiffSession(
	t *testing.T, seed int64, phase int, noFocus bool,
) (steps, final map[string]string, quick int64) {
	t.Helper()
	files := diffModule()
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
	for _, name := range diffFiles {
		docs[name] = &doc{orig: files[name], text: files[name], version: 1}
		c.open(name, files[name])
	}
	for _, name := range diffFiles {
		c.firstPublishAt(name, 1)
	}
	srv.untilIdle(t)

	rng := rand.New(rand.NewSource(seed))
	steps = make(map[string]string)
	for step := range 18 {
		name := diffFiles[rng.Intn(len(diffFiles))]
		d := docs[name]
		d.text = diffEdit(rng, name, d.orig, d.text)
		d.version++
		c.change(name, d.version, d.text)
		steps[fmt.Sprintf("step %02d %s v%d", step, name, d.version)] = c.firstPublishAt(name, d.version)
	}

	// The full runs that wait for the settle delay run now.
	dirs := append(srv.diags.deb.Keys(), filepath.Join(c.root, "lib"))
	for _, dir := range dirs {
		srv.diags.deb.Run(dir, 0, func(ctx context.Context) { srv.diags.runFull(ctx, dir) })
	}
	srv.untilIdle(t)
	final = make(map[string]string)
	for _, name := range diffFiles {
		final[name] = c.lastPublish(name)
	}
	return steps, final, srv.pipeline.FocusedChecks()
}

func TestDiagnosticsQuickMatchesFullRun(t *testing.T) {
	for _, phase := range []int{check.LastPhase, version.Phase} {
		t.Run(fmt.Sprintf("phase %d", phase), func(t *testing.T) {
			compared, focused := 0, int64(0)
			for seed := int64(1); seed <= 25; seed++ {
				quickSteps, quickFinal, n := runDiffSession(t, seed, phase, false)
				fullSteps, fullFinal, none := runDiffSession(t, seed, phase, true)
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
				for _, name := range diffFiles {
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
