package printer

import "go/token"

// A lineTable answers the position (unadjusted by //line directives) of a token.Pos of the
// file it was made for, which is the file the printer prints nearly always. It is what
// token.FileSet.PositionFor computes, without the lock and the search from scratch: the
// printer asks for positions that mostly move forward, so the line found last is the
// first one tried. The zero value is ready to use.
type lineTable struct {
	file   *token.File
	starts []int // the offset of the start of each line of file
	last   int   // the index of the line found last
}

// position returns fset.PositionFor(pos, false).
func (t *lineTable) position(fset *token.FileSet, pos token.Pos) token.Position {
	if pos == token.NoPos {
		return token.Position{}
	}
	f := t.file
	if f == nil || int(pos) < f.Base() || int(pos) > f.Base()+f.Size() {
		f = fset.File(pos)
		if f == nil {
			return token.Position{}
		}
		t.file, t.starts, t.last = f, f.Lines(), 0
	}
	offset := int(pos) - f.Base()
	line := t.lineOf(offset)
	position := token.Position{Filename: f.Name(), Offset: offset}
	if line >= 0 {
		position.Line = line + 1
		position.Column = offset - t.starts[line] + 1
	}
	return position
}

// lineOf returns the index of the line that holds offset, or -1.
func (t *lineTable) lineOf(offset int) int {
	starts := t.starts
	i := t.last
	if i < len(starts) && starts[i] <= offset && (i+1 == len(starts) || offset < starts[i+1]) {
		return i
	}
	if i+1 < len(starts) && starts[i+1] <= offset && (i+2 == len(starts) || offset < starts[i+2]) {
		t.last = i + 1
		return i + 1
	}
	lo, hi := 0, len(starts) // the last index with starts[index] <= offset
	for lo < hi {
		mid := int(uint(lo+hi) >> 1)
		if starts[mid] <= offset {
			lo = mid + 1
		} else {
			hi = mid
		}
	}
	t.last = max(lo-1, 0)
	return lo - 1
}
