// phase: 08

// With four channels of which exactly two are ready, each of the two is chosen about half of the time,
// for each of the six pairs: the choice is uniform among the ready cases wherever they are in the list
// (6 x 4000 selections; each side is above 40%, a margin of 12 standard deviations).
package main

import "fmt"

func main() {
	var cs [4]chan int
	for i := range cs {
		cs[i] = make(chan int, 1)
	}
	const trials = 4000
	allBalanced := true
	total := 0
	for i := 0; i < 4; i++ {
		for j := i + 1; j < 4; j++ {
			cs[i] <- 1
			cs[j] <- 1
			first := 0
			for t := 0; t < trials; t++ {
				k := -1
				select {
				case <-cs[0]:
					k = 0
				case <-cs[1]:
					k = 1
				case <-cs[2]:
					k = 2
				case <-cs[3]:
					k = 3
				}
				if k == i {
					first++
				}
				cs[k] <- 1
				total++
			}
			<-cs[i]
			<-cs[j]
			if first < trials*40/100 || first > trials*60/100 {
				allBalanced = false
				fmt.Println("unbalanced", i, j, first)
			}
		}
	}
	fmt.Println(total, allBalanced)
}

// Output:
// 24000 true
