package typedinterop

import (
	"context"
	"encoding/json"
	"strconv"
	"testing"

	"github.com/huuhait/gocpp/internal/typedinterop/probe"
	"github.com/huuhait/gocpp/internal/typedinterop/tsupport"
)

// The report of a program is the same bytes from a cold cache and from a warm one (Stable
// leaves out the timing, the statistics and the cache flags), and a warm run asks the compiler
// for nothing: the probes, the declarations and the kinds all come from the disk (ADR-088).
func TestCacheColdWarmZoo(t *testing.T) {
	clang := tsupport.Clang(t)
	t.Setenv(probe.EnvProbeCache, t.TempDir())
	t.Setenv(probe.EnvNoProbeCache, "")
	ctx := context.Background()
	cold := make(map[string][]byte)
	var names []string
	for _, name := range zooPrograms(t) {
		p, fset := tsupport.CheckDir(t, zooDir(t), name)
		if errs := p.Errors.Diagnostics(); len(errs) > 0 {
			continue // a library that is not installed here
		}
		rep, err := Analyze(ctx, p, fset, Options{Clang: clang, Runner: new(probe.Runner), Decls: true})
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		rep.Stable = true
		b, err := json.Marshal(rep)
		if err != nil {
			t.Fatal(err)
		}
		cold[name] = b
		names = append(names, name)
	}
	if len(names) < 15 {
		t.Fatalf("only %d programs", len(names))
	}
	probe.ResetCacheCounters()
	for _, name := range names {
		p, fset := tsupport.CheckDir(t, zooDir(t), name)
		rn := new(probe.Runner)
		rep, err := Analyze(ctx, p, fset, Options{Clang: clang, Runner: rn, Decls: true})
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if !rep.Table.Cached || rn.Runs() != 0 || rn.DeclRuns() != 0 {
			t.Errorf("%s: warm run: cached %v, %d probe runs, %d declaration runs", name, rep.Table.Cached, rn.Runs(), rn.DeclRuns())
		}
		rep.Stable = true
		b, err := json.Marshal(rep)
		if err != nil {
			t.Fatal(err)
		}
		if string(b) != string(cold[name]) {
			t.Errorf("%s: a warm run prints other bytes than a cold one:\n%s", name, firstDifference(string(cold[name]), string(b)))
		}
	}
	if c := probe.CacheCounters(); c.Runs != 0 {
		t.Errorf("warm runs: %+v", c)
	}
}

// firstDifference shows where two texts differ.
func firstDifference(a, b string) string {
	i := 0
	for i < len(a) && i < len(b) && a[i] == b[i] {
		i++
	}
	lo := max(0, i-80)
	return "cold: ..." + a[lo:min(len(a), i+80)] + "\nwarm: ..." + b[lo:min(len(b), i+80)] + "\n(offset " + strconv.Itoa(i) + ")"
}
