// phase: 09

package main

import (
	"fmt"
	"os"
)

// More output than any pipe holds, written through package os (WriteFile of Go's poll, in one call of
// 1.7 MB and then line by line), and mixed with fmt: the order is kept and nothing is lost.
func main() {
	var buf []byte
	for i := 0; i < 20000; i++ {
		buf = append(buf, fmt.Sprintf("line %d of a large output written through package os, longer than any pipe buffer\n", i)...)
	}
	n, err := os.Stdout.Write(buf)
	if err != nil || n != len(buf) {
		fmt.Println("short write", n, err)
		os.Exit(1)
	}
	for i := 20000; i < 20100; i++ {
		os.Stdout.WriteString(fmt.Sprintf("line %d via WriteString\n", i))
		fmt.Printf("line %d via fmt\n", i)
	}
	fmt.Fprintln(os.Stdout, "end")
}
