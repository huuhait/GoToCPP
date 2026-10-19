// bench: map_int
// category: data
// size: 1000000
// smoke: 2000
// twin: yes
// about: map[int]int insert, lookup and delete of random keys
package main

import "fmt"

var seed uint64 = 12345

func rnd() uint64 {
	seed = seed*6364136223846793005 + 1442695040888963407
	return seed >> 33
}

func main() {
	n := benchN
	m := make(map[int]int)
	for i := 0; i < n; i++ {
		m[int(rnd()%uint64(2*n))] += i
	}
	var sum uint64
	hits := 0
	for i := 0; i < n; i++ {
		if v, ok := m[int(rnd()%uint64(2*n))]; ok {
			hits++
			sum += uint64(v)
		}
	}
	del := 0
	for i := 0; i < n; i++ {
		k := int(rnd() % uint64(2*n))
		if _, ok := m[k]; ok {
			delete(m, k)
			del++
		}
	}
	var rest uint64
	for _, v := range m {
		rest += uint64(v)
	}
	fmt.Println(len(m), hits, sum, del, rest)
}
