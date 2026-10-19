// phase: 08

// len and cap over type sets of strings, slices, arrays, maps and channels.
package main

import "fmt"

type Lenable interface {
	~[]int | ~[3]int | ~map[int]int | ~chan int
}

type Capable interface {
	~[]int | ~[3]int | ~chan int
}

type Seq []int

func Len[T ~string | ~[]byte](x T) int { return len(x) }
func LenAny[T Lenable](x T) int { return len(x) }
func Cap[T Capable](x T) int { return cap(x) }

const ArrLen = len([5]int{})

func ArrayLen[A ~[4]int | ~[4]string]() int {
	var a A
	return len(a)
}

func PtrArrayLen[P *[6]int | *[6]byte](p P) int { return len(p) }

func main() {
	fmt.Println(Len("héllo"), Len([]byte{1, 2}), Len(""))
	ch := make(chan int, 5)
	ch <- 1
	ch <- 2
	fmt.Println(LenAny([]int{1, 2, 3}), LenAny([3]int{}), LenAny(map[int]int{1: 1}), LenAny(ch), LenAny(Seq(nil)))
	fmt.Println(Cap(make([]int, 2, 10)), Cap([3]int{}), Cap(ch), Cap(Seq{1}), Cap(make(chan int)))
	fmt.Println(ArrayLen[[4]int](), ArrayLen[[4]string](), ArrLen)
	var a [6]int
	fmt.Println(PtrArrayLen(&a))
	s := make([]int, 3, 8)
	fmt.Println(Cap(s[1:]), Cap(s[1:2:4]), Len(string(rune(0x1F600))))
}

// Output:
// 6 2 0
// 3 3 1 2 0
// 10 3 5 1 0
// 4 4 5
// 6
// 7 3 4
