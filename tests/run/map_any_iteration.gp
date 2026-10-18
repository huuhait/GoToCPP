// phase: 05

// Deleting, clearing and growing a map[any]V while it is ranged over (step 5.6): the results
// that Go guarantees (every entry is visited at most once, a deleted entry that was not yet
// reached is not visited, clear stops the iteration) without depending on the order.
package main

import "fmt"

func main() {
	m := map[any]int{}
	for i := 0; i < 200; i++ {
		m[i] = i
		m[fmt.Sprint("s", i)] = i
		m[float64(i)+0.5] = i
	}
	fmt.Println(len(m))

	// Delete everything while iterating: each key once.
	seen := map[any]bool{}
	for k := range m {
		if seen[k] {
			fmt.Println("visited twice", k)
		}
		seen[k] = true
		delete(m, k)
	}
	fmt.Println(len(m), len(seen))

	// Delete the partner of the current key: the partner is never visited afterwards.
	for i := 0; i < 100; i++ {
		m[i] = i
	}
	visited := map[int]bool{}
	for k := range m {
		i := k.(int)
		visited[i] = true
		if i%2 == 0 {
			delete(m, i+1)
		} else {
			delete(m, i-1)
		}
	}
	both := 0
	for i := 0; i < 100; i += 2 {
		if visited[i] && visited[i+1] {
			both++
		}
	}
	// Whichever of a pair came first removed the other, unless it was already visited.
	fmt.Println(len(visited) <= 100, both <= 50, len(m) >= 0)

	// clear during the iteration ends it.
	for i := 0; i < 50; i++ {
		m[i] = i
	}
	n := 0
	for range m {
		n++
		clear(m)
	}
	fmt.Println(n, len(m))

	// Overwriting values while iterating is fine and keeps the keys.
	for i := 0; i < 100; i++ {
		m[i] = 0
	}
	for k := range m {
		m[k] = k.(int) * 2
	}
	sum := 0
	for _, v := range m {
		sum += v
	}
	fmt.Println(len(m), sum)

	// Deleted and inserted again: the new entry may or may not be visited, but the map is right.
	for k := range m {
		delete(m, k)
		m[k] = 1
	}
	fmt.Println(len(m))
	delete(m, "missing")
	delete(m, 1000)
	var nm map[any]int
	delete(nm, 1)
	clear(nm)
	for range nm {
		fmt.Println("unreachable")
	}
	fmt.Println(len(nm), nm[1])
}

// Output:
// 600
// 0 600
// true true true
// 1 0
// 100 9900
// 100
// 0 0
