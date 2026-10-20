// phase: 09

package main

import (
	"fmt"
	"io"
	"os"
)

func main() {
	r, w, _ := os.Pipe()
	old := os.Stdout
	var x int
	os.Stdout, x = w, 1
	fmt.Println("captured", x)
	os.Stdout, x = old, 2
	w.Close()
	b, _ := io.ReadAll(r)
	fmt.Printf("%q %d\n", b, x)
	// the same for os.Stdin
	r2, w2, _ := os.Pipe()
	w2.WriteString("42\n")
	w2.Close()
	oldIn := os.Stdin
	os.Stdin, x = r2, 3
	var y int
	n, err := fmt.Scan(&y)
	os.Stdin = oldIn
	fmt.Println(n, err, y, x)
}

// Output:
// "captured 1\n" 2
// 1 <nil> 42 3
