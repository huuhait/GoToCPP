// phase: 09

package main

import (
	"fmt"
	"reflect"
)

type Pt struct{ X, Y int }
type Named int

func main() {
	fmt.Println(reflect.DeepEqual(1, 1), reflect.DeepEqual(1, 2), reflect.DeepEqual(1, int64(1)), reflect.DeepEqual(1, Named(1)))
	fmt.Println(reflect.DeepEqual("a", "a"), reflect.DeepEqual("a", "b"), reflect.DeepEqual(true, true), reflect.DeepEqual(1.5, 1.5))
	fmt.Println(reflect.DeepEqual(nil, nil), reflect.DeepEqual(nil, 1), reflect.DeepEqual(1, nil), reflect.DeepEqual(nil, []int(nil)))
	fmt.Println(reflect.DeepEqual(Pt{1, 2}, Pt{1, 2}), reflect.DeepEqual(Pt{1, 2}, Pt{2, 1}), reflect.DeepEqual(&Pt{1, 2}, &Pt{1, 2}), reflect.DeepEqual(&Pt{1, 2}, Pt{1, 2}))
	fmt.Println(reflect.DeepEqual([3]int{1, 2, 3}, [3]int{1, 2, 3}), reflect.DeepEqual([3]int{1, 2, 3}, [3]int{1, 2, 4}), reflect.DeepEqual([2]int{}, [3]int{}))
	fmt.Println(reflect.DeepEqual(complex(1, 2), complex(1, 2)), reflect.DeepEqual(uint8(1), uint8(1)), reflect.DeepEqual(uint8(1), byte(1)))
	var e1, e2 error
	fmt.Println(reflect.DeepEqual(e1, e2), reflect.DeepEqual(fmt.Errorf("x"), fmt.Errorf("x")), reflect.DeepEqual(fmt.Errorf("x"), fmt.Errorf("y")))
	ch := make(chan int)
	fmt.Println(reflect.DeepEqual(ch, ch), reflect.DeepEqual(ch, make(chan int)), reflect.DeepEqual((chan int)(nil), (chan int)(nil)))
	x := 5
	fmt.Println(reflect.DeepEqual(&x, &x), reflect.DeepEqual(&x, new(int)), reflect.DeepEqual((*int)(nil), (*int)(nil)), reflect.DeepEqual((*int)(nil), new(int)))
}

// Output:
// true false false false
// true false true true
// true false false false
// true false true false
// true false false
// true true true
// true true false
// true false true
// true false true false
