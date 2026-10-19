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
			if !(i1 < 2) {
				break
			}
			t101, err := b1(6)
			if err != nil {
				return 0, err
			}
			c102 := t101
			if !c102 {
				t103, err := f1(v2)
				if err != nil {
					return 0, err
				}
				c104 := (6 + v1) < t103
				if c104 {
					t105, err := b1(v0)
					if err != nil {
						return 0, err
					}
					c104 = t105
				}
				c102 = c104
			}
			if !(c102) {
				break
			}
			{
				t106, err := b1(5)
				if err != nil {
					return 0, err
				}
				c107 := t106
				if c107 {
					t108, err := f1(v0)
					if err != nil {
						return 0, err
					}
					c109 := 1 < t108
					if !c109 {
						t110, err := f1(v2)
						if err != nil {
							return 0, err
						}
						t111, err := f1(v2)
						if err != nil {
							return 0, err
						}
						c109 = t110 < t111
					}
					c107 = c109
				}
				if c107 {
					{
						if err := f3(v0); err != nil {
							return 0, err
						}
					}
					{
						for i2 := 0; ; i2++ {
							if !(i2 < 3) {
								break
							}
							t112, err := b1(v2)
							if err != nil {
								return 0, err
							}
							c113 := t112
							if !c113 {
								t114, err := b1((8 + v3))
								if err != nil {
									return 0, err
								}
								c115 := t114
								if !c115 {
									t116, err := f1(0)
									if err != nil {
										return 0, err
									}
									c115 = t116 < (v1 + 0)
								}
								c113 = c115
							}
							if !(c113) {
								break
							}
							{
								if err := f3((v1 - v1)); err != nil {
									return 0, err
								}
							}
							{
								t117, err := f1(5)
								if err != nil {
									return 0, err
								}
								v3 = ((3 - v3) - t117)
							}
						}
					}
					{
						t118, err := f1(v2)
						if err != nil {
							return 0, err
						}
						a119, b119, err := f2(t118)
						if err != nil {
							return 0, err
						}
						v1, v0 = a119, b119
					}
				} else {
					t120, err := f1(1)
					if err != nil {
						return 0, err
					}
					t121, err := b1(t120)
					if err != nil {
						return 0, err
					}
					if t121 {
						{
							if v3 < 4 {
								break
							}
						}
					} else {
						t122, err := f1(v1)
						if err != nil {
							return 0, err
						}
						t123, err := b1(t122)
						if err != nil {
							return 0, err
						}
						c124 := t123
						if !c124 {
							t125, err := f1(v2)
							if err != nil {
								return 0, err
							}
							t126, err := b1(t125)
							if err != nil {
								return 0, err
							}
							c124 = t126
						}
						c127 := c124
						if c127 {
							t128, err := b1(9)
							if err != nil {
								return 0, err
							}
							c127 = t128
						}
						if c127 {
							{
								v3 = 3
							}
						}
					}
				}
			}
			{
				t129, err := f1(v2)
				if err != nil {
					return 0, err
				}
				t130, err := f1(t129)
				if err != nil {
					return 0, err
				}
				v1 = t130
			}
			{
				for i3 := 0; ; i3++ {
					if !(i3 < 1) {
						break
					}
					t131, err := b1(v1)
					if err != nil {
						return 0, err
					}
					c132 := t131
					if !c132 {
						t133, err := f1(7)
						if err != nil {
							return 0, err
						}
						t134, err := b1(t133)
						if err != nil {
							return 0, err
						}
						c132 = t134
					}
					c135 := c132
					if c135 {
						t136, err := b1(v1)
						if err != nil {
							return 0, err
						}
						c135 = t136
					}
					if !(c135) {
						break
					}
					{
						for i4 := 0; ; i4++ {
							if !(i4 < 3) {
								break
							}
							{
								v1 += v2
							}
						}
					}
					{
						t137, err := f1(v2)
						if err != nil {
							return 0, err
						}
						v3 = t137
					}
					{
						if (7 + 3) < 7 {
							continue
						}
					}
				}
			}
		}
	}
	{
		v0 = ((4 - v3) - (v0 + v3))
	}
	{
		t138, err := f1(4)
		if err != nil {
			return 0, err
		}
		a139, b139, err := f2(t138)
		if err != nil {
			return 0, err
		}
		v2, v3 = a139, b139
	}
	return (v0*1000 + v1*100 + v2*10 + v3) % 100000, nil
}

func main() {
	for seed := 0; seed < 10; seed++ {
		v, err := run(seed)
		fmt.Println("run", seed, v, err, errors.Is(err, ErrBad))
	}
}
