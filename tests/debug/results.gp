// phase: 02

// Multiple and named results, and values of the basic kinds: int, uint8,
// float64, bool and string. gdb shows a uint8 (C++ uint8_t, an unsigned
// char) with its character, and a string (gp::string) as its C++ fields:
// the bytes are *s.gp_data_@s.gp_len_. fmt.Println(divmod(9, 4)) becomes a
// call of a C++ lambda, so its line has code in two functions.
package main

import "fmt"

// divmod has named results, which start at zero and are returned by a bare
// return.
func divmod(a, b int) (q, r int) {
	q = a / b
	r = a % b // gdb: print q = 3; print r = 0; print a = 17; print b = 5; next 17
	return
}

func scale(x float64, s string) (float64, string, bool) {
	y := x * 2.5
	label := s + "!"
	return y, label, y > 10 // gdb: print x = 4; print y = 10; print s.gp_len_ = 2; print *label.gp_data_@label.gp_len_ = "go!"
}

func main() {
	q, r := divmod(17, 5)
	y, label, big := scale(4, "go")
	var u8 uint8 = 250
	u8 += 10
	fmt.Println(q, r, y, label, big, u8) // gdb: print q = 3; print r = 2; print y = 10; print big = false; print u8 = 4 '\004'; print label ~ gp_data_ = 0x[0-9a-f]+ "go!", gp_len_ = 3}$
	fmt.Println(divmod(9, 4)) // gdb: print q = 3; next 33
}
