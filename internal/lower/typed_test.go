package lower

import (
	"go/types"
	"testing"
)

// The decisions of typed code generation that need no compiler (ADR-087): which parameter kinds take a
// gocpp string, when a set of overloads agrees, which functions keep a pointer, and when two
// numbers are the same C++ type. cmd/gocpp's TestTypedRun and TestTypedEmit run the whole.

func TestSameScalar(t *testing.T) {
	i := types.Typ
	for _, c := range []struct {
		a, b types.Type
		want bool
	}{
		{i[types.Int], i[types.Int64], true},
		{i[types.Int32], i[types.Int], false},
		{i[types.Uint], i[types.Int], false},
		{i[types.Uint64], i[types.Uint], true},
		{i[types.Float64], i[types.Float32], false},
		{i[types.Float64], i[types.Int64], false},
		{i[types.Bool], i[types.Bool], true},
		{i[types.Bool], i[types.Int], false},
		{i[types.Uint8], i[types.Int8], false},
	} {
		if got := sameScalar(c.a, c.b); got != c.want {
			t.Errorf("sameScalar(%v, %v) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
}
