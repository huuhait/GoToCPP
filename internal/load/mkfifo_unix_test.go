//go:build unix

package load

import "syscall"

func mkfifo(path string) error { return syscall.Mkfifo(path, 0o644) }
