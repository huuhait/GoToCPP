package main

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
	"testing/fstest"
	"time"
)

// realSite builds the site of the repository's docs.
func realSite(t *testing.T) *site {
	t.Helper()
	s, err := build(filepath.Join("..", "..", "docs"), "https://example.com/repo", "main", embeddedStdlib())
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// TestSiteLinks: every link and anchor of every doc leads somewhere, every page is in the
// sidebar, and the generated reference is complete.
func TestSiteLinks(t *testing.T) {
	s := realSite(t)
	for _, e := range s.errs {
		t.Error(e)
	}
	if len(s.pages) < 150 {
		t.Errorf("only %d pages", len(s.pages))
	}
}

var (
	hrefRx = regexp.MustCompile(`(?:href|src)="([^"]*)"`)
	idRx   = regexp.MustCompile(`\bid="([^"]+)"`)
)

// checkOutput checks the files that a site wrote: every relative href and src leads to a file,
// and every fragment to an id of that file. It returns the problems.
func checkOutput(dir string, known map[string]bool) []string {
	var errs []string
	ids := map[string]map[string]bool{}
	var htmls []string
	_ = filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(p, ".html") {
			htmls = append(htmls, p)
			b, _ := os.ReadFile(p)
			set := map[string]bool{}
			for _, m := range idRx.FindAllStringSubmatch(string(b), -1) {
				if set[m[1]] {
					rel, _ := filepath.Rel(dir, p)
					errs = append(errs, fmt.Sprintf("%s: the id %q is used twice", rel, m[1]))
				}
				set[m[1]] = true
			}
			ids[p] = set
		}
		return nil
	})
	for _, p := range htmls {
		b, _ := os.ReadFile(p)
		rel, _ := filepath.Rel(dir, p)
		for _, m := range hrefRx.FindAllStringSubmatch(string(b), -1) {
			href := strings.ReplaceAll(strings.ReplaceAll(m[1], "&amp;", "&"), "&#43;", "+")
			if isExternal(href) || href == "" {
				continue
			}
			if known[strings.TrimSuffix(filepath.ToSlash(rel), ".html")+".md|"+href] {
				continue
			}
			target, frag, _ := strings.Cut(href, "#")
			file := p
			if target != "" {
				file = filepath.Join(filepath.Dir(p), filepath.FromSlash(target))
			}
			if fi, err := os.Stat(file); err != nil || fi.IsDir() {
				errs = append(errs, fmt.Sprintf("%s: %q leads to no file", rel, m[1]))
				continue
			}
			if frag != "" && strings.HasSuffix(file, ".html") && !ids[file][frag] {
				errs = append(errs, fmt.Sprintf("%s: %q leads to no anchor", rel, m[1]))
			}
		}
	}
	sort.Strings(errs)
	return errs
}

// TestOutputLinks: the written site has no broken link (also the links the template makes: the
// sidebar, the previous and next page, the style and the scripts).
func TestOutputLinks(t *testing.T) {
	s := realSite(t)
	if len(s.errs) > 0 {
		t.Skip("TestSiteLinks reports the errors")
	}
	out := t.TempDir()
	if err := s.write(out); err != nil {
		t.Fatal(err)
	}
	for _, e := range checkOutput(out, s.known) {
		t.Error(e)
	}
	for _, f := range []string{"index.html", "tour/index.html", "tour/07-goroutines.html", "guides/go-developers.html", "stdlib/index.html", "stdlib/strings.html", "assets/style.css", "assets/search-index.js", "assets/search.js", ".docsite"} {
		if _, err := os.Stat(filepath.Join(out, filepath.FromSlash(f))); err != nil {
			t.Errorf("the site has no %s", f)
		}
	}
}

// TestCheckOutputFindsBreakage: the output checker is not vacuous.
func TestCheckOutputFindsBreakage(t *testing.T) {
	dir := t.TempDir()
	write := func(name, body string) {
		if err := writeFile(filepath.Join(dir, name), []byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	write("a.html", `<h1 id="top"></h1><a href="b.html#x">ok</a> <a href="b.html#nope">bad anchor</a> <a href="gone.html">bad file</a> <a href="https://x.org/y">ext</a> <a href="#top">self</a> <a href="#zip">bad self</a>`)
	write("b.html", `<h2 id="x"></h2>`)
	write("c.html", `<h2 id="d"></h2><p id="d"></p>`)
	got := strings.Join(checkOutput(dir, nil), "\n")
	for _, want := range []string{`"b.html#nope" leads to no anchor`, `"gone.html" leads to no file`, `"#zip" leads to no anchor`, `c.html: the id "d" is used twice`} {
		if !strings.Contains(got, want) {
			t.Errorf("not reported: %s\n%s", want, got)
		}
	}
	if strings.Count(got, "\n") != 3 {
		t.Errorf("want exactly 4 problems:\n%s", got)
	}
}

// ---- synthetic docs ----

// tinyStd is a standard library of one package.
func tinyStd() fs.FS {
	return fstest.MapFS{
		"hello/hello.gp":        {Data: []byte("// Package hello says hello. It is small.\npackage hello\n\n// Greet returns a greeting.\nfunc Greet(name string) string { return \"hello \" + name }\n\n// T is a type.\ntype T struct{ Field int }\n\n// M is a method.\nfunc (t *T) M() {}\n\nconst (\n\t// A and B are constants.\n\tA, B = 1, 2\n\tC    = 3\n\t_    = 4\n\tD    = 5\n)\n\nvar (\n\tV = 1\n\tW string\n)\n")},
		"hello/internal/x/x.gp": {Data: []byte("package x\n")},
	}
}

// buildFake writes the files into a temporary repository (docs/ inside) and builds its site.
func buildFake(t *testing.T, files map[string]string) *site {
	t.Helper()
	root := t.TempDir()
	for name, body := range files {
		if err := writeFile(filepath.Join(root, filepath.FromSlash(name)), []byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	s, err := build(filepath.Join(root, "docs"), "https://example.com/repo", "main", tinyStd())
	if err != nil {
		t.Fatal(err)
	}
	return s
}

const fakeNav = "# All\nindex.md | Home\na/b.md | B\n~ a/*.md\nstdlib/index.html\n~ stdlib/*.html\n"

func errsOf(s *site) string { return strings.Join(s.errs, "\n") }

func TestSiteChecksLinks(t *testing.T) {
	base := map[string]string{
		"docs/site/nav.txt": fakeNav,
		"docs/index.md":     "# Home\n\n[b](a/b.md) [b2](a/b.md#section-two) [dir](a/) [ref](../README.md#top) [std](stdlib/hello.html#T.M)\n",
		"docs/a/b.md":       "# B\n\n## Section two\n\n[home](../index.md) [self](#section-two) [c](c.md)\n",
		"docs/a/c.md":       "# C\n\n[up](README.md)\n",
		"docs/a/README.md":  "# A\n",
		"README.md":         "# Repo\n",
	}
	if s := buildFake(t, base); len(s.errs) != 0 {
		t.Fatalf("a good tree has errors:\n%s", errsOf(s))
	}
	mutate := func(name, file, old, new, want string) {
		t.Run(name, func(t *testing.T) {
			files := map[string]string{}
			for k, v := range base {
				files[k] = v
			}
			if !strings.Contains(files[file], old) {
				t.Fatalf("%s does not contain %q", file, old)
			}
			files[file] = strings.Replace(files[file], old, new, 1)
			got := errsOf(buildFake(t, files))
			if !strings.Contains(got, want) {
				t.Errorf("want an error with %q, got:\n%s", want, got)
			}
		})
	}
	mutate("missing page", "docs/a/b.md", "(c.md)", "(nope.md)", `the link "nope.md" leads to no file`)
	mutate("missing anchor", "docs/index.md", "a/b.md#section-two", "a/b.md#section-three", `no anchor "section-three"`)
	mutate("missing self anchor", "docs/a/b.md", "(#section-two)", "(#nowhere)", `no anchor "nowhere"`)
	mutate("missing repo file", "docs/index.md", "../README.md#top", "../NOPE.md", "leads to no file (NOPE.md)")
	mutate("leaves the repository", "docs/index.md", "../README.md#top", "../../x.md", "leaves the repository")
	mutate("missing stdlib anchor", "docs/index.md", "T.M", "T.Nope", `no anchor "T.Nope"`)
	mutate("folder without readme", "docs/index.md", "(a/)", "(site/)", "no README.md or index.md")
	mutate("page not in the nav", "docs/site/nav.txt", "~ a/*.md\n", "", "a/c.md is in no section")
	mutate("nav entry matches nothing", "docs/site/nav.txt", "a/b.md | B", "a/zz.md | B", `"a/zz.md" matches no page`)
	mutate("no title", "docs/a/c.md", "# C", "no heading here", "no level-1 heading")
}

func TestSiteKnownLinks(t *testing.T) {
	files := map[string]string{
		"docs/site/nav.txt":         "# All\nindex.md\n",
		"docs/index.md":             "# Home\n\n[gone](gone.md)\n",
		"docs/site/known-links.txt": "// a comment\nindex.md | gone.md | the page is on another branch\nindex.md | other.md | not broken\n",
	}
	s := buildFake(t, files)
	// The known broken link is not reported; the entry that is not broken any more is.
	if len(s.errs) != 1 || !strings.Contains(s.errs[0], `"index.md|other.md" is not broken any more`) {
		t.Errorf("errors:\n%s", errsOf(s))
	}
	if got := s.staleKnown(); len(got) != 1 || got[0] != "index.md|other.md" {
		t.Errorf("stale entries %v", got)
	}
}

func TestSiteRewritesLinks(t *testing.T) {
	s := buildFake(t, map[string]string{
		"docs/site/nav.txt": "# All\nindex.md\n~ x/*.md\n~ x/*/*.md\n~ stdlib/*.html\nstdlib/index.html\n",
		"docs/index.md":     "# Home\n\n[deep](x/y/z.md#t) [repo](../tools/t.go) [dir](../tools/) [asset](x/pic.png)\n",
		"docs/x/y/z.md":     "# Z\n\n## T\n\n[home](../../index.md) [sib](../w.md)\n",
		"docs/x/w.md":       "# W\n",
		"docs/x/README.md":  "# X\n",
		"docs/x/pic.png":    "png",
		"tools/t.go":        "package t\n",
	})
	if len(s.errs) != 0 {
		t.Fatalf("errors:\n%s", errsOf(s))
	}
	by := func(src string) string { return s.bySrc[src].body }
	for _, want := range []string{
		`href="x/y/z.html#t"`,
		`href="https://example.com/repo/blob/main/tools/t.go"`,
		`href="https://example.com/repo/tree/main/tools"`,
		`href="x/pic.png"`,
	} {
		if !strings.Contains(by("index.md"), want) {
			t.Errorf("index.md: no %s in\n%s", want, by("index.md"))
		}
	}
	for _, want := range []string{`href="../../index.html"`, `href="../w.html"`} {
		if !strings.Contains(by("x/y/z.md"), want) {
			t.Errorf("x/y/z.md: no %s in\n%s", want, by("x/y/z.md"))
		}
	}
	if s.bySrc["x/README.md"].out != "x/index.html" {
		t.Errorf("README.md is written as %s", s.bySrc["x/README.md"].out)
	}
	out := t.TempDir()
	if err := s.write(out); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(out, "x", "pic.png")); err != nil {
		t.Errorf("the asset was not copied: %v", err)
	}
	if errs := checkOutput(out, s.known); len(errs) != 0 {
		t.Errorf("output:\n%s", strings.Join(errs, "\n"))
	}
}

func TestRel(t *testing.T) {
	for _, c := range []struct{ from, to, want string }{
		{".", "a.html", "a.html"},
		{"", "a/b.html", "a/b.html"},
		{"a", "a/b.html", "b.html"},
		{"a", "c.html", "../c.html"},
		{"a/b", "a/c/d.html", "../c/d.html"},
		{"a/b/c", "x.html", "../../../x.html"},
	} {
		if got := rel(c.from, c.to); got != c.want {
			t.Errorf("rel(%q, %q) = %q, want %q", c.from, c.to, got, c.want)
		}
	}
}

func TestWriteRefusesForeignFolder(t *testing.T) {
	s := buildFake(t, map[string]string{"docs/site/nav.txt": "# All\nindex.md\n", "docs/index.md": "# Home\n"})
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "precious.txt"), []byte("mine"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := s.write(dir); err == nil || !strings.Contains(err.Error(), "not a site folder") {
		t.Fatalf("write into a foreign folder: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "precious.txt")); err != nil {
		t.Errorf("the foreign file is gone: %v", err)
	}
	// A folder of an earlier run is replaced, and stale pages are removed.
	site := t.TempDir()
	if err := s.write(site); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(site, "stale.html"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := s.write(site); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(site, "stale.html")); err == nil {
		t.Error("a stale page stayed")
	}
}

// TestSiteDeterministic: two builds write the same bytes.
func TestSiteDeterministic(t *testing.T) {
	a, b := t.TempDir(), t.TempDir()
	for _, dir := range []string{a, b} {
		s := realSite(t)
		if err := s.write(dir); err != nil {
			t.Fatal(err)
		}
	}
	read := func(dir string) map[string]string {
		m := map[string]string{}
		_ = filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
			if err == nil && !d.IsDir() {
				b, _ := os.ReadFile(p)
				rel, _ := filepath.Rel(dir, p)
				m[filepath.ToSlash(rel)] = string(b)
			}
			return nil
		})
		return m
	}
	ma, mb := read(a), read(b)
	if len(ma) != len(mb) {
		t.Fatalf("%d files and %d files", len(ma), len(mb))
	}
	for k, v := range ma {
		if mb[k] != v {
			t.Errorf("%s differs between two builds", k)
		}
	}
}

// ---- the standard library reference ----

func TestStdlibReferenceSynthetic(t *testing.T) {
	s := buildFake(t, map[string]string{"docs/site/nav.txt": "# All\nindex.md\nstdlib/index.html\n~ stdlib/*.html\n", "docs/index.md": "# Home\n"})
	if len(s.errs) != 0 {
		t.Fatalf("errors:\n%s", errsOf(s))
	}
	pg := s.bySrc["stdlib/hello.html"]
	if pg == nil {
		t.Fatal("no page for package hello")
	}
	if s.bySrc["stdlib/hello/internal/x.html"] != nil {
		t.Error("an internal package has a page")
	}
	for _, id := range []string{"Greet", "T", "T.M", "functions", "types"} {
		if !pg.ids[id] {
			t.Errorf("no anchor %s in %v", id, pg.ids)
		}
	}
	if !strings.Contains(pg.body, "Greet returns a greeting.") {
		t.Errorf("the comment of Greet is missing:\n%s", pg.body)
	}
	idx := s.bySrc["stdlib/index.html"].body
	if !strings.Contains(idx, `href="hello.html"`) || !strings.Contains(idx, "Package hello says hello.") {
		t.Errorf("the index does not list the package with its synopsis:\n%s", idx)
	}
}

// TestStdlibReferenceReal: every package of stdlib/ that is not internal has a page, and the
// well known names are there.
func TestStdlibReferenceReal(t *testing.T) {
	s := realSite(t)
	pkgs, err := stdlibPackages(embeddedStdlib())
	if err != nil {
		t.Fatal(err)
	}
	if len(pkgs) < 30 {
		t.Fatalf("only %d packages: %v", len(pkgs), pkgs)
	}
	for _, p := range pkgs {
		if pg := s.bySrc["stdlib/"+p+".html"]; pg == nil {
			t.Errorf("no page for %s", p)
		}
		if path.Base(p) == "internal" || strings.Contains(p, "internal") {
			t.Errorf("internal package %s is listed", p)
		}
	}
	for page, ids := range map[string][]string{
		"strings":       {"Builder", "Builder.WriteString", "Split", "functions", "types"},
		"fmt":           {"Println", "Printf"},
		"sort":          {"Slice", "Sort"},
		"encoding/json": {"Marshal", "Unmarshal"},
		"errors":        {"New", "Is", "As"},
	} {
		pg := s.bySrc["stdlib/"+page+".html"]
		if pg == nil {
			t.Errorf("no page for %s", page)
			continue
		}
		for _, id := range ids {
			if !pg.ids[id] {
				t.Errorf("%s: no anchor %s", page, id)
			}
		}
	}
}

// TestPagerAndSidebar: a page that two sections list gets its previous and next page from the
// section that owns it, and only that section is open.
func TestPagerAndSidebar(t *testing.T) {
	s := buildFake(t, map[string]string{
		"docs/site/nav.txt": "# One\na.md | A\nb.md | B\nc.md | C\n# Two\nz.md | Z\nb.md | B again\nstdlib/index.html\n~ stdlib/*.html\n",
		"docs/a.md":         "# A\n",
		"docs/b.md":         "# B\n",
		"docs/c.md":         "# C\n",
		"docs/z.md":         "# Z\n",
	})
	if len(s.errs) != 0 {
		t.Fatalf("errors:\n%s", errsOf(s))
	}
	v := s.view(s.bySrc["b.md"])
	if v.Prev == nil || v.Prev.Title != "A" || v.Next == nil || v.Next.Title != "C" {
		t.Errorf("prev/next of b.md: %+v %+v", v.Prev, v.Next)
	}
	var open []string
	for _, n := range v.Nav {
		if n.Open {
			open = append(open, n.Title)
		}
	}
	if strings.Join(open, ",") != "One" {
		t.Errorf("open sections %v", open)
	}
	if v := s.view(s.bySrc["z.md"]); v.Prev != nil || v.Next == nil || v.Next.Title != "B again" {
		t.Errorf("prev/next of z.md: %+v %+v", v.Prev, v.Next)
	}
}

// ---- more tests of the site ----

const oneDoc = "# All\nindex.md\n~ a/*.md\n~ stdlib/*.html\nstdlib/index.html\n"

func TestSkipsDotAndSiteFolders(t *testing.T) {
	s := buildFake(t, map[string]string{
		"docs/site/nav.txt":  oneDoc,
		"docs/index.md":      "# Home\n",
		"docs/.hidden/x.md":  "# Hidden\n",
		"docs/a/.cache/y.md": "# Hidden too\n",
		"docs/site/notes.md": "# Not a page\n",
		"docs/a/b.md":        "# B\n",
		"docs/a/.dotfile":    "x",
	})
	if len(s.errs) != 0 {
		t.Fatalf("errors:\n%s", errsOf(s))
	}
	for _, pg := range s.pages {
		if strings.Contains(pg.src, ".hidden") || strings.Contains(pg.src, ".cache") || strings.HasPrefix(pg.src, "site/") {
			t.Errorf("the page %s should be skipped", pg.src)
		}
	}
	if s.bySrc["a/b.md"] == nil || s.bySrc["index.md"] == nil {
		t.Error("the real pages are missing")
	}
}

func TestKnownLinksFormat(t *testing.T) {
	root := t.TempDir()
	for name, body := range map[string]string{
		"docs/site/nav.txt":         "# All\nindex.md\n",
		"docs/index.md":             "# Home\n",
		"docs/site/known-links.txt": "index.md | gone.md\n",
	} {
		if err := writeFile(filepath.Join(root, filepath.FromSlash(name)), []byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	_, err := build(filepath.Join(root, "docs"), "https://example.com/repo", "main", tinyStd())
	if err == nil || !strings.Contains(err.Error(), "want page | link | why") {
		t.Errorf("a malformed known-links.txt: %v", err)
	}
}

func TestEditLinkAndSource(t *testing.T) {
	s := buildFake(t, map[string]string{
		"docs/site/nav.txt": oneDoc,
		"docs/index.md":     "# Home\n",
		"docs/a/b.md":       "# B\n",
	})
	if got, want := s.view(s.bySrc["a/b.md"]).Source, "https://example.com/repo/blob/main/docs/a/b.md"; got != want {
		t.Errorf("source link %q, want %q", got, want)
	}
	if got := s.view(s.bySrc["stdlib/index.html"]).Source; got != "" {
		t.Errorf("a generated page has the source link %q", got)
	}
	out := t.TempDir()
	if err := s.write(out); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(filepath.Join(out, "a", "b.html"))
	if !strings.Contains(string(b), `<a href="https://example.com/repo/blob/main/docs/a/b.md">Edit this page on GitHub</a>`) {
		t.Errorf("no edit link in a/b.html:\n%s", b)
	}
	b, _ = os.ReadFile(filepath.Join(out, "stdlib", "index.html"))
	if strings.Contains(string(b), "Edit this page") {
		t.Error("the generated index has an edit link")
	}
}

// parseSearchIndex reads assets/search-index.js (window.DOCS_INDEX = [...];).
func parseSearchIndex(t *testing.T, js []byte) []searchEntry {
	t.Helper()
	text := strings.TrimSpace(string(js))
	text = strings.TrimSuffix(strings.TrimPrefix(text, "window.DOCS_INDEX = "), ";")
	var idx []searchEntry
	if err := json.Unmarshal([]byte(text), &idx); err != nil {
		t.Fatalf("the index is not JSON: %v\n%s", err, js)
	}
	return idx
}

func TestSearchIndex(t *testing.T) {
	s := buildFake(t, map[string]string{
		"docs/site/nav.txt": "# All\nindex.md\n~ a/*.md\nstdlib/index.html\n~ stdlib/*.html\n",
		"docs/index.md":     "# Home\n",
		"docs/a/b.md":       "# Page B\n\n## Section `x` & <y>\n\n### Sub </script><script>alert(1)\n",
	})
	if len(s.errs) != 0 {
		t.Fatalf("errors:\n%s", errsOf(s))
	}
	raw := s.searchIndex()
	if strings.Contains(string(raw), "</script>") || strings.Contains(string(raw), "<y>") {
		t.Errorf("the index has unescaped HTML:\n%s", raw)
	}
	idx := parseSearchIndex(t, raw)
	want := map[string]searchEntry{
		"Page B":                        {T: "Page B", U: "a/b.html"},
		"Section x & <y>":               {T: "Section x & <y>", U: "a/b.html#section-x--y", P: "Page B", L: 2},
		"Sub </script><script>alert(1)": {T: "Sub </script><script>alert(1)", U: "a/b.html#sub-scriptscriptalert1", P: "Page B", L: 3},
		"hello.Greet":                   {T: "hello.Greet", U: "stdlib/hello.html#Greet", P: "Package hello", L: 3},
		"hello.T.M":                     {T: "hello.T.M", U: "stdlib/hello.html#T.M", P: "Package hello", L: 3},
		"hello.A":                       {T: "hello.A", U: "stdlib/hello.html#A", P: "Package hello", L: 3},
		"hello.B":                       {T: "hello.B", U: "stdlib/hello.html#B", P: "Package hello", L: 3},
		"hello.V":                       {T: "hello.V", U: "stdlib/hello.html#V", P: "Package hello", L: 3},
	}
	got := map[string]searchEntry{}
	for _, e := range idx {
		got[e.T] = e
	}
	for k, w := range want {
		if g, ok := got[k]; !ok || g != w {
			t.Errorf("entry %q: got %+v (present %v), want %+v", k, g, ok, w)
		}
	}
	// The title of a page is one entry, not two (its level-1 heading is the same text).
	n := 0
	for _, e := range idx {
		if e.U == "a/b.html" {
			n++
		}
	}
	if n != 1 {
		t.Errorf("%d entries for the page a/b.html itself", n)
	}
	// No entry for the sections of a package page or for the package heading.
	for _, e := range idx {
		if strings.HasSuffix(e.T, ".functions") || strings.HasSuffix(e.T, ".types") || strings.HasSuffix(e.T, ".constants") {
			t.Errorf("an entry for a section: %+v", e)
		}
	}
	if !sort.SliceIsSorted(idx, func(i, j int) bool { return idx[i].U < idx[j].U }) {
		t.Error("the index is not sorted by URL")
	}
}

func TestStdlibGroupAnchors(t *testing.T) {
	s := buildFake(t, map[string]string{
		"docs/site/nav.txt": "# All\nindex.md\nstdlib/index.html\n~ stdlib/*.html\n",
		"docs/index.md":     "# Home\n",
	})
	pg := s.bySrc["stdlib/hello.html"]
	for _, id := range []string{"A", "B", "C", "D", "V", "W"} {
		if !pg.ids[id] {
			t.Errorf("no anchor %s: %v", id, pg.ids)
		}
	}
	if pg.ids["_"] || pg.ids["Field"] {
		t.Errorf("the blank identifier or a field has an anchor: %v", pg.ids)
	}
	for _, line := range []string{`<a class="decl" id="A"></a>`, `<a class="decl" id="D"></a>`} {
		if !strings.Contains(pg.body, line) {
			t.Errorf("no %s in the page", line)
		}
	}
}

// TestStdlibBodyManyDuplicates: repeated names get distinct anchors and the loop ends.
func TestStdlibBodyManyDuplicates(t *testing.T) {
	text := "package p // import \"p\"\n\nFUNCTIONS\n\n" + strings.Repeat("func F()\n    doc\n\n", 25)
	ids := map[string]bool{}
	body, _ := stdlibBody("p", text, ids, false)
	for _, id := range []string{"F", "F-1", "F-9", "F-10", "F-24"} {
		if !ids[id] {
			t.Errorf("no anchor %s", id)
		}
	}
	if strings.Count(body, `class="decl"`) != 25 {
		t.Errorf("%d anchors", strings.Count(body, `class="decl"`))
	}
}

func TestStdlibStubNote(t *testing.T) {
	text := "package p // import \"p\"\n\nFUNCTIONS\n\nfunc F()\n"
	if b, _ := stdlibBody("p", text, map[string]bool{}, true); !strings.Contains(b, "Declaration stub") {
		t.Error("no stub note")
	}
	if b, _ := stdlibBody("p", text, map[string]bool{}, false); strings.Contains(b, "Declaration stub") {
		t.Error("a stub note for a real package")
	}
	fsys := fstest.MapFS{
		"s/s.gp":    {Data: []byte("// A declaration stub for Phase 01\npackage s\n")},
		"r/r.gp":    {Data: []byte("package r\n")},
		"r/note.md": {Data: []byte("A declaration stub")},
	}
	if !isStub(fsys, "s") || isStub(fsys, "r") || isStub(fsys, "none") {
		t.Errorf("isStub: s=%v r=%v none=%v", isStub(fsys, "s"), isStub(fsys, "r"), isStub(fsys, "none"))
	}
}

func TestTemplateIDsDoNotClash(t *testing.T) {
	s := buildFake(t, map[string]string{
		"docs/site/nav.txt": oneDoc,
		"docs/index.md":     "# Home\n\n## Results\n\n## Q\n\n## Dup\n\n## Dup\n\n## Dup 1\n",
	})
	out := t.TempDir()
	if err := s.write(out); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(filepath.Join(out, "index.html"))
	for id, n := range map[string]int{"results": 1, "q": 1, "gp-search": 1, "gp-results": 1, "dup": 1, "dup-1": 1, "dup-1-1": 1} {
		if got := strings.Count(string(b), `id="`+id+`"`); got != n {
			t.Errorf("id %q appears %d times, want %d", id, got, n)
		}
	}
	if errs := checkOutput(out, nil); len(errs) != 0 {
		t.Errorf("output problems:\n%s", strings.Join(errs, "\n"))
	}
	js, _ := os.ReadFile(filepath.Join(out, "assets", "search.js"))
	for _, id := range []string{"gp-search", "gp-results"} {
		if !strings.Contains(string(js), `"`+id+`"`) {
			t.Errorf("search.js does not use the id %s", id)
		}
	}
}

// ---- the output folder ----

func TestCheckOut(t *testing.T) {
	root := t.TempDir()
	for name, body := range map[string]string{"docs/site/nav.txt": "# All\nindex.md\n", "docs/index.md": "# Home\n"} {
		if err := writeFile(filepath.Join(root, filepath.FromSlash(name)), []byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	s, err := build(filepath.Join(root, "docs"), "https://example.com/repo", "main", tinyStd())
	if err != nil {
		t.Fatal(err)
	}
	other := t.TempDir()
	linkTarget := t.TempDir()
	if err := os.Symlink(linkTarget, filepath.Join(other, "link")); err != nil {
		t.Skip("no symbolic links")
	}
	fileOut := filepath.Join(other, "afile")
	if err := os.WriteFile(fileOut, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	dirMarker := filepath.Join(other, "dirmarker")
	if err := os.MkdirAll(filepath.Join(dirMarker, siteMarker), 0o755); err != nil {
		t.Fatal(err)
	}
	cases := []struct{ name, out, want string }{
		{"inside the docs", filepath.Join(root, "docs", "site-out"), "inside the docs folder"},
		{"the docs folder", filepath.Join(root, "docs"), "inside the docs folder"},
		{"holding the docs", root, "contains the docs or the repository"},
		{"above the repository", filepath.Dir(root), "contains the docs or the repository"},
		{"a link", filepath.Join(other, "link"), "not a folder"},
		{"a file", fileOut, "not a folder"},
		{"a marker that is a folder", dirMarker, "has no .docsite file"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := s.checkOut(c.out)
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Errorf("checkOut(%s) = %v, want an error with %q", c.out, err, c.want)
			}
			if err := s.write(c.out); err == nil {
				t.Errorf("write(%s) succeeded", c.out)
			}
		})
	}
	// Folders that may be written: missing, empty, a site of an earlier run.
	ok := filepath.Join(other, "new", "site")
	if err := s.write(ok); err != nil {
		t.Fatal(err)
	}
	if err := s.write(ok); err != nil {
		t.Fatalf("a second run: %v", err)
	}
	empty := t.TempDir()
	if err := s.write(empty); err != nil {
		t.Fatalf("an empty folder: %v", err)
	}
	// No temporary folder is left next to the site.
	entries, _ := os.ReadDir(filepath.Dir(ok))
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".docsite-new-") {
			t.Errorf("a temporary folder is left: %s", e.Name())
		}
	}
}

// TestWriteFailureKeepsOldSite: a failure in the middle of writing leaves the old site untouched,
// and no half-written folder behind.
func TestWriteFailureKeepsOldSite(t *testing.T) {
	good := buildFake(t, map[string]string{"docs/site/nav.txt": "# All\nindex.md\n", "docs/index.md": "# Home\n"})
	out := filepath.Join(t.TempDir(), "site")
	if err := good.write(out); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(filepath.Join(out, "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	// An asset named "b" (a file) and a page b/c.html: the page cannot be written.
	bad := buildFake(t, map[string]string{
		"docs/site/nav.txt": "# All\nindex.md\n",
		"docs/index.md":     "# Changed home\n",
		"docs/b":            "a file where a folder is needed",
	})
	// "b" is an asset (a file); a page b/c.html needs a folder b.
	bad.pages = append(bad.pages, &page{src: "b/c.md", out: "b/c.html", title: "C", ids: map[string]bool{}})
	if err := bad.write(out); err == nil {
		t.Fatal("the conflicting site was written")
	}
	after, err := os.ReadFile(filepath.Join(out, "index.html"))
	if err != nil || string(after) != string(before) {
		t.Errorf("the old site changed: %v", err)
	}
	entries, _ := os.ReadDir(filepath.Dir(out))
	for _, e := range entries {
		if e.Name() != "site" {
			t.Errorf("left behind: %s", e.Name())
		}
	}
}

// ---- the output folder, round 2 ----

func tinySite(t *testing.T, repo string) *site {
	t.Helper()
	root := t.TempDir()
	for name, body := range map[string]string{"docs/site/nav.txt": "# All\nindex.md\n", "docs/index.md": "# Home\n"} {
		if err := writeFile(filepath.Join(root, filepath.FromSlash(name)), []byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	s, err := build(filepath.Join(root, "docs"), repo, "main", tinyStd())
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// TestWriteMode: the swapped-in site is readable by others (the temporary folder is made with mode
// 0700 and must be opened up), the files too.
func TestWriteMode(t *testing.T) {
	s := tinySite(t, "https://example.com/repo")
	out := filepath.Join(t.TempDir(), "site")
	if err := s.write(out); err != nil {
		t.Fatal(err)
	}
	fi, err := os.Stat(out)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != 0o755 {
		t.Errorf("the site folder has mode %v, want 0755", fi.Mode().Perm())
	}
	if fi, err := os.Stat(filepath.Join(out, "index.html")); err != nil || fi.Mode().Perm()&0o044 != 0o044 {
		t.Errorf("index.html: %v %v", fi, err)
	}
}

// TestLeftoverCleanup: the temporary folders of a killed run (".docsite-new-*" and its ".old") are
// removed when they hold the .docsite marker and are old; nothing else is touched.
func TestLeftoverCleanup(t *testing.T) {
	s := tinySite(t, "https://example.com/repo")
	parent := t.TempDir()
	old := time.Now().Add(-3 * time.Hour)
	mk := func(name string, marker string, mtime time.Time) {
		dir := filepath.Join(parent, name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		switch marker {
		case "file":
			if err := os.WriteFile(filepath.Join(dir, siteMarker), []byte("x"), 0o644); err != nil {
				t.Fatal(err)
			}
		case "dir":
			if err := os.Mkdir(filepath.Join(dir, siteMarker), 0o755); err != nil {
				t.Fatal(err)
			}
		}
		if err := os.WriteFile(filepath.Join(dir, "page.html"), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(dir, mtime, mtime); err != nil {
			t.Fatal(err)
		}
	}
	mk(".docsite-new-aaa", "file", old)
	mk(".docsite-new-aaa.old", "file", old)
	mk(".docsite-new-fresh", "file", time.Now())
	mk(".docsite-new-nomarker", "none", old)
	mk(".docsite-new-dirmarker", "dir", old)
	mk(".docsite-newx", "file", old)
	mk("docsite-new-other", "file", old)
	if err := s.write(filepath.Join(parent, "site")); err != nil {
		t.Fatal(err)
	}
	gone := []string{".docsite-new-aaa", ".docsite-new-aaa.old"}
	kept := []string{".docsite-new-fresh", ".docsite-new-nomarker", ".docsite-new-dirmarker", ".docsite-newx", "docsite-new-other", "site"}
	for _, n := range gone {
		if _, err := os.Lstat(filepath.Join(parent, n)); err == nil {
			t.Errorf("%s was not removed", n)
		}
	}
	for _, n := range kept {
		if _, err := os.Lstat(filepath.Join(parent, n)); err != nil {
			t.Errorf("%s was removed: %v", n, err)
		}
	}
}

// TestRepoURLTrailingSlash: -repo with a trailing slash does not give links with "//".
func TestRepoURLTrailingSlash(t *testing.T) {
	s := tinySite(t, "https://example.com/repo///")
	if got, want := s.view(s.bySrc["index.md"]).Source, "https://example.com/repo/blob/main/docs/index.md"; got != want {
		t.Errorf("source link %q, want %q", got, want)
	}
}

// TestPackageCommentHasNoAnchors: lines of the package comment that look like declarations ("type
// for a discussion of ...") or like section names do not make anchors or headings; only the
// declarations of the sections do.
func TestPackageCommentHasNoAnchors(t *testing.T) {
	text := "package p // import \"p\"\n\nPackage p is a package.\ntype for a discussion of x\nfunc foo is nice\nvar x here\nconst y is\nNOTE ABOUT IT\n\nFUNCTIONS\n\nfunc F()\n    type for a discussion of y\n"
	ids := map[string]bool{}
	body, heads := stdlibBody("p", text, ids, false)
	for _, bad := range []string{"for", "foo", "x", "y", "note-about-it", "NOTE ABOUT IT"} {
		if ids[bad] {
			t.Errorf("an anchor %q from the package comment", bad)
		}
	}
	if !ids["F"] || !ids["functions"] {
		t.Errorf("ids %v", ids)
	}
	if strings.Contains(body, "<h2 id=\"note-about-it\"") {
		t.Error("a line of capitals in the package comment became a heading")
	}
	if len(heads) != 2 { // the title and "functions"
		t.Errorf("headings %v", heads)
	}
}
