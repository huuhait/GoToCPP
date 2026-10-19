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
		t102, err := f1(v0)
		if err != nil {
			return 0, err
		}
		t103, err := b1(t102)
		if err != nil {
			return 0, err
		}
		if t103 {
			{
				v0 = v2
			}
			{
				t104, err := b1((v1 - v0))
				if err != nil {
					return 0, err
				}
				if t104 {
					{
						v1 = v1
					}
					{
						t105, err := b1(v3)
						if err != nil {
							return 0, err
						}
						if t105 {
							{
								t106, err := f1(9)
								if err != nil {
									return 0, err
								}
								v1 += t106
							}
						} else {
							t107, err := f1(v2)
							if err != nil {
								return 0, err
							}
							t108, err := b1(t107)
							if err != nil {
								return 0, err
							}
							if t108 {
								{
									v1 = (v2 + 9)
								}
								{
									v3 = (v1 - 9)
								}
							} else {
								{
									v1 = v0
								}
								{
									t109, err := f1((v1 + v0))
									if err != nil {
										return 0, err
									}
									v1 += t109
								}
							}
						}
					}
				}
			}
			{
				t110, err := f1(v0)
				if err != nil {
					return 0, err
				}
				v0 = (v1 - t110)
			}
		} else {
			t111, err := b1(v0)
			if err != nil {
				return 0, err
			}
			if t111 {
				{
					for i1 := 0; ; i1++ {
						if !(i1 < 3) {
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
							for i2 := 0; ; i2++ {
								if !(i2 < 1) {
									break
								}
								t114, err := b1(v0)
								if err != nil {
									return 0, err
								}
								if !(!(t114)) {
									break
								}
								{
									v2 += v0
								}
								{
									v3 += 2
								}
								{
									if (v1 + v3) < 6 {
										continue
									}
								}
							}
						}
						{
							t115, err := f1(2)
							if err != nil {
								return 0, err
							}
							t116, err := f1(t115)
							if err != nil {
								return 0, err
							}
							v1 = t116
						}
					}
				}
				{
					if err := f3((v0 + v0)); err != nil {
						return 0, err
					}
				}
			} else {
				{
					a117, b117, err := f2(6)
					if err != nil {
						return 0, err
					}
					v1, v2 = a117, b117
				}
			}
		}
	}
	{
		v2 = 7
	}
	{
		t118, err := f1(v2)
		if err != nil {
			return 0, err
		}
		if err := f3(t118); err != nil {
			return 0, err
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
