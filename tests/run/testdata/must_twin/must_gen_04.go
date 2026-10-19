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
		if e := f3(8); err == nil {
			err = e
		}
	}()
	defer func() {
		if e := f3(18); err == nil {
			err = e
		}
	}()
	{
		t101, err := b1(v0)
		if err != nil {
			return 0, err
		}
		if t101 {
			{
				t102, err := f1(v2)
				if err != nil {
					return 0, err
				}
				t103, err := f1(t102)
				if err != nil {
					return 0, err
				}
				v1 = t103
			}
			{
				t104, err := f1(v0)
				if err != nil {
					return 0, err
				}
				switch (t104 - 5) % 3 {
				case 0:
					{
						for i1 := 0; ; i1++ {
							if !(i1 < 1) {
								break
							}
							t105, err := b1(v2)
							if err != nil {
								return 0, err
							}
							c106 := t105
							if !c106 {
								t107, err := f1(v2)
								if err != nil {
									return 0, err
								}
								t108, err := b1(t107)
								if err != nil {
									return 0, err
								}
								c106 = t108
							}
							if !(c106) {
								break
							}
							{
								t109, err := f1(v3)
								if err != nil {
									return 0, err
								}
								v3 = (t109 + v1)
							}
						}
					}
				case 1:
					{
						v2 = v1
					}
					{
						for i2 := 0; ; i2++ {
							if !(i2 < 1) {
								break
							}
							t110, err := b1(v0)
							if err != nil {
								return 0, err
							}
							if !(t110) {
								break
							}
							{
								if err := f3((v1 - 9)); err != nil {
									return 0, err
								}
							}
						}
					}
					{
						t111, err := b1((v3 - v2))
						if err != nil {
							return 0, err
						}
						if t111 {
							{
								v2 = 3
							}
							{
								v1 = (v1 - 1)
							}
							{
								v2 = 7
							}
						} else {
							t112, err := b1((v2 - v0))
							if err != nil {
								return 0, err
							}
							if t112 {
								{
									t113, err := f1(v2)
									if err != nil {
										return 0, err
									}
									v3 = ((v0 + v2) - t113)
								}
								{
									t114, err := f1((9 + v2))
									if err != nil {
										return 0, err
									}
									v3 = t114
								}
							} else {
								t115, err := b1((v3 + v1))
								if err != nil {
									return 0, err
								}
								if t115 {
									{
										a116, b116, err := f2((5 - v0))
										if err != nil {
											return 0, err
										}
										v3, v1 = a116, b116
									}
									{
										t117, err := f1(4)
										if err != nil {
											return 0, err
										}
										a118, b118, err := f2(t117)
										if err != nil {
											return 0, err
										}
										v3, v2 = a118, b118
									}
									{
										v2 += (v1 - v2)
									}
								}
							}
						}
					}
				default:
					{
						t119, err := f1(2)
						if err != nil {
							return 0, err
						}
						t120, err := f1(t119)
						if err != nil {
							return 0, err
						}
						switch t120 % 3 {
						case 0:
							{
								v1 = v3
							}
						case 1:
							{
								v0 = v0
							}
						default:
							{
								t121, err := f1((5 - 7))
								if err != nil {
									return 0, err
								}
								v2 = t121
							}
						}
					}
				}
			}
			{
				switch v2 % 3 {
				case 0:
					{
						t122, err := f1(v3)
						if err != nil {
							return 0, err
						}
						v0 = t122
					}
				case 1:
					{
						for i3 := 0; ; i3++ {
							if !(i3 < 3) {
								break
							}
							t123, err := f1(v0)
							if err != nil {
								return 0, err
							}
							t124, err := b1(t123)
							if err != nil {
								return 0, err
							}
							if !(t124) {
								break
							}
							{
								v1 = v2
							}
						}
					}
					{
						t125, err := b1((v1 - v3))
						if err != nil {
							return 0, err
						}
						if t125 {
							{
								v3 = v0
							}
							{
								t126, err := f1(6)
								if err != nil {
									return 0, err
								}
								v3 = t126
							}
						} else {
							t127, err := b1((5 - 4))
							if err != nil {
								return 0, err
							}
							c128 := t127
							if c128 {
								t129, err := f1(v1)
								if err != nil {
									return 0, err
								}
								t130, err := b1(t129)
								if err != nil {
									return 0, err
								}
								c128 = t130
							}
							c131 := c128
							if c131 {
								t132, err := b1(v2)
								if err != nil {
									return 0, err
								}
								c131 = t132
							}
							if c131 {
								{
									t133, err := f1(7)
									if err != nil {
										return 0, err
									}
									v0 = t133
								}
								{
									if err := f3(0); err != nil {
										return 0, err
									}
								}
							} else {
								t134, err := f1(v0)
								if err != nil {
									return 0, err
								}
								t135, err := b1(t134)
								if err != nil {
									return 0, err
								}
								if t135 {
									{
										v0 = v1
									}
								}
							}
						}
					}
				default:
					{
						t136, err := b1((v0 - v0))
						if err != nil {
							return 0, err
						}
						if t136 {
							{
								if err := f3((0 + v3)); err != nil {
									return 0, err
								}
							}
							{
								v3 = (v1 - v3)
							}
						} else {
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
									t139, err := f1(4)
									if err != nil {
										return 0, err
									}
									a140, b140, err := f2(t139)
									if err != nil {
										return 0, err
									}
									v1, v3 = a140, b140
								}
							} else {
								t141, err := f1(v1)
								if err != nil {
									return 0, err
								}
								t142, err := b1(t141)
								if err != nil {
									return 0, err
								}
								c143 := t142
								if c143 {
									t144, err := f1(1)
									if err != nil {
										return 0, err
									}
									c143 = t144 < (v2 + v1)
								}
								if c143 {
									{
										v0 = ((v2 - v1) - (6 + v3))
									}
									{
										if err := f3((v1 - 7)); err != nil {
											return 0, err
										}
									}
								} else {
									{
										a145, b145, err := f2(v0)
										if err != nil {
											return 0, err
										}
										v2, v1 = a145, b145
									}
									{
										t146, err := f1(v0)
										if err != nil {
											return 0, err
										}
										v0 = (v3 - t146)
									}
									{
										t147, err := f1(v2)
										if err != nil {
											return 0, err
										}
										v2 = t147
									}
								}
							}
						}
					}
					{
						t148, err := f1(v0)
						if err != nil {
							return 0, err
						}
						if err := f3(t148); err != nil {
							return 0, err
						}
					}
				}
			}
		} else {
			t149, err := f1(0)
			if err != nil {
				return 0, err
			}
			t150, err := f1(7)
			if err != nil {
				return 0, err
			}
			c151 := t149 < t150
			if !c151 {
				t152, err := f1(v1)
				if err != nil {
					return 0, err
				}
				c151 = v0 < t152
			}
			if !(c151) {
				{
					for i4 := 0; ; i4++ {
						if !(i4 < 1) {
							break
						}
						{
							t153, err := f1(v0)
							if err != nil {
								return 0, err
							}
							a154, b154, err := f2(t153)
							if err != nil {
								return 0, err
							}
							v2, v1 = a154, b154
						}
					}
				}
			} else {
				{
					for i5 := 0; ; i5++ {
						if !(i5 < 2) {
							break
						}
						{
							t155, err := f1(v0)
							if err != nil {
								return 0, err
							}
							if t155 < 4 {
								break
							}
						}
						{
							t156, err := b1(v0)
							if err != nil {
								return 0, err
							}
							if t156 {
								{
									t157, err := f1(v0)
									if err != nil {
										return 0, err
									}
									v2 = ((2 - v1) + t157)
								}
							} else {
								t158, err := b1(7)
								if err != nil {
									return 0, err
								}
								if t158 {
									{
										t159, err := f1((2 + 4))
										if err != nil {
											return 0, err
										}
										v2 = t159
									}
									{
										t160, err := f1(v1)
										if err != nil {
											return 0, err
										}
										if t160 < 7 {
											break
										}
									}
									{
										if v1 < 3 {
											continue
										}
									}
								} else {
									{
										if err := f3(8); err != nil {
											return 0, err
										}
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
		t161, err := b1(8)
		if err != nil {
			return 0, err
		}
		if t161 {
			{
				t162, err := b1(v0)
				if err != nil {
					return 0, err
				}
				c163 := !(t162)
				if !c163 {
					t164, err := b1(1)
					if err != nil {
						return 0, err
					}
					c163 = t164
				}
				if c163 {
					{
						a165, b165, err := f2((v1 + v1))
						if err != nil {
							return 0, err
						}
						v3, v1 = a165, b165
					}
					{
						if err := f3(v3); err != nil {
							return 0, err
						}
					}
				} else {
					t166, err := f1(v3)
					if err != nil {
						return 0, err
					}
					if v2 < t166 {
						{
							v2 += (v0 - (v3 + 0))
						}
					} else {
						if v3 < (v3 + 7) {
							{
								v0 = v2
							}
							{
								t167, err := f1(6)
								if err != nil {
									return 0, err
								}
								v0 += t167
							}
							{
								t168, err := f1(6)
								if err != nil {
									return 0, err
								}
								v0 += t168
							}
						}
					}
				}
			}
			{
				for i6 := 0; ; i6++ {
					if !(i6 < 1) {
						break
					}
					t169, err := b1(v1)
					if err != nil {
						return 0, err
					}
					if !(t169) {
						break
					}
					{
						t170, err := f1(6)
						if err != nil {
							return 0, err
						}
						if t170 < 5 {
							continue
						}
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
					{
						t172, err := f1(v0)
						if err != nil {
							return 0, err
						}
						t173, err := f1(t172)
						if err != nil {
							return 0, err
						}
						v0 = t173
					}
				}
			}
			{
				for i7 := 0; ; i7++ {
					if !(i7 < 3) {
						break
					}
					t174, err := f1(v0)
					if err != nil {
						return 0, err
					}
					t175, err := b1(t174)
					if err != nil {
						return 0, err
					}
					if !(t175) {
						break
					}
					{
						t176, err := f1(4)
						if err != nil {
							return 0, err
						}
						t177, err := f1(v3)
						if err != nil {
							return 0, err
						}
						v2 += (t176 - t177)
					}
				}
			}
		}
	}
	{
		switch ((v0 - 0) + v2) % 3 {
		case 0:
			{
				t178, err := f1(v2)
				if err != nil {
					return 0, err
				}
				a179, b179, err := f2(t178)
				if err != nil {
					return 0, err
				}
				v1, v3 = a179, b179
			}
			{
				v0 = (v3 + v1)
			}
			{
				t180, err := f1(6)
				if err != nil {
					return 0, err
				}
				v2 = t180
			}
		case 1:
			{
				t181, err := f1(v0)
				if err != nil {
					return 0, err
				}
				t182, err := b1(t181)
				if err != nil {
					return 0, err
				}
				if t182 {
					{
						v2 = (1 + v1)
					}
					{
						if err := f3((v0 + v0)); err != nil {
							return 0, err
						}
					}
					{
						v1 = v1
					}
				} else {
					t183, err := b1(v1)
					if err != nil {
						return 0, err
					}
					c184 := t183
					if !c184 {
						t185, err := f1(v0)
						if err != nil {
							return 0, err
						}
						t186, err := b1(t185)
						if err != nil {
							return 0, err
						}
						c184 = t186
					}
					c187 := c184
					if c187 {
						t188, err := b1(6)
						if err != nil {
							return 0, err
						}
						c187 = t188
					}
					if c187 {
						{
							v2 = v3
						}
						{
							t189, err := f1(v2)
							if err != nil {
								return 0, err
							}
							v3 = (t189 + v0)
						}
					} else {
						t190, err := b1((v1 - v1))
						if err != nil {
							return 0, err
						}
						if t190 {
							{
								v0 = v3
							}
						}
					}
				}
			}
			{
				t191, err := b1(v0)
				if err != nil {
					return 0, err
				}
				if !(t191) {
					{
						t192, err := f1(v2)
						if err != nil {
							return 0, err
						}
						t193, err := f1(t192)
						if err != nil {
							return 0, err
						}
						v2 += t193
					}
					{
						v2 = v1
					}
					{
						t194, err := f1(0)
						if err != nil {
							return 0, err
						}
						v0 = t194
					}
				} else {
					t195, err := b1((v1 - v2))
					if err != nil {
						return 0, err
					}
					if t195 {
						{
							t196, err := f1(1)
							if err != nil {
								return 0, err
							}
							t197, err := f1(t196)
							if err != nil {
								return 0, err
							}
							v1 = t197
						}
					}
				}
			}
		default:
			{
				for i8 := 0; ; i8++ {
					if !(i8 < 2) {
						break
					}
					t198, err := b1(v1)
					if err != nil {
						return 0, err
					}
					if !(t198) {
						break
					}
					{
						a199, b199, err := f2((4 + v2))
						if err != nil {
							return 0, err
						}
						v0, v3 = a199, b199
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
