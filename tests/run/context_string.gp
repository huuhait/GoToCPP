// phase: 09

// The String of the contexts: Background, TODO, WithCancel, WithValue and WithoutCancel chains.
package main

import (
	"context"
	"fmt"
)

type key string
type label struct{ n int }

func (l label) String() string { return fmt.Sprintf("label#%d", l.n) }

func main() {
	fmt.Println(context.Background())
	fmt.Println(context.TODO())
	c, cancel := context.WithCancel(context.Background())
	defer cancel()
	fmt.Println(c)
	v := context.WithValue(c, key("user"), "alice")
	fmt.Println(v)
	v = context.WithValue(v, key("n"), 7)
	fmt.Println(v)
	v = context.WithValue(v, label{1}, label{2})
	fmt.Println(v)
	w := context.WithoutCancel(v)
	fmt.Println(w)
	cc, cancelCause := context.WithCancelCause(context.Background())
	defer cancelCause(nil)
	fmt.Println(cc)
	fmt.Printf("%v|%s\n", context.Background(), context.TODO())
}

// Output:
// context.Background
// context.TODO
// context.Background.WithCancel
// context.Background.WithCancel.WithValue(main.key, alice)
// context.Background.WithCancel.WithValue(main.key, alice).WithValue(main.key, int)
// context.Background.WithCancel.WithValue(main.key, alice).WithValue(main.key, int).WithValue(label#1, label#2)
// context.Background.WithCancel.WithValue(main.key, alice).WithValue(main.key, int).WithValue(label#1, label#2).WithoutCancel
// context.Background.WithCancel
// context.Background|context.TODO
