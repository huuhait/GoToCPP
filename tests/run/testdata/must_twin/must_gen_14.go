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
		if e := f3(18); err == nil {
			err = e
		}
	}()
	{
		t101, err := f1((v2 + v3))
		if err != nil {
			return 0, err
		}
		v1 += t101
	}
	{
		t102, err := f1(5)
		if err != nil {
			return 0, err
		}
		v3 = t102
	}
	{
		t104, err := f1(0)
		if err != nil {
			return 0, err
		}
		if t104 < v3 {
			{
				for i1 := 0; ; i1++ {
					if !(i1 < 3) {
						break
					}
					t105, err := b1((v1 - v1))
					if err != nil {
						return 0, err
					}
					if !(t105) {
						break
					}
					{
						t106, err := f1(v1)
						if err != nil {
							return 0, err
						}
						t107, err := f1(t106)
						if err != nil {
							return 0, err
						}
						v0 += t107
					}
					{
						v1 += 8
					}
				}
			}
		} else {
			if 0 < v3 {
				{
					v1 = (v3 - 2)
				}
				{
					for i2 := 0; ; i2++ {
						if !(i2 < 2) {
							break
						}
						t108, err := f1(v1)
						if err != nil {
							return 0, err
						}
						t109, err := b1(t108)
						if err != nil {
							return 0, err
						}
						c110 := t109
						if !c110 {
							t111, err := b1(v0)
							if err != nil {
								return 0, err
							}
							c110 = t111
						}
						if !(c110) {
							break
						}
						{
							v2 = 3
						}
						{
							t112, err := f1(5)
							if err != nil {
								return 0, err
							}
							t113, err := f1(t112)
							if err != nil {
								return 0, err
							}
							v2 = t113
						}
						{
							t114, err := f1(5)
							if err != nil {
								return 0, err
							}
							t115, err := f1(t114)
							if err != nil {
								return 0, err
							}
							v2 += t115
						}
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
