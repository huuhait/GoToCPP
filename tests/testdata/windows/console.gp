// phase: 09

package main

import (
	"fmt"
	"os"
)

// The standard handles are a console (ssh -tt gives the program one): Stat says it is a character
// device, text in UTF-8 comes out right through fmt and through os.Stdout (WriteConsoleW), and
// reading the console gives UTF-8 (ReadConsoleW).
func main() {
	fi, err := os.Stdout.Stat()
	fmt.Println("console:", err, fi.Mode()&os.ModeCharDevice != 0)
	fmt.Println("fmt: héllo 世界 \U0001F642")
	os.Stdout.WriteString("os: héllo 世界 \U0001F642\n")
	os.Stderr.WriteString("err: é\n")
	os.Stdout.WriteString("type: ")
	buf := make([]byte, 100)
	n, err := os.Stdin.Read(buf)
	fmt.Printf("read %d %v %q\n", n, err, buf[:n])
}
