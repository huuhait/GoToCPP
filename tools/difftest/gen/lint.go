package gen

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"strconv"
	"strings"
)

// Lint checks type-checked generated code for the constructs whose behavior the language
// leaves open or that print what differs between runs, and returns what it found (empty for
// a program that is fine):
//
//   - an import other than the standard packages that the generator knows (fmt);
//   - a range over a map (the order is random in Go);
//   - a %p verb, or a func or chan value or a pointer to a basic type, struct field or slice
//     element, passed to a print function (an address);
//
// (The capacity of a slice after an append that grew it is the one thing that no static check
// here covers: the slice model scenario, which tracks lengths and capacities, is the only
// place that prints the capacity of a slice that was appended to.)
func Lint(fset *token.FileSet, files []*ast.File, info *types.Info) []string {
	var out []string
	report := func(pos token.Pos, format string, args ...any) {
		out = append(out, fset.Position(pos).String()+": "+fmt.Sprintf(format, args...))
	}
	allowedImports := map[string]bool{"fmt": true, "bytes": true, "errors": true, "math": true, "math/bits": true,
		"strconv": true, "strings": true, "sync": true, "time": true, "unicode": true, "unicode/utf8": true}
	// functions of the allowed packages that read the clock, the zone database or the scheduler
	forbiddenSel := map[string]bool{"time.Now": true, "time.Since": true, "time.Until": true, "time.Sleep": true, "time.After": true,
		"time.Tick": true, "time.NewTimer": true, "time.NewTicker": true, "time.AfterFunc": true, "time.Local": true,
		"time.LoadLocation": true, "runtime.Gosched": true}
	for _, f := range files {
		okLines := map[int]bool{}
		for _, cg := range f.Comments {
			for _, c := range cg.List {
				if strings.HasPrefix(c.Text, orderIndependent) {
					okLines[fset.Position(c.Pos()).Line] = true
				}
			}
		}
		for _, im := range f.Imports {
			p, _ := strconv.Unquote(im.Path.Value)
			if !allowedImports[p] {
				report(im.Pos(), "import %q is not allowed in generated programs", p)
			}
		}
		ast.Inspect(f, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.RangeStmt:
				if tv, ok := info.Types[n.X]; ok {
					if _, isMap := tv.Type.Underlying().(*types.Map); isMap && !okLines[fset.Position(n.Pos()).Line] {
						report(n.Pos(), "range over a map (the order is random)")
					}
				}
			case *ast.SelectorExpr:
				if id, ok := n.X.(*ast.Ident); ok && forbiddenSel[id.Name+"."+n.Sel.Name] {
					report(n.Pos(), "%s.%s depends on the clock, the zone database or the scheduler", id.Name, n.Sel.Name)
				}
			case *ast.BasicLit:
				if n.Kind == token.STRING && strings.Contains(n.Value, "%p") {
					report(n.Pos(), "%%p prints an address")
				}
			case *ast.CallExpr:
				if isPrintCall(n) {
					skip := typeVerbArgs(n)
					for i, a := range n.Args {
						if skip[i] {
							continue
						}
						if tv, ok := info.Types[a]; ok && tv.Type != nil {
							if why := addressInside(tv.Type, true, map[types.Type]bool{}); why != "" {
								report(a.Pos(), "printing a value with %s inside", why)
							}
						}
					}
				}
			}
			return true
		})
	}
	return out
}

// typeVerbArgs returns the arguments of a Printf call that a %T verb prints (a type name: no
// address) when the format is a plain string literal; indexes in it ([n], *) turn that off.
func typeVerbArgs(c *ast.CallExpr) map[int]bool {
	skip := map[int]bool{}
	sel, ok := c.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "Printf" || len(c.Args) == 0 {
		return skip
	}
	lit, ok := c.Args[0].(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return skip
	}
	f, err := strconv.Unquote(lit.Value)
	if err != nil || strings.ContainsAny(f, "[*") {
		return skip
	}
	arg := 1
	for i := 0; i < len(f); i++ {
		if f[i] != '%' {
			continue
		}
		i++
		for i < len(f) && strings.IndexByte("+-# 0123456789.", f[i]) >= 0 {
			i++
		}
		if i >= len(f) || f[i] == '%' {
			continue
		}
		if f[i] == 'T' {
			skip[arg] = true
		}
		arg++
	}
	return skip
}

func isPrintCall(c *ast.CallExpr) bool {
	sel, ok := c.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	id, ok := sel.X.(*ast.Ident)
	if !ok || id.Name != "fmt" {
		return false
	}
	switch sel.Sel.Name {
	case "Println", "Printf", "Print", "Sprint", "Sprintf", "Sprintln", "Errorf":
		return true
	}
	return false
}

// addressInside reports what in a value of type t that fmt prints is an address: a func or
// chan anywhere, a pointer anywhere but at the top (where a pointer to a struct, array, slice
// or map prints as &{...}).
func addressInside(t types.Type, top bool, seen map[types.Type]bool) string {
	if seen[t] {
		return ""
	}
	seen[t] = true
	// a type with a String, Error or Format method is printed by it (time.Time, *time.Location)
	ms := types.NewMethodSet(t)
	for _, name := range []string{"String", "Error", "Format"} {
		for i := 0; i < ms.Len(); i++ {
			if ms.At(i).Obj().Name() == name {
				return ""
			}
		}
	}
	switch u := t.Underlying().(type) {
	case *types.Signature:
		return "a func"
	case *types.Chan:
		return "a chan"
	case *types.Pointer:
		if !top {
			return "a pointer"
		}
		switch u.Elem().Underlying().(type) {
		case *types.Struct, *types.Array, *types.Slice, *types.Map:
			return addressInside(u.Elem(), false, seen)
		}
		return "a pointer to a basic value"
	case *types.Slice:
		return addressInside(u.Elem(), false, seen)
	case *types.Array:
		return addressInside(u.Elem(), false, seen)
	case *types.Map:
		if s := addressInside(u.Key(), false, seen); s != "" {
			return s
		}
		return addressInside(u.Elem(), false, seen)
	case *types.Struct:
		for i := 0; i < u.NumFields(); i++ {
			if s := addressInside(u.Field(i).Type(), false, seen); s != "" {
				return s
			}
		}
	}
	return ""
}
