// phase: 02

// Common idioms where C++'s promotion of small integers to int would give
// a different answer than Go: digit checks with s[i]-'0', byte sums that
// overflow, checksums, comparisons of a wrapped result and unsigned
// differences.
package main

import "fmt"

func isDigit(c byte) bool {
	return c-'0' <= 9
}

func isLower(c byte) bool {
	return c-'a' < 26
}

func checksum(s string) uint8 {
	var sum uint8
	for i := 0; i < len(s); i++ {
		sum += s[i]
	}
	return sum
}

func fletcher16(s string) uint16 {
	var a, b uint16
	for i := 0; i < len(s); i++ {
		a = (a + uint16(s[i])) % 255
		b = (b + a) % 255
	}
	return b<<8 | a
}

func hash(s string) uint16 {
	var h uint16 = 5381
	for i := 0; i < len(s); i++ {
		h = h*33 ^ uint16(s[i])
	}
	return h
}

func main() {
	s := "a0Z9 /:~\x80\xff"
	for i := 0; i < len(s); i++ {
		c := s[i]
		fmt.Println(c, isDigit(c), isLower(c), c-'0', c+128, c*2, c<<1>>1, -c)
	}
	fmt.Println(checksum("hello, world"), checksum(""), checksum("\xff\xff\x02"))
	fmt.Println(fletcher16("abcde"), fletcher16("abcdef"), fletcher16("abcdefgh"))
	fmt.Println(hash("gocpp"), hash("a longer string to hash"))

	var a, b uint8 = 200, 100
	fmt.Println(a+b > 250, a+b, a+b < a, a-b > a, b-a > b, b-a)
	var i, j int8 = 100, 100
	fmt.Println(i+j < 0, i+j, i*j, int(i)+int(j), int(i+j))
	var u, v uint16 = 60000, 10000
	fmt.Println(u+v < u, (u+v)/2, u/2+v/2, uint32(u)+uint32(v))
	var p, q int16 = -32768, 1
	fmt.Println(p-q > p, p-q, -p == p, p/-1, p%-1)
	var w uint8 = 255
	w++
	fmt.Println(w == 0, w)
	var k int8 = 1
	for range 10 {
		k *= 3
	}
	fmt.Println(k)
	var m uint8 = 0x0F
	fmt.Println(^m, ^m>>4, ^m&0xF0 == 0xF0, m<<4|m>>4)
}
