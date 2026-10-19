// phase: 08

// go F!(...) names the call as it was written, with the ! before the type arguments (step 10.13).

package main

func gen[T any](x T) (T, error) { return x, nil }

func pair[A, B any](a A, b B) (A, error) { return a, nil }

func main() {
	go gen![int](1)            // ERROR "go gen!\[int\]\(...\): the error of gen can't be returned to anyone from another goroutine"
	go pair![int, string](1, "") // ERROR "go pair!\[int, string\]\(...\): the error of pair can't be returned"
}
