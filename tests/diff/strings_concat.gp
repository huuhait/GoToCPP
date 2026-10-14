// phase: 03

// Concatenation: chains of + (one allocation), += in loops, concatenation in every
// place (arguments, conditions, returns, case expressions), with Go's order of
// the calls in it; and a large string constant.
package main

import "fmt"

var log = ""

// A string constant of 256 KiB, with every byte value in it: one C++ literal.
const (
	c0 = "\x00\x01\x7f\x80\xfe\xff\"\\?\n\t0123456789abcdef"
	c1 = c0 + c0 + c0 + c0
	c2 = c1 + c1 + c1 + c1
	c3 = c2 + c2 + c2 + c2
	c4 = c3 + c3 + c3 + c3
	c5 = c4 + c4 + c4 + c4
	c6 = c5 + c5 + c5 + c5
	c7 = c6 + c6 + c6 + c6
	big = c7 + "end"
)

func w(s string) string {
	log += s
	return s
}

func join(a, b, c string) string {
	return a + "-" + b + "-" + c
}

func main() {
	s := ""
	for i := 0; i < 10; i++ {
		s += "ab"[i%2:i%2+1] + "|"
	}
	fmt.Println(s, len(s))
	t := ""
	for i := 0; i < 1000; i++ {
		t = t + "x"
	}
	fmt.Println(len(t), t[990:])
	fmt.Println(join("a", "b"+"c", ""), w("1")+w("2")+w("3")+w("4")+w("5")+w("6"), log)
	if w("x")+"y" == "xy" {
		fmt.Println("if", log)
	}
	switch w("k") + w("e") + "y" {
	case "ke" + w("y"):
		fmt.Println("no")
	case "k" + "ey":
		fmt.Println("key", log)
	}
	u := "u"
	u += u + u
	u += "-" + u + "-"
	fmt.Println(u, len(u), "a"+"b"+"c" == "abc", "" + "" + "" == "")
	e := ""
	fmt.Println(e+e+e == "", len(e+"z"+e), (e + "q" + e)[0])
	k := big
	sum := 0
	for i := 0; i < len(k); i++ {
		sum += int(k[i])
	}
	fmt.Println(len(big), len(k), sum, k[len(k)-5:], k[:2] == "\x00\x01", k[0], k[5])
	big := "0123456789"
	for i := 0; i < 4; i++ {
		big = big + big + big
	}
	fmt.Println(len(big), big[len(big)-3:])
}
