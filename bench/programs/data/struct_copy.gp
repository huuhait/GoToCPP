// bench: struct_copy
// category: data
// size: 20000
// smoke: 20
// twin: yes
// about: medium structs passed and returned by value, arrays of structs, array copies
package main

import "fmt"

type vec4 struct{ x, y, z, w uint64 }

type body struct {
	pos, vel vec4
	id       uint64
	hist     [4]uint64
}

func add(a, b vec4) vec4 {
	return vec4{a.x + b.x, a.y + b.y, a.z + b.z, a.w + b.w}
}

func step(b body) body {
	b.pos = add(b.pos, b.vel)
	b.hist[b.id&3] = b.pos.x
	b.id++
	return b
}

func fold(g [16]vec4) uint64 {
	var s uint64
	for i := range g {
		s += g[i].x ^ g[i].w
	}
	return s
}

func main() {
	arr := make([]body, 1024)
	for i := range arr {
		arr[i].vel = vec4{uint64(i), uint64(i) * 3, 5, 7}
		arr[i].id = uint64(i)
	}
	for r := 0; r < benchN; r++ {
		for i := range arr {
			arr[i] = step(arr[i])
		}
	}
	var sum uint64
	for i := range arr {
		sum += arr[i].pos.x + arr[i].pos.y + arr[i].hist[2] + arr[i].id
	}

	var grid [16]vec4
	var gsum uint64
	for r := 0; r < benchN*50; r++ {
		g2 := grid
		g2[r&15].x += uint64(r)
		g2[(r+1)&15].w ^= uint64(r) * 3
		gsum += fold(g2)
		grid = g2
	}
	fmt.Println(sum, gsum)
}
