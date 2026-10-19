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
		if e := f3(9); err == nil {
			err = e
		}
	}()
	defer func() {
		if e := f3(8); err == nil {
			err = e
		}
	}()
	{
		c101 := v1 < 3
		if c101 {
			t102, err := b1(v0)
			if err != nil {
				return 0, err
			}
			c101 = t102
		}
		if !(c101) {
			{
				for i1 := 0; ; i1++ {
					if !(i1 < 1) {
						break
					}
					t103, err := b1(v2)
					if err != nil {
						return 0, err
					}
					if !(!(t103)) {
						break
					}
					{
						for i2 := 0; ; i2++ {
							if !(i2 < 2) {
								break
							}
							t104, err := f1(v0)
							if err != nil {
								return 0, err
							}
							t105, err := b1(t104)
							if err != nil {
								return 0, err
							}
							c106 := t105
							if !c106 {
								t107, err := b1(v0)
								if err != nil {
									return 0, err
								}
								c106 = t107
							}
							c108 := c106
							if !c108 {
								t109, err := b1(9)
								if err != nil {
									return 0, err
								}
								c108 = t109
							}
							if !(c108) {
								break
							}
							{
								t110, err := f1(v2)
								if err != nil {
									return 0, err
								}
								a111, b111, err := f2(t110)
								if err != nil {
									return 0, err
								}
								v3, v1 = a111, b111
							}
							{
								v0 += v3
							}
							{
								t112, err := f1(v1)
								if err != nil {
									return 0, err
								}
								t113, err := f1(v3)
								if err != nil {
									return 0, err
								}
								v0 = (t112 - t113)
							}
						}
					}
					{
						a114, b114, err := f2(0)
						if err != nil {
							return 0, err
						}
						v3, v2 = a114, b114
					}
					{
						for i3 := 0; ; i3++ {
							if !(i3 < 2) {
								break
							}
							t115, err := f1(v0)
							if err != nil {
								return 0, err
							}
							t116, err := b1(t115)
							if err != nil {
								return 0, err
							}
							c117 := t116
							if !c117 {
								t118, err := b1((v2 - v1))
								if err != nil {
									return 0, err
								}
								c117 = t118
							}
							if !(c117) {
								break
							}
							{
								t119, err := f1(v3)
								if err != nil {
									return 0, err
								}
								if t119 < 4 {
									break
								}
							}
						}
					}
				}
			}
			{
				t120, err := b1((v1 - v1))
				if err != nil {
					return 0, err
				}
				if t120 {
					{
						v1 = v3
					}
					{
						t121, err := f1(3)
						if err != nil {
							return 0, err
						}
						v2 = t121
					}
				} else {
					t122, err := b1((v3 + v3))
					if err != nil {
						return 0, err
					}
					if t122 {
						{
							for i4 := 0; ; i4++ {
								if !(i4 < 2) {
									break
								}
								t123, err := b1((v1 + v2))
								if err != nil {
									return 0, err
								}
								c124 := t123
								if c124 {
									t125, err := b1(v3)
									if err != nil {
										return 0, err
									}
									c124 = t125
								}
								if !(c124) {
									break
								}
								{
									v0 = (v2 + (v1 - v0))
								}
								{
									if 3 < 3 {
										continue
									}
								}
								{
									v3 = ((v0 + v1) - (v2 + v0))
								}
							}
						}
						{
							for i5 := 0; ; i5++ {
								if !(i5 < 3) {
									break
								}
								t126, err := f1(v0)
								if err != nil {
									return 0, err
								}
								t127, err := b1(t126)
								if err != nil {
									return 0, err
								}
								if !(t127) {
									break
								}
								{
									v1 += v3
								}
								{
									t128, err := f1(v2)
									if err != nil {
										return 0, err
									}
									v2 = (t128 + (v1 + 5))
								}
								{
									t129, err := f1(v2)
									if err != nil {
										return 0, err
									}
									v2 = t129
								}
							}
						}
						{
							t130, err := f1(5)
							if err != nil {
								return 0, err
							}
							if err := f3(t130); err != nil {
								return 0, err
							}
						}
					} else {
						t131, err := b1((9 + 5))
						if err != nil {
							return 0, err
						}
						if t131 {
							{
								for i6 := 0; ; i6++ {
									if !(i6 < 1) {
										break
									}
									t132, err := f1(v3)
									if err != nil {
										return 0, err
									}
									if !(v1 < t132) {
										break
									}
									{
										t133, err := f1(v2)
										if err != nil {
											return 0, err
										}
										if t133 < 7 {
											continue
										}
									}
									{
										v0 = v1
									}
								}
							}
							{
								c135 := (v0 - v2) < v3
								if !c135 {
									t136, err := b1((v3 + 6))
									if err != nil {
										return 0, err
									}
									c135 = !(t136)
								}
								if c135 {
									{
										t137, err := f1(v0)
										if err != nil {
											return 0, err
										}
										if err := f3(t137); err != nil {
											return 0, err
										}
									}
								} else {
									t138, err := b1(v3)
									if err != nil {
										return 0, err
									}
									if !(!(t138)) {
										{
											a139, b139, err := f2((v3 - 5))
											if err != nil {
												return 0, err
											}
											v3, v2 = a139, b139
										}
										{
											v3 = v0
										}
									}
								}
							}
						}
					}
				}
			}
			{
				if v2 < (2 + 7) {
					{
						t141, err := f1((v3 + 3))
						if err != nil {
							return 0, err
						}
						switch t141 % 3 {
						case 0:
							{
								t142, err := f1(9)
								if err != nil {
									return 0, err
								}
								t143, err := f1(t142)
								if err != nil {
									return 0, err
								}
								v1 = t143
							}
							{
								t144, err := f1(v2)
								if err != nil {
									return 0, err
								}
								v2 += t144
							}
						case 1:
							{
								a145, b145, err := f2((v3 + v0))
								if err != nil {
									return 0, err
								}
								v1, v2 = a145, b145
							}
						default:
							{
								t146, err := f1(v0)
								if err != nil {
									return 0, err
								}
								t147, err := f1(t146)
								if err != nil {
									return 0, err
								}
								v1 += t147
							}
							{
								t148, err := f1(v3)
								if err != nil {
									return 0, err
								}
								t149, err := f1(t148)
								if err != nil {
									return 0, err
								}
								v0 = t149
							}
						}
					}
					{
						v1 = (v1 - v1)
					}
					{
						t150, err := b1(v2)
						if err != nil {
							return 0, err
						}
						c151 := !(t150)
						if c151 {
							t152, err := b1(v1)
							if err != nil {
								return 0, err
							}
							c151 = t152
						}
						if c151 {
							{
								t153, err := f1(v0)
								if err != nil {
									return 0, err
								}
								t154, err := f1(t153)
								if err != nil {
									return 0, err
								}
								v0 += t154
							}
							{
								t155, err := f1(v1)
								if err != nil {
									return 0, err
								}
								a156, b156, err := f2(t155)
								if err != nil {
									return 0, err
								}
								v0, v1 = a156, b156
							}
							{
								v0 = v3
							}
						} else {
							t157, err := b1((5 - v1))
							if err != nil {
								return 0, err
							}
							if t157 {
								{
									v1 = v2
								}
								{
									t158, err := f1(v1)
									if err != nil {
										return 0, err
									}
									v0 = t158
								}
								{
									v2 = v0
								}
							}
						}
					}
				} else {
					t159, err := f1(v0)
					if err != nil {
						return 0, err
					}
					t160, err := b1(t159)
					if err != nil {
						return 0, err
					}
					if t160 {
						{
							t161, err := f1(v0)
							if err != nil {
								return 0, err
							}
							t162, err := f1(t161)
							if err != nil {
								return 0, err
							}
							switch t162 % 3 {
							case 0:
								{
									t163, err := f1(v3)
									if err != nil {
										return 0, err
									}
									if err := f3(t163); err != nil {
										return 0, err
									}
								}
								{
									if err := f3((v0 - 3)); err != nil {
										return 0, err
									}
								}
								{
									a164, b164, err := f2(v1)
									if err != nil {
										return 0, err
									}
									v0, v2 = a164, b164
								}
							case 1:
								{
									v2 += (v0 + (v1 - 8))
								}
								{
									t165, err := f1(v0)
									if err != nil {
										return 0, err
									}
									if err := f3(t165); err != nil {
										return 0, err
									}
								}
							default:
								{
									t166, err := f1(v0)
									if err != nil {
										return 0, err
									}
									v1 = t166
								}
							}
						}
						{
							t167, err := f1(v1)
							if err != nil {
								return 0, err
							}
							v0 = t167
						}
						{
							a168, b168, err := f2((v0 + v1))
							if err != nil {
								return 0, err
							}
							v2, v3 = a168, b168
						}
					} else {
						{
							t169, err := f1(v1)
							if err != nil {
								return 0, err
							}
							v2 = t169
						}
						{
							t170, err := f1((v0 - 7))
							if err != nil {
								return 0, err
							}
							switch t170 % 3 {
							case 0:
								{
									t171, err := f1(v2)
									if err != nil {
										return 0, err
									}
									if err := f3(t171); err != nil {
										return 0, err
									}
								}
							case 1:
								{
									t172, err := f1(v3)
									if err != nil {
										return 0, err
									}
									a173, b173, err := f2(t172)
									if err != nil {
										return 0, err
									}
									v2, v3 = a173, b173
								}
							default:
								{
									v0 += v3
								}
								{
									t174, err := f1(v3)
									if err != nil {
										return 0, err
									}
									if err := f3(t174); err != nil {
										return 0, err
									}
								}
							}
						}
					}
				}
			}
		} else {
			{
				v1 = ((v1 - v1) - (v2 + v3))
			}
		}
	}
	{
		t176, err := b1((v3 - v1))
		if err != nil {
			return 0, err
		}
		if t176 {
			{
				t177, err := f1(v0)
				if err != nil {
					return 0, err
				}
				t178, err := f1(t177)
				if err != nil {
					return 0, err
				}
				v3 = t178
			}
			{
				v3 = v0
			}
			{
				if err := f3((v0 - v3)); err != nil {
					return 0, err
				}
			}
		} else {
			t179, err := b1(v1)
			if err != nil {
				return 0, err
			}
			if t179 {
				{
					t180, err := f1(v1)
					if err != nil {
						return 0, err
					}
					t181, err := f1(t180)
					if err != nil {
						return 0, err
					}
					v3 = t181
				}
				{
					t182, err := f1(v3)
					if err != nil {
						return 0, err
					}
					v0 += t182
				}
			} else {
				t183, err := b1(v1)
				if err != nil {
					return 0, err
				}
				c184 := t183
				if c184 {
					t185, err := b1(v2)
					if err != nil {
						return 0, err
					}
					c184 = t185
				}
				if c184 {
					{
						t186, err := b1(3)
						if err != nil {
							return 0, err
						}
						if t186 {
							{
								t187, err := b1((v0 + v2))
								if err != nil {
									return 0, err
								}
								c188 := t187
								if !c188 {
									t189, err := b1(v1)
									if err != nil {
										return 0, err
									}
									c188 = t189
								}
								if c188 {
									{
										t190, err := f1(v2)
										if err != nil {
											return 0, err
										}
										v3 = t190
									}
									{
										v1 = (v1 + v2)
									}
								} else {
									t191, err := b1(v3)
									if err != nil {
										return 0, err
									}
									c192 := t191
									if !c192 {
										t193, err := b1((v0 + 8))
										if err != nil {
											return 0, err
										}
										c192 = t193
									}
									if c192 {
										{
											t194, err := f1(v2)
											if err != nil {
												return 0, err
											}
											a195, b195, err := f2(t194)
											if err != nil {
												return 0, err
											}
											v3, v2 = a195, b195
										}
										{
											t196, err := f1(3)
											if err != nil {
												return 0, err
											}
											v1 += t196
										}
									} else {
										t197, err := f1(v1)
										if err != nil {
											return 0, err
										}
										t198, err := b1(t197)
										if err != nil {
											return 0, err
										}
										if t198 {
											{
												v0 = v1
											}
										}
									}
								}
							}
							{
								t199, err := f1(v2)
								if err != nil {
									return 0, err
								}
								if t199 < (v3 - v2) {
									{
										t200, err := f1(v3)
										if err != nil {
											return 0, err
										}
										v2 = t200
									}
								}
							}
							{
								t201, err := f1(v2)
								if err != nil {
									return 0, err
								}
								a202, b202, err := f2(t201)
								if err != nil {
									return 0, err
								}
								v2, v1 = a202, b202
							}
						} else {
							t203, err := b1((v3 + v3))
							if err != nil {
								return 0, err
							}
							if t203 {
								{
									v1 = ((v0 - 0) + v3)
								}
							}
						}
					}
				}
			}
		}
	}
	{
		for i7 := 0; ; i7++ {
			if !(i7 < 1) {
				break
			}
			t204, err := b1(0)
			if err != nil {
				return 0, err
			}
			if !(t204) {
				break
			}
			{
				if (v0 - 4) < 4 {
					break
				}
			}
			{
				t205, err := b1((v3 - v3))
				if err != nil {
					return 0, err
				}
				if t205 {
					{
						t206, err := f1((v2 - v0))
						if err != nil {
							return 0, err
						}
						v0 += t206
					}
				} else {
					t207, err := b1((v3 + v0))
					if err != nil {
						return 0, err
					}
					if t207 {
						{
							t208, err := f1(5)
							if err != nil {
								return 0, err
							}
							if t208 < 6 {
								continue
							}
						}
					} else {
						t209, err := b1(v3)
						if err != nil {
							return 0, err
						}
						c210 := t209
						if c210 {
							t211, err := b1(v2)
							if err != nil {
								return 0, err
							}
							c210 = t211
						}
						if c210 {
							{
								t212, err := f1(v1)
								if err != nil {
									return 0, err
								}
								a213, b213, err := f2(t212)
								if err != nil {
									return 0, err
								}
								v1, v3 = a213, b213
							}
							{
								v0 = v0
							}
						}
					}
				}
			}
			{
				t214, err := f1(v0)
				if err != nil {
					return 0, err
				}
				a215, b215, err := f2(t214)
				if err != nil {
					return 0, err
				}
				v1, v3 = a215, b215
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
