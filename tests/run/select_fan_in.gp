// phase: 08

// Fan-in with select: three producers, one consumer loop that disables a channel by setting it to
// nil when it is closed. The result is sorted: the interleaving is not defined.
package main

import "fmt"

func produce(name string, n int) <-chan string {
	c := make(chan string)
	go func() {
		defer close(c)
		for i := 0; i < n; i++ {
			c <- fmt.Sprintf("%s%d", name, i)
		}
	}()
	return c
}

func sortInts(a []int) {
	for i := 1; i < len(a); i++ {
		for j := i; j > 0 && a[j-1] > a[j]; j-- {
			a[j-1], a[j] = a[j], a[j-1]
		}
	}
}

func sortStrings(a []string) {
	for i := 1; i < len(a); i++ {
		for j := i; j > 0 && a[j-1] > a[j]; j-- {
			a[j-1], a[j] = a[j], a[j-1]
		}
	}
}

func main() {
	a, b, c := produce("a", 5), produce("b", 3), produce("c", 4)
	var all []string
	for a != nil || b != nil || c != nil {
		select {
		case v, ok := <-a:
			if !ok {
				a = nil
				break
			}
			all = append(all, v)
		case v, ok := <-b:
			if !ok {
				b = nil
				break
			}
			all = append(all, v)
		case v, ok := <-c:
			if !ok {
				c = nil
				break
			}
			all = append(all, v)
		}
	}
	sortStrings(all)
	fmt.Println(len(all), all)
}

// Output:
// 12 [a0 a1 a2 a3 a4 b0 b1 b2 c0 c1 c2 c3]
