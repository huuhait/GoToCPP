// bench: select_loop
// category: runtime
// size: 1000000
// smoke: 5000
// twin: no
// about: a consumer selects over two data channels and a quit channel while two producers send
package main

import "fmt"

func main() {
	n := benchN
	a := make(chan int, 16)
	b := make(chan int, 16)
	quit := make(chan bool)
	go func() {
		for i := 0; i < n/2; i++ {
			a <- i
		}
		quit <- true
	}()
	go func() {
		for i := 0; i < n/2; i++ {
			b <- i * 2
		}
		quit <- true
	}()
	sa, sb, na, nb, stopped := 0, 0, 0, 0, 0
	for stopped < 2 || na+nb < n {
		select {
		case v := <-a:
			sa += v
			na++
		case v := <-b:
			sb += v
			nb++
		case <-quit:
			stopped++
		}
	}
	fmt.Println(sa, sb, na, nb)
}
