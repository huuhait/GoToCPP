//test: cmd=check
package main

type P struct{ X, Y int }

func main() {
	p := P{
		X: 1,
		Z: 2,
	}
	_ = p
}
