// Package stdcount counts how many of Go's own test functions pass under gocpp test (Phase 09 step
// 9.10).
//
// tools/portstd/gotests/convert_std.py writes tests/stdgo/<pkg>/ (Go's test files, patched) and
// tests/stdgo/manifest/<pkg>.json: every Test, Example and Fuzz function of the package's test
// files for linux/amd64 with what the conversion did to it (taken, or excluded with a reason), and
// the functions that are expected to fail or skip in gocpp, each with a class and a reason. Run
// executes the test binary of a package (built by gocpp test -c with GOCPP_TEST_STDLIB=tests/stdgo)
// and reads the result of each test function from the -test.v output; Tally applies the counting
// rule of the Phase 09 README (2.8):
//
//   - a test function counts as passing when it reports PASS;
//   - a skipped test is not passing, unless it is skipped for a documented divergence;
//   - a function that is excluded (its file or its function is not taken) is not passing either,
//     with its written reason, unless the reason is a documented divergence;
//   - the budget is passing / (selected - divergences), over the test functions of the selected
//     files.
//
// A divergence is a difference from Go that the plan documents and that the test measures:
// allocation counts (testing.AllocsPerRun: gocpp allocates differently), goroutine stack depth
// (fixed stacks, D24), the garbage collector and finalizers (reference counting, ADR-004), the
// runtime's memory statistics.
package stdcount

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
)

// A Manifest is tests/stdgo/manifest/<pkg>.json.
type Manifest struct {
	Package  string                 `json:"package"`
	Go       string                 `json:"go"`
	Args     []string               `json:"args"`
	Files    map[string]FileEntry   `json:"files"`
	Tests    []TestEntry            `json:"tests"`
	Expected map[string]ExpectEntry `json:"expected"`
	Patches  []PatchEntry           `json:"patches"`
}

// A FileEntry says whether a test file of Go is taken ("run") or left out ("excluded", with the
// reason).
type FileEntry struct {
	Status string `json:"status"`
	Reason string `json:"reason"`
}

// A TestEntry is one Test, Example, Fuzz or Benchmark function of Go's test files.
type TestEntry struct {
	Name   string `json:"name"`
	File   string `json:"file"`
	Kind   string `json:"kind"` // test, example, fuzz or benchmark
	Status string `json:"status"`
	Reason string `json:"reason"`
}

// An ExpectEntry says that a test function fails or skips in gocpp, and why.
type ExpectEntry struct {
	Class  string `json:"class"` // divergence, bug, unsupported or flaky
	Reason string `json:"reason"`
}

// A PatchEntry is a change to a test file, with its reason.
type PatchEntry struct {
	File   string `json:"file"`
	Reason string `json:"reason"`
}

// LoadManifest reads the manifest of the package with the given import path.
func LoadManifest(stdgo, pkg string) (*Manifest, error) {
	name := strings.ReplaceAll(pkg, "/", "__") + ".json"
	data, err := os.ReadFile(filepath.Join(stdgo, "manifest", name))
	if err != nil {
		return nil, err
	}
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return &m, nil
}

// Result is what one run of a test function reported.
type Result struct {
	Status string   // pass, fail, skip or "" (no result: the binary ended first)
	Lines  []string // the lines the test logged (without the === and --- lines)
}

// Parse reads the output of a test binary run with -test.v: the result of each top-level test
// function. A result line starts at column 0 ("--- PASS: TestX (0.00s)"); the lines of a test
// before its result line, and the lines after a failing result line (a panic), belong to it.
func Parse(out []byte) (results map[string]*Result, running []string, last string) {
	results = make(map[string]*Result)
	var pending []string // lines since the last result line
	started := map[string]bool{}
	var order []string
	sc := bufio.NewScanner(bytes.NewReader(out))
	sc.Buffer(make([]byte, 1<<20), 1<<26)
	resultLine := regexp.MustCompile(`^--- (PASS|FAIL|SKIP): (\S+) \(`)
	runLine := regexp.MustCompile(`^=== RUN   (\S+)$`)
	for sc.Scan() {
		line := sc.Text()
		if m := resultLine.FindStringSubmatch(line); m != nil {
			res := &Result{Status: strings.ToLower(m[1]), Lines: pending}
			results[m[2]] = res
			if !strings.HasPrefix(line, " ") {
				last = m[2]
			}
			pending = nil
			continue
		}
		if m := runLine.FindStringSubmatch(line); m != nil {
			if !strings.Contains(m[1], "/") {
				started[m[1]] = true
				order = append(order, m[1])
			}
			continue
		}
		if strings.HasPrefix(line, "=== ") {
			continue
		}
		pending = append(pending, line)
	}
	for _, name := range order {
		if results[name] == nil {
			running = append(running, name)
		}
	}
	return results, running, last
}

// A Count is the counts of one package.
type Count struct {
	Package string

	// Total is over the functions of all the test files of Go for linux/amd64 (benchmarks and
	// examples without an Output comment, which go test does not run either, are left out).
	Total int

	// The following are over the selected files (FILES of the specification).
	Selected   int
	Passed     int
	Failed     int // fail or crash, not explained as a divergence
	Skipped    int // skipped, not explained as a divergence
	Dropped    int // excluded functions of the selected files (the reason is in the manifest)
	Divergence int // failing, skipped or dropped for a documented divergence: out of the budget

	// Excluded is the number of functions of the files that are not taken.
	Excluded int

	Examples, ExamplesPassed int

	Problems []string // what a check rejects: unexplained failures, stale expectations
	Rows     []Row
}

// A Row is one test function in the report.
type Row struct {
	Name, File, Kind string
	Outcome          string // pass, fail, skip, dropped, file excluded, divergence
	Class            string
	Reason           string
}

// Budget is the share of passing functions, in percent, that each package has to reach.
const Budget = 90.0

// Percent returns the share of passing functions among the selected ones without the divergences.
func (c *Count) Percent() float64 {
	denom := c.Selected - c.Divergence
	if denom <= 0 {
		return 100
	}
	return 100 * float64(c.Passed) / float64(denom)
}

// Overall returns the share of passing functions among all the functions of Go's test files.
func (c *Count) Overall() float64 {
	if c.Total == 0 {
		return 100
	}
	return 100 * float64(c.Passed) / float64(c.Total)
}

// Rejects returns what a check of the package rejects: the unexplained failures and skips, the
// stale expectations, and a pass share under the budget.
func (c *Count) Rejects() []string {
	rejects := slices.Clone(c.Problems)
	if c.Percent() < Budget {
		rejects = append(rejects,
			fmt.Sprintf("%.1f %% pass, under the %.0f %% budget", c.Percent(), Budget))
	}
	return rejects
}

// Tally applies the counting rule to the manifest and the results of a run.
func Tally(m *Manifest, results map[string]*Result) *Count {
	c := &Count{Package: m.Package}
	for _, t := range m.Tests {
		if t.Kind == "benchmark" || t.Kind == "compileonly" {
			continue
		}
		c.Total++
		row := Row{Name: t.Name, File: t.File, Kind: t.Kind}
		switch {
		case m.Files[t.File].Status != "run":
			c.Excluded++
			row.Outcome = "file excluded"
			row.Reason = t.Reason
		case t.Status != "run":
			c.Selected++
			c.tallyDropped(&row, t)
		default:
			c.Selected++
			c.tallyRun(&row, t, results[t.Name], m.Expected[t.Name])
		}
		c.Rows = append(c.Rows, row)
	}
	return c
}

// tallyDropped counts a function of a selected file that the conversion left out.
func (c *Count) tallyDropped(row *Row, t TestEntry) {
	c.Dropped++
	row.Outcome = "dropped"
	row.Reason = t.Reason
	if isDivergenceReason(t.Reason) {
		c.Divergence++
		row.Class = "divergence"
	}
}

// tallyRun counts a function that ran: res is nil when the test binary ended before the function
// reported, exp is the zero value when the specification does not expect it to fail or skip.
func (c *Count) tallyRun(row *Row, t TestEntry, res *Result, exp ExpectEntry) {
	expected := exp.Class != ""
	status := "none"
	if res != nil {
		status = res.Status
	}
	if t.Kind == "example" {
		c.Examples++
	}

	if status == "pass" {
		c.Passed++
		if t.Kind == "example" {
			c.ExamplesPassed++
		}
		row.Outcome = "pass"
		if expected && exp.Class != "flaky" {
			c.Problems = append(c.Problems, fmt.Sprintf(
				"%s: expected to fail (%s) but it passes: remove the expectation",
				t.Name, exp.Reason))
		}
		return
	}

	row.Outcome = status
	if status == "none" {
		row.Outcome = "fail"
		row.Reason = "no result: the test binary ended before it ran"
	}
	if res != nil {
		row.Reason = strings.Join(firstLines(res.Lines, 3), " | ")
	}
	if !expected {
		if status == "skip" {
			c.Skipped++
		} else {
			c.Failed++
		}
		c.Problems = append(c.Problems, fmt.Sprintf(
			"%s: %s and not expected to (%s)", t.Name, row.Outcome, row.Reason))
		return
	}

	row.Class = exp.Class
	row.Reason = exp.Reason
	switch {
	case exp.Class == "divergence":
		c.Divergence++
	case status == "skip":
		c.Skipped++
	default:
		c.Failed++
	}
}

func firstLines(lines []string, n int) []string {
	var out []string
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if l == "" {
			continue
		}
		out = append(out, l)
		if len(out) == n {
			break
		}
	}
	return out
}

// isDivergenceReason reports whether the reason of a dropped function names a documented
// divergence.
func isDivergenceReason(reason string) bool {
	return strings.HasPrefix(reason, "divergence:")
}

// Table renders the counts of several packages as a Markdown table.
func Table(counts []*Count) string {
	var b strings.Builder
	b.WriteString("| package | functions | selected | passed | failed | skipped | dropped |" +
		" divergences | not taken | pass % | overall % |\n")
	b.WriteString("|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|\n")
	var sum Count
	for _, c := range counts {
		b.WriteString(tableRow(c.Package, c))
		sum.Total += c.Total
		sum.Selected += c.Selected
		sum.Passed += c.Passed
		sum.Failed += c.Failed
		sum.Skipped += c.Skipped
		sum.Dropped += c.Dropped
		sum.Divergence += c.Divergence
		sum.Excluded += c.Excluded
	}
	b.WriteString(tableRow("total", &sum))
	return b.String()
}

// tableRow renders the counts of one package, or of all of them, as a row of Table.
func tableRow(name string, c *Count) string {
	return fmt.Sprintf("| %s | %d | %d | %d | %d | %d | %d | %d | %d | %.1f | %.1f |\n",
		name, c.Total, c.Selected, c.Passed, c.Failed, c.Skipped, c.Dropped, c.Divergence,
		c.Excluded, c.Percent(), c.Overall())
}

// Detail renders the functions of one package that do not pass, with their reasons.
func Detail(c *Count) string {
	var b strings.Builder
	fmt.Fprintf(&b, "### %s\n\n", c.Package)
	rows := slices.Clone(c.Rows)
	sort.SliceStable(rows, func(i, j int) bool { return rows[i].Outcome < rows[j].Outcome })
	for _, r := range rows {
		if r.Outcome == "pass" {
			continue
		}
		class := r.Class
		if class != "" {
			class = " [" + class + "]"
		}
		fmt.Fprintf(&b, "- %s (%s, %s)%s: %s\n", r.Name, r.File, r.Outcome, class, r.Reason)
	}
	return b.String()
}
