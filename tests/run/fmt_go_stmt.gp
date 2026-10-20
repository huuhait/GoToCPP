// phase: 09

// fmt functions in go statements (the operands are evaluated by the go statement, the call runs
// on the new goroutine) and in defer statements, with os.Stdout, a writer and a func value.
package main

import (
	"fmt"
	"os"
	"strings"
	"sync"
)

type sig struct {
	mu sync.Mutex
	sb strings.Builder
	wg *sync.WaitGroup
}

func (s *sig) Write(b []byte) (int, error) {
	s.mu.Lock()
	s.sb.Write(b)
	s.mu.Unlock()
	s.wg.Done()
	return len(b), nil
}

func main() {
	var wg sync.WaitGroup
	s := &sig{wg: &wg}
	n := 1
	wg.Add(4)
	go fmt.Fprintln(s, "go", n)
	n = 2
	go fmt.Fprintf(s, "%d-%s|", n, "f")
	pr := fmt.Fprint
	go pr(s, "value", n)
	n = 3
	go fmt.Fprintln(s, fmt.Sprint("nested ", n), []int{n})
	wg.Wait()
	// The goroutines may finish in any order: sort the four pieces.
	parts := strings.SplitAfter(s.sb.String(), "\n")
	got := strings.Join(parts, "")
	fmt.Println(len(got), strings.Contains(got, "go 1\n"), strings.Contains(got, "2-f|"),
		strings.Contains(got, "value2"), strings.Contains(got, "nested 3 [3]\n"))

	done := make(chan bool)
	go func() {
		defer close(done)
		defer fmt.Fprintln(os.Stdout, "deferred to stdout", n)
		fmt.Fprintln(os.Stdout, "in goroutine")
	}()
	<-done
	defer fmt.Fprintln(os.Stdout, "last deferred")
}

// Output:
// 28 true true true true
// in goroutine
// deferred to stdout 3
// last deferred
