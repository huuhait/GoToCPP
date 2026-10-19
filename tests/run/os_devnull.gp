// phase: 09

package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Println(os.DevNull)
	f, err := os.OpenFile(os.DevNull, os.O_RDWR, 0)
	fmt.Println(err)
	n, err := f.Write([]byte("discard me"))
	fmt.Println(n, err)
	buf := make([]byte, 4)
	n, err = f.Read(buf)
	fmt.Println(n, err)
	st, _ := f.Stat()
	fmt.Println(st.Mode()&os.ModeDevice != 0, st.Mode()&os.ModeCharDevice != 0, st.Mode().IsRegular())
	fmt.Println(st.Mode().Type() == os.ModeDevice|os.ModeCharDevice)
	f.Close()
}

// Output:
// /dev/null
// <nil>
// 10 <nil>
// 0 EOF
// true true false
// true
