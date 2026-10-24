//go:build unix

package probe

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestCacheReadOnlyFolder(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root writes anywhere")
	}
	f := newFake(t)
	f.header(t, "a.hpp", "//\n")
	ro := filepath.Join(t.TempDir(), "ro")
	os.Mkdir(ro, 0o500)
	f.cache = filepath.Join(ro, "typed-v1")
	names := fnNames(3)
	set := fnSet(names...)
	rn := new(Runner)
	writeDown.Delete(f.cache)
	ResetCacheCounters()
	tab := probeWith(t, f, rn, set, names...)
	// The first write found the folder unwritable; the other entries (three units and the context)
	// were not tried.
	if c := CacheCounters(); c.StoreErrors != 1 || c.Stores != 0 {
		t.Errorf("counters %+v", c)
	}
	writeDown.Delete(f.cache)
	if !tab.Results[0].OK || f.runs() != 1 {
		t.Fatalf("%d runs, %+v", f.runs(), tab.Results[0])
	}
	// The Runner keeps its answers in memory.
	probeWith(t, f, rn, set, names...)
	if f.runs() != 1 {
		t.Errorf("%d runs: no memory", f.runs())
	}
	// A new Runner runs again (nothing could be written), without failing.
	probeWith(t, f, new(Runner), set, names...)
	if f.runs() != 2 {
		t.Errorf("%d runs", f.runs())
	}
	if _, err := os.Stat(f.cache); err == nil {
		t.Error("something was written")
	}
}

func TestCancelKillsTheBatches(t *testing.T) {
	f := newFake(t)
	f.header(t, "a.hpp", "//\n")
	pidfile := filepath.Join(t.TempDir(), "pids")
	t.Setenv("FAKE_PIDS", pidfile)
	f.cxx = fakeCXX(t, `case " $* " in *" -E "*) exit 0;; esac  # the toolchain's question
sleep 300 &
echo $! >> "$FAKE_PIDS"
sleep 300`)
	t.Setenv("GOCPP_PROBE_BATCH", "2")
	t.Setenv(EnvJobs, "3")
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	names := fnNames(6)
	go func() {
		_, err := new(Runner).Probe(ctx, f.cfg(), fnSet(names...), gc(names...))
		done <- err
	}()
	var pids []int
	deadline := time.Now().Add(time.Minute) // (only ends a hang)
	for time.Now().Before(deadline) && len(pids) < 3 {
		b, _ := os.ReadFile(pidfile)
		pids = nil
		for _, s := range strings.Fields(string(b)) {
			if p, err := strconv.Atoi(s); err == nil {
				pids = append(pids, p)
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Cleanup(func() {
		for _, p := range pids {
			syscall.Kill(p, syscall.SIGKILL)
		}
	})
	if len(pids) != 3 {
		t.Fatalf("%d compilers started", len(pids))
	}
	cancel()
	select {
	case err := <-done:
		if err == nil || !strings.Contains(err.Error(), "stopped") {
			t.Fatalf("error %v", err)
		}
	case <-time.After(time.Minute):
		t.Fatal("Probe didn't return after the cancel")
	}
	for _, p := range pids {
		if !gone(p) {
			t.Errorf("the child %d of a batch's compiler is still running", p)
		}
	}
	if now, _ := gate.running(); now != 0 {
		t.Errorf("%d slots still taken", now)
	}
	if entries, _ := os.ReadDir(filepath.Join(f.cache, "locks")); len(entries) != 0 {
		t.Errorf("locks left: %v", entries)
	}
}

// A compiler that exits and leaves a child behind (a driver that started a helper): the group is
// killed when the run is over, so that nothing writes into the temporary folder that is removed.
func TestCompilerChildrenDieWithTheRun(t *testing.T) {
	pids := filepath.Join(t.TempDir(), "pids")
	t.Setenv("FAKE_PIDS", pids)
	cxx := fakeCXX(t, `sleep 300 >/dev/null 2>&1 &
echo $! >> "$FAKE_PIDS"
case " $* " in *" -E "*) exit 0;; esac
printf '%s\n' '`+goodDecl+`'`)
	dead := func(what string) {
		t.Helper()
		b, _ := os.ReadFile(pids)
		fields := strings.Fields(string(b))
		if len(fields) == 0 {
			t.Fatalf("%s: no child was started", what)
		}
		p, _ := strconv.Atoi(fields[len(fields)-1])
		t.Cleanup(func() { syscall.Kill(p, syscall.SIGKILL) })
		if !gone(p) {
			t.Errorf("%s: the child %d of the compiler is still running", what, p)
		}
	}
	cfg := Config{CXX: cxx, Dir: t.TempDir(), CacheDir: "off"}
	if _, err := new(Runner).Probe(context.Background(), cfg, fnSet("f0"), gc("f0")); err != nil {
		t.Fatal(err)
	}
	dead("a probe run")
	if _, err := runAux(context.Background(), cfg, []string{"-E", "-o", os.DevNull}, "x\n"); err != nil {
		t.Fatal(err)
	}
	dead("an auxiliary run")
}
