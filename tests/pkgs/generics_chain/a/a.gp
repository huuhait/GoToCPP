package a

type Number interface{ ~int | ~float64 }

func Double[T Number](x T) T { return x * 2 }

func Apply[T any](x T, fs ...func(T) T) T {
	for _, f := range fs {
		x = f(x)
	}
	return x
}

type Meters float64
