// phase: 08

// Channels of pointers, slices, maps, funcs and interfaces share what they point to.
package main

import "fmt"

type Node struct {
	val  int
	next *Node
}

type failure struct{ code int }

func (f failure) Error() string { return fmt.Sprint("failed ", f.code) }

type Speaker interface{ Speak() string }
type Dog struct{ name string }

func (d Dog) Speak() string { return d.name + " says woof" }

type Cat struct{ lives int }

func (c *Cat) Speak() string { c.lives--; return fmt.Sprint("meow (", c.lives, " lives left)") }

func main() {
	cp := make(chan *Node, 2)
	n := &Node{val: 1}
	cp <- n
	cp <- nil
	got := <-cp
	got.val = 99
	fmt.Println(n.val, got == n, <-cp == nil)

	cs := make(chan []int, 1)
	s := []int{1, 2, 3}
	cs <- s
	r := <-cs
	r[1] = 20
	fmt.Println(s, len(r))

	cm := make(chan map[string]int, 1)
	m := map[string]int{"a": 1}
	cm <- m
	rm := <-cm
	rm["b"] = 2
	fmt.Println(len(m), m["b"])

	cf := make(chan func(int) int, 3)
	for i := 1; i <= 3; i++ {
		cf <- func(x int) int { return x * i }
	}
	for range 3 {
		fmt.Print((<-cf)(10), ",")
	}
	fmt.Println()

	ci := make(chan Speaker, 3)
	ci <- Dog{"rex"}
	c := &Cat{9}
	ci <- c
	ci <- nil
	fmt.Println((<-ci).Speak())
	fmt.Println((<-ci).Speak(), c.lives)
	fmt.Println(<-ci == nil)

	ca := make(chan any, 5)
	ca <- 1
	ca <- "s"
	ca <- 2.5
	ca <- []int{1}
	ca <- nil
	for i := 0; i < 5; i++ {
		switch v := (<-ca).(type) {
		case int:
			fmt.Println("int", v)
		case string:
			fmt.Println("string", v)
		case float64:
			fmt.Println("float64", v)
		case []int:
			fmt.Println("slice", v)
		case nil:
			fmt.Println("nil")
		}
	}
	ce := make(chan error, 2)
	ce <- failure{3}
	ce <- nil
	fmt.Println(<-ce, <-ce)
}

// Output:
// 99 true true
// [1 20 3] 3
// 2 2
// 10,20,30,
// rex says woof
// meow (8 lives left) 8
// true
// int 1
// string s
// float64 2.5
// slice [1]
// nil
// failed 3 <nil>
