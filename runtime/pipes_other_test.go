//go:build !linux

package rtfiles_test

import "testing"

// runPipes checks pipe behavior that needs Linux system calls (pipes_linux_test.go).
func runPipes(t *testing.T, bin, goBin string) {
	t.Skip("the pipe tests use Linux system calls")
}
