package main

import "fmt"

func main() {
	c := complex(1.5, -2)
	d := c * c
	fmt.Println(d, real(d), imag(c) < 0)
}
