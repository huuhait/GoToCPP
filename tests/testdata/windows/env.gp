// phase: 06

package main

import (
	"fmt"

	"<stdlib.h>"
)

func main() {
	v := stdlib.getenv("GOCPP_WIN_VAR")
	if v == nil {
		fmt.Println("unset")
		return
	}
	fmt.Println(C.GoString(v))
}
