package server

import (
	"strings"
	"testing"
)

// The probes of the dynamic family's reach (the reviewer's c-series): a type T
// with a Close method, a function that takes a value out as a Closer
// (x.(Closer)), and one way or another that a T reaches it. The rename of
// T.Close must change the three places (the interface's method, the call
// through it, the method) or, for a type that nothing reaches (c29), only its
// own; each form of conversion that scanTies does not see is a silent change
// of what the program prints (the assertion fails). The counts are those of the
// wide family of dev, which preserves behaviour.

const probeHead = `package main

import (
	"fmt"
%s)

type Closer interface{ Close() error }

type T struct{}

func (*T) Close() error { fmt.Println("closed"); return nil }

func closeIt(x any) {
	if c, ok := x.(Closer); ok {
		c.Close()
	} else {
		fmt.Println("not closer")
	}
}
%s
func main() {
%s
}
`

var reachProbes = []struct {
	name, body, decls, imports, mark string
	edits                            int
}{
	{"c01_direct", `closeIt(&T{})`, ``, ``, "func (*T) Cl|ose", 3},
	{"c02_reflect", `v := reflect.New(reflect.TypeOf(T{}))
	closeIt(v.Interface())`, ``, `	"reflect"
`, "func (*T) Cl|ose", 3},
	{"c02b_reflect_ptrtype", `v := reflect.New(reflect.TypeOf((*T)(nil)).Elem())
	closeIt(v.Interface())`, ``, `	"reflect"
`, "func (*T) Cl|ose", 3},
	{"c03_generic_body", `F(&T{})`, `
func F[X any](v X) { closeIt(v) }
`, ``, "func (*T) Cl|ose", 3},
	{"c03b_generic_inferred_value", `G(T{})`, `
func G[X any](v X) { fmt.Println("g"); _ = any(v); closeIt(v) }
`, ``, "func (*T) Cl|ose", 3},
	{"c05_embed_named", `closeIt(W{&T{}})`, `
type W struct{ *T }
`, ``, "func (*T) Cl|ose", 3},
	{"c06_embed_anon", `closeIt(struct{ *T }{&T{}})`, ``, ``, "func (*T) Cl|ose", 3},
	{"c06b_embed_local", `type L struct{ *T }
	closeIt(L{&T{}})`, ``, ``, "func (*T) Cl|ose", 3},
	{"c06c_embed_generic_wrapper", `closeIt(Box[int]{T: &T{}})`, `
type Box[X any] struct {
	*T
	v X
}
`, ``, "func (*T) Cl|ose", 3},
	{"c06d_embed_alias_anon", `closeIt(A{&T{}})`, `
type A = struct{ *T }
`, ``, "func (*T) Cl|ose", 3},
	{"c06e_embed_two_levels", `closeIt(W2{W{&T{}}})`, `
type W struct{ *T }

type W2 struct{ W }
`, ``, "func (*T) Cl|ose", 3},
	{"c30_return_multi_forward", `a, _ := g()
	closeIt(a)`, `
func f() (*T, error) { return &T{}, nil }

func g() (any, error) { return f() }
`, ``, "func (*T) Cl|ose", 3},
	{"c31_assign_multi", `var a any
	var err error
	a, err = f()
	_ = err
	closeIt(a)`, `
func f() (*T, error) { return &T{}, nil }
`, ``, "func (*T) Cl|ose", 3},
	{"c32_call_forward", `h(f())`, `
func f() (*T, error) { return &T{}, nil }

func h(a any, err error) { closeIt(a) }
`, ``, "func (*T) Cl|ose", 3},
	{"c33_commaok_map", `m := map[string]*T{"k": {}}
	var a any
	var ok bool
	a, ok = m["k"]
	_ = ok
	closeIt(a)`, ``, ``, "func (*T) Cl|ose", 3},
	{"c34_range_assign", `ts := []*T{{}}
	var a any
	for _, a = range ts {
	}
	closeIt(a)`, ``, ``, "func (*T) Cl|ose", 3},
	{"c35_vardecl_multi", `var a, err any = f()
	_ = err
	closeIt(a)`, `
func f() (*T, error) { return &T{}, nil }
`, ``, "func (*T) Cl|ose", 3},
	{"c36_commaok_chan", `ch := make(chan *T, 1)
	ch <- &T{}
	var a any
	var ok bool
	a, ok = <-ch
	_ = ok
	closeIt(a)`, ``, ``, "func (*T) Cl|ose", 3},
	{"c37_struct_field_ptrtoiface", `var a any
	p := &a
	*p = &T{}
	closeIt(a)`, ``, ``, "func (*T) Cl|ose", 3},
	{"c38_closure_result", `f := func() any { return &T{} }
	closeIt(f())`, ``, ``, "func (*T) Cl|ose", 3},
	{"c39_select_recv", `ch := make(chan *T, 1)
	ch <- &T{}
	var a any
	select {
	case a = <-ch:
	}
	closeIt(a)`, ``, ``, "func (*T) Cl|ose", 3},
	{"c40_keyed_literal", `h := H{c: &T{}}
	closeIt(h.c)`, `
type H struct{ c any }
`, ``, "func (*T) Cl|ose", 3},
	{"c41_switch_case_value", `var v any
	t := &T{}
	v = nil
	switch v {
	case t:
		fmt.Println("same")
	default:
		fmt.Println("other")
	}
	closeIt(any(nil))
	func(x any) { closeIt(x) }(t)`, ``, ``, "func (*T) Cl|ose", 3},
	{"c42_map_key_index", `m := map[any]int{}
	t := &T{}
	m[t]++
	for k := range m {
		closeIt(k)
	}`, ``, ``, "func (*T) Cl|ose", 3},
	{"c43_panic_recover", `defer func() {
		closeIt(recover())
	}()
	panic(&T{})`, ``, ``, "func (*T) Cl|ose", 3},
	{"c44_defer_arg", `defer closeIt(&T{})`, ``, ``, "func (*T) Cl|ose", 3},
	{"c45_method_expr_call", `var s S
	S.Put(s, &T{})`, `
type S struct{}

func (S) Put(x any) { closeIt(x) }
`, ``, "func (*T) Cl|ose", 3},
	{"c46_array_lit", `a := [1]any{&T{}}
	closeIt(a[0])`, ``, ``, "func (*T) Cl|ose", 3},
	{"c47_cond_expr_ifaceassign", `var x Closer
	if true {
		x = &T{}
	}
	closeIt(x)`, ``, ``, "func (*T) Cl|ose", 3},
	{"c48_named_result_bare", `closeIt(res())`, `
func res() (r any) {
	r = &T{}
	return
}
`, ``, "func (*T) Cl|ose", 3},
	{"c49_two_returns_tuple_iface", `a, b := pair()
	closeIt(a)
	_ = b`, `
func one() *T { return &T{} }

func pair() (any, any) { return one(), nil }
`, ``, "func (*T) Cl|ose", 3},
	{"c07_return_iface", `closeIt(mk())`, `
func mk() Closer { return &T{} }
`, ``, "func (*T) Cl|ose", 3},
	{"c08_chan", `ch := make(chan any, 1)
	ch <- &T{}
	closeIt(<-ch)`, ``, ``, "func (*T) Cl|ose", 3},
	{"c09_map", `m := map[string]any{}
	m["a"] = &T{}
	closeIt(m["a"])`, ``, ``, "func (*T) Cl|ose", 3},
	{"c10_slicelit", `s := []Closer{&T{}}
	closeIt(s[0])`, ``, ``, "func (*T) Cl|ose", 3},
	{"c11_field", `h := H{}
	h.c = &T{}
	closeIt(h.c)`, `
type H struct{ c any }
`, ``, "func (*T) Cl|ose", 3},
	{"c12_variadic", `many(&T{})`, `
func many(xs ...any) {
	for _, x := range xs {
		closeIt(x)
	}
}
`, ``, "func (*T) Cl|ose", 3},
	{"c15_constraint", `G(&T{})`, `
func G[X Closer](x X) { closeIt(x) }
`, ``, "func (*T) Cl|ose", 3},
	{"c16_iface_embed", `var rc RC = &T{}
	closeIt(rc)`, `
type RC interface {
	Closer
	Read()
}

func (*T) Read() {}
`, ``, "func (*T) Cl|ose", 3},
	{"c18_value_recv", `closeIt(V{})`, `
type V struct{}

func (V) Close() error { fmt.Println("v closed"); return nil }
`, ``, "func (V) Cl|ose", 3},
	{"c19_valrecv_ptrconv", `closeIt(&V{})`, `
type V struct{}

func (V) Close() error { fmt.Println("v closed"); return nil }
`, ``, "func (V) Cl|ose", 3},
	{"c20_global", `closeIt(g)`, `
var g any = &T{}
`, ``, "func (*T) Cl|ose", 3},
	{"c21_go_closure", `done := make(chan bool)
	go func() { closeIt(&T{}); done <- true }()
	<-done`, ``, ``, "func (*T) Cl|ose", 3},
	{"c22_method_arg", `var s S
	s.Put(&T{})`, `
type S struct{}

func (S) Put(x any) { closeIt(x) }
`, ``, "func (*T) Cl|ose", 3},
	{"c23_append_any", `var xs []any
	xs = append(xs, &T{})
	closeIt(xs[0])`, ``, ``, "func (*T) Cl|ose", 3},
	{"c24_range_conv", `ts := []*T{{}}
	for _, t := range ts {
		var x any = t
		closeIt(x)
	}`, ``, ``, "func (*T) Cl|ose", 3},
	{"c25_typeswitch", `closeIt2(&T{})`, `
func closeIt2(x any) {
	switch c := x.(type) {
	case Closer:
		c.Close()
	default:
		fmt.Println("other")
	}
}
`, ``, "func (*T) Cl|ose", 4},
	{"c26_errors_as", `var err error = &E{}
	var t interface{ Temporary() bool }
	fmt.Println(errors.As(err, &t))`, `
type E struct{}

func (*E) Error() string   { return "e" }
func (*E) Temporary() bool { return true }
`, `	"errors"
`, "func (*E) Te|mporary", 2},
	{"c28_method_value", `f := (&T{}).Close
	f()
	closeIt(&T{})`, ``, ``, "func (*T) Cl|ose", 4},
	{"c29_never_converted", `t := &T{}
	t.Close()
	closeIt(&U{})`, `
type U struct{}

func (*U) Close() error { fmt.Println("u closed"); return nil }
`, ``, "func (*T) Cl|ose", 2},
	{"c50_reflect_field", `type S struct{ F *T }
	v := reflect.ValueOf(S{&T{}})
	closeIt(v.Field(0).Interface())`, ``, `	"reflect"
`, "func (*T) Cl|ose", 3},
	{"c51_reflect_slice_elem", `ts := []*T{{}}
	v := reflect.ValueOf(ts)
	closeIt(v.Index(0).Interface())`, ``, `	"reflect"
`, "func (*T) Cl|ose", 3},
	{"c52_reflect_map_elem", `m := map[string]*T{"a": {}}
	v := reflect.ValueOf(m)
	closeIt(v.MapIndex(reflect.ValueOf("a")).Interface())`, ``, `	"reflect"
`, "func (*T) Cl|ose", 3},
	{"c53_reflect_new_from_type", `var p *T
	typ := reflect.TypeOf(p).Elem()
	closeIt(reflect.New(typ).Interface())`, ``, `	"reflect"
`, "func (*T) Cl|ose", 3},
	{"n01_promote_same_depth", `closeIt(W{&T{}, &U{}})`, `
type U struct{}

func (*U) ZqNew() error { fmt.Println("u"); return nil }

type W struct {
	*T
	*U
}
`, ``, "func (*T) Cl|ose", -1},
	{"n02_promote_shadow_deeper", `w := W{&T{}, X{&V{}}}
	fmt.Println(w.ZqNew())
	closeIt(w)`, `
type V struct{}

func (*V) ZqNew() int { return 7 }

type X struct{ *V }

type W struct {
	*T
	X
}
`, ``, "func (*T) Cl|ose", -1},
	{"n03_field_with_new_name", `w := W{&T{}, 5}
	fmt.Println(w.ZqNew)
	closeIt(w)`, `
type W struct {
	*T
	ZqNew int
}
`, ``, "func (*T) Cl|ose", -1},
	{"n04_method_value_embedded_iface", `w := W{&T{}}
	f := w.Close
	f()
	closeIt(w)`, `
type W struct{ Closer }
`, ``, "func (*T) Cl|ose", 4},
	{"n05_generic_two_level", `closeIt(G2[int]{Box[int]{T: &T{}}})`, `
type Box[X any] struct {
	*T
	v X
}

type G2[X any] struct{ Box[X] }
`, ``, "func (*T) Cl|ose", 3},
	{"n06_sprintf_nested", `type In struct{ P *T }
	type Out struct{ I In }
	o := Out{In{&T{}}}
	_ = fmt.Sprintf("%v", o)
	closeIt(reflect.ValueOf(o).Field(0).Field(0).Interface())`, ``, `	"reflect"
`, "func (*T) Cl|ose", 3},
	{"n07_json_like_walk", `type Cfg struct {
		Name string
		C    *T
	}
	walk(reflect.ValueOf(any(Cfg{"x", &T{}})))`, `
func walk(v reflect.Value) {
	switch v.Kind() {
	case reflect.Struct:
		for i := range v.NumField() {
			if v.Field(i).CanInterface() {
				walk(v.Field(i))
			}
		}
	default:
		if v.CanInterface() {
			closeIt(v.Interface())
		}
	}
}
`, `	"reflect"
`, "func (*T) Cl|ose", 3},
	{"n08_swapper", `ts := []*T{{}, {}}
	sw := reflect.Swapper(ts)
	sw(0, 1)
	closeIt(reflect.ValueOf(ts).Index(1).Interface())`, ``, `	"reflect"
`, "func (*T) Cl|ose", 3},
	{"n09_defer_any", `t := &T{}
	defer closeIt(any(t))`, ``, ``, "func (*T) Cl|ose", 3},
	{"n10_select_send", `ch := make(chan any, 1)
	select {
	case ch <- &T{}:
	}
	closeIt(<-ch)`, ``, ``, "func (*T) Cl|ose", 3},
	{"n11_anon_func_returns_any", `closeIt(func() any { return &T{} }())`, ``, ``, "func (*T) Cl|ose", 3},
	{"n12_method_expr_value", `var fn any = (*T).Close
	_ = fn
	closeIt(&T{})`, ``, ``, "func (*T) Cl|ose", 4},
	{"n13_chan_struct_reflect", `type S struct{ P *T }
	ch := make(chan any, 1)
	ch <- S{&T{}}
	v := <-ch
	closeIt(reflect.ValueOf(v).Field(0).Interface())`, ``, `	"reflect"
`, "func (*T) Cl|ose", 3},
	{"n14_array_of_array", `var a [2][2]*T
	a[0][1] = &T{}
	closeIt(reflect.ValueOf(a).Index(0).Index(1).Interface())`, ``, `	"reflect"
`, "func (*T) Cl|ose", 3},
	{"n15_func_results", `f := func() *T { return &T{} }
	v := reflect.ValueOf(f).Call(nil)[0].Interface()
	closeIt(v)`, ``, `	"reflect"
`, "func (*T) Cl|ose", 3},
	{"n16_ptr_to_ptr", `pp := new(*T)
	*pp = &T{}
	closeIt(reflect.ValueOf(pp).Elem().Interface())`, ``, `	"reflect"
`, "func (*T) Cl|ose", 3},
	{"n17_map_key_struct", `type K struct{ P *T }
	m := map[K]int{{&T{}}: 1}
	closeIt(reflect.ValueOf(m).MapKeys()[0].Field(0).Interface())`, ``, `	"reflect"
`, "func (*T) Cl|ose", 3},
	{"n18_key_range_chan", `ch := make(chan *T, 1)
	ch <- &T{}
	close(ch)
	var a any
	for a = range ch {
	}
	closeIt(a)`, ``, ``, "func (*T) Cl|ose", 3},
	{"n19_key_range_map", `m := map[*T]int{{}: 1}
	var a any
	for a = range m {
	}
	closeIt(a)`, ``, ``, "func (*T) Cl|ose", 3},
	{"n20_switch_tag", `t := &T{}
	var v any = t
	switch t {
	case v:
		fmt.Println("same")
	}
	func(x any) { closeIt(x) }(t)`, ``, ``, "func (*T) Cl|ose", 3},
	{"n21_ptr_ptr_conv", `t := &T{}
	pt := &t
	closeIt(*pt)`, ``, ``, "func (*T) Cl|ose", 3},
	{"n22_embed_iface_in_iface_conflict", `var j J = &T{}
	closeIt(j)`, `
type Other interface{ ZqNew() error }

type J interface {
	Closer
	Other
}

func (*T) ZqNew() error { return nil }
`, ``, "func (*T) Cl|ose", -1},
	{"n23_chan_reflect_recv", `ch := make(chan *T, 1)
	ch <- &T{}
	var c any = ch
	x, _ := reflect.ValueOf(c).Recv()
	closeIt(x.Interface())`, ``, `	"reflect"
`, "func (*T) Cl|ose", 3},
	{"n24_named_pointer_type", `p := PT(&T{})
	closeIt(reflect.ValueOf(p).Elem().Addr().Interface())`, `
type PT *T
`, `	"reflect"
`, "func (*T) Cl|ose", 3},
	{"n25_map_key_assign_index", `m := map[any]int{}
	t := &T{}
	m[t] = 1
	for k := range m {
		closeIt(k)
	}`, ``, ``, "func (*T) Cl|ose", 3},
}

func TestDynamicReachProbes(t *testing.T) {
	for _, p := range reachProbes {
		t.Run(p.name, func(t *testing.T) {
			src := strings.Replace(strings.Replace(strings.Replace(probeHead, "%s", p.imports, 1), "%s", p.decls, 1), "%s", p.body, 1)
			files := map[string]string{"gocpp.mod": "module example.com/probe\n", "main.gp": src}
			c := startRefs(t, files, plainInit)
			got, rerr := c.rename("main.gp", p.mark, "ZqNew")
			if p.edits < 0 { // refused: the program would change with no error
				if rerr == nil {
					t.Fatalf("renamed, want a refusal:\n%s", got["main.gp"])
				}
				return
			}
			if rerr != nil {
				t.Fatal(rerr)
			}
			if n := strings.Count(got["main.gp"], "ZqNew"); n != p.edits {
				t.Errorf("%d places renamed, want %d:\n%s", n, p.edits, got["main.gp"])
			}
		})
	}
}

// TestDynamicReachProbesMulti: the same in several packages: the assertion in
// a package that does not import the type, in generic code, a conversion in
// the type's package, in its test file, an embedding in another package.
func TestDynamicReachProbesMulti(t *testing.T) {
	for _, p := range []struct {
		name       string
		files      map[string]string
		file, mark string
	}{
		{"m01_assert_in_lib", map[string]string{
			"gocpp.mod":    "module example.com/probe\n",
			"lib/lib.gp":   "package lib\n\nimport \"fmt\"\n\ntype Closer interface{ Close() error }\n\nfunc CloseIt(x any) {\n\tif c, ok := x.(Closer); ok {\n\t\tc.Close()\n\t} else {\n\t\tfmt.Println(\"not closer\")\n\t}\n}\n",
			"impl/impl.gp": "package impl\n\nimport \"fmt\"\n\ntype T struct{}\n\nfunc (*T) Close() error { fmt.Println(\"closed\"); return nil }\n",
			"main.gp":      "package main\n\nimport (\n\t\"example.com/probe/impl\"\n\t\"example.com/probe/lib\"\n)\n\nfunc main() { lib.CloseIt(&impl.T{}) }\n",
		}, "impl/impl.gp", "func (*T) Cl|ose"},
		{"m02_generic_in_lib", map[string]string{
			"gocpp.mod":    "module example.com/probe\n",
			"lib/lib.gp":   "package lib\n\nimport \"fmt\"\n\ntype Closer interface{ Close() error }\n\nfunc F[X any](v X) {\n\tvar a any = v\n\tif c, ok := a.(Closer); ok {\n\t\tc.Close()\n\t} else {\n\t\tfmt.Println(\"not closer\")\n\t}\n}\n",
			"impl/impl.gp": "package impl\n\nimport \"fmt\"\n\ntype T struct{}\n\nfunc (*T) Close() error { fmt.Println(\"closed\"); return nil }\n",
			"main.gp":      "package main\n\nimport (\n\t\"example.com/probe/impl\"\n\t\"example.com/probe/lib\"\n)\n\nfunc main() { lib.F(&impl.T{}) }\n",
		}, "impl/impl.gp", "func (*T) Cl|ose"},
		{"m03_conv_in_impl_pkg", map[string]string{
			"gocpp.mod":    "module example.com/probe\n",
			"lib/lib.gp":   "package lib\n\nimport \"fmt\"\n\ntype Closer interface{ Close() error }\n\nfunc CloseIt(x any) {\n\tif c, ok := x.(Closer); ok {\n\t\tc.Close()\n\t} else {\n\t\tfmt.Println(\"not closer\")\n\t}\n}\n",
			"impl/impl.gp": "package impl\n\nimport \"fmt\"\n\ntype T struct{}\n\nfunc (*T) Close() error { fmt.Println(\"closed\"); return nil }\n\nfunc New() any { return &T{} }\n",
			"main.gp":      "package main\n\nimport (\n\t\"example.com/probe/impl\"\n\t\"example.com/probe/lib\"\n)\n\nfunc main() { lib.CloseIt(impl.New()) }\n",
		}, "impl/impl.gp", "func (*T) Cl|ose"},
		{"m04_test_file_conv", map[string]string{
			"gocpp.mod":         "module example.com/probe\n",
			"lib/lib.gp":        "package lib\n\nimport \"fmt\"\n\ntype Shutr interface{ Shut() error }\n\nfunc ShutIt(x any) {\n\tif c, ok := x.(Shutr); ok {\n\t\tc.Shut()\n\t} else {\n\t\tfmt.Println(\"not closer\")\n\t}\n}\n",
			"impl/impl.gp":      "package impl\n\nimport \"fmt\"\n\ntype T struct{}\n\nfunc (*T) Shut() error { fmt.Println(\"closed\"); return nil }\n",
			"impl/impl_test.gp": "package impl\n\nimport (\n\t\"testing\"\n\n\t\"example.com/probe/lib\"\n)\n\nfunc TestIt(t *testing.T) { lib.ShutIt(&T{}) }\n",
			"main.gp":           "package main\n\nfunc main() {}\n",
		}, "impl/impl.gp", "func (*T) Sh|ut"},
		{"m05_embed_other_pkg", map[string]string{
			"gocpp.mod":    "module example.com/probe\n",
			"lib/lib.gp":   "package lib\n\nimport \"fmt\"\n\ntype Closer interface{ Close() error }\n\nfunc CloseIt(x any) {\n\tif c, ok := x.(Closer); ok {\n\t\tc.Close()\n\t} else {\n\t\tfmt.Println(\"not closer\")\n\t}\n}\n",
			"impl/impl.gp": "package impl\n\nimport \"fmt\"\n\ntype T struct{}\n\nfunc (*T) Close() error { fmt.Println(\"closed\"); return nil }\n",
			"main.gp":      "package main\n\nimport (\n\t\"example.com/probe/impl\"\n\t\"example.com/probe/lib\"\n)\n\ntype W struct{ *impl.T }\n\nfunc main() { lib.CloseIt(W{&impl.T{}}) }\n",
		}, "impl/impl.gp", "func (*T) Cl|ose"},
	} {
		t.Run(p.name, func(t *testing.T) {
			c := startRefs(t, p.files, plainInit)
			got, rerr := c.rename(p.file, p.mark, "ZqNew")
			if rerr != nil {
				t.Fatal(rerr)
			}
			n := 0
			for _, text := range got {
				n += strings.Count(text, "ZqNew")
			}
			if n != 3 {
				t.Errorf("%d places renamed, want 3: %v", n, got)
			}
		})
	}
}
