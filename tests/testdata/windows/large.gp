// phase: 06

package main

import "fmt"

func main() {
	for i := 0; i < 60000; i++ {
		fmt.Println("line", i, "of a large output that is longer than any pipe buffer")
	}
}
