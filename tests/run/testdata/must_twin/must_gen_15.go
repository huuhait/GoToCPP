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
		if e := f3(13); err == nil {
			err = e
		}
	}()
	{
		t101, err := f1(v0)
		if err != nil {
			return 0, err
		}
		t102, err := f1(v2)
		if err != nil {
			return 0, err
		}
		switch (t101 + t102) % 3 {
		case 0:
			{
				t103, err := f1(v1)
				if err != nil {
					return 0, err
				}
				if (9 + v0) < t103 {
					{
						t104, err := f1(3)
						if err != nil {
							return 0, err
						}
						v3 = (t104 + (v3 - v2))
					}
					{
						v2 += v1
					}
					{
						a105, b105, err := f2((v2 - v0))
						if err != nil {
							return 0, err
						}
						v1, v0 = a105, b105
					}
				} else {
					t106, err := f1(0)
					if err != nil {
						return 0, err
					}
					t107, err := b1(t106)
					if err != nil {
						return 0, err
					}
					if t107 {
						{
							t108, err := f1(v3)
							if err != nil {
								return 0, err
							}
							t109, err := b1(t108)
							if err != nil {
								return 0, err
							}
							if t109 {
								{
									t110, err := f1(v3)
									if err != nil {
										return 0, err
									}
									v2 = t110
								}
								{
									t111, err := f1(v3)
									if err != nil {
										return 0, err
									}
									v2 = t111
								}
							} else {
								{
									if err := f3(v3); err != nil {
										return 0, err
									}
								}
								{
									v0 = 9
								}
							}
						}
					} else {
						t112, err := b1((v0 + v3))
						if err != nil {
							return 0, err
						}
						if t112 {
							{
								t113, err := f1(5)
								if err != nil {
									return 0, err
								}
								if err := f3(t113); err != nil {
									return 0, err
								}
							}
							{
								for i1 := 0; ; i1++ {
									if !(i1 < 1) {
										break
									}
									t114, err := f1(8)
									if err != nil {
										return 0, err
									}
									c115 := t114 < v0
									if c115 {
										t116, err := b1(v1)
										if err != nil {
											return 0, err
										}
										c115 = t116
									}
									if !(c115) {
										break
									}
									{
										if err := f3(v2); err != nil {
											return 0, err
										}
									}
									{
										if (v2 + 8) < 3 {
											continue
										}
									}
								}
							}
							{
								t117, err := f1(v3)
								if err != nil {
									return 0, err
								}
								v3 = t117
							}
						} else {
							{
								t118, err := f1(v1)
								if err != nil {
									return 0, err
								}
								v0 = t118
							}
							{
								for i2 := 0; ; i2++ {
									if !(i2 < 3) {
										break
									}
									{
										t119, err := f1(v3)
										if err != nil {
											return 0, err
										}
										a120, b120, err := f2(t119)
										if err != nil {
											return 0, err
										}
										v0, v3 = a120, b120
									}
								}
							}
							{
								t121, err := b1((v1 - 0))
								if err != nil {
									return 0, err
								}
								if t121 {
									{
										v1 = v2
									}
								} else {
									t122, err := f1(5)
									if err != nil {
										return 0, err
									}
									t123, err := b1(t122)
									if err != nil {
										return 0, err
									}
									if t123 {
										{
											t124, err := f1(v3)
											if err != nil {
												return 0, err
											}
											t125, err := f1(t124)
											if err != nil {
												return 0, err
											}
											v1 = t125
										}
										{
											v3 = 2
										}
										{
											t126, err := f1(v0)
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
					}
				}
			}
		case 1:
			{
				t128, err := f1(v1)
				if err != nil {
					return 0, err
				}
				a129, b129, err := f2(t128)
				if err != nil {
					return 0, err
				}
				v1, v0 = a129, b129
			}
			{
				t131, err := f1(v0)
				if err != nil {
					return 0, err
				}
				t132, err := b1(t131)
				if err != nil {
					return 0, err
				}
				c133 := t132
				if c133 {
					t134, err := f1(v1)
					if err != nil {
						return 0, err
					}
					t135, err := b1(t134)
					if err != nil {
						return 0, err
					}
					c133 = t135
				}
				c136 := c133
				if c136 {
					t137, err := b1(v0)
					if err != nil {
						return 0, err
					}
					c138 := t137
					if c138 {
						t139, err := b1(v0)
						if err != nil {
							return 0, err
						}
						c138 = t139
					}
					c136 = c138
				}
				if c136 {
					{
						t140, err := f1(v1)
						if err != nil {
							return 0, err
						}
						v2 += t140
					}
					{
						t141, err := b1(v2)
						if err != nil {
							return 0, err
						}
						c142 := t141
						if !c142 {
							t143, err := b1(v0)
							if err != nil {
								return 0, err
							}
							c142 = t143
						}
						if c142 {
							{
								t144, err := f1(v3)
								if err != nil {
									return 0, err
								}
								t145, err := f1(t144)
								if err != nil {
									return 0, err
								}
								v3 = t145
							}
							{
								t146, err := f1(v0)
								if err != nil {
									return 0, err
								}
								t147, err := f1(t146)
								if err != nil {
									return 0, err
								}
								v3 += t147
							}
						} else {
							t148, err := b1(v1)
							if err != nil {
								return 0, err
							}
							if t148 {
								{
									t149, err := f1(v3)
									if err != nil {
										return 0, err
									}
									v3 += ((1 - v3) + t149)
								}
							} else {
								t150, err := b1(v3)
								if err != nil {
									return 0, err
								}
								c151 := t150
								if c151 {
									t152, err := f1(v2)
									if err != nil {
										return 0, err
									}
									t153, err := b1(t152)
									if err != nil {
										return 0, err
									}
									c151 = t153
								}
								if c151 {
									{
										v3 += v3
									}
								}
							}
						}
					}
				} else {
					t154, err := f1(v1)
					if err != nil {
						return 0, err
					}
					t155, err := b1(t154)
					if err != nil {
						return 0, err
					}
					if t155 {
						{
							for i3 := 0; ; i3++ {
								if !(i3 < 3) {
									break
								}
								t156, err := f1(v0)
								if err != nil {
									return 0, err
								}
								t157, err := b1(t156)
								if err != nil {
									return 0, err
								}
								if !(t157) {
									break
								}
								{
									a158, b158, err := f2(v2)
									if err != nil {
										return 0, err
									}
									v0, v2 = a158, b158
								}
								{
									if 3 < 3 {
										break
									}
								}
							}
						}
					} else {
						t159, err := b1((v3 + v0))
						if err != nil {
							return 0, err
						}
						if t159 {
							{
								t160, err := f1(v0)
								if err != nil {
									return 0, err
								}
								v3 += t160
							}
						}
					}
				}
			}
			{
				t161, err := b1((v0 - v0))
				if err != nil {
					return 0, err
				}
				if t161 {
					{
						t162, err := f1(v3)
						if err != nil {
							return 0, err
						}
						switch (t162 - 3) % 3 {
						case 0:
							{
								v2 = (v0 - 6)
							}
						case 1:
							{
								t163, err := f1(7)
								if err != nil {
									return 0, err
								}
								t164, err := f1(t163)
								if err != nil {
									return 0, err
								}
								v1 += t164
							}
							{
								v0 = (v0 + (v3 - v1))
							}
							{
								t165, err := f1(v3)
								if err != nil {
									return 0, err
								}
								a166, b166, err := f2(t165)
								if err != nil {
									return 0, err
								}
								v1, v2 = a166, b166
							}
						default:
							{
								t167, err := f1(v1)
								if err != nil {
									return 0, err
								}
								v3 = (t167 + (v0 - v0))
							}
							{
								t168, err := f1(v1)
								if err != nil {
									return 0, err
								}
								a169, b169, err := f2(t168)
								if err != nil {
									return 0, err
								}
								v1, v3 = a169, b169
							}
							{
								if err := f3((v1 - v0)); err != nil {
									return 0, err
								}
							}
						}
					}
					{
						v2 = v2
					}
				} else {
					t170, err := b1(1)
					if err != nil {
						return 0, err
					}
					if !(t170) {
						{
							t171, err := f1(v0)
							if err != nil {
								return 0, err
							}
							t172, err := b1(t171)
							if err != nil {
								return 0, err
							}
							if t172 {
								{
									a173, b173, err := f2((7 + 8))
									if err != nil {
										return 0, err
									}
									v2, v3 = a173, b173
								}
								{
									t174, err := f1((6 + v2))
									if err != nil {
										return 0, err
									}
									v3 += t174
								}
							} else {
								t175, err := b1((5 - 1))
								if err != nil {
									return 0, err
								}
								if t175 {
									{
										v3 = v2
									}
									{
										t176, err := f1(v1)
										if err != nil {
											return 0, err
										}
										t177, err := f1(t176)
										if err != nil {
											return 0, err
										}
										v2 += t177
									}
									{
										if err := f3(v1); err != nil {
											return 0, err
										}
									}
								} else {
									t178, err := b1((v0 + 5))
									if err != nil {
										return 0, err
									}
									if t178 {
										{
											t179, err := f1(v2)
											if err != nil {
												return 0, err
											}
											v1 = t179
										}
									} else {
										{
											t180, err := f1(v0)
											if err != nil {
												return 0, err
											}
											if err := f3(t180); err != nil {
												return 0, err
											}
										}
										{
											t181, err := f1(1)
											if err != nil {
												return 0, err
											}
											v3 += t181
										}
										{
											t182, err := f1(v1)
											if err != nil {
												return 0, err
											}
											v2 = (v0 + t182)
										}
									}
								}
							}
						}
						{
							t183, err := f1(v2)
							if err != nil {
								return 0, err
							}
							t184, err := b1(t183)
							if err != nil {
								return 0, err
							}
							if t184 {
								{
									t185, err := f1(v0)
									if err != nil {
										return 0, err
									}
									t186, err := f1(t185)
									if err != nil {
										return 0, err
									}
									v3 = t186
								}
								{
									t187, err := f1(4)
									if err != nil {
										return 0, err
									}
									t188, err := f1(t187)
									if err != nil {
										return 0, err
									}
									v1 = t188
								}
							} else {
								t189, err := b1(v3)
								if err != nil {
									return 0, err
								}
								c190 := (t189 || (v3 + v1) < (v2 + v3))
								if !c190 {
									t191, err := b1(4)
									if err != nil {
										return 0, err
									}
									c190 = t191
								}
								if c190 {
									{
										if err := f3(v3); err != nil {
											return 0, err
										}
									}
									{
										if err := f3((2 - v2)); err != nil {
											return 0, err
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
								t192, err := b1(v0)
								if err != nil {
									return 0, err
								}
								if !(t192) {
									break
								}
								{
									if v2 < 3 {
										continue
									}
								}
								{
									t193, err := f1(v1)
									if err != nil {
										return 0, err
									}
									v3 = (t193 + v0)
								}
								{
									if (v1 - v0) < 6 {
										break
									}
								}
							}
						}
					} else {
						{
							t194, err := f1(v0)
							if err != nil {
								return 0, err
							}
							v3 += t194
						}
						{
							for i5 := 0; ; i5++ {
								if !(i5 < 1) {
									break
								}
								{
									t195, err := f1(v1)
									if err != nil {
										return 0, err
									}
									if t195 < 6 {
										continue
									}
								}
							}
						}
						{
							a196, b196, err := f2((6 + v0))
							if err != nil {
								return 0, err
							}
							v2, v1 = a196, b196
						}
					}
				}
			}
		default:
			{
				t197, err := b1((v2 + 6))
				if err != nil {
					return 0, err
				}
				if t197 {
					{
						v0 += v0
					}
				} else {
					t198, err := b1(v2)
					if err != nil {
						return 0, err
					}
					if t198 {
						{
							t199, err := f1(v1)
							if err != nil {
								return 0, err
							}
							if !(!(t199 < (7 - 0))) {
								{
									t200, err := f1((v0 + v2))
									if err != nil {
										return 0, err
									}
									v3 = t200
								}
								{
									v3 = v0
								}
								{
									t201, err := f1((v2 + 8))
									if err != nil {
										return 0, err
									}
									v3 = t201
								}
							} else {
								t202, err := b1(v3)
								if err != nil {
									return 0, err
								}
								if t202 {
									{
										t203, err := f1(v3)
										if err != nil {
											return 0, err
										}
										a204, b204, err := f2(t203)
										if err != nil {
											return 0, err
										}
										v1, v0 = a204, b204
									}
									{
										v0 = v2
									}
									{
										v1 += v3
									}
								} else {
									{
										v3 += v2
									}
									{
										t205, err := f1(v0)
										if err != nil {
											return 0, err
										}
										v2 = t205
									}
								}
							}
						}
					} else {
						{
							t206, err := f1(v2)
							if err != nil {
								return 0, err
							}
							t207, err := f1(5)
							if err != nil {
								return 0, err
							}
							switch (t206 - t207) % 3 {
							case 0:
								{
									v1 = v1
								}
							case 1:
								{
									t208, err := f1(v0)
									if err != nil {
										return 0, err
									}
									if err := f3(t208); err != nil {
										return 0, err
									}
								}
							default:
								{
									t209, err := f1(v2)
									if err != nil {
										return 0, err
									}
									t210, err := f1(t209)
									if err != nil {
										return 0, err
									}
									v1 = t210
								}
								{
									if err := f3(0); err != nil {
										return 0, err
									}
								}
								{
									a211, b211, err := f2(v3)
									if err != nil {
										return 0, err
									}
									v3, v1 = a211, b211
								}
							}
						}
					}
				}
			}
		}
	}
	{
		t212, err := f1(v2)
		if err != nil {
			return 0, err
		}
		v2 = t212
	}
	{
		t213, err := f1(v2)
		if err != nil {
			return 0, err
		}
		a214, b214, err := f2(t213)
		if err != nil {
			return 0, err
		}
		v2, v1 = a214, b214
	}
	return (v0*1000 + v1*100 + v2*10 + v3) % 100000, nil
}

func main() {
	for seed := 0; seed < 10; seed++ {
		v, err := run(seed)
		fmt.Println("run", seed, v, err, errors.Is(err, ErrBad))
	}
}
