// phase: 05

package main

import "fmt"

func loop() (out []string) {
outer:
	for i := 0; i < 4; i++ {
		defer func() { out = append(out, fmt.Sprint("deferred ", i)) }()
		for j := 0; j < 4; j++ {
			switch {
			case j == 1:
				continue
			case j == 2 && i == 1:
				continue outer
			case i == 3:
				break outer
			}
			out = append(out, fmt.Sprint("body ", i, j))
		}
	}
	return out
}

func gotoDefer() (n int) {
	defer func() { n *= 3 }()
	i := 0
retry:
	i++
	defer func() { n += 100 }()
	if i < 3 {
		goto retry
	}
	return i
}

func panicInRange() (last int) {
	defer func() {
		recover()
		last = -last
	}()
	for i := range 10 {
		last = i
		if i == 4 {
			panic("stop")
		}
	}
	return 0
}

func main() {
	fmt.Println(loop())
	fmt.Println(gotoDefer())
	fmt.Println(panicInRange())
}
