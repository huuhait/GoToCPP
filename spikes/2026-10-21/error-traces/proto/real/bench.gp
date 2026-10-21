package main

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"
)

var ErrBad = errors.New("bad")

func leaf(n int, mode int) (int, error) {
	if n < 0 {
		if mode == 1 || mode == 3 {
			return 0, errors.New("fresh")
		}
		return 0, ErrBad
	}
	return n + 1, nil
}

func mid!(n int, mode int) int {
	v := leaf!(n, mode)
	return v + 1
}

func top!(n int, mode int) int {
	return mid!(n, mode) * 2
}

func app!(n int, mode int) int {
	return top!(n, mode) + 3
}

// The same chain with context added by errors.Wrapf at two levels (mode 3).
func midw!(n int, mode int) (v int, err error) {
	defer errors.Wrapf(&err, "midw")
	v = leaf!(n, mode)
	return v + 1, nil
}

func topw!(n int, mode int) (v int, err error) {
	defer errors.Wrapf(&err, "topw")
	v = midw!(n, mode)
	return v * 2, nil
}

func appw!(n int, mode int) int {
	return topw!(n, mode) + 3
}

func main() {
	iters, _ := strconv.Atoi(os.Args[1])
	mode, _ := strconv.Atoi(os.Args[2])
	sum := 0
	start := time.Now()
	for i := 0; i < iters; i++ {
		n := 5 + i&1
		if mode != 0 {
			n = -1
		}
		var v int
		var err error
		if mode == 3 {
			v, err = appw(n, mode)
		} else {
			v, err = app(n, mode)
		}
		if err != nil {
			sum++
		}
		sum += v
	}
	d := time.Since(start)
	fmt.Printf("%.2f ns/op (sum %d)\n", float64(d.Nanoseconds())/float64(iters), sum)
}
