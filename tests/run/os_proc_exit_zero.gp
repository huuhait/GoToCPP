// phase: 09

package main

import (
	"fmt"
	"os"
)

func main() {
	defer fmt.Println("deferred: must not run")
	fmt.Print("no newline at the end")
	os.Exit(0)
}

// Output:
// no newline at the end
