// phase: 08

// Merge and fan-in of channels of any element type, with goroutines.
package main

import "fmt"

func Gen[T any](vs ...T) <-chan T {
	ch := make(chan T, len(vs))
	for _, v := range vs {
		ch <- v
	}
	close(ch)
	return ch
}

func Merge[T any](a, b <-chan T) <-chan T {
	out := make(chan T)
	done := make(chan bool)
	pump := func(c <-chan T) {
		for {
			v, ok := <-c
			if !ok {
				break
			}
			out <- v
		}
		done <- true
	}
	go pump(a)
	go pump(b)
	go func() {
		<-done
		<-done
		close(out)
	}()
	return out
}

func Collect[T any](c <-chan T) []T {
	var out []T
	for {
		v, ok := <-c
		if !ok {
			return out
		}
		out = append(out, v)
	}
}

func Sum[N ~int | ~float64](c <-chan N) N {
	var s N
	for _, v := range Collect(c) {
		s += v
	}
	return s
}

func main() {
	fmt.Println(Sum(Merge(Gen(1, 2, 3), Gen(10, 20))))
	fmt.Println(Sum(Merge(Gen(0.5, 0.25), Gen[float64]())))
	words := Collect(Merge(Gen("a"), Gen("b")))
	fmt.Println(len(words), words[0] != words[1])
	type pair struct{ a, b int }
	ps := Collect(Merge(Gen(pair{1, 2}), Gen(pair{3, 4})))
	fmt.Println(len(ps), ps[0].a+ps[1].a, ps[0].b+ps[1].b)
	fmt.Println(len(Collect(Merge(Gen[int](), Gen[int]()))))
}

// Output:
// 36
// 0.75
// 2 true
// 2 4 6
// 0
