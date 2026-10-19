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
			if !(i1 < 1) {
				break
			}
			t101, err := f1(v1)
			if err != nil {
				return 0, err
			}
			t102, err := b1(t101)
			if err != nil {
				return 0, err
			}
			c103 := (t102 || v1 < v3)
			if c103 {
				t104, err := f1(v2)
				if err != nil {
					return 0, err
				}
				t105, err := b1(t104)
				if err != nil {
					return 0, err
				}
				c103 = !(t105)
			}
			if !(c103) {
				break
			}
			{
				t106, err := f1(v2)
				if err != nil {
					return 0, err
				}
				if t106 < 5 {
					continue
				}
			}
			{
				t108, err := b1((1 - v2))
				if err != nil {
					return 0, err
				}
				if !(t108) {
					{
						t109, err := b1(2)
						if err != nil {
							return 0, err
						}
						if t109 {
							{
								if err := f3(8); err != nil {
									return 0, err
								}
							}
							{
								if err := f3((v1 + 5)); err != nil {
									return 0, err
								}
							}
							{
								t110, err := f1(v1)
								if err != nil {
									return 0, err
								}
								v3 = ((v1 + 1) - t110)
							}
						} else {
							t111, err := f1(v3)
							if err != nil {
								return 0, err
							}
							t112, err := b1(t111)
							if err != nil {
								return 0, err
							}
							if t112 {
								{
									a113, b113, err := f2(v2)
									if err != nil {
										return 0, err
									}
									v3, v0 = a113, b113
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
							} else {
								{
									t116, err := f1(v2)
									if err != nil {
										return 0, err
									}
									if err := f3(t116); err != nil {
										return 0, err
									}
								}
							}
						}
					}
				} else {
					t117, err := b1((v3 - 8))
					if err != nil {
						return 0, err
					}
					if t117 {
						{
							t119, err := f1(v3)
							if err != nil {
								return 0, err
							}
							c120 := t119 < (v1 - v2)
							if c120 {
								t121, err := f1(3)
								if err != nil {
									return 0, err
								}
								t122, err := b1(t121)
								if err != nil {
									return 0, err
								}
								c120 = !(t122)
							}
							if c120 {
								{
									v3 = v0
								}
								{
									t123, err := f1((v0 - v0))
									if err != nil {
										return 0, err
									}
									v1 = t123
								}
								{
									t124, err := f1(v0)
									if err != nil {
										return 0, err
									}
									t125, err := f1(t124)
									if err != nil {
										return 0, err
									}
									v3 = t125
								}
							} else {
								t126, err := b1(v2)
								if err != nil {
									return 0, err
								}
								c127 := t126
								if !c127 {
									t128, err := b1((v0 - v2))
									if err != nil {
										return 0, err
									}
									c127 = t128
								}
								if c127 {
									{
										t129, err := f1(v2)
										if err != nil {
											return 0, err
										}
										a130, b130, err := f2(t129)
										if err != nil {
											return 0, err
										}
										v0, v1 = a130, b130
									}
								} else {
									t131, err := f1(v1)
									if err != nil {
										return 0, err
									}
									t132, err := b1(t131)
									if err != nil {
										return 0, err
									}
									if t132 {
										{
											t133, err := f1(v3)
											if err != nil {
												return 0, err
											}
											t134, err := f1(t133)
											if err != nil {
												return 0, err
											}
											v2 += t134
										}
									} else {
										{
											v0 += 5
										}
										{
											t135, err := f1(v1)
											if err != nil {
												return 0, err
											}
											v2 += t135
										}
										{
											t136, err := f1(v1)
											if err != nil {
												return 0, err
											}
											t137, err := f1(v0)
											if err != nil {
												return 0, err
											}
											v3 = (t136 + t137)
										}
									}
								}
							}
						}
						{
							if v0 < 4 {
								goto sw107
							}
						}
						{
							for i2 := 0; ; i2++ {
								if !(i2 < 2) {
									break
								}
								t138, err := b1(v3)
								if err != nil {
									return 0, err
								}
								c139 := t138
								if c139 {
									t140, err := f1(1)
									if err != nil {
										return 0, err
									}
									t141, err := b1(t140)
									if err != nil {
										return 0, err
									}
									c139 = t141
								}
								c142 := c139
								if c142 {
									t143, err := f1(v2)
									if err != nil {
										return 0, err
									}
									t144, err := b1(t143)
									if err != nil {
										return 0, err
									}
									c142 = t144
								}
								if !(c142) {
									break
								}
								{
									a145, b145, err := f2((v2 - v2))
									if err != nil {
										return 0, err
									}
									v2, v3 = a145, b145
								}
								{
									t146, err := f1(v1)
									if err != nil {
										return 0, err
									}
									v2 += t146
								}
								{
									t147, err := f1(v0)
									if err != nil {
										return 0, err
									}
									if err := f3(t147); err != nil {
										return 0, err
									}
								}
							}
						}
					}
				}
			}
			sw107:
			{
			}
			{
				v1 += v2
			}
		}
	}
	{
		t148, err := b1((v0 - 6))
		if err != nil {
			return 0, err
		}
		if t148 {
			{
				for i3 := 0; ; i3++ {
					if !(i3 < 3) {
						break
					}
					{
						t149, err := f1(v1)
						if err != nil {
							return 0, err
						}
						switch (6 - t149) % 3 {
						case 0:
							{
								v3 = 1
							}
							{
								if v2 < 7 {
									break
								}
							}
						case 1:
							{
								if (v3 + v2) < 5 {
									continue
								}
							}
							{
								t150, err := f1((v2 + v1))
								if err != nil {
									return 0, err
								}
								v1 = t150
							}
							{
								t151, err := f1(v3)
								if err != nil {
									return 0, err
								}
								a152, b152, err := f2(t151)
								if err != nil {
									return 0, err
								}
								v1, v2 = a152, b152
							}
						default:
							{
								if err := f3((v0 + 7)); err != nil {
									return 0, err
								}
							}
						}
					}
					{
						t153, err := f1(8)
						if err != nil {
							return 0, err
						}
						switch t153 % 3 {
						case 0:
							{
								if (7 - v2) < 7 {
									break
								}
							}
							{
								if (v0 + v3) < 7 {
									continue
								}
							}
						case 1:
							{
								if (v3 - v3) < 5 {
									continue
								}
							}
							{
								t154, err := f1(v3)
								if err != nil {
									return 0, err
								}
								if t154 < 6 {
									break
								}
							}
						default:
							{
								t155, err := f1(v1)
								if err != nil {
									return 0, err
								}
								t156, err := f1(t155)
								if err != nil {
									return 0, err
								}
								v2 = t156
							}
						}
					}
					{
						c158 := (v0 - 4) < 8
						if c158 {
							t159, err := b1((4 - v1))
							if err != nil {
								return 0, err
							}
							c158 = t159
						}
						c160 := c158
						if c160 {
							t161, err := f1(v3)
							if err != nil {
								return 0, err
							}
							t162, err := b1(t161)
							if err != nil {
								return 0, err
							}
							c160 = t162
						}
						if c160 {
							{
								t163, err := f1(v1)
								if err != nil {
									return 0, err
								}
								v0 = t163
							}
							{
								if (v3 - v0) < 4 {
									goto sw157
								}
							}
						} else {
							t164, err := f1(2)
							if err != nil {
								return 0, err
							}
							t165, err := b1(t164)
							if err != nil {
								return 0, err
							}
							if t165 {
								{
									if (v3 - v1) < 7 {
										continue
									}
								}
								{
									if 5 < 7 {
										goto sw157
									}
								}
							} else {
								{
									t166, err := f1(v1)
									if err != nil {
										return 0, err
									}
									a167, b167, err := f2(t166)
									if err != nil {
										return 0, err
									}
									v0, v1 = a167, b167
								}
							}
						}
					}
					sw157:
					{
					}
				}
			}
			{
				t168, err := b1((v3 + 7))
				if err != nil {
					return 0, err
				}
				if !(t168) {
					{
						v3 += 6
					}
					{
						v0 = v1
					}
				} else {
					t169, err := b1(v3)
					if err != nil {
						return 0, err
					}
					if t169 {
						{
							if err := f3(v2); err != nil {
								return 0, err
							}
						}
						{
							t170, err := f1((8 + v1))
							if err != nil {
								return 0, err
							}
							v1 = t170
						}
					} else {
						{
							switch 9 % 3 {
							case 0:
								{
									v1 = v1
								}
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
									v1 = (v1 + v0)
								}
							default:
								{
									t172, err := f1(v1)
									if err != nil {
										return 0, err
									}
									v3 = t172
								}
								{
									v0 = (v2 + v1)
								}
							}
						}
						{
							t174, err := f1(5)
							if err != nil {
								return 0, err
							}
							if t174 < (v2 - v0) {
								{
									v2 = v2
								}
								{
									t175, err := f1(v0)
									if err != nil {
										return 0, err
									}
									v0 = t175
								}
							} else {
								t176, err := f1(v3)
								if err != nil {
									return 0, err
								}
								t177, err := b1(t176)
								if err != nil {
									return 0, err
								}
								if t177 {
									{
										v2 = (v0 + v2)
									}
									{
										t178, err := f1(5)
										if err != nil {
											return 0, err
										}
										v2 = t178
									}
									{
										v2 += v0
									}
								} else {
									t179, err := f1(v3)
									if err != nil {
										return 0, err
									}
									t180, err := b1(t179)
									if err != nil {
										return 0, err
									}
									if t180 {
										{
											v2 = (v0 - 1)
										}
									} else {
										{
											v0 = (3 + v3)
										}
										{
											v3 = v1
										}
										{
											v3 = 6
										}
									}
								}
							}
						}
						{
							t181, err := f1(v1)
							if err != nil {
								return 0, err
							}
							v2 = t181
						}
					}
				}
			}
			{
				for i4 := 0; ; i4++ {
					if !(i4 < 2) {
						break
					}
					t182, err := f1(0)
					if err != nil {
						return 0, err
					}
					t183, err := b1(t182)
					if err != nil {
						return 0, err
					}
					if !(t183) {
						break
					}
					{
						if err := f3((v3 - v0)); err != nil {
							return 0, err
						}
					}
					{
						t184, err := b1(v1)
						if err != nil {
							return 0, err
						}
						if t184 {
							{
								t185, err := f1((v3 - v3))
								if err != nil {
									return 0, err
								}
								v2 = t185
							}
						} else {
							{
								t186, err := f1(6)
								if err != nil {
									return 0, err
								}
								v1 = (t186 + (v1 + v2))
							}
							{
								if err := f3((v0 - v3)); err != nil {
									return 0, err
								}
							}
						}
					}
					{
						switch 9 % 3 {
						case 0:
							{
								v2 += ((v2 - v3) + (v0 + 3))
							}
							{
								t187, err := f1(v2)
								if err != nil {
									return 0, err
								}
								a188, b188, err := f2(t187)
								if err != nil {
									return 0, err
								}
								v1, v0 = a188, b188
							}
							{
								v3 = 7
							}
						case 1:
							{
								t189, err := f1(v3)
								if err != nil {
									return 0, err
								}
								if t189 < 6 {
									break
								}
							}
						default:
							{
								t190, err := f1(v0)
								if err != nil {
									return 0, err
								}
								if err := f3(t190); err != nil {
									return 0, err
								}
							}
							{
								t191, err := f1(v1)
								if err != nil {
									return 0, err
								}
								t192, err := f1(t191)
								if err != nil {
									return 0, err
								}
								v0 = t192
							}
						}
					}
				}
			}
		} else {
			t193, err := b1((2 - v3))
			if err != nil {
				return 0, err
			}
			if t193 {
				{
					for i5 := 0; ; i5++ {
						if !(i5 < 2) {
							break
						}
						t194, err := f1(9)
						if err != nil {
							return 0, err
						}
						t195, err := b1(t194)
						if err != nil {
							return 0, err
						}
						if !(!(t195)) {
							break
						}
						{
							a196, b196, err := f2(v2)
							if err != nil {
								return 0, err
							}
							v1, v0 = a196, b196
						}
						{
							t197, err := b1(0)
							if err != nil {
								return 0, err
							}
							c198 := t197
							if !c198 {
								t199, err := f1(v0)
								if err != nil {
									return 0, err
								}
								t200, err := f1(5)
								if err != nil {
									return 0, err
								}
								c198 = t199 < t200
							}
							c201 := c198
							if c201 {
								t202, err := b1(v3)
								if err != nil {
									return 0, err
								}
								c201 = t202
							}
							if c201 {
								{
									if (v1 + v1) < 7 {
										break
									}
								}
								{
									t203, err := f1((v1 + 5))
									if err != nil {
										return 0, err
									}
									v1 = t203
								}
								{
									if v1 < 6 {
										break
									}
								}
							} else {
								{
									t204, err := f1(v0)
									if err != nil {
										return 0, err
									}
									if err := f3(t204); err != nil {
										return 0, err
									}
								}
								{
									if v2 < 3 {
										continue
									}
								}
								{
									if (v0 + v3) < 6 {
										break
									}
								}
							}
						}
					}
				}
				{
					t205, err := f1(v3)
					if err != nil {
						return 0, err
					}
					v2 = t205
				}
				{
					for i6 := 0; ; i6++ {
						if !(i6 < 1) {
							break
						}
						t206, err := f1(1)
						if err != nil {
							return 0, err
						}
						t207, err := f1(v0)
						if err != nil {
							return 0, err
						}
						if !(t206 < t207) {
							break
						}
						{
							for i7 := 0; ; i7++ {
								if !(i7 < 2) {
									break
								}
								t208, err := b1(v2)
								if err != nil {
									return 0, err
								}
								if !(t208) {
									break
								}
								{
									if err := f3(v2); err != nil {
										return 0, err
									}
								}
							}
						}
						{
							t209, err := f1(v2)
							if err != nil {
								return 0, err
							}
							v3 = t209
						}
						{
							for i8 := 0; ; i8++ {
								if !(i8 < 2) {
									break
								}
								t210, err := b1(v0)
								if err != nil {
									return 0, err
								}
								if !(t210) {
									break
								}
								{
									t211, err := f1(v1)
									if err != nil {
										return 0, err
									}
									if t211 < 6 {
										break
									}
								}
							}
						}
					}
				}
			}
		}
	}
	{
		for i9 := 0; ; i9++ {
			if !(i9 < 1) {
				break
			}
			t212, err := f1(v0)
			if err != nil {
				return 0, err
			}
			t213, err := b1(t212)
			if err != nil {
				return 0, err
			}
			if !(t213) {
				break
			}
			{
				for i10 := 0; ; i10++ {
					if !(i10 < 2) {
						break
					}
					t214, err := b1(v3)
					if err != nil {
						return 0, err
					}
					c215 := t214
					if c215 {
						t216, err := b1(6)
						if err != nil {
							return 0, err
						}
						c217 := t216
						if c217 {
							t218, err := b1((9 + v0))
							if err != nil {
								return 0, err
							}
							c217 = t218
						}
						c215 = c217
					}
					if !(c215) {
						break
					}
					{
						t219, err := b1((9 - v0))
						if err != nil {
							return 0, err
						}
						if t219 {
							{
								t220, err := f1(v0)
								if err != nil {
									return 0, err
								}
								v0 += t220
							}
						}
					}
					{
						for i11 := 0; ; i11++ {
							if !(i11 < 1) {
								break
							}
							t221, err := b1((1 + v1))
							if err != nil {
								return 0, err
							}
							if !(t221) {
								break
							}
							{
								t222, err := f1(8)
								if err != nil {
									return 0, err
								}
								a223, b223, err := f2(t222)
								if err != nil {
									return 0, err
								}
								v2, v1 = a223, b223
							}
						}
					}
				}
			}
			{
				if v1 < 4 {
					continue
				}
			}
			{
				t224, err := f1(v0)
				if err != nil {
					return 0, err
				}
				t225, err := f1(3)
				if err != nil {
					return 0, err
				}
				v3 = (t224 - t225)
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
