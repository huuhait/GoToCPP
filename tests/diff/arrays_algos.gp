// phase: 03

// Small algorithms on arrays: a sieve, a fixed-size matrix product, insertion sort on an
// array (through a slice of it), a histogram, a ring buffer and a 2D flood count.
package main

import "fmt"

const n = 3

func mul(a, b [n][n]int) [n][n]int {
	var c [n][n]int
	for i := range n {
		for j := range n {
			for k := range n {
				c[i][j] += a[i][k] * b[k][j]
			}
		}
	}
	return c
}

func sortInts(s []int) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}

var buf [4]int
var head, size int

func push(v int) {
	buf[(head+size)%len(buf)] = v
	if size < len(buf) {
		size++
	} else {
		head = (head + 1) % len(buf)
	}
}

func main() {
	var composite [50]bool
	count := 0
	for i := 2; i < len(composite); i++ {
		if composite[i] {
			continue
		}
		count++
		for j := i * i; j < len(composite); j += i {
			composite[j] = true
		}
	}
	fmt.Println("primes below 50:", count)

	a := [n][n]int{{1, 2, 0}, {0, 1, 0}, {2, 0, 1}}
	id := [n][n]int{{1, 0, 0}, {0, 1, 0}, {0, 0, 1}}
	fmt.Println(mul(a, id) == a, mul(id, a) == a)
	sq := mul(a, a)
	for _, row := range sq {
		fmt.Println(row[0], row[1], row[2])
	}

	vals := [8]int{5, 2, 9, 1, 5, 6, 0, 3}
	sortInts(vals[:])
	fmt.Println(vals[0], vals[1], vals[7], vals == [8]int{0, 1, 2, 3, 5, 5, 6, 9})

	var hist [5]int
	for _, w := range []string{"go", "cpp", "array", "x", "slice", "map"} {
		hist[len(w)-1]++
	}
	for i, h := range hist {
		fmt.Print(i+1, ":", h, " ")
	}
	fmt.Println()

	for i := range 6 {
		push(i * i)
	}
	for i := range size {
		fmt.Print(buf[(head+i)%len(buf)], " ")
	}
	fmt.Println()

	field := [4][5]byte{
		{'#', '.', '.', '#', '.'},
		{'#', '#', '.', '#', '.'},
		{'.', '.', '.', '.', '#'},
		{'#', '.', '#', '#', '#'},
	}
	islands := 0
	var seen [4][5]bool
	var stack [][2]int
	for r := range field {
		for c := range field[r] {
			if field[r][c] != '#' || seen[r][c] {
				continue
			}
			islands++
			stack = append(stack[:0], [2]int{r, c})
			seen[r][c] = true
			for len(stack) > 0 {
				p := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				for _, d := range [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
					y, x := p[0]+d[0], p[1]+d[1]
					if y >= 0 && y < len(field) && x >= 0 && x < len(field[0]) && field[y][x] == '#' && !seen[y][x] {
						seen[y][x] = true
						stack = append(stack, [2]int{y, x})
					}
				}
			}
		}
	}
	fmt.Println("islands:", islands)
}
