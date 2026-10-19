// bench: map_string
// category: data
// size: 500000
// smoke: 2000
// twin: yes
// about: map[string]int insert, lookup and delete of generated string keys
package main

import "fmt"

var seed uint64 = 777

func rnd() uint64 {
	seed = seed*6364136223846793005 + 1442695040888963407
	return seed >> 33
}

func key(x uint64) string { return "key-" + fmt.Sprint(int(x)) }

func main() {
	n := benchN
	m := make(map[string]int)
	for i := 0; i < n; i++ {
		m[key(rnd()%uint64(2*n))] += i
	}
	var sum uint64
	hits := 0
	for i := 0; i < n; i++ {
		if v, ok := m[key(rnd()%uint64(2*n))]; ok {
			hits++
			sum += uint64(v)
		}
	}
	del := 0
	for i := 0; i < n; i++ {
		k := key(rnd() % uint64(2*n))
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
