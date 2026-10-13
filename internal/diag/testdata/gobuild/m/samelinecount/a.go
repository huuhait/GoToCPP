// The dropped second d does not count: go build still shows u2 to u10.
package samelinecount

func f() {
	println(d + d)
	println(u2)
	println(u3)
	println(u4)
	println(u5)
	println(u6)
	println(u7)
	println(u8)
	println(u9)
	println(u10)
}
