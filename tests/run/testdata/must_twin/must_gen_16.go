package main

import (
	"errors"
	"fmt"
)

var ErrBad = errors.New("bad")

func f1(n int) (int, error) {
	fmt.Println("f1", n)
	if n%5 == 3 {
		return 0, fmt.Errorf("f1(%d): %w", n, ErrBad)
	}
	return n*2 + 1, nil
}

func f2(n int) (int, int, error) {
	fmt.Println("f2", n)
	if n%6 == 2 {
		return 0, 0, fmt.Errorf("f2(%d): %w", n, ErrBad)
	}
	return n + 1, n - 1, nil
}

func f3(n int) error {
	fmt.Println("f3", n)
	if n%7 == 4 {
		return fmt.Errorf("f3(%d): %w", n, ErrBad)
	}
	return nil
}

func b1(n int) (bool, error) {
	fmt.Println("b1", n)
	if n%8 == 5 {
		return false, fmt.Errorf("b1(%d): %w", n, ErrBad)
	}
	return n%2 == 0, nil
}

func run(seed int) (res int, err error) {
	v0, v1, v2, v3 := seed, seed*3+1, seed%4, 7-seed
	defer func() {
		if e := f3(17); err == nil {
			err = e
		}
	}()
	{
		t101, err := f1(v3)
		if err != nil {
			return 0, err
		}
		t102, err := b1(t101)
		if err != nil {
			return 0, err
		}
		if t102 {
			{
				t103, err := f1(v0)
				if err != nil {
					return 0, err
				}
				v2 = t103
			}
		} else {
			t104, err := b1((v3 + v2))
			if err != nil {
				return 0, err
			}
			if t104 {
				{
					t105, err := f1(v0)
					if err != nil {
						return 0, err
					}
					t106, err := f1(4)
					if err != nil {
						return 0, err
					}
					v0 = (t105 - t106)
				}
				{
					if err := f3(v1); err != nil {
						return 0, err
					}
				}
			}
		}
	}
	{
		v2 = v1
	}
	{
		t107, err := f1(v3)
		if err != nil {
			return 0, err
		}
		a108, b108, err := f2(t107)
		if err != nil {
			return 0, err
		}
		v3, v2 = a108, b108
	}
	return (v0*1000 + v1*100 + v2*10 + v3) % 100000, nil
}

func main() {
	for seed := 0; seed < 10; seed++ {
		v, err := run(seed)
		fmt.Println("run", seed, v, err, errors.Is(err, ErrBad))
	}
}
