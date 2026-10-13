// The error at the end is found first (package level), so go build
// shows it and u1 to u9, and stops at u9.
package reportorder

func f() {
	println(u1)
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

var last int = undefinedLast
