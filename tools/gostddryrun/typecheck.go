package main

// The type-checking half of the dry run (job 16.1b-tc): load the files that
// select.go picks for each package under the 16.1b tag set and check them with
// go/types.
//
// The importer is a source importer over the tool's own selection, not
// go/importer.Default: it resolves an import the way the gocpp loader does
// (select.go's resolveImport: internal/... under GOROOT/src, a vendored package
// keeps its vendor/ path, an importer inside vendor/ resolves against
// vendor/), selects the files with the same go/build.Context and tags, and
// checks the package recursively with the same go/types configuration. A
// package that cannot be found or has no file under the constraints is a
// finding of the run, not a crash of the tool.
//
// The errors are split in two lists. A real error is a file that does not
// check under the gocpp constraints: the blocking list of steps 16.9 to 16.11.
// An expected gap is an error that follows from the constraints themselves: an
// undefined name declared in a file that the gocpp tag (or CGO_ENABLED=0)
// drops, an import of a package that the gocpp tag empties, or an error of a
// dependency (reported where that dependency is checked). Bodyless
// declarations and the compiler intrinsics are not errors at all under
// go/types; their counts are copied from the syntax dry run.

import (
	"errors"
	"fmt"
	"go/ast"
	"go/build"
	"go/parser"
	"go/scanner"
	"go/token"
	"go/types"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

const typeCheckSchemaVersion = 1

const typeCheckTool = "gostddryrun-typecheck"

// importerDescription is what the JSON says about the importer, so that a run
// can be read without the source of this file.
const importerDescription = "source importer over the tool's own file selection: " +
	"GOROOT/src with the 16.1b build.Context and tags, per-importing-package " +
	"vendor resolution (resolveImport in select.go), recursive and cached; " +
	"unsafe is go/types' predeclared package; a missing package or one with no " +
	"file under the constraints is reported, not fatal"

// A TypeError is one real error: a file that does not check under the gocpp
// constraints. File is relative to GOROOT and uses forward slashes.
type TypeError struct {
	File string `json:"file"`
	Line int    `json:"line"`
	Col  int    `json:"col"`
	Kind string `json:"kind"` // undefined, import_not_found, unused_import, parse, other
	Msg  string `json:"msg"`
}

// An ExpectedGap is a `go/types` error that the constraints themselves cause.
// It is not a real type error. Bodyless declarations and the intrinsics are
// legal Go and never produce an error; their counts are copied from the syntax
// dry run (TypeCheckPackage.Bodyless and Intrinsics).
type ExpectedGap struct {
	Kind string `json:"kind"` // compiler_tag_dropped_file, cgo_file, import_dropped_by_compiler_tag, dependency_failed
	File string `json:"file"`
	Line int    `json:"line"`
	Col  int    `json:"col"`
	Name string `json:"name"`
	Msg  string `json:"msg"`
}

// An ImportFailure is one import that the source importer could not resolve
// or check. Kind is not_found (no such package in GOROOT/src), no_files (the
// package exists but has no selected file for this target),
// dropped_by_compiler_tag (it has files under gc and none under gocpp),
// failed (the dependency's own check failed) or cycle.
type ImportFailure struct {
	From     string `json:"from"`
	Import   string `json:"import"`
	Resolved string `json:"resolved"`
	Kind     string `json:"kind"`
	Detail   string `json:"detail"`
}

// A TypeCheckPackage is the type check of one package. InSet says whether the
// package belongs to the dry run's package set; a package loaded only as an
// import is reported with in_set false (and no bodyless/intrinsics counts,
// which come from the syntax dry run).
type TypeCheckPackage struct {
	Path           string        `json:"path"`
	Name           string        `json:"name"`
	Dir            string        `json:"dir"` // relative to GOROOT
	InSet          bool          `json:"in_set"`
	Status         string        `json:"status"` // checked, no_files, parse_error
	Files          int           `json:"files"`
	Bodyless       int           `json:"bodyless"`
	Intrinsics     int           `json:"intrinsics"`
	CgoFiles       []string      `json:"cgo_files"`
	DroppedByGocpp []string      `json:"dropped_by_gocpp"`
	Errors         []TypeError   `json:"errors"`
	Expected       []ExpectedGap `json:"expected"`
	ImportsFailed  []string      `json:"imports_failed"`
}

// A TypeCheckCount is the number of errors or gaps of one kind.
type TypeCheckCount struct {
	Kind  string `json:"kind"`
	Count int    `json:"count"`
}

// TypeCheckSummary is the whole run.
type TypeCheckSummary struct {
	Packages           int              `json:"packages"`
	PackagesInSet      int              `json:"packages_in_set"`
	Checked            int              `json:"checked"`
	NoFiles            int              `json:"no_files"`
	ParseErrors        int              `json:"parse_errors"`
	FailedPackages     int              `json:"failed_packages"`
	RealErrors         int              `json:"real_errors"`
	ErrorsByKind       []TypeCheckCount `json:"errors_by_kind"`
	ExpectedGaps       int              `json:"expected_gaps"`
	ExpectedByKind     []TypeCheckCount `json:"expected_by_kind"`
	ImportFailures     int              `json:"import_failures"`
	ImportFailureKinds []TypeCheckCount `json:"import_failure_kinds"`
	BodylessInSet      int              `json:"bodyless_in_set"`
	IntrinsicsInSet    int              `json:"intrinsics_in_set"`
	CgoFilesInSet      int              `json:"cgo_files_in_set"`
	DroppedFilesInSet  int              `json:"dropped_by_gocpp_in_set"`
}

// TypeCheckResult is one type-check run, the content of one JSON file.
type TypeCheckResult struct {
	Schema            int                `json:"schema"`
	Tool              string             `json:"tool"`
	GoVersion         string             `json:"go_version"`
	GoLanguageVersion string             `json:"go_language_version"`
	ToolGoVersion     string             `json:"tool_go_version"`
	GOROOT            string             `json:"goroot"`
	Sizes             string             `json:"sizes"`
	Importer          string             `json:"importer"`
	Tags              *TagSet            `json:"tags"`
	PackageSet        []string           `json:"package_set"`
	Packages          []TypeCheckPackage `json:"packages"`
	ImportFailures    []ImportFailure    `json:"import_failures"`
	Summary           TypeCheckSummary   `json:"summary"`
}

// tcEntry is the cached check of one resolved package path; the cache key is
// that path.
type tcEntry struct {
	dir      string
	name     string
	status   string
	nfiles   int
	pkg      *types.Package
	err      error
	errors   []TypeError
	expected []ExpectedGap
	loading  bool
}

// typeChecker is one run: one tag set, one shared FileSet, one cache. It is
// sequential on purpose: the result must be byte-identical between runs, and
// go/types is fast enough that the determinism is worth more than the
// parallelism.
type typeChecker struct {
	goroot    string
	ctxt      *build.Context
	tags      *TagSet
	goVersion string
	sizes     types.Sizes
	fset      *token.FileSet
	cache     map[string]*tcEntry
	failures  []ImportFailure
}

// tcImporter is the go/types importer of one importing package. The same
// package path can resolve to different vendored packages depending on the
// importer, so the importer carries From.
type tcImporter struct {
	c    *typeChecker
	from string
}

func (i tcImporter) Import(path string) (*types.Package, error) {
	return i.c.importFrom(i.from, path)
}

// typeCheck runs the type check of the packages of res under res.Tags.
func typeCheck(goroot string, res *Result) (*TypeCheckResult, error) {
	tags := res.Tags
	sizes := types.SizesFor("gc", tags.GOARCH)
	if sizes == nil {
		return nil, fmt.Errorf("no gc sizes for %s/%s", tags.GOOS, tags.GOARCH)
	}
	c := &typeChecker{
		goroot:    goroot,
		ctxt:      newContext(goroot, tags, "gocpp"),
		tags:      tags,
		goVersion: res.GoMinor, // "go1.25"
		sizes:     sizes,
		fset:      token.NewFileSet(),
		cache:     map[string]*tcEntry{},
	}
	for _, p := range res.Packages {
		c.checkPath(p.Path)
	}
	out := c.result(res)
	return out, nil
}

// errImportCycle marks an import that closes a cycle. The package under
// construction is left alone (it is still being checked by its own caller);
// the importer that closes the cycle records the finding and reports the
// error at its import site.
var errImportCycle = errors.New("import cycle")

// checkPath loads and checks the package at the resolved GOROOT path. It
// always returns an entry; err is the reason the package has no usable
// *types.Package (for the importer to report at the import site).
func (c *typeChecker) checkPath(resolved string) (*tcEntry, error) {
	if e, ok := c.cache[resolved]; ok {
		if e.loading {
			return e, fmt.Errorf("%w through %s", errImportCycle, resolved)
		}
		return e, e.err
	}
	e := &tcEntry{dir: "src/" + resolved}
	c.cache[resolved] = e
	e.loading = true

	dir := filepath.Join(c.goroot, "src", filepath.FromSlash(resolved))
	files, err := selectDir(c.ctxt, dir)
	if err != nil {
		e.status = "no_files"
		e.err = fmt.Errorf("package %s: %v", resolved, err)
		e.loading = false
		return e, e.err
	}
	if len(files.GoFiles) == 0 {
		e.status = "no_files"
		e.err = fmt.Errorf("package %s has no Go file under the gocpp constraints", resolved)
		e.loading = false
		return e, e.err
	}
	e.nfiles = len(files.GoFiles)

	// The declarations of the files the constraints drop, so that an
	// "undefined: X" can be told apart from a real error. A name from a file
	// that the gocpp tag drops (compared with gc) or from a cgo file is an
	// expected gap; a name from any other excluded file is a real error, as it
	// would be under gc too.
	droppedNames, cgoNames := c.droppedDeclarations(dir, files)

	asts := make([]*ast.File, 0, len(files.GoFiles))
	parseFailed := false
	for _, name := range files.GoFiles {
		f, perr := parser.ParseFile(c.fset, filepath.Join(dir, name), nil, 0)
		if perr != nil {
			parseFailed = true
			c.addParseErrors(e, perr)
			continue
		}
		asts = append(asts, f)
		if e.name == "" {
			e.name = f.Name.Name
		}
	}
	if parseFailed {
		e.status = "parse_error"
		e.err = fmt.Errorf("package %s has files that do not parse", resolved)
		e.loading = false
		return e, e.err
	}

	conf := types.Config{
		GoVersion: c.goVersion,
		Sizes:     c.sizes,
		Importer:  tcImporter{c: c, from: resolved},
	}
	conf.Error = func(err error) {
		te, ok := err.(types.Error)
		if !ok {
			e.errors = append(e.errors, TypeError{Kind: "other", Msg: err.Error()})
			return
		}
		file, line, col := c.relPos(te.Fset.Position(te.Pos))
		if gap, ok := c.expectedGap(resolved, file, line, col, te.Msg, droppedNames, cgoNames); ok {
			e.expected = append(e.expected, gap)
			return
		}
		e.errors = append(e.errors, TypeError{File: file, Line: line, Col: col, Kind: errorKind(te.Msg), Msg: te.Msg})
	}
	pkg, err := conf.Check(resolved, c.fset, asts, nil)
	e.pkg = pkg
	e.status = "checked"
	e.err = nil
	if err != nil && len(e.errors) == 0 && len(e.expected) == 0 {
		// The error handler sees every error; this is a guard only.
		e.errors = append(e.errors, TypeError{Kind: "other", Msg: err.Error()})
	}
	e.loading = false
	sortTypeCheckErrors(e.errors)
	sortExpectedGaps(e.expected)
	return e, nil
}

// droppedDeclarations returns the package-scope names declared by the files
// that the constraints drop: names from the files the gocpp tag drops
// (compared with gc) and names from the cgo files (CGO_ENABLED=0).
func (c *typeChecker) droppedDeclarations(dir string, files *DirFiles) (dropped, cgo map[string]string) {
	dropped = map[string]string{}
	cgo = map[string]string{}
	names := func(names map[string]string, path string) {
		f, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			return
		}
		for _, n := range fileDecls(f) {
			names[n] = filepath.Base(path)
		}
	}
	byGocpp, _, err := droppedAdded(c.goroot, c.tags, dir)
	if err == nil {
		for _, name := range byGocpp {
			names(dropped, filepath.Join(dir, name))
		}
	}
	cgoFiles, err := cgoSelected(c.goroot, c.tags, dir)
	if err == nil {
		for _, name := range cgoFiles {
			names(cgo, filepath.Join(dir, name))
		}
	}
	return dropped, cgo
}

// fileDecls returns the package-scope names a file declares: functions
// without a receiver, and the names of type, var and const declarations.
func fileDecls(f *ast.File) []string {
	var out []string
	for _, d := range f.Decls {
		switch d := d.(type) {
		case *ast.FuncDecl:
			if d.Recv == nil {
				out = append(out, d.Name.Name)
			}
		case *ast.GenDecl:
			for _, s := range d.Specs {
				switch s := s.(type) {
				case *ast.TypeSpec:
					out = append(out, s.Name.Name)
				case *ast.ValueSpec:
					for _, n := range s.Names {
						out = append(out, n.Name)
					}
				}
			}
		}
	}
	return out
}

// expectedGap classifies one go/types error as an expected gap, if it is one.
func (c *typeChecker) expectedGap(from, file string, line, col int, msg string, dropped, cgo map[string]string) (ExpectedGap, bool) {
	gap := ExpectedGap{File: file, Line: line, Col: col, Msg: msg}
	if name, ok := strings.CutPrefix(msg, "undefined: "); ok {
		if f, ok := dropped[name]; ok {
			gap.Kind = "compiler_tag_dropped_file"
			gap.Name = name
			gap.Msg = "declared in " + f + ", which the gocpp tag drops"
			return gap, true
		}
		if f, ok := cgo[name]; ok {
			gap.Kind = "cgo_file"
			gap.Name = name
			gap.Msg = "declared in " + f + ", a cgo file dropped by CGO_ENABLED=0"
			return gap, true
		}
		return gap, false
	}
	if imp, ok := strings.CutPrefix(msg, "could not import "); ok {
		if i := strings.Index(imp, " ("); i >= 0 {
			imp = imp[:i]
		}
		kind := c.failureKind(from, imp)
		switch kind {
		case "dropped_by_compiler_tag":
			gap.Kind = "import_dropped_by_compiler_tag"
			gap.Name = imp
			return gap, true
		case "failed":
			gap.Kind = "dependency_failed"
			gap.Name = imp
			return gap, true
		}
		return gap, false
	}
	return gap, false
}

// errorKind names a real error for the counts. import_not_found is the
// importer's finding about the package itself; the other kinds are go/types'
// messages.
func errorKind(msg string) string {
	switch {
	case strings.HasPrefix(msg, "could not import "):
		return "import_not_found"
	case strings.HasPrefix(msg, "imported and not used"):
		return "unused_import"
	case strings.HasPrefix(msg, "undefined: "):
		return "undefined"
	default:
		return "other"
	}
}

// importFrom resolves and checks the import of one package, using the same
// selection and tags as the dry run.
func (c *typeChecker) importFrom(from, imp string) (*types.Package, error) {
	if imp == "unsafe" {
		return types.Unsafe, nil
	}
	if imp == "C" {
		c.failures = append(c.failures, ImportFailure{From: from, Import: imp, Kind: "not_found", Detail: "cgo import with CGO_ENABLED=0"})
		return nil, fmt.Errorf("cgo import \"C\" with CGO_ENABLED=0")
	}
	resolved := resolveImport(c.goroot, from, imp)
	if resolved == "" {
		c.failures = append(c.failures, ImportFailure{From: from, Import: imp, Kind: "not_found", Detail: "no package at GOROOT/src/" + imp})
		return nil, fmt.Errorf("package %s is not in GOROOT/src", imp)
	}
	e, err := c.checkPath(resolved)
	if err != nil {
		kind := "no_files"
		detail := err.Error()
		switch {
		case errors.Is(err, errImportCycle):
			kind = "cycle"
		case e != nil && e.status == "no_files" && c.hasGCFiles(resolved):
			kind = "dropped_by_compiler_tag"
			detail = "the gocpp tag drops every file of " + resolved
		case e != nil && e.status == "parse_error":
			kind = "failed"
		case e != nil && len(e.errors) > 0:
			kind = "failed"
		}
		c.failures = append(c.failures, ImportFailure{From: from, Import: imp, Resolved: resolved, Kind: kind, Detail: detail})
		return nil, err
	}
	return e.pkg, nil
}

// hasGCFiles reports whether the package at resolved has files under the gc
// compiler tag with every other constraint the same; when it does and gocpp
// has none, the compiler tag is what dropped them.
func (c *typeChecker) hasGCFiles(resolved string) bool {
	dir := filepath.Join(c.goroot, "src", filepath.FromSlash(resolved))
	files, err := selectDir(newContext(c.goroot, c.tags, "gc"), dir)
	if err != nil {
		return false
	}
	return len(files.GoFiles) > 0
}

// failureKind returns the kind of the failure recorded for from's import, or
// "" when none was recorded.
func (c *typeChecker) failureKind(from, imp string) string {
	for _, f := range c.failures {
		if f.From == from && f.Import == imp {
			return f.Kind
		}
	}
	return ""
}

// relPos makes a token position GOROOT-relative and slash-separated, so that
// the JSON does not depend on where the source tree lives.
func (c *typeChecker) relPos(pos token.Position) (string, int, int) {
	if pos.Filename == "" {
		return "", 0, 0
	}
	rel, err := filepath.Rel(c.goroot, pos.Filename)
	if err != nil {
		rel = pos.Filename
	}
	return filepath.ToSlash(rel), pos.Line, pos.Column
}

// addParseErrors records a parser error list with positions.
func (c *typeChecker) addParseErrors(e *tcEntry, err error) {
	list, ok := err.(scanner.ErrorList)
	if !ok {
		e.errors = append(e.errors, TypeError{Kind: "parse", Msg: err.Error()})
		return
	}
	for _, se := range list {
		file, line, col := c.relPos(se.Pos)
		e.errors = append(e.errors, TypeError{File: file, Line: line, Col: col, Kind: "parse", Msg: se.Msg})
	}
}

// result builds the output from the cache and the dry run, sorted and with no
// nil slice, so that two runs can be diffed byte for byte.
func (c *typeChecker) result(res *Result) *TypeCheckResult {
	inSet := map[string]*Package{}
	for i := range res.Packages {
		inSet[res.Packages[i].Path] = &res.Packages[i]
	}
	failuresByFrom := map[string][]string{}
	for _, f := range c.failures {
		failuresByFrom[f.From] = append(failuresByFrom[f.From], f.Import)
	}
	paths := make([]string, 0, len(c.cache))
	for p := range c.cache {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	out := &TypeCheckResult{
		Schema:            typeCheckSchemaVersion,
		Tool:              typeCheckTool,
		GoVersion:         res.GoVersion,
		GoLanguageVersion: res.GoMinor,
		ToolGoVersion:     runtime.Version(),
		GOROOT:            res.GOROOT,
		Sizes:             "gc/" + res.Tags.GOARCH,
		Importer:          importerDescription,
		Tags:              res.Tags,
		PackageSet:        res.PackageSet,
		Packages:          []TypeCheckPackage{},
		ImportFailures:    append([]ImportFailure{}, c.failures...),
	}
	for _, p := range paths {
		e := c.cache[p]
		rp := inSet[p]
		tp := TypeCheckPackage{
			Path:           p,
			Name:           e.name,
			Dir:            e.dir,
			InSet:          rp != nil,
			Status:         e.status,
			Files:          e.nfiles,
			CgoFiles:       []string{},
			DroppedByGocpp: []string{},
			Errors:         append([]TypeError{}, e.errors...),
			Expected:       append([]ExpectedGap{}, e.expected...),
			ImportsFailed:  []string{},
		}
		if rp != nil {
			tp.Bodyless = len(rp.Bodyless)
			tp.Intrinsics = len(rp.Intrinsics)
			tp.CgoFiles = append(tp.CgoFiles, rp.CgoFiles...)
			tp.DroppedByGocpp = append(tp.DroppedByGocpp, rp.DroppedByGocpp...)
			sort.Strings(tp.CgoFiles)
			sort.Strings(tp.DroppedByGocpp)
		}
		if imps := failuresByFrom[p]; len(imps) > 0 {
			tp.ImportsFailed = append(tp.ImportsFailed, imps...)
			sort.Strings(tp.ImportsFailed)
			tp.ImportsFailed = dedupe(tp.ImportsFailed)
		}
		if tp.Status == "" {
			tp.Status = "no_files"
		}
		out.Packages = append(out.Packages, tp)
	}
	sort.Slice(out.ImportFailures, func(i, j int) bool {
		a, b := out.ImportFailures[i], out.ImportFailures[j]
		if a.From != b.From {
			return a.From < b.From
		}
		if a.Import != b.Import {
			return a.Import < b.Import
		}
		return a.Kind < b.Kind
	})
	out.Summary = summarizeTypeCheck(out.Packages, out.ImportFailures)
	return out
}

// summarizeTypeCheck computes the counts of a TypeCheckResult.
func summarizeTypeCheck(pkgs []TypeCheckPackage, failures []ImportFailure) TypeCheckSummary {
	s := TypeCheckSummary{}
	errKinds := map[string]int{}
	gapKinds := map[string]int{}
	failKinds := map[string]int{}
	for _, p := range pkgs {
		s.Packages++
		if p.InSet {
			s.PackagesInSet++
			s.BodylessInSet += p.Bodyless
			s.IntrinsicsInSet += p.Intrinsics
			s.CgoFilesInSet += len(p.CgoFiles)
			s.DroppedFilesInSet += len(p.DroppedByGocpp)
		}
		switch p.Status {
		case "checked":
			s.Checked++
		case "no_files":
			s.NoFiles++
		case "parse_error":
			s.ParseErrors++
		}
		if len(p.Errors) > 0 {
			s.FailedPackages++
		}
		for _, e := range p.Errors {
			s.RealErrors++
			errKinds[e.Kind]++
		}
		for _, g := range p.Expected {
			s.ExpectedGaps++
			gapKinds[g.Kind]++
		}
	}
	for _, f := range failures {
		s.ImportFailures++
		failKinds[f.Kind]++
	}
	s.ErrorsByKind = sortedTypeCheckCounts(errKinds)
	s.ExpectedByKind = sortedTypeCheckCounts(gapKinds)
	s.ImportFailureKinds = sortedTypeCheckCounts(failKinds)
	return s
}

// sortedTypeCheckCounts returns the kind counts sorted by kind.
func sortedTypeCheckCounts(m map[string]int) []TypeCheckCount {
	out := make([]TypeCheckCount, 0, len(m))
	for k, n := range m {
		out = append(out, TypeCheckCount{Kind: k, Count: n})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Kind < out[j].Kind })
	return out
}

// sortTypeCheckErrors sorts the errors of one package by position and message.
func sortTypeCheckErrors(errs []TypeError) {
	sort.Slice(errs, func(i, j int) bool {
		a, b := errs[i], errs[j]
		if a.File != b.File {
			return a.File < b.File
		}
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		if a.Col != b.Col {
			return a.Col < b.Col
		}
		return a.Msg < b.Msg
	})
}

// sortExpectedGaps sorts the expected gaps of one package by position.
func sortExpectedGaps(gaps []ExpectedGap) {
	sort.Slice(gaps, func(i, j int) bool {
		a, b := gaps[i], gaps[j]
		if a.File != b.File {
			return a.File < b.File
		}
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		if a.Col != b.Col {
			return a.Col < b.Col
		}
		if a.Kind != b.Kind {
			return a.Kind < b.Kind
		}
		return a.Name < b.Name
	})
}

// writeTypeCheckMarkdown writes the type-check section of the README. tcs are
// the type-check runs next to the README (the fresh one included); the
// linux/amd64 run is the reference like the rest of the file.
func writeTypeCheckMarkdown(b *strings.Builder, res *Result, tcs []*TypeCheckResult) {
	if len(tcs) == 0 {
		return
	}
	byTarget := map[string]*TypeCheckResult{}
	for _, tc := range tcs {
		byTarget[tc.Tags.GOOS+"/"+tc.Tags.GOARCH] = tc
	}
	ref := byTarget[res.Tags.GOOS+"/"+res.Tags.GOARCH]
	if ref == nil {
		ref = byTarget["linux/amd64"]
	}
	if ref == nil {
		var keys []string
		for k := range byTarget {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		ref = byTarget[keys[0]]
	}
	var targets []string
	for t := range byTarget {
		targets = append(targets, t)
	}
	sort.Strings(targets)

	b.WriteString("## Type check (go/types)\n\n")
	fmt.Fprintf(b, "The selected files of every package of the dry run were parsed and checked with `go/types` and a\n")
	b.WriteString("source importer over the tool's own selection, not `go/importer.Default`: the importer resolves\n")
	b.WriteString("an import with the rules of `select.go` (`GOROOT/src`, `internal/...`, a vendored package keeps\n")
	b.WriteString("its `vendor/` path, an importer inside `vendor/` resolves against it), selects the files with\n")
	b.WriteString("the same `go/build.Context` and tags, and checks the package recursively, with a cache. Every\n")
	b.WriteString("error is kept with its position, relative to GOROOT. Configuration:\n")
	fmt.Fprintf(b, "`types.Config{GoVersion: %q, Sizes: types.SizesFor(\"gc\", %q)}`; the `go/types` of\n",
		ref.GoLanguageVersion, ref.Tags.GOARCH)
	fmt.Fprintf(b, "%s did the checking. A package that cannot be found, or that has no file under\n", ref.ToolGoVersion)
	b.WriteString("the constraints, is a finding of the run, not a failure of the tool.\n\n")

	b.WriteString("A **real type error** is a file that does not check under the gocpp constraints: the blocking\n")
	b.WriteString("list for steps 16.9 to 16.11. An **expected gap** follows from the constraints themselves: an\n")
	b.WriteString("undefined name declared in a file that the `gocpp` tag or `CGO_ENABLED=0` drops, an import of\n")
	b.WriteString("a package that the `gocpp` tag empties, or a dependency's own failure (reported where the\n")
	b.WriteString("dependency is checked). Bodyless declarations and the compiler intrinsics are legal Go and\n")
	b.WriteString("type-check; they are counted below and listed by the dry run above. The type check adds no\n")
	b.WriteString("construct of its own: the unsupported constructs of README 2.7 are the ones the sections\n")
	b.WriteString("above list (bodyless declarations, assembly, intrinsics, linknames, cgo, the tag-dropped\n")
	b.WriteString("files), because `go/types` accepts them as legal Go.\n\n")

	b.WriteString("| count |")
	for _, t := range targets {
		fmt.Fprintf(b, " %s |", t)
	}
	b.WriteString("\n|---|")
	for range targets {
		b.WriteString("---|")
	}
	b.WriteString("\n")
	rows := []struct {
		name string
		f    func(TypeCheckSummary) int
	}{
		{"packages in the dry run's set", func(s TypeCheckSummary) int { return s.PackagesInSet }},
		{"packages loaded (the set and its imports)", func(s TypeCheckSummary) int { return s.Packages }},
		{"packages checked", func(s TypeCheckSummary) int { return s.Checked }},
		{"packages with no file under the constraints", func(s TypeCheckSummary) int { return s.NoFiles }},
		{"packages with a parse error", func(s TypeCheckSummary) int { return s.ParseErrors }},
		{"packages with real type errors", func(s TypeCheckSummary) int { return s.FailedPackages }},
		{"real type errors", func(s TypeCheckSummary) int { return s.RealErrors }},
		{"expected gaps", func(s TypeCheckSummary) int { return s.ExpectedGaps }},
		{"import failures", func(s TypeCheckSummary) int { return s.ImportFailures }},
		{"bodyless declarations (expected, not errors)", func(s TypeCheckSummary) int { return s.BodylessInSet }},
		{"compiler intrinsics (expected, not errors)", func(s TypeCheckSummary) int { return s.IntrinsicsInSet }},
		{"cgo files dropped (expected)", func(s TypeCheckSummary) int { return s.CgoFilesInSet }},
		{"files dropped by the `gocpp` tag (expected)", func(s TypeCheckSummary) int { return s.DroppedFilesInSet }},
	}
	for _, row := range rows {
		fmt.Fprintf(b, "| %s |", row.name)
		for _, t := range targets {
			fmt.Fprintf(b, " %d |", row.f(byTarget[t].Summary))
		}
		b.WriteString("\n")
	}
	b.WriteString("\n")

	// Expected gaps by kind.
	kinds := map[string]bool{}
	for _, t := range targets {
		for _, k := range byTarget[t].Summary.ExpectedByKind {
			kinds[k.Kind] = true
		}
	}
	if len(kinds) > 0 {
		var sorted []string
		for k := range kinds {
			sorted = append(sorted, k)
		}
		sort.Strings(sorted)
		b.WriteString("Expected gaps by kind:\n\n| kind |")
		for _, t := range targets {
			fmt.Fprintf(b, " %s |", t)
		}
		b.WriteString("\n|---|")
		for range targets {
			b.WriteString("---|")
		}
		b.WriteString("\n")
		for _, k := range sorted {
			fmt.Fprintf(b, "| `%s` |", k)
			for _, t := range targets {
				n := 0
				for _, kc := range byTarget[t].Summary.ExpectedByKind {
					if kc.Kind == k {
						n = kc.Count
					}
				}
				fmt.Fprintf(b, " %d |", n)
			}
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	// Real errors are the blocking list: print every one.
	var realErrors []string
	for _, t := range targets {
		for _, p := range byTarget[t].Packages {
			for _, e := range p.Errors {
				realErrors = append(realErrors, fmt.Sprintf("%s %s:%d:%d: %s", t, p.Path, e.Line, e.Col, e.Msg))
			}
		}
	}
	if len(realErrors) == 0 {
		b.WriteString("No real type error: every selected package type-checks under the gocpp constraints.\n")
		b.WriteString("The input of steps 16.9 to 16.11 is therefore the translation work the dry run counts\n")
		b.WriteString("above (bodyless declarations, linknames, intrinsics, the overlays), not a repair of the\n")
		b.WriteString("type system. The expected-gap counts say whether the `gocpp` tag or `CGO_ENABLED=0`\n")
		b.WriteString("leaves a reference without its declaration in these packages.\n\n")
	} else {
		fmt.Fprintf(b, "The real type errors, all of them (target, package, file:line:col, message); position 0:0\n")
		b.WriteString("means an error without a source position. These are the blocking list for 16.9 to 16.11.\n\n")
		b.WriteString("```\n")
		for _, e := range realErrors {
			b.WriteString(e)
			b.WriteString("\n")
		}
		b.WriteString("```\n\n")
	}

	// The expected gaps, with the file that the constraints drop.
	var gaps []string
	for _, t := range targets {
		for _, p := range byTarget[t].Packages {
			for _, g := range p.Expected {
				gaps = append(gaps, fmt.Sprintf("%s %s %s:%d %s %s", t, p.Path, g.File, g.Line, g.Kind, g.Name))
			}
		}
	}
	if len(gaps) > 0 {
		b.WriteString("The expected gaps, all of them (target, package, file:line, kind, name):\n\n```\n")
		for _, g := range gaps {
			b.WriteString(g)
			b.WriteString("\n")
		}
		b.WriteString("```\n\n")
	}

	// Import failures.
	var failures []string
	for _, t := range targets {
		for _, f := range byTarget[t].ImportFailures {
			failures = append(failures, fmt.Sprintf("%s %s imports %s: %s", t, f.From, f.Import, f.Kind))
		}
	}
	if len(failures) > 0 {
		b.WriteString("Imports that the source importer could not load:\n\n```\n")
		for _, f := range failures {
			b.WriteString(f)
			b.WriteString("\n")
		}
		b.WriteString("```\n\n")
	}

	if ref.Summary.DroppedFilesInSet == 0 {
		b.WriteString("This set contains no file that the `gocpp` tag drops, so the overlay inputs of README 2.7\n")
		b.WriteString("are outside it. Naming them, `-packages internal/goroot,vendor/golang.org/x/sys/cpu`, shows\n")
		b.WriteString("the gap the overlay closes: `internal/goroot` then has no file under `gocpp`, and the\n")
		b.WriteString("undefined `cpuid`/`xgetbv` references of `golang.org/x/sys/cpu` become expected\n")
		b.WriteString("`compiler_tag_dropped_file` gaps, not real errors.\n\n")
	}

	b.WriteString("Limits of this run: `_test.go` files are not selected (the dry run is the non-test\n")
	b.WriteString("sources), so test-only references are not checked; the targets measured are\n")
	b.WriteString(codeList(targets))
	noArm := true
	for _, t := range targets {
		if strings.HasSuffix(t, "/arm64") {
			noArm = false
		}
	}
	if noArm {
		b.WriteString(" (no arm64 run)")
	}
	b.WriteString(";\nand module overlays outside GOROOT are not looked at.\n\n")
}

// writeTypeCheckJSON writes tc as stable, indented JSON.
func writeTypeCheckJSON(path string, tc *TypeCheckResult) error {
	return writeJSONFile(path, tc)
}

// readTypeCheckJSON reads a type-check file.
func readTypeCheckJSON(path string) (*TypeCheckResult, error) {
	var tc TypeCheckResult
	if err := readJSONFile(path, &tc); err != nil {
		return nil, err
	}
	if tc.Schema != typeCheckSchemaVersion {
		return nil, fmt.Errorf("%s: schema %d, want %d", path, tc.Schema, typeCheckSchemaVersion)
	}
	if tc.Tool != typeCheckTool {
		return nil, fmt.Errorf("%s: tool %q, want %q", path, tc.Tool, typeCheckTool)
	}
	return &tc, nil
}
