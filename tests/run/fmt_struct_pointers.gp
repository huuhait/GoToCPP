// phase: 05

// Pointers in structs: &{...} for a pointer to a struct, array, slice or map at the top level;
// inside a value a pointer prints its address (here replaced by 0xP); nil pointers print <nil>.
package main

import "fmt"

// norm replaces every hexadecimal address (0x...) with 0xP: addresses differ from run to run.
func norm(s string) string {
	var out []byte
	for i := 0; i < len(s); i++ {
		if s[i] == '0' && i+1 < len(s) && s[i+1] == 'x' {
			j := i + 2
			for j < len(s) && (s[j] >= '0' && s[j] <= '9' || s[j] >= 'a' && s[j] <= 'f') {
				j++
			}
			if j > i+2 {
				out = append(out, "0xP"...)
				i = j - 1
				continue
			}
		}
		out = append(out, s[i])
	}
	return string(out)
}

type Node struct {
	V    int
	Next *Node
}

type Tree struct {
	L, R *Tree
	Val  int
}

type Holder struct {
	P  *int
	PS *Point
	PA *[2]int
	PM *map[string]int
}

type Point struct{ X, Y int }

func main() {
	n := Node{1, &Node{2, nil}}
	fmt.Println(norm(fmt.Sprint(n)), norm(fmt.Sprint(&n)))
	fmt.Println(norm(fmt.Sprintf("%+v %+v", n, &n)))
	fmt.Println(norm(fmt.Sprintf("%#v", n)))
	t := &Tree{nil, nil, 1}
	fmt.Println(*t, t)
	fmt.Printf("%v %+v\n", Tree{}, &Tree{})
	x := 5
	h := Holder{&x, &Point{1, 2}, &[2]int{1, 2}, &map[string]int{"a": 1}}
	fmt.Println(norm(fmt.Sprint(h)))
	fmt.Println(norm(fmt.Sprintf("%+v", h)))
	fmt.Println(Holder{})
	fmt.Printf("%v %v %v %v\n", &[2]int{1, 2}, &[]int{3}, &map[int]bool{1: true}, &[0]int{})
	var pp **int
	px := &x
	pp = &px
	fmt.Println(norm(fmt.Sprint(pp)), pp != nil)
	var np *Node
	var ni *int
	fmt.Println(np, ni, []*int{nil}, map[string]*Node{"k": nil})
	fmt.Printf("%v %+v %#v %s %d\n", np, np, np, np, np)
	fmt.Printf("%t %t\n", fmt.Sprint(&x)[:2] == "0x", fmt.Sprintf("%p", &x)[:2] == "0x")
}

// Output:
// {1 0xP} &{1 0xP}
// {V:1 Next:0xP} &{V:1 Next:0xP}
// main.Node{V:1, Next:(*main.Node)(0xP)}
// {<nil> <nil> 1} &{<nil> <nil> 1}
// {<nil> <nil> 0} &{L:<nil> R:<nil> Val:0}
// {0xP 0xP 0xP 0xP}
// {P:0xP PS:0xP PA:0xP PM:0xP}
// {<nil> <nil> <nil> <nil>}
// &[1 2] &[3] &map[1:true] &[]
// 0xP true
// <nil> <nil> [<nil>] map[k:<nil>]
// <nil> <nil> (*main.Node)(nil) %!s(*main.Node=<nil>) 0
// true true
