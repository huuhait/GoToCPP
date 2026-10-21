package printer

import (
	"go/token"
	"testing"
)

// fileOfLines adds a file to fset whose lines are the given lengths.
func fileOfLines(fset *token.FileSet, name string, lengths ...int) *token.File {
	size := 0
	for _, n := range lengths {
		size += n + 1
	}
	f := fset.AddFile(name, -1, size)
	offset := 0
	for _, n := range lengths {
		f.AddLine(offset)
		offset += n + 1
	}
	return f
}

func TestLineTableGivesThePositionsOfTheFileSet(t *testing.T) {
	fset := token.NewFileSet()
	f := fileOfLines(fset, "a.go", 0, 3, 0, 0, 12, 1, 7)

	var table lineTable
	for pos := f.Base(); pos <= f.Base()+f.Size(); pos++ {
		got := table.position(fset, token.Pos(pos))
		want := fset.PositionFor(token.Pos(pos), false)

		if got != want {
			t.Errorf("pos %d: got %v, want %v", pos, got, want)
		}
	}
}

func TestLineTableGivesThePositionsGoingBackward(t *testing.T) {
	fset := token.NewFileSet()
	f := fileOfLines(fset, "a.go", 5, 0, 9, 2, 2, 30)

	var table lineTable
	for pos := f.Base() + f.Size(); pos >= f.Base(); pos-- {
		got := table.position(fset, token.Pos(pos))
		want := fset.PositionFor(token.Pos(pos), false)

		if got != want {
			t.Errorf("pos %d: got %v, want %v", pos, got, want)
		}
	}
}

func TestLineTableGivesThePositionsOfTheFileOfThePosition(t *testing.T) {
	fset := token.NewFileSet()
	first := fileOfLines(fset, "a.go", 4, 4)
	second := fileOfLines(fset, "b.go", 10, 3, 3)

	var table lineTable
	for _, pos := range []token.Pos{
		token.Pos(first.Base() + 3), token.Pos(second.Base() + 12), token.Pos(first.Base()),
		token.Pos(second.Base()), token.Pos(first.Base() + first.Size()),
	} {
		got := table.position(fset, pos)
		want := fset.PositionFor(pos, false)

		if got != want {
			t.Errorf("pos %d: got %v, want %v", pos, got, want)
		}
	}
}

func TestLineTableIgnoresTheLineDirectives(t *testing.T) {
	fset := token.NewFileSet()
	f := fileOfLines(fset, "a.go", 10, 10, 10)
	f.AddLineInfo(f.Offset(token.Pos(f.Base()+12)), "other.go", 50)

	var table lineTable
	pos := token.Pos(f.Base() + 15)
	got := table.position(fset, pos)

	if got.Filename != "a.go" || got.Line != 2 {
		t.Errorf("got %v, want the position in a.go line 2", got)
	}
}

func TestLineTableHasNoPositionWithoutAFile(t *testing.T) {
	fset := token.NewFileSet()
	fileOfLines(fset, "a.go", 4)

	var table lineTable
	for _, pos := range []token.Pos{token.NoPos, token.Pos(5000)} {
		if got := table.position(fset, pos); got != (token.Position{}) {
			t.Errorf("pos %d: got %v, want the zero position", pos, got)
		}
	}
}

func TestLineTableSearchesOnlyWhenTheLineIsNotNearTheLastOne(t *testing.T) {
	fset := token.NewFileSet()
	sizes := make([]int, 200)
	for i := range sizes {
		sizes[i] = 3
	}
	f := fileOfLines(fset, "a.go", sizes...)

	var table lineTable
	table.position(fset, token.Pos(f.Base()+4*100)) // line 101: a search
	table.position(fset, token.Pos(f.Base()+4*101)) // the next line

	if table.last != 101 {
		t.Errorf("last line index %d, want 101", table.last)
	}
}
