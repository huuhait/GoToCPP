// phase: 12
// error: strstr returns a pointer into its argument

// The result of strstr points into the converted string, which lives for the statement only.
package main

import (
	"fmt"

	"<cstring>"
)

func main() {
	s := "hello " + fmt.Sprint(1)
	p := cstring.strstr(s, "o")
	fmt.Println(p != nil)
}
