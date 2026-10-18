// phase: 08

// Fan-out: several workers read the same channel. Fan-in: their results are merged on one
// channel. The merged order is not fixed, so the program sorts before it prints.
package main

import "fmt"

func sortInts(a []int) {
	for i := 1; i < len(a); i++ {
		for j := i; j > 0 && a[j] < a[j-1]; j-- {
			a[j], a[j-1] = a[j-1], a[j]
		}
	}
}

func worker(id int, in <-chan int, out chan<- [2]int) {
	for {
		v, ok := <-in
		if !ok {
			break
		}
		out <- [2]int{v, v * v * v}
	}
	out <- [2]int{-id, 0} // "worker id is done"
}

func main() {
	jobs := make(chan int)
	results := make(chan [2]int)
	const workers = 4
	for w := 1; w <= workers; w++ {
		go worker(w, jobs, results)
	}
	go func() {
		for i := 1; i <= 20; i++ {
			jobs <- i
		}
		close(jobs)
	}()
	var cubes []int
	done := 0
	for done < workers {
		r := <-results
		if r[0] < 0 {
			done++
			continue
		}
		cubes = append(cubes, r[1])
	}
	sortInts(cubes)
	fmt.Println(len(cubes), cubes[0], cubes[19])
	sum := 0
	for _, c := range cubes {
		sum += c
	}
	fmt.Println(sum)
}

// Output:
// 20 1 8000
// 44100
