// phase: 08

// Accounts with a mutex each, in a map of pointers; transfers lock in account order, and the total
// never changes.
package main

import (
	"fmt"
	"sync"
)

type Account struct {
	sync.Mutex
	id      int
	balance int
}

func transfer(from, to *Account, amount int) {
	first, second := from, to
	if first.id > second.id {
		first, second = second, first
	}
	first.Lock()
	defer first.Unlock()
	second.Lock()
	defer second.Unlock()
	from.balance -= amount
	to.balance += amount
}

func main() {
	accounts := map[int]*Account{}
	for i := 0; i < 10; i++ {
		accounts[i] = &Account{id: i, balance: 1000}
	}
	var wg sync.WaitGroup
	for g := 0; g < 30; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 1000; i++ {
				a := accounts[(g+i)%10]
				b := accounts[(g*7+i*3+1)%10]
				if a != b {
					transfer(a, b, (i%5)+1)
				}
			}
		}()
	}
	wg.Wait()
	total := 0
	for _, a := range accounts {
		total += a.balance
	}
	fmt.Println("total:", total)
}

// Output:
// total: 10000
