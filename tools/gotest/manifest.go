package main

import (
	"bufio"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

// testDirs are the folders of Go's test/ that its runner reads (the dirs
// variable of cmd/internal/testdir), in its order.
var testDirs = []string{".", "ken", "chan", "interface", "internal/runtime/sys", "syntax", "dwarf", "fixedbugs", "codegen", "abi", "typeparam", "typeparam/mdempsky", "arenas"}

// listTests returns the .go files of the test folders of root (the folder
// that has test/), as paths relative to test/, sorted.
func listTests(root string) ([]string, error) {
	var out []string
	for _, d := range testDirs {
		ents, err := os.ReadDir(filepath.Join(root, "test", filepath.FromSlash(d)))
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		for _, e := range ents {
			n := e.Name()
			if e.IsDir() || strings.HasPrefix(n, ".") || !strings.HasSuffix(n, ".go") {
				continue
			}
			out = append(out, path.Join(d, n))
		}
	}
	sort.Strings(out)
	return out, nil
}

// hasAsm reports whether the folder test/<dir> has a .s file.
func hasAsmIn(root string) func(string) bool {
	return func(dir string) bool {
		ents, err := os.ReadDir(filepath.Join(root, "test", filepath.FromSlash(dir)))
		if err != nil {
			return false
		}
		for _, e := range ents {
			if strings.HasSuffix(e.Name(), ".s") {
				return true
			}
		}
		return false
	}
}

// dirSourcesIn returns the reader of the .go files of a .dir folder.
func dirSourcesIn(root string) func(string) []string {
	return func(dir string) []string {
		ents, err := os.ReadDir(filepath.Join(root, "test", filepath.FromSlash(dir)))
		if err != nil {
			return nil
		}
		var out []string
		for _, e := range ents {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") {
				continue
			}
			if b, err := os.ReadFile(filepath.Join(root, "test", filepath.FromSlash(dir), e.Name())); err == nil {
				out = append(out, string(b))
			}
		}
		return out
	}
}

// BuildManifest classifies every test of root.
func BuildManifest(root string, tgt Target, overrides map[string]string, needs map[string]string) ([]Test, error) {
	paths, err := listTests(root)
	if err != nil {
		return nil, err
	}
	o := ClassifyOptions{Target: tgt, Overrides: overrides, HasAsm: hasAsmIn(root), DirSources: dirSourcesIn(root)}
	var ts []Test
	for _, p := range paths {
		b, err := os.ReadFile(filepath.Join(root, "test", filepath.FromSlash(p)))
		if err != nil {
			return nil, err
		}
		t := Classify(p, string(b), o)
		if !t.Skipped() {
			t.Needs = needs[p]
		}
		ts = append(ts, t)
	}
	SortTests(ts)
	return ts, nil
}

// manifestHeader is the comment at the top of manifest.txt.
func manifestHeader(pin Pin) string {
	return fmt.Sprintf(`# Go's test/ folder (%s, SHA-256 %s), classified for gocpp by tools/gotest.
# Made by: go run ./tools/gotest manifest. Do not edit by hand: change overrides.txt,
# expect.txt or the classifier and make the file again.
#
# path <TAB> directive <TAB> status <TAB> needs
#   status: applicable | skip:<reason code> (docs/guides/conformance.md lists the codes)
#   needs:  - | needs <step>: the plan step that makes a failing test pass (expect.txt)
`, pin.Version, pin.SHA256)
}

// FormatManifest writes the manifest text.
func FormatManifest(pin Pin, ts []Test) string {
	var b strings.Builder
	b.WriteString(manifestHeader(pin))
	for _, t := range ts {
		needs := t.Needs
		if needs == "" {
			needs = "-"
		}
		fmt.Fprintf(&b, "%s\t%s\t%s\t%s\n", t.Path, t.Directive, t.Status, needs)
	}
	return b.String()
}

// ParseManifest reads manifest.txt.
func ParseManifest(text string) ([]Test, error) {
	var ts []Test
	seen := map[string]bool{}
	codes := reasonCodes()
	sc := bufio.NewScanner(strings.NewReader(text))
	sc.Buffer(nil, 1<<20)
	for n := 1; sc.Scan(); n++ {
		line := sc.Text()
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		f := strings.Split(line, "\t")
		if len(f) != 4 {
			return nil, fmt.Errorf("manifest line %d: want 4 tab-separated fields, got %d", n, len(f))
		}
		t := Test{Path: f[0], Directive: f[1], Status: f[2]}
		if f[3] != "-" {
			t.Needs = f[3]
			if !strings.HasPrefix(t.Needs, "needs ") {
				return nil, fmt.Errorf("manifest line %d: needs %q: want '-' or 'needs <step>'", n, f[3])
			}
		}
		if t.Status != "applicable" {
			c, ok := strings.CutPrefix(t.Status, "skip:")
			if !ok || !codes[c] {
				return nil, fmt.Errorf("manifest line %d: unknown status %q", n, t.Status)
			}
			if t.Needs != "" {
				return nil, fmt.Errorf("manifest line %d: a skipped test has no needs", n)
			}
		}
		if seen[t.Path] {
			return nil, fmt.Errorf("manifest line %d: %s twice", n, t.Path)
		}
		seen[t.Path] = true
		ts = append(ts, t)
	}
	return ts, sc.Err()
}

// ParseNeeds reads expect.txt: lines "<path> needs <step>  # why".
func ParseNeeds(text string) (map[string]string, error) {
	m := map[string]string{}
	for _, cl := range configLines(text) {
		f := strings.Fields(cl.Text)
		if len(f) != 3 || f[1] != "needs" {
			return nil, fmt.Errorf("expect.txt line %d: want <path> needs <step>, got %q", cl.N, cl.Text)
		}
		m[f[0]] = "needs " + f[2]
	}
	return m, nil
}
