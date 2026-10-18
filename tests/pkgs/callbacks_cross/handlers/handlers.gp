package handlers

import (
	"fmt"
	"unsafe"
)

type Tally struct {
	Names []string
	Bytes int
}

// Visit is a callback for C: only Go types in its signature (int32 is C's int).
func Visit(user unsafe.Pointer, i int32) int32 {
	t := (*Tally)(user)
	t.Bytes += int(i)
	return i * i
}

func Square(i int32) int32 { return i * i }

func Report(t *Tally) { fmt.Println("bytes", t.Bytes) }

func OnOpen(name *C.char) { fmt.Println("open", C.GoString(name)) }

func OnData(user unsafe.Pointer, data *C.char, n int32) int32 {
	t := (*Tally)(user)
	t.Names = append(t.Names, C.GoStringN(data, C.int(n)))
	return int32(len(t.Names))
}
