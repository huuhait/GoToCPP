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
		v0 = v3
	}
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
				t103, err := f1(v3)
				if err != nil {
					return 0, err
				}
				t104, err := f1(t103)
				if err != nil {
					return 0, err
				}
				switch t104 % 3 {
				case 0:
					{
						if err := f3(v2); err != nil {
							return 0, err
						}
					}
				case 1:
					{
						t105, err := f1(0)
						if err != nil {
							return 0, err
						}
						switch (v0 - t105) % 3 {
						case 0:
							{
								t106, err := f1(v2)
								if err != nil {
									return 0, err
								}
								v3 = (v3 - t106)
							}
							{
								v2 = 5
							}
						case 1:
							{
								a107, b107, err := f2((v3 - v1))
								if err != nil {
									return 0, err
								}
								v3, v1 = a107, b107
							}
							{
								v1 = (v2 - 3)
							}
						default:
							{
								t108, err := f1(v3)
								if err != nil {
									return 0, err
								}
								t109, err := f1(t108)
								if err != nil {
									return 0, err
								}
								v1 = t109
							}
							{
								t110, err := f1(v2)
								if err != nil {
									return 0, err
								}
								if err := f3(t110); err != nil {
									return 0, err
								}
							}
						}
					}
					{
						v3 = v0
					}
				default:
					{
						t111, err := f1(v3)
						if err != nil {
							return 0, err
						}
						a112, b112, err := f2(t111)
						if err != nil {
							return 0, err
						}
						v3, v2 = a112, b112
					}
					{
						t113, err := f1(v1)
						if err != nil {
							return 0, err
						}
						t114, err := b1(t113)
						if err != nil {
							return 0, err
						}
						if t114 {
							{
								t115, err := f1(v0)
								if err != nil {
									return 0, err
								}
								if err := f3(t115); err != nil {
									return 0, err
								}
							}
							{
								v1 = v3
							}
						} else {
							{
								v1 += v0
							}
							{
								t116, err := f1(v1)
								if err != nil {
									return 0, err
								}
								t117, err := f1(t116)
								if err != nil {
									return 0, err
								}
								v2 += t117
							}
						}
					}
				}
			}
		} else {
			t118, err := f1(v3)
			if err != nil {
				return 0, err
			}
			t119, err := b1(t118)
			if err != nil {
				return 0, err
			}
			c120 := t119
			if c120 {
				t121, err := f1(v0)
				if err != nil {
					return 0, err
				}
				c120 = t121 < v1
			}
			if !(c120) {
				{
					t122, err := b1(1)
					if err != nil {
						return 0, err
					}
					if t122 {
						{
							t123, err := f1((v2 - v0))
							if err != nil {
								return 0, err
							}
							v0 = t123
						}
					} else {
						t124, err := f1(v3)
						if err != nil {
							return 0, err
						}
						if (v1 + 7) < t124 {
							{
								t125, err := f1(0)
								if err != nil {
									return 0, err
								}
								v3 = t125
							}
							{
								t126, err := f1(1)
								if err != nil {
									return 0, err
								}
								a127, b127, err := f2(t126)
								if err != nil {
									return 0, err
								}
								v2, v0 = a127, b127
							}
							{
								t128, err := f1(v0)
								if err != nil {
									return 0, err
								}
								t129, err := f1(t128)
								if err != nil {
									return 0, err
								}
								v1 += t129
							}
						}
					}
				}
			} else {
				t130, err := b1(v1)
				if err != nil {
					return 0, err
				}
				if t130 {
					{
						t131, err := f1(v2)
						if err != nil {
							return 0, err
						}
						if !((0 - 4) < t131) {
							{
								t132, err := f1(2)
								if err != nil {
									return 0, err
								}
								switch (v1 - t132) % 3 {
								case 0:
									{
										t133, err := f1(v1)
										if err != nil {
											return 0, err
										}
										t134, err := f1(t133)
										if err != nil {
											return 0, err
										}
										v1 = t134
									}
									{
										t135, err := f1(2)
										if err != nil {
											return 0, err
										}
										v1 = t135
									}
								case 1:
									{
										t136, err := f1((3 + v3))
										if err != nil {
											return 0, err
										}
										v3 += t136
									}
									{
										t137, err := f1(v2)
										if err != nil {
											return 0, err
										}
										if err := f3(t137); err != nil {
											return 0, err
										}
									}
									{
										a138, b138, err := f2((v3 - v1))
										if err != nil {
											return 0, err
										}
										v1, v0 = a138, b138
									}
								default:
									{
										v0 = (v3 + 0)
									}
									{
										t139, err := f1(v2)
										if err != nil {
											return 0, err
										}
										v1 = t139
									}
									{
										t140, err := f1(v1)
										if err != nil {
											return 0, err
										}
										v2 = t140
									}
								}
							}
							{
								v1 = v3
							}
						}
					}
					{
						t141, err := f1((v3 - v1))
						if err != nil {
							return 0, err
						}
						v1 += t141
					}
					{
						v1 += v1
					}
				}
			}
		}
	}
	{
		t142, err := f1(4)
		if err != nil {
			return 0, err
		}
		v1 = t142
	}
	return (v0*1000 + v1*100 + v2*10 + v3) % 100000, nil
}

func main() {
	for seed := 0; seed < 10; seed++ {
		v, err := run(seed)
		fmt.Println("run", seed, v, err, errors.Is(err, ErrBad))
	}
}
