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
		if (v3 + v0) < (v1 - v2) {
			{
				a101, b101, err := f2((v3 + v2))
				if err != nil {
					return 0, err
				}
				v2, v3 = a101, b101
			}
		} else {
			{
				for i1 := 0; ; i1++ {
					if !(i1 < 2) {
						break
					}
					{
						if 1 < (v1 + 7) {
							{
								t102, err := f1(9)
								if err != nil {
									return 0, err
								}
								if t102 < 5 {
									continue
								}
							}
							{
								if (v1 - v0) < 7 {
									break
								}
							}
							{
								t103, err := f1(v0)
								if err != nil {
									return 0, err
								}
								v0 = ((v0 + 8) + t103)
							}
						} else {
							t104, err := f1(9)
							if err != nil {
								return 0, err
							}
							t105, err := b1(t104)
							if err != nil {
								return 0, err
							}
							if t105 {
								{
									if (3 + v2) < 5 {
										continue
									}
								}
							} else {
								{
									t106, err := f1(v2)
									if err != nil {
										return 0, err
									}
									if t106 < 3 {
										continue
									}
								}
							}
						}
					}
				}
			}
			{
				t108, err := f1(v2)
				if err != nil {
					return 0, err
				}
				t109, err := b1(t108)
				if err != nil {
					return 0, err
				}
				if t109 {
					{
						v1 = 0
					}
				} else {
					t110, err := b1(2)
					if err != nil {
						return 0, err
					}
					c111 := t110
					if c111 {
						t112, err := b1(8)
						if err != nil {
							return 0, err
						}
						c111 = t112
					}
					if c111 {
						{
							t114, err := b1((v0 + v3))
							if err != nil {
								return 0, err
							}
							if t114 {
								{
									t115, err := f1(v2)
									if err != nil {
										return 0, err
									}
									v1 = t115
								}
							} else {
								t116, err := b1((v1 - 7))
								if err != nil {
									return 0, err
								}
								if t116 {
									{
										t117, err := f1((v2 + v3))
										if err != nil {
											return 0, err
										}
										v2 += t117
									}
									{
										if err := f3((v2 - v3)); err != nil {
											return 0, err
										}
									}
									{
										t118, err := f1(v0)
										if err != nil {
											return 0, err
										}
										if err := f3(t118); err != nil {
											return 0, err
										}
									}
								}
							}
						}
						{
							v1 = v3
						}
						{
							t120, err := f1(v3)
							if err != nil {
								return 0, err
							}
							c121 := v0 < t120
							if c121 {
								t122, err := b1(5)
								if err != nil {
									return 0, err
								}
								c121 = t122
							}
							if c121 {
								{
									t123, err := f1(v3)
									if err != nil {
										return 0, err
									}
									t124, err := f1(t123)
									if err != nil {
										return 0, err
									}
									v3 = t124
								}
							} else {
								t125, err := b1((9 - v3))
								if err != nil {
									return 0, err
								}
								c126 := t125
								if c126 {
									t127, err := f1(v2)
									if err != nil {
										return 0, err
									}
									t128, err := b1(t127)
									if err != nil {
										return 0, err
									}
									c126 = t128
								}
								c129 := c126
								if c129 {
									t130, err := f1(v1)
									if err != nil {
										return 0, err
									}
									t131, err := b1(t130)
									if err != nil {
										return 0, err
									}
									c129 = (t131 || 9 < (v0 - v0))
								}
								if c129 {
									{
										t132, err := f1(2)
										if err != nil {
											return 0, err
										}
										v2 = t132
									}
									{
										t133, err := f1(1)
										if err != nil {
											return 0, err
										}
										v2 = ((v0 + v0) + t133)
									}
									{
										t134, err := f1(5)
										if err != nil {
											return 0, err
										}
										v0 = t134
									}
								}
							}
						}
					}
				}
			}
			{
				t135, err := b1((v0 - 1))
				if err != nil {
					return 0, err
				}
				if t135 {
					{
						t136, err := f1(v2)
						if err != nil {
							return 0, err
						}
						v0 += (t136 - (v1 + v2))
					}
					{
						t137, err := f1(v1)
						if err != nil {
							return 0, err
						}
						t138, err := b1(t137)
						if err != nil {
							return 0, err
						}
						if t138 {
							{
								t139, err := f1(v0)
								if err != nil {
									return 0, err
								}
								v1 = t139
							}
							{
								t140, err := f1(v3)
								if err != nil {
									return 0, err
								}
								v1 = ((v2 + 4) + t140)
							}
						} else {
							t141, err := b1(v3)
							if err != nil {
								return 0, err
							}
							if t141 {
								{
									t142, err := f1(v3)
									if err != nil {
										return 0, err
									}
									v0 = t142
								}
								{
									v1 = v3
								}
							} else {
								t143, err := f1(v2)
								if err != nil {
									return 0, err
								}
								t144, err := b1(t143)
								if err != nil {
									return 0, err
								}
								if t144 {
									{
										if err := f3(v3); err != nil {
											return 0, err
										}
									}
								} else {
									{
										a145, b145, err := f2((v3 + v1))
										if err != nil {
											return 0, err
										}
										v0, v3 = a145, b145
									}
								}
							}
						}
					}
					{
						v2 += v3
					}
				}
			}
		}
	}
	{
		v1 = 4
	}
	{
		for i2 := 0; ; i2++ {
			if !(i2 < 2) {
				break
			}
			{
				t146, err := f1(v2)
				if err != nil {
					return 0, err
				}
				if t146 < 6 {
					continue
				}
			}
			{
				t147, err := f1(v1)
				if err != nil {
					return 0, err
				}
				if t147 < 5 {
					break
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
