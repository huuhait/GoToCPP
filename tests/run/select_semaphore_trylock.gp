// phase: 08

// try-lock and a counting semaphore with select and default.
package main

import "fmt"

type tryMutex struct{ c chan struct{} }

func newTry() *tryMutex { return &tryMutex{make(chan struct{}, 1)} }

func (m *tryMutex) tryLock() bool {
	select {
	case m.c <- struct{}{}:
		return true
	default:
		return false
	}
}

func (m *tryMutex) unlock() { <-m.c }

func main() {
	m := newTry()
	fmt.Println(m.tryLock(), m.tryLock())
	m.unlock()
	fmt.Println(m.tryLock())
	m.unlock()

	// 50 goroutines race for the lock; the winners are counted without a data race (the counter is
	// only touched while holding it)
	wins := 0
	done := make(chan bool)
	start := make(chan struct{})
	for i := 0; i < 50; i++ {
		go func() {
			<-start
			if m.tryLock() {
				wins++
				m.unlock()
			}
			done <- true
		}()
	}
	close(start)
	for i := 0; i < 50; i++ {
		<-done
	}
	fmt.Println(wins >= 1 && wins <= 50)
}

// Output:
// true false
// true
// true
