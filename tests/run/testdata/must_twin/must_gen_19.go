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
		t101, err := f1((v1 - v1))
		if err != nil {
			return 0, err
		}
		switch t101 % 3 {
		case 0:
			{
				t102, err := f1(v3)
				if err != nil {
					return 0, err
				}
				t103, err := b1(t102)
				if err != nil {
					return 0, err
				}
				if t103 {
					{
						switch (v3 + (v3 + 9)) % 3 {
						case 0:
							{
								t104, err := f1(v2)
								if err != nil {
									return 0, err
								}
								v2 = t104
							}
						case 1:
							{
								v0 = (8 - 2)
							}
						default:
							{
								v3 = v1
							}
						}
					}
					{
						for i1 := 0; ; i1++ {
							if !(i1 < 3) {
								break
							}
							t105, err := b1(v3)
							if err != nil {
								return 0, err
							}
							c106 := t105
							if c106 {
								t107, err := f1(v0)
								if err != nil {
									return 0, err
								}
								c106 = !((v2 + v0) < t107)
							}
							if !(c106) {
								break
							}
							{
								t108, err := f1(v3)
								if err != nil {
									return 0, err
								}
								a109, b109, err := f2(t108)
								if err != nil {
									return 0, err
								}
								v1, v3 = a109, b109
							}
							{
								if (v1 - v2) < 3 {
									break
								}
							}
						}
					}
					{
						for i2 := 0; ; i2++ {
							if !(i2 < 1) {
								break
							}
							t110, err := b1(v2)
							if err != nil {
								return 0, err
							}
							if !(t110) {
								break
							}
							{
								a111, b111, err := f2(9)
								if err != nil {
									return 0, err
								}
								v3, v0 = a111, b111
							}
						}
					}
				} else {
					t112, err := b1(v3)
					if err != nil {
						return 0, err
					}
					if t112 {
						{
							t113, err := f1(v2)
							if err != nil {
								return 0, err
							}
							if err := f3(t113); err != nil {
								return 0, err
							}
						}
						{
							t114, err := f1(6)
							if err != nil {
								return 0, err
							}
							v3 = t114
						}
					}
				}
			}
			{
				v0 = v0
			}
		case 1:
			{
				for i3 := 0; ; i3++ {
					if !(i3 < 2) {
						break
					}
					{
						t115, err := f1(v3)
						if err != nil {
							return 0, err
						}
						if 8 < t115 {
							{
								v2 = v2
							}
						}
					}
					{
						t116, err := b1((v1 + v0))
						if err != nil {
							return 0, err
						}
						if t116 {
							{
								if 2 < 6 {
									break
								}
							}
						}
					}
					{
						t117, err := f1((v0 - v1))
						if err != nil {
							return 0, err
						}
						v3 = t117
					}
				}
			}
		default:
			{
				t118, err := f1(v0)
				if err != nil {
					return 0, err
				}
				a119, b119, err := f2(t118)
				if err != nil {
					return 0, err
				}
				v2, v0 = a119, b119
			}
			{
				v1 = ((v0 + v1) - 4)
			}
		}
	}
	{
		t120, err := f1(v0)
		if err != nil {
			return 0, err
		}
		switch t120 % 3 {
		case 0:
			{
				t121, err := f1(v3)
				if err != nil {
					return 0, err
				}
				v2 = t121
			}
			{
				t122, err := f1(6)
				if err != nil {
					return 0, err
				}
				t123, err := b1(t122)
				if err != nil {
					return 0, err
				}
				if t123 {
					{
						for i4 := 0; ; i4++ {
							if !(i4 < 1) {
								break
							}
							{
								if 9 < 3 {
									continue
								}
							}
							{
								a124, b124, err := f2(v2)
								if err != nil {
									return 0, err
								}
								v1, v2 = a124, b124
							}
							{
								v0 = (v0 + (v1 - v1))
							}
						}
					}
					{
						t125, err := f1(5)
						if err != nil {
							return 0, err
						}
						a126, b126, err := f2(t125)
						if err != nil {
							return 0, err
						}
						v3, v2 = a126, b126
					}
				}
			}
		case 1:
			{
				t127, err := f1(v3)
				if err != nil {
					return 0, err
				}
				t128, err := f1(t127)
				if err != nil {
					return 0, err
				}
				v2 += t128
			}
			{
				v2 = v0
			}
		default:
			{
				v1 = v1
			}
			{
				a129, b129, err := f2((v2 + v2))
				if err != nil {
					return 0, err
				}
				v2, v3 = a129, b129
			}
			{
				for i5 := 0; ; i5++ {
					if !(i5 < 1) {
						break
					}
					t130, err := f1(v3)
					if err != nil {
						return 0, err
					}
					if !(t130 < (7 + v2)) {
						break
					}
					{
						a131, b131, err := f2((v3 - v0))
						if err != nil {
							return 0, err
						}
						v0, v1 = a131, b131
					}
					{
						c133 := v1 < v1
						if !c133 {
							t134, err := f1(v3)
							if err != nil {
								return 0, err
							}
							t135, err := b1(t134)
							if err != nil {
								return 0, err
							}
							c136 := t135
							if !c136 {
								t137, err := b1(9)
								if err != nil {
									return 0, err
								}
								c136 = t137
							}
							c133 = c136
						}
						if c133 {
							{
								if (v2 - 7) < 4 {
									goto sw132
								}
							}
						} else {
							t138, err := b1((v3 + v0))
							if err != nil {
								return 0, err
							}
							if t138 {
								{
									t139, err := f1(3)
									if err != nil {
										return 0, err
									}
									if t139 < 7 {
										goto sw132
									}
								}
								{
									v1 += v3
								}
							} else {
								{
									v3 = v3
								}
								{
									a140, b140, err := f2((v3 - v0))
									if err != nil {
										return 0, err
									}
									v1, v2 = a140, b140
								}
								{
									t141, err := f1(v0)
									if err != nil {
										return 0, err
									}
									if t141 < 5 {
										continue
									}
								}
							}
						}
					}
					sw132:
					{
					}
				}
			}
		}
	}
	{
		t142, err := b1((2 - v1))
		if err != nil {
			return 0, err
		}
		if t142 {
			{
				for i6 := 0; ; i6++ {
					if !(i6 < 3) {
						break
					}
					t143, err := f1(v0)
					if err != nil {
						return 0, err
					}
					t144, err := b1(t143)
					if err != nil {
						return 0, err
					}
					if !(t144) {
						break
					}
					{
						switch v2 % 3 {
						case 0:
							{
								t145, err := f1(v3)
								if err != nil {
									return 0, err
								}
								t146, err := f1(t145)
								if err != nil {
									return 0, err
								}
								v3 = t146
							}
						case 1:
							{
								if 9 < 7 {
									break
								}
							}
						default:
							{
								a147, b147, err := f2((v2 - v1))
								if err != nil {
									return 0, err
								}
								v2, v1 = a147, b147
							}
						}
					}
					{
						t148, err := b1(v2)
						if err != nil {
							return 0, err
						}
						if !(t148) {
							{
								t149, err := f1(8)
								if err != nil {
									return 0, err
								}
								if err := f3(t149); err != nil {
									return 0, err
								}
							}
						} else {
							t150, err := f1(5)
							if err != nil {
								return 0, err
							}
							t151, err := f1(v2)
							if err != nil {
								return 0, err
							}
							c152 := t150 < t151
							if c152 {
								t153, err := b1((5 + v1))
								if err != nil {
									return 0, err
								}
								c152 = t153
							}
							if !(c152) {
								{
									v3 = v2
								}
								{
									t154, err := f1(v3)
									if err != nil {
										return 0, err
									}
									a155, b155, err := f2(t154)
									if err != nil {
										return 0, err
									}
									v3, v1 = a155, b155
								}
								{
									t156, err := f1(v1)
									if err != nil {
										return 0, err
									}
									v3 = ((v3 - 9) + t156)
								}
							} else {
								t157, err := b1(v0)
								if err != nil {
									return 0, err
								}
								c158 := t157
								if !c158 {
									t159, err := b1(v0)
									if err != nil {
										return 0, err
									}
									c158 = t159
								}
								if c158 {
									{
										v0 += ((v1 - v0) - (v0 - v1))
									}
								}
							}
						}
					}
					{
						t160, err := f1(v3)
						if err != nil {
							return 0, err
						}
						if t160 < 7 {
							break
						}
					}
				}
			}
			{
				for i7 := 0; ; i7++ {
					if !(i7 < 1) {
						break
					}
					t161, err := f1(v1)
					if err != nil {
						return 0, err
					}
					if !((v2 + 8) < t161) {
						break
					}
					{
						for i8 := 0; ; i8++ {
							if !(i8 < 1) {
								break
							}
							t162, err := b1((v1 + v3))
							if err != nil {
								return 0, err
							}
							c163 := t162
							if !c163 {
								t164, err := f1(0)
								if err != nil {
									return 0, err
								}
								t165, err := b1(t164)
								if err != nil {
									return 0, err
								}
								c163 = t165
							}
							c166 := c163
							if c166 {
								t167, err := f1(v2)
								if err != nil {
									return 0, err
								}
								t168, err := b1(t167)
								if err != nil {
									return 0, err
								}
								c166 = t168
							}
							if !(c166) {
								break
							}
							{
								t169, err := f1(0)
								if err != nil {
									return 0, err
								}
								v2 += t169
							}
							{
								v2 = v2
							}
							{
								v3 += v0
							}
						}
					}
				}
			}
		} else {
			t170, err := b1(v1)
			if err != nil {
				return 0, err
			}
			if t170 {
				{
					for i9 := 0; ; i9++ {
						if !(i9 < 3) {
							break
						}
						{
							for i10 := 0; ; i10++ {
								if !(i10 < 3) {
									break
								}
								if !((4 - v0) < 6) {
									break
								}
								{
									v0 = v0
								}
								{
									if (4 + 6) < 5 {
										continue
									}
								}
							}
						}
					}
				}
			} else {
				t171, err := f1(v1)
				if err != nil {
					return 0, err
				}
				c172 := (0 + v2) < t171
				if c172 {
					t173, err := b1((v2 + v2))
					if err != nil {
						return 0, err
					}
					c172 = t173
				}
				c174 := c172
				if c174 {
					t175, err := b1(v0)
					if err != nil {
						return 0, err
					}
					c176 := t175
					if c176 {
						t177, err := f1(v0)
						if err != nil {
							return 0, err
						}
						t178, err := b1(t177)
						if err != nil {
							return 0, err
						}
						c176 = t178
					}
					c174 = c176
				}
				if c174 {
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
