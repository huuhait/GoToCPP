// phase: 09

// Context values: lookup walks to the parent, the nearest key wins, keys of different types do not
// collide, and values survive WithCancel, WithTimeout and WithoutCancel.
package main

import (
	"context"
	"fmt"
	"time"
)

type keyA string
type keyB string
type keyC int

func main() {
	ctx := context.Background()
	fmt.Println(ctx.Value(keyA("x")))

	ctx = context.WithValue(ctx, keyA("x"), "first")
	ctx = context.WithValue(ctx, keyB("x"), "other type")
	ctx = context.WithValue(ctx, keyC(1), 42)
	c1, cancel := context.WithCancel(ctx)
	defer cancel()
	c2, cancel2 := context.WithTimeout(c1, time.Hour)
	defer cancel2()
	c3 := context.WithValue(c2, keyA("x"), "shadowed")
	c4 := context.WithoutCancel(c3)

	for _, c := range []context.Context{ctx, c1, c2, c3, c4} {
		fmt.Println(c.Value(keyA("x")), c.Value(keyB("x")), c.Value(keyC(1)), c.Value(keyC(2)), c.Value("x"))
	}
	cancel()
	<-c2.Done()
	fmt.Println(c3.Err(), c4.Err(), c4.Done() == nil)
	fmt.Println(c4.Value(keyC(1)))

	var nilValue = context.WithValue(context.TODO(), keyA("n"), nil)
	fmt.Println(nilValue.Value(keyA("n")) == nil)

	func() {
		defer func() { fmt.Println(recover()) }()
		context.WithValue(ctx, nil, 1)
	}()
	func() {
		defer func() { fmt.Println(recover()) }()
		context.WithValue(ctx, []int{1}, 1)
	}()
}

// Output:
// <nil>
// first other type 42 <nil> <nil>
// first other type 42 <nil> <nil>
// first other type 42 <nil> <nil>
// shadowed other type 42 <nil> <nil>
// shadowed other type 42 <nil> <nil>
// context canceled <nil> true
// 42
// true
// nil key
// key is not comparable
