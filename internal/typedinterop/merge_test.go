package typedinterop

import (
	"slices"
	"testing"

	"github.com/huuhait/gocpp/internal/typedinterop/probe"
)

func emptyDeclTable() *probe.DeclTable {
	return &probe.DeclTable{
		Records: map[string]*probe.Record{},
		Enums:   map[string]*probe.Enum{},
		Funcs:   map[string]*probe.FuncSet{},
		Canon:   map[string]string{},
		Errors:  map[string]string{},
	}
}

// The files that the merged tables read are listed once each, in order.
func TestMergeDeclsListsEachDependencyOnce(t *testing.T) {
	dst := emptyDeclTable()
	dst.Deps = []string{"/b.hpp", "/a.hpp"}
	src := emptyDeclTable()
	src.Deps = []string{"/c.hpp", "/a.hpp", "/b.hpp"}
	mergeDecls(dst, src)
	want := []string{"/a.hpp", "/b.hpp", "/c.hpp"}
	if !slices.Equal(dst.Deps, want) {
		t.Errorf("deps: %v, want %v", dst.Deps, want)
	}
}
