package probe

import (
	"context"
	"errors"
	"testing"
)

// What a type run does with the answers of its try with the precompiled header (typeRun): a refused answer
// and a compiler error are made again without the header, a canceled call is not. The tests replace typeDump
// to make each case happen at the moment they want; none reads a clock.

// typeDumpWithHeader replaces typeDump for the runs that load the header: after calls the real one,
// the replacement gets its answer and its error and returns what it wants. The other runs are not changed.
func typeDumpWithHeader(t *testing.T, after func(part *declPart, err error) (*declPart, error)) {
	t.Helper()
	real := typeDump
	t.Cleanup(func() { typeDump = real })
	typeDump = func(ctx context.Context, cfg Config, text string, r *typeRun, lineQ map[int]int) (*declPart, error) {
		part, err := real(ctx, cfg, text, r, lineQ)
		if cfg.pch == nil {
			return part, err
		}
		return after(part, err)
	}
}

func tmCase(t *testing.T) (Config, DeclConfig, []DeclQuery) {
	t.Helper()
	cfg := clangCacheConfig(t, t.TempDir())
	t.Setenv(EnvPCH, "wait")
	ResetCacheCounters()
	return cfg, DeclConfig{Includes: []string{"<ctime>"}}, []DeclQuery{{DeclType, "tm"}}
}

// A call canceled right after the refusal of the header's answer returns the cancellation: the run has no
// part to hand to the table, and it does not start the run without the header.
func TestCanceledTypeRunAfterRefusalReturnsTheCancellation(t *testing.T) {
	cfg, dc, qs := tmCase(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	refused := false
	typeDumpWithHeader(t, func(part *declPart, err error) (*declPart, error) {
		refused = part == nil && err == nil
		cancel()
		return part, err
	})
	_, err := new(Runner).Decls(ctx, cfg, dc, qs)
	if !refused {
		t.Fatal("the header's answer for tm was not refused: the test shows nothing")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("error: %v, want the cancellation", err)
	}
	if c := CacheCounters(); c.TypeRechecks != 0 {
		t.Errorf("%d type runs were made again after the cancellation", c.TypeRechecks)
	}
}

// A compiler error in the run with the header is not a refusal: the run is made again without the header,
// and the counter of refused answers does not move.
func TestTypeRunWithHeaderErrorIsMadeAgainWithoutCountingARefusal(t *testing.T) {
	cfg, dc, qs := tmCase(t)
	typeDumpWithHeader(t, func(part *declPart, err error) (*declPart, error) {
		return nil, errors.New("the compiler failed")
	})
	tab, err := new(Runner).Decls(context.Background(), cfg, dc, qs)
	if err != nil {
		t.Fatal(err)
	}
	if tm := tab.Records["tm"]; tm == nil || len(tm.Fields) < 9 {
		t.Errorf("struct tm: %+v", tm)
	}
	c := CacheCounters()
	if c.PCHUses != 1 || c.TypeRechecks != 0 {
		t.Errorf("%d runs loaded the header and %d answers were refused, want 1 and 0", c.PCHUses, c.TypeRechecks)
	}
}

// A refused answer is counted once, and the run without the header gives the whole declaration.
func TestTypeRunRefusedForTheHeaderIsCountedOnce(t *testing.T) {
	cfg, dc, qs := tmCase(t)
	tab, err := new(Runner).Decls(context.Background(), cfg, dc, qs)
	if err != nil {
		t.Fatal(err)
	}
	if tm := tab.Records["tm"]; tm == nil || len(tm.Fields) < 9 {
		t.Errorf("struct tm: %+v", tm)
	}
	if c := CacheCounters(); c.TypeRechecks != 1 {
		t.Errorf("%d answers were refused, want 1", c.TypeRechecks)
	}
}
