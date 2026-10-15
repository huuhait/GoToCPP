// phase: 03

// Breadth-first search on a grid maze (a slice of strings), with a slice as the queue, a map of
// visited points (struct keys) and a map of parents to rebuild the path.
package main

import "fmt"

type pt struct{ r, c int }

var maze = []string{
	"S.#.....",
	".##.###.",
	"....#...",
	".##...#E",
}

func neighbors(p pt) []pt {
	var out []pt
	for _, d := range [4]pt{{0, 1}, {1, 0}, {0, -1}, {-1, 0}} {
		q := pt{p.r + d.r, p.c + d.c}
		if q.r >= 0 && q.r < len(maze) && q.c >= 0 && q.c < len(maze[0]) && maze[q.r][q.c] != '#' {
			out = append(out, q)
		}
	}
	return out
}

func find(ch byte) pt {
	for r, row := range maze {
		for c := 0; c < len(row); c++ {
			if row[c] == ch {
				return pt{r, c}
			}
		}
	}
	return pt{-1, -1}
}

func main() {
	start, end := find('S'), find('E')
	parent := map[pt]pt{}
	seen := map[pt]bool{start: true}
	queue := []pt{start}
	for len(queue) > 0 {
		p := queue[0]
		queue = queue[1:]
		if p == end {
			break
		}
		for _, q := range neighbors(p) {
			if !seen[q] {
				seen[q] = true
				parent[q] = p
				queue = append(queue, q)
			}
		}
	}
	var path []pt
	for p := end; p != start; p = parent[p] {
		path = append(path, p)
	}
	fmt.Println(len(path), len(seen), start, end)
	grid := make([][]byte, len(maze))
	for i, row := range maze {
		grid[i] = []byte(row)
	}
	for _, p := range path[1:] {
		grid[p.r][p.c] = '*'
	}
	for _, row := range grid {
		fmt.Println(string(row))
	}
}

// Output:
// 12 22 {0 0} {3 7}
// S.#.....
// *##.###.
// ****#***
// .##***#E
