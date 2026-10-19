// phase: 08
// ExitCode: 2
// Stderr: ^panic: sync: negative WaitGroup counter\n\ngoroutine 1 \[running\]:\n

// An unrecovered negative counter ends the program with the panic and exit code 2.
package main

import (
	"fmt"
	"sync"
)

func main() {
	var wg sync.WaitGroup
	fmt.Println("start")
	wg.Add(1)
	wg.Done()
	wg.Done()
	fmt.Println("not reached")
}

// Output:
// start
