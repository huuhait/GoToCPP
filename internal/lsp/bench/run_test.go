package bench

import (
	"bytes"
	"context"
	"encoding/json"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/huuhait/gocpp/internal/lsp/protocol"
)

func TestTypingAnswerValidation(t *testing.T) {
	src := "Open 😀\nNext\n"
	for _, tc := range []struct {
		kind, raw string
		ok        bool
	}{
		{"hover", `null`, false},
		{"hover", `{"contents":{"kind":"plaintext","value":"func Open()"},"range":{"start":{"line":0,"character":0},"end":{"line":0,"character":4}}}`, true},
		{"hints", `[]`, false},
		{"hints", `[{"position":{"line":0,"character":99}}]`, false},
		{"hints", `[{"position":{"line":0,"character":4}},{"position":{"line":0,"character":1}}]`, false},
		{"hints", `[{"position":{"line":0,"character":4}}]`, true},
		{"tokens", `{"data":[0,0,4,8,0]}`, true},
		{"tokens", `{"data":[]}`, false},
		{"tokens", `{"data":[0,0,4,8]}`, false},
		{"tokens", `{"data":[0,0,4,8,0,0,2,2,8,0]}`, false},
		{"tokens", `{"data":[0,0,4,8,0,0,4294967295,2,8,0]}`, false},
		{"tokens", `{"data":[0,0,4,8,0,0,5,1,8,0]}`, false}, // end in the UTF-16 emoji, with Open retained
		{"tokens", `{"data":[0,0,4,8,0,0,6,1,8,0]}`, false}, // start in its surrogate pair
		{"tokens", `{"data":[0,0,4,8,0,0,5,2,8,0]}`, true},  // a complete emoji, two UTF-16 code units
		{"tokens", `{"data":[0,0,4,8,0,0,5,4294967295,8,0]}`, false},
		{"tokens", `{"data":[0,0,4,8,0,1,0,1,8,0,4294967295,0,1,8,0]}`, false},
		{"hints", `[{"position":{"line":0,"character":6}}]`, false},
		{"hover", `{"contents":{"kind":"plaintext","value":"func Open()"},"range":{"start":{"line":0,"character":6},"end":{"line":0,"character":7}}}`, false},
	} {
		if err := validateTypingAnswer(tc.kind, []byte(tc.raw), src); (err == nil) != tc.ok {
			t.Errorf("%s %s: %v, want valid=%v", tc.kind, tc.raw, err, tc.ok)
		}
	}
}

func TestFreshEditTexts(t *testing.T) {
	ctx := context.Background()
	proj, cfg := Generate(Small), fakeConfig(t)
	tree := cfg.WorkDir + "/tree"
	if err := proj.Write(tree, ".gp"); err != nil {
		t.Fatal(err)
	}
	s, err := startSession(ctx, cfg, proj, newFake(nil).target("gocpp", ".gp"), tree, cfg.WorkDir+"/cache")
	if err != nil {
		t.Fatal(err)
	}
	defer s.P.Kill()
	if _, err := s.openAndWait(ctx, proj.Hot); err != nil {
		t.Fatal(err)
	}
	sc := keystrokeNew(ScKeystrokeNew, proj.Hot, 10, 2)
	if err := sc.prep(ctx, s); err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{s.Doc(proj.Hot).Text: true}
	for i := 0; i < 12; i++ {
		if _, _, err := sc.run(ctx, s, i); err != nil {
			t.Fatal(err)
		}
		text := s.Doc(proj.Hot).Text
		if seen[text] {
			t.Fatalf("edit %d reuses text already seen by the server", i)
		}
		seen[text] = true
	}
}

// TestFreshCompletionTexts: keep the numbered fresh prefix and the correct generic cursor for both r. and #.
// The fake observes the actual request and text before cleanup, rather than reusing the scenario's offset formula.
func TestFreshCompletionTexts(t *testing.T) {
	for _, tc := range []struct {
		name, text, before string
		delta              int
	}{
		{"ordinary", "\tr.\n", "\tr.", 3},
		{"macro", "\t#\n", "\t#", 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			proj := Generate(Small)
			cfg := fakeConfig(t)
			tree := cfg.WorkDir + "/tree"
			if err := proj.Write(tree, ".gp"); err != nil {
				t.Fatal(err)
			}
			type observed struct {
				text string
				off  int
				err  error
			}
			requests := make(chan observed, 3)
			f := newFake(nil)
			f.observe = func(req *protocol.Request) {
				if req.Method != "textDocument/completion" {
					return
				}
				var p protocol.CompletionParams
				err := json.Unmarshal(req.Params, &p)
				f.mu.Lock()
				text := f.texts[p.TextDocument.URI]
				f.mu.Unlock()
				requests <- observed{text, protocol.NewMapper(text).Offset(p.Position), err}
			}
			s, err := startSession(ctx, cfg, proj, f.target("gocpp", ".gp"), tree, cfg.WorkDir+"/cache")
			if err != nil {
				t.Fatal(err)
			}
			defer s.P.Kill()
			if _, err := s.openAndWait(ctx, proj.Hot); err != nil {
				t.Fatal(err)
			}
			sc := completionAt(tc.name, proj.Hot, "\tr.mu.Lock()\n", tc.text, tc.delta, 3)
			if err := sc.prep(ctx, s); err != nil {
				t.Fatal(err)
			}
			seen := make(map[string]bool)
			for i := 0; i < 3; i++ {
				if _, _, err := sc.run(ctx, s, i); err != nil {
					t.Fatal(err)
				}
				select {
				case request := <-requests:
					if request.err != nil || !strings.HasSuffix(request.text[:request.off], tc.before) {
						t.Fatalf("cursor is not after %q: offset %d (%v)", tc.before, request.off, request.err)
					}
					if seen[request.text] {
						t.Fatal("completion edit repeated a previous text")
					}
					seen[request.text] = true
				default:
					t.Fatal("no actual completion request was observed")
				}
			}
		})
	}
}

func TestFreshDiagnostics(t *testing.T) {
	for _, tc := range []struct {
		name         string
		version      int32
		message      string
		broken, want bool
	}{
		{"new error", 4, "undefined: undefinedName2", true, true},
		{"fixed", 4, "", false, true},
		{"stale version", 3, "undefined: undefinedName2", true, false},
		{"unversioned", 0, "", false, false},
		{"previous error on fixed edit", 4, "undefined: undefinedName1", false, false},
		{"wrong error", 4, "undefined: undefinedName1", true, false},
		{"prefix is not the identifier", 4, "undefined: undefinedName20", true, false},
		{"missing error", 4, "", true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := &Publish{Version: tc.version}
			if tc.message != "" {
				p.Diags = []Diag{{Severity: 1, Message: tc.message}}
			}
			if got := freshDiagnostics(p, 4, "undefinedName2", tc.broken); got != tc.want {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func fakeConfig(t *testing.T) Config {
	return Config{Runs: 1, Scale: 0.1, Timeout: 10 * time.Second, ColdMax: 10 * time.Second, WorkDir: t.TempDir()}
}

var allScenarios = []string{ScInit, ScOpenCold, ScOpenWarm, ScKeystroke, ScKeystrokeDep, ScKeystrokeNew, ScKeystrokeDepNew, ScCompletionDot, ScCompletionEdit,
	ScHover, ScDefinition, ScSignature, ScRefsPopular, ScRefsLocal, ScRefsFamily, ScRename,
	ScRenameType, ScRenameMethod, ScRenameField, ScRenameLocal, ScWorkspaceSymbol, ScDocumentSymbol,
	ScSemanticTokens, ScFormat, ScSwitchFile, ScAPIChange, ScAPIFix}

// The scenarios run to the end against a server that answers at once: every
// scenario has samples in every run, none failed, and the requests the
// scenarios promise were made (counted by the server, not by the harness).
func TestRunProjectFake(t *testing.T) {
	f := newFake(nil)
	cfg := fakeConfig(t)
	cfg.Runs = 2
	res, err := RunProject(context.Background(), cfg, Generate(Small), f.target("gocpp", ".gp"))
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range allScenarios {
		sc := res.Scenario(name)
		if sc == nil {
			t.Errorf("no scenario %s", name)
			continue
		}
		if sc.Fail != 0 || sc.Err != "" {
			t.Errorf("%s: %d failed: %s", name, sc.Fail, sc.Err)
		}
		if sc.Agg.Runs != cfg.Runs || sc.Agg.N == 0 || sc.Agg.P95 <= 0 || sc.Agg.Max < sc.Agg.P95 || sc.Agg.P95 < sc.Agg.P50 {
			t.Errorf("%s: %+v", name, sc.Agg)
		}
	}
	if len(res.Scenarios) != len(allScenarios) {
		t.Errorf("%d scenarios, want %d", len(res.Scenarios), len(allScenarios))
	}
	// 0.1 of 60 repetitions is 6, plus 2 warm-up ones, per run (and one
	// request per repetition).
	if n := f.count("textDocument/hover"); n != 2*(6+2) {
		t.Errorf("%d hover requests, want %d", n, 2*(6+2))
	}
	// A rename is two requests; 5 x 0.1 rounds up to the minimum of 2, plus 1 warm-up. The rename of a
	// function and of the four other forms (type, method, field, local variable) are five scenarios.
	const renames = 5
	if n := f.count("textDocument/rename"); n != renames*2*(2+1) {
		t.Errorf("%d rename requests, want %d", n, renames*2*(2+1))
	}
	if n := f.count("textDocument/prepareRename"); n != renames*2*(2+1) {
		t.Errorf("%d prepareRename requests, want %d", n, renames*2*(2+1))
	}
	if f.count("textDocument/didSave") == 0 || f.count("textDocument/didClose") == 0 {
		t.Errorf("no save or close: %v", f.calls)
	}
	if f.count("initialize") != 2*cfg.Runs || f.count("shutdown") != 2*cfg.Runs {
		t.Errorf("%d initialize, %d shutdown, want %d each", f.count("initialize"), f.count("shutdown"), 2*cfg.Runs)
	}
	// The stats of a gocpp server are kept.
	if !strings.Contains(string(res.Stats), "fake") {
		t.Errorf("stats %s", res.Stats)
	}
	// Every repetition put its edit back: the documents of the last
	// session are the ones on disk.
	f.mu.Lock()
	defer f.mu.Unlock()
	for u, text := range f.texts {
		if strings.Contains(text, "sumx") || strings.Contains(text, ", n int") {
			t.Errorf("%s was left edited", u)
		}
	}
}

// A project with C++ headers: the open of a file that imports them and the
// typo file have their rows, cold and warm; gopls (another target name) has
// none of them.
func TestRunProjectCpp(t *testing.T) {
	f := newFake(nil)
	cfg := fakeConfig(t)
	res, err := RunProject(context.Background(), cfg, Generate(Cpp), f.target("gocpp", ".gp"))
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{ScCppOpenCold, ScCppOpenWarm, ScTypoFirstCold, ScTypoFirstWarm, ScTypoErrorCold, ScTypoErrorWarm} {
		if sc := res.Scenario(name); sc == nil || sc.Agg.Runs != 1 {
			t.Errorf("%s: %+v", name, sc)
		}
	}
	if f.count("textDocument/didOpen") < 6 {
		t.Errorf("%d opens", f.count("textDocument/didOpen"))
	}
	g, err := RunProject(context.Background(), cfg, Generate(Cpp), newFake(nil).target("gopls", ".go"))
	if err != nil {
		t.Fatal(err)
	}
	if g.Scenario(ScCppOpenCold) != nil || g.Scenario(ScTypoFirstCold) != nil {
		t.Errorf("gopls got the C++ rows")
	}
}

// A hover that takes 150 ms trips the gate (budget 10 ms, safety 3: 30 ms),
// and nothing else does; a fast server trips nothing. The gate is the
// mutation test of the whole chain: the harness measures what the server
// does, the rows hold it against the budget.
func TestGateTripsOnSlowRequest(t *testing.T) {
	run := func(delay map[string]time.Duration) []Violation {
		cfg := fakeConfig(t)
		res, err := RunProject(context.Background(), cfg, Generate(Small), newFake(delay).target("gocpp", ".gp"))
		if err != nil {
			t.Fatal(err)
		}
		// A fast fake server fixes the known problem of references-family:
		// the gate says so, which is not what these tests are about.
		var bad []Violation
		for _, v := range Gate(Rows([]*ProjectResult{res}), DefaultGate) {
			if v.Row.Known == "" {
				bad = append(bad, v)
			}
		}
		return bad
	}
	if bad := run(nil); len(bad) != 0 {
		t.Errorf("a fast server fails the gate: %v", bad)
	}
	bad := run(map[string]time.Duration{"textDocument/hover": 150 * time.Millisecond})
	if len(bad) != 1 || bad[0].Row.Scenario != ScHover {
		t.Fatalf("a slow hover: violations %v", bad)
	}
	if !strings.Contains(bad[0].String(), "hover") || bad[0].Row.P95 < 150 {
		t.Errorf("violation: %v (p95 %v)", bad[0], bad[0].Row.P95)
	}
	// Slow diagnostics trip the keystroke rows.
	bad = run(map[string]time.Duration{"diagnostics": 400 * time.Millisecond})
	names := map[string]bool{}
	for _, v := range bad {
		names[v.Row.Scenario] = true
	}
	if !names[ScKeystroke] || !names[ScKeystrokeDep] {
		t.Errorf("slow diagnostics: violations %v", bad)
	}
}

// With gopls measured in the same run, a slow row passes when gopls is
// slow too (a loaded machine), and fails when only gocpp is.
func TestGateVsGopls(t *testing.T) {
	g := func(p95 float64) *ProjectResult {
		return &ProjectResult{Project: "small", Target: "gocpp", Scenarios: []ScenarioResult{{Name: ScHover, Agg: Aggregate{Runs: 1, P95: p95}}}}
	}
	o := func(p95 float64) *ProjectResult {
		return &ProjectResult{Project: "small", Target: "gopls", Scenarios: []ScenarioResult{{Name: ScHover, Agg: Aggregate{Runs: 1, P95: p95}}}}
	}
	for _, c := range []struct {
		name         string
		gocpp, gopls float64
		fail         bool
	}{
		{"within the budget", 8, 0, false},
		{"over the budget, within the safety factor", 25, 0, false},
		{"over the safety factor, no gopls", 40, 0, true},
		{"over the safety factor, gopls as slow", 300, 200, false},
		{"over the safety factor, gopls fast", 100, 10, true},
		{"gopls at the edge of the ratio", 250, 100, false},
		{"just over the ratio", 251, 100, true},
	} {
		results := []*ProjectResult{g(c.gocpp)}
		if c.gopls > 0 {
			results = append(results, o(c.gopls))
		}
		bad := Gate(Rows(results), DefaultGate)
		if (len(bad) > 0) != c.fail {
			t.Errorf("%s: gocpp %v, gopls %v: violations %v, want fail=%v", c.name, c.gocpp, c.gopls, bad, c.fail)
		}
	}
	// A scenario that failed every repetition fails the gate.
	bad := Gate([]Row{{Project: "small", Scenario: ScHover, Budget: 10, Fail: 3, Err: "timeout"}}, DefaultGate)
	if len(bad) != 1 || !strings.Contains(bad[0].String(), "timeout") {
		t.Errorf("a failed scenario: %v", bad)
	}
	// A project that has no budget for a scenario has no row.
	if _, ok := BudgetFor(ScHover, "small"); !ok {
		t.Error("no hover budget")
	}
	if _, ok := BudgetFor(ScOpenCold, "small"); !ok {
		t.Error("no open budget")
	}
	if _, ok := BudgetFor(ScRename+"x", "small"); ok {
		t.Error("a budget for an unknown scenario")
	}
}

// A known problem is left out of the gate while it is a problem, and the gate
// fails when it is fixed (the exemption must not outlive it); a row with
// failed repetitions fails when more than 5% of them failed.
func TestGateKnownAndFailures(t *testing.T) {
	// No budget has a known problem today: give one, for the test.
	saved := slices.Clone(Budgets)
	t.Cleanup(func() { Budgets = saved })
	for i := range Budgets {
		if Budgets[i].Scenario == ScRefsFamily {
			Budgets[i].Known, Budgets[i].KnownIn = "a plan", []string{"medium", "large"}
		}
	}
	known := Row{Project: "medium", Scenario: ScRefsFamily, Budget: 20, Known: "a plan", Samples: 10}
	known.P95 = 700
	if bad := Gate([]Row{known}, DefaultGate); len(bad) != 0 {
		t.Errorf("a known problem over its budget fails the gate: %v", bad)
	}
	known.P95 = 15
	if bad := Gate([]Row{known}, DefaultGate); len(bad) != 1 || !strings.Contains(bad[0].String(), "known problem is fixed") {
		t.Errorf("a known problem that meets its budget: %v", bad)
	}
	// Through Rows: the exemption covers the projects where the problem
	// was recorded (medium, large), not the small one, which meets the budget.
	res := func(project string, p95 float64) *ProjectResult {
		return &ProjectResult{Project: project, Target: "gocpp", Scenarios: []ScenarioResult{{Name: ScRefsFamily, Agg: Aggregate{Runs: 5, N: 8, P95: p95}}}}
	}
	if bad := Gate(Rows([]*ProjectResult{res("small", 15), res("medium", 700), res("large", 900)}), DefaultGate); len(bad) != 0 {
		t.Errorf("small meets the budget, medium and large are known: %v", bad)
	}
	if bad := Gate(Rows([]*ProjectResult{res("small", 15), res("medium", 12)}), DefaultGate); len(bad) != 1 || bad[0].Row.Project != "medium" {
		t.Errorf("medium is fixed, small is not a known row: %v", bad)
	}
	if bad := Gate(Rows([]*ProjectResult{res("small", 2000)}), DefaultGate); len(bad) != 1 || bad[0].Row.Project != "small" {
		t.Errorf("small is slow and not exempt: %v", bad)
	}
	row := Row{Project: "small", Scenario: ScHover, Budget: 10, P95: 1, Samples: 95, Fail: 5, Err: "empty"}
	if bad := Gate([]Row{row}, DefaultGate); len(bad) != 0 {
		t.Errorf("5 of 100 failed: %v", bad)
	}
	row.Fail = 6
	if bad := Gate([]Row{row}, DefaultGate); len(bad) != 1 || !strings.Contains(bad[0].String(), "6 of 101 repetitions failed") {
		t.Errorf("6 of 101 failed: %v", bad)
	}
}

// The tables: gocpp next to gopls, the ratio of the p95s, the budget and
// its status; a row only one server has shows n/a for the other.
func TestMarkdown(t *testing.T) {
	agg := func(first, p50, p95, max float64) Aggregate {
		return Aggregate{Runs: 5, N: 20, First: first, P50: p50, P95: p95, Max: max}
	}
	rep := &Report{
		Machine: Machine{CPU: "Test CPU", Cores: 8, MemGiB: 16, Kernel: "Linux 6", Go: "go1.25", Gocpp: "gocpp dev", Gopls: "gopls v0.21", Date: "2026-10-18 12:00 UTC", LoadAvg: [3]float64{0.5, 0.4, 0.3}, LoadEnd: [3]float64{0.6, 0.5, 0.4}},
		Results: []*ProjectResult{
			{Project: "small", Target: "gocpp", Files: 21, Lines: 2722, RSS: 50, PeakRSS: 60, Scenarios: []ScenarioResult{
				{Name: ScHover, Agg: agg(4, 0.5, 1.25, 3)},
				{Name: ScKeystroke, Agg: agg(90, 60, 130, 140)},
				{Name: ScOpenCold, Agg: agg(900, 900, 1500, 1500)},
				{Name: ScCppOpenCold, Agg: agg(10, 10, 10, 10)},
				{Name: ScRename, Fail: 5, Err: "no answer\nin 60s"},
			}},
			{Project: "small", Target: "gopls", Files: 18, Lines: 2000, RSS: 200, PeakRSS: 250, Scenarios: []ScenarioResult{
				{Name: ScHover, Agg: agg(8, 1, 2.5, 6)},
				{Name: ScKeystroke, Agg: agg(100, 70, 100, 110)},
				{Name: ScOpenCold, Agg: agg(2000, 2000, 4000, 4000)},
			}},
		},
	}
	var b bytes.Buffer
	rep.Markdown(&b)
	out := b.String()
	for _, want := range []string{
		"# Language server benchmarks",
		"| cpu | Test CPU, 8 logical cores, 16 GiB |",
		"| gopls | gopls v0.21 |",
		"### small: 21 files, 2,722 lines",
		"| hover | 4.00 | 0.50 | 1.25 | 3.00 | 8.00 | 1.00 | 2.50 | 6.00 | 0.50 | 10 | met (12% of the budget) |",
		"| keystroke | 90.0 | 60.0 | 130 | 140 | 100 | 70.0 | 100 | 110 | 1.30 | 20 | MISSED (6.5x the budget) |",
		"| open-cold | 900 | 900 | 1500 | 1500 | 2000 | 2000 | 4000 | 4000 | 0.38 | 600 | MISSED (2.5x the budget) |",
		"| open-cpp-cold | 10.0 | 10.0 | 10.0 | 10.0 | n/a | n/a | n/a | n/a | - | - |  |",
		"| rename | failed | - | - | - | n/a | n/a | n/a | n/a | - | 60 | FAILED: no answer in 60s (5 failed) |",
		"Memory at the end of the session (resident set, MiB): gocpp 50 (peak 60), gopls 200 (peak 250).",
		"- gocpp on small: 5 repetitions of rename failed; the first error: `no answer in 60s`",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("the markdown has no %q:\n%s", want, out)
		}
	}
	// And the JSON round trip.
	var j bytes.Buffer
	if err := rep.WriteJSON(&j); err != nil {
		t.Fatal(err)
	}
	back, err := ReadJSON(&j)
	if err != nil || len(back.Results) != 2 || back.Results[0].Scenario(ScHover).Agg.P95 != 1.25 || back.Machine.CPU != "Test CPU" {
		t.Errorf("JSON round trip: %v %+v", err, back)
	}
}

func TestThousandsAndMs(t *testing.T) {
	for in, want := range map[int]string{0: "0", 999: "999", 1000: "1,000", 2722: "2,722", 1234567: "1,234,567"} {
		if got := thousands(in); got != want {
			t.Errorf("thousands(%d) = %s, want %s", in, got, want)
		}
	}
	for in, want := range map[float64]string{0: "0", 0.426: "0.43", 3.414: "3.41", 56.24: "56.2", 412.6: "413"} {
		if got := FormatMs(in); got != want {
			t.Errorf("FormatMs(%v) = %s, want %s", in, got, want)
		}
	}
}
