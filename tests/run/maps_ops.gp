// phase: 03

// Changing an entry: op= and ++ insert a missing key as its zero value first; integers wrap
// like Go's (also the small ones), strings append, floats follow IEEE (x / 0 is +Inf), and
// m[k] = m[k] + 1 is the same as m[k]++.
package main

import "fmt"

func main() {
	m := map[string]int{}
	m["a"] += 5
	m["a"] -= 2
	m["a"] *= 7
	m["b"]++
	m["b"]++
	m["c"]--
	m["d"] = m["d"] + 1
	m["e"] |= 12
	m["e"] &= 6
	m["e"] ^= 1
	m["e"] &^= 1
	m["f"] = 100
	m["f"] /= 3
	m["g"] = 100
	m["g"] %= 7
	m["h"] = -9
	m["h"] /= -1
	m["i"] = 1
	m["i"] <<= 10
	m["i"] >>= 3
	n := 70
	m["j"] = 1
	m["j"] <<= n
	m["k"] = -8
	m["k"] >>= n
	d := 4
	m["l"] = 33
	m["l"] /= d
	m["l"] %= d - 1
	fmt.Println(m["a"], m["b"], m["c"], m["d"], m["e"], m["f"], m["g"], m["h"])
	fmt.Println(m["i"], m["j"], m["k"], m["l"], len(m))

	small := map[int]uint8{}
	small[0] += 200
	small[0] += 100
	small[1]--
	small[2] = 3
	small[2] <<= 7
	fmt.Println(small[0], small[1], small[2])
	signed := map[int]int8{0: 127, 1: -128}
	signed[0]++
	signed[1]--
	signed[2] = -128
	signed[2] /= -1
	signed[3] = 100
	signed[3] *= 3
	fmt.Println(signed[0], signed[1], signed[2], signed[3])
	mins := map[bool]int{true: -9223372036854775808}
	mins[true] /= -1
	mins[false] = -9223372036854775808
	mins[false]--
	fmt.Println(mins[true], mins[false])
	u := map[uint]uint64{}
	u[1]--
	u[2] = 1 << 63
	u[2] *= 2
	fmt.Println(u[1], u[2])

	s := map[int]string{}
	s[0] += "ab"
	s[0] += "cd"
	s[1] = "x"
	s[1] += s[0] + "!"
	fmt.Println(s[0], s[1], len(s[2]))

	f := map[string]float64{"x": 1}
	f["x"] /= 4
	f["y"] -= 0.5
	zero := 0.0
	f["z"] = 1
	f["z"] /= zero
	f32 := map[string]float32{"x": 0.1}
	f32["x"] += 0.2
	fmt.Println(f["x"], f["y"], f["z"], f32["x"], len(f))
}

// Output:
// 21 2 -1 1 4 33 2 9
// 128 0 -1 2 12
// 44 255 128
// -128 127 -128 44
// -9223372036854775808 9223372036854775807
// 18446744073709551615 0
// abcd xabcd! 0
// 0.25 -0.5 +Inf 0.3 3
