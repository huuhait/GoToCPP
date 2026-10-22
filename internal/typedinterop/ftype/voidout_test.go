package ftype

import (
	"go/types"
	"testing"
)

func TestPointerToVoidOut(t *testing.T) {
	obj := types.NewNamed(types.NewTypeName(0, nil, "Obj", nil), types.NewStruct(nil, nil), nil)
	pp := types.NewPointer(types.NewPointer(obj))
	unsafePtr := types.NewPointer(types.Typ[types.UnsafePointer])
	tests := []struct {
		name     string
		from, to types.Type
		want     bool
	}{
		{"**T to *unsafe.Pointer", pp, unsafePtr, true},
		{"*T to *unsafe.Pointer", types.NewPointer(obj), unsafePtr, false},
		{"**T to **T", pp, pp, false},
		{"**T to *int", pp, types.NewPointer(types.Typ[types.Int]), false},
		{"T to *unsafe.Pointer", obj, unsafePtr, false},
		{"**T to unsafe.Pointer", pp, types.Typ[types.UnsafePointer], false},
	}
	for _, tt := range tests {
		if got := pointerToVoidOut(tt.from, tt.to); got != tt.want {
			t.Errorf("%s: pointerToVoidOut = %v, want %v", tt.name, got, tt.want)
		}
	}
}
