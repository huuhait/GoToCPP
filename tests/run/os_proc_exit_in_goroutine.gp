// phase: 09
// ExitCode: 7

package main

import (
	"fmt"
	"os"
	"time"
)

func main() {
	go func() {
		fmt.Println("goroutine exits")
		os.Exit(7)
	}()
	time.Sleep(10 * time.Second)
	fmt.Println("main: must not get here")
}

// Output:
// goroutine exits
