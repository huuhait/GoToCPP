// phase: 09

package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Println(len(os.Args))
	fmt.Println(len(os.Args[0]) > 0)
	os.Args = append(os.Args, "extra")
	fmt.Println(len(os.Args), os.Args[len(os.Args)-1])
	args := os.Args[1:]
	fmt.Println(len(args))
}

// Output:
// 1
// true
// 2 extra
// 1
