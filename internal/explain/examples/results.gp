package main

import "fmt"

func divmod(a, b int) (int, int) {
	return a / b, a % b
}

func main() {
	q, r := divmod(17, 5)
	q, r = r, q
	fmt.Println(q, r)
}
