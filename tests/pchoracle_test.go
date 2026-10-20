package tests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
)

// The oracle of the precompiled header of the probes (Phase 12 step 12.14, probe/pch.go): the answers of the
// typed pipeline for every corpus program that imports a header are the same with the header and without it.
// `gocpp __typed-probe -decls -stable` prints the reference table, the probe results, the declarations and
// the foreign view of each result of the program as JSON; the test runs it twice with empty caches, once with
// GOCPP_PROBE_PCH=off (the old behavior: the oracle) and once with GOCPP_PROBE_PCH=wait (every compiler run
// loads the header), and compares the two documents. It also counts: the second run loaded the header in some
// of its compiler runs (the probe runs and the function runs; not the type runs), the first in none.
//
// Nothing is allowed to differ: the type runs, whose dumps clang cuts short when they come from a precompiled
// header, are made without it.
//
// Which programs. The corpus has the programs of the phases up to GOCPP_TEST_PHASE (default: the current
// phase of the build). At the default phase the test compares 174 programs; with GOCPP_TEST_PHASE=12 it compares
// 286, which is the run that the step's measurements are made with (and the one that TestTypedCheckCorpus,
// whose floor is 200 programs, needs as well). The floor of 150 below only catches a broken loader.

var recheckTotal atomic.Int64

var statsLine = regexp.MustCompile(`stats (\{.*\})`)

type probeOutcome struct {
	docs   []map[string]any
	exit   int
	stderr string
	pch    struct{ builds, uses, rejects, rechecks, runs int }
}

// typedProbe runs `gocpp __typed-probe -decls -stable -stats .` in the program's folder with a cache of its own
// and the setting of GOCPP_PROBE_PCH.
func (c *config) typedProbe(t *testing.T, p *program, pch string) probeOutcome {
	t.Helper()
	cache := t.TempDir()
	env := append(runEnv(p.header),
		"GOCPP_PROBE_PCH="+pch,
		"GOCPP_PROBE_CACHE="+filepath.Join(cache, "probe"),
		"XDG_CACHE_HOME="+cache,
	)
	j := gocppJob(p.dir, env, "", p.header.phase, "__typed-probe", "-decls", "-stable", "-stats", p.arg)
	r := c.run(t, j)
	out := probeOutcome{exit: r.exit, stderr: string(r.stderr)}
	if r.problem != "" {
		t.Fatalf("%s: %s", r.cmd, r.why())
	}
	dec := json.NewDecoder(bytes.NewReader(r.stdout))
	for {
		var doc map[string]any
		if err := dec.Decode(&doc); err == io.EOF {
			break
		} else if err != nil {
			t.Fatalf("%s: the output is not JSON: %v", r.cmd, err)
		}
		out.docs = append(out.docs, doc)
	}
	// The counters are the process's, printed after each package: the last line has the totals.
	for _, m := range statsLine.FindAllStringSubmatch(out.stderr, -1) {
		var st struct {
			Process struct {
				PCHBuilds, PCHUses, PCHRejects, PCHRechecks, Runs int
			}
		}
		if json.Unmarshal([]byte(m[1]), &st) == nil {
			out.pch.builds, out.pch.uses = st.Process.PCHBuilds, st.Process.PCHUses
			out.pch.rejects, out.pch.runs = st.Process.PCHRejects, st.Process.Runs
			out.pch.rechecks = st.Process.PCHRechecks
		}
	}
	return out
}

func TestPCHOracle(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the typed probes are not run on Windows hosts (ADR-108)")
	}
	c := setup(t)
	t.Setenv("GOCPP_INTEROP", "typed") // typedModeSkip answers only in typed mode
	progs := corpusPrograms(t)
	checked := 0
	for _, p := range progs {
		if p.err == nil && c.typedCheckSkip(p) == "" {
			checked++
		}
	}
	t.Logf("%d of %d corpus programs are compared", checked, len(progs))
	t.Cleanup(func() {
		t.Logf("runs made again without the header: %d", recheckTotal.Load())
	})
	// (TestTypedCheckCorpus wants 200: the programs that a machine without a display or a toolchain
	// part skips are fewer here, the bound only catches a broken loader.)
	if checked < 150 {
		t.Fatalf("only %d programs are compared, the corpora have about 290 (a loader or a skip rule broke)", checked)
	}
	for _, p := range progs {
		t.Run(strings.ReplaceAll(p.path, "/", "_"), func(t *testing.T) {
			t.Parallel()
			if p.err != nil {
				t.Fatal(p.err)
			}
			if why := c.typedCheckSkip(p); why != "" {
				t.Skip(why)
			}
			old := c.typedProbe(t, p, "off")
			now := c.typedProbe(t, p, "wait")
			if old.exit != now.exit || len(old.docs) != len(now.docs) {
				t.Fatalf("exit %d with %d documents without the header, %d with %d:\n%s\n%s",
					old.exit, len(old.docs), now.exit, len(now.docs), old.stderr, now.stderr)
			}
			if old.pch.builds != 0 || old.pch.uses != 0 {
				t.Errorf("with GOCPP_PROBE_PCH=off: %d headers made, %d runs loaded one", old.pch.builds, old.pch.uses)
			}
			if now.pch.rejects != 0 {
				t.Errorf("%d runs were refused for the header", now.pch.rejects)
			}
			// The type runs do not load the header (the JSON dump hides the members that clang has not read);
			// the probe runs and the function runs do, so a program with a probe loads it at least once.
			if now.pch.runs > 0 && (now.pch.uses == 0 || now.pch.uses > now.pch.runs) {
				t.Errorf("%d compiler runs, %d of them loaded the header", now.pch.runs, now.pch.uses)
			}
			// Rechecks of a failed PCH run are counted separately; the logical runs stay the same.
			if now.pch.runs != old.pch.runs {
				t.Errorf("%d compiler runs without the header, %d with it", old.pch.runs, now.pch.runs)
			}
			recheckTotal.Add(int64(now.pch.rechecks))
			for i := range old.docs {
				a, _ := json.Marshal(old.docs[i])
				b, _ := json.Marshal(now.docs[i])
				if !bytes.Equal(a, b) {
					t.Errorf("document %d differs with the header:\n%s", i, firstDifference(a, b))
				}
			}
		})
	}
}

// firstDifference describes where two JSON documents first differ, with some context.
func firstDifference(a, b []byte) string {
	n := min(len(a), len(b))
	i := 0
	for i < n && a[i] == b[i] {
		i++
	}
	cut := func(x []byte) string {
		lo, hi := max(0, i-120), min(len(x), i+200)
		return string(x[lo:hi])
	}
	return fmt.Sprintf("at byte %d:\nwithout: ...%s\nwith:    ...%s", i, cut(a), cut(b))
}
