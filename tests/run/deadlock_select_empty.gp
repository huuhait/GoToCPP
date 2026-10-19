// phase: 08
// ExitCode: 2
// Stderr: ^fatal error: all goroutines are asleep - deadlock!\n\ngoroutine 1 \[select \(no cases\)\]:\nmain\.main\(\)\n\t.*:\d+ \+0x[0-9a-f]+\n$

// select {} with no other goroutine.
package main

import "fmt"

func main() {
	fmt.Println("blocking")
	select {}
}

// Output:
// blocking
