package probe

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeLocateDump is the fake compiler's answer to a probe file of the declaration locations
// (cache_fake_test.go): for each "#pragma clang __debug dump NAME" it prints the dump of a
// function "int NAME(int a);" that a header of FAKE_INC declares, in the format of clang 21,
// and an empty lookup for a name that no header declares.
func fakeLocateDump(text string, deps []string) int {
	pragma := regexp.MustCompile(`(?m)^#pragma clang __debug dump (\w+)$`)
	for _, m := range pragma.FindAllStringSubmatch(text, -1) {
		name := m[1]
		fmt.Fprintf(os.Stderr, "lookup results for %s:\n", name)
		if name == "gocpp_loc_end" {
			fmt.Fprintln(os.Stderr, "VarDecl 0x3 <<invalid sloc>> <invalid sloc> gocpp_loc_end 'char'")
		}
		for _, dep := range deps {
			for n, line := range strings.Split(string(mustRead(dep)), "\n") {
				prefix := "int " + name + "(int a);"
				if !strings.HasPrefix(line, prefix) {
					continue
				}
				at := len("int ") + 1
				param := len("int "+name+"(") + 1
				fmt.Fprintf(os.Stderr, "FunctionDecl 0x1 <%s:%d:1, col:%d> col:%d %s 'int (int)'\n",
					dep, n+1, len(prefix)-1, at, name)
				fmt.Fprintf(os.Stderr, "`-ParmVarDecl 0x2 <col:%d, col:%d> col:%d a 'int'\n",
					param, param+4, param+4)
			}
		}
	}
	return 0
}

func locateRequestFor(names ...string) LocateRequest {
	var request LocateRequest
	for _, name := range names {
		request.Names = append(request.Names, LocateName{Name: name})
	}
	return request
}

func locateFake(t *testing.T, fake *fake, runner *Runner, request LocateRequest) *Locations {
	t.Helper()
	got, err := runner.Locate(context.Background(), fake.cfg(), []string{`"api.hpp"`}, request)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func TestLocateFakeAnswers(t *testing.T) {
	defer SetRacyWindow(0)()
	fake := newFake(t)
	fake.header(t, "api.hpp", "int first(int a);\nint second(int a);\n")
	got := locateFake(t, fake, new(Runner), locateRequestFor("second", "missing"))
	d := onlyDecl(t, got.Named("", "second"))
	if d.File != filepath.Join(fake.inc, "api.hpp") || d.Line != 1 || d.Col != 4 || d.Length != 6 ||
		d.Signature != "int second(int a)" {
		t.Errorf("second = %+v", d)
	}
	if found := got.Named("", "missing"); len(found) != 0 {
		t.Errorf("missing = %+v", found)
	}
}

func TestLocateDiskCacheServesAnotherRunner(t *testing.T) {
	defer SetRacyWindow(0)()
	fake := newFake(t)
	fake.header(t, "api.hpp", "int first(int a);\n")
	locateFake(t, fake, new(Runner), locateRequestFor("first"))
	if got := fake.runs(); got != 1 {
		t.Fatalf("dump runs = %d, want 1", got)
	}
	got := locateFake(t, fake, new(Runner), locateRequestFor("first"))
	if runs := fake.runs(); runs != 1 {
		t.Errorf("dump runs after a warm start = %d, want no compiler", runs)
	}
	if d := onlyDecl(t, got.Named("", "first")); d.Line != 0 {
		t.Errorf("first = %+v", d)
	}
}

func TestLocateHeaderEditInvalidates(t *testing.T) {
	defer SetRacyWindow(0)()
	fake := newFake(t)
	fake.header(t, "api.hpp", "int first(int a);\n")
	runner := new(Runner)
	locateFake(t, fake, runner, locateRequestFor("first"))
	fake.header(t, "api.hpp", "// moved\nint first(int a);\n")
	got := locateFake(t, fake, runner, locateRequestFor("first"))
	if d := onlyDecl(t, got.Named("", "first")); d.Line != 1 {
		t.Errorf("first after an edit = %+v, want line 1", d)
	}
	if runs := fake.runs(); runs != 2 {
		t.Errorf("dump runs = %d, want 2", runs)
	}
}

func TestLocateEditedHeaderWithSameSizeIsNotReused(t *testing.T) {
	defer SetRacyWindow(0)()
	fake := newFake(t)
	fake.header(t, "api.hpp", "int first(int a);\nint other(int);\n")
	locateFake(t, fake, new(Runner), locateRequestFor("first"))
	fake.header(t, "api.hpp", "int other(int);\nint first(int a);\n")
	got := locateFake(t, fake, new(Runner), locateRequestFor("first"))
	if d := onlyDecl(t, got.Named("", "first")); d.Line != 1 {
		t.Errorf("first = %+v, want the new line", d)
	}
}

func TestLocateDifferentNamesAreDifferentRuns(t *testing.T) {
	defer SetRacyWindow(0)()
	fake := newFake(t)
	fake.header(t, "api.hpp", "int first(int a);\nint second(int a);\n")
	runner := new(Runner)
	locateFake(t, fake, runner, locateRequestFor("first"))
	locateFake(t, fake, runner, locateRequestFor("second"))
	if runs := fake.runs(); runs != 2 {
		t.Errorf("dump runs = %d, want one for each set of names", runs)
	}
}

func TestLocateConcurrentCallersShareOneRun(t *testing.T) {
	defer SetRacyWindow(0)()
	fake := newFake(t)
	fake.header(t, "api.hpp", "int first(int a);\n")
	runner := new(Runner)
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got, err := runner.Locate(context.Background(), fake.cfg(), []string{`"api.hpp"`},
				locateRequestFor("first"))
			if err != nil || len(got.Named("", "first")) != 1 {
				t.Errorf("got %v, error %v", got, err)
			}
		}()
	}
	wg.Wait()
	if runs := fake.runs(); runs != 1 {
		t.Errorf("dump runs = %d, want 1", runs)
	}
}

func TestLocateFailedCompilerGivesAnError(t *testing.T) {
	defer SetRacyWindow(0)()
	fake := newFake(t)
	got, err := new(Runner).Locate(context.Background(), fake.cfg(), []string{`"FAILNOW.hpp"`},
		locateRequestFor("first"))
	if err == nil || got != nil {
		t.Fatalf("got %v, error %v, want an error and no answers", got, err)
	}
}

func TestLocateCanceledRunIsNotCached(t *testing.T) {
	defer SetRacyWindow(0)()
	fake := newFake(t)
	fake.header(t, "api.hpp", "int first(int a);\n")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := new(Runner).Locate(ctx, fake.cfg(), []string{`"api.hpp"`}, locateRequestFor("first"))
	if err == nil {
		t.Fatal("a canceled request answered")
	}
	got := locateFake(t, fake, new(Runner), locateRequestFor("first"))
	if len(got.Named("", "first")) != 1 {
		t.Error("the next request has no answer")
	}
}

func TestLocateRacyHeaderIsNotKept(t *testing.T) {
	fake := newFake(t)
	fake.header(t, "api.hpp", "int first(int a);\n")
	path := filepath.Join(fake.inc, "api.hpp")
	now := time.Now()
	os.Chtimes(path, now, now)
	defer SetRacyWindow(time.Hour)()
	got, err := new(Runner).Locate(context.Background(), fake.cfg(), []string{`"api.hpp"`},
		locateRequestFor("first"))
	if err == nil {
		t.Fatalf("a header saved just now was accepted: %v", got)
	}
}

func TestLocateEntriesOfOlderKeysAreNeverLookedUp(t *testing.T) {
	defer SetRacyWindow(0)()
	fake := newFake(t)
	fake.header(t, "api.hpp", "int first(int a);\n")
	request := locateRequestFor("first")
	cfg := fake.cfg()
	includes := []string{`"api.hpp"`}
	locateFake(t, fake, new(Runner), request)
	runner := new(Runner)
	dir, write := runner.diskFor(cfg)
	source, _, err := locateSource(includes, request.Names, nil)
	if err != nil {
		t.Fatal(err)
	}
	toolchain := runner.toolchain(context.Background(), cfg, dir, write)
	current := filepath.Join(dir, locateEntryName(locateKeyVersion, cfg, source, toolchain))
	if locateKeyVersion != "locate-v3" {
		t.Fatalf("locateKeyVersion = %q, the rename of the key is pinned as locate-v3", locateKeyVersion)
	}
	body, err := os.ReadFile(current)
	if err != nil {
		t.Fatalf("no entry under the current key: %v", err)
	}
	runs := fake.runs()
	for _, old := range []string{"locate-v1", "locate-v2"} {
		oldPath := filepath.Join(dir, locateEntryName(old, cfg, source, toolchain))
		if oldPath == current {
			t.Fatalf("the key of %s is the current key", old)
		}
		if err := os.MkdirAll(filepath.Dir(oldPath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(oldPath, body, 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Remove(current); err != nil {
			t.Fatal(err)
		}
		locateFake(t, fake, new(Runner), request)
		runs++
		if got := fake.runs(); got != runs {
			t.Fatalf("with an entry of %s only: %d dump runs, want %d (a fresh run)", old, got, runs)
		}
	}
}
