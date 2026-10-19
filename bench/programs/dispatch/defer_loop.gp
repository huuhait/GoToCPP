// bench: defer_loop
// category: dispatch
// size: 40000000
// smoke: 20000
// twin: no
// about: defer in a function called in a loop: deferred calls with arguments and deferred closures
package main

import "fmt"

func bump(p *int, k int) { *p += k }

func viaCall(p *int, i int) {
	defer bump(p, i&7)
	*p ^= i
}

func viaClosure(p *int, i int) {
	defer func() { *p += i & 3 }()
	*p ^= i
}

func twoDefers(p *int, i int) {
	defer bump(p, 1)
	defer func() { *p ^= 5 }()
	*p += i
}

func main() {
	v := 0
	for i := 0; i < benchN; i++ {
		viaCall(&v, i)
		viaClosure(&v, i)
		if i&3 == 0 {
			twoDefers(&v, i)
		}
	}
	fmt.Println(v)
}
