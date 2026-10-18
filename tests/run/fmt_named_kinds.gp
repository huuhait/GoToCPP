// phase: 05

// Named types of every kind with String methods: slices, maps, arrays, funcs, pointers, channels-
// free; the method wins over the underlying value's formatting for %v %s, not for %d; nested in
// each other.
package main

import "fmt"

type IntList []int

func (l IntList) String() string { return fmt.Sprintf("IntList(%d)", len(l)) }

type Dict map[string]int

func (d Dict) String() string { return fmt.Sprintf("Dict(%d)", len(d)) }

type Triple [3]int

func (t Triple) String() string { return fmt.Sprintf("Triple%v", [3]int(t)) }

type Op func(int) int

func (o Op) String() string { return "Op" }

type Ref *int

type Wrapper struct {
	L IntList
	D Dict
	T Triple
	O Op
}

type Celsius float64

func (c Celsius) String() string { return fmt.Sprintf("%.1f°C", float64(c)) }

type Temps []Celsius

type Grid [2][2]Celsius

type Index map[Celsius][]Celsius

func main() {
	l := IntList{1, 2, 3}
	d := Dict{"a": 1}
	t := Triple{1, 2, 3}
	op := Op(func(x int) int { return x })
	fmt.Println(l, d, t, op)
	fmt.Printf("%v %s %d %q %x\n", l, l, l, l, l)
	fmt.Printf("%v %s %d\n", d, d, d)
	fmt.Printf("%v %s %d %v\n", t, t, t, [3]int(t))
	fmt.Printf("%v %s %T\n", op, op, op)
	fmt.Printf("%v %+v\n", Wrapper{l, d, t, op}, Wrapper{l, d, t, op})
	fmt.Printf("%#v\n", Wrapper{l, d, t, nil})
	fmt.Println(Temps{1, 2.55}, Grid{{1, 2}, {3, 4}}, Index{1: {2, 3}})
	fmt.Printf("%v %.2v %5.1f %d\n", Temps{1.25}, Temps{1.25}, Temps{1.25}, Temps{1.25})
	var r Ref
	x := 3
	r = &x
	fmt.Println(r != nil, *r)
	fmt.Println([]IntList{{1}, nil}, map[string]Dict{"k": {}}, [1]Triple{})
	var np Ref
	fmt.Println(np)
}

// Output:
// IntList(3) Dict(1) Triple[1 2 3] Op
// IntList(3) IntList(3) [1 2 3] "IntList(3)" 496e744c697374283329
// Dict(1) Dict(1) map[%!d(string=a):1]
// Triple[1 2 3] Triple[1 2 3] [1 2 3] [1 2 3]
// Op Op main.Op
// {IntList(3) Dict(1) Triple[1 2 3] Op} {L:IntList(3) D:Dict(1) T:Triple[1 2 3] O:Op}
// main.Wrapper{L:main.IntList{1, 2, 3}, D:main.Dict{"a":1}, T:main.Triple{1, 2, 3}, O:(main.Op)(nil)}
// [1.0°C 2.5°C] [[1.0°C 2.0°C] [3.0°C 4.0°C]] map[1.0°C:[2.0°C 3.0°C]]
// [1.2°C] [1.] [  1.2] [%!d(main.Celsius=1.25)]
// true 3
// [IntList(1) IntList(0)] map[k:Dict(0)] [Triple[0 0 0]]
// <nil>
