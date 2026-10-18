// phase: 05

package main

import "fmt"

func lvl3() {
	defer fmt.Println("lvl3 defer")
	panic("deep")
}

func lvl2() {
	defer fmt.Println("lvl2 defer")
	lvl3()
	fmt.Println("lvl2 not reached")
}

func lvl1() (out string) {
	defer func() {
		if r := recover(); r != nil {
			out = fmt.Sprint("lvl1 recovered ", r)
		}
	}()
	defer fmt.Println("lvl1 defer")
	lvl2()
	return "lvl1 normal"
}

func main() {
	defer fmt.Println("main defer")
	fmt.Println(lvl1())
	fmt.Println("main continues")
}
