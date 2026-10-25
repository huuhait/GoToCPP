// Command gclayout prints the real Go sizes, alignments and field offsets that
// internal/check/gcsizes.go (gcLayoutSrc) and the generated runtime/test/gc_layout_test.cpp
// assert (phase-16 README §2.2; job 16.1e). It is testdata, not part of a package: run it with
//
//	go run ./internal/check/testdata/gclayout
//
// Its output was the source of the numbers in gcLayoutSrc; TestGCLayoutShapes checks the same
// numbers at test time with reflect and unsafe (on amd64), so this program is for a person
// reading where a number came from, not for the test.
package main

import (
	"fmt"
	"log/slog"
	"reflect"
	"sync"
	"sync/atomic"
	"unsafe"
	"weak"
)

type empty struct{}

type zLast struct {
	A int64
	Z empty
}

type zFirst struct {
	Z empty
	A int64
}

type zz struct {
	Z1 empty
	Z2 empty
}

type arrZLast struct {
	A int32
	Z [0]int64
}

type arrZFirst struct {
	Z [0]int64
	A int32
}

type arrZMid struct {
	A int32
	Z [0]int64
	B int32
}

type zeroSizeNamed struct {
	z [0]int64
}

type nestedZero struct {
	A int32
	Z zeroSizeNamed
	B int32
}

// print prints a struct's Go layout: size, alignment and every field's offset.
func print(name string, v any) {
	t := reflect.TypeOf(v)
	fmt.Printf("%s size=%d align=%d\n", name, t.Size(), t.Align())
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		fmt.Printf("  %-10s offset=%3d size=%3d align=%d type=%s\n",
			f.Name, f.Offset, f.Type.Size(), f.Type.Align(), f.Type)
	}
}

func main() {
	fmt.Println("== the zero-size shapes")
	print("empty", empty{})
	print("zLast", zLast{})
	print("zFirst", zFirst{})
	print("zz", zz{})
	print("arrZLast", arrZLast{})
	print("arrZFirst", arrZFirst{})
	print("arrZMid", arrZMid{})
	print("nestedZero", nestedZero{})

	fmt.Println("== sync")
	print("sync.Mutex", sync.Mutex{})
	print("sync.WaitGroup", sync.WaitGroup{})
	print("sync.RWMutex", sync.RWMutex{})
	print("sync.Once", sync.Once{})
	print("sync.Pool", sync.Pool{})

	fmt.Println("== sync/atomic")
	print("atomic.Pointer[int]", atomic.Pointer[int]{})
	print("atomic.Int64", atomic.Int64{})

	fmt.Println("== weak")
	print("weak.Pointer[int]", weak.Pointer[int]{})

	fmt.Println("== log/slog")
	print("slog.Value", slog.Value{})
	print("slog.Attr", slog.Attr{})

	// The sizes the table in internal/check/gcsizes.go uses, from unsafe directly.
	fmt.Println("== the GC engine's values")
	var iface interface{ M() }
	var fn func()
	var m map[int]int
	var c chan int
	fmt.Printf("string=%d slice=%d iface=%d func=%d map=%d chan=%d\n",
		unsafe.Sizeof(""), unsafe.Sizeof([]int(nil)), unsafe.Sizeof(iface), unsafe.Sizeof(fn),
		unsafe.Sizeof(m), unsafe.Sizeof(c))

	// Append's capacities are real Go's runtime.growslice, not the compiler's inline append
	// (which may allocate more): reflect calls growslice. runtime/test/gc_values_test.cpp uses
	// these numbers.
	fmt.Println("== append growth (runtime.growslice, through reflect)")
	growth("int64", reflect.TypeOf(int64(0)), 40)
	growth("string", reflect.TypeOf(""), 40)
	growth("byte", reflect.TypeOf(byte(0)), 40)
	growth("struct{}", reflect.TypeOf(struct{}{}), 5)
	// A slice that reaches 600 elements one at a time (its cap at 600 is 848).
	v := reflect.MakeSlice(reflect.SliceOf(reflect.TypeOf(int64(0))), 0, 0)
	for i := 0; i < 600; i++ {
		v = reflect.Append(v, reflect.Zero(reflect.TypeOf(int64(0))))
	}
	fmt.Printf("int64 to 600: %d:%d\n", v.Len(), v.Cap())
	// append(s, t...) with 20 elements at once.
	w := reflect.AppendSlice(reflect.MakeSlice(reflect.SliceOf(reflect.TypeOf(int64(0))), 0, 0),
		reflect.MakeSlice(reflect.SliceOf(reflect.TypeOf(int64(0))), 20, 20))
	fmt.Printf("int64 append 20: %d:%d\n", w.Len(), w.Cap())
}

// growth prints the lengths at which the capacity of a slice grown one element at a time
// changes, as n:cap.
func growth(name string, elem reflect.Type, n int) {
	t := reflect.SliceOf(elem)
	v := reflect.MakeSlice(t, 0, 0)
	prev := 0
	fmt.Printf("%s:", name)
	for i := 0; i < n; i++ {
		v = reflect.Append(v, reflect.Zero(elem))
		if v.Cap() != prev {
			fmt.Printf(" %d:%d", v.Len(), v.Cap())
			prev = v.Cap()
		}
	}
	fmt.Println()
}
