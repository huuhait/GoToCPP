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
		if e := f3(25); err == nil {
			err = e
		}
	}()
	defer func() {
		if e := f3(21); err == nil {
			err = e
		}
	}()
	{
		for i1 := 0; ; i1++ {
			if !(i1 < 2) {
				break
			}
			{
				t101, err := f1(v1)
				if err != nil {
					return 0, err
				}
				v0 += t101
			}
			{
				v0 = (v2 + 6)
			}
			{
				t102, err := b1(4)
				if err != nil {
					return 0, err
				}
				if !(t102) {
					{
						t103, err := f1(v1)
						if err != nil {
							return 0, err
						}
						a104, b104, err := f2(t103)
						if err != nil {
							return 0, err
						}
						v2, v3 = a104, b104
					}
				} else {
					t105, err := f1(v0)
					if err != nil {
						return 0, err
					}
					c106 := t105 < (v0 + v1)
					if c106 {
						t107, err := f1(8)
						if err != nil {
							return 0, err
						}
						t108, err := b1(t107)
						if err != nil {
							return 0, err
						}
						c106 = t108
					}
					if c106 {
						{
							for i2 := 0; ; i2++ {
								if !(i2 < 2) {
									break
								}
								t109, err := f1(1)
								if err != nil {
									return 0, err
								}
								if !((v1 < t109 && ((v3 - 7) < (v2 + v0) || v3 < (v3 - v3)))) {
									break
								}
								{
									t110, err := f1(v3)
									if err != nil {
										return 0, err
									}
									t111, err := f1(t110)
									if err != nil {
										return 0, err
									}
									v0 += t111
								}
								{
									if (v3 + 1) < 4 {
										break
									}
								}
							}
						}
					} else {
						t112, err := f1(v0)
						if err != nil {
							return 0, err
						}
						t113, err := b1(t112)
						if err != nil {
							return 0, err
						}
						if t113 {
							{
								t114, err := b1((v2 - v3))
								if err != nil {
									return 0, err
								}
								if t114 {
									{
										if (v2 - 2) < 5 {
											continue
										}
									}
								} else {
									if (8 - v1) < v2 {
										{
											t115, err := f1(v0)
											if err != nil {
												return 0, err
											}
											t116, err := f1(t115)
											if err != nil {
												return 0, err
											}
											v3 = t116
										}
									} else {
										if v2 < (v3 - v3) {
											{
												if v2 < 5 {
													break
												}
											}
										}
									}
								}
							}
						} else {
							{
								for i3 := 0; ; i3++ {
									if !(i3 < 2) {
										break
									}
									{
										v1 = v1
									}
								}
							}
							{
								t117, err := f1(v2)
								if err != nil {
									return 0, err
								}
								if t117 < 7 {
									continue
								}
							}
						}
					}
				}
			}
		}
	}
	{
		t118, err := f1(v3)
		if err != nil {
			return 0, err
		}
		a119, b119, err := f2(t118)
		if err != nil {
			return 0, err
		}
		v3, v0 = a119, b119
	}
	{
		for i4 := 0; ; i4++ {
			if !(i4 < 2) {
				break
			}
			t120, err := f1(v2)
			if err != nil {
				return 0, err
			}
			t121, err := b1(t120)
			if err != nil {
				return 0, err
			}
			c122 := t121
			if !c122 {
				t123, err := f1(v2)
				if err != nil {
					return 0, err
				}
				t124, err := f1(8)
				if err != nil {
					return 0, err
				}
				c122 = t123 < t124
			}
			if !(c122) {
				break
			}
			{
				for i5 := 0; ; i5++ {
					if !(i5 < 3) {
						break
					}
					t125, err := b1((v2 - v2))
					if err != nil {
						return 0, err
					}
					if !(t125) {
						break
					}
					{
						t126, err := f1(5)
						if err != nil {
							return 0, err
						}
						t127, err := f1(v2)
						if err != nil {
							return 0, err
						}
						v2 = (t126 - t127)
					}
					{
						t128, err := f1((v0 + v0))
						if err != nil {
							return 0, err
						}
						switch t128 % 3 {
						case 0:
							{
								v0 = 9
							}
							{
								t129, err := f1(9)
								if err != nil {
									return 0, err
								}
								if t129 < 6 {
									break
								}
							}
						case 1:
							{
								t130, err := f1(v3)
								if err != nil {
									return 0, err
								}
								a131, b131, err := f2(t130)
								if err != nil {
									return 0, err
								}
								v2, v3 = a131, b131
							}
							{
								t132, err := f1(v2)
								if err != nil {
									return 0, err
								}
								if t132 < 7 {
									continue
								}
							}
						default:
							{
								t133, err := f1(3)
								if err != nil {
									return 0, err
								}
								if t133 < 7 {
									break
								}
							}
							{
								if err := f3(v2); err != nil {
									return 0, err
								}
							}
						}
					}
				}
			}
			{
				switch ((0 - 0) - v2) % 3 {
				case 0:
					{
						t134, err := f1(v1)
						if err != nil {
							return 0, err
						}
						v2 = t134
					}
					{
						t135, err := f1(3)
						if err != nil {
							return 0, err
						}
						v2 = t135
					}
				case 1:
					{
						if v1 < 7 {
							break
						}
					}
					{
						for i6 := 0; ; i6++ {
							if !(i6 < 1) {
								break
							}
							{
								if (v1 - v0) < 4 {
									break
								}
							}
							{
								v0 = v2
							}
						}
					}
				default:
					{
						switch v3 % 3 {
						case 0:
							{
								if v1 < 5 {
									break
								}
							}
							{
								t136, err := f1(v1)
								if err != nil {
									return 0, err
								}
								v3 += (t136 - v1)
							}
						case 1:
							{
								v0 = 2
							}
						default:
							{
								t137, err := f1(v2)
								if err != nil {
									return 0, err
								}
								if err := f3(t137); err != nil {
									return 0, err
								}
							}
						}
					}
					{
						v3 = v2
					}
				}
			}
			{
				for i7 := 0; ; i7++ {
					if !(i7 < 3) {
						break
					}
					c138 := !((4 - v0) < 1)
					if c138 {
						t139, err := b1(v2)
						if err != nil {
							return 0, err
						}
						c138 = t139
					}
					if !(c138) {
						break
					}
					{
						t141, err := f1(1)
						if err != nil {
							return 0, err
						}
						t142, err := f1(v0)
						if err != nil {
							return 0, err
						}
						if t141 < t142 {
							{
								t143, err := f1(3)
								if err != nil {
									return 0, err
								}
								if t143 < 6 {
									goto sw140
								}
							}
							{
								t144, err := f1(v1)
								if err != nil {
									return 0, err
								}
								v1 = t144
							}
						} else {
							t145, err := f1(6)
							if err != nil {
								return 0, err
							}
							if t145 < 3 {
								{
									v1 = v0
								}
								{
									if v1 < 3 {
										goto sw140
									}
								}
								{
									a146, b146, err := f2((v2 + 5))
									if err != nil {
										return 0, err
									}
									v1, v0 = a146, b146
								}
							} else {
								t147, err := f1(9)
								if err != nil {
									return 0, err
								}
								t148, err := f1(v3)
								if err != nil {
									return 0, err
								}
								if t147 < t148 {
									{
										t149, err := f1(v2)
										if err != nil {
											return 0, err
										}
										a150, b150, err := f2(t149)
										if err != nil {
											return 0, err
										}
										v1, v2 = a150, b150
									}
								}
							}
						}
					}
					sw140:
					{
					}
					{
						for i8 := 0; ; i8++ {
							if !(i8 < 2) {
								break
							}
							t151, err := b1(v2)
							if err != nil {
								return 0, err
							}
							if !(t151) {
								break
							}
							{
								t152, err := f1(v1)
								if err != nil {
									return 0, err
								}
								if t152 < 6 {
									continue
								}
							}
						}
					}
					{
						switch v1 % 3 {
						case 0:
							{
								t153, err := f1(v3)
								if err != nil {
									return 0, err
								}
								a154, b154, err := f2(t153)
								if err != nil {
									return 0, err
								}
								v2, v1 = a154, b154
							}
							{
								t155, err := f1(v0)
								if err != nil {
									return 0, err
								}
								if err := f3(t155); err != nil {
									return 0, err
								}
							}
							{
								t156, err := f1(v2)
								if err != nil {
									return 0, err
								}
								if t156 < 5 {
									break
								}
							}
						case 1:
							{
								if err := f3(9); err != nil {
									return 0, err
								}
							}
							{
								t157, err := f1(0)
								if err != nil {
									return 0, err
								}
								v1 = ((v1 + v1) + t157)
							}
							{
								t158, err := f1(v0)
								if err != nil {
									return 0, err
								}
								v3 += (t158 - (v0 + 0))
							}
						default:
							{
								v0 += (v0 + v2)
							}
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
