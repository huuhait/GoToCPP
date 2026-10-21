//go:build linux

package cache

import (
	"context"
	"fmt"
	"go/types"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/huuhait/gocpp/internal/typedinterop/probe"
)

// earlyScript runs the real compiler and records the runs that matter in $EARLY_DIR: a member
// completion run leaves "completion.PID" and a member declaration lookup run (its source has the
// classes derived from the receiver, gocpp_loc_class_N) leaves "locate.PID". With
// EARLY_MODE=meet each of them waits until the other kind has started (a run that waited in vain,
// 3 s, leaves "alone" and goes on); with EARLY_MODE=hold a lookup run waits for the file "open".
const earlyScript = `#!/bin/sh
for last; do :; done
kind=other
case " $* " in *-code-completion-at=*) kind=completion ;; esac
case "$(cat "$last" 2>/dev/null)" in *gocpp_loc_class_*) kind=locate ;; esac
case "$kind" in
  completion|locate) : > "$EARLY_DIR/$kind.$$" ;;
esac
other=locate
if [ "$kind" = locate ]; then other=completion; fi
tries=0
case "$EARLY_MODE:$kind" in
  meet:completion|meet:locate)
    while ! ls "$EARLY_DIR" | grep -q "^$other\."; do
      tries=$((tries + 1))
      if [ "$tries" -gt 300 ]; then : > "$EARLY_DIR/alone"; break; fi
      sleep 0.01
    done
    ;;
  hold:locate)
    while [ ! -e "$EARLY_DIR/open" ]; do
      tries=$((tries + 1))
      if [ "$tries" -gt 3000 ]; then exit 3; fi
      sleep 0.01
    done
    ;;
esac
exec "$EARLY_REAL" "$@"
`

const earlySource = `package main
import "./api.hpp"

func f() {
	var win api.Widget
	_ = win.size()
	_ = win.missing
}
`

const earlyHeader = `struct Widget {
  // Number of items.
  int size() const;
};
`

// earlyWorld makes the world of earlySource with the compiler of earlyScript; the member analysis
// starts when the test asks for the package (earlyOpen).
func earlyWorld(t *testing.T, mode string) (*world, string) {
	t.Helper()
	clang, err := probe.FindClang(context.Background())
	if err != nil {
		t.Skip(err)
	}
	dir := t.TempDir()
	compiler := filepath.Join(dir, "clang++")
	if err := os.WriteFile(compiler, []byte(earlyScript), 0o755); err != nil {
		t.Fatal(err)
	}
	runs := filepath.Join(dir, "runs")
	if err := os.Mkdir(runs, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("EARLY_DIR", runs)
	t.Setenv("EARLY_MODE", mode)
	t.Setenv("EARLY_REAL", clang.Path)
	t.Setenv("GOCPP_CLANG", compiler)
	t.Setenv(probe.EnvJobs, "4") // a member question and the lookup go together: two slots
	t.Setenv("GOCPP_INTEROP", "typed")
	t.Setenv("GOCPP_NOPROBECACHE", "1")
	t.Cleanup(probe.SetRacyWindow(0))
	world := newWorld(t, testConfig(), map[string]string{
		"gocpp.mod": "module example.com/early\n",
		"main.gp":   earlySource,
		"api.hpp":   earlyHeader,
	})
	t.Cleanup(world.p.Close)
	return world, runs
}

func earlyOpen(world *world) (*Package, *Snapshot, func()) {
	pkg := world.pkg(".")
	snap, release := world.s.Snapshot()
	return pkg, snap, release
}

func countRuns(t *testing.T, runs, kind string) int {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(runs, kind+".*"))
	if err != nil {
		t.Fatal(err)
	}
	return len(matches)
}

func firstBinding(facts *memberFacts) (variable *types.Var) {
	for object := range facts.bindings {
		return object
	}
	return nil
}

// The declaration lookup of the second step starts with the member questions: each of them waits
// for the other to have started, and the facts use its answers, although it also asked for a name
// that is no member (win.missing): no lookup run starts after the analysis has ended.
func TestMemberAnalysisStartsTheDeclarationLookupWithTheMemberQuestions(t *testing.T) {
	world, runs := earlyWorld(t, "meet")
	var atEnd atomic.Int32
	analyze := world.p.members.run
	world.p.members.run = func(ctx context.Context, pkg *Package) (*memberFacts, error) {
		facts, err := analyze(ctx, pkg)
		atEnd.Store(int32(countRuns(t, runs, "locate")))
		return facts, err
	}
	pkg, snap, release := earlyOpen(world)
	defer release()
	facts := waitMemberEntryWithin(t, world.p.members, memberEntryKey(snap, pkg), time.Minute)
	waitMemberDecls(t, snap, pkg, firstBinding(facts), "size")
	if _, err := os.Stat(filepath.Join(runs, "alone")); err == nil {
		t.Fatal("a run waited alone: the lookup started only after the member questions")
	}
	if atEnd.Load() == 0 {
		t.Fatal("no lookup run had started when the analysis ended")
	}
	if got := int32(countRuns(t, runs, "locate")); got != atEnd.Load() {
		t.Fatalf("lookup runs = %d, %d of them before the facts: the step ran the lookup again",
			got, atEnd.Load())
	}
}

// A lookup that is slow does not delay the facts for longer than the grace, and its answers come
// later, from a run of its own, when the compiler lets it go.
func TestMemberFactsDoNotWaitForASlowDeclarationLookup(t *testing.T) {
	previous := earlyGrace
	earlyGrace = 50 * time.Millisecond
	defer func() { earlyGrace = previous }()
	world, runs := earlyWorld(t, "hold")
	pkg, snap, release := earlyOpen(world)
	defer release()
	facts := waitMemberEntry(t, world.p.members, memberEntryKey(snap, pkg))
	object := firstBinding(facts)
	if _, ready := snap.ForeignMemberDeclarations(pkg, object, "size"); ready {
		t.Fatal("declarations are there although the lookup is held")
	}
	before := countRuns(t, runs, "locate")
	if err := os.WriteFile(filepath.Join(runs, "open"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	waitMemberDecls(t, snap, pkg, object, "size")
	if got := countRuns(t, runs, "locate"); got <= before {
		t.Fatalf("lookup runs = %d, none after the held one (%d): the stopped lookup gave answers",
			got, before)
	}
}

// analysisOutcome is how an analysis of the member analyzer ended.
type analysisOutcome struct {
	facts *memberFacts
	err   error
}

// recordAnalyses makes the analyzer of the world report how each of its analyses ended.
func recordAnalyses(world *world) <-chan analysisOutcome {
	outcomes := make(chan analysisOutcome, 8)
	analyze := world.p.members.run
	world.p.members.run = func(ctx context.Context, pkg *Package) (*memberFacts, error) {
		facts, err := analyze(ctx, pkg)
		select {
		case outcomes <- analysisOutcome{facts, err}:
		default:
		}
		return facts, err
	}
	return outcomes
}

// changeHeader writes a header that differs from earlyHeader in its size and its lines.
func changeHeader(t *testing.T, world *world) {
	t.Helper()
	changed := "\n\n" + earlyHeader
	header := filepath.Join(world.root, "api.hpp")
	if err := os.WriteFile(header, []byte(changed), 0o644); err != nil {
		t.Fatal(err)
	}
}

// hookEarlyDone sets the hook of the analysis that runs when the early lookup has ended.
func hookEarlyDone(t *testing.T, hook func()) {
	t.Helper()
	earlyDone = hook
	t.Cleanup(func() { earlyDone = nil })
}

// A header that changes after the early lookup has ended, and before the analysis compares the
// inputs with what they were, gives no facts: the names and types were asked for the old header.
func TestMemberAnalysisDropsFactsOfAHeaderChangedAfterTheEarlyLookup(t *testing.T) {
	world, _ := earlyWorld(t, "")
	outcomes := recordAnalyses(world)
	var once sync.Once
	hookEarlyDone(t, func() { once.Do(func() { changeHeader(t, world) }) })
	_, _, release := earlyOpen(world)
	defer release()
	select {
	case outcome := <-outcomes:
		if outcome.err == nil || outcome.facts != nil {
			t.Fatalf("facts of the old header: %v, error %v", outcome.facts, outcome.err)
		}
	case <-time.After(time.Minute):
		t.Fatal("the analysis did not end")
	}
}

// A header that changes after the facts are published, before the declaration step reads the
// answers of the early lookup (they were computed for the old header), gives no declarations.
func TestMemberStepDropsEarlyAnswersOfAHeaderChangedLater(t *testing.T) {
	world, _ := earlyWorld(t, "")
	stepped := make(chan struct{})
	var once sync.Once
	world.p.members.stepStart = func() { once.Do(func() { changeHeader(t, world) }) }
	world.p.members.stepDone = func() { close(stepped) }
	pkg, snap, release := earlyOpen(world)
	defer release()
	facts := waitMemberEntryWithin(t, world.p.members, memberEntryKey(snap, pkg), time.Minute)
	select {
	case <-stepped:
	case <-time.After(time.Minute):
		t.Fatal("the declaration step did not end")
	}
	if found, ready := snap.ForeignMemberDeclarations(pkg, firstBinding(facts), "size"); ready {
		t.Fatalf("declarations of the old header: %v", found)
	}
}

// wideSelectors is a program that writes more member names on a variable than a lookup asks
// for, before the one member that the header has.
func wideSelectors() string {
	var source strings.Builder
	source.WriteString("package main\nimport \"./api.hpp\"\n\nfunc f() {\n\tvar win api.Widget\n")
	for i := range maxMemberDeclNames + 6 {
		fmt.Fprintf(&source, "\t_ = win.absent%d\n", i)
	}
	source.WriteString("\t_ = win.size()\n}\n")
	return source.String()
}

// The early lookup asks for the names in the order of the program, up to a cap; a proven member
// beyond the cap is not in its answers, and the step asks for it itself.
func TestMemberStepAsksForAMemberBeyondTheCapOfTheEarlyLookup(t *testing.T) {
	t.Setenv("GOCPP_INTEROP", "typed")
	t.Setenv("GOCPP_NOPROBECACHE", "1")
	t.Setenv(probe.EnvJobs, "4")
	t.Cleanup(probe.SetRacyWindow(0))
	world := newWorld(t, testConfig(), map[string]string{
		"gocpp.mod": "module example.com/wide\n",
		"main.gp":   wideSelectors(),
		"api.hpp":   earlyHeader,
	})
	t.Cleanup(world.p.Close)
	pkg, snap, release := earlyOpen(world)
	defer release()
	facts := waitMemberEntryWithin(t, world.p.members, memberEntryKey(snap, pkg), time.Minute)
	waitMemberDecls(t, snap, pkg, firstBinding(facts), "size")
}
