package lower

import (
	"strings"
	"testing"

	"github.com/huuhait/gocpp/internal/cppir"
	"github.com/huuhait/gocpp/internal/interop"
)

func TestConcreteInterfaceForeignFallback(t *testing.T) {
	for _, test := range []struct {
		name string
		code string
	}{
		{"opaque argument exposes interface", `
type Value struct{}
func (Value) Read(x int) int { return x }
type Reader interface { Read(int) int }
func main() { var r Reader = Value{}; std.mutate(r); r.Read(2) }
`},
		{"foreign receiver field", `
type Foreign struct { v std.vector[int] }
func (f Foreign) Read(x int) int { return x }
type Reader interface { Read(int) int }
func main() { var r Reader = Foreign{}; r.Read(2) }
`},
		{"foreign argument field", `
type Foreign struct { v std.vector[int] }
type Value struct{}
func (Value) Read(f Foreign) int { return 1 }
type Reader interface { Read(Foreign) int }
func main() { var r Reader = Value{}; r.Read(Foreign{}) }
`},
	} {
		t.Run(test.name, func(t *testing.T) {
			src := "package main\nimport \"<vector>\"\n" + test.code
			fset, pkg, _ := checkInterop(t, src)
			kinds := map[string]interop.Kind{
				"std::vector": interop.Template,
				"std::mutate": interop.Value,
			}
			file, err := Package(fset, pkg, fakeInterop(pkg, kinds))
			if err != nil {
				t.Fatal(err)
			}
			out := string(cppir.Print(file, ""))
			if strings.Contains(out, "gp::detail::icall_ticket(") {
				t.Fatalf("unexpected direct interface call with foreign parts:\n%s", out)
			}
		})
	}
}

const devirtTypes = `package main

func main() {}

type Value struct { n int }
func (v Value) Read(x int) int { return v.n + x }
func (v *Value) Add(x int) int {
	if v == nil { return -1 }
	v.n += x
	return v.n
}
func (v *Value) Keep(x int) *Value { return v }
type Other struct{}
func (Other) Read(x int) int { return -x }
type Reader interface { Read(int) int }
type Adder interface { Add(int) int }
type Keeper interface { Keep(int) *Value }
type Outer struct { Value }
func arg() int { return 2 }
func makeValue() *Value { return &Value{3} }
`

func TestConcreteInterfaceCall(t *testing.T) {
	for _, test := range []struct {
		name string
		code string
		want string
	}{
		{"direct pointer conversion", `
func use() int { return Adder(makeValue()).Add(arg()) }
`, "Value::Add(gp::unbox_borrow<Value>(gp_self)"},
		{"immutable pointer binding", `
func use() int { var v Adder = &Value{3}; return v.Add(2) }
`, "Value::Add(gp::unbox_borrow<Value>(gp_self)"},
		{"immutable value binding", `
func use() int { var v Reader = Value{3}; return v.Read(2) }
`, "gp::unbox<Value>(gp_self).Read("},
		{"explicit conversion binding", `
func use() int { v := Reader(Value{3}); return v.Read(2) }
`, "gp::unbox<Value>(gp_self).Read("},
		{"typed nil pointer", `
func use() int { return Adder((*Value)(nil)).Add(2) }
`, "Value::Add(gp::unbox_borrow<Value>(gp_self)"},
		{"value method nil wrapper", `
func use() int { return Reader((*Value)(nil)).Read(2) }
`, `value method main.Value.Read called using nil *Value pointer`},
		{"owning receiver", `
func use() *Value { var v Keeper = &Value{}; return v.Keep(2) }
`, "Value::Keep(gp::unbox<gp::ptr<Value>>(gp_self)"},
		{"distinct method slot", `
type Both interface { Add(int) int; Read(int) int }
func use() int { var v Both = &Value{}; return v.Read(2) }
`, "gp::detail::icall_ticket(*gp_self.gp_tab(), 1)"},
	} {
		t.Run(test.name, func(t *testing.T) {
			out, err := lowerInst(t, instProgram(t, devirtTypes+test.code))
			if err != nil {
				t.Fatal(err)
			}
			for _, want := range []string{"gp::to_iface<", "[]<int = 0>", "gp::detail::icall_ticket(", test.want} {
				if !strings.Contains(out, want) {
					t.Errorf("missing %q in:\n%s", want, out)
				}
			}
		})
	}
}

func TestDynamicInterfaceCallFallback(t *testing.T) {
	for _, test := range []struct {
		name string
		code string
	}{
		{"reassignment", `
func use() int { var v Reader = Value{3}; v = Other{}; return v.Read(2) }
`},
		{"redeclaration assignment", `
func use() int { v := Reader(Value{3}); v, n := Reader(Other{}), 2; return v.Read(n) }
`},
		{"address alias", `
func change(p *Reader) { *p = Other{} }
func use() int { var v Reader = Value{3}; change(&v); return v.Read(2) }
`},
		{"closure mutation", `
func use() int {
	var v Reader = Value{3}
	func() { v = Other{} }()
	return v.Read(2)
}
`},
		{"nil interface", `
func use() int { var v Reader; return v.Read(2) }
`},
		{"parameter", `
func use(v Reader) int { return v.Read(2) }
`},
		{"unknown callee argument", `
func use(f func(Reader)) int { var v Reader = Value{3}; f(v); return v.Read(2) }
`},
		{"other interface use", `
func use() int { var v Reader = Value{3}; if v == nil { return 0 }; return v.Read(2) }
`},
		{"interface conversion", `
type Reader2 interface { Read(int) int }
func use(v Reader2) int { return Reader(v).Read(2) }
`},
		{"interface copy", `
func use() int { var v Reader = Value{3}; w := v; return w.Read(2) }
`},
		{"package variable", `
var v Reader = Value{3}
func use() int { return v.Read(2) }
`},
		{"field", `
func use() int { v := struct{ r Reader }{Value{3}}; return v.r.Read(2) }
`},
		{"promoted method", `
func use() int { var v Reader = Outer{Value{3}}; return v.Read(2) }
`},
		{"promoted interface receiver", `
type Wrapper struct { Reader }
func use() int { v := Wrapper{Value{3}}; return v.Read(2) }
`},
		{"generic concrete type", `
type Generic[T any] struct { n int }
func (v Generic[T]) Read(x int) int { return v.n + x }
func use() int { var v Reader = Generic[int]{3}; return v.Read(2) }
`},
		{"generic function", `
func use[T interface{ Read(int) int }](v T) int { return Reader(v).Read(2) }
func init() { use(Value{3}) }
`},
		{"generic receiver body", `
type Generic[T any] struct{}
func (g Generic[T]) Read(x int) int { var v Reader = Value{3}; return v.Read(x) }
func init() { Generic[int]{}.Read(2) }
`},
		{"deferred call", `
func use() { var v Reader = Value{3}; defer v.Read(2) }
`},
		{"spawned call", `
func use() { var v Reader = Value{3}; go v.Read(2) }
`},
		{"method value", `
func use() int { var v Reader = Value{3}; f := v.Read; return f(2) }
`},
		{"tuple arguments", `
type PairReader interface { Pair(int, int) int }
func (v Value) Pair(a, b int) int { return v.n + a + b }
func pair() (int, int) { return 1, 2 }
func use() int { var v PairReader = Value{3}; return v.Pair(pair()) }
`},
	} {
		t.Run(test.name, func(t *testing.T) {
			out, err := lowerInst(t, instProgram(t, devirtTypes+test.code))
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(out, "gp::detail::icall_ticket(") {
				t.Fatalf("unexpected direct interface call:\n%s", out)
			}
		})
	}
}
