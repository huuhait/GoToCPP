package icheck

import (
	"strings"
	"testing"

	"github.com/huuhait/gocpp/internal/build"
	"github.com/huuhait/gocpp/internal/typedinterop/probe"
)

// The checker says the same with a cold probe cache and with a warm one, and a warm run
// asks the compiler for nothing (ADR-088).
func TestCacheColdAndWarmGiveTheSameDiagnostics(t *testing.T) {
	dir := zooPath(t)
	t.Setenv(probe.EnvProbeCache, t.TempDir())
	t.Setenv(probe.EnvNoProbeCache, "")
	var names []string
	for _, prefix := range []string{"reject_", "accept_", "gorule_"} {
		names = append(names, zooFiles(t, prefix)...)
	}
	cold := make(map[string]string)
	for _, name := range names {
		cold[name] = format(dir, runCheck(t, dir, name, build.Clang).Diags)
	}
	probe.ResetCacheCounters()
	for _, name := range names {
		if got := format(dir, runCheck(t, dir, name, build.Clang).Diags); got != cold[name] {
			t.Errorf("%s: the diagnostics of a warm run differ:\n%s\nwant:\n%s", name, got, cold[name])
		}
	}
	if c := probe.CacheCounters(); c.Runs != 0 || c.DiskHits == 0 {
		t.Errorf("a warm run: %+v", c)
	}
	if len(names) < 20 || !strings.Contains(strings.Join(names, " "), "reject_") {
		t.Errorf("%d programs", len(names))
	}
}
