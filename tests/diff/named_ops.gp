// phase: 03
// diff: stderr

// Every operator of a named type's underlying type on the named type (step 3.9): the small
// integer types wrap, division and shifts follow Go, floats print through fmt as their values,
// untyped constants convert to the named type, and the op= forms and ++/-- on variables, slice
// elements, fields and map entries.
package main

import "fmt"

type (
	I8    int8
	U8    uint8
	I16   int16
	U16   uint16
	I32   int32
	U32   uint32
	Int   int
	U64   uint64
	F32   float32
	F64   float64
	Shift uint
	Flag  bool
)

type Pair struct {
	a U8
	b F64
}

func main() {
	a, b := I8(100), I8(-100)
	m8 := I8(-128)
	fmt.Println(a+a, b-a, a*3, -m8, ^a, a/7, a%7, b/7, b%7, a&b, a|b, a^b, a&^b, a<<1, b>>2)
	u, v := U8(200), U8(7)
	fmt.Println(u+u, v-u, u*v, -u, ^u, u/v, u%v, u&v, u|v, u^v, u&^v, u<<1, u>>3)
	i16, u16, i32, u32, in, u64 := I16(32767), U16(1), I32(1<<30), U32(1), Int(-7), U64(1)
	fmt.Println(i16+1, u16-2, i32*4, u32<<31<<1, in/2, in%2, u64<<63>>62)
	var n Shift = 3
	fmt.Println(u<<n, a<<n, b>>n, U64(5)<<(n*20), Int(-1)>>(n*30))
	c := Int(-9223372036854775807 - 1)
	fmt.Println(c/-1, c%-1, c-1, -c)
	f, g := F64(1.5), F64(-0.25)
	fmt.Println(f+g, f-g, f*g, f/g, -f, f/0, -f/0, f > g, f == 1.5)
	h := F32(0.1)
	fmt.Println(h, h*3, h+F32(0.2), float32(h) == 0.1)
	var t, fl Flag = true, false
	fmt.Println(t && fl, t || fl, !t, t == fl, t != fl, f > g && t)
	x := Int(10)
	x += 5
	x -= 2
	x *= 3
	x /= 4
	x %= 7
	x <<= 3
	x >>= 1
	x &= 0x1e
	x |= 1
	x ^= 0xff
	x &^= 0xf0
	x++
	x--
	x--
	fmt.Println(x)
	s := []U8{250, 5}
	s[0] += 10
	s[1]--
	s[1] <<= n
	fmt.Println(s[0], s[1])
	p := Pair{a: 255}
	p.a++
	p.b -= 1.5
	fmt.Println(p, p.a == 0)
	m := map[string]U8{}
	m["a"] += 250
	m["a"] += 10
	m["b"]--
	m["b"] <<= 1
	m["c"] = 9
	m["c"] /= 2
	m["c"] %= 3
	m["d"] >>= n
	fmt.Println(m["a"], m["b"], m["c"], m["d"], len(m))
	mf := map[F64]F64{}
	mf[1.5] += 2
	mf[1.5] *= 3
	mf[1.5] /= 4
	fmt.Println(mf[1.5])
	var k Int = 3
	fmt.Println(k*k-k)
	fmt.Printf("%d %v %t %f %s|\n", u, f, t, g, Int(5))
	println(a, u, f, h, t, c)
}
