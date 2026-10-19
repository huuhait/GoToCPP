// phase: 09

package main

import (
	"fmt"
	"reflect"
)

type T struct{ A int }
type G interface{ M() int }

func main() {
	vals := []any{
		map[string][]*T{}, [][]int{}, [2][3]string{}, *new(map[T]struct{}), []map[string]*T{},
		new(**int), []interface{}{}, []any{}, map[string]any{}, [5]*T{}, []error{}, map[G]int{}, new(G),
		new(interface{ M() int }), new(interface {
			M()
			N(int) string
		}), []func(){}, [][]*[]int{},
	}
	for _, v := range vals {
		fmt.Println(reflect.TypeOf(v))
	}
}

// Output:
// map[string][]*main.T
// [][]int
// [2][3]string
// map[main.T]struct {}
// []map[string]*main.T
// ***int
// []interface {}
// []interface {}
// map[string]interface {}
// [5]*main.T
// []error
// map[main.G]int
// *main.G
// *interface { M() int }
// *interface { M(); N(int) string }
// []func()
// [][]*[]int
