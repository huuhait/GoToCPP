// bench: strconv_roundtrip
// category: data
// size: 1500000
// smoke: 2000
// twin: no
// about: strconv.Itoa/Atoi and FormatFloat/ParseFloat round trips
package main

import (
	"fmt"
	"strconv"
)

func main() {
	total := 0
	var fsum float64
	for i := 0; i < benchN; i++ {
		s := strconv.Itoa(i * 7919)
		n, err := strconv.Atoi(s)
		if err != nil {
			panic(err)
		}
		total += n + len(s)
		f := strconv.FormatFloat(float64(i)*0.37, 'f', 3, 64)
		g, err := strconv.ParseFloat(f, 64)
		if err != nil {
			panic(err)
		}
		fsum += g
		total += len(f)
	}
	fmt.Println(total, int64(fsum))
}
