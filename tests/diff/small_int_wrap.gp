// phase: 02

package main

import "fmt"

func main() {
	var a int8 = 127
	a++
	var b uint8 = 200
	b += 100
	var c int16 = -32768
	c--
	var d uint16
	d--
	fmt.Println(a, b, c, d)

	var m int8 = -128
	fmt.Println(m/-1, -m, m*m)
}
