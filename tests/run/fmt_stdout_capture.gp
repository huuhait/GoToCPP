// phase: 09

// fmt.Print, Println and Printf write to the file that the variable os.Stdout holds, so a program
// (or a test) can capture them by assigning a pipe or a file to os.Stdout; Fprint to os.Stdout and
// the builtin println (standard error) behave as in Go; a closed os.Stdout gives an error.
package main

import (
	"fmt"
	"io"
	"os"
)

func capture(f func()) string {
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		panic(err)
	}
	os.Stdout = w
	f()
	w.Close()
	os.Stdout = old
	b, _ := io.ReadAll(r)
	return string(b)
}

func main() {
	out := capture(func() {
		fmt.Println("captured", 1)
		fmt.Printf("%d\n", 2)
		fmt.Print("x\n")
		fmt.Fprintln(os.Stdout, "viaFprintln")
	})
	fmt.Printf("%q\n", out)

	// Nested captures: the inner one gives back the outer one's file.
	outer := capture(func() {
		fmt.Println("outer 1")
		inner := capture(func() { fmt.Println("inner") })
		fmt.Printf("got %q\n", inner)
	})
	fmt.Printf("%q\n", outer)

	f, err := os.CreateTemp("", "capture")
	if err != nil {
		panic(err)
	}
	defer os.Remove(f.Name())
	old := os.Stdout
	os.Stdout = f
	n, err := fmt.Println("to file", 12)
	os.Stdout = old
	fmt.Println(n, err)
	f.Seek(0, 0)
	b, _ := io.ReadAll(f)
	fmt.Printf("%q\n", b)

	// A closed file: Println reports the error, as Go does.
	r, w, _ := os.Pipe()
	w.Close()
	os.Stdout = w
	n, err = fmt.Println("lost")
	os.Stdout = old
	fmt.Println(n, err)
	r.Close()
	fmt.Println("done")
}

// Output:
// "captured 1\n2\nx\nviaFprintln\n"
// "outer 1\ngot \"inner\\n\"\n"
// 11 <nil>
// "to file 12\n"
// 0 write |1: file already closed
// done
