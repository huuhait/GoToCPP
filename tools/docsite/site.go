package main

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

// A page is one HTML page of the site: a Markdown file of the docs folder, or a generated page
// (the standard library reference).
type page struct {
	src       string // slash path of the source in the docs folder; for a generated page its output path
	out       string // slash path of the output file
	title     string
	body      string // HTML of the content
	headings  []heading
	ids       map[string]bool // the anchors of the page
	links     []resolvedLink  // the links to check once every page exists
	section   string          // the nav section; "" when the page is not in the nav
	navTitle  string
	generated bool
}

// A resolvedLink is a link of a page that points at another page and, maybe, an anchor of it.
type resolvedLink struct {
	href   string // as written
	line   int
	target string // the src of the target page
	frag   string
}

// A navSection is a titled group of pages of the sidebar.
type navSection struct {
	title   string
	entries []*navEntry
}

type navEntry struct {
	pattern string // a path, or a glob for hidden entries
	title   string // overrides the title of the page
	hidden  bool
	pages   []*page
}

// A site is the set of pages built from a docs folder.
type site struct {
	docs    string // the docs folder on disk
	repo    string // the repository root on disk: links that leave docs/ must exist here
	repoURL string // the URL of the repository on the web, e.g. https://github.com/huuhait/gocpp
	branch  string
	pages   []*page
	bySrc   map[string]*page
	assets  []string // non-Markdown files of docs/, copied as they are (docs-relative)
	nav     []*navSection
	errs    []string
	known   map[string]bool // "page|href": broken links that are tolerated (docs/site/known-links.txt)
	used    map[string]bool // the known links that were found broken
}

func (s *site) errorf(format string, a ...any) {
	s.errs = append(s.errs, fmt.Sprintf(format, a...))
}

// brokenLink reports a link that leads nowhere, unless known-links.txt lists it.
func (s *site) brokenLink(pg *page, line int, href, format string, a ...any) {
	key := pg.src + "|" + href
	if s.known[key] {
		s.used[key] = true
		return
	}
	s.errorf("%s:%d: the link %q "+format, append([]any{pg.src, line, href}, a...)...)
}

// loadKnown reads docs/site/known-links.txt.
func (s *site) loadKnown() error {
	s.known, s.used = map[string]bool{}, map[string]bool{}
	b, err := os.ReadFile(filepath.Join(s.docs, "site", "known-links.txt"))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	for n, l := range strings.Split(string(b), "\n") {
		l = strings.TrimSpace(l)
		if l == "" || strings.HasPrefix(l, "//") {
			continue
		}
		f := strings.Split(l, "|")
		if len(f) != 3 {
			return fmt.Errorf("known-links.txt:%d: want page | link | why", n+1)
		}
		s.known[strings.TrimSpace(f[0])+"|"+strings.TrimSpace(f[1])] = true
	}
	return nil
}

// staleKnown returns the known links that were not broken.
func (s *site) staleKnown() []string {
	var out []string
	for k := range s.known {
		if !s.used[k] {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}

// load reads the docs folder: every Markdown file becomes a page and every other file is an
// asset. The folder docs/site holds the site's own files (nav.txt) and is not published.
func (s *site) load() error {
	s.bySrc = map[string]*page{}
	var srcs []string
	err := filepath.WalkDir(s.docs, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(s.docs, p)
		rel = filepath.ToSlash(rel)
		if d.IsDir() {
			if rel == "site" || strings.HasPrefix(d.Name(), ".") && rel != "." {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(rel, ".md") {
			srcs = append(srcs, rel)
		} else {
			s.assets = append(s.assets, rel)
		}
		return nil
	})
	if err != nil {
		return err
	}
	sort.Strings(srcs)
	has := map[string]bool{}
	for _, p := range srcs {
		has[p] = true
	}
	for _, src := range srcs {
		pg := &page{src: src, out: outPath(src, has)}
		s.pages = append(s.pages, pg)
		s.bySrc[src] = pg
	}
	return nil
}

// outPath returns the output file of a Markdown source: README.md becomes index.html, unless the
// folder has an index.md.
func outPath(src string, has map[string]bool) string {
	dir, name := path.Split(src)
	switch {
	case name == "README.md" && !has[dir+"index.md"]:
		return dir + "index.html"
	case name == "index.md":
		return dir + "index.html"
	}
	return strings.TrimSuffix(src, ".md") + ".html"
}

// render turns the Markdown of every page into HTML.
func (s *site) render() error {
	for _, pg := range s.pages {
		if pg.generated {
			continue
		}
		b, err := os.ReadFile(filepath.Join(s.docs, filepath.FromSlash(pg.src)))
		if err != nil {
			return err
		}
		pg := pg
		body, r := renderMarkdown(string(b), func(href string, line int) string { return s.rewrite(pg, href, line) })
		pg.body = body
		pg.headings = r.headings
		pg.ids = map[string]bool{}
		for _, h := range r.headings {
			pg.ids[h.ID] = true
		}
		pg.title = r.title
		for _, p := range r.problems {
			s.errorf("%s:%s", pg.src, strings.TrimPrefix(p, "line "))
		}
		if pg.title == "" {
			pg.title = strings.TrimSuffix(path.Base(pg.src), ".md")
			s.errorf("%s: no level-1 heading", pg.src)
		}
	}
	return nil
}

// isExternal reports whether href leaves the site: it has a scheme.
func isExternal(href string) bool {
	if i := strings.IndexAny(href, "/#?"); i >= 0 {
		href = href[:i]
	}
	return strings.Contains(href, ":")
}

// rewrite returns the destination to put in the HTML for a link of a page, and records the link
// for check. A link to a Markdown file of the docs becomes a link to its page, a link to another
// file of the docs stays, and a link to a file of the repository outside docs/ becomes a link to
// the file on the web.
func (s *site) rewrite(pg *page, href string, line int) string {
	if isExternal(href) || href == "" {
		return href
	}
	p, frag, hasFrag := strings.Cut(href, "#")
	if i := strings.IndexByte(p, '?'); i >= 0 {
		s.errorf("%s:%d: the link %q has a query", pg.src, line, href)
		return href
	}
	if p == "" {
		pg.links = append(pg.links, resolvedLink{href, line, pg.src, frag})
		return href
	}
	dir := path.Dir(pg.src)
	target := path.Join(dir, p)
	isDir := strings.HasSuffix(p, "/")
	if strings.HasPrefix(target, "../") || target == ".." {
		return s.repoLink(pg, href, line, target, frag, hasFrag)
	}
	if t := s.bySrc[target]; t != nil {
		pg.links = append(pg.links, resolvedLink{href, line, t.src, frag})
		return rel(path.Dir(pg.out), t.out) + fragSuffix(frag, hasFrag)
	}
	if isDir || isDirectory(filepath.Join(s.docs, filepath.FromSlash(target))) {
		for _, name := range []string{"index.md", "README.md"} {
			if t := s.bySrc[path.Join(target, name)]; t != nil {
				pg.links = append(pg.links, resolvedLink{href, line, t.src, frag})
				return rel(path.Dir(pg.out), t.out) + fragSuffix(frag, hasFrag)
			}
		}
		s.brokenLink(pg, line, href, "leads to a folder with no README.md or index.md")
		return href
	}
	if _, err := os.Stat(filepath.Join(s.docs, filepath.FromSlash(target))); err == nil {
		return href // another file of the docs (an image): copied as it is
	}
	s.brokenLink(pg, line, href, "leads to no file")
	return href
}

func isDirectory(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.IsDir()
}

func fragSuffix(frag string, has bool) string {
	if !has {
		return ""
	}
	return "#" + frag
}

// repoLink handles a link that leaves the docs folder (target starts with ../, relative to
// docs/): the file must exist in the repository, and the page links to it on the web.
func (s *site) repoLink(pg *page, href string, line int, target, frag string, hasFrag bool) string {
	repoRel := path.Clean(path.Join("docs", target))
	if strings.HasPrefix(repoRel, "../") || repoRel == ".." {
		s.errorf("%s:%d: the link %q leaves the repository", pg.src, line, href)
		return href
	}
	fi, err := os.Stat(filepath.Join(s.repo, filepath.FromSlash(repoRel)))
	if err != nil {
		s.brokenLink(pg, line, href, "leads to no file (%s)", repoRel)
		return href
	}
	kind := "blob"
	if fi.IsDir() {
		kind = "tree"
	}
	return s.repoURL + "/" + kind + "/" + s.branch + "/" + repoRel + fragSuffix(frag, hasFrag)
}

// rel returns the relative URL from the folder from to the file to (both slash paths of the
// output).
func rel(from, to string) string {
	if from == "." {
		from = ""
	}
	fp := splitPath(from)
	tp := splitPath(to)
	i := 0
	for i < len(fp) && i < len(tp)-1 && fp[i] == tp[i] {
		i++
	}
	var parts []string
	for range fp[i:] {
		parts = append(parts, "..")
	}
	parts = append(parts, tp[i:]...)
	return strings.Join(parts, "/")
}

func splitPath(p string) []string {
	var out []string
	for _, e := range strings.Split(p, "/") {
		if e != "" && e != "." {
			out = append(out, e)
		}
	}
	return out
}

// check resolves the anchors of the links once every page is rendered.
func (s *site) check() {
	for _, pg := range s.pages {
		for _, l := range pg.links {
			t := s.bySrc[l.target]
			if t == nil {
				s.errorf("%s:%d: the link %q leads to no page", pg.src, l.line, l.href)
				continue
			}
			if l.frag != "" && !t.ids[l.frag] {
				s.brokenLink(pg, l.line, l.href, "leads to no anchor %q of %s", l.frag, t.src)
			}
		}
	}
}

// checkKnown reports the entries of known-links.txt that are not broken any more: they must be
// removed, so that the list cannot hide a new break later.
func (s *site) checkKnown() {
	for _, k := range s.staleKnown() {
		s.errorf("docs/site/known-links.txt: %q is not broken any more: remove the entry", k)
	}
}

// loadNav reads docs/site/nav.txt:
//
//	# Section title
//	path/in/docs.md            a page in the sidebar
//	path/in/docs.md | Title    with a title of its own in the sidebar
//	~ glob/*.md                pages that belong to the section but are not listed
//
// Lines starting with // are comments.
func (s *site) loadNav() error {
	b, err := os.ReadFile(filepath.Join(s.docs, "site", "nav.txt"))
	if err != nil {
		return err
	}
	var cur *navSection
	for n, l := range strings.Split(string(b), "\n") {
		l = strings.TrimSpace(l)
		if l == "" || strings.HasPrefix(l, "//") {
			continue
		}
		if t, ok := strings.CutPrefix(l, "# "); ok {
			cur = &navSection{title: strings.TrimSpace(t)}
			s.nav = append(s.nav, cur)
			continue
		}
		if cur == nil {
			return fmt.Errorf("nav.txt:%d: an entry before the first section", n+1)
		}
		e := &navEntry{}
		if g, ok := strings.CutPrefix(l, "~ "); ok {
			e.hidden = true
			l = strings.TrimSpace(g)
		}
		e.pattern, e.title, _ = strings.Cut(l, "|")
		e.pattern, e.title = strings.TrimSpace(e.pattern), strings.TrimSpace(e.title)
		for _, pg := range s.pages {
			if ok, _ := path.Match(e.pattern, pg.src); ok {
				e.pages = append(e.pages, pg)
				if pg.section == "" { // a page can be listed in more than one section; the first is its own
					pg.section = cur.title
				}
				if e.title != "" {
					pg.navTitle = e.title
				}
			}
		}
		if len(e.pages) == 0 {
			s.errorf("nav.txt:%d: %q matches no page", n+1, e.pattern)
		}
		cur.entries = append(cur.entries, e)
	}
	for _, pg := range s.pages {
		if pg.section == "" && !pg.generated {
			s.errorf("%s is in no section of docs/site/nav.txt", pg.src)
		}
	}
	return nil
}
