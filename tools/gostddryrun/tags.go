package main

// The build constraints of the GC engine's standard library (Phase 16 README
// 2.7): GOOS/GOARCH, the release tags, the compiler tag gocpp, purego and
// math_big_pure_go, and the target release's tool tags (its default
// goexperiment.* set and the architecture level).
//
// The tool tags must be the ones of the Go source tree that is being read, not
// the ones of the Go toolchain that built this tool. So they are read from that
// tree's src/internal/buildcfg (exp.go, zbootstrap.go, and the Flags fields of
// src/internal/goexperiment/flags.go), and an explicit table in this file pins
// the values of every Go release the tool knows, with the lines they were read
// from. For a known release the table and the tree must agree; a disagreement
// is an error, because the defaults changed and the table must be updated on
// purpose. For an unknown release the values are derived from the tree and the
// output says that no table entry checked them.
//
// The environment is never consulted: GOEXPERIMENT, GOAMD64, GOARM64 and
// GOOS/GOARCH from the running shell would make the dry run describe another
// target. GOOS and GOARCH come from -goos and -goarch only.

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// A TagSource is one place in the target tree that a default was read from.
type TagSource struct {
	File string `json:"file"` // relative to GOROOT, with forward slashes
	Line int    `json:"line"`
	What string `json:"what"`
}

// ToolDefaults is the tool tag configuration of one Go release, as read from
// that release's source.
type ToolDefaults struct {
	Version string `json:"version"` // "go1.25.14"
	Minor   string `json:"minor"`   // "go1.25"
	// Experiments is the enabled set for linux and windows on amd64 and arm64,
	// lower-case and sorted ("swissmap", not "SwissMap").
	Experiments         []string    `json:"experiments"`
	DefaultGOEXPERIMENT string      `json:"default_goexperiment"`
	DefaultGOAMD64      string      `json:"default_goamd64"`
	DefaultGOARM64      string      `json:"default_goarm64"`
	Table               string      `json:"table"`    // "go1.25", or "" when no entry exists
	Verified            bool        `json:"verified"` // the table entry agreed with the tree
	Sources             []TagSource `json:"sources"`
}

// knownDefaults pins the release defaults the tool was checked against. The
// values were read from the files and lines in Sources of Go 1.25.14; the
// source lines of a point release do not move, and a new minor version needs
// its own entry (the manager's yearly move, ADR-110).
var knownDefaults = map[string]ToolDefaults{
	"go1.25": {
		Experiments:         []string{"aliastypeparams", "dwarf5", "regabiargs", "regabiwrappers", "swissmap", "synchashtriemap"},
		DefaultGOEXPERIMENT: "",
		DefaultGOAMD64:      "v1",
		DefaultGOARM64:      "v8.0",
		Sources: []TagSource{
			{File: "src/internal/buildcfg/exp.go", Line: 81, What: "baseline := goexperiment.Flags{...}: RegabiWrappers/RegabiArgs = regabiSupported, AliasTypeParams/SwissMap/SyncHashTrieMap = true, Dwarf5 = dwarf5Supported"},
			{File: "src/internal/buildcfg/exp.go", Line: 64, What: "the goarch switch that sets regabiSupported and regabiAlwaysOn (amd64 and arm64: both true)"},
			{File: "src/internal/buildcfg/exp.go", Line: 79, What: "dwarf5Supported := (goos != \"darwin\" && goos != \"ios\" && goos != \"aix\"): true for linux and windows"},
			{File: "src/internal/buildcfg/zbootstrap.go", Line: 8, What: "DefaultGOAMD64 = `v1`"},
			{File: "src/internal/buildcfg/zbootstrap.go", Line: 10, What: "DefaultGOARM64 = `v8.0`"},
			{File: "src/internal/buildcfg/zbootstrap.go", Line: 15, What: "defaultGOEXPERIMENT = ``"},
		},
	},
}

// A TagSet is the complete, explicit build constraint set of one target.
type TagSet struct {
	GOOS        string   `json:"goos"`
	GOARCH      string   `json:"goarch"`
	Compiler    string   `json:"compiler"` // always "gocpp"
	CgoEnabled  bool     `json:"cgo_enabled"`
	ReleaseTags []string `json:"release_tags"`
	ToolTags    []string `json:"tool_tags"`
	BuildTags   []string `json:"build_tags"`
	All         []string `json:"all"` // sorted union, for the record
}

// readToolDefaults reads the tool defaults of the Go tree at goroot.
func readToolDefaults(goroot string) (*ToolDefaults, error) {
	version, err := readVersion(goroot)
	if err != nil {
		return nil, err
	}
	minor, err := goMinor(version)
	if err != nil {
		return nil, err
	}
	d := &ToolDefaults{Version: version, Minor: minor}
	d.Sources = append(d.Sources, TagSource{File: "VERSION", Line: 1, What: version})

	zb, err := parseGoFile(goroot, "src/internal/buildcfg/zbootstrap.go")
	if err != nil {
		return nil, err
	}
	zbconsts := constStrings(zb)
	want := func(name string) (string, error) {
		v, ok := zbconsts[name]
		if !ok {
			return "", fmt.Errorf("%s: no const %s in src/internal/buildcfg/zbootstrap.go", goroot, name)
		}
		return v.value, nil
	}
	if d.DefaultGOAMD64, err = want("DefaultGOAMD64"); err != nil {
		return nil, err
	}
	if d.DefaultGOARM64, err = want("DefaultGOARM64"); err != nil {
		return nil, err
	}
	if d.DefaultGOEXPERIMENT, err = want("defaultGOEXPERIMENT"); err != nil {
		return nil, err
	}
	if v, ok := zbconsts["version"]; ok && v.value != version {
		return nil, fmt.Errorf("%s: VERSION says %s, zbootstrap.go says %s", goroot, version, v.value)
	}
	d.Sources = append(d.Sources,
		TagSource{File: "src/internal/buildcfg/zbootstrap.go", Line: zbconsts["DefaultGOAMD64"].line, What: "DefaultGOAMD64 = " + quote(d.DefaultGOAMD64)},
		TagSource{File: "src/internal/buildcfg/zbootstrap.go", Line: zbconsts["DefaultGOARM64"].line, What: "DefaultGOARM64 = " + quote(d.DefaultGOARM64)},
		TagSource{File: "src/internal/buildcfg/zbootstrap.go", Line: zbconsts["defaultGOEXPERIMENT"].line, What: "defaultGOEXPERIMENT = " + quote(d.DefaultGOEXPERIMENT)},
	)

	exp, err := parseGoFile(goroot, "src/internal/buildcfg/exp.go")
	if err != nil {
		return nil, err
	}
	flags, err := parseGoFile(goroot, "src/internal/goexperiment/flags.go")
	if err != nil {
		return nil, err
	}
	fields, err := experimentFields(flags)
	if err != nil {
		return nil, fmt.Errorf("src/internal/goexperiment/flags.go: %v", err)
	}
	// Evaluate the release's defaults for linux/amd64, and check that linux and
	// windows on amd64 and arm64 get the same set: the table pins one set.
	var first *expResult
	for _, target := range [][2]string{{"linux", "amd64"}, {"linux", "arm64"}, {"windows", "amd64"}, {"windows", "arm64"}} {
		got, err := parseExp(exp, target[0], target[1])
		if err != nil {
			return nil, fmt.Errorf("src/internal/buildcfg/exp.go: %v", err)
		}
		enabled, err := applyGOEXPERIMENT(got.baseline, fields, d.DefaultGOEXPERIMENT)
		if err != nil {
			return nil, fmt.Errorf("src/internal/buildcfg/zbootstrap.go: %v", err)
		}
		got.enabled = enabled
		if first == nil {
			first = got
			continue
		}
		if strings.Join(got.enabled, ",") != strings.Join(first.enabled, ",") {
			return nil, fmt.Errorf("%s: the experiment set differs between targets: %s/%s has %v, linux/amd64 has %v",
				goroot, target[0], target[1], got.enabled, first.enabled)
		}
	}
	d.Experiments = first.enabled
	d.Sources = append(d.Sources,
		TagSource{File: "src/internal/buildcfg/exp.go", Line: first.baselineLine, What: "baseline := goexperiment.Flags{...}"},
		TagSource{File: "src/internal/buildcfg/exp.go", Line: first.switchLine, What: "the goarch switch for regabiSupported/regabiAlwaysOn"},
		TagSource{File: "src/internal/buildcfg/exp.go", Line: first.dwarf5Line, What: "dwarf5Supported"},
	)

	if known, ok := knownDefaults[minor]; ok {
		d.Table = minor
		if err := sameDefaults(&known, d); err != nil {
			return nil, fmt.Errorf("%s: the release defaults differ from the table entry %q: %v; update knownDefaults in tools/gostddryrun/tags.go",
				goroot, minor, err)
		}
		d.Verified = true
	}
	return d, nil
}

func sameDefaults(a, b *ToolDefaults) error {
	if strings.Join(a.Experiments, ",") != strings.Join(b.Experiments, ",") {
		return fmt.Errorf("experiments: table %v, tree %v", a.Experiments, b.Experiments)
	}
	if a.DefaultGOEXPERIMENT != b.DefaultGOEXPERIMENT {
		return fmt.Errorf("defaultGOEXPERIMENT: table %q, tree %q", a.DefaultGOEXPERIMENT, b.DefaultGOEXPERIMENT)
	}
	if a.DefaultGOAMD64 != b.DefaultGOAMD64 {
		return fmt.Errorf("DefaultGOAMD64: table %q, tree %q", a.DefaultGOAMD64, b.DefaultGOAMD64)
	}
	if a.DefaultGOARM64 != b.DefaultGOARM64 {
		return fmt.Errorf("DefaultGOARM64: table %q, tree %q", a.DefaultGOARM64, b.DefaultGOARM64)
	}
	return nil
}

// tagSet returns the complete constraint set for one target.
func (d *ToolDefaults) tagSet(goos, goarch string, extra []string) (*TagSet, error) {
	if goos != "linux" && goos != "windows" {
		return nil, fmt.Errorf("-goos %q: only linux and windows are supported", goos)
	}
	if goarch != "amd64" && goarch != "arm64" {
		return nil, fmt.Errorf("-goarch %q: only amd64 and arm64 are supported", goarch)
	}
	t := &TagSet{
		GOOS:       goos,
		GOARCH:     goarch,
		Compiler:   "gocpp",
		CgoEnabled: false,
	}
	if err := parseMinor(d.Minor); err != nil {
		return nil, err
	}
	n, _ := strconv.Atoi(strings.TrimPrefix(d.Minor, "go1."))
	for i := 1; i <= n; i++ {
		t.ReleaseTags = append(t.ReleaseTags, fmt.Sprintf("go1.%d", i))
	}
	for _, e := range d.Experiments {
		t.ToolTags = append(t.ToolTags, "goexperiment."+e)
	}
	level, err := archLevelTags(goarch, d.DefaultGOAMD64, d.DefaultGOARM64)
	if err != nil {
		return nil, err
	}
	t.ToolTags = append(t.ToolTags, level...)
	sort.Strings(t.ToolTags)

	t.BuildTags = []string{"purego", "math_big_pure_go"}
	if goos == "linux" {
		// unix is implicit in go/build for a Unix GOOS; listing it makes the
		// recorded set exact and independent of go/build's syslist.
		t.BuildTags = append(t.BuildTags, "unix")
	}
	for _, tag := range extra {
		tag = strings.TrimSpace(tag)
		if tag == "" {
			continue
		}
		switch tag {
		case "gc", "gccgo", "gocpp", "cgo":
			return nil, fmt.Errorf("-tags %q: the compiler tag is fixed to gocpp and cgo is off", tag)
		}
		t.BuildTags = append(t.BuildTags, tag)
	}
	sort.Strings(t.BuildTags)
	t.BuildTags = dedupe(t.BuildTags)

	all := append([]string{}, t.ReleaseTags...)
	all = append(all, t.ToolTags...)
	all = append(all, t.BuildTags...)
	all = append(all, goos, goarch, t.Compiler)
	sort.Strings(all)
	t.All = dedupe(all)
	return t, nil
}

// archLevelTags returns the architecture level tags that cmd/go adds, from the
// release's default GOAMD64/GOARM64 (internal/buildcfg/cfg.go, gogoarchTags).
func archLevelTags(goarch, goamd64, goarm64 string) ([]string, error) {
	switch goarch {
	case "amd64":
		n, err := strconv.Atoi(strings.TrimPrefix(goamd64, "v"))
		if err != nil || n < 1 || n > 4 {
			return nil, fmt.Errorf("DefaultGOAMD64 %q: not v1 to v4", goamd64)
		}
		var tags []string
		for i := 1; i <= n; i++ {
			tags = append(tags, fmt.Sprintf("amd64.v%d", i))
		}
		return tags, nil
	case "arm64":
		if len(goarm64) != 4 || goarm64[0] != 'v' || goarm64[2] != '.' {
			return nil, fmt.Errorf("DefaultGOARM64 %q: not vN.M", goarm64)
		}
		major := int(goarm64[1] - '0')
		minor := int(goarm64[3] - '0')
		var tags []string
		for i := 0; i <= minor; i++ {
			tags = append(tags, fmt.Sprintf("arm64.v%d.%d", major, i))
		}
		if major == 9 {
			for i := 0; i <= minor+5 && i <= 9; i++ {
				tags = append(tags, fmt.Sprintf("arm64.v8.%d", i))
			}
		}
		return tags, nil
	}
	return nil, fmt.Errorf("GOARCH %q: no architecture level", goarch)
}

// expResult is the evaluated baseline of exp.go for one target.
type expResult struct {
	baseline     map[string]bool // field name -> value, before defaultGOEXPERIMENT
	enabled      []string        // after defaultGOEXPERIMENT
	baselineLine int
	switchLine   int
	dwarf5Line   int
}

// parseExp evaluates the experiment baseline of ParseGOEXPERIMENT for one
// target, from the parsed exp.go.
func parseExp(file *goFile, goos, goarch string) (*expResult, error) {
	fn := findFunc(file.file, "ParseGOEXPERIMENT")
	if fn == nil || fn.Body == nil {
		return nil, fmt.Errorf("no func ParseGOEXPERIMENT")
	}
	res := &expResult{}
	env := map[string]bool{"regabiSupported": false, "regabiAlwaysOn": false, "dwarf5Supported": false}
	var baseline *ast.CompositeLit
	var evalErr error
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.SwitchStmt:
			if id, ok := n.Tag.(*ast.Ident); ok && id.Name == "goarch" {
				res.switchLine = file.line(n.Pos())
				for _, stmt := range n.Body.List {
					cc, ok := stmt.(*ast.CaseClause)
					if !ok || !caseMatches(cc, goarch) {
						continue
					}
					for _, s := range cc.Body {
						if as, ok := s.(*ast.AssignStmt); ok && len(as.Lhs) == 1 && len(as.Rhs) == 1 {
							if id, ok := as.Lhs[0].(*ast.Ident); ok {
								if b, ok := boolLit(as.Rhs[0]); ok {
									env[id.Name] = b
								}
							}
						}
					}
				}
			}
		case *ast.AssignStmt:
			if len(n.Lhs) == 1 && len(n.Rhs) == 1 {
				if id, ok := n.Lhs[0].(*ast.Ident); ok && id.Name == "dwarf5Supported" {
					res.dwarf5Line = file.line(n.Pos())
					v, err := evalBool(n.Rhs[0], map[string]string{"goos": goos, "goarch": goarch}, env)
					if err != nil {
						evalErr = fmt.Errorf("dwarf5Supported: %v", err)
						return false
					}
					env["dwarf5Supported"] = v
				}
			}
		case *ast.CompositeLit:
			if baseline == nil {
				if sel, ok := n.Type.(*ast.SelectorExpr); ok && sel.Sel.Name == "Flags" {
					baseline = n
					res.baselineLine = file.line(n.Pos())
				}
			}
		}
		return true
	})
	if evalErr != nil {
		return nil, evalErr
	}
	if baseline == nil {
		return nil, fmt.Errorf("no baseline goexperiment.Flags literal in ParseGOEXPERIMENT")
	}
	values := map[string]bool{}
	for _, e := range baseline.Elts {
		kv, ok := e.(*ast.KeyValueExpr)
		if !ok {
			return nil, fmt.Errorf("baseline: an element is not a field: value pair")
		}
		key, ok := kv.Key.(*ast.Ident)
		if !ok {
			return nil, fmt.Errorf("baseline: a key is not an identifier")
		}
		if b, ok := boolLit(kv.Value); ok {
			values[key.Name] = b
			continue
		}
		if v, ok := kv.Value.(*ast.Ident); ok {
			b, known := env[v.Name]
			if !known {
				return nil, fmt.Errorf("baseline: %s = %s, which this tool cannot evaluate", key.Name, v.Name)
			}
			values[key.Name] = b
			continue
		}
		return nil, fmt.Errorf("baseline: %s is not a bool literal", key.Name)
	}
	// The rules of ParseGOEXPERIMENT after the baseline.
	if env["regabiAlwaysOn"] {
		values["RegabiWrappers"] = true
		values["RegabiArgs"] = true
	}
	if !env["regabiSupported"] {
		values["RegabiWrappers"] = false
		values["RegabiArgs"] = false
	}
	res.baseline = values
	return res, nil
}

// applyGOEXPERIMENT applies a defaultGOEXPERIMENT string to a baseline, the way
// ParseGOEXPERIMENT does: "none" clears everything, "noX" clears X, "x" sets
// X, and "regabi" is the alias for the two regabi sub-experiments.
func applyGOEXPERIMENT(baseline map[string]bool, fields []string, goexp string) ([]string, error) {
	values := make(map[string]bool, len(baseline))
	for k, v := range baseline {
		values[k] = v
	}
	byLower := make(map[string]string, len(fields))
	for _, f := range fields {
		byLower[strings.ToLower(f)] = f
	}
	for _, name := range strings.Split(goexp, ",") {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		if name == "none" {
			for f := range values {
				values[f] = false
			}
			continue
		}
		val := true
		if strings.HasPrefix(name, "no") {
			name, val = name[2:], false
		}
		if name == "regabi" {
			values["RegabiWrappers"] = val
			values["RegabiArgs"] = val
			continue
		}
		f, ok := byLower[name]
		if !ok {
			return nil, fmt.Errorf("defaultGOEXPERIMENT %q: unknown experiment %q", goexp, name)
		}
		values[f] = val
	}
	var enabled []string
	for _, f := range fields {
		if values[f] {
			enabled = append(enabled, strings.ToLower(f))
		}
	}
	sort.Strings(enabled)
	return enabled, nil
}

// evalBool evaluates the small boolean expressions of exp.go: comparisons of
// goos/goarch with string literals, &&, ||, !, parentheses and true/false.
func evalBool(e ast.Expr, vars map[string]string, env map[string]bool) (bool, error) {
	switch e := e.(type) {
	case *ast.ParenExpr:
		return evalBool(e.X, vars, env)
	case *ast.UnaryExpr:
		if e.Op == token.NOT {
			b, err := evalBool(e.X, vars, env)
			return !b, err
		}
	case *ast.Ident:
		switch e.Name {
		case "true":
			return true, nil
		case "false":
			return false, nil
		}
		if b, ok := env[e.Name]; ok {
			return b, nil
		}
		return false, fmt.Errorf("unknown identifier %s", e.Name)
	case *ast.BinaryExpr:
		switch e.Op {
		case token.LAND:
			l, err := evalBool(e.X, vars, env)
			if err != nil || !l {
				return false, err
			}
			return evalBool(e.Y, vars, env)
		case token.LOR:
			l, err := evalBool(e.X, vars, env)
			if err != nil || l {
				return l, err
			}
			return evalBool(e.Y, vars, env)
		case token.EQL, token.NEQ:
			l, err := stringValue(e.X, vars)
			if err != nil {
				return false, err
			}
			r, err := stringValue(e.Y, vars)
			if err != nil {
				return false, err
			}
			if e.Op == token.EQL {
				return l == r, nil
			}
			return l != r, nil
		}
	}
	return false, fmt.Errorf("unsupported expression")
}

func stringValue(e ast.Expr, vars map[string]string) (string, error) {
	switch e := e.(type) {
	case *ast.Ident:
		if v, ok := vars[e.Name]; ok {
			return v, nil
		}
		return "", fmt.Errorf("unknown identifier %s", e.Name)
	case *ast.BasicLit:
		if e.Kind == token.STRING {
			return strconv.Unquote(e.Value)
		}
	}
	return "", fmt.Errorf("unsupported string expression")
}

func boolLit(e ast.Expr) (bool, bool) {
	id, ok := e.(*ast.Ident)
	if !ok {
		return false, false
	}
	switch id.Name {
	case "true":
		return true, true
	case "false":
		return false, true
	}
	return false, false
}

func caseMatches(cc *ast.CaseClause, goarch string) bool {
	for _, e := range cc.List {
		if lit, ok := e.(*ast.BasicLit); ok && lit.Kind == token.STRING {
			if v, err := strconv.Unquote(lit.Value); err == nil && v == goarch {
				return true
			}
		}
	}
	return false
}

func findFunc(file *ast.File, name string) *ast.FuncDecl {
	for _, d := range file.Decls {
		if fd, ok := d.(*ast.FuncDecl); ok && fd.Name.Name == name {
			return fd
		}
	}
	return nil
}

// A goFile is a parsed file and the position table that names its lines.
type goFile struct {
	file *ast.File
	fset *token.FileSet
}

func (f *goFile) line(pos token.Pos) int {
	return f.fset.Position(pos).Line
}

// experimentFields returns the field names of goexperiment.Flags.
func experimentFields(f *goFile) ([]string, error) {
	for _, d := range f.file.Decls {
		gd, ok := d.(*ast.GenDecl)
		if !ok || gd.Tok != token.TYPE {
			continue
		}
		for _, s := range gd.Specs {
			ts, ok := s.(*ast.TypeSpec)
			if !ok || ts.Name.Name != "Flags" {
				continue
			}
			st, ok := ts.Type.(*ast.StructType)
			if !ok {
				return nil, fmt.Errorf("goexperiment.Flags is not a struct")
			}
			var names []string
			for _, field := range st.Fields.List {
				if _, ok := field.Type.(*ast.Ident); !ok {
					continue
				}
				for _, n := range field.Names {
					if n.Name != "" && n.Name != "_" {
						names = append(names, n.Name)
					}
				}
			}
			return names, nil
		}
	}
	return nil, fmt.Errorf("no type Flags struct")
}

// parseGoFile parses one file of the tree.
func parseGoFile(goroot, rel string) (*goFile, error) {
	path := filepath.Join(goroot, filepath.FromSlash(rel))
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
	if err != nil {
		return nil, err
	}
	return &goFile{file: f, fset: fset}, nil
}

type constValue struct {
	value string
	line  int
}

// constStrings returns the string constants of a file, by name.
func constStrings(f *goFile) map[string]constValue {
	out := map[string]constValue{}
	for _, d := range f.file.Decls {
		gd, ok := d.(*ast.GenDecl)
		if !ok || gd.Tok != token.CONST {
			continue
		}
		for _, s := range gd.Specs {
			vs, ok := s.(*ast.ValueSpec)
			if !ok {
				continue
			}
			for i, n := range vs.Names {
				if i >= len(vs.Values) {
					continue
				}
				lit, ok := vs.Values[i].(*ast.BasicLit)
				if !ok || lit.Kind != token.STRING {
					continue
				}
				v, err := strconv.Unquote(lit.Value)
				if err != nil {
					continue
				}
				out[n.Name] = constValue{value: v, line: f.line(n.Pos())}
			}
		}
	}
	return out
}

func readVersion(goroot string) (string, error) {
	data, err := os.ReadFile(filepath.Join(goroot, "VERSION"))
	if err != nil {
		return "", fmt.Errorf("not a Go source tree: %v", err)
	}
	fields := strings.Fields(string(data))
	if len(fields) == 0 || !strings.HasPrefix(fields[0], "go1.") {
		return "", fmt.Errorf("%s/VERSION: no go1.x version", goroot)
	}
	return fields[0], nil
}

func goMinor(version string) (string, error) {
	if !strings.HasPrefix(version, "go1.") {
		return "", fmt.Errorf("version %q is not go1.x", version)
	}
	rest := strings.TrimPrefix(version, "go1.")
	i := 0
	for i < len(rest) && rest[i] >= '0' && rest[i] <= '9' {
		i++
	}
	if i == 0 {
		return "", fmt.Errorf("version %q: no minor number", version)
	}
	minor, err := strconv.Atoi(rest[:i])
	if err != nil {
		return "", fmt.Errorf("version %q: %v", version, err)
	}
	return fmt.Sprintf("go1.%d", minor), nil
}

func parseMinor(minor string) error {
	rest := strings.TrimPrefix(minor, "go1.")
	if _, err := strconv.Atoi(rest); err != nil {
		return fmt.Errorf("version minor %q: %v", minor, err)
	}
	return nil
}

func dedupe(sorted []string) []string {
	out := sorted[:0]
	for _, s := range sorted {
		if len(out) == 0 || out[len(out)-1] != s {
			out = append(out, s)
		}
	}
	return out
}

func quote(s string) string {
	return "`" + s + "`"
}
