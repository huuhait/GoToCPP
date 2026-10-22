package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/huuhait/gocpp/internal/build"
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

// The program is removed after the link and after the check that follows it, in the time until gocpp
// build moves it (gocpp run starts it): a removal that the check after the link cannot see. The move and
// the start find the program gone and make the build again; before, the build said
// "gocpp build: stat .../program: no such file or directory".
func TestProgramRemovedBeforeMoveOrStart(t *testing.T) {
	const hello = "hello, gocpp 12 true\nafter\n"
	for _, tc := range []struct {
		name string
		wipe func(p *program)
	}{
		{"only the program", func(p *program) { os.Remove(p.exe) }},
		{"the work folder", func(p *program) { os.RemoveAll(p.work) }},
		{"the temporary folder", func(p *program) { os.RemoveAll(filepath.Dir(p.work)) }},
	} {
		for _, command := range []string{"build", "run"} {
			t.Run(tc.name+" "+command, func(t *testing.T) {
				forEachCXX(t, func(t *testing.T, c testCXX) {
					ownTmp(t)
					dir := inDir(t, map[string]string{"hello.gp": helloSrc})
					var attempts []int
					testHookProgram = func(attempt int, p *program) {
						attempts = append(attempts, attempt)
						if attempt == 0 {
							tc.wipe(p)
						}
					}
					t.Cleanup(func() { testHookProgram = nil })
					prog := filepath.Join(dir, "prog")
					args := []string{"build", "-o", prog, "."}
					if command == "run" {
						args = []string{"run", "hello.gp"}
					}
					r := gocppRun("", false, args...)
					if r.code != 0 {
						t.Fatalf("gocpp %s = %+v, want a build that goes on in a new work folder", command, r)
					}
					if !strings.Contains(r.stderr, "was removed while the build ran") || !strings.Contains(r.stderr, "building again") ||
						strings.Contains(r.stderr, "no such file") || strings.Contains(r.stderr, "internal error") {
						t.Errorf("stderr = %q, want the note that the work folder was removed and the build repeated", r.stderr)
					}
					if len(attempts) != 2 || attempts[0] != 0 || attempts[1] != 1 {
						t.Errorf("the program was used at attempts %v, want 0 and 1", attempts)
					}
					if command == "run" {
						if r.stdout != hello {
							t.Errorf("gocpp run printed %q, want %q", r.stdout, hello)
						}
						return
					}
					if out, err := exec.Command(prog).Output(); err != nil || string(out) != hello {
						t.Errorf("the program prints %q (%v), want %q", out, err, hello)
					}
				})
			})
		}
	}
}

// GOCPP_WORKLOST=fail: a program that is gone before the move or the start is the error with its own
// status, and nothing is built again.
func TestProgramRemovedBeforeMoveFails(t *testing.T) {
	forEachCXX(t, func(t *testing.T, c testCXX) {
		ownTmp(t)
		t.Setenv("GOCPP_WORKLOST", "fail")
		inDir(t, map[string]string{"hello.gp": helloSrc})
		calls := 0
		testHookProgram = func(_ int, p *program) {
			calls++
			os.Remove(p.exe)
		}
		t.Cleanup(func() { testHookProgram = nil })
		for _, args := range [][]string{{"build", "-o", "prog", "."}, {"run", "hello.gp"}} {
			calls = 0
			r := gocppRun("", false, args...)
			if r.code != exitWorkLost || calls != 1 || !strings.Contains(r.stderr, "GOCPP_WORKLOST=fail: not building again") {
				t.Errorf("gocpp %s = %+v after %d uses, want status %d after 1 and the message", args[0], r, calls, exitWorkLost)
			}
		}
		if _, err := os.Stat("prog"); err == nil {
			t.Error("a program was written")
		}
	})
}

// A program that is gone every time is built again a few times, then the build says what happened.
func TestProgramRemovedEveryTime(t *testing.T) {
	forEachCXX(t, func(t *testing.T, c testCXX) {
		ownTmp(t)
		inDir(t, map[string]string{"hello.gp": helloSrc})
		calls := 0
		testHookProgram = func(_ int, p *program) {
			calls++
			os.Remove(p.exe)
		}
		t.Cleanup(func() { testHookProgram = nil })
		for _, args := range [][]string{{"build", "-o", "prog", "."}, {"run", "hello.gp"}} {
			calls = 0
			r := gocppRun("", false, args...)
			want := fmt.Sprintf("was removed %d times while the build ran", maxWorkAttempts+1)
			if r.code != exitErrors || calls != maxWorkAttempts+1 || !strings.Contains(r.stderr, want) || strings.Contains(r.stderr, "bug in gocpp") {
				t.Errorf("gocpp %s = %+v after %d uses, want status %d after %d and %q", args[0], r, calls, exitErrors, maxWorkAttempts+1, want)
			}
		}
	})
}

// A failed move of a program that is there (here: -o names a folder that has a file in its way) is the
// error of the move, not a lost work folder.
func TestMoveFailureWithProgramThereIsNotRetried(t *testing.T) {
	forEachCXX(t, func(t *testing.T, c testCXX) {
		ownTmp(t)
		inDir(t, map[string]string{"hello.gp": helloSrc, "blocked": "a file"})
		calls := 0
		testHookProgram = func(int, *program) { calls++ }
		t.Cleanup(func() { testHookProgram = nil })
		r := gocppRun("", false, "build", "-o", filepath.Join("blocked", "prog"), ".")
		if r.code != exitErrors || calls != 1 || strings.Contains(r.stderr, "building again") {
			t.Errorf("gocpp build = %+v after %d uses, want the error of the move after one", r, calls)
		}
	})
}

// A compiler that something outside gocpp ends with a signal (pkill, a time limit, the out-of-memory
// killer) is no bug in gocpp: the build says so, with no "internal error", no kept work folder, and no
// build again. A wrapper ends itself with the signal when it compiles the generated file.
func TestCompilerEndedFromOutside(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the wrapper is a shell script")
	}
	for _, sig := range []string{"KILL", "TERM", "HUP"} {
		t.Run(sig, func(t *testing.T) {
			forEachCXX(t, func(t *testing.T, c testCXX) {
				ownTmp(t)
				t.Setenv("GOCPP_NOOBJCACHE", "1")
				inDir(t, map[string]string{"hello.gp": helloSrc})
				wrap := filepath.Join(t.TempDir(), "wrap.sh")
				script := "#!/bin/sh\ncase \"$*\" in *.gp.o*) kill -" + sig + " $$;; esac\nexec \"$@\"\n"
				if err := os.WriteFile(wrap, []byte(script), 0o755); err != nil {
					t.Fatal(err)
				}
				t.Setenv("GOCPP_CXX", "/bin/sh "+wrap+" "+c.name)
				r := gocppRun("", false, "build", "-o", "prog", ".")
				if r.code != exitErrors || !strings.Contains(r.stderr, "was ended by the signal") || !strings.Contains(r.stderr, "no bug in gocpp") ||
					strings.Contains(r.stderr, "internal error") || strings.Contains(r.stderr, "work folder is kept") {
					t.Errorf("gocpp build = %+v, want status %d and the message that something outside ended the compiler", r, exitErrors)
				}
			})
		})
	}
}

// A compiler that crashes is the compiler's bug: still an internal error, with the work folder kept.
func TestCompilerCrashIsStillInternal(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the wrapper is a shell script")
	}
	forEachCXX(t, func(t *testing.T, c testCXX) {
		ownTmp(t)
		t.Setenv("GOCPP_NOOBJCACHE", "1")
		inDir(t, map[string]string{"hello.gp": helloSrc})
		wrap := filepath.Join(t.TempDir(), "wrap.sh")
		script := "#!/bin/sh\ncase \"$*\" in *.gp.o*) kill -SEGV $$;; esac\nexec \"$@\"\n"
		if err := os.WriteFile(wrap, []byte(script), 0o755); err != nil {
			t.Fatal(err)
		}
		t.Setenv("GOCPP_CXX", "/bin/sh "+wrap+" "+c.name)
		r := gocppRun("", false, "build", "-o", "prog", ".")
		if r.code != exitInternal || strings.Contains(r.stderr, "was ended by the signal") || !strings.Contains(r.stderr, "bug in gocpp") {
			t.Errorf("gocpp build = %+v, want an internal error", r)
		}
		if i := strings.Index(r.stderr, "The work folder is kept: "); i >= 0 {
			rest := r.stderr[i+len("The work folder is kept: "):]
			os.RemoveAll(rest[:strings.IndexAny(rest, "\n ")])
		}
	})
}

// A build that is made again after the program was removed before the move has clean counts and a clean
// record: -x counts the compiles of the build that worked, and the build after it is up to date.
func TestProgramRemovedBeforeMoveThenUpToDate(t *testing.T) {
	forEachCXX(t, func(t *testing.T, c testCXX) {
		ownTmp(t)
		t.Setenv("GOCPP_NOOBJCACHE", "0") // the up-to-date check needs the object cache
		dir := inDir(t, map[string]string{"hello.gp": helloSrc})
		prog := filepath.Join(dir, "prog")
		testHookProgram = func(attempt int, p *program) {
			if attempt == 0 {
				os.Remove(p.exe)
			}
		}
		t.Cleanup(func() { testHookProgram = nil })
		r := gocppRun("", false, "build", "-x", "-o", prog, ".")
		if r.code != 0 || !strings.Contains(r.stderr, "building again") {
			t.Fatalf("gocpp build = %+v", r)
		}
		if m := regexp.MustCompile(`# actions: load=(\d+) .*link=(\d+)`).FindStringSubmatch(r.stderr); m == nil || m[1] != "1" || m[2] != "1" {
			t.Errorf("the counts of -x are not those of the build that worked (load=1, link=1):\n%s", r.stderr)
		}
		testHookProgram = nil
		if r := gocppRun("", false, "build", "-x", "-o", prog, "."); r.code != 0 || !strings.Contains(r.stderr, "# up to date") {
			t.Errorf("the build after a build that was made again is not up to date: %+v", r)
		}
		if out, err := exec.Command(prog).Output(); err != nil || string(out) != "hello, gocpp 12 true\nafter\n" {
			t.Errorf("the program prints %q (%v)", out, err)
		}
	})
}

// writeShellScript writes an executable shell script into dir and returns its path.
func writeShellScript(t *testing.T, dir, name, body string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+body), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

// The usual way a compiler is ended from outside: the driver (g++, clang++) lives and a program it started
// (cc1plus, collect2, the assembler, the linker) is ended by a signal, as the out-of-memory killer and
// `pkill -x cc1plus` do. The driver then exits with status 1 and a text. The programs the driver starts are
// replaced by scripts that end themselves with the signal: g++ runs them through -wrapper, clang++ takes the
// assembler from -B and the linker from -fuse-ld.
func TestCompilerChildEndedFromOutside(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the wrappers are shell scripts")
	}
	for _, sig := range []string{"TERM", "KILL"} {
		for _, stage := range []string{"compile", "link"} {
			t.Run(sig+" "+stage, func(t *testing.T) {
				forEachCXX(t, func(t *testing.T, c testCXX) {
					ownTmp(t)
					t.Setenv("GOCPP_NOOBJCACHE", "1")
					inDir(t, map[string]string{"hello.gp": helloSrc})
					dir := t.TempDir()
					kill := "kill -" + sig + " $$\n"
					cxx := c.name
					if strings.Contains(filepath.Base(c.name), "clang") {
						if stage == "compile" {
							writeShellScript(t, dir, "as", kill)
							cxx += " -fno-integrated-as -B " + dir
						} else {
							cxx += " -fuse-ld=" + writeShellScript(t, dir, "ld", kill)
						}
					} else {
						program := map[string]string{"compile": "cc1plus*.gp.cpp", "link": "collect2"}[stage]
						wrapper := writeShellScript(t, dir, "wrap.sh", "case \"$*\" in *"+program+"*) "+kill+";; esac\nexec \"$@\"\n")
						cxx += " -wrapper /bin/sh," + wrapper
					}
					t.Setenv("GOCPP_CXX", cxx)
					r := gocppRun("", false, "build", "-o", "prog", ".")
					if r.code != exitErrors || !strings.Contains(r.stderr, "was ended by the signal") || !strings.Contains(r.stderr, "no bug in gocpp") ||
						strings.Contains(r.stderr, "internal error") || strings.Contains(r.stderr, "work folder is kept") {
						t.Errorf("gocpp build = %+v, want status %d and the message that something outside ended the compiler", r, exitErrors)
					}
				})
			})
		}
	}
}

// The compiler's own crashes stay internal errors, in the words of both compilers: an internal compiler
// error (g++ says it for a cc1plus that died of SIGSEGV) and a clang frontend that crashed.
func TestCompilerCrashTextIsStillInternal(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the wrapper is a shell script")
	}
	for name, text := range map[string]string{
		"gcc ICE":     "g++: internal compiler error: Segmentation fault signal terminated program cc1plus\nPlease submit a full bug report, with preprocessed source.",
		"gcc abort":   "g++: internal compiler error: Aborted signal terminated program cc1plus",
		"clang crash": "clang++: error: unable to execute command: Segmentation fault (core dumped)\nclang++: error: clang frontend command failed due to signal (use -v to see invocation)\nPLEASE submit a bug report to https://github.com/llvm/llvm-project/issues/",
	} {
		t.Run(name, func(t *testing.T) {
			forEachCXX(t, func(t *testing.T, c testCXX) {
				ownTmp(t)
				t.Setenv("GOCPP_NOOBJCACHE", "1")
				inDir(t, map[string]string{"hello.gp": helloSrc})
				// The real compiler for everything but the generated file, which "crashes" with the text.
				wrap := writeShellScript(t, t.TempDir(), "wrap.sh", "case \"$*\" in *.gp.o*) cat <<'EOF'\n"+text+"\nEOF\nexit 1;; esac\nexec \"$@\"\n")
				t.Setenv("GOCPP_CXX", "/bin/sh "+wrap+" "+c.name)
				r := gocppRun("", false, "build", "-o", "prog", ".")
				if r.code != exitInternal || strings.Contains(r.stderr, "was ended by the signal") || !strings.Contains(r.stderr, "bug in gocpp") {
					t.Errorf("gocpp build = %+v, want an internal error", r)
				}
				if i := strings.Index(r.stderr, "The work folder is kept: "); i >= 0 {
					rest := r.stderr[i+len("The work folder is kept: "):]
					os.RemoveAll(rest[:strings.IndexAny(rest, "\n ")])
				}
			})
		})
	}
}

// stampUnits returns the number of compile records in the newest stamp file of the cache folder.
func stampUnits(t *testing.T, cache string) int {
	t.Helper()
	files, _ := filepath.Glob(filepath.Join(cache, "gocpp", "stamp", "*.json"))
	if len(files) != 1 {
		t.Fatalf("%d stamp files in %s, want 1", len(files), cache)
	}
	data, err := os.ReadFile(files[0])
	if err != nil {
		t.Fatal(err)
	}
	var stamp struct {
		Units []json.RawMessage `json:"units"`
	}
	if err := json.Unmarshal(data, &stamp); err != nil {
		t.Fatal(err)
	}
	return len(stamp.Units)
}

// The record of a build that is made again holds the compiles of the build that worked, once: the program
// is moved after the second compile, and the stamp that is saved has the same units as the stamp of a build
// that was not made again.
func TestProgramRemovedBeforeMoveStampHasNoStaleUnits(t *testing.T) {
	c := anyCXX(t)
	ownTmp(t)
	t.Setenv("GOCPP_CXX", c.name)
	t.Setenv("GOCPP_NOOBJCACHE", "0")
	cache := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cache)
	dir := inDir(t, map[string]string{"hello.gp": helloSrc})
	prog := filepath.Join(dir, "prog")
	if r := gocppRun("", false, "build", "-o", prog, "."); r.code != 0 {
		t.Fatalf("gocpp build = %+v", r)
	}
	want := stampUnits(t, cache)
	if want == 0 {
		t.Fatal("the stamp of a plain build has no units")
	}
	// Another build of the same program (the source changed) in which the program is gone before the move.
	if err := os.WriteFile("hello.gp", []byte(helloSrc+"\n// changed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	testHookProgram = func(attempt int, p *program) {
		if attempt == 0 {
			os.Remove(p.exe)
		}
	}
	t.Cleanup(func() { testHookProgram = nil })
	if r := gocppRun("", false, "build", "-o", prog, "."); r.code != 0 || !strings.Contains(r.stderr, "building again") {
		t.Fatalf("gocpp build = %+v, want a build that was made again", r)
	}
	if got := stampUnits(t, cache); got != want {
		t.Errorf("the stamp of the build that was made again has %d units, want %d (the units of the first attempt must not stay)", got, want)
	}
}

// The words of a driver's "ended by a signal" line inside the program's own error are the program's error: a
// C++ file of the package with a syntax error whose line holds them (in a string, or in a #error) shows the
// compiler's message for that line, not "ended by the signal".
func TestOutsideKillWordsInUsersCodeAreNotAKill(t *testing.T) {
	for name, cpp := range map[string]string{
		"string":      "const char* msg = \"g++: fatal error: Killed signal terminated program cc1plus\" nonsense_token;\n",
		"clang":       "const char* msg = \"clang++: error: unable to execute command: Terminated\" nonsense_token;\n",
		"error":       "#error \"g++: fatal error: Killed signal terminated program cc1plus\"\n",
		"raw string":  "const char* msg = R\"(\ng++: fatal error: Killed signal terminated program cc1plus\n)\" nonsense_token;\n",
		"static":      "static_assert(false, \"clang++: error: unable to execute command: Killed\");\n",
		"line at top": "clang: error: unable to execute command: Terminated\n",
		// A message with a newline in it makes the compiler print a line that is the driver's text.
		"gcc newline":   "static_assert(false, \"x\\ng++: fatal error: Killed signal terminated program cc1plus\");\n",
		"clang newline": "static_assert(false, \"x\\nclang++: error: unable to execute command: Terminated\");\n",
		// A file named like the driver's line: its diagnostics start with the words.
		"file named like the driver": "int x\n",
	} {
		t.Run(name, func(t *testing.T) {
			forEachCXX(t, func(t *testing.T, c testCXX) {
				ownTmp(t)
				file := "bad.cpp"
				if name == "file named like the driver" {
					file = "g++: fatal error: Killed signal terminated program cc1plus.cpp"
				}
				inDir(t, map[string]string{"hello.gp": helloSrc, file: cpp})
				r := gocppRun("", false, "build", "-o", "prog", ".")
				if r.code == 0 || strings.Contains(r.stderr, "was ended by the signal") || !strings.Contains(r.stderr, file) {
					t.Errorf("gocpp build = %+v, want the compiler's error in %s and no 'ended by the signal'", r, file)
				}
			})
		})
	}
}

// Several units that fail because the compiler was ended get the message once.
func TestCompilerEndedMessageOnce(t *testing.T) {
	var out strings.Builder
	d := &driver{name: "build", stderr: &out}
	for i := 0; i < 3; i++ {
		d.compilerEnded(&build.CompileError{What: "the generated C++"}, "killed")
	}
	if n := strings.Count(out.String(), "was ended by the signal"); n != 1 {
		t.Errorf("the message came %d times, want once:\n%s", n, out.String())
	}
}
