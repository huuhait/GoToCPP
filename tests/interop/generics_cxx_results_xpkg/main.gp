// phase: 08

// A method of a generic type of another package with the result of a deduced generic call in its body:
// the instance lib.Box[int] is made after both packages are lowered, by lib.
package main

import (
	"fmt"

	"example.com/gcrx/lib"

	"<string>"
)

func main() {
	words := []std.string{"a", "b"}
	fmt.Println(lib.Plain(words), lib.Box[int]{V: 1}.N(words), lib.Box[bool]{}.N(words))
}

// Output:
// 3 7 7
