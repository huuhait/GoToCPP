// phase: 03

// A range owns its original slice header, not a proof about later indexing of the source.
package main

import "fmt"

func appendWithinCapacity() {
	s := make([]int, 2, 4)
	s[0] = 1
	s[1] = 2
	for i, v := range s {
		if i == 0 {
			s = append(s, 3)
			s[1] = 20 // the saved header still shares this array
		}
		fmt.Println("append", i, v)
	}
	fmt.Println("length", len(s))
}

func replaceThroughCallback() {
	s := []int{1, 2, 3}
	alias := s
	calls := 0
	get := func() []int {
		calls++
		return s
	}
	replace := func() {
		alias[2] = 30
		s = []int{9}
	}
	for i, v := range get() {
		if i == 0 {
			replace()
		}
		fmt.Println("replace", i, v)
	}
	fmt.Println("source", calls, len(s), s[0])
}

func checkSourceAfterCallback() {
	defer func() {
		fmt.Println("panic", recover())
	}()

	s := []int{4, 5, 6}
	shorten := func() {
		s = s[:1]
	}
	for i, v := range s {
		if i == 0 {
			shorten()
		}
		fmt.Println("snapshot", i, v)
		// i is valid for the saved range header, but not necessarily for s now.
		fmt.Println("index", s[i])
	}
}

func main() {
	appendWithinCapacity()
	replaceThroughCallback()
	checkSourceAfterCallback()
}
