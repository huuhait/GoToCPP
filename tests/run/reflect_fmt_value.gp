// phase: 09

package main

import (
	"errors"
	"fmt"
	"reflect"
)

type Pt struct{ X, Y int }
type Celsius float64

func (c Celsius) String() string { return fmt.Sprintf("%.1fC", float64(c)) }

type Err struct{}

func (Err) Error() string { return "custom error" }

type hidden struct {
	c Celsius
	P Pt
}

func main() {
	vals := []any{42, "str", 2.5, true, []int{1, 2}, map[string]int{"a": 1}, Pt{1, 2}, &Pt{3, 4}, Celsius(20), Err{}, errors.New("e"), [2]string{"a", "b"}}
	for _, x := range vals {
		v := reflect.ValueOf(x)
		fmt.Println(v)
		fmt.Printf("%v|%+v|%#v|%s|%d|%T\n", v, v, v, v, v, v)
	}
	fmt.Println(reflect.Value{})
	fmt.Printf("%v %d %s\n", reflect.Value{}, reflect.Value{}, reflect.Value{})
	h := reflect.ValueOf(hidden{20, Pt{1, 2}})
	fmt.Println(h.Field(0), h.Field(1), h)
	fmt.Printf("%v %v\n", h.Field(0), h.Field(1))
	vs := []reflect.Value{reflect.ValueOf(1), reflect.ValueOf("two"), reflect.ValueOf(Celsius(3))}
	fmt.Println(vs)
	fmt.Printf("%v %s\n", vs, vs[0].String())
	m := map[string]reflect.Value{"k": reflect.ValueOf(5)}
	fmt.Println(m)
	var x any = 7
	fmt.Println(reflect.ValueOf(&x).Elem(), reflect.ValueOf(&x).Elem().Elem())
	fmt.Println(reflect.ValueOf(reflect.ValueOf(3)))
}

// Output:
// 42
// 42|42|42|%!s(int=42)|42|reflect.Value
// str
// str|str|"str"|str|%!d(string=str)|reflect.Value
// 2.5
// 2.5|2.5|2.5|%!s(float64=2.5)|%!d(float64=2.5)|reflect.Value
// true
// true|true|true|%!s(bool=true)|%!d(bool=true)|reflect.Value
// [1 2]
// [1 2]|[1 2]|[]int{1, 2}|[%!s(int=1) %!s(int=2)]|[1 2]|reflect.Value
// map[a:1]
// map[a:1]|map[a:1]|map[string]int{"a":1}|map[a:%!s(int=1)]|map[%!d(string=a):1]|reflect.Value
// {1 2}
// {1 2}|{X:1 Y:2}|main.Pt{X:1, Y:2}|{%!s(int=1) %!s(int=2)}|{1 2}|reflect.Value
// &{3 4}
// &{3 4}|&{X:3 Y:4}|&main.Pt{X:3, Y:4}|&{%!s(int=3) %!s(int=4)}|&{3 4}|reflect.Value
// 20.0C
// 20.0C|20.0C|20|20.0C|%!d(main.Celsius=20)|reflect.Value
// custom error
// custom error|custom error|main.Err{}|custom error|{}|reflect.Value
// e
// e|e|&errors.errorString{s:"e"}|e|&{%!d(string=e)}|reflect.Value
// [a b]
// [a b]|[a b]|[2]string{"a", "b"}|[a b]|[%!d(string=a) %!d(string=b)]|reflect.Value
// <invalid reflect.Value>
// <invalid reflect.Value> <invalid reflect.Value> <invalid reflect.Value>
// 20 {1 2} {20 {1 2}}
// 20 {1 2}
// [<int Value> two <main.Celsius Value>]
// [<int Value> two <main.Celsius Value>] <int Value>
// map[k:<int Value>]
// 7 7
// <int Value>
