// phase: 02

// switch with fallthrough, default in different places, several values
// per case, a tag of every kind (small ints, unsigned, strings, floats,
// bools) and break out of a switch inside a loop.
package main

import "fmt"

func classify(n int8) string {
	out := ""
	switch n {
	case -128:
		out += "min "
		fallthrough
	case -1, -2, -3:
		out += "negative-ish "
	default:
		out += "default "
		fallthrough
	case 0:
		out += "zero-or-default "
		fallthrough
	case 1:
		out += "one-ish "
	case 127:
		out += "max "
		fallthrough
	case 100:
		out += "hundred "
	}
	return out
}

func unsigned(u uint16) int {
	switch u {
	case 65535:
		return 1
	case 0:
		return 2
	case 255, 256:
		return 3
	}
	return 0
}

func float(f float64) string {
	switch f {
	case 0:
		return "zero"
	case 0.5, 0.25:
		return "fraction"
	case 1e100:
		return "big"
	}
	return "other"
}

func main() {
	values := 0
	for i := -130; i <= 130; i++ {
		n := int8(i)
		if i%37 != 0 && n != -128 && n != 127 && n != 100 && n > 3 && n != -1 {
			continue
		}
		values++
		fmt.Println(i, n, classify(n))
	}
	fmt.Println(values)
	fmt.Println(unsigned(65535), unsigned(0), unsigned(255), unsigned(256), unsigned(1))
	zero := 0.0
	fmt.Println(float(0), float(-zero), float(0.25), float(1e100), float(zero/zero))
	for i := range 6 {
		switch {
		case i == 1:
			fmt.Println("one")
			fallthrough
		case i == 2:
			fmt.Println("one or two")
			if i == 2 {
				break
			}
			fmt.Println("only one")
		case i > 3:
			fmt.Println("big")
			continue
		}
		fmt.Println("end", i)
	}
	switch s := "go"; s + "pher" {
	case "gopher":
		fmt.Println("gopher")
		fallthrough
	case "x":
		fmt.Println("fell into x")
	}
	b := true
	switch b {
	case false:
		fmt.Println("no")
	default:
		fmt.Println("default bool")
	}
}
