//go:build !linux

package interop

// crashBody is the body of a fake compiler that dies of SIGSEGV (a core dump is possible here:
// crash_linux_test.go has the version without).
func crashBody() string { return "kill -SEGV $$" }

func crashIfAsked() {}
