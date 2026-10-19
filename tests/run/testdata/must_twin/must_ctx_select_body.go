package main

import (
	"errors"
	"fmt"
)

var ErrBad = errors.New("bad")

func half(n int) (int, error) {
	fmt.Println("half", n)
	if n%2 != 0 {
		return 0, fmt.Errorf("half %d: %w", n, ErrBad)
	}
	return n / 2, nil
}

func pick(a, b chan int) (int, error) {
	select {
	case v := <-a:
		fmt.Println("from a")
		h, err := half(v)
		if err != nil {
			return 0, err
		}
		return h, nil
	case v := <-b:
		fmt.Println("from b")
		h1, err := half(v)
		if err != nil {
			return 0, err
		}
		h2, err := half(v + 2)
		if err != nil {
			return 0, err
		}
		w := h1 + h2
		return w, nil
	}
}

func poll(c chan int) (int, error) {
	select {
	case v := <-c:
		return v, nil
	default:
		fmt.Println("empty")
		h, err := half(3)
		if err != nil {
			return 0, err
		}
		return h, nil
	}
}

func loop() (sum int, err error) {
	c := make(chan int, 3)
	for _, v := range []int{4, 5, 6} {
		c <- v
	}
	for range 3 {
		select {
		case v := <-c:
			h, err := half(v)
			if err != nil {
				return 0, err
			}
			sum += h
		}
	}
	return sum, nil
}

func main() {
	a, b := make(chan int, 1), make(chan int, 1)
	a <- 8
	fmt.Println(pick(a, b))
	a <- 3
	fmt.Println(pick(a, b))
	b <- 4
	fmt.Println(pick(a, b))
	b <- 5
	fmt.Println(pick(a, b))
	fmt.Println(poll(a))
	a <- 7
	fmt.Println(poll(a))
	fmt.Println(loop())
}
