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
		if e := f3(10); err == nil {
			err = e
		}
	}()
	{
		t101, err := b1(v3)
		if err != nil {
			return 0, err
		}
		if t101 {
			{
				v3 = v2
			}
			{
				switch 0 % 3 {
				case 0:
					{
						v1 = v3
					}
					{
						t102, err := f1(v3)
						if err != nil {
							return 0, err
						}
						t103, err := f1(t102)
						if err != nil {
							return 0, err
						}
						switch t103 % 3 {
						case 0:
							{
								t104, err := f1(v2)
								if err != nil {
									return 0, err
								}
								t105, err := f1(v3)
								if err != nil {
									return 0, err
								}
								v2 = (t104 - t105)
							}
						case 1:
							{
								v1 = (v1 + v1)
							}
						default:
							{
								t106, err := f1(v0)
								if err != nil {
									return 0, err
								}
								t107, err := f1(v0)
								if err != nil {
									return 0, err
								}
								v0 = (t106 - t107)
							}
						}
					}
					{
						t108, err := b1(v0)
						if err != nil {
							return 0, err
						}
						if t108 {
							{
								t109, err := f1(v2)
								if err != nil {
									return 0, err
								}
								t110, err := f1(t109)
								if err != nil {
									return 0, err
								}
								v0 = t110
							}
							{
								v2 = v0
							}
							{
								a111, b111, err := f2(v0)
								if err != nil {
									return 0, err
								}
								v1, v2 = a111, b111
							}
						}
					}
				case 1:
					{
						t112, err := f1(v2)
						if err != nil {
							return 0, err
						}
						v0 += (t112 - (v3 + 9))
					}
					{
						if !(!(v3 < v1)) {
							{
								t113, err := f1(v2)
								if err != nil {
									return 0, err
								}
								v0 = t113
							}
						} else {
							{
								t114, err := f1(0)
								if err != nil {
									return 0, err
								}
								v0 = t114
							}
						}
					}
				default:
					{
						v2 = 0
					}
					{
						if err := f3((v0 - v2)); err != nil {
							return 0, err
						}
					}
				}
			}
			{
				t115, err := b1((v2 - v3))
				if err != nil {
					return 0, err
				}
				if t115 {
					{
						t116, err := f1((v3 + 5))
						if err != nil {
							return 0, err
						}
						switch t116 % 3 {
						case 0:
							{
								if err := f3(v0); err != nil {
									return 0, err
								}
							}
							{
								v1 = (v3 + v2)
							}
						case 1:
							{
								t117, err := f1(v3)
								if err != nil {
									return 0, err
								}
								v0 = t117
							}
							{
								t118, err := f1(v3)
								if err != nil {
									return 0, err
								}
								v3 = (t118 - (8 + 6))
							}
							{
								t119, err := f1(1)
								if err != nil {
									return 0, err
								}
								if err := f3(t119); err != nil {
									return 0, err
								}
							}
						default:
							{
								v0 = v1
							}
						}
					}
				} else {
					t120, err := b1((8 - v3))
					if err != nil {
						return 0, err
					}
					if t120 {
						{
							t121, err := f1((7 + 8))
							if err != nil {
								return 0, err
							}
							v2 = t121
						}
					} else {
						t122, err := f1(v2)
						if err != nil {
							return 0, err
						}
						t123, err := b1(t122)
						if err != nil {
							return 0, err
						}
						if t123 {
							{
								for i1 := 0; ; i1++ {
									if !(i1 < 2) {
										break
									}
									t124, err := b1((v3 - v0))
									if err != nil {
										return 0, err
									}
									c125 := !(t124)
									if c125 {
										t126, err := b1(v3)
										if err != nil {
											return 0, err
										}
										c125 = t126
									}
									if !(c125) {
										break
									}
									{
										if (v1 - 0) < 6 {
											continue
										}
									}
								}
							}
							{
								a127, b127, err := f2(v1)
								if err != nil {
									return 0, err
								}
								v1, v0 = a127, b127
							}
							{
								t128, err := f1(v1)
								if err != nil {
									return 0, err
								}
								v0 = (3 + t128)
							}
						} else {
							{
								v1 = (v2 - (v3 + v0))
							}
							{
								t129, err := f1(4)
								if err != nil {
									return 0, err
								}
								if err := f3(t129); err != nil {
									return 0, err
								}
							}
							{
								t130, err := f1(v0)
								if err != nil {
									return 0, err
								}
								v1 = t130
							}
						}
					}
				}
			}
		} else {
			t131, err := f1(v1)
			if err != nil {
				return 0, err
			}
			if t131 < (v3 + v3) {
				{
					t132, err := b1((v2 - v1))
					if err != nil {
						return 0, err
					}
					if t132 {
						{
							for i2 := 0; ; i2++ {
								if !(i2 < 3) {
									break
								}
								t133, err := b1((2 - v2))
								if err != nil {
									return 0, err
								}
								if !(t133) {
									break
								}
								{
									if v3 < 3 {
										continue
									}
								}
							}
						}
					} else {
						t134, err := f1(v3)
						if err != nil {
							return 0, err
						}
						t135, err := b1(t134)
						if err != nil {
							return 0, err
						}
						if t135 {
							{
								t136, err := b1(0)
								if err != nil {
									return 0, err
								}
								if !(t136) {
									{
										v3 = v1
									}
									{
										a137, b137, err := f2((v0 - 1))
										if err != nil {
											return 0, err
										}
										v0, v1 = a137, b137
									}
									{
										v2 = v3
									}
								} else {
									{
										v0 = (4 - v0)
									}
									{
										v0 = 0
									}
									{
										v2 = v1
									}
								}
							}
							{
								t138, err := b1(v3)
								if err != nil {
									return 0, err
								}
								if t138 {
									{
										t139, err := f1(v1)
										if err != nil {
											return 0, err
										}
										a140, b140, err := f2(t139)
										if err != nil {
											return 0, err
										}
										v1, v3 = a140, b140
									}
									{
										t141, err := f1((v3 + v2))
										if err != nil {
											return 0, err
										}
										v1 = t141
									}
								}
							}
							{
								v3 = 8
							}
						} else {
							t142, err := b1(v2)
							if err != nil {
								return 0, err
							}
							c143 := t142
							if !c143 {
								t144, err := f1(v0)
								if err != nil {
									return 0, err
								}
								t145, err := b1(t144)
								if err != nil {
									return 0, err
								}
								c143 = !(t145)
							}
							if c143 {
								{
									t146, err := b1((v1 + 2))
									if err != nil {
										return 0, err
									}
									c147 := t146
									if c147 {
										t148, err := b1((v0 - 6))
										if err != nil {
											return 0, err
										}
										c147 = t148
									}
									c149 := c147
									if !c149 {
										t150, err := b1(v3)
										if err != nil {
											return 0, err
										}
										c149 = t150
									}
									if c149 {
										{
											v2 = v3
										}
									} else {
										t151, err := b1((v1 - 7))
										if err != nil {
											return 0, err
										}
										c152 := (t151 || (v3 - v1) < v0)
										if !c152 {
											t153, err := b1(v1)
											if err != nil {
												return 0, err
											}
											c152 = t153
										}
										if c152 {
											{
												v1 = (v0 + (v2 - v0))
											}
										} else {
											{
												a154, b154, err := f2(v3)
												if err != nil {
													return 0, err
												}
												v1, v2 = a154, b154
											}
											{
												t155, err := f1(7)
												if err != nil {
													return 0, err
												}
												v3 = t155
											}
										}
									}
								}
								{
									if (v2 + v3) < v3 {
										{
											t156, err := f1(v2)
											if err != nil {
												return 0, err
											}
											a157, b157, err := f2(t156)
											if err != nil {
												return 0, err
											}
											v2, v1 = a157, b157
										}
										{
											t158, err := f1(5)
											if err != nil {
												return 0, err
											}
											t159, err := f1(v2)
											if err != nil {
												return 0, err
											}
											v2 = (t158 + t159)
										}
									} else {
										t160, err := b1(v3)
										if err != nil {
											return 0, err
										}
										if t160 {
											{
												a161, b161, err := f2(0)
												if err != nil {
													return 0, err
												}
												v2, v0 = a161, b161
											}
											{
												t162, err := f1(5)
												if err != nil {
													return 0, err
												}
												t163, err := f1(t162)
												if err != nil {
													return 0, err
												}
												v1 += t163
											}
										}
									}
								}
								{
									t164, err := f1((v3 - v3))
									if err != nil {
										return 0, err
									}
									switch t164 % 3 {
									case 0:
										{
											a165, b165, err := f2(3)
											if err != nil {
												return 0, err
											}
											v1, v2 = a165, b165
										}
										{
											a166, b166, err := f2((v3 + v3))
											if err != nil {
												return 0, err
											}
											v0, v1 = a166, b166
										}
										{
											a167, b167, err := f2(v3)
											if err != nil {
												return 0, err
											}
											v2, v0 = a167, b167
										}
									case 1:
										{
											t168, err := f1(v1)
											if err != nil {
												return 0, err
											}
											v1 = t168
										}
										{
											v3 = v2
										}
										{
											v1 = v0
										}
									default:
										{
											v0 = v1
										}
									}
								}
							} else {
								{
									t169, err := b1((v2 + v2))
									if err != nil {
										return 0, err
									}
									c170 := t169
									if c170 {
										t171, err := b1((9 + v0))
										if err != nil {
											return 0, err
										}
										c170 = t171
									}
									c172 := c170
									if !c172 {
										t173, err := b1(v1)
										if err != nil {
											return 0, err
										}
										c172 = t173
									}
									if c172 {
										{
											t174, err := f1(v1)
											if err != nil {
												return 0, err
											}
											v1 = t174
										}
										{
											t175, err := f1(v3)
											if err != nil {
												return 0, err
											}
											v3 = t175
										}
									} else {
										t176, err := b1(1)
										if err != nil {
											return 0, err
										}
										if !(t176) {
											{
												t177, err := f1(v2)
												if err != nil {
													return 0, err
												}
												t178, err := f1(t177)
												if err != nil {
													return 0, err
												}
												v2 = t178
											}
											{
												v2 = 3
											}
											{
												v0 += ((v2 + v3) + (v2 - v3))
											}
										} else {
											{
												if err := f3((v3 - v2)); err != nil {
													return 0, err
												}
											}
											{
												v1 = v2
											}
											{
												v1 = 0
											}
										}
									}
								}
								{
									t179, err := f1(4)
									if err != nil {
										return 0, err
									}
									v3 = ((0 - v3) + t179)
								}
								{
									if err := f3((v2 - 3)); err != nil {
										return 0, err
									}
								}
							}
						}
					}
				}
				{
					t180, err := f1(v0)
					if err != nil {
						return 0, err
					}
					t181, err := b1(t180)
					if err != nil {
						return 0, err
					}
					if t181 {
						{
							v0 = (v2 - v1)
						}
					} else {
						if (v1 - v2) < v1 {
							{
								a182, b182, err := f2(v2)
								if err != nil {
									return 0, err
								}
								v3, v0 = a182, b182
							}
						}
					}
				}
				{
					t184, err := f1(v3)
					if err != nil {
						return 0, err
					}
					if (v3 - v2) < t184 {
						{
							t185, err := f1(v3)
							if err != nil {
								return 0, err
							}
							a186, b186, err := f2(t185)
							if err != nil {
								return 0, err
							}
							v2, v1 = a186, b186
						}
					} else {
						t187, err := b1(1)
						if err != nil {
							return 0, err
						}
						c188 := (t187 || (3 - v3) < 5)
						if !c188 {
							t189, err := b1((v1 - v0))
							if err != nil {
								return 0, err
							}
							c188 = !(t189)
						}
						if c188 {
							{
								t190, err := f1(v1)
								if err != nil {
									return 0, err
								}
								v3 = t190
							}
							{
								for i3 := 0; ; i3++ {
									if !(i3 < 3) {
										break
									}
									t191, err := b1(v3)
									if err != nil {
										return 0, err
									}
									if !(t191) {
										break
									}
									{
										t192, err := f1(8)
										if err != nil {
											return 0, err
										}
										if err := f3(t192); err != nil {
											return 0, err
										}
									}
									{
										v0 += v3
									}
								}
							}
						} else {
							t193, err := f1(v3)
							if err != nil {
								return 0, err
							}
							t194, err := b1(t193)
							if err != nil {
								return 0, err
							}
							if t194 {
								{
									for i4 := 0; ; i4++ {
										if !(i4 < 2) {
											break
										}
										t195, err := b1(v0)
										if err != nil {
											return 0, err
										}
										c196 := t195
										if !c196 {
											t197, err := f1(v2)
											if err != nil {
												return 0, err
											}
											c198 := v0 < t197
											if !c198 {
												t199, err := f1(v0)
												if err != nil {
													return 0, err
												}
												c198 = t199 < 9
											}
											c196 = c198
										}
										if !(c196) {
											break
										}
										{
											v2 += v2
										}
										{
											t200, err := f1(1)
											if err != nil {
												return 0, err
											}
											v3 = ((v3 - v0) - t200)
										}
									}
								}
								{
									for i5 := 0; ; i5++ {
										if !(i5 < 3) {
											break
										}
										{
											if 2 < 6 {
												break
											}
										}
										{
											t201, err := f1(v1)
											if err != nil {
												return 0, err
											}
											v3 += (v2 - t201)
										}
										{
											t202, err := f1(v0)
											if err != nil {
												return 0, err
											}
											v1 += t202
										}
									}
								}
							}
						}
					}
				}
			} else {
				{
					t203, err := f1(v1)
					if err != nil {
						return 0, err
					}
					t204, err := b1(t203)
					if err != nil {
						return 0, err
					}
					if t204 {
						{
							t205, err := f1(v1)
							if err != nil {
								return 0, err
							}
							if err := f3(t205); err != nil {
								return 0, err
							}
						}
					} else {
						t206, err := f1(9)
						if err != nil {
							return 0, err
						}
						t207, err := b1(t206)
						if err != nil {
							return 0, err
						}
						if t207 {
							{
								a208, b208, err := f2(v3)
								if err != nil {
									return 0, err
								}
								v2, v1 = a208, b208
							}
							{
								t209, err := b1(v3)
								if err != nil {
									return 0, err
								}
								if t209 {
									{
										v3 = 2
									}
									{
										if err := f3(v0); err != nil {
											return 0, err
										}
									}
									{
										v0 = 7
									}
								} else {
									t210, err := b1(v2)
									if err != nil {
										return 0, err
									}
									if t210 {
										{
											v0 = v1
										}
										{
											a211, b211, err := f2((3 - v0))
											if err != nil {
												return 0, err
											}
											v3, v2 = a211, b211
										}
									} else {
										t212, err := f1(v0)
										if err != nil {
											return 0, err
										}
										t213, err := b1(t212)
										if err != nil {
											return 0, err
										}
										if t213 {
											{
												t214, err := f1(v3)
												if err != nil {
													return 0, err
												}
												v1 = ((0 - v1) - t214)
											}
											{
												t215, err := f1(v2)
												if err != nil {
													return 0, err
												}
												t216, err := f1(v0)
												if err != nil {
													return 0, err
												}
												v3 += (t215 + t216)
											}
										}
									}
								}
							}
							{
								t217, err := f1(v1)
								if err != nil {
									return 0, err
								}
								c218 := 0 < t217
								if !c218 {
									t219, err := b1(6)
									if err != nil {
										return 0, err
									}
									c218 = t219
								}
								c220 := c218
								if c220 {
									t221, err := b1(5)
									if err != nil {
										return 0, err
									}
									c220 = t221
								}
								if c220 {
									{
										if err := f3(v0); err != nil {
											return 0, err
										}
									}
									{
										v2 = (v0 + 0)
									}
								} else {
									if (v1 + v3) < (v0 + v1) {
										{
											v2 = v0
										}
										{
											v1 = v1
										}
									} else {
										t222, err := b1(v1)
										if err != nil {
											return 0, err
										}
										if t222 {
											{
												a223, b223, err := f2((v3 - v3))
												if err != nil {
													return 0, err
												}
												v0, v3 = a223, b223
											}
										}
									}
								}
							}
						} else {
							{
								v1 = 7
							}
						}
					}
				}
				{
					t224, err := b1(0)
					if err != nil {
						return 0, err
					}
					if t224 {
						{
							t225, err := f1(v0)
							if err != nil {
								return 0, err
							}
							switch t225 % 3 {
							case 0:
								{
									a226, b226, err := f2((v3 + v3))
									if err != nil {
										return 0, err
									}
									v0, v1 = a226, b226
								}
								{
									t227, err := f1(v3)
									if err != nil {
										return 0, err
									}
									v0 = t227
								}
								{
									t228, err := f1((v1 - v3))
									if err != nil {
										return 0, err
									}
									v2 += t228
								}
							case 1:
								{
									v3 = v2
								}
								{
									v0 = (5 + v2)
								}
							default:
								{
									v3 = ((v2 + v0) - (v1 + 7))
								}
								{
									v2 = (8 - v3)
								}
								{
									t229, err := f1(v3)
									if err != nil {
										return 0, err
									}
									v2 = t229
								}
							}
						}
					} else {
						{
							v2 = (2 + v1)
						}
						{
							for i6 := 0; ; i6++ {
								if !(i6 < 1) {
									break
								}
								t230, err := b1((v2 + v3))
								if err != nil {
									return 0, err
								}
								if !(t230) {
									break
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
					for i7 := 0; ; i7++ {
						if !(i7 < 1) {
							break
						}
						t231, err := f1(v3)
						if err != nil {
							return 0, err
						}
						t232, err := f1(v2)
						if err != nil {
							return 0, err
						}
						if !(t231 < t232) {
							break
						}
						{
							if (v1 - v0) < 3 {
								break
							}
						}
						{
							if err := f3((v2 + v1)); err != nil {
								return 0, err
							}
						}
					}
				}
			}
		}
	}
	{
		a233, b233, err := f2((v0 - v0))
		if err != nil {
			return 0, err
		}
		v1, v2 = a233, b233
	}
	{
		t234, err := b1(v0)
		if err != nil {
			return 0, err
		}
		if !(t234) {
			{
				t235, err := f1((1 - v1))
				if err != nil {
					return 0, err
				}
				v0 = t235
			}
			{
				for i8 := 0; ; i8++ {
					if !(i8 < 3) {
						break
					}
					t236, err := f1(v0)
					if err != nil {
						return 0, err
					}
					if !(!(t236 < (v3 + v3))) {
						break
					}
					{
						t237, err := f1(v3)
						if err != nil {
							return 0, err
						}
						v0 = ((v1 + v1) + t237)
					}
					{
						v1 = v3
					}
					{
						t238, err := f1(v3)
						if err != nil {
							return 0, err
						}
						if t238 < 3 {
							continue
						}
					}
				}
			}
		} else {
			t239, err := f1(5)
			if err != nil {
				return 0, err
			}
			t240, err := f1(v2)
			if err != nil {
				return 0, err
			}
			if t239 < t240 {
				{
					t241, err := f1(3)
					if err != nil {
						return 0, err
					}
					a242, b242, err := f2(t241)
					if err != nil {
						return 0, err
					}
					v1, v2 = a242, b242
				}
				{
					v3 += 4
				}
				{
					for i9 := 0; ; i9++ {
						if !(i9 < 2) {
							break
						}
						c243 := v0 < (v0 - v0)
						if !c243 {
							t244, err := f1(v3)
							if err != nil {
								return 0, err
							}
							c243 = t244 < (v3 + v0)
						}
						if !(c243) {
							break
						}
						{
							if v0 < 5 {
								continue
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
