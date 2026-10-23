package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/huuhait/gocpp/internal/typedinterop/probe"
)

// The typed probes' cache (ADR-088) through the developer command: a warm run prints the same
// bytes as a cold one (-stable leaves out the timing), asks the compiler for nothing, and
// -cache-stats and -cache-clean show and empty the folder.
func TestTypedProbeCache(t *testing.T) {
	t.Setenv("GOCPP_MODCACHE", "off")
	t.Setenv("GOCPP_OFFLINE", "1")
	t.Setenv("GOCPP_PROBE_TIMEOUT", "5m")
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	cache := t.TempDir()
	t.Setenv(probe.EnvProbeCache, cache)
	t.Setenv(probe.EnvNoProbeCache, "")
	inDir(t, map[string]string{"main.gp": diffProgram, "lib.hpp": diffHeader})
	// A header changed in the last two seconds is not kept (the cache's racy rule); this one is ordinary.
	defer probe.SetRacyWindow(0)()

	stats := func() probe.Usage {
		r := gocppRun("", false, "__typed-probe", "-cache-stats")
		if r.code != 0 {
			t.Fatalf("-cache-stats: %+v", r)
		}
		var u probe.Usage
		if err := json.Unmarshal([]byte(r.stdout), &u); err != nil {
			t.Fatalf("%v\n%s", err, r.stdout)
		}
		return u
	}
	if u := stats(); u.Entries != 0 || !strings.HasPrefix(u.Dir, cache) {
		t.Errorf("empty cache: %+v", u)
	}

	probe.ResetCacheCounters()
	cold := gocppRun("", false, "__typed-probe", "-decls", "-stable", "-stats", ".")
	if cold.code != 0 {
		if cold.code == 3 {
			t.Skip("no clang++ for the probe")
		}
		t.Fatalf("cold: %+v", cold)
	}
	coldRuns := probe.CacheCounters().Runs
	if coldRuns == 0 || !strings.Contains(cold.stderr, `"Batches":1`) {
		t.Errorf("cold run: %d compiler runs, stats %q", coldRuns, cold.stderr)
	}
	u := stats()
	if u.Entries == 0 || u.Kinds["c"] == 0 || u.Kinds["u"] == 0 || u.Kinds["d"] == 0 || u.Bytes == 0 {
		t.Errorf("after the cold run: %+v", u)
	}

	probe.ResetCacheCounters()
	warm := gocppRun("", false, "__typed-probe", "-decls", "-stable", "-stats", ".")
	if warm.code != 0 {
		t.Fatalf("warm: %+v", warm)
	}
	if c := probe.CacheCounters(); c.Runs != 0 || c.DiskHits == 0 {
		t.Errorf("warm run: %+v", c)
	}
	if !strings.Contains(warm.stderr, `"Batches":0`) || !strings.Contains(warm.stderr, `"Hits":`) {
		t.Errorf("warm stats %q", warm.stderr)
	}
	if cold.stdout != warm.stdout {
		t.Errorf("a warm run prints other bytes than a cold one (%d and %d bytes)", len(cold.stdout), len(warm.stdout))
	}
	// Without -stable the run's own numbers are in the JSON (and differ).
	if raw := gocppRun("", false, "__typed-probe", "-decls", "."); !strings.Contains(raw.stdout, `"timing"`) {
		t.Errorf("no timing without -stable")
	}

	// GOCPP_PROBE_CACHE=off: no cache, the compiler runs again.
	t.Setenv(probe.EnvProbeCache, "off")
	probe.ResetCacheCounters()
	if off := gocppRun("", false, "__typed-probe", "-decls", "-stable", "."); off.code != 0 || withoutDeps(t, off.stdout) != withoutDeps(t, cold.stdout) {
		a, b := withoutDeps(t, cold.stdout), withoutDeps(t, off.stdout)
		i := 0
		for i < len(a) && i < len(b) && a[i] == b[i] {
			i++
		}
		t.Errorf("cache off: %d, the output differs at %d:\n%s\n%s", off.code, i, a[max(0, i-100):min(len(a), i+150)], b[max(0, i-100):min(len(b), i+150)])
	}
	if probe.CacheCounters().Runs == 0 {
		t.Error("the cache was used while off")
	}
	if r := gocppRun("", false, "__typed-probe", "-cache-stats"); r.code != 0 || !strings.Contains(r.stderr, "is off") {
		t.Errorf("-cache-stats with the cache off: %+v", r)
	}
	t.Setenv(probe.EnvProbeCache, cache)

	// -cache-clean empties the folder, and the next run fills it again.
	r := gocppRun("", false, "__typed-probe", "-cache-clean")
	if r.code != 0 || !strings.Contains(r.stderr, "removed") {
		t.Errorf("-cache-clean: %+v", r)
	}
	if u := stats(); u.Entries != 0 {
		t.Errorf("after -cache-clean: %+v", u)
	}
	probe.ResetCacheCounters()
	again := gocppRun("", false, "__typed-probe", "-decls", "-stable", ".")
	if again.code != 0 || again.stdout != cold.stdout || probe.CacheCounters().Runs == 0 {
		t.Errorf("after the clean: %d, same %v, runs %d", again.code, again.stdout == cold.stdout, probe.CacheCounters().Runs)
	}
	if u := stats(); u.Entries == 0 {
		t.Error("the cache was not filled again")
	}
}

// withoutDeps is the JSON of a report without the lists of headers and the key made from them: a run with
// the cache also lists the places where other headers would hide the ones that were read.
func withoutDeps(t *testing.T, text string) string {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(text), &m); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"table", "decls"} {
		if sub, ok := m[k].(map[string]any); ok {
			delete(sub, "deps")
			delete(sub, "key") // (it has the hash of the contents of the headers listed)
		}
	}
	b, _ := json.Marshal(m)
	return string(b)
}

// Programs of tests/typed emitted one after the other with one probe cache say what each says with
// a cache of its own. The same probe has another reference number in each program (gp_probe_4,
// gp_probe_10), and the cache kept the column of the first one's error: the next program's checker
// read the error one character early, at no argument, and reported a void** out-parameter that the
// lowering adapts ("cannot use ... **adapt.Obj as *unsafe.Pointer").
func TestTypedProbeCacheSharedByPrograms(t *testing.T) {
	typedEnv(t)
	typedIn(t)
	needClang(t)
	t.Setenv("GOCPP_INTEROP", "typed")
	t.Setenv(probe.EnvNoProbeCache, "")
	// The headers were copied a moment ago; the cache's racy rule would keep nothing.
	defer probe.SetRacyWindow(0)()
	for _, seq := range [][]string{
		{"err_adapt_defer.gp", "adapt_voidout.gp"},
		{"hresult_reassign.gp", "hresult_raw.gp", "hresult_error.gp"},
		{"hresult_method_only.gp", "hresult_error.gp", "hresult_raw.gp"},
	} {
		t.Run(strings.TrimSuffix(seq[0], ".gp"), func(t *testing.T) {
			shared := t.TempDir()
			for i, p := range seq {
				t.Setenv(probe.EnvProbeCache, t.TempDir())
				alone := gocppRun("", false, "emit", p)
				t.Setenv(probe.EnvProbeCache, shared)
				probe.ResetCacheCounters()
				got := gocppRun("", false, "emit", p)
				c := probe.CacheCounters()
				if got.code != alone.code || got.stderr != alone.stderr {
					t.Errorf("%s after %v with one cache: exit %d\n%s\nwith a cache of its own: exit %d\n%s",
						p, seq[:i], got.code, got.stderr, alone.code, alone.stderr)
				}
				if i > 0 && c.MemHits+c.DiskHits == 0 {
					t.Errorf("%s after %v: nothing came from the cache (%+v)", p, seq[:i], c)
				}
			}
		})
	}
}
