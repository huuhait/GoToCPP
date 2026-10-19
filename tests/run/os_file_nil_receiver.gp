// phase: 09

package main

import (
	"fmt"
	"os"
)

func main() {
	var f *os.File
	buf := make([]byte, 1)
	n, err := f.Read(buf)
	fmt.Println(n, err)
	n, err = f.Write(buf)
	fmt.Println(n, err)
	n, err = f.WriteString("x")
	fmt.Println(n, err)
	_, err = f.Stat()
	fmt.Println(err)
	_, err = f.Seek(0, 0)
	fmt.Println(err)
	fmt.Println(f.Close())
	fmt.Println(f.Sync())
	fmt.Println(f.Truncate(0))
	fmt.Println(f.Chmod(0))
	_, err = f.ReadDir(1)
	fmt.Println(err)
	_, err = f.Readdirnames(1)
	fmt.Println(err)
	fmt.Println(f.Chdir())
	fmt.Println(f.Name == nil)
}

// Output:
// 0 invalid argument
// 0 invalid argument
// 0 invalid argument
// invalid argument
// invalid argument
// invalid argument
// invalid argument
// invalid argument
// invalid argument
// invalid argument
// invalid argument
// invalid argument
// false
