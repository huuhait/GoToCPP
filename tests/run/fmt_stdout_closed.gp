// phase: 09
// Stderr: ^struct copy: 34 <nil> "into the pipe via \*os.Stdout = \*w\\nfprintln too\\n"\nfd=\d+ n=0 err=write /dev/stdout: file already closed file=""\n$

// fmt.Println after the original os.Stdout is closed returns Go's error and writes nothing: the
// descriptor 1 that the next file gets (os.CreateTemp) must not receive the text. Also a copy of
// the contents of a file onto os.Stdout (*os.Stdout = *w) redirects fmt.Print*.
package main

import (
	"fmt"
	"io"
	"os"
)

func main() {
	r, w, _ := os.Pipe()
	saved := *os.Stdout
	*os.Stdout = *w
	n, err := fmt.Println("into the pipe via *os.Stdout = *w")
	fmt.Fprintln(os.Stdout, "fprintln too")
	*os.Stdout = saved
	w.Close()
	b, _ := io.ReadAll(r)
	fmt.Fprintf(os.Stderr, "struct copy: %d %v %q\n", n, err, b)

	os.Stdout.Close()
	f, err := os.CreateTemp("", "reuse")
	if err != nil {
		panic(err)
	}
	defer os.Remove(f.Name())
	n, err = fmt.Println("SECRET written after stdout closed")
	b, _ = os.ReadFile(f.Name())
	fmt.Fprintf(os.Stderr, "fd=%d n=%d err=%v file=%q\n", f.Fd(), n, err, b)
}

// Output:
