// phase: 02

// Every form of for: three clauses, condition only, forever with break,
// range over an integer (with and without a variable), and nested loops
// with break and continue.
package main

import "fmt"

func main() {
	sum := 0
	for i := 0; i < 10; i++ {
		sum += i
	}
	fmt.Println("three clauses:", sum)

	n := 1
	for n < 100 {
		n *= 3
	}
	fmt.Println("condition only:", n)

	k := 0
	for {
		k++
		if k == 7 {
			break
		}
	}
	fmt.Println("forever:", k)

	for i := range 4 {
		fmt.Print(i, " ")
	}
	fmt.Println("|")

	count := 0
	for range 5 {
		count++
	}
	fmt.Println("range without a variable:", count)

	var u uint8
	for u = range uint8(3) {
		fmt.Print(u, ";")
	}
	fmt.Println(" last", u)

	for i, j := 0, 10; i < j; i, j = i+1, j-2 {
		fmt.Print(i, ",", j, " ")
	}
	fmt.Println("|")

	for i := 0; i < 5; i++ {
		if i%2 == 0 {
			continue
		}
		for j := 0; j < 5; j++ {
			if j > i {
				break
			}
			fmt.Print(i*10+j, " ")
		}
	}
	fmt.Println("|")

	i := 0
	for i < 3 {
		i++
	}
	fmt.Println("counted to", i)
}

// Output:
// three clauses: 45
// condition only: 243
// forever: 7
// 0 1 2 3 |
// range without a variable: 5
// 0;1;2; last 2
// 0,10 1,8 2,6 3,4 |
// 10 11 30 31 32 33 |
// counted to 3
