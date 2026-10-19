// phase: 07

// A generic graph with a map of slices of instances, breadth-first search with a generic queue.
package main

import "fmt"

type Queue[T any] struct{ items []T }

func (q *Queue[T]) Push(v T)   { q.items = append(q.items, v) }
func (q *Queue[T]) Empty() bool { return len(q.items) == 0 }
func (q *Queue[T]) Pop() T {
	v := q.items[0]
	q.items = q.items[1:]
	return v
}

type Graph[N comparable] struct {
	adj map[N][]N
}

func NewGraph[N comparable]() *Graph[N] { return &Graph[N]{adj: map[N][]N{}} }

func (g *Graph[N]) Edge(a, b N) {
	g.adj[a] = append(g.adj[a], b)
	g.adj[b] = append(g.adj[b], a)
}

func (g *Graph[N]) BFS(start N) []N {
	seen := map[N]bool{start: true}
	var order []N
	q := &Queue[N]{}
	q.Push(start)
	for !q.Empty() {
		n := q.Pop()
		order = append(order, n)
		for _, m := range g.adj[n] {
			if !seen[m] {
				seen[m] = true
				q.Push(m)
			}
		}
	}
	return order
}

func (g *Graph[N]) Path(a, b N) []N {
	prev := map[N]N{}
	seen := map[N]bool{a: true}
	q := &Queue[N]{}
	q.Push(a)
	for !q.Empty() {
		n := q.Pop()
		if n == b {
			var path []N
			for cur := b; cur != a; cur = prev[cur] {
				path = append([]N{cur}, path...)
			}
			return append([]N{a}, path...)
		}
		for _, m := range g.adj[n] {
			if !seen[m] {
				seen[m] = true
				prev[m] = n
				q.Push(m)
			}
		}
	}
	return nil
}

type City struct{ Name string }

func main() {
	g := NewGraph[int]()
	for _, e := range [][2]int{{1, 2}, {1, 3}, {2, 4}, {3, 4}, {4, 5}} {
		g.Edge(e[0], e[1])
	}
	fmt.Println(g.BFS(1), g.Path(1, 5), g.Path(5, 1))
	c := NewGraph[City]()
	c.Edge(City{"a"}, City{"b"})
	c.Edge(City{"b"}, City{"c"})
	fmt.Println(c.BFS(City{"c"}), len(c.Path(City{"a"}, City{"z"})))
	fmt.Printf("%T\n", c)
}

// Output:
// [1 2 3 4 5] [1 2 4 5] [5 4 2 1]
// [{c} {b} {a}] 0
// *main.Graph[main.City]
