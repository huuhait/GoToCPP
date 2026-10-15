// phase: 03

// fmt and named types: a named type prints as its underlying value; one with a String method
// in its method set prints through it (for %v %s %q, and by Print and Println, not for %d),
// also inside slices, arrays, maps and structs (exported fields only); a String method on the
// pointer is used for the pointer only; the error forms name the Go type: %!d(main.Name=x).
package main

import "fmt"

type Celsius float64

func (c Celsius) String() string { return fmt.Sprintf("%.1f°C", float64(c)) }

type Meters float64

type Name string

type IDs []int

type Set map[string]bool

type Grid [2][2]int

type Color int

func (c Color) String() string {
	switch c {
	case 0:
		return "red"
	case 1:
		return "green"
	}
	return fmt.Sprintf("Color(%d)", int(c))
}

type Temp struct {
	Where string
	T     Celsius
	t     Celsius
}

type Counter struct{ n int }

func (c *Counter) String() string { return fmt.Sprint("counter=", c.n) }

type Flag bool

func main() {
	fmt.Println(Celsius(21.5), Meters(3), Name("ann"), IDs{1, 2}, Set{"b": true, "a": false}, Grid{{1, 2}, {3, 4}})
	fmt.Println(Color(0), Color(1), Color(7), Flag(true))
	fmt.Printf("%v %s %q %d %f\n", Color(1), Color(1), Color(1), Color(1), Celsius(1.25))
	fmt.Printf("%6.2f|%-8v|%8s\n", Meters(2), Celsius(5), Name("x"))
	fmt.Println([]Celsius{1, 2}, [2]Color{0, 1}, map[Color]Celsius{1: 10, 0: 20}, map[Name]int{"b": 1, "a": 2})
	fmt.Println(Temp{"in", 20, 18}, []Temp{{"out", -5, 0}})
	fmt.Printf("%+v\n", Temp{"in", 20, 18})
	c := Counter{3}
	fmt.Println(c, &c, []*Counter{&c}[0])
	fmt.Printf("%v %v %s\n", c, &c, &c)
	fmt.Printf("%d %s %t %d\n", Name("hi"), Meters(1.5), Color(1), Flag(false))
	fmt.Printf("%s %s %d\n", IDs{1}, Set{"k": true}, Grid{})
	var nc *Counter
	fmt.Println(nc)
	s := fmt.Sprint(Color(0), Color(1), Name("n"), Meters(2))
	fmt.Println(s)
	fmt.Println(fmt.Sprintln(Celsius(1), 2) + "|")
}

// Output:
// 21.5°C 3 ann [1 2] map[a:false b:true] [[1 2] [3 4]]
// red green Color(7) true
// green green "green" 1 1.250000
//   2.00|5.0°C   |       x
// [1.0°C 2.0°C] [red green] map[red:20.0°C green:10.0°C] map[a:2 b:1]
// {in 20.0°C 18} [{out -5.0°C 0}]
// {Where:in T:20.0°C t:18}
// {3} counter=3 counter=3
// {3} counter=3 counter=3
// %!d(main.Name=hi) %!s(main.Meters=1.5) %!t(main.Color=1) %!d(main.Flag=false)
// [%!s(int=1)] map[k:%!s(bool=true)] [[0 0] [0 0]]
// <nil>
// red greenn2
// 1.0°C 2
// |
