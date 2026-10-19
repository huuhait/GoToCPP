// phase: 09
// ExitCode: 3

package main

import (
	"fmt"
	"os"
)

func main() {
	defer fmt.Println("deferred: must not run")
	fmt.Println("before exit")
	os.Exit(3)
	fmt.Println("after exit: must not run")
}

// Output:
// before exit
