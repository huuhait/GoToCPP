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
		if e := f3(19); err == nil {
			err = e
		}
	}()
	defer func() {
		if e := f3(0); err == nil {
			err = e
		}
	}()
	{
		t101, err := f1(v2)
		if err != nil {
			return 0, err
		}
		if err := f3(t101); err != nil {
			return 0, err
		}
	}
	{
		v0 = v0
	}
	{
		for i1 := 0; ; i1++ {
			if !(i1 < 2) {
				break
			}
			t102, err := b1(v0)
			if err != nil {
				return 0, err
			}
			if !(t102) {
				break
			}
			{
				for i2 := 0; ; i2++ {
					if !(i2 < 1) {
						break
					}
					t103, err := f1(v2)
					if err != nil {
						return 0, err
					}
					t104, err := b1(t103)
					if err != nil {
						return 0, err
					}
					if !(t104) {
						break
					}
					{
						t105, err := f1(v2)
						if err != nil {
							return 0, err
						}
						if t105 < 5 {
							continue
						}
					}
					{
						t106, err := f1((v0 - v0))
						if err != nil {
							return 0, err
						}
						v3 = t106
					}
				}
			}
		}
	}
	return (v0*1000 + v1*100 + v2*10 + v3) % 100000, nil
}

func main() {
	for seed := 0; seed < 10; seed++ {
		v, err := run(seed)
		fmt.Println("run", seed, v, err, errors.Is(err, ErrBad))
	}
}
