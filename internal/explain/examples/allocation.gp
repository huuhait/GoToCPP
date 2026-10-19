package main

import "fmt"

type big struct{ a, b, c int }

func main() {
	p := new(big)
	s := make([]int, 0, 4)
	s = append(s, 1, 2)
	var v any = *p
	str := "x" + fmt.Sprint(len(s)) + "y"
	fmt.Println(v, str, []byte(str))
}
