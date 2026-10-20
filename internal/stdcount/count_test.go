package stdcount

import (
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestParseReadsStatusOfEachTopLevelTest(t *testing.T) {
	out := []byte(`=== RUN   TestA
--- PASS: TestA (0.00s)
=== RUN   TestB
    b_test.gp:7: it failed
--- FAIL: TestB (0.00s)
=== RUN   TestC
--- SKIP: TestC (0.00s)
`)

	results, running, last := Parse(out)

	got := []string{results["TestA"].Status, results["TestB"].Status, results["TestC"].Status}
	if !slices.Equal(got, []string{"pass", "fail", "skip"}) {
		t.Fatalf("statuses = %v; want pass, fail, skip", got)
	}
	if len(running) != 0 {
		t.Fatalf("running = %v; want none", running)
	}
	if last != "TestC" {
		t.Fatalf("last = %q; want TestC", last)
	}
}

func TestParseKeepsLogLinesOfTheTest(t *testing.T) {
	out := []byte(`=== RUN   TestB
    b_test.gp:7: first
    b_test.gp:8: second
--- FAIL: TestB (0.00s)
`)

	results, _, _ := Parse(out)

	lines := results["TestB"].Lines
	if len(lines) != 2 || !strings.Contains(lines[0], "first") ||
		!strings.Contains(lines[1], "second") {
		t.Fatalf("lines = %q; want the two log lines", lines)
	}
}

func TestParseReportsTestWithoutResultAsRunning(t *testing.T) {
	out := []byte(`=== RUN   TestA
--- PASS: TestA (0.00s)
=== RUN   TestCrash
panic: boom
`)

	results, running, last := Parse(out)

	if results["TestCrash"] != nil {
		t.Fatalf("TestCrash has a result: %v", results["TestCrash"])
	}
	if !slices.Equal(running, []string{"TestCrash"}) {
		t.Fatalf("running = %v; want [TestCrash]", running)
	}
	if last != "TestA" {
		t.Fatalf("last = %q; want TestA", last)
	}
}

func TestParseIgnoresSubtestsForRunningList(t *testing.T) {
	out := []byte(`=== RUN   TestA
=== RUN   TestA/sub
    --- PASS: TestA/sub (0.00s)
--- PASS: TestA (0.00s)
`)

	results, running, _ := Parse(out)

	if results["TestA"].Status != "pass" || len(running) != 0 {
		t.Fatalf("results = %v, running = %v", results, running)
	}
}

func TestTimedOutNamesRunningTopLevelTests(t *testing.T) {
	out := []byte(`panic: test timed out after 10m0s
	running tests:
		TestSlow (10m0s)
		TestOther/sub (9m0s)
		TestThird (1m0s)

goroutine 1 [running]:
`)

	names := timedOut(out)

	if !slices.Equal(names, []string{"TestSlow", "TestThird"}) {
		t.Fatalf("timedOut = %v; want [TestSlow TestThird]", names)
	}
}

func TestTimedOutWithoutTimeoutIsEmpty(t *testing.T) {
	if names := timedOut([]byte("PASS\n")); names != nil {
		t.Fatalf("timedOut = %v; want none", names)
	}
}

// manifestOf returns a manifest with one taken file and one left-out file.
func manifestOf(tests []TestEntry, expected map[string]ExpectEntry) *Manifest {
	return &Manifest{
		Package: "demo",
		Files: map[string]FileEntry{
			"a_test.go": {Status: "run"},
			"b_test.go": {Status: "excluded", Reason: "it needs net"},
		},
		Tests:    tests,
		Expected: expected,
	}
}

// singleTestManifest returns a manifest with the one taken function TestA and no expectations.
func singleTestManifest() *Manifest {
	test := TestEntry{Name: "TestA", File: "a_test.go", Kind: "test", Status: "run"}
	return manifestOf([]TestEntry{test}, nil)
}

func passing(string) *Result {
	return &Result{Status: "pass"}
}

func TestTallyCountsPassingFunctions(t *testing.T) {
	m := manifestOf([]TestEntry{
		{Name: "TestA", File: "a_test.go", Kind: "test", Status: "run"},
		{Name: "TestB", File: "a_test.go", Kind: "test", Status: "run"},
	}, nil)

	count := Tally(m, map[string]*Result{"TestA": passing("TestA"), "TestB": passing("TestB")})

	if count.Total != 2 || count.Selected != 2 || count.Passed != 2 || len(count.Problems) != 0 {
		t.Fatalf("count = %+v", count)
	}
	if count.Percent() != 100 {
		t.Fatalf("Percent = %v; want 100", count.Percent())
	}
}

func TestTallyLeavesOutFunctionsOfExcludedFiles(t *testing.T) {
	m := manifestOf([]TestEntry{
		{Name: "TestA", File: "a_test.go", Kind: "test", Status: "run"},
		{Name: "TestNet", File: "b_test.go", Kind: "test", Status: "excluded",
			Reason: "it needs net"},
	}, nil)

	count := Tally(m, map[string]*Result{"TestA": passing("TestA")})

	if count.Total != 2 || count.Selected != 1 || count.Excluded != 1 {
		t.Fatalf("count = %+v", count)
	}
	if got := count.Overall(); got != 50 {
		t.Fatalf("Overall = %v; want 50", got)
	}
}

func TestTallyCountsDroppedFunctionAsNotPassing(t *testing.T) {
	m := manifestOf([]TestEntry{
		{Name: "TestA", File: "a_test.go", Kind: "test", Status: "run"},
		{Name: "TestGob", File: "a_test.go", Kind: "test", Status: "excluded",
			Reason: "it needs gob"},
	}, nil)

	count := Tally(m, map[string]*Result{"TestA": passing("TestA")})

	if count.Selected != 2 || count.Dropped != 1 || count.Divergence != 0 {
		t.Fatalf("count = %+v", count)
	}
	if got := count.Percent(); got != 50 {
		t.Fatalf("Percent = %v; want 50", got)
	}
}

func TestTallyTakesDroppedDivergenceOutOfTheDenominator(t *testing.T) {
	m := manifestOf([]TestEntry{
		{Name: "TestA", File: "a_test.go", Kind: "test", Status: "run"},
		{Name: "TestAllocs", File: "a_test.go", Kind: "test", Status: "excluded",
			Reason: "divergence: it counts allocations"},
	}, nil)

	count := Tally(m, map[string]*Result{"TestA": passing("TestA")})

	if count.Dropped != 1 || count.Divergence != 1 {
		t.Fatalf("count = %+v", count)
	}
	if got := count.Percent(); got != 100 {
		t.Fatalf("Percent = %v; want 100", got)
	}
}

func TestTallyReportsUnexpectedFailureAsProblem(t *testing.T) {
	m := singleTestManifest()
	failing := &Result{Status: "fail", Lines: []string{"a_test.gp:3: bad"}}

	count := Tally(m, map[string]*Result{"TestA": failing})

	if count.Failed != 1 || len(count.Problems) != 1 ||
		!strings.Contains(count.Problems[0], "TestA: fail and not expected to") {
		t.Fatalf("count = %+v", count)
	}
}

func TestTallyReportsUnexpectedSkipAsSkippedProblem(t *testing.T) {
	m := singleTestManifest()

	count := Tally(m, map[string]*Result{"TestA": {Status: "skip"}})

	if count.Skipped != 1 || count.Failed != 0 || len(count.Problems) != 1 {
		t.Fatalf("count = %+v", count)
	}
}

func TestTallyReportsMissingResultAsFailure(t *testing.T) {
	m := singleTestManifest()

	count := Tally(m, map[string]*Result{})

	if count.Failed != 1 || !strings.Contains(count.Rows[0].Reason, "ended before it ran") {
		t.Fatalf("count = %+v, rows = %+v", count, count.Rows)
	}
}

func TestTallyAcceptsExpectedFailure(t *testing.T) {
	m := manifestOf(
		[]TestEntry{{Name: "TestA", File: "a_test.go", Kind: "test", Status: "run"}},
		map[string]ExpectEntry{"TestA": {Class: "bug", Reason: "RemoveAll is path based"}},
	)

	count := Tally(m, map[string]*Result{"TestA": {Status: "fail"}})

	if count.Failed != 1 || len(count.Problems) != 0 {
		t.Fatalf("count = %+v", count)
	}
	if count.Rows[0].Class != "bug" || count.Rows[0].Reason != "RemoveAll is path based" {
		t.Fatalf("row = %+v", count.Rows[0])
	}
}

func TestTallyTakesExpectedDivergenceOutOfTheDenominator(t *testing.T) {
	m := manifestOf(
		[]TestEntry{
			{Name: "TestA", File: "a_test.go", Kind: "test", Status: "run"},
			{Name: "TestAllocs", File: "a_test.go", Kind: "test", Status: "run"},
		},
		map[string]ExpectEntry{"TestAllocs": {Class: "divergence", Reason: "allocation counts"}},
	)

	count := Tally(m, map[string]*Result{
		"TestA":      passing("TestA"),
		"TestAllocs": {Status: "fail"},
	})

	if count.Divergence != 1 || count.Failed != 0 {
		t.Fatalf("count = %+v", count)
	}
	if got := count.Percent(); got != 100 {
		t.Fatalf("Percent = %v; want 100", got)
	}
}

func TestTallyRejectsStaleExpectation(t *testing.T) {
	m := manifestOf(
		[]TestEntry{{Name: "TestA", File: "a_test.go", Kind: "test", Status: "run"}},
		map[string]ExpectEntry{"TestA": {Class: "bug", Reason: "fixed meanwhile"}},
	)

	count := Tally(m, map[string]*Result{"TestA": passing("TestA")})

	if len(count.Problems) != 1 || !strings.Contains(count.Problems[0], "remove the expectation") {
		t.Fatalf("problems = %v", count.Problems)
	}
}

func TestTallyAllowsFlakyTestToPass(t *testing.T) {
	m := manifestOf(
		[]TestEntry{{Name: "TestA", File: "a_test.go", Kind: "test", Status: "run"}},
		map[string]ExpectEntry{"TestA": {Class: "flaky", Reason: "a race in Go's test"}},
	)

	count := Tally(m, map[string]*Result{"TestA": passing("TestA")})

	if count.Passed != 1 || len(count.Problems) != 0 {
		t.Fatalf("count = %+v", count)
	}
}

func TestTallyDoesNotCountBenchmarksAndExamplesWithoutOutput(t *testing.T) {
	m := manifestOf([]TestEntry{
		{Name: "TestA", File: "a_test.go", Kind: "test", Status: "run"},
		{Name: "BenchmarkA", File: "a_test.go", Kind: "benchmark", Status: "run"},
		{Name: "ExampleNoOutput", File: "a_test.go", Kind: "compileonly", Status: "run"},
	}, nil)

	count := Tally(m, map[string]*Result{"TestA": passing("TestA")})

	if count.Total != 1 {
		t.Fatalf("Total = %d; want 1", count.Total)
	}
}

func TestTallyCountsExamplesApart(t *testing.T) {
	m := manifestOf([]TestEntry{
		{Name: "ExampleA", File: "a_test.go", Kind: "example", Status: "run"},
		{Name: "ExampleB", File: "a_test.go", Kind: "example", Status: "run"},
	}, nil)

	count := Tally(m, map[string]*Result{
		"ExampleA": passing("ExampleA"),
		"ExampleB": {Status: "fail"},
	})

	if count.Examples != 2 || count.ExamplesPassed != 1 {
		t.Fatalf("count = %+v", count)
	}
}

func TestPercentOfEmptySelectionIsFull(t *testing.T) {
	count := &Count{}

	if count.Percent() != 100 || count.Overall() != 100 {
		t.Fatalf("Percent = %v, Overall = %v; want 100", count.Percent(), count.Overall())
	}
}

func TestTableSumsPackages(t *testing.T) {
	a := &Count{Package: "a", Total: 10, Selected: 8, Passed: 8}
	b := &Count{Package: "b", Total: 10, Selected: 10, Passed: 5, Failed: 5}

	table := Table([]*Count{a, b})

	if !strings.Contains(table, "| total | 20 | 18 | 13 |") {
		t.Fatalf("table = %s", table)
	}
	if !strings.Contains(table, "| b | 10 | 10 | 5 | 5 |") {
		t.Fatalf("table = %s", table)
	}
}

func TestDetailListsOnlyFunctionsThatDoNotPass(t *testing.T) {
	count := &Count{Package: "demo", Rows: []Row{
		{Name: "TestOk", File: "a_test.go", Outcome: "pass"},
		{Name: "TestBad", File: "a_test.go", Outcome: "fail", Class: "bug", Reason: "it is broken"},
	}}

	detail := Detail(count)

	if strings.Contains(detail, "TestOk") ||
		!strings.Contains(detail, "TestBad (a_test.go, fail) [bug]: it is broken") {
		t.Fatalf("detail = %s", detail)
	}
}

func TestLoadManifestReadsPackageWithSlashes(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "manifest"), 0o755); err != nil {
		t.Fatal(err)
	}
	json := `{"package": "encoding/json", "tests": [
		{"name": "TestX", "file": "x_test.go", "kind": "test", "status": "run"}]}`
	path := filepath.Join(dir, "manifest", "encoding__json.json")
	if err := os.WriteFile(path, []byte(json), 0o644); err != nil {
		t.Fatal(err)
	}

	m, err := LoadManifest(dir, "encoding/json")

	if err != nil || m.Package != "encoding/json" || len(m.Tests) != 1 {
		t.Fatalf("LoadManifest = %+v, %v", m, err)
	}
}

func TestLoadManifestOfMissingPackageFails(t *testing.T) {
	if _, err := LoadManifest(t.TempDir(), "nothing"); err == nil {
		t.Fatal("LoadManifest of a missing package succeeded")
	}
}

func TestPercentUsesSelectionWithoutDivergences(t *testing.T) {
	count := &Count{Selected: 10, Divergence: 2, Passed: 6}

	if got := count.Percent(); math.Abs(got-75) > 1e-9 {
		t.Fatalf("Percent = %v; want 75", got)
	}
}

func TestRejectsCleanPackageIsEmpty(t *testing.T) {
	count := &Count{Selected: 10, Passed: 10}

	if rejects := count.Rejects(); len(rejects) != 0 {
		t.Fatalf("Rejects = %q; want none", rejects)
	}
}

func TestRejectsPackageAtTheBudgetIsClean(t *testing.T) {
	count := &Count{Selected: 10, Passed: 9, Failed: 1}

	if rejects := count.Rejects(); len(rejects) != 0 {
		t.Fatalf("Rejects = %q; want none at exactly %v %%", rejects, Budget)
	}
}

func TestRejectsPackageUnderTheBudget(t *testing.T) {
	count := &Count{Package: "demo", Selected: 10, Passed: 8, Failed: 2}

	rejects := count.Rejects()

	if len(rejects) != 1 || !strings.Contains(rejects[0], "80.0 % pass, under the 90 % budget") {
		t.Fatalf("Rejects = %q", rejects)
	}
}

func TestRejectsKeepsProblemsOfTheRun(t *testing.T) {
	count := &Count{Selected: 10, Passed: 10, Problems: []string{"TestA: fail and not expected to"}}

	rejects := count.Rejects()

	if len(rejects) != 1 || rejects[0] != "TestA: fail and not expected to" {
		t.Fatalf("Rejects = %q", rejects)
	}
	if len(count.Problems) != 1 {
		t.Fatalf("Rejects changed Problems: %q", count.Problems)
	}
}

func TestRejectsWithUnexpectedFailureUnderBudgetListsBoth(t *testing.T) {
	m := singleTestManifest()
	count := Tally(m, map[string]*Result{"TestA": {Status: "fail"}})

	rejects := count.Rejects()

	if len(rejects) != 2 {
		t.Fatalf("Rejects = %q; want the failure and the budget", rejects)
	}
}
