// phase: 03

// Deferred calls with arguments of named types (steps 3.9 and 3.13): the arguments are evaluated
// at the defer statement, also when Go converts them implicitly to a parameter's named type, and
// a named result changed before the return.
package main

import "fmt"

type Celsius float64
type IDs []int

func show(c Celsius, ids IDs) {
	fmt.Println("show", c, len(ids))
}

func total(ids IDs) (t Celsius) {
	defer fmt.Println("total of", len(ids))
	for _, v := range ids {
		t += Celsius(v)
	}
	return t * 2
}

func main() {
	c := Celsius(1.5)
	ids := IDs{1, 2}
	defer show(c+1, []int{7, 8, 9})
	defer fmt.Println("deferred", c, ids[0])
	c *= 10
	ids[0] = 100
	fmt.Println(total(ids), c)
}
