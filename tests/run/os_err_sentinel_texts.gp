// phase: 09

package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Println(os.ErrInvalid)
	fmt.Println(os.ErrPermission)
	fmt.Println(os.ErrExist)
	fmt.Println(os.ErrNotExist)
	fmt.Println(os.ErrClosed)
	fmt.Println(os.ErrNoDeadline)
	fmt.Println(os.ErrDeadlineExceeded)
	fmt.Println(os.ErrProcessDone)
	fmt.Printf("%T %T\n", os.ErrDeadlineExceeded, os.ErrClosed)
	var te interface{ Timeout() bool }
	te, ok := os.ErrDeadlineExceeded.(interface{ Timeout() bool })
	fmt.Println(ok, te.Timeout())
	fmt.Println(os.IsTimeout(os.ErrDeadlineExceeded), os.IsTimeout(os.ErrClosed))
}

// Output:
// invalid argument
// permission denied
// file already exists
// file does not exist
// file already closed
// file type does not support deadline
// i/o timeout
// os: process already finished
// *poll.DeadlineExceededError *errors.errorString
// true true
// true false
