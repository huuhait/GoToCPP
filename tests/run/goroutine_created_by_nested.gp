// phase: 08
// ExitCode: 2
// Stderr: ^panic: from the grandchild\n\ngoroutine \d+ \[running\]:\n(?:.+\n\t.+\n)+created by main\.[A-Za-z0-9_.]+ in goroutine \d+\n\t.*:\d+ \+0x[0-9a-f]+\n$

// "created by" names the function with the go statement (an optimized build may have inlined
// spawn into its caller), and the goroutine that ran it (not
// main's id when the go statement is in another goroutine).
package main

func spawn() {
	go func() {
		panic("from the grandchild")
	}()
}

func main() {
	done := make(chan struct{})
	go func() {
		spawn()
		<-done
	}()
	<-done
}

// Output:
