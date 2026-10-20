package main

import (
	"bytes"
	"html"
	"io/fs"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/huuhait/gocpp/internal/doc"
	"github.com/huuhait/gocpp/stdlib"
)

// stdlibPackages returns the import paths of the packages in fsys that have .gp files (not the
// _test files), without the internal packages, sorted.
func stdlibPackages(fsys fs.FS) ([]string, error) {
	seen := map[string]bool{}
	err := fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(p, ".gp") || strings.HasSuffix(p, "_test.gp") {
			return nil
		}
		dir := path.Dir(p)
		if dir == "." || dir == "internal" || strings.HasPrefix(dir, "internal/") || strings.Contains(dir, "/internal/") || strings.HasSuffix(dir, "/internal") || strings.Contains(dir, "testdata") {
			return nil
		}
		seen[dir] = true
		return nil
	})
	if err != nil {
		return nil, err
	}
	var pkgs []string
	for p := range seen {
		pkgs = append(pkgs, p)
	}
	sort.Strings(pkgs)
	return pkgs, nil
}

// addStdlib adds the reference of the standard library to the site: one page for each package,
// made by internal/doc (the same text as "gocpp doc -all pkg"), and an index. fsys is the
// library (stdlib.FS() unless a test says otherwise).
func (s *site) addStdlib(fsys fs.FS) error {
	pkgs, err := stdlibPackages(fsys)
	if err != nil {
		return err
	}
	env := &doc.Env{GOOS: "linux", GOARCH: "amd64", Stdlib: fsys, StdlibDir: "$GOCPP/stdlib"}
	type entry struct{ path, synopsis string }
	var index []entry
	for _, p := range pkgs {
		pkg, err := env.ReadDir(fsys, p, p)
		if err != nil {
			s.errorf("stdlib/%s: %v", p, err)
			continue
		}
		var buf bytes.Buffer
		if err := doc.Render(&buf, pkg, "", doc.Options{All: true}); err != nil {
			s.errorf("stdlib/%s: %v", p, err)
			continue
		}
		out := "stdlib/" + p + ".html"
		pg := &page{src: out, out: out, title: "Package " + p, generated: true, ids: map[string]bool{}}
		pg.body, pg.headings = stdlibBody(p, buf.String(), pg.ids, isStub(fsys, p))
		s.pages = append(s.pages, pg)
		s.bySrc[pg.src] = pg
		index = append(index, entry{p, synopsis(buf.String())})
	}
	var b strings.Builder
	b.WriteString("<h1 id=\"standard-library\">Standard library reference</h1>\n")
	matrix := "the feature matrix"
	if t := s.bySrc["spec/feature-matrix.md"]; t != nil {
		matrix = "the <a href=\"" + rel("stdlib", t.out) + "\">feature matrix</a>"
	}
	b.WriteString("<p>This reference is generated from the <code>.gp</code> sources of <code>stdlib/</code> by the same code as <code>gocpp doc -all</code>, so it shows the declarations and comments of the packages that this version of gocpp has. Most of the packages are Go's own, ported by <code>tools/portstd</code>; " + matrix + " says from which phase of the plan each one is available, and <code>gocpp doc</code> shows the same text in the terminal.</p>\n")
	b.WriteString("<table><thead><tr><th>Package</th><th>Synopsis</th></tr></thead><tbody>\n")
	pg := &page{src: "stdlib/index.html", out: "stdlib/index.html", title: "Standard library reference", generated: true, ids: map[string]bool{"standard-library": true}}
	for _, e := range index {
		b.WriteString("<tr><td><a href=\"" + html.EscapeString(rel("stdlib", "stdlib/"+e.path+".html")) + "\"><code>" + html.EscapeString(e.path) + "</code></a></td><td>" + html.EscapeString(e.synopsis) + "</td></tr>\n")
	}
	b.WriteString("</tbody></table>\n")
	pg.body = b.String()
	pg.headings = []heading{{1, "standard-library", "Standard library reference"}}
	s.pages = append(s.pages, pg)
	s.bySrc[pg.src] = pg
	return nil
}

var (
	funcRx  = regexp.MustCompile(`^(?:func|macro)\s+(?:\((?:\w+\s+)?\*?(\w+)(?:\[[^\]]*\])?\)\s+)?(\w+)`)
	typeRx  = regexp.MustCompile(`^type\s+(\w+)`)
	valueRx = regexp.MustCompile(`^(?:var|const)\s+(\w+)`)
	// groupMemberRx matches the first line of a member of a const ( ... ) or var ( ... ) group.
	groupMemberRx = regexp.MustCompile(`^\t([A-Za-z_]\w*(?:, [A-Za-z_]\w*)*)(?:\s|$)`)
)

// stdlibBody makes the HTML of the output of "doc -all": a preformatted text in which the
// section names are headings and every declaration at the start of a line has an anchor (Name,
// Type, Type.Method), which is added to ids.
func stdlibBody(pkgPath, text string, ids map[string]bool, stub bool) (string, []heading) {
	var b strings.Builder
	var heads []heading
	id := "package-" + strings.NewReplacer("/", "-", ".", "-").Replace(pkgPath)
	ids[id] = true
	heads = append(heads, heading{1, id, "Package " + pkgPath})
	b.WriteString("<h1 id=\"" + id + "\">Package <code>" + html.EscapeString(pkgPath) + "</code></h1>\n")
	b.WriteString("<p class=\"note\">Import path <code>" + html.EscapeString(pkgPath) + "</code>. The same text is shown by <code>gocpp doc -all " + html.EscapeString(pkgPath) + "</code>.</p>\n")
	if stub {
		b.WriteString("<p class=\"note\"><strong>Declaration stub.</strong> The declarations of this package are Go's, so a program that imports it type-checks. Some stubs are implemented by the runtime itself (<code>fmt</code>, <code>weak</code>); for the others the implementation is not there yet: see the feature matrix.</p>\n")
	}
	inPre := false
	group := false
	inSections := false // the package comment, before the first section, has no declarations
	closePre := func() {
		if inPre {
			b.WriteString("</pre>\n")
			inPre = false
		}
	}
	lines := strings.Split(strings.TrimRight(text, "\n"), "\n")
	for i, l := range lines {
		if i == 0 && strings.HasPrefix(l, "package ") {
			continue // the package clause line repeats the title
		}
		if isSection(l) {
			closePre()
			inSections = true
			sid := strings.ToLower(strings.ReplaceAll(l, " ", "-"))
			ids[sid] = true
			heads = append(heads, heading{2, sid, strings.ToLower(l)})
			b.WriteString("<h2 id=\"" + sid + "\">" + html.EscapeString(strings.ToLower(l)) + "</h2>\n")
			continue
		}
		if !inPre {
			b.WriteString("<pre class=\"ref\">")
			inPre = true
		}
		var anchors []string
		if !inSections {
			b.WriteString(html.EscapeString(l) + "\n")
			continue
		}
		switch {
		case l == "const (" || l == "var (":
			group = true
		case group && l == ")":
			group = false
		case group:
			if m := groupMemberRx.FindStringSubmatch(l); m != nil {
				anchors = strings.Split(m[1], ", ")
			}
		}
		anchor := ""
		if m := funcRx.FindStringSubmatch(l); m != nil {
			anchor = m[2]
			if m[1] != "" {
				anchor = m[1] + "." + m[2]
			}
		} else if m := typeRx.FindStringSubmatch(l); m != nil {
			anchor = m[1]
		} else if m := valueRx.FindStringSubmatch(l); m != nil {
			anchor = m[1]
		}
		if anchor != "" {
			anchors = append(anchors, anchor)
		}
		for _, an := range anchors {
			if an == "_" {
				continue
			}
			a := an
			for n := 1; ids[a]; n++ {
				a = an + "-" + strconv.Itoa(n)
			}
			ids[a] = true
			b.WriteString("<a class=\"decl\" id=\"" + html.EscapeString(a) + "\"></a>")
		}
		b.WriteString(html.EscapeString(l) + "\n")
	}
	closePre()
	return b.String(), heads
}

// isStub reports whether a file of the package is a declaration stub made by tools/stubgen.
func isStub(fsys fs.FS, pkg string) bool {
	entries, err := fs.ReadDir(fsys, pkg)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".gp") {
			continue
		}
		b, err := fs.ReadFile(fsys, path.Join(pkg, e.Name()))
		if err == nil && bytes.Contains(b, []byte("A declaration stub")) {
			return true
		}
	}
	return false
}

// synopsis returns the first sentence of the package comment in the text of "doc -all": the
// text after the package clause line, up to the first blank line.
func synopsis(text string) string {
	lines := strings.Split(text, "\n")
	var parts []string
	for i, l := range lines {
		if i == 0 {
			continue // the package clause
		}
		if strings.TrimSpace(l) == "" {
			if len(parts) > 0 {
				break
			}
			continue
		}
		if isSection(l) {
			break
		}
		parts = append(parts, strings.TrimSpace(l))
	}
	s := strings.Join(parts, " ")
	if i := strings.Index(s, ". "); i >= 0 {
		s = s[:i+1]
	}
	if len(s) > 200 {
		s = s[:200] + "..."
	}
	return s
}

// embeddedStdlib is the library of the gocpp binary.
func embeddedStdlib() fs.FS { return stdlib.FS() }

// isSection reports whether the line is a section heading of the output of "doc -all" (a line of
// capitals in the package comment is not one).
func isSection(l string) bool {
	switch l {
	case "CONSTANTS", "VARIABLES", "FUNCTIONS", "TYPES", "BUGS", "EXAMPLES":
		return true
	}
	return false
}
