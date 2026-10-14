//go:build !linux

package main

import "syscall"

// dieOf does nothing outside Linux (see die_linux.go): main exits with
// 128 plus the signal's number, like a shell.
func dieOf(syscall.Signal) {}
