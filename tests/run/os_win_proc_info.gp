// phase: 09

//go:build windows

package main

import (
	"fmt"
	"os"
)

func lower(s string) string {
	b := []byte(s)
	for i, c := range b {
		if 'A' <= c && c <= 'Z' {
			b[i] = c + 32
		}
	}
	return string(b)
}

func hasSuffix(s, suf string) bool { return len(s) >= len(suf) && s[len(s)-len(suf):] == suf }

func e(err error) string {
	if err == nil {
		return "<nil>"
	}
	return err.Error()
}

func main() {
	exe, err := os.Executable()
	fmt.Println(err, hasSuffix(lower(exe), ".exe"), len(exe) > 3 && exe[1] == ':')
	fi, err := os.Stat(exe)
	fmt.Println(err, fi.Mode().IsRegular(), fi.Size() > 100000)
	fmt.Println(len(os.Args) >= 1, hasSuffix(lower(os.Args[0]), ".exe"))
	h, err := os.Hostname()
	fmt.Println(err, len(h) > 0)
	pid := os.Getpid()
	fmt.Println(pid > 0, os.Getppid() > 0, os.Getppid() != pid)
	fmt.Println(os.Getuid(), os.Geteuid(), os.Getgid(), os.Getegid(), os.Getpagesize())

	p, err := os.FindProcess(pid)
	fmt.Println(err, p.Pid == pid)
	fmt.Println(e(p.Signal(os.Interrupt)))
	fmt.Println(p.Release())
	fmt.Println(e(p.Release()))
	_, err = os.FindProcess(0x7ffffff0)
	fmt.Println(e(err))
	// StartProcess is not built in gocpp.
	_, err = os.StartProcess(`C:\Windows\System32\cmd.exe`, []string{"cmd"}, &os.ProcAttr{})
	fmt.Println(err != nil)
}

// Output:
// <nil> true true
// <nil> true true
// true true
// <nil> true
// true true true
// -1 -1 -1 -1 4096
// <nil> true
// not supported by windows
// <nil>
// invalid argument
// OpenProcess: The parameter is incorrect.
// true
