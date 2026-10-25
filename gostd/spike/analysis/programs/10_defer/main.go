// defer, panic and recover: LIFO order, multiple results, named results changed by a deferred
// function, recover in a deferred function, and a panic that is recovered before it reaches
// main.
package main

func pair() (int, int) { return 1, 2 }

func named() (n int, err string) {
	defer func() { n++ }()
	return 5, "ok"
}

func safe(f func()) (msg string) {
	defer func() {
		if r := recover(); r != nil {
			s, _ := r.(string)
			println("recovered value", s)
			msg = "recovered"
		}
	}()
	f()
	return "clean"
}

func main() {
	defer println("deferred 1")
	defer println("deferred 2")
	a, b := pair()
	println(a, b)
	n, err := named()
	println(n, err)
	println(safe(func() { println("before panic") }))
	println(safe(func() { panic("boom") }))
	println("after safe")
}
