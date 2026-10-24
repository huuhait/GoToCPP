// phase: 08
// ExitCode: 2
// flags: -release
// Stderr: ^panic: runtime error: index out of range \[5\] with length 3\n$

// A -release program: the panic of a goroutine prints its message only, without the goroutine,
// its trace or "created by", while another goroutine waits on a channel.
package main

func waiter(c chan int) {
	<-c
}

func get(xs []int, i int) int {
	return xs[i]
}

func worker(i int, done chan int) {
	xs := []int{1, 2, 3}
	done <- get(xs, i)
}

func main() {
	go waiter(make(chan int))
	done := make(chan int)
	go worker(5, done)
	<-done
}

// Output:
