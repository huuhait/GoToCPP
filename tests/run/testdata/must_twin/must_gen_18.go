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
		if e := f3(28); err == nil {
			err = e
		}
	}()
	{
		for i1 := 0; ; i1++ {
			if !(i1 < 3) {
				break
			}
			t101, err := f1(v2)
			if err != nil {
				return 0, err
			}
			t102, err := b1(t101)
			if err != nil {
				return 0, err
			}
			if !(t102) {
				break
			}
			{
				t103, err := f1(v0)
				if err != nil {
					return 0, err
				}
				if t103 < 6 {
					continue
				}
			}
			{
				t104, err := f1(0)
				if err != nil {
					return 0, err
				}
				if t104 < 6 {
					break
				}
			}
		}
	}
	{
		switch (v2 + 8) % 3 {
		case 0:
			{
				t106, err := b1(7)
				if err != nil {
					return 0, err
				}
				if t106 {
					{
						t108, err := b1((v0 - v3))
						if err != nil {
							return 0, err
						}
						if t108 {
							{
								t109, err := f1(v1)
								if err != nil {
									return 0, err
								}
								v0 = t109
							}
							{
								t110, err := f1(v0)
								if err != nil {
									return 0, err
								}
								v3 = t110
							}
						} else {
							{
								v2 = (v1 + v1)
							}
							{
								t111, err := f1(v0)
								if err != nil {
									return 0, err
								}
								v3 += (t111 + (v1 - v0))
							}
							{
								v3 = 5
							}
						}
					}
					{
						t112, err := f1(v3)
						if err != nil {
							return 0, err
						}
						t113, err := b1(t112)
						if err != nil {
							return 0, err
						}
						if t113 {
							{
								if err := f3(v3); err != nil {
									return 0, err
								}
							}
						} else {
							{
								t114, err := f1(v1)
								if err != nil {
									return 0, err
								}
								a115, b115, err := f2(t114)
								if err != nil {
									return 0, err
								}
								v1, v3 = a115, b115
							}
						}
					}
				}
			}
		case 1:
			{
				t117, err := b1(4)
				if err != nil {
					return 0, err
				}
				if !(t117) {
					{
						t118, err := f1(v1)
						if err != nil {
							return 0, err
						}
						v2 = t118
					}
				} else {
					t119, err := b1((v2 + v2))
					if err != nil {
						return 0, err
					}
					if !(!(t119)) {
						{
							t120, err := f1((7 + v1))
							if err != nil {
								return 0, err
							}
							v2 = t120
						}
					} else {
						{
							for i2 := 0; ; i2++ {
								if !(i2 < 3) {
									break
								}
								t121, err := f1(v3)
								if err != nil {
									return 0, err
								}
								t122, err := b1(t121)
								if err != nil {
									return 0, err
								}
								if !(t122) {
									break
								}
								{
									t123, err := f1((8 - v0))
									if err != nil {
										return 0, err
									}
									v0 = t123
								}
							}
						}
						{
							t124, err := f1(v0)
							if err != nil {
								return 0, err
							}
							v0 = t124
						}
					}
				}
			}
			{
				v0 = v0
			}
		default:
			{
				t125, err := b1((v2 - v3))
				if err != nil {
					return 0, err
				}
				if t125 {
					{
						t127, err := f1(v0)
						if err != nil {
							return 0, err
						}
						t128, err := b1(t127)
						if err != nil {
							return 0, err
						}
						if t128 {
							{
								t129, err := f1(v2)
								if err != nil {
									return 0, err
								}
								v3 += (v3 + t129)
							}
							{
								a130, b130, err := f2(v0)
								if err != nil {
									return 0, err
								}
								v1, v2 = a130, b130
							}
							{
								v3 = ((1 + v1) + v0)
							}
						} else {
							if (v2 - 3) < v2 {
								{
									t131, err := f1(2)
									if err != nil {
										return 0, err
									}
									v3 = t131
								}
								{
									t132, err := f1(v0)
									if err != nil {
										return 0, err
									}
									v2 = t132
								}
								{
									v3 = (2 + v3)
								}
							} else {
								t133, err := b1((1 - 2))
								if err != nil {
									return 0, err
								}
								if t133 {
									{
										v0 = v1
									}
									{
										v3 = v2
									}
								} else {
									{
										if err := f3(v1); err != nil {
											return 0, err
										}
									}
									{
										v0 = 6
									}
									{
										t134, err := f1(v3)
										if err != nil {
											return 0, err
										}
										v2 = t134
									}
								}
							}
						}
					}
					{
						t135, err := f1(1)
						if err != nil {
							return 0, err
						}
						v0 += (t135 + v1)
					}
					{
						v2 = (v2 - v2)
					}
				} else {
					t136, err := b1((v3 - v2))
					if err != nil {
						return 0, err
					}
					if t136 {
						{
							t137, err := f1(v3)
							if err != nil {
								return 0, err
							}
							t138, err := b1(t137)
							if err != nil {
								return 0, err
							}
							if t138 {
								{
									v2 = (v3 - 2)
								}
								{
									v2 = v1
								}
								{
									t139, err := f1(1)
									if err != nil {
										return 0, err
									}
									if err := f3(t139); err != nil {
										return 0, err
									}
								}
							} else {
								t140, err := b1(v1)
								if err != nil {
									return 0, err
								}
								if t140 {
									{
										t141, err := f1(v3)
										if err != nil {
											return 0, err
										}
										a142, b142, err := f2(t141)
										if err != nil {
											return 0, err
										}
										v1, v0 = a142, b142
									}
									{
										v3 = v3
									}
								}
							}
						}
						{
							t143, err := f1(v1)
							if err != nil {
								return 0, err
							}
							a144, b144, err := f2(t143)
							if err != nil {
								return 0, err
							}
							v2, v1 = a144, b144
						}
					} else {
						t145, err := b1((0 - v0))
						if err != nil {
							return 0, err
						}
						if t145 {
							{
								switch v0 % 3 {
								case 0:
									{
										t146, err := f1(9)
										if err != nil {
											return 0, err
										}
										t147, err := f1(t146)
										if err != nil {
											return 0, err
										}
										v3 += t147
									}
									{
										v3 += 2
									}
									{
										v1 = (0 - 2)
									}
								case 1:
									{
										t148, err := f1(v3)
										if err != nil {
											return 0, err
										}
										v1 = t148
									}
									{
										t149, err := f1(9)
										if err != nil {
											return 0, err
										}
										t150, err := f1(t149)
										if err != nil {
											return 0, err
										}
										v3 += t150
									}
								default:
									{
										t151, err := f1(5)
										if err != nil {
											return 0, err
										}
										a152, b152, err := f2(t151)
										if err != nil {
											return 0, err
										}
										v0, v1 = a152, b152
									}
									{
										if err := f3((v3 + v0)); err != nil {
											return 0, err
										}
									}
									{
										a153, b153, err := f2((v1 + v3))
										if err != nil {
											return 0, err
										}
										v3, v2 = a153, b153
									}
								}
							}
							{
								for i3 := 0; ; i3++ {
									if !(i3 < 2) {
										break
									}
									{
										t154, err := f1(v2)
										if err != nil {
											return 0, err
										}
										if t154 < 4 {
											continue
										}
									}
								}
							}
							{
								v0 = (v2 + (v2 + v1))
							}
						} else {
							{
								t155, err := f1(7)
								if err != nil {
									return 0, err
								}
								t156, err := f1(t155)
								if err != nil {
									return 0, err
								}
								switch t156 % 3 {
								case 0:
									{
										t157, err := f1(v1)
										if err != nil {
											return 0, err
										}
										if err := f3(t157); err != nil {
											return 0, err
										}
									}
									{
										v2 = (v2 - v3)
									}
									{
										t158, err := f1(v0)
										if err != nil {
											return 0, err
										}
										v2 += t158
									}
								case 1:
									{
										v1 = v2
									}
									{
										t159, err := f1(v3)
										if err != nil {
											return 0, err
										}
										if err := f3(t159); err != nil {
											return 0, err
										}
									}
								default:
									{
										t160, err := f1(9)
										if err != nil {
											return 0, err
										}
										v2 = t160
									}
								}
							}
						}
					}
				}
			}
			{
				switch v3 % 3 {
				case 0:
					{
						for i4 := 0; ; i4++ {
							if !(i4 < 3) {
								break
							}
							{
								v1 += ((v3 + v0) + (8 + v3))
							}
							{
								if v3 < 7 {
									continue
								}
							}
							{
								v1 = (v1 - v3)
							}
						}
					}
					{
						t161, err := f1(v3)
						if err != nil {
							return 0, err
						}
						v0 = (v1 - t161)
					}
					{
						t162, err := f1((v1 + v3))
						if err != nil {
							return 0, err
						}
						switch t162 % 3 {
						case 0:
							{
								t163, err := f1(v2)
								if err != nil {
									return 0, err
								}
								v2 = t163
							}
						case 1:
							{
								if err := f3(v0); err != nil {
									return 0, err
								}
							}
							{
								v3 = v0
							}
							{
								t164, err := f1(v1)
								if err != nil {
									return 0, err
								}
								v2 = t164
							}
						default:
							{
								t165, err := f1(6)
								if err != nil {
									return 0, err
								}
								v1 = t165
							}
						}
					}
				case 1:
					{
						a166, b166, err := f2(v2)
						if err != nil {
							return 0, err
						}
						v2, v1 = a166, b166
					}
					{
						switch v0 % 3 {
						case 0:
							{
								if err := f3((v0 - v3)); err != nil {
									return 0, err
								}
							}
						case 1:
							{
								v2 = v0
							}
							{
								t167, err := f1((v0 + v1))
								if err != nil {
									return 0, err
								}
								v3 = t167
							}
							{
								v1 += v0
							}
						default:
							{
								v1 = v1
							}
							{
								v0 += (v2 - 2)
							}
							{
								if err := f3((7 - v0)); err != nil {
									return 0, err
								}
							}
						}
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
						v3, v1 = a169, b169
					}
				default:
					{
						v1 = 7
					}
				}
			}
		}
	}
	{
		for i5 := 0; ; i5++ {
			if !(i5 < 2) {
				break
			}
			t170, err := b1(v0)
			if err != nil {
				return 0, err
			}
			if !(t170) {
				break
			}
			{
				t172, err := b1(v2)
				if err != nil {
					return 0, err
				}
				if t172 {
					{
						v1 = v2
					}
					{
						if v3 < 4 {
							continue
						}
					}
					{
						t173, err := f1(v2)
						if err != nil {
							return 0, err
						}
						v2 = (v2 - t173)
					}
				} else {
					t174, err := b1(v0)
					if err != nil {
						return 0, err
					}
					c175 := t174
					if !c175 {
						t176, err := b1(v3)
						if err != nil {
							return 0, err
						}
						c175 = t176
					}
					if c175 {
						{
							v2 += ((v2 - v2) - v0)
						}
					}
				}
			}
			{
				switch v0 % 3 {
				case 0:
					{
						t177, err := f1(v3)
						if err != nil {
							return 0, err
						}
						if err := f3(t177); err != nil {
							return 0, err
						}
					}
				case 1:
					{
						t178, err := f1(v1)
						if err != nil {
							return 0, err
						}
						v0 = ((v1 - v1) + t178)
					}
					{
						t179, err := f1(v1)
						if err != nil {
							return 0, err
						}
						t180, err := f1(t179)
						if err != nil {
							return 0, err
						}
						v3 = t180
					}
				default:
					{
						t181, err := f1(v0)
						if err != nil {
							return 0, err
						}
						v0 = ((5 + v3) - t181)
					}
					{
						t182, err := f1(v1)
						if err != nil {
							return 0, err
						}
						if t182 < 4 {
							break
						}
					}
				}
			}
			{
				t183, err := f1(v0)
				if err != nil {
					return 0, err
				}
				t184, err := f1(t183)
				if err != nil {
					return 0, err
				}
				v2 = t184
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
