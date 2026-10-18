package util

func Id[T any](x T) T { return x }

func Pair[A, B any](a A, b B) (B, A) { return b, a }

func Map[T, U any](xs []T, f func(T) U) []U {
	var out []U
	for _, x := range xs {
		out = append(out, f(x))
	}
	return out
}

func Dup[T any](x T) []T { return []T{x, x} }
