package check

import "go/types"

// CoreType returns the slice or map type that every type of the type set of the constraint c has as
// its underlying type (S ~[]E: []E, M ~map[K]V: map[K]V; also through embedded interfaces), or nil.
// The lowering makes such a type argument that slice or map (lower's shapeArgs), and check follows the
// same rule (genericcinst.go, pkgcvars.go).
func CoreType(c types.Type) types.Type {
	it, ok := c.Underlying().(*types.Interface)
	if !ok {
		return nil
	}
	var found types.Type
	for i := range it.NumEmbeddeds() {
		var ct types.Type
		switch e := it.EmbeddedType(i).(type) {
		case *types.Union:
			if e.Len() != 1 {
				return nil
			}
			ct = sliceOrMap(e.Term(0).Type().Underlying())
		default:
			ct = CoreType(e)
			if ct == nil {
				if _, isIface := e.Underlying().(*types.Interface); isIface {
					continue
				}
				ct = sliceOrMap(e.Underlying())
			}
		}
		if ct == nil {
			continue
		}
		if found != nil {
			return nil
		}
		found = ct
	}
	return found
}

// sliceOrMap returns t when it is a slice or a map type, else nil.
func sliceOrMap(t types.Type) types.Type {
	switch t.(type) {
	case *types.Slice, *types.Map:
		return t
	}
	return nil
}
