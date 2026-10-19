// phase: 09

package main

import (
	"fmt"
	"io"
	"os"
)

func main() {
	b, err := io.ReadAll(os.Stdin)
	fmt.Println(len(b), err)
	n, err := os.Stdin.Read(make([]byte, 4))
	fmt.Println(n, err == io.EOF)
}

// Output:
// 0 <nil>
// 0 true
