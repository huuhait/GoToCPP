//test: cmd=check
package main

func f(n int) int {
	if n > 0 {
		return 1
	}
}

func main() { println(f(1)) }
