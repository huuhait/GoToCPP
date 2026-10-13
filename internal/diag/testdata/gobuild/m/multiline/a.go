// Messages with more lines (have/want, missing method).
package multiline

type I interface{ M() }

type T struct{}

func g(int) {}

func f(i I) {
	g(1, 2)
	_ = i.(T)
	var _ I = T{}
}
