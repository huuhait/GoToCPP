// phase: 08

// A C++ object that a go statement's func literal uses is boxed, like one a closure uses: the
// goroutine and the function share the one object, and the object is alive until the last user is
// done (the box is reference counted), also when the function has returned. Access is serialized
// with channels here; a std::mutex in a box works the same way.
package main

import (
	"fmt"

	"<mutex>"
	"<vector>"
)

func main() {
	v := std.vector[int]{}
	done := make(chan bool)
	go func() {
		v.push_back(5)
		v.push_back(6)
		done <- true
	}()
	<-done
	fmt.Println(int(v.size()), int(v[1]))

	// Several goroutines and a C++ mutex in a box.
	var mu std.mutex
	total := 0
	for i := 1; i <= 4; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				mu.lock()
				total += i
				mu.unlock()
			}
			done <- true
		}()
	}
	for i := 0; i < 4; i++ {
		<-done
	}
	fmt.Println(total)

	// The goroutine outlives the function that made the object.
	out := make(chan int)
	func() {
		w := std.vector[int]{1, 2, 3}
		go func() {
			out <- int(w.size())
		}()
	}()
	fmt.Println(<-out)
}

// Output:
// 2 6
// 1000
// 3
