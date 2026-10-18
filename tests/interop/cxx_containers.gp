// phase: 04

// More of the standard library at the boundary (step 4.18): set (ordered, find, lower_bound,
// insert returns a pair), pair and tuple (first/second, std.get, make_tuple, structured results
// from a function), array, deque, list, stack/queue/priority_queue adapters, bitset, string_view,
// a stringstream, and std::function stored in a map.
package main

import (
	"fmt"

	"<algorithm>"
	"<array>"
	"<bitset>"
	"<deque>"
	"<functional>"
	"<list>"
	"<map>"
	"<queue>"
	"<set>"
	"<sstream>"
	"<stack>"
	"<string>"
	"<string_view>"
	"<tuple>"
	"<utility>"
)

func divmod(a, b int) std.pair[int, int] {
	return std.make_pair(a/b, a%b)
}

func main() {
	s := std.set[int]{5, 1, 3}
	r := s.insert(3) // a pair<iterator, bool>: the element was there
	r2 := s.insert(4)
	fmt.Println(int(s.size()), r.second, r2.second, int(*r2.first))
	for _, x := range s {
		fmt.Print(int(x), ",")
	}
	fmt.Println()
	fmt.Println(int(*s.lower_bound(2)), s.find(9) == s.end(), int(s.count(5)), int(s.erase(1)))

	lo, hi := std.minmax(17, 5) // a C++ pair result is two values
	pr := divmod(9, 4)
	fmt.Println(int(lo), int(hi), int(pr.first), int(pr.second))

	t := std.make_tuple(1, std.string("two"), 3.5)
	fmt.Println(int(std.get[0](t)), string(std.get[1](t)), float64(std.get[2](t)))
	fmt.Println(int(std.get[C.int](t)), int(std.tuple_size_v[std.tuple[int, int, int]]))

	a := std.array[C.int, 4]{4, 3, 2, 1}
	a[2] = 20
	sum := 0
	for _, x := range a {
		sum += int(x)
	}
	fmt.Println(int(a.size()), int(a.front()), int(a.back()), sum, int(std.size(a)))

	d := std.deque[int]{}
	d.push_back(2)
	d.push_front(1)
	d.push_back(3)
	fmt.Println(int(d.front()), int(d.back()), int(d[1]), int(d.size()))
	l := std.list[int]{3, 1, 2}
	l.sort()
	l.push_front(0)
	for _, x := range l {
		fmt.Print(int(x), ",")
	}
	fmt.Println()

	var st std.stack[int]
	st.push(1)
	st.push(2)
	fmt.Println(int(st.top()), int(st.size()))
	st.pop()
	fmt.Println(int(st.top()))
	var qu std.queue[std.string]
	qu.push("a")
	qu.push("b")
	fmt.Println(string(qu.front()), string(qu.back()))
	qu.pop()
	fmt.Println(string(qu.front()), bool(qu.empty()))
	var pq std.priority_queue[int]
	for _, x := range []int{3, 9, 4} {
		pq.push(x)
	}
	fmt.Println(int(pq.top()))
	pq.pop()
	fmt.Println(int(pq.top()), int(pq.size()))

	bs := std.bitset[8](0b1011)
	bs.set(7)
	fmt.Println(int(bs.count()), bs.test(1), bs.test(2), string(bs.to_string()))

	text := std.string("hello world")
	sv := std.string_view(text)
	fmt.Println(int(sv.size()), string(sv.substr(6)), int(sv.find("o")), sv.starts_with("hell"))

	var ss std.stringstream
	_ = ss << "n=" << 5 << ' ' << std.string("s") // C++ operators keep C++ meaning on C++ values
	fmt.Println(string(ss.str()))

	ops := std.map[std.string, std.function[func(C.int, C.int) C.int]]{}
	ops["add"] = func(a, b C.int) C.int { return a + b }
	ops["mul"] = func(a, b C.int) C.int { return a * b }
	fmt.Println(int(ops["add"](3, 4)), int(ops["mul"](3, 4)), int(ops.size()))
}

// Output:
// 4 false true 4
// 1,3,4,5,
// 3 true 1 1
// 5 17 2 1
// 1 two 3.5
// 1 3
// 4 4 1 28 4
// 1 3 2 3
// 0,1,2,3,
// 2 2
// 1
// a b
// b false
// 9
// 4 2
// 4 true false 10001011
// 11 world 4 true
// n=5 s
// 7 12 2
