// phase: 03

// range with = over an array: the targets are evaluated in every iteration, so a call in a
// target (t[f()]) can change the array or drop the last pointer to it, and a target can be a
// part of the array itself (arr[0]). Go ranges over a copy, so none of them changes what the
// loop sees.
package main

import "fmt"

var p *[3]int
var gs [][3]int
var arr [3]int

func dropP() int {
	p = nil
	return 0
}

func setP() int {
	*p = [3]int{7, 7, 7}
	return 0
}

func dropGs() int {
	gs = nil
	return 0
}

func setArr() int {
	arr = [3]int{9, 9, 9}
	return 0
}

func main() {
	var t [1]int
	var u [3]int
	var i int

	p = &[3]int{1, 2, 3}
	sum := 0
	for i, t[dropP()] = range *p {
		sum += i + t[0]
	}
	fmt.Println(sum, p == nil)

	p = &[3]int{1, 2, 3}
	sum = 0
	for i, t[setP()] = range *p {
		sum += i + t[0]
	}
	fmt.Println(sum)

	gs = [][3]int{{4, 5, 6}}
	sum = 0
	for i, t[dropGs()] = range gs[0] {
		sum += i + t[0]
	}
	fmt.Println(sum, gs == nil)

	arr = [3]int{1, 2, 3}
	sum = 0
	for i, t[setArr()] = range arr {
		sum += i + t[0]
	}
	fmt.Println(sum)

	arr = [3]int{1, 2, 3}
	sum = 0
	for u[setArr()], t[0] = range arr {
		sum += t[0]
	}
	fmt.Println(sum, u[0])

	arr = [3]int{1, 2, 3}
	sum = 0
	for i, arr[0] = range arr {
		sum += i + arr[0]
	}
	fmt.Println(sum, arr[0])

	loc := [3]int{1, 2, 3}
	sum = 0
	for i, loc[2] = range loc {
		sum += i + loc[2]
	}
	fmt.Println(sum, loc[2])
	for loc[0], loc[1] = range loc {
	}
	fmt.Println(loc[0], loc[1], loc[2])
}
