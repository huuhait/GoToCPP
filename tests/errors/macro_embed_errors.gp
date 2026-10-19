// phase: 10

// Mistakes in the use of #embed (Phase 10 step 10.8): the file is named by one string literal, a file of
// the source file's folder or below it.

package main

import "fmt"

func main() {
	fmt.Println(#embed())                       // ERROR "#embed takes one argument, a string literal"
	fmt.Println(#embed("a", "b"))               // ERROR "#embed takes one argument, a string literal"
	name := "embeddata/x.txt"
	fmt.Println(#embed(name))                   // ERROR "#embed takes one argument, a string literal"
	fmt.Println(#embed("embeddata/missing.txt")) // ERROR "#embed embeddata/missing.txt: no matching files found"
	fmt.Println(#embed("../x.txt"))             // ERROR "#embed ../x.txt: invalid file name"
	fmt.Println(#embed("/etc/passwd"))          // ERROR "#embed /etc/passwd: invalid file name"
	fmt.Println(#embed("embeddata"))            // ERROR "#embed embeddata: a folder can't be embedded as a string"
	fmt.Println(#embed("embeddata/./x.txt"))    // ERROR "invalid file name"
	fmt.Println(#embed("embeddata//x.txt"))     // ERROR "invalid file name"
	fmt.Println(#embed(`a\b`))                  // ERROR "no matching files found"
	fmt.Println(#embed("embeddata/x.*"))        // ERROR "no matching files found"
}
