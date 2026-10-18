package main

import (
	"fmt"

	"example.com/fwdnames/mid"
)

func main() {
	c := mid.Cat{}
	c.N = "x"
	fmt.Println(c.Both(), c.Hello())
}
