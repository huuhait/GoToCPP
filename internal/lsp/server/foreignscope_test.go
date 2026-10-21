package server

import (
	"strings"
	"testing"

	"github.com/huuhait/gocpp/internal/load"
	"github.com/huuhait/gocpp/internal/lsp/cache"
	"github.com/huuhait/gocpp/internal/typedinterop/probe"
)

func TestForeignScope(t *testing.T) {
	header := &load.CppUse{Kind: load.HeaderUse}
	std := &load.CppUse{Kind: load.NamespaceUse, Namespace: "::std"}
	nested := &load.CppUse{Kind: load.NamespaceUse, Namespace: "::std::filesystem"}
	for _, c := range []struct {
		name       string
		use        *load.CppUse
		names      []string
		scope, sym string
		ok         bool
	}{
		{"header function", header, []string{"InitWindow"}, "", "InitWindow", true},
		{"namespace type", std, []string{"string"}, "std", "string", true},
		{"nested namespace of the rename", nested, []string{"path"}, "std::filesystem", "path", true},
		{"a path after a namespace", std, []string{"chrono", "seconds"}, "std::chrono", "seconds", true},
		{"a path after a header", header, []string{"ns", "Thing"}, "ns", "Thing", true},
		{"the use itself", std, nil, "", "", false},
		{"a use of another kind", &load.CppUse{Kind: load.UnnamedHeaderUse}, []string{"x"}, "", "", false},
	} {
		t.Run(c.name, func(t *testing.T) {
			scope, name, ok := foreignScope(c.use, c.names)
			if scope != c.scope || name != c.sym || ok != c.ok {
				t.Errorf("foreignScope = %q %q %v, want %q %q %v", scope, name, ok, c.scope, c.sym, c.ok)
			}
		})
	}
}

func TestTrustedViewIsTheCheckedTextOfTheRequest(t *testing.T) {
	pkg := new(cache.Package)
	for _, c := range []struct {
		name  string
		query *query
		want  bool
	}{
		{"the checked text", &query{pkg: pkg}, true},
		{"no package", &query{}, false},
		{"a repaired check", &query{pkg: pkg, isFixed: true}, false},
		{"a repaired check that blanked a body", &query{pkg: pkg, blanked: new(repair)}, false},
		{"a view of an older text", &query{pkg: pkg, shift: new(shift)}, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := c.query.trustedView(); got != c.want {
				t.Errorf("trustedView = %v, want %v", got, c.want)
			}
		})
	}
}

func TestDeclarationNoteTellsOverloadsFromOtherDeclarations(t *testing.T) {
	structure := probe.Declaration{Kind: "type", File: "/inc/stat.h", Line: 9}
	function := probe.Declaration{Kind: "function", File: "/inc/stat.h", Line: 20}
	other := probe.Declaration{Kind: "function", File: "/inc/stat.h", Line: 21}
	for _, c := range []struct {
		name  string
		decls []probe.Declaration
		want  string
	}{
		{"one", []probe.Declaration{function}, "C/C++ function `stat`, declared at `/inc/stat.h:21`."},
		{"overloads", []probe.Declaration{function, other}, "has 2 declarations (overloads)"},
		{"a struct and a function", []probe.Declaration{structure, function}, "has 2 declarations, at"},
	} {
		t.Run(c.name, func(t *testing.T) {
			got := declarationNote("stat", c.decls)
			notOverloads := c.name == "a struct and a function"
			if !strings.Contains(got, c.want) || notOverloads && strings.Contains(got, "overloads") {
				t.Errorf("note = %q, want %q", got, c.want)
			}
		})
	}
}
