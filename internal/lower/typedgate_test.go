package lower

import (
	"context"
	"go/ast"
	"strings"
	"testing"

	"github.com/huuhait/gocpp/internal/interop"
	"github.com/huuhait/gocpp/internal/typedinterop"
	"github.com/huuhait/gocpp/internal/typedinterop/probe"
	"github.com/huuhait/gocpp/internal/typedinterop/tsupport"
)

// typedFor checks src (main.gp, with the header h.hpp) with the real compiler and returns the
// lowering's state with the typed report of the package, and the report.
func typedFor(t *testing.T, src, hdr string) (*gen, *typedinterop.Report) {
	t.Helper()
	clang := tsupport.Clang(t)
	pkg, fset := tsupport.CheckSource(t, src, map[string]string{"h.hpp": hdr})
	rep, err := typedinterop.Analyze(context.Background(), pkg, fset, typedinterop.Options{Clang: clang, Decls: true, StringToCStr: true})
	if err != nil {
		t.Fatal(err)
	}
	g := newGen(fset, pkg)
	g.interop = &Interop{}
	g.interop.SetTyped(rep, false)
	return g, rep
}

// callTo returns the first call in the package whose text starts with prefix.
func callTo(t *testing.T, g *gen, prefix string) *ast.CallExpr {
	t.Helper()
	var found *ast.CallExpr
	for _, f := range g.pkg.Load.Files {
		ast.Inspect(f.AST, func(n ast.Node) bool {
			if c, ok := n.(*ast.CallExpr); ok && found == nil && strings.HasPrefix(g.srcText(c), prefix) {
				found = c
			}
			return found == nil
		})
	}
	if found == nil {
		t.Fatalf("no call %s...", prefix)
	}
	return found
}

// TestTypedValueGates checks when the typed view of an expression is used (tvalOf): not when
// clang recovered from an error in it, not for a type with no foreign view, and nothing at all when
// the headers do not compile for the probe (SetTyped).
func TestTypedValueGates(t *testing.T) {
	const hdr = `#pragma once
class Box {
    int hid() { return 1; }
public:
    int pub() { return 2; }
};
__int128 big();
int plain(int);
`
	const src = `package main

import "./h.hpp"

func main() {
	var b h.Box
	_ = b.pub()
	_ = b.hid()
	_ = h.big()
	_ = h.plain(1)
}
`
	g, rep := typedFor(t, src, hdr)
	if v := g.tvalOf(callTo(t, g, "h.plain")); v == nil {
		t.Errorf("the call of an ordinary function has no typed view")
	}
	if v := g.tvalOf(callTo(t, g, "b.pub")); v == nil {
		t.Errorf("the call of a public method has no typed view")
	}
	if v := g.tvalOf(callTo(t, g, "b.hid")); v != nil {
		t.Errorf("the call of a private method (clang recovered from the error) has a typed view: %v", v.Value)
	}
	if v := g.tvalOf(callTo(t, g, "h.big")); v != nil {
		t.Errorf("a __int128 result (opaque in the foreign view) has a typed view: %v", v.Value)
	}

	// Headers that do not compile for the probe, or cannot be included: no typed lowering.
	bad := *rep
	bad.IncludeErrors = []interop.IncludeError{{Include: "<nope>", Msg: "no such header"}}
	in := &Interop{}
	in.SetTyped(&bad, false)
	if in.typed != nil {
		t.Errorf("SetTyped took a report with include errors")
	}
	tab := *rep.Table
	tab.HeaderErrors = append(tab.HeaderErrors[:0:0], probe.Diag{File: "h.hpp", Line: 1, Severity: "error", Msg: "bad"})
	bad2 := *rep
	bad2.Table = &tab
	in = &Interop{}
	in.SetTyped(&bad2, false)
	if in.typed != nil {
		t.Errorf("SetTyped took a report with header errors")
	}
	in = &Interop{}
	in.SetTyped(rep, false)
	if in.typed == nil {
		t.Errorf("SetTyped refused a good report")
	}
}
