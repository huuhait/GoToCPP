//go:build linux

package cache

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/huuhait/gocpp/internal/typedinterop/probe"
)

// rendezvousScript runs the real compiler, except that a member completion run (the question of
// the members of one receiver type) leaves a file in $RENDEZVOUS_DIR and waits until two such
// runs have started. The wait has no time limit, which a loaded machine could reach: questions
// asked one after the other never end, and the test fails at its hang limit. The wait ends when
// the test process ($RENDEZVOUS_TEST_PID) is gone.
const rendezvousScript = `#!/bin/sh
case " $* " in
  *-code-completion-at=*)
    : > "$RENDEZVOUS_DIR/run.$$"
    while [ "$(ls "$RENDEZVOUS_DIR" | wc -l)" -lt 2 ]; do
      kill -0 "$RENDEZVOUS_TEST_PID" 2>/dev/null || exit 3
      sleep 0.01
    done
    ;;
esac
exec "$RENDEZVOUS_REAL" "$@"
`

const twoReceiversHeader = `struct Widget { int size() const; };
struct Other { double ratio() const; };
`

const twoReceiversSource = `package main
import "./api.hpp"

func f() {
	var win api.Widget
	var other api.Other
	_ = win.size()
	_ = other.ratio()
}
`

func TestMemberAnalysisAsksTheMembersOfDifferentReceiversTogether(t *testing.T) {
	clang, err := probe.FindClang(context.Background())
	if err != nil {
		t.Skip(err)
	}
	dir := t.TempDir()
	compiler := filepath.Join(dir, "clang++")
	if err := os.WriteFile(compiler, []byte(rendezvousScript), 0o755); err != nil {
		t.Fatal(err)
	}
	runs := filepath.Join(dir, "runs")
	if err := os.Mkdir(runs, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("RENDEZVOUS_DIR", runs)
	t.Setenv("RENDEZVOUS_TEST_PID", strconv.Itoa(os.Getpid()))
	// A question that waits alone must hang until the test's limit, not be stopped by the limit
	// of a compiler run and asked again on its own.
	t.Setenv(probe.EnvTimeout, "10m")
	t.Setenv("RENDEZVOUS_REAL", clang.Path)
	t.Setenv("GOCPP_CLANG", compiler)
	t.Setenv(probe.EnvJobs, "4") // two runs together need two compiler slots
	t.Setenv("GOCPP_INTEROP", "typed")
	t.Setenv("GOCPP_NOPROBECACHE", "1")
	t.Cleanup(probe.SetRacyWindow(0))
	world := newWorld(t, testConfig(), map[string]string{
		"gocpp.mod": "module example.com/members\n",
		"main.gp":   twoReceiversSource,
		"api.hpp":   twoReceiversHeader,
	})
	t.Cleanup(world.p.Close)
	pkg := world.pkg(".")
	snap, release := world.s.Snapshot()
	defer release()
	// (questions asked one after the other wait for each other at the compiler: no facts)
	facts := waitMemberEntryWithin(t, world.p.members, memberEntryKey(snap, pkg), hangLimit)
	if len(facts.bindings) != 2 {
		t.Fatalf("bindings = %d, want the two variables", len(facts.bindings))
	}
	if entries, _ := os.ReadDir(runs); len(entries) < 2 {
		t.Fatalf("member questions that were asked = %d, want one for each receiver", len(entries))
	}
}
