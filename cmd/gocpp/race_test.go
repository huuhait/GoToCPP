package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// raceSrc is a program with a data race: two goroutines add to a global, and main goes on to print
// a line that the tests look for (a program that GORACE stops does not print it).
const raceSrc = `package main

import "fmt"

var counter int

func work(done chan bool) {
	for i := 0; i < 100; i++ {
		counter++
	}
	done <- true
}

func main() {
	done := make(chan bool)
	go work(done)
	go work(done)
	<-done
	<-done
	fmt.Println("after the race")
}
`

// quietSrc is a program without a race: the same work, one goroutine at a time.
const quietSrc = `package main

import "fmt"

var counter int

func work(done chan bool) {
	for i := 0; i < 100; i++ {
		counter++
	}
	done <- true
}

func main() {
	done := make(chan bool)
	go work(done)
	<-done
	go work(done)
	<-done
	fmt.Println("after", counter)
}
`

// TestRaceRun builds and runs programs with -race, with each compiler: the report in Go's form and
// exit code 66, the options of GORACE, TSan's own text with GOCPP_RACE_RAW, a program without a
// race, and -race with -asan.
func TestRaceRun(t *testing.T) {
	forEachCXX(t, func(t *testing.T, c testCXX) {
		t.Setenv("GOCPP_GATE_PHASE", "08")
		t.Setenv("GORACE", "")
		t.Setenv("TSAN_OPTIONS", "")
		inDir(t, map[string]string{"race.gp": raceSrc, "quiet.gp": quietSrc})

		r := gocppRun("", false, "run", "-race", "race.gp")
		report := regexp.MustCompile(`(?s)^==================\nWARNING: DATA RACE\n(Write|Read) at 0x[0-9a-f]+ by goroutine \d+:\n  main\.work\(\)\n      \S+race\.gp:9 \+0x[0-9a-f]+\n\nPrevious write at 0x[0-9a-f]+ by goroutine \d+:\n  main\.work\(\)\n      \S+race\.gp:9 \+0x[0-9a-f]+\n\nGoroutine \d+ \((running|finished)\) created at:\n  main\.main\(\)\n      \S+race\.gp:1[67] \+0x[0-9a-f]+\n\nGoroutine \d+ \((running|finished)\) created at:\n  main\.main\(\)\n      \S+race\.gp:1[67] \+0x[0-9a-f]+\n==================\nFound 1 data race\(s\)\n$`)
		if r.code != 66 || r.stdout != "after the race\n" || !report.MatchString(r.stderr) {
			t.Errorf("gocpp run -race race.gp = %+v, want exit 66, the output and Go's report on stderr, matching\n%s", r, report)
		}

		// GORACE: exitcode=N.
		t.Setenv("GORACE", "exitcode=7")
		if r := gocppRun("", false, "run", "-race", "race.gp"); r.code != 7 || !strings.Contains(r.stderr, "WARNING: DATA RACE") {
			t.Errorf("GORACE=exitcode=7: %+v, want exit 7 and the report", r)
		}
		// GORACE: halt_on_error=1 stops at the report: the program does not go on, and the report is
		// still Go's (the reporter is another process, so the dying callback can wait for it).
		t.Setenv("GORACE", "halt_on_error=1")
		if r := gocppRun("", false, "run", "-race", "race.gp"); r.code != 66 || strings.Contains(r.stdout, "after the race") ||
			!strings.Contains(r.stderr, "WARNING: DATA RACE") || strings.Contains(r.stderr, "Found") {
			t.Errorf("GORACE=halt_on_error=1: %+v, want exit 66, no output and Go's report without a count", r)
		}
		// GORACE: log_path sends TSan's own text to a file and keeps the report out of stderr.
		logs := filepath.Join(t.TempDir(), "tsan")
		t.Setenv("GORACE", "log_path="+logs)
		if r := gocppRun("", false, "run", "-race", "race.gp"); r.code != 66 || r.stdout != "after the race\n" || r.stderr != "" {
			t.Errorf("GORACE=log_path: %+v, want exit 66, the output and an empty stderr", r)
		}
		if files, _ := filepath.Glob(logs + ".*"); len(files) != 1 {
			t.Errorf("GORACE=log_path: files %v, want one with TSan's report", files)
		} else if b, _ := os.ReadFile(files[0]); !strings.Contains(string(b), "WARNING: ThreadSanitizer: data race") {
			t.Errorf("%s = %q, want TSan's report", files[0], b)
		}
		// GORACE: strip_path_prefix shortens the file names of Go's form.
		dir, _ := os.Getwd()
		t.Setenv("GORACE", "strip_path_prefix="+dir+string(filepath.Separator))
		if r := gocppRun("", false, "run", "-race", "race.gp"); r.code != 66 || !regexp.MustCompile(`(?m)^      race\.gp:9 \+0x`).MatchString(r.stderr) {
			t.Errorf("GORACE=strip_path_prefix: %+v, want file names without the folder", r)
		}
		// TSAN_OPTIONS wins over GORACE.
		t.Setenv("TSAN_OPTIONS", "halt_on_error=0")
		if r := gocppRun("", false, "run", "-race", "race.gp"); r.code != 66 || r.stdout != "after the race\n" {
			t.Errorf("TSAN_OPTIONS=halt_on_error=0: %+v, want the program to go on", r)
		}
		t.Setenv("TSAN_OPTIONS", "")
		t.Setenv("GORACE", "")

		// GOCPP_RACE_RAW: TSan's own text.
		t.Setenv("GOCPP_RACE_RAW", "1")
		if r := gocppRun("", false, "run", "-race", "race.gp"); r.code != 66 || !strings.Contains(r.stderr, "WARNING: ThreadSanitizer: data race") ||
			strings.Contains(r.stderr, "WARNING: DATA RACE") {
			t.Errorf("GOCPP_RACE_RAW=1: %+v, want TSan's report", r)
		}
		t.Setenv("GOCPP_RACE_RAW", "")

		// A program without a race: silent, and its own exit code.
		if r := gocppRun("", false, "run", "-race", "quiet.gp"); r != (result{0, "after 200\n", ""}) {
			t.Errorf("gocpp run -race quiet.gp = %+v, want exit 0, the output and no report", r)
		}

		// The two sanitizers don't work together.
		if r := gocppRun("", false, "build", "-race", "-asan", "quiet.gp"); r.code != 2 || !strings.Contains(r.stderr, "-race and -asan can't be used together") {
			t.Errorf("gocpp build -race -asan = %+v, want exit 2 and the message", r)
		}
	})
}
