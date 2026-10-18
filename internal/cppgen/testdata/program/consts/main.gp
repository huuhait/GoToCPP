package main

import (
	"fmt"

	"example.com/consts/k"
)

func main() {
	fmt.Println(k.Huge/k.Big > 0, k.Small, k.Pi2, k.F32, k.U8, k.Neg, uint64(k.Mask), len(k.Str), k.Ch, k.Flag, k.Cplx, k.Sh)
	fmt.Println(k.Dur.Millis(), k.Unicode, k.Ratio(), k.Second)
	fmt.Printf("%q\n", k.Str)
}
