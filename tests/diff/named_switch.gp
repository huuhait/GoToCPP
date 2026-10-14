// phase: 03

// switch on values of named types (step 3.9): an enum of a named integer type is a C++ switch
// on its value with the constants folded; named strings, floats and bools compare like their
// underlying types; fallthrough, case lists and case values that are not constants.
package main

import "fmt"

type Color int

const (
	Red Color = iota
	Green
	Blue
)

type Mode string
type Ratio float64
type On bool

func name(c Color) string {
	switch c {
	case Red:
		return "red"
	case Green, Blue:
		return "green or blue"
	}
	return "?"
}

func next(c Color) Color { return (c + 1) % 3 }

func main() {
	for c := range Color(4) {
		fmt.Println(c, name(c))
	}
	c := Blue
	switch c {
	case Red:
		fmt.Println("r")
	case Blue:
		fmt.Println("b")
		fallthrough
	case Green:
		fmt.Println("g after b")
	default:
		fmt.Println("none")
	}
	switch next(c) {
	case next(Green):
		fmt.Println("next of green")
	case Red:
		fmt.Println("red")
	}
	m := Mode("fast")
	switch m {
	case "slow":
		fmt.Println("slow")
	case Mode("fa") + "st":
		fmt.Println("fast")
	}
	r := Ratio(0.5)
	switch r * 2 {
	case 1:
		fmt.Println("one")
	}
	var on On = r > 0.25
	switch on {
	case false:
		fmt.Println("off")
	case true:
		fmt.Println("on")
	}
	switch on {
	case r < 0:
		fmt.Println("never")
	case true, On(false):
		fmt.Println("on or off")
	}
	switch {
	case c == Blue && bool(on):
		fmt.Println("blue and on")
	}
	var colors = map[Color]string{Red: "R", Blue: "B"}
	fmt.Println(colors[Blue], len(colors), colors[Green] == "")
	arr := [3]Color{Blue, Red}
	fmt.Println(arr[0] == Blue, arr[2])
	fmt.Printf("%d %v %s %t\n", c, r, m, on)
}
