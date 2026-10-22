// phase: 12
// error: wcstol is given the address of a wchar_t pointer .*use C.WString\(text\)

// The address of a wide character pointer next to a wide string: wcstol stores a pointer into the string.
package main

import (
	"fmt"

	"<cwchar>"
)

func main() {
	var end *C.wchar_t
	text := "12" + fmt.Sprint(3)
	fmt.Println(cwchar.wcstol(text, &end, 10))
}
