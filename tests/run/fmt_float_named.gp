// phase: 05

// Named float types print as their underlying float; the error forms name the type; %v of
// Stringer floats.
package main

import "fmt"

type Celsius float64

type Ratio float32

func main() {
	fmt.Printf("%v %.1f %g %e %6.2f|%-8.3v|\n", Celsius(21.5), Celsius(21.55), Celsius(1e21), Celsius(1234.5), Celsius(3.14159), Celsius(2.71828))
	fmt.Printf("%v %.2f %g %s %d\n", Ratio(0.1), Ratio(0.125), Ratio(1)/3, Ratio(2), Ratio(3))
	fmt.Println(Celsius(1.5), Ratio(0.1), []Celsius{1, 2.5}, map[string]Ratio{"a": 0.5})
	fmt.Printf("%T %T %#v %#v\n", Celsius(1), Ratio(1), Celsius(2), Ratio(0.25))
}

// Output:
// 21.5 21.6 1e+21 1.234500e+03   3.14|2.72    |
// 0.1 0.12 0.33333334 %!s(main.Ratio=2) %!d(main.Ratio=3)
// 1.5 0.1 [1 2.5] map[a:0.5]
// main.Celsius main.Ratio 2 0.25
