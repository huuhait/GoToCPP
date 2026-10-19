// bench: collatz
// category: cpu
// size: 3000000
// smoke: 3000
// twin: yes
// about: longest Collatz chain below n (integer loop with a branch per step)
package main

import "fmt"

func main() {
	best, bestLen, steps := 1, 1, 0
	for i := 1; i < benchN; i++ {
		x, l := i, 1
		for x != 1 {
			if x&1 == 0 {
				x >>= 1
			} else {
				x = 3*x + 1
			}
			l++
		}
		steps += l
		if l > bestLen {
			best, bestLen = i, l
		}
	}
	fmt.Println(best, bestLen, steps)
}
