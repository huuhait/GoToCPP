// bench: fmt_println
// category: runtime
// size: 400000
// smoke: 2000
// twin: no
// about: fmt.Println of lines with an int, a string and a float to standard output (the runner hashes the output)
package main

import "fmt"

func main() {
	for i := 0; i < benchN; i++ {
		fmt.Println("line", i, "of", benchN, true, i%10)
	}
}
