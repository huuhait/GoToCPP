// Spike: must functions (`func f!() T`) and must calls (`f!()`) on STOCK go/types.
//
// gocpp syntax (decided 2026-10-13):
//
//	func loadConfig!(path string) Config { ... }  every error inside it that the
//	                                              code doesn't take is returned
//	cfg := loadConfig!(path)                      ! on one call, in any function
//	cfg, err := loadConfig(path)                  no !: plain Go
//
// Steps: (1) the scanner turns `name!(` into one name (here: the letter ǃ);
// (2) a must function gets a real error result, and `return x` becomes
// `return x, nil`; (3) go/types checks once; (4) calls whose error is dropped are
// found with the types from (3) and lowered to `if err != nil { return ... }`;
// (5) go/types checks again and must report 0 errors. The result is plain Go,
// which this spike then runs with `go run`.
package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
)

const userSrc = `package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	Port int ` + "`json:\"port\"`" + `
}

// A must function: every error inside it that the code doesn't take is
// returned for you. Callers see func loadConfig(path string) (Config, error).
func loadConfig!(path string) Config {
	var cfg Config
	data := os.ReadFile(path)
	json.Unmarshal(data, &cfg)
	if cfg.Port == 0 {
		return Config{}, errors.New("port is missing") // one extra value: fail on purpose
	}
	return cfg
}

// Nested: Atoi's error is returned, its value is used.
func double!(s string) int {
	return strconv.Atoi(s) * 2
}

// A normal function: ! on one call.
func port(path string) (int, error) {
	cfg := loadConfig!(path)
	return cfg.Port, nil
}

// No ! at all: plain Go, the error is handled by hand.
func portByHand(path string) (int, error) {
	cfg, err := loadConfig(path)
	if err != nil {
		return 0, err
	}
	return cfg.Port, nil
}

func main!() {
	q, err := portByHand(os.Args[1]) // taken by hand: left alone
	fmt.Println("by hand:", q, err)
	p := port(os.Args[1]) // error: printed, exit 1
	fmt.Println("port:", p, "double:", double("21"))
}
`

const bang = "ǃ" // stands for the token the patched scanner produces

var bangRe = regexp.MustCompile(`\b([A-Za-z_][A-Za-z0-9_]*)!\(`)

func scan(src string) string {
	return bangRe.ReplaceAllStringFunc(src, func(m string) string {
		if name := m[:len(m)-2]; !token.IsKeyword(name) {
			return name + bang + "("
		}
		return m
	})
}

var errorType = types.Universe.Lookup("error").Type()

func main() {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "main.gp", scan(userSrc), parser.ParseComments)
	if err != nil {
		panic(err)
	}

	// (2) must functions and must calls.
	mustFn := map[*ast.FuncDecl]bool{}
	mustCall := map[*ast.CallExpr]bool{}
	for _, d := range f.Decls {
		fd, ok := d.(*ast.FuncDecl)
		if !ok || !strings.HasSuffix(fd.Name.Name, bang) {
			continue
		}
		fd.Name.Name = strings.TrimSuffix(fd.Name.Name, bang)
		mustFn[fd] = true
		if fd.Recv == nil && (fd.Name.Name == "main" || fd.Name.Name == "init") {
			continue // main! keeps no results: an error is printed and exits 1
		}
		if fd.Type.Results == nil {
			fd.Type.Results = &ast.FieldList{}
		}
		declared := fd.Type.Results.NumFields()
		named := declared > 0 && len(fd.Type.Results.List[0].Names) > 0
		field := &ast.Field{Type: ast.NewIdent("error")}
		if named {
			field.Names = []*ast.Ident{ast.NewIdent("gp_err")}
		}
		fd.Type.Results.List = append(fd.Type.Results.List, field)
		inspectOwn(fd.Body, func(n ast.Node) {
			if r, ok := n.(*ast.ReturnStmt); ok && len(r.Results) == declared && !(named && declared > 0 && len(r.Results) == 0) {
				r.Results = append(r.Results, ast.NewIdent("nil")) // return x  →  return x, nil
			}
		})
	}
	ast.Inspect(f, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok {
			switch fun := call.Fun.(type) {
			case *ast.Ident:
				if strings.HasSuffix(fun.Name, bang) {
					fun.Name = strings.TrimSuffix(fun.Name, bang)
					mustCall[call] = true
				}
			case *ast.SelectorExpr:
				if strings.HasSuffix(fun.Sel.Name, bang) {
					fun.Sel.Name = strings.TrimSuffix(fun.Sel.Name, bang)
					mustCall[call] = true
				}
			}
		}
		return true
	})

	// (3) first check. Errors are expected here: data := os.ReadFile(path) is
	// "assignment mismatch" until step (4) lowers it.
	imp := importer.ForCompiler(fset, "source", nil)
	info := &types.Info{Types: map[ast.Expr]types.TypeAndValue{}, Defs: map[*ast.Ident]types.Object{}, Uses: map[*ast.Ident]types.Object{}}
	pass1 := 0
	conf := types.Config{Importer: imp, Error: func(error) { pass1++ }}
	pkg, _ := conf.Check("main", fset, []*ast.File{f}, info)
	fmt.Printf("check 1: %d expected errors (dropped errors not lowered yet)\n", pass1)

	// (4) lower every call whose error is dropped.
	qual := func(p *types.Package) string {
		if p == pkg {
			return ""
		}
		return p.Name()
	}
	needExit := false
	for _, d := range f.Decls {
		fd, ok := d.(*ast.FuncDecl)
		if !ok || fd.Body == nil {
			continue
		}
		sig := info.Defs[fd.Name].Type().(*types.Signature)
		l := &lowerer{info: info, qual: qual, must: mustFn[fd], calls: mustCall, sig: sig,
			exit: fd.Recv == nil && fd.Name.Name == "main"}
		fd.Body.List = l.block(fd.Body.List)
		needExit = needExit || l.usedExit
	}
	if needExit {
		addImport(f, "path/filepath")
		helper, _ := parser.ParseFile(fset, "", "package p\nfunc gp_exit(err error) {\n"+
			"\tfmt.Fprintf(os.Stderr, \"%s: %v\\n\", filepath.Base(os.Args[0]), err)\n\tos.Exit(1)\n}\n", 0)
		f.Decls = append(f.Decls, helper.Decls[0])
	}

	// (5) second check on the printed result: must be clean. Comments are
	// dropped because go/printer can't place them next to new nodes.
	f.Comments = nil
	var out bytes.Buffer
	if err := format.Node(&out, fset, f); err != nil {
		panic(err)
	}
	fset2 := token.NewFileSet()
	f2, err := parser.ParseFile(fset2, "main.go", out.Bytes(), parser.ParseComments)
	if err != nil {
		fmt.Println(out.String())
		panic(err)
	}
	pass2 := 0
	conf2 := types.Config{Importer: importer.ForCompiler(fset2, "source", nil), Error: func(e error) { pass2++; fmt.Println("  check 2 error:", e) }}
	conf2.Check("main", fset2, []*ast.File{f2}, nil)
	fmt.Printf("check 2: %d errors\n", pass2)
	if len(os.Args) > 1 {
		fmt.Println("---- generated Go ----")
		fmt.Print(out.String())
		fmt.Println("----------------------")
	}

	// Run it for real: with a good config, a config without a port, and a missing file.
	dir, _ := os.MkdirTemp("", "mustfn")
	defer os.RemoveAll(dir)
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module mustfn\n\ngo 1.25\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "main.go"), out.Bytes(), 0o644)
	os.WriteFile(filepath.Join(dir, "good.json"), []byte(`{"port": 8080}`), 0o644)
	os.WriteFile(filepath.Join(dir, "noport.json"), []byte(`{}`), 0o644)
	build := exec.Command("go", "build", "-o", "mustfn", ".")
	build.Dir = dir
	if b, err := build.CombinedOutput(); err != nil {
		fmt.Println(string(b))
		panic(err)
	}
	for _, arg := range []string{"good.json", "noport.json", "missing.json"} {
		cmd := exec.Command("./mustfn", arg)
		cmd.Dir = dir
		b, _ := cmd.CombinedOutput()
		fmt.Printf("run %-13s exit %d\n", arg, cmd.ProcessState.ExitCode())
		for _, line := range strings.Split(strings.TrimSpace(string(b)), "\n") {
			fmt.Println("    " + line)
		}
	}
}

type lowerer struct {
	info     *types.Info
	qual     types.Qualifier
	must     bool // inside a must function: lower every dropped error
	exit     bool // main: print the error and exit 1
	calls    map[*ast.CallExpr]bool
	sig      *types.Signature
	n        int
	usedExit bool
}

func (l *lowerer) block(list []ast.Stmt) []ast.Stmt {
	var out []ast.Stmt
	for _, s := range list {
		switch s := s.(type) {
		case *ast.BlockStmt:
			s.List = l.block(s.List)
		case *ast.IfStmt:
			s.Body.List = l.block(s.Body.List)
			if e, ok := s.Else.(*ast.BlockStmt); ok {
				e.List = l.block(e.List)
			}
		case *ast.ForStmt:
			s.Body.List = l.block(s.Body.List)
		case *ast.RangeStmt:
			s.Body.List = l.block(s.Body.List)
		default:
			out = append(out, l.stmt(s)...)
			continue
		}
		out = append(out, s)
	}
	return out
}

type site struct {
	call   *ast.CallExpr
	parent ast.Node
}

// stmt lowers one simple statement: every handled call is moved before it.
func (l *lowerer) stmt(s ast.Stmt) []ast.Stmt {
	var sites []site
	var stack []ast.Node
	ast.Inspect(s, func(n ast.Node) bool {
		if n == nil {
			top := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			if call, ok := top.(*ast.CallExpr); ok && len(stack) > 0 {
				sites = append(sites, site{call, stack[len(stack)-1]}) // post-order: inner calls first
			}
			return true
		}
		if _, ok := n.(*ast.FuncLit); ok {
			return false // a func literal is another function
		}
		stack = append(stack, n)
		return true
	})
	var pre []ast.Stmt
	for _, st := range sites {
		res := l.results(st.call)
		if res == nil || res.Len() == 0 || !types.Identical(res.At(res.Len()-1).Type(), errorType) {
			continue
		}
		if !l.calls[st.call] && !(l.must && l.dropped(s, st, res)) {
			continue
		}
		k := l.n
		l.n++
		var lhs []ast.Expr
		var vals []ast.Expr
		for i := 0; i < res.Len()-1; i++ {
			name := fmt.Sprintf("gp_t%d_%d", k, i)
			if _, isStmt := st.parent.(*ast.ExprStmt); isStmt {
				name = "_"
			}
			lhs = append(lhs, ast.NewIdent(name))
			vals = append(vals, ast.NewIdent(name))
		}
		errName := fmt.Sprintf("gp_e%d", k)
		lhs = append(lhs, ast.NewIdent(errName))
		pre = append(pre, &ast.AssignStmt{Lhs: lhs, Tok: token.DEFINE, Rhs: []ast.Expr{st.call}}, l.check(errName))
		switch p := st.parent.(type) {
		case *ast.ExprStmt:
			s = nil // the whole statement was the call
		case *ast.AssignStmt:
			if len(p.Rhs) == 1 && p.Rhs[0] == st.call && len(vals) > 1 {
				p.Rhs = vals // a, b := f()  with f() (A, B, error)
				break
			}
			replace(p, st.call, vals[0])
		default:
			replace(p, st.call, vals[0])
		}
	}
	if s != nil {
		pre = append(pre, s)
	}
	return pre
}

// dropped reports whether a must function drops this call's error.
func (l *lowerer) dropped(s ast.Stmt, st site, res *types.Tuple) bool {
	switch p := st.parent.(type) {
	case *ast.ExprStmt:
		return true
	case *ast.AssignStmt:
		if len(p.Rhs) == 1 && p.Rhs[0] == st.call {
			return len(p.Lhs) == res.Len()-1
		}
	case *ast.CallExpr:
		if len(p.Args) == 1 && p.Args[0] == st.call {
			if sig, ok := l.info.Types[p.Fun].Type.(*types.Signature); ok && (sig.Variadic() || sig.Params().Len() == res.Len()) {
				return false // f(g()): all results are passed on, the error too
			}
		}
	}
	return res.Len() == 2 // (T, error) used as one value
}

func (l *lowerer) results(call *ast.CallExpr) *types.Tuple {
	tv, ok := l.info.Types[call.Fun]
	if !ok || tv.IsType() {
		return nil
	}
	if sig, ok := tv.Type.Underlying().(*types.Signature); ok {
		return sig.Results()
	}
	return nil
}

// check builds `if err != nil { return zero values, err }`, or the exit in main.
func (l *lowerer) check(errName string) ast.Stmt {
	var body ast.Stmt
	res := l.sig.Results()
	switch {
	case l.exit:
		l.usedExit = true
		body = &ast.ExprStmt{X: &ast.CallExpr{Fun: ast.NewIdent("gp_exit"), Args: []ast.Expr{ast.NewIdent(errName)}}}
	case res.Len() > 0 && types.Identical(res.At(res.Len()-1).Type(), errorType):
		var vals []ast.Expr
		for i := 0; i < res.Len()-1; i++ {
			zero, _ := parser.ParseExpr("*new(" + types.TypeString(res.At(i).Type(), l.qual) + ")")
			vals = append(vals, zero)
		}
		body = &ast.ReturnStmt{Results: append(vals, ast.NewIdent(errName))}
	default:
		body = &ast.ExprStmt{X: &ast.CallExpr{Fun: ast.NewIdent("panic"), Args: []ast.Expr{ast.NewIdent(errName)}}}
	}
	return &ast.IfStmt{
		Cond: &ast.BinaryExpr{X: ast.NewIdent(errName), Op: token.NEQ, Y: ast.NewIdent("nil")},
		Body: &ast.BlockStmt{List: []ast.Stmt{body}},
	}
}

// inspectOwn visits a function body without entering func literals.
func inspectOwn(n ast.Node, f func(ast.Node)) {
	ast.Inspect(n, func(n ast.Node) bool {
		if _, ok := n.(*ast.FuncLit); ok {
			return false
		}
		if n != nil {
			f(n)
		}
		return true
	})
}

// replace swaps old for new in one field of parent (a field or a slice element).
func replace(parent ast.Node, old, new ast.Expr) {
	v := reflect.ValueOf(parent).Elem()
	for i := 0; i < v.NumField(); i++ {
		fv := v.Field(i)
		switch {
		case fv.Kind() == reflect.Interface && !fv.IsNil() && fv.Interface() == old:
			fv.Set(reflect.ValueOf(new))
			return
		case fv.Kind() == reflect.Slice:
			for j := 0; j < fv.Len(); j++ {
				if e := fv.Index(j); e.Kind() == reflect.Interface && !e.IsNil() && e.Interface() == old {
					e.Set(reflect.ValueOf(new))
					return
				}
			}
		}
	}
	panic("replace: not found")
}

func addImport(f *ast.File, path string) {
	for _, imp := range f.Imports {
		if imp.Path.Value == strconv.Quote(path) {
			return
		}
	}
	for _, d := range f.Decls {
		if gd, ok := d.(*ast.GenDecl); ok && gd.Tok == token.IMPORT {
			gd.Specs = append(gd.Specs, &ast.ImportSpec{Path: &ast.BasicLit{Kind: token.STRING, Value: strconv.Quote(path)}})
			return
		}
	}
}
