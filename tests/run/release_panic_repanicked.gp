// phase: 08
// flags: -release
// ExitCode: 2
// Stderr: ^panic: same \[recovered, repanicked\]\n$

// A -release program: a deferred call recovers a panic and panics again with the same value,
// which Go prints as one line with "[recovered, repanicked]", and nothing else.
package main

func main() {
	defer func() {
		r := recover()
		panic(r)
	}()
	panic("same")
}

// Output:
