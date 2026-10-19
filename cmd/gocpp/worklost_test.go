package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
	"time"
)

// The tests of worklost.go: something else removes the work folder of a build while the compilers
// run in it. The hook of the link removes it, as a cleaner of the temporary folder would, at the
// place a test names: the build must go on in a new folder and print what it did, or say what
// happened when it can't (never "internal error: generated C++ did not compile": the program
// is fine).

// ownTmp gives the test a TMPDIR of its own (the work folders of its builds are in it), which the
// test may remove: the shared TMPDIR of the package holds the cache folder.
func ownTmp(t *testing.T) string {
	t.Helper()
	tmp := filepath.Join(t.TempDir(), "tmp")
	if err := os.Mkdir(tmp, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TMPDIR", tmp)
	t.Setenv("GOCPP_WORKLOST", "") // the default: the build makes its folder again (the tests' TestMain says fail)
	return tmp
}

// wipeGlob removes what `rm -rf dir/*` removes: the names that do not start with a dot stay out of it,
// the dotfiles stay.
func wipeGlob(dir string) {
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if !strings.HasPrefix(e.Name(), ".") {
			os.RemoveAll(filepath.Join(dir, e.Name()))
		}
	}
}

// removeProgram removes the program that the link wrote (in the work folder, named like the
// current folder) and nothing else: the markers stay.
func removeProgram(work string) {
	wd, _ := os.Getwd()
	os.Remove(filepath.Join(work, filepath.Base(wd)+exeSuffix))
}

// wipeContents removes everything in dir and leaves dir.
func wipeContents(dir string) {
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		os.RemoveAll(filepath.Join(dir, e.Name()))
	}
}

func TestWorkFolderRemovedDuringBuild(t *testing.T) {
	const hello = "hello, gocpp 12 true\nafter\n"
	for _, tc := range []struct {
		name  string
		phase string
		wipe  func(work string)
	}{
		{"folder gone before the compiles", "before", func(work string) { os.RemoveAll(work) }},
		{"temporary folder gone before the compiles", "before", func(work string) { os.RemoveAll(filepath.Dir(work)) }},
		{"contents gone before the compiles", "before", func(work string) { wipeContents(work) }},
		{"folder gone after the link", "after", func(work string) { os.RemoveAll(work) }},
		{"contents gone after the link", "after", func(work string) { wipeContents(work) }},
		{"contents but not the dotfiles gone before the compiles", "before", wipeGlob},
		{"contents but not the dotfiles gone after the link", "after", wipeGlob},
		{"only the program gone after the link", "after", removeProgram},
	} {
		t.Run(tc.name, func(t *testing.T) {
			forEachCXX(t, func(t *testing.T, c testCXX) {
				ownTmp(t)
				dir := inDir(t, map[string]string{"hello.gp": helloSrc})
				var attempts []int
				testHookLink = func(phase string, attempt int, work string) {
					if phase != tc.phase {
						return
					}
					attempts = append(attempts, attempt)
					if attempt == 0 {
						tc.wipe(work)
					}
				}
				t.Cleanup(func() { testHookLink = nil })
				prog := filepath.Join(dir, "prog")
				r := gocppRun("", false, "build", "-o", prog, ".")
				if r.code != 0 {
					t.Fatalf("gocpp build = %+v, want a build that goes on in a new work folder", r)
				}
				if !strings.Contains(r.stderr, "was removed while the build ran") || !strings.Contains(r.stderr, "building again") || strings.Contains(r.stderr, "internal error") {
					t.Errorf("stderr = %q, want the note that the work folder was removed and the build repeated", r.stderr)
				}
				if len(attempts) != 2 {
					t.Errorf("the link ran %v, want the first attempt and one more", attempts)
				}
				out, err := exec.Command(prog).Output()
				if err != nil || string(out) != hello {
					t.Errorf("the program prints %q (%v), want %q", out, err, hello)
				}
				// gocpp run builds in the same way.
				attempts = nil
				if r := gocppRun("", false, "run", "hello.gp"); r.code != 0 || r.stdout != hello {
					t.Errorf("gocpp run = %+v", r)
				}
			})
		})
	}
}

// A build whose work folder is removed every time says so and stops: three more tries, then the
// message that names the folder.
func TestWorkFolderRemovedEveryTime(t *testing.T) {
	forEachCXX(t, func(t *testing.T, c testCXX) {
		ownTmp(t)
		inDir(t, map[string]string{"hello.gp": helloSrc})
		links := 0
		testHookLink = func(phase string, _ int, work string) {
			if phase == "before" {
				links++
				os.RemoveAll(work)
			}
		}
		t.Cleanup(func() { testHookLink = nil })
		r := gocppRun("", false, "build", "-o", "prog", ".")
		if r.code != exitErrors || links != maxWorkAttempts+1 {
			t.Fatalf("gocpp build = %+v after %d links, want status %d after %d", r, links, exitErrors, maxWorkAttempts+1)
		}
		want := fmt.Sprintf("was removed %d times while the build ran: another process or a cleanup of the temporary folder", maxWorkAttempts+1)
		if !strings.Contains(r.stderr, want) || strings.Contains(r.stderr, "bug in gocpp") {
			t.Errorf("stderr = %q, want it to say %q", r.stderr, want)
		}
		if _, err := os.Stat("prog"); err == nil {
			t.Error("a program was written")
		}
	})
}

// A work folder that is there is never taken for a lost one: a program that does not compile is
// reported once, as an internal error, with its work folder kept for the user.
func TestWorkFolderIntactErrorIsNotRetried(t *testing.T) {
	forEachCXX(t, func(t *testing.T, c testCXX) {
		ownTmp(t)
		inDir(t, map[string]string{"hello.gp": helloSrc})
		testHookCPP = func(_ string, cpp []byte) []byte { return append(cpp, "\nthis is not C++\n"...) }
		links := 0
		testHookLink = func(phase string, _ int, _ string) {
			if phase == "before" {
				links++
			}
		}
		t.Cleanup(func() { testHookCPP, testHookLink = nil, nil })
		r := gocppRun("", false, "build", "-o", "prog", ".")
		if r.code != exitInternal || links != 1 || strings.Contains(r.stderr, "building again") || !strings.Contains(r.stderr, "did not compile") {
			t.Errorf("gocpp build = %+v after %d links, want one internal error", r, links)
		}
		if i := strings.Index(r.stderr, "The work folder is kept: "); i >= 0 {
			rest := r.stderr[i+len("The work folder is kept: "):]
			os.RemoveAll(rest[:strings.IndexAny(rest, "\n ")])
		}
	})
}

// TMPDIR that is not there (the folder of a shell that ended) is made, as the compilers need it too.
func TestWorkFolderMissingTmpdir(t *testing.T) {
	forEachCXX(t, func(t *testing.T, c testCXX) {
		tmp := filepath.Join(t.TempDir(), "gone", "tmp")
		t.Setenv("TMPDIR", tmp)
		inDir(t, map[string]string{"hello.gp": helloSrc})
		r := gocppRun("", false, "run", "hello.gp")
		if r.code != 0 || r.stdout != "hello, gocpp 12 true\nafter\n" || strings.Contains(r.stderr, "building again") {
			t.Errorf("gocpp run = %+v with TMPDIR=%s that does not exist", r, tmp)
		}
		if entries, _ := os.ReadDir(tmp); len(entries) != 0 {
			t.Errorf("%d entries left in %s", len(entries), tmp)
		}
	})
}

// The folder of the generated files of a program is not removed by the trimming of another build
// while a build holds its lock, however old its time is (the time is the time of the last use, set
// only after the files are written).
func TestTrimGenKeepsFolderOfARunningBuild(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()
	mk := func(name string, age time.Duration) string {
		p := filepath.Join(dir, name)
		if err := os.Mkdir(p, 0o755); err != nil {
			t.Fatal(err)
		}
		os.Chtimes(p, now.Add(-age), now.Add(-age))
		return p
	}
	oldest := mk("oldest", 400*24*time.Hour)
	for i := 0; i < maxGenFolders; i++ {
		mk(fmt.Sprintf("old%03d", i), 10*24*time.Hour+time.Duration(i)*time.Minute)
	}
	l, busy, err := lockFile(oldest + ".lock")
	if err != nil || busy {
		t.Skipf("no file locks here: %v", err)
	}
	folders := func() (n int) {
		entries, _ := os.ReadDir(dir)
		for _, e := range entries {
			if e.IsDir() {
				n++
			}
		}
		return n
	}
	trimGen(dir, filepath.Join(dir, "keep"), now)
	if _, err := os.Stat(oldest); err != nil {
		t.Errorf("the folder of a build that holds its lock was removed: %v", err)
	}
	if got := folders(); got != maxGenFolders {
		t.Errorf("%d folders left, want %d: the next oldest goes instead", got, maxGenFolders)
	}
	l.unlock()
	mk("another", 300*24*time.Hour)
	trimGen(dir, filepath.Join(dir, "keep"), now)
	if _, err := os.Stat(oldest); err == nil {
		t.Error("the folder of no build was not removed")
	}
	if got := folders(); got != maxGenFolders {
		t.Errorf("%d folders left, want %d", got, maxGenFolders)
	}
}

// GOCPP_WORKLOST=fail: the repository's own tests and campaigns get an error with its own status, not a
// build that went again.
func TestWorkFolderLostFails(t *testing.T) {
	forEachCXX(t, func(t *testing.T, c testCXX) {
		ownTmp(t)
		t.Setenv("GOCPP_WORKLOST", "fail")
		inDir(t, map[string]string{"hello.gp": helloSrc})
		links := 0
		testHookLink = func(phase string, _ int, work string) {
			if phase == "before" {
				links++
				os.RemoveAll(work)
			}
		}
		t.Cleanup(func() { testHookLink = nil })
		r := gocppRun("", false, "build", "-o", "prog", ".")
		if r.code != exitWorkLost || links != 1 || !strings.Contains(r.stderr, "GOCPP_WORKLOST=fail: not building again") || strings.Contains(r.stderr, "internal error") {
			t.Errorf("gocpp build = %+v after %d links, want status %d after 1 link and the message", r, links, exitWorkLost)
		}
		if _, err := os.Stat("prog"); err == nil {
			t.Error("a program was written")
		}
	})
}

// A build that is made again has a clean stamp and clean counts: the build after it is up to date, and -x
// counts the compiles of the build that worked.
func TestWorkFolderRetryThenUpToDate(t *testing.T) {
	forEachCXX(t, func(t *testing.T, c testCXX) {
		ownTmp(t)
		t.Setenv("GOCPP_NOOBJCACHE", "0") // the up-to-date check needs the object cache
		dir := inDir(t, map[string]string{"hello.gp": helloSrc})
		prog := filepath.Join(dir, "prog")
		testHookLink = func(phase string, attempt int, work string) {
			if phase == "after" && attempt == 0 {
				wipeContents(work)
			}
		}
		t.Cleanup(func() { testHookLink = nil })
		r := gocppRun("", false, "build", "-x", "-o", prog, ".")
		if r.code != 0 || !strings.Contains(r.stderr, "building again") {
			t.Fatalf("gocpp build = %+v", r)
		}
		if m := regexp.MustCompile(`# actions: .*link=(\d+)`).FindStringSubmatch(r.stderr); m == nil || m[1] != "1" {
			t.Errorf("the counts of -x are not those of the build that worked (link=1):\n%s", r.stderr)
		}
		testHookLink = nil
		if r := gocppRun("", false, "build", "-x", "-o", prog, "."); r.code != 0 || !strings.Contains(r.stderr, "# up to date") {
			t.Errorf("the build after a build that was made again is not up to date: %+v", r)
		}
		if out, err := exec.Command(prog).Output(); err != nil || string(out) != "hello, gocpp 12 true\nafter\n" {
			t.Errorf("the program prints %q (%v)", out, err)
		}
	})
}

// With -work the generated files are in the work folder (gen/): when it is removed they are written
// again with the new folder.
func TestWorkFolderRemovedWithWork(t *testing.T) {
	forEachCXX(t, func(t *testing.T, c testCXX) {
		ownTmp(t)
		dir := inDir(t, map[string]string{"hello.gp": helloSrc})
		testHookLink = func(phase string, attempt int, work string) {
			if phase == "before" && attempt == 0 {
				os.RemoveAll(work)
			}
		}
		t.Cleanup(func() { testHookLink = nil })
		prog := filepath.Join(dir, "prog")
		r := gocppRun("", false, "build", "-work", "-o", prog, ".")
		work := workDir(t, r.stderr)
		if r.code != 0 || !strings.Contains(r.stderr, "building again") {
			t.Fatalf("gocpp build -work = %+v", r)
		}
		if _, err := os.Stat(filepath.Join(work, "gen", "gpkg", "command-line-arguments", "hello.gp.cpp")); err != nil {
			t.Errorf("the generated files are not in the new work folder: %v", err)
		}
		if out, err := exec.Command(prog).Output(); err != nil || string(out) != "hello, gocpp 12 true\nafter\n" {
			t.Errorf("the program prints %q (%v)", out, err)
		}
	})
}

// The work folder is emptied after the compiler wrote an object and before the object cache stored it:
// the store fails (that build is not recorded for the up-to-date check), the link finds no object, and
// the build is made again. What the failed attempt recorded must not stay: the build after it is up to
// date. (A compiler wrapper does the cleaning, as a cleaner of /tmp would, in the middle of the compile.)
func TestWorkFolderEmptiedDuringCompileThenUpToDate(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the wrapper is a shell script")
	}
	forEachCXX(t, func(t *testing.T, c testCXX) {
		ownTmp(t)
		t.Setenv("GOCPP_NOOBJCACHE", "0")
		dir := inDir(t, map[string]string{"hello.gp": helloSrc})
		mark := filepath.Join(t.TempDir(), "done")
		wrap := filepath.Join(t.TempDir(), "wrap.sh")
		script := "#!/bin/sh\n\"$@\" || exit $?\ncase \"$*\" in *.gp.o*)\n  if [ ! -e " + mark + " ]; then : > " + mark + "; rm -rf ./* ./.[!.]*; fi;;\nesac\n"
		if err := os.WriteFile(wrap, []byte(script), 0o755); err != nil {
			t.Fatal(err)
		}
		t.Setenv("GOCPP_CXX", "/bin/sh "+wrap+" "+c.name)
		prog := filepath.Join(dir, "prog")
		r := gocppRun("", false, "build", "-x", "-o", prog, ".")
		if r.code != 0 || !strings.Contains(r.stderr, "building again") {
			t.Fatalf("gocpp build = %+v, want a build that was made again", r)
		}
		if r := gocppRun("", false, "build", "-x", "-o", prog, "."); r.code != 0 || !strings.Contains(r.stderr, "# up to date") {
			t.Errorf("the build after a build that was made again is not up to date: %+v", r)
		}
	})
}

// removeGenFolder reads the time of the folder again under the lock (a build that finished since the
// look of trimGen has used the folder just now), and takes the lock even when the lock file is not
// there yet (a build that is just starting has made the folder and not the file).
func TestRemoveGenFolder(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()
	mk := func(name string, age time.Duration) string {
		p := filepath.Join(dir, name)
		if err := os.Mkdir(p, 0o755); err != nil {
			t.Fatal(err)
		}
		os.Chtimes(p, now.Add(-age), now.Add(-age))
		return p
	}
	used, old := mk("used", time.Minute), mk("old", 2*maxGenAge)
	if removeGenFolder(used, now) {
		t.Error("a folder used a minute ago was removed")
	}
	if _, err := os.Stat(used); err != nil {
		t.Error("the folder used a minute ago is gone")
	}
	// The lock is taken while the folder goes: a build that comes then waits for it.
	var waited bool
	testHookGenRemove = func(gen string) {
		_, busy, err := lockFile(gen + ".lock")
		waited = err == nil && busy
	}
	t.Cleanup(func() { testHookGenRemove = nil })
	if !removeGenFolder(old, now) {
		t.Error("an old folder was not removed")
	}
	if _, err := os.Stat(old); err == nil {
		t.Error("the old folder is still there")
	}
	if !waited {
		t.Error("the lock of the folder was not held while it was removed (or there are no file locks here)")
	}
}
