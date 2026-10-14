// phase: 01

// gocpp:link m // WARNING "an ordinary comment"
package main

func main() {
	x := 1 // ERROR "declared and not used: x" // WARNING "w1" "w2"
	_ = 2 // WARNING "only a warning" // ERROR "then an error"
	_ = 3 // WARNING "LINE"
	_ = 4 // WARNINGS is not a WARNING comment
	_ = 5 //// WARNING "turned off"
}
