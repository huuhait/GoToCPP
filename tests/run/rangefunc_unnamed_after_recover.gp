// phase: 07

package main

import "fmt"

func bad(yield func(int) bool) {
	yield(1)
	panic("iter panic")
}

func named() (r int) {
	defer func() { fmt.Println("named recovered", recover(), r) }()
	for x := range bad {
		return x + 41
	}
	return 0
}

func unnamed() int {
	defer func() { fmt.Println("unnamed recovered", recover()) }()
	for x := range bad {
		return x + 41
	}
	return 0
}

func two() (string, int) {
	defer func() { fmt.Println("two recovered", recover()) }()
	for x := range bad {
		return "s", x
	}
	return "none", 0
}

func main() {
	fmt.Println(named())
	fmt.Println(unnamed())
	fmt.Println(two())
}

// Output:
// named recovered iter panic 42
// 42
// unnamed recovered iter panic
// 42
// two recovered iter panic
// s 1
