package cache

import "testing"

const headerGroupsSource = `package main
import "./api.hpp"

func f() {
	var win api.Widget
	_ = win.size()
	_ = api.make()
}
`

func headerGroupsPackage(t *testing.T) *Package {
	t.Helper()
	world := newWorld(t, testConfig(), map[string]string{
		"gocpp.mod": "module example.com/groups\n",
		"main.gp":   headerGroupsSource,
		"api.hpp":   memberAPIHeader,
	})
	t.Cleanup(world.p.Close)
	return world.pkg(".")
}

func TestHeaderGroupsAreMadeOncePerPackage(t *testing.T) {
	pkg := headerGroupsPackage(t)
	first := pkg.headerGroups()
	if len(first) != 1 || len(first[0].includes) != 1 {
		t.Fatalf("groups = %+v, want one group with the one include", first)
	}
	if len(first[0].names) == 0 {
		t.Fatalf("group has no names: %+v", first[0])
	}
	if second := pkg.headerGroups(); second[0] != first[0] {
		t.Fatal("the groups were made again for the same package")
	}
}

func TestOpaqueVariableUseIsFoundOncePerPackage(t *testing.T) {
	pkg := headerGroupsPackage(t)
	if !pkg.hasOpaqueVariableUse() {
		t.Fatal("the package uses a variable of a header type, but none was found")
	}
	if _, memoized := pkg.memo.Load(opaqueVariableUseKey{}); !memoized {
		t.Fatal("the answer was not kept with the package")
	}
}
