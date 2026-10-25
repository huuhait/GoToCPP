// phase: 09

// The descriptor of a composite type (a slice, map, array, pointer, channel or func type) is made at
// its first use, and goroutines that start at the same time race for it: the descriptors have one
// pointer each, whatever thread made it (gocpp/type.hpp desc_get). Every goroutine boxes the values
// of every type in an order of its own, after the same start signal, so each first use is a race
// (run it with -race and -asan: the first use must be free of data races, and every thread must see
// the same descriptor, so the type switches and %T agree).
package main

import (
	"fmt"
	"sort"
	"sync"
)

type S0 struct{ a, b int }
type S1 struct{ a, b int }
type S2 struct{ a, b int }
type S3 struct{ a, b int }
type S4 struct{ a, b int }
type S5 struct{ a, b int }
type S6 struct{ a, b int }
type S7 struct{ a, b int }
type S8 struct{ a, b int }
type S9 struct{ a, b int }
type S10 struct{ a, b int }
type S11 struct{ a, b int }
type S12 struct{ a, b int }
type S13 struct{ a, b int }
type S14 struct{ a, b int }
type S15 struct{ a, b int }
type S16 struct{ a, b int }
type S17 struct{ a, b int }
type S18 struct{ a, b int }
type S19 struct{ a, b int }
type S20 struct{ a, b int }
type S21 struct{ a, b int }
type S22 struct{ a, b int }
type S23 struct{ a, b int }

var makers = []func(i int) any{
	func(i int) any { return []S0{{i, i}} },
	func(i int) any { return map[string]S0{"k": {i, i}} },
	func(i int) any { return [3]S0{} },
	func(i int) any { return &S0{i, i} },
	func(i int) any { return make(chan S0, 1) },
	func(i int) any { return func() S0 { return S0{i, i} } },
	func(i int) any { return []S1{{i, i}} },
	func(i int) any { return map[string]S1{"k": {i, i}} },
	func(i int) any { return [3]S1{} },
	func(i int) any { return &S1{i, i} },
	func(i int) any { return make(chan S1, 1) },
	func(i int) any { return func() S1 { return S1{i, i} } },
	func(i int) any { return []S2{{i, i}} },
	func(i int) any { return map[string]S2{"k": {i, i}} },
	func(i int) any { return [3]S2{} },
	func(i int) any { return &S2{i, i} },
	func(i int) any { return make(chan S2, 1) },
	func(i int) any { return func() S2 { return S2{i, i} } },
	func(i int) any { return []S3{{i, i}} },
	func(i int) any { return map[string]S3{"k": {i, i}} },
	func(i int) any { return [3]S3{} },
	func(i int) any { return &S3{i, i} },
	func(i int) any { return make(chan S3, 1) },
	func(i int) any { return func() S3 { return S3{i, i} } },
	func(i int) any { return []S4{{i, i}} },
	func(i int) any { return map[string]S4{"k": {i, i}} },
	func(i int) any { return [3]S4{} },
	func(i int) any { return &S4{i, i} },
	func(i int) any { return make(chan S4, 1) },
	func(i int) any { return func() S4 { return S4{i, i} } },
	func(i int) any { return []S5{{i, i}} },
	func(i int) any { return map[string]S5{"k": {i, i}} },
	func(i int) any { return [3]S5{} },
	func(i int) any { return &S5{i, i} },
	func(i int) any { return make(chan S5, 1) },
	func(i int) any { return func() S5 { return S5{i, i} } },
	func(i int) any { return []S6{{i, i}} },
	func(i int) any { return map[string]S6{"k": {i, i}} },
	func(i int) any { return [3]S6{} },
	func(i int) any { return &S6{i, i} },
	func(i int) any { return make(chan S6, 1) },
	func(i int) any { return func() S6 { return S6{i, i} } },
	func(i int) any { return []S7{{i, i}} },
	func(i int) any { return map[string]S7{"k": {i, i}} },
	func(i int) any { return [3]S7{} },
	func(i int) any { return &S7{i, i} },
	func(i int) any { return make(chan S7, 1) },
	func(i int) any { return func() S7 { return S7{i, i} } },
	func(i int) any { return []S8{{i, i}} },
	func(i int) any { return map[string]S8{"k": {i, i}} },
	func(i int) any { return [3]S8{} },
	func(i int) any { return &S8{i, i} },
	func(i int) any { return make(chan S8, 1) },
	func(i int) any { return func() S8 { return S8{i, i} } },
	func(i int) any { return []S9{{i, i}} },
	func(i int) any { return map[string]S9{"k": {i, i}} },
	func(i int) any { return [3]S9{} },
	func(i int) any { return &S9{i, i} },
	func(i int) any { return make(chan S9, 1) },
	func(i int) any { return func() S9 { return S9{i, i} } },
	func(i int) any { return []S10{{i, i}} },
	func(i int) any { return map[string]S10{"k": {i, i}} },
	func(i int) any { return [3]S10{} },
	func(i int) any { return &S10{i, i} },
	func(i int) any { return make(chan S10, 1) },
	func(i int) any { return func() S10 { return S10{i, i} } },
	func(i int) any { return []S11{{i, i}} },
	func(i int) any { return map[string]S11{"k": {i, i}} },
	func(i int) any { return [3]S11{} },
	func(i int) any { return &S11{i, i} },
	func(i int) any { return make(chan S11, 1) },
	func(i int) any { return func() S11 { return S11{i, i} } },
	func(i int) any { return []S12{{i, i}} },
	func(i int) any { return map[string]S12{"k": {i, i}} },
	func(i int) any { return [3]S12{} },
	func(i int) any { return &S12{i, i} },
	func(i int) any { return make(chan S12, 1) },
	func(i int) any { return func() S12 { return S12{i, i} } },
	func(i int) any { return []S13{{i, i}} },
	func(i int) any { return map[string]S13{"k": {i, i}} },
	func(i int) any { return [3]S13{} },
	func(i int) any { return &S13{i, i} },
	func(i int) any { return make(chan S13, 1) },
	func(i int) any { return func() S13 { return S13{i, i} } },
	func(i int) any { return []S14{{i, i}} },
	func(i int) any { return map[string]S14{"k": {i, i}} },
	func(i int) any { return [3]S14{} },
	func(i int) any { return &S14{i, i} },
	func(i int) any { return make(chan S14, 1) },
	func(i int) any { return func() S14 { return S14{i, i} } },
	func(i int) any { return []S15{{i, i}} },
	func(i int) any { return map[string]S15{"k": {i, i}} },
	func(i int) any { return [3]S15{} },
	func(i int) any { return &S15{i, i} },
	func(i int) any { return make(chan S15, 1) },
	func(i int) any { return func() S15 { return S15{i, i} } },
	func(i int) any { return []S16{{i, i}} },
	func(i int) any { return map[string]S16{"k": {i, i}} },
	func(i int) any { return [3]S16{} },
	func(i int) any { return &S16{i, i} },
	func(i int) any { return make(chan S16, 1) },
	func(i int) any { return func() S16 { return S16{i, i} } },
	func(i int) any { return []S17{{i, i}} },
	func(i int) any { return map[string]S17{"k": {i, i}} },
	func(i int) any { return [3]S17{} },
	func(i int) any { return &S17{i, i} },
	func(i int) any { return make(chan S17, 1) },
	func(i int) any { return func() S17 { return S17{i, i} } },
	func(i int) any { return []S18{{i, i}} },
	func(i int) any { return map[string]S18{"k": {i, i}} },
	func(i int) any { return [3]S18{} },
	func(i int) any { return &S18{i, i} },
	func(i int) any { return make(chan S18, 1) },
	func(i int) any { return func() S18 { return S18{i, i} } },
	func(i int) any { return []S19{{i, i}} },
	func(i int) any { return map[string]S19{"k": {i, i}} },
	func(i int) any { return [3]S19{} },
	func(i int) any { return &S19{i, i} },
	func(i int) any { return make(chan S19, 1) },
	func(i int) any { return func() S19 { return S19{i, i} } },
	func(i int) any { return []S20{{i, i}} },
	func(i int) any { return map[string]S20{"k": {i, i}} },
	func(i int) any { return [3]S20{} },
	func(i int) any { return &S20{i, i} },
	func(i int) any { return make(chan S20, 1) },
	func(i int) any { return func() S20 { return S20{i, i} } },
	func(i int) any { return []S21{{i, i}} },
	func(i int) any { return map[string]S21{"k": {i, i}} },
	func(i int) any { return [3]S21{} },
	func(i int) any { return &S21{i, i} },
	func(i int) any { return make(chan S21, 1) },
	func(i int) any { return func() S21 { return S21{i, i} } },
	func(i int) any { return []S22{{i, i}} },
	func(i int) any { return map[string]S22{"k": {i, i}} },
	func(i int) any { return [3]S22{} },
	func(i int) any { return &S22{i, i} },
	func(i int) any { return make(chan S22, 1) },
	func(i int) any { return func() S22 { return S22{i, i} } },
	func(i int) any { return []S23{{i, i}} },
	func(i int) any { return map[string]S23{"k": {i, i}} },
	func(i int) any { return [3]S23{} },
	func(i int) any { return &S23{i, i} },
	func(i int) any { return make(chan S23, 1) },
	func(i int) any { return func() S23 { return S23{i, i} } },
}

func main() {
	const goroutines = 16
	var wg sync.WaitGroup
	start := make(chan struct{})
	var mu sync.Mutex
	names := map[string]int{}
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			<-start
			local := map[string]int{}
			for k := range makers {
				v := makers[(k*7+g*13)%len(makers)](g)
				local[fmt.Sprintf("%T", v)]++
				switch v.(type) {
				case []S0, map[string]S0, [3]S0, *S0, chan S0, func() S0:
				}
			}
			mu.Lock()
			for k, n := range local {
				names[k] += n
			}
			mu.Unlock()
		}(g)
	}
	close(start)
	wg.Wait()
	keys := make([]string, 0, len(names))
	for k := range names {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	bad := 0
	for _, k := range keys {
		if names[k] != goroutines {
			bad++
		}
	}
	fmt.Println(len(keys), bad)
	fmt.Println(keys[0], keys[len(keys)-1])
}

// Output:
// 144 0
// *main.S0 map[string]main.S9
