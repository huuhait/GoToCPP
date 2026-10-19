package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// triage assigns owner steps to failing tests. For every failing test of a
// results file whose cluster matches a rule of clusters.txt it writes
// "<path> needs <step>" to expect.txt, then makes manifest.txt again so
// that its last column says what each failing test waits for. A test that
// passes is dropped from expect.txt: progress tracking is automatic.
func cmdTriage(args []string, stdout, stderr io.Writer) error {
	var c commonFlags
	fl := newFlagSet("triage", stderr)
	c.register(fl)
	dry := fl.Bool("n", false, "print what would change, write nothing")
	if err := fl.Parse(args); err != nil {
		return exitError{2, ""}
	}
	if fl.NArg() == 0 {
		return usageError("triage: name one or more results files")
	}
	rs, err := ReadResults(fl.Args()...)
	if err != nil {
		return err
	}
	rules, err := c.loadRules()
	if err != nil {
		return err
	}
	old, err := c.readConfig("expect.txt", true)
	if err != nil {
		return err
	}
	prev, err := ParseNeeds(old)
	if err != nil {
		return err
	}
	// tests that were not run keep their entries
	ran := map[string]bool{}
	needs := map[string]string{}
	unowned := map[string]int{}
	for _, r := range rs.Results {
		if r.Outcome == Skipped {
			continue
		}
		ran[r.Path] = true
		if r.Outcome == Pass {
			continue
		}
		if o := ownerOf(rules, r.Cluster); o != "" {
			needs[r.Path] = "needs " + o
		} else {
			unowned[r.Cluster]++
		}
	}
	for p, n := range prev {
		if !ran[p] {
			needs[p] = n
		}
	}
	var b strings.Builder
	b.WriteString("# The failing tests of Go's test/ folder and the plan step that makes each pass.\n" +
		"# Made by: go run ./tools/gotest triage results.json (from clusters.txt). A test that passes\n" +
		"# leaves this file by itself at the next triage; the manifest's last column shows it.\n#\n# path needs step\n")
	paths := make([]string, 0, len(needs))
	for p := range needs {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	for _, p := range paths {
		fmt.Fprintf(&b, "%s %s\n", p, needs[p])
	}
	fmt.Fprintf(stdout, "%d failing tests have an owner, %d clusters have none:\n", len(paths), len(unowned))
	keys := make([]string, 0, len(unowned))
	for k := range unowned {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		return unowned[keys[i]] > unowned[keys[j]] || unowned[keys[i]] == unowned[keys[j]] && keys[i] < keys[j]
	})
	for i, k := range keys {
		if i == 40 {
			fmt.Fprintf(stdout, "  ... and %d more\n", len(keys)-i)
			break
		}
		fmt.Fprintf(stdout, "%5d  %s\n", unowned[k], k)
	}
	if *dry {
		return nil
	}
	dir, err := c.configDir()
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "expect.txt"), []byte(b.String()), 0o644); err != nil {
		return err
	}
	return cmdManifest([]string{"-config", dir, "-cache", c.cache, "-root", c.root}, io.Discard, stderr)
}
