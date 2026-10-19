package probe

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/huuhait/gocpp/internal/interop"
	"github.com/huuhait/gocpp/internal/typedinterop/collect"
)

// helpers: a macro LIBV defined by a header; the answer is its value, the header errors and the runs.
func lookupWrite(t *testing.T, p, s string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-time.Hour)
	os.Chtimes(p, old, old)
}

func lookupAsk(t *testing.T, cfg Config, include string) (val string, herrs int, runs int) {
	t.Helper()
	r := ref(pth("LIBV"))
	r.WantValue = true
	set := collect.NewSet(r)
	rn := new(Runner)
	tab, err := rn.Probe(context.Background(), cfg, set, GenConfig{Includes: []string{include}, Kinds: map[string]interop.Kind{"LIBV": interop.Macro}})
	if err != nil {
		t.Fatal(err)
	}
	val = "?"
	if v := byKey(tab, "::LIBV"); v != nil && v.Value != nil {
		val = v.Value.String()
	} else if v != nil {
		val = "err:" + v.Error
	}
	return val, len(tab.HeaderErrors), rn.Runs()
}

func lookupWant(t *testing.T, what string, cfg Config, include, wantVal string, wantHErrs int) {
	t.Helper()
	v, h, runs := lookupAsk(t, cfg, include)
	cfg2 := cfg
	cfg2.CacheDir = "off"
	v2, h2, _ := lookupAsk(t, cfg2, include)
	st := "ok"
	if v != v2 || h != h2 {
		st = "STALE"
	}
	t.Logf("%-52s cached=%s/%d (runs %d) truth=%s/%d  %s", what, v, h, runs, v2, h2, st)
	if st == "STALE" {
		t.Errorf("%s: STALE HIT: cached %s/%d, truth %s/%d", what, v, h, v2, h2)
	}
}

// Lookups that the compiler does and the cache must watch (review of step 12.7): a folder of -I that
// is made later, a quoted include's own folder, a header that hides another in a header.
func TestRealClangLookups(t *testing.T) {

	t.Run("nested_in_subfolder_I", func(t *testing.T) {
		d := t.TempDir()
		cfg := clangCacheConfig(t, d, "-I"+d+"/inc1", "-I"+d+"/inc2")
		lookupWrite(t, d+"/inc1/lib.hpp", "#include <x.h>\n")
		lookupWant(t, "x.h missing", cfg, `<lib.hpp>`, "?", 1)
		lookupWrite(t, d+"/inc2/x.h", "#define LIBV 6\n")
		lookupWant(t, "x.h appears in later -I", cfg, `<lib.hpp>`, "6", 0)
		lookupWrite(t, d+"/inc1/x.h", "#define LIBV 7\n")
		lookupWant(t, "x.h appears in earlier -I (next to includer)", cfg, `<lib.hpp>`, "7", 0)
	})
	t.Run("quote_shadow_in_includer_dir", func(t *testing.T) {
		d := t.TempDir()
		cfg := clangCacheConfig(t, d, "-I"+d+"/inc")
		lookupWrite(t, d+"/sub/lib.hpp", "#include \"dep.h\"\n")
		lookupWrite(t, d+"/inc/dep.h", "#define LIBV 1\n")
		cfg.Flags = append(cfg.Flags, "-I"+d+"/sub")
		lookupWant(t, "dep.h via -I inc", cfg, `<lib.hpp>`, "1", 0)
		lookupWant(t, "again", cfg, `<lib.hpp>`, "1", 0)
		lookupWrite(t, d+"/sub/dep.h", "#define LIBV 2\n")
		lookupWant(t, "dep.h created next to includer (quote lookup)", cfg, `<lib.hpp>`, "2", 0)
	})
	t.Run("I_folder_created_later", func(t *testing.T) {
		d := t.TempDir()
		cfg := clangCacheConfig(t, d, "-I"+d+"/later", "-I"+d+"/inc")
		lookupWrite(t, d+"/inc/lib.hpp", "#include <dep.h>\n")
		lookupWrite(t, d+"/inc/dep.h", "#define LIBV 1\n")
		lookupWant(t, "-Ilater does not exist", cfg, `<lib.hpp>`, "1", 0)
		lookupWant(t, "again", cfg, `<lib.hpp>`, "1", 0)
		lookupWrite(t, d+"/later/dep.h", "#define LIBV 4\n")
		lookupWant(t, "later/dep.h created (nested include)", cfg, `<lib.hpp>`, "4", 0)
	})
	t.Run("ABA_variants", func(t *testing.T) {
		d := t.TempDir()
		cfg := clangCacheConfig(t, d)
		lookupWrite(t, d+"/lib.hpp", "#define LIBV 1\n")
		lookupWant(t, "A", cfg, `"lib.hpp"`, "1", 0)
		lookupWrite(t, d+"/lib.hpp", "#define LIBV 2\n")
		lookupWant(t, "B", cfg, `"lib.hpp"`, "2", 0)
		lookupWrite(t, d+"/lib.hpp", "#define LIBV 1\n")
		_, _, runs := lookupAsk(t, cfg, `"lib.hpp"`)
		lookupWant(t, "A again", cfg, `"lib.hpp"`, "1", 0)
		t.Logf("A again runs=%d (0 expected)", runs)
		if runs != 0 {
			t.Errorf("A->B->A did not hit: %d runs", runs)
		}
	})
	t.Run("cpath_shadow_created", func(t *testing.T) {
		d := t.TempDir()
		lookupWrite(t, d+"/p1/lib.hpp", "#include <dep.h>\n")
		lookupWrite(t, d+"/p1/dep.h", "#define LIBV 1\n")
		lookupWrite(t, d+"/p2/x", "")
		cfg := clangCacheConfig(t, d)
		t.Setenv("CPATH", d+"/p2:"+d+"/p1")
		lookupWant(t, "CPATH=p2:p1", cfg, `<lib.hpp>`, "1", 0)
		lookupWant(t, "again", cfg, `<lib.hpp>`, "1", 0)
		lookupWrite(t, d+"/p2/dep.h", "#define LIBV 5\n")
		lookupWant(t, "p2/dep.h created", cfg, `<lib.hpp>`, "5", 0)
	})
}

func TestRealClangLookups2(t *testing.T) {
	t.Run("dirs_exist_at_first", func(t *testing.T) {
		d := t.TempDir()
		os.MkdirAll(d+"/inc1", 0o755)
		os.MkdirAll(d+"/inc2", 0o755)
		cfg := clangCacheConfig(t, d, "-I"+d+"/inc1", "-I"+d+"/inc2")
		lookupWrite(t, d+"/inc2/lib.hpp", "#include <x.h>\n")
		lookupWrite(t, d+"/inc2/x.h", "#define LIBV 6\n")
		lookupWant(t, "x.h in inc2", cfg, `<lib.hpp>`, "6", 0)
		lookupWant(t, "again", cfg, `<lib.hpp>`, "6", 0)
		lookupWrite(t, d+"/inc1/x.h", "#define LIBV 7\n")
		lookupWant(t, "x.h appears in earlier existing -I", cfg, `<lib.hpp>`, "7", 0)
	})
	t.Run("later_dir_exists_but_earlier_nonexistent", func(t *testing.T) {
		d := t.TempDir()
		os.MkdirAll(d+"/inc2", 0o755)
		cfg := clangCacheConfig(t, d, "-I"+d+"/inc1", "-I"+d+"/inc2")
		lookupWrite(t, d+"/inc2/lib.hpp", "#include <x.h>\n")
		lookupWrite(t, d+"/inc2/x.h", "#define LIBV 6\n")
		lookupWant(t, "inc1 does not exist", cfg, `<lib.hpp>`, "6", 0)
		lookupWant(t, "again", cfg, `<lib.hpp>`, "6", 0)
		lookupWrite(t, d+"/inc1/x.h", "#define LIBV 7\n")
		lookupWant(t, "inc1 created with x.h", cfg, `<lib.hpp>`, "7", 0)
	})
	t.Run("quote_includer_dir_system_like", func(t *testing.T) {
		d := t.TempDir()
		cfg := clangCacheConfig(t, d, "-I"+d+"/inc")
		lookupWrite(t, d+"/inc/lib.hpp", "#include \"sub/dep.h\"\n")
		lookupWrite(t, d+"/inc/sub/dep.h", "#include \"leaf.h\"\n")
		lookupWrite(t, d+"/inc/leaf.h", "#define LIBV 1\n")
		lookupWant(t, "leaf.h found via -I inc", cfg, `<lib.hpp>`, "1", 0)
		lookupWant(t, "again", cfg, `<lib.hpp>`, "1", 0)
		lookupWrite(t, d+"/inc/sub/leaf.h", "#define LIBV 2\n")
		lookupWant(t, "sub/leaf.h created next to includer", cfg, `<lib.hpp>`, "2", 0)
	})
}

// The release of a lock removes the file even when the removal fails for a moment (on Windows a waiter that
// reads the file keeps it from being deleted): it is tried again, and a lock that cannot be removed is counted.
func TestRemoveLockRetries(t *testing.T) {
	defer func(f func(string) error) { removeFile = f }(removeFile)
	busy := errors.New("The process cannot access the file because it is being used by another process.")
	var calls int
	fail := func(n int, err error) {
		calls = 0
		removeFile = func(string) error {
			calls++
			if calls <= n {
				return err
			}
			return nil
		}
	}
	ResetCacheCounters()
	fail(3, busy)
	removeLock("x.lock")
	if calls != 4 || CacheCounters().LockLeaks != 0 {
		t.Errorf("a removal that fails 3 times: %d calls, %d leaks", calls, CacheCounters().LockLeaks)
	}
	fail(1000, fs.ErrNotExist)
	removeLock("x.lock")
	if calls != 1 || CacheCounters().LockLeaks != 0 {
		t.Errorf("a file that is not there: %d calls, %d leaks", calls, CacheCounters().LockLeaks)
	}
	fail(1<<30, busy)
	removeLock("x.lock")
	if calls != lockRemoveTries || CacheCounters().LockLeaks != 1 {
		t.Errorf("a removal that never works: %d calls, %d leaks", calls, CacheCounters().LockLeaks)
	}
}

// A lock is released while another reader has the file open (what a waiter does to see who holds it): on
// Windows the removal fails at first. The file is gone afterwards and nothing leaked.
func TestLockReleaseWhileRead(t *testing.T) {
	dir := t.TempDir()
	ResetCacheCounters()
	release, _ := acquireLock(context.Background(), dir, "key", time.Second)
	if release == nil {
		t.Fatal("no lock")
	}
	path := lockFile(dir, "key")
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		time.Sleep(50 * time.Millisecond)
		f.Close()
	}()
	// (Where a file that is open can be deleted, the failure is made.)
	defer func(f func(string) error) { removeFile = f }(removeFile)
	failures := 2
	removeFile = func(p string) error {
		if failures > 0 {
			failures--
			return errors.New("being used by another process")
		}
		return os.Remove(p)
	}
	release()
	if _, err := os.Stat(path); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("the lock file is still there: %v", err)
	}
	if CacheCounters().LockLeaks != 0 || failures != 0 {
		t.Errorf("%d leaks, %d failures not used", CacheCounters().LockLeaks, failures)
	}
}
