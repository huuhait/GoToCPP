//test: cmd=check
package main

func main() {
	e := "éé" + 1 // combining accents
	z := "👨‍👩‍👧" + 2
	_, _ = e, z
}
