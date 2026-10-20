// phase: 09

package main

import (
	"fmt"
	"os"
)

func main() {
	saved := os.Stdout
	os.Stdout = nil
	n, err := fmt.Println("to nil")
	os.Stdout = saved
	fmt.Println(n, err)
	c := *os.Stdout
	os.Stdout = &c
	fmt.Println("via copy")
	os.Stdout = saved
}

// Output:
// 0 invalid argument
// via copy
