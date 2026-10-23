// phase: 09
//go:build linux

// syscall.Getcwd returns n = -1 with its error, as Go's raw system call does (ERANGE for a buffer
// that is too small), and syscall.Uname fills every field of Utsname, not only Nodename; Open and
// Readlink of a path with a NUL byte return 0 with EINVAL (dev.syscalldiffs).

package main

import (
	"fmt"
	"os"
	"syscall"
)

func str(b [65]int8) string {
	s := ""
	for _, c := range b {
		if c == 0 {
			break
		}
		s += string(rune(byte(c)))
	}
	return s
}

func main() {
	for _, size := range []int{0, 1, 2} {
		n, err := syscall.Getcwd(make([]byte, size))
		fmt.Println(size, n, err)
	}
	buf := make([]byte, 4096)
	n, err := syscall.Getcwd(buf)
	wd, _ := os.Getwd()
	fmt.Println(n == len(wd)+1, err, buf[n-1] == 0, string(buf[:n-1]) == wd)

	var u syscall.Utsname
	fmt.Println(syscall.Uname(&u))
	fmt.Printf("%q %q\n", str(u.Sysname), str(u.Machine))
	fmt.Printf("%q\n%q\n%q\n", str(u.Release), str(u.Version), str(u.Domainname))
	host, err := os.Hostname()
	fmt.Println(host == str(u.Nodename), host != "", err)

	// A path with a NUL byte fails before the system call: the count is 0, not -1. A missing
	// path fails in the system call: -1.
	fd, err := syscall.Open("a\x00b", syscall.O_RDONLY, 0)
	fmt.Println(fd, err)
	n, err = syscall.Readlink("a\x00b", buf)
	fmt.Println(n, err)
	fd, err = syscall.Open("/nonexistent/x", syscall.O_RDONLY, 0)
	fmt.Println(fd, err)
	n, err = syscall.Readlink("/nonexistent/x", buf)
	fmt.Println(n, err)
}
