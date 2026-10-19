// phase: 09

//go:build windows

package main

import (
	"fmt"
	"os"
)

func must(err error) {
	if err != nil {
		panic(err)
	}
}

func cur(f *os.File) int64 {
	off, err := f.Seek(0, 1)
	must(err)
	return off
}

// ReadAt and WriteAt do not move the file offset: on a Windows handle the system's ReadFile with an offset
// does move the file pointer, and package os puts it back.
func main() {
	dir, err := os.MkdirTemp("", "gpwin")
	must(err)
	defer os.RemoveAll(dir)
	f, err := os.Create(dir + `\f.txt`)
	must(err)
	defer f.Close()
	_, err = f.WriteString("0123456789")
	must(err)
	fmt.Println("after write:", cur(f))
	_, err = f.Seek(5, 0)
	must(err)

	buf := make([]byte, 3)
	n, err := f.ReadAt(buf, 0)
	fmt.Println(n, err, string(buf), "off", cur(f))
	n, err = f.WriteAt([]byte("AB"), 8)
	fmt.Println(n, err, "off", cur(f))
	n, err = f.ReadAt(buf, 9)
	fmt.Println(n, err, string(buf[:n]), "off", cur(f))
	n, err = f.WriteAt([]byte("Z"), 12)
	fmt.Println(n, err, "off", cur(f))

	// Sequential reads and writes go on from the offset.
	n, err = f.Read(buf[:2])
	fmt.Println(n, err, string(buf[:2]), "off", cur(f))
	n, err = f.Write([]byte("xy"))
	fmt.Println(n, err, "off", cur(f))
	_, err = f.Seek(0, 0)
	must(err)
	all := make([]byte, 20)
	n, err = f.Read(all)
	fmt.Printf("%d %v %q\n", n, err, all[:n])

	// An error of ReadAt does not move it either.
	_, err = f.Seek(2, 0)
	must(err)
	_, err = f.ReadAt(buf, -1)
	fmt.Println(err != nil, "off", cur(f))
	_, err = f.ReadAt(buf, 100)
	fmt.Println(err, "off", cur(f))
}

// Output:
// after write: 10
// 3 <nil> 012 off 5
// 2 <nil> off 5
// 1 EOF B off 5
// 1 <nil> off 5
// 2 <nil> 56 off 7
// 2 <nil> off 9
// 13 <nil> "0123456xyB\x00\x00Z"
// true off 2
// EOF off 2
