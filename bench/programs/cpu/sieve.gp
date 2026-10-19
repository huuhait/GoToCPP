// bench: sieve
// category: cpu
// size: 50000000
// smoke: 20000
// twin: yes
// about: sieve of Eratosthenes over a []bool
package main

import "fmt"

func main() {
	n := benchN
	composite := make([]bool, n+1)
	for i := 2; i*i <= n; i++ {
		if !composite[i] {
			for j := i * i; j <= n; j += i {
				composite[j] = true
			}
		}
	}
	count, last, sum := 0, 0, 0
	for i := 2; i <= n; i++ {
		if !composite[i] {
			count++
			last = i
			sum += i % 1000
		}
	}
	fmt.Println(count, last, sum)
}
