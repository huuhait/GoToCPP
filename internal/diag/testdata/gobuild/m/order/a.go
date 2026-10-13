// The error in b.go is found first: go/types and the go compiler check
// package-level variables before function bodies.
package order

func f() {
	println(undefinedInBody)
}
