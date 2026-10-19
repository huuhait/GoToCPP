package bench

import (
	"fmt"
	"slices"
)

// A Budget is a row of the speed contract (Phase 11 README §2.1, ADR-015):
// the p95 of a scenario must stay under P95Ms.
type Budget struct {
	Scenario string
	P95Ms    float64
	What     string // the row of the contract, in words
	// Projects are the reference projects the budget applies to (nil: all).
	// The contract is for a 50k-line project; the smaller ones are held to
	// it too, since they have less to do.
	Projects []string
	// Known is a missed budget that is a known problem with a written
	// plan: the row is in the table, the gate leaves it out. It covers
	// only the projects in KnownIn (where the problem was recorded): a
	// smaller project that meets the budget is not "fixed".
	Known   string
	KnownIn []string
}

// knownFor returns the known problem of the budget for the project, or "".
func (b Budget) knownFor(project string) string {
	if slices.Contains(b.KnownIn, project) {
		return b.Known
	}
	return ""
}

// Budgets are the budgets of the language server that the benchmarks can
// measure: README §2.1, one row each. The memory budget is MemoryBudgetMiB.
var Budgets = []Budget{
	{ScInit, 100, "server start → initialize answered < 100 ms", nil, "", nil},
	{ScOpenWarm, 300, "open a project → first diagnostics < 300 ms (warm cache)", nil, "", nil},
	{ScOpenCold, 2000, "open a project → first diagnostics < 2 s (cold)", nil, "", nil},
	{ScKeystroke, 100, "keystroke → updated diagnostics < 100 ms", nil, "", nil},
	{ScKeystrokeDep, 100, "keystroke in a package that others import → updated diagnostics < 100 ms", nil, "", nil},
	{ScCompletionDot, 50, "completion < 50 ms", nil, "", nil},
	{ScCompletionEdit, 50, "completion right after the keystroke < 50 ms", nil, "", nil},
	{ScHover, 30, "hover < 30 ms", nil, "", nil},
	{ScDefinition, 30, "go to definition < 30 ms", nil, "", nil},
	{ScSignature, 30, "signature help < 30 ms", nil, "", nil},
	{ScFormat, 20, "format a 1,000-line file < 20 ms", nil, "", nil},
	{ScRefsPopular, 300, "find references across the workspace < 300 ms", nil, "", nil},
	{ScRefsLocal, 300, "find references across the workspace < 300 ms", nil, "", nil},
	{ScRefsFamily, 300, "find references of a method that an interface of the whole module shares < 300 ms", nil, "", nil},
	{ScRename, 300, "rename across the workspace < 300 ms", nil, "", nil},
}

// MemoryBudgetMiB is the memory budget: 400 MB for a 50k-line project.
const MemoryBudgetMiB = 400

// BudgetFor returns the budget of the scenario for the project, if it
// has one.
func BudgetFor(scenario, project string) (Budget, bool) {
	for _, b := range Budgets {
		if b.Scenario == scenario && (b.Projects == nil || slices.Contains(b.Projects, project)) {
			return b, true
		}
	}
	return Budget{}, false
}

// A Row is one budget row of one project: how the measured p95 compares.
type Row struct {
	Project, Scenario string
	Budget            float64 // ms
	P95               float64 // gocpp, ms
	GoplsP95          float64 // ms; 0 when gopls was not measured or has no figure
	Ratio             float64 // P95 / budget
	VsGopls           float64 // P95 / GoplsP95; 0 when there is none
	Known             string  // Budget.Known
	Samples           int     // the measured repetitions of gocpp (all runs)
	Fail              int     // the failed repetitions of gocpp
	Err               string
}

// Rows returns the budget rows of the results (gocpp's), with gopls's
// figures when results has them.
func Rows(results []*ProjectResult) []Row {
	var rows []Row
	for _, r := range results {
		if r.Target != "gocpp" {
			continue
		}
		var gopls *ProjectResult
		for _, g := range results {
			if g.Target == "gopls" && g.Project == r.Project {
				gopls = g
			}
		}
		for _, b := range Budgets {
			if _, ok := BudgetFor(b.Scenario, r.Project); !ok {
				continue
			}
			sc := r.Scenario(b.Scenario)
			if sc == nil {
				continue
			}
			row := Row{Project: r.Project, Scenario: b.Scenario, Budget: b.P95Ms, P95: sc.Agg.P95, Fail: sc.Fail, Err: sc.Err, Known: b.knownFor(r.Project), Samples: sc.Agg.N * sc.Agg.Runs}
			row.Ratio = row.P95 / row.Budget
			if gopls != nil {
				if g := gopls.Scenario(b.Scenario); g != nil && g.Agg.Runs > 0 {
					row.GoplsP95 = g.Agg.P95
					row.VsGopls = row.P95 / g.Agg.P95
				}
			}
			rows = append(rows, row)
		}
	}
	return rows
}

// Status says whether a row meets its budget, in words.
func (r Row) Status() string {
	switch {
	case r.Fail > 0 && r.P95 == 0:
		return "FAILED: " + oneLine(r.Err)
	case r.P95 <= r.Budget:
		return fmt.Sprintf("met (%.0f%% of the budget)", 100*r.Ratio)
	case r.Known != "":
		return fmt.Sprintf("MISSED (%.1fx the budget), known", r.Ratio)
	default:
		return fmt.Sprintf("MISSED (%.1fx the budget)", r.Ratio)
	}
}

// GateOptions are the tolerances of the CI gate: a budget is robust on a
// shared machine only with a safety factor.
type GateOptions struct {
	// Safety is the factor over the budget that a row may reach (3: a hover
	// budget of 30 ms fails at 90 ms).
	Safety float64
	// VsGopls is the factor over gopls's p95 of the same run that a row may
	// reach (2.5; the goal of the plan is 1.5, the gate adds the noise).
	VsGopls float64
	// MinMs is the smallest p95 that can fail: below it, a ratio to gopls is
	// timer noise.
	MinMs float64
}

// DefaultGate is the gate of go test.
var DefaultGate = GateOptions{Safety: 3, VsGopls: 2.5, MinMs: 5}

// maxFailRate is the share of the repetitions of a row that may fail (a
// timeout, an empty answer) before the gate says so: the p95 of the rest
// would hide them.
const maxFailRate = 0.05

// A Violation is a row that fails the gate.
type Violation struct {
	Row    Row
	Reason string
}

func (v Violation) String() string {
	return fmt.Sprintf("%s %s: %s", v.Row.Project, v.Row.Scenario, v.Reason)
}

// Gate returns the rows that fail: a row passes when its p95 is within
// Safety times the budget, or (when gopls was measured in the same run)
// within VsGopls times gopls's p95: a loaded machine slows both servers,
// so only a row that is slow in absolute terms and against gopls is a
// regression. A scenario that failed for every repetition (a timeout, an
// empty answer) fails too.
func Gate(rows []Row, opt GateOptions) []Violation {
	var out []Violation
	for _, r := range rows {
		switch {
		case r.Known != "":
			// An exemption must not outlive the fix.
			if r.P95 > 0 && r.P95 <= r.Budget && r.Fail == 0 {
				out = append(out, Violation{r, fmt.Sprintf("the known problem is fixed (p95 %.1f ms, budget %.0f ms): take Known off the budget", r.P95, r.Budget)})
			}
		case r.P95 == 0 && r.Fail > 0:
			out = append(out, Violation{r, "every repetition failed: " + r.Err})
		case float64(r.Fail) > maxFailRate*float64(r.Fail+r.Samples):
			out = append(out, Violation{r, fmt.Sprintf("%d of %d repetitions failed: %s", r.Fail, r.Fail+r.Samples, r.Err)})
		case r.P95 <= max(r.Budget*opt.Safety, opt.MinMs):
		case r.GoplsP95 > 0 && r.P95 <= max(r.GoplsP95*opt.VsGopls, opt.MinMs):
		default:
			why := fmt.Sprintf("p95 %.1f ms is over %.0fx the budget of %.0f ms", r.P95, opt.Safety, r.Budget)
			if r.GoplsP95 > 0 {
				why += fmt.Sprintf(" and over %.1fx gopls's %.1f ms", opt.VsGopls, r.GoplsP95)
			}
			out = append(out, Violation{r, why})
		}
	}
	return out
}
