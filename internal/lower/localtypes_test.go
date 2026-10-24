package lower

import (
	"go/token"
	"testing"
)

// TestPlaceInFuncAcrossFiles checks the place of a local type in its function's tag
// (localOrigin): within one file it is the offset from the function's start, and across files (a
// method that #Derive writes in another file) the file's name and the offset in it, which do not
// change when another file of the package grows (the token.Pos difference did: the names of the
// instances of derive_json_plain changed with the size of stdlib/sync/oncefunc.gp).
func TestPlaceInFuncAcrossFiles(t *testing.T) {
	tags := func(gap int) (same, other string) {
		fset := token.NewFileSet()
		a := fset.AddFile("/p/a.gp", -1, 1000)
		fset.AddFile("/p/between.gp", -1, gap)
		b := fset.AddFile("/p/z_derive.gp", -1, 1000)
		g := &gen{fset: fset}
		return g.placeInFunc(a.Pos(120), a.Pos(100)), g.placeInFunc(b.Pos(30), a.Pos(100))
	}
	same1, other1 := tags(10)
	same2, other2 := tags(5000)
	if same1 != "20" || same2 != "20" {
		t.Errorf("in one file: %q and %q, want 20", same1, same2)
	}
	if other1 != "z_derive_gp_30" || other2 != other1 {
		t.Errorf("across files: %q and %q, want z_derive_gp_30 both", other1, other2)
	}
}
