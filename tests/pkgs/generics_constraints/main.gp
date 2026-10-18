// phase: 07

// Constraints declared in another package: type sets, unions of constraints, embedded interfaces with methods, and main's own types satisfying them.
package main

import (
	"fmt"

	"example.com/gcons/cons"
)

type Cm int8

func (c Cm) String() string { return fmt.Sprint(int8(c), "cm") }

type Ratio float64

func (r Ratio) String() string { return fmt.Sprintf("%.0f%%", float64(r)*100) }

type Person struct{ name string }

func (p Person) Name() string { return p.name }

type Robot struct{ id int }

func (r *Robot) Name() string { return fmt.Sprint("robot-", r.id) }

func main() {
	fmt.Println(cons.Abs(-5), cons.Abs(2.5), cons.Abs(Cm(-100)), cons.Abs(Ratio(-0.5)), cons.Abs[int8](-128))
	fmt.Println(cons.Clamp(15, 0, 10), cons.Clamp(-0.5, 0, 1), cons.Clamp(Cm(5), 1, 3))
	fmt.Println(cons.Greet(Person{"ann"}, Person{"bob"}), cons.Greet(&Robot{7}), cons.Greet[cons.Named](Person{"c"}, &Robot{1}))
	fmt.Println(cons.Show(Cm(12)), cons.Show(Ratio(0.25)))
}

// Output:
// 5 2.5 100cm 50% -128
// 10 0 3cm
// [hello ann hello bob] [hello robot-7] [hello c hello robot-1]
// 12cm=12 25%=0.25
