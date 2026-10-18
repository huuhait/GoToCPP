// phase: 08

// A mutex made of a one-element channel protects a map shared by many goroutines.
package main

import "fmt"

type SafeMap struct {
	lock chan struct{}
	m    map[string]int
}

func NewSafeMap() *SafeMap { return &SafeMap{lock: make(chan struct{}, 1), m: map[string]int{}} }

func (s *SafeMap) Inc(k string) {
	s.lock <- struct{}{}
	s.m[k]++
	<-s.lock
}

func (s *SafeMap) Get(k string) int {
	s.lock <- struct{}{}
	defer func() { <-s.lock }()
	return s.m[k]
}

func main() {
	sm := NewSafeMap()
	done := make(chan bool)
	keys := []string{"a", "b", "c"}
	for g := 0; g < 12; g++ {
		go func() {
			for i := 0; i < 300; i++ {
				sm.Inc(keys[(g+i)%3])
			}
			done <- true
		}()
	}
	for g := 0; g < 12; g++ {
		<-done
	}
	fmt.Println(sm.Get("a")+sm.Get("b")+sm.Get("c"), sm.Get("a") == 1200, len(sm.m))
}

// Output:
// 3600 true 3
