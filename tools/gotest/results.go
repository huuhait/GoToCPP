package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
)

// Outcomes of a test run.
const (
	Pass    = "pass"
	Fail    = "fail"
	Timeout = "timeout" // counts as a failure
	Skipped = "skip"    // not applicable (manifest) or not run (-limit, shards)
)

// A Result is the outcome of one test.
type Result struct {
	Path      string `json:"path"`
	Directive string `json:"directive"`
	Outcome   string `json:"outcome"`
	Stage     string `json:"stage,omitempty"`   // where it failed: check, build, run, compare, go (real Go)
	Cluster   string `json:"cluster,omitempty"` // the failure cluster key (cluster.go)
	Detail    string `json:"detail,omitempty"`  // the first lines of the output
	Millis    int64  `json:"ms"`
}

// A ResultSet is the content of a results JSON file.
type ResultSet struct {
	GoVersion string   `json:"go_version"` // the pinned Go version of test/
	Gocpp     string   `json:"gocpp"`      // gocpp version output
	Compiler  string   `json:"cxx"`        // the C++ compiler
	Shard     string   `json:"shard,omitempty"`
	Results   []Result `json:"results"`
}

// ReadResults reads results files and merges them: a later file's result
// of a path replaces an earlier one (so that the shards of a campaign, or a
// re-run of the failures, combine).
func ReadResults(files ...string) (*ResultSet, error) {
	merged := &ResultSet{}
	idx := map[string]int{}
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			return nil, err
		}
		var rs ResultSet
		if err := json.Unmarshal(b, &rs); err != nil {
			return nil, fmt.Errorf("%s: %v", f, err)
		}
		if merged.GoVersion == "" {
			merged.GoVersion, merged.Gocpp, merged.Compiler = rs.GoVersion, rs.Gocpp, rs.Compiler
		}
		for _, r := range rs.Results {
			r.recluster()
			if i, ok := idx[r.Path]; ok {
				merged.Results[i] = r
			} else {
				idx[r.Path] = len(merged.Results)
				merged.Results = append(merged.Results, r)
			}
		}
	}
	sort.Slice(merged.Results, func(i, j int) bool { return merged.Results[i].Path < merged.Results[j].Path })
	return merged, nil
}

// recluster derives the cluster again from the stored detail, so that a
// better clusterOf applies to results made by an older run. A result without
// detail keeps its key.
func (r *Result) recluster() {
	if r.Outcome != Fail && r.Outcome != Timeout || r.Detail == "" || r.Stage == "" {
		return
	}
	stage := r.Stage
	if r.Outcome == Timeout {
		stage = "timeout-" + stage
	}
	r.Cluster = clusterOf(stage, r.Detail)
}

// WriteJSON writes the set, sorted by path, with one result per line so
// that diffs of two runs read well.
func (rs *ResultSet) WriteJSON(path string) error {
	sort.Slice(rs.Results, func(i, j int) bool { return rs.Results[i].Path < rs.Results[j].Path })
	var b strings.Builder
	b.WriteString("{\n")
	fmt.Fprintf(&b, "  \"go_version\": %q,\n  \"gocpp\": %q,\n  \"cxx\": %q,\n", rs.GoVersion, rs.Gocpp, rs.Compiler)
	if rs.Shard != "" {
		fmt.Fprintf(&b, "  \"shard\": %q,\n", rs.Shard)
	}
	b.WriteString("  \"results\": [\n")
	for i, r := range rs.Results {
		line, err := json.Marshal(r)
		if err != nil {
			return err
		}
		b.WriteString("    ")
		b.Write(line)
		if i < len(rs.Results)-1 {
			b.WriteByte(',')
		}
		b.WriteByte('\n')
	}
	b.WriteString("  ]\n}\n")
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

// Passing returns the paths that passed, sorted.
func (rs *ResultSet) Passing() []string {
	var out []string
	for _, r := range rs.Results {
		if r.Outcome == Pass {
			out = append(out, r.Path)
		}
	}
	sort.Strings(out)
	return out
}
