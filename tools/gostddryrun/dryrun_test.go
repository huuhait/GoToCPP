package main

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestDryRunFakeTree(t *testing.T) {
	res, err := dryRun(&options{
		goroot:      fakeGOROOT,
		goos:        "linux",
		goarch:      "amd64",
		packages:    []string{"example"},
		packagesSet: true,
		closure:     true,
	})
	if err != nil {
		t.Fatal(err)
	}
	var paths []string
	for _, p := range res.Packages {
		paths = append(paths, p.Path)
	}
	if strings.Join(paths, ",") != "example,example/dep" {
		t.Errorf("packages = %v", paths)
	}
	c := res.Summary.Closure
	if c.Packages != 2 || c.Bodyless != 2 || c.Linknames != 5 {
		t.Errorf("closure counts = %+v", c)
	}
	if c.GocppDroppedFiles != 1 || c.GocppAddedFiles != 1 {
		t.Errorf("dropped/added = %d/%d, want 1/1", c.GocppDroppedFiles, c.GocppAddedFiles)
	}
	if c.RuntimeHookTargets != 0 || c.BodylessTargetsOutsideRuntime != 0 {
		t.Errorf("runtime counts = %+v", c)
	}
	if !res.ToolDefaults.Verified || res.Tags.Compiler != "gocpp" {
		t.Errorf("defaults = %+v", res.ToolDefaults)
	}

	// The JSON must not contain null slices: a later version's file is diffed
	// against this one.
	data, err := json.Marshal(res)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "null") {
		t.Errorf("the JSON has a null: %s", data)
	}
}

// TestMarkdownPrefersTheFreshRun checks that a stale JSON for the target of
// the current run, next to the README, does not override the fresh result.
func TestMarkdownPrefersTheFreshRun(t *testing.T) {
	res, err := dryRun(&options{
		goroot:      fakeGOROOT,
		goos:        "linux",
		goarch:      "amd64",
		packages:    []string{"example"},
		packagesSet: true,
		closure:     true,
	})
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	md := filepath.Join(dir, "README.md")
	stale := *res
	stale.Summary.Closure.Linknames = 999
	if err := writeJSON(filepath.Join(dir, "linux-amd64.json"), &stale); err != nil {
		t.Fatal(err)
	}
	siblings, err := readSiblings(md, res)
	if err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	if err := writeMarkdown(&b, res, siblings, nil); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(b.String(), "999") {
		t.Error("the stale JSON for the current target overrode the fresh run")
	}
	if !strings.Contains(b.String(), "| `//go:linkname` directives | 5 |") {
		t.Errorf("the README does not show the fresh linkname count:\n%s", b.String())
	}
}

func TestDryRunWithoutClosure(t *testing.T) {
	res, err := dryRun(&options{
		goroot:      fakeGOROOT,
		goos:        "linux",
		goarch:      "amd64",
		packages:    []string{"example"},
		packagesSet: true,
		closure:     false,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.ClosureWalked {
		t.Error("ClosureWalked = true")
	}
	if res.Summary.Closure.Packages != 1 {
		t.Errorf("closure packages = %d, want the named set of 1", res.Summary.Closure.Packages)
	}
}
