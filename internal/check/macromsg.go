package check

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"regexp"
	"strconv"
	"strings"

	"github.com/huuhait/gocpp/internal/diag"
	"github.com/huuhait/gocpp/internal/load"
)

// Messages about macros (Phase 10, step 10.9). The loader turns a macro Assert into the function
// Assert (with hidden parameters) and each use #Assert(x) into a call of it
// (internal/load/macros.go), so go/types talks about a function the user wrote as a macro. Two
// things give the messages back the words of the user's code:
//
//   - macroFixer changes the messages that name a macro:
//
//     cannot use 1 (untyped int constant) as bool value in argument to util.Assert
//     ->  ... in argument to #util.Assert
//     in call to Dbg, cannot infer T  ->  in call to #Dbg, cannot infer T
//
//     and adds where the macro is declared, so that the error points at the use (its place)
//     and at the macro (the last line).
//
//   - plainMacroUses finds the uses of a macro's function that are not the calls that the
//     rewrite made: Assert(x, "m") and f := util.Dbg. go/types accepts the name and then complains
//     about the hidden parameters, so these uses get one message ("Assert is a macro: write
//     #Assert(...)") and the errors of go/types inside the call are dropped. It finds the
//     opposite too: a call that the rewrite made whose name a variable of the function hides
//     (a local show next to #show(1)), which would call the variable.

var macroCallTo = regexp.MustCompile(`\b(argument|call) to ((?:[A-Za-z_][A-Za-z0-9_]*\.)?)([A-Za-z_][A-Za-z0-9_]*)\b`)

// hasMacros reports whether the package, or a package it imports, declares a macro.
func hasMacros(lp *load.Package) bool {
	if len(lp.Macros) > 0 {
		return true
	}
	for _, dep := range lp.Imports {
		if len(dep.Macros) > 0 {
			return true
		}
	}
	return false
}

// macroFixer returns the function that fixes the messages of the package lp, or nil if neither lp
// nor a package it imports declares a macro.
func macroFixer(lp *load.Package) func(string) string {
	if !hasMacros(lp) {
		return nil
	}
	own := lp.Macros
	deps := map[string]*load.Package{} // by the name that the files give the package: its name
	for _, dep := range lp.Imports {
		if len(dep.Macros) > 0 {
			deps[dep.Name] = dep
		}
	}
	return func(msg string) string {
		// "Assert("main.gp", 19, ..., (func() string literal)) (no value) used as value" says the rewritten call.
		if i := strings.Index(msg, "(...) (no value) used as value"); i < 0 {
			if j := strings.Index(msg, ") (no value) used as value"); j > 0 {
				callee, _, _ := strings.Cut(msg, "(")
				name := callee
				var sig *load.MacroSig
				if q, n, ok := strings.Cut(callee, "."); ok {
					if dep := deps[q]; dep != nil {
						sig, name = dep.Macros[n], callee
					}
				} else {
					sig = own[callee]
				}
				if sig != nil {
					return "#" + name + "(...) has no value: the macro has no result, so it can't be used as a value\n\tmacro " + sig.Name + " is declared at " + sig.File + ":" + strconv.Itoa(sig.Line)
				}
			}
		}
		// "cannot use Min("g.gp", 20, "main.main", 1, 2) (value of type int) as ..." says the rewritten call.
		if rest, ok := strings.CutPrefix(msg, "cannot use "); ok {
			if k := strings.Index(rest, ") (value of type "); k > 0 {
				callee, _, _ := strings.Cut(rest, "(")
				var sig *load.MacroSig
				if q, n, ok := strings.Cut(callee, "."); ok {
					if dep := deps[q]; dep != nil {
						sig = dep.Macros[n]
					}
				} else {
					sig = own[callee]
				}
				if sig != nil && !strings.Contains(callee, " ") {
					return "cannot use #" + callee + "(...) (value of type " + rest[k+len(") (value of type "):] +
						"\n\tmacro " + sig.Name + " is declared at " + sig.File + ":" + strconv.Itoa(sig.Line)
				}
			}
		}
		var declared *load.MacroSig
		msg = macroCallTo.ReplaceAllStringFunc(msg, func(m string) string {
			sub := macroCallTo.FindStringSubmatch(m)
			what, qual, name := sub[1], sub[2], sub[3]
			var sig *load.MacroSig
			if qual == "" {
				sig = own[name]
			} else if dep := deps[strings.TrimSuffix(qual, ".")]; dep != nil {
				sig = dep.Macros[name]
			}
			if sig == nil {
				return m
			}
			if declared == nil {
				declared = sig
			}
			return what + " to #" + qual + name
		})
		if name, ok := strings.CutSuffix(strings.SplitN(msg, "\n", 2)[0], " redeclared in this block"); ok && own[name] != nil {
			msg = strings.Replace(msg, " redeclared in this block", " redeclared in this block (a macro and a func, or two macros, can't have the same name)", 1)
		}
		if declared != nil {
			msg += "\n\tmacro " + declared.Name + " is declared at " + declared.File + ":" + strconv.Itoa(declared.Line)
		}
		return msg
	}
}

// A plainUse is a use of the function of a macro that is not a call that the rewrite made.
type plainUse struct {
	id       *ast.Ident
	text     string // how the use names the macro: Assert, util.Assert
	from, to token.Pos
	hidden   bool   // a call of the rewrite that a declaration of the same name hides
	what     string // what hides it: "a variable (or a parameter)", "an import"
}

// plainMacroUses finds the uses of the functions of macros in info that are not the calls that the
// loader's rewrite made (Package.MacroUses), with the extent of the call that holds each. It
// returns nil for a package that sees no macro.
func plainMacroUses(lp *load.Package, files []*ast.File, info *types.Info) []plainUse {
	if !hasMacros(lp) {
		return nil
	}
	var plain map[*ast.Ident]bool
	for id, obj := range info.Uses {
		fn, ok := obj.(*types.Func)
		if !ok || fn.Pkg() == nil || fn.Signature().Recv() != nil || lp.MacroUses[id] != nil {
			continue
		}
		var macros map[string]*load.MacroSig
		if fn.Pkg().Path() == lp.ImportPath {
			macros = lp.Macros
		} else if dep := lp.Imports[fn.Pkg().Path()]; dep != nil {
			macros = dep.Macros
		}
		if macros[fn.Name()] == nil || fn.Parent() != fn.Pkg().Scope() {
			continue
		}
		if plain == nil {
			plain = make(map[*ast.Ident]bool)
		}
		plain[id] = true
	}
	var out []plainUse
	// The calls that the rewrite made whose callee is not the macro: a variable hides its name.
	for id, mu := range lp.MacroUses {
		if mu.Call == nil || !id.Pos().IsValid() {
			continue
		}
		fn, ok := info.Uses[id].(*types.Func)
		if ok && fn.Pkg() != nil && fn.Pkg().Path() == mu.Path && fn.Name() == mu.Sig.Name && fn.Parent() == fn.Pkg().Scope() {
			continue
		}
		if _, used := info.Uses[id]; !used {
			continue // not checked (an error before)
		}
		what := "a variable (or a parameter)"
		switch info.Uses[id].(type) {
		case *types.PkgName:
			what = "an import"
		case *types.Func, *types.TypeName, *types.Const:
			what = "a declaration"
		}
		out = append(out, plainUse{id: id, text: id.Name, from: mu.Call.Pos(), to: mu.Call.End(), hidden: true, what: what})
	}
	if plain == nil {
		return out
	}
	// The call that holds each use, if it is the callee of one.
	callee := func(e ast.Expr) *ast.Ident {
		for {
			switch x := e.(type) {
			case *ast.ParenExpr:
				e = x.X
			case *ast.IndexExpr:
				e = x.X
			case *ast.IndexListExpr:
				e = x.X
			case *ast.Ident:
				return x
			case *ast.SelectorExpr:
				return x.Sel
			default:
				return nil
			}
		}
	}
	done := make(map[*ast.Ident]bool)
	sels := make(map[*ast.Ident]*ast.SelectorExpr) // pkg.Name: the extent of a use is the selector's
	for _, f := range files {
		ast.Inspect(f, func(n ast.Node) bool {
			if sel, ok := n.(*ast.SelectorExpr); ok && plain[sel.Sel] {
				sels[sel.Sel] = sel
			}
			if call, ok := n.(*ast.CallExpr); ok {
				if id := callee(call.Fun); id != nil && plain[id] && !done[id] {
					done[id] = true
					out = append(out, plainUse{id: id, text: types.ExprString(call.Fun), from: call.Pos(), to: call.End()})
				}
			}
			return true
		})
	}
	for id := range plain {
		if !done[id] {
			text := id.Name
			if obj := info.Uses[id]; obj != nil && obj.Pkg() != nil && obj.Pkg().Path() != lp.ImportPath {
				text = obj.Pkg().Name() + "." + id.Name
			}
			from, to := id.Pos(), id.End()
			if sel := sels[id]; sel != nil {
				from, to = sel.Pos(), sel.End()
			}
			out = append(out, plainUse{id: id, text: text, from: from, to: to})
		}
	}
	return out
}

// reportPlainMacroUses adds the errors for the plain uses of macros to errs, and returns the
// function that tells the errors of go/types that are inside one of them (to be dropped).
func reportPlainMacroUses(fset *token.FileSet, errs *diag.List, plain []plainUse) func(token.Pos) bool {
	for _, u := range plain {
		msg := fmt.Sprintf("%s is a macro: write #%s(...)", u.text, u.text)
		if u.hidden {
			msg = fmt.Sprintf("the macro %s is hidden here by %s with the same name: rename it", u.text, u.what)
		}
		errs.Add(diag.Diagnostic{Pos: fset.Position(u.from), End: fset.Position(u.to), Msg: msg})
	}
	return func(pos token.Pos) bool {
		for _, u := range plain {
			if u.from <= pos && pos < u.to {
				return true
			}
		}
		return false
	}
}
