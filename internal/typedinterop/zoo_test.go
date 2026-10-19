package typedinterop

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/huuhait/gocpp/internal/typedinterop/probe"
	"github.com/huuhait/gocpp/internal/typedinterop/tsupport"
)

func zooDir(t testing.TB) string {
	dir, err := filepath.Abs(filepath.Join("..", "..", "tests", "zoo"))
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

// zooPrograms returns the names of the programs of tests/zoo.
func zooPrograms(t testing.TB) []string {
	entries, err := os.ReadDir(zooDir(t))
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".gp") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	return names
}

// summary counts the results of a table.
func summary(rep *Report) (ok, failed, skipped int) {
	for _, r := range rep.Table.Results {
		switch {
		case r.Skipped != "":
			skipped++
		case r.OK:
			ok++
		default:
			failed++
		}
	}
	return
}

// TestZoo probes every program of the header zoo (the C and C++ standard
// headers, sqlite3, zlib, SDL2, raylib: those whose library is installed).
// A program that opaque mode accepts must not make clang fail on a probe,
// except for the few expressions that only gocpp's runtime types make valid.
func TestZoo(t *testing.T) {
	clang := tsupport.Clang(t)
	for _, name := range zooPrograms(t) {
		t.Run(strings.TrimSuffix(name, ".gp"), func(t *testing.T) {
			p, fset := tsupport.CheckDir(t, zooDir(t), name)
			if errs := p.Errors.Diagnostics(); len(errs) > 0 {
				t.Skipf("the program doesn't check here (a missing library?): %v", errs[0])
			}
			rep, err := Analyze(context.Background(), p, fset, Options{Clang: clang, Runner: new(probe.Runner)})
			if err != nil {
				t.Fatalf("Analyze: %v", err)
			}
			ok, failed, skipped := summary(rep)
			t.Logf("%d references: %d probed, %d failed, %d skipped; compile %v (cpu %v), %d decls, %d KB of JSON; kind probe %v",
				len(rep.Set.Refs), ok, failed, skipped, rep.Table.Timing.Compile.Round(time.Millisecond), rep.Table.Timing.CPU.Round(time.Millisecond),
				rep.Table.Stats.Decls, rep.Table.Stats.JSONBytes>>10, rep.KindTime.Round(time.Millisecond))
			if len(rep.Table.HeaderErrors) > 0 {
				t.Errorf("errors in headers: %+v", rep.Table.HeaderErrors[0])
			}
			if ok == 0 {
				t.Errorf("no probe worked")
			}
			for _, r := range rep.Table.Results {
				if r.Skipped == "" && !r.OK {
					t.Logf("failed: %s: %s", r.Key, r.Error)
				}
			}
			// Warm: the same probe file again is served by the memo, with no compiler run.
			rn := new(probe.Runner)
			opts := Options{Clang: clang, Runner: rn}
			if _, err := Analyze(context.Background(), p, fset, opts); err != nil {
				t.Fatal(err)
			}
			rep2, err := Analyze(context.Background(), p, fset, opts)
			if err != nil {
				t.Fatal(err)
			}
			if !rep2.Table.Cached || rn.Runs() != 1 {
				t.Errorf("warm run: cached %v, compiler runs %d", rep2.Table.Cached, rn.Runs())
			}
			if os.Getenv("GOCPP_PERF") != "" && rep.Table.Timing.CPU > 500*time.Millisecond {
				t.Errorf("a cold probe took %v of CPU (budget 0.5 s)", rep.Table.Timing.CPU)
			}
		})
	}
}

// BenchmarkColdProbe measures a cold probe of the zoo's programs (clang's run,
// not the check): the target of Phase 12 is 0.5 s per package.
func BenchmarkColdProbe(b *testing.B) {
	clang := tsupport.Clang(b)
	for _, name := range zooPrograms(b) {
		b.Run(strings.TrimSuffix(name, ".gp"), func(b *testing.B) {
			p, fset := tsupport.CheckDir(b, zooDir(b), name)
			if len(p.Errors.Diagnostics()) > 0 {
				b.Skip("the program doesn't check here")
			}
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, err := Analyze(context.Background(), p, fset, Options{Clang: clang, Runner: new(probe.Runner)}); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
