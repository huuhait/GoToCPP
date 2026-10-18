// Quick runs the checks that matter for what you changed, in a minute or two instead of the
// ten that "make ci" takes. It is for the edit-test loop; "make ci" (and "make staticcheck",
// and "make ci-sanitize" for runtime or emitter changes) is still the gate before a change is
// handed over or merged.
//
// Usage, from the repository root, in the dev shell:
//
//	make quick                      # what differs from the merge base with dev
//	go run ./tools/quick -base main # another base branch
//	go run ./tools/quick -n         # only print the commands
//
// What it runs, at the same time where it can:
//
//   - gofmt -l on the changed Go files, and go vet and go test on the Go packages with a
//     changed file (not their dependents; ./tests and ./runtime are handled below);
//   - for each changed test fixture (tests/<kind>/<name>.gp, its .cpp.golden, or
//     examples/<name>/...), only that fixture, with one compiler (GOCPP_CXX, else g++);
//   - a small fixed smoke set of fixtures, so a change in the front end or the emitter that no
//     package test notices still gets one end-to-end run;
//   - when runtime/ changed: the runtime's fast tests, and its C++ unit tests with one
//     compiler in the debug mode only (not -O2 and not the sanitizers).
//
// Fixtures of a phase after the last built one (internal/version) are skipped unless
// GOCPP_TEST_PHASE says so, as for go test. "make quick" runs the phases up to CI_TEST_PHASE of
// the Makefile (08) like "make ci" (it sets GOCPP_TEST_PHASE unless you did); a fixture with a
// "// pending:" line is skipped (GOCPP_TEST_PENDING=1 runs it). GOCPP_TEST_PHASE=09 make quick
// includes the fixtures of a phase that is being built.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"
)

// kinds maps a folder of tests/ to the test function that runs its fixtures.
var kinds = map[string]string{
	"run": "TestRun", "errors": "TestErrors", "golden": "TestGolden",
	"diff": "TestDiff", "interop": "TestInterop", "debug": "TestDebug",
}

// smoke are fixtures that always run: one of each common kind, small and fast.
var smoke = map[string][]string{
	"TestRun":    {"algo_primes", "arrays_basics"},
	"TestErrors": {"cgo_helpers"},
	"TestGolden": {"control"},
}

func main() {
	base := flag.String("base", "dev", "the branch to compare with")
	dry := flag.Bool("n", false, "print the commands and don't run them")
	flag.Parse()

	files, err := changedFiles(*base)
	if err != nil {
		fmt.Fprintln(os.Stderr, "quick:", err)
		os.Exit(2)
	}
	cxx := os.Getenv("GOCPP_CXX")
	if cxx == "" {
		cxx = "g++"
	}
	plan := makePlan(files, cxx)
	if len(plan.steps) == 0 {
		fmt.Println("quick: nothing to run")
		return
	}
	fmt.Printf("quick: %d changed files against %s; GOCPP_CXX=%s\n", len(files), *base, cxx)
	for _, n := range plan.notes {
		fmt.Println("note:", n)
	}
	if *dry {
		for _, s := range plan.steps {
			fmt.Println(s.String())
		}
		return
	}
	os.Exit(run(plan.steps))
}

// A step is one command.
type step struct {
	name string
	env  []string
	args []string
}

func (s step) String() string {
	return strings.Join(append(slices.Clone(s.env), s.args...), " ")
}

type plan struct {
	steps []step
	notes []string
}

// makePlan decides what to run for the changed files (paths from the repository root, with
// slashes), with the compiler cxx for the programs.
func makePlan(files []string, cxx string) plan {
	var p plan
	env := []string{"GOCPP_CXX=" + cxx}

	var goFiles []string
	pkgs := map[string]bool{}
	runtimeChanged := false
	fixtures := map[string]map[string]bool{} // test function -> fixture names
	for _, f := range files {
		switch {
		case strings.HasPrefix(f, "spikes/") || strings.Contains(f, "/testdata/"):
			continue
		case strings.HasPrefix(f, "runtime/"):
			runtimeChanged = true
		}
		if strings.HasSuffix(f, ".go") && exists(f) {
			goFiles = append(goFiles, f)
			if dir := path.Dir(f); dir != "." && dir != "tests" && dir != "runtime" {
				pkgs["./"+dir] = true
			}
		}
		if fn, name, ok := fixtureOf(f); ok {
			if fixtures[fn] == nil {
				fixtures[fn] = map[string]bool{}
			}
			fixtures[fn][name] = true
		} else if strings.HasPrefix(f, "tests/") && strings.Count(f, "/") == 2 && !strings.HasSuffix(f, "_test.go") && !strings.HasSuffix(f, ".md") {
			p.notes = append(p.notes, f+" is a helper of several fixtures: run the whole folder with make ci")
		}
	}

	if len(goFiles) > 0 {
		p.steps = append(p.steps, step{name: "gofmt", args: append([]string{"sh", "-c", `bad=$(gofmt -l "$@"); if [ -n "$bad" ]; then echo "gofmt: these files need formatting:"; echo "$bad"; exit 1; fi`, "sh"}, goFiles...)})
	}
	list := sortedKeys(pkgs)
	if len(list) > 0 {
		p.steps = append(p.steps,
			step{name: "go vet", args: append([]string{"go", "vet"}, list...)},
			step{name: "go test packages", args: append([]string{"go", "test", "-count=1", "-timeout", "30m"}, list...)})
	}
	if runtimeChanged {
		p.steps = append(p.steps,
			step{name: "runtime fast tests", args: []string{"go", "test", "-count=1", "-timeout", "30m", "./runtime", "-run", "^(TestFSHasEveryFile|TestSources|TestLayout|TestIsPrintTable|TestNoPlainMembers|TestNoPlainMembersFinds|TestRejected|TestNoOSHeadersOutsideOS|TestCheckIncludes)$"}},
			step{name: "runtime C++ unit tests (O0)", env: env, args: []string{"go", "test", "-count=1", "-timeout", "30m", "./runtime", "-run", "^TestCXX$/^" + regexp.QuoteMeta(cxx) + "$/^O0$"}})
	}
	// Fixtures: the changed ones, then the smoke set. One go test per test function, since
	// -run matches the subtests of one top-level test.
	for fn, names := range smoke {
		if fixtures[fn] == nil {
			fixtures[fn] = map[string]bool{}
		}
		for _, n := range names {
			fixtures[fn][n] = true
		}
	}
	var fns []string
	for fn := range fixtures {
		fns = append(fns, fn)
	}
	sort.Strings(fns)
	for _, fn := range fns {
		p.steps = append(p.steps, step{
			name: fn, env: env,
			args: []string{"go", "test", "-count=1", "-timeout", "30m", "./tests", "-run", fixtureRegex(fn, sortedKeys(fixtures[fn]))},
		})
	}
	return p
}

// fixtureOf maps a changed file to the test function and the fixture name it belongs to:
// tests/<kind>/<name>.gp (also <name>.cpp.golden in golden), tests/pkgs/<name>/... and examples/<name>/...
func fixtureOf(f string) (fn, name string, ok bool) {
	parts := strings.Split(f, "/")
	switch {
	case len(parts) == 3 && parts[0] == "tests":
		fn, ok = kinds[parts[1]]
		if !ok {
			return "", "", false
		}
		base := parts[2]
		switch {
		case strings.HasSuffix(base, ".cpp.golden"):
			name = strings.TrimSuffix(base, ".cpp.golden")
		case strings.HasSuffix(base, ".gp"):
			name = strings.TrimSuffix(base, ".gp")
		default:
			return "", "", false
		}
		return fn, name, true
	case len(parts) >= 4 && parts[0] == "tests" && parts[1] == "pkgs" && parts[2] == "errors":
		return "TestPkgErrors", parts[3], true // tests/pkgs/errors/<name>/...: a module folder
	case len(parts) >= 4 && parts[0] == "tests" && parts[1] == "pkgs":
		return "TestPkgs", parts[2], true // tests/pkgs/<name>/...: a module folder
	case len(parts) >= 3 && parts[0] == "examples":
		return "TestExamples", parts[1], true
	}
	return "", "", false
}

// fixtureRegex is the -run pattern for the fixtures names of test function fn: the subtests
// are named <fixture>/<compiler>.
func fixtureRegex(fn string, names []string) string {
	q := make([]string, len(names))
	for i, n := range names {
		q[i] = regexp.QuoteMeta(n)
	}
	return "^" + fn + "$/^(" + strings.Join(q, "|") + ")$"
}

func sortedKeys(m map[string]bool) []string {
	var s []string
	for k := range m {
		s = append(s, k)
	}
	sort.Strings(s)
	return s
}

func exists(f string) bool {
	_, err := os.Stat(f)
	return err == nil
}

// changedFiles lists the files that differ from the merge base with base (committed, staged
// and unstaged) and the new files that git doesn't ignore, as paths from the repository root.
func changedFiles(base string) ([]string, error) {
	root, err := git("rev-parse", "--show-toplevel")
	if err != nil {
		return nil, err
	}
	if err := os.Chdir(strings.TrimSpace(root)); err != nil {
		return nil, err
	}
	mb := "HEAD"
	if out, err := git("merge-base", "HEAD", base); err == nil {
		mb = strings.TrimSpace(out)
	} else {
		fmt.Fprintf(os.Stderr, "quick: no merge base with %s (%v): comparing with HEAD\n", base, err)
	}
	seen := map[string]bool{}
	var files []string
	for _, args := range [][]string{
		{"diff", "--name-only", "--diff-filter=ACMRTD", mb},
		{"ls-files", "--others", "--exclude-standard"},
	} {
		out, err := git(args...)
		if err != nil {
			return nil, err
		}
		for _, f := range strings.Split(out, "\n") {
			if f = filepath.ToSlash(strings.TrimSpace(f)); f != "" && !seen[f] {
				seen[f] = true
				files = append(files, f)
			}
		}
	}
	return files, nil
}

func git(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git %s: %v: %s", strings.Join(args, " "), err, strings.TrimSpace(errb.String()))
	}
	return out.String(), nil
}

// run runs the steps at the same time, prints each one's output in order with its time, and
// returns the exit code: 1 if any failed.
func run(steps []step) int {
	type result struct {
		out []byte
		err error
		d   time.Duration
	}
	res := make([]result, len(steps))
	var wg sync.WaitGroup
	for i, s := range steps {
		wg.Go(func() {
			start := time.Now()
			cmd := exec.Command(s.args[0], s.args[1:]...)
			cmd.Env = append(os.Environ(), s.env...)
			out, err := cmd.CombinedOutput()
			res[i] = result{out, err, time.Since(start)}
		})
	}
	wg.Wait()
	failed := 0
	for i, s := range steps {
		r := res[i]
		status := "ok"
		if r.err != nil {
			status, failed = "FAIL", failed+1
		}
		fmt.Printf("== %-28s %-4s %6.1fs\n", s.name, status, r.d.Seconds())
		if r.err != nil {
			os.Stdout.Write(r.out)
		}
	}
	if failed > 0 {
		fmt.Printf("quick: %d of %d steps failed. Not a substitute for make ci.\n", failed, len(steps))
		return 1
	}
	fmt.Printf("quick: all %d steps passed. Run make ci (and make staticcheck) before handing the change over.\n", len(steps))
	return 0
}
