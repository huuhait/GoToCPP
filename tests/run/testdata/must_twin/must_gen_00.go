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
			t101, err := f1(v1)
			if err != nil {
				return 0, err
			}
			if !((4 + v1) < t101) {
				break
			}
			{
				t102, err := f1(v3)
				if err != nil {
					return 0, err
				}
				a103, b103, err := f2(t102)
				if err != nil {
					return 0, err
				}
				v3, v2 = a103, b103
			}
			{
				for i2 := 0; ; i2++ {
					if !(i2 < 2) {
						break
					}
					t104, err := b1(v1)
					if err != nil {
						return 0, err
					}
					c105 := t104
					if !c105 {
						t106, err := b1((v2 + v3))
						if err != nil {
							return 0, err
						}
						c105 = !(t106)
					}
					if !(c105) {
						break
					}
					{
						v3 += (v3 - v0)
					}
					{
						t107, err := f1(1)
						if err != nil {
							return 0, err
						}
						switch (v1 - t107) % 3 {
						case 0:
							{
								a108, b108, err := f2(v3)
								if err != nil {
									return 0, err
								}
								v3, v0 = a108, b108
							}
							{
								t109, err := f1(0)
								if err != nil {
									return 0, err
								}
								if t109 < 6 {
									break
								}
							}
							{
								v1 = v1
							}
						case 1:
							{
								if err := f3(v0); err != nil {
									return 0, err
								}
							}
							{
								a110, b110, err := f2((v2 + v3))
								if err != nil {
									return 0, err
								}
								v3, v0 = a110, b110
							}
							{
								if v1 < 3 {
									continue
								}
							}
						default:
							{
								if 7 < 5 {
									break
								}
							}
						}
					}
				}
			}
			{
				t111, err := f1(v1)
				if err != nil {
					return 0, err
				}
				t112, err := f1(v0)
				if err != nil {
					return 0, err
				}
				if t111 < t112 {
					{
						if v1 < 6 {
							continue
						}
					}
				} else {
					{
						for i3 := 0; ; i3++ {
							if !(i3 < 1) {
								break
							}
							t113, err := f1(6)
							if err != nil {
								return 0, err
							}
							t114, err := b1(t113)
							if err != nil {
								return 0, err
							}
							if !(t114) {
								break
							}
							{
								t115, err := f1(v3)
								if err != nil {
									return 0, err
								}
								v3 += ((v2 - v3) + t115)
							}
							{
								t116, err := f1(v0)
								if err != nil {
									return 0, err
								}
								t117, err := f1(t116)
								if err != nil {
									return 0, err
								}
								v3 = t117
							}
							{
								a118, b118, err := f2(v2)
								if err != nil {
									return 0, err
								}
								v1, v3 = a118, b118
							}
						}
					}
				}
			}
		}
	}
	{
		if 7 < v2 {
			{
				t119, err := f1((9 + v1))
				if err != nil {
					return 0, err
				}
				v0 = t119
			}
			{
				for i4 := 0; ; i4++ {
					if !(i4 < 3) {
						break
					}
					t120, err := b1(1)
					if err != nil {
						return 0, err
					}
					if !(t120) {
						break
					}
					{
						t121, err := f1(v0)
						if err != nil {
							return 0, err
						}
						t122, err := f1(t121)
						if err != nil {
							return 0, err
						}
						v2 = t122
					}
					{
						if err := f3(v3); err != nil {
							return 0, err
						}
					}
					{
						v1 = 6
					}
				}
			}
			{
				t123, err := f1(0)
				if err != nil {
					return 0, err
				}
				if 0 < t123 {
					{
						for i5 := 0; ; i5++ {
							if !(i5 < 3) {
								break
							}
							{
								a124, b124, err := f2((v2 - v0))
								if err != nil {
									return 0, err
								}
								v1, v0 = a124, b124
							}
							{
								v0 = ((v1 - 8) - 5)
							}
							{
								t125, err := f1(2)
								if err != nil {
									return 0, err
								}
								v0 = (t125 + v3)
							}
						}
					}
				} else {
					t126, err := b1(v3)
					if err != nil {
						return 0, err
					}
					c127 := t126
					if c127 {
						t128, err := f1(v0)
						if err != nil {
							return 0, err
						}
						c127 = v1 < t128
					}
					if c127 {
						{
							t129, err := f1(v3)
							if err != nil {
								return 0, err
							}
							t130, err := b1(t129)
							if err != nil {
								return 0, err
							}
							c131 := t130
							if c131 {
								t132, err := f1(v1)
								if err != nil {
									return 0, err
								}
								c131 = (v3 - v3) < t132
							}
							c133 := c131
							if !c133 {
								t134, err := f1(v2)
								if err != nil {
									return 0, err
								}
								c133 = v2 < t134
							}
							if c133 {
								{
									if err := f3(v2); err != nil {
										return 0, err
									}
								}
								{
									v1 = 1
								}
							} else {
								t135, err := b1(5)
								if err != nil {
									return 0, err
								}
								c136 := t135
								if c136 {
									t137, err := f1(v3)
									if err != nil {
										return 0, err
									}
									t138, err := b1(t137)
									if err != nil {
										return 0, err
									}
									c136 = t138
								}
								if c136 {
									{
										v0 += v2
									}
								} else {
									t139, err := b1((8 + v2))
									if err != nil {
										return 0, err
									}
									c140 := (t139 && 7 < v3)
									if c140 {
										t141, err := f1(1)
										if err != nil {
											return 0, err
										}
										t142, err := b1(t141)
										if err != nil {
											return 0, err
										}
										c140 = t142
									}
									if c140 {
										{
											v1 = v2
										}
									} else {
										{
											v0 = (7 - v3)
										}
									}
								}
							}
						}
					} else {
						{
							t143, err := b1(v3)
							if err != nil {
								return 0, err
							}
							if t143 {
								{
									v3 = 5
								}
								{
									t144, err := f1(5)
									if err != nil {
										return 0, err
									}
									t145, err := f1(t144)
									if err != nil {
										return 0, err
									}
									v1 = t145
								}
							} else {
								if !((v2 < v3 && v1 < (v2 - v1))) {
									{
										a146, b146, err := f2(v0)
										if err != nil {
											return 0, err
										}
										v1, v3 = a146, b146
									}
									{
										v2 = 3
									}
									{
										a147, b147, err := f2(v2)
										if err != nil {
											return 0, err
										}
										v3, v2 = a147, b147
									}
								} else {
									t148, err := b1((3 + v0))
									if err != nil {
										return 0, err
									}
									if t148 {
										{
											t149, err := f1(v2)
											if err != nil {
												return 0, err
											}
											v2 = t149
										}
										{
											v3 = (v1 - v3)
										}
										{
											v2 = v1
										}
									}
								}
							}
						}
					}
				}
			}
		} else {
			t150, err := b1((v2 + v0))
			if err != nil {
				return 0, err
			}
			if t150 {
				{
					for i6 := 0; ; i6++ {
						if !(i6 < 2) {
							break
						}
						t151, err := b1(v3)
						if err != nil {
							return 0, err
						}
						c152 := t151
						if !c152 {
							t153, err := b1(7)
							if err != nil {
								return 0, err
							}
							c152 = (t153 && v1 < (v3 + 9))
						}
						if !(c152) {
							break
						}
						{
							if err := f3((v1 + v0)); err != nil {
								return 0, err
							}
						}
					}
				}
				{
					for i7 := 0; ; i7++ {
						if !(i7 < 1) {
							break
						}
						c154 := 2 < v1
						if !c154 {
							t155, err := b1(v0)
							if err != nil {
								return 0, err
							}
							c154 = t155
						}
						if !(c154) {
							break
						}
						{
							switch (v2 - v2) % 3 {
							case 0:
								{
									t156, err := f1(5)
									if err != nil {
										return 0, err
									}
									a157, b157, err := f2(t156)
									if err != nil {
										return 0, err
									}
									v2, v0 = a157, b157
								}
								{
									if (v1 - v1) < 4 {
										break
									}
								}
							case 1:
								{
									t158, err := f1(v3)
									if err != nil {
										return 0, err
									}
									t159, err := f1(t158)
									if err != nil {
										return 0, err
									}
									v1 = t159
								}
								{
									t160, err := f1(v1)
									if err != nil {
										return 0, err
									}
									t161, err := f1(t160)
									if err != nil {
										return 0, err
									}
									v0 = t161
								}
								{
									if err := f3(4); err != nil {
										return 0, err
									}
								}
							default:
								{
									if (v0 - v2) < 7 {
										continue
									}
								}
							}
						}
					}
				}
				{
					t162, err := f1(v0)
					if err != nil {
						return 0, err
					}
					a163, b163, err := f2(t162)
					if err != nil {
						return 0, err
					}
					v2, v3 = a163, b163
				}
			}
		}
	}
	{
		t164, err := f1(v0)
		if err != nil {
			return 0, err
		}
		switch (t164 + (v0 + v2)) % 3 {
		case 0:
			{
				for i8 := 0; ; i8++ {
					if !(i8 < 1) {
						break
					}
					t165, err := b1(v3)
					if err != nil {
						return 0, err
					}
					if !(t165) {
						break
					}
					{
						for i9 := 0; ; i9++ {
							if !(i9 < 1) {
								break
							}
							t166, err := b1((v1 - v2))
							if err != nil {
								return 0, err
							}
							if !(t166) {
								break
							}
							{
								t167, err := f1((0 - v3))
								if err != nil {
									return 0, err
								}
								v2 = t167
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
			{
				t168, err := b1((v0 - v3))
				if err != nil {
					return 0, err
				}
				if t168 {
					{
						t169, err := f1((3 - v2))
						if err != nil {
							return 0, err
						}
						v0 = t169
					}
					{
						v1 = v0
					}
				} else {
					t170, err := f1(2)
					if err != nil {
						return 0, err
					}
					t171, err := b1(t170)
					if err != nil {
						return 0, err
					}
					if t171 {
						{
							t173, err := f1(v0)
							if err != nil {
								return 0, err
							}
							t174, err := b1(t173)
							if err != nil {
								return 0, err
							}
							if t174 {
								{
									v3 = (v2 + 0)
								}
								{
									t175, err := f1(4)
									if err != nil {
										return 0, err
									}
									t176, err := f1(v2)
									if err != nil {
										return 0, err
									}
									v1 = (t175 - t176)
								}
								{
									t177, err := f1(v3)
									if err != nil {
										return 0, err
									}
									t178, err := f1(t177)
									if err != nil {
										return 0, err
									}
									v3 = t178
								}
							} else {
								t179, err := b1(1)
								if err != nil {
									return 0, err
								}
								if !(t179) {
									{
										a180, b180, err := f2((9 + v1))
										if err != nil {
											return 0, err
										}
										v0, v1 = a180, b180
									}
								} else {
									t181, err := b1((v0 - v0))
									if err != nil {
										return 0, err
									}
									if t181 {
										{
											t182, err := f1(6)
											if err != nil {
												return 0, err
											}
											t183, err := f1(t182)
											if err != nil {
												return 0, err
											}
											v2 = t183
										}
										{
											t184, err := f1(v1)
											if err != nil {
												return 0, err
											}
											v0 = (t184 - (v3 - v3))
										}
										{
											t185, err := f1((v3 + v3))
											if err != nil {
												return 0, err
											}
											v0 += t185
										}
									} else {
										{
											v0 = 6
										}
										{
											t186, err := f1((v1 - v1))
											if err != nil {
												return 0, err
											}
											v0 = t186
										}
									}
								}
							}
						}
						{
							for i10 := 0; ; i10++ {
								if !(i10 < 2) {
									break
								}
								{
									t187, err := f1(v1)
									if err != nil {
										return 0, err
									}
									a188, b188, err := f2(t187)
									if err != nil {
										return 0, err
									}
									v3, v2 = a188, b188
								}
							}
						}
						{
							if err := f3(v1); err != nil {
								return 0, err
							}
						}
					} else {
						t189, err := b1((v3 - v3))
						if err != nil {
							return 0, err
						}
						c190 := !(t189)
						if c190 {
							t191, err := b1(v3)
							if err != nil {
								return 0, err
							}
							c192 := t191
							if c192 {
								t193, err := f1(v1)
								if err != nil {
									return 0, err
								}
								c192 = t193 < v1
							}
							c190 = c192
						}
						if c190 {
							{
								t194, err := f1(v2)
								if err != nil {
									return 0, err
								}
								v1 = t194
							}
							{
								t195, err := f1((v2 - v0))
								if err != nil {
									return 0, err
								}
								v2 = t195
							}
							{
								t196, err := f1(v1)
								if err != nil {
									return 0, err
								}
								t197, err := f1(t196)
								if err != nil {
									return 0, err
								}
								v0 = t197
							}
						}
					}
				}
			}
		case 1:
			{
				t198, err := f1(v3)
				if err != nil {
					return 0, err
				}
				c199 := (v3 - v3) < t198
				if c199 {
					t200, err := f1(v2)
					if err != nil {
						return 0, err
					}
					t201, err := b1(t200)
					if err != nil {
						return 0, err
					}
					c199 = t201
				}
				c202 := c199
				if !c202 {
					t203, err := f1(v2)
					if err != nil {
						return 0, err
					}
					t204, err := b1(t203)
					if err != nil {
						return 0, err
					}
					c202 = t204
				}
				if c202 {
					{
						v0 = (v0 + v1)
					}
					{
						for i11 := 0; ; i11++ {
							if !(i11 < 1) {
								break
							}
							t205, err := f1(1)
							if err != nil {
								return 0, err
							}
							t206, err := b1(t205)
							if err != nil {
								return 0, err
							}
							if !(t206) {
								break
							}
							{
								if (v2 + v2) < 3 {
									break
								}
							}
							{
								if err := f3(v1); err != nil {
									return 0, err
								}
							}
							{
								v2 += v1
							}
						}
					}
					{
						for i12 := 0; ; i12++ {
							if !(i12 < 1) {
								break
							}
							t207, err := b1(0)
							if err != nil {
								return 0, err
							}
							c208 := t207
							if !c208 {
								t209, err := b1(v1)
								if err != nil {
									return 0, err
								}
								c208 = t209
							}
							if !(c208) {
								break
							}
							{
								if err := f3(v3); err != nil {
									return 0, err
								}
							}
							{
								if v2 < 3 {
									continue
								}
							}
						}
					}
				} else {
					c210 := (9 - 7) < v0
					if c210 {
						c211 := v2 < 9
						if !c211 {
							t212, err := f1(6)
							if err != nil {
								return 0, err
							}
							c211 = t212 < (v2 - 7)
						}
						c210 = c211
					}
					if c210 {
						{
							t213, err := f1(v1)
							if err != nil {
								return 0, err
							}
							if err := f3(t213); err != nil {
								return 0, err
							}
						}
						{
							t214, err := b1((8 - 7))
							if err != nil {
								return 0, err
							}
							if t214 {
								{
									if err := f3(v1); err != nil {
										return 0, err
									}
								}
								{
									v3 += v3
								}
								{
									if err := f3((v1 + v1)); err != nil {
										return 0, err
									}
								}
							} else {
								t215, err := b1(v0)
								if err != nil {
									return 0, err
								}
								if t215 {
									{
										v0 = v3
									}
								} else {
									t216, err := b1(2)
									if err != nil {
										return 0, err
									}
									c217 := t216
									if !c217 {
										t218, err := b1(7)
										if err != nil {
											return 0, err
										}
										c217 = t218
									}
									if c217 {
										{
											if err := f3((v2 + v1)); err != nil {
												return 0, err
											}
										}
										{
											t219, err := f1(1)
											if err != nil {
												return 0, err
											}
											v0 = t219
										}
										{
											t220, err := f1(1)
											if err != nil {
												return 0, err
											}
											if err := f3(t220); err != nil {
												return 0, err
											}
										}
									}
								}
							}
						}
						{
							t221, err := f1((v0 + 1))
							if err != nil {
								return 0, err
							}
							v0 = t221
						}
					} else {
						t222, err := f1(v2)
						if err != nil {
							return 0, err
						}
						t223, err := b1(t222)
						if err != nil {
							return 0, err
						}
						if t223 {
							{
								t224, err := f1(v1)
								if err != nil {
									return 0, err
								}
								v3 = t224
							}
							{
								v1 = v0
							}
							{
								t225, err := f1(5)
								if err != nil {
									return 0, err
								}
								t226, err := f1(t225)
								if err != nil {
									return 0, err
								}
								switch t226 % 3 {
								case 0:
									{
										v3 = v2
									}
									{
										t227, err := f1(v3)
										if err != nil {
											return 0, err
										}
										t228, err := f1(t227)
										if err != nil {
											return 0, err
										}
										v0 = t228
									}
									{
										t229, err := f1(v1)
										if err != nil {
											return 0, err
										}
										a230, b230, err := f2(t229)
										if err != nil {
											return 0, err
										}
										v3, v0 = a230, b230
									}
								case 1:
									{
										t231, err := f1(v2)
										if err != nil {
											return 0, err
										}
										if err := f3(t231); err != nil {
											return 0, err
										}
									}
								default:
									{
										t232, err := f1(2)
										if err != nil {
											return 0, err
										}
										v3 = t232
									}
									{
										t233, err := f1(v3)
										if err != nil {
											return 0, err
										}
										v0 = t233
									}
									{
										v1 = v1
									}
								}
							}
						}
					}
				}
			}
			{
				v2 = (2 + v0)
			}
			{
				v2 = v2
			}
		default:
			{
				v1 = v0
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
