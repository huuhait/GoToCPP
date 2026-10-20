// phase: 05

// fmt.Println(args...) reads the elements of the slice while it prints, as Go does: a String method
// that stores into the slice changes the operands that come after it.
package main

import "fmt"

var shared []any

type Mut struct{ tag string }

func (m *Mut) String() string {
	shared[1] = "changed by " + m.tag
	shared[2] = 99
	return "mut-" + m.tag
}

func fresh() {
	shared = []any{&Mut{"a"}, "orig", 1, "tail"}
}

func main() {
	fresh()
	fmt.Println(shared...)
	fresh()
	fmt.Print(shared...)
	fmt.Println()
	fresh()
	fmt.Printf("%v|%v|%d|%s\n", shared...)
	fresh()
	fmt.Println(fmt.Sprint(shared...), fmt.Sprintln(shared...) == "mut-a changed by a 99 tail\n")
	fresh()
	fmt.Printf("%v\n", shared[1:]...)
	fresh()
	fmt.Println(fmt.Sprintf("%v", shared...)) // the extra operands read after the first changed them
}

// Output:
// mut-a changed by a 99 tail
// mut-achanged by a99tail
// mut-a|changed by a|99|tail
// mut-achanged by a99tail true
// orig
// %!(EXTRA int=1, string=tail)mut-a%!(EXTRA string=changed by a, int=99, string=tail)
