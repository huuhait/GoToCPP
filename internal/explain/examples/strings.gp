package main

import "fmt"

func main() {
	a := "go"
	b := a + "++"
	c := b + "!" + a
	bs := []byte(c)
	fmt.Println(c[1:3], string(bs[:2]), string(rune(65)))
}
