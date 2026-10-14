// Substrings and concatenation (Phase 03 step 3.4): s[a:b], s[a:], s[:b] and s[:]
// share the bytes of s (gp::sub checks the bounds), with bounds of any integer type;
// a chain of + is one gp::concat; the order of calls, reads and bound checks is Go's
// (gc's: a slice expression is evaluated in its place among the calls).
package main

import "fmt"

var calls = 0

func next() int {
	calls++
	return calls
}

var g = "global"
var n = 1

// shift changes g and n.
func shift() int {
	g, n = "XYZ123", 3
	return 1
}

// reset sets them back.
func reset() {
	g, n = "abcdef", 1
}

func grow() string {
	g += "!"
	return "<" + g + ">"
}

func main() {
	s := "hello, world"
	i, j := 7, 12
	var lo uint8 = 1
	var hi int16 = 4
	fmt.Println(s[i:j], s[i:], s[:5], s[:], s[j:], s[lo:hi], s[lo:], s[:hi], len(s[3:3]))
	fmt.Println(s[0:], s[1:4] == "ell", s[i:j][1:3], "literal"[3:], s[len(s)-1:], s[i:][:2])
	fmt.Println(s[next():next()], s[next():], calls)

	// Concatenation: two operands stay a + b, longer chains are one gp::concat.
	t := s[:5] + "!" + s[i:] + "?"
	u := s + (t + s[:1])
	fmt.Println(t, u, len(t+u), t[5], s[0:0] == "")
	w := ""
	for k := 0; k < 5; k++ {
		w += s[k:k+1] + "-"
		w += "|"
	}
	fmt.Println(w, len(w))
	fmt.Println(g+grow()+g, g)
	x := "a"
	x = x + x + x
	x += x + "b"
	fmt.Println(x, "c"+x+"d"+grow())

	// gc evaluates a slice expression in its place among the calls (it reads g and n
	// there), and its bounds that are not a name, a constant or len of a name before it.
	reset()
	fmt.Println(g[n:], shift())
	reset()
	fmt.Println(g[1:n+1], g[n:len(g)], shift())
	reset()
	y := g[n:] + g[:shift()]
	reset()
	fmt.Println(y, len(g[n:]), g[n:] == "bcdef", g+g[n:], g+"-", shift())
	reset()
	fmt.Println(g[n+0:shift()+2], n)
	reset()
	fmt.Println(g[n:shift()+2], n)
	reset()
	fmt.Println(g[:n+1][n:], shift(), g[n:][:n])
}
