// Spike: can stock go/parser + go/types (FakeImportC) act as the gocpp front-end?
//
// Import model (decided 2026-10-13):
//   - Every header import is a package named after its file:
//     "<raylib.h>" gives raylib.InitWindow, "./physics.h" gives physics.step,
//     "<cstdio>" gives cstdio.printf. Its members are the header's C-level names.
//   - Every C++ namespace the headers declare is a package too: std.vector[int],
//     std.chrono.seconds. No import line is needed for a namespace.
//   - A file name that is a Go keyword or predeclared name ("<map>",
//     "<string.h>"), or is already taken by another import, gives no name.
//   - C keeps only C's built-in types and helpers: C.int, C.cstr(s).
//
// Before go/types runs, raylib.X becomes C.X, and ns.X becomes C.ns.X when ns
// is not declared anywhere in Go. A compiler probe then checks that every such
// ns really is a C++ namespace; if not, it is a normal "undefined: ns" error.
package main

import (
	"fmt"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

type header struct {
	path string
	name string // package name, "" if the header has none
	why  string // why it has no name
}

func isHeader(path string) bool {
	if strings.HasPrefix(path, "<") && strings.HasSuffix(path, ">") {
		return true
	}
	for _, ext := range []string{".h", ".hh", ".hpp", ".hxx", ".h++", ".inl"} {
		if strings.HasSuffix(path, ext) {
			return true
		}
	}
	return false
}

// defaultName gives the package name of a header: its file name without
// directory and extension, with characters Go doesn't allow replaced by '_'.
func defaultName(path string) (string, string) {
	base := filepath.Base(strings.Trim(path, "<>"))
	base = strings.TrimSuffix(base, filepath.Ext(base))
	name := regexp.MustCompile(`[^A-Za-z0-9_]`).ReplaceAllString(base, "_")
	switch {
	case name == "" || name[0] >= '0' && name[0] <= '9':
		return "", "not a Go name"
	case token.IsKeyword(name):
		return "", "`" + name + "` is a Go keyword"
	case types.Universe.Lookup(name) != nil:
		return "", "`" + name + "` is a predeclared Go name"
	}
	return name, ""
}

func main() {
	file := os.Args[1]
	verbose := len(os.Args) > 2
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, file, nil, parser.ParseComments|parser.AllErrors)
	if err != nil {
		fmt.Println("PARSE ERROR:", err)
		return
	}
	fmt.Println("parse: OK")

	// 1. Sort imports into gocpp packages and headers.
	var headers []header
	goNames := map[string]bool{}
	var firstImport *ast.GenDecl
	for _, d := range f.Decls {
		gd, ok := d.(*ast.GenDecl)
		if !ok || gd.Tok != token.IMPORT {
			continue
		}
		if firstImport == nil {
			firstImport = gd
		}
		var kept []ast.Spec
		for _, s := range gd.Specs {
			imp := s.(*ast.ImportSpec)
			path, _ := strconv.Unquote(imp.Path.Value)
			if !isHeader(path) {
				name := path[strings.LastIndex(path, "/")+1:]
				if imp.Name != nil {
					name = imp.Name.Name
				}
				goNames[name] = true
				kept = append(kept, s)
				continue
			}
			h := header{path: path}
			switch {
			case imp.Name != nil && imp.Name.Name == "_":
				h.why = "imported with _"
			case imp.Name != nil:
				h.name = imp.Name.Name
			default:
				h.name, h.why = defaultName(path)
			}
			headers = append(headers, h)
		}
		gd.Specs = kept // headers become #include lines; go/types never sees them
	}
	headerNames := map[string]bool{}
	for i, h := range headers {
		if h.name != "" && goNames[h.name] {
			headers[i].name, headers[i].why = "", "`"+h.name+"` is taken by a Go import"
		}
		if headers[i].name != "" {
			headerNames[headers[i].name] = true
		}
		if headers[i].name != "" {
			fmt.Printf("  header %-22s package %s\n", h.path, headers[i].name)
		} else {
			fmt.Printf("  header %-22s no package name (%s)\n", h.path, headers[i].why)
		}
	}

	// 2. Rewrite selectors. go/parser resolves locals and this file's own
	// declarations (Obj != nil), so a local named std is left alone.
	type use struct {
		name string
		pos  token.Pos
	}
	var nsUses []use
	usesC := false
	ast.Inspect(f, func(n ast.Node) bool {
		if id, ok := n.(*ast.Ident); ok && id.Name == "C" {
			usesC = true
		}
		sel, ok := n.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		id, ok := sel.X.(*ast.Ident)
		if !ok || id.Obj != nil || id.Name == "C" || goNames[id.Name] {
			return true
		}
		switch {
		case headerNames[id.Name]: // raylib.X -> C.X
			sel.X = &ast.Ident{NamePos: id.NamePos, Name: "C"}
		case types.Universe.Lookup(id.Name) != nil: // string.x stays a Go error
			return true
		default: // ns.X -> C.ns.X, checked by the namespace probe below
			sel.X = &ast.SelectorExpr{X: &ast.Ident{NamePos: id.NamePos, Name: "C"}, Sel: id}
			nsUses = append(nsUses, use{id.Name, id.Pos()})
		}
		usesC = true
		return true
	})
	if usesC && firstImport != nil {
		firstImport.Specs = append(firstImport.Specs, &ast.ImportSpec{
			Path: &ast.BasicLit{Kind: token.STRING, Value: strconv.Quote("C")},
		})
	}

	// 3. Namespace probe: one -fsyntax-only file, one line per candidate.
	nerr := 0
	seen := map[string]bool{}
	var cands []string
	for _, u := range nsUses {
		if !seen[u.name] {
			seen[u.name] = true
			cands = append(cands, u.name)
		}
	}
	sort.Strings(cands)
	if len(cands) > 0 {
		for _, cxx := range []string{"clang++", "g++"} {
			isNS, err := probe(cxx, filepath.Dir(file), headers, cands)
			if err != nil {
				fmt.Printf("  namespace probe (%s): %v\n", cxx, err)
				continue
			}
			var yes, no []string
			for _, c := range cands {
				if isNS[c] {
					yes = append(yes, c)
				} else {
					no = append(no, c)
				}
			}
			fmt.Printf("  namespace probe (%s): namespaces %v, not namespaces %v\n", cxx, yes, no)
			if cxx == "clang++" {
				for _, u := range nsUses {
					if !isNS[u.name] {
						nerr++
						fmt.Printf("  GOCPP ERROR: %s: undefined: %s\n", fset.Position(u.pos), u.name)
					}
				}
			}
		}
	}

	// 4. Type-check with stock go/types.
	conf := types.Config{
		Importer:    importer.ForCompiler(fset, "source", nil),
		FakeImportC: true,
		Error: func(err error) {
			nerr++
			msg := err.Error()
			for _, h := range headers {
				if h.name == "" && h.why != "" && strings.Contains(msg, " "+strings.Trim(strings.Fields(h.why)[0], "`")+".") {
					msg += fmt.Sprintf("  [hint: to use the C names of %s, give it a name: import c%s %q]",
						h.path, strings.Trim(strings.Fields(h.why)[0], "`"), h.path)
				}
			}
			fmt.Println("  TYPE ERROR:", msg)
		},
	}
	info := &types.Info{
		Types: map[ast.Expr]types.TypeAndValue{},
		Defs:  map[*ast.Ident]types.Object{},
		Uses:  map[*ast.Ident]types.Object{},
	}
	conf.Check("main", fset, []*ast.File{f}, info)
	fmt.Printf("errors: %d\n", nerr)

	if verbose {
		fmt.Println("local variable types:")
		var lines []string
		for id, obj := range info.Defs {
			if v, ok := obj.(*types.Var); ok && !v.IsField() && v.Parent() != nil && v.Parent() != v.Pkg().Scope() {
				lines = append(lines, fmt.Sprintf("  %-6s %-6s %s", fset.Position(id.Pos()).String()[len(file)+1:], id.Name, v.Type()))
			}
		}
		sort.Strings(lines)
		fmt.Println(strings.Join(lines, "\n"))
	}
}

// probe asks the C++ compiler which candidate names are namespaces. Headers
// that aren't installed are skipped with __has_include, so the spike runs anywhere.
func probe(cxx, dir string, headers []header, cands []string) (map[string]bool, error) {
	if _, err := exec.LookPath(cxx); err != nil {
		return nil, err
	}
	var b strings.Builder
	for _, h := range headers {
		inc := h.path
		if !strings.HasPrefix(inc, "<") {
			inc = strconv.Quote(inc)
		}
		fmt.Fprintf(&b, "#if __has_include(%s)\n#include %s\n#endif\n", inc, inc)
	}
	first := strings.Count(b.String(), "\n") + 1
	for i, c := range cands {
		fmt.Fprintf(&b, "namespace gp_ns_%d = ::%s;\n", i, c)
	}
	tmp, err := os.MkdirTemp("", "gp-nsprobe")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmp)
	src := filepath.Join(tmp, "probe.cpp")
	if err := os.WriteFile(src, []byte(b.String()), 0o644); err != nil {
		return nil, err
	}
	limit := "-ferror-limit=0"
	if cxx == "g++" {
		limit = "-fmax-errors=0"
	}
	out, _ := exec.Command(cxx, "-std=c++20", "-fsyntax-only", limit, "-I", dir, src).CombinedOutput()
	bad := map[int]bool{}
	for _, m := range regexp.MustCompile(`probe\.cpp:(\d+):\d+: error`).FindAllStringSubmatch(string(out), -1) {
		n, _ := strconv.Atoi(m[1])
		bad[n] = true
	}
	isNS := map[string]bool{}
	for i, c := range cands {
		isNS[c] = !bad[first+i]
	}
	return isNS, nil
}
