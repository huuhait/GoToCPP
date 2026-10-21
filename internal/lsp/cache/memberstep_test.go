package cache

import (
	"context"
	"errors"
	"go/types"
	"sync/atomic"
	"testing"
	"time"

	"github.com/huuhait/gocpp/internal/typedinterop/probe"
)

// The declarations of members are a second step after the member facts are published: a step
// that is slow or fails must not delay or lose the names and types of the facts.

func memberWorldWithStep(t *testing.T, stepDone func(),
	step func(context.Context) (map[string][]probe.Declaration, int64, error),
) (
	*world, *Package, *Snapshot, *types.Var, func(),
) {
	t.Helper()
	t.Setenv("GOCPP_INTEROP", "typed")
	t.Setenv("GOCPP_NOPROBECACHE", "1")
	t.Cleanup(probe.SetRacyWindow(0))
	world := newWorld(t, testConfig(), map[string]string{
		"gocpp.mod": "module example.com/members\n",
		"main.gp":   memberSource,
		"api.hpp":   memberAPIHeader,
	})
	t.Cleanup(world.p.Close)
	world.p.members.stepDone = stepDone
	run := world.p.members.run
	world.p.members.run = func(ctx context.Context, pkg *Package) (*memberFacts, error) {
		facts, err := run(ctx, pkg)
		if facts != nil {
			if facts.pending == nil {
				t.Error("the analysis has no second step for a program that writes members")
			}
			facts.pending = step
		}
		return facts, err
	}
	pkg := world.pkg(".")
	snap, release := world.s.Snapshot()
	facts := waitMemberEntry(t, world.p.members, memberEntryKey(snap, pkg))
	var win *types.Var
	for object := range facts.bindings {
		win = object
	}
	return world, pkg, snap, win, release
}

func TestMemberFactsDoNotWaitForTheDeclarationStep(t *testing.T) {
	started := make(chan struct{}, 1)
	release := make(chan struct{})
	step := func(ctx context.Context) (map[string][]probe.Declaration, int64, error) {
		select {
		case started <- struct{}{}:
		default:
		}
		select {
		case <-release:
		case <-ctx.Done():
		}
		return map[string][]probe.Declaration{
			memberDeclKey("Widget", "size"): {{Kind: "method", Signature: "int Widget::size() const"}},
		}, 64, nil
	}
	_, pkg, snap, win, done := memberWorldWithStep(t, nil, step)
	defer done()
	headerSignal(t, started)
	names, incomplete, ready := snap.MemberSymbols(pkg, win)
	if !ready || incomplete || len(names) != 3 {
		t.Fatalf("member names while the step runs = %v %v %v", names, incomplete, ready)
	}
	if native, _, ok := snap.ForeignVariableType(pkg, win); !ok || native != "Widget" {
		t.Fatalf("native type while the step runs = %q %v", native, ok)
	}
	if got, ok := snap.ForeignMemberDeclarations(pkg, win, "size"); ok {
		t.Fatalf("declarations before the step is done: %v", got)
	}
	close(release)
	waitMemberDecls(t, snap, pkg, win, "size")
	if names, _, ready := snap.MemberSymbols(pkg, win); !ready || len(names) != 3 {
		t.Errorf("member names after the step = %v %v", names, ready)
	}
}

func TestMemberFactsSurviveAFailingDeclarationStep(t *testing.T) {
	var calls atomic.Int32
	step := func(context.Context) (map[string][]probe.Declaration, int64, error) {
		calls.Add(1)
		return nil, 0, errors.New("the compiler failed")
	}
	_, pkg, snap, win, done := memberWorldWithStep(t, nil, step)
	defer done()
	deadline := time.Now().Add(15 * time.Second)
	for calls.Load() < 2 { // the step and its one retry
		if time.Now().After(deadline) {
			t.Fatalf("the step was run %d times", calls.Load())
		}
		time.Sleep(5 * time.Millisecond)
	}
	if names, _, ready := snap.MemberSymbols(pkg, win); !ready || len(names) != 3 {
		t.Fatalf("member names after a failed step = %v %v", names, ready)
	}
	if native, _, ok := snap.ForeignVariableType(pkg, win); !ok || native != "Widget" {
		t.Fatalf("native type after a failed step = %q %v", native, ok)
	}
	if got, ok := snap.ForeignMemberDeclarations(pkg, win, "size"); ok {
		t.Fatalf("declarations of a failed step: %v", got)
	}
}

func TestMemberDeclarationStepOfAReplacedEntryIsDropped(t *testing.T) {
	started := make(chan struct{}, 1)
	release := make(chan struct{})
	step := func(ctx context.Context) (map[string][]probe.Declaration, int64, error) {
		started <- struct{}{}
		<-release
		return map[string][]probe.Declaration{
			memberDeclKey("Widget", "size"): {{Kind: "method", Signature: "stale"}},
		}, 64, nil
	}
	ended := make(chan struct{})
	world, pkg, snap, win, done := memberWorldWithStep(t, func() { close(ended) }, step)
	defer done()
	headerSignal(t, started)
	world.p.members.mu.Lock()
	for _, entry := range world.p.members.entries {
		entry.facts = nil // the entry was refreshed: its facts are not the ones the step was for
	}
	world.p.members.mu.Unlock()
	close(release)
	headerSignal(t, ended)
	if got, ok := snap.ForeignMemberDeclarations(pkg, win, "size"); ok {
		t.Fatalf("the step of replaced facts published: %v", got)
	}
}
