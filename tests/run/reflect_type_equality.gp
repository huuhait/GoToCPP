// phase: 09

package main

import (
	"fmt"
	"reflect"
)

type MyInt int
type Pt struct{ X int }
type Alias = Pt

func main() {
	var a, b int = 1, 2
	fmt.Println(reflect.TypeOf(a) == reflect.TypeOf(b))
	fmt.Println(reflect.TypeOf(a) == reflect.TypeOf(int64(1)))
	fmt.Println(reflect.TypeOf(a) == reflect.TypeOf(MyInt(1)))
	fmt.Println(reflect.TypeOf(uint8(1)) == reflect.TypeOf(byte(1)))
	fmt.Println(reflect.TypeOf(Pt{}) == reflect.TypeOf(Alias{}))
	fmt.Println(reflect.TypeOf([]int{}) == reflect.TypeOf([]int(nil)))
	fmt.Println(reflect.TypeOf(map[string]int{}) == reflect.TypeOf(map[string]int64{}))
	fmt.Println(reflect.TypeOf(&a) == reflect.TypeOf(&b))
	fmt.Println(reflect.TypeOf(struct{ X int }{}) == reflect.TypeOf(Pt{}))
	fmt.Println(reflect.TypeOf(struct{ X int }{}) == reflect.TypeOf(struct{ X int }{}))
	fmt.Println(reflect.TypeOf(make(chan int)) == reflect.TypeOf(make(<-chan int)))
	fmt.Println(reflect.TypeOf(make(chan int)) == reflect.TypeOf(make(chan int)))
	m := map[reflect.Type]string{reflect.TypeOf(1): "int", reflect.TypeOf(""): "string"}
	fmt.Println(m[reflect.TypeOf(5)], m[reflect.TypeOf("x")], len(m[reflect.TypeOf(2.5)]))
	var e1, e2 any = reflect.TypeOf(1), reflect.TypeOf(1)
	fmt.Println(e1 == e2)
	fmt.Printf("%T %v\n", reflect.TypeOf(1), reflect.TypeOf(1))
}

// Output:
// true
// false
// false
// true
// true
// true
// false
// true
// false
// true
// false
// true
// int string 0
// true
// *reflect.rtype int
