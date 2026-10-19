package main

import "<cstdlib>"

type Celsius float64

func main() {
	c := Celsius(1.5)
	_ = cstdlib.abs(c)
}
