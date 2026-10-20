package wintest

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// A Status is how a test ended.
type Status string

const (
	Pass  Status = "pass"
	Fail  Status = "fail"
	Skip  Status = "skip"  // not run: it does not apply (Item.Skip says why)
	XFail Status = "xfail" // listed as an expected failure, and it failed
	XPass Status = "xpass" // listed as an expected failure, and it passed: the entry is stale (an error)
	Error Status = "error" // the machine or the connection failed before the test had a result (an error; a resume runs it again)
)

// An Entry is the result of one test in the results file.
type Entry struct {
	Status Status `json:"status"`
	Group  string `json:"group"`
	Phase  int    `json:"phase"`
	Key    string `json:"key,omitempty"`    // what the result depends on; a resume reuses a pass only if it is equal
	Kind   string `json:"kind,omitempty"`   // for a skip: the kind of the reason
	Reason string `json:"reason,omitempty"` // a skip's or expected failure's reason, or the first line of the problem
	Detail string `json:"detail,omitempty"` // the problems, shortened
	Ms     int64  `json:"ms,omitempty"`     // how long the program ran on the machine
}

// Results is the results file: one JSON document, written after every batch (atomically), so a run that is
// stopped can be resumed and the file is always whole.
type Results struct {
	Version    int               `json:"version"`
	Generation string            `json:"generation"` // the state of the checkout that made the results
	Target     string            `json:"target"`
	Tests      map[string]*Entry `json:"tests"`
}

const resultsVersion = 1

// NewResults returns an empty results document.
func NewResults(generation, target string) *Results {
	return &Results{Version: resultsVersion, Generation: generation, Target: target, Tests: make(map[string]*Entry)}
}

// LoadResults reads the results file; a file that does not exist gives an empty document with ok false.
func LoadResults(path string) (r *Results, ok bool, err error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	r = new(Results)
	if err := json.Unmarshal(data, r); err != nil {
		return nil, false, fmt.Errorf("%s: %v (delete it to start again)", path, err)
	}
	if r.Version != resultsVersion {
		return nil, false, fmt.Errorf("%s: version %d, want %d (delete it to start again)", path, r.Version, resultsVersion)
	}
	if r.Tests == nil {
		r.Tests = make(map[string]*Entry)
	}
	return r, true, nil
}

// Save writes the file whole or not at all.
func (r *Results) Save(path string) error {
	data, err := json.MarshalIndent(r, "", " ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(data, '\n'), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// A Report is the end of a run: the counts and the list of what is wrong.
type Report struct {
	Results    *Results
	Resumed    int    // tests whose result of an earlier run was kept
	Planned    int    // the tests the run was asked to do
	Incomplete string // why the run did not start or did not finish, or ""
}

// Count returns how many tests have the status.
func (r *Report) Count(s Status) int {
	n := 0
	for _, e := range r.Results.Tests {
		if e.Status == s {
			n++
		}
	}
	return n
}

// Bad returns the names of the tests that make the run fail (fail, xpass, error), sorted.
func (r *Report) Bad() []string {
	var bad []string
	for name, e := range r.Results.Tests {
		switch e.Status {
		case Fail, XPass, Error:
			bad = append(bad, name)
		}
	}
	sort.Strings(bad)
	return bad
}

// OK says whether the run found nothing wrong.
func (r *Report) OK() bool { return len(r.Bad()) == 0 && r.Incomplete == "" }

var statusOrder = []Status{Pass, Fail, XFail, XPass, Skip, Error}

func counts(entries []*Entry) string {
	var n [6]int
	for _, e := range entries {
		for i, s := range statusOrder {
			if e.Status == s {
				n[i]++
			}
		}
	}
	var parts []string
	for i, s := range statusOrder {
		parts = append(parts, fmt.Sprintf("%s %d", s, n[i]))
	}
	return strings.Join(parts, ", ")
}

// Summary is the stable text of a run (no times, sorted): the totals, the totals for each group
// (tests/run, tests/pkgs) and each phase, the skips by kind, then one line for each test that is wrong.
func (r *Report) Summary() string {
	var b strings.Builder
	var all []*Entry
	byGroup := map[string][]*Entry{}
	byPhase := map[int][]*Entry{}
	kinds := map[string]int{}
	for _, name := range sortedNames(r.Results.Tests) {
		e := r.Results.Tests[name]
		all = append(all, e)
		byGroup[e.Group] = append(byGroup[e.Group], e)
		byPhase[e.Phase] = append(byPhase[e.Phase], e)
		if e.Status == Skip {
			kinds[e.Kind]++
		}
	}
	if r.Incomplete != "" {
		fmt.Fprintf(&b, "RUN DID NOT COMPLETE: %s\n", r.Incomplete)
	}
	if r.Planned > len(all) {
		fmt.Fprintf(&b, "windows suite (%s): %d of %d tests have a result: %s\n", r.Results.Target, len(all), r.Planned, counts(all))
	} else {
		fmt.Fprintf(&b, "windows suite (%s): %d tests: %s\n", r.Results.Target, len(all), counts(all))
	}
	if r.Resumed > 0 {
		fmt.Fprintf(&b, "resumed: %d results of an earlier run were kept\n", r.Resumed)
	}
	var groups []string
	for g := range byGroup {
		groups = append(groups, g)
	}
	sort.Strings(groups)
	for _, g := range groups {
		fmt.Fprintf(&b, "  %-8s %s\n", g, counts(byGroup[g]))
	}
	var phases []int
	for p := range byPhase {
		phases = append(phases, p)
	}
	sort.Ints(phases)
	for _, p := range phases {
		fmt.Fprintf(&b, "  phase %02d %s\n", p, counts(byPhase[p]))
	}
	var ks []string
	for k := range kinds {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	for _, k := range ks {
		fmt.Fprintf(&b, "  skipped (%s): %d\n", k, kinds[k])
	}
	for _, name := range r.Bad() {
		e := r.Results.Tests[name]
		switch e.Status {
		case XPass:
			fmt.Fprintf(&b, "STALE %s: listed as an expected failure (%s), but it passes now: remove the entry\n", name, e.Reason)
		default:
			fmt.Fprintf(&b, "%s %s: %s\n", strings.ToUpper(string(e.Status)), name, e.Reason)
		}
	}
	return b.String()
}

func sortedNames(m map[string]*Entry) []string {
	names := make([]string, 0, len(m))
	for n := range m {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}
