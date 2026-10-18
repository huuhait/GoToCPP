// phase: 05

// Type assertions in all their forms: x.(T) of concrete and interface types, v, ok := x.(T) with
// :=, = and var, in if/else chains and with an init statement (step 5.5).
package main

import "fmt"

type Animal interface{ Sound() string }
type Walker interface{ Walk() string }

type Dog struct{ name string }
type Bird struct{ name string }

func (d Dog) Sound() string  { return d.name + " woof" }
func (d Dog) Walk() string   { return d.name + " walks" }
func (b *Bird) Sound() string { return b.name + " tweet" }

func get(i int) any {
	switch i {
	case 0:
		return Dog{"rex"}
	case 1:
		return &Bird{"tweety"}
	case 2:
		return 42
	case 3:
		return "text"
	}
	return nil
}

func main() {
	for i := 0; i < 5; i++ {
		x := get(i)
		if d, ok := x.(Dog); ok {
			fmt.Println("dog", d.name)
		} else if b, ok := x.(*Bird); ok {
			fmt.Println("bird", b.name)
		} else if n, ok := x.(int); ok {
			fmt.Println("int", n)
		} else if s, ok := x.(string); ok && len(s) > 2 {
			fmt.Println("string", s)
		} else {
			fmt.Println("other", x == nil)
		}
	}
	var d Dog
	var ok bool
	d, ok = get(0).(Dog)
	fmt.Println(d.name, ok)
	d, ok = get(1).(Dog)
	fmt.Println(d.name == "", ok)
	var a, ok2 = get(0).(Animal)
	fmt.Println(a.Sound(), ok2)
	if w, ok := get(0).(Walker); ok {
		fmt.Println(w.Walk())
	}
	if _, ok := get(1).(Walker); !ok {
		fmt.Println("bird can't walk")
	}
	_, ok = get(4).(Animal)
	fmt.Println(ok)
	fmt.Println(get(0).(Dog).name, get(0).(Animal).Sound(), get(1).(*Bird).name)
	n := get(2).(int) + 1
	fmt.Println(n, len(get(3).(string)))
}

// Output:
// dog rex
// bird tweety
// int 42
// string text
// other true
// rex true
// true false
// rex woof true
// rex walks
// bird can't walk
// false
// rex rex woof tweety
// 43 4
