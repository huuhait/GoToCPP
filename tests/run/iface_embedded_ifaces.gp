// phase: 05

// Interfaces that embed interfaces, and the method set of the embedded one (step 5.4).
package main

import "fmt"

type Named interface{ Name() string }
type Aged interface{ Age() int }
type Person interface {
	Named
	Aged
	Describe() string
}
type Employee interface {
	Person
	Salary() int
}

type emp struct {
	name   string
	age    int
	salary int
}

func (e emp) Name() string     { return e.name }
func (e emp) Age() int         { return e.age }
func (e emp) Salary() int      { return e.salary }
func (e emp) Describe() string { return fmt.Sprint(e.name, " ", e.age) }

func show(n Named) string { return n.Name() }

func main() {
	var e Employee = emp{"ann", 30, 100}
	var p Person = e
	var n Named = p
	var a Aged = e
	fmt.Println(e.Salary(), p.Describe(), n.Name(), a.Age())
	fmt.Println(show(e), show(p), show(emp{"bob", 1, 2}))
	var x any = e
	if pp, ok := x.(Person); ok {
		fmt.Println("person", pp.Describe())
	}
	if _, ok := x.(Employee); ok {
		fmt.Println("employee")
	}
	var anon interface {
		Named
		Salary() int
	} = e
	fmt.Println(anon.Name(), anon.Salary())
	var back Employee = anon.(Employee)
	fmt.Println(back.Age())
}

// Output:
// 100 ann 30 ann 30
// ann ann bob
// person ann 30
// employee
// ann 100
// 30
