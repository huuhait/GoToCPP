// phase: 12
// error: set may keep the pointer that it is given .*copy the code units of title into memory from C.malloc

// char16_t has no C.WString: the advice for a kept UTF-16 string is a copy in memory from C.malloc,
// and not a helper that makes wchar_t.
package main

import (
	"fmt"

	"./adapt.hpp"
)

func main() {
	var label adapt.Label16
	title := "t" + fmt.Sprint(1)
	label.set(title)
	fmt.Println(label.text != nil)
}
