// phase: 08

// Many goroutines read the same data and copy the same pointers: reference counts are not races.
package main

import "fmt"

type Node struct {
	val  int
	next *Node
}

func sum(n *Node) int {
	t := 0
	for ; n != nil; n = n.next {
		t += n.val
	}
	return t
}

func main() {
	list := &Node{1, &Node{2, &Node{3, nil}}}
	names := []string{"a", "b", "c"}
	done := make(chan int)
	for g := 0; g < 8; g++ {
		go func() {
			t := 0
			for i := 0; i < 100; i++ {
				p := list
				q := p.next
				t += sum(p) + len(names[i%3]) + q.val
			}
			done <- t
		}()
	}
	total := 0
	for g := 0; g < 8; g++ {
		total += <-done
	}
	fmt.Println(total)
}

// Output:
// 7200
