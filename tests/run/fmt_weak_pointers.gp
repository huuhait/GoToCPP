// phase: 05

// weak.Pointer prints as Go's struct { _ [0]*T; u unsafe.Pointer }: %v {[] <nil>}, %+v with the
// field names, %#v with the types of the fields; %T names the instance; %d of the zero value.
package main

import (
	"fmt"
	"weak"
)

type T struct{ N int }

func main() {
	var z weak.Pointer[T]
	var zi weak.Pointer[int]
	fmt.Printf("%v %+v\n", z, z)
	fmt.Printf("%#v\n%#v\n", z, zi)
	fmt.Printf("%T %T %d\n", z, []weak.Pointer[T]{}, z)
	fmt.Println(z, []weak.Pointer[int]{zi}, struct{ W weak.Pointer[T] }{})
	fmt.Printf("%#v\n", struct{ W weak.Pointer[T] }{})
}

// Output:
// {[] <nil>} {_:[] u:<nil>}
// weak.Pointer[main.T]{_:[0]*main.T{}, u:(unsafe.Pointer)(nil)}
// weak.Pointer[int]{_:[0]*int{}, u:(unsafe.Pointer)(nil)}
// weak.Pointer[main.T] []weak.Pointer[main.T] {[] 0}
// {[] <nil>} [{[] <nil>}] {{[] <nil>}}
// struct { W weak.Pointer[main.T] }{W:weak.Pointer[main.T]{_:[0]*main.T{}, u:(unsafe.Pointer)(nil)}}
