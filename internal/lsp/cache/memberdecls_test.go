package cache

import (
	"go/types"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/huuhait/gocpp/internal/typedinterop/probe"
)

const memberAPIHeader = `struct Base { int width; };
struct Widget : Base {
  // Number of items.
  int size() const;
  int size(int limit) const;
  int other;
};
Widget make();
struct Other { double size(char c) const; };
Other makeOther();
`

const memberSource = `package main
import "./api.hpp"

func f() {
	var win api.Widget
	_ = win.size()
	_ = win.width
	_ = win.missing
}
`

func memberWorld(t *testing.T, source string) (*world, *Package, *Snapshot, func(), *types.Var) {
	t.Helper()
	t.Setenv("GOCPP_INTEROP", "typed")
	t.Setenv("GOCPP_NOPROBECACHE", "1")
	t.Cleanup(probe.SetRacyWindow(0))
	world := newWorld(t, testConfig(), map[string]string{
		"gocpp.mod": "module example.com/members\n",
		"main.gp":   source,
		"api.hpp":   memberAPIHeader,
	})
	t.Cleanup(world.p.Close)
	pkg := world.pkg(".")
	snap, release := world.s.Snapshot()
	facts := waitMemberEntry(t, world.p.members, memberEntryKey(snap, pkg))
	var variable *types.Var
	for object := range facts.bindings {
		variable = object
	}
	if variable == nil {
		release()
		t.Fatal("no binding")
	}
	return world, pkg, snap, release, variable
}

// waitMemberDecls waits for the second step of the analysis: its facts replace the published ones.
func waitMemberDecls(t *testing.T, snap *Snapshot, pkg *Package, object *types.Var, name string) {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for {
		if _, ok := snap.ForeignMemberDeclarations(pkg, object, name); ok {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("no declarations of %s after the analysis", name)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestMemberDeclarationsOfAProvenReceiver(t *testing.T) {
	world, pkg, snap, release, win := memberWorld(t, memberSource)
	defer release()
	waitMemberDecls(t, snap, pkg, win, "size")
	header := filepath.Join(world.root, "api.hpp")
	t.Run("overload set", func(t *testing.T) {
		decls, ok := snap.ForeignMemberDeclarations(pkg, win, "size")
		if !ok || len(decls) != 2 {
			t.Fatalf("size = %v %v", decls, ok)
		}
		if decls[0].File != header || decls[0].Line != 3 || decls[0].Col != 6 ||
			decls[0].Signature != "int Widget::size() const" || decls[0].Doc != "Number of items." {
			t.Errorf("size() = %+v", decls[0])
		}
		if decls[1].Line != 4 || decls[1].Signature != "int Widget::size(int limit) const" {
			t.Errorf("size(int) = %+v", decls[1])
		}
	})
	t.Run("inherited member points into the base", func(t *testing.T) {
		decls, ok := snap.ForeignMemberDeclarations(pkg, win, "width")
		if !ok || len(decls) != 1 || decls[0].Line != 0 || decls[0].Signature != "int Widget::width" {
			t.Fatalf("width = %v %v", decls, ok)
		}
	})
	t.Run("a member that the file does not write has no fact", func(t *testing.T) {
		if decls, ok := snap.ForeignMemberDeclarations(pkg, win, "other"); ok {
			t.Errorf("other = %v", decls)
		}
	})
	t.Run("a member that the class does not have has no fact", func(t *testing.T) {
		if decls, ok := snap.ForeignMemberDeclarations(pkg, win, "missing"); ok {
			t.Errorf("missing = %v", decls)
		}
	})
	t.Run("a repaired variant has no fact", func(t *testing.T) {
		variant := &Package{Check: pkg.Check, Program: pkg.Program, variant: true}
		if decls, ok := snap.ForeignMemberDeclarations(variant, win, "size"); ok {
			t.Errorf("variant = %v", decls)
		}
	})
	t.Run("another binding of the same spelling has no fact", func(t *testing.T) {
		other := types.NewVar(win.Pos(), win.Pkg(), win.Name(), win.Type())
		if decls, ok := snap.ForeignMemberDeclarations(pkg, other, "size"); ok {
			t.Errorf("other binding = %v", decls)
		}
	})
	t.Run("the answer is a copy", func(t *testing.T) {
		decls, _ := snap.ForeignMemberDeclarations(pkg, win, "size")
		decls[0].Signature = "changed"
		again, _ := snap.ForeignMemberDeclarations(pkg, win, "size")
		if again[0].Signature == "changed" {
			t.Error("the facts were changed through an answer")
		}
	})
}

func TestMemberDeclarationsAreRefreshedWithTheHeaders(t *testing.T) {
	world, pkg, snap, release, win := memberWorld(t, memberSource)
	defer release()
	key := memberEntryKey(snap, pkg)
	edited := "\n\n" + memberAPIHeader
	if err := os.WriteFile(filepath.Join(world.root, "api.hpp"), []byte(edited), 0o600); err != nil {
		t.Fatal(err)
	}
	world.p.HeadersChanged()
	waitMemberEntry(t, world.p.members, key)
	waitMemberDecls(t, snap, pkg, win, "size")
	decls, ok := snap.ForeignMemberDeclarations(pkg, win, "size")
	if !ok || len(decls) != 2 || decls[0].Line != 5 {
		t.Fatalf("size after the edit = %v %v, want line 5", decls, ok)
	}
}

func TestMemberDeclarationsOfAShadowingBindingAreItsOwn(t *testing.T) {
	source := `package main
import "./api.hpp"

func f() {
	win := api.make()
	_ = win.size()
	{
		win := api.makeOther()
		_ = win.size()
	}
}
`
	_, pkg, snap, release, win := memberWorld(t, source)
	defer release()
	waitMemberDecls(t, snap, pkg, win, "size")
	var outer, inner *types.Var
	for id, object := range pkg.Check.Info.Defs {
		if variable, ok := object.(*types.Var); ok && id.Name == "win" {
			if outer == nil || variable.Pos() < outer.Pos() {
				inner, outer = outer, variable
			} else {
				inner = variable
			}
		}
	}
	if outer == nil || inner == nil {
		t.Fatal("no two bindings")
	}
	got, ok := snap.ForeignMemberDeclarations(pkg, outer, "size")
	if !ok || len(got) != 2 || got[0].Signature != "int Widget::size() const" {
		t.Errorf("outer win.size = %v %v, want Widget's", got, ok)
	}
	got, ok = snap.ForeignMemberDeclarations(pkg, inner, "size")
	if !ok || len(got) != 1 || got[0].Signature != "double Other::size(char c) const" {
		t.Errorf("inner win.size = %v %v, want Other's", got, ok)
	}
}
