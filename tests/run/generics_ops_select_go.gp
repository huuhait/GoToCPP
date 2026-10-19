// phase: 08

// select, go and sync with type parameters.
package main

import "fmt"

type Result struct{ V int }

func Merge[T any](a, b <-chan T) []T {
	var out []T
	for a != nil || b != nil {
		select {
		case v, ok := <-a:
			if !ok {
				a = nil
				continue
			}
			out = append(out, v)
		case v, ok := <-b:
			if !ok {
				b = nil
				continue
			}
			out = append(out, v)
		}
	}
	return out
}

func TryRecv[T any](c <-chan T) (T, string) {
	select {
	case v := <-c:
		return v, "got"
	default:
		var z T
		return z, "empty"
	}
}

func TrySend[T any](c chan<- T, v T) bool {
	select {
	case c <- v:
		return true
	default:
		return false
	}
}

func Fan[T any](n int, f func(int) T) []T {
	res := make(chan struct {
		i int
		v T
	}, n)
	for i := 0; i < n; i++ {
		go func() { res <- struct {
			i int
			v T
		}{i, f(i)} }()
	}
	out := make([]T, n)
	for range n {
		r := <-res
		out[r.i] = r.v
	}
	return out
}

func First[T any](cs ...chan T) (T, int) {
	for {
		for i, c := range cs {
			select {
			case v := <-c:
				return v, i
			default:
			}
		}
	}
}

func Done[T any](v T) <-chan T {
	c := make(chan T, 1)
	c <- v
	return c
}

func main() {
	a := make(chan int, 3)
	b := make(chan int, 3)
	for i := 0; i < 3; i++ {
		a <- i
		b <- 10 + i
	}
	close(a)
	close(b)
	m := Merge(a, b)
	sum := 0
	for _, v := range m {
		sum += v
	}
	fmt.Println(len(m), sum)
	fmt.Println(TryRecv(make(chan string)))
	fmt.Println(TryRecv(Done("hi")))
	fmt.Println(TryRecv(Done(Result{1}.V)))
	c := make(chan float64, 1)
	fmt.Println(TrySend(c, 1.5), TrySend(c, 2.5), <-c, TrySend(c, 3))
	fmt.Println(Fan(5, func(i int) string { return fmt.Sprint("w", i) }), Fan(3, func(i int) int { return i * i }))
	x, y := make(chan string, 1), make(chan string, 1)
	y <- "from y"
	fmt.Println(First(x, y))
}

// Output:
// 6 36
//  empty
// hi got
// 1 got
// true false 1.5 true
// [w0 w1 w2 w3 w4] [0 1 4]
// from y 1
