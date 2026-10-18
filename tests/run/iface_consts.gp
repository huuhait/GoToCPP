// phase: 05

// Constants of every kind converted to interface types: typed, untyped, iota, big, rune
// (step 5.4).
package main

import "fmt"

type Color int

const (
	Red Color = iota
	Green
	Blue
)

func (c Color) String() string { return [...]string{"red", "green", "blue"}[c] }

type Stringer interface{ String() string }

const Pi = 3.14159
const Big = 1 << 62
const Greeting = "hi"
const Letter = 'q'
const Yes = true

type Weekday uint8

const Sunday Weekday = 0

func main() {
	var s Stringer = Green
	fmt.Println(s.String())
	items := []any{Red, 1, Pi, Big, Greeting, Letter, Yes, Sunday, 'a', "b", 1e3, 07, 0x10, 1.0}
	types := ""
	for _, it := range items {
		switch it.(type) {
		case Color:
			types += "Color "
		case int:
			types += "int "
		case float64:
			types += "float64 "
		case string:
			types += "string "
		case rune:
			types += "rune "
		case bool:
			types += "bool "
		case Weekday:
			types += "Weekday "
		}
	}
	fmt.Println(types[:len(types)-1])
	fmt.Println(items[3].(int) == Big, items[2].(float64) == Pi, items[5].(rune) == 'q')
	var c Color = Blue
	var a any = c
	_, isInt := a.(int)
	fmt.Println(isInt, a.(Color) == Blue, a.(Stringer).String())
}

// Output:
// green
// Color int float64 int string rune bool Weekday rune string float64 int int float64
// true true true
// false true blue
