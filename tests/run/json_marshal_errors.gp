// phase: 09
// lsan: cycle
// encoding/json Marshal errors: UnsupportedTypeError, UnsupportedValueError, cycles, depth, MarshalerError, errors.As on each type, Indent errors.

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
)

type Node struct {
	Val  int
	Next *Node
}

type Rec struct {
	Name string
	Kids []*Rec
}

type MapCycle map[string]any

type SliceCycle []any

type HasChan struct {
	A int
	C chan int
}

type HasFunc struct {
	F func()
}

type Cx struct {
	Z complex128
}

type MarshalErr struct{}

func (MarshalErr) MarshalJSON() ([]byte, error) { return nil, errors.New("custom failure") }

type Opaque struct{}

func report(label string, v any) {
	_, err := json.Marshal(v)
	fmt.Printf("%s: %v\n", label, err)
	var ut *json.UnsupportedTypeError
	var uv *json.UnsupportedValueError
	var me *json.MarshalerError
	switch {
	case errors.As(err, &ut):
		fmt.Printf("  UnsupportedTypeError Type=%v\n", ut.Type)
	case errors.As(err, &uv):
		fmt.Printf("  UnsupportedValueError Value=%v Str=%q\n", uv.Value.Kind(), uv.Str)
	case errors.As(err, &me):
		fmt.Printf("  MarshalerError Type=%v Err=%v\n", me.Type, me.Err)
	}
}

func main() {
	report("chan", make(chan int))
	report("func", func() {})
	report("complex64", complex64(1))
	report("complex128", complex128(1))
	report("struct with chan", HasChan{A: 1})
	report("struct with nil chan", HasChan{A: 1, C: nil})
	report("struct with func", HasFunc{})
	report("struct with complex", Cx{})
	report("slice of chan", []chan int{nil})
	report("map of func", map[string]func(){"a": nil})
	report("map with chan value", map[string]chan int{})
	report("ptr to chan", new(chan int))
	report("NaN", math.NaN())
	report("Inf in struct", struct{ F float64 }{math.Inf(-1)})
	report("custom marshal error", MarshalErr{})
	report("custom marshal error nested", []any{1, MarshalErr{}})
	report("unsupported map key", map[float32]int{1: 1})
	report("unsupported map key struct", map[Node]int{})
	report("ok", Opaque{})

	n := &Node{Val: 1}
	n.Next = &Node{Val: 2, Next: n}
	report("pointer cycle", n)

	r := &Rec{Name: "r"}
	r.Kids = []*Rec{{Name: "a"}, r}
	report("slice cycle", r)

	m := MapCycle{}
	m["self"] = m
	report("map cycle", m)

	var s SliceCycle = SliceCycle{nil}
	s[0] = s
	report("slice of any cycle", s)

	// A deep (non-cyclic) value is fine.
	var deep any = 1
	for i := 0; i < 500; i++ {
		deep = []any{deep}
	}
	b, err := json.Marshal(deep)
	fmt.Println(len(b), err)

	// A DAG that shares a node is not a cycle.
	shared := &Node{Val: 9}
	b, err = json.Marshal([]*Node{shared, shared})
	fmt.Println(string(b), err)

	_, err = json.MarshalIndent(math.NaN(), "", "  ")
	fmt.Println(err)
	var uv *json.UnsupportedValueError
	fmt.Println(errors.As(err, &uv))
}

// Output:
// chan: json: unsupported type: chan int
//   UnsupportedTypeError Type=chan int
// func: json: unsupported type: func()
//   UnsupportedTypeError Type=func()
// complex64: json: unsupported type: complex64
//   UnsupportedTypeError Type=complex64
// complex128: json: unsupported type: complex128
//   UnsupportedTypeError Type=complex128
// struct with chan: json: unsupported type: chan int
//   UnsupportedTypeError Type=chan int
// struct with nil chan: json: unsupported type: chan int
//   UnsupportedTypeError Type=chan int
// struct with func: json: unsupported type: func()
//   UnsupportedTypeError Type=func()
// struct with complex: json: unsupported type: complex128
//   UnsupportedTypeError Type=complex128
// slice of chan: json: unsupported type: chan int
//   UnsupportedTypeError Type=chan int
// map of func: json: unsupported type: func()
//   UnsupportedTypeError Type=func()
// map with chan value: <nil>
// ptr to chan: json: unsupported type: chan int
//   UnsupportedTypeError Type=chan int
// NaN: json: unsupported value: NaN
//   UnsupportedValueError Value=float64 Str="NaN"
// Inf in struct: json: unsupported value: -Inf
//   UnsupportedValueError Value=float64 Str="-Inf"
// custom marshal error: json: error calling MarshalJSON for type main.MarshalErr: custom failure
//   MarshalerError Type=main.MarshalErr Err=custom failure
// custom marshal error nested: json: error calling MarshalJSON for type main.MarshalErr: custom failure
//   MarshalerError Type=main.MarshalErr Err=custom failure
// unsupported map key: json: unsupported type: map[float32]int
//   UnsupportedTypeError Type=map[float32]int
// unsupported map key struct: json: unsupported type: map[main.Node]int
//   UnsupportedTypeError Type=map[main.Node]int
// ok: <nil>
// pointer cycle: json: unsupported value: encountered a cycle via *main.Node
//   UnsupportedValueError Value=ptr Str="encountered a cycle via *main.Node"
// slice cycle: json: unsupported value: encountered a cycle via *main.Rec
//   UnsupportedValueError Value=ptr Str="encountered a cycle via *main.Rec"
// map cycle: json: unsupported value: encountered a cycle via main.MapCycle
//   UnsupportedValueError Value=map Str="encountered a cycle via main.MapCycle"
// slice of any cycle: json: unsupported value: encountered a cycle via main.SliceCycle
//   UnsupportedValueError Value=slice Str="encountered a cycle via main.SliceCycle"
// 1001 <nil>
// [{"Val":9,"Next":null},{"Val":9,"Next":null}] <nil>
// json: unsupported value: NaN
// true
