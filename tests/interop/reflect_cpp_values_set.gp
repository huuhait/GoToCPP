// phase: 09

// reflect writes and reads of C/C++ values (step 9.3, ADR-035): a C++ value is a C++ object, never a
// gocpp one, so every writer goes by the object's own type: SetString on a std::string assigns the
// bytes (a std::string_view can't take them and says so), numbers of 1, 2, 4 and 8 bytes are set
// as such, a wide integer, a class that can't be copied and a union say "not supported" or
// "opaque" with a Go panic; Set uses the type's own copy; a class is a struct without fields.
package main

import (
	"fmt"
	"reflect"

	"./reflectcpp.hpp"
	"<string>"
	"<string_view>"
	"<vector>"
	"<map>"
)

type Rec struct {
	Name  std.string
	View  std.string_view
	N     C.int
	Small C.uchar
	F     C.float
	D     C.double
	B     bool
	Sh    reflectcpp.rc.shade
	Tn    reflectcpp.rc.tone
	P     reflectcpp.rc.point
	Bits  reflectcpp.rc.bits
	U     reflectcpp.rc.either
	W     reflectcpp.rc.wide
	Big   C.longlong
	Vec   std.vector[int]
	Ptr   *C.int
	Raw   *C.char
	Fn    reflectcpp.rc.binop
	Arr   [3]C.int
	Wrap  [2]std.string
}

type Holder struct {
	Pin reflectcpp.rc.pinned
}

func panics(f func()) (p bool) {
	defer func() { p = recover() != nil }()
	f()
	return
}

func try(name string, f func()) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println(name, "panic:", r)
		}
	}()
	f()
}

func main() {
	var n C.int = 5
	r := Rec{Name: std.string("original"), N: 3, Ptr: &n}
	rv := reflect.ValueOf(&r).Elem()
	rt := rv.Type()
	for i := 0; i < rv.NumField(); i++ {
		f := rv.Field(i)
		fmt.Println(rt.Field(i).Name, f.Kind(), f.CanSet(), f.CanAddr())
	}

	// std::string
	s := rv.FieldByName("Name")
	s.SetString("a rather long replacement string, longer than the small string buffer")
	fmt.Println(s.String(), s.Len(), r.Name.size())
	s.SetString("")
	fmt.Println(s.Len(), r.Name.size(), s.IsZero())
	s.SetString("abc")
	fmt.Println(s.Index(1), s.Slice(1, 3), s.Interface().(std.string) == "abc")
	fmt.Println(s.UnsafePointer() != nil, s.Pointer() != 0)
	s.Set(reflect.ValueOf(any(std.string("via Set"))))
	fmt.Println(r.Name.size(), s.String())
	s.SetZero()
	fmt.Println(r.Name.size(), s.Len())
	try("string.SetInt", func() { s.SetInt(1) })
	try("string.SetBytes", func() { s.SetBytes([]byte("x")) })
	try("string.Set(go string)", func() { s.Set(reflect.ValueOf("go")) })
	try("string.Convert", func() { s.Convert(reflect.TypeOf("")) })
	try("string.Elem", func() { s.Elem() })
	try("string.Field", func() { s.Field(0) })
	try("string.Index range", func() { s.Index(9) })
	r.Name = std.string("copy")
	cp := rv.Field(0).Interface().(std.string)
	cp.append("!")
	fmt.Println(r.Name.size(), cp.size())

	// std::string_view
	v := rv.FieldByName("View")
	try("view.SetString", func() { v.SetString("x") })
	fmt.Println(v.Kind(), v.Len())

	// numbers
	n1 := rv.FieldByName("N")
	n1.SetInt(-7)
	fmt.Println(r.N, n1.Int(), n1.Type())
	n1.SetInt(1 << 33)
	fmt.Println(r.N)
	sm := rv.FieldByName("Small")
	sm.SetUint(300)
	fmt.Println(r.Small, sm.Uint())
	fl := rv.FieldByName("F")
	fl.SetFloat(1.5)
	fmt.Println(r.F, fl.Float())
	d := rv.FieldByName("D")
	d.SetFloat(2.25)
	fmt.Println(r.D, d.Float())
	b := rv.FieldByName("B")
	b.SetBool(true)
	fmt.Println(r.B, b.Bool())
	try("int.SetString", func() { n1.SetString("x") })
	try("int.SetFloat", func() { n1.SetFloat(1) })
	try("int.SetUint", func() { n1.SetUint(1) })
	try("uint.SetInt", func() { sm.SetInt(1) })
	try("float.SetInt", func() { fl.SetInt(1) })
	try("bool.SetInt", func() { b.SetInt(1) })
	try("int.Set(C.int)", func() { n1.Set(reflect.ValueOf(any(C.int(1)))) })
	n1.Set(reflect.ValueOf(int32(11)))
	fmt.Println(r.N)
	n1.SetZero()
	fmt.Println(r.N)
	conv := reflect.ValueOf(int64(70000)).Convert(sm.Type())
	fmt.Println(conv.Type() == sm.Type(), conv.Uint())
	conv = n1.Convert(reflect.TypeOf(int64(0)))
	fmt.Println(conv.Kind(), conv.Int())

	// enums
	sh := rv.FieldByName("Sh")
	sh.SetUint(2)
	fmt.Println(sh.Kind(), sh.Uint(), r.Sh)
	tn := rv.FieldByName("Tn")
	tn.SetUint(9)
	fmt.Println(tn.Kind(), tn.Uint(), r.Tn)
	try("enum.SetString", func() { sh.SetString("x") })

	// classes: structs without fields
	for _, name := range []string{"P", "Bits", "U", "Vec", "W"} {
		f := rv.FieldByName(name)
		fmt.Println(name, f.Kind(), f.NumField(), f.Type().NumField(), f.CanSet())
		ops := []func(){
			func() { f.Field(0) },
			func() { f.SetInt(1) },
			func() { f.SetString("x") },
			func() { f.SetBool(true) },
			func() { f.SetFloat(1) },
			func() { f.SetUint(1) },
			func() { f.SetBytes(nil) },
			func() { f.SetLen(1) },
			func() { f.SetMapIndex(reflect.ValueOf(1), reflect.ValueOf(2)) },
			func() { f.Index(0) },
			func() { f.Elem() },
			func() { f.Len() },
			func() { f.MapKeys() },
			func() { f.Slice(0, 0) },
			func() { f.Bytes() },
			func() { f.Convert(reflect.TypeOf(1)) },
		}
		bad := 0
		for _, op := range ops {
			if panics(op) {
				bad++
			}
		}
		fmt.Println(name, "ops that panic:", bad, "of", len(ops))
		try(name+".SetInt", func() { f.SetInt(1) })
		try(name+".Field", func() { f.Field(0) })
		fmt.Println(name, f.Addr().Kind(), reflect.TypeOf(f.Interface()) == f.Type(), f.IsZero())
	}
	p := rv.FieldByName("P")
	try("P.Set", func() { p.Set(reflect.ValueOf(any(reflectcpp.rc.point{3, 4}))) })
	fmt.Println(r.P.x, r.P.y)
	p.SetZero()
	fmt.Println(r.P.x, r.P.y)
	vec := rv.FieldByName("Vec")
	try("Vec.Set", func() {
		x := std.vector[int]{}
		x.push_back(7)
		x.push_back(8)
		vec.Set(reflect.ValueOf(any(x)))
	})
	fmt.Println(r.Vec.size())
	try("Vec.SetZero", func() { vec.SetZero() })
	fmt.Println(r.Vec.size())
	var h Holder
	pinnedType := reflect.TypeOf(&h).Elem().Field(0).Type
	fmt.Println(pinnedType.Kind(), pinnedType.NumMethod())
	try("pinned.New", func() {
		x := reflect.New(pinnedType)
		fmt.Println(x.Elem().Kind())
		x.Elem().Set(reflect.New(pinnedType).Elem())
	})
	try("pinned.Zero", func() { reflect.Zero(pinnedType) })

	// the wide integer
	w := rv.FieldByName("W")
	try("W.Set", func() { w.Set(reflect.ValueOf(any(reflectcpp.rc.wide{}))) })
	big := reflect.New(reflect.TypeOf(any(r.W.big))).Elem()
	fmt.Println(big.Kind())
	try("int128.SetInt", func() { big.SetInt(-1) })
	try("int128.SetUint", func() { big.SetUint(1) })

	// pointers
	pv := rv.FieldByName("Ptr")
	fmt.Println(pv.Kind(), pv.IsNil(), pv.Pointer() != 0)
	try("ptr.Elem", func() { pv.Elem() })
	try("ptr.SetInt", func() { pv.SetInt(99) })
	try("ptr.Type().Elem", func() { pv.Type().Elem() })
	fmt.Println(n)
	raw := rv.FieldByName("Raw")
	fmt.Println(raw.Kind(), raw.IsNil(), raw.Pointer() == 0)
	try("raw.Elem", func() { raw.Elem() })
	try("raw.SetInt", func() { raw.SetInt(1) })
	try("raw.SetString", func() { raw.SetString("x") })
	try("raw.Index", func() { raw.Index(0) })
	try("raw.Len", func() { raw.Len() })
	try("raw.Set(nil)", func() { raw.SetZero() })
	fmt.Println(raw.IsNil())
	fn := rv.FieldByName("Fn")
	fmt.Println(fn.Kind(), fn.IsNil())
	try("fn.SetInt", func() { fn.SetInt(1) })
	try("fn.Elem", func() { fn.Elem() })

	// arrays of C numbers and of std::string
	arr := rv.FieldByName("Arr")
	fmt.Println(arr.Kind(), arr.Len(), arr.Type().Elem())
	arr.Index(1).SetInt(8)
	fmt.Println(r.Arr[1], arr.Index(1).Int())
	arr.Index(2).Set(reflect.ValueOf(int32(9)))
	fmt.Println(r.Arr)
	wr := rv.FieldByName("Wrap")
	wr.Index(0).SetString("zero")
	wr.Index(1).SetString("a longer string that does not fit the small buffer at all")
	fmt.Println(r.Wrap[0].size(), r.Wrap[1].size(), wr.Index(1).String())
	nv := reflect.New(reflect.TypeOf(any(std.string(""))))
	nv.Elem().SetString("fresh")
	fmt.Println(nv.Elem().Len(), nv.Elem().Interface().(std.string) == "fresh")
	fmt.Println(reflect.DeepEqual(any(std.string("a")), any(std.string("a"))), reflect.DeepEqual(any(std.string("a")), any(std.string("b"))))
}

// Output:
// Name string true true
// View string true true
// N int32 true true
// Small uint8 true true
// F float32 true true
// D float64 true true
// B bool true true
// Sh uint32 true true
// Tn uint8 true true
// P struct true true
// Bits struct true true
// U struct true true
// W struct true true
// Big int64 true true
// Vec struct true true
// Ptr unsafe.Pointer true true
// Raw unsafe.Pointer true true
// Fn unsafe.Pointer true true
// Arr array true true
// Wrap array true true
// a rather long replacement string, longer than the small string buffer 69 69
// 0 0 true
// 98 bc true
// true true
// 7 via Set
// 0 0
// string.SetInt panic: reflect: call of reflect.Value.SetInt on string Value
// string.SetBytes panic: reflect: call of reflect.Value.SetBytes on string Value
// string.Set(go string) panic: reflect.Set: value of type string is not assignable to type std::string
// string.Convert panic: reflect.Value.Convert: value of type std::string cannot be converted to type string
// string.Elem panic: reflect: call of reflect.Value.Elem on string Value
// string.Field panic: reflect: call of reflect.Value.Field on string Value
// string.Index range panic: reflect: string index out of range
// 4 5
// view.SetString panic: reflect: SetString on a C++ value of type std::string_view is not supported
// string 0
// -7 -7 int32
// 0
// 44 44
// 1.5 1.5
// 2.25 2.25
// true true
// int.SetString panic: reflect: call of reflect.Value.SetString on int32 Value
// int.SetFloat panic: reflect: call of reflect.Value.SetFloat on int32 Value
// int.SetUint panic: reflect: call of reflect.Value.SetUint on int32 Value
// uint.SetInt panic: reflect: call of reflect.Value.SetInt on uint8 Value
// float.SetInt panic: reflect: call of reflect.Value.SetInt on float32 Value
// bool.SetInt panic: reflect: call of reflect.Value.SetInt on bool Value
// int.Set(C.int) panic: reflect.Set: value of type int32 is not assignable to type int32
// 11
// 0
// true 112
// int64 0
// uint32 2 2
// uint8 9 9
// enum.SetString panic: reflect: call of reflect.Value.SetString on uint32 Value
// P struct 0 0 true
// P ops that panic: 16 of 16
// P.SetInt panic: reflect: call of reflect.Value.SetInt on struct Value
// P.Field panic: reflect: Field index out of range
// P ptr true true
// Bits struct 0 0 true
// Bits ops that panic: 16 of 16
// Bits.SetInt panic: reflect: call of reflect.Value.SetInt on struct Value
// Bits.Field panic: reflect: Field index out of range
// Bits ptr true true
// U struct 0 0 true
// U ops that panic: 16 of 16
// U.SetInt panic: reflect: call of reflect.Value.SetInt on struct Value
// U.Field panic: reflect: Field index out of range
// U ptr true true
// Vec struct 0 0 true
// Vec ops that panic: 16 of 16
// Vec.SetInt panic: reflect: call of reflect.Value.SetInt on struct Value
// Vec.Field panic: reflect: Field index out of range
// Vec ptr true true
// W struct 0 0 true
// W ops that panic: 16 of 16
// W.SetInt panic: reflect: call of reflect.Value.SetInt on struct Value
// W.Field panic: reflect: Field index out of range
// W ptr true true
// 3 4
// 0 0
// 2
// 0
// struct 0
// struct
// pinned.New panic: reflect: C++ type rc::pinned is opaque (it can't be made or assigned)
// struct
// int128.SetInt panic: reflect: call of reflect.Value.SetInt on struct Value
// int128.SetUint panic: reflect: call of reflect.Value.SetUint on struct Value
// unsafe.Pointer false true
// ptr.Elem panic: reflect: call of reflect.Value.Elem on unsafe.Pointer Value
// ptr.SetInt panic: reflect: call of reflect.Value.SetInt on unsafe.Pointer Value
// ptr.Type().Elem panic: reflect: Elem of invalid type gp::cptr<int>
// 5
// unsafe.Pointer true true
// raw.Elem panic: reflect: call of reflect.Value.Elem on unsafe.Pointer Value
// raw.SetInt panic: reflect: call of reflect.Value.SetInt on unsafe.Pointer Value
// raw.SetString panic: reflect: call of reflect.Value.SetString on unsafe.Pointer Value
// raw.Index panic: reflect: call of reflect.Value.Index on unsafe.Pointer Value
// raw.Len panic: reflect: call of reflect.Value.Len on unsafe.Pointer Value
// true
// unsafe.Pointer true
// fn.SetInt panic: reflect: call of reflect.Value.SetInt on unsafe.Pointer Value
// fn.Elem panic: reflect: call of reflect.Value.Elem on unsafe.Pointer Value
// array 3 int32
// 8 8
// [0 8 9]
// 4 57 a longer string that does not fit the small buffer at all
// 5 true
// true false
