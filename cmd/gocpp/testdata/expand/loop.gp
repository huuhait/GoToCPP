package main

import "strconv"

func f() (int, error) {
	n := 0
	for i := 0; i < 3; i += strconv.Atoi!("1") {
		// body comment
		n++
		/* tail */
	}
	return n, nil
}

func main() {}
