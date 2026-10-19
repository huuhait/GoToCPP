// phase: 09

package main

import (
	"fmt"
	"reflect"
)

type Node struct {
	Val  int
	Next *Node
}

func main() {
	var head *Node
	hv := reflect.ValueOf(&head).Elem()
	hv.Set(reflect.New(hv.Type().Elem()))
	hv.Elem().Field(0).SetInt(1)
	next := reflect.New(reflect.TypeOf(Node{}))
	next.Elem().Field(0).SetInt(2)
	hv.Elem().Field(1).Set(next)
	fmt.Println(head.Val, head.Next.Val, head.Next.Next == nil)
	x := 5
	var p *int
	reflect.ValueOf(&p).Elem().Set(reflect.ValueOf(&x))
	*p = 6
	fmt.Println(x)
	reflect.ValueOf(&p).Elem().Elem().SetInt(7)
	fmt.Println(x, *p)
	var pp **int = &p
	reflect.ValueOf(pp).Elem().Elem().SetInt(8)
	fmt.Println(x)
	reflect.ValueOf(&p).Elem().Set(reflect.Zero(reflect.TypeOf(p)))
	fmt.Println(p == nil)
}

// Output:
// 1 2 true
// 6
// 7 7
// 8
// true
