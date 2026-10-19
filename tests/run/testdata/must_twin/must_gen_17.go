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
	{
		for i1 := 0; ; i1++ {
			if !(i1 < 3) {
				break
			}
			{
				for i2 := 0; ; i2++ {
					if !(i2 < 2) {
						break
					}
					t101, err := b1(v2)
					if err != nil {
						return 0, err
					}
					if !(t101) {
						break
					}
					{
						if (v0 + 4) < 7 {
							continue
						}
					}
					{
						if err := f3(7); err != nil {
							return 0, err
						}
					}
					{
						for i3 := 0; ; i3++ {
							if !(i3 < 1) {
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
								t103, err := f1(v2)
								if err != nil {
									return 0, err
								}
								v1 = t103
							}
							{
								t104, err := f1(v2)
								if err != nil {
									return 0, err
								}
								t105, err := f1(t104)
								if err != nil {
									return 0, err
								}
								v1 = t105
							}
						}
					}
				}
			}
			{
				for i4 := 0; ; i4++ {
					if !(i4 < 3) {
						break
					}
					t106, err := f1(v3)
					if err != nil {
						return 0, err
					}
					c107 := (v1 - v1) < t106
					if !c107 {
						t108, err := f1(v2)
						if err != nil {
							return 0, err
						}
						t109, err := b1(t108)
						if err != nil {
							return 0, err
						}
						c107 = t109
					}
					c110 := c107
					if c110 {
						t111, err := b1(v2)
						if err != nil {
							return 0, err
						}
						c110 = t111
					}
					if !(c110) {
						break
					}
					{
						for i5 := 0; ; i5++ {
							if !(i5 < 2) {
								break
							}
							t112, err := f1(v3)
							if err != nil {
								return 0, err
							}
							t113, err := b1(t112)
							if err != nil {
								return 0, err
							}
							if !(t113) {
								break
							}
							{
								if err := f3(v3); err != nil {
									return 0, err
								}
							}
						}
					}
				}
			}
		}
	}
	{
		switch v0 % 3 {
		case 0:
			{
				v2 = (9 - (v2 - 1))
			}
			{
				for i6 := 0; ; i6++ {
					if !(i6 < 2) {
						break
					}
					t114, err := f1(v2)
					if err != nil {
						return 0, err
					}
					if !(t114 < (9 - v1)) {
						break
					}
					{
						if v3 < 6 {
							break
						}
					}
					{
						t115, err := f1(v0)
						if err != nil {
							return 0, err
						}
						a116, b116, err := f2(t115)
						if err != nil {
							return 0, err
						}
						v1, v0 = a116, b116
					}
					{
						v2 = v3
					}
				}
			}
		case 1:
			{
				t118, err := b1((v2 - 5))
				if err != nil {
					return 0, err
				}
				if t118 {
					{
						t119, err := f1((v2 - v2))
						if err != nil {
							return 0, err
						}
						v2 += t119
					}
					{
						t120, err := f1(v3)
						if err != nil {
							return 0, err
						}
						a121, b121, err := f2(t120)
						if err != nil {
							return 0, err
						}
						v3, v1 = a121, b121
					}
					{
						t122, err := f1((0 + v3))
						if err != nil {
							return 0, err
						}
						v2 = t122
					}
				} else {
					t123, err := b1(5)
					if err != nil {
						return 0, err
					}
					if t123 {
						{
							v1 = (v3 - v2)
						}
						{
							t124, err := f1(v0)
							if err != nil {
								return 0, err
							}
							v0 += ((v3 - v0) - t124)
						}
					} else {
						t125, err := f1(v2)
						if err != nil {
							return 0, err
						}
						t126, err := b1(t125)
						if err != nil {
							return 0, err
						}
						if t126 {
							{
								t127, err := f1(3)
								if err != nil {
									return 0, err
								}
								a128, b128, err := f2(t127)
								if err != nil {
									return 0, err
								}
								v2, v1 = a128, b128
							}
						}
					}
				}
			}
		default:
			{
				if err := f3(0); err != nil {
					return 0, err
				}
			}
		}
	}
	{
		t129, err := f1(4)
		if err != nil {
			return 0, err
		}
		t130, err := f1(t129)
		if err != nil {
			return 0, err
		}
		v2 += t130
	}
	return (v0*1000 + v1*100 + v2*10 + v3) % 100000, nil
}

func main() {
	for seed := 0; seed < 10; seed++ {
		v, err := run(seed)
		fmt.Println("run", seed, v, err, errors.Is(err, ErrBad))
	}
}
