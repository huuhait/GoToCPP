package typedinterop

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/huuhait/gocpp/internal/typedinterop/probe"
	"github.com/huuhait/gocpp/internal/typedinterop/tsupport"
)

// BenchmarkWarmProbe measures what a warm typed run costs on top of the check: the kind probe's
// and the probes' answers all come from the cache on disk (a new Runner per call, as a new
// process has). Set GOCPP_BENCH_PROGRAM to the .gp file of a program (in its own folder, with the
// headers it imports); GOCPP_BENCH_DECLS=1 includes the declarations.
func BenchmarkWarmProbe(b *testing.B) {
	file := os.Getenv("GOCPP_BENCH_PROGRAM")
	if file == "" {
		b.Skip("GOCPP_BENCH_PROGRAM is not set")
	}
	cache, err := os.MkdirTemp("", "gocpp-bench-")
	if err != nil {
		b.Fatal(err)
	}
	defer os.RemoveAll(cache)
	b.Setenv(probe.EnvProbeCache, cache)
	b.Setenv(probe.EnvNoProbeCache, "")
	clang := tsupport.Clang(b)
	p, fset := tsupport.CheckDir(b, filepath.Dir(file), filepath.Base(file))
	opts := func() Options {
		return Options{Clang: clang, Runner: new(probe.Runner), Decls: os.Getenv("GOCPP_BENCH_DECLS") != ""}
	}
	if _, err := Analyze(context.Background(), p, fset, opts()); err != nil {
		b.Fatal(err)
	}
	probe.ResetCacheCounters()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rep, err := Analyze(context.Background(), p, fset, opts())
		if err != nil {
			b.Fatal(err)
		}
		if !rep.Table.Cached {
			b.Fatal("not served from the cache")
		}
	}
	if c := probe.CacheCounters(); c.Runs != 0 {
		b.Errorf("warm run made %d compiler runs", c.Runs)
	}
}
