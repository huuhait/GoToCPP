package stdlib_test

import (
	"go/ast"
	"go/importer"
	"go/token"
	"go/types"
	"io/fs"
	"path"
	"sort"
	"strings"
	"testing"

	"github.com/huuhait/gocpp/internal/lower"
	"github.com/huuhait/gocpp/internal/syntax/parser"
	"github.com/huuhait/gocpp/stdlib"
)

// TestStdGoLayouts checks that every exported struct type of the standard packages has the layout (size,
// alignment, pointers) that real Go 1.25 gives it on linux/amd64, because the capacity that
// append gives a slice depends on it (runtime/include/gocpp/golayout.hpp computes Go's layout
// from the fields of the gocpp struct). A type whose gocpp fields differ from Go's (a stub that
// leaves fields out, sync.Cond) is in lower.StdGoLayouts with Go's numbers, and this test checks
// them against real Go; a type that is not there must have the same layout as Go's.
func TestStdGoLayouts(t *testing.T) {
	sizes := types.SizesFor("gc", "amd64")
	table := lower.StdGoLayouts()
	fset := token.NewFileSet()
	imp := importer.ForCompiler(token.NewFileSet(), "source", nil)

	var lay func(types.Type) (int64, int64, bool)
	hasPtr := func(t types.Type) bool {
		_, _, p := lay(t)
		return p
	}
	_ = hasPtr
	// lay is gc's layout with the table's numbers for the types that it lists (a gocpp type that
	// contains one gets Go's size of it, as the C++ computes).
	lay = func(t types.Type) (int64, int64, bool) {
		if n, ok := types.Unalias(t).(*types.Named); ok && n.Obj().Pkg() != nil {
			if l, ok := table[n.Origin().Obj().Pkg().Path()+"."+n.Origin().Obj().Name()]; ok {
				return l.Size, l.Align, l.Ptrs
			}
		}
		switch u := t.Underlying().(type) {
		case *types.Basic:
			return sizes.Sizeof(t), sizes.Alignof(t), u.Info()&types.IsString != 0 || u.Kind() == types.UnsafePointer
		case *types.Array:
			s, a, p := lay(u.Elem())
			return s * u.Len(), a, p && u.Len() > 0
		case *types.Struct:
			var off, last, lastSize, align int64 = 0, 0, 0, 1
			ptrs := false
			for i := range u.NumFields() {
				s, a, p := lay(u.Field(i).Type())
				off = (off + a - 1) / a * a
				last, lastSize = off, s
				off += s
				align = max(align, a)
				ptrs = ptrs || p
			}
			if u.NumFields() == 0 {
				return 0, 1, false
			}
			if last > 0 && lastSize == 0 {
				lastSize = 1
			}
			return (last + lastSize + align - 1) / align * align, align, ptrs
		}
		return sizes.Sizeof(t), sizes.Alignof(t), true
	}

	otherOS := []string{"_windows", "_darwin", "_plan9", "_js", "_wasm", "_freebsd", "_openbsd", "_netbsd", "_solaris", "_aix", "_arm64", "_386", "_arm", "_riscv64", "_s390x", "_ppc64", "_mips", "_test"}
	dirs := map[string][]string{}
	fs.WalkDir(stdlib.FS(), ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || path.Ext(p) != ".gp" {
			return nil
		}
		base := strings.TrimSuffix(path.Base(p), ".gp")
		for _, o := range otherOS {
			if strings.Contains(base, o) {
				return nil
			}
		}
		dirs[path.Dir(p)] = append(dirs[path.Dir(p)], p)
		return nil
	})
	var keys []string
	for k := range dirs {
		if !strings.Contains(k, "internal") && !strings.HasPrefix(k, "runtime") {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)

	seen := map[string]bool{}
	checked := 0
	for _, dir := range keys {
		var files []*ast.File
		for _, p := range dirs[dir] {
			src, err := fs.ReadFile(stdlib.FS(), p)
			if err != nil {
				t.Fatal(err)
			}
			f, err := parser.ParseFile(fset, p, src, 0)
			if err != nil {
				continue // TestStubsParse reports it
			}
			files = append(files, f)
		}
		conf := types.Config{Importer: imp, Error: func(error) {}, FakeImportC: true}
		pkg, _ := conf.Check(dir, fset, files, nil)
		real, err := imp.Import(dir)
		if pkg == nil || err != nil {
			t.Logf("%s: not compared (%v)", dir, err)
			continue
		}
		for _, name := range pkg.Scope().Names() {
			tn, ok := pkg.Scope().Lookup(name).(*types.TypeName)
			if !ok {
				continue
			}
			if _, ok := tn.Type().Underlying().(*types.Struct); !ok || !tn.Exported() {
				continue // a program cannot name an unexported type
			}
			ro := real.Scope().Lookup(name)
			if ro == nil {
				continue // a type that Go's package does not have
			}
			key := dir + "." + name
			rs, ra, rp := sizes.Sizeof(ro.Type()), sizes.Alignof(ro.Type()), true
			if _, _, p := layOf(ro.Type(), sizes); !p {
				rp = false
			}
			if named, ok := tn.Type().(*types.Named); ok && named.TypeParams().Len() > 0 {
				// A generic type: its layout does not depend on the type argument here (the
				// table value is checked against Go's with the argument int).
				if l, ok := table[key]; ok {
					seen[key] = true
					if l.Size != rs || l.Align != ra || l.Ptrs != rp {
						t.Errorf("%s: table says %+v, real Go %d/%d/%v", key, l, rs, ra, rp)
					}
				}
				continue
			}
			checked++
			if l, ok := table[key]; ok {
				seen[key] = true
				if l.Size != rs || l.Align != ra || l.Ptrs != rp {
					t.Errorf("%s: StdGoLayouts says %+v, real Go has size %d, align %d, pointers %v", key, l, rs, ra, rp)
				}
				continue
			}
			// Own fields, nested types as the table has them.
			st := tn.Type().Underlying()
			s, a, p := lay(st)
			if s != rs || a != ra || p != rp {
				t.Errorf("%s: the gocpp struct has size %d, align %d, pointers %v; real Go %d, %d, %v: add it to lower.stdGoLayouts (internal/lower/golayouts.go)", key, s, a, p, rs, ra, rp)
			}
		}
	}
	for k := range table {
		if !seen[k] {
			t.Errorf("lower.stdGoLayouts has %s, which is not a struct of the standard packages", k)
		}
	}
	t.Logf("%d struct types compared with real Go", checked)
}

// layOf is gc's layout of a type of real Go (no table): its pointer-ness is what matters.
func layOf(t types.Type, sizes types.Sizes) (int64, int64, bool) {
	var ptrs func(types.Type) bool
	ptrs = func(t types.Type) bool {
		switch u := t.Underlying().(type) {
		case *types.Basic:
			return u.Info()&types.IsString != 0 || u.Kind() == types.UnsafePointer
		case *types.Array:
			return u.Len() > 0 && ptrs(u.Elem())
		case *types.Struct:
			for i := range u.NumFields() {
				if ptrs(u.Field(i).Type()) {
					return true
				}
			}
			return false
		}
		return true
	}
	return sizes.Sizeof(t), sizes.Alignof(t), ptrs(t)
}
