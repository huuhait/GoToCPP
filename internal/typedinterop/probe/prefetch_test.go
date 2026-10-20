package probe

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
)

// The question about the includes of a preamble (includes.go) is asked beside the compile that needs its answer,
// not after it, and once for the runs that ask it at the same time.

func prefetchQueries(prefix string, n int) []DeclQuery {
	var qs []DeclQuery
	for i := 0; i < n; i++ {
		qs = append(qs, DeclQuery{DeclType, prefix + string(rune('A'+i))})
	}
	return qs
}

func TestIncludesQuestionIsAskedBesideTheCompile(t *testing.T) {
	f := newFake(t)
	f.header(t, "lib.hpp", "#include \"inner.hpp\"\n")
	f.header(t, "inner.hpp", "// inner\n")
	t.Setenv("FAKE_BARRIER", filepath.Join(t.TempDir(), "asked"))
	dc := DeclConfig{Includes: []string{`"lib.hpp"`}}
	tab, err := new(Runner).Decls(context.Background(), f.cfg(), dc, prefetchQueries("N", 1))
	if err != nil {
		t.Fatal(err)
	}
	if len(tab.Errors) != 0 && f.runs() != 1 {
		t.Errorf("%d runs, errors %v", f.runs(), tab.Errors)
	}
	if f.runs() != 1 || f.count("D") != 1 {
		t.Errorf("%d compiles asked the includes %d times, want 1 and 1", f.runs(), f.count("D"))
	}
	for name, msg := range tab.Errors {
		if msg != "" && strings.Contains(msg, "did not come") {
			t.Errorf("%s: %s", name, msg)
		}
	}
}

// The declaration runs of one call have different texts and the same preamble: one question serves them
// all, also when they run at the same time, and a later call asks none.
func TestIncludesQuestionIsSharedByConcurrentRuns(t *testing.T) {
	f := newFake(t)
	f.header(t, "lib.hpp", "#include \"inner.hpp\"\n")
	f.header(t, "inner.hpp", "// inner\n")
	t.Setenv("FAKE_SLEEP", "0.3") // (the runs overlap)
	dc := DeclConfig{Includes: []string{`"lib.hpp"`}, Parallel: 4}
	rn := new(Runner)
	if _, err := rn.Decls(context.Background(), f.cfg(), dc, prefetchQueries("P", 4)); err != nil {
		t.Fatal(err)
	}
	if f.runs() != 4 || f.count("D") != 1 {
		t.Errorf("4 concurrent runs: %d compiles, the includes asked %d times, want 4 and 1", f.runs(), f.count("D"))
	}
	if _, err := rn.Decls(context.Background(), f.cfg(), dc, prefetchQueries("Q", 4)); err != nil {
		t.Fatal(err)
	}
	if f.runs() != 8 || f.count("D") != 1 {
		t.Errorf("a second call: %d compiles, the includes asked %d times, want 8 and 1", f.runs(), f.count("D"))
	}
}
