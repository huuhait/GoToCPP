// phase: 05

// Several types in one case keep v as the interface; nil and default cases; default in the
// middle is tried last (step 5.5).
package main

import "fmt"

type Stringer interface{ String() string }
type N int

func (n N) String() string { return fmt.Sprint("N", int(n)) }

func classify(x any) string {
	switch v := x.(type) {
	case int, int8, int16:
		_, isInt := v.(int)
		return fmt.Sprint("small int ", isInt)
	default:
		return "default"
	case nil:
		return "nil"
	case string, []byte:
		return fmt.Sprintf("text %v", v != nil)
	case Stringer:
		return "stringer " + v.String()
	case float32, float64:
		return "float"
	}
}

func onlyNil(x any) bool {
	switch x.(type) {
	case nil:
		return true
	}
	return false
}

func main() {
	for _, x := range []any{1, int8(2), int16(3), nil, "s", []byte("b"), N(4), float32(1), 2.5, true, int32(1)} {
		fmt.Println(classify(x))
	}
	var e error
	var a any = e
	fmt.Println(onlyNil(nil), onlyNil(a), onlyNil(0), onlyNil(e))
}

// Output:
// small int true
// small int false
// small int false
// nil
// text true
// text true
// stringer N4
// float
// float
// default
// default
// true true false true
