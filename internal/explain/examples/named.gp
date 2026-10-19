package main

import "fmt"

type Celsius float64

type IDs []int

func (c Celsius) F() float64 { return float64(c)*9/5 + 32 }

func main() {
	c := Celsius(36.6)
	var ids IDs = []int{1, 2}
	ids = append(ids, 3)
	fmt.Println(c.F(), c*2, len(ids))
}
