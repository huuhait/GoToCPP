package tests

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// The interop docs (docs/spec/interop.md and docs/spec/interop-cookbook.md) show programs that
// must not rot. A fenced block that follows a marker is a copy of a test file:
//
//	<!-- test: tests/interop/cookbook_c_call.gp -->
//	```go
//	...
//	```
//
// is compared with the file without its harness header (the // phase: line and the other
// header lines of tests/README.md, and the blank line after them) and without the trailing
// // Output: block. `<!-- output: PATH -->` before a block compares it with the // Output: block
// of the file (the lines without their "// "). Any other file (a header, a .cpp, a gocpp.mod) is
// compared whole. A ```go block with no such marker must carry `<!-- illustration: reason -->`
// (a fragment, or code that can't run here), so that a complete program is never left unchecked.
// An output marker needs the test marker of its file before it, and a shown program that prints
// something needs its output shown. So the programs of the docs are tests of tests/interop, which run with both
// compilers and under the sanitizers.
var docFiles = []string{"docs/spec/interop.md", "docs/spec/interop-cookbook.md"}

var (
	docMarkerRx    = regexp.MustCompile(`^<!-- (test|output): (\S+) -->$`)
	illustrationRx = regexp.MustCompile(`^<!-- illustration: \S.* -->$`)
	docLinkRx      = regexp.MustCompile(`\]\(([^)\s]+)\)`)
	codeSpanRx     = regexp.MustCompile("`[^`]*`")
	headerRx       = regexp.MustCompile(`^// (phase|pkg-config|ExitCode|display|Stderr|lsan|divergence):`)
)

// docBody returns what a doc block shows of the test file src: without the harness header and
// without the // Output: block (kind "test"), or the Output lines (kind "output").
func docBody(kind, path string, src string) string {
	if !strings.HasSuffix(path, ".gp") {
		return strings.TrimRight(src, "\n")
	}
	lines := strings.Split(strings.TrimRight(src, "\n"), "\n")
	out := -1
	for i, l := range lines {
		if l == "// Output:" {
			out = i
		}
	}
	if kind == "output" {
		var b []string
		if out >= 0 {
			for _, l := range lines[out+1:] {
				l = strings.TrimPrefix(l, "//")
				b = append(b, strings.TrimPrefix(l, " "))
			}
		}
		return strings.Join(b, "\n")
	}
	if out >= 0 {
		lines = lines[:out]
	}
	start := 0
	for start < len(lines) && headerRx.MatchString(lines[start]) {
		start++
	}
	for start < len(lines) && strings.TrimSpace(lines[start]) == "" {
		start++
	}
	return strings.TrimRight(strings.Join(lines[start:], "\n"), "\n")
}

// docBlocks returns the marked blocks of a doc: the marker's kind and path, the text of the
// fenced block after it, and the line of the marker.
type docBlock struct {
	kind, path, text string
	line             int
}

func docBlocks(t *testing.T, name, src string) []docBlock {
	var blocks []docBlock
	lines := strings.Split(src, "\n")
	for i := 0; i < len(lines); i++ {
		m := docMarkerRx.FindStringSubmatch(lines[i])
		if m == nil {
			if strings.HasPrefix(lines[i], "```go") {
				// A go block with no marker before it: a test file or an illustration (a fragment, or
				// a program that can't run here) must be said.
				k := i - 1
				for k >= 0 && strings.TrimSpace(lines[k]) == "" {
					k--
				}
				if k < 0 || !illustrationRx.MatchString(lines[k]) {
					t.Errorf("%s:%d: a go block with no <!-- test: PATH --> marker (a copy of a test) and no <!-- illustration: reason --> marker before it", name, i+1)
				}
				for i++; i < len(lines) && lines[i] != "```"; i++ {
				}
			}
			continue
		}
		j := i + 1
		for j < len(lines) && strings.TrimSpace(lines[j]) == "" {
			j++
		}
		if j >= len(lines) || !strings.HasPrefix(lines[j], "```") {
			t.Errorf("%s:%d: the marker %q is not followed by a fenced block", name, i+1, lines[i])
			continue
		}
		fence := lines[j]
		var body []string
		k := j + 1
		for k < len(lines) && lines[k] != "```" {
			body = append(body, lines[k])
			k++
		}
		if k >= len(lines) {
			t.Errorf("%s:%d: the block after the marker is not closed", name, j+1)
			continue
		}
		_ = fence
		blocks = append(blocks, docBlock{kind: m[1], path: m[2], text: strings.Join(body, "\n"), line: i + 1})
		i = k
	}
	return blocks
}

// TestDocSnippets checks that every marked block of the interop docs equals its test file, that
// every cookbook test file is shown in the cookbook, and that every relative link of the docs
// leads to a file (and, for another Markdown file, to a heading) that exists.
func TestDocSnippets(t *testing.T) {
	root := filepath.Join("..")
	shown := make(map[string]bool)
	for _, doc := range docFiles {
		b, err := os.ReadFile(filepath.Join(root, doc))
		if err != nil {
			t.Fatal(err)
		}
		src := string(b)
		tested, outputs := make(map[string]bool), make(map[string]bool)
		for _, blk := range docBlocks(t, doc, src) {
			file, err := os.ReadFile(filepath.Join(root, blk.path))
			if err != nil {
				t.Errorf("%s:%d: %v", doc, blk.line, err)
				continue
			}
			if blk.kind == "test" {
				shown[blk.path] = true
				tested[blk.path] = true
			} else if !tested[blk.path] {
				t.Errorf("%s:%d: an output marker for %s with no <!-- test: %s --> marker before it in this doc", doc, blk.line, blk.path, blk.path)
			} else {
				outputs[blk.path] = true
			}
			want := docBody(blk.kind, blk.path, string(file))
			if blk.text != want {
				t.Errorf("%s:%d: the block is not %s %s (without its harness header and Output block).\n--- the block\n%s\n--- the file\n%s",
					doc, blk.line, blk.kind, blk.path, blk.text, want)
			}
		}
		// A shown program that prints something has its output shown too.
		for path := range tested {
			if file, err := os.ReadFile(filepath.Join(root, path)); err == nil && strings.HasSuffix(path, ".gp") &&
				strings.TrimSpace(docBody("output", path, string(file))) != "" && !outputs[path] {
				t.Errorf("%s: %s is shown without its output (add an <!-- output: %s --> block)", doc, path, path)
			}
		}
		checkDocLinks(t, root, doc, src)
	}
	// Every cookbook and doc test is shown in the docs.
	entries := globAll(root, "tests/interop/cookbook_*", "tests/interop/doc_*")
	for _, e := range entries {
		rel := filepath.ToSlash(strings.TrimPrefix(e, root+string(filepath.Separator)))
		if fi, err := os.Stat(e); err == nil && fi.IsDir() {
			sub, _ := filepath.Glob(filepath.Join(e, "*"))
			ok := false
			for _, s := range sub {
				if shown[filepath.ToSlash(strings.TrimPrefix(s, root+string(filepath.Separator)))] {
					ok = true
				}
			}
			if !ok {
				t.Errorf("%s: no file of this test is shown in the interop docs", rel)
			}
			continue
		}
		if !shown[rel] {
			t.Errorf("%s: a cookbook or doc test that the interop docs do not show (add a <!-- test: %s --> block)", rel, rel)
		}
	}
}

// checkDocLinks checks the relative links of a doc.
func checkDocLinks(t *testing.T, root, doc, src string) {
	dir := filepath.Dir(filepath.Join(root, doc))
	inFence := false
	for n, line := range strings.Split(src, "\n") {
		if strings.HasPrefix(line, "```") {
			inFence = !inFence
		}
		if inFence {
			continue
		}
		for _, m := range docLinkRx.FindAllStringSubmatch(codeSpanRx.ReplaceAllString(line, ""), -1) {
			link := m[1]
			if strings.Contains(link, "://") || strings.HasPrefix(link, "mailto:") {
				continue
			}
			target, anchor, _ := strings.Cut(link, "#")
			path := filepath.Join(dir, target)
			if target == "" {
				path = filepath.Join(root, doc)
			}
			if _, err := os.Stat(path); err != nil {
				t.Errorf("%s:%d: the link %q leads to no file", doc, n+1, link)
				continue
			}
			if anchor != "" && strings.HasSuffix(path, ".md") {
				if !hasHeading(t, path, anchor) {
					t.Errorf("%s:%d: the link %q leads to no heading of %s", doc, n+1, link, filepath.ToSlash(target))
				}
			}
		}
	}
}

var slugDropRx = regexp.MustCompile(`[^\p{L}\p{N} _-]`)

// hasHeading reports whether the Markdown file has a heading whose GitHub anchor is anchor.
func hasHeading(t *testing.T, path, anchor string) bool {
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	seen := make(map[string]int)
	inFence := false
	for _, line := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(line, "```") {
			inFence = !inFence
		}
		if inFence || !strings.HasPrefix(line, "#") {
			continue
		}
		text := strings.TrimSpace(strings.TrimLeft(line, "#"))
		text = strings.NewReplacer("`", "", "*", "").Replace(text)
		slug := strings.ReplaceAll(slugDropRx.ReplaceAllString(strings.ToLower(text), ""), " ", "-")
		n := seen[slug]
		seen[slug]++
		if n > 0 {
			slug += "-" + string(rune('0'+n))
		}
		if slug == anchor {
			return true
		}
	}
	return false
}

// globAll returns the files that match the patterns, relative to root.
func globAll(root string, patterns ...string) []string {
	var all []string
	for _, p := range patterns {
		m, _ := filepath.Glob(filepath.Join(root, p))
		all = append(all, m...)
	}
	return all
}
