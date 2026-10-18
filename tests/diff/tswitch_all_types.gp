// phase: 08

// Type switches and %T tell int, int64, rune, byte, named types, aliases, slices, arrays, funcs, pointers and anonymous types apart.
package main

import "fmt"

type MyInt int
type Alias = int
type Celsius float64
type Strs []string
type Fn func(int) int

type Shape interface{ Area() float64 }
type Sq struct{ s float64 }

func (s Sq) Area() float64 { return s.s * s.s }

type Namer interface{ Name() string }
type Both struct{}

func (Both) Area() float64 { return 1 }
func (Both) Name() string   { return "both" }

func describe(v any) string {
	switch x := v.(type) {
	case nil:
		return "nil"
	case int:
		return fmt.Sprintf("int %d", x)
	case int64:
		return fmt.Sprintf("int64 %d", x)
	case int32:
		return fmt.Sprintf("int32/rune %d", x)
	case uint8:
		return fmt.Sprintf("uint8/byte %d", x)
	case MyInt:
		return fmt.Sprintf("MyInt %d", x)
	case Celsius:
		return fmt.Sprintf("Celsius %.1f", float64(x))
	case float64, float32:
		return fmt.Sprintf("float %v (%T)", x, x)
	case string:
		return "string " + x
	case []int:
		return fmt.Sprintf("[]int len %d", len(x))
	case []int64:
		return fmt.Sprintf("[]int64 len %d", len(x))
	case Strs:
		return fmt.Sprintf("Strs %v", []string(x))
	case [3]int:
		return "array3"
	case [4]int:
		return "array4"
	case map[string]int:
		return fmt.Sprintf("map %d", len(x))
	case func(int) int:
		return fmt.Sprintf("func %d", x(1))
	case Fn:
		return fmt.Sprintf("Fn %d", x(2))
	case *int:
		return fmt.Sprintf("*int %d", *x)
	case **int:
		return "**int"
	case error:
		return "error " + x.Error()
	case Shape:
		if n, ok := x.(Namer); ok {
			return "shape+namer " + n.Name()
		}
		return fmt.Sprintf("shape %.1f", x.Area())
	case struct{ A int }:
		return fmt.Sprintf("anon struct %d", x.A)
	case chan int:
		return "chan int"
	case interface{ Foo() }:
		return "foo"
	case bool:
		return fmt.Sprint("bool ", x)
	case complex128:
		return fmt.Sprint("complex ", x)
	default:
		return fmt.Sprintf("other %T", x)
	}
}

func multi(v any) string {
	switch x := v.(type) {
	case int, int64:
		return fmt.Sprintf("integer %v %T", x, x)
	case string, []byte:
		return fmt.Sprintf("texty %v", x)
	case nil:
		return "nil"
	}
	return "none"
}

func main() {
	n := 5
	pn := &n
	var alias Alias = 3
	vals := []any{nil, 1, int64(2), rune('a'), byte(7), MyInt(8), Celsius(36.6), 1.5, float32(2.5), "s",
		[]int{1, 2}, []int64{1}, Strs{"a"}, [3]int{}, [4]int{}, map[string]int{"a": 1},
		func(i int) int { return i + 1 }, Fn(func(i int) int { return i * 10 }), pn, &pn,
		fmt.Errorf("e"), Sq{2}, Both{}, struct{ A int }{4}, make(chan int), true, 2i, alias, uint(1), int8(1), []any{1},
		struct{ B int }{1}, &struct{ A int }{1}, [0]int{}, uintptr(1)}
	for _, v := range vals {
		fmt.Println(describe(v))
	}
	for _, v := range []any{1, int64(1), "x", []byte("b"), nil, 1.5} {
		fmt.Println(multi(v))
	}
	var s Shape = Sq{3}
	switch x := s.(type) {
	case Namer:
		fmt.Println("namer", x.Name())
	case Sq:
		fmt.Println("sq", x.s)
	}
	var e error
	switch e.(type) {
	case nil:
		fmt.Println("nil error")
	default:
		fmt.Println("non-nil")
	}
	switch v := any(MyInt(3)).(type) {
	case int:
		fmt.Println("int", v)
	default:
		fmt.Printf("%T\n", v)
	}
}
