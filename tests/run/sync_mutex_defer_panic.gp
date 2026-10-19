// phase: 08

// defer mu.Unlock() releases the lock when the function panics, and recover() continues.
package main

import (
	"fmt"
	"sync"
)

var mu sync.Mutex

func risky(n int) (err error) {
	mu.Lock()
	defer mu.Unlock()
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("recovered: %v", r)
		}
	}()
	if n%2 == 1 {
		panic(fmt.Sprintf("odd %d", n))
	}
	return nil
}

func unrecovered(n int) {
	mu.Lock()
	defer mu.Unlock()
	if n > 0 {
		var m map[string]int
		m["x"] = 1
	}
}

func main() {
	for i := 0; i < 4; i++ {
		fmt.Println(i, risky(i))
	}
	fmt.Println("free after recovered panics:", mu.TryLock())
	mu.Unlock()

	func() {
		defer func() { fmt.Println("outer:", recover()) }()
		unrecovered(1)
	}()
	fmt.Println("free after unrecovered panic:", mu.TryLock())
	mu.Unlock()

	var wg sync.WaitGroup
	results := make([]error, 8)
	for i := range results {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results[i] = risky(i)
		}()
	}
	wg.Wait()
	for i, e := range results {
		fmt.Println(i, e)
	}
}

// Output:
// 0 <nil>
// 1 recovered: odd 1
// 2 <nil>
// 3 recovered: odd 3
// free after recovered panics: true
// outer: assignment to entry in nil map
// free after unrecovered panic: true
// 0 <nil>
// 1 recovered: odd 1
// 2 <nil>
// 3 recovered: odd 3
// 4 <nil>
// 5 recovered: odd 5
// 6 <nil>
// 7 recovered: odd 7
