// The second x on line 6 has the same line and message as the error
// before it, so go build drops it.
package dups

func f() {
	println(x + x)
	println(x)
}
