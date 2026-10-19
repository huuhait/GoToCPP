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
		if err := f3(v3); err != nil {
			return 0, err
		}
	}
	{
		t101, err := f1(v1)
		if err != nil {
			return 0, err
		}
		t102, err := b1(t101)
		if err != nil {
			return 0, err
		}
		c103 := t102
		if !c103 {
			t104, err := b1(2)
			if err != nil {
				return 0, err
			}
			c105 := t104
			if !c105 {
				t106, err := f1(4)
				if err != nil {
					return 0, err
				}
				t107, err := b1(t106)
				if err != nil {
					return 0, err
				}
				c105 = t107
			}
			c103 = c105
		}
		if c103 {
			{
				v3 = v1
			}
			{
				t108, err := b1((6 + 9))
				if err != nil {
					return 0, err
				}
				if t108 {
					{
						t109, err := f1(v0)
						if err != nil {
							return 0, err
						}
						v2 = t109
					}
					{
						v0 = (v3 - 5)
					}
				} else {
					t110, err := f1(1)
					if err != nil {
						return 0, err
					}
					c111 := (v3 - 9) < t110
					if c111 {
						t112, err := b1(8)
						if err != nil {
							return 0, err
						}
						c111 = t112
					}
					c113 := c111
					if !c113 {
						c114 := (4 - v2) < (2 + 3)
						if c114 {
							t115, err := f1(v0)
							if err != nil {
								return 0, err
							}
							t116, err := b1(t115)
							if err != nil {
								return 0, err
							}
							c114 = t116
						}
						c113 = c114
					}
					if c113 {
						{
							v2 = v2
						}
					} else {
						t117, err := b1((v1 - v3))
						if err != nil {
							return 0, err
						}
						c118 := t117
						if c118 {
							t119, err := b1(6)
							if err != nil {
								return 0, err
							}
							c120 := t119
							if c120 {
								t121, err := f1(v2)
								if err != nil {
									return 0, err
								}
								c120 = t121 < v3
							}
							c118 = c120
						}
						if c118 {
							{
								t122, err := f1((2 - 5))
								if err != nil {
									return 0, err
								}
								v1 = t122
							}
							{
								t123, err := f1(9)
								if err != nil {
									return 0, err
								}
								v1 = t123
							}
						} else {
							{
								t124, err := f1(v0)
								if err != nil {
									return 0, err
								}
								v3 = t124
							}
							{
								t125, err := f1(1)
								if err != nil {
									return 0, err
								}
								v2 = (t125 - (0 - 6))
							}
							{
								t126, err := f1(v3)
								if err != nil {
									return 0, err
								}
								t127, err := f1(t126)
								if err != nil {
									return 0, err
								}
								v0 = t127
							}
						}
					}
				}
			}
			{
				if err := f3(9); err != nil {
					return 0, err
				}
			}
		} else {
			{
				t128, err := f1(v0)
				if err != nil {
					return 0, err
				}
				t129, err := f1(t128)
				if err != nil {
					return 0, err
				}
				v2 = t129
			}
		}
	}
	{
		t130, err := f1(v0)
		if err != nil {
			return 0, err
		}
		v1 = t130
	}
	return (v0*1000 + v1*100 + v2*10 + v3) % 100000, nil
}

func main() {
	for seed := 0; seed < 10; seed++ {
		v, err := run(seed)
		fmt.Println("run", seed, v, err, errors.Is(err, ErrBad))
	}
}
