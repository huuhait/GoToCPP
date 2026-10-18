// phase: 05

// Method values of interfaces that hold large values and pointers to large values (step 5.9):
// the interface keeps its own copy of a large array struct, so the method value sees the value
// of the conversion, not later changes of the variable.
package main

import "fmt"

type Big struct {
	data [40000]int
	tag  string
}

func (b Big) Sum() int {
	t := 0
	for _, x := range b.data {
		t += x
	}
	return t
}

func (b *Big) Set(i, v int) { b.data[i] = v }

func (b Big) Tag() string { return b.tag }

type Summer interface{ Sum() int }
type Setter interface {
	Summer
	Set(i, v int)
}

func main() {
	var b Big
	b.data[0], b.data[39999] = 1, 2
	b.tag = "big"
	var s Summer = b
	sum := s.Sum
	b.data[1] = 100
	fmt.Println(sum(), b.Sum(), s.Sum())

	var st Setter = &b
	set := st.Set
	total := st.Sum
	set(2, 1000)
	fmt.Println(total(), b.Sum(), sum())

	// A method value of a pointer receiver on a large value keeps the pointer.
	bp := &Big{tag: "heap"}
	var s2 Setter = bp
	f := s2.Set
	g := s2.Sum
	for i := 0; i < 10; i++ {
		f(i, i)
	}
	fmt.Println(g(), bp.Sum(), bp.Tag())

	// Many copies of the method value share one box and each keeps it alive.
	var fs []func() int
	for i := 0; i < 5; i++ {
		c := b
		c.data[3] = i
		var sm Summer = c
		fs = append(fs, sm.Sum)
	}
	for _, f := range fs {
		fmt.Print(f(), " ")
	}
	fmt.Println()
}

// Output:
// 3 103 3
// 1103 1103 3
// 45 45 heap
// 1103 1104 1105 1106 1107
