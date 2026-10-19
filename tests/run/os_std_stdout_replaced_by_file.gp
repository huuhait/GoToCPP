// phase: 09

package main

import (
	"fmt"
	"os"
)

func main() {
	d, _ := os.MkdirTemp("", "gpos")
	defer os.RemoveAll(d)
	f, _ := os.Create(d + "/out")
	old := os.Stdout
	os.Stdout = f
	fmt.Fprintln(os.Stdout, "into the file")
	os.Stdout = old
	f.Close()
	b, _ := os.ReadFile(d + "/out")
	fmt.Printf("%q\n", b)
	fmt.Println("restored")
}

// Output:
// "into the file\n"
// restored
