// phase: 07

// A type switch on any(v) inside generic code sees the Go type of the type argument.
package main

import "fmt"

type MyInt int
type Celsius float64

type Shape interface{ Area() int }
type Sq struct{ s int }

func (s Sq) Area() int { return s.s * s.s }

func Describe[T any](v T) string {
	switch x := any(v).(type) {
	case nil:
		return "nil"
	case int:
		return fmt.Sprint("int ", x+1)
	case int64:
		return fmt.Sprint("int64 ", x+1)
	case MyInt:
		return fmt.Sprint("MyInt ", x+1)
	case float64, float32:
		return fmt.Sprint("float ", x)
	case string:
		return "string " + x
	case []int:
		return fmt.Sprint("slice of ", len(x))
	case map[string]int:
		return fmt.Sprint("map of ", len(x))
	case Shape:
		return fmt.Sprint("shape ", x.Area())
	case error:
		return "error " + x.Error()
	case func() int:
		return fmt.Sprint("func ", x())
	case *int:
		return fmt.Sprint("ptr ", *x)
	case struct{ A int }:
		return fmt.Sprint("anon ", x.A)
	}
	return fmt.Sprintf("other %T", v)
}

func main() {
	n := 5
	fmt.Println(Describe(1), Describe(int64(1)), Describe(MyInt(1)), Describe(Celsius(1)))
	fmt.Println(Describe(2.5), Describe(float32(2.5)), Describe("s"), Describe([]int{1, 2}))
	fmt.Println(Describe(map[string]int{"a": 1}), Describe(Sq{3}), Describe(fmt.Errorf("e")))
	fmt.Println(Describe(func() int { return 7 }), Describe(&n), Describe(struct{ A int }{4}))
	fmt.Println(Describe[any](nil), Describe[error](nil), Describe[Shape](Sq{2}), Describe[any](8))
	fmt.Println(Describe([2]int{}), Describe(true), Describe(int8(1)), Describe(&Sq{1}))
}

// Output:
// int 2 int64 2 MyInt 2 other main.Celsius
// float 2.5 float 2.5 string s slice of 2
// map of 1 shape 9 error e
// func 7 ptr 5 anon 4
// nil nil shape 4 int 9
// other [2]int other bool other int8 shape 1
