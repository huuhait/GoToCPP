package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strings"
	"text/tabwriter"
)

// A ClusterRule maps failure cluster keys to the plan step that owns the
// missing feature (clusters.txt: "<step> <TAB> <regexp>", first match wins).
type ClusterRule struct {
	Owner string
	Rx    *regexp.Regexp
	Text  string
}

// ParseClusterRules reads clusters.txt.
func ParseClusterRules(text string) ([]ClusterRule, error) {
	var rules []ClusterRule
	for n, line := range strings.Split(text, "\n") {
		if i := strings.Index(line, "  #"); i >= 0 {
			line = line[:i]
		}
		if strings.HasPrefix(strings.TrimSpace(line), "#") || strings.TrimSpace(line) == "" {
			continue
		}
		owner, rx, ok := strings.Cut(line, "\t")
		if !ok || strings.TrimSpace(owner) == "" {
			return nil, fmt.Errorf("clusters.txt line %d: want <step> TAB <regexp>", n+1)
		}
		re, err := regexp.Compile(strings.TrimSpace(rx))
		if err != nil {
			return nil, fmt.Errorf("clusters.txt line %d: %v", n+1, err)
		}
		rules = append(rules, ClusterRule{Owner: strings.TrimSpace(owner), Rx: re, Text: strings.TrimSpace(rx)})
	}
	return rules, nil
}

// ownerOf returns the step that owns the cluster key ("" when no rule fits).
func ownerOf(rules []ClusterRule, key string) string {
	for _, r := range rules {
		if r.Rx.MatchString(key) {
			return r.Owner
		}
	}
	return ""
}

// A Row counts tests.
type Row struct {
	Name       string `json:"name"`
	Applicable int    `json:"applicable"`
	Run        int    `json:"run"`
	Pass       int    `json:"pass"`
}

// Rate is the pass rate over the applicable tests (tests that were not run
// count as not passing).
func (r Row) Rate() float64 {
	if r.Applicable == 0 {
		return 0
	}
	return 100 * float64(r.Pass) / float64(r.Applicable)
}

// A ClusterRow is one failure cluster.
type ClusterRow struct {
	Key     string   `json:"key"`
	Owner   string   `json:"owner"`
	Count   int      `json:"count"`
	Samples []string `json:"samples"`
}

// A Report is the counts of a run.
type Report struct {
	GoVersion  string         `json:"go_version"`
	Gocpp      string         `json:"gocpp"`
	Cxx        string         `json:"cxx"`
	Total      int            `json:"total"`
	Skipped    []Row          `json:"-"`
	SkipCounts map[string]int `json:"skipped"`
	Overall    Row            `json:"overall"`
	ByDirIve   []Row          `json:"by_directive"`
	ByDir      []Row          `json:"by_folder"`
	Clusters   []ClusterRow   `json:"clusters"`
	ByOwner    map[string]int `json:"waiting_by_owner"`
	Stale      []string       `json:"stale_needs"`
	NotRun     int            `json:"not_run"`
}

func folderOf(p string) string {
	if i := strings.LastIndex(p, "/"); i >= 0 {
		return p[:i]
	}
	return "."
}

// BuildReport counts the results against the manifest.
func BuildReport(ts []Test, rs *ResultSet, rules []ClusterRule) *Report {
	byPath := map[string]Result{}
	for _, r := range rs.Results {
		byPath[r.Path] = r
	}
	rep := &Report{GoVersion: rs.GoVersion, Gocpp: rs.Gocpp, Cxx: rs.Compiler, Total: len(ts),
		SkipCounts: map[string]int{}, ByOwner: map[string]int{}}
	dir := map[string]*Row{}
	fold := map[string]*Row{}
	clusters := map[string]*ClusterRow{}
	row := func(m map[string]*Row, k string) *Row {
		if m[k] == nil {
			m[k] = &Row{Name: k}
		}
		return m[k]
	}
	for _, t := range ts {
		if t.Skipped() {
			rep.SkipCounts[t.Status]++
			continue
		}
		res, ran := byPath[t.Path]
		ran = ran && res.Outcome != Skipped
		rows := []*Row{&rep.Overall, row(dir, t.Directive), row(fold, folderOf(t.Path))}
		for _, r := range rows {
			r.Applicable++
			if ran {
				r.Run++
			}
			if ran && res.Outcome == Pass {
				r.Pass++
			}
		}
		switch {
		case !ran:
			rep.NotRun++
		case res.Outcome == Pass:
			if t.Needs != "" {
				rep.Stale = append(rep.Stale, t.Path+" ("+t.Needs+")")
			}
		default:
			c := clusters[res.Cluster]
			if c == nil {
				c = &ClusterRow{Key: res.Cluster, Owner: ownerOf(rules, res.Cluster)}
				if c.Owner == "" && t.Needs != "" {
					c.Owner = strings.TrimPrefix(t.Needs, "needs ")
				}
				clusters[res.Cluster] = c
			}
			c.Count++
			if len(c.Samples) < 4 {
				c.Samples = append(c.Samples, t.Path)
			}
		}
	}
	rep.Overall.Name = "all applicable"
	for _, k := range sortedKeys(dir) {
		rep.ByDirIve = append(rep.ByDirIve, *dir[k])
	}
	for _, k := range sortedKeys(fold) {
		rep.ByDir = append(rep.ByDir, *fold[k])
	}
	for _, c := range clusters {
		rep.Clusters = append(rep.Clusters, *c)
		o := c.Owner
		if o == "" {
			o = "unowned"
		}
		rep.ByOwner[o] += c.Count
	}
	sort.Slice(rep.Clusters, func(i, j int) bool {
		a, b := rep.Clusters[i], rep.Clusters[j]
		if a.Count != b.Count {
			return a.Count > b.Count
		}
		return a.Key < b.Key
	})
	sort.Strings(rep.Stale)
	return rep
}

// runPct is the pass rate of the tests that were run (a partial run).
func runPct(r Row) string {
	if r.Run == 0 {
		return "0.0%"
	}
	return fmt.Sprintf("%.1f%%", 100*float64(r.Pass)/float64(r.Run))
}

func pct(r Row) string { return fmt.Sprintf("%.1f%%", r.Rate()) }

// WriteText prints the report for the terminal.
func (rep *Report) WriteText(w io.Writer, topClusters int) {
	fmt.Fprintf(w, "Go test/ %s: %d tests, %d applicable, %d run, %d pass (%s of the applicable, %s of the run)\n",
		rep.GoVersion, rep.Total, rep.Overall.Applicable, rep.Overall.Run, rep.Overall.Pass, pct(rep.Overall), runPct(rep.Overall))
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "\nBY DIRECTIVE\tapplicable\trun\tpass\trate")
	for _, r := range rep.ByDirIve {
		fmt.Fprintf(tw, "%s\t%d\t%d\t%d\t%s\n", r.Name, r.Applicable, r.Run, r.Pass, pct(r))
	}
	fmt.Fprintln(tw, "\nBY FOLDER\tapplicable\trun\tpass\trate")
	for _, r := range rep.ByDir {
		fmt.Fprintf(tw, "%s\t%d\t%d\t%d\t%s\n", r.Name, r.Applicable, r.Run, r.Pass, pct(r))
	}
	fmt.Fprintln(tw, "\nNOT APPLICABLE\tcount\t\t\t")
	for _, k := range sortedKeys(rep.SkipCounts) {
		fmt.Fprintf(tw, "%s\t%d\t\t\t\n", k, rep.SkipCounts[k])
	}
	tw.Flush()
	fmt.Fprintf(w, "\nFAILURE CLUSTERS (top %d of %d)\n", min(topClusters, len(rep.Clusters)), len(rep.Clusters))
	for i, c := range rep.Clusters {
		if i == topClusters {
			break
		}
		o := c.Owner
		if o == "" {
			o = "unowned"
		}
		fmt.Fprintf(w, "%5d  [%s] %s   e.g. %s\n", c.Count, o, c.Key, strings.Join(c.Samples[:min(2, len(c.Samples))], ", "))
	}
	if len(rep.Stale) > 0 {
		fmt.Fprintf(w, "\n%d tests pass although expect.txt says they wait for a step (remove them from expect.txt):\n", len(rep.Stale))
		for _, s := range rep.Stale {
			fmt.Fprintln(w, "  "+s)
		}
	}
}

// WriteMarkdown writes the committed snapshot docs/conformance/go-test.md.
// It has nothing that changes between two runs of the same code (no dates,
// no timings).
func (rep *Report) WriteMarkdown(w io.Writer, topClusters int) {
	fmt.Fprintf(w, "# Go `test/` conformance\n\n")
	fmt.Fprintf(w, "Generated by `go run ./tools/gotest report -md` (step 15.10); see [the guide](../guides/conformance.md). "+
		"Go's own tests, version %s, run through gocpp (C++ compiler: %s). The v1.0 goal is at least 90 %% of the applicable "+
		"run tests ([Phase 15](../../plans/2026-10-13-gocpp/phase-15-hardening-release/README.md)). "+
		"cgo counts as off (the tests that need it are \"not applicable\": build-tag); the run used gocpp's default build mode, and clang++ and -asan have not been run.\n\n", rep.GoVersion, rep.Cxx)
	fmt.Fprintf(w, "| | tests |\n|---|---:|\n| in the selected folders of `test/` | %d |\n| not applicable to gocpp | %d |\n| applicable | %d |\n| run in this snapshot | %d |\n| **pass** | **%d (%s of the applicable, %s of the run)** |\n\n",
		rep.Total, rep.Total-rep.Overall.Applicable, rep.Overall.Applicable, rep.Overall.Run, rep.Overall.Pass, pct(rep.Overall), runPct(rep.Overall))
	fmt.Fprintf(w, "A test that was not run counts as not passing.\n\n## By directive\n\n| directive | applicable | pass | rate |\n|---|---:|---:|---:|\n")
	for _, r := range rep.ByDirIve {
		fmt.Fprintf(w, "| `%s` | %d | %d | %s |\n", r.Name, r.Applicable, r.Pass, pct(r))
	}
	fmt.Fprintf(w, "\n## By folder\n\n| folder | applicable | pass | rate |\n|---|---:|---:|---:|\n")
	for _, r := range rep.ByDir {
		fmt.Fprintf(w, "| `%s` | %d | %d | %s |\n", r.Name, r.Applicable, r.Pass, pct(r))
	}
	fmt.Fprintf(w, "\n## Not applicable\n\n| reason | tests |\n|---|---:|\n")
	for _, k := range sortedKeys(rep.SkipCounts) {
		fmt.Fprintf(w, "| `%s` | %d |\n", k, rep.SkipCounts[k])
	}
	fmt.Fprintf(w, "\n## Failure clusters\n\nThe failing tests grouped by the first diagnostic. The owner is the plan step that "+
		"adds the missing feature (`tools/gotest/clusters.txt`); `unowned` clusters still need triage.\n\n| tests | owner | cluster | examples |\n|---:|---|---|---|\n")
	for i, c := range rep.Clusters {
		if i == topClusters {
			fmt.Fprintf(w, "| %d | | _%d more clusters_ | |\n", sumRest(rep.Clusters[i:]), len(rep.Clusters)-i)
			break
		}
		o := c.Owner
		if o == "" {
			o = "unowned"
		}
		ex := make([]string, 0, 2)
		for _, s := range c.Samples[:min(2, len(c.Samples))] {
			ex = append(ex, "`"+s+"`")
		}
		fmt.Fprintf(w, "| %d | %s | %s | %s |\n", c.Count, o, mdEscape(c.Key), strings.Join(ex, ", "))
	}
	fmt.Fprintf(w, "\n## Waiting for a step\n\n| owner | failing tests |\n|---|---:|\n")
	for _, k := range sortedKeys(rep.ByOwner) {
		fmt.Fprintf(w, "| %s | %d |\n", k, rep.ByOwner[k])
	}
}

func sumRest(cs []ClusterRow) int {
	n := 0
	for _, c := range cs {
		n += c.Count
	}
	return n
}

func mdEscape(s string) string {
	s = strings.ReplaceAll(s, "|", "\\|")
	return "`" + strings.ReplaceAll(s, "`", "'") + "`"
}

func cmdReport(args []string, stdout, stderr io.Writer) error {
	var c commonFlags
	fl := newFlagSet("report", stderr)
	c.register(fl)
	md := fl.String("md", "", "write the markdown snapshot here (docs/conformance/go-test.md)")
	js := fl.String("json", "", "write the report as JSON here")
	top := fl.Int("top", 25, "failure clusters to show")
	if err := fl.Parse(args); err != nil {
		return exitError{2, ""}
	}
	if fl.NArg() == 0 {
		return usageError("report: name one or more results files (go run ./tools/gotest run -out results.json)")
	}
	rs, err := ReadResults(fl.Args()...)
	if err != nil {
		return err
	}
	ts, err := c.loadManifest()
	if err != nil {
		return err
	}
	rules, err := c.loadRules()
	if err != nil {
		return err
	}
	rep := BuildReport(ts, rs, rules)
	rep.WriteText(stdout, *top)
	if *md != "" {
		var b strings.Builder
		rep.WriteMarkdown(&b, *top)
		if err := os.WriteFile(*md, []byte(b.String()), 0o644); err != nil {
			return err
		}
	}
	if *js != "" {
		b, err := json.MarshalIndent(rep, "", "  ")
		if err != nil {
			return err
		}
		return os.WriteFile(*js, append(b, '\n'), 0o644)
	}
	return nil
}

// loadRules reads clusters.txt.
func (c *commonFlags) loadRules() ([]ClusterRule, error) {
	t, err := c.readConfig("clusters.txt", true)
	if err != nil {
		return nil, err
	}
	return ParseClusterRules(t)
}
