package lower

import (
	"go/ast"
	"go/types"
)

// A tinfo is what lowering reads of the type checker: go/types' Info, with the selections as
// the lowering's own type. A selection of go/types cannot be made by anyone but the checker,
// and the body of a generic function that is instantiated (generics.go) needs the selections
// of its instance: x.String() with x of the type parameter T selects the method of the
// constraint, but in the instance for T = Celsius it selects Celsius.String.
type tinfo struct {
	*types.Info

	// Selections shadows Info.Selections, and Instances Info.Instances (a list of type arguments
	// that only go/types can make is a slice here).
	Selections map[*ast.SelectorExpr]*selection
	Instances  map[*ast.Ident]typeInst
}

// newTinfo returns the tinfo of the Info that go/types made.
func newTinfo(info *types.Info) *tinfo {
	t := &tinfo{Info: info, Selections: make(map[*ast.SelectorExpr]*selection, len(info.Selections)),
		Instances: make(map[*ast.Ident]typeInst, len(info.Instances))}
	for id, in := range info.Instances {
		targs := make([]types.Type, in.TypeArgs.Len())
		for i := range targs {
			targs[i] = in.TypeArgs.At(i)
		}
		t.Instances[id] = typeInst{targs: targs, typ: in.Type}
	}
	for x, s := range info.Selections {
		t.Selections[x] = &selection{kind: s.Kind(), recv: s.Recv(), obj: s.Obj(), index: s.Index(), indirect: s.Indirect()}
	}
	return t
}

// A selection is a *types.Selection that the lowering can make: x.f, x.m or T.m, with the
// path through the embedded fields to it.
type selection struct {
	kind     types.SelectionKind
	recv     types.Type
	obj      types.Object
	index    []int
	indirect bool
}

// Kind returns the kind of the selection (FieldVal, MethodVal, MethodExpr).
func (s *selection) Kind() types.SelectionKind { return s.kind }

// Recv returns the type of x in x.f.
func (s *selection) Recv() types.Type { return s.recv }

// Obj returns the field or method that is selected.
func (s *selection) Obj() types.Object { return s.obj }

// Index returns the path of field indexes from Recv to Obj; for a method, the last index is the
// method's index in the method set of its (embedded) type.
func (s *selection) Index() []int { return s.index }

// Indirect reports whether the path goes through a pointer.
func (s *selection) Indirect() bool { return s.indirect }

// wrapSelection returns the selection of go/types s as the lowering's.
func wrapSelection(s *types.Selection) *selection {
	if s == nil {
		return nil
	}
	return &selection{kind: s.Kind(), recv: s.Recv(), obj: s.Obj(), index: s.Index(), indirect: s.Indirect()}
}
