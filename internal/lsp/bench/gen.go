// Package bench is the language server's benchmark harness (Phase 11, step
// 11.11): a generator of the reference projects, a client for any LSP server
// over stdio (the gocpp language server and gopls), the scripted scenarios
// that it replays, the statistics, and the report with the side-by-side
// table of gocpp and gopls. tools/lspbench is its command line.
//
// The reference projects are generated, never checked in: the same seed
// always gives byte-identical files, and the same source is valid Go and
// valid gocpp, so gopls can open it as .go files and gocpp lsp as .gp files
// (honest side by side). The generated code is made of the things editors
// meet all day: structs with embedding, methods, interfaces, errors and
// fmt calls, goroutines and channels, and calls between packages.
package bench

import (
	"fmt"
	"math/rand/v2"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// A Spec says what to generate.
type Spec struct {
	Name     string // "small", "medium", "large", "cpp"
	Seed     uint64
	Packages int    // the generated packages (core and main come on top)
	Files    int    // the files of each package
	Lite     bool   // shorter files (about 35 lines, not 70), for the large project
	Module   string // the module path; "example.com/bench/<name>" when empty
	// Cpp adds CppPackages packages (gocpp only: they import C++ headers,
	// like examples/cpp-std, and a header of their own) and a package that
	// has a typo in a std name (the namespace probe finds it).
	CppPackages int
	// MacroPackages adds that many packages that use the forms of Phase 10 (gocpp only): `!`
	// calls and must functions, macros of the package mac00, and #Derive, in every file. The
	// language server's features for them are measured on MacroFile (step 11.13).
	MacroPackages int
	// MacroFiles is the number of files of each macro package (Files when 0).
	MacroFiles int
}

// The three reference projects and the one with C++ headers. The seeds are
// fixed, so a report names a project by its name alone.
var (
	Small  = Spec{Name: "small", Seed: 1, Packages: 3, Files: 6}
	Medium = Spec{Name: "medium", Seed: 2, Packages: 20, Files: 10}
	Large  = Spec{Name: "large", Seed: 3, Packages: 100, Files: 15, Lite: true}
	Cpp    = Spec{Name: "cpp", Seed: 4, Packages: 10, Files: 8, CppPackages: 6}
	// Macro is a small project whose packages use `!`, macros and #Derive everywhere (gocpp only).
	Macro = Spec{Name: "macro", Seed: 5, Packages: 4, Files: 6, MacroPackages: 8}
	// BigPkg is one package of 46 files (3.7k lines) full of `!`, macros and #Derive, next to the file of its
	// macros: the shape in which a keystroke in a file with a syntax error costs the most (gocpp only).
	BigPkg = Spec{Name: "bigpkg", Seed: 6, Packages: 1, Files: 2, MacroPackages: 2, MacroFiles: 46}
)

// Specs returns the reference projects, small to large.
func Specs() []Spec { return []Spec{Small, Medium, Large, Cpp, Macro, BigPkg} }

// SpecByName returns the reference project called name.
func SpecByName(name string) (Spec, bool) {
	for _, s := range Specs() {
		if s.Name == name {
			return s, true
		}
	}
	return Spec{}, false
}

// A FileKind says what a generated file is.
type FileKind int

const (
	Source FileKind = iota // a Go/gocpp file: Rel has no extension, Write adds it
	Header                 // a C++ header (gocpp only)
)

// A File is one generated file.
type File struct {
	Rel     string // slash-separated, relative to the project; no extension for Source
	Content string
	Kind    FileKind
}

// A Project is a generated project.
type Project struct {
	Spec   Spec
	Module string
	Files  []File // sorted by Rel

	// Hot is the file the scenarios work in (a package in the middle of
	// the dependency chain, so it has importers and imports), Dep a file
	// of a package that Hot's package imports, Core the file of core.Base,
	// and Dependent a file of a package that imports Hot's package. All
	// are Rel names of Source files.
	Hot, Dep, Core, Dependent string
	// HotPkg is the directory of Hot's package.
	HotPkg string
	// Typo is the Rel of the file with the typo in a namespace name (empty without
	// C++ packages), CppFile a file that imports C++ headers (idem).
	Typo, CppFile string
	// MacroFile is a file full of `!` calls, macro uses and #Derive (empty without MacroPackages).
	MacroFile string
	// Big is a file of about 1,000 lines (the format-on-save budget is for
	// such a file), APIFile a file of the last package, whose exported
	// function Label the scenarios change, and APIUser the file that calls
	// it (cmd/app/main).
	Big, APIFile, APIUser string
	// Queries are workspace symbol queries that have answers.
	Queries []string
	// Lines is the number of lines of the source files.
	Lines int
}

// typeName returns the name of the type of file k of package i.
func typeName(i, k int) string { return fmt.Sprintf("%s%d", nouns[(i*7+k*3)%len(nouns)], k) }

// Source returns the content of the source file rel, or "".
func (p *Project) Source(rel string) string {
	for _, f := range p.Files {
		if f.Rel == rel && f.Kind == Source {
			return f.Content
		}
	}
	return ""
}

// SourceFiles returns the Rel names of the Source files that are Go-valid
// too (without the C++ ones), sorted.
func (p *Project) SourceFiles(goOnly bool) []string {
	var out []string
	for _, f := range p.Files {
		if f.Kind == Source && !(goOnly && isCppPath(f.Rel)) {
			out = append(out, f.Rel)
		}
	}
	return out
}

// Write writes the project to dir for gocpp (ext ".gp", with gocpp.mod and
// the headers) or for Go (".go", with go.mod, without what only gocpp can
// read: headers and the packages that import them).
const projectGoVersion = "1.25.0" // supported modern semantics, identical in every measurement cohort

func (p *Project) Write(dir, ext string) error {
	gocpp := ext == ".gp"
	mod := "go.mod"
	body := "module " + p.Module + "\n\ngo " + projectGoVersion + "\n"
	if gocpp {
		mod, body = "gocpp.mod", "module "+p.Module+"\n"
	}
	if err := writeFile(filepath.Join(dir, mod), body); err != nil {
		return err
	}
	for _, f := range p.Files {
		rel := f.Rel
		switch {
		case f.Kind == Header && !gocpp:
			continue
		case f.Kind == Source:
			if !gocpp && isCppPath(rel) {
				continue
			}
			rel += ext
		}
		if err := writeFile(filepath.Join(dir, filepath.FromSlash(rel)), f.Content); err != nil {
			return err
		}
	}
	return nil
}

// isCppPath reports whether the package of rel is gocpp only (it imports C++ headers, or uses the
// forms of Phase 10): Go cannot read it.
func isCppPath(rel string) bool {
	return strings.HasPrefix(rel, "cpp") || strings.HasPrefix(rel, "mac")
}

func writeFile(path, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0o644)
}

// nouns name the types, so that the code reads like code.
var nouns = []string{"Order", "Cache", "Queue", "Ledger", "Widget", "Session", "Account", "Parcel", "Route", "Ticket",
	"Invoice", "Buffer", "Worker", "Stream", "Record", "Channel", "Region", "Policy", "Sensor", "Report"}

// Generate makes the project of spec. The same spec gives the same files.
func Generate(spec Spec) *Project {
	if spec.Module == "" {
		spec.Module = "example.com/bench/" + spec.Name
	}
	g := &generator{spec: spec, rng: rand.New(rand.NewPCG(spec.Seed, spec.Seed^0x9e3779b97f4a7c15))}
	return g.generate()
}

type generator struct {
	spec  Spec
	rng   *rand.Rand
	files []File
}

func pkgName(i int) string { return fmt.Sprintf("pkg%03d", i) }

// deps returns the packages that package i imports: up to three lower ones.
func (g *generator) deps(i int) []int {
	if i == 0 {
		return nil
	}
	n := min(i, 1+g.rng.IntN(3))
	seen := map[int]bool{}
	var out []int
	for len(out) < n {
		d := g.rng.IntN(i)
		if i > 1 && g.rng.IntN(3) > 0 {
			d = max(i-1-g.rng.IntN(min(i, 4)), 0) // mostly close ones: a chain with fan-in
		}
		if !seen[d] {
			seen[d] = true
			out = append(out, d)
		}
	}
	sort.Ints(out)
	return out
}

// hotIndexes returns the package and the file number of the hot file (Project.Hot) of the spec: a
// package in the middle of the dependency chain, its fourth file (or its last).
func hotIndexes(s Spec) (pkg, file int) {
	return s.Packages / 2, min(3, s.Files-1)
}

func (g *generator) add(rel, content string, kind FileKind) {
	g.files = append(g.files, File{Rel: rel, Content: content, Kind: kind})
}

func (g *generator) generate() *Project {
	s := g.spec
	p := &Project{Spec: s, Module: s.Module}
	g.add("core/core", g.core(), Source)
	g.add("core/log", g.coreLog(), Source)
	p.Core = "core/core"
	allDeps := make([][]int, s.Packages)
	for i := range s.Packages {
		allDeps[i] = g.deps(i)
		for k := range s.Files {
			g.add(fmt.Sprintf("%s/f%02d", pkgName(i), k), g.file(i, k, allDeps[i]), Source)
		}
	}
	for i := range s.CppPackages {
		g.cppPackage(i)
	}
	if s.CppPackages > 0 {
		p.Typo, p.CppFile = "cpptypo/typo", "cpp00/f00"
		g.add(p.Typo, g.cppTypo(), Source)
	}
	for i := range s.MacroPackages {
		g.macroPackage(i)
	}
	if s.MacroPackages > 0 {
		p.MacroFile = fmt.Sprintf("mac%02d/f%02d", min(3, s.MacroPackages-1), min(2, g.macroFiles()-1))
	}
	p.Big = "bigfile/big"
	g.add(p.Big, g.bigFile(), Source)
	p.APIFile, p.APIUser = fmt.Sprintf("%s/f00", pkgName(s.Packages-1)), "cmd/app/main"
	g.add("cmd/app/main", g.mainFile(allDeps), Source)

	hot, hotFile := hotIndexes(s)
	p.HotPkg = pkgName(hot)
	p.Hot = fmt.Sprintf("%s/f%02d", p.HotPkg, hotFile)
	dep := 0
	if len(allDeps[hot]) > 0 {
		dep = allDeps[hot][len(allDeps[hot])-1]
	}
	p.Dep = fmt.Sprintf("%s/f%02d", pkgName(dep), min(1, s.Files-1))
	p.Dependent = fmt.Sprintf("%s/f%02d", pkgName(min(hot+1, s.Packages-1)), min(2, s.Files-1))
	for i := hot + 1; i < s.Packages; i++ { // the first package after hot that imports it
		imports := false
		for _, d := range allDeps[i] {
			imports = imports || d == hot
		}
		if imports {
			p.Dependent = fmt.Sprintf("%s/f%02d", pkgName(i), min(2, s.Files-1))
			break
		}
	}
	p.Queries = []string{"New" + typeName(0, 0), "Sum" + typeName(hot, hotFile), typeName(s.Packages-1, 0), "Label"}
	sort.Slice(g.files, func(i, j int) bool { return g.files[i].Rel < g.files[j].Rel })
	p.Files = g.files
	for _, f := range p.Files {
		if f.Kind == Source {
			p.Lines += strings.Count(f.Content, "\n")
		}
	}
	return p
}

func (g *generator) core() string {
	return `// Package core has the types that every package of the project uses.
package core

import (
	"errors"
	"fmt"
)

// ErrEmpty is returned for an empty name or tag.
var ErrEmpty = errors.New("core: empty")

// ErrMissing is wrapped by the errors of failed lookups.
var ErrMissing = errors.New("core: missing")

// Namer is implemented by everything that has a name.
type Namer interface {
	Name() string
}

// Adder is implemented by every collection of the project (a method name
// that many types share, and an interface that all of them satisfy).
type Adder interface {
	Add(tag string) (int, error)
}

// Base is embedded in the types of the project: an ID and a name.
type Base struct {
	ID   int
	name string
}

var nextID int

// NewBase returns a Base with a new ID.
func NewBase(name string) Base {
	nextID++
	return Base{ID: nextID, name: name}
}

// Name returns the name of the base.
func (b *Base) Name() string { return b.name }

// Describe returns a one-line description.
func (b *Base) Describe() string { return fmt.Sprintf("%s#%d", b.name, b.ID) }

// Stats counts items by size.
type Stats struct {
	Empty, Small, Large int
	Total               int
}

// Add adds the counts of o.
func (s *Stats) Add(o Stats) {
	s.Empty += o.Empty
	s.Small += o.Small
	s.Large += o.Large
	s.Total += o.Total
}

// Clamp returns n limited to lo..hi.
func Clamp(n, lo, hi int) int {
	if n < lo {
		return lo
	}
	if n > hi {
		return hi
	}
	return n
}

// Label returns s without spaces, for use as a tag.
func Label(s string) string {
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		if s[i] != ' ' {
			out = append(out, s[i])
		}
	}
	return string(out)
}
`
}

func (g *generator) coreLog() string {
	return `package core

import "fmt"

// Level is the importance of a log message.
type Level int

// The levels.
const (
	Debug Level = iota
	Info
	Warn
)

// Logger receives messages.
type Logger interface {
	Logf(level Level, format string, args ...any)
}

// Event is a message with its level; every package sends them.
type Event struct {
	Level Level
	Text  string
}

// String formats the event.
func (e Event) String() string { return fmt.Sprintf("[%d] %s", e.Level, e.Text) }

// Emit sends an event to l.
func Emit(l Logger, level Level, format string, args ...any) Event {
	e := Event{Level: level, Text: fmt.Sprintf(format, args...)}
	if l != nil {
		l.Logf(level, "%s", e.Text)
	}
	return e
}
`
}

// A body is a generated file under construction: its code and the imports
// it needs.
type body struct {
	b       strings.Builder
	imports map[string]bool
}

func (b *body) printf(format string, args ...any) { fmt.Fprintf(&b.b, format, args...) }
func (b *body) use(path string)                   { b.imports[path] = true }

// file returns the file k of package i, which imports the packages deps.
func (g *generator) file(i, k int, deps []int) string {
	noun := nouns[(i*7+k*3)%len(nouns)]
	T := typeName(i, k)
	lower := strings.ToLower(T)
	b := &body{imports: map[string]bool{}}
	b.use("fmt")
	b.use("strings")
	b.use("sync")
	b.use(g.spec.Module + "/core")
	full := !g.spec.Lite
	// The hot file (Project.Hot) of a project of short files has the method Collect too: the
	// rename-method row renames it (a method that no interface lists).
	hotPkg, hotK := hotIndexes(g.spec)
	hotFile := i == hotPkg && k == hotK

	b.printf("// %s keeps a set of tags for one %s.\ntype %s struct {\n\tcore.Base\n\tTitle string\n\tCount int\n\tTags  []string\n\tindex map[string]int\n\tmu    sync.Mutex\n}\n\n", T, strings.ToLower(noun), T)
	b.printf("// New%s returns an empty %s called name.\nfunc New%s(name string) *%s {\n\treturn &%s{Base: core.NewBase(name), Title: name, index: map[string]int{}}\n}\n\n", T, T, T, T, T)
	b.printf("// Add records tag and returns its position.\nfunc (r *%s) Add(tag string) (int, error) {\n\tif tag == \"\" {\n\t\treturn -1, core.ErrEmpty\n\t}\n\tr.mu.Lock()\n\tdefer r.mu.Unlock()\n\tif i, ok := r.index[tag]; ok {\n\t\treturn i, nil\n\t}\n\tr.Tags = append(r.Tags, tag)\n\tr.index[tag] = len(r.Tags) - 1\n\tr.Count++\n\treturn len(r.Tags) - 1, nil\n}\n\n", T)
	if full {
		b.printf("// Find returns the position of tag, or an error that wraps core.ErrMissing.\nfunc (r *%s) Find(tag string) (int, error) {\n\tr.mu.Lock()\n\tdefer r.mu.Unlock()\n\tif i, ok := r.index[tag]; ok {\n\t\treturn i, nil\n\t}\n\treturn -1, fmt.Errorf(\"%s: %%q: %%w\", tag, core.ErrMissing)\n}\n\n", T, lower)
	}
	b.printf("// String implements fmt.Stringer.\nfunc (r *%s) String() string {\n\treturn fmt.Sprintf(\"%s(%%s: %%s)\", r.Title, strings.Join(r.Tags, \",\"))\n}\n\n", T, T)
	if k == 0 {
		b.printf("// Label returns the tag form of s, for the packages that use this one.\nfunc Label(s string) string { return core.Label(strings.TrimSpace(s)) }\n\n")
	}
	if full {
		b.printf("// Self returns the value itself, which only a %s can: %sLike is satisfied by no other type.\nfunc (r *%s) Self() *%s { return r }\n\n", T, T, T, T)
		b.printf("// %sLike is what the users of a %s need.\ntype %sLike interface {\n\tAdd(tag string) (int, error)\n\tFind(tag string) (int, error)\n\tSelf() *%s\n\tcore.Namer\n}\n\nvar _ %sLike = (*%s)(nil)\n\n", T, T, T, T, T, T)
	}
	if full || hotFile {
		b.printf("// Collect applies f to every tag in its own goroutine and gathers the results.\nfunc (r *%s) Collect(f func(string) int) []int {\n\tout := make(chan int, len(r.Tags))\n\tvar wg sync.WaitGroup\n\tfor _, t := range r.Tags {\n\t\twg.Add(1)\n\t\tgo func(t string) {\n\t\t\tdefer wg.Done()\n\t\t\tout <- f(t)\n\t\t}(t)\n\t}\n\twg.Wait()\n\tclose(out)\n\tvar res []int\n\tfor v := range out {\n\t\tres = append(res, v)\n\t}\n\treturn res\n}\n\n", T)
	}
	// Sum calls a function of a package that this one imports.
	label := "core.Label"
	if len(deps) > 0 {
		d := deps[k%len(deps)]
		b.use(g.spec.Module + "/" + pkgName(d))
		label = pkgName(d) + ".Label"
	}
	recv := "[]*" + T
	if full {
		recv = "[]" + T + "Like"
	}
	b.printf("// Sum%s adds a tag labelled by the package that this one builds on.\nfunc Sum%s(items %s, tag string) (int, error) {\n\tsum := 0\n\tfor _, it := range items {\n\t\tn, err := it.Add(%s(tag))\n\t\tif err != nil {\n\t\t\treturn 0, fmt.Errorf(\"sum %s: %%w\", err)\n\t\t}\n\t\tsum += n\n\t}\n\treturn sum, nil\n}\n\n", T, T, recv, label, lower)
	if full {
		b.printf("// Stats%s summarises the counts of items.\nfunc Stats%s(items []*%s) core.Stats {\n\tvar st core.Stats\n\tfor _, it := range items {\n\t\tswitch {\n\t\tcase it.Count == 0:\n\t\t\tst.Empty++\n\t\tcase it.Count < 4:\n\t\t\tst.Small++\n\t\tdefault:\n\t\t\tst.Large++\n\t\t}\n\t\tst.Total += it.Count\n\t}\n\treturn st\n}\n", T, T, T)
	}
	if full {
		b.printf("\n// Drain%s reads values from in until it is closed or done fires.\nfunc Drain%s(in <-chan int, done <-chan struct{}) int {\n\ttotal := 0\n\tfor {\n\t\tselect {\n\t\tcase v, ok := <-in:\n\t\t\tif !ok {\n\t\t\t\treturn total\n\t\t\t}\n\t\t\ttotal += v\n\t\tcase <-done:\n\t\t\treturn total\n\t\t}\n\t}\n}\n", T, T)
		// A few filler functions, so that files differ in size.
		for n := range g.rng.IntN(3) {
			b.printf("\n// Weigh%s%d scores a tag by its length and the position of its first space.\nfunc Weigh%s%d(tag string) int {\n\tscore := len(tag)\n\tif i := strings.IndexByte(tag, ' '); i >= 0 {\n\t\tscore += i\n\t}\n\tfor _, c := range tag {\n\t\tif c >= 'a' && c <= 'z' {\n\t\t\tscore++\n\t\t}\n\t}\n\treturn core.Clamp(score, 0, %d)\n}\n", T, n, T, n, 50+g.rng.IntN(50))
		}
	}
	return header(g.spec.Module, pkgName(i), b)
}

// header returns the file text: the package clause, the imports (standard
// ones first, then the project's, each sorted) and the body.
func header(module, pkg string, b *body) string {
	var std, own []string
	for path := range b.imports {
		if strings.HasPrefix(path, module) {
			own = append(own, path)
		} else {
			std = append(std, path)
		}
	}
	sort.Strings(std)
	sort.Strings(own)
	var out strings.Builder
	fmt.Fprintf(&out, "package %s\n\nimport (\n", pkg)
	for _, p := range std {
		fmt.Fprintf(&out, "\t%q\n", p)
	}
	if len(std) > 0 && len(own) > 0 {
		out.WriteString("\n")
	}
	for _, p := range own {
		fmt.Fprintf(&out, "\t%q\n", p)
	}
	out.WriteString(")\n\n")
	out.WriteString(b.b.String())
	return out.String()
}

// mainFile is the program: it builds a value of the first file of a few
// packages and prints them.
func (g *generator) mainFile(deps [][]int) string {
	b := &body{imports: map[string]bool{"fmt": true}}
	b.printf("func main() {\n")
	shown := 0
	for i := g.spec.Packages - 1; i >= 0 && shown < 3; i-- {
		noun := nouns[(i*7)%len(nouns)]
		T := fmt.Sprintf("%s%d", noun, 0)
		b.use(g.spec.Module + "/" + pkgName(i))
		b.printf("\tv%d := %s.New%s(%q)\n\tif _, err := v%d.Add(%s.Label(\" tag \")); err != nil {\n\t\tfmt.Println(\"error:\", err)\n\t}\n\tfmt.Println(v%d)\n", i, pkgName(i), T, "main"+fmt.Sprint(i), i, pkgName(i), i)
		shown++
	}
	b.printf("}\n")
	return header(g.spec.Module, "main", b)
}

// cppPackage adds package cpp<i>: files that use std containers through
// header imports, and a header of the package's own.
func (g *generator) cppPackage(i int) {
	dir := fmt.Sprintf("cpp%02d", i)
	g.add(dir+"/shape.hpp", "#pragma once\n\nstruct Box {\n  int w = 0;\n  int h = 0;\n  int area() const { return w * h; }\n};\n\ninline int perimeter(const Box& b) { return 2 * (b.w + b.h); }\n", Header)
	for k := range 3 {
		b := &body{imports: map[string]bool{"fmt": true}}
		b.use("<map>")
		b.use("<string>")
		b.use("<vector>")
		b.use("<algorithm>")
		b.use("./shape.hpp")
		b.printf("// Count%d_%d counts the words of text with a std::map.\nfunc Count%d_%d(words []string) int {\n\tcounts := std.map[std.string, int]{}\n\tfor _, w := range words {\n\t\tcounts[std.string(w)]++\n\t}\n\treturn int(counts.size())\n}\n\n", i, k, i, k)
		b.printf("// Sorted%d_%d sorts numbers with std::sort.\nfunc Sorted%d_%d(in []int) []int {\n\tv := std.vector[int]{}\n\tfor _, n := range in {\n\t\tv.push_back(n)\n\t}\n\tstd.sort(v.begin(), v.end())\n\tvar out []int\n\tfor _, n := range v {\n\t\tout = append(out, int(n))\n\t}\n\treturn out\n}\n\n", i, k, i, k)
		b.printf("// Area%d_%d uses the box of the package's own header.\nfunc Area%d_%d(w, h int) string {\n\tbox := shape.Box{}\n\tbox.w = w\n\tbox.h = h\n\treturn fmt.Sprint(box.area(), shape.perimeter(box))\n}\n", i, k, i, k)
		g.add(fmt.Sprintf("%s/f%02d", dir, k), headerCpp(dir, b), Source)
	}
}

// headerCpp is header for the C++ packages: header imports are plain
// strings after the gocpp imports, like examples/cpp-std.
func headerCpp(pkg string, b *body) string {
	var std, hdr []string
	for path := range b.imports {
		if strings.HasPrefix(path, "<") || strings.HasPrefix(path, "./") {
			hdr = append(hdr, path)
		} else {
			std = append(std, path)
		}
	}
	sort.Strings(std)
	sort.Strings(hdr)
	var out strings.Builder
	fmt.Fprintf(&out, "package %s\n\nimport (\n", pkg)
	for _, p := range std {
		fmt.Fprintf(&out, "\t%q\n", p)
	}
	out.WriteString("\n")
	for _, p := range hdr {
		fmt.Fprintf(&out, "\t%q\n", p)
	}
	out.WriteString(")\n\n")
	out.WriteString(b.b.String())
	return out.String()
}

// cppTypo is a package whose namespace name has a typo (stdd): the namespace probe
// finds it, in the background, and the error comes when it is done.
func (g *generator) cppTypo() string {
	return "package cpptypo\n\nimport (\n\t\"<string>\"\n)\n\n// Name returns a std::string.\nfunc Name() int {\n\ts := stdd.string(\"x\")\n\treturn int(s.size())\n}\n"
}

// AnchorNeedles are the texts that every generated file of a package has
// (the scenarios search for them to place their edits and requests).
var AnchorNeedles = []string{
	"\tr.mu.Lock()\n",
	"\tsum := 0\n",
	"fmt.Sprintf(",
	"strings.Join(",
	"core.NewBase(",
	"func (r *",
}

// bigFile is a file of about a thousand lines: forty functions with loops,
// a switch, a map and errors.
func (g *generator) bigFile() string {
	var b strings.Builder
	b.WriteString("// Package bigfile has one long file, for the format-on-save budget.\npackage bigfile\n\nimport (\n\t\"errors\"\n\t\"fmt\"\n)\n")
	for n := range 40 {
		fmt.Fprintf(&b, "\n// Compute%[1]d folds the values by a rotating rule.\nfunc Compute%[1]d(vals []int, limit int) (int, error) {\n\tif len(vals) == 0 {\n\t\treturn 0, errors.New(\"compute%[1]d: no values\")\n\t}\n\tacc := 0\n\tseen := map[int]bool{}\n\tfor i, v := range vals {\n\t\tswitch {\n\t\tcase v < 0:\n\t\t\tacc -= v * (i + %[2]d)\n\t\tcase v%%2 == 0:\n\t\t\tacc += v / 2\n\t\tdefault:\n\t\t\tacc += v*3 + %[3]d\n\t\t}\n\t\tif seen[v] {\n\t\t\tcontinue\n\t\t}\n\t\tseen[v] = true\n\t\tif acc > limit {\n\t\t\treturn acc, fmt.Errorf(\"compute%[1]d: %%d over limit %%d\", acc, limit)\n\t\t}\n\t}\n\treturn acc, nil\n}\n", n, 1+g.rng.IntN(5), 1+g.rng.IntN(9))
	}
	return b.String()
}

// macroPackage adds the package mac<i>. mac00 declares the macros; the others use them, with `!` calls,
// must functions (which handle the calls that drop their errors), lazy arguments, a macro on top of a func,
// and #Derive.
func (g *generator) macroPackage(i int) {
	dir := fmt.Sprintf("mac%02d", i)
	if i == 0 {
		g.add(dir+"/macros", macroDecls(g.spec.Module), Source)
		return
	}
	for k := range g.macroFiles() {
		g.add(fmt.Sprintf("%s/f%02d", dir, k), macroFile(g.spec.Module, dir, i, k), Source)
	}
}

func (g *generator) macroFiles() int {
	if g.spec.MacroFiles > 0 {
		return g.spec.MacroFiles
	}
	return g.spec.Files
}

// macroDecls is mac00: a macro of every shape.
func macroDecls(module string) string {
	return `// Package mac00 declares the macros that the other macro packages use.
package mac00

import (
	"fmt"
	"os"
	"sync"
)

// Assert panics with the place of the use when cond is false. msg is lazy: it is evaluated only then.
macro Assert(cond bool, msg lazy string) {
	if !cond {
		panic(fmt.Sprintf("%s:%d: assertion failed: %s: %s", #file(), #line(), #text(cond), msg()))
	}
}

// Dbg prints an expression, its text and its value, and gives the value back.
macro Dbg[T any](v T) T {
	fmt.Fprintf(os.Stderr, "[%s:%d] %s = %v\n", #file(), #line(), #text(v), v)
	return v
}

// Trace prints when the function it stands on starts and ends.
macro Trace() func() {
	fmt.Println("enter", #func())
	return func() { fmt.Println("exit ", #func()) }
}

// Locked holds mu while the function runs.
macro Locked(mu *sync.Mutex) func() {
	mu.Lock()
	return mu.Unlock
}

// Count counts the calls of the function it stands on, and runs it.
macro Count(n *int) {
	*n++
	#body()
}
`
}

// macroFile is file k of macro package i: eight functions that use the forms of Phase 10.
func macroFile(module, pkg string, i, k int) string {
	var b strings.Builder
	t := fmt.Sprintf("%s%d", nouns[(i*5+k)%len(nouns)], k)
	fmt.Fprintf(&b, "package %s\n\nimport (\n\t\"fmt\"\n\t\"os\"\n\t\"strconv\"\n\t\"strings\"\n\t\"sync\"\n\n\t\"%s/mac00\"\n)\n\n", pkg, module)
	fmt.Fprintf(&b, `var (
	mu%[1]s    sync.Mutex
	calls%[1]s int
)

// %[1]s is a record; #Derive writes its Equal and Clone.
#Derive(Equal, Clone)
type %[1]s struct {
	ID   int
	Name string
	Tags []string
}

// Load%[1]s reads a number from a file. It is a must function: the errors of the calls that drop
// them are returned.
func Load%[1]s!(path string) (n int) {
	f := os.Open(path)
	defer f.Close()
	data := os.ReadFile(path)
	n = strconv.Atoi(strings.TrimSpace(string(data)))
	return n
}

// Parse%[1]s reads two numbers; the errors of both are returned by the ! calls.
func Parse%[1]s(a, b string) (int, error) {
	x := strconv.Atoi!(a)
	y := strconv.Atoi!(b)
	#mac00.Assert(x >= 0, fmt.Sprint("negative: ", x))
	return x + y, nil
}

// Sum%[1]s uses the macros inside code.
func Sum%[1]s(path string, extra int) (int, error) {
	n := Load%[1]s!(path)
	total := #mac00.Dbg(n + extra)
	#mac00.Assert(total != 0, "zero")
	return total, nil
}

// Guard%[1]s has macros on top of it.
#mac00.Trace()
#mac00.Locked(&mu%[1]s)
func Guard%[1]s(r %[1]s) %[1]s {
	return r.Clone()
}

// Counted%[1]s is wrapped by Count.
#mac00.Count(&calls%[1]s)
func Counted%[1]s(a int) int {
	return a * 2
}

// Must%[1]s panics on an error: it has no error result.
func Must%[1]s(s string) int {
	return strconv.Atoi!(s)
}

// Close%[1]s closes the file and returns the error of the close if nothing failed before.
func Close%[1]s(path string) (int, error) {
	f := os.Open!(path)
	defer f.Close!()
	st := f.Stat!()
	return int(st.Size()), nil
}

// Same%[1]s compares two records with the derived method.
func Same%[1]s(a, b %[1]s) bool {
	return a.Equal(b)
}
`, t)
	return b.String()
}
