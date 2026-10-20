package lib

import "fmt"

type Node struct {
	Value int
	pad   [8]int
}

func Read(p *Node, s string, n int) int { return p.Value + len(s) + n }

func Show(s string, n int) {
	fmt.Println("qualified defer", s, n)
}

func Send(s string, n int, done chan string) {
	if n != 1 {
		panic("wrong qualified count")
	}
	done <- s
}
