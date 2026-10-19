// phase: 09

// The standard library's growth that rests on append and make gives Go's capacities too:
// slices.Grow, Insert, Clone and Concat, strings.Builder, bytes.Buffer, reflect.Append,
// strings.Fields and Split, fmt.Appendf.
package main

import (
	"bytes"
	"fmt"
	"os"
	"reflect"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

var sink any

// growth appends n zero values of T one at a time and prints the distinct capacities. T is a
// standard type whose gocpp declaration has other fields than Go's (sync.Cond, time.Timer): the
// element size must be Go's.
func growth[T any](name string, n int) {
	var s []T
	var zero T
	last := -1
	out := name + ":"
	for i := 0; i < n; i++ {
		s = append(s, zero)
		if cap(s) != last {
			last = cap(s)
			out += fmt.Sprint(" ", last)
		}
	}
	sink = s
	fmt.Println(out)
}

func reflectGrowth[T any](name string) {
	var xs []T
	rv := reflect.ValueOf(&xs).Elem()
	zero := reflect.Zero(rv.Type().Elem())
	last := -1
	out := name + ":"
	for i := 0; i < 700; i++ {
		rv.Set(reflect.Append(rv, zero))
		if rv.Cap() != last {
			last = rv.Cap()
			out += fmt.Sprint(" ", last)
		}
	}
	sink = xs
	fmt.Println(out)
}

func main() {
	g := slices.Grow([]int{1, 2, 3}, 10)
	sink = g
	fmt.Println("grow", len(g), cap(g))
	for _, n := range []int{0, 1, 5, 40, 600} {
		var x []string
		x = slices.Grow(x, n)
		sink = x
		var y []byte
		y = slices.Grow(y, n)
		sink = y
		fmt.Print(" ", cap(x), "/", cap(y))
	}
	fmt.Println()
	ins := slices.Insert([]string{"a", "b", "c"}, 1, "x", "y", "z")
	sink = ins
	cl := slices.Clone([]any{1, 2, 3, 4, 5})
	sink = cl
	cc := slices.Concat([]byte("abc"), []byte("defgh"), []byte{1})
	sink = cc
	fmt.Println("insert", len(ins), cap(ins), "clone", len(cl), cap(cl), "concat", len(cc), cap(cc))

	var sb strings.Builder
	last := -1
	out := "builder:"
	for i := 0; i < 3000; i++ {
		sb.WriteString("ab")
		if sb.Cap() != last {
			last = sb.Cap()
			out += fmt.Sprint(" ", last)
		}
	}
	fmt.Println(out)
	var sb2 strings.Builder
	sb2.Grow(10)
	fmt.Println("builder grow", sb2.Cap())
	sb2.WriteString("hello world, hello")
	fmt.Println("builder grown", sb2.Len(), sb2.Cap())

	var buf bytes.Buffer
	last = -1
	out = "buffer:"
	for i := 0; i < 3000; i++ {
		buf.WriteString("abc")
		if buf.Cap() != last {
			last = buf.Cap()
			out += fmt.Sprint(" ", last)
		}
	}
	fmt.Println(out)
	var b2 bytes.Buffer
	b2.Grow(100)
	fmt.Println("buffer grow", b2.Cap())
	b2.Write(make([]byte, 300))
	b2.WriteByte('x')
	fmt.Println("buffer grown", b2.Len(), b2.Cap())

	growth[time.Timer]("time.Timer", 3000)
	growth[time.Ticker]("time.Ticker", 3000)
	growth[time.Time]("time.Time", 3000)
	growth[time.Duration]("time.Duration", 3000)
	growth[sync.Mutex]("sync.Mutex", 3000)
	growth[sync.RWMutex]("sync.RWMutex", 3000)
	growth[sync.WaitGroup]("sync.WaitGroup", 3000)
	growth[sync.Once]("sync.Once", 3000)
	growth[sync.Map]("sync.Map", 3000)
	growth[sync.Cond]("sync.Cond", 3000)
	growth[sync.Pool]("sync.Pool", 3000)
	growth[atomic.Int32]("atomic.Int32", 3000)
	growth[atomic.Int64]("atomic.Int64", 3000)
	growth[atomic.Uint32]("atomic.Uint32", 3000)
	growth[atomic.Bool]("atomic.Bool", 3000)
	growth[atomic.Value]("atomic.Value", 3000)
	growth[atomic.Pointer[int]]("atomic.Pointer", 3000)
	growth[os.File]("os.File", 3000)
	growth[strings.Builder]("strings.Builder", 3000)
	growth[bytes.Buffer]("bytes.Buffer", 3000)
	growth[reflect.Value]("reflect.Value", 3000)
	growth[reflect.StructField]("reflect.StructField", 3000)
	reflectGrowth[string]("reflect string")
	reflectGrowth[int]("reflect int")
	reflectGrowth[any]("reflect any")
	reflectGrowth[[]byte]("reflect []byte")
	reflectGrowth[struct{ a int8 }]("reflect struct")
	rb := reflect.AppendSlice(reflect.ValueOf([]string(nil)), reflect.ValueOf(make([]string, 33)))
	sink = rb.Interface()
	fmt.Println("reflect appendslice", rb.Len(), rb.Cap())

	fs := strings.Fields(strings.Repeat("ab cd ", 40))
	sink = fs
	sp := strings.Split(strings.Repeat("a,", 30), ",")
	sink = sp
	fmt.Println("fields", len(fs), cap(fs), "split", len(sp), cap(sp))
	ap := fmt.Appendf(nil, "%d-%s", 42, "answer")
	sink = ap
	fmt.Println("appendf", len(ap), cap(ap))
}

// Output:
// grow 3 14
//  0/0 1/8 5/8 43/48 607/640
// insert 6 6 clone 5 5 concat 9 16
// builder: 8 16 32 64 128 256 512 896 1408 2048 3072 4096 5376 6912
// builder grow 16
// builder grown 18 32
// buffer: 64 128 256 512 1024 2048 4096 8192 16384
// buffer grow 112
// buffer grown 301 320
// time.Timer: 1 2 4 8 16 32 71 143 303 591 1023 1535 2560 3584
// time.Ticker: 1 2 4 8 16 32 71 143 303 591 1023 1535 2560 3584
// time.Time: 1 2 4 8 16 37 74 170 341 682 1135 1706 2389 3413
// time.Duration: 1 2 4 8 16 32 64 128 256 512 848 1280 1792 2560 3408
// sync.Mutex: 1 2 4 8 16 32 64 128 256 512 848 1280 1792 2560 3408
// sync.RWMutex: 1 2 4 8 16 32 64 128 256 512 853 1365 2048 3072
// sync.WaitGroup: 1 2 4 8 16 32 64 128 256 512 848 1280 1792 2560 3584
// sync.Once: 1 2 4 8 16 32 64 128 256 512 853 1365 2048 3413
// sync.Map: 1 2 4 8 18 37 85 170 341 682 1194 1706 2389 3242
// sync.Cond: 1 2 4 8 18 36 73 146 292 585 1024 1609 2340 3218
// sync.Pool: 1 2 4 8 17 35 76 153 307 614 1024 1638 2252 3072
// atomic.Int32: 2 4 8 16 32 64 128 256 512 864 1344 2048 3072
// atomic.Int64: 1 2 4 8 16 32 64 128 256 512 848 1280 1792 2560 3408
// atomic.Uint32: 2 4 8 16 32 64 128 256 512 864 1344 2048 3072
// atomic.Bool: 2 4 8 16 32 64 128 256 512 864 1344 2048 3072
// atomic.Value: 1 2 4 8 16 32 71 143 303 591 1023 1535 2560 3584
// atomic.Pointer: 1 2 4 8 16 32 64 143 287 607 1023 1535 2303 3071
// os.File: 1 2 4 8 16 32 64 143 287 607 1023 1535 2303 3071
// strings.Builder: 1 2 4 8 16 35 71 151 303 575 1023 1536 2304 3072
// bytes.Buffer: 1 2 4 8 17 35 76 153 307 614 1024 1638 2252 3072
// reflect.Value: 1 2 4 8 16 37 74 170 341 682 1135 1706 2389 3413
// reflect.StructField: 1 2 4 8 17 39 78 157 315 630 1024 1496 2126 2914 3859
// reflect string: 1 2 4 8 16 32 71 143 303 591 1023
// reflect int: 1 2 4 8 16 32 64 128 256 512 848
// reflect any: 1 2 4 8 16 32 71 143 303 591 1023
// reflect []byte: 1 2 4 8 16 37 74 170 341 682 1135
// reflect struct: 8 16 32 64 128 256 512 896
// reflect appendslice 33 35
// fields 80 80 split 31 31
// appendf 9 16
