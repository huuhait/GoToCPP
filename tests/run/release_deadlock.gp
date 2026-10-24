// phase: 08
// flags: -release
// ExitCode: 2
// Stderr: ^fatal error: all goroutines are asleep - deadlock!\n$

// A -release program: the deadlock report is its line only, without the goroutines.
package main

func worker(c chan int) {
	c <- 1
	c <- 2
}

func main() {
	c := make(chan int)
	go worker(c)
	<-c
	select {}
}

// Output:
