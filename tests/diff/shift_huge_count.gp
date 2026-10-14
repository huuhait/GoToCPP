// phase: 03

// Shifts by a huge count that is not a constant. The count 1<<n - 2 is a uint (the untyped 1
// takes the type of the count), so for n = 0 it wraps to 1<<64 - 1: x >> c is then 0 (-1 for
// a negative x) and x << c is 0, also when the result is masked and used as the index of
// copy, of a slice expression and of an index expression (TestRandomOrder writes such counts).
// Every count reads gz, so that gc can't fold it to a constant: gc 1.25 on amd64 miscompiles
// ((x >> c) & m) << k for a constant c >= 1<<63 (its rewrite rule compares c as a signed
// number), which made seed 5021 of TestRandomOrder print other values with real Go.
package main

import "fmt"

var gz = 0
var gsl = []int{1, 2, 3, 4, 5}
var gal = gsl[0:2]

func e(tag, v int) int {
	fmt.Print("e", tag, " ")
	return v
}

func show() {
	fmt.Println(gsl[0], gsl[1], gsl[2], gsl[3], gsl[4])
}

func main() {
	p := -1
	var c uint = 1<<uint(gz) - 2
	fmt.Println(c, 3>>c, -3>>c, 3<<c, (3>>c)&3, (-3>>c)&3)

	copy(gsl[(e(1, 3)>>(1<<(int(p-p)&127+gz)-2))&3:], gal)
	show()
	copy(gsl[(e(2, -3)>>(1<<(int(p-p)&127+gz)-2))&3:], gal)
	show()
	t := gsl[(e(3, 3)>>(1<<(int(p-p)&127+gz)-2))&3:]
	fmt.Println(len(t), gsl[(e(4, 7)>>c)&3], gsl[(e(5, -7)>>c)&3])

	// Counts around the width: 62 to 65.
	for k := range 4 {
		s := uint(62 + k + gz)
		fmt.Println(k, 1>>s, -1>>s, 1<<s, int8(-128)>>s, uint8(255)<<s, uint64(1<<63)>>s, (e(6+k, 3)>>s)&3)
	}
}
