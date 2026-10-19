package main

import (
	"fmt"

	"example.com/initfail/boot"
)

func main() {
	fmt.Println("main must not run", boot.ErrBoot)
}
