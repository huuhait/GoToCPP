package cons

import "fmt"

type Integer interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64
}

type Float interface{ ~float32 | ~float64 }

type Number interface{ Integer | Float }

type Named interface{ Name() string }

type Both interface {
	Number
	fmt.Stringer
}

func Abs[T Number](x T) T {
	if x < 0 {
		return -x
	}
	return x
}

func Clamp[T Number](x, lo, hi T) T { return min(max(x, lo), hi) }

func Greet[T Named](xs ...T) []string {
	var out []string
	for _, x := range xs {
		out = append(out, "hello "+x.Name())
	}
	return out
}

func Show[T Both](x T) string { return x.String() + "=" + fmt.Sprint(float64(x)) }
