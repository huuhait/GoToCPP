// phase: 12
// error: wcschr returns a pointer into its argument

// The result of wcschr points into the converted string, which lives for the statement only.
package main

import (
	"fmt"

	"<cwchar>"
)

func main() {
	text := "hello " + fmt.Sprint(1)
	found := cwchar.wcschr(text, 'o')
	fmt.Println(found != nil)
}
