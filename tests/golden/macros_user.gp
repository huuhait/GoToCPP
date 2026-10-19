// phase: 05

package main

import "fmt"

macro Assert(cond bool, msg lazy string) {
	if !cond {
		panic(fmt.Sprintf("%s:%d: %s: %s", #file(), #line(), #text(cond), msg()))
	}
}

macro Dbg[T any](v T) T {
	fmt.Println(#text(v), v)
	return v
}

macro Trace() func() {
	fmt.Println("enter", #func())
	return func() { fmt.Println("exit", #func()) }
}

macro Retry(n int) {
	for i := 0; i < n; i++ {
		#body()
	}
}

#Trace()
#Retry(2)
func work(a, b int) (int, error) {
	#Assert(b != 0, fmt.Sprint("b of ", a))
	return a / b, nil
}

func main() {
	x, err := work(6, #Dbg(3))
	fmt.Println(x, err)
}
