// phase: 09

package main

import (
	"fmt"
	"os"
)

func main() {
	r, w, err := os.Pipe()
	fmt.Println(err)
	r.Close()
	_, err = r.Read(make([]byte, 1))
	fmt.Println(err != nil, os.IsNotExist(err))
	fmt.Println(err.(*os.PathError).Err == os.ErrClosed, err.(*os.PathError).Op)
	w.Close()
	fmt.Println(w.Close() != nil)
}

// Output:
// <nil>
// true false
// true read
// true
